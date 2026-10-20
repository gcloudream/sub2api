//go:build unit

package service

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"entgo.io/ent/dialect/sql/schema"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/migrate"
	"github.com/Wei-Shaw/sub2api/ent/paymentauditlog"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	_ "github.com/Wei-Shaw/sub2api/ent/runtime"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// This opt-in test is intended only for a disposable, empty PostgreSQL database.
// The database-name and empty-schema guards run before any DDL. No live payment
// provider, application setting, model endpoint, or production database is used.
func TestBlindBoxPostgresConcurrency(t *testing.T) {
	dsn := os.Getenv("BLINDBOX_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set BLINDBOX_TEST_POSTGRES_DSN for an isolated empty PostgreSQL database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	openClient := func() (*sql.DB, *dbent.Client) {
		db, err := sql.Open("postgres", dsn)
		require.NoError(t, err)
		db.SetMaxOpenConns(16)
		db.SetMaxIdleConns(16)
		t.Cleanup(func() { _ = db.Close() })
		require.NoError(t, db.PingContext(ctx))
		client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
		t.Cleanup(func() { _ = client.Close() })
		return db, client
	}
	db, firstClient := openClient()
	var databaseName string
	require.NoError(t, db.QueryRowContext(ctx, "SELECT current_database()").Scan(&databaseName))
	if !strings.HasPrefix(databaseName, "blindbox_test") && !strings.HasPrefix(databaseName, "fantasy_blindbox_test") {
		t.Fatal("refusing DDL: disposable database name must start with blindbox_test or fantasy_blindbox_test")
	}
	var tableCount int
	require.NoError(t, db.QueryRowContext(ctx, "SELECT count(*) FROM information_schema.tables WHERE table_schema = 'public' AND table_type = 'BASE TABLE'").Scan(&tableCount))
	if tableCount != 0 {
		t.Fatal("refusing DDL: disposable PostgreSQL public schema must be empty")
	}
	// Only the three tables used by the transaction core are needed. Foreign
	// keys to unrelated subscription tables are deliberately omitted here.
	require.NoError(t, migrate.Create(ctx, firstClient.Schema, []*schema.Table{
		migrate.UsersTable, migrate.PaymentOrdersTable, migrate.PaymentAuditLogsTable,
	}, migrate.WithForeignKeys(false)))
	_, secondClient := openClient()
	services := []*PaymentService{{entClient: firstClient}, {entClient: secondClient}}
	user, err := firstClient.User.Create().SetEmail("blindbox-pg@example.test").SetPasswordHash("isolated-test-only").SetUsername("blindbox-pg").SetBalance(123.45).Save(ctx)
	require.NoError(t, err)
	before, err := firstClient.User.Get(ctx, user.ID)
	require.NoError(t, err)
	actor := &User{ID: user.ID, Email: user.Email, Username: user.Username}
	const contenders = 16

	t.Run("cross_client_daily_reservation", func(t *testing.T) {
		start := make(chan struct{})
		results := make(chan error, contenders)
		var wg sync.WaitGroup
		for i := 0; i < contenders; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-start
				_, err := services[i%len(services)].createOrderInTx(ctx, CreateOrderRequest{
					UserID: user.ID, Amount: BlindBoxPrice, PaymentType: payment.TypeAlipay,
					OrderType: payment.OrderTypeBalance, RechargeMode: RechargeModeBlindBox,
				}, actor, nil, &PaymentConfig{BlindBoxEnabled: true, BalanceRechargeMultiplier: 6, MaxPendingOrders: 3},
					0, BlindBoxPrice, 0, BlindBoxPrice, &payment.InstanceSelection{
						InstanceID: "1", ProviderKey: payment.TypeEasyPay, PaymentMode: "qrcode",
						Config: map[string]string{"pid": "isolated-test-merchant"},
					})
				results <- err
			}(i)
		}
		close(start)
		wg.Wait()
		close(results)
		successes, rejected := 0, 0
		for err := range results {
			if err == nil {
				successes++
				continue
			}
			require.Equal(t, "BLIND_BOX_DAILY_LIMIT", infraerrors.Reason(err), "unexpected concurrent creation error: %v", err)
			rejected++
		}
		require.Equal(t, 1, successes)
		require.Equal(t, contenders-1, rejected)
		count, err := firstClient.PaymentOrder.Query().Where(paymentorder.UserIDEQ(user.ID)).Count(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, count)
	})

	t.Run("cross_client_atomic_paid_draw", func(t *testing.T) {
		order, err := firstClient.PaymentOrder.Query().Where(paymentorder.UserIDEQ(user.ID)).Only(ctx)
		require.NoError(t, err)
		require.Zero(t, order.Amount)
		require.Nil(t, order.PaidAt)
		require.NotContains(t, blindBoxMetadata(order), "draw_amount")
		start := make(chan struct{})
		type drawResult struct {
			changed bool
			err     error
		}
		results := make(chan drawResult, contenders)
		var wg sync.WaitGroup
		for i := 0; i < contenders; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-start
				changed, err := services[i%len(services)].markBlindBoxPaid(ctx, order, "isolated-confirmed-trade", BlindBoxPrice, time.Now())
				results <- drawResult{changed: changed, err: err}
			}(i)
		}
		close(start)
		wg.Wait()
		close(results)
		commits := 0
		for result := range results {
			require.NoError(t, result.err)
			if result.changed {
				commits++
			}
		}
		require.Equal(t, 1, commits)
		persisted, err := firstClient.PaymentOrder.Get(ctx, order.ID)
		require.NoError(t, err)
		draw, valid := validBlindBoxDraw(persisted)
		require.True(t, valid)
		require.Equal(t, float64(draw), persisted.Amount)
		require.Equal(t, OrderStatusPaid, persisted.Status)
		require.Equal(t, BlindBoxPrice, persisted.PayAmount)
		count, err := firstClient.PaymentAuditLog.Query().Where(
			paymentauditlog.OrderIDEQ(fmt.Sprint(order.ID)), paymentauditlog.ActionEQ("BLIND_BOX_DRAWN"),
		).Count(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, count)
		// Replays through the other application client keep the same result.
		changed, err := services[1].markBlindBoxPaid(ctx, order, "isolated-confirmed-trade", BlindBoxPrice, time.Now())
		require.NoError(t, err)
		require.False(t, changed)
		afterReplay, err := secondClient.PaymentOrder.Get(ctx, order.ID)
		require.NoError(t, err)
		require.Equal(t, persisted.ProviderSnapshot, afterReplay.ProviderSnapshot)
		require.Equal(t, persisted.Amount, afterReplay.Amount)
	})
	// Neither reservation nor draw credits a balance; only later fulfillment
	// may do that. The raw no-op row lock also leaves the user timestamp intact.
	after, err := firstClient.User.Get(ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, before.Balance, after.Balance)
	require.True(t, before.UpdatedAt.Equal(after.UpdatedAt), "user row lock changed updated_at")
}

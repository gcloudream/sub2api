//go:build unit

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"testing"
	"time"

	"entgo.io/ent"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentauditlog"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestBlindBoxExactProbabilityDistribution(t *testing.T) {
	counts := map[int]int{}
	sum := 0
	for ticket := 0; ticket < 10000; ticket++ {
		n, err := blindBoxAmountForTicket(ticket)
		require.NoError(t, err)
		counts[n]++
		sum += n
	}
	require.Len(t, counts, 51)
	for n := 50; n <= 100; n++ {
		require.Equal(t, blindBoxWeight(n), counts[n], "amount %d", n)
	}
	require.Equal(t, 616355, sum)
	require.InDelta(t, 61.6355, float64(sum)/10000, 1e-12)
	for _, ticket := range []int{-1, 10000} {
		_, err := blindBoxAmountForTicket(ticket)
		require.Error(t, err)
	}
	cfg := GetBlindBoxCheckout(&PaymentConfig{Enabled: true, BlindBoxEnabled: true})
	total := 0.0
	for _, p := range cfg.Probabilities {
		require.InDelta(t, p.Probability, float64(p.Max-p.Min+1)*p.PerIntegerProbability, 1e-12)
		total += p.Probability
	}
	require.InDelta(t, 1, total, 1e-12)
	for i := 0; i < 100; i++ {
		n, err := drawBlindBoxAmount()
		require.NoError(t, err)
		require.GreaterOrEqual(t, n, 50)
		require.LessOrEqual(t, n, 100)
	}
}

func TestBlindBoxRequestValidationAndOrdinaryRechargePreserved(t *testing.T) {
	svc := &PaymentService{}
	cfg := &PaymentConfig{Enabled: true, BlindBoxEnabled: true, BalanceRechargeMultiplier: 6, MinAmount: 1, MaxAmount: 500, RechargeFeeRate: 20}
	base := CreateOrderRequest{Amount: 9.9, OrderType: payment.OrderTypeBalance, RechargeMode: RechargeModeBlindBox}
	_, err := svc.validateOrderInput(context.Background(), base, cfg)
	require.NoError(t, err)
	amount, limit := calculateCreateOrderAmounts(base, nil, cfg)
	require.Zero(t, amount)
	require.Equal(t, 9.9, limit)
	for _, invalid := range []float64{0, 9.89, 9.90001, 10, math.NaN(), math.Inf(1)} {
		r := base
		r.Amount = invalid
		_, err = svc.validateOrderInput(context.Background(), r, cfg)
		require.Error(t, err)
	}
	for _, typ := range []string{payment.OrderTypeSubscription, "invalid"} {
		r := base
		r.OrderType = typ
		_, err = svc.validateOrderInput(context.Background(), r, cfg)
		require.Error(t, err)
	}
	r := base
	r.PlanID = 1
	_, err = svc.validateOrderInput(context.Background(), r, cfg)
	require.Error(t, err)
	r = base
	r.RechargeMode = "unknown"
	_, err = svc.validateOrderInput(context.Background(), r, cfg)
	require.Equal(t, "INVALID_RECHARGE_MODE", infraerrors.Reason(err))
	cfg.BlindBoxEnabled = false
	_, err = svc.validateOrderInput(context.Background(), base, cfg)
	require.Equal(t, "BLIND_BOX_DISABLED", infraerrors.Reason(err))
	base.RechargeMode = ""
	_, err = svc.validateOrderInput(context.Background(), base, cfg)
	require.NoError(t, err)
	amount, limit = calculateCreateOrderAmounts(base, nil, cfg)
	require.Equal(t, 59.4, amount)
	require.Equal(t, 9.9, limit)
	base.Amount = 20
	amount, _ = calculateCreateOrderAmounts(base, nil, cfg)
	require.Equal(t, 126.0, amount)
	require.False(t, (&PaymentConfigService{}).parsePaymentConfig(map[string]string{}).BlindBoxEnabled)
	require.True(t, (&PaymentConfigService{}).parsePaymentConfig(map[string]string{SettingBlindBoxEnabled: "true"}).BlindBoxEnabled)
}

func createBlindBoxTestUser(t *testing.T, client *dbent.Client) *User {
	t.Helper()
	u, err := client.User.Create().SetEmail("box@example.com").SetPasswordHash("hash").SetUsername("box-user").Save(context.Background())
	require.NoError(t, err)
	return &User{ID: u.ID, Email: u.Email, Username: u.Username}
}

func createBlindBoxTestOrder(t *testing.T, svc *PaymentService, u *User) *dbent.PaymentOrder {
	t.Helper()
	req := CreateOrderRequest{UserID: u.ID, Amount: 9.9, OrderType: payment.OrderTypeBalance, PaymentType: payment.TypeAlipay, RechargeMode: RechargeModeBlindBox}
	o, err := svc.createOrderInTx(context.Background(), req, u, nil, &PaymentConfig{BlindBoxEnabled: true, BalanceRechargeMultiplier: 6, MaxPendingOrders: 3}, 0, 9.9, 0, 9.9, &payment.InstanceSelection{InstanceID: "1", ProviderKey: payment.TypeEasyPay, PaymentMode: "qrcode", Config: map[string]string{"pid": "test-merchant"}})
	require.NoError(t, err)
	return o
}

func TestBlindBoxUnpaidSnapshotAndDailyReservation(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	svc := &PaymentService{entClient: client}
	u := createBlindBoxTestUser(t, client)
	userBefore, err := client.User.Get(ctx, u.ID)
	require.NoError(t, err)
	o := createBlindBoxTestOrder(t, svc, u)
	require.Zero(t, o.Amount)
	require.Equal(t, 9.9, o.PayAmount)
	require.Nil(t, o.PaidAt)
	require.False(t, BlindBoxResult(o).Drawn)
	require.Nil(t, BlindBoxResult(o).Amount)
	require.NotContains(t, blindBoxMetadata(o), "draw_amount")
	require.NotContains(t, blindBoxMetadata(o), "drawn_at")
	for _, status := range []string{OrderStatusPending, OrderStatusCancelled, OrderStatusExpired, OrderStatusFailed, OrderStatusRefunded} {
		_, err = client.PaymentOrder.UpdateOneID(o.ID).SetStatus(status).Save(ctx)
		require.NoError(t, err)
		tx, err := client.Tx(ctx)
		require.NoError(t, err)
		err = svc.reserveBlindBoxQuota(ctx, tx, u.ID, time.Now())
		require.Equal(t, "BLIND_BOX_DAILY_LIMIT", infraerrors.Reason(err))
		require.NoError(t, tx.Rollback())
	}
	tx, err := client.Tx(ctx)
	require.NoError(t, err)
	err = svc.reserveBlindBoxQuota(ctx, tx, u.ID, time.Now().AddDate(0, 0, 1))
	require.NoError(t, err)
	require.NoError(t, tx.Rollback())
	userAfter, err := client.User.Get(ctx, u.ID)
	require.NoError(t, err)
	require.Equal(t, userBefore.UpdatedAt, userAfter.UpdatedAt)
	require.Equal(t, userBefore.Balance, userAfter.Balance)
	start, end := blindBoxDayBounds(time.Date(2026, 10, 20, 16, 0, 0, 0, time.UTC))
	require.Equal(t, "2026-10-21T00:00:00+08:00", start.Format(time.RFC3339))
	require.Equal(t, 24*time.Hour, end.Sub(start))
}

func TestBlindBoxPaidDrawFrozenAndFulfillsExactlyOnce(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	ensurePaymentAuditOrderActionUniqueIndex(t, ctx, client)
	svc := &PaymentService{entClient: client}
	u := createBlindBoxTestUser(t, client)
	o := createBlindBoxTestOrder(t, svc, u)
	// Feature disable after purchase never changes the committed purchase rules.
	svc.configService = &PaymentConfigService{settingRepo: &paymentConfigSettingRepoStub{values: map[string]string{SettingBlindBoxEnabled: "false"}}}
	changed, err := svc.markBlindBoxPaid(ctx, o, "trade-1", 9.9, time.Now())
	require.NoError(t, err)
	require.True(t, changed)
	drawn, err := client.PaymentOrder.Get(ctx, o.ID)
	require.NoError(t, err)
	require.True(t, BlindBoxResult(drawn).Drawn)
	require.GreaterOrEqual(t, drawn.Amount, 50.0)
	require.LessOrEqual(t, drawn.Amount, 100.0)
	require.Equal(t, math.Trunc(drawn.Amount), drawn.Amount)
	require.Equal(t, 59.4, affiliateRebateBaseAmount(drawn))
	for i := 0; i < 3; i++ {
		changed, err = svc.markBlindBoxPaid(ctx, o, "trade-1", 9.9, time.Now())
		require.NoError(t, err)
		require.False(t, changed)
	}
	again, err := client.PaymentOrder.Get(ctx, o.ID)
	require.NoError(t, err)
	require.Equal(t, drawn.ProviderSnapshot, again.ProviderSnapshot)
	require.Equal(t, drawn.Amount, again.Amount)
	auditCount, err := client.PaymentAuditLog.Query().Where(paymentauditlog.ActionEQ("BLIND_BOX_DRAWN")).Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, auditCount)
	repo := &paymentFulfillmentRedeemRepo{}
	credited := 0.0
	userRepo := &mockUserRepo{getByIDUser: &User{ID: u.ID}}
	userRepo.updateBalanceFn = func(_ context.Context, id int64, amount float64) error {
		require.Equal(t, u.ID, id)
		credited += amount
		return nil
	}
	cache := &paymentFulfillmentRedeemCacheStub{}
	svc.redeemService = NewRedeemService(repo, userRepo, nil, cache, nil, client, nil, nil)
	svc.userRepo = userRepo
	require.NoError(t, svc.ExecuteBalanceFulfillment(ctx, o.ID))
	require.NoError(t, svc.ExecuteBalanceFulfillment(ctx, o.ID))
	require.Equal(t, drawn.Amount, credited)
	require.Equal(t, 1, repo.createCalls)
	require.Len(t, repo.useCalls, 1)
	completed, err := client.PaymentOrder.Get(ctx, o.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusCompleted, completed.Status)
	require.Equal(t, drawn.Amount, completed.Amount)
}

func TestBlindBoxLatePaymentAndInvalidFrozenRules(t *testing.T) {
	for _, status := range []string{OrderStatusCancelled, OrderStatusExpired, OrderStatusFailed} {
		t.Run(status, func(t *testing.T) {
			ctx := context.Background()
			client := newPaymentConfigServiceTestClient(t)
			svc := &PaymentService{entClient: client}
			u := createBlindBoxTestUser(t, client)
			o := createBlindBoxTestOrder(t, svc, u)
			o, err := client.PaymentOrder.UpdateOneID(o.ID).SetStatus(status).SetUpdatedAt(time.Now().Add(-24 * time.Hour)).Save(ctx)
			require.NoError(t, err)
			changed, err := svc.markBlindBoxPaid(ctx, o, "late-trade", 9.9, time.Now())
			require.NoError(t, err)
			require.True(t, changed)
			drawn, err := client.PaymentOrder.Get(ctx, o.ID)
			require.NoError(t, err)
			require.True(t, BlindBoxResult(drawn).Drawn)
		})
	}
	t.Run("unknown rules fail closed", func(t *testing.T) {
		ctx := context.Background()
		client := newPaymentConfigServiceTestClient(t)
		svc := &PaymentService{entClient: client}
		u := createBlindBoxTestUser(t, client)
		o := createBlindBoxTestOrder(t, svc, u)
		o.ProviderSnapshot[blindBoxSnapshotKey].(map[string]any)["rules_version"] = "unknown-version"
		o, err := client.PaymentOrder.UpdateOneID(o.ID).SetProviderSnapshot(o.ProviderSnapshot).Save(ctx)
		require.NoError(t, err)
		changed, err := svc.markBlindBoxPaid(ctx, o, "trade", 9.9, time.Now())
		require.Error(t, err)
		require.False(t, changed)
		current, err := client.PaymentOrder.Get(ctx, o.ID)
		require.NoError(t, err)
		require.Zero(t, current.Amount)
		require.Nil(t, current.PaidAt)
	})
}

func TestBlindBoxConcurrentCreationCannotBypassQuota(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	svc := &PaymentService{entClient: client}
	u := createBlindBoxTestUser(t, client)
	var wg sync.WaitGroup
	success := make(chan bool, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.createOrderInTx(ctx, CreateOrderRequest{UserID: u.ID, Amount: 9.9, OrderType: payment.OrderTypeBalance, RechargeMode: RechargeModeBlindBox, PaymentType: payment.TypeAlipay}, u, nil, &PaymentConfig{MaxPendingOrders: 3, BalanceRechargeMultiplier: 6}, 0, 9.9, 0, 9.9, nil)
			success <- err == nil
		}()
	}
	wg.Wait()
	close(success)
	n := 0
	for ok := range success {
		if ok {
			n++
		}
	}
	require.Equal(t, 1, n)
	count, err := client.PaymentOrder.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func TestBlindBoxPendingResumeAndLegacyOrdersStayOrdinary(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	svc := &PaymentService{entClient: client}
	u := createBlindBoxTestUser(t, client)
	o := createBlindBoxTestOrder(t, svc, u)
	o, err := client.PaymentOrder.UpdateOneID(o.ID).SetQrCode("https://example.test/same-qr").Save(ctx)
	require.NoError(t, err)
	r, err := svc.pendingBlindBoxResponse(ctx, CreateOrderRequest{UserID: u.ID})
	require.NoError(t, err)
	require.NotNil(t, r)
	require.Equal(t, o.ID, r.OrderID)
	require.Equal(t, *o.QrCode, r.QRCode)
	require.False(t, r.BlindBox.Drawn)
	old := &dbent.PaymentOrder{OrderType: payment.OrderTypeBalance, Amount: 59.4, PayAmount: 9.9}
	require.Equal(t, RechargeModeBalance, PaymentOrderRechargeMode(old))
	require.Nil(t, BlindBoxResult(old))
	require.Equal(t, 59.4, affiliateRebateBaseAmount(old))
	body, err := json.Marshal(r)
	require.NoError(t, err)
	require.NotContains(t, string(body), "provider_snapshot")
	require.NotContains(t, string(body), "merchant_id")
	require.Contains(t, string(body), `"recharge_mode":"blind_box"`)
}

func TestBlindBoxBackgroundRecoverySelectsOnlyCommittedBlindBox(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	svc := &PaymentService{entClient: client}
	u := createBlindBoxTestUser(t, client)
	o := createBlindBoxTestOrder(t, svc, u)
	changed, err := svc.markBlindBoxPaid(ctx, o, "trade", 9.9, time.Now())
	require.NoError(t, err)
	require.True(t, changed)
	// Model a process crash after PAID commit; redeemed balance already exists.
	drawn, err := client.PaymentOrder.Get(ctx, o.ID)
	require.NoError(t, err)
	usedBy := u.ID
	repo := &paymentFulfillmentRedeemRepo{codesByCode: map[string]*RedeemCode{drawn.RechargeCode: {Code: drawn.RechargeCode, Type: RedeemTypeBalance, Value: drawn.Amount, Status: StatusUsed, UsedBy: &usedBy}}}
	svc.redeemService = &RedeemService{redeemRepo: repo}
	legacy, err := client.PaymentOrder.Create().SetUserID(u.ID).SetUserEmail(u.Email).SetUserName(u.Username).SetAmount(59.4).SetPayAmount(9.9).SetRechargeCode("legacy").SetOutTradeNo(fmt.Sprintf("legacy-%d", time.Now().UnixNano())).SetPaymentType(payment.TypeAlipay).SetPaymentTradeNo("legacy").SetOrderType(payment.OrderTypeBalance).SetStatus(OrderStatusPaid).SetPaidAt(time.Now()).SetExpiresAt(time.Now()).SetClientIP("").SetSrcHost("").Save(ctx)
	require.NoError(t, err)
	recovered, err := svc.RecoverPaidBlindBoxOrders(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, recovered)
	current, err := client.PaymentOrder.Get(ctx, legacy.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusPaid, current.Status)
}

func TestBlindBoxConcurrentPaidNotificationsPersistOneResult(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	ensurePaymentAuditOrderActionUniqueIndex(t, ctx, client)
	svc := &PaymentService{entClient: client}
	u := createBlindBoxTestUser(t, client)
	o := createBlindBoxTestOrder(t, svc, u)
	var wg sync.WaitGroup
	committed := make(chan bool, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, _ := svc.markBlindBoxPaid(ctx, o, "same-trade", 9.9, time.Now())
			committed <- ok
		}()
	}
	wg.Wait()
	close(committed)
	wins := 0
	for ok := range committed {
		if ok {
			wins++
		}
	}
	require.Equal(t, 1, wins)
	current, err := client.PaymentOrder.Get(ctx, o.ID)
	require.NoError(t, err)
	before := current.Amount
	require.True(t, BlindBoxResult(current).Drawn)
	changed, err := svc.markBlindBoxPaid(ctx, o, "same-trade", 9.9, time.Now())
	require.NoError(t, err)
	require.False(t, changed)
	current, err = client.PaymentOrder.Get(ctx, o.ID)
	require.NoError(t, err)
	require.Equal(t, before, current.Amount)
	count, err := client.PaymentAuditLog.Query().Where(paymentauditlog.ActionEQ("BLIND_BOX_DRAWN")).Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func TestBlindBoxRecoveryDoesNotStarveBehindInvalidDraws(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	ensurePaymentAuditOrderActionUniqueIndex(t, ctx, client)
	svc := &PaymentService{entClient: client}
	u := createBlindBoxTestUser(t, client)
	// A batch of corrupt records must be surfaced for manual review and excluded
	// from later recovery queries, while a valid paid purchase behind it progresses.
	for i := 0; i < 20; i++ {
		_, err := client.PaymentOrder.Create().SetUserID(u.ID).SetUserEmail(u.Email).SetUserName(u.Username).SetAmount(0).SetPayAmount(9.9).SetRechargeCode(fmt.Sprintf("corrupt-%d", i)).SetOutTradeNo(fmt.Sprintf("corrupt-out-%d", i)).SetPaymentType(payment.TypeAlipay).SetPaymentTradeNo("trade").SetOrderType(payment.OrderTypeBalance).SetStatus(OrderStatusPaid).SetPaidAt(time.Now()).SetExpiresAt(time.Now()).SetProviderSnapshot(map[string]any{blindBoxSnapshotKey: map[string]any{"mode": RechargeModeBlindBox, "rules_version": "unknown-version"}}).SetClientIP("").SetSrcHost("").Save(ctx)
		require.NoError(t, err)
	}
	// Make another user to avoid the daily creation guard used by real purchases.
	user2, err := client.User.Create().SetEmail("box2@example.com").SetPasswordHash("hash").SetUsername("box2-user").Save(ctx)
	require.NoError(t, err)
	o := createBlindBoxTestOrder(t, svc, &User{ID: user2.ID, Email: user2.Email, Username: user2.Username})
	changed, err := svc.markBlindBoxPaid(ctx, o, "paid-behind-invalid", 9.9, time.Now())
	require.NoError(t, err)
	require.True(t, changed)
	drawn, err := client.PaymentOrder.Get(ctx, o.ID)
	require.NoError(t, err)
	usedBy := user2.ID
	repo := &paymentFulfillmentRedeemRepo{codesByCode: map[string]*RedeemCode{drawn.RechargeCode: {Code: drawn.RechargeCode, Type: RedeemTypeBalance, Value: drawn.Amount, Status: StatusUsed, UsedBy: &usedBy}}}
	svc.redeemService = &RedeemService{redeemRepo: repo}
	recovered, err := svc.RecoverPaidBlindBoxOrders(ctx)
	require.NoError(t, err)
	require.Zero(t, recovered)
	recovered, err = svc.RecoverPaidBlindBoxOrders(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, recovered)
	count, err := client.PaymentAuditLog.Query().Where(paymentauditlog.ActionEQ("BLIND_BOX_RECOVERY_INVALID")).Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 20, count)
}

func TestBlindBoxLostPaidCASKeepsGatewayRetryable(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	svc := &PaymentService{entClient: client}
	u := createBlindBoxTestUser(t, client)
	o := createBlindBoxTestOrder(t, svc, u)
	client.PaymentOrder.Use(func(next ent.Mutator) ent.Mutator {
		return ent.MutateFunc(func(ctx context.Context, m ent.Mutation) (ent.Value, error) {
			mutation := m.(*dbent.PaymentOrderMutation)
			if status, ok := mutation.Status(); ok && status == OrderStatusPaid {
				// Simulate an old cancellation writer winning after markBlindBoxPaid read
				// the order but before its final compare-and-swap.
				_, err := mutation.Client().ExecContext(ctx, "UPDATE payment_orders SET status = $1 WHERE id = $2", OrderStatusCancelled, o.ID)
				if err != nil {
					return nil, err
				}
			}
			return next.Mutate(ctx, m)
		})
	})
	changed, err := svc.markBlindBoxPaid(ctx, o, "real-payment", 9.9, time.Now())
	require.False(t, changed)
	require.Equal(t, "BLIND_BOX_ORDER_CHANGED", infraerrors.Reason(err))
	current, err := client.PaymentOrder.Get(ctx, o.ID)
	require.NoError(t, err)
	require.Nil(t, current.PaidAt)
	require.Zero(t, current.Amount)
	require.NotContains(t, blindBoxMetadata(current), "draw_amount")
	count, err := client.PaymentAuditLog.Query().Where(paymentauditlog.ActionEQ("BLIND_BOX_DRAWN")).Count(ctx)
	require.NoError(t, err)
	require.Zero(t, count)
}

func TestPaymentSubjectDescribesBlindBoxAndPreservesOrdinaryTitle(t *testing.T) {
	svc := &PaymentService{}
	cfg := &PaymentConfig{ProductNamePrefix: "幻想家", ProductNameSuffix: "站内额度"}
	blind := svc.buildCreateOrderPaymentSubject(CreateOrderRequest{RechargeMode: RechargeModeBlindBox}, nil, 9.9, cfg, nil)
	require.Equal(t, "幻想家 额度盲盒 9.90 CNY（随机50-100额度） 站内额度", blind)
	ordinary := svc.buildCreateOrderPaymentSubject(CreateOrderRequest{}, nil, 9.9, cfg, nil)
	require.Equal(t, svc.buildPaymentSubject(nil, 9.9, cfg, nil), ordinary)
	require.NotContains(t, ordinary, "盲盒")
}

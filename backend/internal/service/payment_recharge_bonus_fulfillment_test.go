//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

func TestRechargeBonusFulfillmentUsesStoredAmountExactlyOnce(t *testing.T) {
	for _, tc := range []struct {
		name         string
		storedCredit float64
		payAmount    float64
		bonusAmount  float64
	}{
		{name: "Fantasy fixed bonus order", storedCredit: 126, payAmount: 20, bonusAmount: 6},
		{name: "official threshold order", storedCredit: 144, payAmount: 20, bonusAmount: 24},
		{name: "official discount order", storedCredit: 120, payAmount: 16, bonusAmount: 24},
		{name: "existing order keeps its original credit", storedCredit: 120, payAmount: 20},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			client := newPaymentConfigServiceTestClient(t)
			ensurePaymentAuditOrderActionUniqueIndex(t, ctx, client)
			order := createPaymentFulfillmentSubscriptionOrder(t, ctx, client, OrderStatusPaid, time.Now())
			order, err := client.PaymentOrder.UpdateOneID(order.ID).
				SetOrderType(payment.OrderTypeBalance).
				ClearPlanID().ClearSubscriptionGroupID().ClearSubscriptionDays().
				SetAmount(tc.storedCredit).SetPayAmount(tc.payAmount).SetBonusAmount(tc.bonusAmount).
				Save(ctx)
			require.NoError(t, err)

			redeemRepo := &paymentFulfillmentRedeemRepo{}
			credited := 0.0
			creditCalls := 0
			userRepo := &mockUserRepo{getByIDUser: &User{ID: order.UserID}}
			userRepo.updateBalanceFn = func(_ context.Context, id int64, amount float64) error {
				require.Equal(t, order.UserID, id)
				credited += amount
				creditCalls++
				return nil
			}
			redeemService := NewRedeemService(redeemRepo, userRepo, nil, &paymentFulfillmentRedeemCacheStub{}, nil, client, nil, nil)
			svc := &PaymentService{entClient: client, redeemService: redeemService, userRepo: userRepo}

			require.NoError(t, svc.ExecuteBalanceFulfillment(ctx, order.ID))
			require.NoError(t, svc.ExecuteBalanceFulfillment(ctx, order.ID))
			require.Equal(t, tc.storedCredit, credited)
			require.Equal(t, 1, creditCalls)
			require.Equal(t, 1, redeemRepo.createCalls)
			require.Len(t, redeemRepo.useCalls, 1)
			reloaded, err := client.PaymentOrder.Get(ctx, order.ID)
			require.NoError(t, err)
			require.Equal(t, OrderStatusCompleted, reloaded.Status)
			require.Equal(t, tc.storedCredit, reloaded.Amount)
			require.Equal(t, tc.payAmount, reloaded.PayAmount)
			require.Equal(t, tc.bonusAmount, reloaded.BonusAmount)
		})
	}
}

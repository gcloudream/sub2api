//go:build unit

package handler

import (
	"encoding/json"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestBlindBoxOrderResponsesExposeOnlyCommittedDraw(t *testing.T) {
	now := time.Now()
	order := &dbent.PaymentOrder{ID: 1, OrderType: "balance", PayAmount: 9.9, Status: service.OrderStatusPending, ProviderSnapshot: map[string]any{
		"merchant_id": "not-public", "fantasy_recharge": map[string]any{"mode": "blind_box", "rules_version": service.BlindBoxRulesVersion},
	}}
	pending := sanitizePaymentOrderForResponse(order)
	require.Equal(t, "blind_box", pending.RechargeMode)
	require.False(t, pending.BlindBox.Drawn)
	require.Nil(t, pending.BlindBox.Amount)
	body, err := json.Marshal(pending)
	require.NoError(t, err)
	require.NotContains(t, string(body), "merchant_id")
	require.NotContains(t, string(body), "not-public")
	require.NotContains(t, string(body), "provider_snapshot")
	order.Amount = 64
	order.PaidAt = &now
	order.Status = service.OrderStatusCompleted
	order.ProviderSnapshot["fantasy_recharge"].(map[string]any)["draw_amount"] = 64
	order.ProviderSnapshot["fantasy_recharge"].(map[string]any)["drawn_at"] = now.Format(time.RFC3339Nano)
	complete := sanitizePaymentOrderForResponse(order)
	require.True(t, complete.BlindBox.Drawn)
	require.Equal(t, 64, *complete.BlindBox.Amount)
	public := buildPublicOrderResult(order)
	require.Equal(t, "blind_box", public.RechargeMode)
	require.Equal(t, 64, *public.BlindBox.Amount)
	ordinary := sanitizePaymentOrderForResponse(&dbent.PaymentOrder{OrderType: "balance", Amount: 59.4, PayAmount: 9.9})
	require.Equal(t, "balance", ordinary.RechargeMode)
	require.Nil(t, ordinary.BlindBox)
}

func TestBlindBoxCreateRequestRequiresExplicitMode(t *testing.T) {
	var req CreateOrderRequest
	require.NoError(t, json.Unmarshal([]byte(`{"amount":9.9,"payment_type":"alipay","recharge_mode":"blind_box"}`), &req))
	require.Equal(t, "blind_box", req.RechargeMode)
	var ordinary CreateOrderRequest
	require.NoError(t, json.Unmarshal([]byte(`{"amount":9.9,"payment_type":"alipay"}`), &ordinary))
	require.Empty(t, ordinary.RechargeMode)
}

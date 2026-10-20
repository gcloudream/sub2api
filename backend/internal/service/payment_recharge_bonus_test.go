package service

import (
	"math"
	"reflect"
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
)

func TestRechargeBonusTiersAndCredits(t *testing.T) {
	t.Parallel()
	tests := []struct {
		amount  float64
		percent float64
		credit  float64
	}{
		{20, 5, 126},
		{50, 7, 321},
		{100, 10, 660},
		{200, 12, 1344},
		{500, 15, 3450},
	}
	tiers := GetRechargeBonusTiers()
	if len(tiers) != len(tests) {
		t.Fatalf("tier count = %d, want %d", len(tiers), len(tests))
	}
	for i, tt := range tests {
		if tiers[i].Amount != tt.amount || tiers[i].BonusPercent != tt.percent {
			t.Errorf("tier[%d] = %+v, want amount %v with %v percent", i, tiers[i], tt.amount, tt.percent)
		}
		if got := calculateCreditedBalance(tt.amount, 6); got != tt.credit {
			t.Errorf("credited balance for %v = %v, want %v", tt.amount, got, tt.credit)
		}
	}
}

func TestRechargeBonusRequiresExactAmount(t *testing.T) {
	t.Parallel()
	for _, amount := range []float64{1, 5, 9.9, 10, 19.99, 20.01, 49.99, 50.01, 99.99, 100.01, 199.99, 200.01, 499.99, 500.01, math.Nextafter(20, 0), math.Nextafter(20, 21)} {
		want := math.Round(amount*6*100) / 100
		if got := calculateCreditedBalance(amount, 6); got != want {
			t.Errorf("custom amount %.17g credited %v, want base credit %v", amount, got, want)
		}
	}
}

func TestRechargeBonusUsesRoundedBaseCredit(t *testing.T) {
	t.Parallel()
	if got := calculateCreditedBalance(50, 0.14); got != 7.49 {
		t.Errorf("credited balance = %v, want 7.49", got)
	}
	if got := calculateCreditedBalance(20, 0.33333); got != 7 {
		t.Errorf("rounded credited balance = %v, want 7", got)
	}
	// Base 6.292 rounds to 6.29, then its 5% bonus rounds to 0.31.
	// Rounding only once after multiplying would incorrectly produce 6.61.
	if got := calculateCreditedBalance(20, 0.3146); got != 6.6 {
		t.Errorf("base plus rounded bonus = %v, want 6.60", got)
	}
}

func TestRechargeBonusTierCopiesCannotChangePrices(t *testing.T) {
	t.Parallel()
	tiers := GetRechargeBonusTiers()
	tiers[0].BonusPercent = 100
	if got := calculateCreditedBalance(20, 6); got != 126 {
		t.Fatalf("caller changed credited balance to %v, want 126", got)
	}
	if reflect.DeepEqual(tiers, GetRechargeBonusTiers()) {
		t.Fatal("expected independent tier slices")
	}
}

func TestRechargeBonusDoesNotChangePaymentOrSubscriptionPrices(t *testing.T) {
	t.Parallel()
	cfg := &PaymentConfig{BalanceRechargeMultiplier: 6, RechargeFeeRate: 2.5}
	credit, limit := calculateCreateOrderAmounts(CreateOrderRequest{Amount: 20, OrderType: payment.OrderTypeBalance}, nil, cfg)
	if credit != 126 || limit != 20 {
		t.Fatalf("balance amounts = (%v, %v), want credit 126 and original limit amount 20", credit, limit)
	}
	payText, payAmount, err := calculateCreateOrderPayAmountForOrderType(limit, cfg.RechargeFeeRate, "CNY", payment.OrderTypeBalance, 7.15)
	if err != nil || payText != "20.50" || payAmount != 20.5 {
		t.Fatalf("gateway amount = (%q, %v, %v), want only original amount plus fee", payText, payAmount, err)
	}
	credit, limit = calculateCreateOrderAmounts(CreateOrderRequest{Amount: 500, OrderType: payment.OrderTypeSubscription}, &dbent.SubscriptionPlan{Price: 20}, cfg)
	if credit != 20 || limit != 20 {
		t.Fatalf("subscription amounts = (%v, %v), want unchanged plan price 20", credit, limit)
	}
}

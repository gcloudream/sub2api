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
	tiers := GetFixedRechargeBonusTiers(nil)
	if len(tiers) != len(tests) {
		t.Fatalf("tier count = %d, want %d", len(tiers), len(tests))
	}
	for i, tt := range tests {
		if tiers[i].Amount != tt.amount || tiers[i].BonusPercent != tt.percent {
			t.Errorf("tier[%d] = %+v, want amount %v with %v percent", i, tiers[i], tt.amount, tt.percent)
		}
		if got := calculateFixedRechargeCredit(tt.amount, 6); got != tt.credit {
			t.Errorf("credited balance for %v = %v, want %v", tt.amount, got, tt.credit)
		}
	}
}

func TestRechargeBonusRequiresExactAmount(t *testing.T) {
	t.Parallel()
	for _, amount := range []float64{1, 5, 9.9, 10, 19.99, 20.01, 49.99, 50.01, 99.99, 100.01, 199.99, 200.01, 499.99, 500.01, math.Nextafter(20, 0), math.Nextafter(20, 21)} {
		want := math.Round(amount*6*100) / 100
		if got := calculateFixedRechargeCredit(amount, 6); got != want {
			t.Errorf("custom amount %.17g credited %v, want base credit %v", amount, got, want)
		}
	}
}

func TestRechargeBonusUsesRoundedBaseCredit(t *testing.T) {
	t.Parallel()
	if got := calculateFixedRechargeCredit(50, 0.14); got != 7.49 {
		t.Errorf("credited balance = %v, want 7.49", got)
	}
	if got := calculateFixedRechargeCredit(20, 0.33333); got != 7 {
		t.Errorf("rounded credited balance = %v, want 7", got)
	}
	// Base 6.292 rounds to 6.29, then its 5% bonus rounds to 0.31.
	// Rounding only once after multiplying would incorrectly produce 6.61.
	if got := calculateFixedRechargeCredit(20, 0.3146); got != 6.6 {
		t.Errorf("base plus rounded bonus = %v, want 6.60", got)
	}
}

func TestRechargeDecimalRoundingParity(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		amount     float64
		multiplier float64
		tiers      []RechargeBonusTier
		mode       string
		base       float64
		want       rechargeBonusQuote
	}{
		{name: "fixed 20 at half cent", amount: 20, multiplier: 1.00125, base: 20.03,
			want: rechargeBonusQuote{PayBase: 20, Credited: 21.03, Bonus: 1, Percent: 5}},
		{name: "fixed 50 at half cent", amount: 50, multiplier: 1.0009, base: 50.05,
			want: rechargeBonusQuote{PayBase: 50, Credited: 53.55, Bonus: 3.5, Percent: 7}},
		{name: "fixed below half cent", amount: 20, multiplier: 1.001249, base: 20.02,
			want: rechargeBonusQuote{PayBase: 20, Credited: 21.02, Bonus: 1, Percent: 5}},
		{name: "threshold 20 at half cent", amount: 20, multiplier: 1.00125, base: 20.03,
			tiers: []RechargeBonusTier{{MinAmount: 10, BonusPercent: 5}},
			want:  rechargeBonusQuote{PayBase: 20, Credited: 21.03, Bonus: 1, Percent: 5}},
		{name: "threshold 50 at half cent", amount: 50, multiplier: 1.0009, base: 50.05,
			tiers: []RechargeBonusTier{{MinAmount: 10, BonusPercent: 7}},
			want:  rechargeBonusQuote{PayBase: 50, Credited: 53.55, Bonus: 3.5, Percent: 7}},
		{name: "threshold below half cent", amount: 20, multiplier: 1.001249, base: 20.02,
			tiers: []RechargeBonusTier{{MinAmount: 10, BonusPercent: 5}},
			want:  rechargeBonusQuote{PayBase: 20, Credited: 21.02, Bonus: 1, Percent: 5}},
		{name: "discount base at half cent", amount: 20, multiplier: 1.00125, base: 20.03, mode: RechargeBonusModeDiscount,
			tiers: []RechargeBonusTier{{MinAmount: 10, BonusPercent: 5}},
			want:  rechargeBonusQuote{PayBase: 19, Credited: 20.03, Bonus: 1.01, Percent: 5}},
		{name: "discount base below half cent", amount: 20, multiplier: 1.001249, base: 20.02, mode: RechargeBonusModeDiscount,
			tiers: []RechargeBonusTier{{MinAmount: 10, BonusPercent: 5}},
			want:  rechargeBonusQuote{PayBase: 19, Credited: 20.02, Bonus: 1, Percent: 5}},
		{name: "discount payable at half cent", amount: 20.01, multiplier: 1.00125, base: 20.04, mode: RechargeBonusModeDiscount,
			tiers: []RechargeBonusTier{{MinAmount: 10, BonusPercent: 50}},
			want:  rechargeBonusQuote{PayBase: 10.01, Credited: 20.04, Bonus: 10.02, Percent: 50}},
		{name: "discount tiny payable stays positive", amount: 10, multiplier: 1, base: 10, mode: RechargeBonusModeDiscount,
			tiers: []RechargeBonusTier{{MinAmount: 1, BonusPercent: 99.95}},
			want:  rechargeBonusQuote{PayBase: 0.01, Credited: 10, Bonus: 9.99, Percent: 99.95}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := calculateCreditedBalance(tc.amount, tc.multiplier); got != tc.base {
				t.Fatalf("rounded base = %v, want %v", got, tc.base)
			}
			cfg := &PaymentConfig{BalanceRechargeMultiplier: tc.multiplier, RechargeBonusTiers: tc.tiers, RechargeBonusMode: tc.mode}
			if got := quoteRechargeBonus(cfg, tc.amount, "CNY"); got != tc.want {
				t.Fatalf("decimal quote = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestRechargeBonusTierCopiesCannotChangePrices(t *testing.T) {
	t.Parallel()
	tiers := GetFixedRechargeBonusTiers(nil)
	tiers[0].BonusPercent = 100
	if got := calculateFixedRechargeCredit(20, 6); got != 126 {
		t.Fatalf("caller changed credited balance to %v, want 126", got)
	}
	if reflect.DeepEqual(tiers, GetFixedRechargeBonusTiers(nil)) {
		t.Fatal("expected independent tier slices")
	}
}

func TestRechargeBonusDoesNotChangePaymentOrSubscriptionPrices(t *testing.T) {
	t.Parallel()
	cfg := &PaymentConfig{BalanceRechargeMultiplier: 6, RechargeFeeRate: 2.5}
	credit, limit, _ := calculateCreateOrderAmounts(CreateOrderRequest{Amount: 20, OrderType: payment.OrderTypeBalance}, nil, cfg, "CNY")
	if credit != 126 || limit != 20 {
		t.Fatalf("balance amounts = (%v, %v), want credit 126 and original limit amount 20", credit, limit)
	}
	payText, payAmount, err := calculateCreateOrderPayAmountForOrderType(limit, cfg.RechargeFeeRate, "CNY", payment.OrderTypeBalance, 7.15)
	if err != nil || payText != "20.50" || payAmount != 20.5 {
		t.Fatalf("gateway amount = (%q, %v, %v), want only original amount plus fee", payText, payAmount, err)
	}
	credit, limit, _ = calculateCreateOrderAmounts(CreateOrderRequest{Amount: 500, OrderType: payment.OrderTypeSubscription}, &dbent.SubscriptionPlan{Price: 20}, cfg, "CNY")
	if credit != 20 || limit != 20 {
		t.Fatalf("subscription amounts = (%v, %v), want unchanged plan price 20", credit, limit)
	}
}

func TestRechargeOfferSourcesNeverStack(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		amount   float64
		tiers    []RechargeBonusTier
		mode     string
		credited float64
		payBase  float64
		bonus    float64
	}{
		{name: "Fantasy exact default", amount: 20, credited: 126, payBase: 20, bonus: 6},
		{name: "nearby amount has no fixed bonus", amount: 20.01, credited: 120.06, payBase: 20.01},
		{name: "official threshold overrides fixed", amount: 20, tiers: []RechargeBonusTier{{MinAmount: 10, BonusPercent: 20}}, credited: 144, payBase: 20, bonus: 24},
		{name: "unmatched official threshold suppresses fixed", amount: 20, tiers: []RechargeBonusTier{{MinAmount: 50, BonusPercent: 20}}, credited: 120, payBase: 20},
		{name: "explicit zero percent suppresses fixed", amount: 20, tiers: []RechargeBonusTier{{MinAmount: 0, BonusPercent: 0}}, credited: 120, payBase: 20},
		{name: "official discount does not inherit fixed bonus", amount: 20, tiers: []RechargeBonusTier{{MinAmount: 10, BonusPercent: 20}}, mode: RechargeBonusModeDiscount, credited: 120, payBase: 16, bonus: 24},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &PaymentConfig{BalanceRechargeMultiplier: 6, RechargeBonusTiers: tc.tiers, RechargeBonusMode: tc.mode}
			credited, payBase, bonus := calculateCreateOrderAmounts(CreateOrderRequest{OrderType: payment.OrderTypeBalance, Amount: tc.amount}, nil, cfg, "CNY")
			if credited != tc.credited || payBase != tc.payBase || bonus != tc.bonus {
				t.Fatalf("order quote = (%v, %v, %v), want (%v, %v, %v)", credited, payBase, bonus, tc.credited, tc.payBase, tc.bonus)
			}
			fixed := GetFixedRechargeBonusTiers(cfg)
			if (len(fixed) > 0) != (len(tc.tiers) == 0) {
				t.Fatalf("fixed offer visibility does not match active source: %v", fixed)
			}
		})
	}
}

func TestBlindBoxAndSubscriptionBypassConfiguredPromotions(t *testing.T) {
	t.Parallel()
	cfg := &PaymentConfig{BalanceRechargeMultiplier: 6, RechargeBonusMode: RechargeBonusModeDiscount,
		RechargeBonusTiers: []RechargeBonusTier{{MinAmount: 0, BonusPercent: 50}}}
	amount, payBase, bonus := calculateCreateOrderAmounts(CreateOrderRequest{OrderType: payment.OrderTypeBalance, Amount: BlindBoxPrice, RechargeMode: RechargeModeBlindBox}, nil, cfg, "CNY")
	if amount != 0 || payBase != BlindBoxPrice || bonus != 0 {
		t.Fatalf("unpaid blind box received a promotion or draw: (%v, %v, %v)", amount, payBase, bonus)
	}
	amount, payBase, bonus = calculateCreateOrderAmounts(CreateOrderRequest{OrderType: payment.OrderTypeSubscription, Amount: 500}, &dbent.SubscriptionPlan{Price: 20}, cfg, "CNY")
	if amount != 20 || payBase != 20 || bonus != 0 {
		t.Fatalf("subscription received a balance promotion: (%v, %v, %v)", amount, payBase, bonus)
	}
}

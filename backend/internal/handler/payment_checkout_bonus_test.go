package handler

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestCheckoutInfoPublishesRechargeBonusTiers(t *testing.T) {
	t.Parallel()
	raw, err := json.Marshal(checkoutInfoResponse{RechargeFixedBonusTiers: service.GetFixedRechargeBonusTiers(nil)})
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Tiers []struct {
			Amount       float64 `json:"amount"`
			BonusPercent float64 `json:"bonus_percent"`
		} `json:"recharge_fixed_bonus_tiers"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	want := []float64{20, 5, 50, 7, 100, 10, 200, 12, 500, 15}
	got := make([]float64, 0, len(payload.Tiers)*2)
	for _, tier := range payload.Tiers {
		got = append(got, tier.Amount, tier.BonusPercent)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("public recharge bonus tiers = %v, want %v", got, want)
	}
}

func TestCheckoutInfoKeepsFixedAndThresholdContractsSeparate(t *testing.T) {
	t.Parallel()
	cfg := &service.PaymentConfig{RechargeBonusTiers: []service.RechargeBonusTier{{MinAmount: 10, BonusPercent: 20}}}
	raw, err := json.Marshal(checkoutInfoResponse{
		RechargeBonusTiers:      cfg.RechargeBonusTiers,
		RechargeFixedBonusTiers: service.GetFixedRechargeBonusTiers(cfg),
	})
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Fixed     []service.FixedRechargeBonusTier `json:"recharge_fixed_bonus_tiers"`
		Threshold []service.RechargeBonusTier      `json:"recharge_bonus_tiers"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Fixed == nil || len(payload.Fixed) != 0 || !reflect.DeepEqual(payload.Threshold, cfg.RechargeBonusTiers) {
		t.Fatalf("checkout mixed or lost offer sources: %s", raw)
	}
}

package service

import (
	"math"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/shopspring/decimal"
)

const defaultBalanceRechargeMultiplier = 1.0

// RechargeBonusTier grants extra balance for an exact recharge amount. Percent
// values are whole percentages (5 means an additional 5% of the base credit).
type RechargeBonusTier struct {
	Amount       float64 `json:"amount"`
	BonusPercent float64 `json:"bonus_percent"`
}

// GetRechargeBonusTiers returns the same server-owned offers used to price new
// balance orders. A fresh slice prevents callers from changing pricing rules.
func GetRechargeBonusTiers() []RechargeBonusTier {
	return []RechargeBonusTier{
		{Amount: 20, BonusPercent: 5},
		{Amount: 50, BonusPercent: 7},
		{Amount: 100, BonusPercent: 10},
		{Amount: 200, BonusPercent: 12},
		{Amount: 500, BonusPercent: 15},
	}
}

func normalizeBalanceRechargeMultiplier(multiplier float64) float64 {
	if math.IsNaN(multiplier) || math.IsInf(multiplier, 0) || multiplier <= 0 {
		return defaultBalanceRechargeMultiplier
	}
	return multiplier
}

// normalizeSubscriptionUSDToCNYRate 将非法值归一为 0（换算关闭）。
// 与余额倍率不同，0 是合法状态：表示订阅保持 price 直付的存量行为。
func normalizeSubscriptionUSDToCNYRate(rate float64) float64 {
	if math.IsNaN(rate) || math.IsInf(rate, 0) || rate < 0 {
		return 0
	}
	return rate
}

func calculateCreditedBalance(paymentAmount, multiplier float64) float64 {
	amount := decimal.NewFromFloat(paymentAmount)
	baseCredit := amount.
		Mul(decimal.NewFromFloat(normalizeBalanceRechargeMultiplier(multiplier))).
		Round(2)
	for _, tier := range GetRechargeBonusTiers() {
		// Match before rounding: nearby custom amounts must not inherit a tier.
		if amount.Equal(decimal.NewFromFloat(tier.Amount)) {
			bonus := baseCredit.Mul(decimal.NewFromFloat(tier.BonusPercent)).Div(decimal.NewFromInt(100)).Round(2)
			return baseCredit.Add(bonus).InexactFloat64()
		}
	}
	return baseCredit.InexactFloat64()
}

func calculateGatewayRefundAmount(orderAmount, payAmount, refundAmount float64, currency string) float64 {
	if orderAmount <= 0 || payAmount <= 0 || refundAmount <= 0 {
		return 0
	}
	fractionDigits := int32(payment.CurrencyMaxFractionDigits(currency))
	if math.Abs(refundAmount-orderAmount) <= paymentAmountToleranceForCurrency(currency) {
		return decimal.NewFromFloat(payAmount).Round(fractionDigits).InexactFloat64()
	}
	return decimal.NewFromFloat(payAmount).
		Mul(decimal.NewFromFloat(refundAmount)).
		Div(decimal.NewFromFloat(orderAmount)).
		Round(fractionDigits).
		InexactFloat64()
}

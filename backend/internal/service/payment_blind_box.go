package service

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"time"

	"entgo.io/ent/dialect/sql"
	"entgo.io/ent/dialect/sql/sqljson"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/shopspring/decimal"
)

const (
	RechargeModeBalance    = "balance"
	RechargeModeBlindBox   = "blind_box"
	SettingBlindBoxEnabled = "FANTASY_BLIND_BOX_ENABLED"
	BlindBoxRulesVersion   = "2026-10-20-v1"
	BlindBoxPrice          = 9.9
	blindBoxSnapshotKey    = "fantasy_recharge"
)

var blindBoxLocation = time.FixedZone("Asia/Shanghai", 8*60*60)

type BlindBoxProbability struct {
	Min                   int     `json:"min"`
	Max                   int     `json:"max"`
	Probability           float64 `json:"probability"`
	PerIntegerProbability float64 `json:"per_integer_probability"`
}

type BlindBoxCheckout struct {
	Enabled       bool                  `json:"enabled"`
	Price         float64               `json:"price"`
	Currency      string                `json:"currency"`
	Min           int                   `json:"min"`
	Max           int                   `json:"max"`
	DailyLimit    int                   `json:"daily_limit"`
	RulesVersion  string                `json:"rules_version"`
	Probabilities []BlindBoxProbability `json:"probabilities"`
}

type BlindBoxOrderResult struct {
	RulesVersion string `json:"rules_version"`
	Min          int    `json:"min"`
	Max          int    `json:"max"`
	Drawn        bool   `json:"drawn"`
	Amount       *int   `json:"amount,omitempty"`
}

func GetBlindBoxCheckout(cfg *PaymentConfig) BlindBoxCheckout {
	return BlindBoxCheckout{
		Enabled: cfg != nil && cfg.Enabled && cfg.BlindBoxEnabled && !cfg.BalanceDisabled,
		Price:   BlindBoxPrice, Currency: "CNY", Min: 50, Max: 100, DailyLimit: 1,
		RulesVersion: BlindBoxRulesVersion,
		Probabilities: []BlindBoxProbability{
			{50, 59, .35, .035}, {60, 69, .60, .06}, {70, 79, .04, .004},
			{80, 89, .007, .0007}, {90, 99, .002, .0002}, {100, 100, .001, .001},
		},
	}
}

func blindBoxMetadata(o *dbent.PaymentOrder) map[string]any {
	if o == nil || o.OrderType != payment.OrderTypeBalance {
		return nil
	}
	v, _ := o.ProviderSnapshot[blindBoxSnapshotKey].(map[string]any)
	if v["mode"] != RechargeModeBlindBox {
		return nil
	}
	return v
}

func PaymentOrderRechargeMode(o *dbent.PaymentOrder) string {
	if blindBoxMetadata(o) != nil {
		return RechargeModeBlindBox
	}
	return RechargeModeBalance
}

// BlindBoxResult exposes only the purchased rules and committed result; it never
// exposes payment-provider snapshots or a pre-payment random value.
func BlindBoxResult(o *dbent.PaymentOrder) *BlindBoxOrderResult {
	m := blindBoxMetadata(o)
	if m == nil {
		return nil
	}
	r := &BlindBoxOrderResult{RulesVersion: psSnapshotStringValue(m["rules_version"]), Min: 50, Max: 100}
	if n, ok := validBlindBoxDraw(o); ok {
		r.Drawn = true
		r.Amount = &n
	}
	return r
}

func validBlindBoxDraw(o *dbent.PaymentOrder) (int, bool) {
	m := blindBoxMetadata(o)
	if m == nil || m["rules_version"] != BlindBoxRulesVersion || psSnapshotStringValue(m["drawn_at"]) == "" {
		return 0, false
	}
	n := psSnapshotIntValue(m["draw_amount"])
	return n, n >= 50 && n <= 100 && o.Amount == float64(n) && o.PaidAt != nil
}

func validateBlindBoxRequest(req CreateOrderRequest, cfg *PaymentConfig) error {
	if req.RechargeMode == "" || req.RechargeMode == RechargeModeBalance {
		return nil
	}
	if req.RechargeMode != RechargeModeBlindBox {
		return infraerrors.BadRequest("INVALID_RECHARGE_MODE", "unknown recharge mode")
	}
	if req.OrderType != payment.OrderTypeBalance || req.PlanID != 0 ||
		math.IsNaN(req.Amount) || math.IsInf(req.Amount, 0) || !decimal.NewFromFloat(req.Amount).Equal(decimal.NewFromFloat(BlindBoxPrice)) {
		return infraerrors.BadRequest("INVALID_BLIND_BOX_ORDER", "blind box requires a 9.9 CNY balance order")
	}
	if !cfg.BlindBoxEnabled {
		return infraerrors.Forbidden("BLIND_BOX_DISABLED", "blind box is disabled")
	}
	// OAuth resumes currently represent ordinary recharges. Do not silently lose
	// the explicit purchase mode while redirecting through a WeChat OAuth flow.
	if req.IsWeChatBrowser && req.PaymentType == payment.TypeWxpay && req.OpenID == "" {
		return infraerrors.BadRequest("BLIND_BOX_PAYMENT_CONTEXT_UNSUPPORTED", "choose Alipay for this blind box purchase")
	}
	return nil
}

func blindBoxDayBounds(now time.Time) (time.Time, time.Time) {
	n := now.In(blindBoxLocation)
	start := time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, blindBoxLocation)
	return start, start.AddDate(0, 0, 1)
}

// A harmless primary-key update obtains a per-user database row lock. Unlike
// process mutexes, this serializes purchases across all application instances;
// it deliberately bypasses Ent update hooks and changes no user data.
func lockBlindBoxUser(ctx context.Context, tx *dbent.Tx, userID int64) error {
	r, err := tx.ExecContext(ctx, "UPDATE users SET id = id WHERE id = $1", userID)
	if err != nil {
		return fmt.Errorf("lock blind box quota: %w", err)
	}
	n, err := r.RowsAffected()
	if err != nil || n != 1 {
		return fmt.Errorf("blind box quota user not found")
	}
	return nil
}

func (s *PaymentService) reserveBlindBoxQuota(ctx context.Context, tx *dbent.Tx, userID int64, now time.Time) error {
	if err := lockBlindBoxUser(ctx, tx, userID); err != nil {
		return err
	}
	start, end := blindBoxDayBounds(now)
	orders, err := tx.PaymentOrder.Query().Where(paymentorder.UserIDEQ(userID), paymentorder.CreatedAtGTE(start), paymentorder.CreatedAtLT(end)).All(ctx)
	if err != nil {
		return fmt.Errorf("query blind box quota: %w", err)
	}
	for _, o := range orders {
		// Cancelled/expired QR codes can still receive late gateway payments, so
		// their reservation must survive cancellation. No result exists yet.
		if blindBoxMetadata(o) != nil {
			return infraerrors.TooManyRequests("BLIND_BOX_DAILY_LIMIT", "only one blind box order may be created per day (Asia/Shanghai); cancelled and expired orders still count")
		}
	}
	return nil
}

func addBlindBoxSnapshot(snapshot map[string]any, req CreateOrderRequest, cfg *PaymentConfig, now time.Time) map[string]any {
	if req.RechargeMode != RechargeModeBlindBox {
		return snapshot
	}
	if snapshot == nil {
		snapshot = map[string]any{}
	}
	snapshot[blindBoxSnapshotKey] = map[string]any{
		"mode": RechargeModeBlindBox, "rules_version": BlindBoxRulesVersion,
		"price": BlindBoxPrice, "currency": "CNY", "participation_day": now.In(blindBoxLocation).Format("2006-01-02"),
		"standard_amount": calculateCreditedBalance(BlindBoxPrice, cfg.BalanceRechargeMultiplier),
	}
	return snapshot
}

func blindBoxWeight(amount int) int {
	switch {
	case amount >= 50 && amount <= 59:
		return 350
	case amount >= 60 && amount <= 69:
		return 600
	case amount >= 70 && amount <= 79:
		return 40
	case amount >= 80 && amount <= 89:
		return 7
	case amount >= 90 && amount <= 99:
		return 2
	case amount == 100:
		return 10
	default:
		return 0
	}
}

func blindBoxAmountForTicket(ticket int) (int, error) {
	if ticket < 0 || ticket >= 10000 {
		return 0, fmt.Errorf("invalid blind box ticket")
	}
	for n := 50; n <= 100; n++ {
		if ticket < blindBoxWeight(n) {
			return n, nil
		}
		ticket -= blindBoxWeight(n)
	}
	return 0, fmt.Errorf("invalid blind box probability total")
}

func drawBlindBoxAmount() (int, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(10000))
	if err != nil {
		return 0, fmt.Errorf("draw blind box: %w", err)
	}
	return blindBoxAmountForTicket(int(n.Int64()))
}

// markBlindBoxPaid commits the draw together with the paid transition, before
// fulfillment can create a redeem code. A crash or duplicate callback therefore
// reuses exactly the persisted amount rather than drawing again.
func (s *PaymentService) markBlindBoxPaid(ctx context.Context, previous *dbent.PaymentOrder, tradeNo string, paid float64, now time.Time) (bool, error) {
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return false, fmt.Errorf("begin blind box transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := lockBlindBoxUser(ctx, tx, previous.UserID); err != nil {
		return false, err
	}
	o, err := tx.PaymentOrder.Get(ctx, previous.ID)
	if err != nil {
		return false, err
	}
	eligible := o.Status == OrderStatusPending || o.Status == OrderStatusCancelled || o.Status == OrderStatusExpired || (o.Status == OrderStatusFailed && o.PaidAt == nil)
	if !eligible {
		return false, nil
	}
	m := blindBoxMetadata(o)
	if m == nil || m["rules_version"] != BlindBoxRulesVersion || PaymentOrderCurrency(o) != "CNY" || o.PayAmount != BlindBoxPrice || paid != BlindBoxPrice || o.Amount != 0 || m["currency"] != "CNY" || m["price"] != BlindBoxPrice {
		return false, fmt.Errorf("invalid persisted blind box purchase")
	}
	if _, ok := m["draw_amount"]; ok {
		return false, fmt.Errorf("unpaid blind box unexpectedly contains a result")
	}
	amount, err := drawBlindBoxAmount()
	if err != nil {
		return false, err
	}
	snapshot := make(map[string]any, len(o.ProviderSnapshot))
	for k, v := range o.ProviderSnapshot {
		snapshot[k] = v
	}
	meta := make(map[string]any, len(m)+2)
	for k, v := range m {
		meta[k] = v
	}
	meta["draw_amount"], meta["drawn_at"] = amount, now.UTC().Format(time.RFC3339Nano)
	snapshot[blindBoxSnapshotKey] = meta
	updated, err := tx.PaymentOrder.Update().Where(
		paymentorder.IDEQ(o.ID), paymentorder.StatusEQ(o.Status), paymentorder.PaidAtIsNil(), paymentorder.AmountEQ(0),
	).SetAmount(float64(amount)).SetProviderSnapshot(snapshot).
		SetStatus(OrderStatusPaid).SetPayAmount(paid).SetPaymentTradeNo(tradeNo).SetPaidAt(now).
		ClearFailedAt().ClearFailedReason().Save(ctx)
	if err != nil {
		return false, fmt.Errorf("save blind box draw: %w", err)
	}
	if updated != 1 {
		// Cancellation and provider-error writers do not take the quota lock.
		// A lost CAS must make the gateway retry; acknowledging success here
		// could otherwise strand a paid CANCELLED order without a draw.
		return false, infraerrors.Conflict("BLIND_BOX_ORDER_CHANGED", "blind box order changed; retry payment confirmation")
	}
	detail, _ := json.Marshal(map[string]any{"rules_version": BlindBoxRulesVersion, "amount": amount, "pay_amount": paid})
	_, err = tx.PaymentAuditLog.Create().SetOrderID(strconv.FormatInt(o.ID, 10)).SetAction("BLIND_BOX_DRAWN").SetOperator("system").SetDetail(string(detail)).Save(ctx)
	if err != nil {
		return false, fmt.Errorf("audit blind box draw: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit blind box draw: %w", err)
	}
	return true, nil
}

// RecoverPaidBlindBoxOrders repairs the narrow crash window between the atomic
// paid/draw commit and fulfillment. Ordinary orders and undrawn orders are never
// selected. Existing leases and redeem-code idempotency remain authoritative.
func (s *PaymentService) RecoverPaidBlindBoxOrders(ctx context.Context) (int, error) {
	orders, err := s.entClient.PaymentOrder.Query().Where(
		paymentorder.Or(paymentorder.StatusIn(OrderStatusPaid, OrderStatusFailed), paymentorder.And(paymentorder.StatusEQ(OrderStatusRecharging), paymentorder.UpdatedAtLTE(time.Now().Add(-paymentFulfillmentLeaseDuration)))),
		paymentorder.PaidAtNotNil(),
		func(sel *sql.Selector) {
			sel.Where(sqljson.ValueEQ(paymentorder.FieldProviderSnapshot, RechargeModeBlindBox, sqljson.Path(blindBoxSnapshotKey, "mode")))
			sel.Where(sql.P(func(b *sql.Builder) {
				b.WriteString("NOT EXISTS (SELECT 1 FROM payment_audit_logs AS recovery_audit WHERE recovery_audit.order_id = CAST(").Ident(sel.C(paymentorder.FieldID)).WriteString(" AS TEXT) AND recovery_audit.action = ").Arg("BLIND_BOX_RECOVERY_INVALID").WriteByte(')')
			}))
		},
	).Order(dbent.Asc(paymentorder.FieldUpdatedAt)).Limit(20).All(ctx)
	if err != nil {
		return 0, err
	}
	recovered := 0
	for _, o := range orders {
		if _, ok := validBlindBoxDraw(o); !ok {
			s.writeAuditLog(ctx, o.ID, "BLIND_BOX_RECOVERY_INVALID", "system", map[string]any{"reason": "committed blind box draw failed validation; manual review required"})
			continue
		}
		if err := s.executeFulfillment(ctx, o.ID); err != nil {
			continue
		}
		current, err := s.entClient.PaymentOrder.Get(ctx, o.ID)
		if err == nil && current.Status == OrderStatusCompleted {
			recovered++
		}
	}
	return recovered, nil
}

// A returning buyer continues the same pending QR code without consuming a
// second quota slot or issuing another upstream order.
func (s *PaymentService) pendingBlindBoxResponse(ctx context.Context, req CreateOrderRequest) (*CreateOrderResponse, error) {
	start, end := blindBoxDayBounds(time.Now())
	orders, err := s.entClient.PaymentOrder.Query().Where(paymentorder.UserIDEQ(req.UserID), paymentorder.StatusEQ(OrderStatusPending), paymentorder.CreatedAtGTE(start), paymentorder.CreatedAtLT(end), paymentorder.ExpiresAtGT(time.Now())).All(ctx)
	if err != nil {
		return nil, err
	}
	for _, o := range orders {
		if blindBoxMetadata(o) == nil || (psStringValue(o.PayURL) == "" && psStringValue(o.QrCode) == "") {
			continue
		}
		resp := &CreateOrderResponse{OrderID: o.ID, Amount: o.Amount, PayAmount: o.PayAmount, FeeRate: o.FeeRate, Status: o.Status,
			ResultType: payment.CreatePaymentResultOrderCreated, PaymentType: o.PaymentType, OutTradeNo: o.OutTradeNo,
			PayURL: psStringValue(o.PayURL), QRCode: psStringValue(o.QrCode), Currency: PaymentOrderCurrency(o), ExpiresAt: o.ExpiresAt,
			RechargeMode: RechargeModeBlindBox, BlindBox: BlindBoxResult(o)}
		if snapshot := psOrderProviderSnapshot(o); snapshot != nil {
			resp.PaymentMode = snapshot.PaymentMode
		}
		canonical, err := CanonicalizeReturnURL(req.ReturnURL, req.SrcHost, req.SrcURL)
		if err != nil {
			return nil, err
		}
		if resume := s.paymentResume(); resume != nil && canonical != "" && resume.isSigningConfigured() {
			resp.ResumeToken, err = resume.CreateToken(ResumeTokenClaims{OrderID: o.ID, UserID: o.UserID, ProviderInstanceID: psStringValue(o.ProviderInstanceID), ProviderKey: psStringValue(o.ProviderKey), PaymentType: o.PaymentType, CanonicalReturnURL: canonical})
			if err != nil {
				return nil, err
			}
		}
		return resp, nil
	}
	return nil, nil
}

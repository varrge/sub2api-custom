package integration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentauditlog"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// Every case owns its wallet and gateway; the shared database is disposable and
// created only by the real-migration helper, never by reusing the configured DB.
func TestBalanceRefundRecoveryPostgres(t *testing.T) {
	db, client := paymentWebhookSecurityDB(t)
	ctx := context.Background()
	users := repository.NewUserRepository(client, db)
	svc := service.NewPaymentService(client, payment.NewRegistry(), payment.NewDefaultLoadBalancer(client, nil), nil, nil,
		service.NewPaymentConfigService(client, repository.NewSettingRepository(client), nil), users, nil, nil)
	const initialBalance = 31.25
	const initialTotalRecharged = 70.75
	const refundAmount = 8.0

	type auditFault struct {
		orderID int64
		action  string
	}
	var fault atomic.Pointer[auditFault]
	client.PaymentAuditLog.Use(func(next dbent.Mutator) dbent.Mutator {
		return dbent.MutateFunc(func(ctx context.Context, mutation dbent.Mutation) (dbent.Value, error) {
			if m, ok := mutation.(*dbent.PaymentAuditLogMutation); ok {
				orderID, _ := m.OrderID()
				action, _ := m.Action()
				if f := fault.Load(); f != nil && orderID == strconv.FormatInt(f.orderID, 10) && action == f.action {
					return nil, errors.New("injected payment audit failure")
				}
			}
			return next.Mutate(ctx, mutation)
		})
	})
	type gatewayObservation struct {
		form    url.Values
		status  string
		balance float64
		started int
		err     error
	}
	type fixture struct {
		user         *dbent.User
		order        *dbent.PaymentOrder
		calls        atomic.Int32
		observations chan gatewayObservation
	}
	makeFixture := func(t *testing.T, mode string) *fixture {
		t.Helper()
		f := &fixture{observations: make(chan gatewayObservation, 10)}
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			call := f.calls.Add(1)
			err := r.ParseForm()
			observation := gatewayObservation{form: r.PostForm, err: err}
			if err == nil {
				observation.err = db.QueryRowContext(r.Context(), `SELECT o.status, u.balance,
					(SELECT COUNT(*) FROM payment_audit_logs WHERE order_id=o.id::text AND action='REFUND_STARTED')
					FROM payment_orders o JOIN users u ON u.id=o.user_id WHERE o.id=$1`, f.order.ID).
					Scan(&observation.status, &observation.balance, &observation.started)
			}
			f.observations <- observation
			if mode == "lost_response" || mode == "two_lost_then_success" && call <= 2 {
				hijacker, ok := w.(http.Hijacker)
				if !ok {
					t.Error("gateway connection does not support hijacking")
					return
				}
				conn, _, err := hijacker.Hijack()
				if err != nil {
					t.Errorf("hijack gateway connection: %v", err)
					return
				}
				_ = conn.Close()
				return
			}
			w.Header().Set("Content-Type", "application/json")
			if mode == "rejected" {
				_, _ = w.Write([]byte(`{"code":-1,"msg":"refund rejected"}`))
				return
			}
			_, _ = w.Write([]byte(`{"code":1,"msg":"success"}`))
		}))
		t.Cleanup(server.Close)
		cfg, err := json.Marshal(map[string]string{"pid": "1001", "pkey": "refund-test-secret", "apiBase": server.URL,
			"notifyUrl": "https://merchant.test/notify", "returnUrl": "https://merchant.test/return"})
		require.NoError(t, err)
		instance, err := client.PaymentProviderInstance.Create().SetName("refund-fixture").SetProviderKey(payment.TypeEasyPay).
			SetConfig(string(cfg)).SetSupportedTypes(payment.TypeWxpay).SetRefundEnabled(true).Save(ctx)
		require.NoError(t, err)
		f.user, err = client.User.Create().SetEmail(fmt.Sprintf("refund-%d@example.test", instance.ID)).SetPasswordHash("test-only").
			SetBalance(initialBalance).SetTotalRecharged(initialTotalRecharged).Save(ctx)
		require.NoError(t, err)
		instanceID := strconv.FormatInt(instance.ID, 10)
		f.order, err = client.PaymentOrder.Create().SetUserID(f.user.ID).SetUserEmail(f.user.Email).SetUserName("refund-fixture").
			SetAmount(20).SetPayAmount(10).SetFeeRate(0).SetRechargeCode("PAY-refund-" + instanceID).
			SetOutTradeNo("sub2_refund_" + instanceID).SetPaymentTradeNo("gateway_" + instanceID).SetPaymentType(payment.TypeWxpay).
			SetProviderInstanceID(instanceID).SetProviderKey(payment.TypeEasyPay).
			SetProviderSnapshot(map[string]any{"schema_version": 1, "provider_instance_id": instanceID, "provider_key": payment.TypeEasyPay, "merchant_id": "1001"}).
			SetOrderType(payment.OrderTypeBalance).SetStatus(payment.OrderStatusCompleted).SetPaidAt(time.Now().Add(-time.Hour)).
			SetCompletedAt(time.Now().Add(-time.Hour)).SetExpiresAt(time.Now().Add(time.Hour)).SetClientIP("127.0.0.1").SetSrcHost("merchant.test").Save(ctx)
		require.NoError(t, err)
		return f
	}
	assertState := func(t *testing.T, f *fixture, status string, balance float64) *dbent.PaymentOrder {
		t.Helper()
		order, err := client.PaymentOrder.Get(ctx, f.order.ID)
		require.NoError(t, err)
		require.Equal(t, status, order.Status)
		user, err := client.User.Get(ctx, f.user.ID)
		require.NoError(t, err)
		require.Equal(t, balance, user.Balance)
		require.Equal(t, initialTotalRecharged, user.TotalRecharged, "refund restoration must not count as new recharge")
		return order
	}
	auditCount := func(t *testing.T, f *fixture, action string) int {
		t.Helper()
		count, err := client.PaymentAuditLog.Query().Where(paymentauditlog.OrderIDEQ(strconv.FormatInt(f.order.ID, 10)), paymentauditlog.ActionEQ(action)).Count(ctx)
		require.NoError(t, err)
		return count
	}
	prepare := func(t *testing.T, f *fixture) *service.RefundPlan {
		t.Helper()
		plan, early, err := svc.PrepareRefund(ctx, f.order.ID, refundAmount, "refund integration regression", false, true)
		require.NoError(t, err)
		require.Nil(t, early)
		require.NotNil(t, plan)
		return plan
	}
	assertGatewaySawCommittedHold := func(t *testing.T, f *fixture, balance float64) {
		t.Helper()
		observation := <-f.observations
		require.NoError(t, observation.err)
		require.Equal(t, payment.OrderStatusRefunding, observation.status)
		require.Equal(t, balance, observation.balance)
		require.Equal(t, 1, observation.started)
		require.Equal(t, "4.00", observation.form.Get("money"), "refund channel amount follows the original payment ratio")
		require.Equal(t, f.order.OutTradeNo, observation.form.Get("out_trade_no"))
	}
	makePending := func(t *testing.T) (*fixture, *dbent.PaymentOrder) {
		t.Helper()
		f := makeFixture(t, "lost_response")
		result, err := svc.ExecuteRefund(ctx, prepare(t, f))
		require.NoError(t, err)
		require.False(t, result.Success)
		order := assertState(t, f, payment.OrderStatusRefundPending, initialBalance-refundAmount)
		require.Equal(t, refundAmount, order.RefundAmount)
		require.Equal(t, int32(1), f.calls.Load())
		assertGatewaySawCommittedHold(t, f, initialBalance-refundAmount)
		require.Equal(t, 1, auditCount(t, f, "REFUND_PENDING"))
		return f, order
	}
	confirmInput := func(order *dbent.PaymentOrder, outcome string) service.ConfirmBalanceRefundInput {
		return service.ConfirmBalanceRefundInput{Outcome: outcome, Reference: "verified gateway refund ledger", UpdatedAt: order.UpdatedAt}
	}

	t.Run("success commits the exact balance deduction before calling gateway", func(t *testing.T) {
		f := makeFixture(t, "success")
		plan := prepare(t, f)
		assertState(t, f, payment.OrderStatusCompleted, initialBalance)
		require.Zero(t, f.calls.Load())
		result, err := svc.ExecuteRefund(ctx, plan)
		require.NoError(t, err)
		require.True(t, result.Success)
		require.Equal(t, refundAmount, result.BalanceDeducted)
		assertState(t, f, payment.OrderStatusPartiallyRefunded, initialBalance-refundAmount)
		assertGatewaySawCommittedHold(t, f, initialBalance-refundAmount)
		require.Equal(t, int32(1), f.calls.Load())
	})
	t.Run("lost response holds balance and rejects resending", func(t *testing.T) {
		f, _ := makePending(t)
		_, _, err := svc.PrepareRefund(ctx, f.order.ID, refundAmount, "retry", true, true)
		require.Error(t, err)
		_, err = svc.QueryAndFinalizeRefund(ctx, f.order.ID)
		require.Error(t, err, "EasyPay has no refund query; reconciliation must remain manual")
		assertState(t, f, payment.OrderStatusRefundPending, initialBalance-refundAmount)
		require.Equal(t, int32(1), f.calls.Load())
	})
	t.Run("explicit gateway rejection restores the committed deduction", func(t *testing.T) {
		f := makeFixture(t, "rejected")
		result, err := svc.ExecuteRefund(ctx, prepare(t, f))
		require.NoError(t, err)
		require.False(t, result.Success)
		assertState(t, f, payment.OrderStatusRefundFailed, initialBalance)
		assertGatewaySawCommittedHold(t, f, initialBalance-refundAmount)
		require.Equal(t, int32(1), f.calls.Load())
		require.Equal(t, 1, auditCount(t, f, "REFUND_FAILED"))
	})
	for _, tc := range []struct {
		name    string
		balance float64
		deduct  bool
		held    float64
	}{
		{"force partial available balance", 3.25, true, 3.25},
		{"force zero available balance", 0, true, 0},
		{"deduction disabled", initialBalance, false, 0},
	} {
		t.Run("failure restores only actual held amount for "+tc.name, func(t *testing.T) {
			f := makeFixture(t, "lost_response")
			_, err := client.User.UpdateOneID(f.user.ID).SetBalance(tc.balance).Save(ctx)
			require.NoError(t, err)
			plan, early, err := svc.PrepareRefund(ctx, f.order.ID, refundAmount, "verify actual hold", true, tc.deduct)
			require.NoError(t, err)
			require.Nil(t, early)
			_, err = svc.ExecuteRefund(ctx, plan)
			require.NoError(t, err)
			order := assertState(t, f, payment.OrderStatusRefundPending, tc.balance-tc.held)
			assertGatewaySawCommittedHold(t, f, tc.balance-tc.held)
			input := confirmInput(order, payment.ProviderStatusFailed)
			input.BalanceDeducted = &tc.held
			_, err = svc.ConfirmBalanceRefund(ctx, order.ID, input)
			require.NoError(t, err)
			assertState(t, f, payment.OrderStatusRefundFailed, tc.balance)
			require.Equal(t, int32(1), f.calls.Load())
		})
	}
	t.Run("manual failure restores once without increasing recharge totals", func(t *testing.T) {
		f, order := makePending(t)
		input := confirmInput(order, payment.ProviderStatusFailed)
		result, err := svc.ConfirmBalanceRefund(ctx, order.ID, input)
		require.NoError(t, err)
		require.True(t, result.Resolved)
		require.False(t, result.Success)
		assertState(t, f, payment.OrderStatusRefundFailed, initialBalance)
		_, _ = svc.ConfirmBalanceRefund(ctx, order.ID, input)
		assertState(t, f, payment.OrderStatusRefundFailed, initialBalance)
		require.Equal(t, 1, auditCount(t, f, "REFUND_FAILED"))
		require.Equal(t, 1, auditCount(t, f, "REFUND_MANUALLY_VERIFIED"))
		require.Equal(t, int32(1), f.calls.Load())
	})
	t.Run("manual pending success never deducts a second time", func(t *testing.T) {
		f, order := makePending(t)
		input := confirmInput(order, payment.ProviderStatusSuccess)
		result, err := svc.ConfirmBalanceRefund(ctx, order.ID, input)
		require.NoError(t, err)
		require.True(t, result.Success)
		require.True(t, result.Resolved)
		assertState(t, f, payment.OrderStatusPartiallyRefunded, initialBalance-refundAmount)
		_, _ = svc.ConfirmBalanceRefund(ctx, order.ID, input)
		assertState(t, f, payment.OrderStatusPartiallyRefunded, initialBalance-refundAmount)
		require.Equal(t, 1, auditCount(t, f, "REFUND_MANUALLY_VERIFIED"))
		require.Equal(t, int32(1), f.calls.Load())
	})
	t.Run("same order retries preserve separate refund attempts and exact balance", func(t *testing.T) {
		f := makeFixture(t, "two_lost_then_success")
		amounts := []float64{8, 6, 10}
		var oldConfirmation service.ConfirmBalanceRefundInput
		for attempt, amount := range amounts {
			plan, early, err := svc.PrepareRefund(ctx, f.order.ID, amount, fmt.Sprintf("refund attempt %d", attempt+1), false, true)
			require.NoError(t, err)
			require.Nil(t, early)
			result, err := svc.ExecuteRefund(ctx, plan)
			require.NoError(t, err, "same order must be able to append another REFUND_STARTED audit")
			observation := <-f.observations
			require.NoError(t, observation.err)
			require.Equal(t, payment.OrderStatusRefunding, observation.status)
			require.Equal(t, initialBalance-amount, observation.balance)
			require.Equal(t, attempt+1, observation.started)
			require.Equal(t, fmt.Sprintf("%.2f", amount/2), observation.form.Get("money"))
			require.Equal(t, int32(attempt+1), f.calls.Load())
			if attempt < 2 {
				require.False(t, result.Success)
				order := assertState(t, f, payment.OrderStatusRefundPending, initialBalance-amount)
				require.Equal(t, amount, order.RefundAmount)
				if attempt > 0 {
					_, err = svc.ConfirmBalanceRefund(ctx, order.ID, oldConfirmation)
					require.Error(t, err, "the previous attempt's confirmation cannot restore this attempt's hold")
					assertState(t, f, payment.OrderStatusRefundPending, initialBalance-amount)
				}
				input := confirmInput(order, payment.ProviderStatusFailed)
				input.Reference = fmt.Sprintf("gateway ledger verified failed attempt %d", attempt+1)
				result, err = svc.ConfirmBalanceRefund(ctx, order.ID, input)
				require.NoError(t, err)
				require.True(t, result.Resolved)
				require.False(t, result.Success)
				assertState(t, f, payment.OrderStatusRefundFailed, initialBalance)
				_, _ = svc.ConfirmBalanceRefund(ctx, order.ID, input)
				assertState(t, f, payment.OrderStatusRefundFailed, initialBalance)
				oldConfirmation = input
			} else {
				require.True(t, result.Success)
				require.Equal(t, amount, result.BalanceDeducted)
				assertState(t, f, payment.OrderStatusPartiallyRefunded, initialBalance-amount)
			}
		}
		for action, expected := range map[string]int{"REFUND_STARTED": 3, "REFUND_PENDING": 2, "REFUND_FAILED": 2, "REFUND_MANUALLY_VERIFIED": 2, "REFUND_SUCCESS": 1} {
			require.Equal(t, expected, auditCount(t, f, action), action)
		}
		var claims []time.Time
		for _, action := range []string{"REFUND_STARTED", "REFUND_PENDING", "REFUND_MANUALLY_VERIFIED"} {
			logs, err := client.PaymentAuditLog.Query().Where(paymentauditlog.OrderIDEQ(strconv.FormatInt(f.order.ID, 10)), paymentauditlog.ActionEQ(action)).Order(paymentauditlog.ByID()).All(ctx)
			require.NoError(t, err)
			for i, log := range logs {
				var detail struct {
					RefundAmount    float64   `json:"refundAmount"`
					BalanceDeducted float64   `json:"balanceDeducted"`
					GatewayAmount   float64   `json:"gatewayAmount"`
					ClaimedAt       time.Time `json:"claimedAt"`
				}
				require.NoError(t, json.Unmarshal([]byte(log.Detail), &detail))
				require.Equal(t, amounts[i], detail.RefundAmount, action)
				require.Equal(t, amounts[i], detail.BalanceDeducted, action)
				switch action {
				case "REFUND_STARTED":
					require.Equal(t, amounts[i]/2, detail.GatewayAmount)
					require.False(t, detail.ClaimedAt.IsZero())
					if i > 0 {
						require.True(t, detail.ClaimedAt.After(claims[i-1]))
					}
					claims = append(claims, detail.ClaimedAt)
				case "REFUND_PENDING":
					require.Equal(t, amounts[i]/2, detail.GatewayAmount)
					require.True(t, detail.ClaimedAt.Equal(claims[i]), "pending audit must identify its own attempt")
				}
			}
		}
	})
	for _, action := range []string{"AFFILIATE_REBATE_APPLIED", "RECHARGE_SUCCESS"} {
		t.Run("refund migration preserves audit deduplication for "+action, func(t *testing.T) {
			f := makeFixture(t, "success")
			orderID := strconv.FormatInt(f.order.ID, 10)
			_, err := client.PaymentAuditLog.Create().SetOrderID(orderID).SetAction(action).SetDetail(`{"first":true}`).Save(ctx)
			require.NoError(t, err)
			_, err = client.PaymentAuditLog.Create().SetOrderID(orderID).SetAction(action).SetDetail(`{"duplicate":true}`).Save(ctx)
			require.True(t, dbent.IsConstraintError(err), "duplicate non-refund audit must still violate the database constraint: %v", err)
			result, err := db.ExecContext(ctx, `INSERT INTO payment_audit_logs (order_id, action, detail, operator, created_at)
				VALUES ($1, $2, '{}', 'test', NOW()) ON CONFLICT DO NOTHING`, orderID, action)
			require.NoError(t, err, "targetless ON CONFLICT must work with the partial unique index")
			affected, err := result.RowsAffected()
			require.NoError(t, err)
			require.Zero(t, affected)
			require.Equal(t, 1, auditCount(t, f, action))
			assertState(t, f, payment.OrderStatusCompleted, initialBalance)
			require.Zero(t, f.calls.Load())
		})
	}
	t.Run("missing payment trade number cannot refund", func(t *testing.T) {
		f := makeFixture(t, "success")
		_, err := client.PaymentOrder.UpdateOneID(f.order.ID).SetPaymentTradeNo("").Save(ctx)
		require.NoError(t, err)
		_, _, err = svc.PrepareRefund(ctx, f.order.ID, refundAmount, "missing trade", false, true)
		require.Error(t, err)
		assertState(t, f, payment.OrderStatusCompleted, initialBalance)
		require.Zero(t, f.calls.Load())
		require.Zero(t, auditCount(t, f, "REFUND_STARTED"))
	})
	t.Run("audit failure rolls back claim deduction and prevents gateway call", func(t *testing.T) {
		f := makeFixture(t, "success")
		plan := prepare(t, f)
		fault.Store(&auditFault{f.order.ID, "REFUND_STARTED"})
		t.Cleanup(func() { fault.Store(nil) })
		_, err := svc.ExecuteRefund(ctx, plan)
		require.ErrorContains(t, err, "injected payment audit failure")
		order := assertState(t, f, payment.OrderStatusCompleted, initialBalance)
		require.Zero(t, order.RefundAmount)
		require.True(t, order.UpdatedAt.Equal(f.order.UpdatedAt))
		require.Zero(t, f.calls.Load())
		require.Zero(t, auditCount(t, f, "REFUND_STARTED"))
	})
	t.Run("manual failure audit error atomically rolls back restoration", func(t *testing.T) {
		f, order := makePending(t)
		fault.Store(&auditFault{f.order.ID, "REFUND_MANUALLY_VERIFIED"})
		t.Cleanup(func() { fault.Store(nil) })
		_, err := svc.ConfirmBalanceRefund(ctx, order.ID, confirmInput(order, payment.ProviderStatusFailed))
		require.ErrorContains(t, err, "injected payment audit failure")
		fresh := assertState(t, f, payment.OrderStatusRefundPending, initialBalance-refundAmount)
		require.True(t, fresh.UpdatedAt.Equal(order.UpdatedAt))
		require.Zero(t, auditCount(t, f, "REFUND_FAILED"))
		fault.Store(nil)
		_, err = svc.ConfirmBalanceRefund(ctx, order.ID, confirmInput(order, payment.ProviderStatusFailed))
		require.NoError(t, err)
		assertState(t, f, payment.OrderStatusRefundFailed, initialBalance)
	})
	t.Run("stale durable claim recovers without resending or deducting again", func(t *testing.T) {
		f := makeFixture(t, "lost_response")
		fault.Store(&auditFault{f.order.ID, "REFUND_PENDING"})
		t.Cleanup(func() { fault.Store(nil) })
		_, err := svc.ExecuteRefund(ctx, prepare(t, f))
		require.ErrorContains(t, err, "injected payment audit failure")
		assertState(t, f, payment.OrderStatusRefunding, initialBalance-refundAmount)
		assertGatewaySawCommittedHold(t, f, initialBalance-refundAmount)
		fault.Store(nil)
		_, err = svc.QueryAndFinalizeRefund(ctx, f.order.ID)
		require.Error(t, err, "an active refund cannot yet be recovered")
		stale := time.Now().Add(-10 * time.Minute).UTC().Truncate(time.Microsecond)
		_, err = client.PaymentOrder.UpdateOneID(f.order.ID).SetUpdatedAt(stale).Save(ctx)
		require.NoError(t, err)
		started, err := client.PaymentAuditLog.Query().Where(paymentauditlog.OrderIDEQ(strconv.FormatInt(f.order.ID, 10)), paymentauditlog.ActionEQ("REFUND_STARTED")).Only(ctx)
		require.NoError(t, err)
		var detail map[string]any
		require.NoError(t, json.Unmarshal([]byte(started.Detail), &detail))
		detail["claimedAt"], detail["requestedAt"] = stale, stale
		raw, err := json.Marshal(detail)
		require.NoError(t, err)
		_, err = client.PaymentAuditLog.UpdateOneID(started.ID).SetDetail(string(raw)).Save(ctx)
		require.NoError(t, err)
		_, err = svc.QueryAndFinalizeRefund(ctx, f.order.ID)
		require.Error(t, err, "EasyPay cannot query refund status")
		order := assertState(t, f, payment.OrderStatusRefundPending, initialBalance-refundAmount)
		require.Equal(t, int32(1), f.calls.Load())
		_, err = svc.ConfirmBalanceRefund(ctx, order.ID, confirmInput(order, payment.ProviderStatusSuccess))
		require.NoError(t, err)
		assertState(t, f, payment.OrderStatusPartiallyRefunded, initialBalance-refundAmount)
		require.Equal(t, int32(1), f.calls.Load())
	})
	t.Run("saved refund amounts cannot be overridden by manual input", func(t *testing.T) {
		f, order := makePending(t)
		for _, field := range []string{"refund", "held"} {
			input := confirmInput(order, payment.ProviderStatusFailed)
			forged := 12.0
			if field == "refund" {
				input.RefundAmount = &forged
			} else {
				input.BalanceDeducted = &forged
			}
			_, err := svc.ConfirmBalanceRefund(ctx, order.ID, input)
			require.Error(t, err)
			assertState(t, f, payment.OrderStatusRefundPending, initialBalance-refundAmount)
		}
		require.Zero(t, auditCount(t, f, "REFUND_MANUALLY_VERIFIED"))
	})
	t.Run("stale manual version cannot finalize", func(t *testing.T) {
		f, order := makePending(t)
		input := confirmInput(order, payment.ProviderStatusFailed)
		input.UpdatedAt = input.UpdatedAt.Add(-time.Second)
		_, err := svc.ConfirmBalanceRefund(ctx, order.ID, input)
		require.Error(t, err)
		assertState(t, f, payment.OrderStatusRefundPending, initialBalance-refundAmount)
	})
	for _, outcome := range []string{payment.ProviderStatusSuccess, payment.ProviderStatusFailed} {
		t.Run("legacy stale refund requires explicit amounts for "+outcome, func(t *testing.T) {
			f := makeFixture(t, "success")
			order, err := client.PaymentOrder.UpdateOneID(f.order.ID).SetStatus(payment.OrderStatusRefunding).
				SetRefundAmount(refundAmount).SetUpdatedAt(time.Now().Add(-10 * time.Minute).UTC().Truncate(time.Microsecond)).Save(ctx)
			require.NoError(t, err)
			input := confirmInput(order, outcome)
			_, err = svc.ConfirmBalanceRefund(ctx, order.ID, input)
			require.Error(t, err)
			amount := refundAmount
			input.RefundAmount = &amount
			_, err = svc.ConfirmBalanceRefund(ctx, order.ID, input)
			require.Error(t, err, "omitted balance deduction is not an explicit zero")
			assertState(t, f, payment.OrderStatusRefunding, initialBalance)
			zero := 0.0
			input.BalanceDeducted = &zero
			result, err := svc.ConfirmBalanceRefund(ctx, order.ID, input)
			require.NoError(t, err)
			require.True(t, result.Resolved)
			status := payment.OrderStatusRefundFailed
			if outcome == payment.ProviderStatusSuccess {
				status = payment.OrderStatusPartiallyRefunded
			}
			assertState(t, f, status, initialBalance)
			require.Zero(t, f.calls.Load())
		})
	}
	t.Run("concurrent confirmations restore the held balance exactly once", func(t *testing.T) {
		f, order := makePending(t)
		input := confirmInput(order, payment.ProviderStatusFailed)
		var wg sync.WaitGroup
		var completed atomic.Int32
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				result, err := svc.ConfirmBalanceRefund(ctx, order.ID, input)
				if err == nil && result != nil && result.Resolved {
					completed.Add(1)
				}
			}()
		}
		wg.Wait()
		require.Equal(t, int32(1), completed.Load())
		assertState(t, f, payment.OrderStatusRefundFailed, initialBalance)
		require.Equal(t, 1, auditCount(t, f, "REFUND_FAILED"))
		require.Equal(t, 1, auditCount(t, f, "REFUND_MANUALLY_VERIFIED"))
		require.Equal(t, int32(1), f.calls.Load())
	})
}

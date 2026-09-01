//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

func TestGetUserFundingSummarySeparatesOrdersAndStandaloneRedeems(t *testing.T) {
	client := newPaymentOrderLifecycleTestClient(t)
	ctx := context.Background()
	now := time.Now().UTC()

	user, err := client.User.Create().
		SetEmail("funding-summary@example.com").
		SetPasswordHash("test").
		Save(ctx)
	require.NoError(t, err)

	createUsedRedeem := func(code, codeType string, value float64) {
		t.Helper()
		_, createErr := client.RedeemCode.Create().
			SetCode(code).
			SetType(codeType).
			SetValue(value).
			SetStatus(StatusUsed).
			SetUsedBy(user.ID).
			SetUsedAt(now).
			Save(ctx)
		require.NoError(t, createErr)
	}

	createUsedRedeem("ORDER-CREDIT", RedeemTypeBalance, 800)
	createUsedRedeem("STANDALONE-CREDIT", RedeemTypeBalance, 70)
	createUsedRedeem("ADMIN-CREDIT", AdjustmentTypeAdminBalance, 5)
	createUsedRedeem("NEGATIVE-ADJUSTMENT", AdjustmentTypeAdminBalance, -10)
	createUsedRedeem("CONCURRENCY-CODE", RedeemTypeConcurrency, 3)

	_, err = client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(800).
		SetPayAmount(800).
		SetRechargeCode("ORDER-CREDIT").
		SetOutTradeNo("funding-summary-order").
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("funding-summary-trade").
		SetOrderType(payment.OrderTypeBalance).
		SetStatus(OrderStatusCompleted).
		SetExpiresAt(now.Add(time.Hour)).
		SetClientIP("127.0.0.1").
		SetSrcHost("example.com").
		Save(ctx)
	require.NoError(t, err)

	summary, err := (&PaymentService{entClient: client}).GetUserFundingSummary(ctx, user.ID)
	require.NoError(t, err)
	require.InDelta(t, 800, summary.OrderRecharged, 1e-9)
	require.InDelta(t, 75, summary.RedeemRecharged, 1e-9)
	require.InDelta(t, 875, summary.TotalRecharged, 1e-9)
}

func TestGetUserFundingSummarySupportsRedeemOnlyAccount(t *testing.T) {
	client := newPaymentOrderLifecycleTestClient(t)
	ctx := context.Background()
	now := time.Now().UTC()

	user, err := client.User.Create().
		SetEmail("redeem-only@example.com").
		SetPasswordHash("test").
		Save(ctx)
	require.NoError(t, err)
	_, err = client.RedeemCode.Create().
		SetCode("REDEEM-ONLY-CREDIT").
		SetType(RedeemTypeBalance).
		SetValue(125).
		SetStatus(StatusUsed).
		SetUsedBy(user.ID).
		SetUsedAt(now).
		Save(ctx)
	require.NoError(t, err)

	summary, err := (&PaymentService{entClient: client}).GetUserFundingSummary(ctx, user.ID)
	require.NoError(t, err)
	require.Zero(t, summary.OrderRecharged)
	require.InDelta(t, 125, summary.RedeemRecharged, 1e-9)
	require.InDelta(t, 125, summary.TotalRecharged, 1e-9)
}

func TestGetUserFundingSummaryReadsCompletedOrderWithoutRedeemMirror(t *testing.T) {
	client := newPaymentOrderLifecycleTestClient(t)
	ctx := context.Background()
	now := time.Now().UTC()

	user, err := client.User.Create().
		SetEmail("order-only@example.com").
		SetPasswordHash("test").
		Save(ctx)
	require.NoError(t, err)
	_, err = client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(320).
		SetPayAmount(320).
		SetRechargeCode("ORDER-WITHOUT-REDEEM-MIRROR").
		SetOutTradeNo("funding-summary-order-only").
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("funding-summary-order-only-trade").
		SetOrderType(payment.OrderTypeBalance).
		SetStatus(OrderStatusCompleted).
		SetExpiresAt(now.Add(time.Hour)).
		SetClientIP("127.0.0.1").
		SetSrcHost("example.com").
		Save(ctx)
	require.NoError(t, err)

	summary, err := (&PaymentService{entClient: client}).GetUserFundingSummary(ctx, user.ID)
	require.NoError(t, err)
	require.InDelta(t, 320, summary.OrderRecharged, 1e-9)
	require.Zero(t, summary.RedeemRecharged)
	require.InDelta(t, 320, summary.TotalRecharged, 1e-9)
}

func TestGetUserFundingSummarySubtractsSuccessfulOrderRefund(t *testing.T) {
	client := newPaymentOrderLifecycleTestClient(t)
	ctx := context.Background()
	now := time.Now().UTC()

	user, err := client.User.Create().
		SetEmail("refunded-order@example.com").
		SetPasswordHash("test").
		Save(ctx)
	require.NoError(t, err)
	_, err = client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(200).
		SetPayAmount(200).
		SetRechargeCode("PARTIALLY-REFUNDED-ORDER").
		SetOutTradeNo("funding-summary-refunded-order").
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("funding-summary-refunded-trade").
		SetOrderType(payment.OrderTypeBalance).
		SetStatus(OrderStatusPartiallyRefunded).
		SetRefundAmount(60).
		SetExpiresAt(now.Add(time.Hour)).
		SetClientIP("127.0.0.1").
		SetSrcHost("example.com").
		Save(ctx)
	require.NoError(t, err)

	summary, err := (&PaymentService{entClient: client}).GetUserFundingSummary(ctx, user.ID)
	require.NoError(t, err)
	require.InDelta(t, 140, summary.OrderRecharged, 1e-9)
	require.Zero(t, summary.RedeemRecharged)
	require.InDelta(t, 140, summary.TotalRecharged, 1e-9)
}

func TestGetUserFundingSummaryCountsFailedOrderWhenItsCreditWasApplied(t *testing.T) {
	client := newPaymentOrderLifecycleTestClient(t)
	ctx := context.Background()
	now := time.Now().UTC()

	user, err := client.User.Create().
		SetEmail("failed-but-credited@example.com").
		SetPasswordHash("test").
		Save(ctx)
	require.NoError(t, err)
	_, err = client.RedeemCode.Create().
		SetCode("FAILED-ORDER-CREDIT").
		SetType(RedeemTypeBalance).
		SetValue(90).
		SetStatus(StatusUsed).
		SetUsedBy(user.ID).
		SetUsedAt(now).
		Save(ctx)
	require.NoError(t, err)
	_, err = client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(90).
		SetPayAmount(90).
		SetRechargeCode("FAILED-ORDER-CREDIT").
		SetOutTradeNo("funding-summary-failed-credited").
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("funding-summary-failed-credited-trade").
		SetOrderType(payment.OrderTypeBalance).
		SetStatus(OrderStatusFailed).
		SetExpiresAt(now.Add(time.Hour)).
		SetClientIP("127.0.0.1").
		SetSrcHost("example.com").
		Save(ctx)
	require.NoError(t, err)

	summary, err := (&PaymentService{entClient: client}).GetUserFundingSummary(ctx, user.ID)
	require.NoError(t, err)
	require.InDelta(t, 90, summary.OrderRecharged, 1e-9)
	require.Zero(t, summary.RedeemRecharged)
	require.InDelta(t, 90, summary.TotalRecharged, 1e-9)
}

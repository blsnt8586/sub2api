package service

import (
	"context"
	"errors"
	"fmt"
)

// UserFundingSummary separates balance credited by payment orders from
// standalone balance redemptions.
type UserFundingSummary struct {
	OrderRecharged  float64 `json:"order_recharged"`
	RedeemRecharged float64 `json:"redeem_recharged"`
	TotalRecharged  float64 `json:"total_recharged"`
}

// GetUserFundingSummary returns lifetime successful positive balance credits.
// Orders and redemptions are summed from their own business tables. Balance
// order fulfillment creates an internal redeem code, so those codes are
// excluded from the standalone redemption subtotal to avoid double counting.
func (s *PaymentService) GetUserFundingSummary(ctx context.Context, userID int64) (*UserFundingSummary, error) {
	if s == nil || s.entClient == nil {
		return nil, errors.New("payment service is not ready")
	}
	if userID <= 0 {
		return nil, errors.New("invalid user id")
	}

	rows, err := s.entClient.QueryContext(ctx, `
		SELECT
			COALESCE((
				SELECT SUM(
					CASE
						WHEN po.status IN ('REFUNDED', 'PARTIALLY_REFUNDED')
							THEN CASE WHEN po.amount > po.refund_amount THEN po.amount - po.refund_amount ELSE 0 END
						ELSE po.amount
					END
				)
				FROM payment_orders po
				WHERE po.user_id = $1
				  AND po.order_type = 'balance'
				  AND po.amount > 0
				  AND (
					po.status IN (
						'COMPLETED',
						'REFUND_REQUESTED',
						'REFUNDING',
						'REFUND_PENDING',
						'REFUND_FAILED',
						'PARTIALLY_REFUNDED',
						'REFUNDED'
					)
					OR EXISTS (
					SELECT 1
					FROM redeem_codes rc
					WHERE rc.code = po.recharge_code
					  AND rc.used_by = po.user_id
					  AND rc.status = 'used'
					  AND rc.value > 0
					  AND rc.type = 'balance'
					)
				  )
			), 0) AS order_recharged,
			COALESCE((
				SELECT SUM(rc.value)
				FROM redeem_codes rc
				WHERE rc.used_by = $1
				  AND rc.status = 'used'
				  AND rc.value > 0
				  AND rc.type IN ('balance', 'admin_balance')
				  AND NOT EXISTS (
					SELECT 1
					FROM payment_orders po
					WHERE po.user_id = rc.used_by
					  AND po.order_type = 'balance'
					  AND po.recharge_code = rc.code
				  )
			), 0) AS redeem_recharged
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("query user funding summary: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := &UserFundingSummary{}
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("read user funding summary: %w", err)
		}
		return result, nil
	}
	if err := rows.Scan(&result.OrderRecharged, &result.RedeemRecharged); err != nil {
		return nil, fmt.Errorf("scan user funding summary: %w", err)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read user funding summary: %w", err)
	}
	result.TotalRecharged = result.OrderRecharged + result.RedeemRecharged
	return result, nil
}

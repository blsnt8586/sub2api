package repository

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
)

var _ service.ProbeManagedAccountStateRepository = (*accountRepository)(nil)
var _ service.RuntimeRecoverableAccountStateRepository = (*accountRepository)(nil)
var _ service.AdminAccountStateRepository = (*accountRepository)(nil)

func probeOwnershipPayload(managed, runtimeRecoverable, groupsExhausted bool, exhaustedMessage string) (string, error) {
	payload, err := json.Marshal(map[string]any{
		service.Sub2APIProbeManagedAccountStatusExtraKey:      managed,
		service.Sub2APIRuntimeRecoverableAccountErrorExtraKey: runtimeRecoverable,
		service.Sub2APIProbeGroupsExhaustedExtraKey:           groupsExhausted,
		service.Sub2APIProbeGroupsExhaustedMessageExtraKey:    exhaustedMessage,
	})
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func probeOwnershipMatchPayload(key string) string {
	payload, _ := json.Marshal(map[string]any{key: true})
	return string(payload)
}

func adminProbeOwnershipReleasePayload() (string, error) {
	payload, err := json.Marshal(map[string]any{
		service.Sub2APIProbeManagedAccountStatusExtraKey:      false,
		service.Sub2APIRuntimeRecoverableAccountErrorExtraKey: false,
		service.Sub2APIProbeGroupsExhaustedExtraKey:           false,
		service.Sub2APIProbeGroupsExhaustedMessageExtraKey:    "",
		service.Sub2APIAdminAccountStateGenerationExtraKey:    uuid.NewString(),
	})
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

// SetProbeManagedError atomically claims probe ownership and quarantines the
// account. The admin generation prevents a stale probe from overriding a
// manual state change; existing probe/runtime ownership remains claimable only
// within the same generation.
func (r *accountRepository) SetProbeManagedError(ctx context.Context, id int64, errorMsg string, groupsExhausted bool, expectedAdminGeneration string) (bool, error) {
	if r == nil || r.sql == nil {
		return false, errors.New("account repository SQL executor is not configured")
	}
	exhaustedMessage := ""
	if groupsExhausted {
		exhaustedMessage = errorMsg
	}
	payload, err := probeOwnershipPayload(true, false, groupsExhausted, exhaustedMessage)
	if err != nil {
		return false, err
	}
	result, err := r.sql.ExecContext(ctx, `
		WITH updated AS (
			UPDATE accounts AS a
			SET status = $1,
				error_message = $2,
				schedulable = FALSE,
				extra = COALESCE(a.extra, '{}'::jsonb) || $3::jsonb,
				updated_at = NOW()
			WHERE a.id = $4
				AND a.deleted_at IS NULL
				AND (
					(a.status = $5 AND a.schedulable IS TRUE)
					OR COALESCE(a.extra, '{}'::jsonb) @> $6::jsonb
					OR COALESCE(a.extra, '{}'::jsonb) @> $7::jsonb
				)
				AND COALESCE(a.extra ->> $8::text, '') = $9
			RETURNING a.id
		)
		INSERT INTO scheduler_outbox (event_type, account_id, group_id, payload)
		SELECT $10, updated.id, NULL, NULL FROM updated
	`, service.StatusError, errorMsg, payload, id, service.StatusActive,
		probeOwnershipMatchPayload(service.Sub2APIRuntimeRecoverableAccountErrorExtraKey),
		probeOwnershipMatchPayload(service.Sub2APIProbeManagedAccountStatusExtraKey),
		service.Sub2APIAdminAccountStateGenerationExtraKey, expectedAdminGeneration,
		service.SchedulerOutboxEventAccountChanged)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil || affected == 0 {
		return false, err
	}
	r.syncSchedulerAccountSnapshotDetached(ctx, id)
	return true, nil
}

// RecoverProbeManagedAccount clears only a state that still carries probe or
// request-time recoverable ownership. Runtime limit/overload columns are left
// untouched because they may have been re-armed by a newer in-flight request.
func (r *accountRepository) RecoverProbeManagedAccount(ctx context.Context, id int64, expectedAdminGeneration string) (bool, error) {
	if r == nil || r.sql == nil {
		return false, errors.New("account repository SQL executor is not configured")
	}
	payload, err := probeOwnershipPayload(false, false, false, "")
	if err != nil {
		return false, err
	}
	result, err := r.sql.ExecContext(ctx, `
		WITH updated AS (
			UPDATE accounts AS a
			SET status = $1,
				error_message = '',
				schedulable = TRUE,
				extra = COALESCE(a.extra, '{}'::jsonb) || $2::jsonb,
				updated_at = NOW()
			WHERE a.id = $3
				AND a.deleted_at IS NULL
				AND (a.status = $4 OR (a.status = $1 AND a.schedulable IS TRUE))
				AND (
					COALESCE(a.extra, '{}'::jsonb) @> $5::jsonb
					OR COALESCE(a.extra, '{}'::jsonb) @> $6::jsonb
				)
				AND COALESCE(a.extra ->> $7::text, '') = $8
			RETURNING a.id
		)
		INSERT INTO scheduler_outbox (event_type, account_id, group_id, payload)
		SELECT $9, updated.id, NULL, NULL FROM updated
	`, service.StatusActive, payload, id, service.StatusError,
		probeOwnershipMatchPayload(service.Sub2APIProbeManagedAccountStatusExtraKey),
		probeOwnershipMatchPayload(service.Sub2APIRuntimeRecoverableAccountErrorExtraKey),
		service.Sub2APIAdminAccountStateGenerationExtraKey, expectedAdminGeneration,
		service.SchedulerOutboxEventAccountChanged)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil || affected == 0 {
		return false, err
	}
	r.syncSchedulerAccountSnapshotDetached(ctx, id)
	return true, nil
}

func (r *accountRepository) SetRuntimeRecoverableAccountError(ctx context.Context, id int64, errorMsg, expectedAdminGeneration string) (bool, error) {
	if r == nil || r.sql == nil {
		return false, errors.New("account repository SQL executor is not configured")
	}
	payload, err := probeOwnershipPayload(false, true, false, "")
	if err != nil {
		return false, err
	}
	result, err := r.sql.ExecContext(ctx, `
		WITH updated AS (
			UPDATE accounts AS a
			SET status = $1,
				error_message = $2,
				schedulable = FALSE,
				extra = COALESCE(a.extra, '{}'::jsonb) || $3::jsonb,
				updated_at = NOW()
			WHERE a.id = $4
				AND a.deleted_at IS NULL
				AND (
					(a.status = $5 AND a.schedulable IS TRUE)
					OR COALESCE(a.extra, '{}'::jsonb) @> $6::jsonb
				)
				AND COALESCE(a.extra ->> $7::text, '') = $8
			RETURNING a.id
		)
		INSERT INTO scheduler_outbox (event_type, account_id, group_id, payload)
		SELECT $9, updated.id, NULL, NULL FROM updated
	`, service.StatusError, errorMsg, payload, id, service.StatusActive,
		probeOwnershipMatchPayload(service.Sub2APIRuntimeRecoverableAccountErrorExtraKey),
		service.Sub2APIAdminAccountStateGenerationExtraKey, expectedAdminGeneration,
		service.SchedulerOutboxEventAccountChanged)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil || affected == 0 {
		return false, err
	}
	r.syncSchedulerAccountSnapshotDetached(ctx, id)
	return true, nil
}

func (r *accountRepository) SetAdminAccountError(ctx context.Context, id int64, errorMsg string) error {
	if r == nil || r.sql == nil {
		return errors.New("account repository SQL executor is not configured")
	}
	payload, err := adminProbeOwnershipReleasePayload()
	if err != nil {
		return err
	}
	result, err := r.sql.ExecContext(ctx, `
		WITH updated AS (
			UPDATE accounts AS a
			SET status = $1, error_message = $2, schedulable = FALSE,
				extra = COALESCE(a.extra, '{}'::jsonb) || $3::jsonb, updated_at = NOW()
			WHERE a.id = $4 AND a.deleted_at IS NULL
			RETURNING a.id
		)
		INSERT INTO scheduler_outbox (event_type, account_id, group_id, payload)
		SELECT $5, updated.id, NULL, NULL FROM updated
	`, service.StatusError, errorMsg, payload, id, service.SchedulerOutboxEventAccountChanged)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return service.ErrAccountNotFound
	}
	r.syncSchedulerAccountSnapshotDetached(ctx, id)
	return nil
}

func (r *accountRepository) ClearAdminAccountError(ctx context.Context, id int64) error {
	if r == nil || r.sql == nil {
		return errors.New("account repository SQL executor is not configured")
	}
	payload, err := adminProbeOwnershipReleasePayload()
	if err != nil {
		return err
	}
	result, err := r.sql.ExecContext(ctx, `
		WITH updated AS (
			UPDATE accounts AS a
			SET status = $1, error_message = '', schedulable = TRUE,
				extra = COALESCE(a.extra, '{}'::jsonb) || $2::jsonb, updated_at = NOW()
			WHERE a.id = $3 AND a.deleted_at IS NULL
			RETURNING a.id
		)
		INSERT INTO scheduler_outbox (event_type, account_id, group_id, payload)
		SELECT $4, updated.id, NULL, NULL FROM updated
	`, service.StatusActive, payload, id, service.SchedulerOutboxEventAccountChanged)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return service.ErrAccountNotFound
	}
	r.syncSchedulerAccountSnapshotDetached(ctx, id)
	return nil
}

func (r *accountRepository) SetAdminAccountSchedulable(ctx context.Context, id int64, schedulable bool) error {
	if r == nil || r.sql == nil {
		return errors.New("account repository SQL executor is not configured")
	}
	payload, err := adminProbeOwnershipReleasePayload()
	if err != nil {
		return err
	}
	result, err := r.sql.ExecContext(ctx, `
		WITH updated AS (
			UPDATE accounts AS a
			SET schedulable = $1,
				extra = COALESCE(a.extra, '{}'::jsonb) || CASE
					WHEN a.status = $5 AND a.schedulable IS TRUE AND $1 = FALSE THEN $2::jsonb
					ELSE '{}'::jsonb
				END,
				updated_at = NOW()
			WHERE a.id = $3 AND a.deleted_at IS NULL
			RETURNING a.id
		)
		INSERT INTO scheduler_outbox (event_type, account_id, group_id, payload)
		SELECT $4, updated.id, NULL, NULL FROM updated
	`, schedulable, payload, id, service.SchedulerOutboxEventAccountChanged, service.StatusActive)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return service.ErrAccountNotFound
	}
	r.syncSchedulerAccountSnapshotDetached(ctx, id)
	return nil
}

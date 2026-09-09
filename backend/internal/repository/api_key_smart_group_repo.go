package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type apiKeySmartGroupRepository struct {
	db *sql.DB
}

func NewAPIKeySmartGroupRepository(db *sql.DB) service.APIKeySmartGroupStateRepository {
	return &apiKeySmartGroupRepository{db: db}
}

func scanSmartGroupState(rows *sql.Rows) (*service.APIKeySmartGroupRuntimeState, error) {
	if !rows.Next() {
		return nil, sql.ErrNoRows
	}
	state := &service.APIKeySmartGroupRuntimeState{}
	var groupIDsJSON []byte
	if err := rows.Scan(
		&state.APIKeyID,
		&state.UserID,
		&state.Key,
		&state.GroupID,
		&groupIDsJSON,
		&state.FailureThreshold,
		&state.RecoveryIntervalSeconds,
		&state.ConsecutiveFailures,
		&state.HealthySince,
		&state.LastProbeAt,
		&state.ProbeLeaseUntil,
	); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(groupIDsJSON, &state.SmartGroupIDs); err != nil {
		return nil, err
	}
	return state, rows.Err()
}

func (r *apiKeySmartGroupRepository) RecordSmartGroupOutcome(ctx context.Context, apiKeyID, groupID int64, success bool, errorMessage string, now time.Time) (*service.APIKeySmartGroupRuntimeState, error) {
	rows, err := r.db.QueryContext(ctx, `
		UPDATE api_keys
		   SET smart_group_consecutive_failures = CASE WHEN $3 THEN 0 ELSE smart_group_consecutive_failures + 1 END,
		       smart_group_healthy_since = CASE WHEN $3 THEN COALESCE(smart_group_healthy_since, $5) ELSE NULL END,
		       smart_group_last_error = CASE WHEN $3 THEN '' ELSE LEFT($4, 2000) END,
		       updated_at = $5
		 WHERE id = $1
		   AND group_id = $2
		   AND deleted_at IS NULL
		   AND smart_group_enabled = TRUE
		RETURNING id, user_id, key, group_id, smart_group_ids,
		          smart_group_failure_threshold, smart_group_recovery_interval_seconds,
		          smart_group_consecutive_failures, smart_group_healthy_since,
		          smart_group_last_probe_at, smart_group_probe_lease_until`,
		apiKeyID, groupID, success, errorMessage, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	state, err := scanSmartGroupState(rows)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return state, err
}

func (r *apiKeySmartGroupRepository) TryAcquireSmartGroupProbe(ctx context.Context, apiKeyID, groupID int64, now time.Time, cooldown, lease time.Duration) (*service.APIKeySmartGroupRuntimeState, bool, error) {
	rows, err := r.db.QueryContext(ctx, `
		UPDATE api_keys
		   SET smart_group_probe_lease_until = $5,
		       smart_group_last_probe_at = $3,
		       updated_at = $3
		 WHERE id = $1
		   AND group_id = $2
		   AND deleted_at IS NULL
		   AND smart_group_enabled = TRUE
		   AND (smart_group_probe_lease_until IS NULL OR smart_group_probe_lease_until <= $3)
		   AND (smart_group_last_probe_at IS NULL OR smart_group_last_probe_at <= $4)
			   AND (
			       smart_group_consecutive_failures >= smart_group_failure_threshold
		       OR (
		           smart_group_healthy_since IS NOT NULL
		           AND smart_group_healthy_since + smart_group_recovery_interval_seconds * INTERVAL '1 second' <= $3
		       )
		   )
		RETURNING id, user_id, key, group_id, smart_group_ids,
		          smart_group_failure_threshold, smart_group_recovery_interval_seconds,
		          smart_group_consecutive_failures, smart_group_healthy_since,
		          smart_group_last_probe_at, smart_group_probe_lease_until`,
		apiKeyID, groupID, now, now.Add(-cooldown), now.Add(lease))
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	state, err := scanSmartGroupState(rows)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	return state, err == nil, err
}

func (r *apiKeySmartGroupRepository) CompleteSmartGroupSwitch(ctx context.Context, apiKeyID, expectedGroupID, newGroupID int64, reason string, expectedLeaseUntil, now time.Time) (string, bool, error) {
	// Keep the route update and its audit row in one transaction.  Otherwise a
	// successful switch could be visible to users while its history write is
	// lost due to a transient database failure.
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return "", false, err
	}
	rollback := func() {
		_ = tx.Rollback()
	}

	var (
		fromGroupID   int64
		fromGroupName string
		toGroupName   string
	)
	err = tx.QueryRowContext(ctx, `
		SELECT k.group_id,
		       COALESCE(from_group.name, ''),
		       COALESCE(to_group.name, '')
		  FROM api_keys k
		  LEFT JOIN groups from_group ON from_group.id = k.group_id
		  LEFT JOIN groups to_group ON to_group.id = $3
		 WHERE k.id = $1
		   AND k.group_id = $2
		   AND k.deleted_at IS NULL
		   AND k.smart_group_enabled = TRUE
		   AND k.smart_group_probe_lease_until = $6
		   AND k.smart_group_ids @> to_jsonb(ARRAY[$3::bigint])
		   AND (
		       ($4 = 'failure' AND k.smart_group_consecutive_failures >= k.smart_group_failure_threshold)
		       OR ($4 = 'cost_recovery' AND k.smart_group_healthy_since IS NOT NULL
		           AND k.smart_group_healthy_since + k.smart_group_recovery_interval_seconds * INTERVAL '1 second' <= $5)
		   )
		 FOR UPDATE OF k`, apiKeyID, expectedGroupID, newGroupID, reason, now, expectedLeaseUntil).
		Scan(&fromGroupID, &fromGroupName, &toGroupName)
	if err == sql.ErrNoRows {
		rollback()
		return "", false, nil
	}
	if err != nil {
		rollback()
		return "", false, err
	}

	var switchedKey string
	err = tx.QueryRowContext(ctx, `
		UPDATE api_keys
		   SET group_id = $3,
		       smart_group_consecutive_failures = 0,
		       smart_group_healthy_since = $5,
		       smart_group_last_switch_at = $5,
		       smart_group_last_switch_reason = $4,
		       smart_group_last_error = '',
		       smart_group_probe_lease_until = NULL,
		       updated_at = $5
		 WHERE id = $1
		   AND group_id = $2
		   AND deleted_at IS NULL
		   AND smart_group_enabled = TRUE
		   AND smart_group_probe_lease_until = $6
		   AND smart_group_ids @> to_jsonb(ARRAY[$3::bigint])
		   AND (
		       ($4 = 'failure' AND smart_group_consecutive_failures >= smart_group_failure_threshold)
		       OR ($4 = 'cost_recovery' AND smart_group_healthy_since IS NOT NULL
		           AND smart_group_healthy_since + smart_group_recovery_interval_seconds * INTERVAL '1 second' <= $5)
		   )
		RETURNING key`, apiKeyID, expectedGroupID, newGroupID, reason, now, expectedLeaseUntil).Scan(&switchedKey)
	if err == sql.ErrNoRows {
		rollback()
		return "", false, nil
	}
	if err != nil {
		rollback()
		return "", false, err
	}

	if _, err = tx.ExecContext(ctx, `
		INSERT INTO api_key_smart_group_switch_logs
		       (api_key_id, from_group_id, from_group_name, to_group_id, to_group_name, reason, switched_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		apiKeyID, fromGroupID, fromGroupName, newGroupID, toGroupName, reason, now); err != nil {
		rollback()
		return "", false, err
	}
	// Enforce the three-day retention window even on installations where no
	// switch has occurred for a while; the hourly service cleanup is a second
	// line of defence for completely idle keys.
	if _, err = tx.ExecContext(ctx, `
		DELETE FROM api_key_smart_group_switch_logs
		 WHERE switched_at < $1`, now.Add(-service.SmartGroupSwitchLogRetention)); err != nil {
		rollback()
		return "", false, err
	}
	if err = tx.Commit(); err != nil {
		rollback()
		return "", false, err
	}
	return switchedKey, true, nil
}

// CompleteSmartGroupFailureSwitch atomically advances an unhealthy key to the
// next configured group without probing it first. The triggering user request
// has already proved the current route unhealthy; delaying the route update for
// another upstream call only extends the outage. The new route starts with no
// healthy_since timestamp and must earn its stable window through real success.
func (r *apiKeySmartGroupRepository) CompleteSmartGroupFailureSwitch(ctx context.Context, apiKeyID, expectedGroupID, newGroupID int64, now time.Time) (string, bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return "", false, err
	}
	rollback := func() { _ = tx.Rollback() }

	var (
		fromGroupName string
		toGroupName   string
	)
	err = tx.QueryRowContext(ctx, `
		SELECT COALESCE(from_group.name, ''), COALESCE(to_group.name, '')
		  FROM api_keys k
		  LEFT JOIN groups from_group ON from_group.id = k.group_id
		  LEFT JOIN groups to_group ON to_group.id = $3
		 WHERE k.id = $1
		   AND k.group_id = $2
		   AND k.deleted_at IS NULL
		   AND k.smart_group_enabled = TRUE
		   AND k.smart_group_consecutive_failures >= k.smart_group_failure_threshold
		   AND k.smart_group_ids @> to_jsonb(ARRAY[$3::bigint])
		 FOR UPDATE OF k`, apiKeyID, expectedGroupID, newGroupID).
		Scan(&fromGroupName, &toGroupName)
	if err == sql.ErrNoRows {
		rollback()
		return "", false, nil
	}
	if err != nil {
		rollback()
		return "", false, err
	}

	var switchedKey string
	err = tx.QueryRowContext(ctx, `
		UPDATE api_keys
		   SET group_id = $3,
		       smart_group_consecutive_failures = 0,
		       smart_group_healthy_since = NULL,
		       smart_group_last_switch_at = $4,
		       smart_group_last_switch_reason = 'failure',
		       smart_group_probe_lease_until = NULL,
		       updated_at = $4
		 WHERE id = $1
		   AND group_id = $2
		   AND deleted_at IS NULL
		   AND smart_group_enabled = TRUE
		   AND smart_group_consecutive_failures >= smart_group_failure_threshold
		   AND smart_group_ids @> to_jsonb(ARRAY[$3::bigint])
		RETURNING key`, apiKeyID, expectedGroupID, newGroupID, now).Scan(&switchedKey)
	if err == sql.ErrNoRows {
		rollback()
		return "", false, nil
	}
	if err != nil {
		rollback()
		return "", false, err
	}

	if _, err = tx.ExecContext(ctx, `
		INSERT INTO api_key_smart_group_switch_logs
		       (api_key_id, from_group_id, from_group_name, to_group_id, to_group_name, reason, switched_at)
		VALUES ($1, $2, $3, $4, $5, 'failure', $6)`,
		apiKeyID, expectedGroupID, fromGroupName, newGroupID, toGroupName, now); err != nil {
		rollback()
		return "", false, err
	}
	if _, err = tx.ExecContext(ctx, `
		DELETE FROM api_key_smart_group_switch_logs
		 WHERE switched_at < $1`, now.Add(-service.SmartGroupSwitchLogRetention)); err != nil {
		rollback()
		return "", false, err
	}
	if err = tx.Commit(); err != nil {
		rollback()
		return "", false, err
	}
	return switchedKey, true, nil
}

func (r *apiKeySmartGroupRepository) ListSmartGroupSwitchLogs(ctx context.Context, userID, apiKeyID int64, since time.Time, limit int) ([]service.APIKeySmartGroupSwitchLog, error) {
	if limit <= 0 || limit > service.SmartGroupSwitchLogMaxItems {
		limit = service.SmartGroupSwitchLogMaxItems
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT l.id, l.api_key_id, l.from_group_id, l.from_group_name,
		       l.to_group_id, l.to_group_name, l.reason, l.switched_at
		  FROM api_key_smart_group_switch_logs l
		  JOIN api_keys k ON k.id = l.api_key_id
		 WHERE k.user_id = $1
		   AND k.id = $2
		   AND k.deleted_at IS NULL
		   AND l.switched_at >= $3
		 ORDER BY l.switched_at DESC, l.id DESC
		 LIMIT $4`, userID, apiKeyID, since, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	logs := make([]service.APIKeySmartGroupSwitchLog, 0)
	for rows.Next() {
		var entry service.APIKeySmartGroupSwitchLog
		if err := rows.Scan(
			&entry.ID,
			&entry.APIKeyID,
			&entry.FromGroupID,
			&entry.FromGroupName,
			&entry.ToGroupID,
			&entry.ToGroupName,
			&entry.Reason,
			&entry.SwitchedAt,
		); err != nil {
			return nil, err
		}
		logs = append(logs, entry)
	}
	return logs, rows.Err()
}

func (r *apiKeySmartGroupRepository) DeleteExpiredSmartGroupSwitchLogs(ctx context.Context, before time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM api_key_smart_group_switch_logs
		 WHERE switched_at < $1`, before)
	return err
}

func (r *apiKeySmartGroupRepository) CompleteSmartGroupProbeWithoutSwitch(ctx context.Context, apiKeyID, expectedGroupID int64, reason, errorMessage string, expectedLeaseUntil, now time.Time) (string, error) {
	rows, err := r.db.QueryContext(ctx, `
		UPDATE api_keys
		   SET smart_group_probe_lease_until = NULL,
		       smart_group_healthy_since = CASE
		           WHEN $3 = 'cost_recovery' AND smart_group_healthy_since IS NOT NULL THEN $5
		           ELSE smart_group_healthy_since
		       END,
		       smart_group_last_error = LEFT($4, 2000),
		       updated_at = $5
		 WHERE id = $1
		   AND group_id = $2
		   AND deleted_at IS NULL
		   AND smart_group_enabled = TRUE
		   AND smart_group_probe_lease_until = $6
		RETURNING key`, apiKeyID, expectedGroupID, reason, errorMessage, now, expectedLeaseUntil)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	if !rows.Next() {
		return "", rows.Err()
	}
	var key string
	if err := rows.Scan(&key); err != nil {
		return "", err
	}
	return key, rows.Err()
}

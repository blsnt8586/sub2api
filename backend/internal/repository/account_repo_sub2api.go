// Sub2API 二开扩展：账号的 Provider 关联与定时优化相关数据访问。
//
// This adapter delegates shared account reads to the upstream repository and
// owns only Provider-specific SQL writes. Keeping it separate prevents the
// upstream AccountRepository contract and unrelated test doubles from growing
// whenever Provider management gains a capability.
package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type sub2APIAccountRepository struct {
	base service.AccountRepository
	sql  sqlExecutor
}

var _ service.ProbeManagedAccountStateRepository = (*sub2APIAccountRepository)(nil)

func NewSub2APIAccountRepository(base service.AccountRepository, db *sql.DB) service.Sub2APIAccountRepository {
	return &sub2APIAccountRepository{base: base, sql: db}
}

func (r *sub2APIAccountRepository) GetByID(ctx context.Context, id int64) (*service.Account, error) {
	return r.base.GetByID(ctx, id)
}

func (r *sub2APIAccountRepository) GetByIDs(ctx context.Context, ids []int64) ([]*service.Account, error) {
	return r.base.GetByIDs(ctx, ids)
}

func (r *sub2APIAccountRepository) SetError(ctx context.Context, id int64, errorMsg string) error {
	return r.base.SetError(ctx, id, errorMsg)
}

func (r *sub2APIAccountRepository) ClearError(ctx context.Context, id int64) error {
	return r.base.ClearError(ctx, id)
}

func (r *sub2APIAccountRepository) UpdateExtra(ctx context.Context, id int64, updates map[string]any) error {
	return r.base.UpdateExtra(ctx, id, updates)
}

func (r *sub2APIAccountRepository) SetProbeManagedError(ctx context.Context, id int64, message string, groupsExhausted bool, expectedAdminGeneration string) (bool, error) {
	stateRepo, ok := r.base.(service.ProbeManagedAccountStateRepository)
	if !ok {
		return false, errors.New("base account repository does not support atomic probe state")
	}
	return stateRepo.SetProbeManagedError(ctx, id, message, groupsExhausted, expectedAdminGeneration)
}

func (r *sub2APIAccountRepository) RecoverProbeManagedAccount(ctx context.Context, id int64, expectedAdminGeneration string) (bool, error) {
	stateRepo, ok := r.base.(service.ProbeManagedAccountStateRepository)
	if !ok {
		return false, errors.New("base account repository does not support atomic probe state")
	}
	return stateRepo.RecoverProbeManagedAccount(ctx, id, expectedAdminGeneration)
}

// These runtime-state methods remain on the Provider adapter's compatibility
// surface. Probe recovery deliberately leaves newer rate-limit and temporary
// scheduling blocks to their owning state machines.
func (r *sub2APIAccountRepository) ClearTempUnschedulable(ctx context.Context, id int64) error {
	return r.base.ClearTempUnschedulable(ctx, id)
}

func (r *sub2APIAccountRepository) ClearRateLimit(ctx context.Context, id int64) error {
	return r.base.ClearRateLimit(ctx, id)
}

// UpdateProviderLink 更新 Account 的 Provider 关联
func (r *sub2APIAccountRepository) UpdateProviderLink(ctx context.Context, accountID, providerID, providerAPIKeyID int64) error {
	result, err := r.sql.ExecContext(ctx,
		`UPDATE accounts
			 SET provider_id = $1, provider_api_key_id = $2,
			     proxy_id = (SELECT proxy_id FROM sub2api_providers WHERE id = $1 AND deleted_at IS NULL),
			     proxy_fallback_origin_id = NULL,
			     remote_group_id = NULL, remote_group_name = NULL,
			     remote_group_multiplier = NULL, remote_group_synced_at = NULL,
			     sub2api_optimize_group_id = NULL,
			     extra = COALESCE(extra, '{}'::jsonb) - 'sub2api_optimize_group_ids',
			     updated_at = NOW()
			 WHERE id = $3 AND deleted_at IS NULL`,
		providerID, providerAPIKeyID, accountID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	if err := enqueueSchedulerOutbox(ctx, r.sql, service.SchedulerOutboxEventAccountChanged, &accountID, nil, nil); err != nil {
		logger.LegacyPrintf("repository.account", "[SchedulerOutbox] enqueue provider link change failed: account=%d err=%v", accountID, err)
	}
	return nil
}

// ClearProviderLink 清除 Account 的 Provider 关联
func (r *sub2APIAccountRepository) ClearProviderLink(ctx context.Context, accountID, providerID int64) error {
	result, err := r.sql.ExecContext(ctx,
		`UPDATE accounts
			 SET provider_id = NULL, provider_api_key_id = NULL,
			     proxy_id = NULL, proxy_fallback_origin_id = NULL,
			     remote_group_id = NULL, remote_group_name = NULL, remote_group_multiplier = NULL, remote_group_synced_at = NULL,
		     sub2api_optimize_enabled = FALSE, sub2api_optimize_group_id = NULL,
		     extra = COALESCE(extra, '{}'::jsonb) - 'sub2api_optimize_group_ids',
		     updated_at = NOW()
		 WHERE id = $1 AND provider_id = $2 AND deleted_at IS NULL`,
		accountID, providerID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	if err := enqueueSchedulerOutbox(ctx, r.sql, service.SchedulerOutboxEventAccountChanged, &accountID, nil, nil); err != nil {
		logger.LegacyPrintf("repository.account", "[SchedulerOutbox] enqueue provider unlink change failed: account=%d err=%v", accountID, err)
	}
	return nil
}

// UpdateProviderAccountsProxy applies the Provider's selected route to all
// linked accounts. Clearing the Provider proxy restores direct connections.
func (r *sub2APIAccountRepository) UpdateProviderAccountsProxy(ctx context.Context, providerID int64, proxyID *int64) error {
	rows, err := r.sql.QueryContext(ctx, `
		UPDATE accounts
		   SET proxy_id = $1, proxy_fallback_origin_id = NULL, updated_at = NOW()
		 WHERE provider_id = $2 AND deleted_at IS NULL
		 RETURNING id`, proxyID, providerID)
	if err != nil {
		return err
	}
	accountIDs := make([]int64, 0)
	for rows.Next() {
		var accountID int64
		if err := rows.Scan(&accountID); err != nil {
			_ = rows.Close()
			return err
		}
		accountIDs = append(accountIDs, accountID)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if len(accountIDs) == 0 {
		return nil
	}
	sort.Slice(accountIDs, func(i, j int) bool { return accountIDs[i] < accountIDs[j] })
	if err := enqueueSchedulerOutbox(ctx, r.sql, service.SchedulerOutboxEventAccountBulkChanged, nil, nil, map[string]any{
		"account_ids": accountIDs,
	}); err != nil {
		logger.LegacyPrintf("repository.account", "[SchedulerOutbox] enqueue provider proxy changes failed: provider=%d err=%v", providerID, err)
	}
	return nil
}

// UpdateRemoteGroupBinding persists the remote group's stable ID together with
// its display cache.
func (r *sub2APIAccountRepository) UpdateRemoteGroupBinding(ctx context.Context, accountID, groupID int64, groupName string, multiplier float64) error {
	result, err := r.sql.ExecContext(ctx,
		`UPDATE accounts
		 SET remote_group_id = $1, remote_group_name = $2, remote_group_multiplier = $3,
		     remote_group_synced_at = NOW(), updated_at = NOW()
		 WHERE id = $4 AND deleted_at IS NULL`,
		groupID, groupName, multiplier, accountID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *sub2APIAccountRepository) UpdateRemoteGroupIdentity(ctx context.Context, accountID, groupID int64) error {
	result, err := r.sql.ExecContext(ctx,
		`UPDATE accounts
		 SET remote_group_id = $1, remote_group_name = NULL, remote_group_multiplier = NULL,
		     remote_group_synced_at = NOW(), updated_at = NOW()
		 WHERE id = $2 AND deleted_at IS NULL`,
		groupID, accountID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// ClearRemoteGroupBinding removes a stale remote group/key snapshot while
// keeping the local Provider account association intact. A missing upstream
// key must not continue contributing an obsolete procurement multiplier to a
// dynamic local group.
func (r *sub2APIAccountRepository) ClearRemoteGroupBinding(ctx context.Context, accountID int64) error {
	result, err := r.sql.ExecContext(ctx,
		`UPDATE accounts
		 SET remote_group_id = NULL, remote_group_name = NULL,
		     remote_group_multiplier = NULL, remote_group_synced_at = NOW(),
		     updated_at = NOW()
		 WHERE id = $1 AND deleted_at IS NULL`,
		accountID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// UpdateSub2APIOptimizeSettings 全量覆盖账号的定时优化配置。
// enabled=false 时仍保留倍率、模型和可选分组；enabled=true 时由 service
// 和数据库约束保证三个必填项非空。
func (r *sub2APIAccountRepository) UpdateSub2APIOptimizeSettings(ctx context.Context, providerID, accountID int64, enabled bool, minMultiplier, maxMultiplier *float64, testModel *string, groupID *int64) error {
	result, err := r.sql.ExecContext(ctx,
		`UPDATE accounts
		 SET sub2api_optimize_enabled = $1, sub2api_min_multiplier = $2, sub2api_max_multiplier = $3,
		     sub2api_test_model = $4, sub2api_optimize_group_id = $5, updated_at = NOW()
		 WHERE id = $6 AND provider_id = $7 AND provider_api_key_id IS NOT NULL AND deleted_at IS NULL`,
		enabled, minMultiplier, maxMultiplier, testModel, groupID, accountID, providerID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// UpdateSub2APIOptimizeSettingsWithGroupIDs persists the multi-select remote
// group restriction in account.extra while mirroring the first ID to the
// legacy scalar column for older readers.
func (r *sub2APIAccountRepository) UpdateSub2APIOptimizeSettingsWithGroupIDs(ctx context.Context, providerID, accountID int64, enabled bool, minMultiplier, maxMultiplier *float64, testModel *string, groupIDs []int64) error {
	groupIDs = service.NormalizeSub2APIOptimizeGroupIDsForPersistence(groupIDs)
	var legacyGroupID *int64
	if len(groupIDs) > 0 {
		legacyGroupID = &groupIDs[0]
	}
	var encoded any
	if len(groupIDs) > 0 {
		data, err := json.Marshal(groupIDs)
		if err != nil {
			return err
		}
		encoded = string(data)
	}
	result, err := r.sql.ExecContext(ctx,
		`UPDATE accounts
		 SET sub2api_optimize_enabled = $1, sub2api_min_multiplier = $2, sub2api_max_multiplier = $3,
		     sub2api_test_model = $4, sub2api_optimize_group_id = $5,
		     extra = CASE WHEN $8::jsonb IS NULL
		       THEN COALESCE(extra, '{}'::jsonb) - 'sub2api_optimize_group_ids'
		       ELSE COALESCE(extra, '{}'::jsonb) || jsonb_build_object('sub2api_optimize_group_ids', $8::jsonb)
		     END,
		     updated_at = NOW()
		 WHERE id = $6 AND provider_id = $7 AND provider_api_key_id IS NOT NULL AND deleted_at IS NULL`,
		enabled, minMultiplier, maxMultiplier, testModel, legacyGroupID, accountID, providerID, encoded)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// ListByProviderID 获取关联到指定 Provider 的所有 Account（含远端分组信息）
func (r *sub2APIAccountRepository) ListByProviderID(ctx context.Context, providerID int64) ([]service.Account, error) {
	rows, err := r.sql.QueryContext(ctx, `
		SELECT id, name, platform, status, extra,
		       provider_id, provider_api_key_id, remote_group_id,
		       remote_group_name, remote_group_multiplier, remote_group_synced_at,
		       sub2api_optimize_enabled, sub2api_min_multiplier, sub2api_max_multiplier, sub2api_test_model,
		       sub2api_optimize_group_id
		  FROM accounts
		 WHERE provider_id = $1
		   AND deleted_at IS NULL
		 ORDER BY id ASC`,
		providerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []service.Account
	for rows.Next() {
		var a service.Account
		var provID, keyID, groupID sql.NullInt64
		var extraJSON []byte
		var groupName sql.NullString
		var groupMult sql.NullFloat64
		var groupSyncedAt sql.NullTime
		var minMult sql.NullFloat64
		var maxMult sql.NullFloat64
		var testModel sql.NullString
		var optimizeEnabled sql.NullBool
		var optimizeGroupID sql.NullInt64
		if err := rows.Scan(
			&a.ID, &a.Name, &a.Platform, &a.Status, &extraJSON,
			&provID, &keyID, &groupID,
			&groupName, &groupMult, &groupSyncedAt,
			&optimizeEnabled, &minMult, &maxMult, &testModel, &optimizeGroupID,
		); err != nil {
			return nil, err
		}
		if provID.Valid {
			v := provID.Int64
			a.ProviderID = &v
		}
		if len(extraJSON) > 0 {
			_ = json.Unmarshal(extraJSON, &a.Extra)
		}
		a.Sub2APIOptimizeGroupIDs = service.ParseSub2APIOptimizeGroupIDs(a.Extra)
		if keyID.Valid {
			v := keyID.Int64
			a.ProviderAPIKeyID = &v
		}
		if groupID.Valid {
			v := groupID.Int64
			a.RemoteGroupID = &v
		}
		if groupName.Valid {
			a.RemoteGroupName = &groupName.String
		}
		if groupMult.Valid {
			a.RemoteGroupMultiplier = &groupMult.Float64
		}
		if groupSyncedAt.Valid {
			t := groupSyncedAt.Time
			a.RemoteGroupSyncedAt = &t
		}
		if maxMult.Valid {
			a.Sub2APIMaxMultiplier = &maxMult.Float64
		}
		if minMult.Valid {
			a.Sub2APIMinMultiplier = &minMult.Float64
		}
		if testModel.Valid {
			a.Sub2APITestModel = &testModel.String
		}
		if optimizeEnabled.Valid {
			a.Sub2APIOptimizeEnabled = optimizeEnabled.Bool
		}
		if optimizeGroupID.Valid {
			v := optimizeGroupID.Int64
			a.Sub2APIOptimizeGroupID = &v
		}
		accounts = append(accounts, a)
	}
	return accounts, rows.Err()
}

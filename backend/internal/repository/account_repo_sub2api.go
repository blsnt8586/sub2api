// Sub2API 二开扩展：账号的 Provider 关联与定时优化相关数据访问。
//
// 这些方法都挂在 accountRepository 上（Go 允许同包 struct 方法跨文件），
// 但刻意从上游的 account_repo.go 中分离出来，独立成文件，以便同步上游时
// 避开与上游改动的合并冲突。详见 memory: sub2api-fork-isolation-principle。
package repository

import (
	"context"
	"database/sql"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// UpdateProviderLink 更新 Account 的 Provider 关联
func (r *accountRepository) UpdateProviderLink(ctx context.Context, accountID, providerID, providerAPIKeyID int64) error {
	_, err := r.sql.ExecContext(ctx,
		`UPDATE accounts
		 SET provider_id = $1, provider_api_key_id = $2, updated_at = NOW()
		 WHERE id = $3 AND deleted_at IS NULL`,
		providerID, providerAPIKeyID, accountID)
	return err
}

// ClearProviderLink 清除 Account 的 Provider 关联
func (r *accountRepository) ClearProviderLink(ctx context.Context, accountID, providerID int64) error {
	_, err := r.sql.ExecContext(ctx,
		`UPDATE accounts
		 SET provider_id = NULL, provider_api_key_id = NULL,
		     remote_group_name = NULL, remote_group_multiplier = NULL, remote_group_synced_at = NULL,
		     updated_at = NOW()
		 WHERE id = $1 AND provider_id = $2 AND deleted_at IS NULL`,
		accountID, providerID)
	return err
}

// UpdateRemoteGroupInfo 更新远程分组缓存信息
func (r *accountRepository) UpdateRemoteGroupInfo(ctx context.Context, accountID int64, groupName string, multiplier float64) error {
	_, err := r.sql.ExecContext(ctx,
		`UPDATE accounts
		 SET remote_group_name = $1, remote_group_multiplier = $2, remote_group_synced_at = NOW(), updated_at = NOW()
		 WHERE id = $3 AND deleted_at IS NULL`,
		groupName, multiplier, accountID)
	return err
}

// UpdateSub2APIOptimizeSettings 全量覆盖账号的定时优化配置（是否参与 + 倍率上限 + 测试模型）。
// enabled 独立控制是否参与定时优化；maxMultiplier/testModel 即使 enabled=false 也照常持久化保留，
// 便于用户重新开启时沿用上次的配置。maxMultiplier 为 nil 表示未设上限；testModel 为 nil 表示按平台默认。
func (r *accountRepository) UpdateSub2APIOptimizeSettings(ctx context.Context, accountID int64, enabled bool, minMultiplier, maxMultiplier *float64, testModel *string) error {
	_, err := r.sql.ExecContext(ctx,
		`UPDATE accounts
		 SET sub2api_optimize_enabled = $1, sub2api_min_multiplier = $2, sub2api_max_multiplier = $3, sub2api_test_model = $4, updated_at = NOW()
		 WHERE id = $5 AND deleted_at IS NULL`,
		enabled, minMultiplier, maxMultiplier, testModel, accountID)
	return err
}

// ListByProviderID 获取关联到指定 Provider 的所有 Account（含远端分组信息）
func (r *accountRepository) ListByProviderID(ctx context.Context, providerID int64) ([]service.Account, error) {
	rows, err := r.sql.QueryContext(ctx, `
		SELECT id, name, platform, status,
		       provider_id, provider_api_key_id,
		       remote_group_name, remote_group_multiplier, remote_group_synced_at,
		       sub2api_optimize_enabled, sub2api_min_multiplier, sub2api_max_multiplier, sub2api_test_model
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
		var provID, keyID sql.NullInt64
		var groupName sql.NullString
		var groupMult sql.NullFloat64
		var groupSyncedAt sql.NullTime
		var minMult sql.NullFloat64
		var maxMult sql.NullFloat64
		var testModel sql.NullString
		var optimizeEnabled sql.NullBool
		if err := rows.Scan(
			&a.ID, &a.Name, &a.Platform, &a.Status,
			&provID, &keyID,
			&groupName, &groupMult, &groupSyncedAt,
			&optimizeEnabled, &minMult, &maxMult, &testModel,
		); err != nil {
			return nil, err
		}
		if provID.Valid {
			v := provID.Int64
			a.ProviderID = &v
		}
		if keyID.Valid {
			v := keyID.Int64
			a.ProviderAPIKeyID = &v
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
		accounts = append(accounts, a)
	}
	return accounts, rows.Err()
}

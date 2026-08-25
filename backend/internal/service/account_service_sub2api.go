package service

import "context"

// Sub2APIAccountRepository is the Provider subsystem's narrow account port.
// It deliberately does not extend AccountRepository: unrelated gateway,
// billing, and scheduler repositories must not implement Provider-only writes.
type Sub2APIAccountRepository interface {
	GetByID(ctx context.Context, id int64) (*Account, error)
	GetByIDs(ctx context.Context, ids []int64) ([]*Account, error)
	SetError(ctx context.Context, id int64, errorMsg string) error
	ClearError(ctx context.Context, id int64) error
	UpdateExtra(ctx context.Context, id int64, updates map[string]any) error
	// UpdateProviderLink 更新 Account 的 Provider 关联
	UpdateProviderLink(ctx context.Context, accountID, providerID, providerAPIKeyID int64) error
	// ClearProviderLink 清除 Account 的 Provider 关联
	ClearProviderLink(ctx context.Context, accountID, providerID int64) error
	// UpdateProviderAccountsProxy applies one Provider route to its linked accounts.
	UpdateProviderAccountsProxy(ctx context.Context, providerID int64, proxyID *int64) error
	// UpdateRemoteGroupBinding persists the stable remote group identity and display cache.
	UpdateRemoteGroupBinding(ctx context.Context, accountID, groupID int64, groupName string, multiplier float64) error
	// UpdateRemoteGroupIdentity persists an ID while clearing unavailable display metadata.
	UpdateRemoteGroupIdentity(ctx context.Context, accountID, groupID int64) error
	// ListByProviderID 获取关联到指定 Provider 的所有 Account
	ListByProviderID(ctx context.Context, providerID int64) ([]Account, error)
	// UpdateSub2APIOptimizeSettings 全量覆盖账号的定时优化设置（是否参与、倍率下限、倍率上限、测试模型）
	UpdateSub2APIOptimizeSettings(ctx context.Context, providerID, accountID int64, enabled bool, minMultiplier, maxMultiplier *float64, testModel *string) error
}

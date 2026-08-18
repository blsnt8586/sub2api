package service

import "context"

// Sub2APIAccountRepository 是 AccountRepository 的二开扩展子接口，
// 聚合 Sub2API Provider 关联与定时优化相关的数据访问方法。
//
// 单独成文件的目的：把二开新增的接口方法与上游 AccountRepository 主体隔离，
// 主接口只需嵌入本接口一行，上游改动 account_service.go 时不会与这些方法产生冲突。
// 新增 Sub2API 相关仓储方法请加在这里，不要动主接口。
type Sub2APIAccountRepository interface {
	// UpdateProviderLink 更新 Account 的 Provider 关联
	UpdateProviderLink(ctx context.Context, accountID, providerID, providerAPIKeyID int64) error
	// ClearProviderLink 清除 Account 的 Provider 关联
	ClearProviderLink(ctx context.Context, accountID, providerID int64) error
	// UpdateRemoteGroupInfo 更新远程分组缓存信息
	UpdateRemoteGroupInfo(ctx context.Context, accountID int64, groupName string, multiplier float64) error
	// ListByProviderID 获取关联到指定 Provider 的所有 Account
	ListByProviderID(ctx context.Context, providerID int64) ([]Account, error)
	// UpdateSub2APIOptimizeSettings 全量覆盖账号的定时优化设置（是否参与、倍率下限、倍率上限、测试模型）
	UpdateSub2APIOptimizeSettings(ctx context.Context, providerID, accountID int64, enabled bool, minMultiplier, maxMultiplier *float64, testModel *string) error
}

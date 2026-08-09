package service

import (
	"context"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/sub2api"
)

var (
	ErrProviderNotFound    = infraerrors.NotFound("PROVIDER_NOT_FOUND", "provider not found")
	ErrProviderExists      = infraerrors.Conflict("PROVIDER_EXISTS", "provider with same base_url and email already exists")
	ErrInvalidProviderType = infraerrors.BadRequest("INVALID_PROVIDER_TYPE", "unsupported provider type")
)

// Sub2APIProviderRepository 定义 Provider 数据访问接口
type Sub2APIProviderRepository interface {
	Create(ctx context.Context, input *CreateSub2APIProviderInput) (*ent.Sub2APIProvider, error)
	GetByID(ctx context.Context, id int64) (*ent.Sub2APIProvider, error)
	GetByIDWithAccounts(ctx context.Context, id int64) (*ent.Sub2APIProvider, error)
	List(ctx context.Context, filters *Sub2APIProviderFilters, page, pageSize int) ([]*ent.Sub2APIProvider, int, error)
	ListAll(ctx context.Context, filters *Sub2APIProviderFilters) ([]*ent.Sub2APIProvider, error)
	Update(ctx context.Context, id int64, input *UpdateSub2APIProviderInput) (*ent.Sub2APIProvider, error)
	Delete(ctx context.Context, id int64) error
	UpdateSyncStatus(ctx context.Context, id int64, status string, errorMsg *string) error
	UpdateAPIPaths(ctx context.Context, id int64, keysPath, groupsPath string) error
}

// Repository Input/Filter Types
type CreateSub2APIProviderInput struct {
	Name         string
	BaseURL      string
	ProviderType string
	Email        string
	Password     string
	Notes        *string
}

type UpdateSub2APIProviderInput struct {
	Name     *string
	BaseURL  *string
	Email    *string
	Password *string
	Status   *string
	Notes    *string
}

type Sub2APIProviderFilters struct {
	Status string
	Search string
}

// Sub2APIProviderService 处理 Provider 业务逻辑
type Sub2APIProviderService struct {
	repo        Sub2APIProviderRepository
	accountRepo AccountRepository
	tokenCache  *sub2api.TokenCache // 复用各上游的登录 token，避免每次重新登录
}

// NewSub2APIProviderService 创建 Service 实例
func NewSub2APIProviderService(repo Sub2APIProviderRepository, accountRepo AccountRepository, tokenCache *sub2api.TokenCache) *Sub2APIProviderService {
	return &Sub2APIProviderService{repo: repo, accountRepo: accountRepo, tokenCache: tokenCache}
}

// newAuthedClient 为指定 Provider 创建已登录的客户端。
// 优先使用缓存 token，缓存未命中时登录并写入缓存。
func (s *Sub2APIProviderService) newAuthedClient(ctx context.Context, provider *ent.Sub2APIProvider) (*sub2api.Client, error) {
	client := sub2api.NewClient(provider.BaseURL, provider.Email, provider.PasswordEncrypted)
	if err := client.EnsureLoggedIn(ctx, int64(provider.ID), s.tokenCache); err != nil {
		return nil, fmt.Errorf("login to provider %d failed: %w", provider.ID, err)
	}
	return client, nil
}

// withAuthRetry 用已登录的 client 执行 fn；若收到 401 则清除缓存重新登录后重试一次。
func (s *Sub2APIProviderService) withAuthRetry(ctx context.Context, client *sub2api.Client, providerID int64, fn func() error) error {
	if err := fn(); err != nil {
		if sub2api.IsUnauthorized(err) {
			s.tokenCache.Evict(providerID)
			if loginErr := client.Login(ctx); loginErr != nil {
				return fmt.Errorf("re-login after 401 failed: %w", loginErr)
			}
			s.tokenCache.Set(providerID, client.Token)
			return fn()
		}
		return err
	}
	return nil
}

// CreateProvider 创建 Provider
func (s *Sub2APIProviderService) CreateProvider(ctx context.Context, input *CreateProviderInput) (*Provider, error) {
	// 上游类型：未显式指定时用默认（sub2api），并校验受支持
	providerType := input.ProviderType
	if providerType == "" {
		providerType = domain.ProviderTypeDefault
	}
	if !domain.IsValidProviderType(providerType) {
		return nil, ErrInvalidProviderType
	}

	// 调用 Repository
	provider, err := s.repo.Create(ctx, &CreateSub2APIProviderInput{
		Name:         input.Name,
		BaseURL:      input.BaseURL,
		ProviderType: providerType,
		Email:        input.Email,
		Password:     input.Password, // 阶段1明文存储
		Notes:        input.Notes,
	})

	if err != nil {
		if ent.IsConstraintError(err) {
			return nil, ErrProviderExists
		}
		return nil, fmt.Errorf("create provider failed: %w", err)
	}

	return providerFromEnt(provider), nil
}

// GetProvider 根据 ID 获取 Provider
func (s *Sub2APIProviderService) GetProvider(ctx context.Context, id int64) (*Provider, error) {
	provider, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrProviderNotFound
		}
		return nil, fmt.Errorf("get provider failed: %w", err)
	}

	return providerFromEnt(provider), nil
}

// GetProviderWithAccounts 获取 Provider 及其关联的 Accounts
func (s *Sub2APIProviderService) GetProviderWithAccounts(ctx context.Context, id int64) (*ProviderWithAccounts, error) {
	provider, err := s.repo.GetByIDWithAccounts(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrProviderNotFound
		}
		return nil, fmt.Errorf("get provider with accounts failed: %w", err)
	}

	return &ProviderWithAccounts{
		Provider:      providerFromEnt(provider),
		AccountsCount: len(provider.Edges.Accounts),
	}, nil
}

// ListProviders 列出 Provider（分页）
func (s *Sub2APIProviderService) ListProviders(
	ctx context.Context,
	page, pageSize int,
	status, search string,
) ([]*Provider, int, error) {
	filters := &Sub2APIProviderFilters{
		Status: status,
		Search: search,
	}

	providers, total, err := s.repo.List(ctx, filters, page, pageSize)
	if err != nil {
		return nil, 0, fmt.Errorf("list providers failed: %w", err)
	}

	result := make([]*Provider, len(providers))
	for i, p := range providers {
		result[i] = providerFromEnt(p)
	}

	return result, total, nil
}

// ListAllProviders 列出所有 Provider（不分页）
func (s *Sub2APIProviderService) ListAllProviders(ctx context.Context, status string) ([]*Provider, error) {
	filters := &Sub2APIProviderFilters{
		Status: status,
	}

	providers, err := s.repo.ListAll(ctx, filters)
	if err != nil {
		return nil, fmt.Errorf("list all providers failed: %w", err)
	}

	result := make([]*Provider, len(providers))
	for i, p := range providers {
		result[i] = providerFromEnt(p)
	}

	return result, nil
}

// UpdateProvider 更新 Provider
func (s *Sub2APIProviderService) UpdateProvider(ctx context.Context, id int64, input *UpdateProviderInput) (*Provider, error) {
	// 先检查是否存在
	_, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrProviderNotFound
		}
		return nil, err
	}

	// 更新
	provider, err := s.repo.Update(ctx, id, &UpdateSub2APIProviderInput{
		Name:     input.Name,
		BaseURL:  input.BaseURL,
		Email:    input.Email,
		Password: input.Password,
		Status:   input.Status,
		Notes:    input.Notes,
	})

	if err != nil {
		if ent.IsConstraintError(err) {
			return nil, ErrProviderExists
		}
		return nil, fmt.Errorf("update provider failed: %w", err)
	}

	return providerFromEnt(provider), nil
}

// DeleteProvider 删除 Provider
func (s *Sub2APIProviderService) DeleteProvider(ctx context.Context, id int64) error {
	// 先检查是否存在
	_, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return ErrProviderNotFound
		}
		return err
	}

	// 软删除
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete provider failed: %w", err)
	}

	return nil
}

// Helper functions

func providerFromEnt(e *ent.Sub2APIProvider) *Provider {
	if e == nil {
		return nil
	}

	// 转换时间字段
	var lastSyncAt *string
	if e.LastSyncAt != nil {
		s := e.LastSyncAt.Format("2006-01-02T15:04:05Z07:00")
		lastSyncAt = &s
	}

	// 关联账号数：仅在 eager-load（WithAccounts）后才有值，否则为 0
	accountsCount := len(e.Edges.Accounts)

	return &Provider{
		ID:             e.ID,
		Name:           e.Name,
		BaseURL:        e.BaseURL,
		ProviderType:   e.ProviderType,
		Status:         e.Status,
		Notes:          e.Notes,
		Email:          e.Email,
		Password:       e.PasswordEncrypted, // 阶段1明文，阶段7会解密
		APIPathKeys:    e.APIPathKeys,
		APIPathGroups:  e.APIPathGroups,
		LastSyncAt:     lastSyncAt,
		LastSyncStatus: e.LastSyncStatus,
		LastSyncError:  e.LastSyncError,
		CreatedAt:      e.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:      e.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		AccountsCount:  accountsCount,
	}
}

// Service Types

// Provider Service 层的 Provider 模型
type Provider struct {
	ID             int64   `json:"id"`
	Name           string  `json:"name"`
	BaseURL        string  `json:"base_url"`
	ProviderType   string  `json:"provider_type"`
	Status         string  `json:"status"`
	Notes          *string `json:"notes,omitempty"`
	Email          string  `json:"email"`
	Password       string  `json:"-"` // 不序列化到 JSON
	APIPathKeys    *string `json:"api_path_keys,omitempty"`
	APIPathGroups  *string `json:"api_path_groups,omitempty"`
	LastSyncAt     *string `json:"last_sync_at,omitempty"`
	LastSyncStatus *string `json:"last_sync_status,omitempty"`
	LastSyncError  *string `json:"last_sync_error,omitempty"`
	CreatedAt      string  `json:"created_at"`
	UpdatedAt      string  `json:"updated_at"`
	AccountsCount  int     `json:"accounts_count"`
}

// ProviderWithAccounts Provider 及其关联的 Account 数量
type ProviderWithAccounts struct {
	Provider      *Provider `json:"provider"`
	AccountsCount int       `json:"accounts_count"`
}

// CreateProviderInput 创建 Provider 的输入
type CreateProviderInput struct {
	Name         string  `json:"name"`
	BaseURL      string  `json:"base_url"`
	ProviderType string  `json:"provider_type,omitempty"`
	Email        string  `json:"email"`
	Password     string  `json:"password"`
	Notes        *string `json:"notes,omitempty"`
}

// UpdateProviderInput 更新 Provider 的输入
type UpdateProviderInput struct {
	Name     *string `json:"name,omitempty"`
	BaseURL  *string `json:"base_url,omitempty"`
	Email    *string `json:"email,omitempty"`
	Password *string `json:"password,omitempty"`
	Status   *string `json:"status,omitempty"`
	Notes    *string `json:"notes,omitempty"`
}

// DetectAndUpdateAPIPaths 探测并更新 API 路径
func (s *Sub2APIProviderService) DetectAndUpdateAPIPaths(ctx context.Context, id int64) (*PathDetectionResult, error) {
	// 获取 Provider
	provider, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrProviderNotFound
		}
		return nil, fmt.Errorf("get provider failed: %w", err)
	}

	// 创建已登录客户端（优先使用缓存 token）
	client, err := s.newAuthedClient(ctx, provider)
	if err != nil {
		errMsg := err.Error()
		_ = s.repo.UpdateSyncStatus(ctx, id, "failed", &errMsg)
		return nil, err
	}

	// 探测路径
	detector := sub2api.NewPathDetector(client)
	paths, err := detector.DetectAllPaths(ctx)
	if err != nil {
		// 更新同步状态为失败
		errMsg := err.Error()
		_ = s.repo.UpdateSyncStatus(ctx, id, "failed", &errMsg)
		return nil, fmt.Errorf("detect paths failed: %w", err)
	}

	// 更新路径到数据库
	if err := s.repo.UpdateAPIPaths(ctx, id, paths.KeysPath, paths.GroupsPath); err != nil {
		return nil, fmt.Errorf("update api paths failed: %w", err)
	}

	// 更新同步状态为成功
	if err := s.repo.UpdateSyncStatus(ctx, id, "success", nil); err != nil {
		return nil, fmt.Errorf("update sync status failed: %w", err)
	}

	return &PathDetectionResult{
		KeysPath:   paths.KeysPath,
		GroupsPath: paths.GroupsPath,
	}, nil
}

// TestProviderConnection 测试 Provider 连接（同时更新同步状态）
func (s *Sub2APIProviderService) TestProviderConnection(ctx context.Context, id int64) error {
	// 获取 Provider
	provider, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return ErrProviderNotFound
		}
		return fmt.Errorf("get provider failed: %w", err)
	}

	// 创建已登录客户端（优先使用缓存 token）并测试连接
	client, err := s.newAuthedClient(ctx, provider)
	if err != nil {
		errMsg := err.Error()
		_ = s.repo.UpdateSyncStatus(ctx, id, "failed", &errMsg)
		return infraerrors.ServiceUnavailable("PROVIDER_CONNECTION_FAILED", fmt.Sprintf("login failed: %s", err.Error()))
	}
	detector := sub2api.NewPathDetector(client)

	if err := detector.TestConnection(ctx); err != nil {
		// 更新同步状态为失败
		errMsg := err.Error()
		_ = s.repo.UpdateSyncStatus(ctx, id, "failed", &errMsg)
		return infraerrors.ServiceUnavailable(
			"PROVIDER_CONNECTION_FAILED",
			fmt.Sprintf("connection test failed: %s", err.Error()),
		)
	}

	// 更新同步状态为成功
	_ = s.repo.UpdateSyncStatus(ctx, id, "success", nil)

	return nil
}

// PathDetectionResult 路径探测结果
type PathDetectionResult struct {
	KeysPath   string `json:"keys_path"`
	GroupsPath string `json:"groups_path"`
}

// AccountProviderLink Account 与 Provider 关联信息
type AccountProviderLink struct {
	AccountID             int64    `json:"account_id"`
	AccountName           string   `json:"account_name"`
	AccountPlatform       string   `json:"account_platform"`
	ProviderID            int64    `json:"provider_id"`
	ProviderAPIKeyID      *int64   `json:"provider_api_key_id,omitempty"`
	RemoteGroupName       *string  `json:"remote_group_name,omitempty"`
	RemoteGroupMultiplier *float64 `json:"remote_group_multiplier,omitempty"`
	RemoteGroupSyncedAt   *string  `json:"remote_group_synced_at,omitempty"`
}

// LinkAccount 将 Account 关联到 Provider，并自动查找远程 APIKey ID
func (s *Sub2APIProviderService) LinkAccount(
	ctx context.Context,
	providerID, accountID int64,
) (*AccountProviderLink, error) {
	// 获取 Provider
	provider, err := s.repo.GetByID(ctx, providerID)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrProviderNotFound
		}
		return nil, fmt.Errorf("get provider failed: %w", err)
	}

	// 获取 Account（需要 AccountRepository）
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return nil, infraerrors.NotFound("ACCOUNT_NOT_FOUND", "account not found")
	}

	// 获取 Account 的 api_key（从 credentials 中读取）
	apiKey, _ := account.Credentials["api_key"].(string)
	if apiKey == "" {
		return nil, infraerrors.BadRequest("ACCOUNT_NO_API_KEY", "account has no api_key in credentials")
	}

	// 登录远程 Sub2API 并查找 APIKey ID（优先使用缓存 token）
	client, err := s.newAuthedClient(ctx, provider)
	if err != nil {
		return nil, infraerrors.ServiceUnavailable(
			"PROVIDER_CONNECTION_FAILED",
			fmt.Sprintf("login failed: %s", err.Error()),
		)
	}

	// 确定 Keys 路径
	keysPath := "/api/v1/keys"
	if provider.APIPathKeys != nil && *provider.APIPathKeys != "" {
		keysPath = *provider.APIPathKeys
	}

	// 获取远程 APIKeys 列表，查找匹配的
	remoteKeys, err := client.GetAPIKeys(ctx, keysPath)
	if err != nil {
		return nil, infraerrors.ServiceUnavailable(
			"PROVIDER_FETCH_KEYS_FAILED",
			fmt.Sprintf("fetch api keys failed: %s", err.Error()),
		)
	}

	var remoteKeyID *int64
	for _, k := range remoteKeys {
		if k.Key == apiKey {
			id := k.ID
			remoteKeyID = &id
			break
		}
	}

	if remoteKeyID == nil {
		return nil, infraerrors.NotFound(
			"REMOTE_API_KEY_NOT_FOUND",
			fmt.Sprintf("api key not found on remote provider, please ensure the key exists: %s...", apiKey[:min(16, len(apiKey))]),
		)
	}

	// 更新 Account 的 provider 关联
	if err := s.accountRepo.UpdateProviderLink(ctx, accountID, providerID, *remoteKeyID); err != nil {
		return nil, fmt.Errorf("update account provider link failed: %w", err)
	}

	return &AccountProviderLink{
		AccountID:        accountID,
		AccountName:      account.Name,
		AccountPlatform:  account.Platform,
		ProviderID:       providerID,
		ProviderAPIKeyID: remoteKeyID,
	}, nil
}

// UnlinkAccount 解除 Account 与 Provider 的关联
func (s *Sub2APIProviderService) UnlinkAccount(
	ctx context.Context,
	providerID, accountID int64,
) error {
	// 验证 Provider 存在
	_, err := s.repo.GetByID(ctx, providerID)
	if err != nil {
		if ent.IsNotFound(err) {
			return ErrProviderNotFound
		}
		return fmt.Errorf("get provider failed: %w", err)
	}

	// 清除 Account 的 provider 关联
	if err := s.accountRepo.ClearProviderLink(ctx, accountID, providerID); err != nil {
		return fmt.Errorf("clear account provider link failed: %w", err)
	}

	return nil
}

// LinkedAccountInfo 关联账号的详细信息（含远端分组缓存）
type LinkedAccountInfo struct {
	ID                     int64    `json:"id"`
	Name                   string   `json:"name"`
	Platform               string   `json:"platform"`
	Status                 string   `json:"status"`
	ProviderID             int64    `json:"provider_id"`
	ProviderAPIKeyID       *int64   `json:"provider_api_key_id,omitempty"`
	RemoteGroupName        *string  `json:"remote_group_name,omitempty"`
	RemoteGroupMultiplier  *float64 `json:"remote_group_multiplier,omitempty"`
	RemoteGroupSyncedAt    *string  `json:"remote_group_synced_at,omitempty"`
	Sub2APIOptimizeEnabled bool     `json:"sub2api_optimize_enabled"`
	Sub2APIMinMultiplier   *float64 `json:"sub2api_min_multiplier,omitempty"`
	Sub2APIMaxMultiplier   *float64 `json:"sub2api_max_multiplier,omitempty"`
	Sub2APITestModel       *string  `json:"sub2api_test_model,omitempty"`
}

// GetLinkedAccounts 返回关联到指定 Provider 的所有账号信息。
// 当 sync=true 时，会登录上游实时拉取每个账号当前所在分组并刷新本地缓存；
// 若上游同步失败则降级返回缓存数据（不影响面板展示）。
func (s *Sub2APIProviderService) GetLinkedAccounts(ctx context.Context, providerID int64, sync bool) ([]*LinkedAccountInfo, error) {
	provider, err := s.repo.GetByID(ctx, providerID)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrProviderNotFound
		}
		return nil, fmt.Errorf("get provider failed: %w", err)
	}

	accounts, err := s.accountRepo.ListByProviderID(ctx, providerID)
	if err != nil {
		return nil, fmt.Errorf("list linked accounts failed: %w", err)
	}

	// 实时同步：登录上游拉一次 keys，按 keyID 匹配当前分组，回写缓存
	if sync && len(accounts) > 0 {
		s.syncRemoteGroups(ctx, provider, accounts)
	}

	result := make([]*LinkedAccountInfo, 0, len(accounts))
	for i := range accounts {
		acc := &accounts[i]
		info := &LinkedAccountInfo{
			ID:                     acc.ID,
			Name:                   acc.Name,
			Platform:               acc.Platform,
			Status:                 acc.Status,
			ProviderID:             providerID,
			ProviderAPIKeyID:       acc.ProviderAPIKeyID,
			RemoteGroupName:        acc.RemoteGroupName,
			RemoteGroupMultiplier:  acc.RemoteGroupMultiplier,
			Sub2APIOptimizeEnabled: acc.Sub2APIOptimizeEnabled,
			Sub2APIMinMultiplier:   acc.Sub2APIMinMultiplier,
			Sub2APIMaxMultiplier:   acc.Sub2APIMaxMultiplier,
			Sub2APITestModel:       acc.Sub2APITestModel,
		}
		if acc.RemoteGroupSyncedAt != nil {
			t := acc.RemoteGroupSyncedAt.Format("2006-01-02T15:04:05Z07:00")
			info.RemoteGroupSyncedAt = &t
		}
		result = append(result, info)
	}
	return result, nil
}

// syncRemoteGroups 登录上游拉取一次 API Keys，将每个账号的当前分组回写到 accounts 切片与数据库缓存。
// 该函数尽力而为：任何上游错误都被吞掉，保证面板仍能展示已有缓存。
func (s *Sub2APIProviderService) syncRemoteGroups(ctx context.Context, provider *ent.Sub2APIProvider, accounts []Account) {
	// 优先使用缓存 token，避免每次打开面板都重新登录
	client, err := s.newAuthedClient(ctx, provider)
	if err != nil {
		return
	}

	keysPath := "/api/v1/keys"
	if provider.APIPathKeys != nil && *provider.APIPathKeys != "" {
		keysPath = *provider.APIPathKeys
	}

	remoteKeys, err := client.GetAPIKeys(ctx, keysPath)
	if err != nil {
		return
	}

	// keyID -> 当前分组信息
	type groupInfo struct {
		name       string
		multiplier float64
	}
	byKeyID := make(map[int64]groupInfo, len(remoteKeys))
	for _, k := range remoteKeys {
		if k.Group != nil {
			byKeyID[k.ID] = groupInfo{name: k.Group.Name, multiplier: k.Group.RateMultiplier}
		}
	}

	now := time.Now()
	for i := range accounts {
		acc := &accounts[i]
		if acc.ProviderAPIKeyID == nil {
			continue
		}
		gi, ok := byKeyID[*acc.ProviderAPIKeyID]
		if !ok {
			continue
		}
		// 回写内存中的切片，供本次响应使用
		name := gi.name
		mult := gi.multiplier
		acc.RemoteGroupName = &name
		acc.RemoteGroupMultiplier = &mult
		acc.RemoteGroupSyncedAt = &now
		// 持久化缓存（非致命）
		_ = s.accountRepo.UpdateRemoteGroupInfo(ctx, acc.ID, gi.name, gi.multiplier)
	}
}

package service

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/sub2api"
)

var (
	ErrProviderNotFound       = infraerrors.NotFound("PROVIDER_NOT_FOUND", "provider not found")
	ErrProviderExists         = infraerrors.Conflict("PROVIDER_EXISTS", "provider with same base_url and email already exists")
	ErrInvalidProviderType    = infraerrors.BadRequest("INVALID_PROVIDER_TYPE", "unsupported provider type")
	ErrInvalidProviderAuth    = infraerrors.BadRequest("INVALID_PROVIDER_AUTH", "invalid provider authentication configuration")
	ErrInvalidProviderBaseURL = infraerrors.BadRequest("INVALID_PROVIDER_BASE_URL", "enter the Sub2API site root, not a page or API path")
	ErrInvalidProviderProxy   = infraerrors.BadRequest("INVALID_PROVIDER_PROXY", "provider proxy must be active and not expired")
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
	PersistTokenPair(ctx context.Context, id int64, accessTokenEncrypted, refreshTokenEncrypted string, expiresAt, refreshedAt time.Time) error
	UpdateAuthError(ctx context.Context, id int64, errorMsg *string) error
}

// Sub2APIProviderRemoteOverviewCache stores only the latest Provider asset
// snapshot and the latest collection attempt. It is deliberately separate
// from probe health persistence because asset collection failures must never
// change Provider availability.
type Sub2APIProviderRemoteOverviewCache interface {
	GetMany(ctx context.Context, providerIDs []int64) (map[int64]*Sub2APIProviderRemoteOverview, error)
	StoreSuccess(ctx context.Context, overview *Sub2APIProviderRemoteOverview) error
	StoreFailure(ctx context.Context, providerID int64, source string, attemptedAt time.Time, errorMessage string) error
}

type Sub2APIProviderProxyRepository interface {
	GetByID(ctx context.Context, id int64) (*Proxy, error)
}

// Repository Input/Filter Types
type CreateSub2APIProviderInput struct {
	Name         string
	BaseURL      string
	ProviderType string
	Email        string
	Password     string
	AuthMode     string
	AccessToken  *string
	RefreshToken *string
	TokenExpires *time.Time
	Notes        *string
	ProxyID      *int64
}

// OptionalProviderProxyID preserves update semantics: Set=false keeps the
// current proxy, Set=true with Value=nil clears it, and a non-nil Value sets it.
type OptionalProviderProxyID struct {
	Set   bool
	Value *int64
}

type UpdateSub2APIProviderInput struct {
	Name                  *string
	BaseURL               *string
	Email                 *string
	Password              *string
	AuthMode              *string
	AccessTokenEncrypted  *string
	RefreshTokenEncrypted *string
	AccessTokenExpiresAt  *time.Time
	ClearTokenPair        bool
	TokenPairUpdated      bool
	Status                *string
	Notes                 *string
	ProxyID               OptionalProviderProxyID
}

type Sub2APIProviderFilters struct {
	Status string
	Search string
}

// Sub2APIProviderService 处理 Provider 业务逻辑
type Sub2APIProviderService struct {
	repo                         Sub2APIProviderRepository
	accountRepo                  AccountRepository
	proxyRepo                    Sub2APIProviderProxyRepository
	tokenCache                   *sub2api.TokenCache // 复用各上游的登录 token，避免每次重新登录
	encryptor                    SecretEncryptor
	remoteOverviewCache          Sub2APIProviderRemoteOverviewCache
	tokenEncryptionKeyConfigured bool
}

// NewSub2APIProviderService 创建 Service 实例
func NewSub2APIProviderService(repo Sub2APIProviderRepository, accountRepo AccountRepository, proxyRepo ProxyRepository, tokenCache *sub2api.TokenCache, encryptor SecretEncryptor, remoteOverviewCache Sub2APIProviderRemoteOverviewCache, cfg *config.Config) *Sub2APIProviderService {
	return &Sub2APIProviderService{
		repo: repo, accountRepo: accountRepo, proxyRepo: proxyRepo, tokenCache: tokenCache, encryptor: encryptor, remoteOverviewCache: remoteOverviewCache,
		tokenEncryptionKeyConfigured: cfg != nil && cfg.Totp.EncryptionKeyConfigured,
	}
}

// newAuthedClient 为指定 Provider 创建已登录的客户端。
// 优先使用缓存 token，缓存未命中时登录并写入缓存。
func (s *Sub2APIProviderService) newAuthedClient(ctx context.Context, provider *ent.Sub2APIProvider) (*sub2api.Client, error) {
	return newAuthedSub2APIProviderClient(ctx, provider, s.repo, s.tokenCache, s.encryptor)
}

func newAuthedSub2APIProviderClient(
	ctx context.Context,
	provider *ent.Sub2APIProvider,
	repo Sub2APIProviderRepository,
	tokenCache *sub2api.TokenCache,
	encryptor SecretEncryptor,
) (*sub2api.Client, error) {
	if provider == nil {
		return nil, fmt.Errorf("provider is nil")
	}
	client := sub2api.NewClient(provider.BaseURL, provider.Email, provider.PasswordEncrypted)
	if err := configureSub2APIProviderProxy(client, provider); err != nil {
		return nil, err
	}
	authMode := strings.TrimSpace(provider.AuthMode)
	if authMode == "" {
		authMode = domain.Sub2APIProviderAuthModePassword
	}
	if authMode == domain.Sub2APIProviderAuthModeTokenPair {
		if encryptor == nil {
			return nil, fmt.Errorf("provider %d token encryption is unavailable", provider.ID)
		}
		pair, cached := sub2api.TokenPair{}, false
		if tokenCache != nil {
			pair, cached = tokenCache.GetTokenPair(provider.ID)
		}
		if !cached {
			accessToken, err := decryptProviderSecret(encryptor, provider.AccessTokenEncrypted, "access token")
			if err != nil {
				return nil, fmt.Errorf("load provider %d access token: %w", provider.ID, err)
			}
			refreshToken, err := decryptProviderSecret(encryptor, provider.RefreshTokenEncrypted, "refresh token")
			if err != nil {
				return nil, fmt.Errorf("load provider %d refresh token: %w", provider.ID, err)
			}
			pair = sub2api.NewTokenPair(accessToken, refreshToken, provider.AccessTokenExpiresAt)
			if tokenCache != nil {
				tokenCache.SeedTokenPair(provider.ID, pair)
			}
		}
		if tokenCache == nil {
			client.Token = pair.AccessToken
			client.RefreshToken = pair.RefreshToken
			client.TokenExpiresIn = time.Until(pair.ExpiresAt)
		}
		client.ConfigureImportedTokenAuth(func(updateCtx context.Context, updated sub2api.TokenPair) error {
			accessEncrypted, err := encryptor.Encrypt(updated.AccessToken)
			if err != nil {
				return fmt.Errorf("encrypt access token: %w", err)
			}
			refreshEncrypted, err := encryptor.Encrypt(updated.RefreshToken)
			if err != nil {
				return fmt.Errorf("encrypt refresh token: %w", err)
			}
			return repo.PersistTokenPair(updateCtx, provider.ID, accessEncrypted, refreshEncrypted, updated.ExpiresAt, time.Now())
		})
	}
	if err := client.EnsureLoggedIn(ctx, provider.ID, tokenCache); err != nil {
		errMsg := trimProviderAuthError(err)
		_ = repo.UpdateAuthError(ctx, provider.ID, &errMsg)
		return nil, fmt.Errorf("authenticate provider %d failed: %w", provider.ID, err)
	}
	if provider.LastAuthError != nil {
		_ = repo.UpdateAuthError(ctx, provider.ID, nil)
	}
	return client, nil
}

func configureSub2APIProviderProxy(client *sub2api.Client, provider *ent.Sub2APIProvider) error {
	if provider.ProxyID == nil {
		return nil
	}
	proxyEntity := provider.Edges.Proxy
	if proxyEntity == nil {
		return fmt.Errorf("provider %d proxy %d is unavailable", provider.ID, *provider.ProxyID)
	}
	if proxyEntity.Status != StatusActive || (proxyEntity.ExpiresAt != nil && !proxyEntity.ExpiresAt.After(time.Now())) {
		return fmt.Errorf("provider %d proxy %d is inactive or expired", provider.ID, *provider.ProxyID)
	}
	proxyConfig := &Proxy{
		ID:       proxyEntity.ID,
		Protocol: proxyEntity.Protocol,
		Host:     proxyEntity.Host,
		Port:     proxyEntity.Port,
		Status:   proxyEntity.Status,
	}
	if proxyEntity.Username != nil {
		proxyConfig.Username = *proxyEntity.Username
	}
	if proxyEntity.Password != nil {
		proxyConfig.Password = *proxyEntity.Password
	}
	if err := client.ConfigureProxy(proxyConfig.URL()); err != nil {
		return fmt.Errorf("configure provider %d proxy: %w", provider.ID, err)
	}
	return nil
}

func decryptProviderSecret(encryptor SecretEncryptor, ciphertext *string, name string) (string, error) {
	if ciphertext == nil || strings.TrimSpace(*ciphertext) == "" {
		return "", fmt.Errorf("%s is not configured", name)
	}
	plaintext, err := encryptor.Decrypt(*ciphertext)
	if err != nil {
		return "", fmt.Errorf("decrypt %s: %w", name, err)
	}
	if strings.TrimSpace(plaintext) == "" {
		return "", fmt.Errorf("%s is empty", name)
	}
	return plaintext, nil
}

func trimProviderAuthError(err error) string {
	if err == nil {
		return ""
	}
	message := strings.TrimSpace(err.Error())
	if len(message) > 512 {
		return message[:512]
	}
	return message
}

// CreateProvider 创建 Provider
func (s *Sub2APIProviderService) CreateProvider(ctx context.Context, input *CreateProviderInput) (*Provider, error) {
	baseURL, err := normalizeSub2APIProviderBaseURL(input.BaseURL)
	if err != nil {
		return nil, ErrInvalidProviderBaseURL.WithCause(err)
	}
	// 上游类型：未显式指定时用默认（sub2api），并校验受支持
	providerType := input.ProviderType
	if providerType == "" {
		providerType = domain.ProviderTypeDefault
	}
	if !domain.IsValidProviderType(providerType) {
		return nil, ErrInvalidProviderType
	}
	auth, err := s.prepareCreateProviderAuth(input)
	if err != nil {
		return nil, err
	}
	if err := s.validateProviderProxy(ctx, input.ProxyID); err != nil {
		return nil, err
	}

	// 调用 Repository
	provider, err := s.repo.Create(ctx, &CreateSub2APIProviderInput{
		Name:         input.Name,
		BaseURL:      baseURL,
		ProviderType: providerType,
		Email:        input.Email,
		Password:     input.Password, // 兼容旧 password 模式
		AuthMode:     auth.mode,
		AccessToken:  auth.accessEncrypted,
		RefreshToken: auth.refreshEncrypted,
		TokenExpires: auth.expiresAt,
		Notes:        input.Notes,
		ProxyID:      input.ProxyID,
	})

	if err != nil {
		if ent.IsConstraintError(err) {
			return nil, ErrProviderExists
		}
		return nil, fmt.Errorf("create provider failed: %w", err)
	}

	return providerFromEnt(provider), nil
}

type preparedProviderAuth struct {
	mode             string
	accessEncrypted  *string
	refreshEncrypted *string
	expiresAt        *time.Time
}

func (s *Sub2APIProviderService) prepareCreateProviderAuth(input *CreateProviderInput) (*preparedProviderAuth, error) {
	mode := strings.TrimSpace(input.AuthMode)
	if mode == "" {
		mode = domain.Sub2APIProviderAuthModePassword
	}
	if !domain.IsValidSub2APIProviderAuthMode(mode) {
		return nil, ErrInvalidProviderAuth
	}
	prepared := &preparedProviderAuth{mode: mode}
	if mode == domain.Sub2APIProviderAuthModePassword {
		if strings.TrimSpace(input.Password) == "" {
			return nil, infraerrors.BadRequest("PROVIDER_PASSWORD_REQUIRED", "password is required for password authentication")
		}
		return prepared, nil
	}
	if input.AccessToken == nil || strings.TrimSpace(*input.AccessToken) == "" || input.RefreshToken == nil || strings.TrimSpace(*input.RefreshToken) == "" {
		return nil, infraerrors.BadRequest("PROVIDER_TOKEN_PAIR_REQUIRED", "access_token and refresh_token are required for token authentication")
	}
	if s.encryptor == nil {
		return nil, fmt.Errorf("provider token encryption is unavailable")
	}
	if !s.tokenEncryptionKeyConfigured {
		return nil, infraerrors.ServiceUnavailable("PROVIDER_TOKEN_ENCRYPTION_KEY_REQUIRED", "configure a fixed totp.encryption_key before importing provider tokens")
	}
	accessEncrypted, err := s.encryptor.Encrypt(strings.TrimSpace(*input.AccessToken))
	if err != nil {
		return nil, fmt.Errorf("encrypt provider access token: %w", err)
	}
	refreshEncrypted, err := s.encryptor.Encrypt(strings.TrimSpace(*input.RefreshToken))
	if err != nil {
		return nil, fmt.Errorf("encrypt provider refresh token: %w", err)
	}
	pair := sub2api.NewTokenPair(strings.TrimSpace(*input.AccessToken), strings.TrimSpace(*input.RefreshToken), nil)
	prepared.accessEncrypted = &accessEncrypted
	prepared.refreshEncrypted = &refreshEncrypted
	prepared.expiresAt = &pair.ExpiresAt
	return prepared, nil
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
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrProviderNotFound
		}
		return nil, err
	}

	authUpdate, err := s.prepareUpdateProviderAuth(existing, input)
	if err != nil {
		return nil, err
	}
	if input.ProxyID.Set {
		if err := s.validateProviderProxy(ctx, input.ProxyID.Value); err != nil {
			return nil, err
		}
	}
	var proxyUpdater interface {
		UpdateProviderAccountsProxy(context.Context, int64, *int64) error
	}
	if input.ProxyID.Set {
		var ok bool
		proxyUpdater, ok = s.accountRepo.(interface {
			UpdateProviderAccountsProxy(context.Context, int64, *int64) error
		})
		if !ok {
			return nil, fmt.Errorf("account repository does not support provider proxy synchronization")
		}
	}
	var baseURL *string
	if input.BaseURL != nil {
		normalized, normalizeErr := normalizeSub2APIProviderBaseURL(*input.BaseURL)
		if normalizeErr != nil {
			return nil, ErrInvalidProviderBaseURL.WithCause(normalizeErr)
		}
		baseURL = &normalized
	}
	// 更新
	provider, err := s.repo.Update(ctx, id, &UpdateSub2APIProviderInput{
		Name:                  input.Name,
		BaseURL:               baseURL,
		Email:                 input.Email,
		Password:              input.Password,
		AuthMode:              authUpdate.AuthMode,
		AccessTokenEncrypted:  authUpdate.AccessTokenEncrypted,
		RefreshTokenEncrypted: authUpdate.RefreshTokenEncrypted,
		AccessTokenExpiresAt:  authUpdate.AccessTokenExpiresAt,
		ClearTokenPair:        authUpdate.ClearTokenPair,
		TokenPairUpdated:      authUpdate.TokenPairUpdated,
		Status:                input.Status,
		Notes:                 input.Notes,
		ProxyID:               input.ProxyID,
	})

	if err != nil {
		if ent.IsConstraintError(err) {
			return nil, ErrProviderExists
		}
		return nil, fmt.Errorf("update provider failed: %w", err)
	}
	if input.ProxyID.Set {
		if err := proxyUpdater.UpdateProviderAccountsProxy(ctx, id, input.ProxyID.Value); err != nil {
			rollbackInput := &UpdateSub2APIProviderInput{
				ProxyID: OptionalProviderProxyID{Set: true, Value: existing.ProxyID},
			}
			if _, rollbackErr := s.repo.Update(ctx, id, rollbackInput); rollbackErr != nil {
				return nil, fmt.Errorf("sync provider account proxies failed: %w (provider proxy rollback failed: %v)", err, rollbackErr)
			}
			return nil, fmt.Errorf("sync provider account proxies failed: %w", err)
		}
	}
	// Provider 地址或凭据可能已经变化；统一清除旧 Token 对，避免继续使用
	// 旧账号的长期 Refresh Token。下一次请求会按最新配置重新认证。
	if s.tokenCache != nil {
		s.tokenCache.Evict(id)
	}

	return providerFromEnt(provider), nil
}

func (s *Sub2APIProviderService) validateProviderProxy(ctx context.Context, proxyID *int64) error {
	if proxyID == nil {
		return nil
	}
	if *proxyID <= 0 || s.proxyRepo == nil {
		return ErrInvalidProviderProxy
	}
	proxyConfig, err := s.proxyRepo.GetByID(ctx, *proxyID)
	if err != nil {
		return ErrInvalidProviderProxy.WithCause(err)
	}
	if !proxyConfig.IsActive() || proxyConfig.IsExpired(time.Now()) {
		return ErrInvalidProviderProxy
	}
	return nil
}

func (s *Sub2APIProviderService) prepareUpdateProviderAuth(existing *ent.Sub2APIProvider, input *UpdateProviderInput) (*UpdateSub2APIProviderInput, error) {
	update := &UpdateSub2APIProviderInput{}
	mode := strings.TrimSpace(existing.AuthMode)
	if mode == "" {
		mode = domain.Sub2APIProviderAuthModePassword
	}
	if input.AuthMode != nil {
		mode = strings.TrimSpace(*input.AuthMode)
		if !domain.IsValidSub2APIProviderAuthMode(mode) {
			return nil, ErrInvalidProviderAuth
		}
		update.AuthMode = &mode
	}
	if mode == domain.Sub2APIProviderAuthModePassword {
		password := existing.PasswordEncrypted
		if input.Password != nil {
			password = *input.Password
		}
		if strings.TrimSpace(password) == "" {
			return nil, infraerrors.BadRequest("PROVIDER_PASSWORD_REQUIRED", "password is required for password authentication")
		}
		if existing.AuthMode == domain.Sub2APIProviderAuthModeTokenPair || input.AuthMode != nil {
			update.ClearTokenPair = true
		}
		return update, nil
	}
	if s.encryptor == nil {
		return nil, fmt.Errorf("provider token encryption is unavailable")
	}
	if (input.AccessToken != nil && strings.TrimSpace(*input.AccessToken) != "") ||
		(input.RefreshToken != nil && strings.TrimSpace(*input.RefreshToken) != "") ||
		existing.AuthMode != domain.Sub2APIProviderAuthModeTokenPair {
		if !s.tokenEncryptionKeyConfigured {
			return nil, infraerrors.ServiceUnavailable("PROVIDER_TOKEN_ENCRYPTION_KEY_REQUIRED", "configure a fixed totp.encryption_key before importing provider tokens")
		}
	}
	hasAccess := existing.AccessTokenEncrypted != nil && strings.TrimSpace(*existing.AccessTokenEncrypted) != ""
	hasRefresh := existing.RefreshTokenEncrypted != nil && strings.TrimSpace(*existing.RefreshTokenEncrypted) != ""
	if input.AccessToken != nil && strings.TrimSpace(*input.AccessToken) != "" {
		access := strings.TrimSpace(*input.AccessToken)
		encrypted, err := s.encryptor.Encrypt(access)
		if err != nil {
			return nil, fmt.Errorf("encrypt provider access token: %w", err)
		}
		update.AccessTokenEncrypted = &encrypted
		pair := sub2api.NewTokenPair(access, "", nil)
		update.AccessTokenExpiresAt = &pair.ExpiresAt
		update.TokenPairUpdated = true
		hasAccess = true
	}
	if input.RefreshToken != nil && strings.TrimSpace(*input.RefreshToken) != "" {
		encrypted, err := s.encryptor.Encrypt(strings.TrimSpace(*input.RefreshToken))
		if err != nil {
			return nil, fmt.Errorf("encrypt provider refresh token: %w", err)
		}
		update.RefreshTokenEncrypted = &encrypted
		update.TokenPairUpdated = true
		hasRefresh = true
	}
	if !hasAccess || !hasRefresh {
		return nil, infraerrors.BadRequest("PROVIDER_TOKEN_PAIR_REQUIRED", "access_token and refresh_token are required for token authentication")
	}
	return update, nil
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
	if s.tokenCache != nil {
		s.tokenCache.Evict(id)
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
		ID:                   e.ID,
		Name:                 e.Name,
		BaseURL:              e.BaseURL,
		ProviderType:         e.ProviderType,
		Status:               e.Status,
		Notes:                e.Notes,
		ProxyID:              e.ProxyID,
		ProxyName:            providerProxyName(e),
		Email:                e.Email,
		Password:             e.PasswordEncrypted, // 阶段1明文，阶段7会解密
		AuthMode:             e.AuthMode,
		HasAccessToken:       e.AccessTokenEncrypted != nil && *e.AccessTokenEncrypted != "",
		HasRefreshToken:      e.RefreshTokenEncrypted != nil && *e.RefreshTokenEncrypted != "",
		AccessTokenExpiresAt: formatProviderTime(e.AccessTokenExpiresAt),
		LastTokenRefreshAt:   formatProviderTime(e.LastTokenRefreshAt),
		LastAuthError:        e.LastAuthError,
		APIPathKeys:          e.APIPathKeys,
		APIPathGroups:        e.APIPathGroups,
		LastSyncAt:           lastSyncAt,
		LastSyncStatus:       e.LastSyncStatus,
		LastSyncError:        e.LastSyncError,
		CreatedAt:            e.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:            e.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		AccountsCount:        accountsCount,
	}
}

func providerProxyName(provider *ent.Sub2APIProvider) *string {
	if provider == nil || provider.Edges.Proxy == nil {
		return nil
	}
	name := provider.Edges.Proxy.Name
	return &name
}

func formatProviderTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.Format(time.RFC3339)
	return &formatted
}

// Service Types

// Provider Service 层的 Provider 模型
type Provider struct {
	ID                   int64   `json:"id"`
	Name                 string  `json:"name"`
	BaseURL              string  `json:"base_url"`
	ProviderType         string  `json:"provider_type"`
	Status               string  `json:"status"`
	Notes                *string `json:"notes,omitempty"`
	ProxyID              *int64  `json:"proxy_id"`
	ProxyName            *string `json:"proxy_name,omitempty"`
	Email                string  `json:"email"`
	Password             string  `json:"-"` // 不序列化到 JSON
	AuthMode             string  `json:"auth_mode"`
	HasAccessToken       bool    `json:"has_access_token"`
	HasRefreshToken      bool    `json:"has_refresh_token"`
	AccessTokenExpiresAt *string `json:"access_token_expires_at,omitempty"`
	LastTokenRefreshAt   *string `json:"last_token_refresh_at,omitempty"`
	LastAuthError        *string `json:"last_auth_error,omitempty"`
	APIPathKeys          *string `json:"api_path_keys,omitempty"`
	APIPathGroups        *string `json:"api_path_groups,omitempty"`
	LastSyncAt           *string `json:"last_sync_at,omitempty"`
	LastSyncStatus       *string `json:"last_sync_status,omitempty"`
	LastSyncError        *string `json:"last_sync_error,omitempty"`
	CreatedAt            string  `json:"created_at"`
	UpdatedAt            string  `json:"updated_at"`
	AccountsCount        int     `json:"accounts_count"`
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
	AuthMode     string  `json:"auth_mode,omitempty"`
	AccessToken  *string `json:"access_token,omitempty"`
	RefreshToken *string `json:"refresh_token,omitempty"`
	Notes        *string `json:"notes,omitempty"`
	ProxyID      *int64  `json:"proxy_id,omitempty"`
}

// UpdateProviderInput 更新 Provider 的输入
type UpdateProviderInput struct {
	Name         *string                 `json:"name,omitempty"`
	BaseURL      *string                 `json:"base_url,omitempty"`
	Email        *string                 `json:"email,omitempty"`
	Password     *string                 `json:"password,omitempty"`
	AuthMode     *string                 `json:"auth_mode,omitempty"`
	AccessToken  *string                 `json:"access_token,omitempty"`
	RefreshToken *string                 `json:"refresh_token,omitempty"`
	Status       *string                 `json:"status,omitempty"`
	Notes        *string                 `json:"notes,omitempty"`
	ProxyID      OptionalProviderProxyID `json:"-"`
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

// Sub2APIProviderRemoteGroupRate is one remote group visible to the Provider
// login account. DefaultMultiplier comes from the group definition while
// EffectiveMultiplier includes a user-specific /groups/rates override.
type Sub2APIProviderRemoteGroupRate struct {
	ID                  int64   `json:"id"`
	Name                string  `json:"name"`
	Description         string  `json:"description,omitempty"`
	Platform            string  `json:"platform,omitempty"`
	Status              string  `json:"status,omitempty"`
	DefaultMultiplier   float64 `json:"default_multiplier"`
	EffectiveMultiplier float64 `json:"effective_multiplier"`
	HasCustomRate       bool    `json:"has_custom_rate"`
}

// Sub2APIProviderRemoteOverview is a live, non-persisted snapshot of the
// Provider account's wallet and visible group rates. It deliberately remains
// separate from account route probes: this is control-plane commercial data,
// not evidence that an individual model route can serve traffic.
type Sub2APIProviderRemoteOverview struct {
	ProviderID             int64                            `json:"provider_id"`
	Available              bool                             `json:"available"`
	Balance                float64                          `json:"balance"`
	Groups                 []Sub2APIProviderRemoteGroupRate `json:"groups"`
	RateOverridesAvailable bool                             `json:"rate_overrides_available"`
	SampledAt              time.Time                        `json:"sampled_at"`
	Source                 string                           `json:"source"`
	LastAttemptedAt        time.Time                        `json:"last_attempted_at"`
	LastAttemptSource      string                           `json:"last_attempt_source"`
	LastError              *string                          `json:"last_error,omitempty"`
	LastErrorAt            *time.Time                       `json:"last_error_at,omitempty"`
}

const (
	Sub2APIProviderRemoteOverviewSourceManual       = "manual"
	Sub2APIProviderRemoteOverviewSourceControlProbe = "control_probe"
)

// GetRemoteOverview reads balance and visible group rates directly from the
// upstream Sub2API instance for an administrator-triggered refresh. Scheduled
// collection is performed by the control probe with the same authenticated
// client; the 15-second UI polling reads only the Redis snapshot.
func (s *Sub2APIProviderService) GetRemoteOverview(ctx context.Context, id int64) (*Sub2APIProviderRemoteOverview, error) {
	provider, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrProviderNotFound
		}
		return nil, fmt.Errorf("get provider failed: %w", err)
	}
	attemptedAt := time.Now().UTC()

	client, err := s.newAuthedClient(ctx, provider)
	if err != nil {
		storeRemoteOverviewFailure(ctx, s.remoteOverviewCache, id, Sub2APIProviderRemoteOverviewSourceManual, attemptedAt, err)
		return nil, infraerrors.ServiceUnavailable(
			"PROVIDER_REMOTE_OVERVIEW_FAILED",
			fmt.Sprintf("authenticate remote provider failed: %s", err.Error()),
		)
	}

	groupsPath := "/api/v1/groups/available"
	if provider.APIPathGroups != nil && strings.TrimSpace(*provider.APIPathGroups) != "" {
		groupsPath = strings.TrimSpace(*provider.APIPathGroups)
	}
	groups, err := client.GetGroups(ctx, groupsPath)
	if err != nil {
		storeRemoteOverviewFailure(ctx, s.remoteOverviewCache, id, Sub2APIProviderRemoteOverviewSourceManual, attemptedAt, err)
		return nil, infraerrors.ServiceUnavailable(
			"PROVIDER_REMOTE_OVERVIEW_FAILED",
			fmt.Sprintf("read remote groups failed: %s", err.Error()),
		)
	}
	overview, err := collectSub2APIProviderRemoteOverview(ctx, id, client, groups, Sub2APIProviderRemoteOverviewSourceManual, attemptedAt)
	if err != nil {
		storeRemoteOverviewFailure(ctx, s.remoteOverviewCache, id, Sub2APIProviderRemoteOverviewSourceManual, attemptedAt, err)
		return nil, infraerrors.ServiceUnavailable(
			"PROVIDER_REMOTE_OVERVIEW_FAILED",
			fmt.Sprintf("read remote balance failed: %s", err.Error()),
		)
	}
	storeRemoteOverviewSuccess(ctx, s.remoteOverviewCache, overview)
	return overview, nil
}

func collectSub2APIProviderRemoteOverview(ctx context.Context, providerID int64, client *sub2api.Client, groups []sub2api.Group, source string, attemptedAt time.Time) (*Sub2APIProviderRemoteOverview, error) {
	balance, err := client.GetCurrentUserBalance(ctx)
	if err != nil {
		return nil, err
	}
	rateOverrides, ratesErr := client.GetGroupRates(ctx)
	result := &Sub2APIProviderRemoteOverview{
		ProviderID:             providerID,
		Available:              true,
		Balance:                balance,
		Groups:                 make([]Sub2APIProviderRemoteGroupRate, 0, len(groups)),
		RateOverridesAvailable: ratesErr == nil,
		SampledAt:              attemptedAt,
		Source:                 source,
		LastAttemptedAt:        attemptedAt,
		LastAttemptSource:      source,
	}
	for _, group := range groups {
		effective := group.RateMultiplier
		override, overridden := rateOverrides[strconv.FormatInt(group.ID, 10)]
		if overridden {
			effective = override
		}
		result.Groups = append(result.Groups, Sub2APIProviderRemoteGroupRate{
			ID:                  group.ID,
			Name:                group.Name,
			Description:         group.Description,
			Platform:            group.Platform,
			Status:              group.Status,
			DefaultMultiplier:   group.RateMultiplier,
			EffectiveMultiplier: effective,
			HasCustomRate:       overridden,
		})
	}
	sort.SliceStable(result.Groups, func(i, j int) bool {
		if result.Groups[i].EffectiveMultiplier == result.Groups[j].EffectiveMultiplier {
			return result.Groups[i].Name < result.Groups[j].Name
		}
		return result.Groups[i].EffectiveMultiplier < result.Groups[j].EffectiveMultiplier
	})
	return result, nil
}

// GetCachedRemoteOverviews returns the latest Redis-backed asset state for the
// requested Providers. Missing entries are omitted rather than triggering live
// upstream traffic.
func (s *Sub2APIProviderService) GetCachedRemoteOverviews(ctx context.Context, providerIDs []int64) ([]*Sub2APIProviderRemoteOverview, error) {
	if len(providerIDs) == 0 || s.remoteOverviewCache == nil {
		return []*Sub2APIProviderRemoteOverview{}, nil
	}
	byProviderID, err := s.remoteOverviewCache.GetMany(ctx, providerIDs)
	if err != nil {
		return nil, fmt.Errorf("read cached provider assets: %w", err)
	}
	result := make([]*Sub2APIProviderRemoteOverview, 0, len(byProviderID))
	for _, providerID := range providerIDs {
		if overview := byProviderID[providerID]; overview != nil {
			result = append(result, overview)
		}
	}
	return result, nil
}

func storeRemoteOverviewSuccess(ctx context.Context, cache Sub2APIProviderRemoteOverviewCache, overview *Sub2APIProviderRemoteOverview) {
	if cache == nil || overview == nil {
		return
	}
	cacheCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	_ = cache.StoreSuccess(cacheCtx, overview)
}

func storeRemoteOverviewFailure(ctx context.Context, cache Sub2APIProviderRemoteOverviewCache, providerID int64, source string, attemptedAt time.Time, err error) {
	if cache == nil || err == nil {
		return
	}
	cacheCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	_ = cache.StoreFailure(cacheCtx, providerID, source, attemptedAt, trimProbeError(err))
}

// AccountProviderLink Account 与 Provider 关联信息
type AccountProviderLink struct {
	AccountID             int64    `json:"account_id"`
	AccountName           string   `json:"account_name"`
	AccountPlatform       string   `json:"account_platform"`
	ProviderID            int64    `json:"provider_id"`
	ProviderAPIKeyID      *int64   `json:"provider_api_key_id,omitempty"`
	RemoteGroupID         *int64   `json:"remote_group_id,omitempty"`
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
	RemoteGroupID          *int64   `json:"remote_group_id,omitempty"`
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
			RemoteGroupID:          acc.RemoteGroupID,
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
		id         int64
		name       string
		multiplier float64
	}
	byKeyID := make(map[int64]groupInfo, len(remoteKeys))
	for _, k := range remoteKeys {
		if k.Group != nil {
			byKeyID[k.ID] = groupInfo{id: k.Group.ID, name: k.Group.Name, multiplier: k.Group.RateMultiplier}
		} else if k.GroupID > 0 {
			byKeyID[k.ID] = groupInfo{id: k.GroupID}
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
		groupID := gi.id
		if groupID > 0 {
			acc.RemoteGroupID = &groupID
		}
		acc.RemoteGroupName = &name
		acc.RemoteGroupMultiplier = &mult
		acc.RemoteGroupSyncedAt = &now
		// 持久化缓存（非致命）
		if updater, ok := s.accountRepo.(interface {
			UpdateRemoteGroupBinding(context.Context, int64, int64, string, float64) error
		}); ok && gi.id > 0 {
			_ = updater.UpdateRemoteGroupBinding(ctx, acc.ID, gi.id, gi.name, gi.multiplier)
		} else {
			_ = s.accountRepo.UpdateRemoteGroupInfo(ctx, acc.ID, gi.name, gi.multiplier)
		}
	}
}

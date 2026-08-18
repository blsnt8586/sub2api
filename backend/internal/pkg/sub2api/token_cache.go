package sub2api

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

const (
	defaultTokenTTL = 23 * time.Hour // 保守值：大多数 JWT 有效期为 24h
	tokenExpirySkew = 5 * time.Minute
)

// TokenCache 缓存每个上游 Provider 的登录 Token 对，减少重复登录开销。
// authGroup 保证同一个 Provider 的登录或 Refresh Token 轮换只执行一次。
type TokenCache struct {
	mu        sync.RWMutex
	items     map[int64]*tokenEntry
	authGroup singleflight.Group
}

type tokenEntry struct {
	accessToken  string
	refreshToken string
	expiresAt    time.Time
}

// TokenPair 是客户端可复用的 Provider 认证状态。
type TokenPair struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
}

// NewTokenPair builds a reusable token pair. expiresAt is the already
// conservative expiry persisted by the Provider service; when absent it is
// derived from the JWT (or the default TTL for opaque access tokens).
func NewTokenPair(accessToken, refreshToken string, expiresAt *time.Time) TokenPair {
	pair := TokenPair{AccessToken: accessToken, RefreshToken: refreshToken}
	if expiresAt != nil && !expiresAt.IsZero() {
		pair.ExpiresAt = *expiresAt
	} else {
		pair.ExpiresAt = accessTokenExpiresAt(accessToken, 0)
	}
	return pair
}

// NewTokenCache 创建 TokenCache 实例
func NewTokenCache() *TokenCache {
	return &TokenCache{
		items: make(map[int64]*tokenEntry),
	}
}

// Get 取缓存 token；若不存在或已过期返回 "", false
func (c *TokenCache) Get(providerID int64) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.items[providerID]
	if !ok || time.Now().After(e.expiresAt) {
		return "", false
	}
	return e.accessToken, e.accessToken != ""
}

// Set 写入 token，TTL 从 JWT exp 字段解析（提前 5 分钟失效以防边界）；
// 解析失败则使用 defaultTokenTTL。保留该方法用于兼容只返回 Access Token 的旧上游。
func (c *TokenCache) Set(providerID int64, token string) {
	c.SetTokenPair(providerID, token, "", 0)
}

// GetTokenPair 返回完整 Token 对。Access Token 即使已过期也会返回，以便调用方
// 判断 401 是否已经被其他并发请求恢复；Refresh Token 可继续用于无密码续期。
func (c *TokenCache) GetTokenPair(providerID int64) (TokenPair, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.items[providerID]
	if !ok {
		return TokenPair{}, false
	}
	return TokenPair{
		AccessToken:  e.accessToken,
		RefreshToken: e.refreshToken,
		ExpiresAt:    e.expiresAt,
	}, e.accessToken != "" || e.refreshToken != ""
}

// SetTokenPair 原子替换 Access/Refresh Token。expiresIn 为上游显式返回的
// Access Token 有效期；未返回时从 JWT exp 推导，再失败才使用保守默认值。
func (c *TokenCache) SetTokenPair(providerID int64, accessToken, refreshToken string, expiresIn time.Duration) {
	c.SeedTokenPair(providerID, TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    accessTokenExpiresAt(accessToken, expiresIn),
	})
}

// SeedTokenPair atomically restores a persisted token pair without applying a
// second expiry skew.
func (c *TokenCache) SeedTokenPair(providerID int64, pair TokenPair) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[providerID] = &tokenEntry{
		accessToken:  pair.AccessToken,
		refreshToken: pair.RefreshToken,
		expiresAt:    pair.ExpiresAt,
	}
}

// EvictAccess 仅使 Access Token 失效，保留 Refresh Token 用于无密码续期。
func (c *TokenCache) EvictAccess(providerID int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.items[providerID]; ok {
		e.expiresAt = time.Now()
	}
}

// Evict 主动清除指定 Provider 的完整认证状态。
func (c *TokenCache) Evict(providerID int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, providerID)
}

func (c *TokenCache) authenticate(providerID int64, fn func() (TokenPair, error)) (TokenPair, error) {
	value, err, _ := c.authGroup.Do(strconv.FormatInt(providerID, 10), func() (interface{}, error) {
		return fn()
	})
	if err != nil {
		return TokenPair{}, err
	}
	pair, ok := value.(TokenPair)
	if !ok {
		return TokenPair{}, fmt.Errorf("unexpected token cache authentication result")
	}
	return pair, nil
}

func (p TokenPair) accessTokenValid(now time.Time) bool {
	return p.AccessToken != "" && now.Before(p.ExpiresAt)
}

func accessTokenExpiresAt(token string, expiresIn time.Duration) time.Time {
	now := time.Now()
	expiresAt := now.Add(defaultTokenTTL)
	if expiresIn > 0 {
		expiresAt = now.Add(expiresIn)
	} else if jwtExpiresAt, err := jwtExpiry(token); err == nil {
		expiresAt = jwtExpiresAt
	}

	remaining := expiresAt.Sub(now)
	if remaining <= 0 {
		return now
	}
	skew := tokenExpirySkew
	if proportionalSkew := remaining / 10; proportionalSkew < skew {
		skew = proportionalSkew
	}
	return expiresAt.Add(-skew)
}

// jwtExpiry 从 JWT token 中解析 exp 声明
func jwtExpiry(token string) (time.Time, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return time.Time{}, fmt.Errorf("not a JWT")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return time.Time{}, fmt.Errorf("decode payload: %w", err)
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil || claims.Exp == 0 {
		return time.Time{}, fmt.Errorf("no exp claim")
	}
	return time.Unix(claims.Exp, 0), nil
}

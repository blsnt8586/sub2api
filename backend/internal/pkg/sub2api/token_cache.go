package sub2api

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

const defaultTokenTTL = 23 * time.Hour // 保守值：大多数 JWT 有效期为 24h

// TokenCache 缓存每个上游 Provider 的登录 Token，减少重复登录开销。
// 并发安全，可在多个请求间共享同一实例。
type TokenCache struct {
	mu    sync.RWMutex
	items map[int64]*tokenEntry
}

type tokenEntry struct {
	token     string
	expiresAt time.Time
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
	return e.token, true
}

// Set 写入 token，TTL 从 JWT exp 字段解析（提前 5 分钟失效以防边界）；
// 解析失败则使用 defaultTokenTTL
func (c *TokenCache) Set(providerID int64, token string) {
	ttl := defaultTokenTTL
	if exp, err := jwtExpiry(token); err == nil {
		remaining := time.Until(exp) - 5*time.Minute
		if remaining > 0 {
			ttl = remaining
		}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[providerID] = &tokenEntry{
		token:     token,
		expiresAt: time.Now().Add(ttl),
	}
}

// Evict 主动清除指定 provider 的 token（收到 401 后调用，强制下次重新登录）
func (c *TokenCache) Evict(providerID int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, providerID)
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

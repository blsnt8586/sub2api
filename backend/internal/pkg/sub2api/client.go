package sub2api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ErrUnauthorized 表示上游返回了 401，token 已失效
type ErrUnauthorized struct {
	Body string
}

func (e *ErrUnauthorized) Error() string {
	return "unauthorized (401): " + e.Body
}

// IsUnauthorized 判断 error 是否为 401
func IsUnauthorized(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*ErrUnauthorized)
	return ok
}

// Client Sub2API HTTP 客户端
type Client struct {
	BaseURL    string
	Email      string
	Password   string
	HTTPClient *http.Client
	Token      string // JWT Token
}

// NewClient 创建 Sub2API 客户端
func NewClient(baseURL, email, password string) *Client {
	return &Client{
		BaseURL:  baseURL,
		Email:    email,
		Password: password,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// LoginRequest 登录请求
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginResponse 登录响应
type LoginResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		AccessToken string `json:"access_token"`
	} `json:"data"`
}

// Login 使用 email + password 登录
func (c *Client) Login(ctx context.Context) error {
	loginReq := LoginRequest{
		Email:    c.Email,
		Password: c.Password,
	}

	var resp LoginResponse
	if err := c.makeRequest(ctx, http.MethodPost, "/api/v1/auth/login", loginReq, &resp); err != nil {
		return fmt.Errorf("login request failed: %w", err)
	}

	if resp.Code != 0 {
		return fmt.Errorf("login failed: code=%d, message=%s", resp.Code, resp.Message)
	}

	if resp.Data.AccessToken == "" {
		return fmt.Errorf("login response missing access_token")
	}

	c.Token = resp.Data.AccessToken
	return nil
}

// APIKey API Key 结构
type APIKey struct {
	ID              int64   `json:"id"`
	Name            string  `json:"name"`
	Key             string  `json:"key"`
	Status          string  `json:"status"`
	GroupID         int64   `json:"group_id"`
	GroupName       string  `json:"-"` // 从嵌套的 group 对象中读取
	GroupMultiplier float64 `json:"-"` // 从嵌套的 group 对象中读取
	Group           *struct {
		ID             int64   `json:"id"`
		Name           string  `json:"name"`
		RateMultiplier float64 `json:"rate_multiplier"`
	} `json:"group"`
	CreatedAt string `json:"created_at"`
}

// Group 分组结构
type Group struct {
	ID             int64   `json:"id"`
	Name           string  `json:"name"`
	RateMultiplier float64 `json:"rate_multiplier"`
	Platform       string  `json:"platform"`
	Status         string  `json:"status"`
}

// APIKeysResponse API Keys 响应
type APIKeysResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Items []APIKey `json:"items"`
		Total int64    `json:"total"`
	} `json:"data"`
}

// GroupsResponse 分组响应
type GroupsResponse struct {
	Code    int     `json:"code"`
	Message string  `json:"message"`
	Data    []Group `json:"data"`
}

// GetAPIKeys 获取 API Keys
func (c *Client) GetAPIKeys(ctx context.Context, path string) ([]APIKey, error) {
	var resp APIKeysResponse
	if err := c.makeRequestWithAuth(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, fmt.Errorf("get api keys failed: %w", err)
	}

	if resp.Code != 0 {
		return nil, fmt.Errorf("api keys response error: code=%d, message=%s", resp.Code, resp.Message)
	}

	return resp.Data.Items, nil
}

// GetGroups 获取分组列表
func (c *Client) GetGroups(ctx context.Context, path string) ([]Group, error) {
	var resp GroupsResponse
	if err := c.makeRequestWithAuth(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, fmt.Errorf("get groups failed: %w", err)
	}

	if resp.Code != 0 {
		return nil, fmt.Errorf("groups response error: code=%d, message=%s", resp.Code, resp.Message)
	}

	return resp.Data, nil
}

// makeRequest 发起 HTTP 请求（不带认证）
func (c *Client) makeRequest(ctx context.Context, method, path string, body, result interface{}) error {
	url := c.BaseURL + path

	var bodyReader io.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("http error: status=%d, body=%s", resp.StatusCode, string(respBody))
	}

	if result != nil {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("unmarshal response: %w, body=%s", err, string(respBody))
		}
	}

	return nil
}

// EnsureLoggedIn 优先从缓存中取 token；缓存未命中时登录并写入缓存。
// 应在所有需要认证的操作前调用，替代直接调用 Login。
func (c *Client) EnsureLoggedIn(ctx context.Context, providerID int64, cache *TokenCache) error {
	if cache != nil {
		if token, ok := cache.Get(providerID); ok {
			c.Token = token
			return nil
		}
	}
	if err := c.Login(ctx); err != nil {
		return err
	}
	if cache != nil {
		cache.Set(providerID, c.Token)
	}
	return nil
}

// makeRequestWithAuth 发起带认证的 HTTP 请求。
// 当服务端返回 401 时，返回 *ErrUnauthorized，调用方可据此刷新 token 并重试。
func (c *Client) makeRequestWithAuth(ctx context.Context, method, path string, body, result interface{}) error {
	if c.Token == "" {
		return fmt.Errorf("token is empty, call EnsureLoggedIn first")
	}

	url := c.BaseURL + path

	var bodyReader io.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	// 401 单独返回，上层可做 token 刷新后重试
	if resp.StatusCode == http.StatusUnauthorized {
		return &ErrUnauthorized{Body: string(respBody)}
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("http error: status=%d, body=%s", resp.StatusCode, string(respBody))
	}

	if result != nil {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("unmarshal response: %w, body=%s", err, string(respBody))
		}
	}

	return nil
}

// UpdateAPIKeyGroupRequest 更新 APIKey 分组请求
type UpdateAPIKeyGroupRequest struct {
	GroupID int64 `json:"group_id"`
}

// UpdateAPIKeyGroupResponse 更新 APIKey 分组响应
type UpdateAPIKeyGroupResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		ID      int64  `json:"id"`
		GroupID int64  `json:"group_id"`
		Name    string `json:"name"`
	} `json:"data"`
}

// UpdateAPIKeyGroup 修改远程 APIKey 的分组
func (c *Client) UpdateAPIKeyGroup(ctx context.Context, keysPath string, keyID, groupID int64) error {
	// 构造 PUT 路径，如 /api/v1/keys/272
	path := fmt.Sprintf("%s/%d", keysPath, keyID)

	var resp UpdateAPIKeyGroupResponse
	if err := c.makeRequestWithAuth(ctx, "PUT", path, UpdateAPIKeyGroupRequest{GroupID: groupID}, &resp); err != nil {
		return fmt.Errorf("update api key group request failed: %w", err)
	}

	if resp.Code != 0 {
		return fmt.Errorf("update api key group failed: code=%d, message=%s", resp.Code, resp.Message)
	}

	return nil
}

package sub2api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/httpclient"
	"github.com/Wei-Shaw/sub2api/internal/util/httputil"
)

// ErrUnauthorized 表示上游返回了 401，token 已失效
type ErrUnauthorized struct {
	Body string
}

func (e *ErrUnauthorized) Error() string {
	return "unauthorized (401): " + e.Body
}

// ErrAuthInteractionRequired means a non-password Provider can no longer
// refresh its imported token pair and requires an administrator to import a
// new dedicated session.
type ErrAuthInteractionRequired struct {
	Cause error
}

func (e *ErrAuthInteractionRequired) Error() string {
	if e == nil || e.Cause == nil {
		return "provider authentication requires token re-import"
	}
	return "provider authentication requires token re-import: " + e.Cause.Error()
}

func (e *ErrAuthInteractionRequired) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// ErrCloudflareChallenge identifies a Cloudflare edge challenge without
// retaining the challenge HTML in logs or probe history.
type ErrCloudflareChallenge struct {
	StatusCode int
	RayID      string
}

func (e *ErrCloudflareChallenge) Error() string {
	message := fmt.Sprintf("cloudflare challenge: status=%d", e.StatusCode)
	if e.RayID != "" {
		message += ", cf-ray=" + e.RayID
	}
	return message
}

// ErrCloudflareAccessDenied identifies a Cloudflare Access login/denial page.
type ErrCloudflareAccessDenied struct {
	StatusCode int
	RayID      string
}

func (e *ErrCloudflareAccessDenied) Error() string {
	message := fmt.Sprintf("cloudflare access denied: status=%d", e.StatusCode)
	if e.RayID != "" {
		message += ", cf-ray=" + e.RayID
	}
	return message
}

// HTTPError is a bounded upstream HTTP failure safe for operational logs.
type HTTPError struct {
	StatusCode int
	Body       string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("http error: status=%d, body=%s", e.StatusCode, e.Body)
}

// ErrUnexpectedResponse is returned when an endpoint expected to return the
// Sub2API JSON envelope instead serves an HTML page (usually a copied /keys
// frontend route or an edge error page). It deliberately carries no response
// body because HTML pages may contain cookies, challenge markup, or scripts.
type ErrUnexpectedResponse struct {
	StatusCode  int
	ContentType string
}

func (e *ErrUnexpectedResponse) Error() string {
	message := fmt.Sprintf("unexpected upstream response: status=%d", e.StatusCode)
	if e.ContentType != "" {
		message += ", content_type=" + e.ContentType
	}
	return message
}

// IsUnauthorized 判断 error 是否为 401
func IsUnauthorized(err error) bool {
	if err == nil {
		return false
	}
	var unauthorized *ErrUnauthorized
	return errors.As(err, &unauthorized)
}

// Client Sub2API HTTP 客户端
type Client struct {
	BaseURL        string
	Email          string
	Password       string
	HTTPClient     *http.Client
	Token          string // JWT Access Token
	RefreshToken   string
	TokenExpiresIn time.Duration

	providerID int64
	tokenCache *TokenCache

	disablePasswordLogin bool
	onTokenPairUpdated   func(context.Context, TokenPair) error
}

// NewClient 创建 Sub2API 客户端
func NewClient(baseURL, email, password string) *Client {
	return &Client{
		BaseURL:  normalizeBaseURL(baseURL),
		Email:    email,
		Password: password,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ConfigureProxy routes every request made by this client through proxyURL.
// Invalid proxy configuration fails closed instead of silently falling back to
// a direct connection. The returned HTTP client has its own timeout field while
// reusing the shared transport pool.
func (c *Client) ConfigureProxy(proxyURL string) error {
	shared, err := httpclient.GetClient(httpclient.Options{
		ProxyURL: proxyURL,
		Timeout:  30 * time.Second,
	})
	if err != nil {
		return fmt.Errorf("configure provider proxy: %w", err)
	}
	client := *shared
	c.HTTPClient = &client
	return nil
}

// normalizeBaseURL keeps request construction compatible with providers entered
// with a trailing slash (for example, https://o10.top/). The client appends
// protocol paths itself; retaining that slash would produce //api/... and some
// reverse proxies route that path to the SPA instead of the backend.
func normalizeBaseURL(baseURL string) string {
	normalized := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	parsed, err := url.Parse(normalized)
	if err != nil || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return normalized
	}
	path := strings.TrimRight(parsed.Path, "/")
	for _, suffix := range []string{"/api/v1/keys", "/api/v1", "/keys"} {
		if path == suffix {
			path = ""
			break
		}
		if strings.HasSuffix(path, suffix) {
			path = strings.TrimSuffix(path, suffix)
			break
		}
	}
	parsed.Path = strings.TrimRight(path, "/")
	parsed.RawPath = ""
	return strings.TrimRight(parsed.String(), "/")
}

func (c *Client) requestURL(path string) string {
	base := normalizeBaseURL(c.BaseURL)
	return base + "/" + strings.TrimLeft(path, "/")
}

// ConfigureImportedTokenAuth prevents password fallback and persists every
// rotated refresh token before the new token pair is used by the caller.
func (c *Client) ConfigureImportedTokenAuth(onUpdated func(context.Context, TokenPair) error) {
	c.disablePasswordLogin = true
	c.onTokenPairUpdated = onUpdated
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
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
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
	c.RefreshToken = resp.Data.RefreshToken
	c.TokenExpiresIn = time.Duration(resp.Data.ExpiresIn) * time.Second
	return c.storeBoundTokenPair(ctx)
}

// RefreshTokenRequest 使用远程登录接口返回的 Refresh Token 换取新的 Token 对。
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// RefreshTokenResponse 与新版 Sub2API /api/v1/auth/refresh 响应一致。
type RefreshTokenResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
	} `json:"data"`
}

// Refresh 使用 Refresh Token 轮换新的 Token 对。新版上游会同时返回新的
// Refresh Token；若兼容实现未返回，则继续保留当前 Refresh Token。
func (c *Client) Refresh(ctx context.Context) error {
	if c.RefreshToken == "" {
		return fmt.Errorf("refresh token is empty")
	}
	currentRefreshToken := c.RefreshToken
	var resp RefreshTokenResponse
	if err := c.makeRequest(ctx, http.MethodPost, "/api/v1/auth/refresh", RefreshTokenRequest{
		RefreshToken: currentRefreshToken,
	}, &resp); err != nil {
		return fmt.Errorf("refresh request failed: %w", err)
	}
	if resp.Code != 0 {
		return fmt.Errorf("refresh failed: code=%d, message=%s", resp.Code, resp.Message)
	}
	if resp.Data.AccessToken == "" {
		return fmt.Errorf("refresh response missing access_token")
	}
	if resp.Data.RefreshToken == "" {
		resp.Data.RefreshToken = currentRefreshToken
	}
	c.Token = resp.Data.AccessToken
	c.RefreshToken = resp.Data.RefreshToken
	c.TokenExpiresIn = time.Duration(resp.Data.ExpiresIn) * time.Second
	return c.storeBoundTokenPair(ctx)
}

func (c *Client) storeBoundTokenPair(ctx context.Context) error {
	pair := TokenPair{
		AccessToken:  c.Token,
		RefreshToken: c.RefreshToken,
		ExpiresAt:    accessTokenExpiresAt(c.Token, c.TokenExpiresIn),
	}
	if c.onTokenPairUpdated != nil {
		if err := c.onTokenPairUpdated(ctx, pair); err != nil {
			return fmt.Errorf("persist rotated token pair: %w", err)
		}
	}
	if c.tokenCache != nil {
		c.tokenCache.SeedTokenPair(c.providerID, pair)
	}
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
	Description    string  `json:"description"`
	RateMultiplier float64 `json:"rate_multiplier"`
	Platform       string  `json:"platform"`
	Status         string  `json:"status"`
}

// CurrentUserBalance is the wallet balance exposed by the remote Sub2API
// instance for the authenticated Provider account.
type CurrentUserBalance struct {
	Balance float64 `json:"balance"`
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

// CurrentUserResponse is the narrow subset of /auth/me needed by Provider
// monitoring. Keeping the response narrow avoids coupling this client to the
// remote instance's complete user profile schema.
type CurrentUserResponse struct {
	Code    int                `json:"code"`
	Message string             `json:"message"`
	Data    CurrentUserBalance `json:"data"`
}

// GroupRatesResponse contains user-specific rate overrides keyed by group ID.
// JSON object keys are strings even when the upstream Go map uses integer IDs.
type GroupRatesResponse struct {
	Code    int                `json:"code"`
	Message string             `json:"message"`
	Data    map[string]float64 `json:"data"`
}

// GetAPIKeys 获取 API Keys
func (c *Client) GetAPIKeys(ctx context.Context, path string) ([]APIKey, error) {
	const pageSize = 100
	all := make([]APIKey, 0)
	seen := make(map[int64]struct{})
	for page := 1; page <= 100; page++ {
		items, total, err := c.GetAPIKeysPage(ctx, path, page, pageSize)
		if err != nil {
			return nil, fmt.Errorf("get api keys failed: %w", err)
		}
		before := len(all)
		for _, item := range items {
			if _, ok := seen[item.ID]; ok {
				continue
			}
			seen[item.ID] = struct{}{}
			all = append(all, item)
		}
		if len(items) == 0 || len(all) == before || (total > 0 && int64(len(all)) >= total) || (total <= 0 && len(items) < pageSize) {
			break
		}
	}
	return all, nil
}

// GetAPIKeysPage returns one page of remote API keys. Sub2API defaults to a
// small page size, so callers that need a complete key inventory must iterate.
func (c *Client) GetAPIKeysPage(ctx context.Context, path string, page, pageSize int) ([]APIKey, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 1000 {
		pageSize = 100
	}
	u, err := url.Parse(path)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid api keys path: %w", err)
	}
	q := u.Query()
	q.Set("page", strconv.Itoa(page))
	q.Set("page_size", strconv.Itoa(pageSize))
	u.RawQuery = q.Encode()
	var resp APIKeysResponse
	if err := c.makeRequestWithAuth(ctx, http.MethodGet, u.String(), nil, &resp); err != nil {
		return nil, 0, err
	}
	if resp.Code != 0 {
		return nil, 0, fmt.Errorf("api keys response error: code=%d, message=%s", resp.Code, resp.Message)
	}
	return resp.Data.Items, resp.Data.Total, nil
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

// GetCurrentUserBalance reads the authenticated Provider account's remote
// wallet balance. It uses the same refresh-aware authentication path as keys
// and groups, so imported token pairs are rotated and persisted normally.
func (c *Client) GetCurrentUserBalance(ctx context.Context) (float64, error) {
	var resp CurrentUserResponse
	if err := c.makeRequestWithAuth(ctx, http.MethodGet, "/api/v1/auth/me", nil, &resp); err != nil {
		return 0, fmt.Errorf("get current user balance failed: %w", err)
	}
	if resp.Code != 0 {
		return 0, fmt.Errorf("current user response error: code=%d, message=%s", resp.Code, resp.Message)
	}
	return resp.Data.Balance, nil
}

// GetGroupRates reads user-specific group multiplier overrides. Callers should
// fall back to Group.RateMultiplier when an older compatible upstream does not
// expose this optional endpoint.
func (c *Client) GetGroupRates(ctx context.Context) (map[string]float64, error) {
	var resp GroupRatesResponse
	if err := c.makeRequestWithAuth(ctx, http.MethodGet, "/api/v1/groups/rates", nil, &resp); err != nil {
		return nil, fmt.Errorf("get group rates failed: %w", err)
	}
	if resp.Code != 0 {
		return nil, fmt.Errorf("group rates response error: code=%d, message=%s", resp.Code, resp.Message)
	}
	if resp.Data == nil {
		return map[string]float64{}, nil
	}
	return resp.Data, nil
}

// ProbeHealth checks an unauthenticated control-plane health endpoint. The
// response body is intentionally ignored; HTTP success proves reachability and
// the authenticated Keys/Groups checks validate the API contract separately.
func (c *Client) ProbeHealth(ctx context.Context, path string) error {
	return c.makeRequest(ctx, http.MethodGet, path, nil, nil)
}

// makeRequest 发起 HTTP 请求（不带认证）
func (c *Client) makeRequest(ctx context.Context, method, path string, body, result interface{}) error {
	url := c.requestURL(path)

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

	if err := classifyHTTPResponseError(resp, respBody); err != nil {
		return err
	}

	if result != nil {
		if isUnexpectedJSONResponse(resp, respBody) {
			return newUnexpectedResponseError(resp)
		}
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("unmarshal response: %w, body=%s", err, httputil.TruncateBody(respBody, 512))
		}
	}

	return nil
}

// EnsureLoggedIn 优先复用有效 Access Token；Access Token 过期时优先使用
// Refresh Token 轮换。密码模式可在刷新失败后重新登录，导入 Token 模式则返回需人工处理的错误。
func (c *Client) EnsureLoggedIn(ctx context.Context, providerID int64, cache *TokenCache) error {
	c.providerID = providerID
	c.tokenCache = cache
	if cache == nil {
		if c.Token != "" && time.Now().Before(accessTokenExpiresAt(c.Token, c.TokenExpiresIn)) {
			return nil
		}
		if c.RefreshToken != "" {
			if err := c.Refresh(ctx); err == nil {
				return nil
			} else if c.disablePasswordLogin {
				return &ErrAuthInteractionRequired{Cause: err}
			}
		}
		if c.disablePasswordLogin {
			return &ErrAuthInteractionRequired{Cause: errors.New("no reusable token pair is configured")}
		}
		return c.Login(ctx)
	}
	if pair, ok := cache.GetTokenPair(providerID); ok && pair.accessTokenValid(time.Now()) {
		c.applyTokenPair(pair)
		return nil
	}
	return c.authenticate(ctx, false, "")
}

// Reauthenticate 恢复一次被上游 401 拒绝的认证状态。若其他并发请求已经
// 完成轮换，直接采用缓存中的新 Token；否则按认证模式串行执行 Refresh 或密码登录。
func (c *Client) Reauthenticate(ctx context.Context) error {
	if c.tokenCache == nil {
		if c.RefreshToken != "" {
			if err := c.Refresh(ctx); err == nil {
				return nil
			} else if ctx.Err() != nil {
				return ctx.Err()
			}
		}
		if c.disablePasswordLogin {
			return &ErrAuthInteractionRequired{Cause: errors.New("refresh token is missing or invalid")}
		}
		return c.Login(ctx)
	}
	return c.authenticate(ctx, true, c.Token)
}

func (c *Client) authenticate(ctx context.Context, force bool, rejectedAccessToken string) error {
	pair, err := c.tokenCache.authenticate(c.providerID, func() (TokenPair, error) {
		cached, _ := c.tokenCache.GetTokenPair(c.providerID)
		if cached.accessTokenValid(time.Now()) && (!force || cached.AccessToken != rejectedAccessToken) {
			return cached, nil
		}

		authClient := NewClient(c.BaseURL, c.Email, c.Password)
		authClient.HTTPClient = c.HTTPClient
		authClient.providerID = c.providerID
		authClient.tokenCache = c.tokenCache
		authClient.disablePasswordLogin = c.disablePasswordLogin
		authClient.onTokenPairUpdated = c.onTokenPairUpdated
		authClient.RefreshToken = cached.RefreshToken
		if authClient.RefreshToken == "" {
			authClient.RefreshToken = c.RefreshToken
		}

		var refreshErr error
		if authClient.RefreshToken != "" {
			refreshErr = authClient.Refresh(ctx)
			if refreshErr == nil {
				pair, _ := c.tokenCache.GetTokenPair(c.providerID)
				return pair, nil
			}
			if ctx.Err() != nil {
				return TokenPair{}, ctx.Err()
			}
		}
		if c.disablePasswordLogin {
			if refreshErr == nil {
				refreshErr = errors.New("refresh token is missing")
			}
			return TokenPair{}, &ErrAuthInteractionRequired{Cause: refreshErr}
		}

		if loginErr := authClient.Login(ctx); loginErr != nil {
			if refreshErr != nil {
				return TokenPair{}, fmt.Errorf("refresh token failed: %v; password login failed: %w", refreshErr, loginErr)
			}
			return TokenPair{}, loginErr
		}
		pair, _ := c.tokenCache.GetTokenPair(c.providerID)
		return pair, nil
	})
	if err != nil {
		return err
	}
	c.applyTokenPair(pair)
	return nil
}

func (c *Client) applyTokenPair(pair TokenPair) {
	c.Token = pair.AccessToken
	c.RefreshToken = pair.RefreshToken
	if pair.ExpiresAt.IsZero() {
		c.TokenExpiresIn = 0
		return
	}
	c.TokenExpiresIn = time.Until(pair.ExpiresAt)
}

// makeRequestWithAuth 发起带认证的 HTTP 请求。
// 服务端返回 401 时自动恢复认证并重试原请求一次。
func (c *Client) makeRequestWithAuth(ctx context.Context, method, path string, body, result interface{}) error {
	err := c.doRequestWithAuth(ctx, method, path, body, result)
	if !IsUnauthorized(err) {
		return err
	}
	if authErr := c.Reauthenticate(ctx); authErr != nil {
		return fmt.Errorf("reauthenticate after 401 failed: %w", authErr)
	}
	return c.doRequestWithAuth(ctx, method, path, body, result)
}

func (c *Client) doRequestWithAuth(ctx context.Context, method, path string, body, result interface{}) error {
	if c.Token == "" {
		return fmt.Errorf("token is empty, call EnsureLoggedIn first")
	}

	url := c.requestURL(path)

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

	if err := classifyHTTPResponseError(resp, respBody); err != nil {
		return err
	}

	if result != nil {
		if isUnexpectedJSONResponse(resp, respBody) {
			return newUnexpectedResponseError(resp)
		}
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("unmarshal response: %w, body=%s", err, httputil.TruncateBody(respBody, 512))
		}
	}

	return nil
}

func classifyHTTPResponseError(resp *http.Response, body []byte) error {
	if resp == nil {
		return nil
	}
	if httputil.IsCloudflareChallengeResponse(resp.StatusCode, resp.Header, body) {
		return &ErrCloudflareChallenge{
			StatusCode: resp.StatusCode,
			RayID:      httputil.ExtractCloudflareRayID(resp.Header, body),
		}
	}
	if isCloudflareAccessResponse(resp, body) {
		return &ErrCloudflareAccessDenied{
			StatusCode: resp.StatusCode,
			RayID:      httputil.ExtractCloudflareRayID(resp.Header, body),
		}
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return &ErrUnauthorized{Body: httputil.TruncateBody(body, 512)}
	}
	if resp.StatusCode >= http.StatusBadRequest {
		if isUnexpectedJSONResponse(resp, body) {
			return newUnexpectedResponseError(resp)
		}
		return &HTTPError{StatusCode: resp.StatusCode, Body: httputil.TruncateBody(body, 512)}
	}
	return nil
}

func isUnexpectedJSONResponse(resp *http.Response, body []byte) bool {
	if resp == nil {
		return false
	}
	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	if strings.Contains(contentType, "text/html") || strings.Contains(contentType, "application/xhtml") {
		return true
	}
	trimmed := bytes.TrimSpace(body)
	return len(trimmed) > 0 && trimmed[0] == '<'
}

func newUnexpectedResponseError(resp *http.Response) error {
	if resp == nil {
		return &ErrUnexpectedResponse{}
	}
	return &ErrUnexpectedResponse{
		StatusCode:  resp.StatusCode,
		ContentType: strings.TrimSpace(resp.Header.Get("Content-Type")),
	}
}

func isCloudflareAccessResponse(resp *http.Response, body []byte) bool {
	preview := strings.ToLower(httputil.TruncateBody(body, 4096))
	requestURL := ""
	if resp.Request != nil && resp.Request.URL != nil {
		requestURL = strings.ToLower(resp.Request.URL.String())
	}
	if strings.Contains(requestURL, "/cdn-cgi/access/") || strings.Contains(requestURL, "cloudflareaccess.com") {
		return true
	}
	if strings.Contains(preview, "cloudflare access") &&
		(strings.Contains(preview, "access denied") || strings.Contains(preview, "sign in")) {
		return true
	}
	return false
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

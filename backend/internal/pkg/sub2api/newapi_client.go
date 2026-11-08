package sub2api

// New API uses a different control-plane protocol from Sub2API. Keep that
// protocol here while reusing the existing provider services and account state.
import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"math"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

var errNewAPIUnsupportedStats = errors.New("New API does not expose equivalent funding, cache or per-key period statistics; unavailable")

type newAPISession struct {
	Token         string
	RefreshCookie string
	UserID        int64
	UserGroup     string
	Cookies       []*http.Cookie
	ExpiresAt     time.Time
}

type newAPIClient struct {
	client       *Client
	session      newAPISession
	tokenAuth    bool
	refreshAuth  bool
	onRefresh    func(context.Context, string, string, time.Time) error
	quotaPerUnit float64
	groupsPath   string
}

func (c *Client) IsNewAPI() bool { return c.newAPI != nil }

// ConfigureNewAPI selects New API password/session authentication, or an
// imported browser/management bearer token with the numeric user ID in Email.
func (c *Client) ConfigureNewAPI(accessToken string) error {
	return c.configureNewAPI(accessToken, "", nil)
}

// ConfigureNewAPIBrowserSession configures a bearer token plus the New API
// HttpOnly refresh cookie exported from the same browser session.
func (c *Client) ConfigureNewAPIBrowserSession(accessToken, refreshCookie string, onRefresh func(context.Context, string, string, time.Time) error) error {
	return c.configureNewAPI(accessToken, refreshCookie, onRefresh)
}

func (c *Client) configureNewAPI(accessToken, refreshCookie string, onRefresh func(context.Context, string, string, time.Time) error) error {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return err
	}
	httpClient := *c.HTTPClient
	httpClient.Jar = jar
	// Control APIs must not silently redirect into an SPA or another site.
	httpClient.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	c.HTTPClient = &httpClient
	n := &newAPIClient{client: c, groupsPath: "/api/user/self/groups"}
	if accessToken != "" {
		id, err := strconv.ParseInt(strings.TrimSpace(c.Email), 10, 64)
		if err != nil || id <= 0 {
			return errors.New("New API bearer session requires a positive numeric user ID")
		}
		n.tokenAuth = true
		n.refreshAuth = strings.TrimSpace(refreshCookie) != ""
		n.onRefresh = onRefresh
		n.session = newAPISession{Token: strings.TrimSpace(accessToken), RefreshCookie: strings.TrimSpace(refreshCookie), UserID: id, ExpiresAt: accessTokenExpiresAt(accessToken, 0)}
	}
	c.newAPI = n
	return nil
}

// request always checks success, even when the upstream uses HTTP 200 for an
// authentication or application error. Never retain response bodies in errors:
// token endpoints can contain full secrets.
func (n *newAPIClient) request(ctx context.Context, method, path string, body, out any, auth bool) error {
	var input io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		input = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, n.client.requestURL(path), input)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	if auth {
		if n.session.Token != "" {
			req.Header.Set("Authorization", "Bearer "+n.session.Token)
		}
		if n.session.UserID > 0 {
			req.Header.Set("New-Api-User", strconv.FormatInt(n.session.UserID, 10))
		}
	}
	resp, err := n.client.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("New API request failed: %w", err)
	}
	defer resp.Body.Close()
	const maxBody = 8 << 20
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return err
	}
	if len(b) > maxBody {
		return errors.New("New API response exceeds 8 MiB")
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return &ErrUnauthorized{Body: "New API authentication rejected; check credentials or reauthenticate"}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &HTTPError{StatusCode: resp.StatusCode, Body: "New API control endpoint rejected the request"}
	}
	var envelope struct {
		Success *bool           `json:"success"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(b, &envelope); err != nil || envelope.Success == nil {
		return &ErrUnexpectedResponse{StatusCode: resp.StatusCode, ContentType: resp.Header.Get("Content-Type")}
	}
	if !*envelope.Success {
		return errors.New("New API returned success=false; check authentication, permissions, 2FA or captcha requirements")
	}
	if out != nil {
		if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
			return errors.New("New API response is missing data")
		}
		if err := json.Unmarshal(envelope.Data, out); err != nil {
			return errors.New("New API response has an incompatible data format")
		}
	}
	return nil
}

func (n *newAPIClient) authedRequest(ctx context.Context, method, path string, body, out any) error {
	err := n.request(ctx, method, path, body, out, true)
	if !IsUnauthorized(err) || (!n.refreshAuth && n.tokenAuth) {
		return err
	}
	if n.refreshAuth {
		if err := n.refresh(ctx); err != nil {
			return err
		}
		return n.request(ctx, method, path, body, out, true)
	}
	if err := n.login(ctx); err != nil {
		return err
	}
	return n.request(ctx, method, path, body, out, true)
}

func (n *newAPIClient) ensureLoggedIn(ctx context.Context) error {
	if n.tokenAuth {
		if n.refreshAuth && !time.Now().Add(time.Minute).Before(n.session.ExpiresAt) {
			if err := n.refresh(ctx); err != nil {
				return err
			}
		}
		_, err := n.profile(ctx)
		return err
	}
	cache := n.client.tokenCache
	if cache == nil {
		return n.login(ctx)
	}
	load := func() (newAPISession, bool) {
		cache.mu.RLock()
		defer cache.mu.RUnlock()
		s, ok := cache.newAPISessions[n.client.providerID]
		return s, ok && time.Now().Before(s.ExpiresAt)
	}
	if s, ok := load(); ok {
		n.applySession(s)
		return nil
	}
	_, err, _ := cache.authGroup.Do("newapi:"+strconv.FormatInt(n.client.providerID, 10), func() (any, error) {
		if s, ok := load(); ok {
			return s, nil
		}
		if err := n.login(ctx); err != nil {
			return nil, err
		}
		return n.session, nil
	})
	if err != nil {
		return err
	}
	if s, ok := load(); ok {
		n.applySession(s)
		return nil
	}
	return errors.New("New API login session was invalidated; retry")
}

func (n *newAPIClient) applySession(s newAPISession) {
	n.session = s
	req, _ := http.NewRequest(http.MethodGet, n.client.BaseURL, nil)
	n.client.HTTPClient.Jar.SetCookies(req.URL, s.Cookies)
}

func (n *newAPIClient) refresh(ctx context.Context) error {
	cache := n.client.tokenCache
	if cache == nil {
		return n.refreshSession(ctx)
	}
	_, err, _ := cache.authGroup.Do("newapi-refresh:"+strconv.FormatInt(n.client.providerID, 10), func() (any, error) {
		cache.mu.RLock()
		latest, ok := cache.newAPISessions[n.client.providerID]
		cache.mu.RUnlock()
		if ok && latest.Token != "" && latest.Token != n.session.Token {
			n.applySession(latest)
			if time.Now().Add(time.Minute).Before(latest.ExpiresAt) {
				return nil, nil
			}
		}
		return nil, n.refreshSession(ctx)
	})
	return err
}

func (n *newAPIClient) refreshSession(ctx context.Context) error {
	if strings.TrimSpace(n.session.RefreshCookie) == "" {
		return errors.New("New API refresh cookie is missing; re-import browser credentials")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.client.requestURL("/api/user/auth/refresh"), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Cookie", "new_api_refresh="+n.session.RefreshCookie)
	if base, err := url.Parse(n.client.BaseURL); err == nil {
		req.Header.Set("Origin", base.Scheme+"://"+base.Host)
		req.Header.Set("Referer", base.Scheme+"://"+base.Host+"/")
	}
	resp, err := n.client.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("New API session refresh failed: %w", err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &ErrUnauthorized{Body: "New API refresh session rejected"}
	}
	var envelope struct {
		Success *bool `json:"success"`
		Data    struct {
			AccessToken string `json:"access_token"`
			ExpiresAt   int64  `json:"access_expires_at"`
			User        struct {
				ID int64 `json:"id"`
			} `json:"user"`
		} `json:"data"`
	}
	if err := json.Unmarshal(b, &envelope); err != nil || envelope.Success == nil || !*envelope.Success || envelope.Data.AccessToken == "" || envelope.Data.User.ID != n.session.UserID {
		return errors.New("New API refresh response is invalid or belongs to another user")
	}
	refreshCookie := n.session.RefreshCookie
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "new_api_refresh" && cookie.Value != "" {
			refreshCookie = cookie.Value
		}
	}
	expiresAt := time.Now().Add(time.Hour)
	if envelope.Data.ExpiresAt > 0 {
		expiresAt = time.Unix(envelope.Data.ExpiresAt, 0).Add(-time.Minute)
	}
	n.session.Token, n.session.RefreshCookie, n.session.ExpiresAt = envelope.Data.AccessToken, refreshCookie, expiresAt
	if n.onRefresh != nil {
		if err := n.onRefresh(ctx, n.session.Token, n.session.RefreshCookie, n.session.ExpiresAt); err != nil {
			return err
		}
	}
	if cache := n.client.tokenCache; cache != nil {
		cache.mu.Lock()
		if cache.newAPISessions == nil {
			cache.newAPISessions = make(map[int64]newAPISession)
		}
		cache.newAPISessions[n.client.providerID] = n.session
		cache.mu.Unlock()
	}
	return nil
}

func (n *newAPIClient) login(ctx context.Context) error {
	if n.tokenAuth {
		_, err := n.profile(ctx)
		return err
	}
	// Start a fresh session so a partial 2FA response cannot reuse old cookies.
	jar, _ := cookiejar.New(nil)
	n.client.HTTPClient.Jar = jar
	n.session = newAPISession{}
	var data struct {
		ID              int64  `json:"id"`
		AccessToken     string `json:"access_token"`
		AccessExpiresAt int64  `json:"access_expires_at"`
		User            struct {
			ID int64 `json:"id"`
		} `json:"user"`
		Require2FA bool `json:"require_2fa"`
	}
	if err := n.request(ctx, http.MethodPost, "/api/user/login", map[string]string{"username": n.client.Email, "password": n.client.Password}, &data, false); err != nil {
		return err
	}
	if data.Require2FA {
		return errors.New("New API requires interactive 2FA; use a management access token")
	}
	id := data.ID
	if data.User.ID > 0 {
		id = data.User.ID
	}
	req, _ := http.NewRequest(http.MethodGet, n.client.BaseURL+"/api/user/self", nil)
	cookies := jar.Cookies(req.URL)
	if id <= 0 || (data.AccessToken == "" && len(cookies) == 0) {
		return errors.New("New API login did not return a usable session; use a management access token for 2FA, captcha or encrypted-password sites")
	}
	n.session = newAPISession{Token: data.AccessToken, UserID: id, Cookies: cookies, ExpiresAt: time.Now().Add(time.Hour)}
	if data.AccessExpiresAt > 0 && time.Unix(data.AccessExpiresAt, 0).Before(n.session.ExpiresAt) {
		n.session.ExpiresAt = time.Unix(data.AccessExpiresAt, 0).Add(-time.Minute)
	}
	// Validate both the session and its identity before publishing it to cache.
	if _, err := n.profileRequest(ctx, false); err != nil {
		return err
	}
	if cache := n.client.tokenCache; cache != nil {
		cache.mu.Lock()
		defer cache.mu.Unlock()
		if cache.newAPISessions == nil {
			cache.newAPISessions = make(map[int64]newAPISession)
		}
		cache.newAPISessions[n.client.providerID] = n.session
	}
	return nil
}

func (n *newAPIClient) health(ctx context.Context) error {
	return n.request(ctx, http.MethodGet, "/api/status", nil, nil, false)
}

func (n *newAPIClient) quotaUnit(ctx context.Context) (float64, error) {
	if n.quotaPerUnit > 0 {
		return n.quotaPerUnit, nil
	}
	var status struct {
		QuotaPerUnit float64 `json:"quota_per_unit"`
	}
	if err := n.request(ctx, http.MethodGet, "/api/status", nil, &status, false); err != nil {
		return 0, err
	}
	if status.QuotaPerUnit <= 0 || math.IsNaN(status.QuotaPerUnit) || math.IsInf(status.QuotaPerUnit, 0) {
		return 0, errors.New("New API status has no valid quota_per_unit; balance conversion unavailable")
	}
	// quota_per_unit is the raw quota per base monetary unit. UI currency
	// conversions are presentation only; provider.remote_cost_divisor is applied
	// once by the service for sites with customized accounting units.
	n.quotaPerUnit = status.QuotaPerUnit
	return n.quotaPerUnit, nil
}

func (n *newAPIClient) profile(ctx context.Context) (CurrentUserProfile, error) {
	return n.profileRequest(ctx, true)
}

func (n *newAPIClient) profileRequest(ctx context.Context, retry bool) (CurrentUserProfile, error) {
	var user struct {
		ID        int64    `json:"id"`
		Quota     *float64 `json:"quota"`
		UsedQuota *float64 `json:"used_quota"`
		Group     string   `json:"group"`
	}
	request := n.request
	var err error
	if retry {
		err = n.authedRequest(ctx, http.MethodGet, "/api/user/self", nil, &user)
	} else {
		err = request(ctx, http.MethodGet, "/api/user/self", nil, &user, true)
	}
	if err != nil {
		return CurrentUserProfile{}, err
	}
	if user.ID <= 0 || user.ID != n.session.UserID {
		return CurrentUserProfile{}, errors.New("New API authenticated user ID does not match the configured login identity")
	}
	if user.Quota == nil || user.UsedQuota == nil {
		return CurrentUserProfile{}, errors.New("New API profile is missing quota fields")
	}
	unit, err := n.quotaUnit(ctx)
	if err != nil {
		return CurrentUserProfile{}, err
	}
	n.session.UserGroup = user.Group
	used := *user.UsedQuota / unit
	return CurrentUserProfile{Balance: *user.Quota / unit, TotalConsumed: &used}, nil
}

// IDs must remain stable across requests, map iteration order and restarts, and
// fit exactly in a JavaScript number. A collision is an error, never a merge.
func newAPIGroupID(name string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte("newapi:group:" + name))
	return int64(h.Sum64()%((1<<53)-1)) + 1
}

func (n *newAPIClient) groups(ctx context.Context) ([]Group, error) {
	var data map[string]struct {
		Ratio       json.RawMessage `json:"ratio"`
		Description string          `json:"desc"`
	}
	// /api/user/groups is a public legacy route in New API. It does not
	// reliably apply the authenticated user's group ratio. Never use that
	// endpoint as a fallback for procurement pricing.
	if err := n.authedRequest(ctx, http.MethodGet, n.groupsPath, nil, &data); err != nil {
		return nil, err
	}
	groups := make([]Group, 0, len(data))
	seen := map[int64]string{}
	for name, item := range data {
		// "auto" has a dynamic ratio, so it cannot be used as a fixed cost or
		// optimization candidate. Its token binding is retained by ID only.
		if string(item.Ratio) == `"auto"` {
			continue
		}
		var ratio float64
		if err := json.Unmarshal(item.Ratio, &ratio); len(item.Ratio) == 0 || string(item.Ratio) == "null" || err != nil || ratio < 0 || math.IsNaN(ratio) || math.IsInf(ratio, 0) {
			return nil, fmt.Errorf("New API group %q has no valid fixed ratio", name)
		}
		id := newAPIGroupID(name)
		if other, ok := seen[id]; ok && other != name {
			return nil, errors.New("New API group ID collision")
		}
		seen[id] = name
		groups = append(groups, Group{ID: id, Name: name, Description: item.Description, RateMultiplier: ratio, Status: "active"})
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Name < groups[j].Name })
	return groups, nil
}

func (n *newAPIClient) keysPage(ctx context.Context, page, size int) ([]APIKey, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 100
	}
	var data json.RawMessage
	path := fmt.Sprintf("/api/token/?p=%d&page_size=%d", page, size)
	if err := n.authedRequest(ctx, http.MethodGet, path, nil, &data); err != nil {
		return nil, 0, err
	}
	type token struct {
		ID          int64  `json:"id"`
		Name        string `json:"name"`
		Key         string `json:"key"`
		Group       string `json:"group"`
		Status      int    `json:"status"`
		CreatedTime int64  `json:"created_time"`
	}
	var tokens []token
	var total int64
	if len(data) > 0 && data[0] == '[' {
		if err := json.Unmarshal(data, &tokens); err != nil {
			return nil, 0, errors.New("invalid New API token list")
		}
	} else {
		var list struct {
			Items []token `json:"items"`
			Total *int64  `json:"total"`
		}
		if err := json.Unmarshal(data, &list); err != nil || list.Total == nil || *list.Total < 0 || (list.Items == nil && *list.Total != 0) {
			return nil, 0, errors.New("invalid New API paginated token list")
		}
		tokens = list.Items
		total = *list.Total
	}
	result := make([]APIKey, 0, len(tokens))
	for _, t := range tokens {
		if t.ID <= 0 {
			return nil, 0, errors.New("New API token has an invalid ID")
		}
		status := "disabled"
		if t.Status == 1 {
			status = "active"
		}
		group := t.Group
		if group == "" {
			group = n.session.UserGroup
		}
		key := APIKey{ID: t.ID, Name: t.Name, Key: t.Key, GroupID: newAPIGroupID(group), Status: status}
		if strings.Contains(t.Key, "*") {
			key.MaskedKey = t.Key
			key.Key = ""
		}
		if t.CreatedTime > 0 {
			key.CreatedAt = time.Unix(t.CreatedTime, 0).UTC().Format(time.RFC3339)
		}
		result = append(result, key)
	}
	return result, total, nil
}

// ResolveAPIKeysForLink reveals only masked candidates for an explicit link
// operation. The service still requires an exact full-key match. Health probes
// and inventory reads never request full token secrets.
func (c *Client) ResolveAPIKeysForLink(ctx context.Context, localKey string, keys []APIKey) ([]APIKey, error) {
	if !c.IsNewAPI() {
		return keys, nil
	}
	local := strings.TrimPrefix(strings.TrimSpace(localKey), "sk-")
	result := append([]APIKey(nil), keys...)
	candidates := 0
	for i := range result {
		masked := strings.TrimPrefix(result[i].MaskedKey, "sk-")
		first, last := strings.IndexByte(masked, '*'), strings.LastIndexByte(masked, '*')
		if first < 4 || len(masked)-last-1 < 4 || !strings.HasPrefix(local, masked[:first]) || !strings.HasSuffix(local, masked[last+1:]) {
			continue
		}
		candidates++
		if candidates > 20 {
			return nil, errors.New("too many masked New API token candidates; use a more specific token")
		}
		var data struct {
			Key string `json:"key"`
		}
		if err := c.newAPI.authedRequest(ctx, http.MethodPost, fmt.Sprintf("/api/token/%d/key", result[i].ID), nil, &data); err != nil {
			return nil, fmt.Errorf("New API requires permission to reveal the candidate token for exact matching: %w", err)
		}
		if data.Key == "" || strings.Contains(data.Key, "*") {
			return nil, errors.New("New API did not reveal a complete token")
		}
		result[i].Key = data.Key
		// A mask can never serve as fallback evidence after an exact check.
		result[i].MaskedKey = ""
	}
	for i := range result {
		result[i].MaskedKey = ""
		result[i].KeyPrefix = ""
	}
	return result, nil
}

// Never return an incomplete inventory: linking and group synchronization must
// not interpret a missing page as a deleted token.
func (n *newAPIClient) allKeys(ctx context.Context) ([]APIKey, error) {
	const pageSize = 100
	all := make([]APIKey, 0)
	seen := map[int64]bool{}
	for page := 1; page <= 100; page++ {
		keys, total, err := n.keysPage(ctx, page, pageSize)
		if err != nil {
			return nil, err
		}
		for _, key := range keys {
			if seen[key.ID] {
				return nil, errors.New("New API returned duplicate tokens across pages; inventory incomplete")
			}
			seen[key.ID] = true
			all = append(all, key)
		}
		if total > 0 && int64(len(all)) >= total {
			return all, nil
		}
		if len(keys) < pageSize {
			if total > int64(len(all)) {
				return nil, errors.New("New API token pagination ended before total; inventory incomplete")
			}
			return all, nil
		}
	}
	return nil, errors.New("New API token inventory exceeds 100 pages; inventory incomplete")
}

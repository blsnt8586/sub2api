package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

type OpenAIVideoEndpoint string

const (
	OpenAIVideoEndpointCreate  OpenAIVideoEndpoint = "create"
	OpenAIVideoEndpointStatus  OpenAIVideoEndpoint = "status"
	OpenAIVideoEndpointContent OpenAIVideoEndpoint = "content"
	// OpenAIVideoEndpointDelete permanently removes an OpenAI video task and
	// its stored assets.  This follows the official DELETE /videos/{id}
	// contract; it is intentionally distinct from Canvas' POST cancel route.
	OpenAIVideoEndpointDelete OpenAIVideoEndpoint = "delete"

	openAIPlatformVideosURL = "https://api.openai.com/v1/videos"
)

func (e OpenAIVideoEndpoint) IsCreate() bool { return e == OpenAIVideoEndpointCreate }

func (e OpenAIVideoEndpoint) IsDelete() bool { return e == OpenAIVideoEndpointDelete }

type OpenAIVideoRequestInfo struct {
	Model             string
	Prompt            string
	Size              string
	Seconds           int
	ContentType       string
	HasInputReference bool
}

func ParseOpenAIVideoRequest(contentType string, body []byte) OpenAIVideoRequestInfo {
	info := OpenAIVideoRequestInfo{ContentType: strings.TrimSpace(contentType)}
	if gjson.ValidBytes(body) {
		info.Model = strings.TrimSpace(gjson.GetBytes(body, "model").String())
		info.Prompt = strings.TrimSpace(gjson.GetBytes(body, "prompt").String())
		info.Size = strings.TrimSpace(gjson.GetBytes(body, "size").String())
		info.Seconds = openAIVideoSeconds(gjson.GetBytes(body, "seconds"))
		if info.Seconds <= 0 {
			info.Seconds = openAIVideoSeconds(gjson.GetBytes(body, "duration"))
		}
		info.HasInputReference = gjson.GetBytes(body, "input_reference").Exists()
		return info
	}
	mediaType, params, err := mime.ParseMediaType(info.ContentType)
	if err != nil || !strings.EqualFold(mediaType, "multipart/form-data") {
		return info
	}
	reader := multipart.NewReader(bytes.NewReader(body), params["boundary"])
	for {
		part, err := reader.NextPart()
		if err != nil {
			break
		}
		name := strings.TrimSpace(part.FormName())
		if name == "input_reference" {
			info.HasInputReference = true
		}
		if part.FileName() != "" {
			_ = part.Close()
			continue
		}
		value, _ := io.ReadAll(io.LimitReader(part, 1<<20))
		_ = part.Close()
		switch name {
		case "model":
			info.Model = strings.TrimSpace(string(value))
		case "prompt":
			info.Prompt = strings.TrimSpace(string(value))
		case "size":
			info.Size = strings.TrimSpace(string(value))
		case "seconds", "duration":
			info.Seconds, _ = strconv.Atoi(strings.TrimSpace(string(value)))
		}
	}
	return info
}

func openAIVideoSeconds(value gjson.Result) int {
	if !value.Exists() {
		return 0
	}
	if value.Type == gjson.Number {
		return int(value.Int())
	}
	seconds, _ := strconv.Atoi(strings.TrimSpace(value.String()))
	return seconds
}

func normalizeOpenAIVideoCreateBody(body []byte, contentType, model string) ([]byte, string, error) {
	mediaType, _, mediaErr := mime.ParseMediaType(contentType)
	if mediaErr == nil && strings.EqualFold(mediaType, "multipart/form-data") {
		return normalizeOpenAIVideoMultipartCreateBody(body, contentType, model)
	}
	out, contentType, err := rewriteOpenAIImagesModel(body, contentType, model)
	if err != nil || !gjson.ValidBytes(out) {
		return out, contentType, err
	}
	if !gjson.GetBytes(out, "seconds").Exists() && gjson.GetBytes(out, "duration").Exists() {
		seconds := strings.TrimSpace(gjson.GetBytes(out, "duration").String())
		out, err = sjson.SetBytes(out, "seconds", seconds)
		if err != nil {
			return nil, "", err
		}
	}
	for _, field := range []string{"duration", "resolution", "fps", "generate_audio"} {
		out, err = sjson.DeleteBytes(out, field)
		if err != nil {
			return nil, "", err
		}
	}
	return out, contentType, nil
}

func normalizeOpenAIVideoMultipartCreateBody(body []byte, contentType, model string) ([]byte, string, error) {
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return nil, "", fmt.Errorf("parse multipart content-type: %w", err)
	}
	boundary := strings.TrimSpace(params["boundary"])
	if boundary == "" {
		return nil, "", fmt.Errorf("multipart boundary is required")
	}

	hasSeconds := false
	scan := multipart.NewReader(bytes.NewReader(body), boundary)
	for {
		part, nextErr := scan.NextPart()
		if nextErr == io.EOF {
			break
		}
		if nextErr != nil {
			return nil, "", fmt.Errorf("read multipart body: %w", nextErr)
		}
		if part.FileName() == "" && strings.TrimSpace(part.FormName()) == "seconds" {
			hasSeconds = true
		}
		_, _ = io.Copy(io.Discard, part)
		_ = part.Close()
	}

	reader := multipart.NewReader(bytes.NewReader(body), boundary)
	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)
	modelWritten := false
	for {
		part, nextErr := reader.NextPart()
		if nextErr == io.EOF {
			break
		}
		if nextErr != nil {
			return nil, "", fmt.Errorf("read multipart body: %w", nextErr)
		}
		name := strings.TrimSpace(part.FormName())
		isFile := part.FileName() != ""
		switch name {
		case "duration":
			if !hasSeconds && !isFile {
				value, readErr := io.ReadAll(part)
				_ = part.Close()
				if readErr != nil {
					return nil, "", fmt.Errorf("read multipart duration: %w", readErr)
				}
				if err := writer.WriteField("seconds", strings.TrimSpace(string(value))); err != nil {
					return nil, "", fmt.Errorf("rewrite multipart duration: %w", err)
				}
				continue
			}
			_ = part.Close()
			continue
		case "resolution", "fps", "generate_audio":
			_ = part.Close()
			continue
		}

		target, createErr := writer.CreatePart(cloneMultipartHeader(part.Header))
		if createErr != nil {
			_ = part.Close()
			return nil, "", fmt.Errorf("create multipart part: %w", createErr)
		}
		if name == "model" && !isFile {
			_, err = target.Write([]byte(strings.TrimSpace(model)))
			modelWritten = true
		} else {
			_, err = io.Copy(target, part)
		}
		_ = part.Close()
		if err != nil {
			return nil, "", fmt.Errorf("copy multipart part: %w", err)
		}
	}
	if !modelWritten {
		if err := writer.WriteField("model", strings.TrimSpace(model)); err != nil {
			return nil, "", fmt.Errorf("append multipart model field: %w", err)
		}
	}
	if err := writer.Close(); err != nil {
		return nil, "", fmt.Errorf("finalize multipart body: %w", err)
	}
	return buffer.Bytes(), writer.FormDataContentType(), nil
}

func (s *OpenAIGatewayService) ForwardOpenAIVideo(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	endpoint OpenAIVideoEndpoint,
	requestID string,
	body []byte,
	contentType string,
	channelMappedModel string,
) (*OpenAIForwardResult, error) {
	start := time.Now()
	if account == nil || account.Platform != PlatformOpenAI || account.Type != AccountTypeAPIKey {
		return nil, fmt.Errorf("OpenAI Videos API requires an OpenAI API-key account")
	}
	info := ParseOpenAIVideoRequest(contentType, body)
	requestModel := info.Model
	if mapped := strings.TrimSpace(channelMappedModel); mapped != "" {
		requestModel = mapped
	}
	upstreamModel := account.GetMappedModel(requestModel)
	if endpoint.IsCreate() {
		if strings.TrimSpace(upstreamModel) == "" {
			return nil, fmt.Errorf("model is required")
		}
		var err error
		body, contentType, err = normalizeOpenAIVideoCreateBody(body, contentType, upstreamModel)
		if err != nil {
			return nil, err
		}
		info = ParseOpenAIVideoRequest(contentType, body)
		SetOpsUpstreamModel(c, upstreamModel)
	}

	upstreamCtx, release := detachUpstreamContext(ctx)
	defer release()
	token, _, err := s.GetAccessToken(upstreamCtx, account)
	if err != nil {
		return nil, err
	}
	upstreamReq, err := s.buildOpenAIVideoRequest(upstreamCtx, c, account, endpoint, requestID, body, contentType, token)
	if err != nil {
		return nil, err
	}
	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	upstreamStart := time.Now()
	resp, err := s.doOpenAIUpstream(upstreamReq, proxyURL, account)
	SetOpsLatencyMs(c, OpsUpstreamLatencyMsKey, time.Since(upstreamStart).Milliseconds())
	if err != nil {
		return nil, s.handleOpenAIUpstreamTransportError(ctx, c, account, err, false)
	}
	defer func() { _ = resp.Body.Close() }()
	requestHeaderID := strings.TrimSpace(resp.Header.Get("x-request-id"))
	if resp.StatusCode >= http.StatusBadRequest {
		respBody := s.readUpstreamErrorBody(resp)
		message := sanitizeUpstreamErrorMessage(extractUpstreamErrorMessage(respBody))
		if s.shouldFailoverOpenAIUpstreamResponse(resp.StatusCode, message, respBody) {
			return nil, newOpenAIUpstreamFailoverError(resp.StatusCode, resp.Header, respBody, message, false)
		}
		writeGrokMediaResponse(c, resp, respBody, s.responseHeaderFilter)
		return nil, fmt.Errorf("OpenAI video upstream returned status %d", resp.StatusCode)
	}

	if endpoint == OpenAIVideoEndpointContent {
		if err := writeGrokMediaContentResponse(c, resp); err != nil {
			return nil, err
		}
		return &OpenAIForwardResult{
			RequestID: requestHeaderID, ResponseID: requestID,
			ResponseHeaders: resp.Header.Clone(), Duration: time.Since(start), VideoCount: 1,
		}, nil
	}

	var respBody []byte
	if resp.Body != nil {
		respBody, err = ReadUpstreamResponseBody(resp.Body, s.cfg, c, openAITooLargeError)
		if err != nil {
			return nil, err
		}
	}
	responseID := strings.TrimSpace(gjson.GetBytes(respBody, "id").String())
	if responseID == "" {
		responseID = strings.TrimSpace(requestID)
	}
	if endpoint.IsCreate() && responseID == "" {
		return nil, &UpstreamFailoverError{StatusCode: http.StatusBadGateway, ResponseBody: respBody, ResponseHeaders: resp.Header.Clone()}
	}
	result := &OpenAIForwardResult{
		RequestID: requestHeaderID, ResponseID: responseID,
		Model: requestModel, BillingModel: requestModel, UpstreamModel: upstreamModel,
		ResponseHeaders: resp.Header.Clone(), ResponseStatusCode: resp.StatusCode,
		ResponseContentType: strings.TrimSpace(resp.Header.Get("Content-Type")), ResponseBody: respBody,
		Duration:        time.Since(start),
		VideoResolution: openAIVideoResolutionFromSize(info.Size), VideoDurationSeconds: info.Seconds,
	}
	if endpoint == OpenAIVideoEndpointStatus {
		result.Model = strings.TrimSpace(gjson.GetBytes(respBody, "model").String())
		result.BillingModel = result.Model
		result.UpstreamModel = result.Model
		if size := strings.TrimSpace(gjson.GetBytes(respBody, "size").String()); size != "" {
			result.VideoResolution = openAIVideoResolutionFromSize(size)
		}
		if seconds := openAIVideoSeconds(gjson.GetBytes(respBody, "seconds")); seconds > 0 {
			result.VideoDurationSeconds = seconds
		}
		if strings.EqualFold(strings.TrimSpace(gjson.GetBytes(respBody, "status").String()), "completed") {
			result.VideoCount = 1
		}
	}
	return result, nil
}

// CommitOpenAIVideoResponse writes a buffered JSON create/status response only
// after the handler has persisted task affinity and billing metadata.
func (s *OpenAIGatewayService) CommitOpenAIVideoResponse(c *gin.Context, result *OpenAIForwardResult) {
	if c == nil || result == nil || result.ResponseStatusCode <= 0 {
		return
	}
	writeOpenAIPassthroughResponseHeaders(c.Writer.Header(), result.ResponseHeaders, s.responseHeaderFilter)
	contentType := strings.TrimSpace(result.ResponseContentType)
	if contentType == "" {
		contentType = "application/json"
	}
	c.Data(result.ResponseStatusCode, contentType, result.ResponseBody)
}

func (s *OpenAIGatewayService) buildOpenAIVideoRequest(ctx context.Context, c *gin.Context, account *Account, endpoint OpenAIVideoEndpoint, requestID string, body []byte, contentType, token string) (*http.Request, error) {
	baseURL := strings.TrimSpace(account.GetOpenAIBaseURL())
	targetURL := openAIPlatformVideosURL
	if baseURL != "" {
		validated, err := s.validateUpstreamBaseURL(baseURL)
		if err != nil {
			return nil, err
		}
		targetURL = buildOpenAIEndpointURL(validated, "/v1/videos")
	}
	method := http.MethodPost
	if endpoint != OpenAIVideoEndpointCreate {
		method = http.MethodGet
		if endpoint.IsDelete() {
			method = http.MethodDelete
		}
		targetURL = strings.TrimRight(targetURL, "/") + "/" + requestID
		if endpoint == OpenAIVideoEndpointContent {
			targetURL += "/content"
		}
	}
	var reader io.Reader
	if endpoint == OpenAIVideoEndpointCreate {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(WithHTTPUpstreamProfile(ctx, HTTPUpstreamProfileOpenAI), method, targetURL, reader)
	if err != nil {
		return nil, err
	}
	headers, err := s.buildOpenAIAuthenticationHeaders(ctx, account, token)
	if err != nil {
		return nil, err
	}
	for key, values := range headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	if endpoint == OpenAIVideoEndpointCreate && strings.TrimSpace(contentType) != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if c != nil {
		if rangeHeader := strings.TrimSpace(c.GetHeader("Range")); rangeHeader != "" && endpoint == OpenAIVideoEndpointContent {
			req.Header.Set("Range", rangeHeader)
		}
	}
	req.Header.Set("Accept", "application/json")
	if endpoint == OpenAIVideoEndpointContent {
		req.Header.Set("Accept", "*/*")
	}
	if userAgent := account.GetOpenAIUserAgent(); userAgent != "" {
		req.Header.Set("User-Agent", userAgent)
	}
	account.ApplyHeaderOverrides(req.Header)
	return req, nil
}

func openAIVideoResolutionFromSize(size string) string {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(size)), "x")
	if len(parts) != 2 {
		return ""
	}
	w, _ := strconv.Atoi(parts[0])
	h, _ := strconv.Atoi(parts[1])
	shortEdge := w
	if h < shortEdge {
		shortEdge = h
	}
	switch {
	case shortEdge >= 1080:
		return VideoBillingResolution1080P
	case shortEdge >= 1024:
		return VideoBillingResolution1024P
	case shortEdge >= 720:
		return VideoBillingResolution720P
	default:
		return VideoBillingResolution480P
	}
}

type OpenAIVideoPendingBilling struct {
	Model                string `json:"model"`
	BillingModel         string `json:"billing_model,omitempty"`
	UpstreamModel        string `json:"upstream_model,omitempty"`
	VideoResolution      string `json:"video_resolution,omitempty"`
	VideoDurationSeconds int    `json:"video_duration_seconds,omitempty"`
	OriginalModel        string `json:"original_model,omitempty"`
	CreatedAt            string `json:"created_at,omitempty"`
}

func OpenAIVideoRequestSessionHash(requestID string, userID, apiKeyID int64) string {
	if strings.TrimSpace(requestID) == "" || userID <= 0 || apiKeyID <= 0 {
		return ""
	}
	return "openai-video:" + DeriveSessionHashFromSeed(fmt.Sprintf("%d:%d:%s", userID, apiKeyID, requestID))
}

func (s *OpenAIGatewayService) BindOpenAIVideoRequestAccount(ctx context.Context, groupID *int64, requestID string, userID, apiKeyID, accountID int64) error {
	if s == nil || s.cache == nil || accountID <= 0 {
		return fmt.Errorf("OpenAI video request binding is unavailable")
	}
	cacheKey := s.openAISessionCacheKey(OpenAIVideoRequestSessionHash(requestID, userID, apiKeyID))
	if cacheKey == "" {
		return fmt.Errorf("OpenAI video request binding is invalid")
	}
	return s.cache.SetSessionAccountID(ctx, derefGroupID(groupID), cacheKey, accountID, openAIVideoPendingBillingTTL(s.cfg))
}

func (s *OpenAIGatewayService) ResolveOpenAIVideoRequestAccount(ctx context.Context, groupID *int64, requestID string, userID, apiKeyID int64) (int64, error) {
	if s == nil || s.cache == nil {
		return 0, fmt.Errorf("OpenAI video request binding is unavailable")
	}
	cacheKey := s.openAISessionCacheKey(OpenAIVideoRequestSessionHash(requestID, userID, apiKeyID))
	if cacheKey == "" {
		return 0, fmt.Errorf("OpenAI video request binding is invalid")
	}
	return s.cache.GetSessionAccountID(ctx, derefGroupID(groupID), cacheKey)
}

// SelectBoundOpenAIVideoAccount admits only the account that created an async
// video job. Status and content endpoints cannot fail over because upstream job
// IDs are scoped to the creating API key.
func (s *OpenAIGatewayService) SelectBoundOpenAIVideoAccount(ctx context.Context, groupID *int64, accountID int64) (*AccountSelectionResult, error) {
	if s == nil || accountID <= 0 {
		return nil, ErrNoAvailableAccounts
	}
	ctx = s.withOpenAIQuotaAutoPauseContext(ctx)
	ctx = s.withOpenAIGroupPrivacyRequirement(ctx, groupID)
	account, err := s.getSchedulableAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	account = s.recheckSelectedOpenAIAccountFromDB(ctx, account, groupID, PlatformOpenAI, "", false, OpenAIEndpointCapabilityVideos)
	if account == nil {
		return nil, ErrNoAvailableAccounts
	}
	acquired, acquireErr := s.tryAcquireAccountSlot(ctx, account.ID, account.Concurrency)
	if acquireErr == nil && acquired != nil && acquired.Acquired {
		return &AccountSelectionResult{Account: account, Acquired: true, ReleaseFunc: acquired.ReleaseFunc}, nil
	}
	if s.concurrencyService != nil {
		cfg := s.schedulingConfig()
		return &AccountSelectionResult{
			Account: account,
			WaitPlan: &AccountWaitPlan{
				AccountID: account.ID, MaxConcurrency: account.Concurrency,
				Timeout: cfg.StickySessionWaitTimeout, MaxWaiting: cfg.StickySessionMaxWaiting,
			},
		}, nil
	}
	return nil, ErrNoAvailableAccounts
}

// SelectBoundOpenAIVideoAccountForControl resolves the account that created a
// video without consuming its generation concurrency slot.  Control-plane
// operations such as DELETE must remain usable when that slot is already full;
// the account/platform checks are still performed so a request cannot be sent
// through an unrelated account.
func (s *OpenAIGatewayService) SelectBoundOpenAIVideoAccountForControl(ctx context.Context, groupID *int64, accountID int64) (*AccountSelectionResult, error) {
	if s == nil || accountID <= 0 {
		return nil, ErrNoAvailableAccounts
	}
	var (
		account *Account
		err     error
	)
	// Use the durable repository snapshot when available. The scheduler cache
	// intentionally omits blocked accounts, but a control-plane delete must be
	// able to reach the account that owns an in-flight task even in that state.
	if s.accountRepo != nil {
		account, err = s.accountRepo.GetByID(ctx, accountID)
	} else if s.schedulerSnapshot != nil {
		account, err = s.schedulerSnapshot.GetAccount(ctx, accountID)
	} else {
		return nil, ErrNoAvailableAccounts
	}
	if err != nil || account == nil || !account.IsActive() || account.Platform != PlatformOpenAI || account.Type != AccountTypeAPIKey || !account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityVideos) || !s.openAIAccountMatchesSchedulingGroup(account, groupID) {
		return nil, ErrNoAvailableAccounts
	}
	return &AccountSelectionResult{Account: account}, nil
}

func openAIVideoPendingBillingKey(requestID string, userID, apiKeyID int64) string {
	if strings.TrimSpace(requestID) == "" || userID <= 0 || apiKeyID <= 0 {
		return ""
	}
	return fmt.Sprintf("openai-video:%d:%d:%s", userID, apiKeyID, requestID)
}

func openAIVideoPendingBillingTTL(_ *config.Config) time.Duration { return 24 * time.Hour }

func (s *OpenAIGatewayService) StoreOpenAIVideoPendingBilling(ctx context.Context, requestID string, userID, apiKeyID int64, pending OpenAIVideoPendingBilling) error {
	key := openAIVideoPendingBillingKey(requestID, userID, apiKeyID)
	if s == nil || s.cache == nil || key == "" {
		return fmt.Errorf("OpenAI video pending billing cache is unavailable")
	}
	if strings.TrimSpace(pending.CreatedAt) == "" {
		pending.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	payload, err := json.Marshal(pending)
	if err != nil {
		return err
	}
	return s.cache.SetGrokVideoPendingBilling(ctx, key, payload, openAIVideoPendingBillingTTL(s.cfg))
}

func (s *OpenAIGatewayService) LoadOpenAIVideoPendingBilling(ctx context.Context, requestID string, userID, apiKeyID int64) (*OpenAIVideoPendingBilling, error) {
	key := openAIVideoPendingBillingKey(requestID, userID, apiKeyID)
	if s == nil || s.cache == nil || key == "" {
		return nil, fmt.Errorf("OpenAI video pending billing cache is unavailable")
	}
	payload, err := s.cache.GetGrokVideoPendingBilling(ctx, key)
	if err != nil || len(payload) == 0 {
		return nil, err
	}
	var pending OpenAIVideoPendingBilling
	if err := json.Unmarshal(payload, &pending); err != nil {
		return nil, err
	}
	return &pending, nil
}

func (s *OpenAIGatewayService) ClaimOpenAIVideoBilling(ctx context.Context, requestID string, userID, apiKeyID int64) (bool, error) {
	key := openAIVideoPendingBillingKey(requestID, userID, apiKeyID)
	if s == nil || s.cache == nil || key == "" {
		return false, fmt.Errorf("OpenAI video billing claim cache is unavailable")
	}
	return s.cache.ClaimGrokVideoBilled(ctx, key, 48*time.Hour)
}

func (s *OpenAIGatewayService) ReleaseOpenAIVideoBilling(ctx context.Context, requestID string, userID, apiKeyID int64) error {
	key := openAIVideoPendingBillingKey(requestID, userID, apiKeyID)
	if s == nil || s.cache == nil || key == "" {
		return fmt.Errorf("OpenAI video billing claim cache is unavailable")
	}
	return s.cache.ReleaseGrokVideoBilled(ctx, key)
}

func StableOpenAIVideoBillingRequestID(requestID string) string {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" || strings.HasPrefix(requestID, "openai-video:") {
		return requestID
	}
	return "openai-video:" + requestID
}

func NormalizeOpenAIVideoBillingDuration(seconds int) int {
	if seconds <= 0 {
		return 8
	}
	if seconds > 20 {
		return 20
	}
	return seconds
}

package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func (h *OpenAIGatewayHandler) OpenAIVideoCreation(c *gin.Context) {
	h.handleOpenAIVideo(c, service.OpenAIVideoEndpointCreate, "")
}

func (h *OpenAIGatewayHandler) OpenAIVideoStatus(c *gin.Context) {
	h.handleOpenAIVideo(c, service.OpenAIVideoEndpointStatus, c.Param("request_id"))
}

func (h *OpenAIGatewayHandler) OpenAIVideoContent(c *gin.Context) {
	h.handleOpenAIVideo(c, service.OpenAIVideoEndpointContent, c.Param("request_id"))
}

// OpenAIVideoDelete implements the official DELETE /videos/{video_id}
// operation.  The request is still resolved through the creation-time account
// binding so a video ID can never be deleted through another user's key or a
// different upstream account.
func (h *OpenAIGatewayHandler) OpenAIVideoDelete(c *gin.Context) {
	h.handleOpenAIVideo(c, service.OpenAIVideoEndpointDelete, c.Param("request_id"))
}

func (h *OpenAIGatewayHandler) handleOpenAIVideo(c *gin.Context, endpoint service.OpenAIVideoEndpoint, requestID string) {
	streamStarted := false
	defer h.recoverResponsesPanic(c, &streamStarted)
	if !h.ensureResponsesDependencies(c, nil) {
		return
	}
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusInternalServerError, "api_error", "User context not found")
		return
	}
	reqLog := requestLogger(c, "handler.openai_gateway.openai_video",
		zap.Int64("user_id", subject.UserID), zap.Int64("api_key_id", apiKey.ID),
		zap.Any("group_id", apiKey.GroupID), zap.String("endpoint", string(endpoint)))

	body := []byte(nil)
	contentType := c.GetHeader("Content-Type")
	if endpoint.IsCreate() {
		var err error
		body, err = pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
		if err != nil {
			if maxErr, ok := extractMaxBytesError(err); ok {
				h.errorResponse(c, http.StatusRequestEntityTooLarge, "invalid_request_error", buildBodyTooLargeMessage(maxErr.Limit))
				return
			}
			h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to read request body")
			return
		}
		if len(body) == 0 {
			h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Request body is empty")
			return
		}
	} else if strings.TrimSpace(requestID) == "" {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "request_id is required")
		return
	}

	requestInfo := service.ParseOpenAIVideoRequest(contentType, body)
	requestModel := requestInfo.Model
	if endpoint.IsCreate() && strings.TrimSpace(requestModel) == "" {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "model is required")
		return
	}
	setOpsRequestContext(c, requestModel, false)
	setOpsEndpointContext(c, "", int16(service.RequestTypeSync))
	if endpoint.IsCreate() {
		if !service.GroupAllowsImageGeneration(apiKey.Group) {
			h.errorResponse(c, http.StatusForbidden, "permission_error", service.ImageGenerationPermissionMessage())
			return
		}
		if decision := h.checkSecurityAudit(c, reqLog, apiKey, subject, service.ContentModerationProtocolOpenAIImages, requestModel, body); decision != nil && !decision.AllowNextStage {
			h.openAISecurityAuditError(c, decision)
			return
		}
		releaseImage, acquired := h.acquireImageGenerationSlot(c, streamStarted)
		if !acquired {
			return
		}
		if releaseImage != nil {
			defer releaseImage()
		}
	}

	if h.errorPassthroughService != nil {
		service.BindErrorPassthroughService(c, h.errorPassthroughService)
	}
	subscription, _ := middleware2.GetSubscriptionFromContext(c)
	// DELETE is a control-plane cleanup operation. It must remain available
	// when a user's generation quota or normal concurrency budget is exhausted;
	// otherwise an in-flight upstream job can keep running (and accruing cost)
	// precisely when the user most needs to stop it.
	if !endpoint.IsDelete() {
		releaseUser, acquired := h.acquireResponsesUserSlot(c, subject.UserID, subject.Concurrency, false, &streamStarted, reqLog)
		if !acquired {
			return
		}
		if releaseUser != nil {
			defer releaseUser()
		}
		if err := h.billingCacheService.CheckBillingEligibility(c.Request.Context(), apiKey.User, apiKey, apiKey.Group, subscription, service.QuotaPlatform(c.Request.Context(), apiKey)); err != nil {
			status, code, message, retryAfter := billingErrorDetails(err)
			if retryAfter > 0 {
				c.Header("Retry-After", strconv.Itoa(retryAfter))
			}
			h.errorResponse(c, status, code, message)
			return
		}
	}

	requestCtx := service.WithOpenAIProfitControlSuppressed(c.Request.Context())
	sessionHash := h.gatewayService.GenerateExplicitSessionHash(c, body)
	boundAccountID := int64(0)
	if !endpoint.IsCreate() {
		sessionHash = service.OpenAIVideoRequestSessionHash(requestID, subject.UserID, apiKey.ID)
		var err error
		boundAccountID, err = h.gatewayService.ResolveOpenAIVideoRequestAccount(requestCtx, apiKey.GroupID, requestID, subject.UserID, apiKey.ID)
		if err != nil || boundAccountID <= 0 {
			h.errorResponse(c, http.StatusNotFound, "not_found_error", "Video request not found")
			return
		}
	}

	channelMapping, _ := h.gatewayService.ResolveChannelMappingAndRestrict(requestCtx, apiKey.GroupID, requestModel)
	routingModel := requestModel
	if mapped := strings.TrimSpace(channelMapping.MappedModel); mapped != "" {
		routingModel = mapped
	}
	failedAccountIDs := make(map[int64]struct{})
	maxSwitches := h.maxAccountSwitches
	if maxSwitches <= 0 {
		maxSwitches = 3
	}
	switchCount := 0
	createdAt := time.Now().UTC().Format(time.RFC3339Nano)

	for {
		var selection *service.AccountSelectionResult
		var err error
		if boundAccountID > 0 {
			if endpoint.IsDelete() {
				selection, err = h.gatewayService.SelectBoundOpenAIVideoAccountForControl(requestCtx, apiKey.GroupID, boundAccountID)
			} else {
				selection, err = h.gatewayService.SelectBoundOpenAIVideoAccount(requestCtx, apiKey.GroupID, boundAccountID)
			}
		} else {
			selection, _, err = h.gatewayService.SelectAccountWithSchedulerForCapability(
				requestCtx, apiKey.GroupID, "", sessionHash, routingModel, failedAccountIDs,
				service.OpenAIUpstreamTransportHTTPSSE, service.OpenAIEndpointCapabilityVideos,
				false, false, false, service.PlatformOpenAI,
			)
		}
		if err != nil || selection == nil || selection.Account == nil {
			markOpsRoutingCapacityLimited(c)
			h.errorResponse(c, http.StatusServiceUnavailable, "openai_video_no_eligible_account", "No eligible OpenAI video accounts")
			return
		}
		account := selection.Account
		setOpsSelectedAccount(c, account.ID, account.Platform)
		var releaseAccount func()
		if !endpoint.IsDelete() {
			var slotResult openAISlotAcquireResult
			releaseAccount, slotResult = h.acquireResponsesAccountSlot(c, apiKey.GroupID, sessionHash, selection, false, &streamStarted, reqLog)
			if slotResult != openAISlotAcquireOK {
				return
			}
		}
		writerSize := c.Writer.Size()
		result, forwardErr := func() (*service.OpenAIForwardResult, error) {
			defer func() {
				if releaseAccount != nil {
					releaseAccount()
				}
			}()
			return h.gatewayService.ForwardOpenAIVideo(requestCtx, c, account, endpoint, requestID, body, contentType, channelMapping.MappedModel)
		}()
		if forwardErr != nil {
			if service.IsResponseCommitted(c) || c.Writer.Size() != writerSize {
				return
			}
			var failoverErr *service.UpstreamFailoverError
			if !errors.As(forwardErr, &failoverErr) || !endpoint.IsCreate() || !failoverErr.ShouldRetryNextAccount() {
				h.errorResponse(c, http.StatusBadGateway, "upstream_error", "Upstream request failed")
				return
			}
			h.gatewayService.ReportOpenAIAccountScheduleResult(account, account.GetMappedModel(routingModel), false, nil)
			failedAccountIDs[account.ID] = struct{}{}
			if switchCount >= maxSwitches {
				h.handleFailoverExhausted(c, failoverErr, false)
				return
			}
			switchCount++
			continue
		}
		h.gatewayService.ReportOpenAIAccountScheduleResult(account, firstNonEmptyString(result.UpstreamModel, account.GetMappedModel(routingModel)), true, nil)

		if endpoint.IsCreate() && result != nil && strings.TrimSpace(result.ResponseID) != "" {
			if err := h.gatewayService.BindOpenAIVideoRequestAccount(requestCtx, apiKey.GroupID, result.ResponseID, subject.UserID, apiKey.ID, account.ID); err != nil {
				reqLog.Error("openai_video.bind_request_failed", zap.String("request_id", result.ResponseID), zap.Error(err))
				h.errorResponse(c, http.StatusBadGateway, "upstream_state_error", "Failed to persist video request state")
				return
			}
			pending := service.OpenAIVideoPendingBilling{
				Model: requestModel, BillingModel: firstNonEmptyString(result.BillingModel, requestModel),
				UpstreamModel: result.UpstreamModel, VideoResolution: result.VideoResolution,
				VideoDurationSeconds: result.VideoDurationSeconds,
				OriginalModel:        clientRequestedModel(c, requestModel), CreatedAt: createdAt,
			}
			if err := h.gatewayService.StoreOpenAIVideoPendingBilling(requestCtx, result.ResponseID, subject.UserID, apiKey.ID, pending); err != nil {
				reqLog.Error("openai_video.store_pending_billing_failed", zap.String("request_id", result.ResponseID), zap.Error(err))
				h.errorResponse(c, http.StatusBadGateway, "upstream_state_error", "Failed to persist video billing state")
				return
			}
		}
		// Only status/content responses can contain a completed video result and
		// therefore trigger deferred usage billing.  DELETE is a control-plane
		// operation and must never claim or record media usage.
		if endpoint == service.OpenAIVideoEndpointStatus || endpoint == service.OpenAIVideoEndpointContent {
			if billed := prepareOpenAIVideoCompletionBilling(requestCtx, h, reqLog, apiKey, subject, requestID, result); billed != nil {
				recordOpenAIVideoUsage(c, h, reqLog, apiKey, subject, subscription, account, billed, requestID)
			}
		}
		h.gatewayService.CommitOpenAIVideoResponse(c, result)
		return
	}
}

func prepareOpenAIVideoCompletionBilling(ctx context.Context, h *OpenAIGatewayHandler, reqLog *zap.Logger, apiKey *service.APIKey, subject middleware2.AuthSubject, requestID string, result *service.OpenAIForwardResult) *service.OpenAIForwardResult {
	if h == nil || h.gatewayService == nil || apiKey == nil || result == nil || result.VideoCount <= 0 {
		return nil
	}
	pending, err := h.gatewayService.LoadOpenAIVideoPendingBilling(ctx, requestID, subject.UserID, apiKey.ID)
	if err != nil || pending == nil {
		reqLog.Error("openai_video.pending_billing_missing", zap.String("request_id", requestID), zap.Error(err))
		return nil
	}
	claimed, err := h.gatewayService.ClaimOpenAIVideoBilling(ctx, requestID, subject.UserID, apiKey.ID)
	if err != nil || !claimed {
		return nil
	}
	merged := *result
	merged.Model = firstNonEmptyString(pending.Model, pending.OriginalModel, merged.Model, pending.BillingModel)
	merged.BillingModel = firstNonEmptyString(pending.BillingModel, pending.Model, merged.BillingModel, merged.Model)
	merged.UpstreamModel = firstNonEmptyString(pending.UpstreamModel, merged.UpstreamModel)
	merged.ResponseID = firstNonEmptyString(merged.ResponseID, requestID)
	merged.RequestID = service.StableOpenAIVideoBillingRequestID(requestID)
	merged.VideoCount = 1
	merged.ImageCount = 0
	merged.VideoResolution = firstNonEmptyString(merged.VideoResolution, pending.VideoResolution)
	merged.VideoDurationSeconds = service.NormalizeOpenAIVideoBillingDuration(maxInt(merged.VideoDurationSeconds, pending.VideoDurationSeconds))
	if elapsed := service.GrokVideoE2EDuration(pending.CreatedAt, time.Now()); elapsed > 0 {
		merged.Duration = elapsed
	}
	return &merged
}

func recordOpenAIVideoUsage(c *gin.Context, h *OpenAIGatewayHandler, reqLog *zap.Logger, apiKey *service.APIKey, subject middleware2.AuthSubject, subscription *service.UserSubscription, account *service.Account, result *service.OpenAIForwardResult, requestID string) {
	result.RequestID = service.StableOpenAIVideoBillingRequestID(requestID)
	h.submitOpenAIUsageRecordTask(c.Request.Context(), result, func(ctx context.Context) {
		err := h.gatewayService.RecordUsage(ctx, &service.OpenAIRecordUsageInput{
			Result: result, APIKey: apiKey, User: apiKey.User, Account: account, Subscription: subscription,
			InboundEndpoint: GetInboundEndpoint(c), UpstreamEndpoint: GetUpstreamEndpoint(c, account.Platform),
			UserAgent: c.GetHeader("User-Agent"), IPAddress: ip.GetClientIP(c),
			RequestPayloadHash: service.HashUsageRequestPayload([]byte(requestID)), APIKeyService: h.apiKeyService,
			QuotaPlatform: service.QuotaPlatform(c.Request.Context(), apiKey), SessionID: service.ExtractClientSessionID(c),
			ChannelUsageFields: service.ChannelUsageFields{OriginalModel: result.Model, ChannelMappedModel: result.BillingModel},
		})
		if err == nil {
			return
		}
		if releaseErr := h.gatewayService.ReleaseOpenAIVideoBilling(ctx, requestID, subject.UserID, apiKey.ID); releaseErr != nil {
			reqLog.Warn("openai_video.billing_claim_release_failed", zap.Error(releaseErr))
		}
		logger.L().Error("openai_video.record_usage_failed", zap.Error(err), zap.String("request_id", requestID))
	})
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

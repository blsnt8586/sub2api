package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// CanvasAudioCreation 处理音频任务创建：POST /v1/audio/generations。
func (h *OpenAIGatewayHandler) CanvasAudioCreation(c *gin.Context) {
	h.handleCanvasAudio(c, service.JimengAudioGeneration, "")
}

// CanvasAudioStatus 处理音频任务状态查询：GET /v1/audio/{id}。
func (h *OpenAIGatewayHandler) CanvasAudioStatus(c *gin.Context) {
	h.handleCanvasAudio(c, service.CanvasAudioStatus, c.Param("id"))
}

// CanvasAudioCancel 处理音频任务取消：POST /v1/audio/{id}/cancel。
func (h *OpenAIGatewayHandler) CanvasAudioCancel(c *gin.Context) {
	h.handleCanvasAudio(c, service.CanvasAudioCancel, c.Param("id"))
}

// handleCanvasAudio 是音频三端点的公共前置流程。
// 结构与 handleCanvasVideo 对齐：鉴权 → 读 body → 计费资格 → 转发循环。
func (h *OpenAIGatewayHandler) handleCanvasAudio(c *gin.Context, endpoint service.CanvasAudioEndpoint, taskID string) {
	streamStarted := false
	defer h.recoverResponsesPanic(c, &streamStarted)

	requestStart := time.Now()
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

	isCreate := endpoint == service.JimengAudioGeneration

	reqLog := requestLogger(
		c,
		"handler.openai_gateway.jimeng_audio",
		zap.Int64("user_id", subject.UserID),
		zap.Int64("api_key_id", apiKey.ID),
		zap.Any("group_id", apiKey.GroupID),
		zap.String("endpoint", string(endpoint)),
	)
	if !h.ensureResponsesDependencies(c, reqLog) {
		return
	}

	var body []byte
	var err error
	if isCreate {
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
	} else if strings.TrimSpace(taskID) == "" {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "id is required")
		return
	}

	contentType := c.GetHeader("Content-Type")
	requestModel := service.ExtractCanvasAudioModel(body)
	// sound-effects-v2 的 model 可省略（AIV2API schema 默认值），此处补齐默认模型，
	// 使选号与计费拿到确定的模型名。
	if isCreate && strings.TrimSpace(requestModel) == "" {
		requestModel = service.CanvasAudioDefaultModel
	}
	// 音频参数本地校验
	if isCreate {
		if verr := service.ValidateCanvasAudioRequest(body); verr != nil {
			h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", verr.Error())
			return
		}
	}

	reqLog = reqLog.With(zap.String("model", requestModel))
	setOpsRequestContext(c, requestModel, false)
	setOpsEndpointContext(c, "", int16(service.RequestTypeSync))

	if h.errorPassthroughService != nil {
		service.BindErrorPassthroughService(c, h.errorPassthroughService)
	}

	subscription, _ := middleware2.GetSubscriptionFromContext(c)
	service.SetOpsLatencyMs(c, service.OpsAuthLatencyMsKey, time.Since(requestStart).Milliseconds())

	userReleaseFunc, acquired := h.acquireResponsesUserSlot(c, subject.UserID, subject.Concurrency, false, &streamStarted, reqLog)
	if !acquired {
		return
	}
	if userReleaseFunc != nil {
		defer userReleaseFunc()
	}

	if err := h.billingCacheService.CheckBillingEligibility(c.Request.Context(), apiKey.User, apiKey, apiKey.Group, subscription, service.QuotaPlatform(c.Request.Context(), apiKey)); err != nil {
		reqLog.Info("jimeng_audio.billing_eligibility_check_failed", zap.Error(err))
		status, code, message, retryAfter := billingErrorDetails(err)
		if retryAfter > 0 {
			c.Header("Retry-After", strconv.Itoa(retryAfter))
		}
		h.errorResponse(c, status, code, message)
		return
	}

	h.runCanvasAudioForwardLoop(c, endpoint, taskID, body, contentType, requestModel, apiKey, subject, subscription, &streamStarted, reqLog)
}

// runCanvasAudioForwardLoop 走与视频一致的调度 + failover 循环。
//
// 选号必须传 service.PlatformCanvas：通用的 SelectAccountForModelWithExclusions
// 内部硬编码 PlatformOpenAI，会导致 avi2api 账号一个都命中不了。
func (h *OpenAIGatewayHandler) runCanvasAudioForwardLoop(
	c *gin.Context,
	endpoint service.CanvasAudioEndpoint,
	taskID string,
	body []byte,
	contentType string,
	requestModel string,
	apiKey *service.APIKey,
	subject middleware2.AuthSubject,
	subscription *service.UserSubscription,
	streamStarted *bool,
	reqLog *zap.Logger,
) {
	isCreate := endpoint == service.JimengAudioGeneration

	// 状态查询 / 取消复用创建任务时绑定的账号（粘性会话）。
	var sessionHash string
	if isCreate {
		sessionHash = h.gatewayService.GenerateExplicitSessionHash(c, body)
	} else {
		sessionHash = service.CanvasAudioTaskSessionHash(taskID)
	}

	requestCtx := c.Request.Context()
	failedAccountIDs := make(map[int64]struct{})
	sameAccountRetryCount := make(map[int64]int)
	profitVetoCount := 0
	var lastFailoverErr *service.UpstreamFailoverError
	switchCount := 0
	maxAccountSwitches := h.maxAccountSwitches
	if maxAccountSwitches <= 0 {
		maxAccountSwitches = 3
	}
	routingStart := time.Now()

	for {
		selection, scheduleDecision, err := h.gatewayService.SelectAccountWithSchedulerForCapability(
			requestCtx,
			apiKey.GroupID,
			"",
			sessionHash,
			requestModel,
			failedAccountIDs,
			service.OpenAIUpstreamTransportHTTPSSE,
			"",
			false,
			false,
			false,
			service.PlatformCanvas,
		)
		if err != nil {
			reqLog.Warn("jimeng_audio.account_select_failed",
				zap.Error(err),
				zap.Int("excluded_account_count", len(failedAccountIDs)),
			)
			if len(failedAccountIDs) == 0 {
				cls := classifyNoAccountErrorFromGin(c, h.gatewayService, apiKey, requestModel, requestModel, service.PlatformCanvas)
				if !cls.ModelNotFound {
					markOpsRoutingCapacityLimitedIfNoAvailable(c, err)
				}
				h.errorResponse(c, cls.Status, cls.ErrType, cls.Message)
				return
			}
			if lastFailoverErr != nil {
				h.handleFailoverExhausted(c, lastFailoverErr, false)
			} else {
				h.errorResponse(c, http.StatusBadGateway, "api_error", "Upstream request failed")
			}
			return
		}
		if selection == nil || selection.Account == nil {
			cls := classifyNoAccountErrorFromGin(c, h.gatewayService, apiKey, requestModel, requestModel, service.PlatformCanvas)
			if !cls.ModelNotFound {
				markOpsRoutingCapacityLimited(c)
			}
			h.errorResponse(c, cls.Status, cls.ErrType, cls.Message)
			return
		}

		reqLog.Debug("jimeng_audio.account_schedule_decision",
			zap.String("layer", scheduleDecision.Layer),
			zap.Bool("sticky_session_hit", scheduleDecision.StickySessionHit),
			zap.Int("candidate_count", scheduleDecision.CandidateCount),
		)

		account := selection.Account
		setOpsSelectedAccount(c, account.ID, account.Platform)

		accountReleaseFunc, slotResult := h.acquireResponsesAccountSlot(c, apiKey.GroupID, sessionHash, selection, false, streamStarted, reqLog)
		if slotResult == openAISlotAcquireProfitVetoed {
			// 媒体路径已显式豁免利润门，此分支仅防御性兜底，受否决上限约束。[CUSTOM]
			if !recordOpenAIProfitVeto(failedAccountIDs, account.ID, &profitVetoCount) {
				h.handleOpenAIProfitVetoExhausted(c, *streamStarted, reqLog, profitVetoCount)
				return
			}
			continue
		}
		if slotResult != openAISlotAcquireOK {
			return
		}

		service.SetOpsLatencyMs(c, service.OpsRoutingLatencyMsKey, time.Since(routingStart).Milliseconds())
		forwardStart := time.Now()
		writerSizeBeforeForward := c.Writer.Size()
		result, err := func() (*service.OpenAIForwardResult, error) {
			defer func() {
				if accountReleaseFunc != nil {
					accountReleaseFunc()
				}
			}()
			return h.gatewayService.ForwardCanvasAudio(requestCtx, c, account, endpoint, taskID, body, contentType)
		}()

		forwardDurationMs := time.Since(forwardStart).Milliseconds()
		upstreamLatencyMs, _ := getContextInt64(c, service.OpsUpstreamLatencyMsKey)
		responseLatencyMs := forwardDurationMs
		if upstreamLatencyMs > 0 && forwardDurationMs > upstreamLatencyMs {
			responseLatencyMs = forwardDurationMs - upstreamLatencyMs
		}
		service.SetOpsLatencyMs(c, service.OpsResponseLatencyMsKey, responseLatencyMs)

		if err != nil {
			var failoverErr *service.UpstreamFailoverError
			if errors.As(err, &failoverErr) {
				h.gatewayService.ReportOpenAIAccountScheduleResult(account.ID, requestModel, false, nil)
				if c.Writer.Size() != writerSizeBeforeForward {
					h.handleFailoverExhausted(c, failoverErr, true)
					return
				}
				if failoverErr.RetryableOnSameAccount {
					retryLimit := account.GetPoolModeRetryCount()
					if sameAccountRetryCount[account.ID] < retryLimit {
						sameAccountRetryCount[account.ID]++
						select {
						case <-requestCtx.Done():
							return
						case <-time.After(sameAccountRetryDelay):
						}
						continue
					}
				}
				h.gatewayService.RecordOpenAIAccountSwitch()
				failedAccountIDs[account.ID] = struct{}{}
				lastFailoverErr = failoverErr
				if switchCount >= maxAccountSwitches {
					h.handleFailoverExhausted(c, failoverErr, false)
					return
				}
				switchCount++
				reqLog.Warn("jimeng_audio.upstream_failover_switching",
					zap.Int64("account_id", account.ID),
					zap.Int("upstream_status", failoverErr.StatusCode),
					zap.Int("switch_count", switchCount),
				)
				continue
			}
			h.gatewayService.ReportOpenAIAccountScheduleResult(account.ID, requestModel, false, nil)
			if c.Writer.Size() == writerSizeBeforeForward {
				h.errorResponse(c, http.StatusBadGateway, "upstream_error", "Upstream request failed")
			}
			reqLog.Warn("jimeng_audio.forward_failed",
				zap.Int64("account_id", account.ID),
				zap.Error(err),
			)
			return
		}

		h.gatewayService.ReportOpenAIAccountScheduleResult(account.ID, requestModel, true, nil)
		if isCreate && strings.TrimSpace(result.ResponseID) != "" {
			if err := h.gatewayService.BindCanvasAudioTaskAccount(requestCtx, apiKey.GroupID, result.ResponseID, account.ID); err != nil {
				reqLog.Warn("jimeng_audio.bind_task_account_failed",
					zap.Int64("account_id", account.ID),
					zap.String("task_id", result.ResponseID),
					zap.Error(err),
				)
			}
		}
		// 完成时扣费：仅当本次是轮询（非 create）且 service 层判定终态成功
		// （result.VideoCount>0）时扣一次费。提交时不扣、失败/取消不扣。
		if !isCreate && result != nil && result.VideoCount > 0 {
			recordCanvasAsyncCompletionUsage(c, h, reqLog, apiKey, subject, subscription, account, result, taskID)
		}
		reqLog.Debug("jimeng_audio.request_completed",
			zap.Int64("account_id", account.ID),
			zap.Int("switch_count", switchCount),
		)
		return
	}
}

// 完成时计费统一走 recordCanvasAsyncCompletionUsage（canvas_async_billing.go）。

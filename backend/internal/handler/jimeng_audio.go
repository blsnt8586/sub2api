package handler

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// JimengAudioCreation 处理即梦音频任务创建：POST /v1/audio/generations。
func (h *OpenAIGatewayHandler) JimengAudioCreation(c *gin.Context) {
	h.handleJimengAudio(c, service.JimengAudioGeneration, "")
}

// JimengAudioStatus 处理即梦音频任务状态查询：GET /v1/audio/{id}。
func (h *OpenAIGatewayHandler) JimengAudioStatus(c *gin.Context) {
	h.handleJimengAudio(c, service.JimengAudioStatus, c.Param("id"))
}

// JimengAudioCancel 处理即梦音频任务取消：POST /v1/audio/{id}/cancel。
func (h *OpenAIGatewayHandler) JimengAudioCancel(c *gin.Context) {
	h.handleJimengAudio(c, service.JimengAudioCancel, c.Param("id"))
}

func (h *OpenAIGatewayHandler) handleJimengAudio(c *gin.Context, endpoint service.JimengAudioEndpoint, taskID string) {
	streamStarted := false
	defer h.recoverResponsesPanic(c, &streamStarted)

	_ = time.Now()
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
	contentType := c.GetHeader("Content-Type")
	if endpoint == service.JimengAudioGeneration {
		body, err = io.ReadAll(c.Request.Body)
		if err != nil {
			h.errorResponse(c, http.StatusBadRequest, "invalid_request", "Failed to read request body")
			return
		}
	}

	if h.errorPassthroughService != nil {
		service.BindErrorPassthroughService(c, h.errorPassthroughService)
	}

	group := apiKey.Group
	if group == nil {
		reqLog.Error("group not found in api key")
		h.errorResponse(c, http.StatusInternalServerError, "api_error", "Group not found")
		return
	}

	subscription, _ := middleware2.GetSubscriptionFromContext(c)

	userReleaseFunc, acquired := h.acquireResponsesUserSlot(c, subject.UserID, subject.Concurrency, false, &streamStarted, reqLog)
	if !acquired {
		return
	}
	if userReleaseFunc != nil {
		defer userReleaseFunc()
	}

	ctx := c.Request.Context()
	result, account, usageErr := h.runJimengAudioForwardLoop(ctx, c, group, endpoint, taskID, body, contentType, reqLog)
	if usageErr != nil {
		if errors.Is(usageErr, context.Canceled) {
			reqLog.Info("audio request canceled by client")
			return
		}
		reqLog.Error("jimeng audio forward failed", zap.Error(usageErr))
		if !service.IsResponseCommitted(c) {
			h.errorResponse(c, http.StatusBadGateway, "upstream_error", "Audio generation failed")
		}
		return
	}

	if endpoint == service.JimengAudioGeneration && account != nil && apiKey != nil && subscription != nil {
		// 音频计费逻辑待实现
		_ = result
	}
}

func (h *OpenAIGatewayHandler) runJimengAudioForwardLoop(
	ctx context.Context,
	c *gin.Context,
	group *service.Group,
	endpoint service.JimengAudioEndpoint,
	taskID string,
	body []byte,
	contentType string,
	reqLog *zap.Logger,
) (*service.OpenAIForwardResult, *service.Account, error) {
	var lastErr error
	maxRetries := 3
	excludedAccountIDs := make(map[int64]struct{})

	for retry := 0; retry < maxRetries; retry++ {
		account, err := h.gatewayService.SelectAccountForModelWithExclusions(ctx, &group.ID, "", "", excludedAccountIDs)
		if err != nil {
			reqLog.Warn("no available jimeng account", zap.Error(err), zap.Int("excluded_count", len(excludedAccountIDs)))
			return nil, nil, err
		}

		result, err := h.gatewayService.ForwardJimengAudio(ctx, c, account, endpoint, taskID, body, contentType)
		if err == nil {
			return result, account, nil
		}

		lastErr = err
		excludedAccountIDs[int64(account.ID)] = struct{}{}

		if retry < maxRetries-1 {
			reqLog.Info("jimeng audio retry", zap.Int("retry", retry+1), zap.Int64("account_id", int64(account.ID)), zap.Error(err))
			time.Sleep(100 * time.Millisecond)
		}
	}

	return nil, nil, lastErr
}

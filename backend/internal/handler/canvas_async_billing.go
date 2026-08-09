package handler

import (
	"context"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// recordCanvasAsyncCompletionUsage 在异步任务「轮询命中终态成功」时落一次用量记录并扣费。
//
// 二开背景：canvas 三个异步端点原本在提交时扣费，失败无退。改为完成时扣费后，
// canvas-api 的服务端 worker 会持续轮询 GET 状态（经 sub2api），任务终态必被某次
// 轮询命中，那次轮询的 service 层已填好计费字段（result.VideoCount>0），由此函数扣费。
//
// 幂等（关键）：同一任务会被轮询几十次，每次都是独立 HTTP 请求，中间件为每次生成
// 不同的 ClientRequestID。若直接沿用，resolveUsageBillingRequestID 会为每次轮询取到
// 不同 request_id → 幂等失效 → 重复扣费。故此处把 ctx 的 ClientRequestID 覆盖成
// task 级稳定值 canvas_async_task:{taskID}，使：
//   - usage_billing_dedup(request_id, api_key_id) 唯一约束只放行一次扣费；
//   - usage_logs 同键 ON CONFLICT DO NOTHING 只写一行；
//
// 二次及以后的轮询静默跳过（Applied:false），不报错、不重复扣。
//
// 通道：走 submitMandatoryUsageRecordTask（而非普通通道）——普通通道在 worker 池队列满
// 时按 drop 策略静默丢弃；而 canvas-api 一旦 MarkSucceeded 便停止轮询，丢了就永久漏计费。
// mandatory 通道在无法入队时内联同步执行，保证不丢。
func recordCanvasAsyncCompletionUsage(
	c *gin.Context,
	h *OpenAIGatewayHandler,
	reqLog *zap.Logger,
	apiKey *service.APIKey,
	subject middleware2.AuthSubject,
	subscription *service.UserSubscription,
	account *service.Account,
	result *service.OpenAIForwardResult,
	taskID string,
) {
	billingRequestID := service.CanvasAsyncBillingRequestID(taskID)
	if billingRequestID == "" {
		// 无 task id 无从保证幂等，宁可不扣也不冒重复扣费风险。
		reqLog.Warn("canvas_async.completion_billing_skipped_no_task_id")
		return
	}

	billingModel := strings.TrimSpace(result.Model)
	userAgent := c.GetHeader("User-Agent")
	clientIP := ip.GetClientIP(c)
	inboundEndpoint := GetInboundEndpoint(c)
	upstreamEndpoint := GetUpstreamEndpoint(c, account.Platform)
	quotaPlatform := service.QuotaPlatform(c.Request.Context(), apiKey)
	channelUsageFields := service.ChannelUsageFields{
		OriginalModel:      billingModel,
		ChannelMappedModel: billingModel,
	}

	// 覆盖 ClientRequestID 为 task 级稳定值：resolveUsageBillingRequestID 走最高优先级，
	// 对同一任务的每次轮询恒定，从而实现按 task 幂等。fingerprint 同样从稳定 key 派生。
	billingParent := context.WithValue(c.Request.Context(), ctxkey.ClientRequestID, billingRequestID)
	payloadHash := service.HashUsageRequestPayload([]byte(billingRequestID))

	h.submitMandatoryUsageRecordTask(billingParent, func(ctx context.Context) {
		if err := h.gatewayService.RecordUsage(ctx, &service.OpenAIRecordUsageInput{
			Result:             result,
			APIKey:             apiKey,
			User:               apiKey.User,
			Account:            account,
			Subscription:       subscription,
			InboundEndpoint:    inboundEndpoint,
			UpstreamEndpoint:   upstreamEndpoint,
			UserAgent:          userAgent,
			IPAddress:          clientIP,
			RequestPayloadHash: payloadHash,
			APIKeyService:      h.apiKeyService,
			QuotaPlatform:      quotaPlatform,
			ChannelUsageFields: channelUsageFields,
		}); err != nil {
			reqLog.Warn("canvas_async.completion_record_usage_failed",
				zap.Int64("user_id", subject.UserID),
				zap.Int64("api_key_id", apiKey.ID),
				zap.String("task_id", taskID),
				zap.String("model", billingModel),
				zap.Error(err))
		}
	})
}

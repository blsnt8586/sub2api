// Package avi2api 实现 AIV2API 上游协议层：URL 构建。
//
// AIV2API 是图像/视频/音频生成网关，通过 base_url + api_key 接入：
//
//	图像（同步，OpenAI 兼容）：
//	  - POST /v1/images/generations   文生图
//	  - POST /v1/images/edits         图生图（multipart，1-6 张参考图）
//	视频（异步任务）：
//	  - POST /v1/videos/generations   创建视频任务，返回 Task{id,status,...}
//	  - GET  /v1/videos/{id}          轮询任务状态，成功后 result.data[0].url 为 MP4
//	  - POST /v1/videos/{id}/cancel   取消排队中的视频任务
//	音频（异步任务）：
//	  - POST /v1/audio/generations    创建音频任务
//	  - GET  /v1/audio/{id}           轮询音频任务状态
//	  - POST /v1/audio/{id}/cancel    取消排队中的音频任务
//
// 本包只负责路径拼接；base_url allowlist 安全校验由 service 层完成。
package avi2api

import (
	"fmt"
	"net/url"
	"strings"
)

// DefaultBaseURL 是 AIV2API 接入的占位默认值，仅在账号未显式配置 base_url 时兜底。
const DefaultBaseURL = "https://api.avi2api.example.com/v1"

// normalizeBaseURL 去除首尾空白与末尾斜杠，并保证以 /v1 结尾，
// 兼容管理员填入 "https://host" 或 "https://host/v1" 两种写法。
func normalizeBaseURL(baseURL string) (string, error) {
	trimmed := strings.TrimSpace(baseURL)
	if trimmed == "" {
		return "", fmt.Errorf("base url is required")
	}
	trimmed = strings.TrimRight(trimmed, "/")
	if !strings.HasSuffix(trimmed, "/v1") {
		trimmed += "/v1"
	}
	return trimmed, nil
}

// BuildImagesGenerationsURL 构建文生图 URL：{base}/v1/images/generations。
func BuildImagesGenerationsURL(baseURL string) (string, error) {
	base, err := normalizeBaseURL(baseURL)
	if err != nil {
		return "", err
	}
	return base + "/images/generations", nil
}

// BuildImagesEditsURL 构建图生图 URL：{base}/v1/images/edits。
func BuildImagesEditsURL(baseURL string) (string, error) {
	base, err := normalizeBaseURL(baseURL)
	if err != nil {
		return "", err
	}
	return base + "/images/edits", nil
}

// BuildVideosGenerationsURL 构建创建视频任务的 URL：{base}/v1/videos/generations。
func BuildVideosGenerationsURL(baseURL string) (string, error) {
	base, err := normalizeBaseURL(baseURL)
	if err != nil {
		return "", err
	}
	return base + "/videos/generations", nil
}

// BuildVideoStatusURL 构建查询视频任务状态的 URL：{base}/v1/videos/{id}。
func BuildVideoStatusURL(baseURL, taskID string) (string, error) {
	base, err := normalizeBaseURL(baseURL)
	if err != nil {
		return "", err
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return "", fmt.Errorf("task id is required")
	}
	return base + "/videos/" + url.PathEscape(taskID), nil
}

// BuildVideoCancelURL 构建取消视频任务的 URL：{base}/v1/videos/{id}/cancel。
func BuildVideoCancelURL(baseURL, taskID string) (string, error) {
	base, err := normalizeBaseURL(baseURL)
	if err != nil {
		return "", err
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return "", fmt.Errorf("task id is required")
	}
	return base + "/videos/" + url.PathEscape(taskID) + "/cancel", nil
}

// BuildAudioGenerationsURL 构建创建音频任务的 URL：{base}/v1/audio/generations。
func BuildAudioGenerationsURL(baseURL string) (string, error) {
	base, err := normalizeBaseURL(baseURL)
	if err != nil {
		return "", err
	}
	return base + "/audio/generations", nil
}

// BuildAudioStatusURL 构建查询音频任务状态的 URL：{base}/v1/audio/{id}。
func BuildAudioStatusURL(baseURL, taskID string) (string, error) {
	base, err := normalizeBaseURL(baseURL)
	if err != nil {
		return "", err
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return "", fmt.Errorf("task id is required")
	}
	return base + "/audio/" + url.PathEscape(taskID), nil
}

// BuildAudioCancelURL 构建取消音频任务的 URL：{base}/v1/audio/{id}/cancel。
func BuildAudioCancelURL(baseURL, taskID string) (string, error) {
	base, err := normalizeBaseURL(baseURL)
	if err != nil {
		return "", err
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return "", fmt.Errorf("task id is required")
	}
	return base + "/audio/" + url.PathEscape(taskID) + "/cancel", nil
}

// BuildAsyncImageTaskURL 构建异步图像任务提交 URL：{base}/v1/tasks/images。
// 异步图像任务仅支持文生图（AsyncImageRequest，additionalProperties:false，无 image 字段）。
// 图生图仍需用同步接口 BuildImagesEditsURL，附 Idempotency-Key 保证崩溃安全。
func BuildAsyncImageTaskURL(baseURL string) (string, error) {
	base, err := normalizeBaseURL(baseURL)
	if err != nil {
		return "", err
	}
	return base + "/tasks/images", nil
}

// BuildTaskStatusURL 构建统一任务状态查询 URL：{base}/v1/tasks/{id}。
// 该端点服务 image / video / audio 三种 kind，是修复 canvas-api task_poller
// 轮询恒 404 缺陷的关键——poller 原先就打这个路径，但 sub2api 之前没有此路由。
func BuildTaskStatusURL(baseURL, taskID string) (string, error) {
	base, err := normalizeBaseURL(baseURL)
	if err != nil {
		return "", err
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return "", fmt.Errorf("task id is required")
	}
	return base + "/tasks/" + url.PathEscape(taskID), nil
}

// BuildTaskCancelURL 构建统一任务取消 URL：{base}/v1/tasks/{id}/cancel。
func BuildTaskCancelURL(baseURL, taskID string) (string, error) {
	base, err := normalizeBaseURL(baseURL)
	if err != nil {
		return "", err
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return "", fmt.Errorf("task id is required")
	}
	return base + "/tasks/" + url.PathEscape(taskID) + "/cancel", nil
}

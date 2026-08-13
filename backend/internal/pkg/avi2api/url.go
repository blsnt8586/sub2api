// Package avi2api 实现 AIV2API 上游协议层：URL 构建。
//
// AIV2API 是图像/视频/音频生成网关，通过 base_url + api_key 接入：
//
//	图像（异步任务）：
//	  - POST /v1/images/generations   创建图像任务（JSON 文生图或 multipart 图生图）
//	  - GET  /v1/images/{id}          轮询图像任务状态
//	  - POST /v1/images/{id}/cancel   取消排队中的图像任务
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

// BuildImagesGenerationsURL 构建图像任务创建 URL：{base}/v1/images/generations。
func BuildImagesGenerationsURL(baseURL string) (string, error) {
	base, err := normalizeBaseURL(baseURL)
	if err != nil {
		return "", err
	}
	return base + "/images/generations", nil
}

// BuildImagesEditsURL 是旧 AIV2API 1.x 图生图 URL helper。
// 2.0 调用方应使用 BuildImageGenerationTaskURL，并以 multipart 提交参考图。
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

// BuildImageGenerationTaskURL 构建异步图像任务提交 URL：
// {base}/v1/images/generations。AIV2API 2.0 在同一路径按 Content-Type 区分
// JSON 文生图与 multipart 图生图。
func BuildImageGenerationTaskURL(baseURL string) (string, error) {
	base, err := normalizeBaseURL(baseURL)
	if err != nil {
		return "", err
	}
	return base + "/images/generations", nil
}

// BuildImageStatusURL 构建图像任务状态查询 URL：{base}/v1/images/{id}。
func BuildImageStatusURL(baseURL, taskID string) (string, error) {
	base, err := normalizeBaseURL(baseURL)
	if err != nil {
		return "", err
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return "", fmt.Errorf("task id is required")
	}
	return base + "/images/" + url.PathEscape(taskID), nil
}

// BuildImageCancelURL 构建图像任务取消 URL：{base}/v1/images/{id}/cancel。
func BuildImageCancelURL(baseURL, taskID string) (string, error) {
	base, err := normalizeBaseURL(baseURL)
	if err != nil {
		return "", err
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return "", fmt.Errorf("task id is required")
	}
	return base + "/images/" + url.PathEscape(taskID) + "/cancel", nil
}

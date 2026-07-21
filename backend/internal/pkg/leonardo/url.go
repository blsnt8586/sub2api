// Package leonardo 实现 Leonardo 上游（对外伪装为即梦 jimeng 平台的一个 vendor 子类型）
// 的协议层：URL 构建与请求/响应解析。
//
// Leonardo 是 OpenAI Images 兼容的图像/视频生成网关，通过第三方 base_url + api_key 接入：
//
//	图像（同步，OpenAI 兼容）：
//	  - POST /v1/images/generations   文生图
//	  - POST /v1/images/edits         图生图（multipart，1-6 张参考图）
//	视频（异步任务）：
//	  - POST /v1/videos/generations   创建视频任务，返回 {id,status}
//	  - GET  /v1/videos/{id}          轮询任务状态，成功后 data[0].url 为 MP4
//
// 本包只负责路径拼接与格式规范化；第三方 base_url 的 allowlist 安全校验由
// service 层通过 Security.URLAllowlist 完成（对齐 Grok/OpenAI/即梦 api_key 路径）。
package leonardo

import (
	"fmt"
	"net/url"
	"strings"
)

// DefaultBaseURL 是 Leonardo 接入的占位默认值，仅在账号未显式配置 base_url 时兜底。
const DefaultBaseURL = "https://api.leonardo.example.com/v1"

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

// BuildVideoStatusURL 构建查询任务状态的 URL：{base}/v1/videos/{task_id}。
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

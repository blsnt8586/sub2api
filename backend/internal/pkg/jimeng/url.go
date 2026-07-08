// Package jimeng 实现即梦（Jimeng）视频生成平台的协议层：URL 构建与请求解析。
//
// 即梦通过第三方 base_url + api_key 接入，接口固定为：
//   - POST /v1/videos              创建视频任务
//   - GET  /v1/videos/{task_id}    查询任务状态
//   - GET  /v1/videos/{task_id}/content  下载视频内容
//
// 本包只负责路径拼接与格式规范化；第三方 base_url 的 allowlist 安全校验由
// service 层通过 Security.URLAllowlist 完成（对齐 Grok/OpenAI api_key 路径）。
package jimeng

import (
	"fmt"
	"net/url"
	"strings"
)

// DefaultBaseURL 是即梦接入的占位默认值，仅在账号未显式配置 base_url 时兜底。
const DefaultBaseURL = "https://api.jimeng.example.com/v1"

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

// BuildVideosURL 构建创建视频任务的 URL：{base}/v1/videos。
func BuildVideosURL(baseURL string) (string, error) {
	base, err := normalizeBaseURL(baseURL)
	if err != nil {
		return "", err
	}
	return base + "/videos", nil
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

// BuildVideoContentURL 构建下载视频内容的 URL：{base}/v1/videos/{task_id}/content。
func BuildVideoContentURL(baseURL, taskID string) (string, error) {
	base, err := normalizeBaseURL(baseURL)
	if err != nil {
		return "", err
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return "", fmt.Errorf("task id is required")
	}
	return base + "/videos/" + url.PathEscape(taskID) + "/content", nil
}

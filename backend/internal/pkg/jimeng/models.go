package jimeng

import (
	"strings"

	"github.com/tidwall/gjson"
)

// VideoRequest 是即梦创建视频任务的请求体。
//
// 兼容两种时长字段写法：
//   - duration（整数，新提供商如 cangyuansuanli）
//   - seconds（字符串或数字，旧接口兼容）
type VideoRequest struct {
	Model       string `json:"model"`
	Prompt      string `json:"prompt"`
	Duration    int    `json:"duration,omitempty"`
	Seconds     string `json:"seconds,omitempty"`
	AspectRatio string `json:"aspect_ratio"`
}

// VideoRequestInfo 携带从入站请求体解析出的关键字段，供计费与日志使用。
type VideoRequestInfo struct {
	Model       string
	Prompt      string
	Seconds     string
	AspectRatio string
}

// ExtractVideoModel 从请求体中提取 model 字段（trim 空白），无法解析时返回空串。
func ExtractVideoModel(body []byte) string {
	return ParseVideoRequest(body).Model
}

// ParseVideoRequest 解析即梦视频请求体，兼容 duration（整数）和 seconds（字符串/数字）两种写法。
func ParseVideoRequest(body []byte) VideoRequestInfo {
	info := VideoRequestInfo{}
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return info
	}
	info.Model = strings.TrimSpace(gjson.GetBytes(body, "model").String())
	info.Prompt = strings.TrimSpace(gjson.GetBytes(body, "prompt").String())
	info.AspectRatio = strings.TrimSpace(gjson.GetBytes(body, "aspect_ratio").String())
	// 优先读 duration（新提供商整数字段），fallback 到 seconds（旧接口兼容）
	if d := gjson.GetBytes(body, "duration"); d.Exists() {
		info.Seconds = strings.TrimSpace(d.String())
	} else if s := gjson.GetBytes(body, "seconds"); s.Exists() {
		info.Seconds = strings.TrimSpace(s.String())
	}
	return info
}

// ParseAudioRequest 解析即梦音频请求体
func ParseAudioRequest(body []byte) VideoRequestInfo {
	info := VideoRequestInfo{}
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return info
	}
	info.Model = strings.TrimSpace(gjson.GetBytes(body, "model").String())
	info.Prompt = strings.TrimSpace(gjson.GetBytes(body, "prompt").String())
	// 音频可能也有 duration 字段
	if d := gjson.GetBytes(body, "duration"); d.Exists() {
		info.Seconds = strings.TrimSpace(d.String())
	}
	return info
}


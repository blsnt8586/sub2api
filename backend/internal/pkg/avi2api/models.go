package avi2api

import (
	"bytes"
	"io"
	"mime"
	"mime/multipart"
	"strconv"
	"strings"

	"github.com/tidwall/gjson"
)

// multipartFieldMaxSize 限制解析 multipart 文本字段时读取的字节数。
// 文件字段（参考图/视频/音频）不读入内存，body 原样透传给上游。
const multipartFieldMaxSize = 1 << 20 // 1 MiB

// ────────────────────────────── 图像 ──────────────────────────────

// ImageRequestInfo 携带图像请求的关键字段，供计费与日志使用。
type ImageRequestInfo struct {
	Model  string
	Prompt string
	// Size 如 "1024x1024" / "2048x2048"，用于计费档位归一化。
	Size string
	// N 是请求生成的图片数量（默认 1）。
	N        int
	NPresent bool
}

// ExtractImageModel 从图像请求体中提取 model 字段，兼容 JSON 与 multipart。
func ExtractImageModel(contentType string, body []byte) string {
	return ParseImageRequest(contentType, body).Model
}

// ParseImageRequest 解析 AIV2API 图像请求。
// /images/generations 的 JSON 文生图直接读字段；multipart 图生图从 form 字段提取。
func ParseImageRequest(contentType string, body []byte) ImageRequestInfo {
	info := ImageRequestInfo{N: 1}
	if gjson.ValidBytes(body) {
		info.Model = strings.TrimSpace(gjson.GetBytes(body, "model").String())
		info.Prompt = strings.TrimSpace(gjson.GetBytes(body, "prompt").String())
		info.Size = strings.TrimSpace(gjson.GetBytes(body, "size").String())
		if n := gjson.GetBytes(body, "n"); n.Exists() && n.Type == gjson.Number {
			info.N, info.NPresent = int(n.Int()), true
		}
	} else {
		parseImageMultipartFields(contentType, body, &info)
	}
	if !info.NPresent {
		info.N = 1
	}
	return info
}

func parseImageMultipartFields(contentType string, body []byte, info *ImageRequestInfo) {
	if info == nil {
		return
	}
	mr := newMultipartReader(contentType, body)
	if mr == nil {
		return
	}
	for {
		part, err := mr.NextPart()
		if err != nil {
			return
		}
		name := strings.TrimSpace(part.FormName())
		if name == "" || strings.TrimSpace(part.FileName()) != "" {
			_ = part.Close()
			continue
		}
		data, err := io.ReadAll(io.LimitReader(part, multipartFieldMaxSize))
		_ = part.Close()
		if err != nil {
			return
		}
		value := strings.TrimSpace(string(data))
		switch name {
		case "model":
			info.Model = value
		case "prompt":
			info.Prompt = value
		case "size":
			info.Size = value
		case "n":
			if n, err := strconv.Atoi(value); err == nil {
				info.N, info.NPresent = n, true
			}
		}
	}
}

// ────────────────────────────── 视频 ──────────────────────────────

// VideoRequestInfo 携带视频请求的关键字段，供选号、计费与日志使用。
//
// AIV2API 视频接口有两种请求格式：
//   - JSON（无参考）：所有字段直接解析
//   - multipart（参考图/首尾帧/参考视频/参考音频）：只解析文本字段，
//     文件字段不读入内存，body 逐字节透传给上游
type VideoRequestInfo struct {
	Model  string
	Prompt string
	// Seconds 存 duration 整数值的字符串形式，用于计费（VideoSeconds）。
	Seconds    string
	Size       string
	Resolution string
}

// ExtractVideoModel 从视频请求体中提取 model 字段，兼容 JSON 与 multipart。
func ExtractVideoModel(contentType string, body []byte) string {
	return ParseVideoRequest(contentType, body).Model
}

// ParseVideoRequest 解析 AIV2API 视频请求，兼容 JSON 与 multipart 两种格式。
//
//	无参考模式：application/json，直接 gjson 解析
//	参考模式：multipart/form-data，只提取文本字段（model/prompt/duration/size/resolution）
func ParseVideoRequest(contentType string, body []byte) VideoRequestInfo {
	info := VideoRequestInfo{}
	if gjson.ValidBytes(body) {
		// JSON 模式：无参考视频
		info.Model = strings.TrimSpace(gjson.GetBytes(body, "model").String())
		info.Prompt = strings.TrimSpace(gjson.GetBytes(body, "prompt").String())
		info.Size = strings.TrimSpace(gjson.GetBytes(body, "size").String())
		info.Resolution = strings.TrimSpace(gjson.GetBytes(body, "resolution").String())
		// duration 优先，fallback seconds（向后兼容旧写法）
		if d := gjson.GetBytes(body, "duration"); d.Exists() {
			info.Seconds = strings.TrimSpace(d.String())
		} else if s := gjson.GetBytes(body, "seconds"); s.Exists() {
			info.Seconds = strings.TrimSpace(s.String())
		}
	} else {
		// multipart 模式：参考图/首尾帧/参考视频/参考音频
		parseVideoMultipartFields(contentType, body, &info)
	}
	return info
}

func parseVideoMultipartFields(contentType string, body []byte, info *VideoRequestInfo) {
	if info == nil {
		return
	}
	mr := newMultipartReader(contentType, body)
	if mr == nil {
		return
	}
	for {
		part, err := mr.NextPart()
		if err != nil {
			return
		}
		name := strings.TrimSpace(part.FormName())
		// 跳过文件字段（参考图/视频/音频/首尾帧），不读入内存
		if name == "" || strings.TrimSpace(part.FileName()) != "" {
			_ = part.Close()
			continue
		}
		data, err := io.ReadAll(io.LimitReader(part, multipartFieldMaxSize))
		_ = part.Close()
		if err != nil {
			return
		}
		value := strings.TrimSpace(string(data))
		switch name {
		case "model":
			info.Model = value
		case "prompt":
			info.Prompt = value
		case "duration":
			info.Seconds = value
		case "size":
			info.Size = value
		case "resolution":
			info.Resolution = value
		}
	}
}

// ────────────────────────────── 音频 ──────────────────────────────

// AudioRequestInfo 携带音频请求的关键字段，供选号与计费使用。
type AudioRequestInfo struct {
	Model  string
	Prompt string
	// Seconds 存音频时长（dialogue-v3/sound-effects-v2 的 duration，sound-effects-v2 单位秒）。
	Seconds string
	// DurationMinutes 存 music-v1 的 duration_minutes 字段。
	DurationMinutes string
}

// ParseAudioRequest 解析 AIV2API 音频请求（仅 JSON，音频不支持 multipart）。
//
// AIV2API 音频三种模型：
//   - dialogue-v3：TTS，duration 字段为秒数（整数）
//   - music-v1：音乐生成，duration_minutes 字段为分钟数
//   - sound-effects-v2：音效，duration 字段为秒数
func ParseAudioRequest(body []byte) AudioRequestInfo {
	info := AudioRequestInfo{}
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return info
	}
	info.Model = strings.TrimSpace(gjson.GetBytes(body, "model").String())
	info.Prompt = strings.TrimSpace(gjson.GetBytes(body, "prompt").String())
	if d := gjson.GetBytes(body, "duration"); d.Exists() {
		info.Seconds = strings.TrimSpace(d.String())
	}
	if dm := gjson.GetBytes(body, "duration_minutes"); dm.Exists() {
		info.DurationMinutes = strings.TrimSpace(dm.String())
	}
	return info
}

// ────────────────────────────── 工具 ──────────────────────────────

// newMultipartReader 从 Content-Type 与 body 构建 multipart.Reader。
// 返回 nil 表示解析失败（非 multipart 或缺 boundary）。
func newMultipartReader(contentType string, body []byte) *multipart.Reader {
	mediaType, params, err := mime.ParseMediaType(strings.TrimSpace(contentType))
	if err != nil || !strings.EqualFold(mediaType, "multipart/form-data") {
		return nil
	}
	boundary := strings.TrimSpace(params["boundary"])
	if boundary == "" {
		return nil
	}
	return multipart.NewReader(bytes.NewReader(body), boundary)
}

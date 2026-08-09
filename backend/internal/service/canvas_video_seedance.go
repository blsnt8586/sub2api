package service

// Seedance（火山 Ark Plan v3）视频协议兼容层。
//
// 背景：部分客户端（如 infinite-canvas）在识别到模型名含 "seedance" 时，会按
// 火山 Ark Plan 原生协议发请求，路径为 /v1/contents/generations/tasks，body 格式：
//
//	{
//	  "model": "seedance-2.0-fast",
//	  "content": [{"type":"text","text":"提示词"}, ...],
//	  "ratio":"9:16","resolution":"480p","duration":4,...
//	}
//
// 上游 AIV2API 接受字段：model, prompt, duration, size, resolution, generate_audio。
//
// 本文件负责把前者翻译成后者。翻译在 handler 读取 body 之后进行，
// 对已经是 OpenAI 风格（带 prompt）的 body 无任何副作用。

import (
	"encoding/json"
	"strings"

	"github.com/tidwall/gjson"
)

// IsSeedanceVideoCreateBody 判断请求体是否为 Seedance/Ark Plan 原生格式。
// 判据：存在 content 数组且没有顶层 prompt 字段。
func IsSeedanceVideoCreateBody(body []byte) bool {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return false
	}
	if strings.TrimSpace(gjson.GetBytes(body, "prompt").String()) != "" {
		return false
	}
	return gjson.GetBytes(body, "content").IsArray()
}

// ConvertSeedanceVideoCreateBody 把 Seedance/Ark Plan 原生请求体翻译为 AIV2API
// 期望的 AIV2API 风格 body（model/prompt/duration/size/resolution/generate_audio）。
// 非 Seedance 格式原样返回。
//
// AIV2API VideoRequest 接受字段：model, prompt, duration, size, resolution, generate_audio。
// 其余 Seedance 扩展字段（fps / watermark 等）必须丢弃，否则上游返回 400。
func ConvertSeedanceVideoCreateBody(body []byte) []byte {
	if !IsSeedanceVideoCreateBody(body) {
		return body
	}

	parsed := gjson.ParseBytes(body)
	converted := map[string]any{}

	// model 原样透传
	if model := strings.TrimSpace(parsed.Get("model").String()); model != "" {
		converted["model"] = model
	}

	// content[type=text].text → prompt（多段换行拼接）
	var promptParts []string
	parsed.Get("content").ForEach(func(_, item gjson.Result) bool {
		if item.Get("type").String() == "text" {
			if text := strings.TrimSpace(item.Get("text").String()); text != "" {
				promptParts = append(promptParts, text)
			}
		}
		// image_url / video_url / audio_url 上游不支持，安静丢弃
		return true
	})
	if len(promptParts) > 0 {
		converted["prompt"] = strings.Join(promptParts, "\n")
	}

	// duration：-1 = 自适应/不限，省略交由上游默认；其余正整数原样透传
	if d := parsed.Get("duration"); d.Exists() {
		if seconds := d.Int(); seconds > 0 {
			converted["duration"] = seconds
		}
	}

	// size：从 ratio 映射到 AIV2API 枚举（1280x720 或 720x1280）
	if ratio := strings.TrimSpace(parsed.Get("ratio").String()); ratio != "" {
		if ratio == "9:16" || ratio == "2:3" || ratio == "3:4" {
			converted["size"] = "720x1280"
		} else {
			converted["size"] = "1280x720"
		}
	}

	// resolution：480p/720p/1080p/2160p 白名单透传，其余丢弃
	if res := strings.TrimSpace(parsed.Get("resolution").String()); res != "" {
		switch res {
		case "480p", "720p", "1080p", "2160p":
			converted["resolution"] = res
		}
	}

	// fps 已移除：AIV2API v1.1.2 的视频 schema 全部为 additionalProperties:false
	// 且均无 fps 字段，透传会导致上游 400。watermark 等 Seedance 扩展字段同理丢弃。

	// generate_audio：布尔值原样透传（默认 true，省略时上游采用默认）
	if ga := parsed.Get("generate_audio"); ga.Exists() {
		converted["generate_audio"] = ga.Bool()
	}

	result, err := json.Marshal(converted)
	if err != nil {
		return body
	}
	return result
}

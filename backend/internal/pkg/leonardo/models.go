package leonardo

import (
	"bytes"
	"io"
	"mime"
	"mime/multipart"
	"strconv"
	"strings"

	"github.com/tidwall/gjson"
)

// multipartFieldMaxSize 限制解析 multipart 文本字段时读取的字节数，
// 避免恶意超大字段耗尽内存。参考图二进制内容不在此解析（body 原样透传）。
const multipartFieldMaxSize = 1 << 20 // 1 MiB

// ImageRequestInfo 携带从入站图像请求体解析出的关键字段，供计费与日志使用。
type ImageRequestInfo struct {
	Model  string
	Prompt string
	// Size 是请求的目标尺寸（如 "1024x1024" / "2048x2048"），用于计费档位归一化。
	Size string
	// N 是请求生成的图片数量（默认 1）。
	N int
}

// ExtractImageModel 从图像请求体中提取 model 字段，兼容 JSON 与 multipart。
func ExtractImageModel(contentType string, body []byte) string {
	return ParseImageRequest(contentType, body).Model
}

// ParseImageRequest 解析 Leonardo 图像请求。
// JSON 请求（/images/generations）直接读字段；multipart 请求（/images/edits）
// 从 form 字段中提取。multipart 场景下字段值可能不易解析，解析失败时保持零值，
// 计费档位由 service 层用 NormalizeImageBillingTierOrDefault 兜底为默认档。
func ParseImageRequest(contentType string, body []byte) ImageRequestInfo {
	info := ImageRequestInfo{N: 1}
	if gjson.ValidBytes(body) {
		info.Model = strings.TrimSpace(gjson.GetBytes(body, "model").String())
		info.Prompt = strings.TrimSpace(gjson.GetBytes(body, "prompt").String())
		info.Size = strings.TrimSpace(gjson.GetBytes(body, "size").String())
		if n := gjson.GetBytes(body, "n"); n.Exists() && n.Type == gjson.Number {
			info.N = int(n.Int())
		}
	} else {
		parseImageMultipartRequest(contentType, body, &info)
	}
	if info.N <= 0 {
		info.N = 1
	}
	return info
}

// CountImages 统计成功响应体中 data 数组的元素个数（即生成的图片数量）。
// 解析失败或无 data 数组时返回 0，由调用方回退到请求侧的 N。
func CountImages(body []byte) int {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return 0
	}
	data := gjson.GetBytes(body, "data")
	if !data.IsArray() {
		return 0
	}
	return len(data.Array())
}

// parseImageMultipartRequest 从 multipart/form-data（/images/edits）中提取
// 计费相关的文本字段（model / prompt / size / n）。参考图二进制内容不解析，
// 请求 body 由 service 层原样透传给上游。
func parseImageMultipartRequest(contentType string, body []byte, info *ImageRequestInfo) {
	if info == nil {
		return
	}
	mediaType, params, err := mime.ParseMediaType(strings.TrimSpace(contentType))
	if err != nil || !strings.EqualFold(mediaType, "multipart/form-data") {
		return
	}
	boundary := strings.TrimSpace(params["boundary"])
	if boundary == "" {
		return
	}
	reader := multipart.NewReader(bytes.NewReader(body), boundary)
	for {
		part, err := reader.NextPart()
		if err != nil {
			return
		}
		name := strings.TrimSpace(part.FormName())
		// 仅解析文本字段；文件字段（参考图）跳过，不读入内存。
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
				info.N = n
			}
		}
	}
}

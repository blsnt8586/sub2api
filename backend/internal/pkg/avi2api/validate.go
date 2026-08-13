package avi2api

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"strconv"
	"strings"

	"github.com/tidwall/gjson"
)

// ─────────────────────────────────────────────
// 错误类型
// ─────────────────────────────────────────────

// ValidationError 是参数校验失败时返回的结构化错误。
// Field 为出错的参数名（空表示通用错误），Message 是面向用户的描述。
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("invalid parameter '%s': %s", e.Field, e.Message)
	}
	return e.Message
}

func validationErr(field, msg string, args ...any) *ValidationError {
	return &ValidationError{Field: field, Message: fmt.Sprintf(msg, args...)}
}

// ─────────────────────────────────────────────
// 视频请求校验
// ─────────────────────────────────────────────

// ValidateVideoRequest 在转发前校验视频请求的参数合法性。
//
// 校验流程：
//  1. model 必须是已注册模型
//  2. JSON 请求：duration/size/resolution/generate_audio 按模型约束校验
//  3. multipart 请求：检测携带的文件字段，校验参考模式是否被该模型支持，
//     以及首尾帧与其他参考素材的互斥规则
//
// body 原样保留，不修改。组合规则由上游二次校验，此处仅做"明显非法"过滤。
func ValidateVideoRequest(contentType string, body []byte) *ValidationError {
	// 1. 提取 model
	info := ParseVideoRequest(contentType, body)
	model := strings.TrimSpace(info.Model)
	if model == "" {
		return validationErr("model", "model is required")
	}

	caps := LookupVideoModel(model)
	if caps == nil {
		return validationErr("model", "unsupported model %q; supported: %s",
			model, strings.Join(AllVideoModels(), ", "))
	}

	// 2. JSON 请求校验
	if gjson.ValidBytes(body) {
		return validateVideoJSONParams(caps, body)
	}

	// 3. multipart 请求校验
	return validateVideoMultipartRef(caps, contentType, body)
}

// validateVideoJSONParams 校验 JSON body 中的参数。
func validateVideoJSONParams(caps *VideoModelCaps, body []byte) *ValidationError {
	// duration
	if d := gjson.GetBytes(body, "duration"); d.Exists() {
		dur := int(d.Int())
		if err := validateDuration(caps, dur); err != nil {
			return err
		}
	}

	// size
	if s := gjson.GetBytes(body, "size"); s.Exists() {
		size := s.String()
		if !containsStr(caps.AllowedSizes, size) {
			return validationErr("size",
				"model %q does not support size %q; allowed: %s",
				caps.Model, size, strings.Join(caps.AllowedSizes, ", "))
		}
	}

	// resolution
	if r := gjson.GetBytes(body, "resolution"); r.Exists() {
		res := r.String()
		if !containsStr(caps.AllowedResolutions, res) {
			return validationErr("resolution",
				"model %q does not support resolution %q; allowed: %s",
				caps.Model, res, strings.Join(caps.AllowedResolutions, ", "))
		}
	}

	// generate_audio
	if ga := gjson.GetBytes(body, "generate_audio"); ga.Exists() {
		switch caps.GenerateAudio {
		case GenAudioUnsupported:
			return validationErr("generate_audio",
				"model %q does not support generate_audio", caps.Model)
		case GenAudioAlwaysTrue:
			if !ga.Bool() {
				return validationErr("generate_audio",
					"model %q requires generate_audio=true", caps.Model)
			}
		}
	}

	return nil
}

// validateVideoMultipartRef 检测 multipart 请求中的文件字段并校验参考模式。
func validateVideoMultipartRef(caps *VideoModelCaps, contentType string, body []byte) *ValidationError {
	fileFields := extractMultipartFileFieldNames(contentType, body)
	if len(fileFields) == 0 {
		// 没有文件字段但 Content-Type 是 multipart：让上游判断
		return nil
	}

	hasImage := containsField(fileFields, "image")
	hasStartFrame := containsField(fileFields, "start_frame")
	hasEndFrame := containsField(fileFields, "end_frame")
	hasVideo := containsField(fileFields, "video")
	hasAudio := containsField(fileFields, "audio")

	// 确定请求的参考模式
	isFrameMode := hasStartFrame || hasEndFrame
	isImageMode := hasImage
	isVideoMode := hasVideo
	isAudioMode := hasAudio

	// end_frame 必须搭配 start_frame
	if hasEndFrame && !hasStartFrame {
		return validationErr("end_frame", "end_frame requires start_frame")
	}

	// 首尾帧与其他参考素材互斥
	if isFrameMode && (isImageMode || isVideoMode || isAudioMode) {
		return validationErr("start_frame",
			"start_frame/end_frame cannot be combined with image, video, or audio references")
	}

	// 校验参考模式是否被该模型支持
	if isFrameMode && !capSupports(caps, RefModeFrame) {
		return validationErr("start_frame",
			"model %q does not support frame references (start_frame/end_frame)", caps.Model)
	}
	if isImageMode && !capSupports(caps, RefModeImage) {
		return validationErr("image",
			"model %q does not support image references", caps.Model)
	}
	if isVideoMode && !capSupports(caps, RefModeVideo) {
		return validationErr("video",
			"model %q does not support video references", caps.Model)
	}
	if isAudioMode && !capSupports(caps, RefModeAudio) {
		return validationErr("audio",
			"model %q does not support audio references", caps.Model)
	}

	// 音频参考依赖校验
	if isAudioMode && caps.AudioRefRequiresImageOrVideo {
		if !isImageMode && !isVideoMode {
			return validationErr("audio",
				"model %q audio reference requires an accompanying image or video reference", caps.Model)
		}
	}
	// MiniMax H3：音频只能搭配普通图片（非首尾帧）
	if isAudioMode && caps.Model == "minimax-h3" && !isImageMode {
		return validationErr("audio",
			"model %q audio reference requires an image reference", caps.Model)
	}

	// 同时校验 multipart 中携带的文本字段（duration/size/resolution）
	textInfo := ParseVideoRequest(contentType, body)
	if textInfo.Seconds != "" {
		if dur, err := parseDurationStr(textInfo.Seconds); err == nil {
			if verr := validateDuration(caps, dur); verr != nil {
				return verr
			}
		}
	}
	if textInfo.Size != "" && !containsStr(caps.AllowedSizes, textInfo.Size) {
		return validationErr("size",
			"model %q does not support size %q; allowed: %s",
			caps.Model, textInfo.Size, strings.Join(caps.AllowedSizes, ", "))
	}
	if textInfo.Resolution != "" && !containsStr(caps.AllowedResolutions, textInfo.Resolution) {
		return validationErr("resolution",
			"model %q does not support resolution %q; allowed: %s",
			caps.Model, textInfo.Resolution, strings.Join(caps.AllowedResolutions, ", "))
	}

	return nil
}

// ─────────────────────────────────────────────
// 音频请求校验
// ─────────────────────────────────────────────

// ValidateAudioRequest 校验音频请求参数（仅 JSON）。
func ValidateAudioRequest(body []byte) *ValidationError {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return validationErr("", "request body must be valid JSON")
	}

	// sound-effects-v2 的 model 字段可省略（schema 有默认值），补齐后校验
	model := strings.TrimSpace(gjson.GetBytes(body, "model").String())
	if model == "" {
		model = "sound-effects-v2"
	}

	caps := LookupAudioModel(model)
	if caps == nil {
		supported := make([]string, 0, len(audioModels))
		for _, a := range audioModels {
			supported = append(supported, a.Model)
		}
		return validationErr("model", "unsupported audio model %q; supported: %s",
			model, strings.Join(supported, ", "))
	}

	prompt := strings.TrimSpace(gjson.GetBytes(body, "prompt").String())
	if prompt == "" {
		return validationErr("prompt", "prompt is required")
	}

	// n
	if n := gjson.GetBytes(body, "n"); n.Exists() {
		nv := int(n.Int())
		if nv < caps.NMin || nv > caps.NMax {
			return validationErr("n",
				"model %q: n must be between %d and %d", model, caps.NMin, caps.NMax)
		}
	}

	switch model {
	case "dialogue-v3":
		if voice := gjson.GetBytes(body, "voice"); voice.Exists() {
			v := voice.String()
			if len(caps.AllowedVoices) > 0 && !containsStr(caps.AllowedVoices, v) {
				return validationErr("voice",
					"unsupported voice %q; allowed: %s", v, strings.Join(caps.AllowedVoices, ", "))
			}
		}
		if pi := gjson.GetBytes(body, "prompt_influence"); pi.Exists() {
			if pi.Float() < 0 || pi.Float() > 1 {
				return validationErr("prompt_influence", "must be between 0 and 1")
			}
		}

	case "music-v1":
		if dm := gjson.GetBytes(body, "duration_minutes"); dm.Exists() {
			v := int(dm.Int())
			if v < caps.DurationMinMin || v > caps.DurationMinMax {
				return validationErr("duration_minutes",
					"must be between %d and %d", caps.DurationMinMin, caps.DurationMinMax)
			}
		}

	case "sound-effects-v2":
		if d := gjson.GetBytes(body, "duration"); d.Exists() {
			v := int(d.Int())
			if v < caps.DurationSecMin || v > caps.DurationSecMax {
				return validationErr("duration",
					"model %q: duration must be between %d and %d seconds",
					model, caps.DurationSecMin, caps.DurationSecMax)
			}
		}
		if pi := gjson.GetBytes(body, "prompt_influence"); pi.Exists() {
			if pi.Float() < 0 || pi.Float() > 1 {
				return validationErr("prompt_influence", "must be between 0 and 1")
			}
		}
	}

	return nil
}

// ─────────────────────────────────────────────
// 图像请求校验
// ─────────────────────────────────────────────

// ValidateImageRequest 校验图像请求参数（JSON 或 multipart）。
func ValidateImageRequest(isEdits bool, contentType string, body []byte) *ValidationError {
	info := ParseImageRequest(contentType, body)
	model := strings.TrimSpace(info.Model)
	if model == "" {
		return validationErr("model", "model is required")
	}

	caps := LookupImageModel(model)
	if caps == nil {
		supported := make([]string, 0, len(imageModels))
		for _, img := range imageModels {
			supported = append(supported, img.Model)
		}
		return validationErr("model", "unsupported image model %q; supported: %s",
			model, strings.Join(supported, ", "))
	}

	if isEdits && !caps.SupportsEdits {
		return validationErr("model",
			"model %q does not support reference-image generation", model)
	}

	if strings.TrimSpace(info.Prompt) == "" {
		return validationErr("prompt", "prompt is required")
	}
	if len([]rune(info.Prompt)) > 9999 {
		return validationErr("prompt", "prompt must not exceed 9999 characters")
	}

	if size := strings.TrimSpace(info.Size); size != "" && !strings.EqualFold(size, "auto") {
		if !validImageSizeForModel(model, size) {
			return validationErr("size", "model %q does not support image size %q", model, size)
		}
	}

	// n
	//
	// 用 NMaxPerRequest 而非 NMax：后者是前端的并发扇出预算（N 个各 n=1 的请求），
	// 拿它当单请求上限会放过必被上游拒的组合（如 gpt-image-2 + n=3）。
	if info.NPresent {
		nMax := caps.NMaxPerRequest
		if nMax <= 0 {
			nMax = caps.NMax
		}
		if info.N < 1 || info.N > nMax {
			return validationErr("n",
				"model %q: n must be between 1 and %d", model, nMax)
		}
	}

	if isEdits {
		imageCount := countMultipartFileFields(contentType, body, "image")
		if imageCount == 0 {
			return validationErr("image", "at least one reference image is required")
		}
		if imageCount > 6 {
			return validationErr("image", "at most 6 reference images are allowed")
		}
	}

	return nil
}

var gptImage2Widths = intSet(
	768, 832, 848, 864, 896, 928, 1024, 1136, 1152, 1184, 1200, 1248, 1264,
	1344, 1376, 1536, 1584, 1648, 1696, 1792, 1856, 2016, 2048, 2336, 2448,
	2560, 2880, 3200, 3264, 3504, 3584, 3808,
)

var gptImage2Heights = intSet(
	672, 768, 832, 848, 864, 896, 928, 1024, 1136, 1152, 1184, 1200, 1248,
	1264, 1344, 1376, 1536, 1632, 1648, 1696, 1792, 1856, 2016, 2048, 2336,
	2448, 2560, 2880, 3200, 3264, 3504, 3584,
)

var nanoBananaWidths = intSet(
	768, 848, 896, 928, 1024, 1152, 1200, 1264, 1376, 1536, 1584, 1696, 1792,
	1856, 2048, 2304, 2400, 2528, 2752, 3072, 3168, 3392, 3584, 3712, 4096,
	4608, 4800, 5056, 5504, 6336,
)

var nanoBananaHeights = intSet(
	672, 768, 848, 896, 928, 1024, 1152, 1200, 1264, 1344, 1376, 1536, 1696,
	1792, 1856, 2048, 2304, 2400, 2528, 2688, 2752, 3072, 3392, 3584, 3712,
	4096, 4608, 4800, 5056, 5504,
)

func intSet(values ...int) map[int]struct{} {
	out := make(map[int]struct{}, len(values))
	for _, value := range values {
		out[value] = struct{}{}
	}
	return out
}

func validImageSizeForModel(model, size string) bool {
	width, height, ok := parseImageDimensionsStrict(size)
	if !ok {
		return false
	}
	switch model {
	case "gpt-image-2":
		_, widthOK := gptImage2Widths[width]
		_, heightOK := gptImage2Heights[height]
		pixels := int64(width) * int64(height)
		longer, shorter := width, height
		if height > width {
			longer, shorter = height, width
		}
		return widthOK && heightOK && pixels >= 655360 && pixels <= 8294400 && longer <= 3*shorter
	case "nano-banana-2", "nano-banana-pro":
		_, widthOK := nanoBananaWidths[width]
		_, heightOK := nanoBananaHeights[height]
		return widthOK && heightOK
	case "seedream-5.0-pro":
		return width >= 768 && width <= 2048 && height >= 768 && height <= 2048
	default:
		return true
	}
}

func parseImageDimensionsStrict(size string) (int, int, bool) {
	left, right, ok := strings.Cut(strings.ToLower(strings.TrimSpace(size)), "x")
	if !ok || strings.Contains(right, "x") {
		return 0, 0, false
	}
	width, widthErr := strconv.Atoi(left)
	height, heightErr := strconv.Atoi(right)
	if widthErr != nil || heightErr != nil || width <= 0 || height <= 0 {
		return 0, 0, false
	}
	return width, height, true
}

// ─────────────────────────────────────────────
// 工具函数
// ─────────────────────────────────────────────

func validateDuration(caps *VideoModelCaps, dur int) *ValidationError {
	if dur <= 0 {
		return nil // 省略 duration，上游使用默认值
	}
	if caps.Duration.isEnum() {
		for _, v := range caps.Duration.Enum {
			if v == dur {
				return nil
			}
		}
		parts := make([]string, len(caps.Duration.Enum))
		for i, v := range caps.Duration.Enum {
			parts[i] = fmt.Sprintf("%d", v)
		}
		return validationErr("duration",
			"model %q only accepts duration values: %s",
			caps.Model, strings.Join(parts, ", "))
	}
	if dur < caps.Duration.Min || dur > caps.Duration.Max {
		return validationErr("duration",
			"model %q: duration must be between %d and %d",
			caps.Model, caps.Duration.Min, caps.Duration.Max)
	}
	return nil
}

func parseDurationStr(s string) (int, error) {
	var v int
	_, err := fmt.Sscanf(s, "%d", &v)
	return v, err
}

func containsStr(slice []string, val string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}

func containsField(fields []string, name string) bool {
	// 匹配 "name" 或 "name[]"（两种 multipart 字段命名风格）
	for _, f := range fields {
		if f == name || f == name+"[]" {
			return true
		}
	}
	return false
}

func capSupports(caps *VideoModelCaps, mode ReferenceMode) bool {
	for _, m := range caps.AllowedRefModes {
		if m == mode {
			return true
		}
	}
	return false
}

// extractMultipartFileFieldNames 从 multipart body 中提取文件字段名列表（不读取文件内容）。
func extractMultipartFileFieldNames(contentType string, body []byte) []string {
	mediaType, params, err := mime.ParseMediaType(strings.TrimSpace(contentType))
	if err != nil || !strings.EqualFold(mediaType, "multipart/form-data") {
		return nil
	}
	boundary := strings.TrimSpace(params["boundary"])
	if boundary == "" {
		return nil
	}
	reader := multipart.NewReader(bytes.NewReader(body), boundary)
	var names []string
	for {
		part, err := reader.NextPart()
		if err != nil {
			break
		}
		name := strings.TrimSpace(part.FormName())
		isFile := strings.TrimSpace(part.FileName()) != ""
		_, _ = io.Copy(io.Discard, io.LimitReader(part, 1))
		_ = part.Close()
		if name != "" && isFile {
			names = append(names, name)
		}
	}
	return names
}

func countMultipartFileFields(contentType string, body []byte, field string) int {
	count := 0
	for _, name := range extractMultipartFileFieldNames(contentType, body) {
		if name == field || name == field+"[]" {
			count++
		}
	}
	return count
}

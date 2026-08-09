package avi2api

// caps_dto.go 定义模型能力的对外 JSON 表示。
//
// 存在意义：本包的 registry 是 canvas 平台模型约束的**唯一权威来源**，
// infinite-canvas 前端在启动时拉取本 DTO 覆盖其内置兜底表，避免两份 caps
// 手工同步产生漂移（历史上曾因此出现 resolution/capability 判定错误）。
//
// 字段命名与前端 `canvas-model-caps.ts` 的类型逐一对应，改动需两侧同步。
// 三处表示差异必须在此转换：
//   - DurationConstraint：Go 靠 Enum 是否非空隐式判别，前端要显式 kind 标签
//   - GenAudioMode：Go 是 int 常量，对外必须是字符串
//   - ReferenceMode：Go 的 RefModeFrame 字面量是 "start_frame"（贴合 multipart
//     字段名），前端语义标签是 "frame"

// DurationDTO 时长约束。Kind 为 "enum" 时只有 Values 有效，
// 为 "range" 时只有 Min/Max 有效；Default 两种模式都有效。
type DurationDTO struct {
	Kind    string `json:"kind"`
	Values  []int  `json:"values,omitempty"`
	Min     int    `json:"min,omitempty"`
	Max     int    `json:"max,omitempty"`
	Default int    `json:"default"`
}

// VideoModelCapsDTO 视频模型能力。
type VideoModelCapsDTO struct {
	Model                        string      `json:"model"`
	Duration                     DurationDTO `json:"duration"`
	AllowedSizes                 []string    `json:"allowedSizes"`
	DefaultSize                  string      `json:"defaultSize"`
	AllowedResolutions           []string    `json:"allowedResolutions"`
	DefaultResolution            string      `json:"defaultResolution"`
	GenerateAudio                string      `json:"generateAudio"`
	AllowedRefModes              []string    `json:"allowedRefModes"`
	ImageMaxCount                int         `json:"imageMaxCount"`
	VideoMaxCount                int         `json:"videoMaxCount"`
	AudioMaxCount                int         `json:"audioMaxCount"`
	AudioRefRequiresImageOrVideo bool        `json:"audioRefRequiresImageOrVideo"`
	ImageRefFixesSize            bool        `json:"imageRefFixesSize"`
	ImageRefFixesDuration        int         `json:"imageRefFixesDuration"`
}

// ImageModelCapsDTO 图像模型能力。
type ImageModelCapsDTO struct {
	Model                string `json:"model"`
	NMax                 int    `json:"nMax"`
	NDefault             int    `json:"nDefault"`
	HasQuality           bool   `json:"hasQuality"`
	SupportsEdits        bool   `json:"supportsEdits"`
	HasReferenceStrength bool   `json:"hasReferenceStrength"`
}

// AudioModelCapsDTO 音频模型能力。
type AudioModelCapsDTO struct {
	Model                string   `json:"model"`
	NMin                 int      `json:"nMin"`
	NMax                 int      `json:"nMax"`
	NDefault             int      `json:"nDefault"`
	DurationSecMin       int      `json:"durationSecMin"`
	DurationSecMax       int      `json:"durationSecMax"`
	DurationSecDefault   int      `json:"durationSecDefault"`
	DurationMinMin       int      `json:"durationMinMin"`
	DurationMinMax       int      `json:"durationMinMax"`
	DurationMinDefault   int      `json:"durationMinDefault"`
	AllowedVoices        []string `json:"allowedVoices"`
	HasForceInstrumental bool     `json:"hasForceInstrumental"`
	HasLoop              bool     `json:"hasLoop"`
	HasPromptInfluence   bool     `json:"hasPromptInfluence"`
}

// ModelCapsDTO 是能力接口的完整响应体。
type ModelCapsDTO struct {
	Video []VideoModelCapsDTO `json:"video"`
	Image []ImageModelCapsDTO `json:"image"`
	Audio []AudioModelCapsDTO `json:"audio"`
}

// genAudioModeString 把内部枚举转成对外字符串。
// 与前端 GenAudioMode 联合类型一致；未知值按 optional 处理（最宽松，
// 让上游而非本地校验去拒绝，避免前端误禁一个实际可用的字段）。
func genAudioModeString(m GenAudioMode) string {
	switch m {
	case GenAudioAlwaysTrue:
		return "always-true"
	case GenAudioUnsupported:
		return "unsupported"
	default:
		return "optional"
	}
}

// refModeString 把内部参考模式转成前端语义标签。
// RefModeFrame 的字面量是 multipart 字段名 "start_frame"，前端用 "frame"。
func refModeString(m ReferenceMode) string {
	if m == RefModeFrame {
		return "frame"
	}
	return string(m)
}

// strSlice 拷贝字符串切片，并保证空值序列化成 [] 而非 null。
// 前端会对这些字段直接做 .includes() / .map()，拿到 null 会抛异常。
func strSlice(in []string) []string {
	out := make([]string, 0, len(in))
	return append(out, in...)
}

// durationDTO 把隐式判别的 DurationConstraint 转成带 kind 标签的形式。
func durationDTO(d DurationConstraint) DurationDTO {
	if d.isEnum() {
		return DurationDTO{
			Kind:    "enum",
			Values:  append([]int(nil), d.Enum...),
			Default: d.Default,
		}
	}
	return DurationDTO{
		Kind:    "range",
		Min:     d.Min,
		Max:     d.Max,
		Default: d.Default,
	}
}

// AllModelCapsDTO 返回全部已注册模型的能力描述，供前端拉取。
// 切片顺序与注册表声明顺序一致，保证响应稳定（便于前端缓存比对与调试）。
func AllModelCapsDTO() ModelCapsDTO {
	out := ModelCapsDTO{
		Video: make([]VideoModelCapsDTO, 0, len(videoModels)),
		Image: make([]ImageModelCapsDTO, 0, len(imageModels)),
		Audio: make([]AudioModelCapsDTO, 0, len(audioModels)),
	}

	for _, c := range videoModels {
		refModes := make([]string, 0, len(c.AllowedRefModes))
		for _, m := range c.AllowedRefModes {
			refModes = append(refModes, refModeString(m))
		}
		out.Video = append(out.Video, VideoModelCapsDTO{
			Model:                        c.Model,
			Duration:                     durationDTO(c.Duration),
			AllowedSizes:                 strSlice(c.AllowedSizes),
			DefaultSize:                  c.DefaultSize,
			AllowedResolutions:           strSlice(c.AllowedResolutions),
			DefaultResolution:            c.DefaultResolution,
			GenerateAudio:                genAudioModeString(c.GenerateAudio),
			AllowedRefModes:              refModes,
			ImageMaxCount:                c.ImageMaxCount,
			VideoMaxCount:                c.VideoMaxCount,
			AudioMaxCount:                c.AudioMaxCount,
			AudioRefRequiresImageOrVideo: c.AudioRefRequiresImageOrVideo,
			ImageRefFixesSize:            c.ImageRefFixesSize,
			ImageRefFixesDuration:        c.ImageRefFixesDuration,
		})
	}

	for _, c := range imageModels {
		out.Image = append(out.Image, ImageModelCapsDTO{
			Model:                c.Model,
			NMax:                 c.NMax,
			NDefault:             c.NDefault,
			HasQuality:           c.HasQuality,
			SupportsEdits:        c.SupportsEdits,
			HasReferenceStrength: c.HasReferenceStrength,
		})
	}

	for _, c := range audioModels {
		out.Audio = append(out.Audio, AudioModelCapsDTO{
			Model:                c.Model,
			NMin:                 c.NMin,
			NMax:                 c.NMax,
			NDefault:             c.NDefault,
			DurationSecMin:       c.DurationSecMin,
			DurationSecMax:       c.DurationSecMax,
			DurationSecDefault:   c.DurationSecDefault,
			DurationMinMin:       c.DurationMinMin,
			DurationMinMax:       c.DurationMinMax,
			DurationMinDefault:   c.DurationMinDefault,
			AllowedVoices:        strSlice(c.AllowedVoices),
			HasForceInstrumental: c.HasForceInstrumental,
			HasLoop:              c.HasLoop,
			HasPromptInfluence:   c.HasPromptInfluence,
		})
	}

	return out
}

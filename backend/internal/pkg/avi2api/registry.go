// Package avi2api — registry.go
// 模型能力注册表：定义 canvas 平台所有支持的视频/图像/音频模型及其参数约束。
//
// 数据来源：AIV2API OpenAPI v2.0.0（/openapi.json）。
// 上游更新模型时，只需在此文件追加或修改对应条目，校验逻辑（validate.go）无需改动。
package avi2api

// ─────────────────────────────────────────────
// 通用约束类型
// ─────────────────────────────────────────────

// DurationConstraint 描述 duration 参数的合法范围。
// Enum 非空时，只接受 Enum 中的整数值；否则接受 [Min, Max] 闭区间内的整数。
type DurationConstraint struct {
	Min     int
	Max     int
	Enum    []int // 非空时优先使用枚举
	Default int
}

func (d DurationConstraint) isEnum() bool { return len(d.Enum) > 0 }

// GenAudioMode 描述 generate_audio 字段的支持状态。
type GenAudioMode int

const (
	GenAudioOptional    GenAudioMode = iota // 可选布尔值，默认 true
	GenAudioAlwaysTrue                      // 恒为 true（如 minimax-h3），传 false 会被拒绝
	GenAudioUnsupported                     // 上游 schema 无此字段（如 gemini-omni-flash）
)

// ReferenceMode 标识 multipart 请求中携带的参考素材类型。
type ReferenceMode string

const (
	RefModeImage ReferenceMode = "image"       // 参考图（image / image[]）
	RefModeFrame ReferenceMode = "start_frame" // 首尾帧（start_frame + 可选 end_frame）
	RefModeVideo ReferenceMode = "video"       // 参考视频（video / video[]）
	RefModeAudio ReferenceMode = "audio"       // 参考音频（audio / audio[]）
)

// ─────────────────────────────────────────────
// 视频模型能力
// ─────────────────────────────────────────────

// VideoModelCaps 描述单个视频模型的全部参数约束。
type VideoModelCaps struct {
	Model string

	Duration           DurationConstraint
	AllowedSizes       []string
	DefaultSize        string
	AllowedResolutions []string
	DefaultResolution  string

	GenerateAudio GenAudioMode

	// AllowedRefModes 列出此模型支持的 multipart 参考素材类型。
	// 空表示只支持纯 JSON（无参考模式）。
	AllowedRefModes []ReferenceMode

	// ImageRefConstraints 描述图像参考的限制（最多几张）。
	ImageMaxCount int

	// VideoRefConstraints 描述视频参考的限制（最多几个）。
	VideoMaxCount int

	// AudioRefConstraints 描述音频参考的限制（最多几个）。
	AudioMaxCount int

	// AudioRefRequiresImageOrVideo 为 true 时，audio 参考必须同时提供 image 或 video。
	AudioRefRequiresImageOrVideo bool

	// ImageRefFixesSize 为 true 时，携带图像参考会强制 size=1280x720（veo-3.1 only）。
	ImageRefFixesSize bool

	// ImageRefFixesDuration 非零时，携带图像参考会强制 duration=该值。
	ImageRefFixesDuration int
}

// videoRegistry 是视频模型注册表（模型名 → 能力描述）。
var videoRegistry = buildVideoRegistry()

func buildVideoRegistry() map[string]*VideoModelCaps {
	m := map[string]*VideoModelCaps{}
	for _, c := range videoModels {
		c := c // 避免循环变量陷阱
		m[c.Model] = &c
	}
	return m
}

// LookupVideoModel 返回视频模型能力，未知模型返回 nil。
func LookupVideoModel(model string) *VideoModelCaps {
	return videoRegistry[model]
}

// AllVideoModels 返回所有已注册视频模型名称列表（有序）。
func AllVideoModels() []string {
	names := make([]string, 0, len(videoModels))
	for _, c := range videoModels {
		names = append(names, c.Model)
	}
	return names
}

// AllImageModels 返回所有已注册图像模型名称列表（有序）。
func AllImageModels() []string {
	names := make([]string, 0, len(imageModels))
	for _, c := range imageModels {
		names = append(names, c.Model)
	}
	return names
}

// AllAudioModels 返回所有已注册音频模型名称列表（有序）。
func AllAudioModels() []string {
	names := make([]string, 0, len(audioModels))
	for _, c := range audioModels {
		names = append(names, c.Model)
	}
	return names
}

// AllModels 返回 canvas 平台全部模型（图像 + 视频 + 音频），用于分组模型列表候选。
func AllModels() []string {
	names := make([]string, 0, len(imageModels)+len(videoModels)+len(audioModels))
	names = append(names, AllImageModels()...)
	names = append(names, AllVideoModels()...)
	names = append(names, AllAudioModels()...)
	return names
}

var videoModels = []VideoModelCaps{
	// ── FLUX 3 Video ──────────────────────────────────────────────────
	// 上游 size 与 resolution 价位档强绑定（720p / 1080p），此处平铺全部
	// 合法 size；size↔resolution 组合规则交由上游二次校验。
	{
		Model:    "flux-3-video",
		Duration: DurationConstraint{Min: 5, Max: 20, Default: 8},
		AllowedSizes: []string{
			// 720p 档
			"1470x630", "1360x680", "1280x720", "1112x834", "960x960", "834x1112", "720x1280",
			// 1080p 档
			"2520x1080", "2160x1080", "1920x1080", "1440x1080", "1440x1440", "1080x1440", "1080x1920",
		},
		DefaultSize:        "1280x720",
		AllowedResolutions: []string{"720p", "1080p"},
		DefaultResolution:  "720p",
		GenerateAudio:      GenAudioOptional,
		// 首尾帧 或 单个视频续写文件（二选一），不支持图片参考
		AllowedRefModes: []ReferenceMode{RefModeFrame, RefModeVideo},
		VideoMaxCount:   1,
	},
	// ── Seedance 2.0（旗舰）────────────────────────────────────────────
	{
		Model:                        "seedance-2.0",
		Duration:                     DurationConstraint{Min: 4, Max: 15, Default: 8},
		AllowedSizes:                 []string{"1280x720", "720x1280"},
		DefaultSize:                  "1280x720",
		AllowedResolutions:           []string{"480p", "720p", "1080p", "2160p"},
		DefaultResolution:            "720p",
		GenerateAudio:                GenAudioOptional,
		AllowedRefModes:              []ReferenceMode{RefModeImage, RefModeFrame, RefModeVideo, RefModeAudio},
		ImageMaxCount:                4,
		VideoMaxCount:                3,
		AudioMaxCount:                1,
		AudioRefRequiresImageOrVideo: true,
	},
	// ── Seedance 2.0 Fast ─────────────────────────────────────────────
	{
		Model:                        "seedance-2.0-fast",
		Duration:                     DurationConstraint{Min: 4, Max: 15, Default: 8},
		AllowedSizes:                 []string{"1280x720", "720x1280"},
		DefaultSize:                  "1280x720",
		AllowedResolutions:           []string{"480p", "720p"},
		DefaultResolution:            "720p",
		GenerateAudio:                GenAudioOptional,
		AllowedRefModes:              []ReferenceMode{RefModeImage, RefModeFrame, RefModeVideo, RefModeAudio},
		ImageMaxCount:                4,
		VideoMaxCount:                3,
		AudioMaxCount:                1,
		AudioRefRequiresImageOrVideo: true,
	},
	// ── Seedance 2.0 Mini ─────────────────────────────────────────────
	{
		Model:                        "seedance-2.0-mini",
		Duration:                     DurationConstraint{Min: 4, Max: 15, Default: 8},
		AllowedSizes:                 []string{"1280x720", "720x1280"},
		DefaultSize:                  "1280x720",
		AllowedResolutions:           []string{"480p", "720p"},
		DefaultResolution:            "720p",
		GenerateAudio:                GenAudioOptional,
		AllowedRefModes:              []ReferenceMode{RefModeImage, RefModeFrame, RefModeVideo, RefModeAudio},
		ImageMaxCount:                4,
		VideoMaxCount:                3,
		AudioMaxCount:                1,
		AudioRefRequiresImageOrVideo: true,
	},
	// ── Veo 3.1（标准）────────────────────────────────────────────────
	{
		Model:              "veo-3.1",
		Duration:           DurationConstraint{Enum: []int{4, 6, 8}, Default: 8},
		AllowedSizes:       []string{"1280x720", "720x1280"},
		DefaultSize:        "1280x720",
		AllowedResolutions: []string{"720p", "1080p", "2160p"},
		DefaultResolution:  "720p",
		GenerateAudio:      GenAudioOptional,
		AllowedRefModes:    []ReferenceMode{RefModeImage, RefModeFrame},
		ImageMaxCount:      3,
		// 携带普通图片参考时 size 和 duration 被上游固定：size=1280x720, duration=8
		ImageRefFixesSize:     true,
		ImageRefFixesDuration: 8,
	},
	// ── Veo 3.1 Fast ──────────────────────────────────────────────────
	{
		Model:              "veo-3.1-fast",
		Duration:           DurationConstraint{Enum: []int{4, 6, 8}, Default: 8},
		AllowedSizes:       []string{"1280x720", "720x1280"},
		DefaultSize:        "1280x720",
		AllowedResolutions: []string{"720p", "1080p", "2160p"},
		DefaultResolution:  "720p",
		GenerateAudio:      GenAudioOptional,
		// 仅支持首尾帧参考（start_frame 必填）
		AllowedRefModes: []ReferenceMode{RefModeFrame},
	},
	// ── Kling O3 Omni ─────────────────────────────────────────────────
	// size 与 resolution 价位档强绑定（720p / 1080p / 2160p）。
	{
		Model:    "kling-o3-omni",
		Duration: DurationConstraint{Min: 3, Max: 15, Default: 5},
		AllowedSizes: []string{
			// 720p 档
			"1280x720", "720x1280", "960x960",
			// 1080p 档
			"1920x1080", "1080x1920", "1440x1440",
			// 2160p 档
			"3840x2160", "2160x3840", "2880x2880",
		},
		DefaultSize:        "1920x1080",
		AllowedResolutions: []string{"720p", "1080p", "2160p"},
		DefaultResolution:  "1080p",
		GenerateAudio:      GenAudioOptional,
		// 首尾帧 / 最多 7 张图片 / 单个视频（含 reference_strength）
		AllowedRefModes: []ReferenceMode{RefModeImage, RefModeFrame, RefModeVideo},
		ImageMaxCount:   7,
		VideoMaxCount:   1,
	},
	// ── Grok Imagine 1.5 ──────────────────────────────────────────────
	// 仅支持首尾帧参考（start_frame 必填），拒绝其他所有参考字段。
	// size 与 resolution 价位档强绑定（480p / 720p / 1080p）。
	{
		Model:    "grok-imagine-1.5",
		Duration: DurationConstraint{Min: 3, Max: 15, Default: 6},
		AllowedSizes: []string{
			// 480p 档
			"736x400", "400x736", "544x544",
			// 720p 档
			"1280x720", "720x1280", "960x960",
			// 1080p 档
			"1888x1072", "1072x1888", "1424x1424",
		},
		DefaultSize:        "736x400",
		AllowedResolutions: []string{"480p", "720p", "1080p"},
		DefaultResolution:  "480p",
		GenerateAudio:      GenAudioOptional,
		AllowedRefModes:    []ReferenceMode{RefModeFrame},
	},
	// ── MiniMax H3 ────────────────────────────────────────────────────
	{
		Model:    "minimax-h3",
		Duration: DurationConstraint{Min: 5, Max: 15, Default: 5},
		AllowedSizes: []string{
			"3360x1440", "2560x1440", "1920x1440",
			"1440x1440", "1440x1920", "1440x2560",
		},
		DefaultSize:        "2560x1440",
		AllowedResolutions: []string{"1440p"},
		DefaultResolution:  "1440p",
		// generate_audio 恒为 true，传 false 上游会拒绝
		GenerateAudio:   GenAudioAlwaysTrue,
		AllowedRefModes: []ReferenceMode{RefModeImage, RefModeFrame, RefModeAudio},
		ImageMaxCount:   5,
		AudioMaxCount:   3,
		// MiniMax H3 的音频参考需要普通图片参考（非首尾帧）
		AudioRefRequiresImageOrVideo: false, // 特殊：仅需 image
	},
}

// ─────────────────────────────────────────────
// 音频模型能力
// ─────────────────────────────────────────────

// AudioModelCaps 描述单个音频模型的参数约束。
type AudioModelCaps struct {
	Model string

	// N：生成数量
	NMin     int
	NMax     int
	NDefault int

	// Duration（秒）：dialogue-v3/sound-effects-v2 使用
	DurationSecMin     int
	DurationSecMax     int
	DurationSecDefault int

	// DurationMinutes：music-v1 专用（分钟数）
	DurationMinMin     int
	DurationMinMax     int
	DurationMinDefault int

	// 特有字段
	AllowedVoices        []string // dialogue-v3：voice 枚举
	HasForceInstrumental bool     // music-v1：force_instrumental
	HasLoop              bool     // sound-effects-v2：loop
	HasPromptInfluence   bool     // dialogue-v3 / sound-effects-v2
}

var audioRegistry = buildAudioRegistry()

func buildAudioRegistry() map[string]*AudioModelCaps {
	m := map[string]*AudioModelCaps{}
	for _, c := range audioModels {
		c := c
		m[c.Model] = &c
	}
	return m
}

// LookupAudioModel 返回音频模型能力，未知模型返回 nil。
func LookupAudioModel(model string) *AudioModelCaps {
	return audioRegistry[model]
}

var audioModels = []AudioModelCaps{
	// ── dialogue-v3（TTS）─────────────────────────────────────────────
	{
		Model: "dialogue-v3",
		NMin:  1, NMax: 4, NDefault: 1,
		HasPromptInfluence: true,
		AllowedVoices: []string{
			"roger", "sarah", "laura", "charlie", "george", "callum",
			"river", "harry", "liam", "alice", "matilda", "will",
			"jessica", "eric", "bella", "chris", "brian", "daniel",
			"lily", "adam", "bill",
		},
	},
	// ── music-v1（音乐生成）────────────────────────────────────────────
	{
		Model: "music-v1",
		NMin:  1, NMax: 4, NDefault: 1,
		DurationMinMin:       1,
		DurationMinMax:       10,
		DurationMinDefault:   1,
		HasForceInstrumental: true,
	},
	// ── sound-effects-v2（音效）────────────────────────────────────────
	{
		Model: "sound-effects-v2",
		NMin:  1, NMax: 4, NDefault: 1,
		DurationSecMin:     1,
		DurationSecMax:     22,
		DurationSecDefault: 2,
		HasLoop:            true,
		HasPromptInfluence: true,
	},
}

// ─────────────────────────────────────────────
// 图像模型能力
// ─────────────────────────────────────────────

// ImageModelCaps 描述单个图像模型的参数约束。
type ImageModelCaps struct {
	Model string

	// N：生成数量
	NMax     int
	NDefault int

	// NMaxPerRequest 是**单次上游请求**允许的最大 n，与 NMax 语义不同：
	// NMax 是前端一次生成动作的并发预算（开 N 个各 n=1 的独立请求），
	// NMaxPerRequest 才是上游 schema 对单请求 n 的硬上限。
	//
	// 两者混用会让 n=3 + gpt-image-2 在本地校验通过、被上游 400 拒
	// （上游 x-model-rules 对该模型的 n 最大值是 1）。
	// 为 0 时回落到 NMax（即未单独声明的模型沿用旧行为）。
	NMaxPerRequest int

	// Quality：gpt-image-2 专用
	HasQuality bool

	// 是否支持 /images/generations 的 multipart 图生图模式
	SupportsEdits bool

	// ReferenceStrength：multipart 图生图参考强度
	HasReferenceStrength bool
}

var imageRegistry = buildImageRegistry()

func buildImageRegistry() map[string]*ImageModelCaps {
	m := map[string]*ImageModelCaps{}
	for _, c := range imageModels {
		c := c
		m[c.Model] = &c
	}
	return m
}

// LookupImageModel 返回图像模型能力，未知模型返回 nil。
func LookupImageModel(model string) *ImageModelCaps {
	return imageRegistry[model]
}

var imageModels = []ImageModelCaps{
	{
		Model: "gpt-image-2",
		NMax:  6, NDefault: 1, // 前端并发6个独立请求各 n=1，不受 Leonardo 单次限制
		NMaxPerRequest:       1, // 上游 schema 对该模型单请求只允许 1 张
		HasQuality:           true,
		SupportsEdits:        true,
		HasReferenceStrength: true,
	},
	{
		Model: "nano-banana-2",
		NMax:  6, NDefault: 1,
		NMaxPerRequest:       4,
		SupportsEdits:        true,
		HasReferenceStrength: true,
	},
	{
		Model: "nano-banana-pro",
		NMax:  6, NDefault: 1,
		NMaxPerRequest:       4,
		SupportsEdits:        true,
		HasReferenceStrength: true,
	},
	{
		Model: "seedream-5.0-pro",
		NMax:  6, NDefault: 1,
		NMaxPerRequest:       4,
		SupportsEdits:        true,
		HasReferenceStrength: true,
	},
}

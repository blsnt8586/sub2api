//go:build unit

package avi2api_test

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/avi2api"
	"github.com/stretchr/testify/require"
)

// DTO 是 infinite-canvas 前端的唯一 caps 来源，任何字段遗漏都会让前端退回
// 内置兜底表并悄悄产生漂移。这些测试锁定「全覆盖」与三处表示转换。

func TestAllModelCapsDTOCoversEveryRegisteredModel(t *testing.T) {
	dto := avi2api.AllModelCapsDTO()

	videoNames := make([]string, 0, len(dto.Video))
	for _, c := range dto.Video {
		videoNames = append(videoNames, c.Model)
	}
	imageNames := make([]string, 0, len(dto.Image))
	for _, c := range dto.Image {
		imageNames = append(imageNames, c.Model)
	}
	audioNames := make([]string, 0, len(dto.Audio))
	for _, c := range dto.Audio {
		audioNames = append(audioNames, c.Model)
	}

	// 顺序也要一致：前端按响应顺序渲染下拉，稳定顺序便于缓存比对
	require.Equal(t, avi2api.AllVideoModels(), videoNames, "video DTO 必须与注册表等价且同序")
	require.Equal(t, avi2api.AllImageModels(), imageNames, "image DTO 必须与注册表等价且同序")
	require.Equal(t, avi2api.AllAudioModels(), audioNames, "audio DTO 必须与注册表等价且同序")
}

func TestVideoCapsDTOMatchesRegistry(t *testing.T) {
	dto := avi2api.AllModelCapsDTO()
	for _, got := range dto.Video {
		caps := avi2api.LookupVideoModel(got.Model)
		require.NotNil(t, caps, "DTO 里的模型必须能在注册表查到: %s", got.Model)

		require.Equal(t, caps.AllowedSizes, got.AllowedSizes, "%s.allowedSizes", got.Model)
		require.Equal(t, caps.DefaultSize, got.DefaultSize, "%s.defaultSize", got.Model)
		require.Equal(t, caps.AllowedResolutions, got.AllowedResolutions, "%s.allowedResolutions", got.Model)
		require.Equal(t, caps.DefaultResolution, got.DefaultResolution, "%s.defaultResolution", got.Model)
		require.Equal(t, caps.ImageMaxCount, got.ImageMaxCount, "%s.imageMaxCount", got.Model)
		require.Equal(t, caps.VideoMaxCount, got.VideoMaxCount, "%s.videoMaxCount", got.Model)
		require.Equal(t, caps.AudioMaxCount, got.AudioMaxCount, "%s.audioMaxCount", got.Model)
		require.Equal(t, caps.ImageRefFixesDuration, got.ImageRefFixesDuration, "%s.imageRefFixesDuration", got.Model)

		// resolution 必须带 p 后缀——曾因前端剥后缀比较导致所有 canvas 视频模型
		// 的 resolution 参数被上游拒绝
		for _, res := range got.AllowedResolutions {
			require.Regexp(t, `^\d+p$`, res, "%s 的 resolution 必须形如 720p", got.Model)
		}
		require.Regexp(t, `^\d+p$`, got.DefaultResolution, "%s.defaultResolution", got.Model)
		require.Contains(t, got.AllowedResolutions, got.DefaultResolution,
			"%s 的默认分辨率必须在允许列表内", got.Model)
		require.Contains(t, got.AllowedSizes, got.DefaultSize,
			"%s 的默认尺寸必须在允许列表内", got.Model)
	}
}

func TestDurationDTOCarriesExplicitKind(t *testing.T) {
	dto := avi2api.AllModelCapsDTO()
	sawEnum, sawRange := false, false

	for _, c := range dto.Video {
		switch c.Duration.Kind {
		case "enum":
			sawEnum = true
			require.NotEmpty(t, c.Duration.Values, "%s enum 模式必须有 values", c.Model)
			require.Contains(t, c.Duration.Values, c.Duration.Default,
				"%s enum 默认值必须在 values 内", c.Model)
		case "range":
			sawRange = true
			require.Positive(t, c.Duration.Max, "%s range 模式必须有 max", c.Model)
			require.LessOrEqual(t, c.Duration.Min, c.Duration.Default, "%s range 默认值下界", c.Model)
			require.LessOrEqual(t, c.Duration.Default, c.Duration.Max, "%s range 默认值上界", c.Model)
		default:
			t.Fatalf("%s 的 duration.kind 非法: %q", c.Model, c.Duration.Kind)
		}
	}

	// 两种模式都要有样本，否则说明转换分支有一条从未被覆盖
	require.True(t, sawEnum, "注册表应至少有一个 enum 时长模型（如 veo-3.1）")
	require.True(t, sawRange, "注册表应至少有一个 range 时长模型（如 seedance-2.0）")
}

func TestGenerateAudioSerializedAsString(t *testing.T) {
	dto := avi2api.AllModelCapsDTO()
	valid := map[string]bool{"optional": true, "always-true": true, "unsupported": true}
	seen := map[string]bool{}

	for _, c := range dto.Video {
		require.True(t, valid[c.GenerateAudio],
			"%s 的 generateAudio 值非法: %q", c.Model, c.GenerateAudio)
		seen[c.GenerateAudio] = true
	}

	// minimax-h3 恒为 true 的边界值必须能正确序列化。
	// 注：上游 v1.5.0 起已无 GenAudioUnsupported 模型（gemini-omni-flash 已下架），
	// "unsupported" 分支的序列化由 caps_dto 单测（genAudioModeString）单独保证。
	require.True(t, seen["always-true"], "应有 always-true 模型（minimax-h3）")
}

func TestRefModeFrameMappedToSemanticLabel(t *testing.T) {
	dto := avi2api.AllModelCapsDTO()
	sawFrame := false

	for _, c := range dto.Video {
		for _, mode := range c.AllowedRefModes {
			// 内部字面量是 multipart 字段名 start_frame，对外必须是语义标签 frame
			require.NotEqual(t, "start_frame", mode,
				"%s 的 refMode 不应泄露 multipart 字段名", c.Model)
			require.Contains(t, []string{"image", "frame", "video", "audio"}, mode,
				"%s 的 refMode 值非法: %q", c.Model, mode)
			if mode == "frame" {
				sawFrame = true
			}
		}
	}
	require.True(t, sawFrame, "应有支持首尾帧的模型（veo-3.1）")
}

func TestModelCapsDTOSerializesToStableJSON(t *testing.T) {
	dto := avi2api.AllModelCapsDTO()
	raw, err := json.Marshal(dto)
	require.NoError(t, err)

	// 前端按 camelCase 读取，字段名错了会静默拿到 undefined
	var probe struct {
		Video []struct {
			Model              string   `json:"model"`
			AllowedResolutions []string `json:"allowedResolutions"`
			DefaultResolution  string   `json:"defaultResolution"`
			GenerateAudio      string   `json:"generateAudio"`
			Duration           struct {
				Kind    string `json:"kind"`
				Default int    `json:"default"`
			} `json:"duration"`
		} `json:"video"`
		Image []struct {
			Model      string `json:"model"`
			HasQuality bool   `json:"hasQuality"`
		} `json:"image"`
		Audio []struct {
			Model         string   `json:"model"`
			AllowedVoices []string `json:"allowedVoices"`
		} `json:"audio"`
	}
	require.NoError(t, json.Unmarshal(raw, &probe))
	require.NotEmpty(t, probe.Video)
	require.NotEmpty(t, probe.Image)
	require.NotEmpty(t, probe.Audio)

	for _, v := range probe.Video {
		require.NotEmpty(t, v.Model)
		require.NotEmpty(t, v.AllowedResolutions, "%s 的 allowedResolutions 不应为空", v.Model)
		require.NotEmpty(t, v.DefaultResolution, "%s 的 defaultResolution 不应为空", v.Model)
		require.NotEmpty(t, v.GenerateAudio, "%s 的 generateAudio 不应为空", v.Model)
		require.NotEmpty(t, v.Duration.Kind, "%s 的 duration.kind 不应为空", v.Model)
	}

	// allowedVoices 为空的模型（music-v1）序列化后应是 [] 或缺省，不能是 null——
	// 前端会对它做 .includes()，null 会抛异常
	require.NotContains(t, string(raw), `"allowedVoices":null`,
		"allowedVoices 不能序列化成 null")
}

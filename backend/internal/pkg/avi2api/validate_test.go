//go:build unit

package avi2api_test

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/avi2api"
	"github.com/stretchr/testify/require"
)

// ─── 视频注册表 ────────────────────────────────────────────────────

func TestLookupVideoModelKnown(t *testing.T) {
	for _, name := range avi2api.AllVideoModels() {
		caps := avi2api.LookupVideoModel(name)
		require.NotNil(t, caps, "model %s should be in registry", name)
		require.Equal(t, name, caps.Model)
	}
}

func TestLookupVideoModelUnknown(t *testing.T) {
	require.Nil(t, avi2api.LookupVideoModel("nonexistent-model"))
}

// ─── 视频 JSON 校验 ────────────────────────────────────────────────

func TestValidateVideoRequest_UnknownModel(t *testing.T) {
	body := `{"model":"mystery-model-99","prompt":"test"}`
	err := avi2api.ValidateVideoRequest("application/json", []byte(body))
	require.NotNil(t, err)
	require.Equal(t, "model", err.Field)
}

func TestValidateVideoRequest_MissingModel(t *testing.T) {
	body := `{"prompt":"test"}`
	err := avi2api.ValidateVideoRequest("application/json", []byte(body))
	require.NotNil(t, err)
	require.Equal(t, "model", err.Field)
}

func TestValidateVideoRequest_ValidJSON(t *testing.T) {
	cases := []struct {
		model string
		extra string
	}{
		{"seedance-2.0", `"duration":8,"size":"1280x720","resolution":"720p"`},
		{"seedance-2.0-fast", `"duration":4`},
		{"seedance-2.0-mini", `"resolution":"480p"`},
		{"gemini-omni-flash", `"duration":5`},
		{"veo-3.1", `"duration":8`},
		{"veo-3.1-fast", `"duration":4`},
		{"veo-3.1-lite", `"duration":6`},
		{"kling-3.0", `"duration":5`},
		{"minimax-h3", `"size":"2560x1440"`},
	}
	for _, tc := range cases {
		t.Run(tc.model, func(t *testing.T) {
			body := fmt.Sprintf(`{"model":%q,"prompt":"test",%s}`, tc.model, tc.extra)
			err := avi2api.ValidateVideoRequest("application/json", []byte(body))
			require.Nil(t, err, "model %s should pass validation: %v", tc.model, err)
		})
	}
}

func TestValidateVideoRequest_DurationEnumViolation(t *testing.T) {
	// veo-3.1 只接受 4/6/8
	body := `{"model":"veo-3.1","prompt":"test","duration":10}`
	err := avi2api.ValidateVideoRequest("application/json", []byte(body))
	require.NotNil(t, err)
	require.Equal(t, "duration", err.Field)
}

func TestValidateVideoRequest_DurationRangeViolation(t *testing.T) {
	// seedance-2.0 接受 4–15，传 2 非法
	body := `{"model":"seedance-2.0","prompt":"test","duration":2}`
	err := avi2api.ValidateVideoRequest("application/json", []byte(body))
	require.NotNil(t, err)
	require.Equal(t, "duration", err.Field)
}

func TestValidateVideoRequest_InvalidSize(t *testing.T) {
	// minimax-h3 只有 6 种 1440p 尺寸
	body := `{"model":"minimax-h3","prompt":"test","size":"1280x720"}`
	err := avi2api.ValidateVideoRequest("application/json", []byte(body))
	require.NotNil(t, err)
	require.Equal(t, "size", err.Field)
}

func TestValidateVideoRequest_InvalidResolution(t *testing.T) {
	// veo-3.1-lite 只有 720p/1080p
	body := `{"model":"veo-3.1-lite","prompt":"test","resolution":"2160p"}`
	err := avi2api.ValidateVideoRequest("application/json", []byte(body))
	require.NotNil(t, err)
	require.Equal(t, "resolution", err.Field)
}

func TestValidateVideoRequest_GenAudioUnsupported(t *testing.T) {
	// gemini-omni-flash 无 generate_audio 字段
	body := `{"model":"gemini-omni-flash","prompt":"test","generate_audio":true}`
	err := avi2api.ValidateVideoRequest("application/json", []byte(body))
	require.NotNil(t, err)
	require.Equal(t, "generate_audio", err.Field)
}

func TestValidateVideoRequest_GenAudioAlwaysTrue(t *testing.T) {
	// minimax-h3 不允许传 false
	body := `{"model":"minimax-h3","prompt":"test","size":"1440x1440","generate_audio":false}`
	err := avi2api.ValidateVideoRequest("application/json", []byte(body))
	require.NotNil(t, err)
	require.Equal(t, "generate_audio", err.Field)
}

// ─── 视频 multipart 校验 ───────────────────────────────────────────

func buildMultipartBody(t *testing.T, fields map[string]string, files map[string]string) ([]byte, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range fields {
		require.NoError(t, w.WriteField(k, v))
	}
	for name, filename := range files {
		fw, err := w.CreateFormFile(name, filename)
		require.NoError(t, err)
		_, _ = fw.Write([]byte("FAKE_BINARY"))
	}
	require.NoError(t, w.Close())
	return buf.Bytes(), w.FormDataContentType()
}

func TestValidateVideoRequest_FrameModeValid(t *testing.T) {
	body, ct := buildMultipartBody(t,
		map[string]string{"model": "seedance-2.0", "prompt": "test"},
		map[string]string{"start_frame": "first.png"},
	)
	err := avi2api.ValidateVideoRequest(ct, body)
	require.Nil(t, err)
}

func TestValidateVideoRequest_FrameModeExclusivity(t *testing.T) {
	// start_frame + image 互斥
	body, ct := buildMultipartBody(t,
		map[string]string{"model": "seedance-2.0", "prompt": "test"},
		map[string]string{"start_frame": "first.png", "image": "ref.jpg"},
	)
	err := avi2api.ValidateVideoRequest(ct, body)
	require.NotNil(t, err)
	require.Equal(t, "start_frame", err.Field)
}

func TestValidateVideoRequest_EndFrameRequiresStartFrame(t *testing.T) {
	body, ct := buildMultipartBody(t,
		map[string]string{"model": "seedance-2.0", "prompt": "test"},
		map[string]string{"end_frame": "last.png"},
	)
	err := avi2api.ValidateVideoRequest(ct, body)
	require.NotNil(t, err)
	require.Equal(t, "end_frame", err.Field)
}

func TestValidateVideoRequest_RefModeNotSupportedByModel(t *testing.T) {
	// kling-3.0 不支持图片参考
	body, ct := buildMultipartBody(t,
		map[string]string{"model": "kling-3.0", "prompt": "test"},
		map[string]string{"image": "ref.jpg"},
	)
	err := avi2api.ValidateVideoRequest(ct, body)
	require.NotNil(t, err)
	require.Equal(t, "image", err.Field)
}

func TestValidateVideoRequest_VideoRefOnlySeedance(t *testing.T) {
	// veo-3.1 不支持视频参考
	body, ct := buildMultipartBody(t,
		map[string]string{"model": "veo-3.1", "prompt": "test"},
		map[string]string{"video": "clip.mp4"},
	)
	err := avi2api.ValidateVideoRequest(ct, body)
	require.NotNil(t, err)
	require.Equal(t, "video", err.Field)
}

func TestValidateVideoRequest_AudioRefRequiresImageOrVideo(t *testing.T) {
	// seedance-2.0 音频参考没有陪跑素材
	body, ct := buildMultipartBody(t,
		map[string]string{"model": "seedance-2.0", "prompt": "test"},
		map[string]string{"audio": "bg.mp3"},
	)
	err := avi2api.ValidateVideoRequest(ct, body)
	require.NotNil(t, err)
	require.Equal(t, "audio", err.Field)
}

func TestValidateVideoRequest_AudioWithImageValid(t *testing.T) {
	// seedance-2.0 音频 + 图片组合合法
	body, ct := buildMultipartBody(t,
		map[string]string{"model": "seedance-2.0", "prompt": "test"},
		map[string]string{"image": "ref.jpg", "audio": "bg.mp3"},
	)
	err := avi2api.ValidateVideoRequest(ct, body)
	require.Nil(t, err)
}

// ─── 音频校验 ──────────────────────────────────────────────────────

func TestValidateAudioRequest_ValidDialogue(t *testing.T) {
	body := `{"model":"dialogue-v3","prompt":"Hello world","voice":"george","language":"en"}`
	err := avi2api.ValidateAudioRequest([]byte(body))
	require.Nil(t, err)
}

func TestValidateAudioRequest_InvalidVoice(t *testing.T) {
	body := `{"model":"dialogue-v3","prompt":"Hello","voice":"nonexistent-voice"}`
	err := avi2api.ValidateAudioRequest([]byte(body))
	require.NotNil(t, err)
	require.Equal(t, "voice", err.Field)
}

func TestValidateAudioRequest_MusicDurationMinutes(t *testing.T) {
	// music-v1: duration_minutes 1–10
	body := `{"model":"music-v1","prompt":"jazz","duration_minutes":15}`
	err := avi2api.ValidateAudioRequest([]byte(body))
	require.NotNil(t, err)
	require.Equal(t, "duration_minutes", err.Field)
}

func TestValidateAudioRequest_SoundEffectsDuration(t *testing.T) {
	// sound-effects-v2: duration 1–22（秒）
	body := `{"model":"sound-effects-v2","prompt":"thunder","duration":30}`
	err := avi2api.ValidateAudioRequest([]byte(body))
	require.NotNil(t, err)
	require.Equal(t, "duration", err.Field)
}

func TestValidateAudioRequest_SoundEffectsDefaultModel(t *testing.T) {
	// model 字段省略时默认 sound-effects-v2
	body := `{"prompt":"thunder"}`
	err := avi2api.ValidateAudioRequest([]byte(body))
	require.Nil(t, err)
}

func TestValidateAudioRequest_UnknownModel(t *testing.T) {
	body := `{"model":"tts-9000","prompt":"test"}`
	err := avi2api.ValidateAudioRequest([]byte(body))
	require.NotNil(t, err)
	require.Equal(t, "model", err.Field)
}

// ─── 图像校验 ──────────────────────────────────────────────────────

func TestValidateImageRequest_ValidGeneration(t *testing.T) {
	body := `{"model":"nano-banana-2","prompt":"a cat","size":"1024x1024","n":2}`
	err := avi2api.ValidateImageRequest(false, "application/json", []byte(body))
	require.Nil(t, err)
}

func TestValidateImageRequest_NExceedsMax(t *testing.T) {
	// gpt-image-2 max n=1
	body := `{"model":"gpt-image-2","prompt":"a cat","n":3}`
	err := avi2api.ValidateImageRequest(false, "application/json", []byte(body))
	require.NotNil(t, err)
	require.Equal(t, "n", err.Field)
}

func TestValidateImageRequest_UnknownModel(t *testing.T) {
	body := `{"model":"dall-e-4","prompt":"a cat"}`
	err := avi2api.ValidateImageRequest(false, "application/json", []byte(body))
	require.NotNil(t, err)
	require.Equal(t, "model", err.Field)
}

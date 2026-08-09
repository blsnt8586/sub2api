//go:build unit

package service

import (
	"encoding/json"
	"testing"
)

// TestConvertSeedanceVideoCreateBodyDropsFPS 锁定 fps 必须被丢弃。
//
// AVI2API v1.1.2 的全部视频 schema 都是 additionalProperties:false 且均无 fps
// 字段，透传任何 fps 值都会让上游返回 400。watermark 等 Seedance 扩展字段同理。
func TestConvertSeedanceVideoCreateBodyDropsFPS(t *testing.T) {
	for _, tt := range []struct {
		name string
		body string
	}{
		{"fps_30", `{"model":"seedance-2.0","content":[{"type":"text","text":"t"}],"fps":30}`},
		{"fps_60", `{"model":"seedance-2.0","content":[{"type":"text","text":"t"}],"fps":60}`},
		{"fps_24", `{"model":"seedance-2.0","content":[{"type":"text","text":"t"}],"fps":24}`},
		{"fps_非白名单", `{"model":"seedance-2.0","content":[{"type":"text","text":"t"}],"fps":15}`},
		{"fps_负数", `{"model":"seedance-2.0","content":[{"type":"text","text":"t"}],"fps":-1}`},
		{"fps_缺失", `{"model":"seedance-2.0","content":[{"type":"text","text":"t"}]}`},
		{"watermark", `{"model":"seedance-2.0","content":[{"type":"text","text":"t"}],"watermark":true}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			out := ConvertSeedanceVideoCreateBody([]byte(tt.body))
			var result map[string]interface{}
			if err := json.Unmarshal(out, &result); err != nil {
				t.Fatalf("输出不是合法 JSON: %v", err)
			}
			if v, ok := result["fps"]; ok {
				t.Errorf("fps 必须被丢弃，实际透传了: %v", v)
			}
			if v, ok := result["watermark"]; ok {
				t.Errorf("watermark 必须被丢弃，实际透传了: %v", v)
			}
		})
	}
}

// TestConvertSeedanceVideoCreateBodyLeavesOpenAIStyleBodyUntouched 覆盖非 Seedance
// 格式（已带顶层 prompt）：整个 body 原样返回，转换器不介入，因此 fps 之类的
// 字段仍在——是否合法交由上游判定。
func TestConvertSeedanceVideoCreateBodyLeavesOpenAIStyleBodyUntouched(t *testing.T) {
	body := `{"model":"seedance-2.0","prompt":"test","fps":30}`
	out := ConvertSeedanceVideoCreateBody([]byte(body))
	if string(out) != body {
		t.Errorf("非 Seedance 格式应原样返回\n期望: %s\n实际: %s", body, out)
	}
}

// TestConvertSeedanceVideoCreateBodyMapsArkPlanFields 覆盖 Ark Plan v3 → AVI2API
// 的字段映射：content[text] → prompt、ratio → size、duration/resolution/
// generate_audio 透传。
func TestConvertSeedanceVideoCreateBodyMapsArkPlanFields(t *testing.T) {
	body := `{
		"model": "seedance-2.0",
		"content": [{"type": "text", "text": "一朵玫瑰花开放"}],
		"duration": 5,
		"ratio": "16:9",
		"resolution": "1080p",
		"fps": 30,
		"generate_audio": false
	}`

	out := ConvertSeedanceVideoCreateBody([]byte(body))
	var result map[string]interface{}
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("JSON 解析失败: %v", err)
	}

	for k, want := range map[string]interface{}{
		"model":          "seedance-2.0",
		"prompt":         "一朵玫瑰花开放",
		"duration":       int64(5),
		"size":           "1280x720",
		"resolution":     "1080p",
		"generate_audio": false,
	} {
		got, ok := result[k]
		if !ok {
			t.Errorf("字段 %q 缺失", k)
			continue
		}
		switch w := want.(type) {
		case int64:
			if int64(got.(float64)) != w {
				t.Errorf("字段 %q: 期望 %v，实际 %v", k, w, got)
			}
		default:
			if got != want {
				t.Errorf("字段 %q: 期望 %v，实际 %v", k, want, got)
			}
		}
	}

	// fps 不得出现在转换结果里
	if v, ok := result["fps"]; ok {
		t.Errorf("fps 必须被丢弃，实际透传了: %v", v)
	}
}

// TestConvertSeedanceVideoCreateBodyMapsPortraitRatio 覆盖竖屏比例映射。
func TestConvertSeedanceVideoCreateBodyMapsPortraitRatio(t *testing.T) {
	for _, ratio := range []string{"9:16", "2:3", "3:4"} {
		body := `{"model":"seedance-2.0","content":[{"type":"text","text":"t"}],"ratio":"` + ratio + `"}`
		out := ConvertSeedanceVideoCreateBody([]byte(body))
		var result map[string]interface{}
		if err := json.Unmarshal(out, &result); err != nil {
			t.Fatalf("ratio=%s JSON 解析失败: %v", ratio, err)
		}
		if result["size"] != "720x1280" {
			t.Errorf("ratio=%s 期望 size=720x1280，实际 %v", ratio, result["size"])
		}
	}
}

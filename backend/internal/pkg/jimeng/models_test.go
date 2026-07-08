//go:build unit

package jimeng

import "testing"

func TestExtractVideoModel(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{name: "standard", body: `{"model":"video-ds-2.0-fast","prompt":"x","seconds":"15","aspect_ratio":"9:16"}`, want: "video-ds-2.0-fast"},
		{name: "whitespace trimmed", body: `{"model":"  video-ds-2.0  "}`, want: "video-ds-2.0"},
		{name: "missing", body: `{"prompt":"x"}`, want: ""},
		{name: "invalid json", body: `not json`, want: ""},
		{name: "empty body", body: ``, want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ExtractVideoModel([]byte(tc.body)); got != tc.want {
				t.Errorf("ExtractVideoModel(%q) = %q, want %q", tc.body, got, tc.want)
			}
		})
	}
}

func TestParseVideoRequest(t *testing.T) {
	body := `{"model":"video-ds-2.0-fast","prompt":"hello","seconds":"15","aspect_ratio":"9:16"}`
	info := ParseVideoRequest([]byte(body))
	if info.Model != "video-ds-2.0-fast" {
		t.Errorf("Model = %q, want video-ds-2.0-fast", info.Model)
	}
	if info.Prompt != "hello" {
		t.Errorf("Prompt = %q, want hello", info.Prompt)
	}
	if info.Seconds != "15" {
		t.Errorf("Seconds = %q, want 15", info.Seconds)
	}
	if info.AspectRatio != "9:16" {
		t.Errorf("AspectRatio = %q, want 9:16", info.AspectRatio)
	}
}

// TestParseVideoRequestSecondsAsNumber 验证即使上游/客户端误传数字 seconds，
// 解析层也能提取为字符串（即梦要求 seconds 必须是字符串）。
func TestParseVideoRequestSecondsAsNumber(t *testing.T) {
	body := `{"model":"video-ds-2.0","prompt":"x","seconds":15,"aspect_ratio":"16:9"}`
	info := ParseVideoRequest([]byte(body))
	if info.Seconds != "15" {
		t.Errorf("Seconds = %q, want 15 (coerced from number)", info.Seconds)
	}
}

func TestParseVideoRequestInvalidJSON(t *testing.T) {
	info := ParseVideoRequest([]byte(`garbage`))
	if info.Model != "" || info.Prompt != "" {
		t.Errorf("ParseVideoRequest on invalid json should return empty info, got %+v", info)
	}
}

//go:build unit

package leonardo

import "testing"

func TestParseImageRequestJSON(t *testing.T) {
	body := `{"model":"gpt-image-2","prompt":"a cat","size":"1024x1024","n":2}`
	info := ParseImageRequest("application/json", []byte(body))
	if info.Model != "gpt-image-2" {
		t.Errorf("Model = %q, want gpt-image-2", info.Model)
	}
	if info.Prompt != "a cat" {
		t.Errorf("Prompt = %q, want a cat", info.Prompt)
	}
	if info.Size != "1024x1024" {
		t.Errorf("Size = %q, want 1024x1024", info.Size)
	}
	if info.N != 2 {
		t.Errorf("N = %d, want 2", info.N)
	}
}

func TestParseImageRequestDefaultsN(t *testing.T) {
	info := ParseImageRequest("application/json", []byte(`{"model":"nano-banana-2","prompt":"x"}`))
	if info.N != 1 {
		t.Errorf("N = %d, want 1 (default)", info.N)
	}
}

func TestParseImageRequestInvalidJSONFallsBackToDefault(t *testing.T) {
	info := ParseImageRequest("application/json", []byte(`garbage`))
	if info.N != 1 {
		t.Errorf("N = %d, want 1 for invalid body", info.N)
	}
}

func TestExtractImageModel(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{name: "standard", body: `{"model":"seedream-4.5","prompt":"x"}`, want: "seedream-4.5"},
		{name: "whitespace trimmed", body: `{"model":"  gpt-image-2  "}`, want: "gpt-image-2"},
		{name: "missing", body: `{"prompt":"x"}`, want: ""},
		{name: "invalid json", body: `not json`, want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ExtractImageModel("application/json", []byte(tc.body)); got != tc.want {
				t.Errorf("ExtractImageModel(%q) = %q, want %q", tc.body, got, tc.want)
			}
		})
	}
}

func TestCountImages(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int
	}{
		{name: "two urls", body: `{"created":1,"data":[{"url":"a"},{"url":"b"}]}`, want: 2},
		{name: "one b64", body: `{"data":[{"b64_json":"xxx"}]}`, want: 1},
		{name: "empty data", body: `{"data":[]}`, want: 0},
		{name: "no data", body: `{"created":1}`, want: 0},
		{name: "invalid json", body: `garbage`, want: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CountImages([]byte(tc.body)); got != tc.want {
				t.Errorf("CountImages(%q) = %d, want %d", tc.body, got, tc.want)
			}
		})
	}
}

//go:build unit

package service

import "testing"

func TestNormalizeSub2APIProviderBaseURL(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "trailing slash", raw: "https://o10.top/", want: "https://o10.top"},
		{name: "keys page", raw: "https://mdkj.lol/keys", want: "https://mdkj.lol"},
		{name: "keys api endpoint", raw: "https://mdkj.lol/api/v1/keys/", want: "https://mdkj.lol"},
		{name: "deployment prefix", raw: "https://example.com/sub2api/keys", want: "https://example.com/sub2api"},
		{name: "api root", raw: "https://example.com/sub2api/api/v1", want: "https://example.com/sub2api"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeSub2APIProviderBaseURL(tt.raw)
			if err != nil {
				t.Fatalf("normalizeSub2APIProviderBaseURL(%q): %v", tt.raw, err)
			}
			if got != tt.want {
				t.Fatalf("normalizeSub2APIProviderBaseURL(%q)=%q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestNormalizeSub2APIProviderBaseURLRejectsQueryCredentialsAndFragment(t *testing.T) {
	for _, raw := range []string{
		"https://example.com/sub2api?tenant=one",
		"https://admin:secret@example.com/sub2api",
		"https://example.com/sub2api#keys",
	} {
		t.Run(raw, func(t *testing.T) {
			if _, err := normalizeSub2APIProviderBaseURL(raw); err == nil {
				t.Fatalf("normalizeSub2APIProviderBaseURL(%q) unexpectedly succeeded", raw)
			}
		})
	}
}

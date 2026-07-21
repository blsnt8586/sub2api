//go:build unit

package leonardo

import "testing"

func TestBuildImagesGenerationsURL(t *testing.T) {
	cases := []struct {
		name    string
		baseURL string
		want    string
		wantErr bool
	}{
		{name: "host only", baseURL: "https://leo.example.com", want: "https://leo.example.com/v1/images/generations"},
		{name: "host with v1", baseURL: "https://leo.example.com/v1", want: "https://leo.example.com/v1/images/generations"},
		{name: "host with trailing slash", baseURL: "https://leo.example.com/", want: "https://leo.example.com/v1/images/generations"},
		{name: "host with v1 trailing slash", baseURL: "https://leo.example.com/v1/", want: "https://leo.example.com/v1/images/generations"},
		{name: "surrounding whitespace", baseURL: "  https://leo.example.com/v1  ", want: "https://leo.example.com/v1/images/generations"},
		{name: "empty", baseURL: "", wantErr: true},
		{name: "blank", baseURL: "   ", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := BuildImagesGenerationsURL(tc.baseURL)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("BuildImagesGenerationsURL(%q) expected error, got %q", tc.baseURL, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("BuildImagesGenerationsURL(%q) unexpected error: %v", tc.baseURL, err)
			}
			if got != tc.want {
				t.Errorf("BuildImagesGenerationsURL(%q) = %q, want %q", tc.baseURL, got, tc.want)
			}
		})
	}
}

func TestBuildImagesEditsURL(t *testing.T) {
	got, err := BuildImagesEditsURL("https://leo.example.com/v1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "https://leo.example.com/v1/images/edits"; got != want {
		t.Errorf("BuildImagesEditsURL = %q, want %q", got, want)
	}
	if _, err := BuildImagesEditsURL(""); err == nil {
		t.Errorf("BuildImagesEditsURL(\"\") expected error")
	}
}

func TestBuildVideosGenerationsURL(t *testing.T) {
	cases := []struct {
		name    string
		baseURL string
		want    string
		wantErr bool
	}{
		{name: "host only", baseURL: "https://leo.example.com", want: "https://leo.example.com/v1/videos/generations"},
		{name: "host with v1", baseURL: "https://leo.example.com/v1", want: "https://leo.example.com/v1/videos/generations"},
		{name: "empty", baseURL: "", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := BuildVideosGenerationsURL(tc.baseURL)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("BuildVideosGenerationsURL(%q) expected error, got %q", tc.baseURL, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("BuildVideosGenerationsURL(%q) unexpected error: %v", tc.baseURL, err)
			}
			if got != tc.want {
				t.Errorf("BuildVideosGenerationsURL(%q) = %q, want %q", tc.baseURL, got, tc.want)
			}
		})
	}
}

func TestBuildVideoStatusURL(t *testing.T) {
	cases := []struct {
		name    string
		baseURL string
		taskID  string
		want    string
		wantErr bool
	}{
		{name: "host only", baseURL: "https://leo.example.com", taskID: "abc123", want: "https://leo.example.com/v1/videos/abc123"},
		{name: "host with v1", baseURL: "https://leo.example.com/v1", taskID: "abc123", want: "https://leo.example.com/v1/videos/abc123"},
		{name: "task id needs escaping", baseURL: "https://leo.example.com/v1", taskID: "a b/c", want: "https://leo.example.com/v1/videos/a%20b%2Fc"},
		{name: "empty task id", baseURL: "https://leo.example.com/v1", taskID: "", wantErr: true},
		{name: "empty base", baseURL: "", taskID: "abc", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := BuildVideoStatusURL(tc.baseURL, tc.taskID)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("BuildVideoStatusURL(%q,%q) expected error, got %q", tc.baseURL, tc.taskID, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("BuildVideoStatusURL(%q,%q) unexpected error: %v", tc.baseURL, tc.taskID, err)
			}
			if got != tc.want {
				t.Errorf("BuildVideoStatusURL(%q,%q) = %q, want %q", tc.baseURL, tc.taskID, got, tc.want)
			}
		})
	}
}

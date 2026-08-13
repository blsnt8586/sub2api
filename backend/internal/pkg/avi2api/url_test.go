package avi2api

import (
	"strings"
	"testing"
)

func TestBuildImageGenerationTaskURL(t *testing.T) {
	tests := []struct {
		base    string
		want    string
		wantErr bool
	}{
		{"https://api.example.com", "https://api.example.com/v1/images/generations", false},
		{"https://api.example.com/v1", "https://api.example.com/v1/images/generations", false},
		{"https://api.example.com/v1/", "https://api.example.com/v1/images/generations", false},
		{"", "", true},
	}
	for _, tc := range tests {
		got, err := BuildImageGenerationTaskURL(tc.base)
		if tc.wantErr {
			if err == nil {
				t.Errorf("BuildImageGenerationTaskURL(%q): expected error, got nil", tc.base)
			}
			continue
		}
		if err != nil {
			t.Errorf("BuildImageGenerationTaskURL(%q): unexpected error: %v", tc.base, err)
			continue
		}
		if got != tc.want {
			t.Errorf("BuildImageGenerationTaskURL(%q): got %q, want %q", tc.base, got, tc.want)
		}
	}
}

func TestBuildImageStatusURL(t *testing.T) {
	tests := []struct {
		base    string
		id      string
		want    string
		wantErr bool
	}{
		{"https://api.example.com", "task-abc", "https://api.example.com/v1/images/task-abc", false},
		{"https://api.example.com/v1", "task-abc", "https://api.example.com/v1/images/task-abc", false},
		{"https://api.example.com", "", "", true}, // 空 id 报错
		{"", "task-abc", "", true}, // 空 base 报错
		// 含 / 的 taskID 应被转义
		{"https://api.example.com", "a/b", "https://api.example.com/v1/images/a%2Fb", false},
	}
	for _, tc := range tests {
		got, err := BuildImageStatusURL(tc.base, tc.id)
		if tc.wantErr {
			if err == nil {
				t.Errorf("BuildImageStatusURL(%q, %q): expected error, got nil", tc.base, tc.id)
			}
			continue
		}
		if err != nil {
			t.Errorf("BuildImageStatusURL(%q, %q): unexpected error: %v", tc.base, tc.id, err)
			continue
		}
		if got != tc.want {
			t.Errorf("BuildImageStatusURL(%q, %q): got %q, want %q", tc.base, tc.id, got, tc.want)
		}
	}
}

func TestBuildImageCancelURL(t *testing.T) {
	got, err := BuildImageCancelURL("https://api.example.com", "task-xyz")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(got, "/images/task-xyz/cancel") {
		t.Errorf("expected suffix /images/task-xyz/cancel, got %q", got)
	}

	if _, err := BuildImageCancelURL("", "id"); err == nil {
		t.Error("empty base should error")
	}
	if _, err := BuildImageCancelURL("https://x.com", ""); err == nil {
		t.Error("empty id should error")
	}
}

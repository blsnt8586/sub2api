package avi2api

import (
	"strings"
	"testing"
)

func TestBuildAsyncImageTaskURL(t *testing.T) {
	tests := []struct {
		base    string
		want    string
		wantErr bool
	}{
		{"https://api.example.com", "https://api.example.com/v1/tasks/images", false},
		{"https://api.example.com/v1", "https://api.example.com/v1/tasks/images", false},
		{"https://api.example.com/v1/", "https://api.example.com/v1/tasks/images", false},
		{"", "", true},
	}
	for _, tc := range tests {
		got, err := BuildAsyncImageTaskURL(tc.base)
		if tc.wantErr {
			if err == nil {
				t.Errorf("BuildAsyncImageTaskURL(%q): expected error, got nil", tc.base)
			}
			continue
		}
		if err != nil {
			t.Errorf("BuildAsyncImageTaskURL(%q): unexpected error: %v", tc.base, err)
			continue
		}
		if got != tc.want {
			t.Errorf("BuildAsyncImageTaskURL(%q): got %q, want %q", tc.base, got, tc.want)
		}
	}
}

func TestBuildTaskStatusURL(t *testing.T) {
	tests := []struct {
		base    string
		id      string
		want    string
		wantErr bool
	}{
		{"https://api.example.com", "task-abc", "https://api.example.com/v1/tasks/task-abc", false},
		{"https://api.example.com/v1", "task-abc", "https://api.example.com/v1/tasks/task-abc", false},
		{"https://api.example.com", "", "", true},   // 空 id 报错
		{"", "task-abc", "", true},                  // 空 base 报错
		// 含 / 的 taskID 应被转义
		{"https://api.example.com", "a/b", "https://api.example.com/v1/tasks/a%2Fb", false},
	}
	for _, tc := range tests {
		got, err := BuildTaskStatusURL(tc.base, tc.id)
		if tc.wantErr {
			if err == nil {
				t.Errorf("BuildTaskStatusURL(%q, %q): expected error, got nil", tc.base, tc.id)
			}
			continue
		}
		if err != nil {
			t.Errorf("BuildTaskStatusURL(%q, %q): unexpected error: %v", tc.base, tc.id, err)
			continue
		}
		if got != tc.want {
			t.Errorf("BuildTaskStatusURL(%q, %q): got %q, want %q", tc.base, tc.id, got, tc.want)
		}
	}
}

func TestBuildTaskCancelURL(t *testing.T) {
	got, err := BuildTaskCancelURL("https://api.example.com", "task-xyz")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(got, "/tasks/task-xyz/cancel") {
		t.Errorf("expected suffix /tasks/task-xyz/cancel, got %q", got)
	}

	if _, err := BuildTaskCancelURL("", "id"); err == nil {
		t.Error("empty base should error")
	}
	if _, err := BuildTaskCancelURL("https://x.com", ""); err == nil {
		t.Error("empty id should error")
	}
}

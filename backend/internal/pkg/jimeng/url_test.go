//go:build unit

package jimeng

import "testing"

func TestBuildVideosURL(t *testing.T) {
	cases := []struct {
		name    string
		baseURL string
		want    string
		wantErr bool
	}{
		{name: "host only", baseURL: "https://zz1cc.cc.cd", want: "https://zz1cc.cc.cd/v1/videos"},
		{name: "host with v1", baseURL: "https://zz1cc.cc.cd/v1", want: "https://zz1cc.cc.cd/v1/videos"},
		{name: "host with trailing slash", baseURL: "https://zz1cc.cc.cd/", want: "https://zz1cc.cc.cd/v1/videos"},
		{name: "host with v1 trailing slash", baseURL: "https://zz1cc.cc.cd/v1/", want: "https://zz1cc.cc.cd/v1/videos"},
		{name: "surrounding whitespace", baseURL: "  https://zz1cc.cc.cd/v1  ", want: "https://zz1cc.cc.cd/v1/videos"},
		{name: "empty", baseURL: "", wantErr: true},
		{name: "blank", baseURL: "   ", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := BuildVideosURL(tc.baseURL)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("BuildVideosURL(%q) expected error, got %q", tc.baseURL, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("BuildVideosURL(%q) unexpected error: %v", tc.baseURL, err)
			}
			if got != tc.want {
				t.Errorf("BuildVideosURL(%q) = %q, want %q", tc.baseURL, got, tc.want)
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
		{name: "host only", baseURL: "https://zz1cc.cc.cd", taskID: "abc123", want: "https://zz1cc.cc.cd/v1/videos/abc123"},
		{name: "host with v1", baseURL: "https://zz1cc.cc.cd/v1", taskID: "abc123", want: "https://zz1cc.cc.cd/v1/videos/abc123"},
		{name: "task id needs escaping", baseURL: "https://zz1cc.cc.cd/v1", taskID: "a b/c", want: "https://zz1cc.cc.cd/v1/videos/a%20b%2Fc"},
		{name: "empty task id", baseURL: "https://zz1cc.cc.cd/v1", taskID: "", wantErr: true},
		{name: "blank task id", baseURL: "https://zz1cc.cc.cd/v1", taskID: "  ", wantErr: true},
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

func TestBuildVideoContentURL(t *testing.T) {
	cases := []struct {
		name    string
		baseURL string
		taskID  string
		want    string
		wantErr bool
	}{
		{name: "host only", baseURL: "https://zz1cc.cc.cd", taskID: "abc123", want: "https://zz1cc.cc.cd/v1/videos/abc123/content"},
		{name: "host with v1", baseURL: "https://zz1cc.cc.cd/v1", taskID: "abc123", want: "https://zz1cc.cc.cd/v1/videos/abc123/content"},
		{name: "empty task id", baseURL: "https://zz1cc.cc.cd/v1", taskID: "", wantErr: true},
		{name: "empty base", baseURL: "", taskID: "abc", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := BuildVideoContentURL(tc.baseURL, tc.taskID)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("BuildVideoContentURL(%q,%q) expected error, got %q", tc.baseURL, tc.taskID, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("BuildVideoContentURL(%q,%q) unexpected error: %v", tc.baseURL, tc.taskID, err)
			}
			if got != tc.want {
				t.Errorf("BuildVideoContentURL(%q,%q) = %q, want %q", tc.baseURL, tc.taskID, got, tc.want)
			}
		})
	}
}

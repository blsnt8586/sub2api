//go:build unit

package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type toolDownloadRoundTripFunc func(*http.Request) (*http.Response, error)

func (f toolDownloadRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func toolTestRelease(version string) string {
	assets := []map[string]any{}
	for _, suffix := range []string{"windows-x64-setup.exe", "windows-x64.zip", "macos-universal.dmg"} {
		name := "CodexPlusPlus-" + version + "-" + suffix
		contents := "installer " + name
		assets = append(assets, map[string]any{"name": name, "size": len(contents), "state": "uploaded", "digest": fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(contents))), "browser_download_url": toolDownloadsRepository + "/releases/download/" + version + "/" + name})
	}
	b, _ := json.Marshal(map[string]any{"tag_name": version, "html_url": toolDownloadsRepository + "/releases/tag/" + version, "published_at": "2026-11-04T17:09:56Z", "assets": assets})
	return string(b)
}

func newToolTestService(t *testing.T, metadata http.HandlerFunc, binary http.HandlerFunc) (*toolDownloadMirror, *atomic.Int32) {
	t.Helper()
	server := httptest.NewServer(metadata)
	t.Cleanup(server.Close)
	calls := &atomic.Int32{}
	client := server.Client()
	underlying := client.Transport
	client.Transport = toolDownloadRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "github.com" {
			calls.Add(1)
			w := httptest.NewRecorder()
			if binary != nil {
				binary(w, r)
			} else {
				fmt.Fprint(w, "installer "+filepath.Base(r.URL.Path))
			}
			return w.Result(), nil
		}
		return underlying.RoundTrip(r)
	})
	s := newToolDownloadsService(client, server.URL)
	s.mirrorDir = t.TempDir()
	t.Cleanup(s.Stop)
	return s, calls
}
func toolDownloadsDue(s *toolDownloadMirror) {
	s.mu.Lock()
	defer s.mu.Unlock()
	past := time.Now().Add(-time.Minute)
	s.snapshot.NextCheckAt = &past
}
func toolRefresh(t *testing.T, s *toolDownloadMirror) ToolDownloadsSnapshot {
	t.Helper()
	_, err := s.refresh(context.Background())
	require.NoError(t, err)
	snapshot, err := s.Snapshot(context.Background())
	require.NoError(t, err)
	return snapshot
}

func TestToolDownloadsDailyCacheAndConcurrentVisits(t *testing.T) {
	var calls atomic.Int32
	s, binaries := newToolTestService(t, func(w http.ResponseWriter, r *http.Request) { calls.Add(1); fmt.Fprint(w, toolTestRelease("v1")) }, nil)
	var wg sync.WaitGroup
	for range 30 {
		wg.Go(func() {
			_, err := s.refresh(context.Background())
			if err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	snap, err := s.Snapshot(context.Background())
	require.NoError(t, err)
	require.True(t, snap.Available)
	require.Len(t, snap.Assets, 3)
	require.EqualValues(t, 1, calls.Load())
	require.EqualValues(t, 3, binaries.Load())
	require.Equal(t, 24*time.Hour, snap.NextCheckAt.Sub(*snap.CheckedAt))
	for _, a := range snap.Assets {
		require.True(t, strings.HasPrefix(a.URL, "/api/v1/tool-downloads/files/"))
		require.NotContains(t, a.URL, "github.com")
	}
	snap.Assets[0].URL = "mutated"
	next, err := s.Snapshot(context.Background())
	require.NoError(t, err)
	require.NotEqual(t, "mutated", next.Assets[0].URL)
}

func TestToolDownloadsRetainsLastReleaseOnFailureAndUsesETag(t *testing.T) {
	var mode atomic.Int32
	s, binaries := newToolTestService(t, func(w http.ResponseWriter, r *http.Request) {
		switch mode.Load() {
		case 1:
			w.WriteHeader(429)
		case 2:
			require.Equal(t, `"release-v1"`, r.Header.Get("If-None-Match"))
			w.WriteHeader(304)
		default:
			w.Header().Set("ETag", `"release-v1"`)
			fmt.Fprint(w, toolTestRelease("v1"))
		}
	}, nil)
	initial := toolRefresh(t, s)
	require.True(t, initial.Available)
	mode.Store(1)
	toolDownloadsDue(s)
	stale := toolRefresh(t, s)
	require.True(t, stale.Stale)
	require.Equal(t, initial.Assets, stale.Assets)
	require.Equal(t, time.Hour, stale.NextCheckAt.Sub(*stale.CheckedAt))
	mode.Store(2)
	toolDownloadsDue(s)
	fresh := toolRefresh(t, s)
	require.False(t, fresh.Stale)
	require.Equal(t, initial.Assets, fresh.Assets)
	require.EqualValues(t, 3, binaries.Load())
}

func TestToolDownloadsPersistsFilesAndKeepsTwoVersions(t *testing.T) {
	var version atomic.Int32
	version.Store(1)
	s, _ := newToolTestService(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, toolTestRelease(fmt.Sprintf("v%d", version.Load())))
	}, nil)
	first := toolRefresh(t, s)
	require.True(t, first.Available)
	restored := newToolDownloadsService(s.client, s.endpoint)
	restored.mirrorDir = s.mirrorDir
	t.Cleanup(restored.Stop)
	require.NoError(t, restored.loadMirror())
	snap, err := restored.Snapshot(context.Background())
	require.NoError(t, err)
	require.Equal(t, first.Assets, snap.Assets)
	firstID := s.releases[0].ID
	version.Store(2)
	toolDownloadsDue(s)
	second := toolRefresh(t, s)
	require.True(t, second.Available)
	f, a, err := s.OpenDownload(firstID, first.Assets[0].Name)
	require.NoError(t, err)
	body, err := io.ReadAll(f)
	f.Close()
	require.NoError(t, err)
	require.Equal(t, "installer "+a.Name, string(body))
	version.Store(3)
	toolDownloadsDue(s)
	third := toolRefresh(t, s)
	require.Equal(t, "v3", third.Version)
	_, _, err = s.OpenDownload(firstID, a.Name)
	require.ErrorIs(t, err, os.ErrNotExist)
	require.Len(t, s.releases, 2)
	entries, err := os.ReadDir(filepath.Join(s.mirrorDir, "files"))
	require.NoError(t, err)
	require.Len(t, entries, 2)
	_, _, err = s.OpenDownload(s.releases[0].ID, "../../manifest.json")
	require.ErrorIs(t, err, os.ErrNotExist)
	_, _, err = s.OpenDownload(strings.Repeat("a", 64), a.Name)
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestToolDownloadsFailedInstallNeverPublishesPartialRelease(t *testing.T) {
	for _, mode := range []string{"truncated", "oversized", "checksum", "http"} {
		t.Run(mode, func(t *testing.T) {
			var fail atomic.Bool
			s, _ := newToolTestService(t, func(w http.ResponseWriter, r *http.Request) {
				v := "v1"
				if fail.Load() {
					v = "v2"
				}
				fmt.Fprint(w, toolTestRelease(v))
			}, func(w http.ResponseWriter, r *http.Request) {
				name := filepath.Base(r.URL.Path)
				body := "installer " + name
				if fail.Load() && strings.HasSuffix(name, ".dmg") {
					switch mode {
					case "truncated":
						body = body[:3]
					case "oversized":
						body += "extra"
					case "checksum":
						body = strings.Repeat("x", len(body))
					case "http":
						w.WriteHeader(503)
						return
					}
				}
				fmt.Fprint(w, body)
			})
			original := toolRefresh(t, s)
			require.True(t, original.Available)
			fail.Store(true)
			toolDownloadsDue(s)
			failed := toolRefresh(t, s)
			require.True(t, failed.Stale)
			require.Equal(t, "v1", failed.Version)
			require.Equal(t, original.Assets, failed.Assets)
			require.Len(t, s.releases, 1)
			entries, err := os.ReadDir(filepath.Join(s.mirrorDir, "files"))
			require.NoError(t, err)
			require.Len(t, entries, 2, "completed files stay in an unpublished staging directory for retry")
			restored := newToolDownloadsService(s.client, s.endpoint)
			restored.mirrorDir = s.mirrorDir
			t.Cleanup(restored.Stop)
			require.NoError(t, restored.loadMirror())
			require.Equal(t, "v1", restored.snapshot.Version)
		})
	}
}

func TestToolDownloadsSnapshotDoesNotWaitForBinarySync(t *testing.T) {
	started, finish := make(chan struct{}), make(chan struct{})
	var first sync.Once
	s, _ := newToolTestService(t, func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, toolTestRelease("v1")) }, func(w http.ResponseWriter, r *http.Request) {
		first.Do(func() { close(started); <-finish })
		fmt.Fprint(w, "installer "+filepath.Base(r.URL.Path))
	})
	done := make(chan struct{})
	go func() { defer close(done); _, _ = s.refresh(context.Background()) }()
	<-started
	snap, err := s.Snapshot(context.Background())
	require.NoError(t, err)
	require.False(t, snap.Available)
	require.True(t, snap.Syncing)
	require.Empty(t, snap.Assets)
	close(finish)
	<-done
	snap, err = s.Snapshot(context.Background())
	require.NoError(t, err)
	require.True(t, snap.Available)
	require.False(t, snap.Syncing)
}

func TestToolDownloadsRejectsUntrustedOrPrereleaseMetadata(t *testing.T) {
	for _, body := range []string{
		`{"tag_name":"v3","html_url":"https://evil.example/releases/tag/v3"}`,
		`{"tag_name":"v3","html_url":"https://github.com/BigPizzaV3/CodexPlusPlus/releases/tag/v3","prerelease":true}`,
		strings.Replace(toolTestRelease("v1"), `"name":"CodexPlusPlus-v1-windows-x64-setup.exe"`, `"name":"../../CodexPlusPlus-v1-windows-x64-setup.exe"`, 1),
	} {
		s, binaries := newToolTestService(t, func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }, nil)
		snap := toolRefresh(t, s)
		require.False(t, snap.Available)
		require.True(t, snap.Stale)
		require.EqualValues(t, 0, binaries.Load())
	}
	for _, raw := range []string{"http://github.com/file", "https://evil.example/file", "https://github.com.evil.example/file", "https://github.com:443/file", "https://user@github.com/file"} {
		u, err := url.Parse(raw)
		require.NoError(t, err)
		require.Error(t, toolDownloadRedirect(&http.Request{URL: u}, nil))
	}
}

func TestToolDownloadsRestoreRejectsCorruptFiles(t *testing.T) {
	s, _ := newToolTestService(t, func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, toolTestRelease("v1")) }, nil)
	snap := toolRefresh(t, s)
	require.True(t, snap.Available)
	file := filepath.Join(s.mirrorDir, "files", s.releases[0].ID, snap.Assets[0].Name)
	require.NoError(t, os.WriteFile(file, []byte(strings.Repeat("x", int(snap.Assets[0].Size))), 0600))
	restored := newToolDownloadsService(s.client, s.endpoint)
	restored.mirrorDir = s.mirrorDir
	t.Cleanup(restored.Stop)
	require.Error(t, restored.loadMirror())
	require.False(t, restored.snapshot.Available)
}

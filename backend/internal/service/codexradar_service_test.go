//go:build unit

package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// newTestRadarService 构造一个指向测试服务器的 CodexRadarService。
func newTestRadarService(imageURL, summaryURL string) *CodexRadarService {
	s := NewCodexRadarService()
	s.imageURL = imageURL
	s.summaryURL = summaryURL
	return s
}

func TestCodexRadarService_FetchAndCache(t *testing.T) {
	var imageHits, summaryHits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/image":
			imageHits.Add(1)
			w.Header().Set("Content-Type", "image/png")
			w.Header().Set("ETag", `"abc123"`)
			_, _ = w.Write([]byte("PNGDATA"))
		case "/summary":
			summaryHits.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"green"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	s := newTestRadarService(srv.URL+"/image", srv.URL+"/summary")

	// 首访：同步拉取，缓存命中。
	s.EnsureFresh(context.Background())
	img := s.ImageSnapshot()
	if !img.Available || string(img.Bytes) != "PNGDATA" {
		t.Fatalf("image snapshot not available: %+v", img)
	}
	if img.ContentType != "image/png" || img.ETag != `"abc123"` {
		t.Fatalf("unexpected image meta: type=%q etag=%q", img.ContentType, img.ETag)
	}
	sum := s.SummarySnapshot()
	if !sum.Available || string(sum.JSON) != `{"status":"green"}` {
		t.Fatalf("summary snapshot not available: %+v", sum)
	}

	// 第二次 EnsureFresh：缓存新鲜，不应再打上游。
	s.EnsureFresh(context.Background())
	if imageHits.Load() != 1 || summaryHits.Load() != 1 {
		t.Fatalf("cache not honored: imageHits=%d summaryHits=%d", imageHits.Load(), summaryHits.Load())
	}
}

func TestCodexRadarService_EmptyWhenNoFetch(t *testing.T) {
	s := NewCodexRadarService()
	if s.ImageSnapshot().Available {
		t.Fatal("expected image unavailable before any fetch")
	}
	if s.SummarySnapshot().Available {
		t.Fatal("expected summary unavailable before any fetch")
	}
}

func TestCodexRadarService_StaleWhileRevalidate(t *testing.T) {
	var version atomic.Int32
	version.Store(1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/image" {
			w.Header().Set("Content-Type", "image/png")
			if version.Load() == 1 {
				_, _ = w.Write([]byte("V1"))
			} else {
				_, _ = w.Write([]byte("V2"))
			}
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	s := newTestRadarService(srv.URL+"/image", srv.URL+"/summary")
	s.ttl = 10 * time.Millisecond
	s.minRetry = 0

	s.EnsureFresh(context.Background())
	if got := string(s.ImageSnapshot().Bytes); got != "V1" {
		t.Fatalf("expected V1, got %q", got)
	}

	// 让缓存过期，切换上游内容到 V2。
	time.Sleep(20 * time.Millisecond)
	version.Store(2)

	// 过期后 EnsureFresh：应立即返回旧数据 V1（stale），后台异步刷新。
	s.EnsureFresh(context.Background())
	if got := string(s.ImageSnapshot().Bytes); got != "V1" {
		t.Fatalf("expected stale V1 immediately, got %q", got)
	}

	// 等后台刷新完成，缓存应更新为 V2。
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if string(s.ImageSnapshot().Bytes) == "V2" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("background refresh did not update cache to V2, still %q", string(s.ImageSnapshot().Bytes))
}

func TestCodexRadarService_UpstreamErrorKeepsOldSnapshot(t *testing.T) {
	var fail atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail.Load() {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if r.URL.Path == "/image" {
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("GOOD"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	s := newTestRadarService(srv.URL+"/image", srv.URL+"/summary")
	s.EnsureFresh(context.Background())
	if got := string(s.ImageSnapshot().Bytes); got != "GOOD" {
		t.Fatalf("expected GOOD, got %q", got)
	}

	// 上游开始报错，手动触发一次刷新：应保留旧数据，不清空。
	fail.Store(true)
	s.fetchNowForTest(context.Background())
	if got := string(s.ImageSnapshot().Bytes); got != "GOOD" {
		t.Fatalf("expected old GOOD retained on upstream error, got %q", got)
	}
}

// fetchNowForTest 绕过节流/TTL 直接拉取一次（仅测试用）。
func (s *CodexRadarService) fetchNowForTest(ctx context.Context) {
	prev, _ := s.cache.Load().(*codexRadarSnapshot)
	if next := s.fetch(ctx, prev); next != nil {
		s.cache.Store(next)
	}
}

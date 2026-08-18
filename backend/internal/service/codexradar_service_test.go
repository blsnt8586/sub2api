//go:build unit

package service

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"math/rand"
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
	s.recommendationsURL = ""
	s.intelligenceURL = ""
	return s
}

func TestCodexRadarService_FetchesStructuredDatasets(t *testing.T) {
	var recommendationHits, intelligenceHits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/recommendations":
			recommendationHits.Add(1)
			_, _ = w.Write([]byte(`{"recommendations":[{"key":"daily","title":"日常开发","items":[]}]}`))
		case "/intelligence":
			intelligenceHits.Add(1)
			_, _ = w.Write([]byte(`{"points":[{"model":"gpt-5.6-sol","effort":"low","iq":80}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	s := NewCodexRadarService()
	s.imageURL = ""
	s.summaryURL = ""
	s.recommendationsURL = srv.URL + "/recommendations"
	s.intelligenceURL = srv.URL + "/intelligence"
	s.EnsureFresh(context.Background())

	snap := s.DataSnapshot()
	if !snap.Available || len(snap.Recommendations) == 0 || len(snap.Intelligence) == 0 {
		t.Fatalf("structured snapshot not available: %+v", snap)
	}
	if recommendationHits.Load() != 1 || intelligenceHits.Load() != 1 {
		t.Fatalf("unexpected upstream hits: recommendations=%d intelligence=%d", recommendationHits.Load(), intelligenceHits.Load())
	}
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
	// 非可解码图（"PNGDATA"）：optimize 原样透传、类型回退 image/png；
	// ETag 改为按优化后字节内容派生的弱 ETag（不再是上游 "abc123"）。
	if img.ContentType != "image/png" || img.ETag != weakETag([]byte("PNGDATA")) {
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

func TestCodexRadarService_WarmupSkipsWhenDisabled(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("PNG"))
	}))
	defer srv.Close()

	s := newTestRadarService(srv.URL+"/image", srv.URL+"/summary")
	s.ConfigureScheduler(func(context.Context) bool { return false }, nil)

	// 功能开关关闭：预热不得触碰第三方。
	s.warmup(context.Background(), "test")
	if hits.Load() != 0 {
		t.Fatalf("expected no fetch when disabled, got %d hits", hits.Load())
	}
	if s.ImageSnapshot().Available {
		t.Fatal("expected no cached image when disabled")
	}
}

func TestCodexRadarService_WarmupFetchesWhenEnabled(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("PNG"))
	}))
	defer srv.Close()

	s := newTestRadarService(srv.URL+"/image", srv.URL+"/summary")
	s.minRetry = 0
	s.ConfigureScheduler(func(context.Context) bool { return true }, nil)

	s.warmup(context.Background(), "test")
	if hits.Load() == 0 {
		t.Fatal("expected fetch when enabled")
	}
	if got := string(s.ImageSnapshot().Bytes); got != "PNG" {
		t.Fatalf("expected cached PNG, got %q", got)
	}
}

func TestCodexRadarService_ForceRefreshBypassesTTL(t *testing.T) {
	var version atomic.Int32
	version.Store(1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/image" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		if version.Load() == 1 {
			_, _ = w.Write([]byte("V1"))
		} else {
			_, _ = w.Write([]byte("V2"))
		}
	}))
	defer srv.Close()

	s := newTestRadarService(srv.URL+"/image", srv.URL+"/summary")
	s.minRetry = 0
	// TTL 很长：EnsureFresh 会认为新鲜而跳过；forceRefresh 应无视 TTL 强拉新版本。
	s.ttl = time.Hour

	s.EnsureFresh(context.Background())
	if got := string(s.ImageSnapshot().Bytes); got != "V1" {
		t.Fatalf("expected V1, got %q", got)
	}

	version.Store(2)
	// EnsureFresh 因缓存新鲜跳过，仍是 V1。
	s.EnsureFresh(context.Background())
	if got := string(s.ImageSnapshot().Bytes); got != "V1" {
		t.Fatalf("expected EnsureFresh to honor TTL and keep V1, got %q", got)
	}
	// forceRefresh 绕过 TTL，拿到 V2。
	s.forceRefresh(context.Background())
	if got := string(s.ImageSnapshot().Bytes); got != "V2" {
		t.Fatalf("expected forceRefresh to bypass TTL and fetch V2, got %q", got)
	}
}

func TestCodexRadarService_StartStopIdempotent(t *testing.T) {
	s := NewCodexRadarService()
	s.ConfigureScheduler(func(context.Context) bool { return false }, nil)

	// 重复 Start/Stop 不得 panic 或死锁。
	s.Start()
	s.Start()
	s.Stop()
	s.Stop()

	// 未配置 enabledFn 时 Start 应退化为纯懒加载（不启动 cron）。
	s2 := NewCodexRadarService()
	s2.Start()
	s2.Stop()
}

func TestOptimizeCodexRadarImage_DownscalesAndShrinks(t *testing.T) {
	// 造一张 2400x1800 的大 PNG（伪随机噪声，PNG 难压缩，原图体积大），
	// 验证优化会等比缩放到上限内并显著变小。固定种子保证可复现。
	const w, h = 2400, 1800
	rng := rand.New(rand.NewSource(1))
	src := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			src.Set(x, y, color.RGBA{R: uint8(rng.Intn(256)), G: uint8(rng.Intn(256)), B: uint8(rng.Intn(256)), A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, src); err != nil {
		t.Fatalf("encode source png: %v", err)
	}
	orig := buf.Bytes()

	opt, ctype := optimizeCodexRadarImage(orig, "image/png")

	if len(opt) >= len(orig) {
		t.Fatalf("expected optimized smaller than original: orig=%d opt=%d", len(orig), len(opt))
	}
	if ctype != "image/jpeg" && ctype != "image/png" {
		t.Fatalf("unexpected content-type %q", ctype)
	}

	// 优化结果必须仍可解码，且最长边被压到上限内。
	decoded, _, err := image.Decode(bytes.NewReader(opt))
	if err != nil {
		t.Fatalf("optimized image not decodable: %v", err)
	}
	b := decoded.Bounds()
	longest := b.Dx()
	if b.Dy() > longest {
		longest = b.Dy()
	}
	if longest > codexRadarMaxImageDimension {
		t.Fatalf("expected longest side <= %d, got %d", codexRadarMaxImageDimension, longest)
	}
}

func TestOptimizeCodexRadarImage_NonImagePassthrough(t *testing.T) {
	// 非可解码内容：原样透传，不丢数据、类型走缺省回退。
	orig := []byte("not-an-image")
	opt, ctype := optimizeCodexRadarImage(orig, "image/png")
	if !bytes.Equal(opt, orig) {
		t.Fatalf("expected passthrough of undecodable bytes")
	}
	if ctype != "image/png" {
		t.Fatalf("expected fallback image/png, got %q", ctype)
	}
}

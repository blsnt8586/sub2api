package service

// 本文件为二开新增：Codex 雷达（codexradar.com）第三方数据代理缓存。
// 与上游完全解耦——拉取、缓存、快照读取全部自包含在此文件，
// 上游文件无任何 hook。功能默认关闭（opt-in），由后台开关 codex_radar_enabled 控制。
//
// 设计要点：
//   - 数据源为第三方社区站点，本平台仅做代理缓存 + 署名转载，不对数据准确性负责。
//   - 纯懒加载：无后台 goroutine，请求命中时按需刷新，进程内 atomic.Value 缓存。
//   - stale-while-revalidate：缓存过期时先返回旧数据、后台异步刷新，避免请求阻塞。
//   - 失败节流：上游抖动时按 minRetry 间隔重试，绝不打爆对方服务器。
// 详见 CUSTOM-CHANGES.md。

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"golang.org/x/sync/singleflight"
)

const (
	// CodexRadarImageURL 是「漫画摘要图」的稳定别名（无时间戳，日更两次，CDN 4h 缓存）。
	CodexRadarImageURL = "https://codexradar.com/assets/radar-high-readout-comic.png"
	// CodexRadarSummaryURL 是公开结构化状态 JSON（重置窗口 / 24-48h 预测 / 降智分等）。
	CodexRadarSummaryURL = "https://codexradar.com/current.json"
	// CodexRadarSourceSite 第三方来源站点首页（用于前端「详情查看」跳转）。
	CodexRadarSourceSite = "https://codexradar.com"
	// CodexRadarAttribution 第三方数据署名（对方 current.json 要求的 attribution_text）。
	CodexRadarAttribution = "数据来自 Codex 雷达 codexradar.com"

	// codexRadarCacheTTL 缓存新鲜期：对方日更两次，30 分钟足够及时且对对方很友好。
	codexRadarCacheTTL = 30 * time.Minute
	// codexRadarMinRetry 拉取失败时的最小重试间隔，避免上游抖动时打爆对方。
	codexRadarMinRetry = 30 * time.Second
	// codexRadarFetchTimeout 单次拉取超时（图约 2.3MB）。
	codexRadarFetchTimeout = 30 * time.Second
	// codexRadarMaxImageBytes 图片大小上限（防御异常大响应），10MB。
	codexRadarMaxImageBytes = 10 << 20
	// codexRadarMaxSummaryBytes 摘要 JSON 大小上限，2MB。
	codexRadarMaxSummaryBytes = 2 << 20
)

// codexRadarSnapshot 是进程内缓存的一份不可变快照，通过 atomic.Value 零锁读取。
type codexRadarSnapshot struct {
	imageBytes   []byte
	imageType    string
	imageETag    string
	summaryBytes []byte
	fetchedAt    time.Time
	ok           bool // 是否含至少一项可用数据
}

// CodexRadarService 代理并缓存 codexradar.com 的公开数据。纯懒加载，无后台任务。
type CodexRadarService struct {
	httpClient      *http.Client
	cache           atomic.Value // *codexRadarSnapshot
	sf              singleflight.Group
	lastAttemptNano atomic.Int64

	imageURL   string
	summaryURL string
	ttl        time.Duration
	minRetry   time.Duration
}

// NewCodexRadarService 创建 Codex 雷达代理服务（wire 注入）。
func NewCodexRadarService() *CodexRadarService {
	return &CodexRadarService{
		httpClient: &http.Client{Timeout: codexRadarFetchTimeout},
		imageURL:   CodexRadarImageURL,
		summaryURL: CodexRadarSummaryURL,
		ttl:        codexRadarCacheTTL,
		minRetry:   codexRadarMinRetry,
	}
}

// EnsureFresh 按需保证缓存新鲜：
//   - 新鲜（未过期）→ 直接返回，不触发网络。
//   - 过期但有旧数据 → 立即返回旧数据，后台异步刷新（stale-while-revalidate）。
//   - 无任何可用缓存 → 同步阻塞拉取一次（singleflight 去重并发首访）。
func (s *CodexRadarService) EnsureFresh(ctx context.Context) {
	snap, _ := s.cache.Load().(*codexRadarSnapshot)
	if snap != nil && snap.ok && time.Since(snap.fetchedAt) < s.ttl {
		return
	}
	if snap != nil && snap.ok {
		bg := context.WithoutCancel(ctx)
		go s.refreshOnce(bg)
		return
	}
	s.refreshOnce(ctx)
}

// refreshOnce 在 singleflight 保护下刷新一次缓存，内含新鲜度复查与失败节流。
func (s *CodexRadarService) refreshOnce(ctx context.Context) {
	_, _, _ = s.sf.Do("refresh", func() (any, error) {
		snap, _ := s.cache.Load().(*codexRadarSnapshot)
		if snap != nil && snap.ok && time.Since(snap.fetchedAt) < s.ttl {
			return nil, nil
		}
		if last := s.lastAttemptNano.Load(); last != 0 {
			if time.Since(time.Unix(0, last)) < s.minRetry {
				return nil, nil // 距上次尝试太近，节流跳过
			}
		}
		s.lastAttemptNano.Store(time.Now().UnixNano())
		if next := s.fetch(ctx, snap); next != nil {
			s.cache.Store(next)
		}
		return nil, nil
	})
}

// fetch 拉取图 + 摘要，构造新快照。任一部分失败时沿用旧快照对应部分（部分成功即可用）。
func (s *CodexRadarService) fetch(ctx context.Context, prev *codexRadarSnapshot) *codexRadarSnapshot {
	fetchCtx, cancel := context.WithTimeout(ctx, codexRadarFetchTimeout)
	defer cancel()

	next := &codexRadarSnapshot{fetchedAt: time.Now()}

	if body, ctype, etag, err := s.get(fetchCtx, s.imageURL, codexRadarMaxImageBytes); err == nil && len(body) > 0 {
		next.imageBytes = body
		next.imageType = ctype
		next.imageETag = etag
	} else {
		if err != nil {
			slog.Warn("codexradar: fetch image failed", "error", err)
		}
		if prev != nil {
			next.imageBytes = prev.imageBytes
			next.imageType = prev.imageType
			next.imageETag = prev.imageETag
		}
	}

	if body, _, _, err := s.get(fetchCtx, s.summaryURL, codexRadarMaxSummaryBytes); err == nil && len(body) > 0 {
		next.summaryBytes = body
	} else {
		if err != nil {
			slog.Warn("codexradar: fetch summary failed", "error", err)
		}
		if prev != nil {
			next.summaryBytes = prev.summaryBytes
		}
	}

	next.ok = len(next.imageBytes) > 0 || len(next.summaryBytes) > 0
	if !next.ok {
		return nil // 全部失败且无旧数据可沿用：不覆盖缓存
	}
	return next
}

// get 发起 GET 并读取有限字节，返回 body、content-type、etag。
func (s *CodexRadarService) get(ctx context.Context, url string, limit int64) (body []byte, contentType, etag string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", "", err
	}
	req.Header.Set("User-Agent", "sub2api-codexradar-proxy/1.0")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, "", "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, "", "", &codexRadarHTTPError{status: resp.StatusCode, url: url}
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit))
	if err != nil {
		return nil, "", "", err
	}
	return data, resp.Header.Get("Content-Type"), resp.Header.Get("ETag"), nil
}

// codexRadarHTTPError 表示上游返回非 200。
type codexRadarHTTPError struct {
	status int
	url    string
}

func (e *codexRadarHTTPError) Error() string {
	return "codexradar: unexpected status " + http.StatusText(e.status) + " from " + e.url
}

// CodexRadarImageResult 是图片快照的只读视图。
type CodexRadarImageResult struct {
	Bytes       []byte
	ContentType string
	ETag        string
	FetchedAt   time.Time
	Available   bool
}

// ImageSnapshot 返回当前缓存的图片视图（不触发网络）。
func (s *CodexRadarService) ImageSnapshot() CodexRadarImageResult {
	snap, _ := s.cache.Load().(*codexRadarSnapshot)
	if snap == nil || len(snap.imageBytes) == 0 {
		return CodexRadarImageResult{}
	}
	ctype := snap.imageType
	if ctype == "" {
		ctype = "image/png"
	}
	return CodexRadarImageResult{
		Bytes:       snap.imageBytes,
		ContentType: ctype,
		ETag:        snap.imageETag,
		FetchedAt:   snap.fetchedAt,
		Available:   true,
	}
}

// CodexRadarSummaryResult 是摘要 JSON 快照的只读视图。
type CodexRadarSummaryResult struct {
	JSON      []byte
	FetchedAt time.Time
	Available bool
}

// SummarySnapshot 返回当前缓存的摘要 JSON 视图（不触发网络）。
func (s *CodexRadarService) SummarySnapshot() CodexRadarSummaryResult {
	snap, _ := s.cache.Load().(*codexRadarSnapshot)
	if snap == nil || len(snap.summaryBytes) == 0 {
		return CodexRadarSummaryResult{}
	}
	return CodexRadarSummaryResult{
		JSON:      snap.summaryBytes,
		FetchedAt: snap.fetchedAt,
		Available: true,
	}
}

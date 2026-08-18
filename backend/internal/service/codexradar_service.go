package service

// 本文件为二开新增：Codex 雷达（codexradar.com）第三方数据代理缓存。
// 与上游完全解耦——拉取、缓存、快照读取全部自包含在此文件，
// 上游文件无任何 hook。功能默认关闭（opt-in），由后台开关 codex_radar_enabled 控制。
//
// 设计要点：
//   - 数据源为第三方社区站点，本平台仅做代理缓存 + 署名转载，不对数据准确性负责。
//   - 懒加载 + 定时预热：请求命中时按需刷新；另有定时器在 07:00–15:00 每小时整点预热，
//     使缓存常年温热，用户不再吃「进程重启 / 缓存过期无人访问」时的冷启动阻塞。
//     预热仅在功能开关（codex_radar_enabled）开启时拉取第三方数据。
//   - stale-while-revalidate：缓存过期时先返回旧数据、后台异步刷新，避免请求阻塞。
//   - 失败节流：上游抖动时按 minRetry 间隔重试，绝不打爆对方服务器。
// 定时预热与上游解耦：只依赖一个「功能是否开启」的只读回调，不 hook 任何上游流程。
// 详见 CUSTOM-CHANGES.md。

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"image"
	"image/color"
	stddraw "image/draw"
	"image/jpeg"
	"image/png"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/robfig/cron/v3"
	xdraw "golang.org/x/image/draw"
	"golang.org/x/sync/singleflight"
)

const (
	// CodexRadarRecommendationsURL 是原站「站长推荐」数据接口。
	CodexRadarRecommendationsURL = "https://codexradar.com/api/radar-insights"
	// CodexRadarIntelligenceURL 是原站「综合智能」数据接口。
	CodexRadarIntelligenceURL = "https://codexradar.com/api/intelligence-efficiency-metrics"
	// 旧接口常量仅供兼容旧测试/调用方；默认服务不再抓取图片和 current.json。
	CodexRadarImageURL   = ""
	CodexRadarSummaryURL = ""
	// CodexRadarSourceSite 第三方来源站点首页（用于前端「详情查看」跳转）。
	CodexRadarSourceSite = "https://codexradar.com"
	// CodexRadarAttribution 第三方数据署名。
	CodexRadarAttribution = "数据来自 Codex 雷达 codexradar.com"

	// codexRadarCacheTTL 缓存新鲜期：与本平台每小时同步周期一致。
	codexRadarCacheTTL = time.Hour
	// codexRadarMinRetry 拉取失败时的最小重试间隔，避免上游抖动时打爆对方。
	codexRadarMinRetry = 30 * time.Second
	// codexRadarFetchTimeout 单次结构化数据拉取超时。
	codexRadarFetchTimeout = 30 * time.Second
	// codexRadarMaxImageBytes 图片大小上限（防御异常大响应），10MB。
	codexRadarMaxImageBytes = 10 << 20
	// codexRadarMaxSummaryBytes 摘要 JSON 大小上限，2MB。
	codexRadarMaxSummaryBytes = 2 << 20

	// codexRadarWarmupSchedule 每小时整点刷新（5 段 cron：分 时 日 月 周）。
	codexRadarWarmupSchedule = "0 * * * *"
	// codexRadarStartupDelay 启动预热延迟：给 DB/迁移留出就绪时间后再读开关并拉取一次，
	// 解决「进程重启后缓存为空、首个用户吃冷启动阻塞」的问题。
	codexRadarStartupDelay = 5 * time.Second
	// codexRadarCronStopTimeout 关闭 cron 时等待在途任务的最长时间。
	codexRadarCronStopTimeout = 3 * time.Second

	// codexRadarMaxImageDimension 优化后图片最长边上限（px）。源站漫画图分辨率远超前端
	// 视口显示所需；缩到 1080px 显著缩小体积、缩短跨境下载耗时。绝不放大。
	codexRadarMaxImageDimension = 1080
	// codexRadarJPEGQuality JPEG 重编码质量。82 对这种大字漫画图清晰度损失很小。
	// 最终会在 JPEG / PNG / 原图三者里取最小的那份。
	codexRadarJPEGQuality = 82
)

// codexRadarSnapshot 是进程内缓存的一份不可变快照，通过 atomic.Value 零锁读取。
type codexRadarSnapshot struct {
	imageBytes           []byte
	imageType            string
	imageETag            string
	summaryBytes         []byte
	recommendationsBytes []byte
	intelligenceBytes    []byte
	fetchedAt            time.Time
	ok                   bool // 是否含至少一项可用数据
}

// CodexRadarService 代理并缓存 codexradar.com 的公开数据。
// 懒加载（请求命中时刷新）+ 定时预热（07:00–15:00 每小时整点后台拉取，保持缓存温热）。
type CodexRadarService struct {
	httpClient      *http.Client
	cache           atomic.Value // *codexRadarSnapshot
	sf              singleflight.Group
	lastAttemptNano atomic.Int64

	imageURL           string
	summaryURL         string
	recommendationsURL string
	intelligenceURL    string
	ttl                time.Duration
	minRetry           time.Duration

	// —— 定时预热（可选，通过 ConfigureScheduler 注入；未配置则退化为纯懒加载）——
	// enabledFn 是「功能开关是否开启」的只读回调，与上游 SettingService 解耦。
	enabledFn func(ctx context.Context) bool
	location  *time.Location // 预热计划所用时区（默认 time.Local）

	mu       sync.Mutex // 守护 cron 生命周期字段
	cron     *cron.Cron
	rootCtx  context.Context    // 预热任务的根 ctx，Stop 时取消以中断在途拉取
	cancel   context.CancelFunc // rootCtx 的取消函数
	started  bool
	stopped  bool
	stopOnce sync.Once
	wg       sync.WaitGroup // 等待启动预热 goroutine 退出
}

// NewCodexRadarService 创建 Codex 雷达代理服务（wire 注入）。
func NewCodexRadarService() *CodexRadarService {
	return &CodexRadarService{
		httpClient:         &http.Client{Timeout: codexRadarFetchTimeout},
		imageURL:           CodexRadarImageURL,
		summaryURL:         CodexRadarSummaryURL,
		recommendationsURL: CodexRadarRecommendationsURL,
		intelligenceURL:    CodexRadarIntelligenceURL,
		ttl:                codexRadarCacheTTL,
		minRetry:           codexRadarMinRetry,
		location:           time.Local,
	}
}

// ConfigureScheduler 注入定时预热所需依赖：功能开关回调 + 时区。
// 与上游解耦——只收一个只读回调，不持有任何上游 service 引用。
// 传入 nil enabledFn 时预热永不拉取（等价于功能关闭）；loc 为 nil 时用 time.Local。
func (s *CodexRadarService) ConfigureScheduler(enabledFn func(ctx context.Context) bool, loc *time.Location) {
	if s == nil {
		return
	}
	s.enabledFn = enabledFn
	if loc != nil {
		s.location = loc
	}
}

// Start 启动定时预热：注册 07:00–15:00 每小时整点的 cron 任务，并在延迟后做一次启动预热。
// 幂等；未配置 enabledFn 时不启动（保持纯懒加载行为，主要用于测试）。
func (s *CodexRadarService) Start() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started || s.stopped {
		return
	}
	if s.enabledFn == nil {
		// 无开关回调：不启动定时器，退化为纯懒加载。
		return
	}
	s.started = true
	s.rootCtx, s.cancel = context.WithCancel(context.Background())

	c := cron.New(cron.WithLocation(s.location))
	if _, err := c.AddFunc(codexRadarWarmupSchedule, func() { s.warmup(s.rootCtx, "cron") }); err != nil {
		// schedule 是硬编码常量，理论上不会出错；出错则放弃定时器但不影响懒加载。
		slog.Warn("codexradar: invalid warmup schedule, scheduler disabled", "schedule", codexRadarWarmupSchedule, "error", err)
		s.started = false
		s.cancel()
		return
	}
	c.Start()
	s.cron = c
	slog.Info("codexradar: warmup scheduler started", "schedule", codexRadarWarmupSchedule, "tz", s.location.String())

	// 启动预热：延迟片刻（等 DB/迁移就绪）后异步拉一次，治「重启后缓存空」的冷启动。
	// 用 rootCtx 以便 Stop 时能中断在途拉取，避免拖慢关闭。
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		select {
		case <-time.After(codexRadarStartupDelay):
			s.warmup(s.rootCtx, "startup")
		case <-s.rootCtx.Done():
		}
	}()
}

// Stop 停止定时预热：取消在途拉取、关闭 cron、等待启动预热 goroutine 退出。幂等。
func (s *CodexRadarService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		s.mu.Lock()
		s.stopped = true
		c := s.cron
		s.cron = nil
		if s.cancel != nil {
			s.cancel()
		}
		s.mu.Unlock()

		if c != nil {
			ctx := c.Stop() // 停止调度，返回的 ctx 在在途任务结束后 Done
			select {
			case <-ctx.Done():
			case <-time.After(codexRadarCronStopTimeout):
				slog.Warn("codexradar: warmup scheduler stop timed out")
			}
		}
		s.wg.Wait()
	})
}

// warmup 执行一次预热拉取：功能开关关闭则跳过（不打第三方），否则强制刷新一次
// （绕过 TTL 新鲜度检查，但仍受 singleflight 去重与 minRetry 失败节流保护）。
// reason 仅用于日志区分触发来源（cron / startup）。
func (s *CodexRadarService) warmup(ctx context.Context, reason string) {
	if s == nil || s.enabledFn == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if ctx.Err() != nil {
		return
	}
	if !s.enabledFn(ctx) {
		return // 功能关闭：不拉取第三方数据（opt-in 策略）
	}
	slog.Debug("codexradar: warmup fetch", "reason", reason)
	s.forceRefresh(ctx)
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

// forceRefresh 由定时预热调用：绕过 TTL 新鲜度检查强制拉取一次，
// 以便及时拿到对方日更两次的最新数据。仍受 singleflight 去重与 minRetry 失败节流保护。
func (s *CodexRadarService) forceRefresh(ctx context.Context) {
	_, _, _ = s.sf.Do("refresh", func() (any, error) {
		if last := s.lastAttemptNano.Load(); last != 0 {
			if time.Since(time.Unix(0, last)) < s.minRetry {
				return nil, nil // 距上次尝试太近，节流跳过
			}
		}
		s.lastAttemptNano.Store(time.Now().UnixNano())
		prev, _ := s.cache.Load().(*codexRadarSnapshot)
		if next := s.fetch(ctx, prev); next != nil {
			s.cache.Store(next)
		}
		return nil, nil
	})
}

// fetch 拉取「站长推荐」和「综合智能」两个结构化接口，构造新快照。
// 旧 imageURL/summaryURL 非空时保留兼容抓取，但默认服务不会设置它们。
func (s *CodexRadarService) fetch(ctx context.Context, prev *codexRadarSnapshot) *codexRadarSnapshot {
	fetchCtx, cancel := context.WithTimeout(ctx, codexRadarFetchTimeout)
	defer cancel()

	next := &codexRadarSnapshot{fetchedAt: time.Now()}

	if s.imageURL != "" {
		if body, ctype, _, err := s.get(fetchCtx, s.imageURL, codexRadarMaxImageBytes); err == nil && len(body) > 0 {
			// 后端侧优化：降分辨率 + 重编码，把 ~2.2MB 压到几百 KB，显著缩短跨境下载耗时。
			// 优化在拉取时做一次（不在请求热路径），失败则原样保留，绝不因优化而丢图。
			optBytes, optType := optimizeCodexRadarImage(body, ctype)
			next.imageBytes = optBytes
			next.imageType = optType
			// ETag 由优化后字节内容派生（弱校验），内容不变则 ETag 不变，支持 304 协商缓存。
			next.imageETag = weakETag(optBytes)
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
	}

	if s.summaryURL != "" {
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
	}

	if body, _, _, err := s.get(fetchCtx, s.recommendationsURL, codexRadarMaxSummaryBytes); err == nil && len(body) > 0 {
		next.recommendationsBytes = body
	} else if err != nil {
		slog.Warn("codexradar: fetch recommendations failed", "error", err)
		if prev != nil {
			next.recommendationsBytes = prev.recommendationsBytes
		}
	} else if prev != nil {
		next.recommendationsBytes = prev.recommendationsBytes
	}

	if body, _, _, err := s.get(fetchCtx, s.intelligenceURL, codexRadarMaxSummaryBytes); err == nil && len(body) > 0 {
		next.intelligenceBytes = body
	} else if err != nil {
		slog.Warn("codexradar: fetch intelligence failed", "error", err)
		if prev != nil {
			next.intelligenceBytes = prev.intelligenceBytes
		}
	} else if prev != nil {
		next.intelligenceBytes = prev.intelligenceBytes
	}

	next.ok = len(next.imageBytes) > 0 || len(next.summaryBytes) > 0 || len(next.recommendationsBytes) > 0 || len(next.intelligenceBytes) > 0
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

// optimizeCodexRadarImage 把源站的大图压小：若最长边超过上限则等比缩放到 1080px，
// 再在 JPEG(q82) / PNG 两种编码里取更小的一份。任何环节失败都原样返回入参（绝不丢图）。
// 返回优化后的字节与对应 Content-Type。
func optimizeCodexRadarImage(orig []byte, origType string) ([]byte, string) {
	if len(orig) == 0 {
		return orig, fallbackImageType(origType)
	}
	src, _, err := image.Decode(bytes.NewReader(orig))
	if err != nil {
		// 非可解码图（或异常内容）：原样透传，不阻断功能。
		slog.Warn("codexradar: decode image for optimize failed, serving original", "error", err)
		return orig, fallbackImageType(origType)
	}

	// 等比缩放：仅在最长边超过上限时缩小（绝不放大）。
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	longest := w
	if h > longest {
		longest = h
	}
	dst := src
	if longest > codexRadarMaxImageDimension {
		scale := float64(codexRadarMaxImageDimension) / float64(longest)
		nw, nh := int(float64(w)*scale), int(float64(h)*scale)
		if nw < 1 {
			nw = 1
		}
		if nh < 1 {
			nh = 1
		}
		resized := image.NewRGBA(image.Rect(0, 0, nw, nh))
		xdraw.CatmullRom.Scale(resized, resized.Bounds(), src, b, xdraw.Over, nil)
		dst = resized
	}

	best := orig
	bestType := fallbackImageType(origType)

	// JPEG 编码需铺白底（源图可能带透明通道，否则透明处会变黑）。
	rgb := image.NewRGBA(dst.Bounds())
	stddraw.Draw(rgb, rgb.Bounds(), image.NewUniform(color.White), image.Point{}, stddraw.Src)
	stddraw.Draw(rgb, rgb.Bounds(), dst, dst.Bounds().Min, stddraw.Over)
	var jpgBuf bytes.Buffer
	if err := jpeg.Encode(&jpgBuf, rgb, &jpeg.Options{Quality: codexRadarJPEGQuality}); err == nil && jpgBuf.Len() > 0 && jpgBuf.Len() < len(best) {
		best = jpgBuf.Bytes()
		bestType = "image/jpeg"
	}

	// PNG 编码（缩放后往往也比原图小；文字类图无损更清晰）。取更小者。
	var pngBuf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	if err := enc.Encode(&pngBuf, dst); err == nil && pngBuf.Len() > 0 && pngBuf.Len() < len(best) {
		best = pngBuf.Bytes()
		bestType = "image/png"
	}

	if len(best) < len(orig) {
		slog.Info("codexradar: image optimized", "orig_bytes", len(orig), "opt_bytes", len(best), "type", bestType)
	}
	return best, bestType
}

// fallbackImageType 归一化 Content-Type，缺省 image/png。
func fallbackImageType(ctype string) string {
	if ctype == "" {
		return "image/png"
	}
	return ctype
}

// weakETag 由字节内容派生一个弱 ETag（W/"<sha256前16位>"），内容不变则值稳定。
func weakETag(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	sum := sha256.Sum256(b)
	return `W/"` + hex.EncodeToString(sum[:8]) + `"`
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

// CodexRadarDataResult 是两个结构化雷达接口的只读快照。
type CodexRadarDataResult struct {
	Recommendations []byte
	Intelligence    []byte
	FetchedAt       time.Time
	Available       bool
}

// DataSnapshot 返回「站长推荐」与「综合智能」的原始 JSON，不触发网络。
func (s *CodexRadarService) DataSnapshot() CodexRadarDataResult {
	snap, _ := s.cache.Load().(*codexRadarSnapshot)
	if snap == nil || (len(snap.recommendationsBytes) == 0 && len(snap.intelligenceBytes) == 0) {
		return CodexRadarDataResult{}
	}
	return CodexRadarDataResult{
		Recommendations: snap.recommendationsBytes,
		Intelligence:    snap.intelligenceBytes,
		FetchedAt:       snap.fetchedAt,
		Available:       len(snap.recommendationsBytes) > 0 || len(snap.intelligenceBytes) > 0,
	}
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

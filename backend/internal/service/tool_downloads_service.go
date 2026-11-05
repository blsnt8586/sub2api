package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

const toolDownloadsRepository = "https://github.com/BigPizzaV3/CodexPlusPlus"
const toolDownloadsLatestAPI = "https://api.github.com/repos/BigPizzaV3/CodexPlusPlus/releases/latest"
const toolDownloadsInterval = 24 * time.Hour

type ToolDownloadAsset struct {
	Name         string `json:"name"`
	Platform     string `json:"platform"`
	Architecture string `json:"architecture"`
	Format       string `json:"format"`
	Size         int64  `json:"size"`
	URL          string `json:"url"`
	Digest       string `json:"digest,omitempty"`
}

type ToolDownloadsSnapshot struct {
	ID                     string              `json:"id"`
	Name                   string              `json:"name"`
	RepositoryURL          string              `json:"repository_url"`
	ReleaseURL             string              `json:"release_url"`
	Version                string              `json:"version"`
	PublishedAt            *time.Time          `json:"published_at,omitempty"`
	CheckedAt              *time.Time          `json:"checked_at,omitempty"`
	NextCheckAt            *time.Time          `json:"next_check_at,omitempty"`
	Available              bool                `json:"available"`
	Stale                  bool                `json:"stale"`
	Syncing                bool                `json:"syncing"`
	RefreshIntervalSeconds int                 `json:"refresh_interval_seconds"`
	Assets                 []ToolDownloadAsset `json:"assets"`
}

// Installs complete releases on disk before advertising same-site download URLs.
// Metadata and binaries are shared across visits; failures preserve the last
// complete release, including across process restarts.
type toolDownloadMirror struct {
	client    *http.Client
	endpoint  string
	ctx       context.Context
	cancel    context.CancelFunc
	done      chan struct{}
	mu        sync.RWMutex
	snapshot  ToolDownloadsSnapshot
	etag      string
	flight    singleflight.Group
	mirrorDir string
	spec      toolDownloadSpec
	releases  []toolMirroredRelease
}

func newToolDownloadsService(client *http.Client, endpoint string) *toolDownloadMirror {
	ctx, cancel := context.WithCancel(context.Background())
	return &toolDownloadMirror{client: client, endpoint: endpoint, spec: toolDownloadCatalog[0], ctx: ctx, cancel: cancel, snapshot: ToolDownloadsSnapshot{
		ID: "codexplusplus", Name: "CodexPlusPlus", RepositoryURL: toolDownloadsRepository, ReleaseURL: toolDownloadsRepository + "/releases/latest", RefreshIntervalSeconds: int(toolDownloadsInterval / time.Second), Assets: []ToolDownloadAsset{},
	}}
}

func (s *toolDownloadMirror) start() {
	if err := s.loadMirror(); err != nil {
		slog.Warn("tool downloads: could not restore disk cache", "tool", s.spec.ID, "error", err)
	}
	s.done = make(chan struct{})
	go func() {
		defer close(s.done)
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			_, _ = s.refresh(s.ctx)
			select {
			case <-s.ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

func (s *toolDownloadMirror) Stop() {
	s.cancel()
	if s.done != nil {
		<-s.done
	}
}

func (s *toolDownloadMirror) Snapshot(ctx context.Context) (ToolDownloadsSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return ToolDownloadsSnapshot{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := s.snapshot
	out.Assets = append([]ToolDownloadAsset{}, out.Assets...)
	return out, nil
}

func (s *toolDownloadMirror) refresh(ctx context.Context) (any, error) {
	s.mu.RLock()
	due := s.snapshot.NextCheckAt == nil || !time.Now().Before(*s.snapshot.NextCheckAt)
	s.mu.RUnlock()
	if !due {
		return nil, nil
	}
	ch := s.flight.DoChan("latest", func() (any, error) {
		s.mu.RLock()
		due := s.snapshot.NextCheckAt == nil || !time.Now().Before(*s.snapshot.NextCheckAt)
		etag := s.etag
		s.mu.RUnlock()
		if !due {
			return nil, nil
		}
		s.mu.Lock()
		s.snapshot.Syncing = true
		s.mu.Unlock()
		workCtx, cancel := context.WithTimeout(s.ctx, 30*time.Minute)
		defer cancel()
		release, newETag, unchanged, err := s.fetch(workCtx, etag)
		var mirrored toolMirroredRelease
		if err == nil && !unchanged {
			mirrored, err = s.mirrorRelease(workCtx, release)
		}
		now := time.Now().UTC()
		next := now.Add(toolDownloadsInterval)
		s.mu.Lock()
		defer s.mu.Unlock()
		s.snapshot.Syncing = false
		if err == nil {
			candidate := s.snapshot
			releases := append([]toolMirroredRelease{}, s.releases...)
			if !unchanged {
				candidate = mirrored.Snapshot
				candidate.ID = s.snapshot.ID
				candidate.Name = s.snapshot.Name
				candidate.RepositoryURL = s.spec.RepositoryURL()
				candidate.RefreshIntervalSeconds = int(toolDownloadsInterval / time.Second)
				candidate.Available = true
				releases = []toolMirroredRelease{mirrored}
				for _, old := range s.releases {
					if old.ID != mirrored.ID && len(releases) < 2 {
						releases = append(releases, old)
					}
				}
			}
			candidate.CheckedAt, candidate.NextCheckAt = &now, &next
			candidate.Stale, candidate.Syncing = false, false
			if err = s.saveMirror(candidate, newETag, releases); err == nil {
				s.snapshot, s.etag, s.releases = candidate, newETag, releases
				s.pruneMirror()
			}
		}
		if err != nil {
			next = now.Add(time.Hour)
			s.snapshot.Stale = true
			slog.Warn("tool downloads: release check failed", "repository", s.spec.RepositoryURL(), "error", err, "retry_at", next)
		}
		s.snapshot.CheckedAt = &now
		s.snapshot.NextCheckAt = &next
		return nil, nil // Transient GitHub errors are represented by stale/available.
	})
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case r := <-ch:
		return r.Val, r.Err
	}
}

func (s *toolDownloadMirror) fetch(ctx context.Context, etag string) (ToolDownloadsSnapshot, string, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.endpoint, nil)
	if err != nil {
		return ToolDownloadsSnapshot{}, "", false, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "Sub2API-ToolDownloads")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return ToolDownloadsSnapshot{}, "", false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotModified && etag != "" {
		return ToolDownloadsSnapshot{}, etag, true, nil
	}
	if resp.StatusCode != http.StatusOK {
		return ToolDownloadsSnapshot{}, "", false, fmt.Errorf("release metadata HTTP %d", resp.StatusCode)
	}
	var release struct {
		TagName     string     `json:"tag_name"`
		HTMLURL     string     `json:"html_url"`
		PublishedAt *time.Time `json:"published_at"`
		Draft       bool       `json:"draft"`
		Prerelease  bool       `json:"prerelease"`
		Assets      []struct {
			Name   string `json:"name"`
			Size   int64  `json:"size"`
			URL    string `json:"browser_download_url"`
			Digest string `json:"digest"`
			State  string `json:"state"`
		} `json:"assets"`
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024+1))
	if err != nil || len(data) > 2*1024*1024 {
		return ToolDownloadsSnapshot{}, "", false, fmt.Errorf("invalid release metadata size")
	}
	if err = json.Unmarshal(data, &release); err != nil {
		return ToolDownloadsSnapshot{}, "", false, err
	}
	if release.Draft || release.Prerelease || strings.TrimSpace(release.TagName) == "" || !s.officialURL(release.HTMLURL, "/releases/tag/") {
		return ToolDownloadsSnapshot{}, "", false, fmt.Errorf("invalid stable release")
	}
	out := ToolDownloadsSnapshot{Version: release.TagName, ReleaseURL: release.HTMLURL, PublishedAt: release.PublishedAt, Assets: []ToolDownloadAsset{}}
	for _, a := range release.Assets {
		if a.Size <= 0 || (a.State != "" && a.State != "uploaded") || !s.officialURL(a.URL, "/releases/download/"+url.PathEscape(release.TagName)+"/") {
			continue
		}
		name := strings.ToLower(a.Name)
		asset := ToolDownloadAsset{Name: a.Name, Size: a.Size, URL: a.URL, Digest: a.Digest}
		platform, architecture, format := s.spec.Classify(name)
		if platform == "" {
			continue
		}
		asset.Platform, asset.Architecture, asset.Format = platform, architecture, format
		out.Assets = append(out.Assets, asset)
	}
	if len(out.Assets) == 0 {
		return ToolDownloadsSnapshot{}, "", false, fmt.Errorf("release has no supported installers")
	}
	return out, resp.Header.Get("ETag"), false, nil
}

func (s *toolDownloadMirror) officialURL(raw, pathPrefix string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Host == "github.com" && u.User == nil && u.RawQuery == "" && u.Fragment == "" && path.Clean(u.Path) == u.Path && strings.HasPrefix(u.EscapedPath(), "/"+s.spec.Repository+pathPrefix)
}

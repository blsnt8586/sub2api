package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
)

const toolDownloadMaxBytes int64 = 512 * 1024 * 1024

var toolDownloadFilename = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,199}$`)
var toolDownloadReleaseID = regexp.MustCompile(`^[a-f0-9]{64}$`)

type toolMirroredRelease struct {
	ID       string                `json:"id"`
	Snapshot ToolDownloadsSnapshot `json:"snapshot"`
}

type toolMirrorManifest struct {
	Schema   int                   `json:"schema"`
	Snapshot ToolDownloadsSnapshot `json:"snapshot"`
	ETag     string                `json:"etag"`
	Releases []toolMirroredRelease `json:"releases"`
}

// Only official release assets and GitHub's release CDN may be followed. The
// request's user cannot supply an upstream URL or a filesystem path.
func toolDownloadRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 5 {
		return fmt.Errorf("too many release download redirects")
	}
	if req.URL.Scheme != "https" || req.URL.User != nil {
		return fmt.Errorf("invalid release download redirect")
	}
	switch req.URL.Host {
	case "github.com", "release-assets.githubusercontent.com", "objects.githubusercontent.com":
		return nil
	default:
		return fmt.Errorf("untrusted release download host")
	}
}

func (s *toolDownloadMirror) mirrorRelease(ctx context.Context, release ToolDownloadsSnapshot) (toolMirroredRelease, error) {
	if s.mirrorDir == "" {
		return toolMirroredRelease{}, fmt.Errorf("tool download storage directory is empty")
	}
	if len(release.Assets) == 0 || len(release.Assets) > 12 {
		return toolMirroredRelease{}, fmt.Errorf("unexpected installer count")
	}
	metadata, _ := json.Marshal(release)
	id := fmt.Sprintf("%x", sha256.Sum256(metadata))
	seen := map[string]bool{}
	for _, a := range release.Assets {
		if !toolDownloadFilename.MatchString(a.Name) || seen[a.Name] || a.Size <= 0 || a.Size > toolDownloadMaxBytes || a.URL != s.spec.RepositoryURL()+"/releases/download/"+url.PathEscape(release.Version)+"/"+url.PathEscape(a.Name) {
			return toolMirroredRelease{}, fmt.Errorf("invalid installer metadata")
		}
		seen[a.Name] = true
	}
	// A changed ETag can represent release-note edits only. Reuse the verified
	// immutable installers when the metadata describing the binaries is equal.
	s.mu.RLock()
	var cached *toolMirroredRelease
	for _, old := range s.releases {
		if old.ID == id {
			copy := old
			cached = &copy
			break
		}
	}
	s.mu.RUnlock()
	if cached != nil && s.verifyMirror(*cached) == nil {
		return *cached, nil
	}
	filesRoot := filepath.Join(s.mirrorDir, "files")
	if err := os.MkdirAll(filesRoot, 0700); err != nil {
		return toolMirroredRelease{}, err
	}
	stage := filepath.Join(filesRoot, ".sync-"+id)
	if err := os.MkdirAll(stage, 0700); err != nil {
		return toolMirroredRelease{}, err
	}
	client := *s.client
	client.Timeout = 15 * time.Minute
	client.CheckRedirect = toolDownloadRedirect
	workers, downloadCtx := errgroup.WithContext(ctx)
	workers.SetLimit(3)
	for i := range release.Assets {
		workers.Go(func() error {
			a := &release.Assets[i]
			digest, err := mirrorToolAsset(downloadCtx, &client, *a, filepath.Join(stage, a.Name))
			if err != nil {
				return fmt.Errorf("mirror %s: %w", a.Name, err)
			}
			a.Digest = "sha256:" + digest
			a.URL = "/api/v1/tool-downloads/files/" + id + "/" + a.Name
			return nil
		})
	}
	if err := workers.Wait(); err != nil {
		return toolMirroredRelease{}, err
	}
	out := toolMirroredRelease{ID: id, Snapshot: release}
	final := filepath.Join(filesRoot, id)
	// Existing immutable content is reusable only after full verification.
	if _, err := os.Stat(final); err == nil {
		if err = s.verifyMirror(out); err == nil {
			_ = os.RemoveAll(stage)
			return out, nil
		}
		// Repair a corrupt cache only after a fully verified replacement is ready.
		if err = os.RemoveAll(final); err != nil {
			return toolMirroredRelease{}, err
		}
	}
	if err := os.Rename(stage, final); err != nil {
		return toolMirroredRelease{}, err
	}
	return out, nil
}

// Complete files survive a failed batch, but only the published manifest can
// expose them. Interrupted transfers use .part and are never treated as ready.
func mirrorToolAsset(ctx context.Context, client *http.Client, asset ToolDownloadAsset, path string) (string, error) {
	if digest, err := verifyToolAsset(path, asset); err == nil {
		return digest, nil
	}
	part := path + ".part"
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		_ = os.Remove(part)
		digest, err := downloadToolAsset(ctx, client, asset, part)
		if err == nil {
			if err = os.Rename(part, path); err == nil {
				return digest, nil
			}
		}
		lastErr = err
		_ = os.Remove(part)
		if attempt < 2 {
			timer := time.NewTimer(time.Duration(attempt+1) * time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return "", ctx.Err()
			case <-timer.C:
			}
		}
	}
	return "", lastErr
}

func verifyToolAsset(path string, asset ToolDownloadAsset) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || !st.Mode().IsRegular() || st.Size() != asset.Size {
		return "", fmt.Errorf("cached installer size mismatch")
	}
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return "", err
	}
	digest := hex.EncodeToString(h.Sum(nil))
	if asset.Digest != "" && !strings.EqualFold(asset.Digest, "sha256:"+digest) {
		return "", fmt.Errorf("cached installer SHA256 mismatch")
	}
	return digest, nil
}

func downloadToolAsset(ctx context.Context, client *http.Client, asset ToolDownloadAsset, path string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.URL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Sub2API-ToolDownloads")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("installer HTTP %d", resp.StatusCode)
	}
	if resp.ContentLength >= 0 && resp.ContentLength != asset.Size {
		return "", fmt.Errorf("installer content length mismatch")
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(resp.Body, asset.Size+1))
	if err != nil {
		return "", err
	}
	if n != asset.Size {
		return "", fmt.Errorf("incomplete or oversized installer: got %d, expected %d", n, asset.Size)
	}
	digest := hex.EncodeToString(h.Sum(nil))
	if asset.Digest != "" && !strings.EqualFold(asset.Digest, "sha256:"+digest) {
		return "", fmt.Errorf("installer SHA256 mismatch")
	}
	if err := f.Sync(); err != nil {
		return "", err
	}
	return digest, f.Close()
}

// OpenDownload only serves files in the last two published manifests. No
// GitHub fetch, redirect, or arbitrary local path occurs on the user request.
func (s *toolDownloadMirror) OpenDownload(releaseID, name string) (*os.File, ToolDownloadAsset, error) {
	if !toolDownloadReleaseID.MatchString(releaseID) || !toolDownloadFilename.MatchString(name) {
		return nil, ToolDownloadAsset{}, os.ErrNotExist
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, r := range s.releases {
		if r.ID != releaseID {
			continue
		}
		for _, a := range r.Snapshot.Assets {
			if a.Name != name {
				continue
			}
			f, err := os.Open(filepath.Join(s.mirrorDir, "files", releaseID, name))
			if err != nil {
				return nil, ToolDownloadAsset{}, err
			}
			st, err := f.Stat()
			if err != nil || !st.Mode().IsRegular() || st.Size() != a.Size {
				f.Close()
				return nil, ToolDownloadAsset{}, os.ErrNotExist
			}
			return f, a, nil
		}
	}
	return nil, ToolDownloadAsset{}, os.ErrNotExist
}

func (s *toolDownloadMirror) verifyMirror(r toolMirroredRelease) error {
	if !toolDownloadReleaseID.MatchString(r.ID) || len(r.Snapshot.Assets) == 0 || len(r.Snapshot.Assets) > 12 {
		return fmt.Errorf("invalid mirror manifest")
	}
	for _, a := range r.Snapshot.Assets {
		if !toolDownloadFilename.MatchString(a.Name) || a.Size <= 0 || a.Size > toolDownloadMaxBytes || a.URL != "/api/v1/tool-downloads/files/"+r.ID+"/"+a.Name {
			return fmt.Errorf("invalid cached installer")
		}
		f, err := os.Open(filepath.Join(s.mirrorDir, "files", r.ID, a.Name))
		if err != nil {
			return err
		}
		st, statErr := f.Stat()
		if statErr != nil || !st.Mode().IsRegular() || st.Size() != a.Size {
			f.Close()
			return fmt.Errorf("cached installer size mismatch")
		}
		h := sha256.New()
		_, err = io.Copy(h, f)
		f.Close()
		if err != nil {
			return err
		}
		if a.Digest != "sha256:"+hex.EncodeToString(h.Sum(nil)) {
			return fmt.Errorf("cached installer SHA256 mismatch")
		}
	}
	return nil
}

func (s *toolDownloadMirror) saveMirror(snapshot ToolDownloadsSnapshot, etag string, releases []toolMirroredRelease) error {
	if err := os.MkdirAll(s.mirrorDir, 0700); err != nil {
		return err
	}
	data, err := json.Marshal(toolMirrorManifest{Schema: 1, Snapshot: snapshot, ETag: etag, Releases: releases})
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(s.mirrorDir, ".manifest-")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if _, err = f.Write(data); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), filepath.Join(s.mirrorDir, "manifest.json"))
}

func (s *toolDownloadMirror) loadMirror() error {
	f, err := os.Open(filepath.Join(s.mirrorDir, "manifest.json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	var m toolMirrorManifest
	if err = json.NewDecoder(io.LimitReader(f, 1024*1024)).Decode(&m); err != nil {
		return err
	}
	if m.Schema != 1 || len(m.Releases) == 0 || len(m.Releases) > 2 {
		return fmt.Errorf("invalid persisted mirror manifest")
	}
	for _, r := range m.Releases {
		if s.verifyMirror(r) == nil {
			s.releases = append(s.releases, r)
		}
	}
	if len(s.releases) == 0 {
		return fmt.Errorf("no verified installers in persisted cache")
	}
	if s.releases[0].ID == m.Releases[0].ID {
		s.snapshot = m.Snapshot
		s.etag = m.ETag
	} else {
		s.snapshot = s.releases[0].Snapshot
		s.snapshot.Stale = true
		s.snapshot.NextCheckAt = nil
	}
	// Never trust a persisted public list independently of its verified files.
	s.snapshot.Assets = s.releases[0].Snapshot.Assets
	s.snapshot.ID = s.spec.ID
	s.snapshot.Name = s.spec.Name
	s.snapshot.RepositoryURL = s.spec.RepositoryURL()
	s.snapshot.RefreshIntervalSeconds = int(toolDownloadsInterval / time.Second)
	s.snapshot.Available = true
	s.snapshot.Syncing = false
	return nil
}

// Retain current and previous complete release; drop failed staging directories
// and older immutable versions after publishing. Existing open downloads remain
// readable on Unix even when their directory is subsequently removed.
func (s *toolDownloadMirror) pruneMirror() {
	entries, err := os.ReadDir(filepath.Join(s.mirrorDir, "files"))
	if err != nil {
		return
	}
	keep := map[string]bool{}
	for _, r := range s.releases {
		keep[r.ID] = true
	}
	for _, e := range entries {
		if e.IsDir() && !keep[e.Name()] && (toolDownloadReleaseID.MatchString(e.Name()) || strings.HasPrefix(e.Name(), ".sync-")) {
			_ = os.RemoveAll(filepath.Join(s.mirrorDir, "files", e.Name()))
		}
	}
}

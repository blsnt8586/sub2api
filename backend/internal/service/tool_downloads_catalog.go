package service

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

type toolDownloadSpec struct{ ID, Name, Repository string }

func (s toolDownloadSpec) RepositoryURL() string { return "https://github.com/" + s.Repository }

// Add future tools here. Each keeps independent metadata, files, ETag and
// update schedule, so one publisher's outage cannot hide the other tools.
var toolDownloadCatalog = []toolDownloadSpec{
	{ID: "codexplusplus", Name: "Codex++", Repository: "BigPizzaV3/CodexPlusPlus"},
	{ID: "cc-switch", Name: "CC Switch", Repository: "farion1231/cc-switch"},
}

func (s toolDownloadSpec) Classify(name string) (platform, architecture, format string) {
	if s.ID == "codexplusplus" {
		switch {
		case strings.HasSuffix(name, "-windows-x64-setup.exe"):
			return "windows", "x64", "exe"
		case strings.HasSuffix(name, "-windows-x64.zip"):
			return "windows", "x64", "zip"
		case strings.HasSuffix(name, "-macos-universal.dmg"):
			return "macos", "universal", "dmg"
		}
	}
	if s.ID == "cc-switch" && strings.HasPrefix(name, "cc-switch-") {
		switch {
		case strings.HasSuffix(name, "-windows.msi"):
			return "windows", "x64", "msi"
		case strings.HasSuffix(name, "-windows-portable.zip"):
			return "windows", "x64", "zip"
		case strings.HasSuffix(name, "-windows-arm64.msi"):
			return "windows", "arm64", "msi"
		case strings.HasSuffix(name, "-windows-arm64-portable.zip"):
			return "windows", "arm64", "zip"
		case strings.HasSuffix(name, "-macos.dmg"):
			return "macos", "universal", "dmg"
		case strings.HasSuffix(name, "-linux-x86_64.appimage"):
			return "linux", "x64", "appimage"
		case strings.HasSuffix(name, "-linux-arm64.appimage"):
			return "linux", "arm64", "appimage"
		case strings.HasSuffix(name, "-linux-x86_64.deb"):
			return "linux", "x64", "deb"
		case strings.HasSuffix(name, "-linux-arm64.deb"):
			return "linux", "arm64", "deb"
		}
	}
	return "", "", ""
}

type ToolDownloadsService struct{ mirrors []*toolDownloadMirror }

func NewToolDownloadsService(cfg *config.Config) *ToolDownloadsService {
	root := strings.TrimSpace(os.Getenv("TOOL_DOWNLOADS_DIR"))
	if root == "" {
		data := strings.TrimSpace(os.Getenv("DATA_DIR"))
		if data == "" && cfg != nil {
			data = cfg.Pricing.DataDir
		}
		if data == "" {
			data = "./data"
		}
		root = filepath.Join(data, "tool-downloads")
	}
	s := &ToolDownloadsService{}
	for _, spec := range toolDownloadCatalog {
		m := newToolDownloadsService(&http.Client{Timeout: 15 * time.Second}, "https://api.github.com/repos/"+spec.Repository+"/releases/latest")
		m.spec = spec
		m.snapshot.ID, m.snapshot.Name = spec.ID, spec.Name
		m.snapshot.RepositoryURL, m.snapshot.ReleaseURL = spec.RepositoryURL(), spec.RepositoryURL()+"/releases/latest"
		m.mirrorDir = root
		// Keep the initial Codex++ disk layout and existing download URLs valid.
		if spec.ID != "codexplusplus" {
			m.mirrorDir = filepath.Join(root, spec.ID)
		}
		m.start()
		s.mirrors = append(s.mirrors, m)
	}
	return s
}

func (s *ToolDownloadsService) Stop() {
	for _, m := range s.mirrors {
		m.Stop()
	}
}
func (s *ToolDownloadsService) Catalog(ctx context.Context) ([]ToolDownloadsSnapshot, error) {
	out := make([]ToolDownloadsSnapshot, 0, len(s.mirrors))
	for _, m := range s.mirrors {
		snapshot, err := m.Snapshot(ctx)
		if err != nil {
			return nil, err
		}
		out = append(out, snapshot)
	}
	return out, nil
}
func (s *ToolDownloadsService) OpenDownload(id, name string) (*os.File, ToolDownloadAsset, error) {
	for _, m := range s.mirrors {
		f, a, err := m.OpenDownload(id, name)
		if err == nil {
			return f, a, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return nil, ToolDownloadAsset{}, err
		}
	}
	return nil, ToolDownloadAsset{}, os.ErrNotExist
}

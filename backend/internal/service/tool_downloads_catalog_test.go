//go:build unit

package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestToolDownloadsCCSwitchCatalogAndCrossRepositoryIsolation(t *testing.T) {
	spec := toolDownloadCatalog[1]
	names := []string{"CC-Switch-v1-Windows.msi", "CC-Switch-v1-Windows-Portable.zip", "CC-Switch-v1-Windows-arm64.msi", "CC-Switch-v1-Windows-arm64-Portable.zip", "CC-Switch-v1-macOS.dmg", "CC-Switch-v1-Linux-x86_64.AppImage", "CC-Switch-v1-Linux-arm64.AppImage", "CC-Switch-v1-Linux-x86_64.deb", "CC-Switch-v1-Linux-arm64.deb", "CC-Switch-v1-Windows.msi.sig", "latest.json"}
	assets := []map[string]any{}
	for _, name := range names {
		contents := "installer " + name
		assets = append(assets, map[string]any{"name": name, "size": len(contents), "digest": fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(contents))), "browser_download_url": spec.RepositoryURL() + "/releases/download/v1/" + name})
	}
	// An otherwise valid filename from another repository must never be mirrored.
	assets = append(assets, map[string]any{"name": names[0], "size": 42, "browser_download_url": toolDownloadsRepository + "/releases/download/v1/" + names[0]})
	raw, err := json.Marshal(map[string]any{"tag_name": "v1", "html_url": spec.RepositoryURL() + "/releases/tag/v1", "assets": assets})
	require.NoError(t, err)
	cc, calls := newToolTestService(t, func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, string(raw)) }, nil)
	cc.spec = spec
	cc.snapshot.ID = spec.ID
	cc.snapshot.Name = spec.Name
	cc.snapshot.RepositoryURL = spec.RepositoryURL()
	snapshot := toolRefresh(t, cc)
	require.True(t, snapshot.Available)
	require.Len(t, snapshot.Assets, 9)
	require.EqualValues(t, 9, calls.Load())
	require.Equal(t, "cc-switch", snapshot.ID)
	require.Equal(t, "CC Switch", snapshot.Name)
	require.Equal(t, "arm64", snapshot.Assets[2].Architecture)
	require.Equal(t, "msi", snapshot.Assets[2].Format)
	require.Equal(t, "linux", snapshot.Assets[5].Platform)
	require.Equal(t, "appimage", snapshot.Assets[5].Format)
	require.False(t, cc.officialURL(toolDownloadsRepository+"/releases/tag/v1", "/releases/tag/"))
	codex, _ := newToolTestService(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) }, nil)
	toolRefresh(t, codex)
	catalog := &ToolDownloadsService{mirrors: []*toolDownloadMirror{codex, cc}}
	tools, err := catalog.Catalog(context.Background())
	require.NoError(t, err)
	require.Len(t, tools, 2)
	require.False(t, tools[0].Available)
	require.True(t, tools[1].Available)
	f, a, err := catalog.OpenDownload(cc.releases[0].ID, snapshot.Assets[0].Name)
	require.NoError(t, err)
	f.Close()
	require.Equal(t, snapshot.Assets[0].URL, a.URL)
}

func TestToolDownloadsRetriesAndReusesCompletedFiles(t *testing.T) {
	var fail atomic.Bool
	fail.Store(true)
	var mu sync.Mutex
	calls := map[string]int{}
	s, _ := newToolTestService(t, func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, toolTestRelease("v1")) }, func(w http.ResponseWriter, r *http.Request) {
		name := filepath.Base(r.URL.Path)
		mu.Lock()
		calls[name]++
		mu.Unlock()
		if strings.HasSuffix(name, ".dmg") && fail.Load() {
			w.WriteHeader(503)
			return
		}
		fmt.Fprint(w, "installer "+name)
	})
	first := toolRefresh(t, s)
	require.False(t, first.Available)
	require.True(t, first.Stale)
	fail.Store(false)
	toolDownloadsDue(s)
	second := toolRefresh(t, s)
	require.True(t, second.Available)
	require.Len(t, second.Assets, 3)
	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, 1, calls["CodexPlusPlus-v1-windows-x64-setup.exe"])
	require.Equal(t, 1, calls["CodexPlusPlus-v1-windows-x64.zip"])
	require.Equal(t, 4, calls["CodexPlusPlus-v1-macos-universal.dmg"])
}

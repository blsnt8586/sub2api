//go:build unit

package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestCodexRadarBenchBindingAndStale(t *testing.T) {
	fail := false
	binding := radarBinding{Enabled: true, Verified: true, Benchmark: "bench", Catalog: "v1", Digest: "digest", Selections: []radarSelection{{"gpt-6-astra", "high"}}}
	good := `{"model":"gpt-6-astra","effort":"high","benchmark":"bench","catalog_version":"v1","task_set_sha256":"digest","score_version":"radar-bench-v2-150","score_max":150.0,"score":100.5,"coverage":55,"required_tasks":64,"graded_runs_count":70}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "binding") {
			_ = json.NewEncoder(w).Encode(binding)
			return
		}
		if r.Header.Get("X-Radar-Bench-Selection") != "user-selected-v1" {
			t.Error("missing selection header")
		}
		if fail {
			http.Error(w, "bad gateway", 502)
			return
		}
		_, _ = w.Write([]byte(good))
	}))
	defer srv.Close()
	s := NewCodexRadarService()
	s.benchURL = srv.URL
	first := s.fetchBench(context.Background(), nil)
	if first == nil {
		t.Fatal("no snapshot")
	}
	var data radarBenchData
	_ = json.Unmarshal(first.intelligenceBytes, &data)
	if data.Fresh != 1 || data.Points[0].Score == nil || *data.Points[0].Score != 100.5 {
		t.Fatalf("bad snapshot: %+v", data)
	}
	observed := data.Points[0].ObservedAt
	fail = true
	second := s.fetchBench(context.Background(), first)
	if second == nil {
		t.Fatal("lost old data")
	}
	_ = json.Unmarshal(second.intelligenceBytes, &data)
	if data.Fresh != 0 || !data.Points[0].Stale || !data.Points[0].ObservedAt.Equal(observed) {
		t.Fatal("stale timestamp changed")
	}
	for _, bad := range []string{strings.Replace(good, `"score_max":150.0`, `"score_max":100`, 1), strings.Replace(good, `"digest"`, `"other"`, 1), strings.Replace(good, `100.5`, `151`, 1), `<html>error</html>`} {
		if _, err := decodeRadarBenchPoint([]byte(bad), binding, binding.Selections[0]); err == nil {
			t.Fatal("accepted invalid summary")
		}
	}
}

func TestCodexRadarBenchLive(t *testing.T) {
	if os.Getenv("RADAR_LIVE_TEST") != "1" {
		t.Skip("explicit live verification only")
	}
	s := NewCodexRadarService()
	ctx, cancel := context.WithTimeout(context.Background(), 95*time.Second)
	defer cancel()
	snap := s.fetchBench(ctx, nil)
	if snap == nil {
		t.Fatal("no public summaries available")
	}
	var data radarBenchData
	if err := json.Unmarshal(snap.intelligenceBytes, &data); err != nil {
		t.Fatal(err)
	}
	t.Logf("live verified benchmark=%s fresh=%d expected=%d", data.Benchmark, data.Fresh, data.Expected)
	if data.Fresh == 0 {
		t.Fatal("no fresh points")
	}
}

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"sync"
	"time"
)

type radarSelection struct {
	Model  string `json:"model"`
	Effort string `json:"effort"`
}
type radarBinding struct {
	Origin     string           `json:"transport_origin"`
	Endpoint   string           `json:"endpoint_path"`
	Enabled    bool             `json:"enabled"`
	Verified   bool             `json:"production_read_verified"`
	Benchmark  string           `json:"benchmark"`
	Catalog    string           `json:"catalog_version"`
	Digest     string           `json:"task_set_sha256"`
	Selections []radarSelection `json:"model_efforts"`
}
type radarBenchPoint struct {
	radarSelection
	Score      *float64                   `json:"score"`
	Coverage   int                        `json:"coverage"`
	Required   int                        `json:"required_tasks"`
	Graded     *int                       `json:"graded_runs_count"`
	Sources    map[string]json.RawMessage `json:"source_scores"`
	Statistics json.RawMessage            `json:"statistics"`
	ObservedAt time.Time                  `json:"observed_at"`
	Stale      bool                       `json:"stale"`
}
type radarBenchData struct {
	Benchmark string            `json:"benchmark"`
	Digest    string            `json:"task_set_sha256"`
	ScoreMax  int               `json:"score_max"`
	Points    []radarBenchPoint `json:"points"`
	Expected  int               `json:"expected"`
	Fresh     int               `json:"fresh"`
}

// fetchBench reads only public summaries, with bounded concurrency and per-cell
// stale fallback. A failed refresh never makes an old observation look fresh.
func (s *CodexRadarService) fetchBench(ctx context.Context, prev *codexRadarSnapshot) *codexRadarSnapshot {
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	body, _, _, err := s.get(ctx, s.benchURL+"/data/radar-bench-binding.json", codexRadarMaxSummaryBytes)
	var binding radarBinding
	if err != nil || json.Unmarshal(body, &binding) != nil || !binding.Enabled || !binding.Verified || len(binding.Selections) == 0 || len(binding.Selections) > 100 {
		slog.Warn("codexradar: RadarBench binding unavailable", "error", err)
		return nil
	}
	scoreURL := s.benchURL + "/api/radar-bench-score"
	if binding.Origin != "" || binding.Endpoint != "" {
		// Only the public source's known transport is permitted; never follow an
		// arbitrary origin supplied by a remote binding document.
		if binding.Origin != "https://api.codexradar.com" || binding.Endpoint != "/api/v1/radar-bench-score" {
			slog.Warn("codexradar: unsupported RadarBench transport")
			return nil
		}
		scoreURL = binding.Origin + binding.Endpoint
	}
	old := map[radarSelection]radarBenchPoint{}
	if prev != nil {
		var data radarBenchData
		if json.Unmarshal(prev.intelligenceBytes, &data) == nil && data.Digest == binding.Digest && data.Benchmark == binding.Benchmark {
			for _, p := range data.Points {
				if time.Since(p.ObservedAt) < 24*time.Hour {
					p.Stale = true
					old[p.radarSelection] = p
				}
			}
		}
	}
	result := radarBenchData{Benchmark: binding.Benchmark, Digest: binding.Digest, ScoreMax: 150, Expected: len(binding.Selections), Points: make([]radarBenchPoint, len(binding.Selections))}
	var wg sync.WaitGroup
	sem := make(chan struct{}, 3)
	for i, selection := range binding.Selections {
		wg.Add(1)
		go func(i int, selection radarSelection) {
			defer wg.Done()
			p := radarBenchPoint{radarSelection: selection, Stale: true}
			if v, ok := old[selection]; ok {
				p = v
			}
			defer func() { result.Points[i] = p }()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			q := url.Values{"model": {selection.Model}, "effort": {selection.Effort}, "view": {"summary"}}
			raw, _, _, err := s.get(ctx, scoreURL+"?"+q.Encode(), codexRadarMaxSummaryBytes)
			if err != nil {
				slog.Warn("codexradar: summary request failed", "model", selection.Model, "effort", selection.Effort, "error", err)
				return
			}
			fresh, err := decodeRadarBenchPoint(raw, binding, selection)
			if err != nil {
				slog.Warn("codexradar: invalid summary", "model", selection.Model, "effort", selection.Effort, "error", err)
				return
			}
			fresh.ObservedAt = time.Now().UTC()
			p = fresh
		}(i, selection)
	}
	wg.Wait()
	usable := false
	for _, p := range result.Points {
		if !p.ObservedAt.IsZero() {
			usable = true
		}
		if !p.Stale {
			result.Fresh++
		}
	}
	if !usable {
		slog.Warn("codexradar: no valid RadarBench summaries", "expected", result.Expected)
		return nil
	}
	slog.Info("codexradar: RadarBench refreshed", "fresh", result.Fresh, "expected", result.Expected)
	raw, err := json.Marshal(result)
	if err != nil {
		return nil
	}
	return &codexRadarSnapshot{intelligenceBytes: raw, fetchedAt: time.Now(), ok: true}
}
func decodeRadarBenchPoint(raw []byte, b radarBinding, selection radarSelection) (radarBenchPoint, error) {
	var wire struct {
		radarBenchPoint
		Benchmark string  `json:"benchmark"`
		Catalog   string  `json:"catalog_version"`
		Digest    string  `json:"task_set_sha256"`
		Version   string  `json:"score_version"`
		Max       float64 `json:"score_max"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return radarBenchPoint{}, err
	}
	if wire.radarSelection != selection || wire.Benchmark != b.Benchmark || wire.Catalog != b.Catalog || wire.Digest != b.Digest || wire.Version != "radar-bench-v2-150" || wire.Max != 150 || wire.Required != 64 || wire.Coverage < 0 || wire.Coverage > 64 || (wire.Score != nil && (*wire.Score < 0 || *wire.Score > 150)) {
		return radarBenchPoint{}, fmt.Errorf("invalid RadarBench summary binding")
	}
	wire.Stale = false
	return wire.radarBenchPoint, nil
}

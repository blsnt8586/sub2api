//go:build unit

package service

import "testing"

func TestPassiveHealthStatusThresholds(t *testing.T) {
	latency := 4200
	degradedLatency := 6200
	cases := []struct {
		name     string
		samples  int
		failures int
		p95      *int
		want     string
	}{
		{name: "insufficient evidence", samples: 2, failures: 2, want: "unknown"},
		{name: "healthy", samples: 10, failures: 1, p95: &latency, want: "healthy"},
		{name: "degraded by errors", samples: 10, failures: 2, p95: &latency, want: "degraded"},
		{name: "degraded by latency", samples: 10, failures: 0, p95: &degradedLatency, want: "degraded"},
		{name: "temporary quarantine", samples: 10, failures: 5, p95: &latency, want: "temporarily_unschedulable"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := passiveHealthStatus(tc.samples, tc.failures, tc.p95); got != tc.want {
				t.Fatalf("passiveHealthStatus()=%q want %q", got, tc.want)
			}
		})
	}
}

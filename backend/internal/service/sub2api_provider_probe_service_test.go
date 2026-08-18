//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/sub2apiproviderproberun"
	"github.com/Wei-Shaw/sub2api/ent/sub2apiproviderprobetargetrun"
	"github.com/Wei-Shaw/sub2api/internal/pkg/sub2api"
)

func TestOverallProbeStatusRequiresARealAvailabilitySignal(t *testing.T) {
	cases := []struct{ control, data, traffic, want string }{
		{"healthy", "unknown", "unknown", "unknown"},
		{"healthy", "unknown", "healthy", "healthy"},
		{"healthy", "healthy", "unknown", "healthy"},
		{"healthy", "unknown", "degraded", "degraded"},
		{"unhealthy", "healthy", "healthy", "unhealthy"},
	}
	for _, tc := range cases {
		if got := overallProbeStatus(tc.control, tc.data, tc.traffic); got != tc.want {
			t.Fatalf("overallProbeStatus(%q,%q,%q)=%q, want %q", tc.control, tc.data, tc.traffic, got, tc.want)
		}
	}
}

func TestControlProbeAvailabilityStatus(t *testing.T) {
	healthErr := errors.New("health endpoint failed")
	keysErr := errors.New("keys endpoint failed")
	groupsErr := errors.New("groups endpoint failed")
	cases := []struct {
		name                          string
		healthErr, keysErr, groupsErr error
		wantStatus                    string
		wantErr                       error
	}{
		{name: "all healthy", wantStatus: "healthy"},
		{name: "health endpoint failed", healthErr: healthErr, wantStatus: "degraded", wantErr: healthErr},
		{name: "keys endpoint failed", keysErr: keysErr, wantStatus: "degraded", wantErr: keysErr},
		{name: "both authenticated APIs failed", keysErr: keysErr, groupsErr: groupsErr, wantStatus: "unhealthy", wantErr: keysErr},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, err := controlProbeAvailabilityStatus(tc.healthErr, tc.keysErr, tc.groupsErr)
			if status != tc.wantStatus || !errors.Is(err, tc.wantErr) {
				t.Fatalf("status=%q err=%v, want status=%q err=%v", status, err, tc.wantStatus, tc.wantErr)
			}
		})
	}
}

func TestClassifyProbeErrorRecognizesAuthenticationBoundaries(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{err: &sub2api.ErrCloudflareChallenge{StatusCode: 403, RayID: "ray-1"}, want: "cloudflare_challenge"},
		{err: &sub2api.ErrCloudflareAccessDenied{StatusCode: 403, RayID: "ray-2"}, want: "cloudflare_access_denied"},
		{err: &sub2api.ErrAuthInteractionRequired{Cause: errors.New("refresh rejected")}, want: "auth_interaction_required"},
		{err: errors.New("turnstile token is required"), want: "captcha_required"},
	}
	for _, tc := range cases {
		if got := classifyProbeError(tc.err); got != tc.want {
			t.Fatalf("classifyProbeError(%v)=%q, want %q", tc.err, got, tc.want)
		}
	}
}

func TestClassifyProbeErrorRecognizesRawCloudflareBlockedHTML(t *testing.T) {
	raw := `API returned 403: <!DOCTYPE html><html><title>Attention Required! | Cloudflare</title><body><h1>Sorry, you have been blocked</h1><span>Cloudflare Ray ID: <strong>a2c1be733de96d41</strong></span></body></html>`
	if got := classifyProbeError(errors.New(raw)); got != "cloudflare_access_denied" {
		t.Fatalf("classifyProbeError(raw Cloudflare HTML)=%q, want cloudflare_access_denied", got)
	}
	summary := trimProbeError(errors.New(raw))
	if summary != "Cloudflare access denied (HTTP 403) (CF-Ray: a2c1be733de96d41)" {
		t.Fatalf("trimProbeError(raw Cloudflare HTML)=%q", summary)
	}
}

func TestProbeCadenceDefaultsSeparateControlAndAccountPlanes(t *testing.T) {
	if defaultProbeControlIntervalSeconds != 1800 {
		t.Fatalf("control probe default=%d, want 1800", defaultProbeControlIntervalSeconds)
	}
	if defaultProbeTargetIntervalSeconds != 30 {
		t.Fatalf("account probe interval default=%d, want 30", defaultProbeTargetIntervalSeconds)
	}
	if defaultProbeTimeoutSeconds != 60 {
		t.Fatalf("account probe timeout default=%d, want 60", defaultProbeTimeoutSeconds)
	}
	if defaultProbeDegradedLatencyMS != 5000 {
		t.Fatalf("account probe degraded latency default=%d, want 5000", defaultProbeDegradedLatencyMS)
	}
}

func TestHasProbeTargetForAccountRequiresMatchingPersistedTarget(t *testing.T) {
	targets := []*ent.Sub2APIProviderProbeTarget{
		nil,
		{ID: 1, AccountID: 11},
		{ID: 2, AccountID: 22},
	}
	if !hasProbeTargetForAccount(targets, 22) {
		t.Fatal("matching account target should verify a concurrent duplicate")
	}
	if hasProbeTargetForAccount(targets, 33) {
		t.Fatal("an unrelated target must not hide a target-creation constraint failure")
	}
}

func TestDefaultProbeTargetCreateInputEnablesNewLinkedAccount(t *testing.T) {
	providerID := int64(13)
	keyID := int64(4443)
	groupID := int64(15)
	groupName := "ChatGPT Benefits"
	cfg := &ent.Sub2APIProviderProbeConfig{
		DataEnabled:        false,
		SelectedAccountIds: []int64{},
		AllowMediaProbe:    false,
		FailureThreshold:   3,
		RecoveryThreshold:  2,
	}
	input := defaultProbeTargetCreateInput(providerID, cfg, Account{
		ID: 11, Platform: " openai ", ProviderAPIKeyID: &keyID,
		RemoteGroupID: &groupID, RemoteGroupName: &groupName,
	})

	if !input.Enabled {
		t.Fatal("new linked account probe must not inherit the retired disabled provider data-plane switch")
	}
	if input.IntervalSeconds != 30 || input.TimeoutSeconds != 60 || input.DegradedLatencyMS != 5000 {
		t.Fatalf("new target defaults=%d/%d/%d, want 30/60/5000", input.IntervalSeconds, input.TimeoutSeconds, input.DegradedLatencyMS)
	}
	if input.ProviderID != providerID || input.AccountID != 11 || input.Platform != "openai" {
		t.Fatalf("new target identity=%+v", input)
	}
	if input.ProviderAPIKeyID == nil || *input.ProviderAPIKeyID != keyID || input.RemoteGroupID == nil || *input.RemoteGroupID != groupID {
		t.Fatalf("new target binding=%+v", input)
	}
}

type upstreamLogRetentionRepositoryStub struct {
	results   []Sub2APIUpstreamLogCleanupResult
	errAtCall int
	calls     int
	cutoffs   []time.Time
	batchSize []int
}

func (r *upstreamLogRetentionRepositoryStub) DeleteUpstreamLogsBefore(
	_ context.Context,
	cutoff time.Time,
	batchSize int,
) (Sub2APIUpstreamLogCleanupResult, error) {
	r.calls++
	r.cutoffs = append(r.cutoffs, cutoff)
	r.batchSize = append(r.batchSize, batchSize)
	if r.errAtCall > 0 && r.calls == r.errAtCall {
		return Sub2APIUpstreamLogCleanupResult{}, errors.New("delete failed")
	}
	if r.calls > len(r.results) {
		return Sub2APIUpstreamLogCleanupResult{}, nil
	}
	return r.results[r.calls-1], nil
}

func TestUpstreamManagementLogsHaveThreeDayRetention(t *testing.T) {
	if upstreamLogRetentionPeriod != 72*time.Hour {
		t.Fatalf("retention=%s, want 72h", upstreamLogRetentionPeriod)
	}
}

func TestDeleteExpiredSub2APIUpstreamLogsDrainsAllBatches(t *testing.T) {
	cutoff := time.Date(2026, 8, 14, 6, 30, 0, 0, time.FixedZone("CST", 8*60*60))
	repo := &upstreamLogRetentionRepositoryStub{results: []Sub2APIUpstreamLogCleanupResult{
		{ProviderProbeRuns: 1000, AccountProbeRuns: 1000, OptimizeLogs: 1000},
		{ProviderProbeRuns: 2, AccountProbeRuns: 3, OptimizeLogs: 4},
	}}

	got, err := deleteExpiredSub2APIUpstreamLogs(context.Background(), repo, cutoff, 1000)
	if err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if repo.calls != 2 {
		t.Fatalf("calls=%d, want 2", repo.calls)
	}
	if got.ProviderProbeRuns != 1002 || got.AccountProbeRuns != 1003 || got.OptimizeLogs != 1004 {
		t.Fatalf("deleted=%+v", got)
	}
	for i := range repo.cutoffs {
		if !repo.cutoffs[i].Equal(cutoff.UTC()) || repo.cutoffs[i].Location() != time.UTC {
			t.Fatalf("cutoff[%d]=%s, want %s UTC", i, repo.cutoffs[i], cutoff.UTC())
		}
		if repo.batchSize[i] != 1000 {
			t.Fatalf("batch_size[%d]=%d, want 1000", i, repo.batchSize[i])
		}
	}
}

func TestDeleteExpiredSub2APIUpstreamLogsStopsAfterRepositoryError(t *testing.T) {
	repo := &upstreamLogRetentionRepositoryStub{
		results:   []Sub2APIUpstreamLogCleanupResult{{ProviderProbeRuns: 1000}},
		errAtCall: 2,
	}

	got, err := deleteExpiredSub2APIUpstreamLogs(context.Background(), repo, time.Now(), 1000)
	if err == nil {
		t.Fatal("expected repository error")
	}
	if repo.calls != 2 || got.ProviderProbeRuns != 1000 {
		t.Fatalf("calls=%d deleted=%+v", repo.calls, got)
	}
}

func TestProviderProbeMediaModelGuard(t *testing.T) {
	for _, model := range []string{"gpt-image-2", "grok-video-3", "veo-3.1", "voice-realtime"} {
		if !isProviderProbeMediaModel(model) {
			t.Fatalf("model %q should require media opt-in", model)
		}
	}
	if isProviderProbeMediaModel("claude-sonnet-4-6") {
		t.Fatal("text model was classified as media")
	}
}

func TestCountConsecutiveProbeFailuresUsesCandidateStatus(t *testing.T) {
	runs := []*ent.Sub2APIProviderProbeRun{
		{OverallStatus: sub2apiproviderproberun.OverallStatusDegraded, Details: map[string]any{"candidate_status": "unhealthy"}},
		{OverallStatus: sub2apiproviderproberun.OverallStatusDegraded, Details: map[string]any{"candidate_status": "unhealthy"}},
		{OverallStatus: sub2apiproviderproberun.OverallStatusHealthy, Details: map[string]any{"candidate_status": "healthy"}},
	}
	if got := countConsecutiveProbeFailures(runs); got != 2 {
		t.Fatalf("count=%d, want 2", got)
	}
}

func TestProbeIntervalDueUsesConfiguredCadence(t *testing.T) {
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	tooRecent := now.Add(-59 * time.Second)
	if probeIntervalDueAt(&tooRecent, 60, now) {
		t.Fatal("probe should not run before its configured interval")
	}
	due := now.Add(-60 * time.Second)
	if !probeIntervalDueAt(&due, 60, now) {
		t.Fatal("probe should run exactly at its configured interval")
	}
	if !probeIntervalDueAt(nil, 60, now) {
		t.Fatal("provider without a previous run should be due")
	}
}

func TestProbeIntervalDueWithJitterKeepsConfiguredIntervalAsMinimum(t *testing.T) {
	now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	last := now.Add(-5 * time.Minute)
	jitter := probeIntervalJitterSeconds(&last, 300, 42)
	if jitter < 0 || jitter > 60 {
		t.Fatalf("jitter=%d, want 0..60 seconds", jitter)
	}
	// The same target/run seed must produce the same delay on every 15s scan.
	if probeIntervalJitterSeconds(&last, 300, 42) != jitter {
		t.Fatal("jittered due decision is not deterministic")
	}
	dueAt := last.Add(time.Duration(300+int(jitter)) * time.Second)
	if probeIntervalDueWithJitterAt(&last, 300, 42, dueAt.Add(-time.Nanosecond)) {
		t.Fatal("jittered probe ran before its delayed due time")
	}
	if !probeIntervalDueWithJitterAt(&last, 300, 42, dueAt) {
		t.Fatal("probe should run at the deterministic delayed due time")
	}
}

func TestProviderManualControlProbeNeverFansOutToAccountTargets(t *testing.T) {
	now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	target := &ent.Sub2APIProviderProbeTarget{Enabled: true, IntervalSeconds: 60}
	if shouldRunProviderProbeTarget(target, false, false, now) {
		t.Fatal("a Provider-level manual control check must not run an account target")
	}
	if !shouldRunProviderProbeTarget(target, true, true, now) {
		t.Fatal("an enabled due target should still run from the scheduled account-target cycle")
	}
	lastRun := now.Add(-30 * time.Second)
	target.LastRunAt = &lastRun
	if shouldRunProviderProbeTarget(target, true, true, now) {
		t.Fatal("a scheduled account target must respect its own interval")
	}
}

func TestDisabledAccountTargetStillHasItsOwnTimeline(t *testing.T) {
	now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	multiplier := 0.4
	health := targetHealthFromTarget(
		&ent.Sub2APIProviderProbeTarget{ID: 7, AccountID: 9, Platform: "openai", Enabled: false},
		&Account{ID: 9, Name: "Independent account", Platform: "openai", RemoteGroupMultiplier: &multiplier},
		nil,
		now,
	)
	if health.Status != "disabled" {
		t.Fatalf("status=%q, want disabled", health.Status)
	}
	if len(health.Buckets) != 0 {
		t.Fatalf("probe samples=%d, want no synthetic samples", len(health.Buckets))
	}
	if health.RemoteGroupMultiplier == nil || *health.RemoteGroupMultiplier != multiplier {
		t.Fatalf("remote group multiplier=%v, want %v", health.RemoteGroupMultiplier, multiplier)
	}
}

func TestTargetTimelineContainsOnlyRealProbeEventsInChronologicalOrder(t *testing.T) {
	now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	latencies := []int{910, 5200, 16000}
	errorMessage := "upstream timeout"
	runs := []*ent.Sub2APIProviderProbeTargetRun{
		{Status: sub2apiproviderprobetargetrun.StatusUnhealthy, LatencyMs: &latencies[2], ErrorMessage: &errorMessage, StartedAt: now.Add(-2 * time.Minute), FinishedAt: now.Add(-105 * time.Second)},
		{Status: sub2apiproviderprobetargetrun.StatusHealthy, LatencyMs: &latencies[1], StartedAt: now.Add(-7 * time.Minute), FinishedAt: now.Add(-6*time.Minute - 55*time.Second)},
		{Status: sub2apiproviderprobetargetrun.StatusHealthy, LatencyMs: &latencies[0], StartedAt: now.Add(-12 * time.Minute), FinishedAt: now.Add(-11*time.Minute - 59*time.Second)},
	}

	samples := buildTargetProbeSamples(runs, 5000)
	if len(samples) != len(runs) {
		t.Fatalf("probe samples=%d, want %d real events", len(samples), len(runs))
	}
	for index, sample := range samples {
		if sample.SampleCount != 1 {
			t.Fatalf("sample %d count=%d, want 1", index, sample.SampleCount)
		}
		if index > 0 && sample.StartedAt.Before(samples[index-1].StartedAt) {
			t.Fatalf("samples are not oldest-to-newest: %+v", samples)
		}
	}
	if samples[0].StartedAt != runs[2].StartedAt || samples[2].EndedAt != runs[0].FinishedAt {
		t.Fatalf("real probe timestamps were not preserved: %+v", samples)
	}
	if samples[1].Status != "degraded" || samples[2].Status != "unhealthy" {
		t.Fatalf("statuses=%q,%q, want degraded,unhealthy", samples[1].Status, samples[2].Status)
	}
	if samples[2].LastError == nil || *samples[2].LastError != errorMessage {
		t.Fatalf("last error=%v, want %q", samples[2].LastError, errorMessage)
	}
}

func TestTargetTimelineKeepsOnlyLatestSixtyRealProbeEvents(t *testing.T) {
	now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	runs := make([]*ent.Sub2APIProviderProbeTargetRun, targetHealthTimelineResultLimit+5)
	for index := range runs {
		startedAt := now.Add(-time.Duration(index) * time.Minute)
		runs[index] = &ent.Sub2APIProviderProbeTargetRun{
			Status:     sub2apiproviderprobetargetrun.StatusHealthy,
			StartedAt:  startedAt,
			FinishedAt: startedAt.Add(time.Second),
		}
	}

	samples := buildTargetProbeSamples(runs, 5000)
	if len(samples) != targetHealthTimelineResultLimit {
		t.Fatalf("probe samples=%d, want %d", len(samples), targetHealthTimelineResultLimit)
	}
	if samples[0].StartedAt != runs[targetHealthTimelineResultLimit-1].StartedAt || samples[len(samples)-1].StartedAt != runs[0].StartedAt {
		t.Fatalf("timeline did not retain the latest %d events", targetHealthTimelineResultLimit)
	}
}

func TestTargetHealthUsesEachAccountsRemoteGroupMultiplier(t *testing.T) {
	now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	firstMultiplier := 0.4
	secondMultiplier := 1.25
	minMultiplier := 0.3
	maxMultiplier := 0.8
	tests := []struct {
		name       string
		targetID   int64
		accountID  int64
		multiplier *float64
	}{
		{name: "discounted account", targetID: 1, accountID: 101, multiplier: &firstMultiplier},
		{name: "premium account", targetID: 2, accountID: 202, multiplier: &secondMultiplier},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			health := targetHealthFromTarget(
				&ent.Sub2APIProviderProbeTarget{ID: tt.targetID, AccountID: tt.accountID, Platform: "openai", Enabled: true},
				&Account{
					ID: tt.accountID, Name: tt.name, Platform: "openai", RemoteGroupMultiplier: tt.multiplier,
					Sub2APIOptimizeEnabled: true, Sub2APIMinMultiplier: &minMultiplier, Sub2APIMaxMultiplier: &maxMultiplier,
				},
				nil,
				now,
			)
			if health.RemoteGroupMultiplier == nil || *health.RemoteGroupMultiplier != *tt.multiplier {
				t.Fatalf("account %d multiplier=%v, want %v", tt.accountID, health.RemoteGroupMultiplier, *tt.multiplier)
			}
			if !health.Sub2APIOptimizeEnabled || health.Sub2APIMinMultiplier == nil || *health.Sub2APIMinMultiplier != minMultiplier || health.Sub2APIMaxMultiplier == nil || *health.Sub2APIMaxMultiplier != maxMultiplier {
				t.Fatalf("account %d optimize bounds not preserved: enabled=%v min=%v max=%v", tt.accountID, health.Sub2APIOptimizeEnabled, health.Sub2APIMinMultiplier, health.Sub2APIMaxMultiplier)
			}
		})
	}
}

func TestTargetHealthUsesEffectiveModelAndDegradesSlowSuccessfulProbe(t *testing.T) {
	now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	accountModel := "gpt-5.1-codex"
	legacyTargetModel := "stale-target-model"
	latency := 5292
	health := targetHealthFromTarget(
		&ent.Sub2APIProviderProbeTarget{
			ID: 7, AccountID: 9, Platform: "openai", Enabled: true, IntervalSeconds: 300, DegradedLatencyMs: 2000,
			TestModel: &legacyTargetModel,
		},
		&Account{ID: 9, Name: "Codex account", Platform: "openai", Sub2APITestModel: &accountModel},
		[]*ent.Sub2APIProviderProbeTargetRun{{
			TargetID: 7, AccountID: 9, Platform: "openai",
			Status: sub2apiproviderprobetargetrun.StatusHealthy, LatencyMs: &latency,
			StartedAt: now.Add(-time.Minute), FinishedAt: now.Add(-time.Minute), CreatedAt: now.Add(-time.Minute),
		}},
		now,
	)
	if health.TestModel == nil || *health.TestModel != accountModel {
		t.Fatalf("effective test model=%v, want %q", health.TestModel, accountModel)
	}
	if health.Status != "degraded" {
		t.Fatalf("status=%q, want degraded for %dms latency over 2000ms threshold", health.Status, latency)
	}
	latestBucket := health.Buckets[len(health.Buckets)-1]
	if latestBucket.Status != "degraded" || latestBucket.MaxHealthLatencyMS == nil || *latestBucket.MaxHealthLatencyMS != latency {
		t.Fatalf("latest bucket=%+v, want degraded with latency %dms", latestBucket, latency)
	}
}

func TestTargetHealthRecalculatesHistoricalLatencyUsingCurrentThreshold(t *testing.T) {
	now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	latency := 3930
	status := sub2apiproviderprobetargetrun.StatusDegraded
	health := targetHealthFromTarget(
		&ent.Sub2APIProviderProbeTarget{ID: 7, AccountID: 9, Platform: "openai", Enabled: true, DegradedLatencyMs: 5000},
		&Account{ID: 9, Name: "Codex account", Platform: "openai"},
		[]*ent.Sub2APIProviderProbeTargetRun{{
			TargetID: 7, AccountID: 9, Platform: "openai", Status: status, LatencyMs: &latency,
			CreatedAt: now.Add(-time.Minute), StartedAt: now.Add(-time.Minute), FinishedAt: now.Add(-time.Minute),
		}},
		now,
	)
	if health.Status != "healthy" {
		t.Fatalf("status=%q, want healthy after threshold raised to 5000ms", health.Status)
	}
	foundHealthy := false
	for _, bucket := range health.Buckets {
		if bucket.SampleCount > 0 {
			foundHealthy = bucket.Status == "healthy"
			break
		}
	}
	if !foundHealthy {
		t.Fatal("sampled bucket did not recalculate to healthy after threshold was raised")
	}
}

func TestTargetHealthKeepsUnavailableFailuresIndependentOfLatencyThreshold(t *testing.T) {
	now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	latency := 100
	health := targetHealthFromRun(&ent.Sub2APIProviderProbeTargetRun{
		TargetID: 7, AccountID: 9, Status: sub2apiproviderprobetargetrun.StatusUnhealthy, LatencyMs: &latency,
		CreatedAt: now.Add(-time.Minute), StartedAt: now.Add(-time.Minute), FinishedAt: now.Add(-time.Minute),
	}, 5000)
	if health.Status != "unhealthy" {
		t.Fatalf("status=%q, want unhealthy for a failed probe", health.Status)
	}
}

func TestTargetHealthKeepsHysteresisFailureVisibleWhenLatencyIsBelowThreshold(t *testing.T) {
	now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	latency := 300
	category := "network"
	health := targetHealthFromRun(&ent.Sub2APIProviderProbeTargetRun{
		TargetID: 7, AccountID: 9, Status: sub2apiproviderprobetargetrun.StatusDegraded, LatencyMs: &latency,
		ErrorCategory: &category, CreatedAt: now.Add(-time.Minute), StartedAt: now.Add(-time.Minute), FinishedAt: now.Add(-time.Minute),
	}, 5000)
	if health.Status != "degraded" {
		t.Fatalf("status=%q, want degraded for a hysteresis failure", health.Status)
	}
}

func TestProbeTargetRejectsIndependentTestModel(t *testing.T) {
	model := "stale-target-model"
	if err := validateProbeTargetInput(&Sub2APIProviderProbeTargetInput{TestModel: &model}); err == nil {
		t.Fatal("probe target must reject an independently configured test model")
	}
}

func TestProbeTargetAcceptsThirtySecondMinimumInterval(t *testing.T) {
	minimum := 30
	if err := validateProbeTargetInput(&Sub2APIProviderProbeTargetInput{IntervalSeconds: &minimum}); err != nil {
		t.Fatalf("30-second interval should be valid: %v", err)
	}
	tooShort := 29
	if err := validateProbeTargetInput(&Sub2APIProviderProbeTargetInput{IntervalSeconds: &tooShort}); err == nil {
		t.Fatal("29-second interval should be rejected")
	}
}

func TestBuildProviderHealthOverviewsUsesControlPlaneBuckets(t *testing.T) {
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	windowStart := now.Add(-24 * time.Hour)
	latency120 := 120
	latency640 := 640
	probeError := "keys endpoint failed"
	run := func(providerID int64, at time.Time, control, overall string) *ent.Sub2APIProviderProbeRun {
		return &ent.Sub2APIProviderProbeRun{
			ProviderID:    providerID,
			ControlStatus: sub2apiproviderproberun.ControlStatus(control),
			OverallStatus: sub2apiproviderproberun.OverallStatus(overall),
			DataStatus:    sub2apiproviderproberun.DataStatusUnknown,
			TrafficStatus: sub2apiproviderproberun.TrafficStatusUnknown,
			StartedAt:     at,
			FinishedAt:    at,
			CreatedAt:     at,
		}
	}

	healthy := run(1, windowStart, "healthy", "unknown")
	healthy.HealthLatencyMs = &latency120
	bucketDuration := time.Duration(providerHealthOverviewBucketSeconds) * time.Second
	bucketBoundaryHealthy := run(1, windowStart.Add(bucketDuration), "healthy", "unknown")
	degraded := run(1, windowStart.Add(bucketDuration+2*time.Minute), "degraded", "degraded")
	degraded.HealthLatencyMs = &latency640
	degraded.ErrorMessage = &probeError
	transientFailure := run(1, windowStart.Add(2*bucketDuration+time.Minute), "unhealthy", "degraded")
	confirmedFailure := run(1, windowStart.Add(3*bucketDuration+time.Minute), "unhealthy", "unhealthy")
	dataOnly := run(1, windowStart.Add(23*time.Hour), "unknown", "healthy")
	dataOnly.DataStatus = sub2apiproviderproberun.DataStatusHealthy

	overviews := buildProviderHealthOverviews(
		[]int64{1, 2},
		[]*ent.Sub2APIProviderProbeRun{healthy, bucketBoundaryHealthy, degraded, transientFailure, confirmedFailure, dataOnly},
		now,
	)
	if len(overviews) != 2 {
		t.Fatalf("overview count=%d, want 2", len(overviews))
	}
	first := overviews[0]
	if len(first.Buckets) != providerHealthOverviewBucketCount {
		t.Fatalf("bucket count=%d, want %d", len(first.Buckets), providerHealthOverviewBucketCount)
	}
	if first.Buckets[0].Status != "healthy" || first.Buckets[0].SampleCount != 1 {
		t.Fatalf("first bucket=%+v, want one healthy sample", first.Buckets[0])
	}
	if first.Buckets[1].Status != "degraded" || first.Buckets[1].SampleCount != 2 || first.Buckets[1].HealthySamples != 1 || first.Buckets[1].DegradedSamples != 1 {
		t.Fatalf("second bucket=%+v, want worst-of-two degraded", first.Buckets[1])
	}
	if first.Buckets[1].MaxHealthLatencyMS == nil || *first.Buckets[1].MaxHealthLatencyMS != latency640 || first.Buckets[1].LastError == nil || *first.Buckets[1].LastError != probeError {
		t.Fatalf("second bucket details=%+v", first.Buckets[1])
	}
	if first.Buckets[2].Status != "degraded" {
		t.Fatalf("transient control failure=%q, want degraded", first.Buckets[2].Status)
	}
	if first.Buckets[3].Status != "unhealthy" {
		t.Fatalf("confirmed control failure=%q, want unhealthy", first.Buckets[3].Status)
	}
	if first.Summary != (Sub2APIProviderHealthSummary{Healthy: 1, Degraded: 2, Unhealthy: 1, Unknown: providerHealthOverviewBucketCount - 4}) {
		t.Fatalf("summary=%+v", first.Summary)
	}
	if first.AvailabilityStatus != "unhealthy" || first.LatestControl == nil || first.LatestControl.ControlStatus != "unhealthy" {
		t.Fatalf("latest control status=%q latest=%+v", first.AvailabilityStatus, first.LatestControl)
	}
	if first.EvidenceStatus != "healthy" || first.Latest == nil || first.Latest.ControlStatus != "unknown" {
		t.Fatalf("latest evidence status=%q latest=%+v", first.EvidenceStatus, first.Latest)
	}

	second := overviews[1]
	if second.AvailabilityStatus != "unknown" || second.Latest != nil || second.Summary.Unknown != providerHealthOverviewBucketCount {
		t.Fatalf("empty provider overview=%+v", second)
	}
}

func TestValidateEffectiveProbeConfigRequiresSixHoursForMedia(t *testing.T) {
	current := &ent.Sub2APIProviderProbeConfig{DataIntervalSeconds: 1800, AllowMediaProbe: false}
	allowMedia := true
	if err := validateEffectiveProbeConfig(current, &Sub2APIProviderProbeConfigInput{AllowMediaProbe: &allowMedia}); err == nil {
		t.Fatal("enabling media probes should reject the existing 30-minute interval")
	}

	sixHours := minMediaProbeIntervalSeconds
	if err := validateEffectiveProbeConfig(current, &Sub2APIProviderProbeConfigInput{AllowMediaProbe: &allowMedia, DataIntervalSeconds: &sixHours}); err != nil {
		t.Fatalf("six-hour media interval should be accepted: %v", err)
	}
}

func TestScheduledProbeDataIntervalClampsInvalidMediaConfig(t *testing.T) {
	cfg := &ent.Sub2APIProviderProbeConfig{DataIntervalSeconds: 300, AllowMediaProbe: true}
	if got := scheduledProbeDataIntervalSeconds(cfg); got != minMediaProbeIntervalSeconds {
		t.Fatalf("scheduled media interval=%d, want %d", got, minMediaProbeIntervalSeconds)
	}
	cfg.AllowMediaProbe = false
	if got := scheduledProbeDataIntervalSeconds(cfg); got != 300 {
		t.Fatalf("scheduled text interval=%d, want 300", got)
	}
}

func TestValidateRepresentativeProbeAccountsAllowsMultipleAccountsPerPlatform(t *testing.T) {
	providerID := int64(9)
	accounts := []*Account{
		{ID: 1, Platform: "anthropic", ProviderID: &providerID},
		{ID: 2, Platform: "openai", ProviderID: &providerID},
		{ID: 3, Platform: " OpenAI ", ProviderID: &providerID},
	}
	if err := validateRepresentativeProbeAccounts(providerID, accounts); err != nil {
		t.Fatalf("multiple linked accounts should be accepted regardless of platform: %v", err)
	}
}

func TestValidateRepresentativeProbeAccountsRejectsWrongProviderAndDuplicates(t *testing.T) {
	providerID := int64(9)
	otherProviderID := int64(10)
	if err := validateRepresentativeProbeAccounts(providerID, []*Account{{ID: 1, Platform: "openai", ProviderID: &otherProviderID}}); err == nil {
		t.Fatal("account linked to another provider should be rejected")
	}
	account := &Account{ID: 1, Platform: "openai", ProviderID: &providerID}
	if err := validateRepresentativeProbeAccounts(providerID, []*Account{account, account}); err == nil {
		t.Fatal("duplicate account should be rejected")
	}
}

func TestHealthFromRunDecodesAccountProbeDetails(t *testing.T) {
	checkedAt := time.Date(2026, 8, 14, 12, 30, 0, 0, time.UTC)
	run := &ent.Sub2APIProviderProbeRun{
		ProviderID:    9,
		OverallStatus: sub2apiproviderproberun.OverallStatusHealthy,
		ControlStatus: sub2apiproviderproberun.ControlStatusHealthy,
		DataStatus:    sub2apiproviderproberun.DataStatusHealthy,
		TrafficStatus: sub2apiproviderproberun.TrafficStatusUnknown,
		Details: map[string]any{
			"data_probe_enabled":          true,
			"data_probe_interval_seconds": float64(1800),
			"probe_account_count":         float64(2),
			"account_probes": []any{
				map[string]any{"account_id": float64(11), "account_name": "primary", "platform": "openai", "status": "healthy", "latency_ms": float64(428), "checked_at": checkedAt.Format(time.RFC3339)},
			},
		},
		FinishedAt: checkedAt,
	}

	health := healthFromRun(run)
	if !health.DataProbeEnabled || health.DataProbeInterval != 1800 || health.ProbeAccountCount != 2 {
		t.Fatalf("decoded probe config=%+v", health)
	}
	if len(health.AccountProbes) != 1 || health.AccountProbes[0].AccountID != 11 || health.AccountProbes[0].LatencyMS == nil || *health.AccountProbes[0].LatencyMS != 428 {
		t.Fatalf("decoded account probes=%+v", health.AccountProbes)
	}
}

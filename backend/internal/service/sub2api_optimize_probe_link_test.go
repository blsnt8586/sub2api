//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/sub2apiproviderprobetargetrun"
	"github.com/Wei-Shaw/sub2api/internal/pkg/sub2api"
)

func TestProbeAutoOptimizeInputRequiresConfirmedUnhealthyRun(t *testing.T) {
	category := "rate_limit"
	message := "upstream returned 429"
	target := &ent.Sub2APIProviderProbeTarget{ID: 11, AccountID: 22, DegradedLatencyMs: 5000}

	for _, status := range []sub2apiproviderprobetargetrun.Status{
		sub2apiproviderprobetargetrun.StatusHealthy,
		sub2apiproviderprobetargetrun.StatusDegraded,
	} {
		if _, ok := probeAutoOptimizeInput(target, &ent.Sub2APIProviderProbeTargetRun{ID: 33, Status: status}); ok {
			t.Fatalf("status %q must not trigger automatic optimization", status)
		}
	}

	input, ok := probeAutoOptimizeInput(target, &ent.Sub2APIProviderProbeTargetRun{
		ID: 33, Status: sub2apiproviderprobetargetrun.StatusUnhealthy,
		ErrorCategory: &category, ErrorMessage: &message,
	})
	if !ok {
		t.Fatal("confirmed unhealthy run should trigger automatic optimization")
	}
	if input.TargetID != 11 || input.AccountID != 22 || input.ProbeRunID != 33 || input.DegradedLatencyMS != 5000 {
		t.Fatalf("unexpected trigger identity: %+v", input)
	}
	if input.ErrorCategory != category || input.ErrorMessage != message {
		t.Fatalf("probe evidence was not preserved: %+v", input)
	}
}

func TestAggressiveProbeCandidatesFilterAndOrderOtherGroups(t *testing.T) {
	candidates := aggressiveProbeCandidates([]sub2api.Group{
		{ID: 10, Platform: "openai", Status: "active", RateMultiplier: 1.0}, // current
		{ID: 11, Platform: "openai", Status: "active", RateMultiplier: 0.8},
		{ID: 12, Platform: "openai", Status: "active", RateMultiplier: 0.5},
		{ID: 13, Platform: "openai", Status: "inactive", RateMultiplier: 0.6},
		{ID: 14, Platform: "anthropic", Status: "active", RateMultiplier: 0.7},
		{ID: 15, Platform: "openai", Status: "active", RateMultiplier: 1.4},
	}, "openai", 10, 0.6, 1.0, nil)
	if len(candidates) != 1 || candidates[0].ID != 11 {
		t.Fatalf("unexpected aggressive candidates: %+v", candidates)
	}
}

func TestAggressiveProbeCandidatesHonorsOptionalSelectedGroup(t *testing.T) {
	selected := int64(12)
	candidates := aggressiveProbeCandidates([]sub2api.Group{
		{ID: 11, Platform: "openai", Status: "active", RateMultiplier: 0.7},
		{ID: 12, Platform: "openai", Status: "active", RateMultiplier: 0.8},
		{ID: 13, Platform: "anthropic", Status: "active", RateMultiplier: 0.8},
	}, "openai", 10, 0.5, 1.0, &selected)
	if len(candidates) != 1 || candidates[0].ID != selected {
		t.Fatalf("unexpected restricted candidates: %+v", candidates)
	}
}

func TestSelectAggressiveProbeSuccessPrefersThresholdThenFastest(t *testing.T) {
	groupA := sub2api.Group{ID: 1, Name: "cheap", RateMultiplier: 0.6}
	groupB := sub2api.Group{ID: 2, Name: "mid", RateMultiplier: 0.8}
	groupC := sub2api.Group{ID: 3, Name: "fast", RateMultiplier: 0.9}

	selected, ok := selectAggressiveProbeSuccess([]aggressiveProbeSuccess{
		{group: groupA, latency: 7000},
		{group: groupB, latency: 4500},
		{group: groupC, latency: 2000},
	}, 5000)
	if !ok || selected.group.ID != groupB.ID {
		t.Fatalf("selected=%+v ok=%v, want first in-threshold group B", selected, ok)
	}

	selected, ok = selectAggressiveProbeSuccess([]aggressiveProbeSuccess{
		{group: groupA, latency: 7000},
		{group: groupB, latency: 6000},
	}, 5000)
	if !ok || selected.group.ID != groupB.ID {
		t.Fatalf("selected=%+v ok=%v, want fastest over-threshold group B", selected, ok)
	}
}

func TestAggressiveProbeCandidatesExhaustedRequiresEveryCandidateTestedAndFailed(t *testing.T) {
	tests := []struct {
		name       string
		candidates int
		tested     int
		failed     int
		exhausted  bool
	}{
		{name: "no candidates", candidates: 0, exhausted: true},
		{name: "all tests failed", candidates: 2, tested: 2, failed: 2, exhausted: true},
		{name: "one candidate switch failed", candidates: 2, tested: 1, failed: 1, exhausted: false},
		{name: "one test succeeded", candidates: 2, tested: 2, failed: 1, exhausted: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := aggressiveProbeCandidatesExhausted(tt.candidates, tt.tested, tt.failed); got != tt.exhausted {
				t.Fatalf("exhausted=%v, want %v", got, tt.exhausted)
			}
		})
	}
}

func TestProbeAutoOptimizeInputRejectsAuthenticationAndCloudflareFailures(t *testing.T) {
	target := &ent.Sub2APIProviderProbeTarget{ID: 11, AccountID: 22}
	for _, category := range []string{"auth", "auth_interaction_required", "captcha_required", "cloudflare_challenge", "cloudflare_access_denied"} {
		t.Run(category, func(t *testing.T) {
			if _, ok := probeAutoOptimizeInput(target, &ent.Sub2APIProviderProbeTargetRun{
				ID: 33, Status: sub2apiproviderprobetargetrun.StatusUnhealthy, ErrorCategory: &category,
			}); ok {
				t.Fatalf("category %q must not trigger group optimization", category)
			}
		})
	}
	for _, category := range []string{"rate_limit", "timeout", "network", "upstream_5xx", "protocol", ""} {
		t.Run("allowed_"+category, func(t *testing.T) {
			if _, ok := probeAutoOptimizeInput(target, &ent.Sub2APIProviderProbeTargetRun{
				ID: 33, Status: sub2apiproviderprobetargetrun.StatusUnhealthy, ErrorCategory: &category,
			}); !ok {
				t.Fatalf("category %q should remain eligible for group optimization", category)
			}
		})
	}
}

func TestProbeAutoOptimizeInputUsesStreaksAndCostInterval(t *testing.T) {
	now := time.Date(2026, 8, 24, 9, 0, 0, 0, time.UTC)
	degradedTarget := &ent.Sub2APIProviderProbeTarget{
		ID: 11, AccountID: 22, DegradedLatencyMs: 5000,
		DegradedOptimizeThreshold: 3,
	}
	degraded := sub2apiproviderprobetargetrun.StatusDegraded
	runs := []*ent.Sub2APIProviderProbeTargetRun{
		{Status: degraded}, {Status: degraded}, {Status: degraded},
	}
	input, ok := probeAutoOptimizeInputWithHistory(degradedTarget, runs[0], runs, now)
	if !ok || input.Trigger != OptimizeLogTriggerProbeDegraded {
		t.Fatalf("degraded streak trigger=%+v ok=%v", input, ok)
	}
	input, ok = probeAutoOptimizeInputWithHistory(degradedTarget, runs[0], runs[:2], now)
	if ok {
		t.Fatal("a short degraded streak must not trigger optimization")
	}

	costTarget := &ent.Sub2APIProviderProbeTarget{
		ID: 12, AccountID: 23, CostOptimizeEnabled: true,
		CostOptimizeIntervalSeconds: 1800, CostOptimizeHealthyThreshold: 3,
	}
	healthy := sub2apiproviderprobetargetrun.StatusHealthy
	healthyRuns := []*ent.Sub2APIProviderProbeTargetRun{{Status: healthy}, {Status: healthy}, {Status: healthy}}
	input, ok = probeAutoOptimizeInputWithHistory(costTarget, healthyRuns[0], healthyRuns, now)
	if !ok || input.Trigger != OptimizeLogTriggerProbeCost {
		t.Fatalf("healthy cost trigger=%+v ok=%v", input, ok)
	}
	last := now.Add(-time.Minute)
	costTarget.LastCostOptimizeAt = &last
	if _, ok = probeAutoOptimizeInputWithHistory(costTarget, healthyRuns[0], healthyRuns, now); ok {
		t.Fatal("cost check must respect its interval")
	}
}

func TestProbeCostOptimizeDueAcceptsThirtyMinutes(t *testing.T) {
	now := time.Date(2026, 8, 24, 9, 0, 0, 0, time.UTC)
	target := &ent.Sub2APIProviderProbeTarget{CostOptimizeIntervalSeconds: 1800}
	last := now.Add(-30 * time.Minute)
	target.LastCostOptimizeAt = &last
	if !probeCostOptimizeDue(target, now) {
		t.Fatal("30-minute cost check should be due")
	}
	last = now.Add(-29 * time.Minute)
	target.LastCostOptimizeAt = &last
	if probeCostOptimizeDue(target, now) {
		t.Fatal("30-minute cost check should not be due early")
	}
}

func TestProbeAutoOptimizeUsesSixHealthyProbesByDefault(t *testing.T) {
	now := time.Date(2026, 8, 24, 9, 0, 0, 0, time.UTC)
	target := &ent.Sub2APIProviderProbeTarget{
		ID: 12, AccountID: 23, CostOptimizeEnabled: true,
		CostOptimizeIntervalSeconds: 1800, CostOptimizeHealthyThreshold: 0,
	}
	healthy := sub2apiproviderprobetargetrun.StatusHealthy
	runs := make([]*ent.Sub2APIProviderProbeTargetRun, 0, 6)
	for i := 0; i < 6; i++ {
		runs = append(runs, &ent.Sub2APIProviderProbeTargetRun{Status: healthy})
	}
	if input, ok := probeAutoOptimizeInputWithHistory(target, runs[0], runs[:5], now); ok || input.Trigger != "" {
		t.Fatal("five healthy probes must not trigger the default six-probe check")
	}
	input, ok := probeAutoOptimizeInputWithHistory(target, runs[0], runs, now)
	if !ok || input.Trigger != OptimizeLogTriggerProbeCost {
		t.Fatalf("six healthy probes trigger=%+v ok=%v, want cost check", input, ok)
	}
}

func TestCostOptimizeIdleFilterFailsClosedForUnknownConcurrency(t *testing.T) {
	candidates := []probeAutoOptimizeCandidate{
		{account: Account{ID: 1}, trigger: Sub2APIProbeAutoOptimizeInput{AccountID: 1, Trigger: OptimizeLogTriggerProbeCost}},
		{account: Account{ID: 2}, trigger: Sub2APIProbeAutoOptimizeInput{AccountID: 2, Trigger: OptimizeLogTriggerProbeCost}},
		{account: Account{ID: 3}, trigger: Sub2APIProbeAutoOptimizeInput{AccountID: 3, Trigger: OptimizeLogTriggerProbeDegraded}},
	}

	filtered := filterProbeAutoOptimizeCandidates(candidates, map[int64]int{1: 0, 2: 1}, true)
	if len(filtered) != 2 {
		t.Fatalf("filtered candidates=%d, want idle cost candidate plus degraded candidate", len(filtered))
	}
	if filtered[0].account.ID != 1 || filtered[1].account.ID != 3 {
		t.Fatalf("filtered candidates=%+v, want accounts 1 and 3", filtered)
	}

	filtered = filterProbeAutoOptimizeCandidates(candidates, nil, false)
	if len(filtered) != 1 || filtered[0].account.ID != 3 {
		t.Fatalf("fail-closed filter=%+v, want only non-cost candidate", filtered)
	}
}

func TestProbeAutoOptimizeCandidatesRequireParticipationAndCompleteSettings(t *testing.T) {
	ready := completeOptimizeAccount()
	ready.ID = 1
	disabled := completeOptimizeAccount()
	disabled.ID = 2
	disabled.Sub2APIOptimizeEnabled = false
	incomplete := completeOptimizeAccount()
	incomplete.ID = 3
	incomplete.Sub2APIMinMultiplier = nil

	candidates := probeAutoOptimizeCandidates(
		[]Account{ready, disabled, incomplete},
		[]Sub2APIProbeAutoOptimizeInput{
			{TargetID: 10, AccountID: 1},
			{TargetID: 10, AccountID: 1},
			{TargetID: 20, AccountID: 2},
			{TargetID: 30, AccountID: 3},
			{TargetID: 40, AccountID: 404},
		},
	)
	if len(candidates) != 1 {
		t.Fatalf("candidate count=%d, want only the ready participating account", len(candidates))
	}
	if candidates[0].account.ID != 1 || candidates[0].trigger.TargetID != 10 {
		t.Fatalf("unexpected candidate: %+v", candidates[0])
	}
}

func TestProbeAutoOptimizeCooldownBlocksUntilRejectedClaimIsReleased(t *testing.T) {
	svc := &Sub2APIOptimizeScheduleService{probeCooldownUntil: make(map[int64]time.Time)}
	ctx := context.Background()

	claim, acquired, err := svc.tryClaimProbeAutoOptimizeCooldown(ctx, 7)
	if err != nil || !acquired || claim == nil {
		t.Fatalf("first claim acquired=%v claim=%v err=%v", acquired, claim != nil, err)
	}
	if _, acquired, err := svc.tryClaimProbeAutoOptimizeCooldown(ctx, 7); err != nil || acquired {
		t.Fatalf("second claim during cooldown acquired=%v err=%v", acquired, err)
	}

	// Admission failure must release the claim so a later unhealthy probe is not
	// suppressed for the full cooldown without an optimization ever starting.
	claim.release()
	if _, acquired, err := svc.tryClaimProbeAutoOptimizeCooldown(ctx, 7); err != nil || !acquired {
		t.Fatalf("claim after rejection acquired=%v err=%v", acquired, err)
	}
}

func TestProbeAutoOptimizeRetryAndSuccessCooldownsAreTenMinutes(t *testing.T) {
	svc := &Sub2APIOptimizeScheduleService{probeCooldownUntil: make(map[int64]time.Time)}
	before := time.Now()
	if _, acquired, err := svc.tryClaimProbeAutoOptimizeCooldown(context.Background(), 17); err != nil || !acquired {
		t.Fatalf("retry claim acquired=%v err=%v", acquired, err)
	}
	retryUntil := svc.probeCooldownUntil[17]
	remaining := retryUntil.Sub(before)
	if remaining < sub2apiProbeAutoOptimizeRetryCooldown-time.Second || remaining > sub2apiProbeAutoOptimizeRetryCooldown+time.Second {
		t.Fatalf("retry cooldown remaining=%v, want about %v", remaining, sub2apiProbeAutoOptimizeRetryCooldown)
	}
	if sub2apiProbeAutoOptimizeRetryCooldown != 10*time.Minute || sub2apiProbeAutoOptimizeSuccessCooldown != 10*time.Minute {
		t.Fatalf("cooldowns retry=%v success=%v, want both 10m", sub2apiProbeAutoOptimizeRetryCooldown, sub2apiProbeAutoOptimizeSuccessCooldown)
	}
}

func TestExtendSuccessfulProbeAutoOptimizeCooldownsOnlyUpgradesChangedAccounts(t *testing.T) {
	svc := &Sub2APIOptimizeScheduleService{probeCooldownUntil: make(map[int64]time.Time)}
	claimed := []probeAutoOptimizeCandidate{
		{account: Account{ID: 1}, trigger: Sub2APIProbeAutoOptimizeInput{TargetID: 101}},
		{account: Account{ID: 2}, trigger: Sub2APIProbeAutoOptimizeInput{TargetID: 202}},
	}
	claims := make([]*probeAutoOptimizeCooldownClaim, 0, len(claimed))
	for _, candidate := range claimed {
		claim, acquired, err := svc.tryClaimProbeAutoOptimizeCooldown(context.Background(), candidate.trigger.TargetID)
		if err != nil || !acquired {
			t.Fatalf("initial target %d claim acquired=%v err=%v", candidate.trigger.TargetID, acquired, err)
		}
		claims = append(claims, claim)
	}

	svc.extendSuccessfulProbeAutoOptimizeCooldowns(claimed, claims, []OptimizeAccountDetail{
		{AccountID: 1, Status: "optimized"},
		{AccountID: 2, Status: "skipped"},
	})

	now := time.Now()
	optimizedRemaining := svc.probeCooldownUntil[101].Sub(now)
	skippedRemaining := svc.probeCooldownUntil[202].Sub(now)
	if optimizedRemaining < sub2apiProbeAutoOptimizeSuccessCooldown-time.Second {
		t.Fatalf("optimized cooldown remaining=%v, want about %v", optimizedRemaining, sub2apiProbeAutoOptimizeSuccessCooldown)
	}
	if skippedRemaining > sub2apiProbeAutoOptimizeRetryCooldown+time.Second {
		t.Fatalf("skipped cooldown remaining=%v, want retry window %v", skippedRemaining, sub2apiProbeAutoOptimizeRetryCooldown)
	}
}

func TestProbeAutoOptimizeCooldownIsSharedAcrossInstances(t *testing.T) {
	cache := &fakeLeaderLockCache{}
	first := &Sub2APIOptimizeScheduleService{lockCache: cache, instanceID: "first"}
	second := &Sub2APIOptimizeScheduleService{lockCache: cache, instanceID: "second"}

	claim, acquired, err := first.tryClaimProbeAutoOptimizeCooldown(context.Background(), 8)
	if err != nil || !acquired {
		t.Fatalf("first distributed claim acquired=%v err=%v", acquired, err)
	}
	if _, acquired, err := second.tryClaimProbeAutoOptimizeCooldown(context.Background(), 8); err != nil || acquired {
		t.Fatalf("peer claim acquired=%v err=%v", acquired, err)
	}
	claim.release()
	if _, acquired, err := second.tryClaimProbeAutoOptimizeCooldown(context.Background(), 8); err != nil || !acquired {
		t.Fatalf("peer claim after release acquired=%v err=%v", acquired, err)
	}
}

func TestSuccessfulProbeCooldownExtensionHasNoUnlockedGap(t *testing.T) {
	cache := &fakeLeaderLockCache{}
	first := &Sub2APIOptimizeScheduleService{lockCache: cache, instanceID: "first"}
	second := &Sub2APIOptimizeScheduleService{lockCache: cache, instanceID: "second"}
	candidate := probeAutoOptimizeCandidate{
		account: Account{ID: 1},
		trigger: Sub2APIProbeAutoOptimizeInput{TargetID: 8},
	}

	claim, acquired, err := first.tryClaimProbeAutoOptimizeCooldown(context.Background(), 8)
	if err != nil || !acquired {
		t.Fatalf("first claim acquired=%v err=%v", acquired, err)
	}
	first.extendSuccessfulProbeAutoOptimizeCooldowns(
		[]probeAutoOptimizeCandidate{candidate},
		[]*probeAutoOptimizeCooldownClaim{claim},
		[]OptimizeAccountDetail{{AccountID: 1, Status: "optimized"}},
	)

	if _, acquired, err := second.tryClaimProbeAutoOptimizeCooldown(context.Background(), 8); err != nil || acquired {
		t.Fatalf("peer claim after atomic extension acquired=%v err=%v", acquired, err)
	}
	cache.mu.Lock()
	ttl := cache.ttls["sub2api:probe:auto-optimize:target:8"]
	cache.mu.Unlock()
	if ttl != 10*time.Minute {
		t.Fatalf("extended ttl=%v, want %v", ttl, sub2apiProbeAutoOptimizeSuccessCooldown)
	}
}

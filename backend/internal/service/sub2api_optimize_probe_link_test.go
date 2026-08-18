//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/sub2apiproviderprobetargetrun"
)

func TestProbeAutoOptimizeInputRequiresConfirmedUnhealthyRun(t *testing.T) {
	category := "rate_limit"
	message := "upstream returned 429"
	target := &ent.Sub2APIProviderProbeTarget{ID: 11, AccountID: 22, FailureThreshold: 3}

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
	if input.TargetID != 11 || input.AccountID != 22 || input.ProbeRunID != 33 || input.FailureThreshold != 3 {
		t.Fatalf("unexpected trigger identity: %+v", input)
	}
	if input.ErrorCategory != category || input.ErrorMessage != message {
		t.Fatalf("probe evidence was not preserved: %+v", input)
	}
}

func TestProbeAutoOptimizeInputRejectsAuthenticationAndCloudflareFailures(t *testing.T) {
	target := &ent.Sub2APIProviderProbeTarget{ID: 11, AccountID: 22, FailureThreshold: 3}
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

func TestProbeAutoOptimizeRetryCooldownIsShorterThanSuccessCooldown(t *testing.T) {
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
	if sub2apiProbeAutoOptimizeRetryCooldown >= sub2apiProbeAutoOptimizeSuccessCooldown {
		t.Fatalf("retry cooldown %v must be shorter than success cooldown %v", sub2apiProbeAutoOptimizeRetryCooldown, sub2apiProbeAutoOptimizeSuccessCooldown)
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
	if ttl != sub2apiProbeAutoOptimizeSuccessCooldown {
		t.Fatalf("extended ttl=%v, want %v", ttl, sub2apiProbeAutoOptimizeSuccessCooldown)
	}
}

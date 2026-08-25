package service

import (
	"context"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/google/uuid"
)

const (
	// All probe-triggered optimization outcomes use a short ten-minute guard.
	// This prevents repeated scheduled probes from switching the same key
	// concurrently while still allowing recovery attempts quickly.
	sub2apiProbeAutoOptimizeRetryCooldown   = 10 * time.Minute
	sub2apiProbeAutoOptimizeSuccessCooldown = 10 * time.Minute
	sub2apiProbeAutoOptimizeTimeout         = 10 * time.Minute
	probeAutoOptimizeTriggerCode            = OptimizeLogTriggerProbeAuto
	maxProbeAutoOptimizeHistory             = 20
)

// Sub2APIProbeAutoOptimizeInput is immutable evidence from one persisted route
// probe. Only scheduled probes submit this input; manual probe buttons remain
// observational and never change a remote group.
type Sub2APIProbeAutoOptimizeInput struct {
	TargetID                 int64
	ProbeRunID               int64
	AccountID                int64
	AccountStatusSyncEnabled bool
	Trigger                  string
	DegradedLatencyMS        int
	ErrorCategory            string
	ErrorMessage             string
}

// Sub2APIProbeTargetBindingSyncer updates the monitoring route after the
// optimizer changes the account's persisted remote group binding.
type Sub2APIProbeTargetBindingSyncer interface {
	SyncProbeTargetBindings(context.Context, int64, []int64) error
	MarkProbeTargetsCostOptimize(context.Context, []int64, time.Time) error
}

type probeAutoOptimizeCandidate struct {
	account Account
	trigger Sub2APIProbeAutoOptimizeInput
}

type probeAutoOptimizeCooldownClaim struct {
	release func()
	extend  func(context.Context, time.Duration) (bool, error)
}

// leaderLockExtender is an optional Redis-backed capability. Extending by
// owner is atomic, unlike releasing a short cooldown and reacquiring a long one.
type leaderLockExtender interface {
	ExtendLeaderLock(context.Context, string, string, time.Duration) (bool, error)
}

func probeAutoOptimizeInput(target *ent.Sub2APIProviderProbeTarget, run *ent.Sub2APIProviderProbeTargetRun) (Sub2APIProbeAutoOptimizeInput, bool) {
	// Compatibility helper for callers/tests that only have one persisted run.
	// A single degraded/healthy sample must never trigger an automatic action.
	return probeAutoOptimizeInputWithHistory(target, run, []*ent.Sub2APIProviderProbeTargetRun{run}, time.Now().UTC())
}

func probeAutoOptimizeInputWithHistory(
	target *ent.Sub2APIProviderProbeTarget,
	run *ent.Sub2APIProviderProbeTargetRun,
	recentRuns []*ent.Sub2APIProviderProbeTargetRun,
	now time.Time,
) (Sub2APIProbeAutoOptimizeInput, bool) {
	if target == nil || run == nil {
		return Sub2APIProbeAutoOptimizeInput{}, false
	}
	input := Sub2APIProbeAutoOptimizeInput{
		TargetID:          target.ID,
		ProbeRunID:        run.ID,
		AccountID:         target.AccountID,
		DegradedLatencyMS: target.DegradedLatencyMs,
	}
	status := string(run.Status)
	degradedThreshold := target.DegradedOptimizeThreshold
	if degradedThreshold < 1 {
		degradedThreshold = 3
	}
	healthyThreshold := target.CostOptimizeHealthyThreshold
	if healthyThreshold < 1 {
		healthyThreshold = 6
	}
	switch status {
	case "unhealthy":
		input.Trigger = OptimizeLogTriggerProbeUnhealthy
	case "degraded":
		if consecutiveProbeStatus(recentRuns, "degraded") < degradedThreshold {
			return Sub2APIProbeAutoOptimizeInput{}, false
		}
		input.Trigger = OptimizeLogTriggerProbeDegraded
	case "healthy":
		if !target.CostOptimizeEnabled ||
			consecutiveProbeStatus(recentRuns, "healthy") < healthyThreshold ||
			!probeCostOptimizeDue(target, now) {
			return Sub2APIProbeAutoOptimizeInput{}, false
		}
		input.Trigger = OptimizeLogTriggerProbeCost
	default:
		return Sub2APIProbeAutoOptimizeInput{}, false
	}
	if run.ErrorCategory != nil {
		input.ErrorCategory = *run.ErrorCategory
	}
	if run.ErrorMessage != nil {
		input.ErrorMessage = *run.ErrorMessage
	}
	if input.Trigger == OptimizeLogTriggerProbeUnhealthy && !probeErrorAllowsAutoOptimize(input.ErrorCategory) {
		return Sub2APIProbeAutoOptimizeInput{}, false
	}
	return input, true
}

func consecutiveProbeStatus(runs []*ent.Sub2APIProviderProbeTargetRun, status string) int {
	count := 0
	for _, run := range runs {
		if run == nil {
			continue
		}
		if string(run.Status) != status {
			break
		}
		count++
	}
	return count
}

func probeCostOptimizeDue(target *ent.Sub2APIProviderProbeTarget, now time.Time) bool {
	if target == nil || target.CostOptimizeIntervalSeconds < 1800 {
		return false
	}
	var baseline *time.Time
	if target.LastCostOptimizeAt != nil {
		value := target.LastCostOptimizeAt.UTC()
		baseline = &value
	}
	if target.RouteChangedAt != nil && (baseline == nil || target.RouteChangedAt.After(*baseline)) {
		value := target.RouteChangedAt.UTC()
		baseline = &value
	}
	return baseline == nil || !now.UTC().Before(baseline.Add(time.Duration(target.CostOptimizeIntervalSeconds)*time.Second))
}

func probeErrorAllowsAutoOptimize(category string) bool {
	switch category {
	case "auth", "auth_interaction_required", "captcha_required", "cloudflare_challenge", "cloudflare_access_denied":
		return false
	default:
		return true
	}
}

func probeAutoOptimizeCandidates(accounts []Account, inputs []Sub2APIProbeAutoOptimizeInput) []probeAutoOptimizeCandidate {
	accountsByID := make(map[int64]Account, len(accounts))
	for _, account := range accounts {
		accountsByID[account.ID] = account
	}
	seenTargets := make(map[int64]struct{}, len(inputs))
	seenAccounts := make(map[int64]struct{}, len(inputs))
	candidates := make([]probeAutoOptimizeCandidate, 0, len(inputs))
	for _, input := range inputs {
		if input.TargetID <= 0 || input.AccountID <= 0 {
			continue
		}
		if _, duplicate := seenTargets[input.TargetID]; duplicate {
			continue
		}
		if _, duplicate := seenAccounts[input.AccountID]; duplicate {
			continue
		}
		account, ok := accountsByID[input.AccountID]
		if !ok || !account.Sub2APIOptimizeEnabled || optimizeAccountConfigError(&account) != "" {
			continue
		}
		seenTargets[input.TargetID] = struct{}{}
		seenAccounts[input.AccountID] = struct{}{}
		candidates = append(candidates, probeAutoOptimizeCandidate{account: account, trigger: input})
	}
	return candidates
}

// TriggerProbeAutoOptimize claims per-target cooldowns, acquires the same
// Provider execution lock used by manual/cron optimization, then runs the
// affected accounts as one asynchronous batch. The method returns only after
// the batch is safely admitted, keeping the probe scheduler responsive.
func (s *Sub2APIOptimizeScheduleService) TriggerProbeAutoOptimize(
	ctx context.Context,
	providerID int64,
	inputs []Sub2APIProbeAutoOptimizeInput,
) (int, error) {
	admission, err := s.TriggerProbeAutoOptimizeWithAdmission(ctx, providerID, inputs)
	return len(admission.TargetIDs), err
}

// TriggerProbeAutoOptimizeWithAdmission is the admission-aware variant used
// by scheduled probes. It reports the exact targets whose cooldown and
// provider execution locks were claimed before the asynchronous optimization
// starts, so the probe can quarantine only the accounts that were not claimed.
func (s *Sub2APIOptimizeScheduleService) TriggerProbeAutoOptimizeWithAdmission(
	ctx context.Context,
	providerID int64,
	inputs []Sub2APIProbeAutoOptimizeInput,
) (Sub2APIProbeAutoOptimizeAdmission, error) {
	if s == nil || len(inputs) == 0 {
		return Sub2APIProbeAutoOptimizeAdmission{}, nil
	}
	accounts, err := s.providerSvc.accountRepo.ListByProviderID(ctx, providerID)
	if err != nil {
		return Sub2APIProbeAutoOptimizeAdmission{}, fmt.Errorf("list probe auto-optimize accounts: %w", err)
	}
	candidates := probeAutoOptimizeCandidates(accounts, inputs)
	candidates = s.onlyIdleCostOptimizeCandidates(ctx, providerID, candidates)
	if len(candidates) == 0 {
		return Sub2APIProbeAutoOptimizeAdmission{}, nil
	}

	claimed := make([]probeAutoOptimizeCandidate, 0, len(candidates))
	cooldownClaims := make([]*probeAutoOptimizeCooldownClaim, 0, len(candidates))
	for _, candidate := range candidates {
		cooldownClaim, acquired, claimErr := s.tryClaimProbeAutoOptimizeCooldown(ctx, candidate.trigger.TargetID)
		if claimErr != nil {
			logger.LegacyPrintf("service.sub2api_optimize_probe", "[Sub2APIProbeAutoOptimize] provider=%d target=%d cooldown claim failed: %v", providerID, candidate.trigger.TargetID, claimErr)
			continue
		}
		if !acquired {
			continue
		}
		claimed = append(claimed, candidate)
		cooldownClaims = append(cooldownClaims, cooldownClaim)
	}
	if len(claimed) == 0 {
		return Sub2APIProbeAutoOptimizeAdmission{}, nil
	}

	releaseClaims := func() {
		for _, claim := range cooldownClaims {
			claim.release()
		}
	}
	releaseExecution, acquired := s.tryAcquire(ctx, providerID)
	if !acquired {
		// A manual/cron run already owns the Provider. Do not burn the probe
		// cooldown; the next unhealthy probe should be able to submit again.
		releaseClaims()
		deferredDetails := make([]OptimizeAccountDetail, 0, len(claimed))
		extraByAccount := make(map[int64]map[string]any, len(claimed))
		for _, candidate := range claimed {
			deferredDetails = append(deferredDetails, OptimizeAccountDetail{
				AccountID:   candidate.account.ID,
				AccountName: candidate.account.Name,
				Status:      "skipped",
			})
			extra := map[string]any{
				"probe_target_id":     candidate.trigger.TargetID,
				"probe_run_id":        candidate.trigger.ProbeRunID,
				"degraded_latency_ms": candidate.trigger.DegradedLatencyMS,
			}
			if candidate.trigger.ErrorCategory != "" {
				extra["probe_error_category"] = candidate.trigger.ErrorCategory
			}
			extra["probe_trigger"] = candidate.trigger.Trigger
			extraByAccount[candidate.account.ID] = extra
		}
		logCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		if err := s.persistOptimizeDeferredLog(
			logCtx,
			providerID,
			nil,
			OptimizeLogTriggerProbeAuto,
			time.Now(),
			"同一上游已有优化任务正在执行，本次探针联动已让位；冷却未消耗，后续异常探针可重试",
			deferredDetails,
			extraByAccount,
		); err != nil {
			logger.LegacyPrintf("service.sub2api_optimize_probe", "[Sub2APIProbeAutoOptimize] provider=%d persist deferred log failed: %v", providerID, err)
		}
		cancel()
		return Sub2APIProbeAutoOptimizeAdmission{}, nil
	}
	provider, err := s.providerSvc.repo.GetByID(ctx, providerID)
	if err != nil {
		releaseExecution()
		releaseClaims()
		return Sub2APIProbeAutoOptimizeAdmission{}, fmt.Errorf("get provider for probe auto-optimize: %w", err)
	}

	startedAt := time.Now()
	logger.LegacyPrintf("service.sub2api_optimize_probe", "[Sub2APIProbeAutoOptimize] provider=%d admitted %d unhealthy target(s)", providerID, len(claimed))
	go func() {
		defer releaseExecution()
		bgCtx, cancel := context.WithTimeout(context.Background(), sub2apiProbeAutoOptimizeTimeout)
		defer cancel()
		// Candidate-group tests are part of the probe state machine. They must
		// not invoke ordinary account-test status side effects either.
		bgCtx = suppressAccountTestStatusMutation(bgCtx)

		batch := make([]Account, 0, len(claimed))
		triggersByAccount := make(map[int64]Sub2APIProbeAutoOptimizeInput, len(claimed))
		probePolicies := make(map[int64]probeOptimizePolicy, len(claimed))
		for _, candidate := range claimed {
			batch = append(batch, candidate.account)
			triggersByAccount[candidate.account.ID] = candidate.trigger
			probePolicies[candidate.account.ID] = probeOptimizePolicy{
				trigger:           candidate.trigger.Trigger,
				degradedLatencyMS: candidate.trigger.DegradedLatencyMS,
			}
		}
		details := s.optimizeAccountsWithProbePolicies(bgCtx, provider, batch, probePolicies)
		s.extendSuccessfulProbeAutoOptimizeCooldowns(claimed, cooldownClaims, details)
		s.finishProbeAutoOptimize(providerID, startedAt, details, triggersByAccount)
	}()

	admission := Sub2APIProbeAutoOptimizeAdmission{
		TargetIDs:  make([]int64, 0, len(claimed)),
		AccountIDs: make([]int64, 0, len(claimed)),
	}
	for _, candidate := range claimed {
		admission.TargetIDs = append(admission.TargetIDs, candidate.trigger.TargetID)
		admission.AccountIDs = append(admission.AccountIDs, candidate.account.ID)
	}
	return admission, nil
}

func (s *Sub2APIOptimizeScheduleService) onlyIdleCostOptimizeCandidates(
	ctx context.Context,
	providerID int64,
	candidates []probeAutoOptimizeCandidate,
) []probeAutoOptimizeCandidate {
	accountIDs := make([]int64, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.trigger.Trigger == OptimizeLogTriggerProbeCost {
			accountIDs = append(accountIDs, candidate.account.ID)
		}
	}
	if len(accountIDs) == 0 {
		return candidates
	}
	if s.concurrencyReader == nil {
		return filterProbeAutoOptimizeCandidates(candidates, nil, false)
	}
	concurrency, err := s.concurrencyReader.GetAccountConcurrencyBatch(ctx, accountIDs)
	if err != nil {
		logger.LegacyPrintf("service.sub2api_optimize_probe", "[Sub2APIProbeAutoOptimize] provider=%d cost check deferred: concurrency unavailable: %v", providerID, err)
		return filterProbeAutoOptimizeCandidates(candidates, nil, false)
	}
	return filterProbeAutoOptimizeCandidates(candidates, concurrency, true)
}

func filterProbeAutoOptimizeCandidates(candidates []probeAutoOptimizeCandidate, concurrency map[int64]int, allowCost bool) []probeAutoOptimizeCandidate {
	filtered := make([]probeAutoOptimizeCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		active, known := concurrency[candidate.account.ID]
		if candidate.trigger.Trigger != OptimizeLogTriggerProbeCost || (allowCost && known && active == 0) {
			filtered = append(filtered, candidate)
		}
	}
	return filtered
}

// tryClaimProbeAutoOptimizeCooldown claims the ten-minute retry window.
func (s *Sub2APIOptimizeScheduleService) tryClaimProbeAutoOptimizeCooldown(ctx context.Context, targetID int64) (*probeAutoOptimizeCooldownClaim, bool, error) {
	return s.tryClaimProbeAutoOptimizeCooldownFor(ctx, targetID, sub2apiProbeAutoOptimizeRetryCooldown)
}

func (s *Sub2APIOptimizeScheduleService) tryClaimProbeAutoOptimizeCooldownFor(
	ctx context.Context,
	targetID int64,
	ttl time.Duration,
) (*probeAutoOptimizeCooldownClaim, bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if ttl <= 0 {
		ttl = sub2apiProbeAutoOptimizeRetryCooldown
	}
	if s.lockCache != nil {
		key := fmt.Sprintf("sub2api:probe:auto-optimize:target:%d", targetID)
		owner := fmt.Sprintf("%s:%s", s.instanceID, uuid.NewString())
		claimCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		acquired, err := s.lockCache.TryAcquireLeaderLock(claimCtx, key, owner, ttl)
		cancel()
		if err != nil {
			return nil, false, err
		}
		if !acquired {
			return nil, false, nil
		}
		claim := &probeAutoOptimizeCooldownClaim{}
		claim.release = func() {
			releaseCtx, releaseCancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer releaseCancel()
			_ = s.lockCache.ReleaseLeaderLock(releaseCtx, key, owner)
		}
		claim.extend = func(extendCtx context.Context, newTTL time.Duration) (bool, error) {
			extender, ok := s.lockCache.(leaderLockExtender)
			if !ok {
				return false, nil
			}
			return extender.ExtendLeaderLock(extendCtx, key, owner, newTTL)
		}
		return claim, true, nil
	}

	now := time.Now()
	expiresAt := now.Add(ttl)
	s.probeCooldownMu.Lock()
	if s.probeCooldownUntil == nil {
		s.probeCooldownUntil = make(map[int64]time.Time)
	}
	if current, exists := s.probeCooldownUntil[targetID]; exists && now.Before(current) {
		s.probeCooldownMu.Unlock()
		return nil, false, nil
	}
	s.probeCooldownUntil[targetID] = expiresAt
	s.probeCooldownMu.Unlock()
	claim := &probeAutoOptimizeCooldownClaim{}
	claim.release = func() {
		s.probeCooldownMu.Lock()
		if current, exists := s.probeCooldownUntil[targetID]; exists && current.Equal(expiresAt) {
			delete(s.probeCooldownUntil, targetID)
		}
		s.probeCooldownMu.Unlock()
	}
	claim.extend = func(_ context.Context, newTTL time.Duration) (bool, error) {
		s.probeCooldownMu.Lock()
		defer s.probeCooldownMu.Unlock()
		current, exists := s.probeCooldownUntil[targetID]
		if !exists || !current.Equal(expiresAt) {
			return false, nil
		}
		expiresAt = time.Now().Add(newTTL)
		s.probeCooldownUntil[targetID] = expiresAt
		return true, nil
	}
	return claim, true, nil
}

// extendSuccessfulProbeAutoOptimizeCooldowns records the same ten-minute
// window for actual group changes; failed/no-op attempts already have it.
func (s *Sub2APIOptimizeScheduleService) extendSuccessfulProbeAutoOptimizeCooldowns(
	claimed []probeAutoOptimizeCandidate,
	claims []*probeAutoOptimizeCooldownClaim,
	details []OptimizeAccountDetail,
) {
	optimizedAccounts := make(map[int64]struct{}, len(details))
	for _, detail := range details {
		if detail.Status == "optimized" {
			optimizedAccounts[detail.AccountID] = struct{}{}
		}
	}
	for i, candidate := range claimed {
		if _, optimized := optimizedAccounts[candidate.account.ID]; !optimized {
			continue
		}
		if i >= len(claims) || claims[i] == nil || claims[i].extend == nil {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		extended, err := claims[i].extend(ctx, sub2apiProbeAutoOptimizeSuccessCooldown)
		cancel()
		if err != nil || !extended {
			logger.LegacyPrintf(
				"service.sub2api_optimize_probe",
				"[Sub2APIProbeAutoOptimize] provider target=%d success cooldown extension extended=%v err=%v",
				candidate.trigger.TargetID,
				extended,
				err,
			)
		}
	}
}

func (s *Sub2APIOptimizeScheduleService) finishProbeAutoOptimize(
	providerID int64,
	startedAt time.Time,
	details []OptimizeAccountDetail,
	triggersByAccount map[int64]Sub2APIProbeAutoOptimizeInput,
) {
	// An admitted probe failure defers account status while asynchronous group
	// comparison runs. A successful switch clears only the probe-owned error;
	// manual/admin errors remain untouched. Inputs that were not admitted are
	// projected by the probe immediately instead of reaching this path.
	for _, detail := range details {
		trigger, ok := triggersByAccount[detail.AccountID]
		if !ok || !trigger.AccountStatusSyncEnabled || (trigger.Trigger != OptimizeLogTriggerProbeUnhealthy && trigger.Trigger != OptimizeLogTriggerProbeDegraded) || s.providerSvc == nil || s.providerSvc.accountRepo == nil {
			continue
		}
		stateCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if detail.Status == "failed" && detail.ProbeExhausted {
			if err := markProbeManagedAccountError(stateCtx, s.providerSvc.accountRepo, detail.AccountID, detail.Reason); err != nil {
				logger.LegacyPrintf("service.sub2api_optimize_probe", "[Sub2APIProbeAutoOptimize] provider=%d account=%d mark exhausted account error failed: %v", providerID, detail.AccountID, err)
			}
			if err := markProbeGroupsExhausted(stateCtx, s.providerSvc.accountRepo, detail.AccountID, detail.Reason); err != nil {
				logger.LegacyPrintf("service.sub2api_optimize_probe", "[Sub2APIProbeAutoOptimize] provider=%d account=%d persist exhausted state failed: %v", providerID, detail.AccountID, err)
			}
		} else if detail.Status == "optimized" || detail.Status == "skipped" {
			if err := clearProbeManagedAccountError(stateCtx, s.providerSvc.accountRepo, detail.AccountID); err != nil {
				logger.LegacyPrintf("service.sub2api_optimize_probe", "[Sub2APIProbeAutoOptimize] provider=%d account=%d clear probe account state failed: %v", providerID, detail.AccountID, err)
			}
		}
		cancel()
	}
	optimizedAccountIDs := make([]int64, 0, len(details))
	for _, detail := range details {
		if detail.Status == "optimized" {
			optimizedAccountIDs = append(optimizedAccountIDs, detail.AccountID)
		}
	}
	if len(optimizedAccountIDs) > 0 && s.probeBindingSyncer != nil {
		syncCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := s.probeBindingSyncer.SyncProbeTargetBindings(syncCtx, providerID, optimizedAccountIDs); err != nil {
			logger.LegacyPrintf("service.sub2api_optimize_probe", "[Sub2APIProbeAutoOptimize] provider=%d sync target bindings failed: %v", providerID, err)
		}
		cancel()
	}
	if s.probeBindingSyncer != nil {
		targetIDs := make([]int64, 0, len(triggersByAccount))
		for _, trigger := range triggersByAccount {
			if trigger.Trigger == OptimizeLogTriggerProbeCost {
				targetIDs = append(targetIDs, trigger.TargetID)
			}
		}
		markCtx, markCancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := s.probeBindingSyncer.MarkProbeTargetsCostOptimize(markCtx, targetIDs, time.Now().UTC()); err != nil {
			logger.LegacyPrintf("service.sub2api_optimize_probe", "[Sub2APIProbeAutoOptimize] provider=%d mark optimize time failed: %v", providerID, err)
		}
		markCancel()
	}

	logCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	extraByAccount := make(map[int64]map[string]any, len(triggersByAccount))
	for _, detail := range details {
		if trigger, ok := triggersByAccount[detail.AccountID]; ok {
			item := map[string]any{
				"probe_target_id":     trigger.TargetID,
				"probe_run_id":        trigger.ProbeRunID,
				"degraded_latency_ms": trigger.DegradedLatencyMS,
				"probe_trigger":       trigger.Trigger,
			}
			if trigger.ErrorCategory != "" {
				item["probe_error_category"] = trigger.ErrorCategory
			}
			if trigger.ErrorMessage != "" {
				item["probe_error_message"] = trigger.ErrorMessage
			}
			extraByAccount[detail.AccountID] = item
		}
	}
	// Older callers/tests may not provide a per-account trigger. Keep their
	// historical probe-unhealthy log value while new scheduled probes retain
	// the generic probe-auto owner plus the concrete trigger in detail.
	logTrigger := probeAutoOptimizeTriggerCode
	hasExplicitTrigger := false
	for _, trigger := range triggersByAccount {
		if trigger.Trigger != "" {
			hasExplicitTrigger = true
			break
		}
	}
	if !hasExplicitTrigger && len(triggersByAccount) > 0 {
		logTrigger = OptimizeLogTriggerProbeUnhealthy
	}
	if err := s.persistOptimizeLog(logCtx, providerID, nil, logTrigger, startedAt, details, extraByAccount); err != nil {
		logger.LegacyPrintf("service.sub2api_optimize_probe", "[Sub2APIProbeAutoOptimize] provider=%d persist log failed: %v", providerID, err)
	}
}

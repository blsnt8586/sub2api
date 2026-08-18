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
	// A failed or no-op optimization may be retried after two 5-minute probe
	// cycles. Only a real group change earns the longer anti-flap cooldown.
	sub2apiProbeAutoOptimizeRetryCooldown   = 10 * time.Minute
	sub2apiProbeAutoOptimizeSuccessCooldown = time.Hour
	sub2apiProbeAutoOptimizeTimeout         = 10 * time.Minute
	probeAutoOptimizeTriggerCode            = OptimizeLogTriggerProbeUnhealthy
)

// Sub2APIProbeAutoOptimizeInput is immutable evidence from one persisted route
// probe. Only scheduled probes submit this input; manual probe buttons remain
// observational and never change a remote group.
type Sub2APIProbeAutoOptimizeInput struct {
	TargetID         int64
	ProbeRunID       int64
	AccountID        int64
	FailureThreshold int
	ErrorCategory    string
	ErrorMessage     string
}

// Sub2APIProbeTargetBindingSyncer updates the monitoring route after the
// optimizer changes the account's persisted remote group binding.
type Sub2APIProbeTargetBindingSyncer interface {
	SyncProbeTargetBindings(context.Context, int64, []int64) error
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
	if target == nil || run == nil || string(run.Status) != "unhealthy" {
		return Sub2APIProbeAutoOptimizeInput{}, false
	}
	input := Sub2APIProbeAutoOptimizeInput{
		TargetID:         target.ID,
		ProbeRunID:       run.ID,
		AccountID:        target.AccountID,
		FailureThreshold: target.FailureThreshold,
	}
	if run.ErrorCategory != nil {
		input.ErrorCategory = *run.ErrorCategory
	}
	if run.ErrorMessage != nil {
		input.ErrorMessage = *run.ErrorMessage
	}
	if !probeErrorAllowsAutoOptimize(input.ErrorCategory) {
		return Sub2APIProbeAutoOptimizeInput{}, false
	}
	return input, true
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
	if s == nil || len(inputs) == 0 {
		return 0, nil
	}
	accounts, err := s.providerSvc.accountRepo.ListByProviderID(ctx, providerID)
	if err != nil {
		return 0, fmt.Errorf("list probe auto-optimize accounts: %w", err)
	}
	candidates := probeAutoOptimizeCandidates(accounts, inputs)
	if len(candidates) == 0 {
		return 0, nil
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
		return 0, nil
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
				"probe_target_id":   candidate.trigger.TargetID,
				"probe_run_id":      candidate.trigger.ProbeRunID,
				"failure_threshold": candidate.trigger.FailureThreshold,
			}
			if candidate.trigger.ErrorCategory != "" {
				extra["probe_error_category"] = candidate.trigger.ErrorCategory
			}
			extraByAccount[candidate.account.ID] = extra
		}
		logCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		if err := s.persistOptimizeDeferredLog(
			logCtx,
			providerID,
			nil,
			OptimizeLogTriggerProbeUnhealthy,
			time.Now(),
			"同一上游已有优化任务正在执行，本次探针联动已让位；冷却未消耗，后续异常探针可重试",
			deferredDetails,
			extraByAccount,
		); err != nil {
			logger.LegacyPrintf("service.sub2api_optimize_probe", "[Sub2APIProbeAutoOptimize] provider=%d persist deferred log failed: %v", providerID, err)
		}
		cancel()
		return 0, nil
	}
	provider, err := s.providerSvc.repo.GetByID(ctx, providerID)
	if err != nil {
		releaseExecution()
		releaseClaims()
		return 0, fmt.Errorf("get provider for probe auto-optimize: %w", err)
	}

	startedAt := time.Now()
	logger.LegacyPrintf("service.sub2api_optimize_probe", "[Sub2APIProbeAutoOptimize] provider=%d admitted %d unhealthy target(s)", providerID, len(claimed))
	go func() {
		defer releaseExecution()
		bgCtx, cancel := context.WithTimeout(context.Background(), sub2apiProbeAutoOptimizeTimeout)
		defer cancel()

		batch := make([]Account, 0, len(claimed))
		triggersByAccount := make(map[int64]Sub2APIProbeAutoOptimizeInput, len(claimed))
		for _, candidate := range claimed {
			batch = append(batch, candidate.account)
			triggersByAccount[candidate.account.ID] = candidate.trigger
		}
		details := s.optimizeAccounts(bgCtx, provider, batch)
		s.extendSuccessfulProbeAutoOptimizeCooldowns(claimed, cooldownClaims, details)
		s.finishProbeAutoOptimize(providerID, startedAt, details, triggersByAccount)
	}()

	return len(claimed), nil
}

// tryClaimProbeAutoOptimizeCooldown uses the shorter retry window initially.
// A completed group change upgrades that target to the success cooldown; a
// failed or no-op attempt remains eligible for a later scheduled probe retry.
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

// extendSuccessfulProbeAutoOptimizeCooldowns keeps failed/no-op attempts on the
// retry cooldown while upgrading actual group changes to the anti-flap window.
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

	logCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	extraByAccount := make(map[int64]map[string]any, len(triggersByAccount))
	for _, detail := range details {
		if trigger, ok := triggersByAccount[detail.AccountID]; ok {
			item := map[string]any{
				"probe_target_id":   trigger.TargetID,
				"probe_run_id":      trigger.ProbeRunID,
				"failure_threshold": trigger.FailureThreshold,
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
	if err := s.persistOptimizeLog(logCtx, providerID, nil, probeAutoOptimizeTriggerCode, startedAt, details, extraByAccount); err != nil {
		logger.LegacyPrintf("service.sub2api_optimize_probe", "[Sub2APIProbeAutoOptimize] provider=%d persist log failed: %v", providerID, err)
	}
}

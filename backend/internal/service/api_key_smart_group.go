package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"go.uber.org/zap"
)

const (
	smartGroupOutcomeQueueSize = 1024
	smartGroupWorkerCount      = 4
	smartGroupProbeQueueSize   = 128
	smartGroupProbeWorkerCount = 2
	smartGroupProbeLease       = 3 * time.Minute
	smartGroupProbeCooldown    = 30 * time.Second
	smartGroupProbeTimeout     = 45 * time.Second
	smartGroupMaxProbeAccounts = 3

	SmartGroupSwitchReasonFailure      = "failure"
	SmartGroupSwitchReasonCostRecovery = "cost_recovery"
)

type APIKeySmartGroupRuntimeState struct {
	APIKeyID                int64
	UserID                  int64
	Key                     string
	GroupID                 int64
	SmartGroupIDs           []int64
	FailureThreshold        int
	RecoveryIntervalSeconds int
	ConsecutiveFailures     int
	HealthySince            *time.Time
	LastProbeAt             *time.Time
	ProbeLeaseUntil         *time.Time
}

// APIKeySmartGroupSwitchLog is a successful automatic route change for an API
// key.  Group names are snapshots captured at switch time so the history stays
// readable even when an administrator later renames or removes a group.
type APIKeySmartGroupSwitchLog struct {
	ID            int64     `json:"id"`
	APIKeyID      int64     `json:"api_key_id"`
	FromGroupID   int64     `json:"from_group_id"`
	FromGroupName string    `json:"from_group_name"`
	ToGroupID     int64     `json:"to_group_id"`
	ToGroupName   string    `json:"to_group_name"`
	Reason        string    `json:"reason"`
	SwitchedAt    time.Time `json:"switched_at"`
}

// APIKeySmartGroupLogRepository is intentionally separate from the runtime
// state repository.  This keeps the smart-group state machine and existing
// test/dynamic repositories source-compatible while allowing the SQL-backed
// repository to expose the history UI and retention cleanup.
type APIKeySmartGroupLogRepository interface {
	ListSmartGroupSwitchLogs(ctx context.Context, userID, apiKeyID int64, since time.Time, limit int) ([]APIKeySmartGroupSwitchLog, error)
	DeleteExpiredSmartGroupSwitchLogs(ctx context.Context, before time.Time) error
}

type APIKeySmartGroupStateRepository interface {
	RecordSmartGroupOutcome(ctx context.Context, apiKeyID, groupID int64, success bool, errorMessage string, now time.Time) (*APIKeySmartGroupRuntimeState, error)
	TryAcquireSmartGroupProbe(ctx context.Context, apiKeyID, groupID int64, now time.Time, cooldown, lease time.Duration) (*APIKeySmartGroupRuntimeState, bool, error)
	CompleteSmartGroupSwitch(ctx context.Context, apiKeyID, expectedGroupID, newGroupID int64, reason string, expectedLeaseUntil, now time.Time) (string, bool, error)
	CompleteSmartGroupProbeWithoutSwitch(ctx context.Context, apiKeyID, expectedGroupID int64, reason, errorMessage string, expectedLeaseUntil, now time.Time) (string, error)
}

// smartGroupFailureSwitchRepository is implemented by the SQL state store.
// Failure failover deliberately has no probe step: once the configured failure
// threshold is reached, the next request should use another configured group
// immediately. Cost recovery continues to use the probe-gated methods above.
// Keep this as an optional capability so lightweight test/dynamic repositories
// remain source-compatible with the state-machine interface.
type smartGroupFailureSwitchRepository interface {
	CompleteSmartGroupFailureSwitch(ctx context.Context, apiKeyID, expectedGroupID, newGroupID int64, now time.Time) (string, bool, error)
}

// SmartGroupProbeAccountRepository is an optional narrow extension of
// AccountRepository. It includes accounts that are currently marked error or
// unschedulable only when that state was written by the probe state machine,
// allowing a later probe to recover them without re-enabling manually disabled
// accounts.
type SmartGroupProbeAccountRepository interface {
	ListProbeCandidatesByGroupIDAndPlatform(ctx context.Context, groupID int64, platform string) ([]Account, error)
}

type smartGroupOutcome struct {
	apiKeyID     int64
	groupID      int64
	success      bool
	errorMessage string
}

type smartGroupProbeJob struct {
	state *APIKeySmartGroupRuntimeState
}

type APIKeySmartGroupService struct {
	repo              APIKeySmartGroupStateRepository
	apiKeyService     *APIKeyService
	accountRepo       AccountRepository
	accountTest       *AccountTestService
	userGroupRateRepo UserGroupRateRepository

	queues     []chan smartGroupOutcome
	probeQueue chan smartGroupProbeJob
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	startOnce  sync.Once
	stopOnce   sync.Once
}

func NewAPIKeySmartGroupService(
	repo APIKeySmartGroupStateRepository,
	apiKeyService *APIKeyService,
	accountRepo AccountRepository,
	accountTest *AccountTestService,
	userGroupRateRepo UserGroupRateRepository,
) *APIKeySmartGroupService {
	ctx, cancel := context.WithCancel(context.Background())
	svc := &APIKeySmartGroupService{
		repo:              repo,
		apiKeyService:     apiKeyService,
		accountRepo:       accountRepo,
		accountTest:       accountTest,
		userGroupRateRepo: userGroupRateRepo,
		queues:            make([]chan smartGroupOutcome, smartGroupWorkerCount),
		probeQueue:        make(chan smartGroupProbeJob, smartGroupProbeQueueSize),
		ctx:               ctx,
		cancel:            cancel,
	}
	return svc
}

func (s *APIKeySmartGroupService) Start() {
	if s == nil || s.repo == nil {
		return
	}
	s.startOnce.Do(func() {
		for i := 0; i < smartGroupWorkerCount; i++ {
			s.queues[i] = make(chan smartGroupOutcome, smartGroupOutcomeQueueSize/smartGroupWorkerCount)
			s.wg.Add(1)
			go s.worker(s.queues[i])
		}
		for i := 0; i < smartGroupProbeWorkerCount; i++ {
			s.wg.Add(1)
			go s.probeWorker()
		}
		if logRepo, ok := s.repo.(APIKeySmartGroupLogRepository); ok {
			s.wg.Add(1)
			go s.cleanupSwitchLogs(logRepo)
		}
	})
}

func (s *APIKeySmartGroupService) probeWorker() {
	defer s.wg.Done()
	for {
		select {
		case <-s.ctx.Done():
			return
		case job := <-s.probeQueue:
			if job.state == nil {
				continue
			}
			probeCtx, cancel := context.WithTimeout(s.ctx, 4*time.Minute)
			now := time.Now()
			state, acquired, err := s.repo.TryAcquireSmartGroupProbe(probeCtx, job.state.APIKeyID, job.state.GroupID, now, smartGroupProbeCooldown, smartGroupProbeLease)
			if err != nil {
				logger.FromContext(probeCtx).Warn("api_key.smart_group_probe_lease_failed", zap.Int64("api_key_id", job.state.APIKeyID), zap.Error(err))
			} else if acquired && state != nil {
				s.probeAndMaybeSwitch(probeCtx, state, SmartGroupSwitchReasonCostRecovery, now)
			}
			cancel()
		}
	}
}

func (s *APIKeySmartGroupService) cleanupSwitchLogs(repo APIKeySmartGroupLogRepository) {
	defer s.wg.Done()
	baseCtx := s.ctx
	if baseCtx == nil {
		baseCtx = context.Background()
	}
	cleanup := func() {
		ctx, cancel := context.WithTimeout(baseCtx, 15*time.Second)
		defer cancel()
		if err := repo.DeleteExpiredSmartGroupSwitchLogs(ctx, time.Now().Add(-SmartGroupSwitchLogRetention)); err != nil {
			logger.FromContext(ctx).Warn("api_key.smart_group_log_cleanup_failed", zap.Error(err))
		}
	}
	// Clean immediately on startup, then hourly.  Writes also prune old rows,
	// so retention remains bounded even on installations with no traffic.
	cleanup()
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-baseCtx.Done():
			return
		case <-ticker.C:
			cleanup()
		}
	}
}

func (s *APIKeySmartGroupService) Stop() {
	if s == nil || s.cancel == nil {
		return
	}
	s.stopOnce.Do(func() {
		s.cancel()
		s.wg.Wait()
	})
}

func (s *APIKeySmartGroupService) ShouldObserveSuccess(apiKey *APIKey, now time.Time) bool {
	if apiKey == nil || !apiKey.SmartGroupEnabled || apiKey.GroupID == nil {
		return false
	}
	if apiKey.SmartGroupConsecutiveFailures > 0 || apiKey.SmartGroupHealthySince == nil {
		return true
	}
	interval := apiKey.SmartGroupRecoveryIntervalSeconds
	if interval <= 0 {
		interval = SmartGroupDefaultRecoveryIntervalSeconds
	}
	return !now.Before(apiKey.SmartGroupHealthySince.Add(time.Duration(interval) * time.Second))
}

func (s *APIKeySmartGroupService) SubmitOutcome(apiKey *APIKey, success bool, errorMessage string) {
	if s == nil || apiKey == nil || !apiKey.SmartGroupEnabled || apiKey.GroupID == nil || len(apiKey.SmartGroupIDs) < 2 {
		return
	}
	outcome := smartGroupOutcome{
		apiKeyID:     apiKey.ID,
		groupID:      *apiKey.GroupID,
		success:      success,
		errorMessage: strings.TrimSpace(errorMessage),
	}
	s.enqueue(outcome)
}

func (s *APIKeySmartGroupService) enqueue(outcome smartGroupOutcome) {
	if len(s.queues) == 0 {
		return
	}
	queue := s.queues[int(outcome.apiKeyID%int64(len(s.queues)))]
	if queue == nil {
		return
	}
	select {
	case queue <- outcome:
	default:
		logger.FromContext(context.Background()).Warn("api_key.smart_group_outcome_queue_full", zap.Int64("api_key_id", outcome.apiKeyID))
	}
}

func (s *APIKeySmartGroupService) worker(queue <-chan smartGroupOutcome) {
	defer s.wg.Done()
	for {
		select {
		case <-s.ctx.Done():
			return
		case outcome := <-queue:
			s.processOutcome(outcome)
		}
	}
}

func (s *APIKeySmartGroupService) processOutcome(outcome smartGroupOutcome) {
	now := time.Now()
	ctx, cancel := context.WithTimeout(s.ctx, 4*time.Minute)
	defer cancel()

	state, err := s.repo.RecordSmartGroupOutcome(ctx, outcome.apiKeyID, outcome.groupID, outcome.success, outcome.errorMessage, now)
	if err != nil || state == nil {
		if err != nil {
			logger.FromContext(ctx).Warn("api_key.smart_group_record_outcome_failed", zap.Int64("api_key_id", outcome.apiKeyID), zap.Error(err))
		}
		return
	}
	if s.apiKeyService != nil {
		s.apiKeyService.InvalidateAuthCacheByKey(ctx, state.Key)
	}

	if state.ConsecutiveFailures >= state.FailureThreshold {
		s.switchAfterFailure(ctx, state, now)
		return
	}
	if !outcome.success {
		return
	}

	// Cost-recovery probing can spend several minutes on account timeouts. Put
	if s.probeQueue == nil {
		return
	}
	select {
	case s.probeQueue <- smartGroupProbeJob{state: state}:
	default:
		logger.FromContext(ctx).Warn("api_key.smart_group_probe_queue_full", zap.Int64("api_key_id", state.APIKeyID))
	}
}

func (s *APIKeySmartGroupService) switchAfterFailure(ctx context.Context, state *APIKeySmartGroupRuntimeState, now time.Time) {
	if state == nil {
		return
	}
	candidates, err := s.runtimeCandidates(ctx, state)
	if err != nil {
		logger.FromContext(ctx).Warn("api_key.smart_group_failure_candidates_failed", zap.Int64("api_key_id", state.APIKeyID), zap.Error(err))
		return
	}
	ordered := orderedSmartGroupProbeCandidates(candidates, state.GroupID, SmartGroupSwitchReasonFailure)
	if len(ordered) == 0 {
		logger.FromContext(ctx).Warn("api_key.smart_group_failure_no_candidate", zap.Int64("api_key_id", state.APIKeyID), zap.Int64("group_id", state.GroupID))
		return
	}
	repo, ok := s.repo.(smartGroupFailureSwitchRepository)
	if !ok {
		// Older/dynamic repositories can still use the previous probe-gated
		// behavior until their state store gains the atomic direct-switch method.
		logger.FromContext(ctx).Warn("api_key.smart_group_failure_direct_switch_unsupported", zap.Int64("api_key_id", state.APIKeyID))
		return
	}
	key, switched, err := repo.CompleteSmartGroupFailureSwitch(ctx, state.APIKeyID, state.GroupID, ordered[0].group.ID, now)
	if err != nil {
		logger.FromContext(ctx).Warn("api_key.smart_group_failure_switch_failed", zap.Int64("api_key_id", state.APIKeyID), zap.Error(err))
		return
	}
	if !switched {
		return
	}
	if s.apiKeyService != nil {
		s.apiKeyService.InvalidateAuthCacheByKey(ctx, key)
	}
	logger.FromContext(ctx).Info("api_key.smart_group_switched",
		zap.Int64("api_key_id", state.APIKeyID),
		zap.Int64("from_group_id", state.GroupID),
		zap.Int64("to_group_id", ordered[0].group.ID),
		zap.String("reason", SmartGroupSwitchReasonFailure),
	)
}

type runtimeSmartGroupCandidate struct {
	group Group
	rate  float64
}

func (s *APIKeySmartGroupService) probeAndMaybeSwitch(ctx context.Context, state *APIKeySmartGroupRuntimeState, reason string, now time.Time) {
	candidates, err := s.runtimeCandidates(ctx, state)
	if err != nil {
		s.finishWithoutSwitch(ctx, state, reason, err.Error(), now)
		return
	}
	ordered := orderedSmartGroupProbeCandidates(candidates, state.GroupID, reason)
	if len(ordered) == 0 {
		s.finishWithoutSwitch(ctx, state, reason, "no eligible candidate group", now)
		return
	}

	errorsByGroup := make([]string, 0, len(ordered))
	for _, candidate := range ordered {
		if err := s.probeGroup(ctx, candidate.group); err != nil {
			errorsByGroup = append(errorsByGroup, fmt.Sprintf("%s: %v", candidate.group.Name, err))
			continue
		}
		expectedLeaseUntil := time.Time{}
		if state.ProbeLeaseUntil != nil {
			expectedLeaseUntil = *state.ProbeLeaseUntil
		}
		key, switched, err := s.repo.CompleteSmartGroupSwitch(ctx, state.APIKeyID, state.GroupID, candidate.group.ID, reason, expectedLeaseUntil, time.Now())
		if err != nil {
			logger.FromContext(ctx).Warn("api_key.smart_group_switch_failed", zap.Int64("api_key_id", state.APIKeyID), zap.Error(err))
			return
		}
		if !switched {
			s.finishWithoutSwitch(ctx, state, reason, "active group state changed while probing", time.Now())
			return
		}
		if s.apiKeyService != nil {
			s.apiKeyService.InvalidateAuthCacheByKey(ctx, key)
			logger.FromContext(ctx).Info("api_key.smart_group_switched",
				zap.Int64("api_key_id", state.APIKeyID),
				zap.Int64("from_group_id", state.GroupID),
				zap.Int64("to_group_id", candidate.group.ID),
				zap.String("reason", reason),
			)
		}
		return
	}

	message := strings.Join(errorsByGroup, "; ")
	if message == "" {
		message = "all candidate group probes failed"
	}
	s.finishWithoutSwitch(ctx, state, reason, message, now)
}

func (s *APIKeySmartGroupService) runtimeCandidates(ctx context.Context, state *APIKeySmartGroupRuntimeState) ([]runtimeSmartGroupCandidate, error) {
	if s.apiKeyService == nil {
		return nil, fmt.Errorf("API key service is unavailable")
	}
	available, err := s.apiKeyService.GetAvailableGroups(ctx, state.UserID)
	if err != nil {
		return nil, err
	}
	selected := make(map[int64]struct{}, len(state.SmartGroupIDs))
	for _, id := range state.SmartGroupIDs {
		selected[id] = struct{}{}
	}
	rates := map[int64]float64{}
	if s.userGroupRateRepo != nil {
		loaded, loadErr := s.userGroupRateRepo.GetByUserID(ctx, state.UserID)
		if loadErr != nil {
			// Cost-recovery decisions must use the user's effective rates. A
			// transient rate-store failure must not silently turn a more
			// expensive group into the selected route.
			return nil, fmt.Errorf("load user group rates: %w", loadErr)
		}
		rates = loaded
	}
	result := make([]runtimeSmartGroupCandidate, 0, len(selected))
	for _, group := range available {
		if _, ok := selected[group.ID]; !ok || !smartGroupPlatformSupported(group.Platform) {
			continue
		}
		rate := group.RateMultiplier
		if userRate, ok := rates[group.ID]; ok {
			rate = userRate
		}
		result = append(result, runtimeSmartGroupCandidate{group: group, rate: rate})
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].rate == result[j].rate {
			return result[i].group.ID < result[j].group.ID
		}
		return result[i].rate < result[j].rate
	})
	return result, nil
}

func orderedSmartGroupProbeCandidates(candidates []runtimeSmartGroupCandidate, currentGroupID int64, reason string) []runtimeSmartGroupCandidate {
	currentIndex := -1
	currentRate := 0.0
	for i := range candidates {
		if candidates[i].group.ID == currentGroupID {
			currentIndex = i
			currentRate = candidates[i].rate
			break
		}
	}
	if currentIndex < 0 {
		if reason == SmartGroupSwitchReasonFailure {
			return append([]runtimeSmartGroupCandidate(nil), candidates...)
		}
		if reason == SmartGroupSwitchReasonCostRecovery {
			// The editor may have removed the old active route. When a real
			// request later reaches the stable recovery threshold, probe the
			// saved candidates from cheapest to most expensive.
			return append([]runtimeSmartGroupCandidate(nil), candidates...)
		}
		return nil
	}
	if reason == SmartGroupSwitchReasonCostRecovery {
		result := make([]runtimeSmartGroupCandidate, 0, currentIndex)
		for _, candidate := range candidates {
			if candidate.rate < currentRate {
				result = append(result, candidate)
			}
		}
		return result
	}

	// Failure failover advances through the effective-rate order and wraps at
	// the end. This avoids immediately bouncing from B back to the just-failed A
	// while still preserving the configured low-to-high routing preference.
	result := make([]runtimeSmartGroupCandidate, 0, len(candidates)-1)
	for offset := 1; offset < len(candidates); offset++ {
		index := (currentIndex + offset) % len(candidates)
		result = append(result, candidates[index])
	}
	return result
}

func (s *APIKeySmartGroupService) probeGroup(ctx context.Context, group Group) error {
	if s.accountRepo == nil || s.accountTest == nil {
		return fmt.Errorf("account probe service is unavailable")
	}
	var accounts []Account
	var err error
	if probeRepo, ok := s.accountRepo.(SmartGroupProbeAccountRepository); ok {
		accounts, err = probeRepo.ListProbeCandidatesByGroupIDAndPlatform(ctx, group.ID, group.Platform)
	} else {
		// Compatibility fallback for test/dynamic repositories that have not yet
		// implemented the recovery-aware query.
		accounts, err = s.accountRepo.ListSchedulableByGroupIDAndPlatform(ctx, group.ID, group.Platform)
	}
	if err != nil {
		return err
	}
	if len(accounts) == 0 {
		return fmt.Errorf("no schedulable accounts")
	}
	limit := len(accounts)
	if limit > smartGroupMaxProbeAccounts {
		limit = smartGroupMaxProbeAccounts
	}
	probeErrors := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		probeCtx, cancel := context.WithTimeout(ctx, smartGroupProbeTimeout)
		result, probeErr := s.accountTest.RunProbeTestBackground(probeCtx, accounts[i].ID, "")
		cancel()
		if probeErr == nil && result != nil && result.Status == "success" {
			return nil
		}
		message := "probe failed"
		if probeErr != nil {
			message = probeErr.Error()
		} else if result != nil && result.ErrorMessage != "" {
			message = result.ErrorMessage
		}
		probeErrors = append(probeErrors, fmt.Sprintf("account %d: %s", accounts[i].ID, message))
	}
	return fmt.Errorf("%s", strings.Join(probeErrors, ", "))
}

func (s *APIKeySmartGroupService) finishWithoutSwitch(ctx context.Context, state *APIKeySmartGroupRuntimeState, reason, message string, now time.Time) {
	expectedLeaseUntil := time.Time{}
	if state.ProbeLeaseUntil != nil {
		expectedLeaseUntil = *state.ProbeLeaseUntil
	}
	key, err := s.repo.CompleteSmartGroupProbeWithoutSwitch(ctx, state.APIKeyID, state.GroupID, reason, message, expectedLeaseUntil, now)
	if err != nil {
		logger.FromContext(ctx).Warn("api_key.smart_group_probe_finish_failed", zap.Int64("api_key_id", state.APIKeyID), zap.Error(err))
		return
	}
	if s.apiKeyService != nil && key != "" {
		s.apiKeyService.InvalidateAuthCacheByKey(ctx, key)
	}
}

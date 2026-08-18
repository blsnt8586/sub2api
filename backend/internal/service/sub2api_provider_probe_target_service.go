package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/ent"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/sub2api"
)

// Sub2APIProviderProbeTargetInput changes one independently scheduled business
// route. A nil field leaves its stored setting unchanged.
type Sub2APIProviderProbeTargetInput struct {
	Enabled           *bool   `json:"enabled"`
	IntervalSeconds   *int    `json:"interval_seconds"`
	TestModel         *string `json:"test_model"`
	AllowMediaProbe   *bool   `json:"allow_media_probe"`
	TimeoutSeconds    *int    `json:"timeout_seconds"`
	DegradedLatencyMS *int    `json:"degraded_latency_ms"`
	FailureThreshold  *int    `json:"failure_threshold"`
	RecoveryThreshold *int    `json:"recovery_threshold"`
}

type Sub2APIProviderProbeTargetCreateInput struct {
	ProviderID        int64
	AccountID         int64
	ProviderAPIKeyID  *int64
	RemoteGroupID     *int64
	RemoteGroupName   *string
	Platform          string
	Enabled           bool
	IntervalSeconds   int
	AllowMediaProbe   bool
	TimeoutSeconds    int
	DegradedLatencyMS int
	FailureThreshold  int
	RecoveryThreshold int
}

type Sub2APIProviderProbeTargetRunInput struct {
	TargetID            int64
	ProviderID          int64
	AccountID           int64
	ProviderAPIKeyID    *int64
	RemoteGroupID       *int64
	RemoteGroupName     *string
	Platform            string
	ModelID             *string
	Status              string
	LatencyMS           *int
	TrafficRequestCount int
	TrafficSuccessRate  *float64
	TrafficP95LatencyMS *int
	ErrorCategory       *string
	ErrorMessage        *string
	StartedAt           time.Time
	FinishedAt          time.Time
}

type Sub2APIProviderProbeTargetTrafficStats struct {
	RequestCount int
	SuccessRate  float64
	P95LatencyMS int
}

// Sub2APIProviderProbeTargetHealth is the public, route-level health model.
// Status is "disabled" when the target is off or a media probe is not allowed.
type Sub2APIProviderProbeTargetHealth struct {
	ID                     int64                         `json:"id"`
	ProviderID             int64                         `json:"provider_id"`
	AccountID              int64                         `json:"account_id"`
	AccountName            string                        `json:"account_name"`
	ProviderAPIKeyID       *int64                        `json:"provider_api_key_id,omitempty"`
	RemoteGroupID          *int64                        `json:"remote_group_id,omitempty"`
	RemoteGroupName        *string                       `json:"remote_group_name,omitempty"`
	RemoteGroupMultiplier  *float64                      `json:"remote_group_multiplier,omitempty"`
	Sub2APIOptimizeEnabled bool                          `json:"sub2api_optimize_enabled"`
	Sub2APIMinMultiplier   *float64                      `json:"sub2api_min_multiplier,omitempty"`
	Sub2APIMaxMultiplier   *float64                      `json:"sub2api_max_multiplier,omitempty"`
	Platform               string                        `json:"platform"`
	Enabled                bool                          `json:"enabled"`
	IntervalSeconds        int                           `json:"interval_seconds"`
	TestModel              *string                       `json:"test_model,omitempty"`
	AllowMediaProbe        bool                          `json:"allow_media_probe"`
	TimeoutSeconds         int                           `json:"timeout_seconds"`
	DegradedLatencyMS      int                           `json:"degraded_latency_ms"`
	FailureThreshold       int                           `json:"failure_threshold"`
	RecoveryThreshold      int                           `json:"recovery_threshold"`
	Status                 string                        `json:"status"`
	LatencyMS              *int                          `json:"latency_ms,omitempty"`
	TrafficRequestCount    int                           `json:"traffic_request_count"`
	TrafficSuccessRate     *float64                      `json:"traffic_success_rate,omitempty"`
	TrafficP95LatencyMS    *int                          `json:"traffic_p95_latency_ms,omitempty"`
	ErrorCategory          *string                       `json:"error_category,omitempty"`
	ErrorMessage           *string                       `json:"error_message,omitempty"`
	LastCheckedAt          *time.Time                    `json:"last_checked_at,omitempty"`
	LastRunAt              *time.Time                    `json:"last_run_at,omitempty"`
	RouteChangedAt         *time.Time                    `json:"route_changed_at,omitempty"`
	ConsecutiveFailures    int                           `json:"consecutive_failures"`
	Buckets                []Sub2APIProviderHealthBucket `json:"buckets"`
}

// ListTargets returns each independently configurable route. sync=true refreshes
// remote API key/group bindings before returning so manual reassignment upstream
// cannot be silently displayed as the previous route.
func (s *Sub2APIProviderProbeService) ListTargets(ctx context.Context, providerID int64, syncRemote bool) ([]*Sub2APIProviderProbeTargetHealth, error) {
	provider, err := s.providerRepo.GetByID(ctx, providerID)
	if err != nil {
		return nil, ErrProviderNotFound
	}
	targets, err := s.ensureTargets(ctx, providerID)
	if err != nil {
		return nil, err
	}
	if syncRemote && len(targets) > 0 {
		if refreshed, refreshErr := s.refreshTargetBindings(ctx, provider, targets); refreshErr == nil {
			targets = refreshed
		}
	}
	return s.targetHealths(ctx, targets, time.Now().UTC())
}

func (s *Sub2APIProviderProbeService) UpdateTarget(ctx context.Context, providerID, targetID int64, input *Sub2APIProviderProbeTargetInput) (*Sub2APIProviderProbeTargetHealth, error) {
	if err := validateProbeTargetInput(input); err != nil {
		return nil, err
	}
	target, err := s.probeRepo.GetTarget(ctx, providerID, targetID)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, infraerrors.NotFound("SUB2API_PROVIDER_PROBE_TARGET_NOT_FOUND", "probe target not found")
		}
		return nil, err
	}
	if err := validateEffectiveProbeTarget(target, input); err != nil {
		return nil, err
	}
	target, err = s.probeRepo.UpdateTarget(ctx, providerID, targetID, input)
	if err != nil {
		return nil, err
	}
	healths, err := s.targetHealths(ctx, []*ent.Sub2APIProviderProbeTarget{target}, time.Now().UTC())
	if err != nil || len(healths) == 0 {
		return nil, err
	}
	return healths[0], nil
}

// RunTargetNow probes one route without changing the Provider control-plane
// cadence. It shares the Provider lock with scheduled probing to prevent a
// manual click and a scheduler cycle from consuming the same route together.
func (s *Sub2APIProviderProbeService) RunTargetNow(ctx context.Context, providerID, targetID int64) (*Sub2APIProviderProbeTargetHealth, error) {
	key := fmt.Sprintf("sub2api:provider-probe:%d", providerID)
	lockCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	release, acquired := tryAcquireSingletonLeaderLock(lockCtx, s.lockCache, s.db, key, s.instanceID, 30*time.Minute)
	cancel()
	if !acquired {
		return nil, fmt.Errorf("provider probe already running")
	}
	defer release()

	provider, err := s.providerRepo.GetByID(ctx, providerID)
	if err != nil {
		return nil, ErrProviderNotFound
	}
	target, err := s.probeRepo.GetTarget(ctx, providerID, targetID)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, infraerrors.NotFound("SUB2API_PROVIDER_PROBE_TARGET_NOT_FOUND", "probe target not found")
		}
		return nil, err
	}
	if refreshed, refreshErr := s.refreshTargetBindings(ctx, provider, []*ent.Sub2APIProviderProbeTarget{target}); refreshErr == nil && len(refreshed) == 1 {
		target = refreshed[0]
	}
	if _, err := s.runTarget(ctx, target); err != nil {
		return nil, err
	}
	healths, err := s.targetHealths(ctx, []*ent.Sub2APIProviderProbeTarget{target}, time.Now().UTC())
	if err != nil || len(healths) == 0 {
		return nil, err
	}
	return healths[0], nil
}

func (s *Sub2APIProviderProbeService) TargetHistory(ctx context.Context, providerID, targetID int64, since time.Time, limit int) ([]*Sub2APIProviderProbeTargetHealth, error) {
	target, err := s.probeRepo.GetTarget(ctx, providerID, targetID)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, infraerrors.NotFound("SUB2API_PROVIDER_PROBE_TARGET_NOT_FOUND", "probe target not found")
		}
		return nil, err
	}
	if limit < 1 || limit > 2000 {
		limit = 100
	}
	runs, err := s.probeRepo.ListTargetRunsSince(ctx, []int64{targetID}, since.UTC())
	if err != nil {
		return nil, err
	}
	if len(runs) > limit {
		runs = runs[:limit]
	}
	out := make([]*Sub2APIProviderProbeTargetHealth, 0, len(runs))
	for _, run := range runs {
		out = append(out, targetHealthFromRun(run, target.DegradedLatencyMs))
	}
	return out, nil
}

func (s *Sub2APIProviderProbeService) ensureTargets(ctx context.Context, providerID int64) ([]*ent.Sub2APIProviderProbeTarget, error) {
	cfg, err := s.ensureConfig(ctx, providerID)
	if err != nil {
		return nil, err
	}
	accounts, err := s.accountRepo.ListByProviderID(ctx, providerID)
	if err != nil {
		return nil, fmt.Errorf("list linked accounts for probe targets: %w", err)
	}
	targets, err := s.probeRepo.ListTargets(ctx, providerID)
	if err != nil {
		return nil, err
	}
	existing := make(map[int64]struct{}, len(targets))
	for _, target := range targets {
		existing[target.AccountID] = struct{}{}
	}
	for _, account := range accounts {
		if _, exists := existing[account.ID]; exists {
			continue
		}
		// New account routes own their business-probe cadence and quality
		// thresholds and start enabled. Existing target rows are never changed
		// here, so an operator's explicit disabled state remains authoritative.
		// The legacy Provider data-plane switch and selected-account list describe
		// the retired shared probe and must not disable newly linked accounts.
		input := defaultProbeTargetCreateInput(providerID, cfg, account)
		if _, createErr := s.probeRepo.CreateTarget(ctx, input); createErr != nil {
			if !ent.IsConstraintError(createErr) {
				return nil, fmt.Errorf("create probe target for account %d: %w", account.ID, createErr)
			}
			// A concurrent overview/list request may have inserted the same
			// provider-account target. Only suppress that verified duplicate;
			// check/range/FK constraint failures must remain visible.
			refreshed, listErr := s.probeRepo.ListTargets(ctx, providerID)
			if listErr != nil {
				return nil, fmt.Errorf("verify probe target for account %d after constraint error: %w", account.ID, listErr)
			}
			if !hasProbeTargetForAccount(refreshed, account.ID) {
				return nil, fmt.Errorf("create probe target for account %d: %w", account.ID, createErr)
			}
		}
	}
	return s.probeRepo.ListTargets(ctx, providerID)
}

func defaultProbeTargetCreateInput(providerID int64, cfg *ent.Sub2APIProviderProbeConfig, account Account) *Sub2APIProviderProbeTargetCreateInput {
	return &Sub2APIProviderProbeTargetCreateInput{
		ProviderID:        providerID,
		AccountID:         account.ID,
		ProviderAPIKeyID:  account.ProviderAPIKeyID,
		RemoteGroupID:     account.RemoteGroupID,
		RemoteGroupName:   account.RemoteGroupName,
		Platform:          strings.TrimSpace(account.Platform),
		Enabled:           true,
		IntervalSeconds:   defaultProbeTargetIntervalSeconds,
		AllowMediaProbe:   cfg.AllowMediaProbe,
		TimeoutSeconds:    defaultProbeTimeoutSeconds,
		DegradedLatencyMS: defaultProbeDegradedLatencyMS,
		FailureThreshold:  cfg.FailureThreshold,
		RecoveryThreshold: cfg.RecoveryThreshold,
	}
}

func hasProbeTargetForAccount(targets []*ent.Sub2APIProviderProbeTarget, accountID int64) bool {
	for _, target := range targets {
		if target != nil && target.AccountID == accountID {
			return true
		}
	}
	return false
}

func (s *Sub2APIProviderProbeService) refreshTargetBindings(ctx context.Context, provider *ent.Sub2APIProvider, targets []*ent.Sub2APIProviderProbeTarget) ([]*ent.Sub2APIProviderProbeTarget, error) {
	if provider == nil || len(targets) == 0 {
		return targets, nil
	}
	client, err := newAuthedSub2APIProviderClient(ctx, provider, s.providerRepo, s.tokenCache, s.encryptor)
	if err != nil {
		return nil, err
	}
	keysPath := "/api/v1/keys"
	if provider.APIPathKeys != nil && *provider.APIPathKeys != "" {
		keysPath = *provider.APIPathKeys
	}
	keys, err := client.GetAPIKeys(ctx, keysPath)
	if err != nil {
		return nil, err
	}
	byID := make(map[int64]sub2api.APIKey, len(keys))
	for _, key := range keys {
		byID[key.ID] = key
	}
	updated := make([]*ent.Sub2APIProviderProbeTarget, 0, len(targets))
	for _, target := range targets {
		if target.ProviderAPIKeyID == nil {
			updated = append(updated, target)
			continue
		}
		key, ok := byID[*target.ProviderAPIKeyID]
		if !ok {
			updated = append(updated, target)
			continue
		}
		groupID := key.GroupID
		var groupName *string
		if key.Group != nil {
			groupID = key.Group.ID
			name := key.Group.Name
			groupName = &name
		}
		var remoteGroupID *int64
		if groupID > 0 {
			remoteGroupID = &groupID
		}
		refreshed, err := s.probeRepo.UpdateTargetBinding(ctx, target.ID, target.ProviderAPIKeyID, remoteGroupID, groupName, target.Platform)
		if err != nil {
			return nil, err
		}
		updated = append(updated, refreshed)
	}
	return updated, nil
}

// SyncProbeTargetBindings applies optimizer-owned account bindings to the
// corresponding monitoring targets immediately. This keeps the route name,
// group ID, traffic window, and next probe aligned without waiting for a UI
// refresh that performs a remote sync.
func (s *Sub2APIProviderProbeService) SyncProbeTargetBindings(ctx context.Context, providerID int64, accountIDs []int64) error {
	if len(accountIDs) == 0 {
		return nil
	}
	accounts, err := s.accountRepo.ListByProviderID(ctx, providerID)
	if err != nil {
		return fmt.Errorf("list accounts for probe binding sync: %w", err)
	}
	wanted := make(map[int64]struct{}, len(accountIDs))
	for _, accountID := range accountIDs {
		wanted[accountID] = struct{}{}
	}
	accountsByID := make(map[int64]Account, len(accountIDs))
	for _, account := range accounts {
		if _, ok := wanted[account.ID]; ok {
			accountsByID[account.ID] = account
		}
	}
	targets, err := s.probeRepo.ListTargets(ctx, providerID)
	if err != nil {
		return fmt.Errorf("list targets for optimized binding sync: %w", err)
	}
	for _, target := range targets {
		account, ok := accountsByID[target.AccountID]
		if !ok {
			continue
		}
		if _, err := s.probeRepo.UpdateTargetBinding(
			ctx,
			target.ID,
			account.ProviderAPIKeyID,
			account.RemoteGroupID,
			account.RemoteGroupName,
			account.Platform,
		); err != nil {
			return fmt.Errorf("sync optimized probe target %d: %w", target.ID, err)
		}
	}
	return nil
}

func (s *Sub2APIProviderProbeService) runTarget(ctx context.Context, target *ent.Sub2APIProviderProbeTarget) (*ent.Sub2APIProviderProbeTargetRun, error) {
	if target == nil {
		return nil, fmt.Errorf("probe target is required")
	}
	started := time.Now()
	account, err := s.accountRepo.GetByID(ctx, target.AccountID)
	if err != nil {
		return s.writeTargetFailure(ctx, target, started, "unhealthy", fmt.Errorf("load account %d: %w", target.AccountID, err))
	}
	if account.ProviderID == nil || *account.ProviderID != target.ProviderID {
		return s.writeTargetFailure(ctx, target, started, "unhealthy", fmt.Errorf("account is no longer linked to provider"))
	}
	modelID := ""
	if configuredModel := configuredAccountTestModel(account); configuredModel != nil {
		modelID = *configuredModel
	}
	if (account.IsCanvas() || isProviderProbeMediaModel(modelID)) && !target.AllowMediaProbe {
		now := time.Now()
		if err := s.probeRepo.MarkTargetRun(ctx, target.ID, now); err != nil {
			return nil, err
		}
		return nil, nil
	}

	probeCtx, cancel := context.WithTimeout(ctx, time.Duration(target.TimeoutSeconds)*time.Second)
	res, testErr := s.accountTest.RunTestBackground(probeCtx, target.AccountID, modelID)
	cancel()
	finished := time.Now()
	status := "healthy"
	var category, message *string
	if testErr != nil || res == nil || res.Status != "success" {
		status = "unhealthy"
		probeErr := testErr
		if probeErr == nil && res != nil && res.ErrorMessage != "" {
			probeErr = fmt.Errorf("%s", res.ErrorMessage)
		}
		if probeErr == nil {
			probeErr = fmt.Errorf("account probe failed")
		}
		cat := classifyProbeError(probeErr)
		msg := trimProbeError(probeErr)
		category, message = &cat, &msg
	}
	var latency *int
	if res != nil {
		value := int(res.LatencyMs)
		latency = &value
		if !res.FinishedAt.IsZero() {
			finished = res.FinishedAt
		}
	}
	status = targetProbeStatus(status, latency, target.DegradedLatencyMs)
	trafficSince := finished.Add(-time.Hour)
	if target.RouteChangedAt != nil && target.RouteChangedAt.After(trafficSince) {
		trafficSince = *target.RouteChangedAt
	}
	traffic, trafficErr := s.probeRepo.TargetTrafficStats(ctx, target.AccountID, trafficSince)
	input := &Sub2APIProviderProbeTargetRunInput{
		TargetID:         target.ID,
		ProviderID:       target.ProviderID,
		AccountID:        target.AccountID,
		ProviderAPIKeyID: target.ProviderAPIKeyID,
		RemoteGroupID:    target.RemoteGroupID,
		RemoteGroupName:  target.RemoteGroupName,
		Platform:         target.Platform,
		Status:           status,
		LatencyMS:        latency,
		ErrorCategory:    category,
		ErrorMessage:     message,
		StartedAt:        started,
		FinishedAt:       finished,
	}
	if modelID != "" {
		input.ModelID = &modelID
	}
	if trafficErr == nil && traffic != nil {
		input.TrafficRequestCount = traffic.RequestCount
		if traffic.RequestCount > 0 {
			rate := traffic.SuccessRate
			p95 := traffic.P95LatencyMS
			input.TrafficSuccessRate, input.TrafficP95LatencyMS = &rate, &p95
			if input.Status == "healthy" && (rate < 95 || p95 > target.DegradedLatencyMs) {
				input.Status = "degraded"
			}
		}
	}
	s.applyTargetHysteresis(ctx, target, input)
	run, err := s.probeRepo.CreateTargetRun(ctx, input)
	if err != nil {
		return nil, err
	}
	if err := s.probeRepo.MarkTargetRun(ctx, target.ID, finished); err != nil {
		return nil, err
	}
	return run, nil
}

func (s *Sub2APIProviderProbeService) writeTargetFailure(ctx context.Context, target *ent.Sub2APIProviderProbeTarget, started time.Time, status string, err error) (*ent.Sub2APIProviderProbeTargetRun, error) {
	if status == "" {
		status = "unhealthy"
	}
	category := classifyProbeError(err)
	message := trimProbeError(err)
	finished := time.Now()
	input := &Sub2APIProviderProbeTargetRunInput{
		TargetID: target.ID, ProviderID: target.ProviderID, AccountID: target.AccountID,
		ProviderAPIKeyID: target.ProviderAPIKeyID, RemoteGroupID: target.RemoteGroupID, RemoteGroupName: target.RemoteGroupName,
		Platform: target.Platform, Status: status, ErrorCategory: &category, ErrorMessage: &message,
		StartedAt: started, FinishedAt: finished,
	}
	s.applyTargetHysteresis(ctx, target, input)
	run, writeErr := s.probeRepo.CreateTargetRun(ctx, input)
	if writeErr != nil {
		return nil, writeErr
	}
	if markErr := s.probeRepo.MarkTargetRun(ctx, target.ID, finished); markErr != nil {
		return nil, markErr
	}
	return run, nil
}

// applyTargetHysteresis prevents one failed route request from immediately
// declaring an otherwise stable path down, and likewise avoids a false
// recovery after a single successful retry.
func (s *Sub2APIProviderProbeService) applyTargetHysteresis(ctx context.Context, target *ent.Sub2APIProviderProbeTarget, input *Sub2APIProviderProbeTargetRunInput) {
	if target == nil || input == nil || (input.Status != "unhealthy" && input.Status != "healthy") {
		return
	}
	runs, err := s.probeRepo.ListTargetRunsSince(ctx, []int64{target.ID}, time.Now().UTC().Add(-24*time.Hour))
	if err != nil {
		return
	}
	if input.Status == "unhealthy" {
		failures := 1
		for _, run := range runs {
			if run == nil || (string(run.Status) != "unhealthy" && string(run.Status) != "degraded") {
				break
			}
			failures++
		}
		if failures < target.FailureThreshold {
			input.Status = "degraded"
		}
		return
	}
	if len(runs) == 0 {
		return
	}
	recoveries := 1
	sawUnhealthy := false
	for _, run := range runs {
		if run == nil {
			continue
		}
		if string(run.Status) == "unhealthy" {
			sawUnhealthy = true
			break
		}
		recoveries++
	}
	if sawUnhealthy && recoveries < target.RecoveryThreshold {
		input.Status = "degraded"
	}
}

func (s *Sub2APIProviderProbeService) targetHealths(ctx context.Context, targets []*ent.Sub2APIProviderProbeTarget, now time.Time) ([]*Sub2APIProviderProbeTargetHealth, error) {
	if len(targets) == 0 {
		return []*Sub2APIProviderProbeTargetHealth{}, nil
	}
	targetIDs := make([]int64, 0, len(targets))
	accountIDs := make([]int64, 0, len(targets))
	for _, target := range targets {
		targetIDs = append(targetIDs, target.ID)
		accountIDs = append(accountIDs, target.AccountID)
	}
	accounts, err := s.accountRepo.GetByIDs(ctx, accountIDs)
	if err != nil {
		return nil, err
	}
	accountsByID := make(map[int64]*Account, len(accounts))
	for _, account := range accounts {
		if account != nil {
			accountsByID[account.ID] = account
		}
	}
	// A route timeline is a sequence of real probe events, not fixed time slots.
	// The repository applies the per-target limit so fast probe cadences do not
	// load the complete retention window into memory.
	runs, err := s.probeRepo.ListRecentTargetRuns(ctx, targetIDs, now.Add(-upstreamLogRetentionPeriod), targetHealthTimelineResultLimit)
	if err != nil {
		return nil, err
	}
	runsByTarget := make(map[int64][]*ent.Sub2APIProviderProbeTargetRun, len(targets))
	for _, run := range runs {
		runsByTarget[run.TargetID] = append(runsByTarget[run.TargetID], run)
	}
	out := make([]*Sub2APIProviderProbeTargetHealth, 0, len(targets))
	for _, target := range targets {
		out = append(out, targetHealthFromTarget(target, accountsByID[target.AccountID], runsByTarget[target.ID], now))
	}
	return out, nil
}

func targetHealthFromTarget(target *ent.Sub2APIProviderProbeTarget, account *Account, runs []*ent.Sub2APIProviderProbeTargetRun, _ time.Time) *Sub2APIProviderProbeTargetHealth {
	health := &Sub2APIProviderProbeTargetHealth{
		ID: target.ID, ProviderID: target.ProviderID, AccountID: target.AccountID, ProviderAPIKeyID: target.ProviderAPIKeyID,
		RemoteGroupID: target.RemoteGroupID, RemoteGroupName: target.RemoteGroupName, Platform: target.Platform,
		Enabled: target.Enabled, IntervalSeconds: target.IntervalSeconds,
		AllowMediaProbe: target.AllowMediaProbe, TimeoutSeconds: target.TimeoutSeconds, DegradedLatencyMS: target.DegradedLatencyMs,
		FailureThreshold: target.FailureThreshold, RecoveryThreshold: target.RecoveryThreshold, LastRunAt: target.LastRunAt,
		RouteChangedAt: target.RouteChangedAt, Status: "unknown",
	}
	if account != nil {
		health.AccountName = account.Name
		health.RemoteGroupMultiplier = account.RemoteGroupMultiplier
		health.Sub2APIOptimizeEnabled = account.Sub2APIOptimizeEnabled
		health.Sub2APIMinMultiplier = account.Sub2APIMinMultiplier
		health.Sub2APIMaxMultiplier = account.Sub2APIMaxMultiplier
		if health.Platform == "" {
			health.Platform = account.Platform
		}
	}
	health.TestModel = configuredAccountTestModel(account)
	health.Buckets = buildTargetProbeSamples(runs, target.DegradedLatencyMs)
	if !target.Enabled {
		health.Status = "disabled"
		return health
	}
	if health.TestModel != nil && isProviderProbeMediaModel(*health.TestModel) && !target.AllowMediaProbe {
		health.Status = "disabled"
		return health
	}
	if account != nil && account.IsCanvas() && !target.AllowMediaProbe {
		health.Status = "disabled"
		return health
	}
	latest := firstTargetProbeRun(runs)
	if latest == nil {
		return health
	}
	health.Status = targetProbeStatusFromRun(latest, target.DegradedLatencyMs)
	health.LatencyMS = latest.LatencyMs
	health.TrafficRequestCount = latest.TrafficRequestCount
	health.TrafficSuccessRate = latest.TrafficSuccessRate
	health.TrafficP95LatencyMS = latest.TrafficP95LatencyMs
	health.ErrorCategory = latest.ErrorCategory
	health.ErrorMessage = latest.ErrorMessage
	health.LastCheckedAt = &latest.FinishedAt
	for _, run := range runs {
		if run == nil {
			continue
		}
		if string(run.Status) != "unhealthy" {
			break
		}
		health.ConsecutiveFailures++
	}
	return health
}

func targetHealthFromRun(run *ent.Sub2APIProviderProbeTargetRun, degradedLatencyMS int) *Sub2APIProviderProbeTargetHealth {
	if run == nil {
		return &Sub2APIProviderProbeTargetHealth{Status: "unknown"}
	}
	return &Sub2APIProviderProbeTargetHealth{
		ID: run.TargetID, ProviderID: run.ProviderID, AccountID: run.AccountID, ProviderAPIKeyID: run.ProviderAPIKeyID,
		RemoteGroupID: run.RemoteGroupID, RemoteGroupName: run.RemoteGroupName, Platform: run.Platform,
		TestModel: run.ModelID, Status: targetProbeStatusFromRun(run, degradedLatencyMS), LatencyMS: run.LatencyMs,
		TrafficRequestCount: run.TrafficRequestCount, TrafficSuccessRate: run.TrafficSuccessRate,
		TrafficP95LatencyMS: run.TrafficP95LatencyMs, ErrorCategory: run.ErrorCategory, ErrorMessage: run.ErrorMessage,
		LastCheckedAt: &run.FinishedAt,
	}
}

func firstTargetProbeRun(runs []*ent.Sub2APIProviderProbeTargetRun) *ent.Sub2APIProviderProbeTargetRun {
	for _, run := range runs {
		if run != nil {
			return run
		}
	}
	return nil
}

// buildTargetProbeSamples converts newest-first repository rows into the
// oldest-to-newest event sequence rendered by the account timeline. Every
// returned point is backed by exactly one persisted probe run.
func buildTargetProbeSamples(runs []*ent.Sub2APIProviderProbeTargetRun, degradedLatencyMS int) []Sub2APIProviderHealthBucket {
	newestFirst := make([]*ent.Sub2APIProviderProbeTargetRun, 0, min(len(runs), targetHealthTimelineResultLimit))
	for _, run := range runs {
		if run == nil {
			continue
		}
		newestFirst = append(newestFirst, run)
		if len(newestFirst) == targetHealthTimelineResultLimit {
			break
		}
	}

	samples := make([]Sub2APIProviderHealthBucket, 0, len(newestFirst))
	for index := len(newestFirst) - 1; index >= 0; index-- {
		run := newestFirst[index]
		status := targetProbeStatusFromRun(run, degradedLatencyMS)
		sample := Sub2APIProviderHealthBucket{
			StartedAt:          run.StartedAt,
			EndedAt:            run.FinishedAt,
			Status:             status,
			SampleCount:        1,
			MaxHealthLatencyMS: run.LatencyMs,
			LastError:          run.ErrorMessage,
		}
		switch status {
		case "healthy":
			sample.HealthySamples = 1
		case "degraded":
			sample.DegradedSamples = 1
		case "unhealthy":
			sample.UnhealthySamples = 1
		}
		samples = append(samples, sample)
	}
	return samples
}

// configuredAccountTestModel is the single source of truth for account probes.
// Sub2APIProviderProbeTarget.TestModel is retained only for database backward
// compatibility; legacy values must never override current account settings.
func configuredAccountTestModel(account *Account) *string {
	if account != nil && account.Sub2APITestModel != nil {
		if model := strings.TrimSpace(*account.Sub2APITestModel); model != "" {
			return &model
		}
	}
	return nil
}

// targetProbeStatusWithTraffic derives display status from the current
// threshold and the recorded evidence. The persisted status is retained for
// real failures, but healthy/degraded latency classifications are recalculated
// so changing a threshold also updates historical timeline points.
func targetProbeStatusWithTraffic(status string, latencyMS *int, degradedLatencyMS int, trafficRequestCount int, trafficSuccessRate *float64, trafficP95LatencyMS *int) string {
	if status == "unhealthy" {
		return status
	}
	trafficDegraded := trafficRequestCount > 0 && ((trafficSuccessRate != nil && *trafficSuccessRate < 95) || (trafficP95LatencyMS != nil && degradedLatencyMS > 0 && *trafficP95LatencyMS > degradedLatencyMS))
	if trafficDegraded {
		return "degraded"
	}
	if latencyMS != nil {
		if degradedLatencyMS > 0 && *latencyMS > degradedLatencyMS {
			return "degraded"
		}
		if status == "healthy" || status == "degraded" {
			return "healthy"
		}
	}
	return status
}

func targetProbeStatusFromRun(run *ent.Sub2APIProviderProbeTargetRun, degradedLatencyMS int) string {
	if run == nil {
		return "unknown"
	}
	status := targetProbeStatusWithTraffic(string(run.Status), run.LatencyMs, degradedLatencyMS, run.TrafficRequestCount, run.TrafficSuccessRate, run.TrafficP95LatencyMs)
	// Hysteresis intentionally records a transient failed request as degraded.
	// Keep that evidence visible even when the failed request happened to report
	// a partial latency value below the current slow-response threshold.
	if status == "healthy" && string(run.Status) == "degraded" && (run.ErrorCategory != nil || run.ErrorMessage != nil) {
		return "degraded"
	}
	return status
}

// targetProbeStatus is kept for callers that only have basic probe evidence.
func targetProbeStatus(status string, latencyMS *int, degradedLatencyMS int) string {
	return targetProbeStatusWithTraffic(status, latencyMS, degradedLatencyMS, 0, nil, nil)
}

func probeStatusSeverity(status string) int {
	switch status {
	case "unhealthy":
		return 3
	case "degraded":
		return 2
	case "healthy":
		return 1
	default:
		return 0
	}
}

func validateProbeTargetInput(input *Sub2APIProviderProbeTargetInput) error {
	if input == nil {
		return invalidProviderProbeConfig("probe target config is required")
	}
	if input.TestModel != nil {
		return invalidProviderProbeConfig("test_model is managed by the linked account configuration")
	}
	if input.IntervalSeconds != nil && (*input.IntervalSeconds < 30 || *input.IntervalSeconds > 86400) {
		return invalidProviderProbeConfig("interval_seconds must be between 30 and 86400")
	}
	if input.TimeoutSeconds != nil && (*input.TimeoutSeconds < 3 || *input.TimeoutSeconds > 120) {
		return invalidProviderProbeConfig("timeout_seconds must be between 3 and 120")
	}
	if input.DegradedLatencyMS != nil && (*input.DegradedLatencyMS < 100 || *input.DegradedLatencyMS > 120000) {
		return invalidProviderProbeConfig("degraded_latency_ms must be between 100 and 120000")
	}
	if input.FailureThreshold != nil && (*input.FailureThreshold < 1 || *input.FailureThreshold > 20) {
		return invalidProviderProbeConfig("failure_threshold must be between 1 and 20")
	}
	if input.RecoveryThreshold != nil && (*input.RecoveryThreshold < 1 || *input.RecoveryThreshold > 20) {
		return invalidProviderProbeConfig("recovery_threshold must be between 1 and 20")
	}
	return nil
}

func validateEffectiveProbeTarget(target *ent.Sub2APIProviderProbeTarget, input *Sub2APIProviderProbeTargetInput) error {
	allowMedia := target.AllowMediaProbe
	if input.AllowMediaProbe != nil {
		allowMedia = *input.AllowMediaProbe
	}
	interval := target.IntervalSeconds
	if input.IntervalSeconds != nil {
		interval = *input.IntervalSeconds
	}
	if allowMedia && interval < minMediaProbeIntervalSeconds {
		return invalidProviderProbeConfig(fmt.Sprintf("interval_seconds must be at least %d when media probes are allowed", minMediaProbeIntervalSeconds))
	}
	return nil
}

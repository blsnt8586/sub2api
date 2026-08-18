package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/ent"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/sub2api"
	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
)

const (
	defaultProbeControlIntervalSeconds = 1800
	defaultProbeDataIntervalSeconds    = 1800
	defaultProbeTargetIntervalSeconds  = 30
	defaultProbeTimeoutSeconds         = 60
	defaultProbeDegradedLatencyMS      = 5000
	defaultProbeFailureThreshold       = 3
	defaultProbeRecoveryThreshold      = 2
	minMediaProbeIntervalSeconds       = 6 * 60 * 60
	upstreamLogRetentionPeriod         = 3 * 24 * time.Hour
	upstreamLogRetentionCheckInterval  = time.Hour
	upstreamLogRetentionBatchSize      = 1000
)

const upstreamLogRetentionLeaderLockKey = "sub2api:upstream-log-retention"

// Sub2APIUpstreamLogCleanupResult reports one bounded physical-delete batch.
// Provider/account configuration and current health snapshots are not part of
// this retention policy.
type Sub2APIUpstreamLogCleanupResult struct {
	ProviderProbeRuns int64
	AccountProbeRuns  int64
	OptimizeLogs      int64
}

func (r Sub2APIUpstreamLogCleanupResult) Total() int64 {
	return r.ProviderProbeRuns + r.AccountProbeRuns + r.OptimizeLogs
}

func (r *Sub2APIUpstreamLogCleanupResult) Add(other Sub2APIUpstreamLogCleanupResult) {
	r.ProviderProbeRuns += other.ProviderProbeRuns
	r.AccountProbeRuns += other.AccountProbeRuns
	r.OptimizeLogs += other.OptimizeLogs
}

// Sub2APIUpstreamLogRetentionRepository is kept narrow so retention batching
// can be tested independently from the full probe repository.
type Sub2APIUpstreamLogRetentionRepository interface {
	DeleteUpstreamLogsBefore(context.Context, time.Time, int) (Sub2APIUpstreamLogCleanupResult, error)
}

// Sub2APIProviderProbeRepository is deliberately separate from the Provider
// repository: probe history is append-only inside its enforced retention window.
type Sub2APIProviderProbeRepository interface {
	Sub2APIUpstreamLogRetentionRepository
	GetConfig(context.Context, int64) (*ent.Sub2APIProviderProbeConfig, error)
	CreateDefaultConfig(context.Context, int64) (*ent.Sub2APIProviderProbeConfig, error)
	UpdateConfig(context.Context, int64, *Sub2APIProviderProbeConfigInput) (*ent.Sub2APIProviderProbeConfig, error)
	MarkRun(context.Context, int64, bool, time.Time) error
	CreateRun(context.Context, *Sub2APIProviderProbeRunInput) (*ent.Sub2APIProviderProbeRun, error)
	LatestRun(context.Context, int64) (*ent.Sub2APIProviderProbeRun, error)
	ListRuns(context.Context, int64, int) ([]*ent.Sub2APIProviderProbeRun, error)
	ListRunsSince(context.Context, []int64, time.Time) ([]*ent.Sub2APIProviderProbeRun, error)
	TrafficStats(context.Context, int64, time.Time) (*Sub2APIProviderTrafficStats, error)
	ListTargets(context.Context, int64) ([]*ent.Sub2APIProviderProbeTarget, error)
	GetTarget(context.Context, int64, int64) (*ent.Sub2APIProviderProbeTarget, error)
	CreateTarget(context.Context, *Sub2APIProviderProbeTargetCreateInput) (*ent.Sub2APIProviderProbeTarget, error)
	UpdateTarget(context.Context, int64, int64, *Sub2APIProviderProbeTargetInput) (*ent.Sub2APIProviderProbeTarget, error)
	UpdateTargetBinding(context.Context, int64, *int64, *int64, *string, string) (*ent.Sub2APIProviderProbeTarget, error)
	MarkTargetRun(context.Context, int64, time.Time) error
	CreateTargetRun(context.Context, *Sub2APIProviderProbeTargetRunInput) (*ent.Sub2APIProviderProbeTargetRun, error)
	ListTargetRunsSince(context.Context, []int64, time.Time) ([]*ent.Sub2APIProviderProbeTargetRun, error)
	ListRecentTargetRuns(context.Context, []int64, time.Time, int) ([]*ent.Sub2APIProviderProbeTargetRun, error)
	TargetTrafficStats(context.Context, int64, time.Time) (*Sub2APIProviderProbeTargetTrafficStats, error)
}

// Sub2APIProbeAutoOptimizeTrigger decouples route probing from the optimizer.
// The scheduled probe submits only routes whose persisted result is unhealthy;
// the optimizer remains the authority for participation, bounds, cooldowns,
// locking, candidate switching, and result logging.
type Sub2APIProbeAutoOptimizeTrigger interface {
	TriggerProbeAutoOptimize(context.Context, int64, []Sub2APIProbeAutoOptimizeInput) (int, error)
}

type Sub2APIProviderProbeConfigInput struct {
	ControlEnabled         *bool   `json:"control_enabled"`
	ControlIntervalSeconds *int    `json:"control_interval_seconds"`
	DataEnabled            *bool   `json:"data_enabled"`
	DataIntervalSeconds    *int    `json:"data_interval_seconds"`
	SelectedAccountIDs     []int64 `json:"selected_account_ids"`
	AllowMediaProbe        *bool   `json:"allow_media_probe"`
	TimeoutSeconds         *int    `json:"timeout_seconds"`
	DegradedLatencyMS      *int    `json:"degraded_latency_ms"`
	FailureThreshold       *int    `json:"failure_threshold"`
	RecoveryThreshold      *int    `json:"recovery_threshold"`
}

type Sub2APIProviderTrafficStats struct {
	RequestCount int
	SuccessRate  float64
	P95LatencyMS int
}

type Sub2APIProviderProbeRunInput struct {
	ProviderID          int64
	OverallStatus       string
	ControlStatus       string
	DataStatus          string
	TrafficStatus       string
	LoginLatencyMS      *int
	HealthLatencyMS     *int
	KeysLatencyMS       *int
	GroupsLatencyMS     *int
	DataProbeCount      int
	DataProbeSuccess    int
	DataProbeFailed     int
	TrafficRequestCount int
	TrafficSuccessRate  *float64
	TrafficP95LatencyMS *int
	ErrorCategory       *string
	ErrorMessage        *string
	Details             map[string]any
	StartedAt           time.Time
	FinishedAt          time.Time
}

type Sub2APIProviderAccountProbe struct {
	AccountID     int64      `json:"account_id"`
	AccountName   string     `json:"account_name,omitempty"`
	Platform      string     `json:"platform,omitempty"`
	Status        string     `json:"status"`
	LatencyMS     *int       `json:"latency_ms,omitempty"`
	ErrorCategory *string    `json:"error_category,omitempty"`
	ErrorMessage  *string    `json:"error_message,omitempty"`
	CheckedAt     *time.Time `json:"checked_at,omitempty"`
}

type Sub2APIProviderHealth struct {
	ProviderID          int64                         `json:"provider_id"`
	Status              string                        `json:"status"`
	ControlStatus       string                        `json:"control_status"`
	DataStatus          string                        `json:"data_status"`
	TrafficStatus       string                        `json:"traffic_status"`
	ConsecutiveFailures int                           `json:"consecutive_failures"`
	LoginLatencyMS      *int                          `json:"login_latency_ms,omitempty"`
	HealthLatencyMS     *int                          `json:"health_latency_ms,omitempty"`
	KeysLatencyMS       *int                          `json:"keys_latency_ms,omitempty"`
	GroupsLatencyMS     *int                          `json:"groups_latency_ms,omitempty"`
	DataProbeCount      int                           `json:"data_probe_count"`
	DataProbeSuccess    int                           `json:"data_probe_success"`
	DataProbeFailed     int                           `json:"data_probe_failed"`
	DataProbeEnabled    bool                          `json:"data_probe_enabled"`
	DataProbeInterval   int                           `json:"data_probe_interval_seconds"`
	ProbeAccountCount   int                           `json:"probe_account_count"`
	AccountProbes       []Sub2APIProviderAccountProbe `json:"account_probes"`
	TrafficRequestCount int                           `json:"traffic_request_count"`
	TrafficSuccessRate  *float64                      `json:"traffic_success_rate,omitempty"`
	TrafficP95LatencyMS *int                          `json:"traffic_p95_latency_ms,omitempty"`
	ErrorCategory       *string                       `json:"error_category,omitempty"`
	ErrorMessage        *string                       `json:"error_message,omitempty"`
	Details             map[string]any                `json:"details,omitempty"`
	LastCheckedAt       *time.Time                    `json:"last_checked_at,omitempty"`
}

type Sub2APIProviderHealthBucket struct {
	StartedAt          time.Time `json:"started_at"`
	EndedAt            time.Time `json:"ended_at"`
	Status             string    `json:"status"`
	SampleCount        int       `json:"sample_count"`
	HealthySamples     int       `json:"healthy_samples"`
	DegradedSamples    int       `json:"degraded_samples"`
	UnhealthySamples   int       `json:"unhealthy_samples"`
	MaxHealthLatencyMS *int      `json:"max_health_latency_ms,omitempty"`
	LastError          *string   `json:"last_error,omitempty"`
}

type Sub2APIProviderHealthSummary struct {
	Healthy   int `json:"healthy"`
	Degraded  int `json:"degraded"`
	Unhealthy int `json:"unhealthy"`
	Unknown   int `json:"unknown"`
}

type Sub2APIProviderHealthOverview struct {
	ProviderID         int64                               `json:"provider_id"`
	AvailabilityStatus string                              `json:"availability_status"`
	EvidenceStatus     string                              `json:"evidence_status"`
	Latest             *Sub2APIProviderHealth              `json:"latest,omitempty"`
	LatestControl      *Sub2APIProviderHealth              `json:"latest_control,omitempty"`
	WindowStartedAt    time.Time                           `json:"window_started_at"`
	WindowEndedAt      time.Time                           `json:"window_ended_at"`
	BucketSeconds      int                                 `json:"bucket_seconds"`
	Buckets            []Sub2APIProviderHealthBucket       `json:"buckets"`
	Summary            Sub2APIProviderHealthSummary        `json:"summary"`
	Routes             []*Sub2APIProviderProbeTargetHealth `json:"routes"`
}

type Sub2APIProviderProbeConfigView struct {
	ID                     int64      `json:"id"`
	ProviderID             int64      `json:"provider_id"`
	ControlEnabled         bool       `json:"control_enabled"`
	ControlIntervalSeconds int        `json:"control_interval_seconds"`
	DataEnabled            bool       `json:"data_enabled"`
	DataIntervalSeconds    int        `json:"data_interval_seconds"`
	SelectedAccountIDs     []int64    `json:"selected_account_ids"`
	AllowMediaProbe        bool       `json:"allow_media_probe"`
	TimeoutSeconds         int        `json:"timeout_seconds"`
	DegradedLatencyMS      int        `json:"degraded_latency_ms"`
	FailureThreshold       int        `json:"failure_threshold"`
	RecoveryThreshold      int        `json:"recovery_threshold"`
	LastControlRunAt       *time.Time `json:"last_control_run_at,omitempty"`
	LastDataRunAt          *time.Time `json:"last_data_run_at,omitempty"`
}

type Sub2APIProviderProbeService struct {
	providerRepo        Sub2APIProviderRepository
	probeRepo           Sub2APIProviderProbeRepository
	accountRepo         AccountRepository
	accountTest         *AccountTestService
	tokenCache          *sub2api.TokenCache
	encryptor           SecretEncryptor
	autoOptimizeTrigger Sub2APIProbeAutoOptimizeTrigger
	remoteOverviewCache Sub2APIProviderRemoteOverviewCache
	lockCache           LeaderLockCache
	db                  *sql.DB
	instanceID          string
	mu                  sync.Mutex
}

func NewSub2APIProviderProbeService(providerRepo Sub2APIProviderRepository, probeRepo Sub2APIProviderProbeRepository, accountRepo AccountRepository, accountTest *AccountTestService, tokenCache *sub2api.TokenCache, encryptor SecretEncryptor) *Sub2APIProviderProbeService {
	return &Sub2APIProviderProbeService{providerRepo: providerRepo, probeRepo: probeRepo, accountRepo: accountRepo, accountTest: accountTest, tokenCache: tokenCache, encryptor: encryptor, instanceID: uuid.NewString()}
}

func (s *Sub2APIProviderProbeService) SetLeaderLock(lockCache LeaderLockCache, db *sql.DB) {
	if s != nil {
		s.lockCache, s.db = lockCache, db
	}
}

func (s *Sub2APIProviderProbeService) SetAutoOptimizeTrigger(trigger Sub2APIProbeAutoOptimizeTrigger) {
	if s != nil {
		s.autoOptimizeTrigger = trigger
	}
}

func (s *Sub2APIProviderProbeService) SetRemoteOverviewCache(cache Sub2APIProviderRemoteOverviewCache) {
	if s != nil {
		s.remoteOverviewCache = cache
	}
}

func (s *Sub2APIProviderProbeService) GetConfig(ctx context.Context, providerID int64) (*Sub2APIProviderProbeConfigView, error) {
	cfg, err := s.ensureConfig(ctx, providerID)
	if err != nil {
		return nil, err
	}
	return probeConfigView(cfg), nil
}

func (s *Sub2APIProviderProbeService) UpdateConfig(ctx context.Context, providerID int64, input *Sub2APIProviderProbeConfigInput) (*Sub2APIProviderProbeConfigView, error) {
	if err := validateProbeConfig(input); err != nil {
		return nil, err
	}
	if _, err := s.providerRepo.GetByID(ctx, providerID); err != nil {
		return nil, ErrProviderNotFound
	}
	cfg, err := s.ensureConfig(ctx, providerID)
	if err != nil {
		return nil, err
	}
	if err := validateEffectiveProbeConfig(cfg, input); err != nil {
		return nil, err
	}

	selectedAccountIDs := cfg.SelectedAccountIds
	if input.SelectedAccountIDs != nil {
		selectedAccountIDs = input.SelectedAccountIDs
	}
	dataEnabled := cfg.DataEnabled
	if input.DataEnabled != nil {
		dataEnabled = *input.DataEnabled
	}
	selectedAccounts := make([]*Account, 0, len(selectedAccountIDs))
	for _, accountID := range selectedAccountIDs {
		account, err := s.accountRepo.GetByID(ctx, accountID)
		if err != nil {
			return nil, invalidProviderProbeConfig(fmt.Sprintf("account %d is not available", accountID))
		}
		selectedAccounts = append(selectedAccounts, account)
	}
	if dataEnabled || input.SelectedAccountIDs != nil {
		if err := validateRepresentativeProbeAccounts(providerID, selectedAccounts); err != nil {
			return nil, err
		}
	}
	cfg, err = s.probeRepo.UpdateConfig(ctx, providerID, input)
	if err != nil {
		return nil, fmt.Errorf("update probe config: %w", err)
	}
	return probeConfigView(cfg), nil
}

func (s *Sub2APIProviderProbeService) RunNow(ctx context.Context, providerID int64) (*Sub2APIProviderHealth, error) {
	// A Provider-level manual check is control-plane only. Business probes are
	// intentionally triggered through RunTargetNow so one click can never fan
	// out into billable model requests for every linked account.
	return s.run(ctx, providerID, false, false)
}

func (s *Sub2APIProviderProbeService) GetHealth(ctx context.Context, providerID int64) (*Sub2APIProviderHealth, error) {
	if _, err := s.providerRepo.GetByID(ctx, providerID); err != nil {
		return nil, ErrProviderNotFound
	}
	run, err := s.probeRepo.LatestRun(ctx, providerID)
	if ent.IsNotFound(err) {
		return &Sub2APIProviderHealth{ProviderID: providerID, Status: "unknown", ControlStatus: "unknown", DataStatus: "unknown", TrafficStatus: "unknown"}, nil
	}
	if err != nil {
		return nil, err
	}
	health := healthFromRun(run)
	if runs, listErr := s.probeRepo.ListRuns(ctx, providerID, 20); listErr == nil {
		health.ConsecutiveFailures = countConsecutiveProbeFailures(runs)
	}
	return health, nil
}

func (s *Sub2APIProviderProbeService) History(ctx context.Context, providerID int64, limit int) ([]*Sub2APIProviderHealth, error) {
	if _, err := s.providerRepo.GetByID(ctx, providerID); err != nil {
		return nil, ErrProviderNotFound
	}
	runs, err := s.probeRepo.ListRuns(ctx, providerID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]*Sub2APIProviderHealth, 0, len(runs))
	for _, run := range runs {
		out = append(out, healthFromRun(run))
	}
	return out, nil
}

func (s *Sub2APIProviderProbeService) HistorySince(ctx context.Context, providerID int64, since time.Time, limit int) ([]*Sub2APIProviderHealth, error) {
	if _, err := s.providerRepo.GetByID(ctx, providerID); err != nil {
		return nil, ErrProviderNotFound
	}
	if limit < 1 || limit > 2000 {
		limit = 100
	}
	runs, err := s.probeRepo.ListRunsSince(ctx, []int64{providerID}, since.UTC())
	if err != nil {
		return nil, err
	}
	if len(runs) > limit {
		runs = runs[:limit]
	}
	out := make([]*Sub2APIProviderHealth, 0, len(runs))
	for _, run := range runs {
		out = append(out, healthFromRun(run))
	}
	return out, nil
}

func (s *Sub2APIProviderProbeService) HealthOverview(ctx context.Context, providerIDs []int64) ([]*Sub2APIProviderHealthOverview, error) {
	if len(providerIDs) == 0 {
		return []*Sub2APIProviderHealthOverview{}, nil
	}
	now := time.Now().UTC()
	runs, err := s.probeRepo.ListRunsSince(ctx, providerIDs, now.Add(-24*time.Hour))
	if err != nil {
		return nil, err
	}
	overviews := buildProviderHealthOverviews(providerIDs, runs, now)
	byProviderID := make(map[int64]*Sub2APIProviderHealthOverview, len(overviews))
	for _, overview := range overviews {
		byProviderID[overview.ProviderID] = overview
	}
	for _, providerID := range providerIDs {
		targets, targetErr := s.ensureTargets(ctx, providerID)
		if targetErr != nil {
			return nil, targetErr
		}
		routes, routeErr := s.targetHealths(ctx, targets, now)
		if routeErr != nil {
			return nil, routeErr
		}
		if overview := byProviderID[providerID]; overview != nil {
			overview.Routes = routes
		}
	}
	return overviews, nil
}

const (
	providerHealthOverviewBucketCount   = 60
	providerHealthOverviewBucketSeconds = 24 * 60
	// Account route timelines are event sequences: one point per real probe.
	targetHealthTimelineResultLimit = 60
)

func buildProviderHealthOverviews(providerIDs []int64, runs []*ent.Sub2APIProviderProbeRun, now time.Time) []*Sub2APIProviderHealthOverview {
	windowEnd := now.UTC()
	bucketDuration := time.Duration(providerHealthOverviewBucketSeconds) * time.Second
	windowStart := windowEnd.Add(-time.Duration(providerHealthOverviewBucketCount) * bucketDuration)
	overviews := make([]*Sub2APIProviderHealthOverview, 0, len(providerIDs))
	byProvider := make(map[int64]*Sub2APIProviderHealthOverview, len(providerIDs))
	latestAt := make(map[int64]time.Time, len(providerIDs))
	latestControlAt := make(map[int64]time.Time, len(providerIDs))
	latestErrorAt := make(map[int64][]time.Time, len(providerIDs))

	for _, providerID := range providerIDs {
		buckets := make([]Sub2APIProviderHealthBucket, providerHealthOverviewBucketCount)
		for i := range buckets {
			buckets[i] = Sub2APIProviderHealthBucket{
				StartedAt: windowStart.Add(time.Duration(i) * bucketDuration),
				EndedAt:   windowStart.Add(time.Duration(i+1) * bucketDuration),
				Status:    "unknown",
			}
		}
		overview := &Sub2APIProviderHealthOverview{
			ProviderID:         providerID,
			AvailabilityStatus: "unknown",
			EvidenceStatus:     "unknown",
			WindowStartedAt:    windowStart,
			WindowEndedAt:      windowEnd,
			BucketSeconds:      providerHealthOverviewBucketSeconds,
			Buckets:            buckets,
		}
		overviews = append(overviews, overview)
		byProvider[providerID] = overview
		latestErrorAt[providerID] = make([]time.Time, providerHealthOverviewBucketCount)
	}

	for _, run := range runs {
		if run == nil {
			continue
		}
		overview, ok := byProvider[run.ProviderID]
		if !ok || run.CreatedAt.Before(windowStart) || run.CreatedAt.After(windowEnd) {
			continue
		}
		if at, exists := latestAt[run.ProviderID]; !exists || run.CreatedAt.After(at) {
			overview.Latest = healthFromRun(run)
			overview.EvidenceStatus = string(run.OverallStatus)
			latestAt[run.ProviderID] = run.CreatedAt
		}
		controlStatus := providerControlTimelineStatus(run)
		if controlStatus == "unknown" {
			continue
		}
		if at, exists := latestControlAt[run.ProviderID]; !exists || run.CreatedAt.After(at) {
			overview.LatestControl = healthFromRun(run)
			overview.AvailabilityStatus = controlStatus
			latestControlAt[run.ProviderID] = run.CreatedAt
		}

		bucketIndex := int(run.CreatedAt.Sub(windowStart) / bucketDuration)
		if bucketIndex == providerHealthOverviewBucketCount {
			bucketIndex--
		}
		if bucketIndex < 0 || bucketIndex >= len(overview.Buckets) {
			continue
		}
		bucket := &overview.Buckets[bucketIndex]
		bucket.SampleCount++
		switch controlStatus {
		case "healthy":
			bucket.HealthySamples++
		case "degraded":
			bucket.DegradedSamples++
		case "unhealthy":
			bucket.UnhealthySamples++
		}
		if healthStatusSeverity(controlStatus) > healthStatusSeverity(bucket.Status) {
			bucket.Status = controlStatus
		}
		if run.HealthLatencyMs != nil && (bucket.MaxHealthLatencyMS == nil || *run.HealthLatencyMs > *bucket.MaxHealthLatencyMS) {
			latency := *run.HealthLatencyMs
			bucket.MaxHealthLatencyMS = &latency
		}
		if run.ErrorMessage != nil && (latestErrorAt[run.ProviderID][bucketIndex].IsZero() || run.CreatedAt.After(latestErrorAt[run.ProviderID][bucketIndex])) {
			errorMessage := *run.ErrorMessage
			bucket.LastError = &errorMessage
			latestErrorAt[run.ProviderID][bucketIndex] = run.CreatedAt
		}
	}

	for _, overview := range overviews {
		for _, bucket := range overview.Buckets {
			switch bucket.Status {
			case "healthy":
				overview.Summary.Healthy++
			case "degraded":
				overview.Summary.Degraded++
			case "unhealthy":
				overview.Summary.Unhealthy++
			default:
				overview.Summary.Unknown++
			}
		}
	}
	return overviews
}

func providerControlTimelineStatus(run *ent.Sub2APIProviderProbeRun) string {
	if run == nil {
		return "unknown"
	}
	switch string(run.ControlStatus) {
	case "healthy":
		return "healthy"
	case "degraded":
		return "degraded"
	case "unhealthy":
		if string(run.OverallStatus) == "unhealthy" {
			return "unhealthy"
		}
		return "degraded"
	default:
		return "unknown"
	}
}

func healthStatusSeverity(status string) int {
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

func (s *Sub2APIProviderProbeService) ensureConfig(ctx context.Context, providerID int64) (*ent.Sub2APIProviderProbeConfig, error) {
	cfg, err := s.probeRepo.GetConfig(ctx, providerID)
	if err == nil {
		return cfg, nil
	}
	if !ent.IsNotFound(err) {
		return nil, err
	}
	cfg, err = s.probeRepo.CreateDefaultConfig(ctx, providerID)
	if ent.IsConstraintError(err) {
		return s.probeRepo.GetConfig(ctx, providerID)
	}
	return cfg, err
}

func (s *Sub2APIProviderProbeService) run(ctx context.Context, providerID int64, scheduled, includeTargets bool) (*Sub2APIProviderHealth, error) {
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
	cfg, err := s.ensureConfig(ctx, providerID)
	if err != nil {
		return nil, err
	}
	started := time.Now()
	result := &Sub2APIProviderProbeRunInput{ProviderID: providerID, ControlStatus: "unknown", DataStatus: "unknown", TrafficStatus: "unknown", OverallStatus: "unknown", Details: map[string]any{}, StartedAt: started}
	traffic, trafficErr := s.probeRepo.TrafficStats(ctx, providerID, started.Add(-time.Hour))
	if trafficErr == nil && traffic != nil {
		result.TrafficRequestCount = traffic.RequestCount
		if traffic.RequestCount > 0 {
			r := traffic.SuccessRate
			result.TrafficSuccessRate = &r
			p := traffic.P95LatencyMS
			result.TrafficP95LatencyMS = &p
			result.TrafficStatus = "healthy"
			if r < 95 || p > cfg.DegradedLatencyMs {
				result.TrafficStatus = "degraded"
			}
		}
	}

	runControl := !scheduled
	if scheduled {
		runControl = cfg.ControlEnabled && probeIntervalDue(cfg.LastControlRunAt, cfg.ControlIntervalSeconds)
	}
	if runControl {
		result.ControlStatus = s.runControl(ctx, provider, cfg, result)
		_ = s.probeRepo.MarkRun(ctx, providerID, false, time.Now())
	}

	// Data-plane probing is route-scoped. Manual Provider checks never enter this
	// loop; each manual business request must name exactly one target.
	targets, targetErr := s.ensureTargets(ctx, providerID)
	if targetErr != nil {
		return nil, targetErr
	}
	var targetRunErr error
	autoOptimizeInputs := make([]Sub2APIProbeAutoOptimizeInput, 0)
	for _, target := range targets {
		if !shouldRunProviderProbeTarget(target, scheduled, includeTargets, time.Now()) {
			continue
		}
		targetRun, err := s.runTarget(ctx, target)
		if err != nil {
			targetRunErr = errors.Join(targetRunErr, err)
			continue
		}
		if scheduled {
			if input, ok := probeAutoOptimizeInput(target, targetRun); ok {
				autoOptimizeInputs = append(autoOptimizeInputs, input)
			}
		}
	}
	if scheduled && s.autoOptimizeTrigger != nil && len(autoOptimizeInputs) > 0 {
		if _, err := s.autoOptimizeTrigger.TriggerProbeAutoOptimize(ctx, providerID, autoOptimizeInputs); err != nil {
			logger.LegacyPrintf("service.sub2api_provider_probe", "[Sub2APIProviderProbe] provider=%d auto optimize trigger failed: %v", providerID, err)
		}
	}
	if !runControl {
		if targetRunErr != nil {
			return nil, targetRunErr
		}
		return s.GetHealth(ctx, providerID)
	}
	result.OverallStatus = overallProbeStatus(result.ControlStatus, result.DataStatus, result.TrafficStatus)
	s.applyHysteresis(ctx, cfg, result)
	result.FinishedAt = time.Now()
	if _, err := s.probeRepo.CreateRun(ctx, result); err != nil {
		return nil, err
	}
	if targetRunErr != nil {
		return nil, targetRunErr
	}
	if health, err := s.GetHealth(ctx, providerID); err == nil {
		return health, nil
	}
	return healthFromInput(result), nil
}

func (s *Sub2APIProviderProbeService) inheritPreviousDataProbe(ctx context.Context, providerID int64, result *Sub2APIProviderProbeRunInput) {
	runs, err := s.probeRepo.ListRuns(ctx, providerID, 100)
	if err != nil {
		return
	}
	for _, run := range runs {
		if run == nil || string(run.DataStatus) == "unknown" {
			continue
		}
		result.DataStatus = string(run.DataStatus)
		result.DataProbeCount = run.DataProbeCount
		result.DataProbeSuccess = run.DataProbeSuccess
		result.DataProbeFailed = run.DataProbeFailed
		if run.Details != nil {
			if accountProbes, ok := run.Details["account_probes"]; ok {
				result.Details["account_probes"] = accountProbes
			}
		}
		return
	}
}

func (s *Sub2APIProviderProbeService) applyHysteresis(ctx context.Context, cfg *ent.Sub2APIProviderProbeConfig, result *Sub2APIProviderProbeRunInput) {
	candidate := result.OverallStatus
	result.Details["candidate_status"] = candidate
	limit := cfg.FailureThreshold
	if cfg.RecoveryThreshold > limit {
		limit = cfg.RecoveryThreshold
	}
	runs, err := s.probeRepo.ListRuns(ctx, result.ProviderID, limit)
	if err != nil || len(runs) == 0 {
		if candidate == "unhealthy" && cfg.FailureThreshold > 1 {
			result.OverallStatus = "degraded"
		}
		return
	}
	previous := string(runs[0].OverallStatus)
	if candidate == "unhealthy" {
		failures := 1
		for _, run := range runs {
			if probeCandidateStatus(run) != "unhealthy" {
				break
			}
			failures++
		}
		if previous != "unhealthy" && failures < cfg.FailureThreshold {
			result.OverallStatus = "degraded"
		}
		return
	}
	if previous == "unhealthy" {
		recoveries := 1
		for _, run := range runs {
			if probeCandidateStatus(run) == "unhealthy" {
				break
			}
			recoveries++
		}
		if recoveries < cfg.RecoveryThreshold {
			result.OverallStatus = "unhealthy"
		}
	}
}

func (s *Sub2APIProviderProbeService) runControl(ctx context.Context, provider *ent.Sub2APIProvider, cfg *ent.Sub2APIProviderProbeConfig, result *Sub2APIProviderProbeRunInput) string {
	stageTimeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	newStageContext := func() (context.Context, context.CancelFunc) {
		return context.WithTimeout(ctx, stageTimeout)
	}
	assetAttemptedAt := time.Now().UTC()
	recordAssetFailure := func(err error) {
		if err == nil || s.remoteOverviewCache == nil {
			return
		}
		storeRemoteOverviewFailure(ctx, s.remoteOverviewCache, provider.ID, Sub2APIProviderRemoteOverviewSourceControlProbe, assetAttemptedAt, err)
		result.Details["asset_snapshot_status"] = "failed"
		result.Details["asset_snapshot_error"] = trimProbeError(err)
	}

	loginStart := time.Now()
	loginCtx, cancel := newStageContext()
	client, err := newAuthedSub2APIProviderClient(loginCtx, provider, s.providerRepo, s.tokenCache, s.encryptor)
	cancel()
	if err != nil {
		recordAssetFailure(err)
		result.ErrorCategory, result.ErrorMessage = providerProbeStringPtr(classifyProbeError(err)), providerProbeStringPtr(trimProbeError(err))
		return "unhealthy"
	}
	client.HTTPClient.Timeout = time.Duration(cfg.TimeoutSeconds) * time.Second
	l := int(time.Since(loginStart).Milliseconds())
	result.LoginLatencyMS = &l
	healthStart := time.Now()
	healthCtx, cancel := newStageContext()
	healthErr := client.ProbeHealth(healthCtx, "/health")
	cancel()
	if healthErr == nil {
		v := int(time.Since(healthStart).Milliseconds())
		result.HealthLatencyMS = &v
	} else {
		result.Details["health_error"] = trimProbeError(healthErr)
	}
	keysPath := "/api/v1/keys"
	if provider.APIPathKeys != nil && *provider.APIPathKeys != "" {
		keysPath = *provider.APIPathKeys
	}
	keysStart := time.Now()
	keysCtx, cancel := newStageContext()
	keys, keysErr := client.GetAPIKeys(keysCtx, keysPath)
	cancel()
	k := int(time.Since(keysStart).Milliseconds())
	result.KeysLatencyMS = &k
	result.Details["key_count"] = len(keys)
	groupsPath := "/api/v1/groups/available"
	if provider.APIPathGroups != nil && *provider.APIPathGroups != "" {
		groupsPath = *provider.APIPathGroups
	}
	groupsStart := time.Now()
	groupsCtx, cancel := newStageContext()
	groups, groupsErr := client.GetGroups(groupsCtx, groupsPath)
	cancel()
	g := int(time.Since(groupsStart).Milliseconds())
	result.GroupsLatencyMS = &g
	result.Details["group_count"] = len(groups)
	if keysErr != nil {
		result.Details["keys_error"] = trimProbeError(keysErr)
	}
	if groupsErr != nil {
		result.Details["groups_error"] = trimProbeError(groupsErr)
	}
	// Asset collection follows the control-plane cadence and reuses the same
	// authenticated client and Groups response. It does not depend on the API
	// keys endpoint. Failures remain best-effort and never affect availability.
	if groupsErr == nil && s.remoteOverviewCache != nil {
		assetCtx, assetCancel := newStageContext()
		overview, assetErr := collectSub2APIProviderRemoteOverview(
			assetCtx,
			provider.ID,
			client,
			groups,
			Sub2APIProviderRemoteOverviewSourceControlProbe,
			assetAttemptedAt,
		)
		assetCancel()
		if assetErr != nil {
			recordAssetFailure(assetErr)
		} else {
			storeRemoteOverviewSuccess(ctx, s.remoteOverviewCache, overview)
			result.Details["asset_snapshot_status"] = "updated"
			result.Details["asset_group_count"] = len(overview.Groups)
		}
	} else if groupsErr != nil {
		recordAssetFailure(groupsErr)
	}
	availabilityStatus, probeErr := controlProbeAvailabilityStatus(healthErr, keysErr, groupsErr)
	if availabilityStatus != "healthy" {
		result.ErrorCategory, result.ErrorMessage = providerProbeStringPtr(classifyProbeError(probeErr)), providerProbeStringPtr(trimProbeError(probeErr))
		return availabilityStatus
	}
	return "healthy"
}

func (s *Sub2APIProviderProbeService) runData(ctx context.Context, cfg *ent.Sub2APIProviderProbeConfig, result *Sub2APIProviderProbeRunInput) string {
	accountProbes := make([]Sub2APIProviderAccountProbe, 0, len(cfg.SelectedAccountIds))
	defer func() {
		result.Details["account_probes"] = accountProbes
	}()
	if len(cfg.SelectedAccountIds) == 0 {
		return "unknown"
	}
	for _, accountID := range cfg.SelectedAccountIds {
		checkedAt := time.Now()
		accountProbe := Sub2APIProviderAccountProbe{AccountID: accountID, Status: "unknown", CheckedAt: &checkedAt}
		account, err := s.accountRepo.GetByID(ctx, accountID)
		if err != nil {
			result.DataProbeCount++
			result.DataProbeFailed++
			probeErr := fmt.Errorf("load account %d: %w", accountID, err)
			accountProbe.Status = "unhealthy"
			accountProbe.ErrorCategory = providerProbeStringPtr(classifyProbeError(probeErr))
			accountProbe.ErrorMessage = providerProbeStringPtr(trimProbeError(probeErr))
			accountProbes = append(accountProbes, accountProbe)
			continue
		}
		accountProbe.AccountName = account.Name
		accountProbe.Platform = account.Platform
		modelID := ""
		if account.Sub2APITestModel != nil {
			modelID = strings.TrimSpace(*account.Sub2APITestModel)
		}
		if (account.IsCanvas() || isProviderProbeMediaModel(modelID)) && !cfg.AllowMediaProbe {
			accountProbe.Status = "disabled"
			accountProbes = append(accountProbes, accountProbe)
			continue
		}
		result.DataProbeCount++
		probeCtx, cancel := context.WithTimeout(ctx, time.Duration(cfg.TimeoutSeconds)*time.Second)
		res, err := s.accountTest.RunTestBackground(probeCtx, accountID, modelID)
		cancel()
		if err == nil && res != nil && res.Status == "success" {
			result.DataProbeSuccess++
			accountProbe.Status = "healthy"
		} else {
			result.DataProbeFailed++
			accountProbe.Status = "unhealthy"
			probeErr := err
			if probeErr == nil && res != nil && res.ErrorMessage != "" {
				probeErr = fmt.Errorf("%s", res.ErrorMessage)
			}
			if probeErr == nil {
				probeErr = fmt.Errorf("account probe failed")
			}
			accountProbe.ErrorCategory = providerProbeStringPtr(classifyProbeError(probeErr))
			accountProbe.ErrorMessage = providerProbeStringPtr(trimProbeError(probeErr))
		}
		if res != nil {
			latency := int(res.LatencyMs)
			accountProbe.LatencyMS = &latency
			if !res.FinishedAt.IsZero() {
				accountProbe.CheckedAt = providerProbeTimePtr(res.FinishedAt)
			}
		}
		accountProbes = append(accountProbes, accountProbe)
	}
	if result.DataProbeCount == 0 {
		return "unknown"
	}
	if result.DataProbeFailed > 0 && result.DataProbeSuccess == 0 {
		return "unhealthy"
	}
	if result.DataProbeFailed > 0 {
		return "degraded"
	}
	return "healthy"
}

func overallProbeStatus(control, data, traffic string) string {
	if control == "unhealthy" {
		return "unhealthy"
	}
	if control == "degraded" || data == "degraded" || traffic == "degraded" {
		return "degraded"
	}
	if control == "unknown" {
		return "unknown"
	}
	if data == "healthy" || traffic == "healthy" {
		return "healthy"
	}
	return "unknown"
}

func controlProbeAvailabilityStatus(healthErr, keysErr, groupsErr error) (string, error) {
	probeErr := keysErr
	if probeErr == nil {
		probeErr = groupsErr
	}
	if probeErr == nil {
		probeErr = healthErr
	}
	if keysErr != nil && groupsErr != nil {
		return "unhealthy", probeErr
	}
	if probeErr != nil {
		return "degraded", probeErr
	}
	return "healthy", nil
}

func healthFromInput(in *Sub2APIProviderProbeRunInput) *Sub2APIProviderHealth {
	health := &Sub2APIProviderHealth{ProviderID: in.ProviderID, Status: in.OverallStatus, ControlStatus: in.ControlStatus, DataStatus: in.DataStatus, TrafficStatus: in.TrafficStatus, LoginLatencyMS: in.LoginLatencyMS, HealthLatencyMS: in.HealthLatencyMS, KeysLatencyMS: in.KeysLatencyMS, GroupsLatencyMS: in.GroupsLatencyMS, DataProbeCount: in.DataProbeCount, DataProbeSuccess: in.DataProbeSuccess, DataProbeFailed: in.DataProbeFailed, TrafficRequestCount: in.TrafficRequestCount, TrafficSuccessRate: in.TrafficSuccessRate, TrafficP95LatencyMS: in.TrafficP95LatencyMS, ErrorCategory: in.ErrorCategory, ErrorMessage: in.ErrorMessage, Details: in.Details, LastCheckedAt: providerProbeTimePtr(in.FinishedAt)}
	applyProviderProbeDetails(health)
	return health
}

func healthFromRun(run *ent.Sub2APIProviderProbeRun) *Sub2APIProviderHealth {
	var failures int
	if run.OverallStatus == "unhealthy" {
		failures = 1
	}
	health := &Sub2APIProviderHealth{ProviderID: run.ProviderID, Status: string(run.OverallStatus), ControlStatus: string(run.ControlStatus), DataStatus: string(run.DataStatus), TrafficStatus: string(run.TrafficStatus), ConsecutiveFailures: failures, LoginLatencyMS: run.LoginLatencyMs, HealthLatencyMS: run.HealthLatencyMs, KeysLatencyMS: run.KeysLatencyMs, GroupsLatencyMS: run.GroupsLatencyMs, DataProbeCount: run.DataProbeCount, DataProbeSuccess: run.DataProbeSuccess, DataProbeFailed: run.DataProbeFailed, TrafficRequestCount: run.TrafficRequestCount, TrafficSuccessRate: run.TrafficSuccessRate, TrafficP95LatencyMS: run.TrafficP95LatencyMs, ErrorCategory: run.ErrorCategory, ErrorMessage: run.ErrorMessage, Details: run.Details, LastCheckedAt: providerProbeTimePtr(run.FinishedAt)}
	applyProviderProbeDetails(health)
	return health
}

func applyProviderProbeDetails(health *Sub2APIProviderHealth) {
	if health == nil {
		return
	}
	health.AccountProbes = []Sub2APIProviderAccountProbe{}
	if health.Details == nil {
		health.DataProbeEnabled = health.DataStatus != "unknown" || health.DataProbeCount > 0
		health.DataProbeInterval = defaultProbeDataIntervalSeconds
		health.ProbeAccountCount = health.DataProbeCount
		return
	}
	if raw, exists := health.Details["data_probe_enabled"]; exists {
		_ = decodeProviderProbeDetail(raw, &health.DataProbeEnabled)
	} else {
		health.DataProbeEnabled = health.DataStatus != "unknown" || health.DataProbeCount > 0
	}
	if raw, exists := health.Details["data_probe_interval_seconds"]; exists {
		_ = decodeProviderProbeDetail(raw, &health.DataProbeInterval)
	}
	if health.DataProbeInterval == 0 {
		health.DataProbeInterval = defaultProbeDataIntervalSeconds
	}
	if raw, exists := health.Details["probe_account_count"]; exists {
		_ = decodeProviderProbeDetail(raw, &health.ProbeAccountCount)
	}
	_ = decodeProviderProbeDetail(health.Details["account_probes"], &health.AccountProbes)
	if health.AccountProbes == nil {
		health.AccountProbes = []Sub2APIProviderAccountProbe{}
	}
	if health.ProbeAccountCount == 0 && len(health.AccountProbes) > 0 {
		health.ProbeAccountCount = len(health.AccountProbes)
	}
}

func decodeProviderProbeDetail(raw any, target any) error {
	if raw == nil {
		return nil
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	return json.Unmarshal(encoded, target)
}

func probeCandidateStatus(run *ent.Sub2APIProviderProbeRun) string {
	if run != nil && run.Details != nil {
		if status, ok := run.Details["candidate_status"].(string); ok && status != "" {
			return status
		}
	}
	if run == nil {
		return "unknown"
	}
	return string(run.OverallStatus)
}

func countConsecutiveProbeFailures(runs []*ent.Sub2APIProviderProbeRun) int {
	count := 0
	for _, run := range runs {
		if probeCandidateStatus(run) != "unhealthy" {
			break
		}
		count++
	}
	return count
}

func probeConfigView(c *ent.Sub2APIProviderProbeConfig) *Sub2APIProviderProbeConfigView {
	return &Sub2APIProviderProbeConfigView{ID: c.ID, ProviderID: c.ProviderID, ControlEnabled: c.ControlEnabled, ControlIntervalSeconds: c.ControlIntervalSeconds, DataEnabled: c.DataEnabled, DataIntervalSeconds: c.DataIntervalSeconds, SelectedAccountIDs: c.SelectedAccountIds, AllowMediaProbe: c.AllowMediaProbe, TimeoutSeconds: c.TimeoutSeconds, DegradedLatencyMS: c.DegradedLatencyMs, FailureThreshold: c.FailureThreshold, RecoveryThreshold: c.RecoveryThreshold, LastControlRunAt: c.LastControlRunAt, LastDataRunAt: c.LastDataRunAt}
}

func validateProbeConfig(in *Sub2APIProviderProbeConfigInput) error {
	if in == nil {
		return invalidProviderProbeConfig("probe config is required")
	}
	if in.ControlIntervalSeconds != nil && (*in.ControlIntervalSeconds < 60 || *in.ControlIntervalSeconds > 86400) {
		return invalidProviderProbeConfig("control_interval_seconds must be between 60 and 86400")
	}
	if in.DataIntervalSeconds != nil && (*in.DataIntervalSeconds < 300 || *in.DataIntervalSeconds > 86400) {
		return invalidProviderProbeConfig("data_interval_seconds must be between 300 and 86400")
	}
	if in.TimeoutSeconds != nil && (*in.TimeoutSeconds < 3 || *in.TimeoutSeconds > 120) {
		return invalidProviderProbeConfig("timeout_seconds must be between 3 and 120")
	}
	if in.DegradedLatencyMS != nil && (*in.DegradedLatencyMS < 100 || *in.DegradedLatencyMS > 120000) {
		return invalidProviderProbeConfig("degraded_latency_ms must be between 100 and 120000")
	}
	if in.FailureThreshold != nil && (*in.FailureThreshold < 1 || *in.FailureThreshold > 20) {
		return invalidProviderProbeConfig("failure_threshold must be between 1 and 20")
	}
	if in.RecoveryThreshold != nil && (*in.RecoveryThreshold < 1 || *in.RecoveryThreshold > 20) {
		return invalidProviderProbeConfig("recovery_threshold must be between 1 and 20")
	}
	return nil
}

func validateEffectiveProbeConfig(current *ent.Sub2APIProviderProbeConfig, in *Sub2APIProviderProbeConfigInput) error {
	allowMedia := current.AllowMediaProbe
	if in.AllowMediaProbe != nil {
		allowMedia = *in.AllowMediaProbe
	}
	dataIntervalSeconds := current.DataIntervalSeconds
	if in.DataIntervalSeconds != nil {
		dataIntervalSeconds = *in.DataIntervalSeconds
	}
	if allowMedia && dataIntervalSeconds < minMediaProbeIntervalSeconds {
		return invalidProviderProbeConfig(fmt.Sprintf("data_interval_seconds must be at least %d when media probes are allowed", minMediaProbeIntervalSeconds))
	}
	return nil
}

func validateRepresentativeProbeAccounts(providerID int64, accounts []*Account) error {
	seenIDs := make(map[int64]struct{}, len(accounts))
	for _, account := range accounts {
		if account == nil || account.ProviderID == nil || *account.ProviderID != providerID {
			accountID := int64(0)
			if account != nil {
				accountID = account.ID
			}
			return invalidProviderProbeConfig(fmt.Sprintf("account %d is not linked to provider %d", accountID, providerID))
		}
		if _, exists := seenIDs[account.ID]; exists {
			return invalidProviderProbeConfig(fmt.Sprintf("account %d is selected more than once", account.ID))
		}
		seenIDs[account.ID] = struct{}{}
	}
	return nil
}

func invalidProviderProbeConfig(message string) error {
	return infraerrors.BadRequest("SUB2API_PROVIDER_PROBE_CONFIG_INVALID", message)
}

func classifyProbeError(err error) string {
	if err == nil {
		return ""
	}
	var cloudflareChallenge *sub2api.ErrCloudflareChallenge
	if errors.As(err, &cloudflareChallenge) {
		return "cloudflare_challenge"
	}
	var cloudflareAccess *sub2api.ErrCloudflareAccessDenied
	if errors.As(err, &cloudflareAccess) {
		return "cloudflare_access_denied"
	}
	var interactionRequired *sub2api.ErrAuthInteractionRequired
	if errors.As(err, &interactionRequired) {
		return "auth_interaction_required"
	}
	e := strings.ToLower(err.Error())
	if category, ok := classifyCloudflareErrorMessage(e); ok {
		return category
	}
	switch {
	case strings.Contains(e, "turnstile"), strings.Contains(e, "captcha"), strings.Contains(e, "验证码"):
		return "captcha_required"
	case strings.Contains(e, "401"), strings.Contains(e, "unauthorized"):
		return "auth"
	case strings.Contains(e, "429"):
		return "rate_limit"
	case strings.Contains(e, "timeout"), strings.Contains(e, "deadline"):
		return "timeout"
	case strings.Contains(e, "tls"), strings.Contains(e, "certificate"):
		return "tls"
	case strings.Contains(e, "http error: status=5"):
		return "upstream_5xx"
	case strings.Contains(e, "http request"):
		return "network"
	default:
		return "protocol"
	}
}
func trimProbeError(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.TrimSpace(err.Error())
	if summary, ok := sanitizeCloudflareErrorMessage(msg); ok {
		return summary
	}
	if len(msg) > 1000 {
		return msg[:1000]
	}
	return msg
}

var cloudflareRayIDPattern = regexp.MustCompile(`(?i)cloudflare\s+ray\s+id:\s*(?:<[^>]*>\s*)*([a-z0-9-]+)`)

// classifyCloudflareErrorMessage recognizes raw Cloudflare HTML returned by
// business endpoints. Control-plane clients already return typed errors, but
// account tests historically returned the response body as a plain string.
func classifyCloudflareErrorMessage(message string) (string, bool) {
	lower := strings.ToLower(strings.TrimSpace(message))
	if lower == "" {
		return "", false
	}
	blocked := strings.Contains(lower, "sorry, you have been blocked") ||
		strings.Contains(lower, "you are unable to access") ||
		strings.Contains(lower, "attention required! | cloudflare")
	challenge := strings.Contains(lower, "challenge-platform") ||
		strings.Contains(lower, "cf-chl-") ||
		strings.Contains(lower, "turnstile") ||
		strings.Contains(lower, "just a moment")
	hasCloudflareMarker := strings.Contains(lower, "cloudflare") || strings.Contains(lower, "cf-ray")
	if !hasCloudflareMarker {
		return "", false
	}
	if !strings.Contains(lower, "403") && !blocked && !challenge {
		return "", false
	}
	if blocked {
		return "cloudflare_access_denied", true
	}
	return "cloudflare_challenge", true
}

func sanitizeCloudflareErrorMessage(message string) (string, bool) {
	category, ok := classifyCloudflareErrorMessage(message)
	if !ok {
		return "", false
	}
	lower := strings.ToLower(message)
	status := ""
	for _, code := range []string{"403", "429", "503", "502", "500"} {
		if strings.Contains(lower, code) {
			status = code
			break
		}
	}
	if status == "" {
		status = "unknown"
	}
	label := "access denied"
	if category == "cloudflare_challenge" {
		label = "challenge required"
	}
	summary := fmt.Sprintf("Cloudflare %s (HTTP %s)", label, status)
	if match := cloudflareRayIDPattern.FindStringSubmatch(message); len(match) > 1 {
		summary += fmt.Sprintf(" (CF-Ray: %s)", match[1])
	}
	return summary, true
}

func isProviderProbeMediaModel(model string) bool {
	m := strings.ToLower(strings.TrimSpace(model))
	if m == "" {
		return false
	}
	for _, marker := range []string{"image", "video", "audio", "voice", "tts", "stt", "realtime", "sora", "veo", "seedance", "kling"} {
		if strings.Contains(m, marker) {
			return true
		}
	}
	return false
}

func probeIntervalDue(last *time.Time, intervalSeconds int) bool {
	return probeIntervalDueAt(last, intervalSeconds, time.Now())
}

func probeIntervalDueAt(last *time.Time, intervalSeconds int, now time.Time) bool {
	if last == nil {
		return true
	}
	return !now.Before(last.Add(time.Duration(intervalSeconds) * time.Second))
}

// probeIntervalDueWithJitterAt keeps the configured interval as the minimum
// cadence, then adds a stable per-run delay of up to 20%. The delay is derived
// from the target and its last run timestamp, so the 15-second runner scan
// cannot re-roll it on every pass or drift between its pre-check and run.
func probeIntervalDueWithJitterAt(last *time.Time, intervalSeconds int, targetID int64, now time.Time) bool {
	if last == nil {
		return true
	}
	if intervalSeconds <= 0 {
		return true
	}
	jitterSeconds := probeIntervalJitterSeconds(last, intervalSeconds, targetID)
	if jitterSeconds <= 0 {
		return probeIntervalDueAt(last, intervalSeconds, now)
	}
	dueAt := last.Add(time.Duration(intervalSeconds+int(jitterSeconds)) * time.Second)
	return !now.Before(dueAt)
}

func probeIntervalJitterSeconds(last *time.Time, intervalSeconds int, targetID int64) int64 {
	if last == nil || intervalSeconds <= 0 {
		return 0
	}
	maxJitter := intervalSeconds / 5
	if maxJitter <= 0 {
		return 0
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(fmt.Sprintf("%d:%d", targetID, last.UnixNano())))
	return int64(h.Sum32()) % int64(maxJitter+1)
}

func shouldRunProviderProbeTarget(target *ent.Sub2APIProviderProbeTarget, scheduled, includeTargets bool, now time.Time) bool {
	if !includeTargets || target == nil || !target.Enabled {
		return false
	}
	return !scheduled || probeIntervalDueWithJitterAt(target.LastRunAt, target.IntervalSeconds, target.ID, now)
}

func scheduledProbeDataIntervalSeconds(cfg *ent.Sub2APIProviderProbeConfig) int {
	if cfg.AllowMediaProbe && cfg.DataIntervalSeconds < minMediaProbeIntervalSeconds {
		return minMediaProbeIntervalSeconds
	}
	return cfg.DataIntervalSeconds
}

func providerProbeStringPtr(v string) *string     { return &v }
func providerProbeTimePtr(v time.Time) *time.Time { return &v }

// Sub2APIProviderProbeRunner scans frequently; each persisted config controls
// due decision is persisted in each config, so restarts do not reset cadence.
type Sub2APIProviderProbeRunner struct {
	service            *Sub2APIProviderProbeService
	cron               *cron.Cron
	lockCache          LeaderLockCache
	db                 *sql.DB
	instanceID         string
	once               sync.Once
	retentionMu        sync.Mutex
	nextRetentionRunAt time.Time
}

func NewSub2APIProviderProbeRunner(service *Sub2APIProviderProbeService, lockCache LeaderLockCache, db *sql.DB) *Sub2APIProviderProbeRunner {
	return &Sub2APIProviderProbeRunner{service: service, lockCache: lockCache, db: db, instanceID: uuid.NewString()}
}
func (r *Sub2APIProviderProbeRunner) Start() {
	if r == nil || r.service == nil {
		return
	}
	r.once.Do(func() { r.cron = cron.New(); _, _ = r.cron.AddFunc("@every 15s", r.runDue); r.cron.Start() })
}
func (r *Sub2APIProviderProbeRunner) Stop() {
	if r != nil && r.cron != nil {
		ctx := r.cron.Stop()
		<-ctx.Done()
	}
}
func (r *Sub2APIProviderProbeRunner) runDue() {
	r.runLogRetentionIfDue(time.Now().UTC())

	lockCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	release, ok := tryAcquireSingletonLeaderLock(lockCtx, r.lockCache, r.db, "sub2api:provider-probe:runner", r.instanceID, 30*time.Minute)
	cancel()
	if !ok {
		return
	}
	defer release()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	providers, err := r.service.providerRepo.ListAll(ctx, &Sub2APIProviderFilters{Status: "active"})
	if err != nil {
		return
	}
	var wg sync.WaitGroup
	workers := make(chan struct{}, 3)
	for _, p := range providers {
		cfg, err := r.service.ensureConfig(ctx, p.ID)
		if err != nil {
			continue
		}
		controlDue := cfg.ControlEnabled && probeIntervalDue(cfg.LastControlRunAt, cfg.ControlIntervalSeconds)
		targets, targetErr := r.service.ensureTargets(ctx, p.ID)
		if targetErr != nil {
			continue
		}
		targetDue := false
		for _, target := range targets {
			if target.Enabled && probeIntervalDueWithJitterAt(target.LastRunAt, target.IntervalSeconds, target.ID, time.Now()) {
				targetDue = true
				break
			}
		}
		if controlDue || targetDue {
			wg.Add(1)
			workers <- struct{}{}
			go func(id int64) {
				defer wg.Done()
				defer func() { <-workers }()
				_, _ = r.service.run(context.Background(), id, true, true)
			}(p.ID)
		}
	}
	wg.Wait()
}

func (r *Sub2APIProviderProbeRunner) runLogRetentionIfDue(now time.Time) {
	if r == nil || r.service == nil || r.service.probeRepo == nil {
		return
	}
	r.retentionMu.Lock()
	if !r.nextRetentionRunAt.IsZero() && now.Before(r.nextRetentionRunAt) {
		r.retentionMu.Unlock()
		return
	}
	r.nextRetentionRunAt = now.Add(upstreamLogRetentionCheckInterval)
	r.retentionMu.Unlock()

	lockCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	release, ok := tryAcquireSingletonLeaderLock(
		lockCtx,
		r.lockCache,
		r.db,
		upstreamLogRetentionLeaderLockKey,
		r.instanceID,
		10*time.Minute,
	)
	cancel()
	if !ok {
		return
	}
	defer release()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cutoff := now.UTC().Add(-upstreamLogRetentionPeriod)
	deleted, err := deleteExpiredSub2APIUpstreamLogs(
		ctx,
		r.service.probeRepo,
		cutoff,
		upstreamLogRetentionBatchSize,
	)
	if err != nil {
		logger.LegacyPrintf("service.sub2api_provider_probe", "[Sub2APIUpstreamLogRetention] cleanup failed: %v", err)
		return
	}
	if deleted.Total() > 0 {
		logger.LegacyPrintf(
			"service.sub2api_provider_probe",
			"[Sub2APIUpstreamLogRetention] deleted provider_probe_runs=%d account_probe_runs=%d optimize_logs=%d cutoff=%s",
			deleted.ProviderProbeRuns,
			deleted.AccountProbeRuns,
			deleted.OptimizeLogs,
			cutoff.Format(time.RFC3339),
		)
	}
}

func deleteExpiredSub2APIUpstreamLogs(
	ctx context.Context,
	repo Sub2APIUpstreamLogRetentionRepository,
	cutoff time.Time,
	batchSize int,
) (Sub2APIUpstreamLogCleanupResult, error) {
	var total Sub2APIUpstreamLogCleanupResult
	if repo == nil {
		return total, errors.New("nil upstream log retention repository")
	}
	if batchSize <= 0 {
		return total, errors.New("upstream log retention batch size must be positive")
	}
	for {
		deleted, err := repo.DeleteUpstreamLogsBefore(ctx, cutoff.UTC(), batchSize)
		total.Add(deleted)
		if err != nil {
			return total, err
		}
		if deleted.ProviderProbeRuns < int64(batchSize) &&
			deleted.AccountProbeRuns < int64(batchSize) &&
			deleted.OptimizeLogs < int64(batchSize) {
			return total, nil
		}
	}
}

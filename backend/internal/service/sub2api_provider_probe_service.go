package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/domain"
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
	providerCredentialRefreshLead      = 15 * time.Minute
	providerCredentialRefreshTimeout   = 90 * time.Second
	providerCredentialRefreshRetry     = 5 * time.Minute
)

const upstreamLogRetentionLeaderLockKey = "sub2api:upstream-log-retention"
const providerAccountDailyCostSnapshotLockKey = "sub2api:provider-account-daily-cost-snapshot"

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

type Sub2APIAnswerProbeResponseRetentionRepository interface {
	ClearAnswerProbeResponsesBefore(context.Context, time.Time, int) (int64, error)
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
	DeleteTargetsByIDs(context.Context, int64, []int64) (int, error)
	GetTarget(context.Context, int64, int64) (*ent.Sub2APIProviderProbeTarget, error)
	CreateTarget(context.Context, *Sub2APIProviderProbeTargetCreateInput) (*ent.Sub2APIProviderProbeTarget, error)
	UpdateTarget(context.Context, int64, int64, *Sub2APIProviderProbeTargetInput) (*ent.Sub2APIProviderProbeTarget, error)
	UpdateTargetBinding(context.Context, int64, *int64, *int64, *string, string) (*ent.Sub2APIProviderProbeTarget, error)
	MarkTargetRun(context.Context, int64, time.Time, string) error
	MarkTargetsCostOptimize(context.Context, []int64, time.Time) error
	CreateTargetRun(context.Context, *Sub2APIProviderProbeTargetRunInput) (*ent.Sub2APIProviderProbeTargetRun, error)
	ListTargetRunsSince(context.Context, []int64, time.Time) ([]*ent.Sub2APIProviderProbeTargetRun, error)
	ListRecentTargetRuns(context.Context, []int64, time.Time, int) ([]*ent.Sub2APIProviderProbeTargetRun, error)
	ListRecentAnswerProbeRuns(context.Context, int64, time.Time, int) ([]*ent.Sub2APIProviderProbeTargetRun, error)
	TargetTrafficStats(context.Context, int64, time.Time) (*Sub2APIProviderProbeTargetTrafficStats, error)
}

// Sub2APIProbeAutoOptimizeTrigger decouples route probing from the optimizer.
// Account target probes persist each failed request as unhealthy, so a
// scheduled probe submits the failure immediately; the optimizer remains the
// authority for participation, bounds, cooldowns, locking, candidate
// switching, and result logging.
type Sub2APIProbeAutoOptimizeTrigger interface {
	TriggerProbeAutoOptimize(context.Context, int64, []Sub2APIProbeAutoOptimizeInput) (int, error)
}

// Sub2APIProbeAutoOptimizeAdmission is the subset of probe inputs that the
// optimizer actually claimed. A trigger may accept only part of a batch when
// a target cooldown, provider lock, concurrency check, or account validation
// filters the rest. The probe must quarantine every input that was not claimed
// instead of assuming the returned count applies to all of them.
type Sub2APIProbeAutoOptimizeAdmission struct {
	TargetIDs  []int64
	AccountIDs []int64
}

// Sub2APIProbeAutoOptimizeAdmissionTrigger is implemented by the production
// optimizer. The legacy trigger interface remains supported for narrow test
// doubles and older callers.
type Sub2APIProbeAutoOptimizeAdmissionTrigger interface {
	TriggerProbeAutoOptimizeWithAdmission(context.Context, int64, []Sub2APIProbeAutoOptimizeInput) (Sub2APIProbeAutoOptimizeAdmission, error)
}

type Sub2APIProviderProbeConfigInput struct {
	ControlEnabled                 *bool   `json:"control_enabled"`
	ControlIntervalSeconds         *int    `json:"control_interval_seconds"`
	DataEnabled                    *bool   `json:"data_enabled"`
	DataIntervalSeconds            *int    `json:"data_interval_seconds"`
	SelectedAccountIDs             []int64 `json:"selected_account_ids"`
	AllowMediaProbe                *bool   `json:"allow_media_probe"`
	TimeoutSeconds                 *int    `json:"timeout_seconds"`
	DegradedLatencyMS              *int    `json:"degraded_latency_ms"`
	FailureThreshold               *int    `json:"failure_threshold"`
	RecoveryThreshold              *int    `json:"recovery_threshold"`
	AccountStatusSyncEnabled       *bool   `json:"account_status_sync_enabled"`
	AccountStatusFailureThreshold  *int    `json:"account_status_failure_threshold"`
	AccountStatusRecoveryThreshold *int    `json:"account_status_recovery_threshold"`
}

type Sub2APIProviderTrafficStats struct {
	RequestCount int
	FailureCount int
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
	ProviderID           int64                         `json:"provider_id"`
	Status               string                        `json:"status"`
	ControlStatus        string                        `json:"control_status"`
	DataStatus           string                        `json:"data_status"`
	TrafficStatus        string                        `json:"traffic_status"`
	ConsecutiveFailures  int                           `json:"consecutive_failures"`
	LoginLatencyMS       *int                          `json:"login_latency_ms,omitempty"`
	HealthLatencyMS      *int                          `json:"health_latency_ms,omitempty"`
	KeysLatencyMS        *int                          `json:"keys_latency_ms,omitempty"`
	GroupsLatencyMS      *int                          `json:"groups_latency_ms,omitempty"`
	DataProbeCount       int                           `json:"data_probe_count"`
	DataProbeSuccess     int                           `json:"data_probe_success"`
	DataProbeFailed      int                           `json:"data_probe_failed"`
	DataProbeEnabled     bool                          `json:"data_probe_enabled"`
	DataProbeInterval    int                           `json:"data_probe_interval_seconds"`
	ProbeAccountCount    int                           `json:"probe_account_count"`
	AccountProbes        []Sub2APIProviderAccountProbe `json:"account_probes"`
	TrafficRequestCount  int                           `json:"traffic_request_count"`
	TrafficFailureCount  int                           `json:"traffic_failure_count"`
	TrafficSuccessRate   *float64                      `json:"traffic_success_rate,omitempty"`
	TrafficP95LatencyMS  *int                          `json:"traffic_p95_latency_ms,omitempty"`
	PassiveHealthStatus  string                        `json:"passive_health_status"`
	PassiveCooldownUntil *time.Time                    `json:"passive_cooldown_until,omitempty"`
	ErrorCategory        *string                       `json:"error_category,omitempty"`
	ErrorMessage         *string                       `json:"error_message,omitempty"`
	Details              map[string]any                `json:"details,omitempty"`
	LastCheckedAt        *time.Time                    `json:"last_checked_at,omitempty"`
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
	ID                             int64      `json:"id"`
	ProviderID                     int64      `json:"provider_id"`
	ControlEnabled                 bool       `json:"control_enabled"`
	ControlIntervalSeconds         int        `json:"control_interval_seconds"`
	DataEnabled                    bool       `json:"data_enabled"`
	DataIntervalSeconds            int        `json:"data_interval_seconds"`
	SelectedAccountIDs             []int64    `json:"selected_account_ids"`
	AllowMediaProbe                bool       `json:"allow_media_probe"`
	TimeoutSeconds                 int        `json:"timeout_seconds"`
	DegradedLatencyMS              int        `json:"degraded_latency_ms"`
	FailureThreshold               int        `json:"failure_threshold"`
	RecoveryThreshold              int        `json:"recovery_threshold"`
	AccountStatusSyncEnabled       bool       `json:"account_status_sync_enabled"`
	AccountStatusFailureThreshold  int        `json:"account_status_failure_threshold"`
	AccountStatusRecoveryThreshold int        `json:"account_status_recovery_threshold"`
	LastControlRunAt               *time.Time `json:"last_control_run_at,omitempty"`
	LastDataRunAt                  *time.Time `json:"last_data_run_at,omitempty"`
}

type Sub2APIProviderProbeService struct {
	providerRepo        Sub2APIProviderRepository
	probeRepo           Sub2APIProviderProbeRepository
	accountRepo         Sub2APIAccountRepository
	accountTest         Sub2APIProbeTestRunner
	rateLimitService    *RateLimitService
	tokenCache          *sub2api.TokenCache
	encryptor           ProviderTokenEncryptor
	usageRepo           UsageLogRepository
	autoOptimizeTrigger Sub2APIProbeAutoOptimizeTrigger
	remoteOverviewCache Sub2APIProviderRemoteOverviewCache
	operationGate       *Sub2APIProviderOperationGate
	lockCache           LeaderLockCache
	db                  *sql.DB
	instanceID          string
}

func NewSub2APIProviderProbeService(providerRepo Sub2APIProviderRepository, probeRepo Sub2APIProviderProbeRepository, accountRepo Sub2APIAccountRepository, accountTest Sub2APIProbeTestRunner, rateLimitService *RateLimitService, tokenCache *sub2api.TokenCache, encryptor ProviderTokenEncryptor, operationGate *Sub2APIProviderOperationGate) *Sub2APIProviderProbeService {
	return &Sub2APIProviderProbeService{providerRepo: providerRepo, probeRepo: probeRepo, accountRepo: accountRepo, accountTest: accountTest, rateLimitService: rateLimitService, tokenCache: tokenCache, encryptor: encryptor, operationGate: operationGate, instanceID: uuid.NewString()}
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

// SetUsageLogRepository wires the optional local revenue reader used when the
// control probe refreshes per-account profit snapshots.
func (s *Sub2APIProviderProbeService) SetUsageLogRepository(repo UsageLogRepository) {
	if s != nil {
		s.usageRepo = repo
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
		health := &Sub2APIProviderHealth{ProviderID: providerID, Status: "unknown", ControlStatus: "unknown", DataStatus: "unknown", TrafficStatus: "unknown"}
		s.applyLiveProviderTraffic(ctx, health)
		return health, nil
	}
	if err != nil {
		return nil, err
	}
	health := healthFromRun(run)
	s.applyLiveProviderTraffic(ctx, health)
	if runs, listErr := s.probeRepo.ListRuns(ctx, providerID, 20); listErr == nil {
		health.ConsecutiveFailures = countConsecutiveProbeFailures(runs)
	}
	return health, nil
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
			if overview.Latest == nil {
				overview.Latest = &Sub2APIProviderHealth{ProviderID: providerID, Status: "unknown", ControlStatus: "unknown", DataStatus: "unknown", TrafficStatus: "unknown"}
			}
			s.applyLiveProviderTraffic(ctx, overview.Latest)
		}
	}
	return overviews, nil
}

const (
	providerHealthOverviewBucketCount   = 60
	providerHealthOverviewBucketSeconds = 24 * 60
	// Account route timelines are event sequences: one point per real user request.
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
		result.Details["traffic_failure_count"] = traffic.FailureCount
		result.Details["passive_health_status"] = passiveHealthStatus(traffic.RequestCount, traffic.FailureCount, func() *int {
			if traffic.RequestCount == 0 {
				return nil
			}
			p := traffic.P95LatencyMS
			return &p
		}())
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
	type completedTargetProbe struct {
		target               *ent.Sub2APIProviderProbeTarget
		run                  *ent.Sub2APIProviderProbeTargetRun
		adminStateGeneration string
	}
	completedTargetProbes := make([]completedTargetProbe, 0, len(targets))
	minimumHealthyStreak := probeAdaptiveMinimumHealthyStreak(cfg)
	for _, target := range targets {
		if !shouldRunProviderProbeTargetWithRecovery(target, scheduled, includeTargets, time.Now(), minimumHealthyStreak) {
			continue
		}
		// Persist the probe first. Account state is projected below from persisted
		// history so failure/recovery thresholds and optimization ordering can be
		// applied consistently across all routes in this scheduler cycle.
		targetRun, adminStateGeneration, acquired, err := s.runTargetIfProviderAvailable(ctx, target)
		if !acquired {
			// An optimizer or remote binding refresh currently owns this Provider.
			// Do not manufacture an unhealthy sample or advance last_run_at; the
			// target remains due and the next runner scan retries it.
			continue
		}
		if err != nil {
			targetRunErr = errors.Join(targetRunErr, err)
			continue
		}
		if scheduled && targetRun != nil {
			completedTargetProbes = append(completedTargetProbes, completedTargetProbe{
				target:               target,
				run:                  targetRun,
				adminStateGeneration: adminStateGeneration,
			})
		}
	}
	autoOptimizeInputs := make([]Sub2APIProbeAutoOptimizeInput, 0, len(completedTargetProbes))
	historyByTarget := make(map[int64][]*ent.Sub2APIProviderProbeTargetRun, len(completedTargetProbes))
	if scheduled && len(completedTargetProbes) > 0 {
		targetIDs := make([]int64, 0, len(completedTargetProbes))
		for _, completed := range completedTargetProbes {
			targetIDs = append(targetIDs, completed.target.ID)
		}
		recentRuns, historyErr := s.probeRepo.ListRecentTargetRuns(
			ctx,
			targetIDs,
			time.Now().UTC().Add(-upstreamLogRetentionPeriod),
			maxProbeAutoOptimizeHistory,
		)
		now := time.Now().UTC()
		runsByTarget := make(map[int64][]*ent.Sub2APIProviderProbeTargetRun, len(targetIDs))
		if historyErr != nil {
			logger.LegacyPrintf("service.sub2api_provider_probe", "[Sub2APIProviderProbe] provider=%d load target history failed: %v", providerID, historyErr)
			// A history read is only required for streak-based degraded/cost
			// actions. Preserve the single-sample unhealthy recovery path when
			// the best-effort history query is temporarily unavailable.
			for _, completed := range completedTargetProbes {
				runsByTarget[completed.target.ID] = []*ent.Sub2APIProviderProbeTargetRun{completed.run}
			}
		} else {
			for _, recentRun := range recentRuns {
				runsByTarget[recentRun.TargetID] = append(runsByTarget[recentRun.TargetID], recentRun)
			}
		}
		for _, completed := range completedTargetProbes {
			history := runsByTarget[completed.target.ID]
			historyByTarget[completed.target.ID] = history
			if input, ok := probeAutoOptimizeInputWithHistory(completed.target, completed.run, history, now); ok {
				input.AccountStatusSyncEnabled = cfg.AccountStatusSyncEnabled
				input.AdminStateGeneration = completed.adminStateGeneration
				failureThreshold := cfg.AccountStatusFailureThreshold
				if failureThreshold < 1 {
					failureThreshold = defaultProbeFailureThreshold
				}
				input.AccountFailureThresholdMet = consecutiveTargetProbeStatus(history, "unhealthy") >= failureThreshold
				autoOptimizeInputs = append(autoOptimizeInputs, input)
			}
		}
	}
	admittedTargets := make(map[int64]struct{})
	if scheduled && s.autoOptimizeTrigger != nil && len(autoOptimizeInputs) > 0 {
		if detailed, ok := s.autoOptimizeTrigger.(Sub2APIProbeAutoOptimizeAdmissionTrigger); ok {
			admission, err := detailed.TriggerProbeAutoOptimizeWithAdmission(ctx, providerID, autoOptimizeInputs)
			for _, targetID := range admission.TargetIDs {
				admittedTargets[targetID] = struct{}{}
			}
			if err != nil {
				logger.LegacyPrintf("service.sub2api_provider_probe", "[Sub2APIProviderProbe] provider=%d auto optimize trigger failed: %v", providerID, err)
			}
		} else {
			admitted, err := s.autoOptimizeTrigger.TriggerProbeAutoOptimize(ctx, providerID, autoOptimizeInputs)
			// A legacy trigger exposes only a count. Fail closed when it
			// admits a partial batch because the probe cannot identify which
			// account was actually claimed.
			if err == nil && admitted == len(autoOptimizeInputs) {
				for _, input := range autoOptimizeInputs {
					admittedTargets[input.TargetID] = struct{}{}
				}
			}
			if err != nil {
				logger.LegacyPrintf("service.sub2api_provider_probe", "[Sub2APIProviderProbe] provider=%d auto optimize trigger failed: %v", providerID, err)
			}
		}
	}
	if cfg.AccountStatusSyncEnabled {
		for _, completed := range completedTargetProbes {
			history := historyByTarget[completed.target.ID]
			_, optimizerAdmitted := admittedTargets[completed.target.ID]
			if err := s.projectAccountStatusFromProbe(ctx, cfg, completed.target, completed.run, history, completed.adminStateGeneration, optimizerAdmitted); err != nil {
				logger.LegacyPrintf("service.sub2api_provider_probe", "[Sub2APIProviderProbe] provider=%d account=%d target=%d project account status failed: %v", providerID, completed.target.AccountID, completed.target.ID, err)
			}
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
	// A stable Key-to-group snapshot is route state, so it shares the same gate
	// as account probes and optimization. If another route operation is active,
	// keep checking platform availability but skip persistence for this cycle.
	var bindingRelease func()
	bindingAcquired := false
	if s.accountRepo != nil && s.probeRepo != nil {
		bindingRelease, bindingAcquired = s.operationGate.TryAcquire(ctx, provider.ID, sub2APIProviderProbeOperationTTL)
		if bindingAcquired {
			defer bindingRelease()
		} else {
			result.Details["binding_sync_status"] = "skipped_busy"
		}
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
	// authenticated client and Groups response. It also samples account profit
	// from local request records and cached upstream group rates. Profit failures
	// remain best-effort and never affect Provider availability.
	var remoteOverview *Sub2APIProviderRemoteOverview
	if groupsErr == nil && s.remoteOverviewCache != nil {
		assetCtx, assetCancel := newStageContext()
		overview, assetErr := collectSub2APIProviderRemoteOverview(
			assetCtx,
			provider.ID,
			client,
			groups,
			provider.RemoteCostDivisor,
			Sub2APIProviderRemoteOverviewSourceControlProbe,
			assetAttemptedAt,
		)
		assetCancel()
		if assetErr != nil {
			recordAssetFailure(assetErr)
		} else {
			remoteOverview = overview
			if s.accountRepo != nil {
				accounts, accountsErr := s.accountRepo.ListByProviderID(ctx, provider.ID)
				if accountsErr != nil {
					message := trimProbeError(accountsErr)
					remoteOverview.Profit = &Sub2APIProviderProfitSummary{Available: false, SampledAt: assetAttemptedAt, Error: &message}
				} else {
					if reader, ok := s.usageRepo.(ProviderLocalUsageReader); ok {
						accountIDs := make([]int64, 0, len(accounts))
						for _, account := range accounts {
							accountIDs = append(accountIDs, account.ID)
						}
						if local, localErr := reader.GetProviderLocalUsage(ctx, accountIDs, time.Time{}, assetAttemptedAt); localErr == nil {
							remoteOverview.LocalUsage = localUsageSnapshot(local)
						}
					}
					remoteOverview.Profit = collectProviderProfitSnapshot(ctx, accounts, s.usageRepo, assetAttemptedAt)
				}
			}
			storeRemoteOverviewSuccess(ctx, s.remoteOverviewCache, overview)
			result.Details["asset_snapshot_status"] = "updated"
			result.Details["asset_group_count"] = len(overview.Groups)
		}
	} else if groupsErr != nil {
		recordAssetFailure(groupsErr)
	}
	if keysErr == nil && groupsErr == nil && bindingAcquired {
		effectiveRates := make(map[int64]float64)
		if remoteOverview != nil {
			for _, group := range remoteOverview.Groups {
				effectiveRates[group.ID] = group.EffectiveMultiplier
			}
		} else {
			// Balance collection and custom group rates are independent upstream
			// surfaces. A temporary /auth/me failure must not make binding sync
			// overwrite a valid custom rate with the catalog default.
			ratesCtx, ratesCancel := newStageContext()
			overrides, ratesErr := client.GetGroupRates(ratesCtx)
			ratesCancel()
			if ratesErr != nil {
				result.Details["binding_sync_rates_error"] = trimProbeError(ratesErr)
			} else {
				for _, group := range groups {
					if rate, ok := overrides[strconv.FormatInt(group.ID, 10)]; ok {
						effectiveRates[group.ID] = rate
					}
				}
			}
		}
		syncResult, syncErr := s.syncControlProbeBindingsLocked(ctx, provider.ID, keys, groups, effectiveRates)
		result.Details["binding_sync_accounts_updated"] = syncResult.AccountsUpdated
		result.Details["binding_sync_accounts_cleared"] = syncResult.AccountsCleared
		result.Details["binding_sync_targets_updated"] = syncResult.TargetsUpdated
		if syncErr != nil {
			result.Details["binding_sync_status"] = "failed"
			result.Details["binding_sync_error"] = trimProbeError(syncErr)
			logger.LegacyPrintf("service.sub2api_provider_probe", "[Sub2APIProviderProbe] provider=%d sync control bindings failed: %v", provider.ID, syncErr)
		} else {
			result.Details["binding_sync_status"] = "updated"
		}
	}
	availabilityStatus, probeErr := controlProbeAvailabilityStatus(healthErr, keysErr, groupsErr)
	if availabilityStatus != "healthy" {
		result.ErrorCategory, result.ErrorMessage = providerProbeStringPtr(classifyProbeError(probeErr)), providerProbeStringPtr(trimProbeError(probeErr))
		return availabilityStatus
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
	if run.Details != nil {
		_ = decodeProviderProbeDetail(run.Details["traffic_failure_count"], &health.TrafficFailureCount)
		_ = decodeProviderProbeDetail(run.Details["passive_health_status"], &health.PassiveHealthStatus)
		_ = decodeProviderProbeDetail(run.Details["passive_cooldown_until"], &health.PassiveCooldownUntil)
	}
	if health.PassiveHealthStatus == "" {
		health.PassiveHealthStatus = passiveHealthStatus(health.TrafficRequestCount, health.TrafficFailureCount, health.TrafficP95LatencyMS)
	}
	return health
}

func passiveHealthStatus(samples, failures int, p95 *int) string {
	if samples < 3 {
		return "unknown"
	}
	if failures >= 3 || float64(failures)/float64(samples) >= 0.5 {
		return "temporarily_unschedulable"
	}
	if float64(failures)/float64(samples) >= 0.2 || (p95 != nil && *p95 > 5000) {
		return "degraded"
	}
	return "healthy"
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
	return &Sub2APIProviderProbeConfigView{ID: c.ID, ProviderID: c.ProviderID, ControlEnabled: c.ControlEnabled, ControlIntervalSeconds: c.ControlIntervalSeconds, DataEnabled: c.DataEnabled, DataIntervalSeconds: c.DataIntervalSeconds, SelectedAccountIDs: c.SelectedAccountIds, AllowMediaProbe: c.AllowMediaProbe, TimeoutSeconds: c.TimeoutSeconds, DegradedLatencyMS: c.DegradedLatencyMs, FailureThreshold: c.FailureThreshold, RecoveryThreshold: c.RecoveryThreshold, AccountStatusSyncEnabled: c.AccountStatusSyncEnabled, AccountStatusFailureThreshold: c.AccountStatusFailureThreshold, AccountStatusRecoveryThreshold: c.AccountStatusRecoveryThreshold, LastControlRunAt: c.LastControlRunAt, LastDataRunAt: c.LastDataRunAt}
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
	if in.AccountStatusFailureThreshold != nil && (*in.AccountStatusFailureThreshold < 1 || *in.AccountStatusFailureThreshold > 20) {
		return invalidProviderProbeConfig("account_status_failure_threshold must be between 1 and 20")
	}
	if in.AccountStatusRecoveryThreshold != nil && (*in.AccountStatusRecoveryThreshold < 1 || *in.AccountStatusRecoveryThreshold > 20) {
		return invalidProviderProbeConfig("account_status_recovery_threshold must be between 1 and 20")
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

func effectiveProbeTargetIntervalSeconds(target *ent.Sub2APIProviderProbeTarget) int {
	return effectiveProbeTargetIntervalSecondsWithRecovery(target, 0)
}

func effectiveProbeTargetIntervalSecondsWithRecovery(target *ent.Sub2APIProviderProbeTarget, minimumHealthyStreak int) int {
	if target == nil || target.IntervalSeconds <= 0 {
		return defaultProbeTargetIntervalSeconds
	}
	base := target.IntervalSeconds
	healthyThreshold := max(target.HealthyIntervalThreshold, minimumHealthyStreak)
	stableThreshold := max(target.StableHealthyThreshold, minimumHealthyStreak)
	if !target.AdaptiveIntervalEnabled || target.ConsecutiveHealthy < healthyThreshold {
		return base
	}
	if target.ConsecutiveHealthy >= stableThreshold {
		return max(base, target.StableHealthyIntervalSeconds)
	}
	return max(base, target.HealthyIntervalSeconds)
}

func probeAdaptiveMinimumHealthyStreak(cfg *ent.Sub2APIProviderProbeConfig) int {
	if cfg == nil || !cfg.AccountStatusSyncEnabled {
		return 0
	}
	return max(cfg.AccountStatusRecoveryThreshold, 0)
}

func shouldRunProviderProbeTarget(target *ent.Sub2APIProviderProbeTarget, scheduled, includeTargets bool, now time.Time) bool {
	return shouldRunProviderProbeTargetWithRecovery(target, scheduled, includeTargets, now, 0)
}

func shouldRunProviderProbeTargetWithRecovery(target *ent.Sub2APIProviderProbeTarget, scheduled, includeTargets bool, now time.Time, minimumHealthyStreak int) bool {
	// Automatic account health comes only from real gateway requests.
	// Legacy enabled/interval settings must never schedule model requests.
	if scheduled || !includeTargets || target == nil || !target.Enabled {
		return false
	}
	return !scheduled || probeIntervalDueWithJitterAt(target.LastRunAt, effectiveProbeTargetIntervalSecondsWithRecovery(target, minimumHealthyStreak), target.ID, now)
}

func shouldRunScheduledAnswerProbe(target *ent.Sub2APIProviderProbeTarget, lastAnswerAt *time.Time, now time.Time) bool {
	if target == nil || !target.AnswerProbeEnabled || target.AnswerProbePrompt == nil || strings.TrimSpace(*target.AnswerProbePrompt) == "" {
		return false
	}
	// Answer tests use the operator's explicit fixed interval. Health-probe
	// adaptive stages and jitter should not change the requested cadence.
	return probeIntervalDueAt(lastAnswerAt, target.AnswerProbeIntervalSeconds, now)
}

func providerProbeStringPtr(v string) *string     { return &v }
func providerProbeTimePtr(v time.Time) *time.Time { return &v }

// Sub2APIProviderProbeRunner scans frequently; each persisted config controls
// due decision is persisted in each config, so restarts do not reset cadence.
type Sub2APIProviderProbeRunner struct {
	service                  *Sub2APIProviderProbeService
	cron                     *cron.Cron
	lockCache                LeaderLockCache
	db                       *sql.DB
	instanceID               string
	once                     sync.Once
	retentionMu              sync.Mutex
	nextRetentionRunAt       time.Time
	nextCostSnapshotAt       time.Time
	credentialRefreshMu      sync.Mutex
	credentialRefreshRetryAt map[int64]time.Time
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
	now := time.Now().UTC()
	r.runLogRetentionIfDue(now)
	r.runProviderAccountCostSnapshotsIfDue(now)

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
		wg.Add(1)
		workers <- struct{}{}
		go func(id int64) {
			defer wg.Done()
			defer func() { <-workers }()
			workCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()
			r.refreshProviderCredentials(workCtx, id, now)
			if err := r.service.runHybridAccountHealth(workCtx, id); err != nil {
				logger.LegacyPrintf("service.sub2api_provider_probe", "account hybrid health: %v", err)
			}
			if err := r.service.runScheduledAnswerProbes(workCtx, id); err != nil {
				logger.LegacyPrintf("service.sub2api_provider_probe", "scheduled answer probe: %v", err)
			}
			cfg, err := r.service.ensureConfig(workCtx, id)
			if err == nil && cfg.ControlEnabled && probeIntervalDue(cfg.LastControlRunAt, cfg.ControlIntervalSeconds) {
				_, _ = r.service.run(workCtx, id, true, false)
			}
		}(p.ID)
	}
	wg.Wait()
}

func (r *Sub2APIProviderProbeRunner) refreshProviderCredentials(ctx context.Context, providerID int64, now time.Time) {
	if r == nil || r.service == nil || providerID <= 0 || r.service.providerRepo == nil {
		return
	}
	r.credentialRefreshMu.Lock()
	if r.credentialRefreshRetryAt != nil && now.Before(r.credentialRefreshRetryAt[providerID]) {
		r.credentialRefreshMu.Unlock()
		return
	}
	r.credentialRefreshMu.Unlock()

	provider, err := r.service.providerRepo.GetByID(ctx, providerID)
	if err != nil || provider == nil || !providerCredentialRefreshDue(provider, now) {
		return
	}
	release, acquired := r.service.operationGate.TryAcquire(ctx, providerID, providerCredentialRefreshTimeout)
	if !acquired {
		return
	}
	defer release()

	refreshCtx, cancel := context.WithTimeout(ctx, providerCredentialRefreshTimeout)
	defer cancel()
	client, err := newAuthedSub2APIProviderClient(refreshCtx, provider, r.service.providerRepo, r.service.tokenCache, r.service.encryptor)
	if err == nil && provider.AccessTokenExpiresAt.After(now) {
		err = client.RefreshProviderCredentials(refreshCtx)
	}
	if err != nil {
		message := trimProviderAuthError(err)
		_ = r.service.providerRepo.UpdateAuthError(ctx, providerID, &message)
		r.credentialRefreshMu.Lock()
		if r.credentialRefreshRetryAt == nil {
			r.credentialRefreshRetryAt = make(map[int64]time.Time)
		}
		r.credentialRefreshRetryAt[providerID] = now.Add(providerCredentialRefreshRetry)
		r.credentialRefreshMu.Unlock()
		logger.LegacyPrintf("service.sub2api_provider_probe", "provider %d scheduled credential refresh failed: %v", providerID, err)
		return
	}
	_ = r.service.providerRepo.UpdateAuthError(ctx, providerID, nil)
	r.credentialRefreshMu.Lock()
	delete(r.credentialRefreshRetryAt, providerID)
	r.credentialRefreshMu.Unlock()
}

func providerCredentialRefreshDue(provider *ent.Sub2APIProvider, now time.Time) bool {
	if provider == nil || provider.AccessTokenExpiresAt == nil || provider.AccessTokenExpiresAt.After(now.Add(providerCredentialRefreshLead)) {
		return false
	}
	mode := strings.TrimSpace(provider.AuthMode)
	switch provider.ProviderType {
	case "newapi":
		return mode == domain.Sub2APIProviderAuthModeBrowserSession && provider.RefreshTokenEncrypted != nil && strings.TrimSpace(*provider.RefreshTokenEncrypted) != ""
	default:
		return mode == domain.Sub2APIProviderAuthModeTokenPair && provider.RefreshTokenEncrypted != nil && strings.TrimSpace(*provider.RefreshTokenEncrypted) != ""
	}
}

func (r *Sub2APIProviderProbeRunner) runProviderAccountCostSnapshotsIfDue(now time.Time) {
	if r == nil || r.db == nil {
		return
	}
	r.retentionMu.Lock()
	if !r.nextCostSnapshotAt.IsZero() && now.Before(r.nextCostSnapshotAt) {
		r.retentionMu.Unlock()
		return
	}
	r.nextCostSnapshotAt = now.Add(time.Hour)
	r.retentionMu.Unlock()

	lockCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	release, ok := tryAcquireSingletonLeaderLock(lockCtx, r.lockCache, r.db, providerAccountDailyCostSnapshotLockKey, r.instanceID, 10*time.Minute)
	cancel()
	if !ok {
		return
	}
	defer release()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	start := time.Date(now.In(time.Local).Year(), now.In(time.Local).Month(), now.In(time.Local).Day(), 0, 0, 0, 0, time.Local).AddDate(0, 0, -1)
	err := r.service.RebuildDailyCosts(ctx, 0, start, start.AddDate(0, 0, 2))
	if err != nil {
		logger.LegacyPrintf("service.sub2api_provider_probe", "[ProviderAccountDailyCostSnapshot] upsert failed: %v", err)
	}
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
	answerRetentionRepo, supportsAnswerRetention := r.service.probeRepo.(Sub2APIAnswerProbeResponseRetentionRepository)
	if !supportsAnswerRetention {
		return
	}
	cleared, err := clearExpiredAnswerProbeResponses(ctx, answerRetentionRepo, now.UTC().Add(-answerProbeHistoryWindow), upstreamLogRetentionBatchSize)
	if err != nil {
		logger.LegacyPrintf("service.sub2api_provider_probe", "[Sub2APIAnswerProbeRetention] cleanup failed: %v", err)
		return
	}
	if cleared > 0 {
		logger.LegacyPrintf("service.sub2api_provider_probe", "[Sub2APIAnswerProbeRetention] cleared response_text=%d cutoff=%s", cleared, now.UTC().Add(-answerProbeHistoryWindow).Format(time.RFC3339))
	}
}

func clearExpiredAnswerProbeResponses(ctx context.Context, repo Sub2APIAnswerProbeResponseRetentionRepository, cutoff time.Time, batchSize int) (int64, error) {
	if repo == nil {
		return 0, nil
	}
	if batchSize <= 0 {
		return 0, errors.New("answer probe response cleanup batch size must be positive")
	}
	var total int64
	for {
		cleared, err := repo.ClearAnswerProbeResponsesBefore(ctx, cutoff.UTC(), batchSize)
		total += cleared
		if err != nil {
			return total, err
		}
		if cleared < int64(batchSize) {
			return total, nil
		}
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

package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/sub2apiproviderprobeconfig"
	"github.com/Wei-Shaw/sub2api/ent/sub2apiproviderproberun"
	"github.com/Wei-Shaw/sub2api/ent/sub2apiproviderprobetarget"
	"github.com/Wei-Shaw/sub2api/ent/sub2apiproviderprobetargetrun"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type sub2APIProviderProbeRepository struct {
	client *ent.Client
	db     *sql.DB
}

func NewSub2APIProviderProbeRepository(client *ent.Client, db *sql.DB) service.Sub2APIProviderProbeRepository {
	return &sub2APIProviderProbeRepository{client: client, db: db}
}

const deleteExpiredSub2APIUpstreamLogBatchSQL = `
WITH expired AS (
  SELECT id
    FROM %s
   WHERE created_at < $1
   ORDER BY id
   LIMIT $2
)
DELETE FROM %s AS logs
USING expired
WHERE logs.id = expired.id`

// DeleteUpstreamLogsBefore physically removes one bounded batch from every
// upstream-management log table. Table names are internal constants; cutoff
// and batch size remain bound parameters.
func (r *sub2APIProviderProbeRepository) DeleteUpstreamLogsBefore(
	ctx context.Context,
	cutoff time.Time,
	batchSize int,
) (service.Sub2APIUpstreamLogCleanupResult, error) {
	var out service.Sub2APIUpstreamLogCleanupResult
	if r == nil || r.db == nil {
		return out, fmt.Errorf("nil database for upstream log retention")
	}
	if batchSize <= 0 {
		return out, fmt.Errorf("upstream log retention batch size must be positive")
	}

	targets := []struct {
		table string
		count *int64
	}{
		{table: "sub2api_provider_probe_runs", count: &out.ProviderProbeRuns},
		{table: "sub2api_provider_probe_target_runs", count: &out.AccountProbeRuns},
		{table: "sub2api_optimize_logs", count: &out.OptimizeLogs},
	}
	for _, target := range targets {
		query := fmt.Sprintf(deleteExpiredSub2APIUpstreamLogBatchSQL, target.table, target.table)
		result, err := r.db.ExecContext(ctx, query, cutoff.UTC(), batchSize)
		if err != nil {
			return out, fmt.Errorf("delete expired %s: %w", target.table, err)
		}
		deleted, err := result.RowsAffected()
		if err != nil {
			return out, fmt.Errorf("count deleted %s: %w", target.table, err)
		}
		*target.count = deleted
	}
	return out, nil
}

func (r *sub2APIProviderProbeRepository) GetConfig(ctx context.Context, providerID int64) (*ent.Sub2APIProviderProbeConfig, error) {
	return r.client.Sub2APIProviderProbeConfig.Query().Where(
		sub2apiproviderprobeconfig.ProviderID(providerID),
	).Only(ctx)
}

func (r *sub2APIProviderProbeRepository) CreateDefaultConfig(ctx context.Context, providerID int64) (*ent.Sub2APIProviderProbeConfig, error) {
	return r.client.Sub2APIProviderProbeConfig.Create().
		SetProviderID(providerID).
		SetControlEnabled(true).
		SetControlIntervalSeconds(1800).
		SetDataEnabled(false).
		SetDataIntervalSeconds(1800).
		SetSelectedAccountIds([]int64{}).
		SetAllowMediaProbe(false).
		SetTimeoutSeconds(15).
		SetDegradedLatencyMs(2000).
		SetFailureThreshold(3).
		SetRecoveryThreshold(2).
		Save(ctx)
}

func (r *sub2APIProviderProbeRepository) UpdateConfig(ctx context.Context, providerID int64, input *service.Sub2APIProviderProbeConfigInput) (*ent.Sub2APIProviderProbeConfig, error) {
	u := r.client.Sub2APIProviderProbeConfig.Update().Where(sub2apiproviderprobeconfig.ProviderID(providerID))
	if input.ControlEnabled != nil {
		u.SetControlEnabled(*input.ControlEnabled)
	}
	if input.ControlIntervalSeconds != nil {
		u.SetControlIntervalSeconds(*input.ControlIntervalSeconds)
	}
	if input.DataEnabled != nil {
		u.SetDataEnabled(*input.DataEnabled)
	}
	if input.DataIntervalSeconds != nil {
		u.SetDataIntervalSeconds(*input.DataIntervalSeconds)
	}
	if input.SelectedAccountIDs != nil {
		u.SetSelectedAccountIds(input.SelectedAccountIDs)
	}
	if input.AllowMediaProbe != nil {
		u.SetAllowMediaProbe(*input.AllowMediaProbe)
	}
	if input.TimeoutSeconds != nil {
		u.SetTimeoutSeconds(*input.TimeoutSeconds)
	}
	if input.DegradedLatencyMS != nil {
		u.SetDegradedLatencyMs(*input.DegradedLatencyMS)
	}
	if input.FailureThreshold != nil {
		u.SetFailureThreshold(*input.FailureThreshold)
	}
	if input.RecoveryThreshold != nil {
		u.SetRecoveryThreshold(*input.RecoveryThreshold)
	}
	if _, err := u.Save(ctx); err != nil {
		return nil, err
	}
	return r.GetConfig(ctx, providerID)
}

func (r *sub2APIProviderProbeRepository) MarkRun(ctx context.Context, providerID int64, dataPlane bool, at time.Time) error {
	u := r.client.Sub2APIProviderProbeConfig.Update().Where(sub2apiproviderprobeconfig.ProviderID(providerID))
	if dataPlane {
		u.SetLastDataRunAt(at)
	} else {
		u.SetLastControlRunAt(at)
	}
	_, err := u.Save(ctx)
	return err
}

func (r *sub2APIProviderProbeRepository) CreateRun(ctx context.Context, input *service.Sub2APIProviderProbeRunInput) (*ent.Sub2APIProviderProbeRun, error) {
	b := r.client.Sub2APIProviderProbeRun.Create().
		SetProviderID(input.ProviderID).
		SetOverallStatus(sub2apiproviderproberun.OverallStatus(input.OverallStatus)).
		SetControlStatus(sub2apiproviderproberun.ControlStatus(input.ControlStatus)).
		SetDataStatus(sub2apiproviderproberun.DataStatus(input.DataStatus)).
		SetTrafficStatus(sub2apiproviderproberun.TrafficStatus(input.TrafficStatus)).
		SetDataProbeCount(input.DataProbeCount).
		SetDataProbeSuccess(input.DataProbeSuccess).
		SetDataProbeFailed(input.DataProbeFailed).
		SetTrafficRequestCount(input.TrafficRequestCount).
		SetDetails(input.Details).
		SetStartedAt(input.StartedAt).
		SetFinishedAt(input.FinishedAt)
	if input.LoginLatencyMS != nil {
		b.SetLoginLatencyMs(*input.LoginLatencyMS)
	}
	if input.HealthLatencyMS != nil {
		b.SetHealthLatencyMs(*input.HealthLatencyMS)
	}
	if input.KeysLatencyMS != nil {
		b.SetKeysLatencyMs(*input.KeysLatencyMS)
	}
	if input.GroupsLatencyMS != nil {
		b.SetGroupsLatencyMs(*input.GroupsLatencyMS)
	}
	if input.TrafficSuccessRate != nil {
		b.SetTrafficSuccessRate(*input.TrafficSuccessRate)
	}
	if input.TrafficP95LatencyMS != nil {
		b.SetTrafficP95LatencyMs(*input.TrafficP95LatencyMS)
	}
	if input.ErrorCategory != nil {
		b.SetErrorCategory(*input.ErrorCategory)
	}
	if input.ErrorMessage != nil {
		b.SetErrorMessage(*input.ErrorMessage)
	}
	return b.Save(ctx)
}

func (r *sub2APIProviderProbeRepository) LatestRun(ctx context.Context, providerID int64) (*ent.Sub2APIProviderProbeRun, error) {
	return r.client.Sub2APIProviderProbeRun.Query().Where(
		sub2apiproviderproberun.ProviderID(providerID),
	).Order(ent.Desc(sub2apiproviderproberun.FieldCreatedAt)).First(ctx)
}

func (r *sub2APIProviderProbeRepository) ListRuns(ctx context.Context, providerID int64, limit int) ([]*ent.Sub2APIProviderProbeRun, error) {
	if limit < 1 || limit > 500 {
		limit = 100
	}
	return r.client.Sub2APIProviderProbeRun.Query().Where(
		sub2apiproviderproberun.ProviderID(providerID),
	).Order(ent.Desc(sub2apiproviderproberun.FieldCreatedAt)).Limit(limit).All(ctx)
}

func (r *sub2APIProviderProbeRepository) ListRunsSince(ctx context.Context, providerIDs []int64, since time.Time) ([]*ent.Sub2APIProviderProbeRun, error) {
	if len(providerIDs) == 0 {
		return []*ent.Sub2APIProviderProbeRun{}, nil
	}
	return r.client.Sub2APIProviderProbeRun.Query().Where(
		sub2apiproviderproberun.ProviderIDIn(providerIDs...),
		sub2apiproviderproberun.CreatedAtGTE(since.UTC()),
	).Order(
		ent.Asc(sub2apiproviderproberun.FieldProviderID),
		ent.Desc(sub2apiproviderproberun.FieldCreatedAt),
	).All(ctx)
}

func (r *sub2APIProviderProbeRepository) TrafficStats(ctx context.Context, providerID int64, since time.Time) (*service.Sub2APIProviderTrafficStats, error) {
	if r.db == nil {
		return &service.Sub2APIProviderTrafficStats{}, nil
	}
	const q = `
WITH requests AS (
  SELECT ul.duration_ms::double precision AS latency_ms, TRUE AS success
    FROM usage_logs ul JOIN accounts a ON a.id = ul.account_id
   WHERE a.provider_id = $1 AND ul.created_at >= $2
  UNION ALL
  SELECT COALESCE(o.response_latency_ms, o.upstream_latency_ms, 0)::double precision, FALSE
    FROM ops_error_logs o JOIN accounts a ON a.id = o.account_id
   WHERE a.provider_id = $1 AND o.created_at >= $2 AND COALESCE(o.status_code, 0) >= 400
), summary AS (
  SELECT COUNT(*)::int AS total,
         COUNT(*) FILTER (WHERE success)::int AS successes,
         percentile_cont(0.95) WITHIN GROUP (ORDER BY latency_ms) AS p95
    FROM requests
)
SELECT total, successes, p95 FROM summary`
	var total, successes int
	var p95 sql.NullFloat64
	if err := r.db.QueryRowContext(ctx, q, providerID, since.UTC()).Scan(&total, &successes, &p95); err != nil {
		return nil, err
	}
	out := &service.Sub2APIProviderTrafficStats{RequestCount: total}
	if total > 0 {
		out.SuccessRate = float64(successes) / float64(total) * 100
		out.P95LatencyMS = int(p95.Float64)
	}
	return out, nil
}

func (r *sub2APIProviderProbeRepository) ListTargets(ctx context.Context, providerID int64) ([]*ent.Sub2APIProviderProbeTarget, error) {
	return r.client.Sub2APIProviderProbeTarget.Query().Where(
		sub2apiproviderprobetarget.ProviderID(providerID),
	).Order(
		ent.Asc(sub2apiproviderprobetarget.FieldAccountID),
	).All(ctx)
}

func (r *sub2APIProviderProbeRepository) GetTarget(ctx context.Context, providerID, targetID int64) (*ent.Sub2APIProviderProbeTarget, error) {
	return r.client.Sub2APIProviderProbeTarget.Query().Where(
		sub2apiproviderprobetarget.ID(targetID),
		sub2apiproviderprobetarget.ProviderID(providerID),
	).Only(ctx)
}

func (r *sub2APIProviderProbeRepository) CreateTarget(ctx context.Context, input *service.Sub2APIProviderProbeTargetCreateInput) (*ent.Sub2APIProviderProbeTarget, error) {
	b := r.client.Sub2APIProviderProbeTarget.Create().
		SetProviderID(input.ProviderID).
		SetAccountID(input.AccountID).
		SetPlatform(input.Platform).
		SetEnabled(input.Enabled).
		SetIntervalSeconds(input.IntervalSeconds).
		SetAllowMediaProbe(input.AllowMediaProbe).
		SetTimeoutSeconds(input.TimeoutSeconds).
		SetDegradedLatencyMs(input.DegradedLatencyMS).
		SetFailureThreshold(input.FailureThreshold).
		SetRecoveryThreshold(input.RecoveryThreshold)
	if input.ProviderAPIKeyID != nil {
		b.SetProviderAPIKeyID(*input.ProviderAPIKeyID)
	}
	if input.RemoteGroupID != nil {
		b.SetRemoteGroupID(*input.RemoteGroupID)
	}
	if input.RemoteGroupName != nil {
		b.SetRemoteGroupName(*input.RemoteGroupName)
	}
	return b.Save(ctx)
}

func (r *sub2APIProviderProbeRepository) UpdateTarget(ctx context.Context, providerID, targetID int64, input *service.Sub2APIProviderProbeTargetInput) (*ent.Sub2APIProviderProbeTarget, error) {
	u := r.client.Sub2APIProviderProbeTarget.Update().Where(
		sub2apiproviderprobetarget.ID(targetID),
		sub2apiproviderprobetarget.ProviderID(providerID),
	)
	if input.Enabled != nil {
		u.SetEnabled(*input.Enabled)
	}
	if input.IntervalSeconds != nil {
		u.SetIntervalSeconds(*input.IntervalSeconds)
	}
	if input.AllowMediaProbe != nil {
		u.SetAllowMediaProbe(*input.AllowMediaProbe)
	}
	if input.TimeoutSeconds != nil {
		u.SetTimeoutSeconds(*input.TimeoutSeconds)
	}
	if input.DegradedLatencyMS != nil {
		u.SetDegradedLatencyMs(*input.DegradedLatencyMS)
	}
	if input.FailureThreshold != nil {
		u.SetFailureThreshold(*input.FailureThreshold)
	}
	if input.RecoveryThreshold != nil {
		u.SetRecoveryThreshold(*input.RecoveryThreshold)
	}
	if _, err := u.Save(ctx); err != nil {
		return nil, err
	}
	return r.GetTarget(ctx, providerID, targetID)
}

func (r *sub2APIProviderProbeRepository) UpdateTargetBinding(ctx context.Context, targetID int64, providerAPIKeyID, remoteGroupID *int64, remoteGroupName *string, platform string) (*ent.Sub2APIProviderProbeTarget, error) {
	target, err := r.client.Sub2APIProviderProbeTarget.Get(ctx, targetID)
	if err != nil {
		return nil, err
	}
	u := target.Update().SetPlatform(platform)
	if providerAPIKeyID == nil {
		u.ClearProviderAPIKeyID()
	} else {
		u.SetProviderAPIKeyID(*providerAPIKeyID)
	}
	if remoteGroupID == nil {
		u.ClearRemoteGroupID()
	} else {
		u.SetRemoteGroupID(*remoteGroupID)
	}
	if remoteGroupName == nil {
		u.ClearRemoteGroupName()
	} else {
		u.SetRemoteGroupName(*remoteGroupName)
	}
	if !sameOptionalInt64(target.RemoteGroupID, remoteGroupID) {
		u.SetRouteChangedAt(time.Now().UTC())
	}
	return u.Save(ctx)
}

func (r *sub2APIProviderProbeRepository) MarkTargetRun(ctx context.Context, targetID int64, at time.Time) error {
	return r.client.Sub2APIProviderProbeTarget.UpdateOneID(targetID).SetLastRunAt(at.UTC()).Exec(ctx)
}

func (r *sub2APIProviderProbeRepository) CreateTargetRun(ctx context.Context, input *service.Sub2APIProviderProbeTargetRunInput) (*ent.Sub2APIProviderProbeTargetRun, error) {
	b := r.client.Sub2APIProviderProbeTargetRun.Create().
		SetTargetID(input.TargetID).
		SetProviderID(input.ProviderID).
		SetAccountID(input.AccountID).
		SetPlatform(input.Platform).
		SetStatus(sub2apiproviderprobetargetrun.Status(input.Status)).
		SetTrafficRequestCount(input.TrafficRequestCount).
		SetStartedAt(input.StartedAt.UTC()).
		SetFinishedAt(input.FinishedAt.UTC())
	if input.ProviderAPIKeyID != nil {
		b.SetProviderAPIKeyID(*input.ProviderAPIKeyID)
	}
	if input.RemoteGroupID != nil {
		b.SetRemoteGroupID(*input.RemoteGroupID)
	}
	if input.RemoteGroupName != nil {
		b.SetRemoteGroupName(*input.RemoteGroupName)
	}
	if input.ModelID != nil {
		b.SetModelID(*input.ModelID)
	}
	if input.LatencyMS != nil {
		b.SetLatencyMs(*input.LatencyMS)
	}
	if input.TrafficSuccessRate != nil {
		b.SetTrafficSuccessRate(*input.TrafficSuccessRate)
	}
	if input.TrafficP95LatencyMS != nil {
		b.SetTrafficP95LatencyMs(*input.TrafficP95LatencyMS)
	}
	if input.ErrorCategory != nil {
		b.SetErrorCategory(*input.ErrorCategory)
	}
	if input.ErrorMessage != nil {
		b.SetErrorMessage(*input.ErrorMessage)
	}
	return b.Save(ctx)
}

func (r *sub2APIProviderProbeRepository) ListTargetRunsSince(ctx context.Context, targetIDs []int64, since time.Time) ([]*ent.Sub2APIProviderProbeTargetRun, error) {
	if len(targetIDs) == 0 {
		return []*ent.Sub2APIProviderProbeTargetRun{}, nil
	}
	return r.client.Sub2APIProviderProbeTargetRun.Query().Where(
		sub2apiproviderprobetargetrun.TargetIDIn(targetIDs...),
		sub2apiproviderprobetargetrun.CreatedAtGTE(since.UTC()),
	).Order(
		ent.Asc(sub2apiproviderprobetargetrun.FieldTargetID),
		ent.Desc(sub2apiproviderprobetargetrun.FieldCreatedAt),
	).All(ctx)
}

// ListRecentTargetRuns returns at most limitPerTarget real probe events for
// each target. Keeping the limit in the query avoids loading the entire
// retention window for fast-cadence account probes.
func (r *sub2APIProviderProbeRepository) ListRecentTargetRuns(ctx context.Context, targetIDs []int64, since time.Time, limitPerTarget int) ([]*ent.Sub2APIProviderProbeTargetRun, error) {
	if len(targetIDs) == 0 || limitPerTarget <= 0 {
		return []*ent.Sub2APIProviderProbeTargetRun{}, nil
	}
	result := make([]*ent.Sub2APIProviderProbeTargetRun, 0, len(targetIDs)*limitPerTarget)
	seen := make(map[int64]struct{}, len(targetIDs))
	for _, targetID := range targetIDs {
		if _, ok := seen[targetID]; ok {
			continue
		}
		seen[targetID] = struct{}{}
		runs, err := r.client.Sub2APIProviderProbeTargetRun.Query().Where(
			sub2apiproviderprobetargetrun.TargetID(targetID),
			sub2apiproviderprobetargetrun.CreatedAtGTE(since.UTC()),
		).Order(
			ent.Desc(sub2apiproviderprobetargetrun.FieldCreatedAt),
		).Limit(limitPerTarget).All(ctx)
		if err != nil {
			return nil, err
		}
		result = append(result, runs...)
	}
	return result, nil
}

func (r *sub2APIProviderProbeRepository) TargetTrafficStats(ctx context.Context, accountID int64, since time.Time) (*service.Sub2APIProviderProbeTargetTrafficStats, error) {
	if r.db == nil {
		return &service.Sub2APIProviderProbeTargetTrafficStats{}, nil
	}
	const q = `
WITH requests AS (
  SELECT ul.duration_ms::double precision AS latency_ms, TRUE AS success
    FROM usage_logs ul
   WHERE ul.account_id = $1 AND ul.created_at >= $2
  UNION ALL
  SELECT COALESCE(o.response_latency_ms, o.upstream_latency_ms, 0)::double precision, FALSE
    FROM ops_error_logs o
   WHERE o.account_id = $1 AND o.created_at >= $2 AND COALESCE(o.status_code, 0) >= 400
), summary AS (
  SELECT COUNT(*)::int AS total,
         COUNT(*) FILTER (WHERE success)::int AS successes,
         percentile_cont(0.95) WITHIN GROUP (ORDER BY latency_ms) AS p95
    FROM requests
)
SELECT total, successes, p95 FROM summary`
	var total, successes int
	var p95 sql.NullFloat64
	if err := r.db.QueryRowContext(ctx, q, accountID, since.UTC()).Scan(&total, &successes, &p95); err != nil {
		return nil, err
	}
	out := &service.Sub2APIProviderProbeTargetTrafficStats{RequestCount: total}
	if total > 0 {
		out.SuccessRate = float64(successes) / float64(total) * 100
		out.P95LatencyMS = int(p95.Float64)
	}
	return out, nil
}

func sameOptionalInt64(a, b *int64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

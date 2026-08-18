package service

import (
	"context"
	"encoding/json"
	"math"
	"time"
)

const (
	optimizeExecutionDispositionKey = "execution_disposition"
	optimizeExecutionExecuted       = "executed"
	optimizeExecutionCoalesced      = "coalesced"
	optimizeExecutionDeferred       = "deferred"

	// A scheduled run may arrive immediately after a probe/manual run released
	// the Provider lock. Reusing that fresh result avoids testing and temporarily
	// switching the same remote Key again while still allowing the cron run to
	// process every other participating account.
	recentOptimizeCoverageWindow = 5 * time.Minute
	recentOptimizeCoveragePage   = 200
)

type recentOptimizeCoverage struct {
	LogID   int64
	Trigger string
}

// persistOptimizeLog is the single audit writer for cron, immediate, probe,
// and manual optimization. Per-account switch_events preserve every remote
// candidate switch and rollback, while the top-level fields support fast
// filtering and run-level summaries.
func (s *Sub2APIOptimizeScheduleService) persistOptimizeLog(
	ctx context.Context,
	providerID int64,
	scheduleID *int64,
	trigger string,
	startedAt time.Time,
	details []OptimizeAccountDetail,
	extraByAccount map[int64]map[string]any,
) error {
	optimized, skipped, failed := 0, 0, 0
	detailMaps := make([]map[string]any, 0, len(details))
	for _, detail := range details {
		switch detail.Status {
		case "optimized":
			optimized++
		case "skipped":
			skipped++
		default:
			failed++
		}
		item := map[string]any{
			"account_id":                    detail.AccountID,
			"account_name":                  detail.AccountName,
			"status":                        detail.Status,
			"old_group":                     detail.OldGroup,
			"new_group":                     detail.NewGroup,
			"old_multiplier":                detail.OldMult,
			"new_multiplier":                detail.NewMult,
			"reason":                        detail.Reason,
			"trigger":                       trigger,
			optimizeExecutionDispositionKey: optimizeExecutionExecuted,
		}
		if len(detail.SwitchEvents) > 0 {
			item["switch_events"] = detail.SwitchEvents
		}
		for key, value := range extraByAccount[detail.AccountID] {
			item[key] = value
		}
		detailMaps = append(detailMaps, item)
	}

	finishedAt := time.Now()
	_, err := s.scheduleRepo.CreateLog(ctx, &CreateOptimizeLogInput{
		ProviderID: providerID,
		ScheduleID: scheduleID,
		Trigger:    trigger,
		Status:     optimizeRunStatus(len(details), optimized, failed),
		Total:      len(details),
		Optimized:  optimized,
		Skipped:    skipped,
		Failed:     failed,
		Detail:     detailMaps,
		StartedAt:  &startedAt,
		FinishedAt: &finishedAt,
	})
	return err
}

// persistOptimizeDeferredLog records an admitted trigger that deliberately did
// not execute because another run already owns the Provider. This is an audit
// event, not an optimization failure: cron remains due and probe cooldown claims
// are released so uncovered work can be retried.
func (s *Sub2APIOptimizeScheduleService) persistOptimizeDeferredLog(
	ctx context.Context,
	providerID int64,
	scheduleID *int64,
	trigger string,
	startedAt time.Time,
	reason string,
	details []OptimizeAccountDetail,
	extraByAccount map[int64]map[string]any,
) error {
	detailMaps := make([]map[string]any, 0, max(1, len(details)))
	for _, detail := range details {
		item := map[string]any{
			"account_id":                    detail.AccountID,
			"account_name":                  detail.AccountName,
			"status":                        "skipped",
			"reason":                        reason,
			"trigger":                       trigger,
			optimizeExecutionDispositionKey: optimizeExecutionDeferred,
		}
		for key, value := range extraByAccount[detail.AccountID] {
			item[key] = value
		}
		detailMaps = append(detailMaps, item)
	}
	if len(detailMaps) == 0 {
		detailMaps = append(detailMaps, map[string]any{
			"status":                        "skipped",
			"reason":                        reason,
			"trigger":                       trigger,
			optimizeExecutionDispositionKey: optimizeExecutionDeferred,
		})
	}

	finishedAt := time.Now()
	_, err := s.scheduleRepo.CreateLog(ctx, &CreateOptimizeLogInput{
		ProviderID: providerID,
		ScheduleID: scheduleID,
		Trigger:    trigger,
		Status:     "skipped",
		Total:      len(details),
		Skipped:    len(details),
		Detail:     detailMaps,
		StartedAt:  &startedAt,
		FinishedAt: &finishedAt,
	})
	return err
}

// recentCoveredAccounts returns account results that a cron run can safely
// coalesce. Logs are newest-first. The newest executed result for an account is
// authoritative: a newer failure prevents an older success from suppressing a
// retry. A recent cron result is observed but never reused, preserving explicit
// high-frequency cron semantics.
func (s *Sub2APIOptimizeScheduleService) recentCoveredAccounts(
	ctx context.Context,
	providerID int64,
	now time.Time,
) (map[int64]recentOptimizeCoverage, error) {
	from := now.Add(-recentOptimizeCoverageWindow)
	seen := make(map[int64]struct{})
	covered := make(map[int64]recentOptimizeCoverage)

	for page := 1; ; page++ {
		logs, total, err := s.scheduleRepo.ListLogs(ctx, providerID, OptimizeLogFilter{
			From:     &from,
			Page:     page,
			PageSize: recentOptimizeCoveragePage,
		})
		if err != nil {
			return nil, err
		}
		for _, logEntry := range logs {
			for _, detail := range logEntry.Detail {
				if disposition, _ := detail[optimizeExecutionDispositionKey].(string); disposition != optimizeExecutionExecuted {
					continue
				}
				accountID, ok := optimizeLogAccountID(detail["account_id"])
				if !ok {
					continue
				}
				if _, alreadySeen := seen[accountID]; alreadySeen {
					continue
				}
				seen[accountID] = struct{}{}

				status, _ := detail["status"].(string)
				if logEntry.Trigger != OptimizeLogTriggerCron && (status == "optimized" || status == "skipped") {
					covered[accountID] = recentOptimizeCoverage{LogID: logEntry.ID, Trigger: logEntry.Trigger}
				}
			}
		}
		if len(logs) == 0 || int64(page*recentOptimizeCoveragePage) >= total {
			break
		}
	}
	return covered, nil
}

func optimizeLogAccountID(value any) (int64, bool) {
	var accountID int64
	switch typed := value.(type) {
	case int:
		accountID = int64(typed)
	case int64:
		accountID = typed
	case float64:
		if math.Trunc(typed) != typed || typed > math.MaxInt64 || typed < math.MinInt64 {
			return 0, false
		}
		accountID = int64(typed)
	case json.Number:
		parsed, err := typed.Int64()
		if err != nil {
			return 0, false
		}
		accountID = parsed
	default:
		return 0, false
	}
	return accountID, accountID > 0
}

func (s *Sub2APIOptimizeScheduleService) persistOptimizeFailureLog(
	ctx context.Context,
	providerID int64,
	scheduleID *int64,
	trigger string,
	startedAt time.Time,
	reason string,
) error {
	finishedAt := time.Now()
	_, err := s.scheduleRepo.CreateLog(ctx, &CreateOptimizeLogInput{
		ProviderID: providerID,
		ScheduleID: scheduleID,
		Trigger:    trigger,
		Status:     "failed",
		Detail:     []map[string]any{{"reason": reason, "trigger": trigger}},
		StartedAt:  &startedAt,
		FinishedAt: &finishedAt,
	})
	return err
}

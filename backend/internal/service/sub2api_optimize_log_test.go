//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/ent"
)

type optimizeLogRepositoryStub struct {
	created          []*CreateOptimizeLogInput
	listedProviderID int64
	listedFilter     OptimizeLogFilter
	logs             []*ent.Sub2APIOptimizeLog
	total            int64
}

func (r *optimizeLogRepositoryStub) GetByProviderID(context.Context, int64) (*ent.Sub2APIOptimizeSchedule, error) {
	return nil, nil
}
func (r *optimizeLogRepositoryStub) Upsert(context.Context, *UpsertOptimizeScheduleInput) (*ent.Sub2APIOptimizeSchedule, error) {
	return nil, nil
}
func (r *optimizeLogRepositoryStub) UpdateRunTimes(context.Context, int64, time.Time, time.Time) error {
	return nil
}
func (r *optimizeLogRepositoryStub) Delete(context.Context, int64) error { return nil }
func (r *optimizeLogRepositoryStub) ListEnabled(context.Context) ([]*ent.Sub2APIOptimizeSchedule, error) {
	return nil, nil
}
func (r *optimizeLogRepositoryStub) ListDue(context.Context, time.Time) ([]*ent.Sub2APIOptimizeSchedule, error) {
	return nil, nil
}
func (r *optimizeLogRepositoryStub) CreateLog(_ context.Context, input *CreateOptimizeLogInput) (*ent.Sub2APIOptimizeLog, error) {
	copyInput := *input
	r.created = append(r.created, &copyInput)
	return &ent.Sub2APIOptimizeLog{ID: int64(len(r.created))}, nil
}
func (r *optimizeLogRepositoryStub) ListLogs(_ context.Context, providerID int64, filter OptimizeLogFilter) ([]*ent.Sub2APIOptimizeLog, int64, error) {
	r.listedProviderID = providerID
	r.listedFilter = filter
	return r.logs, r.total, nil
}

func TestPersistOptimizeLogPreservesEverySwitchEvent(t *testing.T) {
	repo := &optimizeLogRepositoryStub{}
	svc := &Sub2APIOptimizeScheduleService{scheduleRepo: repo}
	startedAt := time.Now().Add(-time.Second)
	scheduleID := int64(9)
	details := []OptimizeAccountDetail{{
		AccountID:   42,
		AccountName: "openai-primary",
		Status:      "optimized",
		OldGroup:    "economy",
		NewGroup:    "standard",
		SwitchEvents: []OptimizeGroupSwitchEvent{
			{Action: "switch", FromGroup: "economy", ToGroup: "discount", Status: "success", TestStatus: "failed", OccurredAt: time.Now().Format(time.RFC3339Nano)},
			{Action: "rollback", FromGroup: "discount", ToGroup: "economy", Status: "success", OccurredAt: time.Now().Format(time.RFC3339Nano)},
			{Action: "switch", FromGroup: "economy", ToGroup: "standard", Status: "success", TestStatus: "passed", OccurredAt: time.Now().Format(time.RFC3339Nano)},
		},
	}}

	err := svc.persistOptimizeLog(context.Background(), 7, &scheduleID, OptimizeLogTriggerScheduleNow, startedAt, details, nil)
	if err != nil {
		t.Fatalf("persist optimize log: %v", err)
	}
	if len(repo.created) != 1 {
		t.Fatalf("created logs=%d, want 1", len(repo.created))
	}
	created := repo.created[0]
	if created.ProviderID != 7 || created.ScheduleID == nil || *created.ScheduleID != scheduleID || created.Trigger != OptimizeLogTriggerScheduleNow {
		t.Fatalf("unexpected audit ownership: %+v", created)
	}
	if created.Status != "success" || created.Total != 1 || created.Optimized != 1 {
		t.Fatalf("unexpected summary: %+v", created)
	}
	events, ok := created.Detail[0]["switch_events"].([]OptimizeGroupSwitchEvent)
	if !ok || len(events) != 3 {
		t.Fatalf("switch events=%#v, want all 3 ordered events", created.Detail[0]["switch_events"])
	}
	if events[0].ToGroup != "discount" || events[1].Action != "rollback" || events[2].ToGroup != "standard" {
		t.Fatalf("switch event order changed: %#v", events)
	}
	if got := created.Detail[0][optimizeExecutionDispositionKey]; got != optimizeExecutionExecuted {
		t.Fatalf("execution disposition=%v, want %q", got, optimizeExecutionExecuted)
	}
}

func TestPersistOptimizeDeferredLogIsAuditableWithoutExecuting(t *testing.T) {
	repo := &optimizeLogRepositoryStub{}
	svc := &Sub2APIOptimizeScheduleService{scheduleRepo: repo}
	scheduleID := int64(9)
	err := svc.persistOptimizeDeferredLog(
		context.Background(),
		7,
		&scheduleID,
		OptimizeLogTriggerCron,
		time.Now(),
		"provider optimization already running",
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("persist deferred optimize log: %v", err)
	}
	if len(repo.created) != 1 {
		t.Fatalf("created logs=%d, want 1", len(repo.created))
	}
	created := repo.created[0]
	if created.Status != "skipped" || created.Trigger != OptimizeLogTriggerCron || created.Total != 0 {
		t.Fatalf("unexpected deferred summary: %+v", created)
	}
	if got := created.Detail[0][optimizeExecutionDispositionKey]; got != optimizeExecutionDeferred {
		t.Fatalf("execution disposition=%v, want %q", got, optimizeExecutionDeferred)
	}
	if got := created.Detail[0]["reason"]; got != "provider optimization already running" {
		t.Fatalf("deferred reason=%v", got)
	}
}

func TestRecentCoveredAccountsUsesNewestExecutedResult(t *testing.T) {
	now := time.Now()
	repo := &optimizeLogRepositoryStub{
		logs: []*ent.Sub2APIOptimizeLog{
			{
				ID: 4, ProviderID: 7, Trigger: OptimizeLogTriggerProbeUnhealthy, CreatedAt: now,
				Detail: []map[string]any{
					{"account_id": float64(42), "status": "failed", optimizeExecutionDispositionKey: optimizeExecutionExecuted},
					{"account_id": float64(43), "status": "skipped", optimizeExecutionDispositionKey: optimizeExecutionExecuted},
				},
			},
			{
				ID: 3, ProviderID: 7, Trigger: OptimizeLogTriggerProbeUnhealthy, CreatedAt: now.Add(-time.Minute),
				Detail: []map[string]any{
					{"account_id": float64(42), "status": "optimized", optimizeExecutionDispositionKey: optimizeExecutionExecuted},
					{"account_id": float64(44), "status": "skipped", optimizeExecutionDispositionKey: optimizeExecutionDeferred},
				},
			},
			{
				ID: 2, ProviderID: 7, Trigger: OptimizeLogTriggerManualAccount, CreatedAt: now.Add(-2 * time.Minute),
				Detail: []map[string]any{{"account_id": int64(44), "status": "optimized", optimizeExecutionDispositionKey: optimizeExecutionExecuted}},
			},
			{
				ID: 1, ProviderID: 7, Trigger: OptimizeLogTriggerCron, CreatedAt: now.Add(-3 * time.Minute),
				Detail: []map[string]any{{"account_id": int64(45), "status": "optimized", optimizeExecutionDispositionKey: optimizeExecutionExecuted}},
			},
		},
		total: 4,
	}
	svc := &Sub2APIOptimizeScheduleService{scheduleRepo: repo}

	covered, err := svc.recentCoveredAccounts(context.Background(), 7, now)
	if err != nil {
		t.Fatalf("recent covered accounts: %v", err)
	}
	if _, ok := covered[42]; ok {
		t.Fatal("newest failed result must not be covered by an older success")
	}
	if got := covered[43]; got.LogID != 4 || got.Trigger != OptimizeLogTriggerProbeUnhealthy {
		t.Fatalf("account 43 coverage=%+v", got)
	}
	if got := covered[44]; got.LogID != 2 || got.Trigger != OptimizeLogTriggerManualAccount {
		t.Fatalf("account 44 coverage=%+v", got)
	}
	if _, ok := covered[45]; ok {
		t.Fatal("a previous cron result must not suppress a later explicit cron cycle")
	}
}

func TestCoalesceRecentOptimizeCoverageOnlySkipsOverlappingAccounts(t *testing.T) {
	groupName := "stable"
	groupMultiplier := 0.8
	coveredAccount := completeOptimizeAccount()
	coveredAccount.ID = 42
	coveredAccount.Name = "covered"
	coveredAccount.RemoteGroupName = &groupName
	coveredAccount.RemoteGroupMultiplier = &groupMultiplier
	pendingAccount := completeOptimizeAccount()
	pendingAccount.ID = 43
	pendingAccount.Name = "pending"

	pending, coalesced, extras := coalesceRecentOptimizeCoverage(
		[]Account{coveredAccount, pendingAccount},
		map[int64]recentOptimizeCoverage{42: {LogID: 11, Trigger: OptimizeLogTriggerProbeUnhealthy}},
	)
	if len(pending) != 1 || pending[0].ID != 43 {
		t.Fatalf("pending=%+v, want only account 43", pending)
	}
	if len(coalesced) != 1 || coalesced[0].AccountID != 42 || coalesced[0].Status != "skipped" {
		t.Fatalf("coalesced=%+v, want only account 42", coalesced)
	}
	if coalesced[0].OldGroup != groupName || coalesced[0].NewMult != groupMultiplier {
		t.Fatalf("coalesced group binding=%+v", coalesced[0])
	}
	if got := extras[42][optimizeExecutionDispositionKey]; got != optimizeExecutionCoalesced {
		t.Fatalf("coalesced disposition=%v", got)
	}
	if got := extras[42]["coalesced_from_log_id"]; got != int64(11) {
		t.Fatalf("coalesced source log=%v", got)
	}
}

func TestRunOptimizeLockContentionWritesDeferredAudit(t *testing.T) {
	repo := &optimizeLogRepositoryStub{}
	svc := &Sub2APIOptimizeScheduleService{
		scheduleRepo: repo,
		running:      map[int64]bool{7: true},
	}
	if err := svc.RunOptimize(context.Background(), 9, 7); err != nil {
		t.Fatalf("run optimize contention: %v", err)
	}
	if len(repo.created) != 1 {
		t.Fatalf("created logs=%d, want 1", len(repo.created))
	}
	created := repo.created[0]
	if created.Trigger != OptimizeLogTriggerCron || created.Status != "skipped" || created.ScheduleID == nil || *created.ScheduleID != 9 {
		t.Fatalf("unexpected deferred cron audit: %+v", created)
	}
}

func TestProbeOptimizeLogDoesNotRequireSchedule(t *testing.T) {
	repo := &optimizeLogRepositoryStub{}
	svc := &Sub2APIOptimizeScheduleService{scheduleRepo: repo}
	svc.finishProbeAutoOptimize(7, time.Now(), []OptimizeAccountDetail{{
		AccountID: 42,
		Status:    "failed",
		Reason:    "no candidate passed",
	}}, map[int64]Sub2APIProbeAutoOptimizeInput{
		42: {TargetID: 13, ProbeRunID: 21, AccountID: 42, FailureThreshold: 3},
	})

	if len(repo.created) != 1 {
		t.Fatalf("created logs=%d, want 1 without a schedule", len(repo.created))
	}
	created := repo.created[0]
	if created.ScheduleID != nil || created.Trigger != OptimizeLogTriggerProbeUnhealthy || created.ProviderID != 7 {
		t.Fatalf("unexpected probe audit ownership: %+v", created)
	}
	if got := created.Detail[0]["probe_target_id"]; got != int64(13) {
		t.Fatalf("probe target id=%v, want 13", got)
	}
}

func TestListOptimizeLogsKeepsProviderScopeAndPagination(t *testing.T) {
	now := time.Now()
	repo := &optimizeLogRepositoryStub{
		logs: []*ent.Sub2APIOptimizeLog{{
			ID: 1, ProviderID: 7, Trigger: OptimizeLogTriggerManualAll,
			Status: "success", CreatedAt: now,
		}},
		total: 23,
	}
	svc := &Sub2APIOptimizeScheduleService{scheduleRepo: repo}
	accountID := int64(42)
	items, total, err := svc.ListLogs(context.Background(), 7, OptimizeLogFilter{
		Trigger: OptimizeLogTriggerManualAll, Status: "success", AccountID: &accountID,
		Keyword: " standard ", Page: 2, PageSize: 10,
	})
	if err != nil {
		t.Fatalf("list logs: %v", err)
	}
	if repo.listedProviderID != 7 || repo.listedFilter.Page != 2 || repo.listedFilter.PageSize != 10 || repo.listedFilter.Keyword != "standard" {
		t.Fatalf("unexpected repository filter: provider=%d filter=%+v", repo.listedProviderID, repo.listedFilter)
	}
	if total != 23 || len(items) != 1 || items[0].ProviderID != 7 || items[0].Trigger != OptimizeLogTriggerManualAll {
		t.Fatalf("unexpected response: total=%d items=%+v", total, items)
	}
}

func TestListOptimizeLogsRejectsInvalidFilters(t *testing.T) {
	svc := &Sub2APIOptimizeScheduleService{scheduleRepo: &optimizeLogRepositoryStub{}}
	for name, filter := range map[string]OptimizeLogFilter{
		"trigger": {Trigger: "unknown"},
		"status":  {Status: "unknown"},
		"range":   {From: optimizeTimePtr(time.Now()), To: optimizeTimePtr(time.Now().Add(-time.Hour))},
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, err := svc.ListLogs(context.Background(), 7, filter); err == nil {
				t.Fatal("expected invalid filter error")
			}
		})
	}
}

func optimizeTimePtr(value time.Time) *time.Time { return &value }

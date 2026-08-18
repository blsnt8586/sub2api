//go:build unit

package repository

import (
	"context"
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestDeleteUpstreamLogsBeforeDeletesEachLogTableWithStrictCutoff(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer func() { _ = db.Close() }()

	cutoff := time.Date(2026, 8, 14, 6, 30, 0, 0, time.UTC)
	tables := []struct {
		name string
		rows int64
	}{
		{name: "sub2api_provider_probe_runs", rows: 2},
		{name: "sub2api_provider_probe_target_runs", rows: 3},
		{name: "sub2api_optimize_logs", rows: 4},
	}
	for _, table := range tables {
		query := fmt.Sprintf(deleteExpiredSub2APIUpstreamLogBatchSQL, table.name, table.name)
		mock.ExpectExec(regexp.QuoteMeta(query)).
			WithArgs(cutoff, 1000).
			WillReturnResult(sqlmock.NewResult(0, table.rows))
	}

	repo := &sub2APIProviderProbeRepository{db: db}
	got, err := repo.DeleteUpstreamLogsBefore(context.Background(), cutoff, 1000)
	if err != nil {
		t.Fatalf("delete upstream logs: %v", err)
	}
	if got.ProviderProbeRuns != 2 || got.AccountProbeRuns != 3 || got.OptimizeLogs != 4 {
		t.Fatalf("deleted=%+v", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestDeleteUpstreamLogsBeforeRejectsMissingDatabaseAndInvalidBatch(t *testing.T) {
	ctx := context.Background()
	if _, err := (&sub2APIProviderProbeRepository{}).DeleteUpstreamLogsBefore(ctx, time.Now(), 1000); err == nil {
		t.Fatal("expected nil database error")
	}

	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer func() { _ = db.Close() }()
	if _, err := (&sub2APIProviderProbeRepository{db: db}).DeleteUpstreamLogsBefore(ctx, time.Now(), 0); err == nil {
		t.Fatal("expected invalid batch size error")
	}
}

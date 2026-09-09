//go:build unit

package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestCompleteSmartGroupSwitchWritesHistoryAndPrunesInOneTransaction(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	repo := &apiKeySmartGroupRepository{db: db}
	now := time.Date(2026, 9, 5, 15, 0, 0, 0, time.UTC)
	leaseUntil := now.Add(3 * time.Minute)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT k.group_id")).
		WithArgs(int64(7), int64(11), int64(12), "failure", now, leaseUntil).
		WillReturnRows(sqlmock.NewRows([]string{"group_id", "from_group_name", "to_group_name"}).AddRow(int64(11), "Expensive", "Economy"))
	mock.ExpectQuery(regexp.QuoteMeta("UPDATE api_keys")).
		WithArgs(int64(7), int64(11), int64(12), "failure", now, leaseUntil).
		WillReturnRows(sqlmock.NewRows([]string{"key"}).AddRow("sk-test"))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO api_key_smart_group_switch_logs")).
		WithArgs(int64(7), int64(11), "Expensive", int64(12), "Economy", "failure", now).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM api_key_smart_group_switch_logs")).
		WithArgs(now.Add(-72 * time.Hour)).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectCommit()

	key, switched, err := repo.CompleteSmartGroupSwitch(context.Background(), 7, 11, 12, "failure", leaseUntil, now)
	if err != nil {
		t.Fatal(err)
	}
	if !switched || key != "sk-test" {
		t.Fatalf("unexpected result: key=%q switched=%v", key, switched)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCompleteSmartGroupSwitchDoesNotWriteHistoryWhenRouteIsNoLongerEligible(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	repo := &apiKeySmartGroupRepository{db: db}
	now := time.Date(2026, 9, 5, 15, 0, 0, 0, time.UTC)
	leaseUntil := now.Add(3 * time.Minute)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT k.group_id")).
		WithArgs(int64(7), int64(11), int64(12), "failure", now, leaseUntil).
		WillReturnRows(sqlmock.NewRows([]string{"group_id", "from_group_name", "to_group_name"}))
	mock.ExpectRollback()

	_, switched, err := repo.CompleteSmartGroupSwitch(context.Background(), 7, 11, 12, "failure", leaseUntil, now)
	if err != nil || switched {
		t.Fatalf("expected no-op when route is no longer eligible, switched=%v err=%v", switched, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCompleteSmartGroupFailureSwitchIsAtomicAndDoesNotProbe(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	repo := &apiKeySmartGroupRepository{db: db}
	now := time.Date(2026, 9, 6, 15, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COALESCE(from_group.name")).
		WithArgs(int64(7), int64(11), int64(12)).
		WillReturnRows(sqlmock.NewRows([]string{"from_group_name", "to_group_name"}).AddRow("Broken", "Backup"))
	mock.ExpectQuery(regexp.QuoteMeta("UPDATE api_keys")).
		WithArgs(int64(7), int64(11), int64(12), now).
		WillReturnRows(sqlmock.NewRows([]string{"key"}).AddRow("sk-test"))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO api_key_smart_group_switch_logs")).
		WithArgs(int64(7), int64(11), "Broken", int64(12), "Backup", now).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM api_key_smart_group_switch_logs")).
		WithArgs(now.Add(-72 * time.Hour)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	key, switched, err := repo.CompleteSmartGroupFailureSwitch(context.Background(), 7, 11, 12, now)
	if err != nil {
		t.Fatal(err)
	}
	if !switched || key != "sk-test" {
		t.Fatalf("unexpected result: key=%q switched=%v", key, switched)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

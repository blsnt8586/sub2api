//go:build unit

package repository

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAdminProbeOwnershipReleasePayloadAdvancesGeneration(t *testing.T) {
	first, err := adminProbeOwnershipReleasePayload()
	require.NoError(t, err)
	second, err := adminProbeOwnershipReleasePayload()
	require.NoError(t, err)

	var firstPayload, secondPayload map[string]any
	require.NoError(t, json.Unmarshal([]byte(first), &firstPayload))
	require.NoError(t, json.Unmarshal([]byte(second), &secondPayload))
	firstGeneration, _ := firstPayload[service.Sub2APIAdminAccountStateGenerationExtraKey].(string)
	secondGeneration, _ := secondPayload[service.Sub2APIAdminAccountStateGenerationExtraKey].(string)
	require.NotEmpty(t, firstGeneration)
	require.NotEmpty(t, secondGeneration)
	require.NotEqual(t, firstGeneration, secondGeneration)
	require.Equal(t, false, firstPayload[service.Sub2APIProbeManagedAccountStatusExtraKey])
	require.Equal(t, false, firstPayload[service.Sub2APIRuntimeRecoverableAccountErrorExtraKey])
}

func TestAccountRepositorySetProbeManagedErrorIsOneConditionalWrite(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec(`(?s)WITH updated AS \(.*UPDATE accounts AS a.*a.status = \$5 AND a.schedulable IS TRUE.*@> \$6::jsonb.*@> \$7::jsonb.*a.extra ->> \$8::text.*INSERT INTO scheduler_outbox`).
		WillReturnResult(sqlmock.NewResult(0, 1))

	repo := &accountRepository{sql: db}
	updated, err := repo.SetProbeManagedError(context.Background(), 17, "probe failed", true, "admin-generation")
	require.NoError(t, err)
	require.True(t, updated)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAccountRepositorySetProbeManagedErrorLosesRaceWithoutOverwriting(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec(`(?s)WITH updated AS \(.*UPDATE accounts AS a`).
		WillReturnResult(sqlmock.NewResult(0, 0))

	repo := &accountRepository{sql: db}
	updated, err := repo.SetProbeManagedError(context.Background(), 17, "stale probe", false, "old-generation")
	require.NoError(t, err)
	require.False(t, updated)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAccountRepositoryRecoverProbeManagedAccountRequiresOwnership(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec(`(?s)WITH updated AS \(.*SET status = \$1.*a.status = \$4.*@> \$5::jsonb.*@> \$6::jsonb.*a.extra ->> \$7::text.*INSERT INTO scheduler_outbox`).
		WillReturnResult(sqlmock.NewResult(0, 1))

	repo := &accountRepository{sql: db}
	updated, err := repo.RecoverProbeManagedAccount(context.Background(), 17, "admin-generation")
	require.NoError(t, err)
	require.True(t, updated)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAccountRepositorySetRuntimeRecoverableErrorIsConditionalAndAtomic(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec(`(?s)WITH updated AS \(.*a.status = \$5 AND a.schedulable IS TRUE.*@> \$6::jsonb.*a.extra ->> \$7::text.*INSERT INTO scheduler_outbox`).
		WillReturnResult(sqlmock.NewResult(0, 1))

	repo := &accountRepository{sql: db}
	updated, err := repo.SetRuntimeRecoverableAccountError(context.Background(), 17, "upstream auth failed", "admin-generation")
	require.NoError(t, err)
	require.True(t, updated)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAccountRepositoryAdminStateWritesReleaseProbeOwnershipAtomically(t *testing.T) {
	tests := []struct {
		name string
		run  func(*accountRepository) error
	}{
		{name: "set error", run: func(repo *accountRepository) error {
			return repo.SetAdminAccountError(context.Background(), 17, "manual error")
		}},
		{name: "clear error", run: func(repo *accountRepository) error {
			return repo.ClearAdminAccountError(context.Background(), 17)
		}},
		{name: "set schedulable", run: func(repo *accountRepository) error {
			return repo.SetAdminAccountSchedulable(context.Background(), 17, false)
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			mock.ExpectExec(`(?s)WITH updated AS \(.*extra = COALESCE\(a.extra, '\{\}'::jsonb\) \|\|.*INSERT INTO scheduler_outbox`).
				WillReturnResult(sqlmock.NewResult(0, 1))

			require.NoError(t, tc.run(&accountRepository{sql: db}))
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestAccountRepositoryBulkUpdateUsesPerRowProbeOwnershipGuards(t *testing.T) {
	exec := &recordingSQLExecutor{result: rowsAffectedResult(1)}
	repo := newAccountRepositoryWithSQL(nil, exec, nil)
	status := "inactive"

	_, err := repo.BulkUpdate(context.Background(), []int64{17}, service.AccountBulkUpdate{
		Status:                            &status,
		ReleaseProbeOwnershipOnNormalStop: true,
	})

	require.NoError(t, err)
	require.GreaterOrEqual(t, len(exec.execQueries), 1)
	query := normalizeSQLWhitespace(exec.execQueries[0])
	require.Contains(t, query, "status = CASE WHEN status = 'error'")
	require.Contains(t, query, "sub2api_probe_managed_account_status")
	require.Contains(t, query, "sub2api_runtime_recoverable_account_error")
	require.Contains(t, query, "CASE WHEN (status = 'active') THEN")
}

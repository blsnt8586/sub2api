//go:build unit

package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestSub2APIAccountRepositoryReturnsNotFoundForStaleWrites(t *testing.T) {
	tests := []struct {
		name       string
		expectExec string
		run        func(*sub2APIAccountRepository) error
	}{
		{name: "link", expectExec: "extra = COALESCE\\(extra, '\\{\\}'::jsonb\\) - 'sub2api_optimize_group_ids'", run: func(repo *sub2APIAccountRepository) error {
			return repo.UpdateProviderLink(context.Background(), 11, 21, 31)
		}},
		{name: "unlink", expectExec: "extra = COALESCE\\(extra, '\\{\\}'::jsonb\\) - 'sub2api_optimize_group_ids'", run: func(repo *sub2APIAccountRepository) error {
			return repo.ClearProviderLink(context.Background(), 11, 21)
		}},
		{name: "remote binding", expectExec: "UPDATE accounts", run: func(repo *sub2APIAccountRepository) error {
			return repo.UpdateRemoteGroupBinding(context.Background(), 11, 41, "group", 0.1)
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			mock.ExpectExec(tc.expectExec).WillReturnResult(sqlmock.NewResult(0, 0))

			err = tc.run(&sub2APIAccountRepository{sql: db})
			require.True(t, errors.Is(err, sql.ErrNoRows), "error=%v", err)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestSub2APIAccountRepositoryIdentityOnlyClearsUnknownGroupCache(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec("remote_group_name = NULL, remote_group_multiplier = NULL").
		WithArgs(int64(41), int64(11)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	repo := &sub2APIAccountRepository{sql: db}
	require.NoError(t, repo.UpdateRemoteGroupIdentity(context.Background(), 11, 41))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSub2APIAccountRepositoryClearRemoteGroupBinding(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec("remote_group_id = NULL, remote_group_name = NULL, remote_group_multiplier = NULL").
		WithArgs(int64(11)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	repo := &sub2APIAccountRepository{sql: db}
	require.NoError(t, repo.ClearRemoteGroupBinding(context.Background(), 11))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSub2APIAccountRepositoryPersistsOptionalOptimizeGroup(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	minMultiplier, maxMultiplier := 0.05, 0.16
	testModel := "gpt-5.6-sol"
	groupID := int64(41)
	mock.ExpectExec("sub2api_optimize_group_id = \\$5").
		WithArgs(true, minMultiplier, maxMultiplier, testModel, groupID, int64(11), int64(21)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	repo := &sub2APIAccountRepository{sql: db}
	require.NoError(t, repo.UpdateSub2APIOptimizeSettings(
		context.Background(), 21, 11, true,
		&minMultiplier, &maxMultiplier, &testModel, &groupID,
	))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSub2APIAccountRepositoryPersistsOptimizeGroupIDsInExtra(t *testing.T) {
	tests := []struct {
		name       string
		groupIDs   []int64
		wantLegacy any
		wantExtra  any
	}{
		{
			name:       "multiple groups are normalized and persisted",
			groupIDs:   []int64{41, 7, 41, 0, -1},
			wantLegacy: int64(7),
			wantExtra:  "[7,41]",
		},
		{
			name:       "empty selection removes persisted restriction",
			groupIDs:   []int64{},
			wantLegacy: nil,
			wantExtra:  nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			minMultiplier, maxMultiplier := 0.05, 0.16
			testModel := "gpt-5.6-sol"
			mock.ExpectExec("sub2api_optimize_group_id = \\$5").
				WithArgs(true, minMultiplier, maxMultiplier, testModel, tc.wantLegacy, int64(11), int64(21), tc.wantExtra).
				WillReturnResult(sqlmock.NewResult(0, 1))

			repo := &sub2APIAccountRepository{sql: db}
			require.NoError(t, repo.UpdateSub2APIOptimizeSettingsWithGroupIDs(
				context.Background(), 21, 11, true,
				&minMultiplier, &maxMultiplier, &testModel, tc.groupIDs,
			))
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

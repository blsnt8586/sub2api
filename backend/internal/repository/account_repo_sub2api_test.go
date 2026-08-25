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
		name string
		run  func(*sub2APIAccountRepository) error
	}{
		{name: "link", run: func(repo *sub2APIAccountRepository) error {
			return repo.UpdateProviderLink(context.Background(), 11, 21, 31)
		}},
		{name: "unlink", run: func(repo *sub2APIAccountRepository) error {
			return repo.ClearProviderLink(context.Background(), 11, 21)
		}},
		{name: "remote binding", run: func(repo *sub2APIAccountRepository) error {
			return repo.UpdateRemoteGroupBinding(context.Background(), 11, 41, "group", 0.1)
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			mock.ExpectExec("UPDATE accounts").WillReturnResult(sqlmock.NewResult(0, 0))

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

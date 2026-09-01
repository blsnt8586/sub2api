//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type accountRepoStubForClearAccountError struct {
	mockAccountRepoForGemini
	account                  *Account
	clearErrorCalls          int
	clearRateLimitCalls      int
	clearAntigravityCalls    int
	clearModelRateLimitCalls int
	clearTempUnschedCalls    int
}

type atomicAdminAccountStateRepoStub struct {
	accountRepoStubForClearAccountError
	setAdminErrorCalls       int
	clearAdminErrorCalls     int
	setAdminSchedulableCalls int
}

func (r *atomicAdminAccountStateRepoStub) SetAdminAccountError(_ context.Context, _ int64, message string) error {
	r.setAdminErrorCalls++
	r.account.Status = StatusError
	r.account.Schedulable = false
	r.account.ErrorMessage = message
	return nil
}

func (r *atomicAdminAccountStateRepoStub) ClearAdminAccountError(context.Context, int64) error {
	r.clearAdminErrorCalls++
	r.account.Status = StatusActive
	r.account.Schedulable = true
	r.account.ErrorMessage = ""
	return nil
}

func (r *atomicAdminAccountStateRepoStub) SetAdminAccountSchedulable(_ context.Context, _ int64, schedulable bool) error {
	r.setAdminSchedulableCalls++
	r.account.Schedulable = schedulable
	return nil
}

func (r *accountRepoStubForClearAccountError) GetByID(ctx context.Context, id int64) (*Account, error) {
	return r.account, nil
}

func (r *accountRepoStubForClearAccountError) ClearError(ctx context.Context, id int64) error {
	r.clearErrorCalls++
	r.account.Status = StatusActive
	r.account.ErrorMessage = ""
	return nil
}

func (r *accountRepoStubForClearAccountError) ClearRateLimit(ctx context.Context, id int64) error {
	r.clearRateLimitCalls++
	r.account.RateLimitedAt = nil
	r.account.RateLimitResetAt = nil
	return nil
}

func (r *accountRepoStubForClearAccountError) ClearAntigravityQuotaScopes(ctx context.Context, id int64) error {
	r.clearAntigravityCalls++
	return nil
}

func (r *accountRepoStubForClearAccountError) ClearModelRateLimits(ctx context.Context, id int64) error {
	r.clearModelRateLimitCalls++
	return nil
}

func (r *accountRepoStubForClearAccountError) ClearTempUnschedulable(ctx context.Context, id int64) error {
	r.clearTempUnschedCalls++
	r.account.TempUnschedulableUntil = nil
	r.account.TempUnschedulableReason = ""
	return nil
}

func TestAdminService_ClearAccountError_AlsoClearsRecoverableRuntimeState(t *testing.T) {
	until := time.Now().Add(10 * time.Minute)
	resetAt := time.Now().Add(5 * time.Minute)
	repo := &accountRepoStubForClearAccountError{
		account: &Account{
			ID:                      31,
			Platform:                PlatformOpenAI,
			Type:                    AccountTypeOAuth,
			Status:                  StatusError,
			ErrorMessage:            "refresh failed",
			RateLimitResetAt:        &resetAt,
			TempUnschedulableUntil:  &until,
			TempUnschedulableReason: "missing refresh token",
		},
	}
	blocker := &runtimeBlockRecorder{}
	svc := &adminServiceImpl{accountRepo: repo, runtimeBlocker: blocker}

	updated, err := svc.ClearAccountError(context.Background(), 31)
	require.NoError(t, err)
	require.NotNil(t, updated)
	require.Equal(t, 1, repo.clearErrorCalls)
	require.Equal(t, 1, repo.clearRateLimitCalls)
	require.Equal(t, 1, repo.clearAntigravityCalls)
	require.Equal(t, 1, repo.clearModelRateLimitCalls)
	require.Equal(t, 1, repo.clearTempUnschedCalls)
	require.Nil(t, updated.RateLimitResetAt)
	require.Nil(t, updated.TempUnschedulableUntil)
	require.Empty(t, updated.TempUnschedulableReason)
	require.Equal(t, []int64{31}, blocker.clearedIDs)
}

func TestAdminServiceAccountStateChangesUseAtomicRepository(t *testing.T) {
	repo := &atomicAdminAccountStateRepoStub{accountRepoStubForClearAccountError: accountRepoStubForClearAccountError{
		account: &Account{ID: 31, Status: StatusError, Schedulable: false},
	}}
	svc := &adminServiceImpl{accountRepo: repo}

	if _, err := svc.ClearAccountError(context.Background(), 31); err != nil {
		t.Fatalf("clear account error: %v", err)
	}
	if repo.clearAdminErrorCalls != 1 || repo.clearErrorCalls != 0 {
		t.Fatalf("clear path calls: atomic=%d legacy=%d", repo.clearAdminErrorCalls, repo.clearErrorCalls)
	}

	if err := svc.SetAccountError(context.Background(), 31, "manual error"); err != nil {
		t.Fatalf("set account error: %v", err)
	}
	if repo.setAdminErrorCalls != 1 || repo.account.ErrorMessage != "manual error" {
		t.Fatalf("set error path: calls=%d account=%+v", repo.setAdminErrorCalls, repo.account)
	}

	if _, err := svc.SetAccountSchedulable(context.Background(), 31, true); err != nil {
		t.Fatalf("set account schedulable: %v", err)
	}
	if repo.setAdminSchedulableCalls != 1 || !repo.account.Schedulable {
		t.Fatalf("set schedulable path: calls=%d account=%+v", repo.setAdminSchedulableCalls, repo.account)
	}
}

package service

import (
	"context"

	"github.com/google/uuid"
)

const (
	Sub2APIProbeManagedAccountStatusExtraKey      = "sub2api_probe_managed_account_status"
	Sub2APIRuntimeRecoverableAccountErrorExtraKey = "sub2api_runtime_recoverable_account_error"
	Sub2APIProbeGroupsExhaustedExtraKey           = "sub2api_probe_groups_exhausted"
	Sub2APIProbeGroupsExhaustedMessageExtraKey    = "sub2api_probe_groups_exhausted_message"
	Sub2APIAdminAccountStateGenerationExtraKey    = "sub2api_admin_account_state_generation"
)

// ProbeManagedAccountStateRepository owns the atomic Account-row transitions
// used by provider probes. Implementations must update the status, ownership
// markers, and scheduler outbox as one database statement.
type ProbeManagedAccountStateRepository interface {
	SetProbeManagedError(ctx context.Context, accountID int64, message string, groupsExhausted bool, expectedAdminGeneration string) (bool, error)
	RecoverProbeManagedAccount(ctx context.Context, accountID int64, expectedAdminGeneration string) (bool, error)
}

// RuntimeRecoverableAccountStateRepository atomically records an upstream
// request error that a later successful provider probe is allowed to recover.
type RuntimeRecoverableAccountStateRepository interface {
	SetRuntimeRecoverableAccountError(ctx context.Context, accountID int64, message, expectedAdminGeneration string) (bool, error)
}

func setRuntimeRecoverableAccountError(ctx context.Context, repo AccountRepository, account *Account, message string) error {
	if account == nil {
		return nil
	}
	if atomicRepo, ok := repo.(RuntimeRecoverableAccountStateRepository); ok {
		_, err := atomicRepo.SetRuntimeRecoverableAccountError(ctx, account.ID, message, sub2APIAdminAccountStateGeneration(account))
		return err
	}
	// Narrow legacy test repositories may embed a nil AccountRepository and do
	// not support UpdateExtra. Keep their prior best-effort marker semantics
	// while ensuring the actual error transition still runs.
	func() {
		defer func() { _ = recover() }()
		_ = repo.UpdateExtra(ctx, account.ID, map[string]any{
			Sub2APIRuntimeRecoverableAccountErrorExtraKey: true,
		})
	}()
	return repo.SetError(ctx, account.ID, message)
}

func sub2APIAdminAccountStateGeneration(account *Account) string {
	if account == nil || account.Extra == nil {
		return ""
	}
	generation, _ := account.Extra[Sub2APIAdminAccountStateGenerationExtraKey].(string)
	return generation
}

func newSub2APIAdminAccountStateGeneration() string {
	return uuid.NewString()
}

func adminProbeOwnershipReleaseUpdates() map[string]any {
	return map[string]any{
		Sub2APIProbeManagedAccountStatusExtraKey:      false,
		Sub2APIRuntimeRecoverableAccountErrorExtraKey: false,
		Sub2APIProbeGroupsExhaustedExtraKey:           false,
		Sub2APIProbeGroupsExhaustedMessageExtraKey:    "",
		Sub2APIAdminAccountStateGenerationExtraKey:    newSub2APIAdminAccountStateGeneration(),
	}
}

// shouldReleaseAdminProbeOwnershipOnStatusChange reports the only status edit
// that is treated as an explicit user stop. Probe-owned error states must stay
// under Provider control even when an edit form resubmits the old status.
func shouldReleaseAdminProbeOwnershipOnStatusChange(account *Account, requestedStatus string) bool {
	if account == nil || account.Status != StatusActive {
		return false
	}
	if requestedStatus != "inactive" && requestedStatus != StatusDisabled {
		return false
	}
	return requestedStatus != account.Status
}

// shouldReleaseAdminProbeOwnershipOnSchedulableChange treats only a normal,
// currently schedulable account being explicitly disabled as a user stop.
func shouldReleaseAdminProbeOwnershipOnSchedulableChange(account *Account, schedulable bool) bool {
	return account != nil && account.Status == StatusActive && account.Schedulable && !schedulable
}

func probeOwnsAbnormalAccountStatus(account *Account) bool {
	if account == nil || account.Status != StatusError || account.Extra == nil {
		return false
	}
	managed, _ := account.Extra[Sub2APIProbeManagedAccountStatusExtraKey].(bool)
	runtimeRecoverable, _ := account.Extra[Sub2APIRuntimeRecoverableAccountErrorExtraKey].(bool)
	return managed || runtimeRecoverable
}

func releaseAdminProbeOwnership(account *Account) {
	if account == nil {
		return
	}
	if account.Extra == nil {
		account.Extra = make(map[string]any)
	}
	for key, value := range adminProbeOwnershipReleaseUpdates() {
		account.Extra[key] = value
	}
}

func clearAdminAccountErrorState(ctx context.Context, repo AccountRepository, accountID int64) error {
	if stateRepo, ok := repo.(AdminAccountStateRepository); ok {
		return stateRepo.ClearAdminAccountError(ctx, accountID)
	}
	if err := repo.ClearError(ctx, accountID); err != nil {
		return err
	}
	return repo.UpdateExtra(ctx, accountID, adminProbeOwnershipReleaseUpdates())
}

func setAdminAccountErrorState(ctx context.Context, repo AccountRepository, accountID int64, message string) error {
	if stateRepo, ok := repo.(AdminAccountStateRepository); ok {
		return stateRepo.SetAdminAccountError(ctx, accountID, message)
	}
	if err := repo.SetError(ctx, accountID, message); err != nil {
		return err
	}
	return repo.UpdateExtra(ctx, accountID, adminProbeOwnershipReleaseUpdates())
}

func setAdminAccountSchedulableState(ctx context.Context, repo AccountRepository, accountID int64, schedulable bool) error {
	account, err := repo.GetByID(ctx, accountID)
	if err != nil {
		return err
	}
	releaseOwnership := shouldReleaseAdminProbeOwnershipOnSchedulableChange(account, schedulable)
	if stateRepo, ok := repo.(AdminAccountStateRepository); ok {
		return stateRepo.SetAdminAccountSchedulable(ctx, accountID, schedulable)
	}
	if err := repo.SetSchedulable(ctx, accountID, schedulable); err != nil {
		return err
	}
	if releaseOwnership {
		return repo.UpdateExtra(ctx, accountID, adminProbeOwnershipReleaseUpdates())
	}
	return nil
}

// AdminAccountStateRepository makes an explicit administrator state change
// atomically. Scheduling ownership is released only for a normal account that
// is explicitly stopped; probe-owned error rows remain recoverable.
type AdminAccountStateRepository interface {
	SetAdminAccountError(ctx context.Context, accountID int64, message string) error
	ClearAdminAccountError(ctx context.Context, accountID int64) error
	SetAdminAccountSchedulable(ctx context.Context, accountID int64, schedulable bool) error
}

// Sub2APIAccountRepository is the Provider subsystem's narrow account port.
// It deliberately does not extend AccountRepository: unrelated gateway,
// billing, and scheduler repositories must not implement Provider-only writes.
type Sub2APIAccountRepository interface {
	GetByID(ctx context.Context, id int64) (*Account, error)
	GetByIDs(ctx context.Context, ids []int64) ([]*Account, error)
	SetError(ctx context.Context, id int64, errorMsg string) error
	ClearError(ctx context.Context, id int64) error
	UpdateExtra(ctx context.Context, id int64, updates map[string]any) error
	// UpdateProviderLink 更新 Account 的 Provider 关联
	UpdateProviderLink(ctx context.Context, accountID, providerID, providerAPIKeyID int64) error
	// ClearProviderLink 清除 Account 的 Provider 关联
	ClearProviderLink(ctx context.Context, accountID, providerID int64) error
	// UpdateProviderAccountsProxy applies one Provider route to its linked accounts.
	UpdateProviderAccountsProxy(ctx context.Context, providerID int64, proxyID *int64) error
	// UpdateRemoteGroupBinding persists the stable remote group identity and display cache.
	UpdateRemoteGroupBinding(ctx context.Context, accountID, groupID int64, groupName string, multiplier float64) error
	// UpdateRemoteGroupIdentity persists an ID while clearing unavailable display metadata.
	UpdateRemoteGroupIdentity(ctx context.Context, accountID, groupID int64) error
	// ListByProviderID 获取关联到指定 Provider 的所有 Account
	ListByProviderID(ctx context.Context, providerID int64) ([]Account, error)
	// UpdateSub2APIOptimizeSettings 全量覆盖账号的定时优化设置。
	UpdateSub2APIOptimizeSettings(ctx context.Context, providerID, accountID int64, enabled bool, minMultiplier, maxMultiplier *float64, testModel *string, groupID *int64) error
}

// Sub2APIOptimizeGroupIDsRepository is an optional extension implemented by
// the Provider account adapter. Keeping it separate preserves compatibility
// with existing AccountRepository test doubles and upstream integrations.
type Sub2APIOptimizeGroupIDsRepository interface {
	UpdateSub2APIOptimizeSettingsWithGroupIDs(ctx context.Context, providerID, accountID int64, enabled bool, minMultiplier, maxMultiplier *float64, testModel *string, groupIDs []int64) error
}

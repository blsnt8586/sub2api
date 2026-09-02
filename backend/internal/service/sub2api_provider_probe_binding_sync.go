package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/pkg/sub2api"
)

type sub2APIControlBindingSyncResult struct {
	AccountsUpdated int
	AccountsCleared int
	TargetsUpdated  int
}

func indexProviderGroupsWithEffectiveRates(groups []sub2api.Group, effectiveRates map[int64]float64) map[int64]sub2api.Group {
	groupsByID := make(map[int64]sub2api.Group, len(groups))
	for _, group := range groups {
		if rate, ok := effectiveRates[group.ID]; ok && rate >= 0 {
			group.RateMultiplier = rate
		}
		groupsByID[group.ID] = group
	}
	return groupsByID
}

// syncControlProbeBindingsLocked projects one stable control-probe snapshot
// into the linked accounts and their route monitors. The caller must own the
// Provider operation gate while both reading the remote keys and calling this
// method so an optimizer's temporary candidate group can never be persisted.
func (s *Sub2APIProviderProbeService) syncControlProbeBindingsLocked(
	ctx context.Context,
	providerID int64,
	keys []sub2api.APIKey,
	groups []sub2api.Group,
	effectiveRates map[int64]float64,
) (sub2APIControlBindingSyncResult, error) {
	result := sub2APIControlBindingSyncResult{}
	if s == nil || s.accountRepo == nil || s.probeRepo == nil {
		return result, nil
	}

	accounts, err := s.accountRepo.ListByProviderID(ctx, providerID)
	if err != nil {
		return result, fmt.Errorf("list linked accounts: %w", err)
	}
	if len(accounts) == 0 {
		return result, nil
	}
	targets, err := s.probeRepo.ListTargets(ctx, providerID)
	if err != nil {
		return result, fmt.Errorf("list probe targets: %w", err)
	}
	targetByAccountID := make(map[int64]*ent.Sub2APIProviderProbeTarget, len(targets))
	for _, target := range targets {
		if target != nil {
			targetByAccountID[target.AccountID] = target
		}
	}

	// Apply the authenticated Provider user's effective multiplier to a copy of
	// the catalog so account display, pricing calculations and optimizer bounds
	// all use the same value.
	groupsByID := indexProviderGroupsWithEffectiveRates(groups, effectiveRates)
	keyBindings := make(map[int64]providerRemoteGroupInfo, len(keys))
	for _, key := range keys {
		if binding, ok := resolveProviderRemoteKeyGroup(key, groupsByID); ok {
			keyBindings[key.ID] = binding
		}
	}

	clearer, canClear := s.accountRepo.(interface {
		ClearRemoteGroupBinding(context.Context, int64) error
	})
	var syncErr error
	for i := range accounts {
		account := &accounts[i]
		if account.ProviderAPIKeyID == nil {
			continue
		}
		target := targetByAccountID[account.ID]
		binding, found := keyBindings[*account.ProviderAPIKeyID]
		if !found {
			if !canClear {
				syncErr = errors.Join(syncErr, fmt.Errorf("account %d: remote binding clear is unsupported", account.ID))
				continue
			}
			if err := clearer.ClearRemoteGroupBinding(ctx, account.ID); err != nil {
				syncErr = errors.Join(syncErr, fmt.Errorf("account %d: clear missing remote key: %w", account.ID, err))
				continue
			}
			result.AccountsCleared++
			if target != nil {
				platform := account.Platform
				if platform == "" {
					platform = target.Platform
				}
				if _, err := s.probeRepo.UpdateTargetBinding(ctx, target.ID, account.ProviderAPIKeyID, nil, nil, platform); err != nil {
					syncErr = errors.Join(syncErr, fmt.Errorf("account %d: clear probe target binding: %w", account.ID, err))
				} else {
					result.TargetsUpdated++
				}
			}
			continue
		}

		var persistErr error
		if binding.complete {
			persistErr = s.accountRepo.UpdateRemoteGroupBinding(ctx, account.ID, binding.id, binding.name, binding.multiplier)
		} else {
			persistErr = s.accountRepo.UpdateRemoteGroupIdentity(ctx, account.ID, binding.id)
		}
		if persistErr != nil {
			syncErr = errors.Join(syncErr, fmt.Errorf("account %d: update remote group: %w", account.ID, persistErr))
			continue
		}
		result.AccountsUpdated++
		if target != nil {
			groupID := binding.id
			var groupName *string
			if binding.complete {
				name := binding.name
				groupName = &name
			}
			platform := account.Platform
			if platform == "" {
				platform = target.Platform
			}
			if _, err := s.probeRepo.UpdateTargetBinding(ctx, target.ID, account.ProviderAPIKeyID, &groupID, groupName, platform); err != nil {
				syncErr = errors.Join(syncErr, fmt.Errorf("account %d: update probe target binding: %w", account.ID, err))
			} else {
				result.TargetsUpdated++
			}
		}
	}
	return result, syncErr
}

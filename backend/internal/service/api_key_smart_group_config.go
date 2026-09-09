package service

import (
	"context"
	"fmt"
	"sort"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var (
	ErrSmartGroupCandidatesInvalid     = infraerrors.BadRequest("SMART_GROUP_CANDIDATES_INVALID", "smart group routing requires 2 to 10 distinct groups")
	ErrSmartGroupPlatformMismatch      = infraerrors.BadRequest("SMART_GROUP_PLATFORM_MISMATCH", "smart group candidates must use the same platform")
	ErrSmartGroupUnsupported           = infraerrors.BadRequest("SMART_GROUP_PLATFORM_UNSUPPORTED", "smart group routing is not supported for composite or canvas groups")
	ErrSmartGroupManualSwitchForbidden = infraerrors.BadRequest("SMART_GROUP_MANUAL_SWITCH_FORBIDDEN", "the active smart group can only change after a successful probe")
)

type smartGroupCandidate struct {
	id   int64
	rate float64
}

func smartGroupPlatformSupported(platform string) bool {
	return platform != PlatformComposite && platform != PlatformCanvas
}

func (s *APIKeyService) normalizeSmartGroupCandidates(
	ctx context.Context,
	user *User,
	groupIDs []int64,
) ([]int64, error) {
	if len(groupIDs) < 2 || len(groupIDs) > SmartGroupMaxCandidates {
		return nil, ErrSmartGroupCandidatesInvalid
	}

	seen := make(map[int64]struct{}, len(groupIDs))
	candidates := make([]smartGroupCandidate, 0, len(groupIDs))
	platform := ""
	userRates := map[int64]float64{}
	if s.userGroupRateRepo != nil {
		rates, err := s.userGroupRateRepo.GetByUserID(ctx, user.ID)
		if err != nil {
			return nil, fmt.Errorf("load user group rates: %w", err)
		}
		userRates = rates
	}

	for _, groupID := range groupIDs {
		if groupID <= 0 {
			return nil, ErrSmartGroupCandidatesInvalid
		}
		if _, exists := seen[groupID]; exists {
			return nil, ErrSmartGroupCandidatesInvalid
		}
		seen[groupID] = struct{}{}

		group, err := s.groupRepo.GetByID(ctx, groupID)
		if err != nil {
			return nil, err
		}
		if group.Status != StatusActive || !s.canUserBindGroup(ctx, user, group) {
			return nil, ErrGroupNotAllowed
		}
		if !smartGroupPlatformSupported(group.Platform) {
			return nil, ErrSmartGroupUnsupported
		}
		if platform == "" {
			platform = group.Platform
		} else if group.Platform != platform {
			return nil, ErrSmartGroupPlatformMismatch
		}

		rate := group.RateMultiplier
		if userRate, ok := userRates[groupID]; ok {
			rate = userRate
		}
		candidates = append(candidates, smartGroupCandidate{id: groupID, rate: rate})
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].rate == candidates[j].rate {
			return candidates[i].id < candidates[j].id
		}
		return candidates[i].rate < candidates[j].rate
	})

	result := make([]int64, 0, len(candidates))
	for _, candidate := range candidates {
		result = append(result, candidate.id)
	}
	return result, nil
}

func smartGroupContains(groupIDs []int64, groupID int64) bool {
	for _, id := range groupIDs {
		if id == groupID {
			return true
		}
	}
	return false
}

func applySmartGroupDefaults(failureThreshold, recoveryIntervalSeconds int) (int, int) {
	if failureThreshold == 0 {
		failureThreshold = SmartGroupDefaultFailureThreshold
	}
	if recoveryIntervalSeconds == 0 {
		recoveryIntervalSeconds = SmartGroupDefaultRecoveryIntervalSeconds
	}
	return failureThreshold, recoveryIntervalSeconds
}

func resetSmartGroupRuntime(apiKey *APIKey) {
	apiKey.SmartGroupConsecutiveFailures = 0
	apiKey.SmartGroupLastError = ""
	apiKey.SmartGroupLastSwitchReason = ""
	apiKey.SmartGroupProbeLeaseUntil = nil
	apiKey.SmartGroupLastProbeAt = nil
	apiKey.SmartGroupLastSwitchAt = nil
	// A configuration save is not health evidence. The first successful real
	// request starts the stable window; a successful independent switch probe
	// starts it in CompleteSmartGroupSwitch.
	apiKey.SmartGroupHealthySince = nil
}

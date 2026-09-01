package service

import (
	"encoding/json"
	"sort"
)

const sub2APIOptimizeGroupIDsExtraKey = "sub2api_optimize_group_ids"

func normalizeSub2APIOptimizeGroupIDs(ids []int64) []int64 {
	if len(ids) == 0 {
		return nil
	}
	seen := make(map[int64]struct{}, len(ids))
	result := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	if len(result) == 0 {
		return nil
	}
	return result
}

// NormalizeSub2APIOptimizeGroupIDsForPersistence exposes the same validation
// to the Provider repository without widening the repository's domain model.
func NormalizeSub2APIOptimizeGroupIDsForPersistence(ids []int64) []int64 {
	return normalizeSub2APIOptimizeGroupIDs(ids)
}

func configuredSub2APIOptimizeGroupIDs(account *Account) []int64 {
	if account == nil {
		return nil
	}
	if ids := normalizeSub2APIOptimizeGroupIDs(account.Sub2APIOptimizeGroupIDs); len(ids) > 0 {
		return ids
	}
	if account.Sub2APIOptimizeGroupID != nil && *account.Sub2APIOptimizeGroupID > 0 {
		return []int64{*account.Sub2APIOptimizeGroupID}
	}
	return nil
}

func parseSub2APIOptimizeGroupIDs(extra map[string]any) []int64 {
	if len(extra) == 0 {
		return nil
	}
	value, ok := extra[sub2APIOptimizeGroupIDsExtraKey]
	if !ok {
		return nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var ids []int64
	if err := json.Unmarshal(encoded, &ids); err != nil {
		return nil
	}
	return normalizeSub2APIOptimizeGroupIDs(ids)
}

// ParseSub2APIOptimizeGroupIDs reads the persisted multi-select setting for
// adapters that materialize Provider account rows outside this package.
func ParseSub2APIOptimizeGroupIDs(extra map[string]any) []int64 {
	return parseSub2APIOptimizeGroupIDs(extra)
}

func groupIDSelected(groupID int64, selectedIDs []int64) bool {
	if len(selectedIDs) == 0 {
		return true
	}
	for _, selectedID := range selectedIDs {
		if selectedID == groupID {
			return true
		}
	}
	return false
}

//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/sub2api"
)

func optimizeFloat(v float64) *float64 { return &v }
func optimizeString(v string) *string  { return &v }
func optimizeInt64(v int64) *int64     { return &v }

func completeOptimizeAccount() Account {
	return Account{
		ID:                     1,
		Name:                   "ready",
		ProviderAPIKeyID:       optimizeInt64(99),
		Sub2APIOptimizeEnabled: true,
		Sub2APIMinMultiplier:   optimizeFloat(0.3),
		Sub2APIMaxMultiplier:   optimizeFloat(0.8),
		Sub2APITestModel:       optimizeString("test-model"),
	}
}

func TestOptimizeReadyAccountsRequiresAllThreeSettings(t *testing.T) {
	readyAccount := completeOptimizeAccount()
	missingMin := completeOptimizeAccount()
	missingMin.ID = 2
	missingMin.Name = "missing-min"
	missingMin.Sub2APIMinMultiplier = nil
	missingMax := completeOptimizeAccount()
	missingMax.ID = 3
	missingMax.Name = "missing-max"
	missingMax.Sub2APIMaxMultiplier = nil
	missingModel := completeOptimizeAccount()
	missingModel.ID = 4
	missingModel.Name = "missing-model"
	missingModel.Sub2APITestModel = nil
	blankModel := completeOptimizeAccount()
	blankModel.ID = 5
	blankModel.Name = "blank-model"
	blankModel.Sub2APITestModel = optimizeString("   ")
	disabled := completeOptimizeAccount()
	disabled.ID = 6
	disabled.Sub2APIOptimizeEnabled = false

	ready, invalid := optimizeReadyAccounts([]Account{
		readyAccount,
		missingMin,
		missingMax,
		missingModel,
		blankModel,
		disabled,
	})
	if len(ready) != 1 || ready[0].ID != readyAccount.ID {
		t.Fatalf("ready accounts = %#v, want only account %d", ready, readyAccount.ID)
	}
	if len(invalid) != 4 {
		t.Fatalf("invalid details = %#v, want 4", invalid)
	}
	for _, detail := range invalid {
		if detail.Status != "failed" || detail.Reason == "" {
			t.Fatalf("invalid detail = %#v, want explicit failed reason", detail)
		}
	}
}

func TestCheckOptimizeReadyRequiresParticipation(t *testing.T) {
	account := completeOptimizeAccount()
	account.Sub2APIOptimizeEnabled = false

	svc := &Sub2APIOptimizeScheduleService{}
	if err := svc.checkOptimizeReady(&account); err == nil {
		t.Fatal("expected disabled account to be rejected")
	}
}

func TestUpdateAccountOptimizeSettingsRejectsMissingRequiredSettings(t *testing.T) {
	svc := &Sub2APIOptimizeScheduleService{}
	max := optimizeFloat(0.8)
	min := optimizeFloat(0.3)
	model := optimizeString("test-model")

	tests := []struct {
		name  string
		min   *float64
		max   *float64
		model *string
	}{
		{name: "missing min", max: max, model: model},
		{name: "missing max", min: min, model: model},
		{name: "missing model", min: min, max: max},
		{name: "blank model", min: min, max: max, model: optimizeString("  ")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := svc.UpdateAccountOptimizeSettings(context.Background(), 7, 1, true, tt.min, tt.max, tt.model, nil); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestUpdateAccountOptimizeSettingsRejectsInvalidOptionalGroup(t *testing.T) {
	svc := &Sub2APIOptimizeScheduleService{}
	invalidGroupID := int64(0)
	err := svc.UpdateAccountOptimizeSettings(
		context.Background(), 7, 1, false, nil, nil, nil, &invalidGroupID,
	)
	if err == nil {
		t.Fatal("expected an invalid optional group to be rejected")
	}
}

func TestOptimizeGroupMatchesOptionalSelectedGroup(t *testing.T) {
	account := completeOptimizeAccount()
	account.Platform = "openai"
	selected := int64(22)
	account.Sub2APIOptimizeGroupID = &selected

	if optimizeGroupMatchesAccount(sub2api.Group{ID: 21, Platform: "openai", Status: "active", RateMultiplier: 0.5}, &account, 0.3, 0.8) {
		t.Fatal("an unselected group must not enter the candidate set")
	}
	if !optimizeGroupMatchesAccount(sub2api.Group{ID: 22, Platform: "openai", Status: "active", RateMultiplier: 0.5}, &account, 0.3, 0.8) {
		t.Fatal("the selected same-platform in-range group should enter the candidate set")
	}
	if optimizeGroupMatchesAccount(sub2api.Group{ID: 22, Platform: "anthropic", Status: "active", RateMultiplier: 0.5}, &account, 0.3, 0.8) {
		t.Fatal("selected group must still match the account platform")
	}
}

func TestOptimizeGroupMatchesMultipleSelectedGroups(t *testing.T) {
	account := completeOptimizeAccount()
	account.Platform = "openai"
	account.Sub2APIOptimizeGroupIDs = []int64{31, 22, 31}

	if optimizeGroupMatchesAccount(sub2api.Group{ID: 21, Platform: "openai", Status: "active", RateMultiplier: 0.5}, &account, 0.3, 0.8) {
		t.Fatal("an unselected group must not enter the candidate set")
	}
	for _, id := range []int64{22, 31} {
		if !optimizeGroupMatchesAccount(sub2api.Group{ID: id, Platform: "openai", Status: "active", RateMultiplier: 0.5}, &account, 0.3, 0.8) {
			t.Fatalf("selected group %d should enter the candidate set", id)
		}
	}
}

func TestApplyOptimizeGroupRateOverridesUsesEffectiveRates(t *testing.T) {
	groups := []sub2api.Group{{ID: 11, RateMultiplier: 0.8}, {ID: 12, RateMultiplier: 1.0}}
	got := applyOptimizeGroupRateOverrides(groups, map[string]float64{"11": 0.08})
	if got[0].RateMultiplier != 0.08 || got[1].RateMultiplier != 1.0 {
		t.Fatalf("overridden groups = %+v", got)
	}
	if groups[0].RateMultiplier != 0.8 {
		t.Fatal("override helper must not mutate the shared group catalog")
	}
}

func TestOptimizeRunStatusDoesNotReportEmptyRunAsSuccess(t *testing.T) {
	if got := optimizeRunStatus(0, 0, 0); got != "skipped" {
		t.Fatalf("empty status = %q, want skipped", got)
	}
	if got := optimizeRunStatus(2, 0, 1); got != "failed" {
		t.Fatalf("failed status = %q, want failed", got)
	}
	if got := optimizeRunStatus(2, 1, 1); got != "partial" {
		t.Fatalf("partial status = %q, want partial", got)
	}
	if got := optimizeRunStatus(2, 1, 0); got != "success" {
		t.Fatalf("success status = %q, want success", got)
	}
}

func TestRetainedCurrentGroupReasonExplainsFailedCheaperCandidates(t *testing.T) {
	plain := retainedCurrentGroupReason(nil)
	if plain != "当前分组实测可用，已是区间内最优分组" {
		t.Fatalf("plain reason=%q", plain)
	}

	withFailedCandidate := retainedCurrentGroupReason([]OptimizeGroupSwitchEvent{
		{Action: "switch", Status: "success", TestStatus: "failed"},
		{Action: "rollback", Status: "success"},
	})
	if withFailedCandidate != "当前分组复测已恢复；此前 1 个更低倍率候选不可用，保留当前分组" {
		t.Fatalf("failed-candidate reason=%q", withFailedCandidate)
	}
}

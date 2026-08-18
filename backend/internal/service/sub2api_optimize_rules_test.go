//go:build unit

package service

import (
	"context"
	"testing"
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
			if err := svc.UpdateAccountOptimizeSettings(context.Background(), 7, 1, true, tt.min, tt.max, tt.model); err == nil {
				t.Fatal("expected validation error")
			}
		})
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

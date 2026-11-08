//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/sub2apiproviderprobetargetrun"
)

// This fixture is an anonymized shape of the read-only 45-server sample
// collected on 2026-11-08: healthy=1169, degraded=269, unhealthy=30
// (protocol=12, timeout=18). It deliberately retains only status and latency
// characteristics so production account identity and error text never enter
// the test suite.
func TestProduction45ProbeShapeRecoversProbeOwnedAccount(t *testing.T) {
	repo := &probeAccountStateRepoStub{account: &Account{
		Status:      StatusActive,
		Schedulable: true,
		Extra:       map[string]any{},
	}}
	svc := &Sub2APIProviderProbeService{accountRepo: repo}
	cfg := &ent.Sub2APIProviderProbeConfig{
		AccountStatusFailureThreshold:  2,
		AccountStatusRecoveryThreshold: 2,
	}
	target := &ent.Sub2APIProviderProbeTarget{ID: 9001, AccountID: 4501}
	// Representative production failure sequence: timeout followed by a
	// protocol failure, then a degraded response and a healthy response.
	timeoutMessage := "probe timeout"
	protocolMessage := "upstream protocol error"
	runs := []*ent.Sub2APIProviderProbeTargetRun{
		{ID: 4, Status: sub2apiproviderprobetargetrun.StatusHealthy, LatencyMs: fixtureIntPtr(7450)},
		{ID: 3, Status: sub2apiproviderprobetargetrun.StatusDegraded, LatencyMs: fixtureIntPtr(16235)},
		{ID: 2, Status: sub2apiproviderprobetargetrun.StatusUnhealthy, LatencyMs: fixtureIntPtr(60001), ErrorMessage: &timeoutMessage},
		{ID: 1, Status: sub2apiproviderprobetargetrun.StatusUnhealthy, LatencyMs: fixtureIntPtr(39873), ErrorMessage: &protocolMessage},
	}

	if err := svc.projectAccountStatusFromProbe(context.Background(), cfg, target, runs[3], runs[3:], "", false); err != nil {
		t.Fatalf("project production-shaped first failure: %v", err)
	}
	if repo.setErrorCalls != 0 {
		t.Fatalf("first failure must not cross the configured threshold: calls=%d", repo.setErrorCalls)
	}
	if err := svc.projectAccountStatusFromProbe(context.Background(), cfg, target, runs[2], runs[2:], "", false); err != nil {
		t.Fatalf("project production-shaped protocol failure: %v", err)
	}
	if repo.setErrorCalls != 1 {
		t.Fatalf("two production-shaped failures should mark the account: calls=%d", repo.setErrorCalls)
	}
	// The production repository applies this error state and ownership marker
	// atomically. Mirror the committed row before simulating recovery samples.
	repo.account.Status = StatusError
	repo.account.Schedulable = false
	repo.account.Extra[probeManagedAccountStatusExtraKey] = true
	if err := svc.projectAccountStatusFromProbe(context.Background(), cfg, target, runs[1], runs[1:], "", false); err != nil {
		t.Fatalf("project degraded recovery sample: %v", err)
	}
	if repo.clearErrorCalls != 0 {
		t.Fatalf("one responsive sample must not satisfy recovery threshold: clears=%d", repo.clearErrorCalls)
	}
	if err := svc.projectAccountStatusFromProbe(context.Background(), cfg, target, runs[0], runs[:2], "", false); err != nil {
		t.Fatalf("project healthy recovery sample: %v", err)
	}
	if repo.clearErrorCalls != 1 {
		t.Fatalf("two consecutive responsive samples should recover the probe-owned account: clears=%d", repo.clearErrorCalls)
	}
}

func TestProduction45ProbeShapeDoesNotRecoverManualAccount(t *testing.T) {
	repo := &probeAccountStateRepoStub{account: &Account{
		Status:      StatusDisabled,
		Schedulable: false,
		Extra:       map[string]any{Sub2APIAdminAccountStateGenerationExtraKey: "manual"},
	}}
	svc := &Sub2APIProviderProbeService{accountRepo: repo}
	cfg := &ent.Sub2APIProviderProbeConfig{AccountStatusFailureThreshold: 1, AccountStatusRecoveryThreshold: 1}
	target := &ent.Sub2APIProviderProbeTarget{ID: 9002, AccountID: 4502}
	run := &ent.Sub2APIProviderProbeTargetRun{ID: 1, Status: sub2apiproviderprobetargetrun.StatusHealthy, LatencyMs: fixtureIntPtr(3583)}

	if err := svc.projectAccountStatusFromProbe(context.Background(), cfg, target, run, []*ent.Sub2APIProviderProbeTargetRun{run}, "manual", false); err != nil {
		t.Fatalf("project manual account sample: %v", err)
	}
	if repo.clearErrorCalls != 0 {
		t.Fatalf("healthy probe must not clear a manually disabled account: clears=%d", repo.clearErrorCalls)
	}
	if repo.account.Status != StatusDisabled || repo.account.Schedulable {
		t.Fatalf("manual account state changed: status=%s schedulable=%t", repo.account.Status, repo.account.Schedulable)
	}
}

func fixtureIntPtr(v int) *int { return &v }

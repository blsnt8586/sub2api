//go:build unit

package service

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/sub2api"
)

type providerProfitUsageRepoStub struct {
	UsageLogRepository
	values map[int64]ProviderAccountRevenue
}

func (s providerProfitUsageRepoStub) GetProviderAccountRevenue(context.Context, []int64, time.Time, time.Time) (map[int64]ProviderAccountRevenue, error) {
	return s.values, nil
}

func TestCollectProviderProfitSnapshotJoinsLocalRevenueAndRemoteAPIKeyCost(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/usage/dashboard/api-keys-usage" {
			t.Fatalf("path=%q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{"stats": map[string]any{
			"101": map[string]any{"today_actual_cost": 2.0, "total_actual_cost": 20.0},
			"102": map[string]any{"today_actual_cost": 1.0, "total_actual_cost": 8.0},
		}}})
	}))
	defer server.Close()

	client := sub2api.NewClient(server.URL, "", "")
	client.Token = "access"
	accounts := []Account{
		{ID: 1, Name: "alpha", ProviderAPIKeyID: profitPtrInt64(101)},
		{ID: 2, Name: "beta", ProviderAPIKeyID: profitPtrInt64(102)},
	}
	reader := providerProfitUsageRepoStub{values: map[int64]ProviderAccountRevenue{
		1: {AccountID: 1, TodayActualCost: 3, TotalActualCost: 30},
		2: {AccountID: 2, TodayActualCost: 2, TotalActualCost: 10},
	}}
	sampledAt := time.Date(2026, 8, 30, 4, 0, 0, 0, time.UTC)
	summary := collectProviderProfitSnapshot(context.Background(), client, accounts, []sub2api.APIKey{{ID: 101}, {ID: 102}}, reader, 10, sampledAt)
	if !summary.Available || summary.TotalRevenue != 40 || math.Abs(summary.TotalRemoteCost-2.8) > 1e-9 || math.Abs(summary.TotalGrossProfit-37.2) > 1e-9 {
		t.Fatalf("summary=%+v", summary)
	}
	if summary.TodayRevenue != 5 || math.Abs(summary.TodayRemoteCost-0.3) > 1e-9 || math.Abs(summary.TodayGrossProfit-4.7) > 1e-9 {
		t.Fatalf("today summary=%+v", summary)
	}
	if len(summary.Accounts) != 2 || summary.Accounts[0].AccountName != "alpha" || summary.Accounts[0].RemoteCost != 2 || summary.Accounts[1].GrossProfit != 9.2 {
		t.Fatalf("accounts=%+v", summary.Accounts)
	}
}

func TestCollectProviderProfitSnapshotReportsMissingRemoteKeyWithoutFailingHealth(t *testing.T) {
	client := sub2api.NewClient("http://127.0.0.1:1", "", "")
	client.Token = "access"
	accounts := []Account{{ID: 1, Name: "alpha", ProviderAPIKeyID: profitPtrInt64(999)}}
	reader := providerProfitUsageRepoStub{values: map[int64]ProviderAccountRevenue{1: {AccountID: 1, TotalActualCost: 5}}}
	summary := collectProviderProfitSnapshot(context.Background(), client, accounts, []sub2api.APIKey{{ID: 999}}, reader, 1, time.Now())
	if summary.Available || summary.Error == nil || len(summary.Accounts) != 1 || summary.Accounts[0].Error == nil {
		t.Fatalf("summary=%+v", summary)
	}
}

func profitPtrInt64(value int64) *int64 { return &value }

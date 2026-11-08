package service

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"
)

// collectProviderProfitSnapshot summarizes only local request revenue and
// upstream group multiplier estimates. It never reads remote financial data.
func collectProviderProfitSnapshot(
	ctx context.Context,
	accounts []Account,
	usageRepo UsageLogRepository,
	sampledAt time.Time,
) *Sub2APIProviderProfitSummary {
	end := sampledAt.UTC()
	start := time.Time{}
	summary := &Sub2APIProviderProfitSummary{
		RangeStart: start,
		RangeEnd:   end,
		SampledAt:  sampledAt,
		Accounts:   make([]Sub2APIProviderAccountProfit, 0, len(accounts)),
	}
	if len(accounts) == 0 {
		summary.Available = true
		summary.MonthProfitAvailable = true
		summary.CumulativeProfitAvailable = true
		summary.CostSource = "local_request_logs"
		return summary
	}
	// Provider profit is intentionally based only on local request records.
	// Do not compare against upstream wallet/recharge/API-key dashboard data:
	// those endpoints are incomplete for historical profitability.
	localReader, localOK := usageRepo.(ProviderAccountLocalProfitReader)
	if !localOK || localReader == nil {
		message := "local usage repository does not support provider profit aggregation"
		summary.Error = &message
		return summary
	}
	accountIDs := make([]int64, 0, len(accounts))
	for _, account := range accounts {
		accountIDs = append(accountIDs, account.ID)
	}
	localByAccount, localErr := localReader.GetProviderAccountLocalProfit(ctx, accountIDs, end)
	if localErr != nil {
		message := fmt.Sprintf("read local provider profit failed: %s", localErr)
		summary.Error = &message
		return summary
	}
	return buildProviderProfitSummary(accounts, localByAccount, sampledAt, "local_request_logs")
}

func providerDailyProfitSnapshot(accounts []Account, snapshots map[int64]ProviderDailySnapshot, sampledAt time.Time) *Sub2APIProviderProfitSummary {
	values := make(map[int64]ProviderAccountLocalProfit, len(snapshots))
	for accountID, snapshot := range snapshots {
		values[accountID] = ProviderAccountLocalProfit{
			AccountID: accountID, CostAvailable: snapshot.CostAvailable && snapshot.BaseCostAvailable,
			TodayRevenue: snapshot.TodayRevenue, TodayProfit: snapshot.TodayProfit,
			MonthRevenue: snapshot.MonthRevenue, MonthProfit: snapshot.MonthProfit,
			CumulativeRevenue: snapshot.TotalRevenue, CumulativeProfit: snapshot.CumulativeProfit,
		}
	}
	return buildProviderProfitSummary(accounts, values, sampledAt, "provider_daily_snapshots")
}

func readProviderDailySnapshots(ctx context.Context, providerID int64, accounts []Account, usageRepo UsageLogRepository, sampledAt time.Time) (map[int64]ProviderDailySnapshot, bool) {
	reader, ok := usageRepo.(ProviderDailySnapshotReader)
	if !ok || reader == nil {
		return nil, false
	}
	ids := make([]int64, 0, len(accounts))
	for _, account := range accounts {
		ids = append(ids, account.ID)
	}
	snapshots, err := reader.GetProviderDailySnapshots(ctx, providerID, ids, sampledAt)
	if err != nil {
		return nil, false
	}
	for _, snapshot := range snapshots {
		if snapshot.HasData {
			return snapshots, true
		}
	}
	return nil, false
}

func providerDailyUsageSnapshot(snapshots map[int64]ProviderDailySnapshot) *Sub2APIProviderRemoteUsageStats {
	usage := &Sub2APIProviderRemoteUsageStats{UsageSource: "provider_daily_snapshots", DashboardAvailable: true}
	for _, snapshot := range snapshots {
		usage.TodayTokens += snapshot.TodayTokens
		usage.TotalTokens += snapshot.TotalTokens
		usage.TodayActualCost += snapshot.TodayRevenue
		usage.TotalActualCost += snapshot.TotalRevenue
	}
	return usage
}

func buildProviderProfitSummary(accounts []Account, localByAccount map[int64]ProviderAccountLocalProfit, sampledAt time.Time, source string) *Sub2APIProviderProfitSummary {
	summary := &Sub2APIProviderProfitSummary{
		RangeStart: time.Time{},
		RangeEnd:   sampledAt.UTC(),
		SampledAt:  sampledAt,
		Accounts:   make([]Sub2APIProviderAccountProfit, 0, len(accounts)),
	}
	if len(accounts) == 0 {
		summary.Available = true
		summary.MonthProfitAvailable = true
		summary.CumulativeProfitAvailable = true
		summary.CostSource = source
		return summary
	}
	allCostsAvailable := true
	monthRevenue := 0.0
	for _, account := range accounts {
		local, exists := localByAccount[account.ID]
		costAvailable := exists && local.CostAvailable
		allCostsAvailable = allCostsAvailable && costAvailable
		item := Sub2APIProviderAccountProfit{
			AccountID: account.ID, AccountName: account.Name, ProviderAPIKeyID: account.ProviderAPIKeyID,
			Revenue: local.CumulativeRevenue, TodayRevenue: local.TodayRevenue,
			EstimatedCost:      local.CumulativeRevenue - local.CumulativeProfit,
			TodayEstimatedCost: local.TodayRevenue - local.TodayProfit,
			GrossProfit:        local.CumulativeProfit, TodayGrossProfit: local.TodayProfit,
			MonthGrossProfit: local.MonthProfit, CumulativeGrossProfit: local.CumulativeProfit,
			EstimatedCostAvailable: costAvailable, CostSource: source,
		}
		if !costAvailable {
			message := "upstream group multiplier or historical base cost snapshot is unavailable"
			item.Error = &message
		}
		item.GrossMargin = safeGrossMargin(item.GrossProfit, item.Revenue)
		item.TodayGrossMargin = safeGrossMargin(item.TodayGrossProfit, item.TodayRevenue)
		summary.Accounts = append(summary.Accounts, item)
		summary.TodayRevenue += local.TodayRevenue
		summary.TodayEstimatedCost += item.TodayEstimatedCost
		monthRevenue += local.MonthRevenue
		summary.TodayGrossProfit += local.TodayProfit
		summary.MonthGrossProfit += local.MonthProfit
		summary.CumulativeGrossProfit += local.CumulativeProfit
		summary.TotalRevenue += local.CumulativeRevenue
		summary.TotalEstimatedCost += local.CumulativeRevenue - local.CumulativeProfit
	}
	summary.TodayGrossMargin = safeGrossMargin(summary.TodayGrossProfit, summary.TodayRevenue)
	summary.MonthGrossMargin = safeGrossMargin(summary.MonthGrossProfit, monthRevenue)
	summary.CumulativeGrossMargin = safeGrossMargin(summary.CumulativeGrossProfit, summary.TotalRevenue)
	summary.TotalGrossProfit = summary.CumulativeGrossProfit
	summary.TotalGrossMargin = summary.CumulativeGrossMargin
	summary.Available = allCostsAvailable
	summary.MonthProfitAvailable = allCostsAvailable
	summary.CumulativeProfitAvailable = allCostsAvailable
	if !allCostsAvailable {
		message := "one or more accounts have incomplete cost inputs for local profit estimates"
		summary.Error = &message
	}
	summary.CostSource = source
	sort.SliceStable(summary.Accounts, func(i, j int) bool { return summary.Accounts[i].AccountName < summary.Accounts[j].AccountName })
	return summary
}

func safeGrossMargin(profit, revenue float64) float64 {
	if revenue == 0 || math.IsNaN(revenue) || math.IsInf(revenue, 0) {
		return 0
	}
	return profit / revenue
}

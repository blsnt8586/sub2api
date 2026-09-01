package service

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/sub2api"
)

// collectProviderProfitSnapshot joins local account charges with remote
// per-API-key costs. Remote usage is fetched in batches of 100 IDs, matching
// the upstream endpoint limit. A failure is reported in the snapshot and does
// not make the surrounding provider health probe fail.
func collectProviderProfitSnapshot(
	ctx context.Context,
	client *sub2api.Client,
	accounts []Account,
	remoteKeys []sub2api.APIKey,
	usageRepo UsageLogRepository,
	remoteCostDivisor float64,
	sampledAt time.Time,
) *Sub2APIProviderProfitSummary {
	end := sampledAt.UTC()
	// Match the upstream API-key dashboard contract: its default range begins
	// 30 calendar days ago, rather than a fixed 720-hour duration across DST.
	start := end.AddDate(0, 0, -30)
	summary := &Sub2APIProviderProfitSummary{
		RangeStart: start,
		RangeEnd:   end,
		SampledAt:  sampledAt,
		Accounts:   make([]Sub2APIProviderAccountProfit, 0, len(accounts)),
	}
	if len(accounts) == 0 {
		summary.Available = true
		return summary
	}
	reader, ok := usageRepo.(ProviderAccountRevenueReader)
	if !ok || reader == nil {
		message := "local usage repository does not support account revenue aggregation"
		summary.Error = &message
		return summary
	}
	accountIDs := make([]int64, 0, len(accounts))
	for _, account := range accounts {
		accountIDs = append(accountIDs, account.ID)
	}
	revenueByAccount, err := reader.GetProviderAccountRevenue(ctx, accountIDs, start, end)
	if err != nil {
		message := fmt.Sprintf("read local account revenue failed: %s", err)
		summary.Error = &message
		return summary
	}

	remoteByKey := make(map[int64]sub2api.APIKeyUsageStats)
	remoteUsageComplete := len(remoteKeys) == 0
	if len(remoteKeys) > 0 {
		if client == nil {
			message := "remote provider client is unavailable"
			summary.Error = &message
		} else {
			remoteUsageComplete = false
			ids := make([]int64, 0, len(remoteKeys))
			needed := make(map[int64]struct{}, len(accounts))
			for _, account := range accounts {
				if account.ProviderAPIKeyID != nil && *account.ProviderAPIKeyID > 0 {
					needed[*account.ProviderAPIKeyID] = struct{}{}
				}
			}
			seen := make(map[int64]struct{}, len(remoteKeys))
			for _, key := range remoteKeys {
				if key.ID <= 0 {
					continue
				}
				if _, required := needed[key.ID]; !required {
					continue
				}
				if _, exists := seen[key.ID]; exists {
					continue
				}
				seen[key.ID] = struct{}{}
				ids = append(ids, key.ID)
			}
			for offset := 0; offset < len(ids); offset += 100 {
				endOffset := offset + 100
				if endOffset > len(ids) {
					endOffset = len(ids)
				}
				stats, usageErr := client.GetAPIKeysUsage(ctx, ids[offset:endOffset])
				if usageErr != nil {
					message := fmt.Sprintf("read remote API key usage failed: %s", usageErr)
					summary.Error = &message
					break
				}
				for id, stats := range stats {
					remoteByKey[id] = stats
				}
			}
			if summary.Error == nil {
				remoteUsageComplete = true
			}
		}
	}
	divisor := effectiveRemoteCostDivisor(remoteCostDivisor)
	allRowsHaveRemoteCost := true
	for _, account := range accounts {
		revenue := revenueByAccount[account.ID]
		item := Sub2APIProviderAccountProfit{
			AccountID:        account.ID,
			AccountName:      account.Name,
			ProviderAPIKeyID: account.ProviderAPIKeyID,
			Revenue:          revenue.TotalActualCost,
			TodayRevenue:     revenue.TodayActualCost,
		}
		if account.ProviderAPIKeyID == nil {
			message := "account has no linked upstream API key"
			item.Error = &message
			allRowsHaveRemoteCost = false
		} else if remote, exists := remoteByKey[*account.ProviderAPIKeyID]; exists {
			item.RemoteCostAvailable = true
			item.RemoteCost = remote.TotalActualCost / divisor
			item.TodayRemoteCost = remote.TodayActualCost / divisor
		} else if summary.Error != nil {
			item.Error = summary.Error
			allRowsHaveRemoteCost = false
		} else {
			message := "upstream API key usage was not returned"
			item.Error = &message
			allRowsHaveRemoteCost = false
		}
		item.GrossProfit = item.Revenue - item.RemoteCost
		item.TodayGrossProfit = item.TodayRevenue - item.TodayRemoteCost
		item.GrossMargin = safeGrossMargin(item.GrossProfit, item.Revenue)
		item.TodayGrossMargin = safeGrossMargin(item.TodayGrossProfit, item.TodayRevenue)
		summary.Accounts = append(summary.Accounts, item)
		summary.TotalRevenue += item.Revenue
		summary.TotalRemoteCost += item.RemoteCost
		summary.TodayRevenue += item.TodayRevenue
		summary.TodayRemoteCost += item.TodayRemoteCost
	}
	summary.TotalGrossProfit = summary.TotalRevenue - summary.TotalRemoteCost
	summary.TodayGrossProfit = summary.TodayRevenue - summary.TodayRemoteCost
	summary.TotalGrossMargin = safeGrossMargin(summary.TotalGrossProfit, summary.TotalRevenue)
	summary.TodayGrossMargin = safeGrossMargin(summary.TodayGrossProfit, summary.TodayRevenue)
	// A summary is available when the local and remote requests completed. Rows
	// with missing/deleted keys remain visible with an explicit row error.
	if !allRowsHaveRemoteCost && summary.Error == nil {
		message := "one or more linked accounts have no remote API-key cost"
		summary.Error = &message
	}
	summary.Available = summary.Error == nil && remoteUsageComplete && allRowsHaveRemoteCost
	sort.SliceStable(summary.Accounts, func(i, j int) bool {
		return summary.Accounts[i].AccountName < summary.Accounts[j].AccountName
	})
	return summary
}

func safeGrossMargin(profit, revenue float64) float64 {
	if revenue == 0 || math.IsNaN(revenue) || math.IsInf(revenue, 0) {
		return 0
	}
	return profit / revenue
}

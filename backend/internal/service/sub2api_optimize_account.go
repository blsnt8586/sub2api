package service

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/pkg/sub2api"
)

// OptimizeAccountDetail 记录单个账号的优化结果。
// 既写入定时任务运行日志，也作为手动优化（单个/批量）的同步返回结构，
// json tag 与前端 OptimizeLogDetail 保持一致，前后端与日志三处共用同一契约。
type OptimizeAccountDetail struct {
	AccountID    int64                      `json:"account_id"`
	AccountName  string                     `json:"account_name"`
	Status       string                     `json:"status"` // optimized / skipped / failed
	OldGroup     string                     `json:"old_group,omitempty"`
	NewGroup     string                     `json:"new_group,omitempty"`
	OldMult      float64                    `json:"old_multiplier,omitempty"`
	NewMult      float64                    `json:"new_multiplier,omitempty"`
	Reason       string                     `json:"reason,omitempty"`
	SwitchEvents []OptimizeGroupSwitchEvent `json:"switch_events,omitempty"`
}

// OptimizeGroupSwitchEvent is an ordered audit event for every remote group
// mutation attempted by the optimizer, including candidate switches and
// rollbacks. A successful switch can still have a failed connectivity test.
type OptimizeGroupSwitchEvent struct {
	Action         string  `json:"action"` // switch / rollback
	FromGroupID    int64   `json:"from_group_id,omitempty"`
	FromGroup      string  `json:"from_group,omitempty"`
	FromMultiplier float64 `json:"from_multiplier,omitempty"`
	ToGroupID      int64   `json:"to_group_id,omitempty"`
	ToGroup        string  `json:"to_group,omitempty"`
	ToMultiplier   float64 `json:"to_multiplier,omitempty"`
	Status         string  `json:"status"`                // success / failed
	TestStatus     string  `json:"test_status,omitempty"` // passed / failed
	Reason         string  `json:"reason,omitempty"`
	OccurredAt     string  `json:"occurred_at"`
}

// optimizeKeyState 表示某个远端 APIKey 的当前分组状态。
type optimizeKeyState struct {
	groupID    int64
	groupName  string
	multiplier float64
}

// optimizeReadyAccounts 将已开启参与的账号分成可执行和配置无效两组。
// 未开启的账号不属于本次任务；已开启但缺少任一必填项的账号必须形成失败明细，
// 不能静默过滤后把一次空跑记录成成功。
func optimizeReadyAccounts(accounts []Account) ([]Account, []OptimizeAccountDetail) {
	var ready []Account
	var invalid []OptimizeAccountDetail
	for _, acc := range accounts {
		if !acc.Sub2APIOptimizeEnabled {
			continue
		}
		if reason := optimizeAccountConfigError(&acc); reason != "" {
			invalid = append(invalid, OptimizeAccountDetail{
				AccountID:   acc.ID,
				AccountName: acc.Name,
				Status:      "failed",
				Reason:      reason,
			})
			continue
		}
		ready = append(ready, acc)
	}
	return ready, invalid
}

// doRunOptimize 是定时优化的核心执行逻辑。
// 登录一次上游→拉一次 groups + keys→遍历账号按倍率上限找最优分组→切换+测试连接→失败回滚尝试下一个。
func (s *Sub2APIOptimizeScheduleService) doRunOptimize(
	ctx context.Context,
	provider *ent.Sub2APIProvider,
	accounts []Account,
) []OptimizeAccountDetail {
	ready, invalid := optimizeReadyAccounts(accounts)
	if len(ready) == 0 {
		return invalid
	}
	return append(s.optimizeAccounts(ctx, provider, ready), invalid...)
}

// doRunCronOptimize coalesces accounts that were just verified by a probe or
// manual run. Only those accounts are skipped; all other participating accounts
// still execute in this cron cycle, so a narrow probe can never satisfy an
// unrelated Provider-wide schedule.
func (s *Sub2APIOptimizeScheduleService) doRunCronOptimize(
	ctx context.Context,
	provider *ent.Sub2APIProvider,
	accounts []Account,
	covered map[int64]recentOptimizeCoverage,
) ([]OptimizeAccountDetail, map[int64]map[string]any) {
	ready, invalid := optimizeReadyAccounts(accounts)
	if len(ready) == 0 {
		return invalid, nil
	}

	pending, coalesced, extraByAccount := coalesceRecentOptimizeCoverage(ready, covered)

	details := make([]OptimizeAccountDetail, 0, len(ready)+len(invalid))
	if len(pending) > 0 {
		details = append(details, s.optimizeAccounts(ctx, provider, pending)...)
	}
	details = append(details, coalesced...)
	details = append(details, invalid...)
	return details, extraByAccount
}

func coalesceRecentOptimizeCoverage(
	ready []Account,
	covered map[int64]recentOptimizeCoverage,
) ([]Account, []OptimizeAccountDetail, map[int64]map[string]any) {
	pending := make([]Account, 0, len(ready))
	coalesced := make([]OptimizeAccountDetail, 0, len(covered))
	extraByAccount := make(map[int64]map[string]any, len(covered))
	for _, account := range ready {
		coverage, ok := covered[account.ID]
		if !ok {
			pending = append(pending, account)
			continue
		}

		groupName := ""
		if account.RemoteGroupName != nil {
			groupName = *account.RemoteGroupName
		}
		groupMultiplier := float64(0)
		if account.RemoteGroupMultiplier != nil {
			groupMultiplier = *account.RemoteGroupMultiplier
		}
		coalesced = append(coalesced, OptimizeAccountDetail{
			AccountID:   account.ID,
			AccountName: account.Name,
			Status:      "skipped",
			OldGroup:    groupName,
			NewGroup:    groupName,
			OldMult:     groupMultiplier,
			NewMult:     groupMultiplier,
			Reason:      "最近 5 分钟内已由其他优化任务完成切组或连通验证，本次定时任务合并跳过",
		})
		extraByAccount[account.ID] = map[string]any{
			optimizeExecutionDispositionKey: optimizeExecutionCoalesced,
			"coalesced_from_log_id":         coverage.LogID,
			"coalesced_from_trigger":        coverage.Trigger,
		}
	}

	if len(extraByAccount) == 0 {
		extraByAccount = nil
	}
	return pending, coalesced, extraByAccount
}

// optimizeAccounts 对一批「已满足前置条件」的账号执行优化，返回每个账号的结果明细。
// 调用方须自行完成参与开关/倍率上限/关联校验（见 optimizeReadyAccounts）。
// 定时任务与手动优化（单个/批量）共用此核心，保证「倍率不超上限 + 模型联通」逻辑一致。
func (s *Sub2APIOptimizeScheduleService) optimizeAccounts(
	ctx context.Context,
	provider *ent.Sub2APIProvider,
	participating []Account,
) []OptimizeAccountDetail {
	details := make([]OptimizeAccountDetail, 0, len(participating))

	// 登录一次（复用 token cache）
	client, err := s.providerSvc.newAuthedClient(ctx, provider)
	if err != nil {
		for _, acc := range participating {
			details = append(details, OptimizeAccountDetail{
				AccountID:   acc.ID,
				AccountName: acc.Name,
				Status:      "failed",
				Reason:      fmt.Sprintf("login failed: %v", err),
			})
		}
		return details
	}

	// 确定路径
	keysPath := "/api/v1/keys"
	if provider.APIPathKeys != nil && *provider.APIPathKeys != "" {
		keysPath = *provider.APIPathKeys
	}
	groupsPath := "/api/v1/groups/available"
	if provider.APIPathGroups != nil && *provider.APIPathGroups != "" {
		groupsPath = *provider.APIPathGroups
	}

	// 拉一次 groups + keys。Client 会在 401 时自动轮换 Token 并重试一次。
	groups, err := client.GetGroups(ctx, groupsPath)
	if err != nil {
		for _, acc := range participating {
			details = append(details, OptimizeAccountDetail{
				AccountID:   acc.ID,
				AccountName: acc.Name,
				Status:      "failed",
				Reason:      fmt.Sprintf("get groups failed: %v", err),
			})
		}
		return details
	}

	currentKeys, err := client.GetAPIKeys(ctx, keysPath)
	if err != nil {
		for _, acc := range participating {
			details = append(details, OptimizeAccountDetail{
				AccountID:   acc.ID,
				AccountName: acc.Name,
				Status:      "failed",
				Reason:      fmt.Sprintf("get keys failed: %v", err),
			})
		}
		return details
	}

	// 构建 keyID -> 当前分组状态映射
	keyStateMap := make(map[int64]optimizeKeyState, len(currentKeys))
	for _, k := range currentKeys {
		ks := optimizeKeyState{groupID: k.GroupID}
		if k.Group != nil {
			ks.groupName = k.Group.Name
			ks.multiplier = k.Group.RateMultiplier
		}
		keyStateMap[k.ID] = ks
	}

	// 逐个账号优化
	for i := range participating {
		acc := participating[i]
		detail := s.optimizeOneAccount(ctx, client, &acc, groups, keyStateMap, keysPath)
		details = append(details, detail)
	}

	return details
}

// optimizeOneAccount 处理单个账号：找候选分组→切换+测试→失败回滚尝试下一个。
func (s *Sub2APIOptimizeScheduleService) optimizeOneAccount(
	ctx context.Context,
	client *sub2api.Client,
	acc *Account,
	groups []sub2api.Group,
	keyStateMap map[int64]optimizeKeyState,
	keysPath string,
) OptimizeAccountDetail {
	detail := OptimizeAccountDetail{
		AccountID:   acc.ID,
		AccountName: acc.Name,
	}

	ks, keyExists := keyStateMap[*acc.ProviderAPIKeyID]
	if !keyExists {
		detail.Status = "failed"
		detail.Reason = "关联的远端 Key 不存在，请重新同步或重新关联账号"
		return detail
	}
	detail.OldGroup = ks.groupName
	detail.OldMult = ks.multiplier

	// 候选分组：平台匹配 + active + 倍率落在必填的 [下限, 上限] 区间，按倍率升序。
	// 比下限还便宜的分组（往往是超卖/特价的不稳定区）会被排除；
	// 若当前分组恰好低于下限，它不会成为候选，引擎会主动上切到 ≥下限 的最便宜可用分组，
	// 用一点成本换质量底线。
	maxMult := *acc.Sub2APIMaxMultiplier
	minMult := *acc.Sub2APIMinMultiplier
	var candidates []sub2api.Group
	for _, g := range groups {
		if g.Platform == acc.Platform && g.Status == "active" &&
			g.RateMultiplier >= minMult && g.RateMultiplier <= maxMult {
			candidates = append(candidates, g)
		}
	}
	if len(candidates) == 0 {
		detail.Status = "failed"
		detail.Reason = "无符合倍率区间的候选分组"
		return detail
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].RateMultiplier < candidates[j].RateMultiplier
	})

	// 从最便宜开始逐个尝试，每个候选都实测连接可用性。
	// 若最便宜候选就是当前分组，则原地实测而不切换：
	//   - 可用 → 保留当前分组，并准确说明此前候选是否失败
	//   - 不可用（如分组被上游降级/超卖返回 5xx）→ 继续往上找能用的更贵分组，
	//     修复「卡在最便宜但已不可用分组上、后续永远只跳过」的问题。
	//
	// lastErr 记录最后一个候选的失败详情，triedCount 记录实际尝试数，
	// 供全部失败时汇总出「上限内无可用分组」的结论（而非只抛最后一个候选的原始错误）。
	var lastErr string
	triedCount := 0
	for _, cand := range candidates {
		isCurrent := cand.ID == ks.groupID
		switchEventIndex := -1

		if !isCurrent {
			// 切换分组；Client 统一处理 Token 恢复。
			switchEvent := OptimizeGroupSwitchEvent{
				Action:         "switch",
				FromGroupID:    ks.groupID,
				FromGroup:      ks.groupName,
				FromMultiplier: ks.multiplier,
				ToGroupID:      cand.ID,
				ToGroup:        cand.Name,
				ToMultiplier:   cand.RateMultiplier,
				Status:         "success",
				OccurredAt:     time.Now().Format(time.RFC3339Nano),
			}
			if err := client.UpdateAPIKeyGroup(ctx, keysPath, *acc.ProviderAPIKeyID, cand.ID); err != nil {
				switchEvent.Status = "failed"
				switchEvent.Reason = err.Error()
				detail.SwitchEvents = append(detail.SwitchEvents, switchEvent)
				lastErr = fmt.Sprintf("切换到 %s 失败: %v", cand.Name, err)
				continue
			}
			detail.SwitchEvents = append(detail.SwitchEvents, switchEvent)
			switchEventIndex = len(detail.SwitchEvents) - 1
		}

		// 实测连接
		triedCount++
		testErr := s.testAccountModel(ctx, acc)
		if testErr == nil {
			if switchEventIndex >= 0 {
				detail.SwitchEvents[switchEventIndex].TestStatus = "passed"
			}
			if isCurrent {
				// 当前分组实测可用 → 无需切换。若此前较低倍率候选
				// 已失败，日志必须说明是复测恢复而不是笼统声称最优。
				detail.Status = "skipped"
				detail.Reason = retainedCurrentGroupReason(detail.SwitchEvents)
				detail.NewGroup = ks.groupName
				detail.NewMult = ks.multiplier
				return detail
			}
			// 切换成功且可用：稳定分组 ID、名称、倍率必须作为一个绑定整体落库。
			if err := s.updateRemoteGroupBinding(ctx, acc.ID, cand.ID, cand.Name, cand.RateMultiplier); err != nil {
				rollbackErr := error(nil)
				if ks.groupID <= 0 {
					detail.Status = "failed"
					detail.Reason = fmt.Sprintf("远端已切换到 %s，但本地状态保存失败且原分组 ID 缺失，无法安全回滚，请立即同步远端状态: %v", cand.Name, err)
					return detail
				}
				rollbackErr = client.UpdateAPIKeyGroup(ctx, keysPath, *acc.ProviderAPIKeyID, ks.groupID)
				rollbackEvent := OptimizeGroupSwitchEvent{
					Action:         "rollback",
					FromGroupID:    cand.ID,
					FromGroup:      cand.Name,
					FromMultiplier: cand.RateMultiplier,
					ToGroupID:      ks.groupID,
					ToGroup:        ks.groupName,
					ToMultiplier:   ks.multiplier,
					Status:         "success",
					Reason:         fmt.Sprintf("本地状态保存失败: %v", err),
					OccurredAt:     time.Now().Format(time.RFC3339Nano),
				}
				if rollbackErr != nil {
					rollbackEvent.Status = "failed"
					rollbackEvent.Reason = fmt.Sprintf("本地状态保存失败: %v; 回滚失败: %v", err, rollbackErr)
				}
				detail.SwitchEvents = append(detail.SwitchEvents, rollbackEvent)
				detail.Status = "failed"
				if rollbackErr != nil {
					detail.Reason = fmt.Sprintf("远端已切换到 %s，但本地状态保存失败且回滚失败，请立即同步远端状态: save=%v, rollback=%v", cand.Name, err, rollbackErr)
				} else {
					detail.Reason = fmt.Sprintf("本地状态保存失败，远端已回滚到原分组: %v", err)
				}
				return detail
			}
			keyStateMap[*acc.ProviderAPIKeyID] = optimizeKeyState{
				groupID: cand.ID, groupName: cand.Name, multiplier: cand.RateMultiplier,
			}
			detail.Status = "optimized"
			detail.NewGroup = cand.Name
			detail.NewMult = cand.RateMultiplier
			return detail
		}
		if switchEventIndex >= 0 {
			detail.SwitchEvents[switchEventIndex].TestStatus = "failed"
			detail.SwitchEvents[switchEventIndex].Reason = testErr.Error()
		}

		// 不可用：切换过来的要回滚到原分组，原地测试的当前分组无需回滚
		if !isCurrent && ks.groupID <= 0 {
			detail.Status = "failed"
			detail.Reason = fmt.Sprintf("分组 %s 测试失败且原分组 ID 缺失，无法安全回滚，请立即同步远端状态: test=%v", cand.Name, testErr)
			return detail
		}
		if !isCurrent {
			rollbackErr := client.UpdateAPIKeyGroup(ctx, keysPath, *acc.ProviderAPIKeyID, ks.groupID)
			rollbackEvent := OptimizeGroupSwitchEvent{
				Action:         "rollback",
				FromGroupID:    cand.ID,
				FromGroup:      cand.Name,
				FromMultiplier: cand.RateMultiplier,
				ToGroupID:      ks.groupID,
				ToGroup:        ks.groupName,
				ToMultiplier:   ks.multiplier,
				Status:         "success",
				Reason:         fmt.Sprintf("候选分组连接测试失败: %v", testErr),
				OccurredAt:     time.Now().Format(time.RFC3339Nano),
			}
			if rollbackErr != nil {
				rollbackEvent.Status = "failed"
				rollbackEvent.Reason = fmt.Sprintf("候选分组连接测试失败: %v; 回滚失败: %v", testErr, rollbackErr)
			}
			detail.SwitchEvents = append(detail.SwitchEvents, rollbackEvent)
			if rollbackErr != nil {
				detail.Status = "failed"
				detail.Reason = fmt.Sprintf("分组 %s 测试失败且回滚原分组失败，请立即同步远端状态: test=%v, rollback=%v", cand.Name, testErr, rollbackErr)
				return detail
			}
		}
		lastErr = fmt.Sprintf("分组 %s 测试失败: %v", cand.Name, testErr)
	}

	// 倍率上限内的候选（有 len(candidates) 个）全部实测不可用：
	// 汇总为「无可用分组」结论，并附最后一次失败详情，便于判断是否需要放宽上限。
	detail.Status = "failed"
	detail.Reason = fmt.Sprintf("倍率上限 ×%g 内无可用分组（共 %d 个候选，已尝试 %d 个均连接失败）：%s",
		maxMult, len(candidates), triedCount, lastErr)
	return detail
}

func retainedCurrentGroupReason(events []OptimizeGroupSwitchEvent) string {
	failedCandidates := 0
	for _, event := range events {
		if event.Action == "switch" && (event.Status == "failed" || event.TestStatus == "failed") {
			failedCandidates++
		}
	}
	if failedCandidates > 0 {
		return fmt.Sprintf("当前分组复测已恢复；此前 %d 个更低倍率候选不可用，保留当前分组", failedCandidates)
	}
	return "当前分组实测可用，已是区间内最优分组"
}

func (s *Sub2APIOptimizeScheduleService) updateRemoteGroupBinding(
	ctx context.Context,
	accountID, groupID int64,
	groupName string,
	multiplier float64,
) error {
	if updater, ok := s.providerSvc.accountRepo.(interface {
		UpdateRemoteGroupBinding(context.Context, int64, int64, string, float64) error
	}); ok {
		return updater.UpdateRemoteGroupBinding(ctx, accountID, groupID, groupName, multiplier)
	}
	return s.providerSvc.accountRepo.UpdateRemoteGroupInfo(ctx, accountID, groupName, multiplier)
}

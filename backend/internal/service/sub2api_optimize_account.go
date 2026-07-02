package service

import (
	"context"
	"fmt"
	"sort"

	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/pkg/sub2api"
)

// OptimizeAccountDetail 记录单个账号的优化结果。
// 既写入定时任务运行日志，也作为手动优化（单个/批量）的同步返回结构，
// json tag 与前端 OptimizeLogDetail 保持一致，前后端与日志三处共用同一契约。
type OptimizeAccountDetail struct {
	AccountID   int64   `json:"account_id"`
	AccountName string  `json:"account_name"`
	Status      string  `json:"status"` // optimized / skipped / failed
	OldGroup    string  `json:"old_group,omitempty"`
	NewGroup    string  `json:"new_group,omitempty"`
	OldMult     float64 `json:"old_multiplier,omitempty"`
	NewMult     float64 `json:"new_multiplier,omitempty"`
	Reason      string  `json:"reason,omitempty"`
}

// optimizeKeyState 表示某个远端 APIKey 的当前分组状态。
type optimizeKeyState struct {
	groupID    int64
	groupName  string
	multiplier float64
}

// optimizeReadyAccounts 过滤出「已开启参与 + 设置了倍率上限 + 已正确关联远端 key」的账号，
// 这三项是执行优化的前置条件（无论定时还是手动触发都必须满足）。
func optimizeReadyAccounts(accounts []Account) []Account {
	var participating []Account
	for _, acc := range accounts {
		// 参与定时优化的前置条件:开关开启 + 上限/下限/测试模型都已填写 + 已关联上游
		if acc.Sub2APIOptimizeEnabled &&
			acc.Sub2APIMaxMultiplier != nil &&
			acc.Sub2APIMinMultiplier != nil &&
			acc.Sub2APITestModel != nil && *acc.Sub2APITestModel != "" &&
			acc.ProviderAPIKeyID != nil {
			participating = append(participating, acc)
		}
	}
	return participating
}

// doRunOptimize 是定时优化的核心执行逻辑。
// 登录一次上游→拉一次 groups + keys→遍历账号按倍率上限找最优分组→切换+测试连接→失败回滚尝试下一个。
func (s *Sub2APIOptimizeScheduleService) doRunOptimize(
	ctx context.Context,
	provider *ent.Sub2APIProvider,
	accounts []Account,
) []OptimizeAccountDetail {
	// 过滤：只处理已开启参与、设置了倍率上限且已正确关联的账号
	participating := optimizeReadyAccounts(accounts)
	if len(participating) == 0 {
		return []OptimizeAccountDetail{}
	}
	return s.optimizeAccounts(ctx, provider, participating)
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

	// 拉一次 groups + keys（用 withAuthRetry 包裹：缓存 token 过期收到 401 时自动重登重试）
	providerID := int64(provider.ID)
	var groups []sub2api.Group
	if err := s.providerSvc.withAuthRetry(ctx, client, providerID, func() error {
		var e error
		groups, e = client.GetGroups(ctx, groupsPath)
		return e
	}); err != nil {
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

	var currentKeys []sub2api.APIKey
	if err := s.providerSvc.withAuthRetry(ctx, client, providerID, func() error {
		var e error
		currentKeys, e = client.GetAPIKeys(ctx, keysPath)
		return e
	}); err != nil {
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
		detail := s.optimizeOneAccount(ctx, client, providerID, &acc, groups, keyStateMap, keysPath)
		details = append(details, detail)
	}

	return details
}

// optimizeOneAccount 处理单个账号：找候选分组→切换+测试→失败回滚尝试下一个。
func (s *Sub2APIOptimizeScheduleService) optimizeOneAccount(
	ctx context.Context,
	client *sub2api.Client,
	providerID int64,
	acc *Account,
	groups []sub2api.Group,
	keyStateMap map[int64]optimizeKeyState,
	keysPath string,
) OptimizeAccountDetail {
	detail := OptimizeAccountDetail{
		AccountID:   acc.ID,
		AccountName: acc.Name,
	}

	ks := keyStateMap[*acc.ProviderAPIKeyID]
	detail.OldGroup = ks.groupName
	detail.OldMult = ks.multiplier

	// 候选分组：平台匹配 + active + 倍率落在 [下限, 上限] 区间，按倍率升序。
	// 下限（min）为 nil 时视为无下限，行为与「只有上限」时完全一致，从最便宜候选开始试。
	// 设了下限后，比下限还便宜的分组（往往是超卖/特价的不稳定区）会被排除；
	// 若当前分组恰好低于下限，它不会成为候选，引擎会主动上切到 ≥下限 的最便宜可用分组，
	// 用一点成本换质量底线。
	maxMult := *acc.Sub2APIMaxMultiplier
	var minMult float64 // 0 表示无下限（倍率恒 ≥ 0）
	if acc.Sub2APIMinMultiplier != nil {
		minMult = *acc.Sub2APIMinMultiplier
	}
	var candidates []sub2api.Group
	for _, g := range groups {
		if g.Platform == acc.Platform && g.Status == "active" &&
			g.RateMultiplier >= minMult && g.RateMultiplier <= maxMult {
			candidates = append(candidates, g)
		}
	}
	if len(candidates) == 0 {
		detail.Status = "skipped"
		detail.Reason = "无符合倍率区间的候选分组"
		return detail
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].RateMultiplier < candidates[j].RateMultiplier
	})

	// 从最便宜开始逐个尝试，每个候选都实测连接可用性。
	// 若最便宜候选就是当前分组，则原地实测而不切换：
	//   - 可用 → skip「已是最优分组」
	//   - 不可用（如分组被上游降级/超卖返回 5xx）→ 继续往上找能用的更贵分组，
	//     修复「卡在最便宜但已不可用分组上、后续永远只跳过」的问题。
	//
	// lastErr 记录最后一个候选的失败详情，triedCount 记录实际尝试数，
	// 供全部失败时汇总出「上限内无可用分组」的结论（而非只抛最后一个候选的原始错误）。
	var lastErr string
	triedCount := 0
	for _, cand := range candidates {
		isCurrent := cand.ID == ks.groupID

		if !isCurrent {
			// 切换分组（withAuthRetry：缓存 token 过期收到 401 时自动重登重试）
			if err := s.providerSvc.withAuthRetry(ctx, client, providerID, func() error {
				return client.UpdateAPIKeyGroup(ctx, keysPath, *acc.ProviderAPIKeyID, cand.ID)
			}); err != nil {
				lastErr = fmt.Sprintf("切换到 %s 失败: %v", cand.Name, err)
				continue
			}
		}

		// 实测连接
		triedCount++
		testErr := s.testAccountModel(ctx, acc)
		if testErr == nil {
			if isCurrent {
				// 当前分组已是最便宜且实测可用 → 无需切换
				detail.Status = "skipped"
				detail.Reason = "已是最优分组"
				detail.NewGroup = ks.groupName
				detail.NewMult = ks.multiplier
				return detail
			}
			// 切换成功且可用：更新缓存并返回
			_ = s.providerSvc.accountRepo.UpdateRemoteGroupInfo(ctx, acc.ID, cand.Name, cand.RateMultiplier)
			detail.Status = "optimized"
			detail.NewGroup = cand.Name
			detail.NewMult = cand.RateMultiplier
			return detail
		}

		// 不可用：切换过来的要回滚到原分组，原地测试的当前分组无需回滚
		if !isCurrent && ks.groupID > 0 {
			_ = s.providerSvc.withAuthRetry(ctx, client, providerID, func() error {
				return client.UpdateAPIKeyGroup(ctx, keysPath, *acc.ProviderAPIKeyID, ks.groupID)
			})
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

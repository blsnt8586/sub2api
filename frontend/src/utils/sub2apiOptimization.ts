import type { LinkedAccountInfo, OptimizeResult, Sub2APIProviderRemoteGroupRate } from '@/api/admin/sub2apiProviders'

/**
 * Remote groups offered by the optional account restriction selector.
 * Runtime optimization applies the same intersection again on the backend.
 */
export function eligibleRemoteOptimizeGroups(
  account: LinkedAccountInfo,
  groups: Sub2APIProviderRemoteGroupRate[]
): Sub2APIProviderRemoteGroupRate[] {
  return groups.filter(group =>
    group.platform === account.platform &&
    (!group.status || group.status === 'active') &&
    (account.sub2api_min_multiplier == null || group.effective_multiplier >= account.sub2api_min_multiplier) &&
    (account.sub2api_max_multiplier == null || group.effective_multiplier <= account.sub2api_max_multiplier)
  )
}

export function selectedOptimizeGroupIDs(account: LinkedAccountInfo): number[] {
  const ids = (account.sub2api_optimize_group_ids ?? [])
    .filter(id => Number.isSafeInteger(id) && id > 0)
  if (ids.length > 0) return [...new Set(ids)].sort((a, b) => a - b)
  const legacy = account.sub2api_optimize_group_id
  return typeof legacy === 'number' && Number.isSafeInteger(legacy) && legacy > 0 ? [legacy] : []
}

/**
 * 将单账号优化结果合并到对应行。
 *
 * 优化接口已经返回最终分组与倍率，因此无需重新加载整个账号面板。
 * failed 结果不会改变远端分组；账号不匹配时也保持原对象引用。
 */
function mergeOptimizeResultIntoAccount(
  account: LinkedAccountInfo,
  result: OptimizeResult
): LinkedAccountInfo {
  if (account.id !== result.account_id || result.status === 'failed') return account

  const hasGroup = result.new_group !== undefined
  const hasMultiplier =
    result.new_multiplier !== undefined && Number.isFinite(result.new_multiplier)

  if (!hasGroup && !hasMultiplier) return account

  return {
    ...account,
    ...(hasGroup ? { remote_group_name: result.new_group } : {}),
    ...(hasMultiplier ? { remote_group_multiplier: result.new_multiplier } : {}),
  }
}

/**
 * 就地更新账号数组中的目标行，保持数组和其他账号对象的引用稳定。
 * 返回 true 表示目标行的分组或倍率已更新。
 */
export function applyOptimizeResultToAccounts(
  accounts: LinkedAccountInfo[],
  result: OptimizeResult
): boolean {
  const accountIndex = accounts.findIndex(account => account.id === result.account_id)
  if (accountIndex === -1) return false

  const current = accounts[accountIndex]
  const updated = mergeOptimizeResultIntoAccount(current, result)
  if (updated === current) return false

  accounts[accountIndex] = updated
  return true
}

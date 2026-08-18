import type { LinkedAccountInfo, OptimizeResult } from '@/api/admin/sub2apiProviders'

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

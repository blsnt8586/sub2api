import { describe, expect, it } from 'vitest'
import type { LinkedAccountInfo, OptimizeResult } from '@/api/admin/sub2apiProviders'
import { applyOptimizeResultToAccounts } from '../sub2apiOptimization'

const account: LinkedAccountInfo = {
  id: 1,
  name: 'OpenAI account',
  platform: 'openai',
  status: 'active',
  provider_id: 7,
  remote_group_name: 'Old group',
  remote_group_multiplier: 1,
  sub2api_optimize_enabled: true,
  sub2api_min_multiplier: 0.1,
  sub2api_max_multiplier: 1,
  sub2api_test_model: 'gpt-5.5',
}

const result = (overrides: Partial<OptimizeResult> = {}): OptimizeResult => ({
  account_id: account.id,
  account_name: account.name,
  status: 'optimized',
  new_group: 'New group',
  new_multiplier: 0.55,
  ...overrides,
})

describe('applyOptimizeResultToAccounts', () => {
  it('updates only the target row while preserving the list and other row identities', () => {
    const otherAccount: LinkedAccountInfo = { ...account, id: 2, name: 'Other account' }
    const accounts = [account, otherAccount]
    const originalList = accounts

    expect(applyOptimizeResultToAccounts(accounts, result())).toBe(true)
    expect(accounts).toBe(originalList)
    expect(accounts[0]).not.toBe(account)
    expect(accounts[0]).toEqual({
      ...account,
      remote_group_name: 'New group',
      remote_group_multiplier: 0.55,
    })
    expect(accounts[1]).toBe(otherAccount)
  })

  it('accepts the current group returned by a skipped optimization', () => {
    const accounts = [account]
    expect(applyOptimizeResultToAccounts(
      accounts,
      result({ status: 'skipped', new_group: 'Old group', new_multiplier: 1 })
    )).toBe(true)

    expect(accounts[0].remote_group_name).toBe('Old group')
    expect(accounts[0].remote_group_multiplier).toBe(1)
  })

  it('does not mutate the row for failed or unrelated results', () => {
    const failedAccounts = [account]
    expect(applyOptimizeResultToAccounts(failedAccounts, result({ status: 'failed' }))).toBe(false)
    expect(failedAccounts[0]).toBe(account)

    const unrelatedAccounts = [account]
    expect(applyOptimizeResultToAccounts(unrelatedAccounts, result({ account_id: 2 }))).toBe(false)
    expect(unrelatedAccounts[0]).toBe(account)
  })
})

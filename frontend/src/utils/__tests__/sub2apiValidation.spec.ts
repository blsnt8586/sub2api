import { describe, expect, it } from 'vitest'
import {
  findIncompleteParticipatingAccounts,
  getMultiplierRangeState,
  isOptimizeConfigComplete,
} from '../sub2apiValidation'

describe('sub2api optimization validation', () => {
  it('requires max, min, and an explicit test model', () => {
    expect(
      isOptimizeConfigComplete({
        sub2api_min_multiplier: 0.3,
        sub2api_max_multiplier: 0.8,
        sub2api_test_model: 'test-model',
      })
    ).toBe(true)
    expect(
      isOptimizeConfigComplete({
        sub2api_min_multiplier: null,
        sub2api_max_multiplier: 0.8,
        sub2api_test_model: 'test-model',
      })
    ).toBe(false)
    expect(
      isOptimizeConfigComplete({
        sub2api_min_multiplier: 0.3,
        sub2api_max_multiplier: 0.8,
        sub2api_test_model: '   ',
      })
    ).toBe(false)
  })

  it('only reports incomplete accounts that opted into optimization', () => {
    const accounts = [
      { name: 'disabled', sub2api_optimize_enabled: false },
      { name: 'invalid', sub2api_optimize_enabled: true },
      {
        name: 'ready',
        sub2api_optimize_enabled: true,
        sub2api_min_multiplier: 0.3,
        sub2api_max_multiplier: 0.8,
        sub2api_test_model: 'test-model',
      },
    ]
    expect(findIncompleteParticipatingAccounts(accounts).map(account => account.name)).toEqual([
      'invalid',
    ])
  })

  it('compares the current multiplier with the account-specific range', () => {
    expect(getMultiplierRangeState(1, 0.2, 0.5)).toBe('above')
    expect(getMultiplierRangeState(0.1, 0.2, 0.5)).toBe('below')
    expect(getMultiplierRangeState(0.4, 0.2, 0.5)).toBe('within')
    expect(getMultiplierRangeState(0.4, null, null)).toBe('unbounded')
  })
})

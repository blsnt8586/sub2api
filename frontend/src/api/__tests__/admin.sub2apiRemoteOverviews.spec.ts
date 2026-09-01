import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, post, put } = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: { get, post, put },
}))

import { getCachedRemoteOverviews, optimizeAll, updateAccountOptimizeSettings } from '@/api/admin/sub2apiProviders'

describe('admin Sub2API cached remote overview API', () => {
  beforeEach(() => {
    get.mockReset()
    post.mockReset()
    put.mockReset()
    get.mockResolvedValue({ data: [] })
    put.mockResolvedValue({ data: { message: 'ok' } })
  })

  it('loads visible Provider snapshots in one local cache request', async () => {
    await expect(getCachedRemoteOverviews([7, 9, 11])).resolves.toEqual([])

    expect(get).toHaveBeenCalledOnce()
    expect(get).toHaveBeenCalledWith('/admin/sub2api-providers/remote-overviews', {
      params: { ids: '7,9,11' },
    })
  })

  it('skips the request when the current page has no Providers', async () => {
    await expect(getCachedRemoteOverviews([])).resolves.toEqual([])
    expect(get).not.toHaveBeenCalled()
  })

  it('normalizes a null batch result from an empty provider to a safe empty list', async () => {
    post.mockResolvedValueOnce({ data: { total: 0, optimized: 0, skipped: 0, failed: 0, results: null } })

    await expect(optimizeAll(7)).resolves.toMatchObject({
      total: 0,
      optimized: 0,
      skipped: 0,
      failed: 0,
      results: [],
    })
  })

  it('persists an optional remote group restriction with the account settings', async () => {
    const payload = {
      enabled: true,
      min_multiplier: 0.05,
      max_multiplier: 0.16,
      test_model: 'gpt-5.6-sol',
      group_id: 41,
    }

    await expect(updateAccountOptimizeSettings(7, 9, payload)).resolves.toEqual({ message: 'ok' })
    expect(put).toHaveBeenCalledWith('/admin/sub2api-providers/7/accounts/9/optimize-settings', payload)
  })
})

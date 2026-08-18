import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get } = vi.hoisted(() => ({
  get: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: { get },
}))

import { getCachedRemoteOverviews } from '@/api/admin/sub2apiProviders'

describe('admin Sub2API cached remote overview API', () => {
  beforeEach(() => {
    get.mockReset()
    get.mockResolvedValue({ data: [] })
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
})

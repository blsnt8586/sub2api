import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get } = vi.hoisted(() => ({
  get: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: { get },
}))

import { listOptimizeLogs } from '@/api/admin/sub2apiProviders'

describe('admin Sub2API optimize log API', () => {
  beforeEach(() => {
    get.mockReset()
    get.mockResolvedValue({
      data: { items: [], total: 0, page: 2, page_size: 20, pages: 0 },
    })
  })

  it('serializes Provider audit filters and pagination', async () => {
    await listOptimizeLogs(7, {
      trigger: 'probe_unhealthy',
      status: 'failed',
      account_id: 42,
      keyword: '  economy group  ',
      from: '2026-08-15T00:00:00.000Z',
      to: '2026-08-16T00:00:00.000Z',
      page: 2,
      page_size: 20,
    })

    expect(get).toHaveBeenCalledWith('/admin/sub2api-providers/7/optimize-logs', {
      params: {
        trigger: 'probe_unhealthy',
        status: 'failed',
        account_id: 42,
        keyword: 'economy group',
        from: '2026-08-15T00:00:00.000Z',
        to: '2026-08-16T00:00:00.000Z',
        page: 2,
        page_size: 20,
      },
    })
  })

  it('omits empty filters and applies stable pagination defaults', async () => {
    await listOptimizeLogs(7, {
      trigger: '',
      status: '',
      keyword: '   ',
    })

    expect(get).toHaveBeenCalledWith('/admin/sub2api-providers/7/optimize-logs', {
      params: { page: 1, page_size: 20 },
    })
  })
})

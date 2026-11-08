import { describe, expect, it, vi } from 'vitest'
import { listMyFeedback, replyMyFeedback } from '../feedback'

const get = vi.hoisted(() => vi.fn())
const post = vi.hoisted(() => vi.fn())
vi.mock('../client', () => ({ apiClient: { get, post } }))

describe('feedback history API', () => {
  it('normalizes an empty server list to an array and requests the requested page', async () => {
    get.mockResolvedValueOnce({ data: { items: null, has_more: false } })

    await expect(listMyFeedback(20)).resolves.toEqual({ items: [], has_more: false })
    expect(get).toHaveBeenCalledWith('/feedback', { params: { limit: 10, offset: 20 } })
  })

  it('posts a user reply to its existing feedback thread', async () => {
    const thread = { id: 12 }
    const form = new FormData()
    post.mockResolvedValueOnce({ data: thread })

    await expect(replyMyFeedback(12, form)).resolves.toEqual(thread)
    expect(post).toHaveBeenCalledWith('/feedback/12/reply', form, {
      headers: { 'Content-Type': 'multipart/form-data' },
    })
  })
})

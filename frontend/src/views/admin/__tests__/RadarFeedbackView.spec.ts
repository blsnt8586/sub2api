import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import RadarFeedbackView from '../RadarFeedbackView.vue'
import type { FeedbackAdminThread, FeedbackInboxUser, FeedbackThreadSummary } from '@/api/feedback'

const api = vi.hoisted(() => ({
  listFeedbackUsers: vi.fn(), listUserFeedback: vi.fn(), getFeedbackThread: vi.fn(),
  markFeedbackRead: vi.fn(), replyFeedback: vi.fn(),
}))
const showError = vi.hoisted(() => vi.fn())
const showSuccess = vi.hoisted(() => vi.fn())
vi.mock('@/api/feedback', () => api)
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError, showSuccess }) }))
vi.mock('vue-i18n', async importOriginal => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: () => '留言管理' }),
}))

const date = '2026-11-06T10:00:00Z'
const makeUser = (id: number): FeedbackInboxUser => ({ user_id: id, username: `用户${id}`, email: `user${id}@example.test`, thread_count: 2, unread_count: 2, updated_at: date, preview: '最新消息' })
const summary = (id: number, userID: number): FeedbackThreadSummary => ({ id, user_id: userID, subject: `问题${id}`, status: 'open', created_at: date, updated_at: date, unread_count: 1, last_message_id: id * 10, preview: `摘要${id}` })
const detail = (id: number): FeedbackAdminThread => ({
  ...summary(id, id < 20 ? 7 : 8), unread_count: 1, last_user_message_id: id * 10,
  messages: [{ id: id * 10, author_type: 'user', author_id: id < 20 ? 7 : 8, content: `完整内容${id}`, created_at: date, attachments: [] }],
})
const wrappers: VueWrapper[] = []
function mountView() {
  const w = mount(RadarFeedbackView, { global: { stubs: { AppLayout: { template: '<div><slot /></div>' }, Icon: true } } })
  wrappers.push(w)
  return w
}
async function settle() { for (let i = 0; i < 4; i++) await flushPromises() }

describe('three-column feedback inbox', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    Object.values(api).forEach(mock => mock.mockReset())
    showError.mockReset(); showSuccess.mockReset()
    api.listFeedbackUsers.mockImplementation(async () => ({ items: [makeUser(7), makeUser(8)], has_more: false }))
    api.listUserFeedback.mockImplementation(async (id: number) => ({ items: id === 7 ? [summary(11, 7), summary(12, 7)] : [summary(21, 8)], has_more: false }))
    api.getFeedbackThread.mockImplementation(async (id: number) => detail(id))
    api.markFeedbackRead.mockResolvedValue(undefined)
    api.replyFeedback.mockResolvedValue(undefined)
  })
  afterEach(() => {
    wrappers.splice(0).forEach(w => w.unmount())
    vi.useRealTimers()
  })

  it('groups senders, isolates each user history and loads only the selected conversation', async () => {
    const w = mountView(); await settle()
    expect(w.get('[aria-label="留言用户列表"]').findAll('[data-testid^="feedback-user-"]')).toHaveLength(2)
    expect(w.get('[aria-label="所选用户的会话历史"]').text()).toContain('问题11')
    expect(w.get('[aria-label="对话内容"]').text()).toContain('完整内容11')
    expect(api.getFeedbackThread).toHaveBeenCalledTimes(1)
    expect(api.markFeedbackRead).toHaveBeenCalledWith(11, 110)
    expect(api.markFeedbackRead).not.toHaveBeenCalledWith(12, 120)
    await w.get('[data-testid="feedback-user-8"]').trigger('click'); await settle()
    expect(api.listUserFeedback).toHaveBeenLastCalledWith(8, '', 0)
    expect(w.get('[aria-label="所选用户的会话历史"]').text()).not.toContain('问题11')
    expect(w.get('[aria-label="对话内容"]').text()).toContain('完整内容21')
    const row = w.get('[data-testid="admin-feedback-thread-21"]')
    expect(row.findAll('time')).toHaveLength(1)
    expect(row.get('time').attributes('datetime')).toBe(date)
    expect(row.get('time').attributes('title')).toContain('2026/11/06')
    expect(row.text()).not.toContain('创建')
    expect(row.text()).not.toContain('更新')
  })

  it('retains each conversation draft and replies to the active thread', async () => {
    const w = mountView(); await settle()
    await w.get('textarea').setValue('已定位原因')
    await w.get('[data-testid="admin-feedback-thread-12"]').trigger('click'); await settle()
    await w.get('textarea').setValue('另一个回复')
    await w.get('[data-testid="admin-feedback-thread-11"]').trigger('click'); await settle()
    expect((w.get('textarea').element as HTMLTextAreaElement).value).toBe('已定位原因')
    await w.get('form').trigger('submit'); await settle()
    expect(api.replyFeedback).toHaveBeenCalledWith(11, expect.any(FormData))
    expect((api.replyFeedback.mock.calls[0][1] as FormData).get('content')).toBe('已定位原因')
    expect(showSuccess).toHaveBeenCalledWith('回复已发送')
  })

  it('ignores a stale detail response after changing users', async () => {
    let resolveOld!: (value: FeedbackAdminThread) => void
    api.getFeedbackThread.mockImplementation((id: number) => id === 11 ? new Promise(resolve => { resolveOld = resolve }) : Promise.resolve(detail(id)))
    const w = mountView(); await settle()
    await w.get('[data-testid="feedback-user-8"]').trigger('click'); await settle()
    resolveOld(detail(11)); await settle()
    expect(w.get('[aria-label="对话内容"]').text()).toContain('完整内容21')
    expect(api.markFeedbackRead).not.toHaveBeenCalledWith(11, 110)
  })

  it('updates unread priority without jumping away from the active conversation', async () => {
    const w = mountView(); await settle()
    api.listFeedbackUsers.mockResolvedValue({ items: [makeUser(8), makeUser(7)], has_more: false })
    await vi.advanceTimersByTimeAsync(15000); await settle()
    const buttons = w.get('[aria-label="留言用户列表"]').findAll('[data-testid^="feedback-user-"]')
    expect(buttons[0].attributes('data-testid')).toBe('feedback-user-8')
    expect(w.get('[aria-label="对话内容"]').text()).toContain('完整内容11')
  })

  it('keeps the open conversation current when a new message moves it outside the filter', async () => {
    const w = mountView(); await settle()
    api.listUserFeedback.mockResolvedValue({ items: [], has_more: false })
    api.getFeedbackThread.mockResolvedValue({ ...detail(11), status: 'closed', messages: [...detail(11).messages, { ...detail(11).messages[0], id: 111, author_type: 'admin', content: '已结束对话' }] })
    await vi.advanceTimersByTimeAsync(15000); await settle()
    expect(w.get('[aria-label="对话内容"]').text()).toContain('已结束对话')
    expect(w.find('textarea').exists()).toBe(false)
    expect(w.get('[data-testid="feedback-user-7"]').attributes('aria-pressed')).toBe('true')
  })

  it('keeps a failed acknowledgement unread and retries it', async () => {
    api.markFeedbackRead.mockRejectedValueOnce(new Error('offline'))
    const w = mountView(); await settle()
    expect(w.get('[aria-label="所选用户的会话历史"]').text()).toContain('1 条未读')
    await w.get('[aria-label="对话内容"]').trigger('scroll'); await settle()
    expect(api.markFeedbackRead).toHaveBeenCalledTimes(2)
  })

  it('filters and paginates the selected user history', async () => {
    api.listUserFeedback.mockImplementation(async (id: number, state: string, offset: number) => ({ items: [summary(offset ? 12 : 11, id)], has_more: !offset }))
    const w = mountView(); await settle()
    await w.get('select').setValue('replied'); await settle()
    expect(api.listUserFeedback).toHaveBeenLastCalledWith(7, 'replied', 0)
    const next = w.get('[aria-label="所选用户的会话历史"]').findAll('button').find(b => b.text() === '下一页')!
    await next.trigger('click'); await settle()
    expect(api.listUserFeedback).toHaveBeenLastCalledWith(7, 'replied', 20)
  })

  it('does not acknowledge a new message while the administrator reads older messages', async () => {
    const w = mountView(); await settle()
    api.markFeedbackRead.mockClear()
    const el = w.get('[aria-label="对话内容"]').element as HTMLElement
    Object.defineProperty(el, 'scrollHeight', { configurable: true, value: 1000 })
    Object.defineProperty(el, 'clientHeight', { configurable: true, value: 200 })
    el.scrollTop = 0
    api.listUserFeedback.mockResolvedValue({ items: [{ ...summary(11, 7), last_message_id: 111 }], has_more: false })
    api.getFeedbackThread.mockResolvedValue({ ...detail(11), unread_count: 1, last_user_message_id: 111, messages: [...detail(11).messages, { ...detail(11).messages[0], id: 111, content: '刚到的新消息' }] })
    await vi.advanceTimersByTimeAsync(15000); await settle()
    expect(w.get('[aria-label="对话内容"]').text()).toContain('刚到的新消息')
    expect(api.markFeedbackRead).not.toHaveBeenCalled()
    el.scrollTop = 800
    await w.get('[aria-label="对话内容"]').trigger('scroll'); await settle()
    expect(api.markFeedbackRead).toHaveBeenCalledWith(11, 111)
  })
})

import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import FeedbackBoard from '../FeedbackBoard.vue'
import type { FeedbackThread, FeedbackThreadPage } from '@/api/feedback'

const listMyFeedback = vi.hoisted(() => vi.fn())
const createFeedback = vi.hoisted(() => vi.fn())
const replyMyFeedback = vi.hoisted(() => vi.fn())
const showError = vi.hoisted(() => vi.fn())
const showSuccess = vi.hoisted(() => vi.fn())

vi.mock('@/api/feedback', () => ({ listMyFeedback, createFeedback, replyMyFeedback }))
vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError, showSuccess }),
}))

const makeThread = (id: number, subject: string, status: FeedbackThread['status'] = 'replied'): FeedbackThread => ({
  id,
  user_id: 7,
  subject,
  status,
  created_at: '2026-11-01T10:00:00Z',
  updated_at: '2026-11-01T11:00:00Z',
  messages: [
    {
      id: id * 10,
      author_type: 'user',
      author_id: 7,
      content: `用户留言 ${id}`,
      created_at: '2026-11-01T10:00:00Z',
      attachments: [],
    },
    {
      id: id * 10 + 1,
      author_type: 'admin',
      author_id: 1,
      content: `管理员回复 ${id}`,
      created_at: '2026-11-01T11:00:00Z',
      attachments: [],
    },
  ],
})

const page = (items: FeedbackThread[], has_more = false): FeedbackThreadPage => ({ items, has_more })
const wrappers: VueWrapper[] = []

function mountBoard() {
  const wrapper = mount(FeedbackBoard, { global: { stubs: { Icon: true } } })
  wrappers.push(wrapper)
  return wrapper
}

describe('FeedbackBoard', () => {
  beforeEach(() => {
    listMyFeedback.mockReset().mockResolvedValue(page([]))
    createFeedback.mockReset()
    replyMyFeedback.mockReset()
    showError.mockReset()
    showSuccess.mockReset()
  })

  afterEach(() => {
    wrappers.splice(0).forEach(wrapper => wrapper.unmount())
  })

  it('opens the emoji picker as an absolutely positioned popover and inserts at the caret', async () => {
    const wrapper = mountBoard()
    await flushPromises()
    const textarea = wrapper.get('textarea').element as HTMLTextAreaElement
    await wrapper.get('textarea').setValue('hello world')
    textarea.focus()
    textarea.setSelectionRange(5, 5)

    await wrapper.get('button[aria-haspopup="dialog"]').trigger('click')
    const popover = wrapper.get('[data-testid="emoji-popover"]')
    expect(popover.classes()).toContain('absolute')
    expect(popover.element.parentElement?.classList.contains('relative')).toBe(true)

    await wrapper.get('[aria-label="插入 😀"]').trigger('click')
    expect((wrapper.get('textarea').element as HTMLTextAreaElement).value).toBe('hello😀 world')
  })

  it('opens a selected history item as a conversation with both sides visible', async () => {
    listMyFeedback.mockResolvedValue(page([
      makeThread(1, '最近问题'),
      makeThread(2, '之前的问题', 'open'),
    ]))
    const wrapper = mountBoard()
    await flushPromises()
    await wrapper.get('[data-testid="feedback-thread-2"]').trigger('click')

    expect(wrapper.text()).toContain('之前的问题')
    expect(wrapper.text()).toContain('用户留言 2')
    expect(wrapper.text()).toContain('管理员回复 2')
    expect(wrapper.text()).toContain('待管理员回复')
  })

  it('sends a reply into the selected thread instead of creating another message', async () => {
    listMyFeedback.mockResolvedValue(page([makeThread(12, '账号问题', 'replied')]))
    replyMyFeedback.mockResolvedValue(makeThread(12, '账号问题', 'open'))
    const wrapper = mountBoard()
    await flushPromises()
    await wrapper.get('textarea').setValue('补充一下问题')
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(replyMyFeedback).toHaveBeenCalledWith(12, expect.any(FormData))
    expect(createFeedback).not.toHaveBeenCalled()
    expect(showSuccess).toHaveBeenCalledWith('回复已发送')
  })

  it('creates a new conversation from the new-message form', async () => {
    const created = makeThread(21, '新的反馈', 'open')
    listMyFeedback.mockResolvedValueOnce(page([])).mockResolvedValueOnce(page([created]))
    createFeedback.mockResolvedValue(created)
    const wrapper = mountBoard()
    await flushPromises()
    await wrapper.get('input').setValue('新的反馈')
    await wrapper.get('textarea').setValue('这是第一条消息')
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(createFeedback).toHaveBeenCalledWith(expect.any(FormData))
    expect((createFeedback.mock.calls[0][0] as FormData).get('subject')).toBe('新的反馈')
    expect(replyMyFeedback).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('用户留言 21')
    expect(wrapper.text()).toContain('待管理员回复')
  })

  it('keeps closed conversations read-only and offers a new conversation', async () => {
    listMyFeedback.mockResolvedValue(page([makeThread(31, '已结束问题', 'closed')]))
    const wrapper = mountBoard()
    await flushPromises()

    expect(wrapper.text()).toContain('这条对话已结束')
    expect(wrapper.find('textarea').exists()).toBe(false)
    await wrapper.get('button').trigger('click')
    expect(wrapper.text()).toContain('新留言')
  })

  it('loads and appends older history pages', async () => {
    const recent = Array.from({ length: 10 }, (_, index) => makeThread(index + 1, `最近留言 ${index + 1}`))
    listMyFeedback
      .mockResolvedValueOnce(page(recent, true))
      .mockResolvedValueOnce(page([makeThread(11, '更早的留言')]))

    const wrapper = mountBoard()
    await flushPromises()
    await wrapper.get('[data-testid="load-more-feedback"]').trigger('click')
    await flushPromises()

    expect(listMyFeedback).toHaveBeenNthCalledWith(2, 10)
    expect(wrapper.text()).toContain('更早的留言')
  })
})

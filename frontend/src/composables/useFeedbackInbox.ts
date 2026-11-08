import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import {
  getFeedbackThread, listFeedbackUsers, listUserFeedback, markFeedbackRead, replyFeedback,
  type FeedbackAdminThread, type FeedbackInboxUser, type FeedbackThreadSummary,
} from '@/api/feedback'
import { useAppStore } from '@/stores/app'

export function useFeedbackInbox() {
  const app = useAppStore()
  const users = ref<FeedbackInboxUser[]>([])
  const threads = ref<FeedbackThreadSummary[]>([])
  const selectedUser = ref<FeedbackInboxUser | null>(null)
  const selectedThreadId = ref<number | null>(null)
  const selectedThread = ref<FeedbackAdminThread | null>(null)
  const userPage = ref(0)
  const threadPage = ref(0)
  const usersHasMore = ref(false)
  const threadsHasMore = ref(false)
  const status = ref('')
  const usersLoading = ref(false)
  const threadsLoading = ref(false)
  const detailLoading = ref(false)
  const usersError = ref(false)
  const threadsError = ref(false)
  const detailError = ref(false)
  const refreshing = ref(false)
  const sending = ref(0)
  const mobilePane = ref<'users' | 'threads' | 'conversation'>('users')
  const drafts = reactive<Record<number, string>>({})
  const transcriptContainer = ref<HTMLElement | null>(null)
  const loading = computed(() => refreshing.value || usersLoading.value || threadsLoading.value || detailLoading.value)
  let alive = true
  let userRequest = 0
  let threadRequest = 0
  let detailRequest = 0
  let timer: ReturnType<typeof setInterval> | undefined
  const reading = new Set<number>()

  function atBottom() {
    const el = transcriptContainer.value
    return !!el && el.scrollHeight - el.scrollTop - el.clientHeight < 24
  }

  async function markVisibleRead() {
    const item = selectedThread.value
    if (!alive || document.visibilityState === 'hidden' || detailLoading.value || !item || !atBottom()) return
    if (window.innerWidth < 1024 && mobilePane.value !== 'conversation') return
    if (item.unread_count === 0 || !item.last_user_message_id || reading.has(item.id)) return
    const watermark = item.last_user_message_id
    const count = item.unread_count
    reading.add(item.id)
    try {
      await markFeedbackRead(item.id, watermark)
      if (!alive) return
      const current = selectedThread.value
      if (current?.id === item.id) {
        current.unread_count = current.messages.filter(m => m.author_type === 'user' && m.id > watermark).length
      }
      const summary = threads.value.find(t => t.id === item.id)
      if (summary) summary.unread_count = Math.max(0, summary.unread_count - count)
      // Re-query aggregates so a newly arrived message stays unread and sorting is authoritative.
      await loadUsers(true)
    } catch {
      // Preserve the unread badge; a later refresh or scroll retries acknowledgement.
    } finally {
      reading.delete(item.id)
    }
  }

  async function openThread(id: number, background = false) {
    const owner = selectedUser.value?.user_id
    const request = ++detailRequest
    const keepPosition = background && selectedThread.value?.id === id
    const follow = !keepPosition || atBottom()
    const oldScrollTop = transcriptContainer.value?.scrollTop ?? 0
    selectedThreadId.value = id
    if (!keepPosition) selectedThread.value = null
    detailLoading.value = true
    detailError.value = false
    try {
      const item = await getFeedbackThread(id)
      if (!alive || request !== detailRequest || owner !== selectedUser.value?.user_id || selectedThreadId.value !== id) return
      selectedThread.value = item
      await nextTick()
      if (!alive || request !== detailRequest) return
      const el = transcriptContainer.value
      if (el) el.scrollTop = follow ? el.scrollHeight : oldScrollTop
    } catch {
      if (alive && request === detailRequest) detailError.value = true
    } finally {
      if (alive && request === detailRequest) {
        detailLoading.value = false
        if (!detailError.value && follow) void markVisibleRead()
      }
    }
  }

  async function loadThreads(resetSelection = false, silent = false) {
    const owner = selectedUser.value?.user_id
    if (!owner) return
    const request = ++threadRequest
    const filter = status.value
    const page = threadPage.value
    if (resetSelection) {
      ++detailRequest
      selectedThreadId.value = null
      selectedThread.value = null
      detailLoading.value = false
      detailError.value = false
    }
    if (!silent) threadsLoading.value = true
    threadsError.value = false
    try {
      const result = await listUserFeedback(owner, filter, page * 20)
      if (!alive || request !== threadRequest || owner !== selectedUser.value?.user_id) return
      threads.value = result.items
      threadsHasMore.value = result.has_more
      if (selectedThreadId.value === null) {
        const first = result.items.find(t => t.unread_count > 0) ?? result.items[0]
        if (first) await openThread(first.id)
      } else {
        const summary = result.items.find(t => t.id === selectedThreadId.value)
        const current = selectedThread.value
        if (summary && (!current || summary.last_message_id !== Math.max(0, ...current.messages.map(m => m.id)) || summary.status !== current.status)) {
          await openThread(summary.id, true)
        } else if (!summary) {
          // A new reply can move this conversation off the current page or status filter.
          // Keep the explicit selection, but still update its messages and status.
          await openThread(selectedThreadId.value, true)
        }
      }
    } catch {
      if (alive && request === threadRequest) threadsError.value = true
    } finally {
      if (alive && request === threadRequest) threadsLoading.value = false
    }
  }

  async function selectUser(user: FeedbackInboxUser, fromClick = true) {
    selectedUser.value = user
    threadPage.value = 0
    threads.value = []
    threadsHasMore.value = false
    if (fromClick) mobilePane.value = 'threads'
    await loadThreads(true)
  }

  async function loadUsers(silent = false) {
    const request = ++userRequest
    if (!silent) usersLoading.value = true
    usersError.value = false
    try {
      const result = await listFeedbackUsers(userPage.value * 20)
      if (!alive || request !== userRequest) return
      users.value = result.items
      usersHasMore.value = result.has_more
      const current = result.items.find(u => u.user_id === selectedUser.value?.user_id)
      if (current) selectedUser.value = current
      if (!selectedUser.value && result.items[0]) await selectUser(result.items[0], false)
    } catch {
      if (alive && request === userRequest) usersError.value = true
    } finally {
      if (alive && request === userRequest) usersLoading.value = false
    }
  }

  async function selectThread(thread: FeedbackThreadSummary) {
    mobilePane.value = 'conversation'
    await openThread(thread.id)
  }

  async function refresh() {
    if (refreshing.value || sending.value || !alive) return
    refreshing.value = true
    try {
      await loadUsers(true)
      await loadThreads(false, true)
      await markVisibleRead()
    } finally {
      if (alive) refreshing.value = false
    }
  }

  async function changeUserPage(delta: number) {
    if (usersLoading.value) return
    userPage.value = Math.max(0, userPage.value + delta)
    await loadUsers()
  }

  async function changeThreadPage(delta: number) {
    if (threadsLoading.value) return
    threadPage.value = Math.max(0, threadPage.value + delta)
    await loadThreads(true)
  }

  async function reply(close = false) {
    const thread = selectedThread.value
    const content = thread && drafts[thread.id]?.trim()
    if (!thread || !content || sending.value || thread.status === 'closed') return
    sending.value = thread.id
    try {
      const form = new FormData()
      form.set('content', content)
      form.set('close', String(close))
      await replyFeedback(thread.id, form)
      if (!alive) return
      drafts[thread.id] = ''
      if (selectedThreadId.value === thread.id) await openThread(thread.id, true)
      await loadUsers(true)
      await loadThreads(false, true)
      app.showSuccess(close ? '已回复并结束对话' : '回复已发送')
    } catch {
      if (alive) app.showError('回复发送失败，已保留草稿')
    } finally {
      if (alive) sending.value = 0
    }
  }

  function onVisibilityChange() {
    if (document.visibilityState === 'visible') void refresh()
  }

  watch(status, () => { threadPage.value = 0; void loadThreads(true) })
  watch(mobilePane, async pane => {
    if (pane === 'conversation') { await nextTick(); void markVisibleRead() }
  })
  onMounted(() => {
    void loadUsers()
    timer = setInterval(() => {
      if (document.visibilityState !== 'hidden') void refresh()
    }, 15000)
    document.addEventListener('visibilitychange', onVisibilityChange)
  })
  onBeforeUnmount(() => {
    alive = false
    ++userRequest; ++threadRequest; ++detailRequest
    if (timer) clearInterval(timer)
    document.removeEventListener('visibilitychange', onVisibilityChange)
  })

  return {
    users, threads, selectedUser, selectedThread, selectedThreadId, status, userPage, threadPage,
    usersHasMore, threadsHasMore, usersLoading, threadsLoading, detailLoading, usersError, threadsError,
    detailError, loading, sending, drafts, mobilePane, transcriptContainer,
    selectUser, selectThread, openThread, refresh, loadUsers, loadThreads, changeUserPage, changeThreadPage,
    reply, markVisibleRead,
  }
}

<template>
  <section class="grid min-h-0 overflow-visible rounded-xl border border-gray-200 bg-white shadow-sm dark:border-dark-700 dark:bg-dark-900 lg:h-[calc(100dvh-13.5rem)] lg:min-h-[32rem] lg:max-h-[54rem] lg:grid-cols-[19rem_minmax(0,1fr)] lg:overflow-hidden">
    <aside class="flex min-h-0 min-w-0 flex-col border-b border-gray-200 dark:border-dark-700 lg:overflow-hidden lg:border-b-0 lg:border-r" aria-label="留言历史">
      <header class="flex shrink-0 items-center justify-between gap-3 border-b border-gray-100 px-4 py-4 dark:border-dark-700">
        <div class="min-w-0">
          <h2 class="text-sm font-semibold text-gray-900 dark:text-white">历史留言</h2>
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">与管理员的对话记录</p>
        </div>
        <button
          type="button"
          class="inline-flex min-h-10 shrink-0 items-center gap-1.5 rounded-lg bg-primary-600 px-3 text-sm font-medium text-white transition-colors hover:bg-primary-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 focus-visible:ring-offset-2 disabled:cursor-wait disabled:opacity-60 dark:focus-visible:ring-offset-dark-900"
          :disabled="sending"
          @click="startNewFeedback"
        >
          <Icon name="plus" size="sm" />
          新留言
        </button>
      </header>

      <div v-if="loading && threads.length === 0" class="px-4 py-8 text-center text-sm text-gray-500 dark:text-gray-400">
        正在加载留言…
      </div>
      <div v-else-if="loadFailed && threads.length === 0" class="px-4 py-8 text-center">
        <p class="text-sm text-rose-700 dark:text-rose-300">留言记录暂时无法加载</p>
        <button type="button" class="mt-2 min-h-10 px-3 text-sm font-medium text-primary-700 underline underline-offset-2 dark:text-primary-300" @click="load">
          重试
        </button>
      </div>
      <div v-else-if="threads.length === 0" class="px-4 py-8 text-center">
        <span class="mx-auto flex h-10 w-10 items-center justify-center rounded-full bg-gray-100 text-gray-500 dark:bg-dark-800 dark:text-gray-400">
          <Icon name="inbox" size="md" />
        </span>
        <p class="mt-3 text-sm font-medium text-gray-800 dark:text-gray-200">还没有对话</p>
        <p class="mt-1 text-xs leading-5 text-gray-500 dark:text-gray-400">提交新留言后，对话会保存在这里。</p>
      </div>
      <div v-else class="max-h-[15rem] min-h-0 flex-1 feedback-scrollbar overflow-y-auto overscroll-contain lg:max-h-none">
        <button
          v-for="thread in threads"
          :key="thread.id"
          type="button"
          class="block min-h-20 w-full border-b border-gray-100 px-4 py-3 text-left transition-colors hover:bg-gray-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-primary-500 dark:border-dark-700 dark:hover:bg-dark-800"
          :data-testid="`feedback-thread-${thread.id}`"
          :class="selectedThreadId === thread.id ? 'bg-primary-50/70 dark:bg-primary-950/30' : 'bg-transparent'"
          :aria-pressed="selectedThreadId === thread.id"
          @click="selectThread(thread)"
        >
          <div class="flex items-start justify-between gap-2">
            <span class="min-w-0 truncate text-sm font-medium text-gray-900 dark:text-white">
              {{ thread.subject || '未命名留言' }}
            </span>
            <span class="shrink-0 rounded-full px-2 py-0.5 text-[11px] font-medium" :class="statusClass(thread.status)">
              {{ statusLabel(thread.status) }}
            </span>
          </div>
          <p class="mt-1 truncate text-xs leading-5 text-gray-500 dark:text-gray-400">{{ previewText(thread) }}</p>
          <time class="mt-1 block text-[11px] text-gray-400 dark:text-gray-500">{{ formatCompactDate(thread.updated_at) }}</time>
        </button>
        <div v-if="hasMore" class="p-3">
          <button
            type="button"
            data-testid="load-more-feedback"
            class="min-h-10 w-full rounded-lg text-sm font-medium text-primary-700 transition-colors hover:bg-primary-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 disabled:cursor-wait disabled:opacity-60 dark:text-primary-300 dark:hover:bg-primary-950/30"
            :disabled="loadingMore"
            @click="loadMore"
          >
            {{ loadingMore ? '加载中…' : '加载更早留言' }}
          </button>
        </div>
      </div>
    </aside>

    <section ref="conversationPanel" class="flex h-[36rem] min-h-0 min-w-0 flex-col overflow-hidden p-4 sm:p-6 lg:h-full" aria-label="留言对话">
      <header class="flex min-h-14 shrink-0 items-start justify-between gap-3 border-b border-gray-100 pb-4 dark:border-dark-700">
        <div class="flex min-w-0 items-start gap-3">
          <span class="mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-primary-50 text-primary-700 dark:bg-primary-950/40 dark:text-primary-300">
            <Icon name="chatBubble" size="sm" />
          </span>
          <div class="min-w-0">
            <h2 class="truncate text-base font-semibold text-gray-900 dark:text-white">
              {{ selectedThread ? (selectedThread.subject || '未命名留言') : '新留言' }}
            </h2>
            <p v-if="selectedThread" class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              创建于 {{ formatDate(selectedThread.created_at) }}
            </p>
            <p v-else class="mt-1 text-xs text-gray-500 dark:text-gray-400">问题、建议或截图都可以发给管理员。</p>
          </div>
        </div>
        <span
          v-if="selectedThread"
          class="shrink-0 rounded-full px-2.5 py-1 text-xs font-medium"
          :class="statusClass(selectedThread.status)"
        >
          {{ statusLabel(selectedThread.status) }}
        </span>
      </header>

      <div
        v-if="selectedThread"
        ref="transcriptContainer"
        class="mt-4 min-h-0 flex-1 space-y-4 feedback-scrollbar overflow-y-auto overscroll-contain rounded-lg bg-gray-50/80 p-3 dark:bg-dark-800/50 sm:p-4"
        aria-label="对话内容" tabindex="0"
        aria-live="polite"
      >
        <div v-if="selectedThread.messages.length === 0" class="py-10 text-center text-sm text-gray-500 dark:text-gray-400">
          这条留言还没有消息。
        </div>
        <div
          v-for="message in selectedThread.messages"
          :key="message.id"
          class="flex"
          :class="message.author_type === 'admin' ? 'justify-start' : 'justify-end'"
        >
          <article
            class="min-w-0 max-w-[94%] rounded-lg border px-3.5 py-3 sm:max-w-[82%]"
            :class="message.author_type === 'admin'
              ? 'border-emerald-100 bg-white dark:border-emerald-900/60 dark:bg-dark-900'
              : 'border-primary-100 bg-primary-50 dark:border-primary-900/50 dark:bg-primary-950/30'"
          >
            <header class="flex flex-wrap items-center justify-between gap-x-4 gap-y-1 text-xs text-gray-500 dark:text-gray-400">
              <span class="font-semibold" :class="message.author_type === 'admin' ? 'text-emerald-700 dark:text-emerald-300' : 'text-primary-700 dark:text-primary-300'">
                {{ message.author_type === 'admin' ? '管理员' : '我' }}
              </span>
              <time>{{ formatDate(message.created_at) }}</time>
            </header>
            <p v-if="message.content" class="mt-2 whitespace-pre-wrap break-words text-sm leading-6 text-gray-800 dark:text-gray-200">
              {{ message.content }}
            </p>
            <div v-if="message.attachments?.length" class="mt-3 flex flex-wrap gap-2">
              <a
                v-for="image in message.attachments"
                :key="image.id"
                :href="image.data_url"
                target="_blank"
                rel="noopener"
                class="block max-w-full"
              >
                <img
                  :src="image.data_url"
                  :alt="image.filename"
                  class="max-h-56 max-w-full rounded-md border border-gray-200 object-contain dark:border-dark-600"
                />
              </a>
            </div>
          </article>
        </div>
      </div>

      <div v-else-if="loading && !newThreadMode" class="mt-4 flex min-h-0 flex-1 items-center justify-center text-sm text-gray-500 dark:text-gray-400">
        正在加载对话…
      </div>

      <div v-if="selectedThread?.status === 'closed'" class="mt-4 flex shrink-0 flex-wrap items-center justify-between gap-3 rounded-lg border border-gray-200 bg-gray-50 px-4 py-3 dark:border-dark-700 dark:bg-dark-800">
        <div>
          <p class="text-sm font-medium text-gray-800 dark:text-gray-200">这条对话已结束</p>
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">如需继续沟通，请新建一条留言。</p>
        </div>
        <button type="button" class="btn btn-secondary min-h-10" @click="startNewFeedback">新建留言</button>
      </div>

      <form v-else class="mt-4 shrink-0 space-y-3" @submit.prevent="submit">
        <label v-if="!selectedThread" class="block text-sm font-medium text-gray-700 dark:text-gray-200">
          主题
          <input v-model="subject" maxlength="120" class="input mt-1.5 min-h-11 w-full" placeholder="简单描述问题或建议" />
        </label>

        <label class="block text-sm font-medium text-gray-700 dark:text-gray-200">
          {{ selectedThread ? '回复管理员' : '内容' }}
          <textarea
            ref="messageInput"
            v-model="content"
            maxlength="40000"
            rows="3"
            class="input mt-1.5 h-24 w-full resize-none leading-6"
            :placeholder="selectedThread ? '继续补充问题，管理员会在这条对话中回复…' : '写下想反馈的内容…'"
          />
        </label>

        <div class="flex flex-wrap items-center justify-between gap-3">
          <div class="flex flex-wrap items-center gap-2">
            <div ref="emojiPicker" class="relative">
              <button
                type="button"
                class="inline-flex min-h-10 items-center gap-2 rounded-lg border border-gray-200 px-3 text-sm text-gray-700 transition-colors hover:bg-gray-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 dark:border-dark-700 dark:text-gray-200 dark:hover:bg-dark-800"
                :aria-expanded="emojiOpen"
                aria-haspopup="dialog"
                :aria-controls="emojiOpen ? 'feedback-emoji-popover' : undefined"
                @click="toggleEmojiPicker"
              >
                <Icon name="chatBubble" size="sm" />
                表情
              </button>
              <div
                v-if="emojiOpen"
                id="feedback-emoji-popover"
                class="emoji-popover absolute bottom-full left-0 z-40 mb-2 w-72 max-w-[calc(100vw-4.5rem)] rounded-xl border border-gray-200 bg-white p-3 shadow-xl dark:border-dark-600 dark:bg-dark-800"
                role="dialog"
                aria-label="选择表情"
                data-testid="emoji-popover"
              >
                <div class="mb-2 flex items-center justify-between">
                  <span class="text-xs font-semibold text-gray-700 dark:text-gray-200">常用表情</span>
                  <button
                    type="button"
                    class="flex h-9 w-9 items-center justify-center rounded-md text-gray-500 hover:bg-gray-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 dark:hover:bg-dark-700"
                    aria-label="关闭表情面板"
                    @click="emojiOpen = false"
                  >
                    <Icon name="x" size="xs" />
                  </button>
                </div>
                <div class="grid grid-cols-8 gap-1" role="group" aria-label="常用表情">
                  <button
                    v-for="emoji in emojis"
                    :key="emoji"
                    type="button"
                    class="flex aspect-square min-h-9 items-center justify-center rounded-md text-xl transition-colors hover:bg-gray-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 dark:hover:bg-dark-700"
                    :aria-label="`插入 ${emoji}`"
                    @click="appendEmoji(emoji)"
                  >
                    {{ emoji }}
                  </button>
                </div>
              </div>
            </div>

            <label class="inline-flex min-h-10 cursor-pointer items-center gap-2 rounded-lg border border-gray-200 px-3 text-sm text-gray-700 transition-colors hover:bg-gray-50 dark:border-dark-700 dark:text-gray-200 dark:hover:bg-dark-800">
              <Icon name="upload" size="sm" />
              添加图片
              <input class="sr-only" type="file" accept="image/png,image/jpeg,image/gif,image/webp" multiple @change="pickFiles" />
            </label>
          </div>
          <span class="basis-full text-xs text-gray-500 dark:text-gray-400 sm:basis-auto">
            PNG、JPG、GIF、WebP · 最多 3 张 · 合计不超过 5 MB
          </span>
        </div>

        <div v-if="files.length" class="flex flex-wrap gap-2 rounded-lg bg-gray-50 p-2 dark:bg-dark-800" aria-label="待上传图片">
          <div
            v-for="(file, index) in files"
            :key="`${file.name}-${index}`"
            class="relative h-20 w-20 overflow-hidden rounded-md border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-900"
          >
            <img :src="previews[index]" :alt="file.name" class="h-full w-full object-cover" />
            <button type="button" class="absolute right-1 top-1 flex h-8 w-8 items-center justify-center rounded-full bg-gray-900/75 text-white" :aria-label="`删除 ${file.name}`" @click="removeFile(index)">
              <Icon name="x" size="xs" />
            </button>
          </div>
        </div>

        <div class="flex justify-end border-t border-gray-100 pt-3 dark:border-dark-700">
          <button class="btn btn-primary min-h-11 min-w-36 justify-center" type="submit" :disabled="sending || (!content.trim() && files.length === 0)">
            <Icon name="chatBubble" size="sm" />
            {{ sending ? '发送中…' : selectedThread ? '发送回复' : '提交留言' }}
          </button>
        </div>
      </form>
    </section>
  </section>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import '@/components/feedback/feedback.css'
import { createFeedback, listMyFeedback, replyMyFeedback, type FeedbackThread } from '@/api/feedback'
import { useAppStore } from '@/stores/app'

const app = useAppStore()
const threads = ref<FeedbackThread[]>([])
const selectedThreadId = ref<number | null>(null)
const newThreadMode = ref(false)
const subject = ref('')
const content = ref('')
const files = ref<File[]>([])
const previews = ref<string[]>([])
const emojiOpen = ref(false)
const hasMore = ref(false)
const loading = ref(false)
const loadingMore = ref(false)
const loadFailed = ref(false)
const sending = ref(false)
const messageInput = ref<HTMLTextAreaElement | null>(null)
const emojiPicker = ref<HTMLElement | null>(null)
const conversationPanel = ref<HTMLElement | null>(null)
const transcriptContainer = ref<HTMLElement | null>(null)
const selectedThread = computed(() => threads.value.find(thread => thread.id === selectedThreadId.value) ?? null)
const emojis = ['😀', '😊', '😂', '🥹', '😍', '🤔', '😅', '😭', '👍', '👎', '👏', '🙏', '❤️', '🎉', '🔥', '✨', '✅', '❌', '💡', '🚀', '🐛', '📷', '💻', '⚡']

function statusLabel(status: string) {
  return ({ open: '待管理员回复', replied: '管理员已回复', closed: '已结束' } as Record<string, string>)[status] ?? status
}

function statusClass(status: string) {
  if (status === 'replied') return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-300'
  if (status === 'closed') return 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
  return 'bg-amber-50 text-amber-700 dark:bg-amber-950/40 dark:text-amber-300'
}

function formatDate(value: string) {
  return new Date(value).toLocaleString()
}

function formatCompactDate(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  const today = new Date()
  if (date.toDateString() === today.toDateString()) {
    return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  }
  return date.toLocaleDateString()
}

function previewText(thread: FeedbackThread) {
  const latest = thread.messages?.[thread.messages.length - 1]
  if (latest?.content) return latest.content
  if (latest?.attachments?.length) return '图片附件'
  return '暂无消息'
}

async function load() {
  loading.value = true
  loadFailed.value = false
  try {
    const page = await listMyFeedback()
    threads.value = page.items
    hasMore.value = page.has_more
    if (!newThreadMode.value && !page.items.some(thread => thread.id === selectedThreadId.value)) {
      selectedThreadId.value = page.items[0]?.id ?? null
    }
  } catch {
    loadFailed.value = true
    app.showError('留言加载失败')
  } finally {
    loading.value = false
  }
}

async function loadMore() {
  if (loadingMore.value || !hasMore.value) return
  loadingMore.value = true
  try {
    const page = await listMyFeedback(threads.value.length)
    threads.value.push(...page.items)
    hasMore.value = page.has_more
  } catch {
    app.showError('更早的留言加载失败')
  } finally {
    loadingMore.value = false
  }
}

function clearDraft() {
  subject.value = ''
  content.value = ''
  files.value = []
  revokePreviews()
}

function startNewFeedback() {
  selectedThreadId.value = null
  newThreadMode.value = true
  emojiOpen.value = false
  clearDraft()
}

async function selectThread(thread: FeedbackThread) {
  selectedThreadId.value = thread.id
  newThreadMode.value = false
  emojiOpen.value = false
  clearDraft()
  await nextTick()
  if (window.innerWidth < 1024) {
    const behavior = window.matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth'
    conversationPanel.value?.scrollIntoView({ behavior, block: 'start' })
  }
}

async function scrollTranscriptToBottom() {
  await nextTick()
  const container = transcriptContainer.value
  if (container) container.scrollTop = container.scrollHeight
}

watch(
  () => [selectedThread.value?.id, selectedThread.value?.messages.length],
  () => void scrollTranscriptToBottom(),
)

function toggleEmojiPicker() {
  emojiOpen.value = !emojiOpen.value
}

async function appendEmoji(emoji: string) {
  const input = messageInput.value
  if (!input) {
    content.value += emoji
    return
  }
  const start = input.selectionStart ?? content.value.length
  const end = input.selectionEnd ?? start
  content.value = `${content.value.slice(0, start)}${emoji}${content.value.slice(end)}`
  await nextTick()
  input.focus()
  input.setSelectionRange(start + emoji.length, start + emoji.length)
}

function onDocumentPointerDown(event: PointerEvent) {
  if (emojiOpen.value && !emojiPicker.value?.contains(event.target as Node)) emojiOpen.value = false
}

function onDocumentKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') emojiOpen.value = false
}

function revokePreviews() {
  previews.value.forEach(URL.revokeObjectURL)
  previews.value = []
}

function pickFiles(event: Event) {
  const input = event.target as HTMLInputElement
  const picked = Array.from(input.files ?? [])
  const next = [...files.value]

  for (const file of picked) {
    if (next.length >= 3) break
    if (!['image/png', 'image/jpeg', 'image/gif', 'image/webp'].includes(file.type) || file.size > 5 * 1024 * 1024) {
      app.showError('仅支持 PNG、JPG、GIF、WebP，且单张不超过 5 MB')
      continue
    }
    if (next.reduce((sum, item) => sum + item.size, 0) + file.size > 5 * 1024 * 1024) {
      app.showError('单条留言图片总大小不能超过 5 MB')
      continue
    }
    next.push(file)
  }

  files.value = next
  revokePreviews()
  previews.value = files.value.map(URL.createObjectURL)
  input.value = ''
}

function removeFile(index: number) {
  files.value.splice(index, 1)
  revokePreviews()
  previews.value = files.value.map(URL.createObjectURL)
}

async function submit() {
  if (sending.value) return
  sending.value = true
  try {
    const form = new FormData()
    form.set('content', content.value)
    files.value.forEach(file => form.append('attachments', file))

    if (selectedThread.value) {
      await replyMyFeedback(selectedThread.value.id, form)
      app.showSuccess('回复已发送')
      clearDraft()
      await load()
    } else {
      form.set('subject', subject.value)
      const created = await createFeedback(form)
      selectedThreadId.value = created.id
      newThreadMode.value = false
      clearDraft()
      await load()
      app.showSuccess('留言已提交')
    }
  } catch {
    app.showError('发送失败，请稍后重试')
  } finally {
    sending.value = false
  }
}

onMounted(() => {
  void load()
  document.addEventListener('pointerdown', onDocumentPointerDown)
  document.addEventListener('keydown', onDocumentKeydown)
})

onBeforeUnmount(() => {
  revokePreviews()
  document.removeEventListener('pointerdown', onDocumentPointerDown)
  document.removeEventListener('keydown', onDocumentKeydown)
})
</script>

<style scoped>
.emoji-popover {
  transform-origin: bottom left;
  animation: emoji-popover-in 140ms ease-out;
}

@keyframes emoji-popover-in {
  from { opacity: 0; transform: translateY(4px) scale(0.98); }
  to { opacity: 1; transform: translateY(0) scale(1); }
}

@media (prefers-reduced-motion: reduce) {
  .emoji-popover { animation: none; }
}
</style>

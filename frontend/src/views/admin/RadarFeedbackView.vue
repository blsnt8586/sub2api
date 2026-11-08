<template>
  <AppLayout>
    <div class="space-y-4">
      <header class="flex flex-wrap items-end justify-between gap-3 border-b border-gray-200 pb-4 dark:border-dark-700">
        <div>
          <h1 class="text-xl font-semibold text-gray-900 dark:text-white">{{ t('nav.feedbackManagement') }}</h1>
          <p class="mt-1 text-sm text-gray-600 dark:text-gray-400">按用户查看对话，优先处理未读消息。</p>
        </div>
        <button class="btn btn-secondary min-h-10 shrink-0 whitespace-nowrap" :disabled="loading" @click="refresh">
          <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />刷新
        </button>
      </header>

      <nav class="flex gap-2 lg:hidden" aria-label="留言管理分栏">
        <button class="btn btn-secondary flex-1" :aria-pressed="mobilePane === 'users'" @click="mobilePane = 'users'">用户</button>
        <button class="btn btn-secondary flex-1" :disabled="!selectedUser" :aria-pressed="mobilePane === 'threads'" @click="mobilePane = 'threads'">会话历史</button>
        <button class="btn btn-secondary flex-1" :disabled="!selectedThreadId" :aria-pressed="mobilePane === 'conversation'" @click="mobilePane = 'conversation'">对话</button>
      </nav>

      <section class="grid h-[calc(100dvh-16rem)] min-h-[32rem] max-h-[54rem] min-w-0 overflow-hidden rounded-2xl border border-gray-200/80 bg-white shadow-sm dark:border-dark-700 dark:bg-dark-900 lg:h-[calc(100dvh-13.5rem)] lg:grid-cols-[11.5rem_13.5rem_minmax(0,1fr)] xl:grid-cols-[12.5rem_15rem_minmax(0,1fr)]" aria-label="留言管理工作区">
        <aside class="min-h-0 min-w-0 flex-col overflow-hidden border-gray-200/80 bg-gray-50/80 dark:border-dark-700 dark:bg-dark-950/30 lg:border-r" :class="mobilePane === 'users' ? 'flex' : 'hidden lg:flex'" aria-label="留言用户列表">
          <header class="flex h-14 shrink-0 items-center justify-between gap-2 border-b border-gray-200/60 px-3.5 dark:border-dark-700">
            <h2 class="text-[13px] font-semibold text-gray-900 dark:text-white">留言用户</h2>
            <span class="text-[11px] text-gray-500 dark:text-gray-400">未读优先</span>
          </header>
          <div v-if="usersError" class="p-4 text-sm text-rose-600 dark:text-rose-300" role="alert">用户列表加载失败<button class="ml-2 underline" @click="loadUsers()">重试</button></div>
          <div v-if="usersLoading && !users.length" class="px-4 py-8 text-center text-sm text-gray-500">正在加载用户…</div>
          <div v-else-if="!usersError && !users.length" class="px-4 py-8 text-center text-sm text-gray-500">暂无用户留言</div>
          <div class="feedback-scrollbar min-h-0 flex-1 space-y-1 overflow-y-auto overscroll-contain p-2">
            <button
              v-for="user in users" :key="user.user_id" type="button"
              class="inbox-list-row"
              :aria-pressed="selectedUser?.user_id === user.user_id" :data-testid="`feedback-user-${user.user_id}`"
              @click="selectUser(user)"
            >
              <div class="flex items-center justify-between gap-2">
                <span class="min-w-0 truncate text-[13px] font-semibold leading-5 text-gray-900 dark:text-gray-100" :title="[user.username, user.email].filter(Boolean).join(' · ')">{{ user.username || user.email || `用户 #${user.user_id}` }}</span>
                <span v-if="user.unread_count" class="min-w-4 shrink-0 rounded-full bg-rose-500 px-1.5 text-center text-[10px] font-medium leading-4 text-white" :aria-label="`${user.unread_count} 条未读消息`">{{ user.unread_count > 99 ? '99+' : user.unread_count }}</span>
              </div>
              <p class="mt-0.5 truncate text-xs leading-4 text-gray-600 dark:text-gray-400">{{ user.preview }}</p>
              <div class="mt-1.5 flex items-center justify-between gap-1 text-[11px] leading-4 text-gray-500 dark:text-gray-400">
                <span class="shrink-0">{{ user.thread_count }} 个对话</span>
                <time class="truncate tabular-nums" :datetime="user.updated_at" :title="formatDate(user.updated_at)">{{ formatLatestTime(user.updated_at) }}</time>
              </div>
            </button>
          </div>
          <footer v-if="userPage || usersHasMore" class="flex shrink-0 items-center justify-between gap-2 border-t border-gray-100 p-3 text-xs dark:border-dark-700">
            <button class="btn btn-secondary !px-2 !py-1" :disabled="!userPage || usersLoading" @click="changeUserPage(-1)">上一页</button>
            <span class="text-gray-500">{{ userPage + 1 }}</span>
            <button class="btn btn-secondary !px-2 !py-1" :disabled="!usersHasMore || usersLoading" @click="changeUserPage(1)">下一页</button>
          </footer>
        </aside>

        <aside class="min-h-0 min-w-0 flex-col overflow-hidden border-gray-200 dark:border-dark-700 lg:border-r" :class="mobilePane === 'threads' ? 'flex' : 'hidden lg:flex'" aria-label="所选用户的会话历史">
          <header class="shrink-0 border-b border-gray-200/60 px-3.5 py-3 dark:border-dark-700">
            <div class="flex items-center justify-between gap-2">
              <h2 class="shrink-0 text-[13px] font-semibold text-gray-900 dark:text-white">会话历史</h2>
              <select v-model="status" class="input h-8 w-28 rounded-lg px-2 py-1 text-xs" aria-label="筛选留言状态" :disabled="!selectedUser">
                <option value="">全部状态</option><option value="open">待回复</option><option value="replied">已回复</option><option value="closed">已结束</option>
              </select>
            </div>
            <p class="mt-1 truncate text-[11px] leading-4 text-gray-500 dark:text-gray-400" :title="selectedUser?.email">{{ selectedUser ? (selectedUser.username || selectedUser.email) : '请先选择用户' }}</p>
          </header>
          <div v-if="threadsError" class="p-4 text-sm text-rose-600 dark:text-rose-300" role="alert">会话加载失败<button class="ml-2 underline" @click="loadThreads()">重试</button></div>
          <div v-if="threadsLoading && !threads.length" class="px-4 py-8 text-center text-sm text-gray-500">正在加载会话…</div>
          <div v-else-if="!threadsError && !threads.length" class="px-4 py-8 text-center text-sm text-gray-500">{{ selectedUser ? '暂无符合条件的会话' : '选择用户后查看对话历史' }}</div>
          <div class="feedback-scrollbar min-h-0 flex-1 space-y-1 overflow-y-auto overscroll-contain p-2">
            <button
              v-for="thread in threads" :key="thread.id" type="button"
              class="inbox-list-row"
              :data-testid="`admin-feedback-thread-${thread.id}`" :aria-pressed="selectedThreadId === thread.id" @click="selectThread(thread)"
            >
              <div class="flex items-center justify-between gap-2">
                <span class="min-w-0 truncate text-[13px] font-semibold leading-5 text-gray-900 dark:text-white" :title="thread.subject">{{ thread.subject || '未命名留言' }}</span>
                <time class="shrink-0 text-[11px] tabular-nums leading-4 text-gray-500 dark:text-gray-400" :datetime="thread.updated_at" :title="formatDate(thread.updated_at)" :aria-label="`最近更新 ${formatDate(thread.updated_at)}`">{{ formatLatestTime(thread.updated_at) }}</time>
              </div>
              <p class="mt-0.5 truncate text-xs leading-4 text-gray-600 dark:text-gray-400">{{ thread.preview }}</p>
              <div class="mt-1.5 flex items-center justify-between gap-2">
                <span class="rounded-md px-1.5 text-[10px] font-medium leading-4" :class="statusClass(thread.status)">{{ statusLabel(thread.status) }}</span>
                <span v-if="thread.unread_count" class="flex items-center gap-1.5 text-[11px] font-medium leading-4 text-primary-700 dark:text-primary-300"><span class="h-1.5 w-1.5 rounded-full bg-primary-500" aria-hidden="true" />{{ thread.unread_count }} 条未读</span>
              </div>
            </button>
          </div>
          <footer v-if="threadPage || threadsHasMore" class="flex shrink-0 items-center justify-between gap-2 border-t border-gray-100 p-3 text-xs dark:border-dark-700">
            <button class="btn btn-secondary !px-2 !py-1" :disabled="!threadPage || threadsLoading" @click="changeThreadPage(-1)">上一页</button><span class="text-gray-500">{{ threadPage + 1 }}</span><button class="btn btn-secondary !px-2 !py-1" :disabled="!threadsHasMore || threadsLoading" @click="changeThreadPage(1)">下一页</button>
          </footer>
        </aside>
        <section :class="mobilePane === 'conversation' ? 'flex' : 'hidden lg:flex'" class="min-h-0 min-w-0 flex-col overflow-hidden p-4 xl:p-5" aria-label="选中的用户对话">
          <div v-if="detailError" class="flex flex-1 flex-col items-center justify-center gap-3 text-sm text-rose-600 dark:text-rose-300" role="alert">
            对话加载失败
            <button class="btn btn-secondary" @click="selectedThreadId && openThread(selectedThreadId)">重试</button>
          </div>
          <div v-else-if="detailLoading && !selectedThread" class="flex flex-1 items-center justify-center text-sm text-gray-500">正在加载对话…</div>
          <template v-else-if="selectedThread">
            <header class="flex min-h-12 shrink-0 items-start justify-between gap-3 border-b border-gray-100 pb-3 dark:border-dark-700">
              <div class="flex min-w-0 items-start gap-3">
                <span class="mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-gray-100 text-gray-600 dark:bg-dark-800 dark:text-gray-300">
                  <Icon name="chatBubble" size="sm" />
                </span>
                <div class="min-w-0">
                  <h2 class="truncate text-base font-semibold text-gray-900 dark:text-white">
                    {{ selectedThread.subject || '未命名留言' }}
                    <span class="ml-1 text-xs font-normal text-gray-500">#{{ selectedThread.id }}</span>
                  </h2>
                  <p class="mt-1 truncate text-xs text-gray-500 dark:text-gray-400">
                    {{ selectedThread.username || '用户' }} · {{ selectedThread.email || `用户 #${selectedThread.user_id}` }}
                  </p>
                </div>
              </div>
              <span class="shrink-0 rounded-full px-2.5 py-1 text-xs font-medium" :class="statusClass(selectedThread.status)">
                {{ statusLabel(selectedThread.status) }}
              </span>
            </header>

            <div ref="transcriptContainer" class="mt-3 min-h-0 flex-1 space-y-3 feedback-scrollbar overflow-y-auto overscroll-contain rounded-xl bg-gray-50/70 p-3 dark:bg-dark-800/30 sm:p-4" aria-label="对话内容" tabindex="0" aria-live="polite" @scroll="markVisibleRead">
              <div v-if="!selectedThread.messages?.length" class="py-10 text-center text-sm text-gray-500 dark:text-gray-400">这条留言还没有消息。</div>
              <div v-for="message in selectedThread.messages" :key="message.id" class="flex" :class="message.author_type === 'admin' ? 'justify-end' : 'justify-start'">
                <article
                  class="min-w-0 max-w-[94%] rounded-xl border px-3.5 py-2.5 sm:max-w-[82%]"
                  :class="message.author_type === 'admin'
                    ? 'border-primary-100/60 bg-primary-50/80 dark:border-primary-900/50 dark:bg-primary-950/30'
                    : 'border-gray-200/70 bg-white dark:border-dark-700 dark:bg-dark-900'"
                >
                  <header class="flex flex-wrap items-center justify-between gap-x-4 gap-y-1 text-xs text-gray-500 dark:text-gray-400">
                    <span class="font-semibold" :class="message.author_type === 'admin' ? 'text-primary-700 dark:text-primary-300' : 'text-gray-700 dark:text-gray-200'">
                      {{ message.author_type === 'admin' ? '管理员' : selectedThread.username || '用户' }}
                    </span>
                    <time class="text-[11px] tabular-nums" :datetime="message.created_at" :title="formatDate(message.created_at)">{{ formatLatestTime(message.created_at) }}</time>
                  </header>
                  <p v-if="message.content" class="mt-2 whitespace-pre-wrap break-words text-sm leading-6 text-gray-800 dark:text-gray-200">{{ message.content }}</p>
                  <div v-if="message.attachments?.length" class="mt-3 flex flex-wrap gap-2">
                    <a v-for="image in message.attachments" :key="image.id" :href="image.data_url" target="_blank" rel="noopener" class="block max-w-full">
                      <img :src="image.data_url" :alt="image.filename" class="max-h-56 max-w-full rounded-md border border-gray-200 object-contain dark:border-dark-600" />
                    </a>
                  </div>
                </article>
              </div>
            </div>

            <div v-if="selectedThread.status === 'closed'" class="mt-4 shrink-0 rounded-lg border border-gray-200 bg-gray-50 px-4 py-3 dark:border-dark-700 dark:bg-dark-800">
              <p class="text-sm font-medium text-gray-800 dark:text-gray-200">这条对话已结束</p>
              <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">历史消息已保留，可在左侧选择其他对话。</p>
            </div>
            <form v-else class="mt-3 shrink-0 space-y-2" @submit.prevent="reply()">
              <label class="block text-xs font-medium text-gray-600 dark:text-gray-300">
                回复用户
                <textarea v-model="drafts[selectedThread.id]" :disabled="sending === selectedThread.id" class="input mt-1.5 h-20 w-full resize-none rounded-xl leading-6" rows="3" maxlength="40000" placeholder="回复会出现在用户的这条对话中…" />
              </label>
              <div class="flex flex-wrap justify-end gap-2 pt-1">
                <button type="button" class="btn btn-secondary min-h-9 rounded-lg px-3 py-2 text-xs shadow-none" :disabled="!!sending || !drafts[selectedThread.id]?.trim()" @click="reply(true)">
                  回复并结束
                </button>
                <button class="btn btn-primary min-h-9 rounded-lg bg-none bg-primary-600 px-4 py-2 text-xs shadow-none hover:bg-primary-700 hover:shadow-none" type="submit" :disabled="!!sending || !drafts[selectedThread.id]?.trim()">
                  <Icon name="chatBubble" size="sm" />
                  {{ sending === selectedThread.id ? '发送中…' : '回复' }}
                </button>
              </div>
            </form>
          </template>
          <div v-else class="flex flex-1 flex-col items-center justify-center text-center">
            <span class="flex h-10 w-10 items-center justify-center rounded-full bg-gray-100 text-gray-500 dark:bg-dark-800 dark:text-gray-400">
              <Icon name="inbox" size="md" />
            </span>
            <p class="mt-3 text-sm font-medium text-gray-800 dark:text-gray-200">选择一条留言</p>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">先选择用户，再选择要查看的对话。</p>
          </div>
        </section>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import '@/components/feedback/feedback.css'
import { useFeedbackInbox } from '@/composables/useFeedbackInbox'

const { t } = useI18n()
const {
  users, threads, selectedUser, selectedThread, selectedThreadId, status, userPage, threadPage,
  usersHasMore, threadsHasMore, usersLoading, threadsLoading, detailLoading, usersError, threadsError,
  detailError, loading, sending, drafts, mobilePane, transcriptContainer,
  selectUser, selectThread, openThread, refresh, loadUsers, loadThreads, changeUserPage, changeThreadPage,
  reply, markVisibleRead,
} = useFeedbackInbox()

function statusLabel(value: string) {
  return ({ open: '待回复', replied: '已回复', closed: '已结束' } as Record<string, string>)[value] ?? value
}
function statusClass(value: string) {
  if (value === 'open') return 'bg-amber-50 text-amber-700 dark:bg-amber-950/30 dark:text-amber-300'
  if (value === 'replied') return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950/30 dark:text-emerald-300'
  return 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
}
function formatDate(value: string) {
  return new Date(value).toLocaleString('zh-CN', { hour12: false, year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
}

function formatLatestTime(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '—'
  const now = new Date()
  const time = date.toLocaleTimeString('zh-CN', { hour12: false, hour: '2-digit', minute: '2-digit' })
  if (date.toDateString() === now.toDateString()) return time
  const day = date.toLocaleDateString('zh-CN', {
    ...(date.getFullYear() !== now.getFullYear() ? { year: 'numeric' as const } : {}),
    month: '2-digit', day: '2-digit',
  })
  return `${day} ${time}`
}
</script>

<style scoped>
.inbox-list-row {
  @apply block w-full rounded-lg px-2.5 py-2.5 text-left transition-colors duration-150;
  @apply hover:bg-gray-100/80 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-primary-500;
  @apply dark:hover:bg-dark-800/70;
}

.inbox-list-row[aria-pressed='true'] {
  @apply bg-primary-50/80 shadow-sm ring-1 ring-primary-100/70;
  @apply dark:bg-primary-950/30 dark:shadow-none dark:ring-primary-900/40;
}

@media (prefers-reduced-motion: reduce) {
  .inbox-list-row { transition: none; }
}
</style>

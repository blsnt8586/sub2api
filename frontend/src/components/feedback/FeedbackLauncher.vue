<template>
  <button
      type="button"
      class="fixed bottom-5 right-5 z-40 flex h-14 w-14 items-center justify-center rounded-full bg-primary-600 text-white shadow-lg shadow-primary-900/20 transition hover:bg-primary-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 focus-visible:ring-offset-2 sm:bottom-7 sm:right-7"
      aria-label="打开留言板"
      :aria-expanded="open"
      @click="open = true"
    >
      <Icon name="chatBubble" size="md" />
      <span v-if="userUnread" class="absolute -right-1 -top-1 flex h-5 min-w-5 items-center justify-center rounded-full border-2 border-white bg-rose-500 px-1 text-[10px] font-semibold leading-none text-white dark:border-dark-900" :aria-label="`${userUnread} 条未读回复`">
        {{ userUnread > 99 ? '99+' : userUnread }}
      </span>
  </button>

  <Teleport to="body">
      <Transition name="feedback-modal">
        <div v-if="open" class="fixed inset-0 z-[70] flex items-end justify-center bg-black/45 p-0 backdrop-blur-[2px] sm:items-center sm:p-5" @click.self="closeModal">
          <section class="flex h-[min(48rem,100dvh)] w-full max-w-5xl flex-col overflow-hidden bg-white shadow-2xl dark:bg-dark-900 sm:h-[min(44rem,calc(100dvh-2.5rem))] sm:rounded-2xl sm:border sm:border-gray-200 dark:sm:border-dark-700" role="dialog" aria-modal="true" aria-labelledby="feedback-modal-title">
            <header class="flex h-14 shrink-0 items-center justify-between border-b border-gray-200 px-4 dark:border-dark-700 sm:px-5">
              <div class="flex min-w-0 items-center gap-2.5">
                <span class="flex h-8 w-8 items-center justify-center rounded-lg bg-primary-50 text-primary-700 dark:bg-primary-950/40 dark:text-primary-300"><Icon name="chatBubble" size="sm" /></span>
                <h2 id="feedback-modal-title" class="truncate text-sm font-semibold text-gray-900 dark:text-white">留言板</h2>
              </div>
              <button type="button" class="flex h-9 w-9 items-center justify-center rounded-lg text-gray-500 transition hover:bg-gray-100 hover:text-gray-800 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 dark:hover:bg-dark-800 dark:hover:text-white" aria-label="关闭留言板" @click="closeModal"><Icon name="x" size="sm" /></button>
            </header>
            <div class="min-h-0 flex-1 p-3 sm:p-4">
              <FeedbackBoard />
            </div>
          </section>
        </div>
      </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import Icon from '@/components/icons/Icon.vue'
import FeedbackBoard from './FeedbackBoard.vue'
import { useFeedbackNotifications } from '@/composables/useFeedbackNotifications'

const open = ref(false)
const route = useRoute()
const router = useRouter()
const { userUnread, markUserMessagesSeen } = useFeedbackNotifications()

function closeModal() {
  open.value = false
  if (route.path === '/feedback') void router.replace('/dashboard')
}

function onKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape' && open.value) closeModal()
}

watch(() => route.path, path => {
  if (path === '/feedback') open.value = true
})
onMounted(() => {
  document.addEventListener('keydown', onKeydown)
  if (route.path === '/feedback') open.value = true
})
onBeforeUnmount(() => document.removeEventListener('keydown', onKeydown))
watch(open, value => { if (value) markUserMessagesSeen() })
</script>

<style scoped>
.feedback-modal-enter-active, .feedback-modal-leave-active { transition: opacity 160ms ease; }
.feedback-modal-enter-active section, .feedback-modal-leave-active section { transition: transform 160ms ease; }
.feedback-modal-enter-from, .feedback-modal-leave-to { opacity: 0; }
.feedback-modal-enter-from section, .feedback-modal-leave-to section { transform: translateY(12px); }
@media (prefers-reduced-motion: reduce) {
  .feedback-modal-enter-active, .feedback-modal-leave-active,
  .feedback-modal-enter-active section, .feedback-modal-leave-active section { transition: none; }
}
</style>

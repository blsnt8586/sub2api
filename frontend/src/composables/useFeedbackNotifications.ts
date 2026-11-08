import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { listFeedbackUsers, listMyFeedback } from '@/api/feedback'
import { useAuthStore } from '@/stores/auth'

const adminUnread = ref(0)
const userUnread = ref(0)
let adminTimer: ReturnType<typeof setInterval> | undefined
let userTimer: ReturnType<typeof setInterval> | undefined
let adminUsers = 0
let userUsers = 0

function userWatermarkKey(userID: number) {
  return `feedback:last-admin-message:${userID}`
}

async function refreshAdminUnread() {
  try {
    let offset = 0
    let total = 0
    let hasMore = true
    while (hasMore && offset < 2000) {
      const page = await listFeedbackUsers(offset)
      total += page.items.reduce((sum, user) => sum + user.unread_count, 0)
      hasMore = page.has_more
      offset += page.items.length
      if (!page.items.length) break
    }
    adminUnread.value = total
  } catch {
    // Keep the last known badge when the inbox request temporarily fails.
  }
}

async function refreshUserUnread(userID: number) {
  try {
    const page = await listMyFeedback()
    const seen = Number(localStorage.getItem(userWatermarkKey(userID)) || 0)
    userUnread.value = page.items.reduce((count, thread) => {
      const lastAdminMessage = [...(thread.messages ?? [])].reverse().find(message => message.author_type === 'admin')
      return count + (lastAdminMessage && lastAdminMessage.id > seen ? 1 : 0)
    }, 0)
  } catch {
    // Keep the last known badge when the board request temporarily fails.
  }
}

export function useFeedbackNotifications() {
  const auth = useAuthStore()
  const isAdmin = computed(() => auth.user?.role === 'admin')
  const userID = computed(() => Number(auth.user?.id ?? 0))

  function markUserMessagesSeen() {
    const id = userID.value
    if (!id) return
    void listMyFeedback().then(page => {
      const latest = page.items.flatMap(thread => thread.messages ?? [])
        .filter(message => message.author_type === 'admin')
        .reduce((max, message) => Math.max(max, message.id), 0)
      if (latest) localStorage.setItem(userWatermarkKey(id), String(latest))
      userUnread.value = 0
    }).catch(() => {})
  }

  onMounted(() => {
    if (isAdmin.value) {
      adminUsers += 1
      if (!adminTimer) {
        void refreshAdminUnread()
        adminTimer = setInterval(() => {
          if (document.visibilityState !== 'hidden') void refreshAdminUnread()
        }, 20000)
      }
    } else if (userID.value) {
      userUsers += 1
      const id = userID.value
      if (!userTimer) {
        void refreshUserUnread(id)
        userTimer = setInterval(() => {
          if (document.visibilityState !== 'hidden') void refreshUserUnread(id)
        }, 20000)
      }
    }
  })

  onBeforeUnmount(() => {
    if (isAdmin.value) {
      adminUsers = Math.max(0, adminUsers - 1)
      if (!adminUsers && adminTimer) { clearInterval(adminTimer); adminTimer = undefined }
    } else if (userID.value) {
      userUsers = Math.max(0, userUsers - 1)
      if (!userUsers && userTimer) { clearInterval(userTimer); userTimer = undefined }
    }
  })

  return { adminUnread, userUnread, isAdmin, markUserMessagesSeen }
}

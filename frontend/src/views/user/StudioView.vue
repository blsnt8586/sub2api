<template>
  <!-- 不用 AppLayout，自己组装 Sidebar + Header，让 iframe flex 占满剩余高度 -->
  <div class="studio-root min-h-screen bg-white">

    <AppSidebar />

    <div
      class="studio-shell transition-all duration-300"
      :class="[sidebarCollapsed ? 'lg:ml-[72px]' : 'lg:ml-64']"
    >
      <AppHeader />

      <iframe
        ref="iframeRef"
        :src="canvasUrl"
        class="studio-frame"
        allow="clipboard-write; clipboard-read; fullscreen; microphone; camera"
        allowfullscreen
        title="视图工作台"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useAppStore, useAuthStore } from '@/stores'
import AppSidebar from '@/components/layout/AppSidebar.vue'
import AppHeader from '@/components/layout/AppHeader.vue'

const appStore = useAppStore()
const authStore = useAuthStore()
const sidebarCollapsed = computed(() => appStore.sidebarCollapsed)
const iframeRef = ref<HTMLIFrameElement | null>(null)

// Canvas URL：带上 token 和 src_host，让 infinite-canvas 自己调接口拿 keys
// 优先级：VITE_CANVAS_URL（开发）→ /canvas/image（生产 nginx 反代）→ :3000（开发后备）
const canvasUrl = computed(() => {
  const envUrl = import.meta.env.VITE_CANVAS_URL as string | undefined
  const base = envUrl
    ?? (import.meta.env.PROD
      ? `${window.location.origin}/canvas/image`
      : `${window.location.protocol}//${window.location.hostname}:3000/image`)
  try {
    const url = new URL(base)
    if (authStore.token) url.searchParams.set('token', authStore.token)
    url.searchParams.set('src_host', window.location.origin)
    return url.toString()
  } catch {
    return base
  }
})
</script>

<style scoped>
.studio-root {
  /* 阻止页面本身滚动 */
  overflow: hidden;
  height: 100vh;
}

.studio-shell {
  display: flex;
  flex-direction: column;
  /* 整个 shell 占满视口高度，不让页面滚动 */
  height: 100vh;
  overflow: hidden;
}

.studio-frame {
  /* flex: 1 + min-height: 0 是让 iframe 占满 header 以下全部空间的关键 */
  flex: 1;
  min-height: 0;
  width: 100%;
  border: 0;
  display: block;
  background: #ffffff;
}
</style>

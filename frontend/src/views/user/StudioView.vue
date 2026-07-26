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
        @load="sendKeysToIframe"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, onMounted } from 'vue'
import { useAppStore, useAuthStore } from '@/stores'
import AppSidebar from '@/components/layout/AppSidebar.vue'
import AppHeader from '@/components/layout/AppHeader.vue'
import { list as listKeys } from '@/api/keys'
import type { ApiKey } from '@/types'

const appStore = useAppStore()
const authStore = useAuthStore()
const sidebarCollapsed = computed(() => appStore.sidebarCollapsed)

const iframeRef = ref<HTMLIFrameElement | null>(null)
const platformKeys = ref<Array<{ id: number; name: string; key: string }>>([])

// 优先读 Vite 环境变量，否则用同 hostname 的 3000 端口（开发/生产均适用）
const canvasUrl = computed(() => {
  const envUrl = import.meta.env.VITE_CANVAS_URL as string | undefined
  if (envUrl) return envUrl
  const { protocol, hostname } = window.location
  // 直接打开生图工作台，跳过首页
  return `${protocol}//${hostname}:3000/image`
})

/** 将当前用户的 active API Key 列表与 JWT token 推送给 infinite-canvas iframe */
const sendKeysToIframe = () => {
  // 用 map 拆成纯对象，避免 Vue reactive proxy 触发 postMessage 的 DataCloneError
  const plainKeys = platformKeys.value.map((k) => ({ id: k.id, name: k.name, key: k.key }))
  iframeRef.value?.contentWindow?.postMessage(
    {
      type: 'sub2api:init',
      keys: plainKeys,
      // JWT token 供 infinite-canvas 调用 canvas-api 做持久化存储
      token: authStore.token ?? '',
    },
    '*',
  )
}

onMounted(async () => {
  try {
    // 拉取全部 active key（最多 200 条，覆盖绝大多数场景）
    const result = await listKeys(1, 200, { status: 'active' })
    platformKeys.value = (result.items ?? []).map((k: ApiKey) => ({
      id: k.id,
      name: k.name,
      key: k.key,
    }))
    // iframe 可能已加载完毕，立即推送一次
    sendKeysToIframe()
  } catch {
    // 拉取失败不影响 iframe 正常使用，用户可手动输入 key
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

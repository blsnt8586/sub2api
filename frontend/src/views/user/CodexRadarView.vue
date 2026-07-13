<!--
  Codex 雷达页面（二开新增，用户 + 管理员共用）。

  数据来源为第三方社区站点 codexradar.com，本平台仅做代理缓存 + 署名转载。
  布局：极简单屏——顶部紧凑标题栏（标题 + 免责说明收进 ⓘ + 刷新），
  中间图片撑满剩余高度居中（object-contain 保证不溢出、无滚动条），
  底部一行细来源署名。功能开关关闭时由路由/侧边栏 featureFlag 拦截。
-->
<template>
  <AppLayout>
    <!-- 标题/副标题由 AppHeader 依路由 meta 渲染，页面内不再重复 -->
    <div class="flex h-[calc(100vh-9rem)] flex-col gap-3 overflow-hidden">
      <!-- 顶部工具栏：仅刷新按钮，右对齐 -->
      <div class="flex flex-shrink-0 justify-end">
        <button type="button" class="btn btn-secondary" :disabled="loading" @click="reload">
          <Icon name="refresh" size="md" class="mr-2" :class="loading ? 'animate-spin' : ''" />
          {{ t('common.refresh') }}
        </button>
      </div>

      <!-- 图片区：吃掉剩余高度，居中，object-contain 缩放不溢出 -->
      <div class="flex min-h-0 flex-1 items-center justify-center">
        <LoadingSpinner v-if="loading && !imageUrl" />
        <img
          v-else-if="imageUrl"
          :src="imageUrl"
          :alt="t('codexRadar.imageAlt')"
          class="max-h-full max-w-full rounded-lg object-contain shadow-sm"
          loading="lazy"
        />
        <EmptyState v-else :description="t('codexRadar.unavailable')" />
      </div>

      <!-- 底部一行细署名 + 更新时间 + 免责说明 ⓘ（点击展开） -->
      <div class="flex flex-shrink-0 items-center justify-center gap-1 text-xs text-gray-400 dark:text-gray-500">
        <span>
          {{ attribution }}
          <template v-if="fetchedAtLabel"> · {{ t('codexRadar.updatedAt', { time: fetchedAtLabel }) }}</template>
        </span>
        <HelpTooltip trigger="click" width-class="w-72">
          <template #trigger>
            <Icon
              name="infoCircle"
              size="xs"
              class="cursor-pointer text-amber-500 transition-colors hover:text-amber-600"
            />
          </template>
          <div class="space-y-1.5">
            <p class="font-medium text-amber-300">{{ t('codexRadar.disclaimer.title') }}</p>
            <p class="text-gray-200">{{ t('codexRadar.disclaimer.body') }}</p>
            <a
              :href="sourceUrl"
              target="_blank"
              rel="noopener noreferrer"
              class="inline-flex items-center gap-1 font-medium text-amber-200 underline hover:text-amber-100"
            >
              {{ t('codexRadar.disclaimer.viewSource') }}
              <Icon name="externalLink" size="xs" />
            </a>
          </div>
        </HelpTooltip>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import { useAppStore } from '@/stores/app'
import {
  getCodexRadarSummary,
  fetchCodexRadarImageObjectURL,
  type CodexRadarSummary
} from '@/api/codexradar'

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const imageUrl = ref('')
const summary = ref<CodexRadarSummary | null>(null)

const sourceUrl = computed(() => summary.value?.source || 'https://codexradar.com')
const attribution = computed(
  () => summary.value?.attribution || t('codexRadar.disclaimer.attributionFallback')
)

const fetchedAtLabel = computed(() => {
  const raw = summary.value?.fetched_at
  if (!raw) return ''
  const d = new Date(raw)
  if (Number.isNaN(d.getTime())) return ''
  return d.toLocaleString()
})

function revokeImage(): void {
  if (imageUrl.value) {
    URL.revokeObjectURL(imageUrl.value)
    imageUrl.value = ''
  }
}

async function reload(): Promise<void> {
  loading.value = true
  try {
    const [summaryRes, nextImageUrl] = await Promise.allSettled([
      getCodexRadarSummary(),
      fetchCodexRadarImageObjectURL()
    ])
    if (summaryRes.status === 'fulfilled') {
      summary.value = summaryRes.value
    }
    if (nextImageUrl.status === 'fulfilled') {
      revokeImage()
      imageUrl.value = nextImageUrl.value
    }
    if (summaryRes.status === 'rejected' && nextImageUrl.status === 'rejected') {
      appStore.showError(t('codexRadar.loadError'))
    }
  } finally {
    loading.value = false
  }
}

onMounted(reload)
onBeforeUnmount(revokeImage)
</script>

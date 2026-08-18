<template>
  <Teleport to="body">
    <div v-if="show && position && provider">
      <!-- 点击外部关闭 -->
      <div class="fixed inset-0 z-[9998]" @click="emit('close')"></div>
      <div
        class="fixed z-[9999] w-48 overflow-hidden rounded-lg bg-white shadow-lg ring-1 ring-black/5 dark:bg-dark-800"
        :style="{ top: position.top + 'px', left: position.left + 'px' }"
        @click.stop
      >
        <div class="py-1">
          <!-- 编辑 -->
          <button
            @click="emit('edit', provider); emit('close')"
            class="flex min-h-11 w-full items-center gap-2 px-4 py-2 text-sm hover:bg-gray-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-primary-500 dark:hover:bg-dark-700"
          >
            <Icon name="edit" size="sm" class="text-gray-500 dark:text-dark-300" />
            {{ t('common.edit') }}
          </button>
          <!-- 启用 / 停用 -->
          <button
            @click="emit('toggle-status', provider); emit('close')"
            :disabled="toggling"
            class="flex min-h-11 w-full items-center gap-2 px-4 py-2 text-sm hover:bg-gray-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-primary-500 disabled:opacity-40 dark:hover:bg-dark-700"
            :class="provider.status === 'active' ? 'text-red-600 dark:text-red-400' : 'text-green-600 dark:text-green-400'"
          >
            <Icon
              :name="toggling ? 'refresh' : (provider.status === 'active' ? 'ban' : 'play')"
              size="sm"
              :class="toggling ? 'animate-spin' : ''"
            />
            {{ provider.status === 'active' ? t('admin.sub2apiProviders.disableProvider') : t('admin.sub2apiProviders.enableProvider') }}
          </button>

          <div class="my-1 border-t border-gray-100 dark:border-dark-700"></div>

          <!-- 探测路径 -->
          <button
            @click="emit('detect-paths', provider); emit('close')"
            :disabled="detecting"
            class="flex min-h-11 w-full items-center gap-2 px-4 py-2 text-sm hover:bg-gray-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-primary-500 disabled:opacity-40 dark:hover:bg-dark-700"
          >
            <Icon :name="detecting ? 'refresh' : 'search'" size="sm" class="text-purple-500" :class="detecting ? 'animate-spin' : ''" />
            {{ t('admin.sub2apiProviders.detectPaths') }}
          </button>
          <button
            @click="emit('test-connection', provider); emit('close')"
            :disabled="testing"
            class="flex min-h-11 w-full items-center gap-2 px-4 py-2 text-sm hover:bg-gray-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-primary-500 disabled:opacity-40 dark:hover:bg-dark-700"
          >
            <Icon :name="testing ? 'refresh' : 'play'" size="sm" class="text-blue-500" :class="testing ? 'animate-spin' : ''" />
            {{ t('admin.sub2apiProviders.testConnection') }}
          </button>
          <button
            @click="emit('probe-settings', provider); emit('close')"
            class="flex min-h-11 w-full items-center gap-2 px-4 py-2 text-sm hover:bg-gray-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-primary-500 dark:hover:bg-dark-700"
          >
            <Icon name="cog" size="sm" class="text-teal-500" />
            {{ t('admin.sub2apiProviders.health.settings') }}
          </button>
          <!-- 批量优化 -->
          <button
            @click="emit('optimize-all', provider); emit('close')"
            :disabled="optimizing"
            class="flex min-h-11 w-full items-center gap-2 px-4 py-2 text-sm hover:bg-gray-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-primary-500 disabled:opacity-40 dark:hover:bg-dark-700"
          >
            <Icon :name="optimizing ? 'refresh' : 'bolt'" size="sm" class="text-orange-500" :class="optimizing ? 'animate-spin' : ''" />
            {{ t('admin.sub2apiProviders.optimizeAll') }}
          </button>
          <!-- 定时优化 -->
          <button
            @click="emit('schedule-optimize', provider); emit('close')"
            class="flex min-h-11 w-full items-center gap-2 px-4 py-2 text-sm hover:bg-gray-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-primary-500 dark:hover:bg-dark-700"
          >
            <Icon name="clock" size="sm" class="text-blue-500" />
            {{ t('admin.sub2apiProviders.scheduleOptimize') }}
          </button>

          <div class="my-1 border-t border-gray-100 dark:border-dark-700"></div>

          <!-- 删除 -->
          <button
            @click="emit('delete', provider); emit('close')"
            class="flex min-h-11 w-full items-center gap-2 px-4 py-2 text-sm text-red-600 hover:bg-red-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-red-500 dark:text-red-400 dark:hover:bg-red-900/20"
          >
            <Icon name="trash" size="sm" />
            {{ t('common.delete') }}
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { watch, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { Sub2APIProvider } from '@/api/admin/sub2apiProviders'

const props = defineProps<{
  show: boolean
  provider: Sub2APIProvider | null
  position: { top: number; left: number } | null
  detecting?: boolean
  optimizing?: boolean
  toggling?: boolean
  testing?: boolean
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'edit', provider: Sub2APIProvider): void
  (e: 'toggle-status', provider: Sub2APIProvider): void
  (e: 'detect-paths', provider: Sub2APIProvider): void
  (e: 'test-connection', provider: Sub2APIProvider): void
  (e: 'probe-settings', provider: Sub2APIProvider): void
  (e: 'optimize-all', provider: Sub2APIProvider): void
  (e: 'schedule-optimize', provider: Sub2APIProvider): void
  (e: 'delete', provider: Sub2APIProvider): void
}>()

const { t } = useI18n()

const handleKeydown = (event: KeyboardEvent) => {
  if (event.key === 'Escape') emit('close')
}

watch(
  () => props.show,
  (visible) => {
    if (visible) window.addEventListener('keydown', handleKeydown)
    else window.removeEventListener('keydown', handleKeydown)
  },
  { immediate: true }
)

onUnmounted(() => window.removeEventListener('keydown', handleKeydown))
</script>

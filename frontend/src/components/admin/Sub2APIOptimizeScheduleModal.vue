<template>
  <BaseDialog
    :show="show"
    :title="t('admin.sub2apiProviders.scheduleOptimizeTitle')"
    width="wide"
    @close="emit('close')"
  >
    <div class="space-y-5">
      <!-- 说明 -->
      <p class="text-sm text-gray-500 dark:text-gray-400">
        {{ t('admin.sub2apiProviders.scheduleOptimizeDesc') }}
      </p>

      <!-- 加载中 -->
      <div v-if="loading" class="flex items-center justify-center py-8">
        <Icon name="refresh" size="lg" class="animate-spin text-gray-400" />
      </div>

      <template v-else>
        <!-- Cron 表达式 -->
        <div class="space-y-1">
          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('admin.sub2apiProviders.cronExpr') }}
          </label>
          <input
            v-model="form.cron_expr"
            type="text"
            :placeholder="t('admin.sub2apiProviders.cronExprPlaceholder')"
            class="input w-full font-mono text-sm"
          />
          <!-- 快捷预设 -->
          <div class="flex flex-wrap gap-2 pt-1">
            <button
              v-for="preset in cronPresets"
              :key="preset.value"
              type="button"
              @click="form.cron_expr = preset.value"
              class="rounded-md border border-gray-200 px-2 py-0.5 text-xs text-gray-600 hover:border-blue-400 hover:text-blue-600 dark:border-dark-600 dark:text-gray-400 dark:hover:border-blue-500 dark:hover:text-blue-400"
              :class="form.cron_expr === preset.value ? 'border-blue-400 bg-blue-50 text-blue-600 dark:border-blue-500 dark:bg-blue-900/20 dark:text-blue-400' : ''"
            >
              {{ preset.label }}
            </button>
          </div>
        </div>

        <!-- 启用开关 -->
        <div class="flex items-center gap-3">
          <button
            type="button"
            @click="form.enabled = !form.enabled"
            class="relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none"
            :class="form.enabled ? 'bg-blue-600' : 'bg-gray-200 dark:bg-dark-600'"
          >
            <span
              class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
              :class="form.enabled ? 'translate-x-5' : 'translate-x-0'"
            />
          </button>
          <span class="text-sm text-gray-700 dark:text-gray-300">
            {{ t('admin.sub2apiProviders.scheduleEnabled') }}
          </span>
        </div>

        <!-- 上次 / 下次执行时间 -->
        <div v-if="schedule" class="flex gap-6 rounded-lg bg-gray-50 p-3 text-sm dark:bg-dark-700">
          <div>
            <span class="text-gray-400 dark:text-gray-500">{{ t('admin.sub2apiProviders.lastRunAt') }}：</span>
            <span class="text-gray-700 dark:text-gray-300">{{ formatTime(schedule.last_run_at) }}</span>
          </div>
          <div>
            <span class="text-gray-400 dark:text-gray-500">{{ t('admin.sub2apiProviders.nextRunAt') }}：</span>
            <span class="text-gray-700 dark:text-gray-300">{{ formatTime(schedule.next_run_at) }}</span>
          </div>
        </div>

        <!-- 最近执行日志 -->
        <div>
          <h4 class="mb-2 text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('admin.sub2apiProviders.recentLogs') }}
          </h4>

          <!-- 执行中提示条：立即执行为后台异步任务，轮询期间告知用户「正在跑、完成自动刷新」 -->
          <div
            v-if="runningNow"
            class="mb-2 flex items-center gap-2 rounded-lg bg-blue-50 px-3 py-2 text-xs text-blue-600 dark:bg-blue-900/20 dark:text-blue-400"
          >
            <Icon name="refresh" size="sm" class="animate-spin" />
            {{ t('admin.sub2apiProviders.runningHint') }}
          </div>

          <div v-if="logsLoading" class="flex justify-center py-4">
            <Icon name="refresh" size="sm" class="animate-spin text-gray-400" />
          </div>

          <div v-else-if="logs.length === 0" class="rounded-lg border border-dashed border-gray-200 py-6 text-center text-sm text-gray-400 dark:border-dark-600">
            {{ t('admin.sub2apiProviders.noLogs') }}
          </div>

          <div v-else class="divide-y divide-gray-100 rounded-lg border border-gray-200 dark:divide-dark-700 dark:border-dark-600">
            <div v-for="log in logs" :key="log.id">
              <!-- 概要行（可点击展开明细） -->
              <button
                type="button"
                class="flex w-full items-start gap-3 px-3 py-2.5 text-left hover:bg-gray-50 dark:hover:bg-dark-700/50"
                @click="toggleLog(log.id)"
              >
                <!-- 状态图标 -->
                <span
                  class="mt-0.5 inline-flex h-5 w-5 flex-shrink-0 items-center justify-center rounded-full text-xs font-semibold"
                  :class="{
                    'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400': log.status === 'success',
                    'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400': log.status === 'failed',
                    'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400': log.status === 'partial',
                  }"
                >
                  {{ log.status === 'success' ? '✓' : log.status === 'failed' ? '✗' : '!' }}
                </span>

                <!-- 信息 -->
                <div class="min-w-0 flex-1">
                  <div class="flex flex-wrap items-center gap-2 text-xs">
                    <span class="font-medium text-gray-700 dark:text-gray-300">
                      {{ t(`admin.sub2apiProviders.logStatus.${log.status}`) }}
                    </span>
                    <span class="text-gray-400">·</span>
                    <span class="text-gray-500">{{ t('admin.sub2apiProviders.logTotal', { total: log.total }) }}</span>
                    <span v-if="log.optimized > 0" class="text-green-600 dark:text-green-400">
                      {{ t('admin.sub2apiProviders.logOptimized', { count: log.optimized }) }}
                    </span>
                    <span v-if="log.skipped > 0" class="text-gray-400">
                      {{ t('admin.sub2apiProviders.logSkipped', { count: log.skipped }) }}
                    </span>
                    <span v-if="log.failed > 0" class="text-red-500 dark:text-red-400">
                      {{ t('admin.sub2apiProviders.logFailed', { count: log.failed }) }}
                    </span>
                  </div>
                  <div class="mt-0.5 flex items-center gap-2 text-xs text-gray-400">
                    <span>{{ formatDateTime(log.started_at) }}</span>
                    <span v-if="formatDuration(log)">· {{ t('admin.sub2apiProviders.logDuration', { duration: formatDuration(log) }) }}</span>
                  </div>
                </div>

                <!-- 展开箭头 -->
                <Icon
                  v-if="log.detail && log.detail.length > 0"
                  name="chevronDown"
                  size="sm"
                  class="mt-0.5 flex-shrink-0 text-gray-400 transition-transform"
                  :class="expandedLogs.has(log.id) ? 'rotate-180' : ''"
                />
              </button>

              <!-- 明细（展开时显示每个账号的处理结果） -->
              <div
                v-if="expandedLogs.has(log.id) && log.detail && log.detail.length > 0"
                class="space-y-1.5 border-t border-gray-100 bg-gray-50/60 px-3 py-2.5 dark:border-dark-700 dark:bg-dark-900/30"
              >
                <div
                  v-for="(d, di) in log.detail"
                  :key="di"
                  class="flex items-start gap-2 text-xs"
                >
                  <span
                    class="mt-0.5 inline-flex h-4 w-4 flex-shrink-0 items-center justify-center rounded-full text-[10px] font-semibold"
                    :class="{
                      'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400': d.status === 'optimized',
                      'bg-gray-200 text-gray-500 dark:bg-dark-600 dark:text-gray-400': d.status === 'skipped',
                      'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400': d.status === 'failed',
                    }"
                  >
                    {{ d.status === 'optimized' ? '✓' : d.status === 'failed' ? '✗' : '–' }}
                  </span>
                  <div class="min-w-0 flex-1">
                    <div class="flex flex-wrap items-center gap-1.5">
                      <span class="font-medium text-gray-700 dark:text-gray-300">{{ d.account_name || `#${d.account_id}` }}</span>
                      <span class="rounded px-1 py-0.5 text-[10px]"
                        :class="{
                          'bg-green-50 text-green-600 dark:bg-green-900/20 dark:text-green-400': d.status === 'optimized',
                          'bg-gray-100 text-gray-500 dark:bg-dark-700 dark:text-gray-400': d.status === 'skipped',
                          'bg-red-50 text-red-500 dark:bg-red-900/20 dark:text-red-400': d.status === 'failed',
                        }"
                      >{{ t(`admin.sub2apiProviders.detailStatus.${d.status}`) }}</span>
                      <!-- 分组变化 -->
                      <span v-if="d.status === 'optimized'" class="font-mono text-gray-500 dark:text-gray-400">
                        {{ d.old_group || '—' }}
                        <span v-if="d.old_multiplier != null" class="text-gray-400">(×{{ d.old_multiplier }})</span>
                        →
                        <span class="text-green-600 dark:text-green-400">{{ d.new_group }}</span>
                        <span v-if="d.new_multiplier != null" class="text-green-500">(×{{ d.new_multiplier }})</span>
                      </span>
                    </div>
                    <div v-if="d.reason" class="mt-0.5 break-all text-gray-400">{{ d.reason }}</div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </template>
    </div>

    <template #footer>
      <div class="flex items-center justify-between gap-3">
        <!-- 左：立即执行 + 删除 -->
        <div class="flex gap-2">
          <button
            v-if="schedule"
            type="button"
            :disabled="runningNow"
            @click="handleRunNow"
            class="btn btn-secondary btn-sm"
          >
            <Icon :name="runningNow ? 'refresh' : 'bolt'" size="sm" class="mr-1" :class="runningNow ? 'animate-spin' : ''" />
            {{ t('admin.sub2apiProviders.runNow') }}
          </button>
          <button
            v-if="schedule"
            type="button"
            :disabled="saving"
            @click="handleDelete"
            class="btn btn-sm text-red-600 hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-900/20"
          >
            <Icon name="trash" size="sm" class="mr-1" />
            {{ t('admin.sub2apiProviders.deleteSchedule') }}
          </button>
        </div>

        <!-- 右：取消 + 保存 -->
        <div class="flex gap-2">
          <button type="button" @click="emit('close')" class="btn btn-secondary btn-sm">
            {{ t('common.cancel') }}
          </button>
          <button
            type="button"
            :disabled="saving || !form.cron_expr.trim()"
            @click="handleSave"
            class="btn btn-primary btn-sm"
          >
            <Icon v-if="saving" name="refresh" size="sm" class="mr-1 animate-spin" />
            {{ t('common.save') }}
          </button>
        </div>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, reactive, watch, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import {
  getOptimizeSchedule,
  upsertOptimizeSchedule,
  deleteOptimizeSchedule,
  runOptimizeNow,
  getLinkedAccounts,
  type OptimizeScheduleInfo,
  type OptimizeLogInfo,
} from '@/api/admin/sub2apiProviders'
import { useAppStore } from '@/stores/app'
import { findIncompleteAccounts } from '@/utils/sub2apiValidation'
import { extractErrorMessage } from '@/utils/errorHandler'

const props = defineProps<{
  show: boolean
  providerId: number
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'saved', schedule: OptimizeScheduleInfo): void
  (e: 'deleted'): void
}>()

const { t } = useI18n()
const appStore = useAppStore()

// ── 状态 ────────────────────────────────────────────────────────────────────
const loading = ref(false)
const logsLoading = ref(false)
const saving = ref(false)
const runningNow = ref(false)

const schedule = ref<OptimizeScheduleInfo | null>(null)
const logs = ref<OptimizeLogInfo[]>([])

// 展开的日志 ID 集合（点击概要行展开账号明细）
const expandedLogs = ref<Set<number>>(new Set())
function toggleLog(id: number) {
  const next = new Set(expandedLogs.value)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  expandedLogs.value = next
}

const form = reactive({
  cron_expr: '0 2 * * *',
  enabled: true,
})

// ── Cron 快捷预设 ────────────────────────────────────────────────────────────
const cronPresets = [
  { label: t('admin.sub2apiProviders.cronPresets.hourly'), value: '0 * * * *' },
  { label: t('admin.sub2apiProviders.cronPresets.daily2am'), value: '0 2 * * *' },
  { label: t('admin.sub2apiProviders.cronPresets.daily6am'), value: '0 6 * * *' },
  { label: t('admin.sub2apiProviders.cronPresets.every6h'), value: '0 */6 * * *' },
  { label: t('admin.sub2apiProviders.cronPresets.weekly'), value: '0 2 * * 1' },
]

// ── 加载数据 ──────────────────────────────────────────────────────────────────
// silent=true 时不显示全屏 spinner（用于「立即执行」后的轮询刷新，只更新日志区）
async function loadData(silent = false) {
  if (!silent) loading.value = true
  try {
    const resp = await getOptimizeSchedule(props.providerId)
    schedule.value = resp.schedule
    logs.value = resp.logs

    // 表单只在首次加载时回填，避免轮询覆盖用户正在编辑的输入
    if (!silent) {
      if (resp.schedule) {
        form.cron_expr = resp.schedule.cron_expr
        form.enabled = resp.schedule.enabled
      } else {
        form.cron_expr = '0 2 * * *'
        form.enabled = true
      }
    }
  } catch {
    if (!silent) appStore.showError(t('admin.sub2apiProviders.scheduleLoadFailed'))
  } finally {
    if (!silent) loading.value = false
  }
}

watch(
  () => props.show,
  (visible) => {
    if (visible) loadData()
    else stopPolling()
  },
  { immediate: true }
)

onUnmounted(() => stopPolling())

// ── 轮询「立即执行」结果 ────────────────────────────────────────────────────────
let pollTimer: ReturnType<typeof setInterval> | null = null

function stopPolling() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
  runningNow.value = false
}

// ── 操作 ──────────────────────────────────────────────────────────────────────
async function handleSave() {
  if (!form.cron_expr.trim()) return
  saving.value = true
  try {
    const result = await upsertOptimizeSchedule(props.providerId, {
      cron_expr: form.cron_expr.trim(),
      enabled: form.enabled,
    })
    schedule.value = result
    appStore.showSuccess(t('admin.sub2apiProviders.scheduleSaved'))
    emit('saved', result)
  } catch {
    appStore.showError(t('admin.sub2apiProviders.scheduleSaveFailed'))
  } finally {
    saving.value = false
  }
}

// 立即执行：后端异步触发（立即返回），前端记录当前日志数并轮询，出现新日志即停止。
async function handleRunNow() {
  // 校验:执行前检查该 provider 下所有账号是否都填写了上限、下限、测试模型
  try {
    const accounts = await getLinkedAccounts(props.providerId)
    const incomplete = findIncompleteAccounts(accounts)
    if (incomplete.length > 0) {
      const names = incomplete.map(a => a.name).join('、')
      appStore.showError(t('admin.sub2apiProviders.optimizeAllIncomplete', { accounts: names }))
      return
    }
  } catch (e: any) {
    appStore.showError(extractErrorMessage(e, t('common.loadFailed')))
    return
  }

  runningNow.value = true
  const prevLogCount = logs.value.length
  try {
    await runOptimizeNow(props.providerId)
    appStore.showSuccess(t('admin.sub2apiProviders.runNowSuccess'))

    stopPolling()
    runningNow.value = true // stopPolling 会置 false，这里恢复
    let ticks = 0
    pollTimer = setInterval(async () => {
      ticks++
      await loadData(true)
      // 出现新日志（说明本次执行已完成）或轮询超过 90s 则停止
      if (logs.value.length > prevLogCount || ticks >= 30) {
        stopPolling()
      }
    }, 3000)
  } catch (e: any) {
    appStore.showError(extractErrorMessage(e, t('admin.sub2apiProviders.runNowFailed')))
    stopPolling()
  }
}

async function handleDelete() {
  if (!confirm(t('admin.sub2apiProviders.deleteScheduleConfirm'))) return
  saving.value = true
  try {
    await deleteOptimizeSchedule(props.providerId)
    schedule.value = null
    logs.value = []
    appStore.showSuccess(t('admin.sub2apiProviders.scheduleDeleted'))
    emit('deleted')
  } catch {
    appStore.showError(t('admin.sub2apiProviders.scheduleSaveFailed'))
  } finally {
    saving.value = false
  }
}

// ── 格式化 ────────────────────────────────────────────────────────────────────
function formatTime(val?: string | null): string {
  if (!val) return t('admin.sub2apiProviders.neverRun')
  return new Date(val).toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

function formatDateTime(val?: string | null): string {
  if (!val) return '—'
  return new Date(val).toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

// 用 started_at/finished_at 计算执行耗时，返回如「3.2s」「1m20s」，无法计算时返回空串
function formatDuration(log: OptimizeLogInfo): string {
  if (!log.started_at || !log.finished_at) return ''
  const ms = new Date(log.finished_at).getTime() - new Date(log.started_at).getTime()
  if (Number.isNaN(ms) || ms < 0) return ''
  const sec = ms / 1000
  if (sec < 60) return `${sec.toFixed(1)}s`
  const m = Math.floor(sec / 60)
  const s = Math.round(sec % 60)
  return `${m}m${s}s`
}
</script>

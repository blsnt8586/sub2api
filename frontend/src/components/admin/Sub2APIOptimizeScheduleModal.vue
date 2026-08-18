<template>
  <BaseDialog
    :show="show"
    :title="t('admin.sub2apiProviders.scheduleOptimizeTitle')"
    width="wide"
    @close="emit('close')"
  >
    <div class="space-y-5">
      <p class="text-sm text-gray-500 dark:text-gray-400">
        {{ t('admin.sub2apiProviders.scheduleOptimizeDesc') }}
      </p>

      <div v-if="loading" class="flex items-center justify-center py-8">
        <Icon name="refresh" size="lg" class="animate-spin text-gray-400" />
      </div>

      <template v-else>
        <div class="space-y-3">
          <div class="space-y-1">
            <label for="optimize-cron" class="block text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.sub2apiProviders.cronExpr') }}
            </label>
            <input
              id="optimize-cron"
              v-model="form.cron_expr"
              type="text"
              :placeholder="t('admin.sub2apiProviders.cronExprPlaceholder')"
              class="input w-full font-mono text-sm"
            />
            <div class="flex flex-wrap gap-2 pt-1">
              <button
                v-for="preset in cronPresets"
                :key="preset.value"
                type="button"
                class="rounded-md border border-gray-200 px-2 py-0.5 text-xs text-gray-600 hover:border-blue-400 hover:text-blue-600 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 dark:border-dark-600 dark:text-gray-400 dark:hover:border-blue-500 dark:hover:text-blue-400"
                :class="form.cron_expr === preset.value ? 'border-blue-400 bg-blue-50 text-blue-600 dark:border-blue-500 dark:bg-blue-900/20 dark:text-blue-400' : ''"
                @click="form.cron_expr = preset.value"
              >
                {{ preset.label }}
              </button>
            </div>
          </div>

          <div class="flex items-center gap-3">
            <button
              type="button"
              role="switch"
              :aria-checked="form.enabled"
              :aria-label="t('admin.sub2apiProviders.scheduleEnabled')"
              class="relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
              :class="form.enabled ? 'bg-blue-600' : 'bg-gray-200 dark:bg-dark-600'"
              @click="form.enabled = !form.enabled"
            >
              <span
                class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200"
                :class="form.enabled ? 'translate-x-5' : 'translate-x-0'"
              />
            </button>
            <span class="text-sm text-gray-700 dark:text-gray-300">
              {{ t('admin.sub2apiProviders.scheduleEnabled') }}
            </span>
          </div>

          <div
            v-if="schedule"
            class="grid gap-2 bg-gray-50 px-3 py-2.5 text-sm dark:bg-dark-700 sm:grid-cols-2"
          >
            <div>
              <span class="text-gray-400 dark:text-gray-500">{{ t('admin.sub2apiProviders.lastRunAt') }}：</span>
              <span class="text-gray-700 dark:text-gray-300">{{ formatTime(schedule.last_run_at) }}</span>
            </div>
            <div>
              <span class="text-gray-400 dark:text-gray-500">{{ t('admin.sub2apiProviders.nextRunAt') }}：</span>
              <span class="text-gray-700 dark:text-gray-300">{{ formatTime(schedule.next_run_at) }}</span>
            </div>
          </div>
        </div>

        <section class="space-y-3 border-t border-gray-200 pt-4 dark:border-dark-700">
          <div class="flex flex-wrap items-center justify-between gap-2">
            <h4 class="text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.sub2apiProviders.recentLogs') }}
            </h4>
            <span class="text-xs text-gray-400">
              {{ t('admin.sub2apiProviders.logResultCount', { total: logsTotal }) }}
            </span>
          </div>

          <div
            v-if="runningNow"
            class="flex items-center gap-2 bg-blue-50 px-3 py-2 text-xs text-blue-600 dark:bg-blue-900/20 dark:text-blue-400"
          >
            <Icon name="refresh" size="sm" class="animate-spin" />
            {{ t('admin.sub2apiProviders.runningHint') }}
          </div>

          <form
            class="grid grid-cols-1 gap-2 border-y border-gray-100 py-3 dark:border-dark-700 sm:grid-cols-2 lg:grid-cols-3"
            @submit.prevent="applyFilters"
          >
            <label class="space-y-1 text-xs text-gray-500 dark:text-gray-400">
              <span>{{ t('admin.sub2apiProviders.logTriggerFilter') }}</span>
              <select v-model="filters.trigger" class="input w-full text-sm">
                <option value="">{{ t('admin.sub2apiProviders.filterAll') }}</option>
                <option v-for="trigger in triggerOptions" :key="trigger" :value="trigger">
                  {{ triggerLabel(trigger) }}
                </option>
              </select>
            </label>

            <label class="space-y-1 text-xs text-gray-500 dark:text-gray-400">
              <span>{{ t('admin.sub2apiProviders.logStatusFilter') }}</span>
              <select v-model="filters.status" class="input w-full text-sm">
                <option value="">{{ t('admin.sub2apiProviders.filterAll') }}</option>
                <option v-for="status in statusOptions" :key="status" :value="status">
                  {{ t(`admin.sub2apiProviders.logStatus.${status}`) }}
                </option>
              </select>
            </label>

            <label class="space-y-1 text-xs text-gray-500 dark:text-gray-400">
              <span>{{ t('admin.sub2apiProviders.logAccountFilter') }}</span>
              <select v-model="filters.account_id" class="input w-full text-sm">
                <option value="">{{ t('admin.sub2apiProviders.filterAll') }}</option>
                <option v-for="account in linkedAccounts" :key="account.id" :value="String(account.id)">
                  {{ account.name }} · {{ account.platform }}
                </option>
              </select>
            </label>

            <label class="space-y-1 text-xs text-gray-500 dark:text-gray-400">
              <span>{{ t('admin.sub2apiProviders.logKeywordFilter') }}</span>
              <input
                v-model="filters.keyword"
                type="search"
                maxlength="200"
                class="input w-full text-sm"
                :placeholder="t('admin.sub2apiProviders.logKeywordPlaceholder')"
              />
            </label>

            <label class="space-y-1 text-xs text-gray-500 dark:text-gray-400">
              <span>{{ t('admin.sub2apiProviders.logFromFilter') }}</span>
              <input v-model="filters.from" type="datetime-local" class="input w-full text-sm" />
            </label>

            <label class="space-y-1 text-xs text-gray-500 dark:text-gray-400">
              <span>{{ t('admin.sub2apiProviders.logToFilter') }}</span>
              <input v-model="filters.to" type="datetime-local" class="input w-full text-sm" />
            </label>

            <div class="flex items-end justify-end gap-2 sm:col-span-2 lg:col-span-3">
              <button
                type="button"
                class="btn btn-secondary btn-sm"
                :title="t('admin.sub2apiProviders.resetFilters')"
                @click="resetFilters"
              >
                <Icon name="x" size="sm" class="mr-1" />
                {{ t('admin.sub2apiProviders.resetFilters') }}
              </button>
              <button type="submit" class="btn btn-primary btn-sm">
                <Icon name="filter" size="sm" class="mr-1" />
                {{ t('admin.sub2apiProviders.applyFilters') }}
              </button>
            </div>
          </form>

          <div v-if="logsLoading" class="flex justify-center py-8">
            <Icon name="refresh" size="sm" class="animate-spin text-gray-400" />
          </div>

          <div
            v-else-if="logsError"
            class="flex items-center justify-between gap-3 border border-red-200 bg-red-50 px-3 py-3 text-sm text-red-600 dark:border-red-900/50 dark:bg-red-900/20 dark:text-red-400"
          >
            <span>{{ t('admin.sub2apiProviders.logsLoadFailed') }}</span>
            <button type="button" class="btn btn-secondary btn-sm" @click="loadLogs()">
              {{ t('admin.sub2apiProviders.retryLogs') }}
            </button>
          </div>

          <div
            v-else-if="logs.length === 0"
            class="border border-dashed border-gray-200 py-7 text-center text-sm text-gray-400 dark:border-dark-600"
          >
            {{ t('admin.sub2apiProviders.noLogs') }}
          </div>

          <div v-else class="overflow-hidden border border-gray-200 dark:border-dark-600">
            <div class="max-h-[26rem] divide-y divide-gray-100 overflow-y-auto dark:divide-dark-700">
              <div v-for="log in logs" :key="log.id">
                <button
                  type="button"
                  class="flex w-full items-start gap-3 px-3 py-2.5 text-left hover:bg-gray-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-blue-500 dark:hover:bg-dark-700/50"
                  @click="toggleLog(log.id)"
                >
                  <span
                    class="mt-0.5 inline-flex h-5 w-5 flex-shrink-0 items-center justify-center rounded-full text-xs font-semibold"
                    :class="statusIconClass(log.status)"
                  >
                    {{ statusIcon(log.status) }}
                  </span>

                  <div class="min-w-0 flex-1">
                    <div class="flex flex-wrap items-center gap-1.5 text-xs">
                      <span class="font-medium text-gray-700 dark:text-gray-300">
                        {{ t(`admin.sub2apiProviders.logStatus.${log.status}`) }}
                      </span>
                      <span class="rounded px-1.5 py-0.5 text-[10px] font-medium" :class="triggerClass(log.trigger)">
                        {{ triggerLabel(log.trigger) }}
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
                    <div class="mt-0.5 flex flex-wrap items-center gap-2 text-xs text-gray-400">
                      <span>{{ formatDateTime(log.started_at || log.created_at) }}</span>
                      <span v-if="formatDuration(log)">· {{ t('admin.sub2apiProviders.logDuration', { duration: formatDuration(log) }) }}</span>
                      <span>· #{{ log.id }}</span>
                    </div>
                  </div>

                  <Icon
                    v-if="log.detail && log.detail.length > 0"
                    name="chevronDown"
                    size="sm"
                    class="mt-0.5 flex-shrink-0 text-gray-400 transition-transform"
                    :class="expandedLogs.has(log.id) ? 'rotate-180' : ''"
                  />
                </button>

                <div
                  v-if="expandedLogs.has(log.id) && log.detail && log.detail.length > 0"
                  class="space-y-2 border-t border-gray-100 bg-gray-50/60 px-3 py-2.5 dark:border-dark-700 dark:bg-dark-900/30"
                >
                  <div v-for="(detail, detailIndex) in log.detail" :key="detailIndex" class="text-xs">
                    <div class="flex items-start gap-2">
                      <span
                        v-if="detail.status"
                        class="mt-0.5 inline-flex h-4 w-4 flex-shrink-0 items-center justify-center rounded-full text-[10px] font-semibold"
                        :class="detailStatusClass(detail.status)"
                      >
                        {{ detail.status === 'optimized' ? '✓' : detail.status === 'failed' ? '✗' : '–' }}
                      </span>
                      <span v-else class="mt-0.5 h-4 w-4 flex-shrink-0" />

                      <div class="min-w-0 flex-1">
                        <div class="flex flex-wrap items-center gap-1.5">
                          <span class="font-medium text-gray-700 dark:text-gray-300">
                            {{ detail.account_name || (detail.account_id ? `#${detail.account_id}` : t('admin.sub2apiProviders.runLevelLog')) }}
                          </span>
                          <span
                            v-if="detail.status"
                            class="rounded px-1 py-0.5 text-[10px]"
                            :class="detailStatusClass(detail.status)"
                          >
                            {{ t(`admin.sub2apiProviders.detailStatus.${detail.status}`) }}
                          </span>
                          <span v-if="detail.status === 'optimized'" class="font-mono text-gray-500 dark:text-gray-400">
                            {{ detail.old_group || '—' }}
                            <span v-if="detail.old_multiplier != null" class="text-gray-400">(×{{ detail.old_multiplier }})</span>
                            →
                            <span class="text-green-600 dark:text-green-400">{{ detail.new_group }}</span>
                            <span v-if="detail.new_multiplier != null" class="text-green-500">(×{{ detail.new_multiplier }})</span>
                          </span>
                        </div>

                        <div
                          v-if="detail.trigger === 'probe_unhealthy'"
                          class="mt-0.5 flex flex-wrap items-center gap-x-1 text-[11px] text-amber-700 dark:text-amber-400"
                        >
                          <span class="font-medium">{{ t('admin.sub2apiProviders.probeAutoOptimizeTrigger') }}</span>
                          <span>·</span>
                          <span>{{ t('admin.sub2apiProviders.probeAutoOptimizeReason', { threshold: detail.failure_threshold ?? '-' }) }}</span>
                        </div>

                        <div v-if="detail.switch_events?.length" class="mt-2 space-y-1 border-l-2 border-gray-200 pl-2 dark:border-dark-600">
                          <div
                            v-for="(event, eventIndex) in detail.switch_events"
                            :key="eventIndex"
                            data-test="optimize-switch-event"
                            class="text-[11px] text-gray-500 dark:text-gray-400"
                          >
                            <div class="flex flex-wrap items-center gap-1.5">
                              <span class="font-medium" :class="event.action === 'rollback' ? 'text-amber-600 dark:text-amber-400' : 'text-blue-600 dark:text-blue-400'">
                                {{ t(`admin.sub2apiProviders.switchAction.${event.action}`) }}
                              </span>
                              <span class="font-mono">
                                {{ event.from_group || '—' }}
                                <span v-if="event.from_multiplier != null">(×{{ event.from_multiplier }})</span>
                                → {{ event.to_group || `#${event.to_group_id}` }}
                                <span v-if="event.to_multiplier != null">(×{{ event.to_multiplier }})</span>
                              </span>
                              <span :class="event.status === 'success' ? 'text-green-600 dark:text-green-400' : 'text-red-500 dark:text-red-400'">
                                {{ t(`admin.sub2apiProviders.switchStatus.${event.status}`) }}
                              </span>
                              <span v-if="event.test_status" :class="event.test_status === 'passed' ? 'text-green-600 dark:text-green-400' : 'text-red-500 dark:text-red-400'">
                                · {{ t(`admin.sub2apiProviders.switchTestStatus.${event.test_status}`) }}
                              </span>
                              <span class="text-gray-400">{{ formatEventTime(event.occurred_at) }}</span>
                            </div>
                            <div v-if="event.reason" class="mt-0.5 break-all text-gray-400">{{ event.reason }}</div>
                          </div>
                        </div>

                        <div v-if="detail.reason" class="mt-0.5 break-all text-gray-400">{{ detail.reason }}</div>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </div>

            <Pagination
              v-if="logsTotal > logsPageSize"
              :total="logsTotal"
              :page="logsPage"
              :page-size="logsPageSize"
              :page-size-options="[10, 20, 50]"
              @update:page="handlePageChange"
              @update:page-size="handlePageSizeChange"
            />
          </div>
        </section>
      </template>
    </div>

    <template #footer>
      <div class="flex w-full flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div class="flex flex-wrap gap-2">
          <button
            v-if="schedule"
            type="button"
            :disabled="runningNow"
            class="btn btn-secondary btn-sm"
            @click="handleRunNow"
          >
            <Icon :name="runningNow ? 'refresh' : 'bolt'" size="sm" class="mr-1" :class="runningNow ? 'animate-spin' : ''" />
            {{ t('admin.sub2apiProviders.runNow') }}
          </button>
          <button
            v-if="schedule"
            type="button"
            :disabled="saving"
            class="btn btn-sm text-red-600 hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-900/20"
            @click="handleDelete"
          >
            <Icon name="trash" size="sm" class="mr-1" />
            {{ t('admin.sub2apiProviders.deleteSchedule') }}
          </button>
        </div>

        <div class="flex justify-end gap-2">
          <button type="button" class="btn btn-secondary btn-sm" @click="emit('close')">
            {{ t('common.cancel') }}
          </button>
          <button
            type="button"
            :disabled="saving || !form.cron_expr.trim()"
            class="btn btn-primary btn-sm"
            @click="handleSave"
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
import { onUnmounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Pagination from '@/components/common/Pagination.vue'
import Icon from '@/components/icons/Icon.vue'
import {
  deleteOptimizeSchedule,
  getLinkedAccounts,
  getOptimizeSchedule,
  listOptimizeLogs,
  runOptimizeNow,
  upsertOptimizeSchedule,
  type LinkedAccountInfo,
  type OptimizeLogInfo,
  type OptimizeLogTrigger,
  type OptimizeScheduleInfo,
} from '@/api/admin/sub2apiProviders'
import { useAppStore } from '@/stores/app'
import { findIncompleteParticipatingAccounts } from '@/utils/sub2apiValidation'
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

const loading = ref(false)
const logsLoading = ref(false)
const logsError = ref(false)
const saving = ref(false)
const runningNow = ref(false)
const schedule = ref<OptimizeScheduleInfo | null>(null)
const logs = ref<OptimizeLogInfo[]>([])
const linkedAccounts = ref<LinkedAccountInfo[]>([])
const logsTotal = ref(0)
const logsPage = ref(1)
const logsPageSize = ref(20)
let scheduleLoadInFlight = false
let logsLoadInFlight = false

const triggerOptions: OptimizeLogTrigger[] = [
  'cron',
  'schedule_now',
  'probe_unhealthy',
  'manual_account',
  'manual_all',
  'legacy',
]
const statusOptions: OptimizeLogInfo['status'][] = ['success', 'partial', 'failed', 'skipped']

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

const filters = reactive({
  trigger: '' as OptimizeLogTrigger | '',
  status: '' as OptimizeLogInfo['status'] | '',
  account_id: '',
  keyword: '',
  from: '',
  to: '',
})

const cronPresets = [
  { label: t('admin.sub2apiProviders.cronPresets.hourly'), value: '0 * * * *' },
  { label: t('admin.sub2apiProviders.cronPresets.daily2am'), value: '0 2 * * *' },
  { label: t('admin.sub2apiProviders.cronPresets.daily6am'), value: '0 6 * * *' },
  { label: t('admin.sub2apiProviders.cronPresets.every6h'), value: '0 */6 * * *' },
  { label: t('admin.sub2apiProviders.cronPresets.weekly'), value: '0 2 * * 1' },
]

async function loadSchedule(silent = false) {
  if (scheduleLoadInFlight) return
  scheduleLoadInFlight = true
  try {
    const response = await getOptimizeSchedule(props.providerId)
    schedule.value = response.schedule
    if (!silent) {
      form.cron_expr = response.schedule?.cron_expr ?? '0 2 * * *'
      form.enabled = response.schedule?.enabled ?? true
    }
  } catch {
    if (!silent) appStore.showError(t('admin.sub2apiProviders.scheduleLoadFailed'))
  } finally {
    scheduleLoadInFlight = false
  }
}

async function loadAccounts() {
  try {
    linkedAccounts.value = await getLinkedAccounts(props.providerId)
  } catch {
    linkedAccounts.value = []
  }
}

function toRFC3339(value: string): string | undefined {
  if (!value) return undefined
  const parsed = new Date(value)
  return Number.isNaN(parsed.getTime()) ? undefined : parsed.toISOString()
}

async function loadLogs(silent = false) {
  if (logsLoadInFlight) return
  logsLoadInFlight = true
  logsError.value = false
  if (!silent) logsLoading.value = true
  try {
    const response = await listOptimizeLogs(props.providerId, {
      trigger: filters.trigger,
      status: filters.status,
      account_id: filters.account_id ? Number(filters.account_id) : undefined,
      keyword: filters.keyword,
      from: toRFC3339(filters.from),
      to: toRFC3339(filters.to),
      page: logsPage.value,
      page_size: logsPageSize.value,
    })
    logs.value = response.items
    logsTotal.value = response.total
    const visibleIDs = new Set(response.items.map(log => log.id))
    expandedLogs.value = new Set([...expandedLogs.value].filter(id => visibleIDs.has(id)))
  } catch {
    logsError.value = true
  } finally {
    logsLoadInFlight = false
    if (!silent) logsLoading.value = false
  }
}

async function loadInitial() {
  loading.value = true
  logsPage.value = 1
  expandedLogs.value = new Set()
  try {
    await Promise.all([loadSchedule(false), loadAccounts(), loadLogs(true)])
  } finally {
    loading.value = false
  }
}

let runPollTimer: ReturnType<typeof setInterval> | null = null
let refreshTimer: ReturnType<typeof setInterval> | null = null

function stopRunPolling() {
  if (runPollTimer) {
    clearInterval(runPollTimer)
    runPollTimer = null
  }
  runningNow.value = false
}

function stopRefreshPolling() {
  if (refreshTimer) {
    clearInterval(refreshTimer)
    refreshTimer = null
  }
}

function startRefreshPolling() {
  stopRefreshPolling()
  refreshTimer = setInterval(() => {
    if (!props.show || document.hidden || runningNow.value) return
    void loadSchedule(true)
    void loadLogs(true)
  }, 30_000)
}

watch(
  () => [props.show, props.providerId] as const,
  ([visible]) => {
    stopRunPolling()
    stopRefreshPolling()
    if (visible) {
      void loadInitial()
      startRefreshPolling()
    }
  },
  { immediate: true }
)

onUnmounted(() => {
  stopRunPolling()
  stopRefreshPolling()
})

async function applyFilters() {
  logsPage.value = 1
  expandedLogs.value = new Set()
  await loadLogs()
}

async function resetFilters() {
  filters.trigger = ''
  filters.status = ''
  filters.account_id = ''
  filters.keyword = ''
  filters.from = ''
  filters.to = ''
  logsPage.value = 1
  expandedLogs.value = new Set()
  await loadLogs()
}

async function handlePageChange(page: number) {
  logsPage.value = page
  expandedLogs.value = new Set()
  await loadLogs()
}

async function handlePageSizeChange(pageSize: number) {
  logsPageSize.value = pageSize
  logsPage.value = 1
  expandedLogs.value = new Set()
  await loadLogs()
}

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

async function handleRunNow() {
  try {
    const accounts = await getLinkedAccounts(props.providerId)
    const incomplete = findIncompleteParticipatingAccounts(accounts)
    if (incomplete.length > 0) {
      appStore.showError(t('admin.sub2apiProviders.optimizeAllIncomplete', {
        accounts: incomplete.map(account => account.name).join('、'),
      }))
      return
    }

    const baseline = await listOptimizeLogs(props.providerId, { page: 1, page_size: 1 })
    const previousLatestLogID = baseline.items[0]?.id
    runningNow.value = true
    await runOptimizeNow(props.providerId)
    appStore.showSuccess(t('admin.sub2apiProviders.runNowSuccess'))

    stopRunPolling()
    runningNow.value = true
    let ticks = 0
    runPollTimer = setInterval(async () => {
      ticks++
      try {
        const latest = await listOptimizeLogs(props.providerId, { page: 1, page_size: 1 })
        if (latest.items[0]?.id !== previousLatestLogID || ticks >= 30) {
          stopRunPolling()
          await Promise.all([loadSchedule(true), loadLogs(true)])
        }
      } catch {
        if (ticks >= 30) stopRunPolling()
      }
    }, 3000)
  } catch (error: unknown) {
    appStore.showError(extractErrorMessage(error, t('admin.sub2apiProviders.runNowFailed')))
    stopRunPolling()
  }
}

async function handleDelete() {
  if (!confirm(t('admin.sub2apiProviders.deleteScheduleConfirm'))) return
  saving.value = true
  try {
    await deleteOptimizeSchedule(props.providerId)
    schedule.value = null
    form.cron_expr = '0 2 * * *'
    form.enabled = true
    await loadLogs(true)
    appStore.showSuccess(t('admin.sub2apiProviders.scheduleDeleted'))
    emit('deleted')
  } catch {
    appStore.showError(t('admin.sub2apiProviders.scheduleSaveFailed'))
  } finally {
    saving.value = false
  }
}

function triggerLabel(trigger: OptimizeLogTrigger): string {
  return t(`admin.sub2apiProviders.logTrigger.${trigger}`)
}

function triggerClass(trigger: OptimizeLogTrigger): string {
  if (trigger === 'probe_unhealthy') return 'bg-amber-50 text-amber-700 dark:bg-amber-900/20 dark:text-amber-400'
  if (trigger === 'manual_account' || trigger === 'manual_all') return 'bg-cyan-50 text-cyan-700 dark:bg-cyan-900/20 dark:text-cyan-400'
  if (trigger === 'legacy') return 'bg-gray-100 text-gray-500 dark:bg-dark-600 dark:text-gray-400'
  return 'bg-blue-50 text-blue-700 dark:bg-blue-900/20 dark:text-blue-400'
}

function statusIcon(status: OptimizeLogInfo['status']): string {
  if (status === 'success') return '✓'
  if (status === 'failed') return '✗'
  if (status === 'partial') return '!'
  return '–'
}

function statusIconClass(status: OptimizeLogInfo['status']): string {
  if (status === 'success') return 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400'
  if (status === 'failed') return 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400'
  if (status === 'partial') return 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400'
  return 'bg-gray-100 text-gray-500 dark:bg-dark-600 dark:text-gray-300'
}

function detailStatusClass(status: 'optimized' | 'skipped' | 'failed'): string {
  if (status === 'optimized') return 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400'
  if (status === 'failed') return 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400'
  return 'bg-gray-200 text-gray-500 dark:bg-dark-600 dark:text-gray-400'
}

function formatTime(value?: string | null): string {
  if (!value) return t('admin.sub2apiProviders.neverRun')
  return new Date(value).toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

function formatDateTime(value?: string | null): string {
  if (!value) return '—'
  return new Date(value).toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

function formatEventTime(value?: string | null): string {
  if (!value) return ''
  return new Date(value).toLocaleTimeString('zh-CN', {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  })
}

function formatDuration(log: OptimizeLogInfo): string {
  if (!log.started_at || !log.finished_at) return ''
  const milliseconds = new Date(log.finished_at).getTime() - new Date(log.started_at).getTime()
  if (Number.isNaN(milliseconds) || milliseconds < 0) return ''
  const seconds = milliseconds / 1000
  if (seconds < 60) return `${seconds.toFixed(1)}s`
  const minutes = Math.floor(seconds / 60)
  return `${minutes}m${Math.round(seconds % 60)}s`
}
</script>

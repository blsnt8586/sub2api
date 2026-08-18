<template>
  <section class="space-y-4" :aria-label="t('admin.sub2apiProviders.health.logs.title')">
    <div class="grid grid-cols-2 gap-2 sm:grid-cols-4">
      <button
        v-for="item in summaryItems"
        :key="item.value"
        type="button"
        class="min-h-16 cursor-pointer rounded-md border px-3 py-2 text-left transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500"
        :class="statusFilter === item.value ? item.activeClass : 'border-gray-200 bg-white hover:border-gray-300 dark:border-dark-600 dark:bg-dark-800 dark:hover:border-dark-500'"
        @click="statusFilter = item.value"
      >
        <span class="block text-[11px] text-gray-500 dark:text-dark-400">{{ item.label }}</span>
        <span class="mt-1 block text-lg font-semibold tabular-nums" :class="item.textClass">{{ item.count }}</span>
      </button>
    </div>

    <div class="flex flex-col gap-2 border-y border-gray-100 py-3 dark:border-dark-700 sm:flex-row sm:items-center sm:justify-between">
      <div class="inline-flex w-full rounded-md bg-gray-100 p-1 dark:bg-dark-900/60 sm:w-auto" role="group" :aria-label="t('admin.sub2apiProviders.health.logs.scope')">
        <button
          v-for="option in scopeOptions"
          :key="option.value"
          type="button"
          class="min-h-9 flex-1 cursor-pointer rounded px-3 text-xs font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 sm:flex-none"
          :class="scopeFilter === option.value ? 'bg-white text-gray-900 shadow-sm dark:bg-dark-700 dark:text-white' : 'text-gray-500 hover:text-gray-800 dark:text-dark-400 dark:hover:text-dark-200'"
          @click="scopeFilter = option.value"
        >
          {{ option.label }}
        </button>
      </div>
      <p class="flex items-center gap-1.5 text-[11px] text-gray-500 dark:text-dark-400">
        <Icon name="shield" size="xs" class="flex-shrink-0" />
        {{ t('admin.sub2apiProviders.health.logs.redactedHint') }}
      </p>
    </div>

    <div v-if="filteredEntries.length" class="divide-y divide-gray-100 border-y border-gray-100 dark:divide-dark-700 dark:border-dark-700">
      <article v-for="entry in visibleEntries" :key="entry.key" class="py-3">
        <div class="flex min-w-0 items-start gap-3">
          <span class="mt-1.5 h-2 w-2 flex-shrink-0 rounded-full" :class="statusDotClass(entry.status)"></span>
          <div class="min-w-0 flex-1">
            <div class="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1">
              <span class="font-medium text-gray-800 dark:text-dark-100">{{ entry.identity }}</span>
              <span class="scope-badge">{{ entry.scopeLabel }}</span>
              <span v-if="entry.platform" class="route-platform">{{ entry.platform }}</span>
              <span v-if="entry.multiplier != null" class="multiplier-badge">×{{ formatMultiplier(entry.multiplier) }}</span>
              <span class="text-xs font-medium" :class="statusTextClass(entry.status)">{{ entry.statusLabel || t(`admin.sub2apiProviders.health.status.${entry.status}`) }}</span>
            </div>

            <div class="mt-1.5 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-gray-500 dark:text-dark-400">
              <span v-if="entry.stage" class="font-medium text-gray-700 dark:text-dark-200">{{ entry.stage }}</span>
              <span v-if="entry.category" class="rounded bg-gray-100 px-1.5 py-0.5 font-mono text-[10px] text-gray-600 dark:bg-dark-700 dark:text-dark-300">{{ entry.category }}</span>
              <span v-if="entry.auditID" class="font-mono text-[10px] text-gray-400 dark:text-dark-500">#{{ entry.auditID }}</span>
              <span v-if="entry.latency != null" class="tabular-nums">{{ entry.latency }} ms</span>
              <span v-if="entry.model" class="max-w-56 truncate font-mono text-[11px]" :title="entry.model">{{ entry.model }}</span>
              <span v-if="entry.trafficCount > 0" class="tabular-nums">{{ t('admin.sub2apiProviders.health.logs.realTraffic', { count: entry.trafficCount, rate: formatPercent(entry.trafficRate) }) }}</span>
              <span v-if="entry.runStats" class="tabular-nums">
                {{ t('admin.sub2apiProviders.health.logs.optimization.runStats', entry.runStats) }}
              </span>
              <time class="ml-auto flex-shrink-0 tabular-nums text-gray-400 dark:text-dark-500" :datetime="entry.checkedAt || undefined">
                {{ entry.checkedAt ? formatDateTime(entry.checkedAt) : '—' }}
              </time>
            </div>

            <p v-if="entry.summary" class="mt-1.5 text-xs text-gray-600 dark:text-dark-300">{{ entry.summary }}</p>

            <div v-if="entry.switchEvents?.length" class="mt-2 border-l-2 border-gray-200 pl-3 dark:border-dark-600">
              <div
                v-for="(event, eventIndex) in entry.switchEvents"
                :key="`${entry.key}-event-${eventIndex}`"
                class="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1 py-1 text-[11px]"
              >
                <Icon :name="event.action === 'rollback' ? 'refresh' : 'swap'" size="xs" class="flex-shrink-0 text-gray-400 dark:text-dark-500" />
                <span class="font-medium text-gray-600 dark:text-dark-300">{{ optimizationActionLabel(event) }}</span>
                <span class="min-w-0 truncate text-gray-500 dark:text-dark-400" :title="optimizationGroupLabel(event.from_group, event.from_group_id, event.from_multiplier)">
                  {{ optimizationGroupLabel(event.from_group, event.from_group_id, event.from_multiplier) }}
                </span>
                <Icon name="arrowRight" size="xs" class="flex-shrink-0 text-gray-300 dark:text-dark-600" />
                <span class="min-w-0 truncate font-medium text-gray-700 dark:text-dark-200" :title="optimizationGroupLabel(event.to_group, event.to_group_id, event.to_multiplier)">
                  {{ optimizationGroupLabel(event.to_group, event.to_group_id, event.to_multiplier) }}
                </span>
                <span class="font-medium" :class="optimizationEventTextClass(event)">{{ optimizationEventLabel(event) }}</span>
                <time class="ml-auto flex-shrink-0 tabular-nums text-gray-400 dark:text-dark-500" :datetime="event.occurred_at">
                  {{ formatDateTime(event.occurred_at) }}
                </time>
              </div>
            </div>
          </div>
        </div>
      </article>

      <button
        v-if="visibleEntries.length < filteredEntries.length"
        type="button"
        class="flex min-h-11 w-full cursor-pointer items-center justify-center gap-1.5 text-xs font-medium text-primary-600 transition-colors hover:bg-gray-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-primary-500 dark:text-primary-400 dark:hover:bg-dark-800"
        @click="visibleLimit += pageSize"
      >
        <Icon name="chevronDown" size="xs" />
        {{ t('admin.sub2apiProviders.health.logs.showMore', { count: filteredEntries.length - visibleEntries.length }) }}
      </button>
    </div>

    <div v-else class="flex min-h-40 flex-col items-center justify-center border-y border-dashed border-gray-200 text-center dark:border-dark-600">
      <Icon name="clock" size="lg" class="text-gray-300 dark:text-dark-600" />
      <p class="mt-2 text-sm font-medium text-gray-600 dark:text-dark-300">{{ t('admin.sub2apiProviders.health.logs.empty') }}</p>
      <p class="mt-1 text-xs text-gray-400 dark:text-dark-500">{{ t('admin.sub2apiProviders.health.logs.emptyHint') }}</p>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type {
  OptimizeGroupSwitchEvent,
  OptimizeLogDetail,
  OptimizeLogInfo,
  ProviderAccountProbeStatus,
  ProviderHealthStatus,
  Sub2APIProviderHealth,
  Sub2APIProviderProbeTargetHealth,
} from '@/api/admin/sub2apiProviders'
import { formatDateTime } from '@/utils/format'
import Icon from '@/components/icons/Icon.vue'

type LogStatus = ProviderHealthStatus | ProviderAccountProbeStatus
type LogScope = 'all' | 'control' | 'account' | 'optimization'
type StatusFilter = 'all' | 'healthy' | 'degraded' | 'unhealthy'

interface DiagnosticEntry {
  key: string
  scope: Exclude<LogScope, 'all'>
  scopeLabel: string
  identity: string
  statusLabel?: string | null
  platform?: string | null
  multiplier?: number | null
  status: LogStatus
  stage?: string | null
  category?: string | null
  latency?: number | null
  model?: string | null
  trafficCount: number
  trafficRate?: number | null
  checkedAt?: string | null
  summary?: string | null
  auditID?: number | null
  runStats?: { optimized: number; skipped: number; failed: number } | null
  switchEvents?: OptimizeGroupSwitchEvent[] | null
}

const props = defineProps<{
  controlHistory: Sub2APIProviderHealth[]
  routes: Sub2APIProviderProbeTargetHealth[]
  routeHistory: Record<number, Sub2APIProviderProbeTargetHealth[]>
  optimizationLogs: OptimizeLogInfo[]
}>()

const { t } = useI18n()
const scopeFilter = ref<LogScope>('all')
const statusFilter = ref<StatusFilter>('all')
const pageSize = 100
const visibleLimit = ref(pageSize)

const scopeOptions = computed(() => [
  { value: 'all' as const, label: t('admin.sub2apiProviders.health.logs.scopes.all') },
  { value: 'control' as const, label: t('admin.sub2apiProviders.health.logs.scopes.control') },
  { value: 'account' as const, label: t('admin.sub2apiProviders.health.logs.scopes.account') },
  { value: 'optimization' as const, label: t('admin.sub2apiProviders.health.logs.scopes.optimization') },
])

const controlStage = (item: Sub2APIProviderHealth) => {
  const details = item.details ?? {}
  if (typeof details.login_error === 'string') return t('admin.sub2apiProviders.health.logs.stages.login')
  if (typeof details.health_error === 'string') return '/health'
  if (typeof details.keys_error === 'string') return 'Keys API'
  if (typeof details.groups_error === 'string') return 'Groups API'
  return item.control_status === 'healthy' ? t('admin.sub2apiProviders.health.logs.stages.completed') : t('admin.sub2apiProviders.health.logs.stages.connection')
}

const safeSummary = (status: LogStatus, category?: string | null) => {
  if (status === 'healthy') return t('admin.sub2apiProviders.health.logs.summaries.healthy')
  if (status === 'degraded') return t('admin.sub2apiProviders.health.logs.summaries.degraded')
  if (status === 'unhealthy') return category
    ? t('admin.sub2apiProviders.health.logs.summaries.failedWithCategory', { category })
    : t('admin.sub2apiProviders.health.logs.summaries.failed')
  if (status === 'disabled') return t('admin.sub2apiProviders.health.logs.summaries.disabled')
  return t('admin.sub2apiProviders.health.logs.summaries.unknown')
}

const hasFailedSwitchEvent = (detail?: OptimizeLogDetail) => (detail?.switch_events ?? []).some(event => (
  event.status === 'failed' || event.test_status === 'failed'
))

const optimizationStatus = (log: OptimizeLogInfo, detail?: OptimizeLogDetail): ProviderHealthStatus => {
  if (detail?.status === 'failed' || log.status === 'failed') return 'unhealthy'
  // A run may reject and roll back an early candidate before a later one passes.
  // The final account result owns the summary status; the event trail below still
  // exposes every failed candidate and rollback.
  if (detail?.status === 'optimized') return 'healthy'
  if (hasFailedSwitchEvent(detail) || log.status === 'partial' || detail?.execution_disposition === 'deferred') return 'degraded'
  return 'healthy'
}

const optimizationStatusLabel = (log: OptimizeLogInfo, detail?: OptimizeLogDetail) => {
  if (detail?.status === 'failed' || log.status === 'failed') return t('admin.sub2apiProviders.health.logs.optimization.outcomes.failed')
  if (detail?.status === 'optimized') return t('admin.sub2apiProviders.health.logs.optimization.outcomes.optimized')
  if ((detail?.switch_events ?? []).some(event => event.action === 'rollback')) return t('admin.sub2apiProviders.health.logs.optimization.outcomes.rolledBack')
  if (detail?.status === 'skipped' || log.status === 'skipped') return t('admin.sub2apiProviders.health.logs.optimization.outcomes.skipped')
  if (log.status === 'partial') return t('admin.sub2apiProviders.health.logs.optimization.outcomes.partial')
  return t('admin.sub2apiProviders.health.logs.optimization.outcomes.completed')
}

const optimizationTriggerLabel = (trigger: OptimizeLogInfo['trigger']) => (
  t(`admin.sub2apiProviders.health.logs.optimization.triggers.${trigger}`)
)

const optimizationSummary = (log: OptimizeLogInfo, detail?: OptimizeLogDetail) => {
  const events = detail?.switch_events ?? []
  if (detail?.status === 'optimized') return t('admin.sub2apiProviders.health.logs.optimization.summaries.optimized')
  const rollback = events.find(event => event.action === 'rollback')
  if (rollback?.status === 'failed') return t('admin.sub2apiProviders.health.logs.optimization.summaries.rollbackFailed')
  if (rollback) return t('admin.sub2apiProviders.health.logs.optimization.summaries.rolledBack')
  if (events.some(event => event.action === 'switch' && event.status === 'failed')) return t('admin.sub2apiProviders.health.logs.optimization.summaries.switchFailed')
  if (detail?.execution_disposition === 'deferred') return t('admin.sub2apiProviders.health.logs.optimization.summaries.deferred')
  if (detail?.execution_disposition === 'coalesced') return t('admin.sub2apiProviders.health.logs.optimization.summaries.coalesced')
  if (detail?.status === 'failed' || log.status === 'failed') return t('admin.sub2apiProviders.health.logs.optimization.summaries.failed')
  return t('admin.sub2apiProviders.health.logs.optimization.summaries.unchanged')
}

const entries = computed<DiagnosticEntry[]>(() => {
  const controlEntries = props.controlHistory.map((item, index): DiagnosticEntry => ({
    key: `control-${item.last_checked_at ?? index}`,
    scope: 'control',
    scopeLabel: t('admin.sub2apiProviders.health.logs.scopes.control'),
    identity: t('admin.sub2apiProviders.health.controlConnection'),
    status: item.control_status,
    stage: controlStage(item),
    category: item.error_category,
    latency: item.health_latency_ms,
    trafficCount: item.traffic_request_count,
    trafficRate: item.traffic_success_rate,
    checkedAt: item.last_checked_at,
    summary: safeSummary(item.control_status, item.error_category),
  }))

  const routeByID = new Map(props.routes.map(route => [route.id, route]))
  const accountEntries = Object.entries(props.routeHistory).flatMap(([targetID, history]) => {
    const route = routeByID.get(Number(targetID))
    return history.map((item, index): DiagnosticEntry => ({
      key: `account-${targetID}-${item.last_checked_at ?? index}`,
      scope: 'account',
      scopeLabel: t('admin.sub2apiProviders.health.logs.scopes.account'),
      identity: route?.account_name || item.account_name || `#${item.account_id}`,
      platform: route?.platform || item.platform,
      multiplier: route?.remote_group_multiplier,
      status: item.status,
      stage: route?.remote_group_name || item.remote_group_name,
      category: item.error_category,
      latency: item.latency_ms,
      model: item.test_model,
      trafficCount: item.traffic_request_count,
      trafficRate: item.traffic_success_rate,
      checkedAt: item.last_checked_at,
      summary: safeSummary(item.status, item.error_category),
    }))
  })

  const optimizationEntries = props.optimizationLogs.flatMap(log => {
    const details: Array<OptimizeLogDetail | undefined> = log.detail?.length ? log.detail : [undefined]
    return details.map((detail, index): DiagnosticEntry => ({
      key: `optimization-${log.id}-${detail?.account_id ?? index}`,
      scope: 'optimization',
      scopeLabel: t('admin.sub2apiProviders.health.logs.scopes.optimization'),
      identity: detail?.account_name || t('admin.sub2apiProviders.health.logs.optimization.runIdentity', { id: log.id }),
      status: optimizationStatus(log, detail),
      statusLabel: optimizationStatusLabel(log, detail),
      multiplier: detail?.new_multiplier,
      stage: optimizationTriggerLabel(log.trigger),
      category: detail?.probe_error_category,
      trafficCount: 0,
      checkedAt: log.finished_at || log.created_at,
      summary: optimizationSummary(log, detail),
      auditID: log.id,
      runStats: { optimized: log.optimized, skipped: log.skipped, failed: log.failed },
      switchEvents: detail?.switch_events,
    }))
  })

  return [...controlEntries, ...accountEntries, ...optimizationEntries].sort((a, b) => (b.checkedAt || '').localeCompare(a.checkedAt || ''))
})

const summaryItems = computed(() => {
  const count = (status?: StatusFilter) => status && status !== 'all'
    ? entries.value.filter(entry => entry.status === status).length
    : entries.value.length
  return [
    { value: 'all' as const, label: t('admin.sub2apiProviders.health.logs.summary.all'), count: count(), textClass: 'text-gray-900 dark:text-white', activeClass: 'border-gray-400 bg-gray-50 dark:border-dark-400 dark:bg-dark-700' },
    { value: 'healthy' as const, label: t('admin.sub2apiProviders.health.logs.summary.healthy'), count: count('healthy'), textClass: 'text-green-600 dark:text-green-400', activeClass: 'border-green-400 bg-green-50 dark:border-green-700 dark:bg-green-900/20' },
    { value: 'degraded' as const, label: t('admin.sub2apiProviders.health.logs.summary.degraded'), count: count('degraded'), textClass: 'text-amber-600 dark:text-amber-400', activeClass: 'border-amber-400 bg-amber-50 dark:border-amber-700 dark:bg-amber-900/20' },
    { value: 'unhealthy' as const, label: t('admin.sub2apiProviders.health.logs.summary.unhealthy'), count: count('unhealthy'), textClass: 'text-red-600 dark:text-red-400', activeClass: 'border-red-400 bg-red-50 dark:border-red-700 dark:bg-red-900/20' },
  ]
})

const filteredEntries = computed(() => entries.value.filter(entry => {
  if (scopeFilter.value !== 'all' && entry.scope !== scopeFilter.value) return false
  return statusFilter.value === 'all' || entry.status === statusFilter.value
}))

const visibleEntries = computed(() => filteredEntries.value.slice(0, visibleLimit.value))

watch([scopeFilter, statusFilter], () => {
  visibleLimit.value = pageSize
})

const statusDotClass = (status: LogStatus) => ({
  healthy: 'bg-green-500',
  degraded: 'bg-amber-400',
  unhealthy: 'bg-red-500',
  unknown: 'border border-gray-300 bg-transparent dark:border-dark-500',
  disabled: 'bg-gray-300 dark:bg-dark-500',
}[status])

const statusTextClass = (status: LogStatus) => ({
  healthy: 'text-green-600 dark:text-green-400',
  degraded: 'text-amber-600 dark:text-amber-400',
  unhealthy: 'text-red-600 dark:text-red-400',
  unknown: 'text-gray-500 dark:text-dark-400',
  disabled: 'text-gray-400 dark:text-dark-500',
}[status])

const formatPercent = (value?: number | null) => value == null ? '—' : `${value.toFixed(value % 1 === 0 ? 0 : 1)}%`
const formatMultiplier = (value: number) => Number.isInteger(value) ? value.toFixed(0) : String(Number(value.toFixed(2)))

const optimizationGroupLabel = (name?: string, id?: number, multiplier?: number) => {
  const identity = name || (id ? `#${id}` : t('admin.sub2apiProviders.health.logs.optimization.unknownGroup'))
  return multiplier == null ? identity : `${identity} ×${formatMultiplier(multiplier)}`
}

const optimizationActionLabel = (event: OptimizeGroupSwitchEvent) => t(
  `admin.sub2apiProviders.health.logs.optimization.actions.${event.action}`
)

const optimizationEventLabel = (event: OptimizeGroupSwitchEvent) => {
  if (event.status === 'failed') return t('admin.sub2apiProviders.health.logs.optimization.events.operationFailed')
  if (event.action === 'rollback') return t('admin.sub2apiProviders.health.logs.optimization.events.rollbackSucceeded')
  if (event.test_status === 'failed') return t('admin.sub2apiProviders.health.logs.optimization.events.testFailed')
  if (event.test_status === 'passed') return t('admin.sub2apiProviders.health.logs.optimization.events.testPassed')
  return t('admin.sub2apiProviders.health.logs.optimization.events.switched')
}

const optimizationEventTextClass = (event: OptimizeGroupSwitchEvent) => {
  if (event.status === 'failed' || event.test_status === 'failed') return 'text-red-600 dark:text-red-400'
  if (event.action === 'rollback') return 'text-amber-600 dark:text-amber-400'
  return 'text-green-600 dark:text-green-400'
}
</script>

<style scoped>
.scope-badge,
.route-platform,
.multiplier-badge {
  border: 1px solid rgb(226 232 240);
  border-radius: 3px;
  color: rgb(100 116 139);
  font-size: 9px;
  line-height: 0.9rem;
  padding: 0 0.25rem;
}

.multiplier-badge {
  border-color: rgb(167 243 208);
  background: rgb(236 253 245);
  color: rgb(4 120 87);
  font-weight: 600;
}

:global(.dark) .scope-badge,
:global(.dark) .route-platform {
  border-color: rgb(71 85 105);
  color: rgb(148 163 184);
}

:global(.dark) .multiplier-badge {
  border-color: rgb(6 78 59);
  background: rgb(6 78 59 / 0.2);
  color: rgb(110 231 183);
}
</style>

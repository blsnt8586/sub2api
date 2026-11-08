<template>
  <article
    class="provider-card flex h-full min-h-[360px] flex-col overflow-hidden rounded-lg border border-gray-200 bg-white shadow-sm transition-[box-shadow,border-color] duration-200 ease-out hover:border-gray-300 hover:shadow-md dark:border-dark-700 dark:bg-dark-800 dark:hover:border-dark-600"
    :class="provider.status === 'inactive' ? 'opacity-90' : ''"
    :style="{ '--provider-entry-delay': `${Math.min(animationIndex ?? 0, 7) * 45}ms` }"
  >
    <div class="flex flex-1 flex-col px-4 py-3">
      <header class="flex min-w-0 items-center justify-between gap-3">
        <div class="flex min-w-0 flex-1 items-center gap-x-1.5 overflow-hidden">
          <h2 class="flex h-8 min-w-0 max-w-[18rem] flex-1 items-center truncate text-sm font-semibold text-gray-900 dark:text-white" :title="provider.name">
            <a
              v-if="providerURL"
              :href="providerURL"
              target="_blank"
              rel="noopener noreferrer"
              data-test="provider-upstream-link"
              class="inline-flex h-8 min-w-0 max-w-full flex-shrink items-center gap-1 text-sm font-semibold text-gray-900 transition-colors hover:text-blue-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 dark:text-white dark:hover:text-blue-300"
              :title="t('admin.sub2apiProviders.openUpstream')"
            >
              <span class="truncate">{{ provider.name }}</span>
              <Icon name="externalLink" size="xs" class="flex-shrink-0" />
            </a>
            <span v-else class="min-w-0 truncate">{{ provider.name }}</span>
          </h2>
          <span :class="['badge inline-flex h-8 flex-shrink-0 items-center px-2 text-[10px]', provider.status === 'active' ? 'badge-success' : 'badge-gray']">
            {{ t(`admin.sub2apiProviders.statusLabels.${provider.status}`) }}
          </span>
          <button
            v-if="provider.status !== 'inactive'"
            type="button"
            class="inline-flex h-8 min-w-0 max-w-[7.5rem] flex-shrink-0 items-center gap-1 rounded-md border border-gray-200 px-2 text-[10px] font-medium tabular-nums transition-colors hover:bg-gray-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 dark:border-dark-600 dark:hover:bg-dark-700"
            data-test="provider-control-status"
            :class="availabilityTextClass"
            :title="t('admin.sub2apiProviders.health.activeProbe')"
            :aria-label="`${t('admin.sub2apiProviders.health.activeProbe')}: ${t(`admin.sub2apiProviders.health.availabilityStatus.${availabilityStatus}`)}`"
            @click="emit('view-health')"
          >
            <span class="h-1.5 w-1.5 flex-shrink-0 rounded-full" :class="availabilityDotClass"></span>
            <span class="truncate">{{ t(`admin.sub2apiProviders.health.availabilityStatus.${availabilityStatus}`) }}</span>
          </button>
          <span
            class="provider-type-tag inline-flex h-8 flex-shrink-0 items-center rounded-md border px-2 text-[10px] font-semibold"
            :class="providerTypeTagClass"
            data-test="provider-type-tag"
          >{{ providerTypeLabel }}</span>
        </div>

        <button
          type="button"
          class="inline-flex h-8 w-8 flex-shrink-0 cursor-pointer items-center justify-center rounded-md text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 focus-visible:ring-offset-2 dark:text-dark-300 dark:hover:bg-dark-700 dark:hover:text-white dark:focus-visible:ring-offset-dark-800"
          :title="t('common.more')"
          :aria-label="`${provider.name} ${t('common.more')}`"
          @click="emit('more', $event)"
        >
          <Icon name="more" size="md" />
        </button>
      </header>

      <p v-if="provider.notes" class="mt-2 truncate text-xs leading-5 text-gray-500 dark:text-dark-400" :title="provider.notes">
        {{ provider.notes }}
      </p>

      <div
        v-if="provider.status === 'inactive'"
        class="mt-3 flex items-start gap-2 rounded-md border border-gray-200 bg-gray-50 px-3 py-2.5 text-xs text-gray-600 dark:border-dark-600 dark:bg-dark-900/40 dark:text-dark-300"
        data-test="provider-probe-paused"
      >
        <Icon name="shield" size="sm" class="mt-0.5 flex-shrink-0 text-gray-400" />
        <div class="min-w-0">
          <p class="font-medium text-gray-700 dark:text-dark-200">{{ t('admin.sub2apiProviders.health.probePaused') }}</p>
          <p class="mt-0.5 text-gray-500 dark:text-dark-400">{{ t('admin.sub2apiProviders.health.probePausedHint') }}</p>
        </div>
      </div>

      <button
        type="button"
        data-test="provider-remote-overview"
        class="mt-3 min-h-14 w-full cursor-pointer rounded-md border border-gray-100 px-3 py-2 text-left transition-colors hover:border-blue-200 hover:bg-blue-50/60 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 disabled:cursor-wait disabled:opacity-70 dark:border-dark-700 dark:hover:border-blue-800 dark:hover:bg-blue-900/10"
        :disabled="remoteOverviewLoading"
        :aria-label="t('admin.sub2apiProviders.remoteOverview.open')"
        @click="emit('view-remote-overview')"
      >
        <span v-if="remoteOverviewLoading && !remoteOverviewAvailable" class="flex min-h-9 items-center justify-center gap-2 text-xs font-medium text-blue-600 dark:text-blue-400">
          <Icon name="refresh" size="sm" class="animate-spin" />
          {{ t('admin.sub2apiProviders.remoteOverview.loading') }}
        </span>
        <span v-else-if="remoteOverviewAvailable" class="block min-w-0" data-test="provider-remote-metrics">
          <span class="grid grid-cols-4 gap-x-3 gap-y-2">
            <span v-for="metric in remoteMetrics" :key="metric.key" class="min-w-0 text-center" :data-test="`remote-metric-${metric.key}`">
              <span class="flex min-w-0 items-center justify-center gap-1 text-[10px] font-medium text-gray-500 dark:text-dark-400">
                <Icon :name="metric.icon" size="xs" class="flex-shrink-0" :class="metric.iconClass" />
                <span class="truncate">{{ t(`admin.sub2apiProviders.remoteOverview.${metric.label}`) }}</span>
              </span>
              <span class="mt-0.5 block truncate text-center text-sm font-semibold tabular-nums" :class="metric.valueClass" :title="metric.title">
                {{ metric.value }}
              </span>
              <span
                v-if="metric.detail"
                class="mt-0.5 block truncate text-center text-[10px] font-medium tabular-nums text-gray-500 dark:text-dark-400"
                :title="metric.detail"
                data-test="remote-metric-detail"
              >
                {{ metric.detail }}
              </span>
            </span>
          </span>
        </span>
        <span v-else-if="remoteOverviewErrorMessage" class="flex min-h-9 items-center justify-between gap-3">
          <span class="flex min-w-0 items-center gap-2 text-xs font-medium text-red-600 dark:text-red-400">
            <Icon name="exclamationCircle" size="sm" class="flex-shrink-0" />
            <span class="truncate">{{ t('admin.sub2apiProviders.remoteOverview.loadFailed') }}</span>
          </span>
          <span class="flex-shrink-0 text-xs text-gray-500 dark:text-dark-300">{{ t('admin.sub2apiProviders.remoteOverview.retry') }}</span>
        </span>
        <span v-else class="flex min-h-9 items-center justify-between gap-3">
          <span class="flex min-w-0 items-center gap-2">
            <Icon name="creditCard" size="sm" class="flex-shrink-0 text-blue-500" />
            <span class="min-w-0">
              <span class="block truncate text-xs font-medium text-gray-700 dark:text-dark-200">{{ t('admin.sub2apiProviders.remoteOverview.title') }}</span>
              <span class="mt-0.5 block truncate text-xs text-gray-500 dark:text-dark-400">{{ t('admin.sub2apiProviders.remoteOverview.notCollected') }}</span>
            </span>
          </span>
          <Icon name="chevronRight" size="sm" class="flex-shrink-0 text-gray-400" />
        </span>
      </button>

      <section class="mt-3 min-w-0 flex-1" :aria-label="t('admin.sub2apiProviders.health.routes.accountProbes')">
        <div class="flex items-center justify-between gap-3">
          <h3 class="text-[11px] font-semibold uppercase text-gray-500 dark:text-dark-400">
            {{ t('admin.sub2apiProviders.health.routes.accountProbes') }}
          </h3>
          <span class="flex items-center gap-2 text-[11px] tabular-nums text-gray-400 dark:text-dark-400">
            <span v-if="abnormalRouteCount" class="font-medium text-red-600 dark:text-red-400">
              {{ t('admin.sub2apiProviders.health.routes.abnormalSummary', { count: abnormalRouteCount }) }}
            </span>
            <span>{{ t('admin.sub2apiProviders.health.routes.enabledSummary', { enabled: enabledRouteCount, total: routes.length }) }}</span>
          </span>
        </div>

        <div v-if="routes.length" data-test="provider-route-list" class="mt-1 divide-y divide-gray-100 dark:divide-dark-700">
          <button
            v-for="route in sortedRoutes"
            :key="route.id"
            type="button"
            class="group w-full min-w-0 cursor-pointer py-2.5 text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 focus-visible:ring-inset"
            :data-test="`provider-route-probe-${route.id}`"
            @click="emit('view-health')"
          >
            <div class="flex min-w-0 items-center gap-2.5">
              <span class="h-1.5 w-1.5 flex-shrink-0 rounded-full" :class="routeStatusDotClass(route.status)"></span>
              <span class="min-w-0 flex-1 truncate text-[13px] font-semibold text-gray-800 group-hover:text-gray-950 dark:text-dark-100 dark:group-hover:text-white" :title="route.account_name">
                {{ route.account_name }}
              </span>
              <span
                v-if="route.account_status && route.account_status !== 'active'"
                class="flex-shrink-0 rounded border px-1.5 py-0.5 text-[10px] font-medium"
                :class="accountStatusClass(route.account_status)"
              >{{ accountStatusLabel(route.account_status) }}</span>
              <span
                v-if="route.remote_group_multiplier != null"
                class="multiplier-badge flex-shrink-0 border tabular-nums"
                :class="multiplierClass(route)"
                :title="multiplierTitle(route)"
                :aria-label="multiplierTitle(route)"
                :data-test="`route-multiplier-${route.id}`"
              >
                <Icon v-if="multiplierOutOfRange(route)" name="exclamationCircle" size="xs" class="flex-shrink-0" />
                ×{{ formatMultiplier(route.remote_group_multiplier) }}
              </span>
              <span
                :class="['route-platform flex-shrink-0 border', platformTagClass(route.platform)]"
                :title="route.platform"
              >{{ route.platform }}</span>
              <span
                v-if="route.sub2api_optimize_enabled"
                class="route-optimize-badge flex-shrink-0 border border-violet-200 bg-violet-50 text-violet-700 dark:border-violet-800 dark:bg-violet-900/20 dark:text-violet-300"
                :title="t('admin.sub2apiProviders.joinScheduleOn')"
                :aria-label="t('admin.sub2apiProviders.joinScheduleOn')"
                role="img"
                :data-test="`route-optimize-${route.id}`"
              >
                <Icon name="bolt" size="xs" aria-hidden="true" />
              </span>
              <span class="flex-shrink-0 text-center text-[11px] font-medium tabular-nums" :class="routeStatusTextClass(route.status)">
                {{ route.latency_ms != null ? `${route.latency_ms} ms` : t(`admin.sub2apiProviders.health.status.${route.status}`) }}
              </span>
            </div>
            <div
              v-if="route.average_duration_ms != null || route.cache_hit_rate_available"
              class="mt-1 flex items-center justify-center gap-3 pl-3.5 text-center text-[10px] tabular-nums text-gray-500 dark:text-dark-400"
              data-test="route-traffic-metrics"
            >
              <span v-if="route.average_duration_ms != null">
                {{ t('admin.sub2apiProviders.remoteOverview.averageResponse') }} {{ formatDuration(route.average_duration_ms) }}
              </span>
              <span v-if="route.cache_hit_rate_available && route.cache_hit_rate != null">
                {{ t('admin.sub2apiProviders.remoteOverview.cacheHitRate') }} {{ formatPercent(route.cache_hit_rate) }}
              </span>
            </div>
            <div class="mt-1.5 min-w-0 pl-3.5">
              <Sub2APIProviderRouteTimeline :route="route" compact :interactive="false" />
            </div>
            <Sub2APIProviderAnswerTimeline class="mt-1.5 pl-3.5" :route="route" />
          </button>

        </div>

        <button
          v-else
          type="button"
          class="mt-2 flex min-h-16 w-full cursor-pointer items-center justify-center border-y border-dashed border-gray-200 text-xs text-gray-400 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 focus-visible:ring-inset dark:border-dark-600 dark:text-dark-400"
          @click="emit('view-accounts')"
        >
          {{ t('admin.sub2apiProviders.health.routes.empty') }}
        </button>
      </section>

      <div class="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1 border-t border-gray-100 pt-2 text-[11px] text-gray-500 dark:border-dark-700 dark:text-dark-400">
        <span v-if="provider.proxy_id" class="inline-flex items-center gap-1.5" :title="t('admin.sub2apiProviders.form.proxyShort')">
          <Icon name="globe" size="xs" class="flex-shrink-0 text-cyan-600 dark:text-cyan-400" />
          <span>{{ t('admin.sub2apiProviders.form.proxyShort') }}</span>
        </span>
        <span class="inline-flex items-center gap-1.5" :title="pathStatusTitle">
          <span class="h-1.5 w-1.5 rounded-full" :class="provider.api_path_keys ? 'bg-green-500' : 'bg-amber-400'"></span>
          {{ t('admin.sub2apiProviders.apiPathLabel') }}：{{ provider.api_path_keys ? t('admin.sub2apiProviders.apiPathStatus.ready') : t('admin.sub2apiProviders.apiPathStatus.notDetected') }}
        </span>
        <span class="inline-flex min-w-0 items-center gap-1.5" :title="authStatusTitle">
          <Icon :name="provider.auth_mode !== 'password' ? 'key' : 'lock'" size="xs" class="flex-shrink-0" :class="authStatusClass" />
          <span class="truncate" :class="authStatusClass">{{ authStatusLabel }}</span>
        </span>
      </div>
    </div>

    <footer class="grid grid-cols-3 gap-1.5 border-t border-gray-100 bg-gray-50/70 px-3 py-2 dark:border-dark-700 dark:bg-dark-900/30">
      <button
        type="button"
        data-test="provider-view-accounts"
        class="card-action border-gray-200 bg-white text-gray-700 dark:border-dark-600 dark:bg-dark-800 dark:text-dark-200 hover:border-indigo-200 hover:bg-indigo-50 hover:text-indigo-700 focus-visible:ring-indigo-500 dark:hover:border-indigo-800 dark:hover:bg-indigo-900/20 dark:hover:text-indigo-300"
        @click="emit('view-accounts')"
      >
        <Icon name="users" size="sm" class="flex-shrink-0" />
        <span class="truncate">{{ t('admin.sub2apiProviders.accountsButton') }}</span>
      </button>

      <button
        type="button"
        data-test="provider-view-logs"
        class="card-action border-gray-200 bg-white text-gray-700 dark:border-dark-600 dark:bg-dark-800 dark:text-dark-200 hover:border-cyan-200 hover:bg-cyan-50 hover:text-cyan-700 focus-visible:ring-cyan-500 dark:hover:border-cyan-800 dark:hover:bg-cyan-900/20 dark:hover:text-cyan-300"
        @click="emit('view-logs')"
      >
        <Icon name="clock" size="sm" class="flex-shrink-0" />
        <span class="truncate">{{ t('admin.sub2apiProviders.health.logsButton') }}</span>
      </button>

      <button
        type="button"
        data-test="provider-manage-probes"
        class="card-action border-gray-200 bg-white text-gray-700 dark:border-dark-600 dark:bg-dark-800 dark:text-dark-200 hover:border-blue-200 hover:bg-blue-50 hover:text-blue-700 focus-visible:ring-blue-500 dark:hover:border-blue-800 dark:hover:bg-blue-900/20 dark:hover:text-blue-300"
        @click="emit('view-health')"
      >
        <Icon name="shield" size="sm" class="flex-shrink-0" />
        <span class="truncate">{{ t('admin.sub2apiProviders.health.routes.manageShort') }}</span>
      </button>
    </footer>
  </article>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type {
  ProviderHealthStatus,
  Sub2APIProvider,
  Sub2APIProviderHealthOverview,
  Sub2APIProviderProbeTargetHealth,
  Sub2APIProviderRemoteOverview,
} from '@/api/admin/sub2apiProviders'
import { getMultiplierRangeState } from '@/utils/sub2apiValidation'
import { platformTagClass } from '@/utils/platformColors'
import Icon from '@/components/icons/Icon.vue'
import Sub2APIProviderRouteTimeline from './Sub2APIProviderRouteTimeline.vue'
import Sub2APIProviderAnswerTimeline from './Sub2APIProviderAnswerTimeline.vue'

const props = defineProps<{
  provider: Sub2APIProvider
  overview?: Sub2APIProviderHealthOverview | null
  remoteOverview?: Sub2APIProviderRemoteOverview | null
  remoteOverviewLoading?: boolean
  remoteOverviewError?: string | null
  animationIndex?: number
  nowTick?: number
}>()

const emit = defineEmits<{
  (e: 'view-accounts'): void
  (e: 'view-logs'): void
  (e: 'view-health'): void
  (e: 'view-remote-overview'): void
  (e: 'more', event: MouseEvent): void
}>()

const { t } = useI18n()

const providerTypeLabel = computed(() => props.provider.provider_type === 'sub2api' ? 'Sub2API' : props.provider.provider_type === 'newapi' ? 'New API' : props.provider.provider_type)
const providerTypeTagClass = computed(() => props.provider.provider_type === 'newapi'
  ? 'border-teal-200 bg-teal-50 text-teal-700 dark:border-teal-800 dark:bg-teal-900/25 dark:text-teal-300'
  : 'border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-800 dark:bg-blue-900/25 dark:text-blue-300')
const authStatusLabel = computed(() => {
  if (props.provider.auth_mode === 'browser_session') return t(props.provider.last_auth_error ? 'admin.sub2apiProviders.auth.reimportRequired' : 'admin.sub2apiProviders.form.newapiBrowserSession')
  if (props.provider.auth_mode === 'access_token') return t(props.provider.last_auth_error ? 'admin.sub2apiProviders.auth.reimportRequired' : 'admin.sub2apiProviders.form.authManagementToken')
  if (props.provider.auth_mode !== 'token_pair') return t('admin.sub2apiProviders.auth.password')
  if (props.provider.last_auth_error) return t('admin.sub2apiProviders.auth.reimportRequired')
  if (props.provider.has_access_token && props.provider.has_refresh_token) return t('admin.sub2apiProviders.auth.tokenReady')
  return t('admin.sub2apiProviders.auth.tokenIncomplete')
})
const authStatusClass = computed(() => {
  if (props.provider.auth_mode === 'access_token' || props.provider.auth_mode === 'browser_session') return props.provider.last_auth_error ? 'text-red-600 dark:text-red-400' : props.provider.has_access_token ? 'text-green-600 dark:text-green-400' : 'text-amber-600 dark:text-amber-400'
  if (props.provider.auth_mode !== 'token_pair') return 'text-gray-500 dark:text-dark-400'
  if (props.provider.last_auth_error) return 'text-red-600 dark:text-red-400'
  return props.provider.has_access_token && props.provider.has_refresh_token
    ? 'text-green-600 dark:text-green-400'
    : 'text-amber-600 dark:text-amber-400'
})
const authStatusTitle = computed(() => props.provider.auth_mode === 'browser_session'
  ? t('admin.sub2apiProviders.form.newapiBrowserTokenHint')
  : props.provider.auth_mode === 'access_token'
  ? t('admin.sub2apiProviders.form.newapiTokenHint')
  : props.provider.auth_mode === 'token_pair'
  ? t('admin.sub2apiProviders.auth.tokenTitle')
  : props.provider.provider_type === 'newapi' ? t('admin.sub2apiProviders.form.newapiPasswordDesc') : t('admin.sub2apiProviders.auth.passwordTitle'))
const providerURL = computed(() => {
  try {
    const url = new URL(props.provider.base_url)
    return url.protocol === 'http:' || url.protocol === 'https:' ? url.href : null
  } catch {
    return null
  }
})

const remoteOverviewAvailable = computed(() => props.remoteOverview?.available === true || props.remoteOverview?.profit != null || props.remoteOverview?.local_usage != null)
const remoteOverviewErrorMessage = computed(() => props.remoteOverviewError || props.remoteOverview?.last_error || null)
const remoteUsage = computed(() => {
  const remote = props.remoteOverview?.usage
  if (remote?.dashboard_available === true) return remote
  return props.remoteOverview?.local_usage ?? remote ?? null
})
const formatMoney = (value: number | null | undefined) => {
  if (value == null || !Number.isFinite(value)) return '-'
  return `$${new Intl.NumberFormat(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 }).format(value)}`
}
const formatCount = (value: number | null | undefined) => {
  if (value == null || !Number.isFinite(value)) return '-'
  return new Intl.NumberFormat(undefined, { maximumFractionDigits: 0 }).format(value)
}
const formatTokens = (value: number | null | undefined) => {
  if (value == null || !Number.isFinite(value)) return '-'
  const absolute = Math.abs(value)
  if (absolute >= 1_000_000) return `${(value / 1_000_000).toFixed(1)}M`
  if (absolute >= 1_000) return `${(value / 1_000).toFixed(1)}K`
  return formatCount(value)
}
const formatDuration = (value: number | null | undefined) => {
  if (value == null || !Number.isFinite(value) || value <= 0) return '-'
  return value >= 1000 ? `${(value / 1000).toFixed(2)}s` : `${Math.round(value)}ms`
}
const formatPercent = (value: number | null | undefined) => {
  if (value == null || !Number.isFinite(value)) return '-'
  return `${(value * 100).toFixed(1)}%`
}
const remoteMetricValue = (value: number | null | undefined, formatter: (value: number | null | undefined) => string) =>
  value == null ? '-' : formatter(value)
type RemoteMetricIcon = 'creditCard' | 'dollar' | 'chart' | 'database' | 'clock'
type RemoteMetric = {
  key: string
  label: string
  icon: RemoteMetricIcon
  iconClass: string
  value: string
  valueClass: string
  title?: string
  detail?: string
}
const remoteMetrics = computed<RemoteMetric[]>(() => {
  const usage = remoteUsage.value
  const metrics: RemoteMetric[] = [
    { key: 'today-consumed', label: 'todayConsumed', icon: 'dollar', iconClass: 'text-purple-500', value: remoteMetricValue(usage?.today_actual_cost, formatMoney), valueClass: 'text-purple-600 dark:text-purple-400' },
    { key: 'today-tokens', label: 'todayTokens', icon: 'database', iconClass: 'text-amber-500', value: remoteMetricValue(usage?.today_tokens, formatTokens), valueClass: 'text-gray-900 dark:text-white' },
    { key: 'total-recharged', label: 'totalRecharged', icon: 'dollar', iconClass: 'text-blue-500', value: props.remoteOverview?.usage?.funding_summary_available ? formatMoney(props.remoteOverview.usage.total_recharged) : '-', valueClass: 'text-blue-600 dark:text-blue-400' },
    { key: 'total-tokens', label: 'totalTokens', icon: 'database', iconClass: 'text-indigo-500', value: remoteMetricValue(usage?.total_tokens, formatTokens), valueClass: 'text-gray-900 dark:text-white' },
  ]
  const profit = props.remoteOverview?.profit
  metrics.push(
    { key: 'balance', label: 'balance', icon: 'creditCard', iconClass: 'text-emerald-500', value: props.remoteOverview?.available ? formatMoney(props.remoteOverview.balance) : '-', valueClass: 'text-emerald-600 dark:text-emerald-400' },
    { key: 'today-profit', label: 'todayProfit', icon: 'dollar', iconClass: 'text-emerald-500', value: profit?.available ? formatMoney(profit.today_gross_profit) : '-', valueClass: profit && profit.today_gross_profit < 0 ? 'text-red-600 dark:text-red-400' : 'text-emerald-600 dark:text-emerald-400', detail: profit?.available ? `${formatPercent(profit.today_gross_margin)} ${t('admin.sub2apiProviders.remoteOverview.marginSuffix')}` : undefined },
    { key: 'month-profit', label: 'monthProfit', icon: 'chart', iconClass: 'text-cyan-500', value: profit?.month_profit_available ? formatMoney(profit.month_gross_profit) : '-', valueClass: profit && profit.month_gross_profit < 0 ? 'text-red-600 dark:text-red-400' : 'text-cyan-600 dark:text-cyan-400', detail: profit?.month_profit_available ? `${formatPercent(profit.month_gross_margin)} ${t('admin.sub2apiProviders.remoteOverview.marginSuffix')}` : undefined },
    { key: 'cumulative-profit', label: 'cumulativeProfit', icon: 'chart', iconClass: 'text-teal-500', value: profit?.cumulative_profit_available ? formatMoney(profit.cumulative_gross_profit) : '-', valueClass: profit && profit.cumulative_gross_profit < 0 ? 'text-red-600 dark:text-red-400' : 'text-teal-600 dark:text-teal-400', detail: profit?.cumulative_profit_available ? `${formatPercent(profit.cumulative_gross_margin)} ${t('admin.sub2apiProviders.remoteOverview.marginSuffix')}` : undefined },
  )
  return metrics
})

const availabilityStatus = computed<ProviderHealthStatus>(() => props.overview?.availability_status ?? 'unknown')
const routes = computed(() => props.overview?.routes ?? [])
const enabledRouteCount = computed(() => routes.value.filter(route => route.enabled).length)
const abnormalRouteCount = computed(() => routes.value.filter(route => route.status === 'unhealthy' || route.status === 'degraded').length)
// The provider pane is the at-a-glance operational view. Keep unhealthy routes
// first, but never hide healthy/disabled accounts behind a secondary panel.
const sortedRoutes = computed(() => [...routes.value]
  .sort((a, b) => routeSeverity(b.status) - routeSeverity(a.status) || a.account_name.localeCompare(b.account_name)))

const availabilityDotClass = computed(() => statusDotClass(availabilityStatus.value))
const availabilityTextClass = computed(() => statusTextClass(availabilityStatus.value))

const routeSeverity = (status: Sub2APIProviderProbeTargetHealth['status']) => ({
  unhealthy: 5,
  degraded: 4,
  unknown: 3,
  disabled: 2,
  healthy: 1,
}[status] ?? 0)

const statusDotClass = (status: ProviderHealthStatus | Sub2APIProviderProbeTargetHealth['status']) => ({
  healthy: 'bg-green-500',
  degraded: 'bg-amber-400',
  unhealthy: 'bg-red-500',
  unknown: 'border border-gray-300 bg-transparent dark:border-dark-500',
  disabled: 'bg-gray-300 dark:bg-dark-500',
}[status])

const statusTextClass = (status: ProviderHealthStatus | Sub2APIProviderProbeTargetHealth['status']) => ({
  healthy: 'text-green-700 dark:text-green-400',
  degraded: 'text-amber-700 dark:text-amber-400',
  unhealthy: 'text-red-700 dark:text-red-400',
  unknown: 'text-gray-500 dark:text-dark-400',
  disabled: 'text-gray-400 dark:text-dark-400',
}[status])

const routeStatusDotClass = (status: Sub2APIProviderProbeTargetHealth['status']) => statusDotClass(status)
const routeStatusTextClass = (status: Sub2APIProviderProbeTargetHealth['status']) => statusTextClass(status)
const accountStatusLabel = (status: string) => ({
  inactive: t('admin.accounts.status.inactive'),
  error: t('admin.accounts.status.error'),
  disabled: t('admin.accounts.status.paused'),
}[status] || status)
const accountStatusClass = (status: string) => status === 'error'
  ? 'border-red-200 bg-red-50 text-red-700 dark:border-red-800 dark:bg-red-900/20 dark:text-red-300'
  : 'border-gray-200 bg-gray-50 text-gray-600 dark:border-dark-600 dark:bg-dark-700 dark:text-dark-300'

const formatMultiplier = (value: number) => Number.isInteger(value) ? value.toFixed(0) : String(Number(value.toFixed(4)))
const multiplierRangeState = (route: Sub2APIProviderProbeTargetHealth) => route.sub2api_optimize_enabled
  ? getMultiplierRangeState(
    route.remote_group_multiplier,
    route.sub2api_min_multiplier,
    route.sub2api_max_multiplier
  )
  : 'unbounded'
const multiplierOutOfRange = (route: Sub2APIProviderProbeTargetHealth) => {
  const state = multiplierRangeState(route)
  return state === 'above' || state === 'below'
}
const multiplierClass = (route: Sub2APIProviderProbeTargetHealth) => {
  if (!route.sub2api_optimize_enabled) {
    return 'border-slate-200 bg-slate-50 text-slate-600 dark:border-slate-700 dark:bg-slate-800/60 dark:text-slate-300'
  }
  switch (multiplierRangeState(route)) {
    case 'below':
      return 'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-300'
    case 'within':
      return 'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-800 dark:bg-emerald-900/20 dark:text-emerald-300'
    case 'above':
      return 'border-red-200 bg-red-50 text-red-700 dark:border-red-800 dark:bg-red-900/20 dark:text-red-300'
    case 'unbounded':
      return 'border-sky-200 bg-sky-50 text-sky-700 dark:border-sky-800 dark:bg-sky-900/20 dark:text-sky-300'
    default:
      return 'border-gray-200 bg-gray-50 text-gray-600 dark:border-dark-600 dark:bg-dark-700 dark:text-dark-300'
  }
}
const multiplierTitle = (route: Sub2APIProviderProbeTargetHealth) => {
  const current = route.remote_group_multiplier == null ? '-' : formatMultiplier(route.remote_group_multiplier)
  if (!route.sub2api_optimize_enabled) {
    return t('admin.sub2apiProviders.multiplierOptimizationDisabled', { current })
  }
  const state = multiplierRangeState(route)
  if (state === 'above') {
    return t('admin.sub2apiProviders.multiplierRangeAboveDetail', { current, max: route.sub2api_max_multiplier })
  }
  if (state === 'below') {
    return t('admin.sub2apiProviders.multiplierRangeBelowDetail', { current, min: route.sub2api_min_multiplier })
  }
  if (state === 'within') {
    return t('admin.sub2apiProviders.multiplierRangeWithinDetail', {
      current,
      min: route.sub2api_min_multiplier,
      max: route.sub2api_max_multiplier,
    })
  }
  return t('admin.sub2apiProviders.multiplierRangeUnconfigured', { current })
}

const pathStatusTitle = computed(() => props.provider.api_path_keys
  ? `Keys: ${props.provider.api_path_keys}\nGroups: ${props.provider.api_path_groups || t('admin.sub2apiProviders.pathsNotDetected')}`
  : t('admin.sub2apiProviders.pathsNotDetectedHint'))
</script>

<style scoped>
@keyframes provider-card-in {
  from {
    opacity: 0;
    transform: translateY(6px);
  }

  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.provider-card {
  animation: provider-card-in 360ms cubic-bezier(0.22, 1, 0.36, 1) both;
  animation-delay: var(--provider-entry-delay, 0ms);
}

.route-platform,
.multiplier-badge,
.route-optimize-badge {
  display: inline-flex;
  align-items: center;
  gap: 0.125rem;
  border-radius: 3px;
  font-size: 10px;
  line-height: 1rem;
  padding: 0 0.375rem;
}

.multiplier-badge {
  font-size: 12px;
  font-weight: 700;
  line-height: 1.25rem;
  padding: 0 0.5rem;
}

.card-action {
  display: inline-flex;
  min-height: 2.75rem;
  min-width: 0;
  cursor: pointer;
  align-items: center;
  justify-content: center;
  gap: 0.375rem;
  border-width: 1px;
  border-style: solid;
  border-radius: 0.375rem;
  padding: 0 0.375rem;
  font-size: 0.6875rem;
  font-weight: 500;
  transition: color 150ms, background-color 150ms, border-color 150ms;
}

.card-action:focus-visible {
  outline: none;
  box-shadow: 0 0 0 2px currentColor;
}

@media (min-width: 640px) {
  .card-action {
    padding: 0 0.5rem;
    font-size: 0.75rem;
  }
}

@media (prefers-reduced-motion: reduce) {
  .provider-card {
    animation: none;
    transition: none;
  }
}
</style>

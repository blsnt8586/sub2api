<template>
  <article
    class="provider-card flex h-full min-h-[360px] flex-col overflow-hidden rounded-lg border border-gray-200 bg-white shadow-sm transition-[box-shadow,border-color] duration-200 ease-out hover:border-gray-300 hover:shadow-md dark:border-dark-700 dark:bg-dark-800 dark:hover:border-dark-600"
    :class="provider.status === 'inactive' ? 'opacity-90' : ''"
    :style="{ '--provider-entry-delay': `${Math.min(animationIndex ?? 0, 7) * 45}ms` }"
  >
    <div class="flex flex-1 flex-col px-4 py-3">
      <header class="flex min-w-0 items-start justify-between gap-3">
        <div class="min-w-0 flex-1">
          <div class="flex min-w-0 items-center gap-2">
            <h2 class="min-w-0 truncate text-sm font-semibold text-gray-900 dark:text-white" :title="provider.name">
              {{ provider.name }}
            </h2>
            <span :class="['badge flex-shrink-0 text-[10px]', provider.status === 'active' ? 'badge-success' : 'badge-gray']">
              {{ t(`admin.sub2apiProviders.statusLabels.${provider.status}`) }}
            </span>
          </div>
          <div class="mt-1.5 flex min-w-0 items-center gap-1.5 text-xs text-gray-500 dark:text-dark-400">
            <span class="flex-shrink-0 font-medium text-blue-600 dark:text-blue-400">{{ providerTypeLabel }}</span>
            <span aria-hidden="true">·</span>
            <a
              v-if="providerURL"
              :href="providerURL"
              target="_blank"
              rel="noopener noreferrer"
              data-test="provider-upstream-link"
              class="inline-flex min-w-0 max-w-[13rem] items-center gap-1 rounded border border-blue-200 bg-blue-50 px-1.5 py-1 font-medium text-blue-700 transition-colors hover:border-blue-300 hover:bg-blue-100 hover:text-blue-800 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 dark:border-blue-800 dark:bg-blue-900/20 dark:text-blue-300 dark:hover:border-blue-700 dark:hover:bg-blue-900/40"
              :title="t('admin.sub2apiProviders.openUpstream')"
            >
              <span class="truncate">{{ providerHostname }}</span>
              <Icon name="externalLink" size="xs" class="flex-shrink-0" />
            </a>
            <span v-else class="min-w-0 truncate" :title="provider.base_url">{{ providerHostname }}</span>
            <span aria-hidden="true">·</span>
            <span class="flex-shrink-0 tabular-nums">{{ t('admin.sub2apiProviders.linkedAccountCount', { count: provider.accounts_count ?? 0 }) }}</span>
          </div>
        </div>

        <button
          type="button"
          class="inline-flex h-11 w-11 flex-shrink-0 cursor-pointer items-center justify-center rounded-md text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 focus-visible:ring-offset-2 dark:text-dark-300 dark:hover:bg-dark-700 dark:hover:text-white dark:focus-visible:ring-offset-dark-800"
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

      <div v-else class="mt-3 rounded-md border border-gray-100 dark:border-dark-700">
        <button
          type="button"
          class="flex min-h-10 w-full min-w-0 cursor-pointer items-center justify-between gap-3 px-3 py-2 text-left transition-colors hover:bg-gray-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 focus-visible:ring-inset dark:hover:bg-dark-700/50"
          data-test="provider-control-status"
          @click="emit('view-health')"
        >
          <span class="flex min-w-0 items-center gap-2">
            <span class="h-2 w-2 flex-shrink-0 rounded-full" :class="availabilityDotClass"></span>
            <span class="truncate text-xs font-medium text-gray-600 dark:text-dark-300">{{ t('admin.sub2apiProviders.health.activeProbe') }}</span>
          </span>
          <span class="flex min-w-0 flex-shrink items-center justify-end text-[11px] tabular-nums" :class="availabilityTextClass">
            <span class="flex-shrink-0">{{ t(`admin.sub2apiProviders.health.availabilityStatus.${availabilityStatus}`) }}</span>
            <span v-if="latestControl?.health_latency_ms != null" class="ml-2 flex-shrink-0 text-gray-400 dark:text-dark-400">{{ latestControl.health_latency_ms }} ms</span>
            <span v-if="latestControl?.last_checked_at" class="ml-2 min-w-0 truncate text-gray-400 dark:text-dark-400" :title="latestControl.last_checked_at">
              {{ formatRelative(latestControl.last_checked_at) }}
            </span>
          </span>
        </button>
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
          <span class="grid grid-cols-3 gap-x-3 gap-y-2">
            <span v-for="metric in remoteMetrics" :key="metric.key" class="min-w-0" :data-test="`remote-metric-${metric.key}`">
              <span class="flex min-w-0 items-center gap-1 text-[10px] font-medium text-gray-500 dark:text-dark-400">
                <Icon :name="metric.icon" size="xs" class="flex-shrink-0" :class="metric.iconClass" />
                <span class="truncate">{{ t(`admin.sub2apiProviders.remoteOverview.${metric.label}`) }}</span>
              </span>
              <span class="mt-0.5 block truncate text-sm font-semibold tabular-nums" :class="metric.valueClass" :title="metric.title">
                {{ metric.value }}
              </span>
              <span
                v-if="metric.detail"
                class="mt-0.5 block truncate text-[10px] font-medium tabular-nums text-gray-500 dark:text-dark-400"
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
              <span class="flex-shrink-0 text-[11px] font-medium tabular-nums" :class="routeStatusTextClass(route.status)">
                {{ route.latency_ms != null ? `${route.latency_ms} ms` : t(`admin.sub2apiProviders.health.status.${route.status}`) }}
              </span>
            </div>
            <div class="mt-1.5 flex min-w-0 items-center gap-2 pl-3.5 text-[11px] text-gray-500 dark:text-dark-400">
              <span class="min-w-0 flex-1 truncate font-medium text-gray-600 dark:text-dark-300" :title="routeIdentityTitle(route)">{{ route.remote_group_name || t('admin.sub2apiProviders.health.routes.unboundGroup') }}</span>
              <span v-if="route.test_model" class="inline-flex min-w-0 max-w-[46%] items-center gap-1 text-gray-500 dark:text-dark-400" :title="route.test_model">
                <Icon name="cpu" size="xs" class="flex-shrink-0" />
                <span class="truncate">{{ route.test_model }}</span>
              </span>
              <span class="flex-shrink-0 tabular-nums">{{ route.last_checked_at ? formatRelative(route.last_checked_at) : t('admin.sub2apiProviders.health.neverChecked') }}</span>
            </div>
            <div class="mt-1.5 min-w-0 pl-3.5">
              <Sub2APIProviderRouteTimeline :route="route" compact />
            </div>
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
          <Icon :name="provider.auth_mode === 'token_pair' ? 'key' : 'lock'" size="xs" class="flex-shrink-0" :class="authStatusClass" />
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
import { formatRelativeTime } from '@/utils/format'
import { getMultiplierRangeState } from '@/utils/sub2apiValidation'
import { platformTagClass } from '@/utils/platformColors'
import Icon from '@/components/icons/Icon.vue'
import Sub2APIProviderRouteTimeline from './Sub2APIProviderRouteTimeline.vue'

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
const formatRelative = (value: string | Date | null | undefined) => {
  void props.nowTick
  return formatRelativeTime(value)
}

const providerTypeLabel = computed(() => props.provider.provider_type === 'sub2api' ? 'Sub2API' : props.provider.provider_type)
const authStatusLabel = computed(() => {
  if (props.provider.auth_mode !== 'token_pair') return t('admin.sub2apiProviders.auth.password')
  if (props.provider.last_auth_error) return t('admin.sub2apiProviders.auth.reimportRequired')
  if (props.provider.has_access_token && props.provider.has_refresh_token) return t('admin.sub2apiProviders.auth.tokenReady')
  return t('admin.sub2apiProviders.auth.tokenIncomplete')
})
const authStatusClass = computed(() => {
  if (props.provider.auth_mode !== 'token_pair') return 'text-gray-500 dark:text-dark-400'
  if (props.provider.last_auth_error) return 'text-red-600 dark:text-red-400'
  return props.provider.has_access_token && props.provider.has_refresh_token
    ? 'text-green-600 dark:text-green-400'
    : 'text-amber-600 dark:text-amber-400'
})
const authStatusTitle = computed(() => props.provider.auth_mode === 'token_pair'
  ? t('admin.sub2apiProviders.auth.tokenTitle')
  : t('admin.sub2apiProviders.auth.passwordTitle'))
const providerHostname = computed(() => {
  try {
    return new URL(props.provider.base_url).host
  } catch {
    return props.provider.base_url
  }
})
const providerURL = computed(() => {
  try {
    const url = new URL(props.provider.base_url)
    return url.protocol === 'http:' || url.protocol === 'https:' ? url.href : null
  } catch {
    return null
  }
})

const remoteOverviewAvailable = computed(() => props.remoteOverview?.available === true)
const remoteOverviewErrorMessage = computed(() => props.remoteOverviewError || props.remoteOverview?.last_error || null)
const remoteUsage = computed(() => props.remoteOverview?.usage ?? null)
const remoteDashboardAvailable = computed(() => remoteUsage.value?.dashboard_available === true)
const formatMoney = (value: number | null | undefined) => {
  if (value == null || !Number.isFinite(value)) return '-'
  return `$${new Intl.NumberFormat(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 4 }).format(value)}`
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
  remoteDashboardAvailable.value ? formatter(value) : '-'
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
    { key: 'balance', label: 'balance', icon: 'creditCard', iconClass: 'text-emerald-500', value: formatMoney(props.remoteOverview?.balance), valueClass: 'text-emerald-600 dark:text-emerald-400' },
    { key: 'total-recharged', label: 'totalRecharged', icon: 'dollar', iconClass: 'text-blue-500', value: formatMoney(usage?.total_recharged), valueClass: 'text-blue-600 dark:text-blue-400', title: usage?.funding_summary_available ? t('admin.sub2apiProviders.remoteOverview.fundingBreakdown', { orders: formatMoney(usage.order_recharged), redeems: formatMoney(usage.redeem_recharged) }) : undefined, detail: usage?.funding_summary_available ? t('admin.sub2apiProviders.remoteOverview.fundingBreakdown', { orders: formatMoney(usage.order_recharged), redeems: formatMoney(usage.redeem_recharged) }) : undefined },
    { key: 'total-consumed', label: 'totalConsumed', icon: 'chart', iconClass: 'text-violet-500', value: formatMoney(usage?.total_consumed), valueClass: 'text-violet-600 dark:text-violet-400' },
    { key: 'today-requests', label: 'todayRequests', icon: 'chart', iconClass: 'text-green-500', value: remoteMetricValue(usage?.today_requests, formatCount), valueClass: 'text-gray-900 dark:text-white' },
    { key: 'today-consumed', label: 'todayConsumed', icon: 'dollar', iconClass: 'text-purple-500', value: remoteMetricValue(usage?.today_actual_cost, formatMoney), valueClass: 'text-purple-600 dark:text-purple-400' },
    { key: 'today-tokens', label: 'todayTokens', icon: 'database', iconClass: 'text-amber-500', value: remoteMetricValue(usage?.today_tokens, formatTokens), valueClass: 'text-gray-900 dark:text-white' },
    { key: 'total-tokens', label: 'totalTokens', icon: 'database', iconClass: 'text-indigo-500', value: remoteMetricValue(usage?.total_tokens, formatTokens), valueClass: 'text-gray-900 dark:text-white' },
    { key: 'cache-hit-rate', label: 'cacheHitRate', icon: 'database', iconClass: 'text-cyan-500', value: usage?.cache_hit_rate_available ? formatPercent(usage.cache_hit_rate) : '-', valueClass: 'text-cyan-600 dark:text-cyan-400' },
    { key: 'average-response', label: 'averageResponse', icon: 'clock', iconClass: 'text-rose-500', value: remoteMetricValue(usage?.average_duration_ms, formatDuration), valueClass: 'text-rose-600 dark:text-rose-400' },
  ]
  const profit = props.remoteOverview?.profit
  if (profit?.available) {
    metrics.push(
      { key: 'today-profit', label: 'todayProfit', icon: 'dollar', iconClass: 'text-emerald-500', value: formatMoney(profit.today_gross_profit), valueClass: profit.today_gross_profit >= 0 ? 'text-emerald-600 dark:text-emerald-400' : 'text-red-600 dark:text-red-400', detail: `${formatPercent(profit.today_gross_margin)} ${t('admin.sub2apiProviders.remoteOverview.marginSuffix')}` },
      { key: 'total-profit', label: 'totalProfit', icon: 'chart', iconClass: 'text-teal-500', value: formatMoney(profit.total_gross_profit), valueClass: profit.total_gross_profit >= 0 ? 'text-teal-600 dark:text-teal-400' : 'text-red-600 dark:text-red-400', detail: `${formatPercent(profit.total_gross_margin)} ${t('admin.sub2apiProviders.remoteOverview.marginSuffix')}` },
    )
  }
  return metrics
})

const latestControl = computed(() => props.overview?.latest_control ?? null)
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

const routeIdentityTitle = (route: Sub2APIProviderProbeTargetHealth) => [
  route.account_name,
  route.platform,
  route.provider_api_key_id != null ? `Key #${route.provider_api_key_id}` : null,
  route.remote_group_name,
  route.remote_group_id != null ? `Group #${route.remote_group_id}` : null,
  route.test_model,
  route.remote_group_multiplier != null ? `×${formatMultiplier(route.remote_group_multiplier)}` : null,
].filter(Boolean).join(' · ')

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

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
            <span class="min-w-0 truncate" :title="provider.base_url">{{ providerHostname }}</span>
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
        <span v-else-if="remoteOverviewAvailable" class="block min-w-0">
          <span class="grid min-h-9 grid-cols-[minmax(0,1fr)_auto_auto] items-center gap-3">
            <span class="min-w-0">
              <span class="flex items-center gap-1.5 text-xs font-medium text-gray-600 dark:text-dark-300">
                <Icon name="creditCard" size="sm" class="flex-shrink-0 text-blue-500" />
                {{ t('admin.sub2apiProviders.remoteOverview.balance') }}
              </span>
              <span class="mt-0.5 block truncate text-base font-semibold tabular-nums text-gray-900 dark:text-white">
                {{ formatBalance(remoteOverview?.balance ?? 0) }}
              </span>
            </span>
            <span class="border-l border-gray-100 pl-3 dark:border-dark-700">
              <span class="block text-xs text-gray-500 dark:text-dark-400">{{ t('admin.sub2apiProviders.remoteOverview.groups') }}</span>
              <span class="mt-0.5 block text-sm font-semibold tabular-nums text-gray-800 dark:text-dark-100">{{ remoteGroups.length }}</span>
            </span>
            <span class="min-w-16 border-l border-gray-100 pl-3 dark:border-dark-700">
              <span class="block text-xs text-gray-500 dark:text-dark-400">{{ t('admin.sub2apiProviders.remoteOverview.rateRange') }}</span>
              <span class="mt-0.5 block text-sm font-semibold tabular-nums text-gray-800 dark:text-dark-100">{{ remoteRateRangeLabel }}</span>
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
            v-for="route in visibleRoutes"
            :key="route.id"
            type="button"
            class="group w-full min-w-0 cursor-pointer py-2 text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 focus-visible:ring-inset"
            :data-test="`provider-route-probe-${route.id}`"
            @click="emit('view-health')"
          >
            <div class="flex min-w-0 items-center gap-2">
              <span class="h-1.5 w-1.5 flex-shrink-0 rounded-full" :class="routeStatusDotClass(route.status)"></span>
              <span class="min-w-0 flex-1 truncate text-xs font-medium text-gray-700 group-hover:text-gray-900 dark:text-dark-200 dark:group-hover:text-white" :title="route.account_name">
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
              <span class="route-platform flex-shrink-0 border border-gray-200 text-gray-500 dark:border-dark-600 dark:text-dark-300">{{ route.platform }}</span>
              <span class="flex-shrink-0 text-[10px] tabular-nums" :class="routeStatusTextClass(route.status)">
                {{ route.latency_ms != null ? `${route.latency_ms} ms` : t(`admin.sub2apiProviders.health.status.${route.status}`) }}
              </span>
            </div>
            <div class="mt-1 flex min-w-0 items-center gap-2 pl-3.5 text-[10px] text-gray-400 dark:text-dark-400">
              <span class="min-w-0 flex-1 truncate" :title="routeIdentityTitle(route)">{{ route.remote_group_name || t('admin.sub2apiProviders.health.routes.unboundGroup') }}</span>
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

          <button
            v-if="hiddenRouteCount > 0"
            type="button"
            data-test="provider-view-all-routes"
            class="flex min-h-10 w-full cursor-pointer items-center justify-center gap-1.5 text-xs font-medium text-primary-600 transition-colors hover:bg-primary-50 hover:text-primary-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 focus-visible:ring-inset dark:text-primary-400 dark:hover:bg-primary-900/20"
            @click="emit('view-accounts')"
          >
            {{ t('admin.sub2apiProviders.health.routes.viewAll', { count: routes.length }) }}
            <Icon name="chevronRight" size="xs" />
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
        <span class="inline-flex items-center gap-1.5" :title="provider.last_sync_error || provider.last_sync_at || undefined">
          <span class="h-1.5 w-1.5 rounded-full" :class="syncDotClass"></span>
          {{ t('admin.sub2apiProviders.accountSyncLabel') }}：{{ provider.last_sync_status ? t(`admin.sub2apiProviders.syncStatus.${provider.last_sync_status}`) : t('admin.sub2apiProviders.syncStatus.never') }}
        </span>
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

const remoteGroups = computed(() => props.remoteOverview?.groups ?? [])
const remoteOverviewAvailable = computed(() => props.remoteOverview?.available === true)
const remoteOverviewErrorMessage = computed(() => props.remoteOverviewError || props.remoteOverview?.last_error || null)
const remoteRateRangeLabel = computed(() => {
  if (remoteGroups.value.length === 0) return '-'
  const values = remoteGroups.value.map(group => group.effective_multiplier)
  const minimum = Math.min(...values)
  const maximum = Math.max(...values)
  return minimum === maximum
    ? `×${formatMultiplier(minimum)}`
    : `×${formatMultiplier(minimum)} - ×${formatMultiplier(maximum)}`
})
const formatBalance = (value: number) => new Intl.NumberFormat(undefined, {
  minimumFractionDigits: 0,
  maximumFractionDigits: 2,
}).format(value)

const latestControl = computed(() => props.overview?.latest_control ?? null)
const availabilityStatus = computed<ProviderHealthStatus>(() => props.overview?.availability_status ?? 'unknown')
const routes = computed(() => props.overview?.routes ?? [])
const enabledRouteCount = computed(() => routes.value.filter(route => route.enabled).length)
const abnormalRouteCount = computed(() => routes.value.filter(route => route.status === 'unhealthy' || route.status === 'degraded').length)
const visibleRoutes = computed(() => [...routes.value]
  .sort((a, b) => routeSeverity(b.status) - routeSeverity(a.status) || a.account_name.localeCompare(b.account_name))
  .slice(0, 3))
const hiddenRouteCount = computed(() => Math.max(0, routes.value.length - visibleRoutes.value.length))

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
const multiplierClass = (route: Sub2APIProviderProbeTargetHealth) => multiplierOutOfRange(route)
  ? 'border-red-200 bg-red-50 text-red-700 dark:border-red-800 dark:bg-red-900/20 dark:text-red-300'
  : 'border-gray-200 bg-gray-50 text-gray-600 dark:border-dark-600 dark:bg-dark-700 dark:text-dark-300'
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

const syncDotClass = computed(() => {
  switch (props.provider.last_sync_status) {
    case 'success': return 'bg-green-500'
    case 'failed': return 'bg-red-500'
    default: return 'bg-gray-300 dark:bg-dark-500'
  }
})

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
.multiplier-badge {
  display: inline-flex;
  align-items: center;
  gap: 0.125rem;
  border-radius: 3px;
  font-size: 9px;
  line-height: 0.9rem;
  padding: 0 0.25rem;
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

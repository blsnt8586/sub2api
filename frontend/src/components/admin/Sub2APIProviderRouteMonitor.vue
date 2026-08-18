<template>
  <section class="route-monitor border-t border-gray-100 pt-5 dark:border-dark-700" :aria-label="t('admin.sub2apiProviders.health.routes.title')">
    <div class="flex flex-wrap items-center justify-between gap-2">
      <div>
        <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.sub2apiProviders.health.routes.title') }}</h3>
        <p class="mt-1 max-w-3xl text-sm leading-5 text-gray-500 dark:text-dark-400">{{ t('admin.sub2apiProviders.health.routes.description') }}</p>
      </div>
      <span class="inline-flex items-center gap-1.5 text-sm tabular-nums text-gray-500 dark:text-dark-300">
        <span class="h-1.5 w-1.5 rounded-full bg-blue-500"></span>
        {{ t('admin.sub2apiProviders.health.routes.count', { count: routes.length }) }}
      </span>
    </div>

    <div v-if="routes.length" class="mt-3 divide-y divide-gray-100 border-y border-gray-100 dark:divide-dark-700 dark:border-dark-700">
      <article v-for="route in routes" :key="route.id" class="min-w-0">
        <div class="route-row min-w-0 py-2.5">
          <div class="flex min-w-0 items-center gap-3">
            <button
              type="button"
              class="min-w-0 flex-1 text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 focus-visible:ring-inset"
              :aria-expanded="expandedRouteID === route.id"
              :data-test="`route-toggle-${route.id}`"
              @click="toggleRoute(route.id)"
            >
              <div class="flex min-w-0 items-center gap-2">
                <span class="h-2 w-2 flex-shrink-0 rounded-full" :class="statusDotClass(route.status)"></span>
                <span class="truncate text-sm font-medium text-gray-800 dark:text-dark-100" :title="route.account_name">{{ route.account_name }}</span>
                <span v-if="route.remote_group_multiplier != null" class="route-multiplier flex-shrink-0">×{{ formatMultiplier(route.remote_group_multiplier) }}</span>
                <span class="route-platform flex-shrink-0 text-gray-500 dark:text-dark-300">{{ route.platform }}</span>
                <span v-if="isDirty(route.id)" class="flex-shrink-0 text-xs font-medium text-amber-600 dark:text-amber-400">{{ t('admin.sub2apiProviders.health.routes.unsaved') }}</span>
                <Icon name="chevronDown" size="xs" class="flex-shrink-0 text-gray-400 transition-transform duration-200 motion-reduce:transition-none" :class="expandedRouteID === route.id ? 'rotate-180' : ''" />
              </div>
              <div class="mt-1 flex min-w-0 items-center gap-1.5 pl-4 text-xs leading-5 text-gray-500 dark:text-dark-400">
                <span class="min-w-0 max-w-[38%] truncate" :title="routeIdentityTitle(route)">{{ route.remote_group_name || t('admin.sub2apiProviders.health.routes.unboundGroup') }}</span>
                <span aria-hidden="true">·</span>
                <span v-if="route.test_model" class="inline-flex min-w-0 max-w-[42%] items-center gap-1" :title="route.test_model">
                  <Icon name="cpu" size="xs" class="flex-shrink-0" />
                  <span class="truncate">{{ route.test_model }}</span>
                </span>
                <span v-else class="truncate">{{ t('admin.sub2apiProviders.health.routes.modelNotSet') }}</span>
                <span class="ml-auto flex-shrink-0" :class="statusTextClass(route.status)">{{ t(`admin.sub2apiProviders.health.status.${route.status}`) }}</span>
              </div>
            </button>

            <div class="hidden min-w-0 items-center gap-3 text-right text-xs sm:flex">
              <span class="w-16 tabular-nums text-gray-600 dark:text-dark-300">{{ route.latency_ms != null ? `${route.latency_ms} ms` : '—' }}</span>
              <span class="w-20 truncate tabular-nums text-gray-400 dark:text-dark-400" :title="route.last_checked_at || undefined">{{ route.last_checked_at ? formatRelative(route.last_checked_at) : t('admin.sub2apiProviders.health.neverChecked') }}</span>
            </div>

            <div class="flex flex-shrink-0 items-center gap-1">
              <button
                type="button"
                class="icon-action min-h-11 min-w-11 cursor-pointer text-gray-500 hover:bg-gray-100 hover:text-gray-800 dark:text-dark-300 dark:hover:bg-dark-700 dark:hover:text-white"
                :title="t('admin.sub2apiProviders.health.routes.history')"
                :aria-label="`${route.account_name} ${t('admin.sub2apiProviders.health.routes.history')}`"
                :data-test="`route-history-${route.id}`"
                @click="openHistory(route.id)"
              >
                <Icon name="clock" size="sm" />
              </button>
              <button
                type="button"
                class="inline-flex min-h-11 cursor-pointer items-center gap-1 rounded-md px-2 text-xs font-medium text-primary-600 transition-colors hover:bg-primary-50 disabled:cursor-not-allowed disabled:opacity-40 dark:text-primary-400 dark:hover:bg-primary-900/20"
                :disabled="runningTargetId === route.id || isDirty(route.id)"
                :title="isDirty(route.id) ? t('admin.sub2apiProviders.health.routes.saveBeforeRun') : t('admin.sub2apiProviders.health.routes.runNow')"
                :aria-label="`${route.account_name} ${t('admin.sub2apiProviders.health.routes.runNow')}`"
                :data-test="`route-run-${route.id}`"
                @click="emit('run', route.id)"
              >
                <Icon :name="runningTargetId === route.id ? 'refresh' : 'play'" size="sm" :class="runningTargetId === route.id ? 'animate-spin' : ''" />
                <span class="hidden lg:inline">{{ t('admin.sub2apiProviders.health.routes.runNowShort') }}</span>
              </button>
            </div>
          </div>

          <Sub2APIProviderRouteTimeline class="mt-2 pl-4" :route="route" />
        </div>

        <div v-if="expandedRouteID === route.id" class="route-detail grid gap-4 border-t border-gray-100 py-3 dark:border-dark-700 md:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
          <div class="grid gap-3 sm:grid-cols-2">
            <label class="route-field flex min-h-11 items-center justify-between gap-3 sm:col-span-2">
              <span class="font-medium">{{ t('admin.sub2apiProviders.health.routes.enabled') }}</span>
              <input
                :checked="route.enabled"
                type="checkbox"
                class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                @change="emit('update', route.id, { enabled: ($event.target as HTMLInputElement).checked })"
              />
            </label>
            <label class="route-field">
              <span class="font-medium">{{ t('admin.sub2apiProviders.health.routes.interval') }}</span>
              <span class="route-field__hint">{{ t('admin.sub2apiProviders.health.routes.intervalHint') }}</span>
              <input
                :value="route.interval_seconds"
                type="number"
                min="30"
                max="86400"
                class="input mt-1"
                @change="emitNumber(route.id, 'interval_seconds', $event)"
              />
            </label>
            <label class="route-field">
              <span class="font-medium">{{ t('admin.sub2apiProviders.health.routes.timeout') }}</span>
              <span class="route-field__hint">{{ t('admin.sub2apiProviders.health.routes.timeoutHint') }}</span>
              <input
                :value="route.timeout_seconds"
                type="number"
                min="3"
                max="120"
                class="input mt-1"
                @change="emitNumber(route.id, 'timeout_seconds', $event)"
              />
            </label>
            <label class="route-field">
              <span class="font-medium">{{ t('admin.sub2apiProviders.health.routes.degradedLatency') }}</span>
              <span class="route-field__hint">{{ t('admin.sub2apiProviders.health.routes.degradedLatencyHint') }}</span>
              <input
                :value="route.degraded_latency_ms"
                type="number"
                min="100"
                max="120000"
                class="input mt-1"
                @change="emitNumber(route.id, 'degraded_latency_ms', $event)"
              />
            </label>
            <div class="route-field" :data-test="`route-account-model-${route.id}`">
              <span class="font-medium">{{ t('admin.sub2apiProviders.health.routes.model') }}</span>
              <div class="mt-1 flex min-h-10 min-w-0 items-center justify-between gap-2 rounded-md border border-gray-200 bg-gray-50 px-3 dark:border-dark-600 dark:bg-dark-800/60">
                <span class="min-w-0 truncate text-sm font-medium text-gray-700 dark:text-dark-200" :title="route.test_model || undefined">
                  {{ route.test_model || t('admin.sub2apiProviders.health.routes.modelNotSet') }}
                </span>
                <span class="flex-shrink-0 text-xs font-medium text-gray-500 dark:text-dark-400">
                  {{ t('admin.sub2apiProviders.health.routes.modelSourceAccount') }}
                </span>
              </div>
            </div>
            <label class="route-field flex min-h-11 items-center justify-between gap-3 sm:col-span-2">
              <span class="font-medium">{{ t('admin.sub2apiProviders.health.routes.allowMedia') }}</span>
              <input
                :checked="route.allow_media_probe"
                type="checkbox"
                class="h-4 w-4 rounded border-red-300 text-red-600 focus:ring-red-500"
                @change="emit('update', route.id, { allow_media_probe: ($event.target as HTMLInputElement).checked })"
              />
            </label>
            <div class="rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-xs leading-5 text-amber-800 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-300 sm:col-span-2">
              <p>{{ t('admin.sub2apiProviders.health.mediaWarning') }}</p>
              <p>{{ t('admin.sub2apiProviders.health.mediaMinInterval') }}</p>
            </div>
          </div>

          <div class="min-w-0 border-t border-gray-100 pt-3 md:border-l md:border-t-0 md:pl-4 md:pt-0 dark:border-dark-700">
            <div class="flex flex-wrap items-center justify-between gap-2">
              <span class="text-sm font-semibold text-gray-700 dark:text-dark-200">{{ t('admin.sub2apiProviders.health.routes.history1h') }}</span>
              <button type="button" class="text-sm text-primary-600 hover:underline dark:text-primary-400" @click="openHistory(route.id)">
                {{ t('admin.sub2apiProviders.health.routes.refreshHistory') }}
              </button>
            </div>
            <div v-if="historyByTarget[route.id]?.length" class="mt-2 max-h-36 divide-y divide-gray-100 overflow-y-auto border-y border-gray-100 dark:divide-dark-700 dark:border-dark-700">
              <div v-for="(item, index) in historyByTarget[route.id]" :key="`${item.last_checked_at || index}-${index}`" class="flex min-h-9 items-center gap-2 py-1.5 text-xs">
                <span class="h-1.5 w-1.5 flex-shrink-0 rounded-full" :class="statusDotClass(item.status)" />
                <span class="w-16 flex-shrink-0" :class="statusTextClass(item.status)">{{ t(`admin.sub2apiProviders.health.status.${item.status}`) }}</span>
                <span class="min-w-0 flex-1 truncate text-gray-500 dark:text-dark-400">{{ item.test_model || route.test_model || '—' }}</span>
                <span class="flex-shrink-0 tabular-nums text-gray-400 dark:text-dark-400">{{ item.latency_ms != null ? `${item.latency_ms} ms` : '—' }}</span>
                <span class="flex-shrink-0 tabular-nums text-gray-400 dark:text-dark-400">{{ item.last_checked_at ? formatRelative(item.last_checked_at) : '—' }}</span>
              </div>
            </div>
            <p v-else class="mt-2 border-y border-gray-100 py-3 text-center text-sm text-gray-500 dark:border-dark-700 dark:text-dark-400">{{ t('admin.sub2apiProviders.health.routes.noHistory1h') }}</p>
          </div>
        </div>
      </article>
    </div>

    <p v-else class="mt-3 border-y border-dashed border-gray-200 py-4 text-center text-sm text-gray-500 dark:border-dark-600 dark:text-dark-400">{{ t('admin.sub2apiProviders.health.routes.empty') }}</p>
  </section>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type {
  ProviderAccountProbeStatus,
  Sub2APIProviderProbeTargetHealth,
  UpdateProviderProbeTargetRequest,
} from '@/api/admin/sub2apiProviders'
import { formatRelativeTime } from '@/utils/format'
import Icon from '@/components/icons/Icon.vue'
import Sub2APIProviderRouteTimeline from './Sub2APIProviderRouteTimeline.vue'

const props = withDefaults(defineProps<{
  routes: Sub2APIProviderProbeTargetHealth[]
  historyByTarget: Record<number, Sub2APIProviderProbeTargetHealth[]>
  runningTargetId?: number | null
  dirtyTargetIds?: number[]
  nowTick?: number
}>(), {
  runningTargetId: null,
  dirtyTargetIds: () => [],
})

const emit = defineEmits<{
  (e: 'update', targetID: number, payload: UpdateProviderProbeTargetRequest): void
  (e: 'run', targetID: number): void
  (e: 'history', targetID: number): void
}>()

const { t } = useI18n()
const expandedRouteID = ref<number | null>(null)
const formatRelative = (value: string | Date | null | undefined) => {
  void props.nowTick
  return formatRelativeTime(value)
}

const toggleRoute = (targetID: number) => {
  expandedRouteID.value = expandedRouteID.value === targetID ? null : targetID
}

const openHistory = (targetID: number) => {
  if (expandedRouteID.value !== targetID) expandedRouteID.value = targetID
  emit('history', targetID)
}

const emitNumber = (targetID: number, key: 'interval_seconds' | 'timeout_seconds' | 'degraded_latency_ms', event: Event) => {
  const value = Number((event.target as HTMLInputElement).value)
  if (Number.isInteger(value)) emit('update', targetID, { [key]: value })
}

const isDirty = (targetID: number) => props.dirtyTargetIds.includes(targetID)
const formatMultiplier = (value: number) => Number.isInteger(value) ? value.toFixed(0) : String(Number(value.toFixed(2)))

const statusDotClass = (status: ProviderAccountProbeStatus) => ({
  healthy: 'bg-green-500',
  degraded: 'bg-amber-400',
  unhealthy: 'bg-red-500',
  unknown: 'border border-gray-300 bg-transparent dark:border-dark-500',
  disabled: 'bg-gray-300 dark:bg-dark-500',
}[status])

const statusTextClass = (status: ProviderAccountProbeStatus) => ({
  healthy: 'text-green-600 dark:text-green-400',
  degraded: 'text-amber-600 dark:text-amber-400',
  unhealthy: 'text-red-600 dark:text-red-400',
  unknown: 'text-gray-500 dark:text-dark-400',
  disabled: 'text-gray-400 dark:text-dark-400',
}[status])

const routeIdentityTitle = (route: Sub2APIProviderProbeTargetHealth) => [
  route.account_name,
  route.platform,
  route.provider_api_key_id != null ? `Key #${route.provider_api_key_id}` : null,
  route.remote_group_name,
  route.remote_group_id != null ? `Group #${route.remote_group_id}` : null,
  route.test_model,
].filter(Boolean).join(' · ')
</script>

<style scoped>
.route-row {
  transition: background-color 180ms ease-out;
}

.route-row:hover {
  background: rgb(248 250 252 / 0.72);
}

.route-platform {
  border: 1px solid rgb(226 232 240);
  border-radius: 3px;
  font-size: 11px;
  line-height: 1rem;
  padding: 0 0.3rem;
}

.route-multiplier {
  border: 1px solid rgb(167 243 208);
  border-radius: 3px;
  background: rgb(236 253 245);
  color: rgb(4 120 87);
  font-size: 11px;
  font-weight: 600;
  line-height: 0.9rem;
  padding: 0 0.25rem;
}

.route-detail {
  animation: route-detail-in 180ms ease-out both;
}

.route-field {
  color: rgb(55 65 81);
  font-size: 0.875rem;
  line-height: 1.25rem;
}

.route-field__hint {
  color: rgb(148 163 184);
  display: block;
  font-size: 0.75rem;
  font-weight: 400;
  line-height: 1.1rem;
  margin-top: 0.2rem;
}

.icon-action {
  align-items: center;
  border-radius: 0.375rem;
  display: inline-flex;
  height: 2.25rem;
  justify-content: center;
  transition: background-color 180ms ease-out, color 180ms ease-out;
  width: 2.25rem;
}

.icon-action:disabled {
  cursor: not-allowed;
  opacity: 0.45;
}

@keyframes route-detail-in {
  from { opacity: 0; transform: translateY(-3px); }
  to { opacity: 1; transform: translateY(0); }
}

:global(.dark) .route-row:hover {
  background: rgb(30 41 59 / 0.42);
}

:global(.dark) .route-platform {
  border-color: rgb(71 85 105);
}

:global(.dark) .route-multiplier {
  border-color: rgb(6 78 59);
  background: rgb(6 78 59 / 0.2);
  color: rgb(110 231 183);
}

:global(.dark) .route-field {
  color: rgb(203 213 225);
}

:global(.dark) .route-field__hint {
  color: rgb(148 163 184);
}

@media (prefers-reduced-motion: reduce) {
  .route-row,
  .icon-action,
  .route-detail {
    animation: none;
    transition: none;
  }
}
</style>

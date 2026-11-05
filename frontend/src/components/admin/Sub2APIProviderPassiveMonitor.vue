<template>
  <section class="border-t border-gray-100 pt-5 dark:border-dark-700" :aria-label="text('accountProbes')">
    <div class="flex flex-wrap items-center justify-between gap-2">
      <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ text('accountProbes') }}</h3>
      <span class="text-xs text-gray-500 dark:text-dark-400">{{ text('count', { count: routes.length }) }}</span>
    </div>
    <p class="mt-1 text-sm leading-6 text-gray-500 dark:text-dark-400">{{ text('hybridDescription') }}</p>
    <div class="mt-3 divide-y divide-gray-100 dark:divide-dark-700">
      <article v-for="route in routes" :key="route.id" class="py-3">
        <button type="button" class="flex min-h-11 w-full items-start justify-between gap-3 text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500" :aria-expanded="expanded === route.id" :data-test="`passive-account-${route.id}`" @click="toggle(route.id)">
          <span class="min-w-0">
            <span class="block break-words text-sm font-semibold text-gray-900 dark:text-white">{{ route.account_name }} <span class="ml-1 text-xs font-normal text-gray-500 dark:text-dark-400">{{ route.platform }}</span></span>
            <span class="mt-1 block text-xs text-gray-500 dark:text-dark-400">{{ route.remote_group_name || text('unboundGroup') }}<span v-if="route.remote_group_multiplier != null" class="ml-2 tabular-nums">×{{ route.remote_group_multiplier }}</span></span>
          </span>
          <span class="flex shrink-0 items-center gap-2 text-xs" :class="statusClass(route.status)">{{ route.hybrid_health?.phase === 'recovering' ? text(route.hybrid_health.last_probe_status ? 'recovering' : 'paused') : t(`admin.sub2apiProviders.health.status.${route.status}`) }}<Icon name="chevronDown" size="xs" :class="expanded === route.id ? 'rotate-180' : ''" /></span>
        </button>
        <dl class="mt-2 grid grid-cols-2 gap-x-4 gap-y-1 sm:grid-cols-4">
          <div class="flex min-w-0 items-center justify-between gap-2 py-1"><dt class="text-xs text-gray-500 dark:text-dark-400">{{ text('requestCount') }}</dt><dd class="shrink-0 text-sm font-medium tabular-nums text-gray-800 dark:text-dark-100">{{ route.traffic_request_count }}</dd></div>
          <div class="flex min-w-0 items-center justify-between gap-2 py-1"><dt class="text-xs text-gray-500 dark:text-dark-400">{{ text('failureCount') }}</dt><dd class="shrink-0 text-sm font-medium tabular-nums text-gray-800 dark:text-dark-100">{{ route.traffic_failure_count ?? 0 }}</dd></div>
          <div class="flex min-w-0 items-center justify-between gap-2 py-1"><dt class="text-xs text-gray-500 dark:text-dark-400">{{ text('successRate') }}</dt><dd class="shrink-0 text-sm font-medium tabular-nums text-gray-800 dark:text-dark-100">{{ route.traffic_success_rate == null ? '—' : `${route.traffic_success_rate.toFixed(1)}%` }}</dd></div>
          <div class="flex min-w-0 items-center justify-between gap-2 py-1"><dt class="text-xs text-gray-500 dark:text-dark-400">{{ text('requestP95') }}</dt><dd class="shrink-0 text-sm font-medium tabular-nums text-gray-800 dark:text-dark-100">{{ route.traffic_p95_latency_ms == null ? '—' : `${route.traffic_p95_latency_ms} ms` }}</dd></div>
        </dl>
        <p v-if="route.hybrid_health?.phase === 'recovering'" class="mt-3 text-sm text-amber-700 dark:text-amber-300"><span v-if="route.hybrid_health.last_probe_status">{{ text('recoveryProgress', { count: route.hybrid_health.recovery_successes, total: route.hybrid_health.policy.recovery_successes }) }}</span><span v-if="route.hybrid_health.next_probe_at" class="ml-2 text-xs">{{ text(route.hybrid_health.last_probe_status ? 'nextRecoveryProbe' : 'firstRecoveryProbe', { time: formatDateTime(route.hybrid_health.next_probe_at) }) }}</span></p>
        <Sub2APIProviderRouteTimeline class="mt-3" :route="route" />
        <div v-if="expanded === route.id" class="mt-3 rounded-lg border border-gray-100 p-3 dark:border-dark-700">
          <p v-if="route.hybrid_health?.phase === 'recovering' && route.hybrid_health.reason" class="break-words text-sm text-amber-700 dark:text-amber-300">{{ route.hybrid_health.reason }}</p>
          <div v-if="route.hybrid_health" class="min-w-0 space-y-2">
            <div class="flex flex-wrap items-center justify-between gap-2">
              <h4 class="text-sm font-semibold text-gray-900 dark:text-white">{{ text('accountPolicy') }}</h4>
              <span v-if="draftPolicies?.[route.account_id]" class="text-xs text-amber-700 dark:text-amber-300">{{ text('unsaved') }}</span>
            </div>
            <label class="flex min-h-11 items-center gap-2 text-sm text-gray-800 dark:text-dark-100"><input type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600" :checked="policyFor(route).enabled" @change="updatePolicy(route, 'enabled', ($event.target as HTMLInputElement).checked)" />{{ text('hybridEnabled') }}</label>
            <div class="grid grid-cols-1 gap-x-3 gap-y-2 sm:grid-cols-2 lg:grid-cols-3">
              <label v-for="field in policyFields" :key="field.key" class="min-w-0 text-xs leading-5 text-gray-600 dark:text-dark-300">
                <span>{{ text(field.label) }}</span>
                <input type="number" class="input mt-1 min-h-11 w-full px-2.5 py-2 text-sm tabular-nums" :min="field.min" :max="field.max" :disabled="!policyFor(route).enabled" :value="policyFor(route)[field.key]" @input="updatePolicy(route, field.key, Number(($event.target as HTMLInputElement).value))" />
              </label>
            </div>
            <details class="text-xs text-gray-500 dark:text-dark-400">
              <summary class="cursor-pointer py-2 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500">{{ text('policyDetails') }}</summary>
              <p class="mt-1 leading-6">{{ text('hybridPolicyHint') }}</p>
            </details>
          </div>
        </div>
      </article>
    </div>
    <p v-if="!routes.length" class="py-4 text-center text-sm text-gray-500 dark:text-dark-400">{{ text('empty') }}</p>
  </section>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ProviderAccountHealthPolicy, Sub2APIProviderProbeTargetHealth } from '@/api/admin/sub2apiProviders'
import { formatDateTime } from '@/utils/format'
import Icon from '@/components/icons/Icon.vue'
import Sub2APIProviderRouteTimeline from './Sub2APIProviderRouteTimeline.vue'
const props = defineProps<{ routes: Sub2APIProviderProbeTargetHealth[]; draftPolicies?: Record<number, ProviderAccountHealthPolicy> }>()
const emit = defineEmits<{ 'update-policy': [accountID: number, policy: ProviderAccountHealthPolicy] }>()
const { t } = useI18n()
const text = (key: string, values: Record<string, string | number> = {}) => t(`admin.sub2apiProviders.health.routes.${key}`, values)
const expanded = ref<number | null>(null)
const toggle = (id: number) => { expanded.value = expanded.value === id ? null : id }
const statusClass = (status: string) => ({ healthy: 'text-green-700 dark:text-green-400', degraded: 'text-amber-700 dark:text-amber-300', unhealthy: 'text-red-700 dark:text-red-300' }[status] ?? 'text-gray-500 dark:text-dark-400')
const policyFields: { key: Exclude<keyof ProviderAccountHealthPolicy, 'enabled'>; label: string; min: number; max: number }[] = [
  { key: 'failure_threshold', label: 'accountFailureThreshold', min: 1, max: 100 },
  { key: 'account_failure_threshold', label: 'accountRequestFailureThreshold', min: 1, max: 10000 },
  { key: 'session_avoid_seconds', label: 'accountSessionAvoid', min: 30, max: 86400 },
  { key: 'recovery_delay_seconds', label: 'recoveryDelay', min: 30, max: 86400 },
  { key: 'timeout_ms', label: 'accountTimeout', min: 100, max: 600000 },
  { key: 'degraded_latency_ms', label: 'accountDegradedLatency', min: 100, max: 600000 },
  { key: 'window_seconds', label: 'accountWindow', min: 60, max: 86400 },
  { key: 'minimum_samples', label: 'accountMinimumSamples', min: 1, max: 10000 },
  { key: 'probe_interval_seconds', label: 'recoveryProbeInterval', min: 30, max: 86400 },
  { key: 'probe_timeout_seconds', label: 'recoveryProbeTimeout', min: 3, max: 120 },
  { key: 'recovery_successes', label: 'recoveryRequiredSuccesses', min: 1, max: 20 },
]
const policyFor = (route: Sub2APIProviderProbeTargetHealth) => props.draftPolicies?.[route.account_id] ?? route.hybrid_health!.policy
const updatePolicy = (route: Sub2APIProviderProbeTargetHealth, key: keyof ProviderAccountHealthPolicy, value: number | boolean) => emit('update-policy', route.account_id, { ...policyFor(route), [key]: value })
</script>

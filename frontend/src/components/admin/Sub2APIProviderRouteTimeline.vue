<template>
  <div class="min-w-0" :data-test="`route-timeline-${route.id}`">
    <div v-if="!compact" class="mb-1 flex min-w-0 items-center justify-between gap-3 text-xs text-gray-500 dark:text-dark-300">
      <span class="min-w-0 truncate">
        {{ t('admin.sub2apiProviders.health.routes.probeHistory') }}
        <span class="ml-1 text-gray-300 dark:text-dark-400">·</span>
        <span class="ml-1 tabular-nums">{{ t('admin.sub2apiProviders.health.routes.slowThreshold', { count: route.degraded_latency_ms }) }}</span>
      </span>
      <span v-if="displayBuckets.length" class="flex-shrink-0 tabular-nums">
        {{ t('admin.sub2apiProviders.health.routes.recentProbeResults', { count: displayBuckets.length }) }}
      </span>
      <span v-else class="flex-shrink-0">
        {{ route.status === 'disabled' ? t('admin.sub2apiProviders.health.status.disabled') : t('admin.sub2apiProviders.health.routes.noProbeResults') }}
      </span>
    </div>
    <div
      v-if="displayBuckets.length"
      class="rounded-[2px] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 focus-visible:ring-offset-1 dark:focus-visible:ring-offset-dark-800"
      :aria-label="activeBucketAriaLabel"
      role="img"
      tabindex="0"
      @keydown.left.prevent="moveSelection(-1)"
      @keydown.right.prevent="moveSelection(1)"
      @keydown.home.prevent="selectBucket(0)"
      @keydown.end.prevent="selectBucket(displayBuckets.length - 1)"
    >
      <div class="route-probe-timeline grid h-3 min-w-0 gap-px overflow-hidden rounded-[2px] border border-gray-100 dark:border-dark-700" :style="timelineGridStyle" aria-hidden="true">
        <span
          v-for="(bucket, index) in displayBuckets"
          :key="`${route.id}-${bucket.started_at || index}`"
          class="route-probe-timeline__bucket min-w-0 cursor-crosshair transition-[filter,opacity,transform] duration-150 motion-reduce:transition-none"
          :class="[
            statusBucketClass(bucket.status),
            index === selectedIndex ? 'scale-y-125 saturate-150' : 'opacity-80 hover:opacity-100',
          ]"
          :style="index === 0 ? firstBucketGridStyle : undefined"
          :title="bucketTitle(bucket)"
          data-test="route-timeline-bucket"
          @pointerenter="selectBucket(index)"
          @click="selectBucket(index)"
        />
      </div>
    </div>
    <div
      v-else
      class="flex min-h-7 items-center justify-center rounded-[2px] border border-dashed border-gray-200 px-3 text-xs text-gray-500 dark:border-dark-600 dark:text-dark-400"
      data-test="route-timeline-empty"
    >
      {{ route.status === 'disabled' ? t('admin.sub2apiProviders.health.status.disabled') : t('admin.sub2apiProviders.health.routes.noProbeResults') }}
    </div>

    <div v-if="!compact && activeBucket" class="mt-1 flex min-h-5 min-w-0 items-center gap-2 text-xs text-gray-500 dark:text-dark-300" aria-live="polite" data-test="route-timeline-detail">
      <span class="inline-flex flex-shrink-0 items-center gap-1 font-medium" :class="statusTextClass(activeBucket.status)">
        <span class="h-1.5 w-1.5 rounded-full" :class="statusDotClass(activeBucket.status)"></span>
        {{ bucketStatusLabel(activeBucket) }}
      </span>
      <span class="flex-shrink-0 tabular-nums">{{ formatProbeTime(activeBucket) }}</span>
      <span class="min-w-0 truncate font-medium tabular-nums text-gray-600 dark:text-dark-300">
        {{ activeBucket.max_health_latency_ms != null ? `${activeBucket.max_health_latency_ms} ms` : t('admin.sub2apiProviders.health.routes.noLatency') }}
      </span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ProviderHealthStatus, Sub2APIProviderHealthBucket, Sub2APIProviderProbeTargetHealth } from '@/api/admin/sub2apiProviders'

const props = withDefaults(defineProps<{
  route: Sub2APIProviderProbeTargetHealth
  compact?: boolean
}>(), {
  compact: false,
})

const { t, locale } = useI18n()
const selectedIndex = ref(0)

const timelineCapacity = 60
// sample_count=0 existed in the old fixed-slot API. Filter it defensively so a
// rolling frontend/backend deployment can never render a synthetic status.
const displayBuckets = computed<Sub2APIProviderHealthBucket[]>(() => (props.route.buckets ?? [])
  .filter(bucket => bucket.sample_count > 0)
  .slice(-timelineCapacity))
const timelineGridStyle = { gridTemplateColumns: `repeat(${timelineCapacity}, minmax(1px, 1fr))` }
const firstBucketGridStyle = computed(() => ({ gridColumnStart: timelineCapacity - displayBuckets.value.length + 1 }))
const activeBucket = computed<Sub2APIProviderHealthBucket | null>(() => displayBuckets.value[selectedIndex.value] ?? null)

const statusBucketClass = (status: string) => ({
  healthy: 'bg-green-500 dark:bg-green-400',
  degraded: 'bg-amber-400',
  unhealthy: 'bg-red-500',
  unknown: 'bg-gray-200 dark:bg-dark-600',
}[status] || 'bg-gray-200 dark:bg-dark-600')

const statusTextClass = (status: ProviderHealthStatus) => ({
  healthy: 'text-green-600 dark:text-green-400',
  degraded: 'text-amber-600 dark:text-amber-400',
  unhealthy: 'text-red-600 dark:text-red-400',
  unknown: 'text-gray-500 dark:text-dark-400',
}[status])

const statusDotClass = (status: ProviderHealthStatus) => ({
  healthy: 'bg-green-500',
  degraded: 'bg-amber-400',
  unhealthy: 'bg-red-500',
  unknown: 'border border-gray-300 bg-transparent dark:border-dark-500',
}[status])

watch(displayBuckets, (buckets) => {
  selectedIndex.value = Math.max(0, buckets.length - 1)
}, { immediate: true })

const selectBucket = (index: number) => {
  if (!displayBuckets.value.length) return
  selectedIndex.value = Math.max(0, Math.min(displayBuckets.value.length - 1, index))
}

const moveSelection = (offset: number) => selectBucket(selectedIndex.value + offset)

const formatProbeTime = (bucket: Sub2APIProviderHealthBucket) => {
  const endedAt = bucket.ended_at ? new Date(bucket.ended_at) : null
  const startedAt = bucket.started_at ? new Date(bucket.started_at) : null
  const probeTime = endedAt && Number.isFinite(endedAt.getTime())
    ? endedAt
    : startedAt && Number.isFinite(startedAt.getTime())
      ? startedAt
      : null

  if (!probeTime) return t('admin.sub2apiProviders.health.neverChecked')

  return new Intl.DateTimeFormat(locale.value, {
    month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit',
  }).format(probeTime)
}

const bucketStatusLabel = (bucket: Sub2APIProviderHealthBucket) => t(`admin.sub2apiProviders.health.status.${bucket.status}`)

const bucketTitle = (bucket: Sub2APIProviderHealthBucket) => {
  const parts = [
    formatProbeTime(bucket),
    bucketStatusLabel(bucket),
  ]
  if (bucket.max_health_latency_ms != null) parts.push(`${bucket.max_health_latency_ms} ms`)
  return parts.join(' · ')
}

const activeBucketAriaLabel = computed(() => {
  if (!activeBucket.value) {
    return [props.route.account_name, props.route.test_model, t('admin.sub2apiProviders.health.routes.noProbeResults')].filter(Boolean).join(' · ')
  }
  return [
    props.route.account_name,
    props.route.test_model,
    t('admin.sub2apiProviders.health.routes.recentProbeResults', { count: displayBuckets.value.length }),
    bucketTitle(activeBucket.value),
  ].filter(Boolean).join(' · ')
})
</script>

<style scoped>
.route-probe-timeline__bucket {
  border-radius: 1px;
}
</style>

<template>
  <section class="min-w-0" data-test="provider-health-timeline">
    <div class="flex flex-wrap items-center justify-between gap-x-3 gap-y-1">
      <span class="text-[11px] font-medium text-gray-600 dark:text-dark-300">
        {{ t('admin.sub2apiProviders.health.platformTimeline24h') }}
      </span>
      <div class="flex flex-wrap items-center justify-end gap-x-2 text-[10px] tabular-nums text-gray-500 dark:text-dark-400">
        <span><span class="mr-1 inline-block h-1.5 w-1.5 rounded-full bg-green-500"></span>{{ summary.healthy }} {{ t('admin.sub2apiProviders.health.bucketShort.healthy') }}</span>
        <span><span class="mr-1 inline-block h-1.5 w-1.5 rounded-sm bg-amber-400"></span>{{ summary.degraded }} {{ t('admin.sub2apiProviders.health.bucketShort.degraded') }}</span>
        <span><span class="mr-1 inline-block h-1.5 w-1.5 rounded-sm bg-red-500"></span>{{ summary.unhealthy }} {{ t('admin.sub2apiProviders.health.bucketShort.unhealthy') }}</span>
        <span><span class="mr-1 inline-block h-1.5 w-1.5 rounded-sm border border-gray-300 dark:border-dark-500"></span>{{ summary.unknown }} {{ t('admin.sub2apiProviders.health.bucketShort.unknown') }}</span>
      </div>
    </div>

    <div class="mt-1.5 grid grid-cols-3 gap-1.5" data-test="timeline-metrics">
      <div class="min-w-0 rounded-sm border border-gray-100 bg-gray-50/70 px-1.5 py-1 dark:border-dark-700 dark:bg-dark-900/40">
        <span class="block truncate text-[9px] uppercase tracking-wide text-gray-400 dark:text-dark-500">
          {{ t('admin.sub2apiProviders.health.knownPeriods') }}
        </span>
        <strong class="mt-0.5 block text-xs font-semibold tabular-nums text-gray-800 dark:text-dark-100">
          {{ knownBucketCount }}/60
        </strong>
      </div>
      <div class="min-w-0 rounded-sm border border-gray-100 bg-gray-50/70 px-1.5 py-1 dark:border-dark-700 dark:bg-dark-900/40">
        <span class="block truncate text-[9px] uppercase tracking-wide text-gray-400 dark:text-dark-500">
          {{ t('admin.sub2apiProviders.health.normalRate') }}
        </span>
        <strong class="mt-0.5 block text-xs font-semibold tabular-nums text-gray-800 dark:text-dark-100">
          {{ normalRate }}
        </strong>
      </div>
      <div class="min-w-0 rounded-sm border border-gray-100 bg-gray-50/70 px-1.5 py-1 dark:border-dark-700 dark:bg-dark-900/40">
        <span class="block truncate text-[9px] uppercase tracking-wide text-gray-400 dark:text-dark-500">
          {{ t('admin.sub2apiProviders.health.sampleTotal') }}
        </span>
        <strong class="mt-0.5 block text-xs font-semibold tabular-nums text-gray-800 dark:text-dark-100">
          {{ totalSampleCount }}
        </strong>
      </div>
    </div>

    <div
      class="mt-2 rounded-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 focus-visible:ring-offset-2 dark:focus-visible:ring-offset-dark-800"
      role="img"
      tabindex="0"
      :aria-label="timelineAriaLabel"
      data-test="timeline-focus-target"
      @keydown.left.prevent="moveSelection(-1)"
      @keydown.right.prevent="moveSelection(1)"
      @keydown.home.prevent="selectBucket(0)"
      @keydown.end.prevent="selectBucket(displayBuckets.length - 1)"
    >
      <div class="grid h-4 w-full gap-px" style="grid-template-columns: repeat(60, minmax(1px, 1fr))" aria-hidden="true">
        <span
          v-for="(bucket, index) in displayBuckets"
          :key="`${bucket.started_at}-${index}`"
          data-test="timeline-bucket"
          class="timeline-bucket h-4 min-w-0 cursor-crosshair rounded-[1px] border transition-[filter,opacity,transform] duration-150 ease-out"
          :class="[
            bucketClass(bucket.status),
            index === selectedIndex ? 'scale-y-125 saturate-150' : 'opacity-80 hover:opacity-100',
            index === displayBuckets.length - 1 && bucket.status !== 'unknown' ? 'timeline-bucket-current' : '',
          ]"
          :title="bucketTitle(bucket)"
          @pointerenter="selectBucket(index)"
          @click="selectBucket(index)"
        ></span>
      </div>
    </div>

    <div class="mt-1 flex items-center justify-between text-[10px] text-gray-400 dark:text-dark-500" aria-hidden="true">
      <span>{{ t('admin.sub2apiProviders.health.hoursAgo24') }}</span>
      <span>{{ t('admin.sub2apiProviders.health.now') }}</span>
    </div>

    <div class="mt-1.5 flex min-h-5 min-w-0 items-center gap-2 text-[11px] text-gray-500 dark:text-dark-400" aria-live="polite" data-test="timeline-detail">
      <span class="inline-flex flex-shrink-0 items-center gap-1 font-medium" :class="statusTextClass(activeBucket.status)">
        <span class="h-1.5 w-1.5 rounded-full" :class="statusDotClass(activeBucket.status)"></span>
        {{ t(`admin.sub2apiProviders.health.status.${activeBucket.status}`) }}
      </span>
      <span class="flex-shrink-0 tabular-nums">{{ formatBucketRange(activeBucket) }}</span>
      <span v-if="activeBucket.sample_count > 0" class="flex-shrink-0">
        {{ t('admin.sub2apiProviders.health.sampleCount', { count: activeBucket.sample_count }) }}
      </span>
      <span v-if="activeBucket.max_health_latency_ms != null" class="flex-shrink-0 tabular-nums">
        {{ activeBucket.max_health_latency_ms }} ms
      </span>
      <span v-if="activeBucket.last_error" class="min-w-0 truncate text-amber-700 dark:text-amber-400">
        {{ t('admin.sub2apiProviders.health.anomalyRecorded') }}
      </span>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type {
  ProviderHealthStatus,
  Sub2APIProviderHealthBucket,
  Sub2APIProviderHealthOverview,
} from '@/api/admin/sub2apiProviders'

const props = defineProps<{
  overview?: Sub2APIProviderHealthOverview | null
}>()

const { t, locale } = useI18n()
const selectedIndex = ref(59)

const fallbackBuckets = computed<Sub2APIProviderHealthBucket[]>(() => {
  const endedAt = new Date()
  const startedAt = new Date(endedAt.getTime() - 24 * 60 * 60 * 1000)
  return Array.from({ length: 60 }, (_, index) => ({
    started_at: new Date(startedAt.getTime() + index * 24 * 60 * 1000).toISOString(),
    ended_at: new Date(startedAt.getTime() + (index + 1) * 24 * 60 * 1000).toISOString(),
    status: 'unknown',
    sample_count: 0,
    healthy_samples: 0,
    degraded_samples: 0,
    unhealthy_samples: 0,
  }))
})

const displayBuckets = computed(() => props.overview?.buckets?.length === 60
  ? props.overview.buckets
  : fallbackBuckets.value)

const summary = computed(() => props.overview?.summary ?? {
  healthy: 0,
  degraded: 0,
  unhealthy: 0,
  unknown: 60,
})
const knownBucketCount = computed(() => Math.max(0, displayBuckets.value.length - summary.value.unknown))
const normalRate = computed(() => {
  if (knownBucketCount.value === 0) return '—'
  return `${Math.round((summary.value.healthy / knownBucketCount.value) * 100)}%`
})
const totalSampleCount = computed(() => displayBuckets.value.reduce((total, bucket) => total + bucket.sample_count, 0))

const activeBucket = computed(() => displayBuckets.value[selectedIndex.value] ?? displayBuckets.value[59])
const timelineAriaLabel = computed(() => t('admin.sub2apiProviders.health.timelineAriaLabel', {
  healthy: summary.value.healthy,
  degraded: summary.value.degraded,
  unhealthy: summary.value.unhealthy,
  unknown: summary.value.unknown,
}))

watch(() => props.overview, () => {
  let latestSampleIndex = -1
  for (let index = displayBuckets.value.length - 1; index >= 0; index--) {
    if (displayBuckets.value[index].sample_count > 0) {
      latestSampleIndex = index
      break
    }
  }
  selectedIndex.value = latestSampleIndex >= 0 ? latestSampleIndex : displayBuckets.value.length - 1
}, { immediate: true })

function selectBucket(index: number) {
  selectedIndex.value = Math.max(0, Math.min(displayBuckets.value.length - 1, index))
}

function moveSelection(offset: number) {
  selectBucket(selectedIndex.value + offset)
}

function bucketClass(status: ProviderHealthStatus): string {
  return {
    healthy: 'border-green-500 bg-green-500 dark:border-green-400 dark:bg-green-400',
    degraded: 'border-amber-500 bg-amber-400 dark:border-amber-400 dark:bg-amber-400',
    unhealthy: 'border-red-600 bg-red-500 dark:border-red-500 dark:bg-red-500',
    unknown: 'border-gray-300 bg-transparent dark:border-dark-500',
  }[status]
}

function statusTextClass(status: ProviderHealthStatus): string {
  return {
    healthy: 'text-green-700 dark:text-green-400',
    degraded: 'text-amber-700 dark:text-amber-400',
    unhealthy: 'text-red-700 dark:text-red-400',
    unknown: 'text-gray-500 dark:text-dark-400',
  }[status]
}

function statusDotClass(status: ProviderHealthStatus): string {
  return {
    healthy: 'bg-green-500',
    degraded: 'rounded-sm bg-amber-400',
    unhealthy: 'rounded-sm bg-red-500',
    unknown: 'border border-gray-300 bg-transparent dark:border-dark-500',
  }[status]
}

function formatBucketRange(bucket: Sub2APIProviderHealthBucket): string {
  const formatter = new Intl.DateTimeFormat(locale.value, { hour: '2-digit', minute: '2-digit' })
  return `${formatter.format(new Date(bucket.started_at))}-${formatter.format(new Date(bucket.ended_at))}`
}

function bucketTitle(bucket: Sub2APIProviderHealthBucket): string {
  const details = [
    formatBucketRange(bucket),
    t(`admin.sub2apiProviders.health.status.${bucket.status}`),
    t('admin.sub2apiProviders.health.sampleCount', { count: bucket.sample_count }),
  ]
  if (bucket.max_health_latency_ms != null) details.push(`${bucket.max_health_latency_ms} ms`)
  if (bucket.last_error) details.push(t('admin.sub2apiProviders.health.anomalyRecorded'))
  return details.join(' · ')
}
</script>

<style scoped>
@keyframes provider-timeline-current {
  0%,
  100% {
    filter: brightness(1);
  }

  50% {
    filter: brightness(1.16);
  }
}

.timeline-bucket-current {
  animation: provider-timeline-current 2.4s ease-in-out infinite;
}

@media (prefers-reduced-motion: reduce) {
  .timeline-bucket,
  .timeline-bucket-current {
    animation: none;
    transition: none;
  }
}
</style>

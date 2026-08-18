<template>
  <AppLayout>
    <div class="mx-auto w-full max-w-[1500px] space-y-6 pb-8">
      <header class="flex flex-col gap-4 border-b border-gray-200 pb-5 dark:border-dark-700 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <div class="flex items-center gap-2">
            <span class="h-2.5 w-2.5 rounded-full bg-cyan-500 shadow-[0_0_0_4px_rgba(6,182,212,0.12)]" aria-hidden="true"></span>
            <h1 class="text-2xl font-semibold tracking-tight text-gray-950 dark:text-white">{{ t('codexRadar.title') }}</h1>
          </div>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('codexRadar.subtitle') }}</p>
        </div>
        <div class="flex items-center gap-3">
          <div class="text-right text-xs text-gray-500 dark:text-gray-400">
            <div>{{ t('codexRadar.hourly') }}</div>
            <div v-if="fetchedAtLabel" class="mt-0.5">{{ t('codexRadar.updatedAt', { time: fetchedAtLabel }) }}</div>
          </div>
          <button type="button" class="btn btn-secondary btn-sm" :disabled="loading" @click="reload">
            <Icon name="refresh" size="sm" class="mr-1.5" :class="loading ? 'animate-spin' : ''" />
            {{ t('codexRadar.refresh') }}
          </button>
        </div>
      </header>

      <div v-if="loading && !summary" class="flex min-h-64 items-center justify-center"><LoadingSpinner /></div>
      <div v-else-if="loadFailed && !summary" class="rounded-xl border border-red-200 bg-red-50 p-8 text-center dark:border-red-900/60 dark:bg-red-950/20">
        <Icon name="exclamationTriangle" size="lg" class="mx-auto text-red-500" />
        <p class="mt-3 text-sm text-red-700 dark:text-red-300">{{ t('codexRadar.loadError') }}</p>
        <button type="button" class="btn btn-secondary btn-sm mt-4" @click="reload">{{ t('common.refresh') }}</button>
      </div>
      <div v-else-if="!available" class="rounded-xl border border-gray-200 bg-gray-50 p-10 text-center dark:border-dark-700 dark:bg-dark-900/40">
        <Icon name="chart" size="lg" class="mx-auto text-gray-400" />
        <p class="mt-3 text-sm text-gray-500 dark:text-gray-400">{{ t('codexRadar.unavailable') }}</p>
      </div>
      <template v-else>
        <section aria-labelledby="recommendations-heading">
          <div class="mb-3 flex items-center justify-between gap-3">
            <div>
              <h2 id="recommendations-heading" class="text-base font-semibold text-gray-900 dark:text-white">{{ t('codexRadar.recommendations') }}</h2>
              <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">{{ recommendationMeta }}</p>
            </div>
            <a :href="sourceUrl" target="_blank" rel="noopener noreferrer" class="inline-flex items-center gap-1 text-xs text-primary-600 hover:underline dark:text-primary-400">
              {{ t('codexRadar.sourceLink') }} <Icon name="externalLink" size="xs" />
            </a>
          </div>
          <div class="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-4">
            <article v-for="group in recommendationGroups" :key="group.key || group.title" class="min-w-0 rounded-xl border border-gray-200 bg-white p-4 shadow-sm dark:border-dark-700 dark:bg-dark-800/70">
              <div class="flex items-start justify-between gap-3">
                <h3 class="truncate text-sm font-semibold text-gray-900 dark:text-white">{{ group.title || group.key }}</h3>
                <span class="shrink-0 rounded-full bg-amber-50 px-2 py-0.5 text-[11px] font-medium text-amber-700 dark:bg-amber-900/30 dark:text-amber-300">{{ group.items?.length || 0 }}</span>
              </div>
              <div v-if="group.items?.length" class="mt-3 space-y-2">
                <div v-for="item in group.items.slice(0, 3)" :key="`${item.model}-${item.effort}`" class="rounded-lg bg-gray-50 p-3 dark:bg-dark-900/60">
                  <div class="flex items-center justify-between gap-2">
                    <span class="truncate text-xs font-semibold text-gray-800 dark:text-gray-100">{{ item.model || '-' }}</span>
                    <span class="rounded bg-gray-200 px-1.5 py-0.5 text-[10px] uppercase text-gray-600 dark:bg-dark-700 dark:text-gray-300">{{ item.effort || '-' }}</span>
                  </div>
                  <div class="mt-2 grid grid-cols-3 gap-2 text-[11px]">
                    <Metric :label="t('codexRadar.iq')" :value="formatNumber(item.iq)" />
                    <Metric :label="t('codexRadar.duration')" :value="formatMinutes(item.average_duration_minutes)" />
                    <Metric :label="t('codexRadar.cost')" :value="formatMoney(item.average_cost_usd)" />
                  </div>
                </div>
              </div>
              <p v-else class="mt-4 text-xs text-gray-400">{{ t('codexRadar.noData') }}</p>
            </article>
          </div>
        </section>

        <section aria-labelledby="intelligence-heading">
          <div class="mb-3 flex items-end justify-between gap-3">
            <div>
              <h2 id="intelligence-heading" class="text-base font-semibold text-gray-900 dark:text-white">{{ t('codexRadar.intelligence') }}</h2>
              <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">{{ intelligencePoints.length }} {{ t('codexRadar.samples') }}</p>
            </div>
          </div>
          <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-4">
            <article v-for="point in intelligencePoints" :key="`${point.model}-${point.effort}`" class="rounded-xl border border-gray-200 bg-white p-4 shadow-sm dark:border-dark-700 dark:bg-dark-800/70">
              <div class="flex items-center justify-between gap-2">
                <h3 class="truncate text-sm font-semibold text-gray-900 dark:text-white">{{ point.model || '-' }}</h3>
                <span class="rounded-md bg-cyan-50 px-2 py-0.5 text-[10px] font-semibold uppercase text-cyan-700 dark:bg-cyan-900/30 dark:text-cyan-300">{{ point.effort || '-' }}</span>
              </div>
              <div class="mt-4 grid grid-cols-2 gap-x-4 gap-y-3">
                <Metric :label="t('codexRadar.iq')" :value="formatNumber(point.iq)" large />
                <Metric :label="t('codexRadar.passed')" :value="formatPassRate(point)" large />
                <Metric :label="t('codexRadar.duration')" :value="formatMinutes(point.average_minutes)" />
                <Metric :label="t('codexRadar.cost')" :value="formatMoney(point.average_price_usd)" />
                <Metric :label="t('codexRadar.samples24h')" :value="formatNumber(point.runs_24h, 0)" />
                <Metric :label="t('codexRadar.samples')" :value="formatNumber(point.runs_total, 0)" />
              </div>
            </article>
          </div>
        </section>
      </template>

      <footer class="flex flex-wrap items-center justify-between gap-2 border-t border-gray-200 pt-4 text-xs text-gray-500 dark:border-dark-700 dark:text-gray-400">
        <span>{{ attribution }}</span>
        <a :href="sourceUrl" target="_blank" rel="noopener noreferrer" class="inline-flex items-center gap-1 hover:text-primary-600 dark:hover:text-primary-400">{{ t('codexRadar.sourceLink') }} <Icon name="externalLink" size="xs" /></a>
      </footer>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import { useAppStore } from '@/stores/app'
import { getCodexRadarSummary, type CodexRadarData, type CodexRadarIntelligencePoint, type CodexRadarRecommendationGroup, type CodexRadarSummary } from '@/api/codexradar'

const Metric = defineComponent({
  props: {
    label: { type: String, default: '' },
    value: { type: String, default: '-' },
    large: { type: Boolean, default: false },
  },
  setup(props) {
    return () => h('div', { class: 'min-w-0' }, [
      h('div', { class: 'truncate text-[10px] uppercase tracking-wide text-gray-400 dark:text-gray-500' }, props.label),
      h('div', {
        class: [
          'mt-0.5 truncate font-semibold text-gray-800 dark:text-gray-100',
          props.large ? 'text-base' : 'text-xs',
        ],
      }, props.value || '-'),
    ])
  },
})

const { t } = useI18n()
const appStore = useAppStore()
const summary = ref<CodexRadarSummary | null>(null)
const loading = ref(false)
const loadFailed = ref(false)
let refreshTimer: ReturnType<typeof setInterval> | undefined

const data = computed<CodexRadarData>(() => (summary.value?.data || {}) as CodexRadarData)
const recommendationGroups = computed<CodexRadarRecommendationGroup[]>(() => data.value.recommendations?.recommendations || [])
const intelligencePoints = computed<CodexRadarIntelligencePoint[]>(() => data.value.intelligence?.points || [])
const available = computed(() => Boolean(summary.value?.available && (recommendationGroups.value.length || intelligencePoints.value.length)))
const sourceUrl = computed(() => summary.value?.source || 'https://codexradar.com/')
const attribution = computed(() => summary.value?.attribution || t('codexRadar.source'))
const fetchedAtLabel = computed(() => {
  const raw = summary.value?.fetched_at
  if (!raw) return ''
  const date = new Date(raw)
  return Number.isNaN(date.getTime()) ? '' : date.toLocaleString()
})
const recommendationMeta = computed(() => {
  const raw = data.value.recommendations as Record<string, unknown> | undefined
  const timestamp = typeof raw?.source_updated_at === 'string' ? raw.source_updated_at : ''
  return timestamp ? `${t('codexRadar.updatedAt', { time: new Date(timestamp).toLocaleString() })}` : t('codexRadar.hourly')
})

function formatNumber(value: unknown, digits = 2): string {
  return typeof value === 'number' && Number.isFinite(value) ? value.toFixed(digits) : '-'
}
function formatMoney(value: unknown): string {
  return typeof value === 'number' && Number.isFinite(value) ? `$${value.toFixed(2)}` : '-'
}
function formatMinutes(value: unknown): string {
  return typeof value === 'number' && Number.isFinite(value) ? `${value.toFixed(1)}m` : '-'
}
function formatPassRate(point: CodexRadarIntelligencePoint): string {
  if (typeof point.passed === 'number' && typeof point.total === 'number' && point.total > 0) return `${((point.passed / point.total) * 100).toFixed(1)}%`
  return '-'
}

async function reload(): Promise<void> {
  loading.value = true
  loadFailed.value = false
  try {
    summary.value = await getCodexRadarSummary()
  } catch (error) {
    loadFailed.value = true
    appStore.showError(t('codexRadar.loadError'))
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  void reload()
  refreshTimer = setInterval(() => void reload(), 60 * 60 * 1000)
})
onBeforeUnmount(() => {
  if (refreshTimer) clearInterval(refreshTimer)
})
</script>

<template>
  <AppLayout>
    <section class="radar-page" :aria-busy="loading">
      <div class="radar-shell">
        <header class="radar-header">
          <div>
            <p class="eyebrow">公开基准 · 150 分制</p>
            <h1>雷达分 <span>· RadarBench</span></h1>
            <p class="subtitle">64 道题覆盖软件工程、视觉空间、终端操作和科学研究</p>
          </div>
          <div class="toolbar">
            <span v-if="data" class="sync-status">
              <span class="status-dot" aria-hidden="true"></span>
              {{ data.fresh }}/{{ data.expected }} 档 · {{ updatedAtLabel }}
            </span>
            <button class="toolbar-button" type="button" :disabled="loading" @click="reload">
              <span aria-hidden="true">↻</span>{{ loading ? '读取中…' : '刷新' }}
            </button>
            <a class="toolbar-button source-button" href="https://codexradar.com/" target="_blank" rel="noopener noreferrer">
              查看原站 <span aria-hidden="true">↗</span>
            </a>
          </div>
        </header>

        <nav class="dimension-tabs" aria-label="评分维度">
          <button
            v-for="tab in tabs"
            :key="tab.key"
            type="button"
            :aria-pressed="selected === tab.key"
            :class="{ selected: selected === tab.key }"
            @click="selected = tab.key"
          >
            <span class="tab-mark" aria-hidden="true"></span>{{ tab.label }}
          </button>
        </nav>

        <p class="legend">
          <span class="legend-bar" aria-hidden="true"></span>
          分数越高代表已判分题目的完成度越高；费用与耗时只显示总分的 API 等价统计。
        </p>

        <p v-if="error" class="message error-message" role="alert">
          数据读取失败，{{ data ? '已保留上次显示的数据。' : '请稍后重试。' }}
        </p>
        <p v-if="!data" class="message empty-message">
          {{ loading ? '正在读取原站模型评分…' : '原站数据暂不可用，请稍后刷新。' }}
        </p>

        <div v-else class="radar-matrix" aria-label="模型推理档位评分矩阵">
          <div class="matrix-heading" aria-hidden="true">
            <span>模型 / 推理档位</span>
            <span v-for="effort in efforts" :key="effort">{{ effort.toUpperCase() }}</span>
          </div>

          <div v-for="row in rows" :key="row.model" class="model-row">
            <div class="model-name">
              <strong>{{ modelLabel(row.model) }}</strong>
              <span>{{ Object.keys(row.cells).length }}/{{ efforts.length }} 档位</span>
            </div>

            <template v-for="effort in efforts" :key="effort">
              <article
                v-if="row.cells[effort]"
                class="score-card"
                :class="{ stale: isStale(row.cells[effort]), 'no-score': dimension(row.cells[effort]).score == null }"
              >
                <div class="card-topline">
                  <span class="effort-label">{{ effort }}</span>
                  <span class="sample-count">{{ dimension(row.cells[effort]).graded_runs_count ?? '—' }} 题次</span>
                </div>
                <div class="score-line">
                  <strong>{{ number(dimension(row.cells[effort]).score) }}</strong>
                  <div v-if="selected === 'total'" class="metrics">
                    <span>{{ money(metric(row.cells[effort], 'api_equivalent_usd')) }}</span>
                    <span>{{ number(minutes(row.cells[effort])) }} 分钟</span>
                  </div>
                </div>
                <progress :value="scorePercent(dimension(row.cells[effort]).score)" max="100" aria-label="雷达分进度" />
                <div class="card-meta">
                  <span>覆盖 {{ dimension(row.cells[effort]).coverage ?? '—' }}/{{ dimension(row.cells[effort]).required_tasks ?? '—' }}</span>
                  <span v-if="partialMetrics(row.cells[effort]) && selected === 'total'">部分样本</span>
                  <span v-else-if="isStale(row.cells[effort])">待更新</span>
                  <span v-else-if="isProvisional(row.cells[effort])">暂定</span>
                </div>
              </article>
              <div v-else class="missing-card" aria-label="暂无此档位数据">—</div>
            </template>
          </div>
        </div>

        <footer class="radar-footer">
          <span>数据来自 Codex 雷达 codexradar.com · 每小时同步 · 读取时间以卡片数据为准</span>
          <span>分项不提供独立费用与耗时</span>
        </footer>
      </div>
    </section>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import { getCodexRadarSummary } from '@/api/codexradar'
import { parseRadarBench, type BenchDimension, type BenchMetric, type BenchPoint } from '@/utils/radarBench'

const data = ref<ReturnType<typeof parseRadarBench>>(null)
const loading = ref(false)
const error = ref(false)
const selected = ref('total')
const now = ref(Date.now())
let refreshTimer: ReturnType<typeof setInterval> | undefined

const efforts = ['ultra', 'max', 'xhigh', 'high', 'medium', 'low']
const tabs = [
  { key: 'total', label: '总雷达分' },
  { key: 'deepswe', label: '软件工程' },
  { key: 'pompeii', label: '视觉空间推理' },
  { key: 'tb4', label: '终端与系统操作' },
  { key: 'science', label: '科学研究' },
]

const rows = computed(() => {
  const grouped = new Map<string, Record<string, BenchPoint>>()
  for (const point of data.value?.points ?? []) {
    if (!grouped.has(point.model)) grouped.set(point.model, {})
    grouped.get(point.model)![point.effort] = point
  }
  const order = ['gpt-6-astra', 'gpt-6.1-sol', 'gpt-6-sol', 'gpt-5.6-sol', 'gpt-6-luna', 'gpt-5.6-luna']
  return [...grouped]
    .map(([model, cells]) => ({ model, cells }))
    .sort((left, right) => (order.indexOf(left.model) < 0 ? 99 : order.indexOf(left.model)) - (order.indexOf(right.model) < 0 ? 99 : order.indexOf(right.model)))
})

const updatedAtLabel = computed(() => {
  const timestamp = data.value?.points.find(point => point.observed_at && Number.isFinite(Date.parse(point.observed_at)))?.observed_at
  if (!timestamp) return '缓存数据'
  return new Intl.DateTimeFormat('zh-CN', { month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit' }).format(new Date(timestamp))
})

function dimension(point: BenchPoint): BenchDimension {
  return selected.value === 'total' ? point : point.source_scores?.[selected.value] ?? {}
}

function metric(point: BenchPoint, key: string): number | null {
  const value: BenchMetric | undefined = point.statistics?.metrics?.[key]
  return value && value.known_samples > 0 && Number.isFinite(value.known_total) ? value.known_total / value.known_samples : null
}

function minutes(point: BenchPoint): number | null {
  const elapsed = metric(point, 'elapsed_ms')
  return elapsed == null ? null : elapsed / 60000
}

function scorePercent(score: number | null | undefined): number {
  return typeof score === 'number' && Number.isFinite(score) ? Math.max(0, Math.min(100, score / 1.5)) : 0
}

function number(value: number | null | undefined): string {
  return typeof value === 'number' && Number.isFinite(value) ? value.toFixed(1) : '—'
}

function money(value: number | null): string {
  return value == null ? '—' : `$${value.toFixed(2)}`
}

function isStale(point: BenchPoint): boolean {
  const observed = Date.parse(point.observed_at ?? '')
  return point.stale || !Number.isFinite(observed) || now.value - observed >= 3600000
}

function isProvisional(point: BenchPoint): boolean {
  const current = dimension(point)
  return (current.coverage ?? 0) < (current.required_tasks ?? 64)
}

function partialMetrics(point: BenchPoint): boolean {
  return ['api_equivalent_usd', 'elapsed_ms'].some(key => (point.statistics?.metrics?.[key]?.missing_samples ?? 0) > 0)
}

function modelLabel(model: string): string {
  return model.replace(/^gpt-/, 'GPT-').replace(/-(astra|sol|luna)$/, (_, suffix: string) => ` ${suffix[0].toUpperCase()}${suffix.slice(1)}`)
}

async function reload(): Promise<void> {
  if (loading.value) return
  loading.value = true
  error.value = false
  try {
    const summary = await getCodexRadarSummary()
    const next = parseRadarBench(summary.data)
    if (next) data.value = next
    else error.value = true
  } catch {
    error.value = true
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  void reload()
  refreshTimer = setInterval(() => {
    now.value = Date.now()
    void reload()
  }, 60000)
})

onUnmounted(() => clearInterval(refreshTimer))
</script>

<style scoped>
.radar-page {
  @apply -mx-4 min-h-[calc(100vh-4rem)] bg-gray-50 px-4 py-4 text-gray-900 dark:bg-dark-950 dark:text-gray-100 md:-mx-6 lg:-mx-8 lg:px-8;
}

.radar-shell {
  @apply mx-auto max-w-[1680px] rounded-2xl border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-900 sm:p-6;
}

.radar-header {
  @apply flex flex-wrap items-end justify-between gap-4 border-b border-gray-100 pb-4 dark:border-dark-700;
}

.eyebrow {
  @apply mb-1 text-xs font-semibold uppercase tracking-[0.16em] text-teal-600 dark:text-teal-300;
}

h1 {
  @apply text-2xl font-bold tracking-tight sm:text-3xl;
}

h1 span {
  @apply font-medium text-gray-400 dark:text-gray-500;
}

.subtitle {
  @apply mt-2 text-sm text-gray-500 dark:text-gray-400;
}

.toolbar {
  @apply flex flex-wrap items-center justify-end gap-2;
}

.sync-status {
  @apply mr-2 inline-flex items-center gap-2 text-xs font-medium text-gray-500 dark:text-gray-400;
}

.status-dot {
  @apply h-2 w-2 rounded-full bg-emerald-500;
  box-shadow: 0 0 0 3px rgb(16 185 129 / 12%);
}

.toolbar-button {
  @apply inline-flex min-h-9 items-center gap-1.5 rounded-lg border border-gray-200 bg-white px-3 text-sm font-medium text-gray-600 transition-colors hover:border-teal-300 hover:bg-teal-50 hover:text-teal-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-teal-500 disabled:cursor-wait disabled:opacity-50 dark:border-dark-600 dark:bg-dark-900 dark:text-gray-300 dark:hover:border-teal-700 dark:hover:bg-teal-900/30 dark:hover:text-teal-300;
}

.source-button {
  @apply no-underline;
}

.dimension-tabs {
  @apply flex flex-wrap gap-2 py-3;
}

.dimension-tabs button {
  @apply inline-flex min-h-9 items-center gap-2 rounded-lg border border-gray-200 bg-gray-50 px-3 text-sm font-medium text-gray-600 transition-colors hover:border-teal-300 hover:text-teal-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-teal-500 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-300 dark:hover:border-teal-700 dark:hover:text-teal-300;
}

.dimension-tabs button.selected {
  @apply border-teal-500 bg-teal-50 text-teal-700 dark:border-teal-600 dark:bg-teal-900/30 dark:text-teal-300;
}

.tab-mark {
  @apply h-1.5 w-1.5 rounded-full bg-gray-300 dark:bg-dark-500;
}

.selected .tab-mark {
  @apply bg-teal-500;
}

.legend {
  @apply mb-4 flex items-center gap-2 text-xs text-gray-500 dark:text-gray-400;
}

.legend-bar {
  @apply inline-block h-1.5 w-8 rounded-full bg-teal-500;
}

.message {
  @apply rounded-xl border px-4 py-5 text-center text-sm;
}

.error-message {
  @apply border-amber-200 bg-amber-50 text-amber-800 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-300;
}

.empty-message {
  @apply border-dashed border-gray-200 bg-gray-50 text-gray-500 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-400;
}

.radar-matrix {
  @apply w-full;
}

.matrix-heading,
.model-row {
  display: grid;
  grid-template-columns: minmax(132px, 0.72fr) repeat(6, minmax(0, 1fr));
  gap: 8px;
}

.matrix-heading {
  @apply mb-2 px-2 text-center text-[10px] font-semibold uppercase tracking-[0.12em] text-gray-400 dark:text-gray-500;
}

.matrix-heading span:first-child {
  @apply text-left;
}

.model-row {
  @apply mb-2;
}

.model-name {
  @apply flex min-w-0 flex-col justify-center rounded-lg border border-transparent px-2 py-2;
}

.model-name strong {
  @apply truncate text-sm font-semibold text-gray-800 dark:text-gray-100;
}

.model-name span {
  @apply mt-1 text-[10px] text-gray-400 dark:text-gray-500;
}

.score-card,
.missing-card {
  min-width: 0;
  min-height: 88px;
}

.score-card {
  @apply rounded-lg border border-gray-200 border-t-2 border-t-teal-400 bg-gray-50 px-2.5 py-1.5 transition-colors dark:border-dark-600 dark:border-t-teal-500 dark:bg-dark-800;
}

.score-card:hover {
  @apply border-gray-300 bg-white dark:border-dark-500 dark:bg-dark-700;
}

.score-card.stale {
  @apply border-t-amber-400 dark:border-t-amber-500;
}

.score-card.no-score {
  @apply border-t-gray-300 dark:border-t-dark-500;
}

.card-topline,
.card-meta {
  @apply flex items-center justify-between gap-2;
}

.card-topline {
  @apply text-[10px] text-gray-400 dark:text-gray-500;
}

.effort-label {
  @apply font-semibold uppercase tracking-wide text-gray-600 dark:text-gray-300;
}

.sample-count {
  @apply truncate;
}

.score-line {
  @apply mt-1 flex items-end justify-between gap-2;
}

.score-line strong {
  @apply text-[23px] font-bold leading-none tabular-nums text-teal-600 dark:text-teal-300;
}

.no-score .score-line strong {
  @apply text-gray-400 dark:text-gray-500;
}

.metrics {
  @apply text-right text-[10px] leading-4 text-gray-500 dark:text-gray-400;
}

progress {
  @apply block h-1.5 w-full overflow-hidden rounded-full border-0 bg-gray-200 dark:bg-dark-600;
  appearance: none;
}

progress::-webkit-progress-bar {
  @apply rounded-full bg-gray-200 dark:bg-dark-600;
}

progress::-webkit-progress-value {
  @apply rounded-full bg-teal-500;
}

progress::-moz-progress-bar {
  @apply rounded-full bg-teal-500;
}

.card-meta {
  @apply mt-1 text-[10px] text-gray-500 dark:text-gray-400;
}

.card-meta span:last-child {
  @apply text-gray-400 dark:text-gray-500;
}

.missing-card {
  @apply flex items-center justify-center rounded-lg border border-dashed border-gray-200 text-sm text-gray-300 dark:border-dark-600 dark:text-dark-500;
}

.radar-footer {
  @apply mt-4 flex flex-wrap items-center justify-between gap-2 border-t border-gray-100 pt-3 text-[11px] text-gray-400 dark:border-dark-700 dark:text-gray-500;
}

@media (max-width: 1180px) {
  .matrix-heading,
  .model-row {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }

  .matrix-heading span:first-child {
    grid-column: 1 / -1;
    text-align: left;
  }

  .model-name {
    grid-column: 1 / -1;
    min-height: 0;
    flex-direction: row;
    align-items: baseline;
    justify-content: space-between;
    padding: 4px 2px;
  }

  .model-name span {
    margin-top: 0;
  }
}

@media (max-width: 640px) {
  .radar-page {
    @apply -mx-4 px-3 py-3;
  }

  .radar-shell {
    @apply rounded-xl p-4;
  }

  .toolbar,
  .toolbar-button {
    width: 100%;
  }

  .sync-status {
    width: 100%;
    margin-right: 0;
  }

  .matrix-heading,
  .model-row {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .score-card,
  .missing-card {
    min-height: 88px;
  }

  .radar-footer {
    align-items: flex-start;
    flex-direction: column;
  }
}

@media (prefers-reduced-motion: reduce) {
  .toolbar-button,
  .dimension-tabs button,
  .score-card {
    transition: none;
  }
}
</style>

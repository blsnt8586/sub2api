<template>
  <AppLayout>
    <div class="radar-page -m-4 min-h-[calc(100vh-4rem)] overflow-hidden bg-gradient-to-br from-slate-50 to-gray-100 text-gray-900 md:-m-6 lg:-m-8">
      <div class="mx-auto w-full max-w-[1640px] px-4 py-5 sm:px-6 lg:px-8 lg:py-7">
        <div v-if="loading && !summary" class="flex min-h-64 items-center justify-center">
          <LoadingSpinner />
        </div>
        <div v-else-if="loadFailed && !summary" class="radar-empty-state">
          <Icon name="exclamationTriangle" size="lg" class="mx-auto text-amber-500" />
          <p class="mt-3 text-sm text-gray-700">{{ t('codexRadar.loadError') }}</p>
          <button type="button" class="radar-secondary-button mt-4" @click="reload">
            {{ t('common.refresh') }}
          </button>
        </div>
        <div v-else-if="!available" class="radar-empty-state">
          <Icon name="chart" size="lg" class="mx-auto text-gray-400" />
          <p class="mt-3 text-sm text-gray-600">{{ t('codexRadar.unavailable') }}</p>
        </div>

        <template v-else>
          <!-- 站长推荐：每组一张表格卡片（模型/档位 | IQ | 耗时 | 费用） -->
          <section class="radar-panel" aria-labelledby="recommendations-heading">
            <div class="radar-section-heading">
              <div class="flex items-start gap-2.5">
                <span class="radar-section-mark mt-1" aria-hidden="true"></span>
                <div>
                  <h2 id="recommendations-heading" class="text-xl font-bold tracking-tight text-gray-900 sm:text-2xl">{{ t('codexRadar.recommendations') }}</h2>
                  <p class="mt-1.5 text-sm text-gray-500">{{ recommendationMeta }}</p>
                </div>
              </div>
              <div class="flex flex-wrap items-center gap-2">
                <div class="radar-update-chip">
                  <span class="h-1.5 w-1.5 rounded-full bg-teal-500" aria-hidden="true"></span>
                  <span>{{ fetchedAtLabel || t('codexRadar.hourly') }}</span>
                </div>
                <button
                  type="button"
                  class="radar-icon-button"
                  :aria-label="t('codexRadar.refresh')"
                  :title="t('codexRadar.refresh')"
                  :disabled="loading"
                  @click="reload"
                >
                  <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
                </button>
                <a
                  :href="sourceUrl"
                  target="_blank"
                  rel="noopener noreferrer"
                  class="radar-source-link"
                >
                  {{ t('codexRadar.sourceLink') }}
                  <Icon name="externalLink" size="xs" />
                </a>
              </div>
            </div>

            <div v-if="recommendationGroups.length" class="grid grid-cols-1 gap-4 md:grid-cols-2 2xl:grid-cols-4">
              <article
                v-for="(group, groupIndex) in recommendationGroups"
                :key="group.key || group.title || groupIndex"
                class="rec-card"
                :class="recommendationTheme(groupIndex)"
              >
                <h3 class="rec-card-header">{{ group.title || group.key }}</h3>
                <div v-if="group.items?.length" class="rec-table">
                  <div class="rec-row rec-head" aria-hidden="true">
                    <span>{{ t('codexRadar.colModelEffort') }}</span>
                    <span class="text-right">{{ t('codexRadar.iq') }}</span>
                    <span class="text-right">{{ t('codexRadar.colDuration') }}</span>
                    <span class="text-right">{{ t('codexRadar.colCost') }}</span>
                  </div>
                  <div
                    v-for="item in group.items.slice(0, 3)"
                    :key="`${item.model}-${item.effort}`"
                    class="rec-row"
                  >
                    <span class="rec-model" :title="`${item.model || '-'} / ${item.effort || '-'}`">{{ displayName(item.model, item.effort) }}</span>
                    <span class="rec-iq">{{ formatInteger(item.iq) }}</span>
                    <span class="rec-time">{{ formatMinutes(item.average_duration_minutes) }}</span>
                    <span class="rec-cost">{{ formatMoney(item.average_cost_usd) }}</span>
                  </div>
                </div>
                <p v-else class="p-3 text-xs text-gray-500">{{ t('codexRadar.noData') }}</p>
              </article>
            </div>
            <div v-else class="radar-empty-inline">
              {{ t('codexRadar.noData') }}
            </div>
          </section>

          <!-- 综合智能：按档位对齐的网格，一行一个模型系列 -->
          <section class="radar-panel mt-4" aria-labelledby="intelligence-heading">
            <div class="flex flex-col gap-3 border-b border-gray-200 pb-4 sm:flex-row sm:items-end sm:justify-between">
              <div class="flex items-start gap-2.5">
                <span class="radar-section-mark mt-1" aria-hidden="true"></span>
                <div>
                  <h2 id="intelligence-heading" class="text-xl font-bold tracking-tight text-gray-900 sm:text-2xl">{{ t('codexRadar.intelligence') }}</h2>
                  <p class="mt-1.5 text-sm text-gray-500">{{ t('codexRadar.intelligenceHint') }}</p>
                </div>
              </div>
              <div class="flex flex-wrap items-center gap-3 text-sm text-gray-500">
                <span class="font-medium">{{ openaiPoints.length }} {{ t('codexRadar.samples') }}</span>
                <span class="hidden h-4 w-px bg-gray-200 sm:block" aria-hidden="true"></span>
                <span>{{ intelligenceMeta }}</span>
                <span class="hidden h-4 w-px bg-gray-200 sm:block" aria-hidden="true"></span>
                <a :href="sourceUrl" target="_blank" rel="noopener noreferrer" class="font-medium text-teal-600 transition-colors hover:text-teal-500">
                  {{ t('codexRadar.sourceLink') }} <Icon name="externalLink" size="xs" class="inline" />
                </a>
              </div>
            </div>

            <div v-if="intelligenceRows.length" class="mt-4 space-y-3">
              <div v-for="row in intelligenceRows" :key="row.key" class="intel-row">
                <article
                  v-for="cell in row.cells"
                  :key="`${cell.point.model}-${cell.point.effort}`"
                  class="intel-card"
                  :class="row.theme"
                  :style="cell.column ? { '--intel-col': String(cell.column) } : undefined"
                >
                  <div class="intel-main">
                    <span class="intel-name" :title="`${cell.point.model || '-'} / ${cell.point.effort || '-'}`">{{ cell.label }}</span>
                    <div class="intel-iq-row">
                      <span class="intel-iq">{{ formatInteger(cell.point.iq) }}</span>
                      <span class="intel-samples">{{ formatInteger(cell.point.runs_24h) }}</span>
                    </div>
                  </div>
                  <div class="intel-side">
                    <span class="intel-price">{{ formatMoney(cell.point.average_price_usd) }}</span>
                    <span class="intel-time">{{ formatMinutes(cell.point.average_minutes) }}</span>
                  </div>
                </article>
              </div>
            </div>
            <div v-else class="radar-empty-inline">
              {{ t('codexRadar.noData') }}
            </div>
          </section>
        </template>

        <footer class="flex flex-wrap items-center justify-between gap-3 px-1 pt-5 text-xs text-gray-500">
          <span>{{ attribution }}</span>
          <a :href="sourceUrl" target="_blank" rel="noopener noreferrer" class="inline-flex items-center gap-1 text-gray-600 transition-colors hover:text-teal-600">
            {{ t('codexRadar.sourceLink') }} <Icon name="externalLink" size="xs" />
          </a>
        </footer>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import { useAppStore } from '@/stores/app'
import {
  getCodexRadarSummary,
  type CodexRadarData,
  type CodexRadarIntelligencePoint,
  type CodexRadarRecommendationGroup,
  type CodexRadarSummary,
} from '@/api/codexradar'

interface FamilyInfo {
  key: string
  label: string
  order: number
  theme: string
}

interface IntelligenceCell {
  point: CodexRadarIntelligencePoint
  /** 1~6 对应 ultra→low 档位列；0 = 未知档位，自动排布。 */
  column: number
  label: string
}

interface IntelligenceRow {
  key: string
  theme: string
  cells: IntelligenceCell[]
}

/** 档位列顺序（与 codexradar.com 一致，从贵到便宜）。 */
const EFFORT_COLUMNS = ['ultra', 'max', 'xhigh', 'high', 'medium', 'low']

const { t } = useI18n()
const appStore = useAppStore()
const summary = ref<CodexRadarSummary | null>(null)
const loading = ref(false)
const loadFailed = ref(false)
let refreshTimer: ReturnType<typeof setInterval> | undefined

const data = computed<CodexRadarData>(() => (summary.value?.data || {}) as CodexRadarData)
const recommendationGroups = computed<CodexRadarRecommendationGroup[]>(() => data.value.recommendations?.recommendations || [])
/** 软件工程能力（deep-swe 基准）原始数据。 */
const softwarePoints = computed<CodexRadarIntelligencePoint[]>(() => data.value.intelligence?.points || [])
/** 视觉空间推理（pompeii-adjacency 基准）原始数据；旧缓存可能缺失。 */
const visualPoints = computed<CodexRadarIntelligencePoint[]>(() => data.value.visual?.points || [])

/**
 * 综合智能（复刻原站合成算法）：两基准按「模型|档位」配对，只纳入两个维度均有
 * 有效成绩的档位；IQ 取等权几何平均，价格/耗时按各自样本数加权平均，运行次数相加。
 * visual 数据缺失（旧缓存/抓取失败）时降级展示软件工程数据，保证页面可用。
 */
const comprehensivePoints = computed<CodexRadarIntelligencePoint[]>(() => {
  const visualByKey = new Map(visualPoints.value.map(point => [`${point.model}|${point.effort}`, point]))
  if (!visualByKey.size) return softwarePoints.value
  const merged: CodexRadarIntelligencePoint[] = []
  for (const software of softwarePoints.value) {
    const visual = visualByKey.get(`${software.model}|${software.effort}`)
    const softwareIq = finiteNumber(software.iq)
    const visualIq = finiteNumber(visual?.iq)
    if (!visual || softwareIq == null || visualIq == null || softwareIq < 0 || visualIq < 0) continue
    merged.push({
      model: software.model,
      effort: software.effort,
      iq: Math.sqrt(softwareIq * visualIq),
      average_price_usd: weightedMetric(software, visual, 'average_price_usd', 'price_samples') ?? undefined,
      average_minutes: weightedMetric(software, visual, 'average_minutes', 'duration_samples') ?? undefined,
      runs_24h: summedMetric(software, visual, 'runs_24h') ?? undefined,
      runs_48h: summedMetric(software, visual, 'runs_48h') ?? undefined,
      runs_total: summedMetric(software, visual, 'runs_total') ?? undefined,
    })
  }
  return merged.length ? merged : softwarePoints.value
})

/** 只展示 OpenAI（gpt-*）系列，过滤掉第三方对比模型。 */
const openaiPoints = computed(() => comprehensivePoints.value.filter(point => String(point.model || '').toLowerCase().includes('gpt')))
const available = computed(() => Boolean(summary.value?.available && (recommendationGroups.value.length || softwarePoints.value.length)))
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
  return timestamp ? t('codexRadar.updatedAt', { time: new Date(timestamp).toLocaleString() }) : t('codexRadar.hourly')
})
const intelligenceMeta = computed(() => {
  const raw = data.value.intelligence as Record<string, unknown> | undefined
  const timestamp = typeof raw?.source_updated_at === 'string' ? raw.source_updated_at : ''
  return timestamp ? t('codexRadar.updatedAt', { time: new Date(timestamp).toLocaleString() }) : t('codexRadar.hourly')
})

/** 识别模型所属系列：gpt 系列合并为 Sol/Terra/Luna/5.5 行，其余模型各自成行。 */
function familyInfo(model: unknown): FamilyInfo {
  const raw = typeof model === 'string' ? model : ''
  const name = raw.toLowerCase()
  if (name.includes('sol')) return { key: 'sol', label: 'Sol', order: 0, theme: 'intel-sol' }
  if (name.includes('terra')) return { key: 'terra', label: 'Terra', order: 1, theme: 'intel-terra' }
  if (name.includes('luna')) return { key: 'luna', label: 'Luna', order: 2, theme: 'intel-luna' }
  if (name.includes('gpt-5.5')) return { key: 'gpt-5.5', label: '5.5', order: 3, theme: 'intel-55' }
  return { key: raw || 'other', label: raw || '-', order: 10, theme: 'intel-default' }
}

function displayName(model: unknown, effort: unknown): string {
  const family = familyInfo(model)
  const level = typeof effort === 'string' && effort ? effort : ''
  return level ? `${family.label} ${level}` : family.label
}

function effortColumn(effort: unknown): number {
  const index = EFFORT_COLUMNS.indexOf(String(effort || '').toLowerCase())
  return index === -1 ? 0 : index + 1
}

const intelligenceRows = computed<IntelligenceRow[]>(() => {
  const rowMap = new Map<string, IntelligenceRow & { order: number }>()
  for (const point of openaiPoints.value) {
    const family = familyInfo(point.model)
    let row = rowMap.get(family.key)
    if (!row) {
      row = { key: family.key, theme: family.theme, order: family.order, cells: [] }
      rowMap.set(family.key, row)
    }
    row.cells.push({ point, column: effortColumn(point.effort), label: displayName(point.model, point.effort) })
  }
  return [...rowMap.values()]
    .map(row => ({
      ...row,
      cells: [...row.cells].sort((a, b) => (a.column || 99) - (b.column || 99)),
    }))
    .sort((a, b) => a.order - b.order || a.key.localeCompare(b.key))
})

function finiteNumber(value: unknown): number | null {
  return typeof value === 'number' && Number.isFinite(value) ? value : null
}

/** 样本权重：优先指定样本字段，缺失时依次回退 valid_tasks / total，最少为 1（与原站一致）。 */
function sampleWeight(point: CodexRadarIntelligencePoint, sampleField: string): number {
  return Math.max(1, finiteNumber(point[sampleField]) ?? finiteNumber(point.valid_tasks) ?? finiteNumber(point.total) ?? 1)
}

/** 两维度均值指标按样本数加权平均；单侧缺失时取另一侧。 */
function weightedMetric(left: CodexRadarIntelligencePoint, right: CodexRadarIntelligencePoint, field: string, sampleField: string): number | null {
  const leftValue = finiteNumber(left[field])
  const rightValue = finiteNumber(right[field])
  if (leftValue == null && rightValue == null) return null
  if (leftValue == null) return rightValue
  if (rightValue == null) return leftValue
  const leftWeight = sampleWeight(left, sampleField)
  const rightWeight = sampleWeight(right, sampleField)
  return (leftValue * leftWeight + rightValue * rightWeight) / (leftWeight + rightWeight)
}

/** 两维度计数指标求和；均缺失时返回 null。 */
function summedMetric(left: CodexRadarIntelligencePoint, right: CodexRadarIntelligencePoint, field: string): number | null {
  const values = [finiteNumber(left[field]), finiteNumber(right[field])].filter((value): value is number => value != null)
  return values.length ? values.reduce((sum, value) => sum + value, 0) : null
}

function formatInteger(value: unknown): string {
  return typeof value === 'number' && Number.isFinite(value) ? Math.round(value).toString() : '-'
}

function formatMoney(value: unknown): string {
  return typeof value === 'number' && Number.isFinite(value) ? `$${value.toFixed(2)}` : '-'
}

function formatMinutes(value: unknown): string {
  return typeof value === 'number' && Number.isFinite(value) ? t('codexRadar.minutesShort', { n: value.toFixed(0) }) : '-'
}

function recommendationTheme(index: number): string {
  return ['rec-green', 'rec-yellow', 'rec-purple', 'rec-blue'][index % 4]
}

async function reload(): Promise<void> {
  loading.value = true
  loadFailed.value = false
  try {
    summary.value = await getCodexRadarSummary()
  } catch {
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

<style scoped>
.radar-page {
  min-width: 0;
  background-image: linear-gradient(rgba(148, 163, 184, 0.08) 1px, transparent 1px), linear-gradient(90deg, rgba(148, 163, 184, 0.08) 1px, transparent 1px);
  background-size: 32px 32px;
}

.radar-page > div {
  min-width: 0;
}

.radar-panel {
  min-width: 0;
  background: rgba(255, 255, 255, 0.95);
  border: 1px solid rgba(203, 213, 225, 0.6);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.05);
  border-radius: 14px;
  padding: 18px;
}

.radar-update-chip,
.radar-icon-button,
.radar-secondary-button,
.radar-source-link {
  min-height: 36px;
  border: 1px solid rgba(203, 213, 225, 0.8);
  border-radius: 8px;
}

.radar-update-chip {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  padding: 0 11px;
  color: #0d9488;
  font-size: 11px;
  font-weight: 600;
  white-space: nowrap;
  background: rgba(240, 253, 250, 0.8);
}

.radar-icon-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  color: #14b8a6;
  background: white;
  transition: border-color 180ms ease, background-color 180ms ease, color 180ms ease;
}

.radar-icon-button:hover:not(:disabled) {
  border-color: #14b8a6;
  background: rgba(240, 253, 250, 0.8);
  color: #0d9488;
}

.radar-icon-button:disabled {
  cursor: not-allowed;
  opacity: 0.5;
}

.radar-source-link {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 0 11px;
  color: #0d9488;
  font-size: 11px;
  font-weight: 600;
  background: white;
  transition: border-color 180ms ease, background-color 180ms ease;
}

.radar-source-link:hover {
  border-color: #14b8a6;
  background: rgba(240, 253, 250, 0.8);
}

.radar-section-heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 14px;
}

.radar-section-mark {
  width: 7px;
  height: 22px;
  border-radius: 999px;
  background: linear-gradient(180deg, #14b8a6, #0d9488);
  box-shadow: 0 0 12px rgba(20, 184, 166, 0.4);
}

/* ============ 站长推荐：表格卡片 ============ */

.rec-card {
  min-width: 0;
  overflow: hidden;
  border: 1.5px solid var(--rec-light);
  border-radius: 12px;
  background: white;
  transition: box-shadow 200ms ease, transform 200ms ease;
}

.rec-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 8px 20px rgba(15, 23, 42, 0.1);
}

.rec-card-header {
  padding: 10px 16px;
  background: linear-gradient(90deg, var(--rec-dark), var(--rec-light));
  color: white;
  font-size: 15px;
  font-weight: 700;
  letter-spacing: 0.02em;
}

.rec-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 42px 58px 64px;
  gap: 6px;
  align-items: center;
  padding: 12px 14px;
}

.rec-head {
  padding: 8px 16px;
  border-bottom: 1px solid #f1f5f9;
  color: #94a3b8;
  font-size: 11px;
  font-weight: 600;
}

.rec-row + .rec-row {
  border-top: 1px solid #f1f5f9;
}

.rec-model {
  overflow: hidden;
  color: #1f2937;
  font-size: 14px;
  font-weight: 700;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.rec-iq {
  color: #111827;
  font-size: 18px;
  font-weight: 800;
  text-align: right;
}

.rec-time {
  color: #6b7280;
  font-size: 13px;
  text-align: right;
}

.rec-cost {
  color: var(--rec-dark);
  font-size: 14px;
  font-weight: 800;
  text-align: right;
}

.rec-green {
  --rec-dark: #0d9488;
  --rec-light: #2dd4bf;
}

.rec-yellow {
  --rec-dark: #d97706;
  --rec-light: #fbbf24;
}

.rec-purple {
  --rec-dark: #7c3aed;
  --rec-light: #a78bfa;
}

.rec-blue {
  --rec-dark: #2563eb;
  --rec-light: #60a5fa;
}

/* ============ 综合智能：档位对齐网格 ============ */

/* 每个系列一行，6 列对应 ultra→low 档位，同档位纵向对齐 */
.intel-row {
  display: grid;
  grid-template-columns: repeat(6, minmax(0, 1fr));
  gap: 12px;
}

.intel-card {
  display: flex;
  min-width: 0;
  min-height: 96px;
  overflow: hidden;
  grid-column: var(--intel-col, auto);
  border: 1.5px solid var(--fam-border);
  border-radius: 10px;
  background: var(--fam-bg);
  transition: transform 180ms ease, box-shadow 180ms ease;
}

.intel-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 8px 18px rgba(15, 23, 42, 0.12);
}

.intel-main {
  display: flex;
  min-width: 0;
  flex: 1;
  flex-direction: column;
  justify-content: space-between;
  padding: 10px 8px 10px 12px;
}

.intel-iq-row {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 6px;
}

.intel-iq-row .intel-samples {
  margin-bottom: 3px;
}

.intel-name {
  overflow: hidden;
  color: #334155;
  font-size: 14px;
  font-weight: 700;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.intel-samples {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 22px;
  padding: 1px 6px;
  border: 1px solid var(--fam-border);
  border-radius: 6px;
  background: white;
  color: var(--fam-accent);
  font-size: 11px;
  font-weight: 800;
  line-height: 1.4;
}

.intel-iq {
  color: var(--fam-accent);
  font-size: 38px;
  font-weight: 900;
  line-height: 1;
  letter-spacing: -0.03em;
}

.intel-side {
  display: flex;
  flex-shrink: 0;
  width: 88px;
  flex-direction: column;
  border-left: 1px solid var(--fam-line);
}

.intel-price,
.intel-time {
  display: flex;
  flex: 1;
  align-items: center;
  justify-content: center;
  padding: 2px 4px;
  white-space: nowrap;
}

.intel-price {
  border-bottom: 1px solid var(--fam-line);
  color: var(--fam-accent);
  font-size: 14px;
  font-weight: 800;
}

.intel-time {
  color: #64748b;
  font-size: 13px;
  font-weight: 600;
}

.intel-sol {
  --fam-border: #f59e0b;
  --fam-accent: #d97706;
  --fam-bg: #fffbeb;
  --fam-line: rgba(245, 158, 11, 0.3);
}

.intel-terra {
  --fam-border: #3b82f6;
  --fam-accent: #2563eb;
  --fam-bg: #eff6ff;
  --fam-line: rgba(59, 130, 246, 0.3);
}

.intel-luna {
  --fam-border: #94a3b8;
  --fam-accent: #475569;
  --fam-bg: #f8fafc;
  --fam-line: rgba(148, 163, 184, 0.4);
}

.intel-55 {
  --fam-border: #06b6d4;
  --fam-accent: #0891b2;
  --fam-bg: #ecfeff;
  --fam-line: rgba(6, 182, 212, 0.3);
}

.intel-default {
  --fam-border: #14b8a6;
  --fam-accent: #0d9488;
  --fam-bg: #f0fdfa;
  --fam-line: rgba(20, 184, 166, 0.3);
}

/* ============ 空态与按钮 ============ */

.radar-empty-state,
.radar-empty-inline {
  border: 1px dashed rgba(148, 163, 184, 0.7);
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.8);
  text-align: center;
}

.radar-empty-state {
  padding: 48px 20px;
}

.radar-empty-inline {
  margin-top: 16px;
  padding: 30px 16px;
  color: #64748b;
  font-size: 13px;
}

.radar-secondary-button {
  display: inline-flex;
  min-height: 38px;
  align-items: center;
  padding: 0 14px;
  background: white;
  color: #334155;
  font-size: 13px;
  font-weight: 600;
  transition: border-color 180ms ease, background-color 180ms ease;
}

.radar-secondary-button:hover {
  border-color: #14b8a6;
  background: rgba(240, 253, 250, 0.8);
}

@media (prefers-reduced-motion: reduce) {
  .radar-icon-button,
  .radar-source-link,
  .rec-card,
  .intel-card {
    transition: none;
  }

  .rec-card:hover,
  .intel-card:hover {
    transform: none;
  }
}

/* 窄屏放弃档位列对齐，自动流式排布 */
@media (max-width: 1279px) {
  .intel-row {
    grid-template-columns: repeat(auto-fill, minmax(170px, 1fr));
  }

  .intel-card {
    grid-column: auto;
  }
}

@media (max-width: 640px) {
  .radar-panel {
    border-radius: 10px;
    padding: 13px;
  }

  .intel-row {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>

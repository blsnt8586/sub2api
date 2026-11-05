<template>
  <BaseDialog :show="show" :title="`${providerName} · ${text('detailTitle')}`" width="wide" @close="emit('close')">
    <div v-if="!overview && localLoading" class="flex min-h-52 items-center justify-center gap-2 text-sm text-gray-500 dark:text-dark-300" role="status">
      <Icon name="refresh" size="md" class="animate-spin motion-reduce:animate-none" />
      {{ text('loadingLocal') }}
    </div>
    <div v-else class="space-y-5">
      <p v-if="localError" role="alert" class="rounded-lg bg-amber-50 px-3 py-2 text-sm text-amber-800 dark:bg-amber-900/15 dark:text-amber-300">{{ text('localRefreshFailed') }}</p>
      <section v-if="profit" class="space-y-3">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <h4 class="text-sm font-semibold text-gray-900 dark:text-white">{{ text('profitSummary') }}</h4>
          <span v-if="profit.sampled_at" class="text-xs text-gray-500 dark:text-dark-400">{{ text('updatedAt', { time: formatDateTime(profit.sampled_at) }) }}</span>
        </div>
        <p v-if="!profit.available" role="status" class="rounded-lg bg-amber-50 px-3 py-2 text-sm text-amber-800 dark:bg-amber-900/15 dark:text-amber-300">{{ text('incompleteProfit') }}</p>
        <div class="grid grid-cols-3 gap-2 sm:gap-3">
          <div v-for="metric in profitMetrics" :key="metric.label" class="min-w-0 rounded-xl border border-gray-200 bg-gray-50/70 px-2 py-4 text-center dark:border-dark-700 dark:bg-dark-900/30">
            <span class="text-xs text-gray-600 sm:text-sm dark:text-dark-300">{{ text(metric.label) }}</span>
            <strong class="mt-1.5 block break-words text-lg font-semibold tabular-nums sm:text-2xl" :class="profitColor(metric.available ? metric.value : undefined)">{{ metric.available ? money(metric.value) : '—' }}</strong>
            <span class="mt-1 block text-xs tabular-nums text-gray-500 dark:text-dark-400">{{ text('profitMargin') }} {{ metric.available ? percent(metric.margin) : '—' }}</span>
          </div>
        </div>
        <div class="grid grid-cols-2 gap-3 rounded-lg border border-gray-100 px-3 py-2.5 text-center dark:border-dark-700">
          <div class="min-w-0 text-xs text-gray-500 dark:text-dark-400">{{ text('todayRevenue') }}<strong class="mt-1 block text-sm font-medium tabular-nums text-gray-800 dark:text-dark-100">{{ money(profit.today_revenue) }}</strong></div>
          <div class="min-w-0 text-xs text-gray-500 dark:text-dark-400">{{ text('todayEstimatedCost') }}<strong class="mt-1 block text-sm font-medium tabular-nums text-gray-800 dark:text-dark-100">{{ profit.available ? money(profit.today_estimated_cost) : '—' }}</strong></div>
        </div>
      </section>
      <p v-else class="py-6 text-center text-sm text-gray-500 dark:text-dark-400">{{ text('profitUnavailable') }}</p>

      <div class="grid grid-cols-3 gap-2 rounded-xl border border-gray-200 px-3 py-4 text-center dark:border-dark-700">
        <div><span class="text-xs text-gray-500 dark:text-dark-400">{{ text('balance') }}</span><strong class="mt-1 block break-words text-lg font-semibold tabular-nums text-gray-900 dark:text-white">{{ overview?.available ? money(overview.balance) : '—' }}</strong></div>
        <div><span class="text-xs text-gray-500 dark:text-dark-400">{{ text('visibleGroups') }}</span><strong class="mt-1 block text-lg font-semibold tabular-nums text-gray-900 dark:text-white">{{ overview?.available ? overview.groups.length : '—' }}</strong></div>
        <div><span class="text-xs text-gray-500 dark:text-dark-400">{{ text('customRates') }}</span><strong class="mt-1 block text-lg font-semibold tabular-nums text-gray-900 dark:text-white">{{ overview?.available ? customRateCount : '—' }}</strong></div>
      </div>

      <section v-if="profit?.accounts.length" class="space-y-3">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <h4 class="text-sm font-semibold text-gray-900 dark:text-white">{{ text('accountDetails') }} <span class="ml-1 font-normal tabular-nums text-gray-500 dark:text-dark-400">{{ profit.accounts.length }}</span></h4>
          <div class="inline-flex rounded-lg bg-gray-100 p-1 dark:bg-dark-900" role="group" :aria-label="text('accountPeriod')">
            <button v-for="period in periods" :key="period" type="button" class="min-h-11 rounded-md px-4 text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500" :class="accountPeriod === period ? 'bg-white text-gray-900 shadow-sm dark:bg-dark-700 dark:text-white' : 'text-gray-500 hover:text-gray-800 dark:text-dark-400 dark:hover:text-dark-100'" :aria-pressed="accountPeriod === period" @click="accountPeriod = period">{{ text(period === 'today' ? 'periodToday' : 'periodCumulative') }}</button>
          </div>
        </div>
        <div class="hidden overflow-hidden rounded-xl border border-gray-200 sm:block dark:border-dark-700">
          <table class="w-full table-fixed text-sm">
            <caption class="sr-only">{{ text('accountDetails') }} · {{ text(accountPeriod === 'today' ? 'periodToday' : 'periodCumulative') }}</caption>
            <thead class="bg-gray-50 text-xs text-gray-500 dark:bg-dark-900/40 dark:text-dark-300">
              <tr><th scope="col" class="w-[28%] px-3 py-3 text-left font-medium">{{ text('profitAccount') }}</th><th v-for="column in columns" :key="column" scope="col" class="px-2 py-3 text-center font-medium">{{ text(column) }}</th></tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr v-for="account in accountRows" :key="account.id">
                <th scope="row" class="break-words px-3 py-3 text-left font-medium text-gray-800 dark:text-dark-100">{{ account.name }}<span v-if="!account.available" class="mt-1 block text-xs font-normal text-amber-700 dark:text-amber-300">{{ text('missingMultiplier') }}</span></th>
                <td class="break-words px-2 py-3 text-center tabular-nums text-gray-700 dark:text-dark-200">{{ money(account.revenue) }}</td>
                <td class="break-words px-2 py-3 text-center tabular-nums text-gray-700 dark:text-dark-200">{{ account.available ? money(account.cost) : '—' }}</td>
                <td class="break-words px-2 py-3 text-center font-semibold tabular-nums" :class="profitColor(account.available ? account.profit : undefined)">{{ account.available ? money(account.profit) : '—' }}</td>
                <td class="break-words px-2 py-3 text-center tabular-nums text-gray-600 dark:text-dark-300">{{ account.available ? percent(account.margin) : '—' }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <div class="space-y-2 sm:hidden">
          <article v-for="account in accountRows" :key="account.id" class="rounded-xl border border-gray-200 p-3 dark:border-dark-700">
            <h5 class="break-words text-sm font-medium text-gray-900 dark:text-white">{{ account.name }}</h5>
            <p v-if="!account.available" class="mt-1 text-xs text-amber-700 dark:text-amber-300">{{ text('missingMultiplier') }}</p>
            <dl class="mt-3 grid grid-cols-2 gap-x-3 gap-y-3 text-center text-xs">
              <div><dt class="text-gray-500 dark:text-dark-400">{{ text('profitRevenue') }}</dt><dd class="mt-1 break-words text-sm tabular-nums text-gray-800 dark:text-dark-100">{{ money(account.revenue) }}</dd></div>
              <div><dt class="text-gray-500 dark:text-dark-400">{{ text('estimatedCost') }}</dt><dd class="mt-1 break-words text-sm tabular-nums text-gray-800 dark:text-dark-100">{{ account.available ? money(account.cost) : '—' }}</dd></div>
              <div><dt class="text-gray-500 dark:text-dark-400">{{ text('profitValue') }}</dt><dd class="mt-1 break-words text-sm font-semibold tabular-nums" :class="profitColor(account.available ? account.profit : undefined)">{{ account.available ? money(account.profit) : '—' }}</dd></div>
              <div><dt class="text-gray-500 dark:text-dark-400">{{ text('profitMargin') }}</dt><dd class="mt-1 text-sm tabular-nums text-gray-800 dark:text-dark-100">{{ account.available ? percent(account.margin) : '—' }}</dd></div>
            </dl>
          </article>
        </div>
      </section>

      <details class="rounded-xl border border-gray-200 dark:border-dark-700">
        <summary class="cursor-pointer px-4 py-3 text-sm font-semibold text-gray-800 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 dark:text-dark-100">{{ text('assetsAndRates') }}</summary>
        <div class="space-y-4 border-t border-gray-100 p-4 dark:border-dark-700">
          <p v-if="remoteError" role="status" class="break-words text-sm text-amber-700 dark:text-amber-300">{{ text('assetRefreshFailed') }}</p>
          <p v-if="!overview?.available" class="text-sm text-gray-500 dark:text-dark-400">{{ text('assetsNotSynced') }}</p>
          <template v-else>
            <p v-if="!overview.rate_overrides_available" class="text-xs leading-5 text-amber-700 dark:text-amber-300">{{ text('rateFallback') }}</p>
            <div v-if="overview.groups.length">
              <label class="mb-1.5 block text-xs font-medium text-gray-600 dark:text-dark-300" :for="searchId">{{ text('searchGroups') }}</label>
              <input :id="searchId" v-model="groupSearch" type="search" class="input min-h-11 w-full" :placeholder="text('searchGroupsPlaceholder')" />
              <div class="mt-3 grid grid-cols-1 gap-2 sm:grid-cols-2">
                <div v-for="group in filteredGroups" :key="group.id" class="flex min-w-0 items-center justify-between gap-3 rounded-lg border border-gray-100 px-3 py-2.5 dark:border-dark-700">
                  <div class="min-w-0">
                    <p class="break-words text-sm font-medium text-gray-800 dark:text-dark-100">{{ group.name }}</p>
                    <div class="mt-1 flex flex-wrap items-center gap-1.5 text-xs text-gray-500 dark:text-dark-400"><span v-if="group.platform">{{ group.platform }}</span><span v-if="group.has_custom_rate" class="rounded bg-blue-50 px-1.5 py-0.5 text-blue-700 dark:bg-blue-900/20 dark:text-blue-300">{{ text('customRate') }}</span></div>
                    <p v-if="group.description" class="mt-1 break-words text-xs text-gray-500 dark:text-dark-400">{{ group.description }}</p>
                  </div>
                  <div class="min-w-14 shrink-0 text-center tabular-nums"><strong class="text-sm text-gray-900 dark:text-white">×{{ multiplier(group.effective_multiplier) }}</strong><span v-if="group.has_custom_rate" class="mt-0.5 block text-xs text-gray-500 line-through dark:text-dark-400">×{{ multiplier(group.default_multiplier) }}</span></div>
                </div>
              </div>
              <p v-if="!filteredGroups.length" class="py-4 text-center text-sm text-gray-500 dark:text-dark-400">{{ text('noMatchingGroups') }}</p>
            </div>
            <p v-else class="text-sm text-gray-500 dark:text-dark-400">{{ text('noGroups') }}</p>
            <p v-if="overview.sampled_at" class="text-xs text-gray-500 dark:text-dark-400">{{ text('assetUpdatedAt', { time: formatDateTime(overview.sampled_at) }) }}</p>
          </template>
          <button type="button" class="btn btn-secondary min-h-11" :disabled="remoteLoading || !upstreamEnabled" @click="emit('sync-upstream')"><Icon name="refresh" size="sm" class="mr-1.5" :class="{ 'animate-spin motion-reduce:animate-none': remoteLoading }" />{{ text('syncAssets') }}</button>
        </div>
      </details>
      <details class="text-xs text-gray-500 dark:text-dark-400">
        <summary class="cursor-pointer py-2 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500">{{ text('calculationRules') }}</summary>
        <p class="mt-1 leading-6">{{ text('calculationExplanation') }}</p>
      </details>
    </div>
    <template #footer>
      <div class="flex w-full items-center justify-end gap-2">
        <button type="button" class="btn btn-secondary min-h-11" @click="emit('close')">{{ t('common.close') }}</button>
        <button type="button" class="btn btn-primary min-h-11" :disabled="localLoading" @click="emit('refresh-local')"><Icon name="refresh" size="sm" class="mr-1.5" :class="{ 'animate-spin motion-reduce:animate-none': localLoading }" />{{ text('refreshProfit') }}</button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, useId, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Sub2APIProviderRemoteOverview } from '@/api/admin/sub2apiProviders'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { formatDateTime } from '@/utils/format'

const props = defineProps<{
  show: boolean
  providerId: number | null
  providerName: string
  overview: Sub2APIProviderRemoteOverview | null
  localLoading: boolean
  remoteLoading: boolean
  upstreamEnabled: boolean
  localError?: string | null
  remoteError?: string | null
}>()
const emit = defineEmits<{ close: []; 'refresh-local': []; 'sync-upstream': [] }>()
const { t } = useI18n()
const text = (key: string, values: Record<string, string | number> = {}) => t(`admin.sub2apiProviders.remoteOverview.${key}`, values)
const periods = ['today', 'cumulative'] as const
const accountPeriod = ref<typeof periods[number]>('today')
const groupSearch = ref('')
const searchId = `provider-groups-${useId()}`
watch(() => [props.show, props.providerId], () => {
  accountPeriod.value = 'today'
  groupSearch.value = ''
})
const profit = computed(() => props.overview?.profit)
const profitMetrics = computed(() => [
  { label: 'todayProfit', value: profit.value?.today_gross_profit, margin: profit.value?.today_gross_margin, available: profit.value?.available },
  { label: 'monthProfit', value: profit.value?.month_gross_profit, margin: profit.value?.month_gross_margin, available: profit.value?.available && profit.value?.month_profit_available },
  { label: 'cumulativeProfit', value: profit.value?.cumulative_gross_profit, margin: profit.value?.cumulative_gross_margin, available: profit.value?.available && profit.value?.cumulative_profit_available },
])
const columns = ['profitRevenue', 'estimatedCost', 'profitValue', 'profitMargin']
const accountRows = computed(() => (profit.value?.accounts ?? []).map(account => {
  const today = accountPeriod.value === 'today'
  return {
    id: account.account_id, name: account.account_name, available: account.estimated_cost_available,
    revenue: today ? account.today_revenue : account.revenue,
    cost: today ? account.today_estimated_cost : account.estimated_cost,
    profit: today ? account.today_gross_profit : account.gross_profit,
    margin: today ? account.today_gross_margin : account.gross_margin,
  }
}))
const customRateCount = computed(() => props.overview?.groups.filter(group => group.has_custom_rate).length ?? 0)
const filteredGroups = computed(() => {
  const query = groupSearch.value.trim().toLocaleLowerCase()
  return (props.overview?.groups ?? []).filter(group => !query || [group.name, group.platform, group.description].some(value => value?.toLocaleLowerCase().includes(query)))
})
const money = (value: number | undefined) => value == null || !Number.isFinite(value) ? '—' : `$${value.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`
const percent = (value: number | undefined) => value == null || !Number.isFinite(value) ? '—' : `${(value * 100).toFixed(1)}%`
const multiplier = (value: number) => Number.isFinite(value) ? String(Number(value.toFixed(4))) : '—'
const profitColor = (value: number | undefined) => value == null ? 'text-gray-500 dark:text-dark-400' : value < 0 ? 'text-red-700 dark:text-red-300' : value > 0 ? 'text-emerald-700 dark:text-emerald-300' : 'text-gray-900 dark:text-dark-100'
</script>

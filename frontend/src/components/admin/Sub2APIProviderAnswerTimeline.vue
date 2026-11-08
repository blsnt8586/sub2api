<template>
  <div v-if="route.answer_probe_enabled" class="flex min-w-0 items-center gap-2 text-[11px] text-gray-500 dark:text-dark-400" :data-test="`answer-timeline-${route.id}`">
    <span class="flex-shrink-0 font-medium">{{ t('admin.sub2apiProviders.health.routes.answerProbeShort') }}</span>
    <div
      v-if="answers.length"
      class="grid min-w-0 flex-1 grid-flow-col gap-1 overflow-hidden py-0.5"
      :style="{ gridTemplateColumns: `repeat(${answers.length}, minmax(0, 1fr))` }"
    >
      <span
        v-for="(answer, index) in answers"
        :key="`${answer.at}-${index}`"
        :class="answer.error
          ? 'block h-7 min-w-0 w-full truncate rounded-[2px] border border-red-100 bg-red-50/70 px-2 leading-[26px] text-red-700 dark:border-red-900/60 dark:bg-red-900/20 dark:text-red-300'
          : 'block h-7 min-w-0 w-full truncate rounded-[2px] border border-blue-100 bg-blue-50/60 px-2 leading-[26px] text-blue-700 dark:border-blue-900/60 dark:bg-blue-900/20 dark:text-blue-300'"
        :title="answerTitle(answer)"
        data-test="answer-timeline-item"
      >{{ answer.text || answer.error || '—' }}</span>
    </div>
    <span v-else class="min-w-0 flex-1 truncate rounded-[2px] border border-dashed border-gray-200 px-2 py-1 text-gray-400 dark:border-dark-600 dark:text-dark-400">
      {{ t('admin.sub2apiProviders.health.routes.answerProbePreviewEmpty') }}
    </span>
    <span v-if="latestAnswerAt" class="flex-shrink-0 tabular-nums text-gray-400 dark:text-dark-400" data-test="answer-timeline-latest-time">
      {{ formatAnswerTime(latestAnswerAt) }}
    </span>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Sub2APIProviderProbeTargetHealth } from '@/api/admin/sub2apiProviders'

const props = defineProps<{ route: Sub2APIProviderProbeTargetHealth }>()
const { t, locale } = useI18n()
const answerDisplayLimit = 6
const answers = computed(() => {
  if (props.route.answer_history?.length) {
    return [...props.route.answer_history]
      .sort((a, b) => Date.parse(a.at || '') - Date.parse(b.at || ''))
      .slice(-answerDisplayLimit)
  }
  if (props.route.answer_text || props.route.answer_at) {
    return [{ at: props.route.answer_at || '', text: props.route.answer_text || '' }]
  }
  return []
})

const latestAnswerAt = computed(() => answers.value.at(-1)?.at || '')

const formatAnswerTime = (value: string) => {
  const date = new Date(value)
  if (!value || !Number.isFinite(date.getTime())) return ''
  return new Intl.DateTimeFormat(locale.value, {
    month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit',
  }).format(date)
}

const answerTitle = (answer: { at: string; text?: string; error?: string }) => {
  const at = formatAnswerTime(answer.at)
  return [at, answer.text || answer.error || ''].filter(Boolean).join(' · ')
}
</script>

<!--
  Canvas 平台按模型定价编辑器 [CUSTOM]

  为什么需要：canvas 平台一个分组下挂着画质、时长差异巨大的多个模型
  （veo-3.1 按秒、kling-o3-omni 按次、gpt-image-2 分 1K/2K/4K 档），
  单一分组全局价无法表达真实成本。

  只写用户显式填过的模型：未填的模型不进 payload，运行时自动回退内置默认价。
  模型清单来自后端注册表接口，前端不维护副本。
-->
<template>
  <div class="space-y-3">
    <div>
      <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
        {{ t('admin.groups.modelPricing.title') }}
      </label>
      <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
        {{ t('admin.groups.modelPricing.hint') }}
      </p>
    </div>

    <div class="space-y-4">
      <p v-if="loadError" class="rounded-lg bg-red-50 px-3 py-2 text-xs text-red-600 dark:bg-red-900/20 dark:text-red-400">
        {{ loadError }}
      </p>
      <p v-else-if="loading" class="text-xs text-gray-500 dark:text-gray-400">
        {{ t('common.loading') }}
      </p>

      <template v-else>
        <!-- 视频模型：按次 / 按秒 -->
        <section v-if="videoModels.length > 0">
          <h4 class="mb-2 text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
            {{ t('admin.groups.modelPricing.videoSection') }}
          </h4>
          <div class="overflow-hidden rounded-lg border border-gray-200 dark:border-dark-600">
            <table class="w-full text-sm">
              <thead class="bg-gray-50 text-xs text-gray-500 dark:bg-dark-700 dark:text-gray-400">
                <tr>
                  <th class="px-3 py-2 text-left font-medium">
                    {{ t('admin.groups.modelPricing.model') }}
                  </th>
                  <th class="w-32 px-3 py-2 text-left font-medium">
                    {{ t('admin.groups.videoPricing.mode') }}
                  </th>
                  <th class="w-32 px-3 py-2 text-left font-medium">
                    {{ t('admin.groups.modelPricing.unitPrice') }}
                  </th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-100 dark:divide-dark-600">
                <tr v-for="model in videoModels" :key="model">
                  <td class="px-3 py-1.5 font-mono text-xs text-gray-900 dark:text-gray-100">
                    {{ model }}
                  </td>
                  <td class="px-3 py-1.5">
                    <select
                      :value="videoMode(model)"
                      class="input !rounded-lg !px-2 !py-1 text-xs"
                      :aria-label="`${model} ${t('admin.groups.videoPricing.mode')}`"
                      @change="onVideoModeChange(model, $event)"
                    >
                      <option value="per_count">{{ t('admin.groups.videoPricing.perCount') }}</option>
                      <option value="per_second">{{ t('admin.groups.videoPricing.perSecond') }}</option>
                    </select>
                  </td>
                  <td class="px-3 py-1.5">
                    <input
                      :value="videoPriceOf(model)"
                      type="number"
                      :step="videoMode(model) === 'per_second' ? 0.0001 : 0.001"
                      min="0"
                      class="input !rounded-lg !px-2 !py-1 text-xs"
                      :placeholder="videoPlaceholder(model)"
                      :aria-label="`${model} ${t('admin.groups.modelPricing.unitPrice')}`"
                      @change="onVideoPriceInput(model, $event)"
                    />
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
          <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.groups.modelPricing.videoPriorityHint') }}
          </p>
        </section>

        <!-- 图像模型：1K / 2K / 4K 档位 -->
        <section v-if="imageModels.length > 0">
          <h4 class="mb-2 text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
            {{ t('admin.groups.modelPricing.imageSection') }}
          </h4>
          <div class="overflow-hidden rounded-lg border border-gray-200 dark:border-dark-600">
            <table class="w-full text-sm">
              <thead class="bg-gray-50 text-xs text-gray-500 dark:bg-dark-700 dark:text-gray-400">
                <tr>
                  <th class="px-3 py-2 text-left font-medium">
                    {{ t('admin.groups.modelPricing.model') }}
                  </th>
                  <th v-for="tier in imageTiers" :key="tier" class="w-28 px-3 py-2 text-left font-medium">
                    {{ tier.toUpperCase() }} ($/张)
                  </th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-100 dark:divide-dark-600">
                <tr v-for="model in imageModels" :key="model">
                  <td class="px-3 py-1.5 font-mono text-xs text-gray-900 dark:text-gray-100">
                    {{ model }}
                  </td>
                  <td v-for="tier in imageTiers" :key="tier" class="px-3 py-1.5">
                    <input
                      :value="imageDraft[model]?.[tier] ?? ''"
                      type="number"
                      step="0.001"
                      min="0"
                      class="input !rounded-lg !px-2 !py-1 text-xs"
                      :placeholder="imagePlaceholder(tier)"
                      :aria-label="`${model} ${tier.toUpperCase()}`"
                      @change="onImageInput(model, tier, $event)"
                    />
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>

        <div class="flex items-center justify-between border-t border-dashed border-gray-200 pt-3 dark:border-dark-700">
          <p class="text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.groups.modelPricing.emptyMeansFallback') }}
          </p>
          <button
            v-if="configuredCount > 0"
            type="button"
            class="text-xs text-red-600 hover:underline dark:text-red-400"
            @click="clearAll"
          >
            {{ t('admin.groups.modelPricing.clearAll') }}
          </button>
        </div>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, toRaw, watch } from 'vue'
// watch 仍用于 props.modelValue 回填
import { useI18n } from 'vue-i18n'
import { getCanvasPricingModels } from '@/api/admin/groups'
import type {
  ModelImagePricing,
  ModelPricingConfig,
  ModelVideoPricing
} from '@/types'

type VideoBillingMode = 'per_count' | 'per_second'

const props = defineProps<{
  modelValue: ModelPricingConfig | null | undefined
}>()

const emit = defineEmits<{
  'update:modelValue': [value: ModelPricingConfig | null]
}>()

const { t } = useI18n()

const imageTiers = ['1k', '2k', '4k'] as const
type ImageTier = (typeof imageTiers)[number]

const loading = ref(false)
const loadError = ref('')
const videoModels = ref<string[]>([])
const imageModels = ref<string[]>([])

// 草稿只保留用户填过的条目；提交时再转成 payload。
const videoDraft = ref<Record<string, ModelVideoPricing>>({})
const imageDraft = ref<Record<string, ModelImagePricing>>({})

const configuredCount = computed(
  () => Object.keys(videoDraft.value).length + Object.keys(imageDraft.value).length
)

// 每行选中的计费方式。有配置的行由「哪个字段非空」反推，
// 未配置的行跟随分组全局方式；用户手动切过的行以这里的选择为准。
const videoModeDraft = ref<Record<string, VideoBillingMode>>({})

function videoMode(model: string): VideoBillingMode {
  const override = videoModeDraft.value[model]
  if (override) return override
  const entry = videoDraft.value[model]
  if (entry?.per_second != null) return 'per_second'
  if (entry?.per_count != null) return 'per_count'
  return 'per_count'
}

/** 当前行输入框里该显示的数值（取所选方式对应的字段）。 */
function videoPriceOf(model: string): number | '' {
  const entry = videoDraft.value[model]
  if (!entry) return ''
  const value = videoMode(model) === 'per_second' ? entry.per_second : entry.per_count
  return value ?? ''
}

/** placeholder 显示默认参考值（视频无全局底价，仅作格式提示）。 */
function videoPlaceholder(model: string): string {
  return videoMode(model) === 'per_second' ? '0.01' : '0.05'
}

function imagePlaceholder(_tier: ImageTier): string {
  return '—'
}

// 空串 / 非数字 / 负数都视为「未配置」，回退分组全局价。
function parsePrice(raw: string): number | null {
  const trimmed = raw.trim()
  if (trimmed === '') return null
  const num = Number(trimmed)
  if (!Number.isFinite(num) || num < 0) return null
  return num
}

function onVideoPriceInput(model: string, event: Event) {
  const price = parsePrice((event.target as HTMLInputElement).value)
  // 两个字段互斥：只写所选方式那个，另一个恒为 undefined，
  // 免得库里留下永不生效的价格让人误判实际收费方式。
  const next: ModelVideoPricing =
    videoMode(model) === 'per_second' ? { per_second: price ?? undefined } : { per_count: price ?? undefined }

  const cleaned: Record<string, ModelVideoPricing> = { ...videoDraft.value }
  if (next.per_count == null && next.per_second == null) {
    delete cleaned[model]
  } else {
    cleaned[model] = next
  }
  videoDraft.value = cleaned
  emitChange()
}

/**
 * 切换某行的计费方式时清空已填价格。
 *
 * 为什么不保留：0.05 在按次下是「每次 5 分」，在按秒下是「8 秒 = 0.4 美元」，
 * 同一个数字含义差 8 倍。带值切换会静默把价格放大近一个数量级，
 * 宁可让用户重填一次。
 */
function onVideoModeChange(model: string, event: Event) {
  const mode = (event.target as HTMLSelectElement).value as VideoBillingMode
  videoModeDraft.value = { ...videoModeDraft.value, [model]: mode }

  if (videoDraft.value[model]) {
    const cleaned = { ...videoDraft.value }
    delete cleaned[model]
    videoDraft.value = cleaned
    emitChange()
  }
}

function onImageInput(model: string, tier: ImageTier, event: Event) {
  const price = parsePrice((event.target as HTMLInputElement).value)
  const current = imageDraft.value[model]
  const next: ModelImagePricing = { ...current, [tier]: price }

  const cleaned: Record<string, ModelImagePricing> = { ...imageDraft.value }
  if (next['1k'] == null && next['2k'] == null && next['4k'] == null) {
    delete cleaned[model]
  } else {
    cleaned[model] = next
  }
  imageDraft.value = cleaned
  emitChange()
}

function clearAll() {
  videoDraft.value = {}
  imageDraft.value = {}
  videoModeDraft.value = {}
  emitChange()
}

// emitChange 发出的 payload 引用。用于识别「自己 emit 又被 v-model 回灌」的回环：
// 这种回环不能重置 videoModeDraft，否则用户刚选的按秒/按次会被立刻抹掉。
let lastEmitted: ModelPricingConfig | undefined

// 只发非空字段，避免 payload 里塞满 null 让后端把「未配置」当成「配置为 0」。
function emitChange() {
  const video: Record<string, ModelVideoPricing> = {}
  for (const [model, price] of Object.entries(videoDraft.value)) {
    const entry: ModelVideoPricing = {}
    if (price.per_count != null) entry.per_count = price.per_count
    if (price.per_second != null) entry.per_second = price.per_second
    if (Object.keys(entry).length > 0) video[model] = entry
  }

  const image: Record<string, ModelImagePricing> = {}
  for (const [model, price] of Object.entries(imageDraft.value)) {
    const entry: ModelImagePricing = {}
    for (const tier of imageTiers) {
      if (price[tier] != null) entry[tier] = price[tier]
    }
    if (Object.keys(entry).length > 0) image[model] = entry
  }

  const hasVideo = Object.keys(video).length > 0
  const hasImage = Object.keys(image).length > 0

  const payload: ModelPricingConfig = {}
  if (hasVideo) payload.video = video
  if (hasImage) payload.image = image
  // 空对象而非 null：后端把空配置视为「清空」，null 会被当成「不修改」。
  lastEmitted = payload
  emit('update:modelValue', payload)
}

function syncFromProp(config: ModelPricingConfig | null | undefined) {
  // 自己 emit 又经 v-model 回灌的同一个对象：drafts 已是最新，
  // 保留 videoModeDraft，避免刚切的计费方式被重置回按次。
  // 父级 editForm 是 reactive()，回灌值会被包成 proxy，必须 toRaw 拆包后比引用。
  if (config != null && toRaw(config) === lastEmitted) return

  videoDraft.value = { ...(config?.video ?? {}) }
  imageDraft.value = { ...(config?.image ?? {}) }
  // 外部真正换了分组（不同引用）才清空手动覆盖，
  // 否则会残留上一个分组的选择。模式回显交给 videoMode() 从字段反推。
  videoModeDraft.value = {}
}

async function loadModels() {
  loading.value = true
  loadError.value = ''
  try {
    const models = await getCanvasPricingModels()
    videoModels.value = models.video
    imageModels.value = models.image
  } catch (error) {
    loadError.value =
      error instanceof Error ? error.message : t('admin.groups.modelPricing.loadFailed')
  } finally {
    loading.value = false
  }
}

// 编辑态下父组件异步填入分组数据，需要跟着回填。
watch(() => props.modelValue, syncFromProp, { immediate: true })

onMounted(() => {
  void loadModels()
})
</script>

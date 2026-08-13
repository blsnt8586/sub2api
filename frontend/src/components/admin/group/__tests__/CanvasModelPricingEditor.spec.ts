/**
 * Canvas 按模型定价编辑器测试
 *
 * 组件设计（2026-08 重构后）：
 *   - 始终展开，mount 时即加载模型列表
 *   - 每个视频模型一行：一个 <select> 计费方式 + 一个 <input> 单价（两种模式互斥）
 *   - 切换计费方式时清空已填价格（防止 8 倍误收费）
 *   - 每个图片模型一行：1K/2K/4K 三个独立价格输入框
 *   - 留空 = 回退到内置默认价（而不是按 $0 计费）——这个语义搞错会漏计费
 */
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { reactive } from 'vue'
import type { ModelPricingConfig } from '@/types'

const mockGetCanvasPricingModels = vi.fn()

vi.mock('@/api/admin/groups', () => ({
  getCanvasPricingModels: (...args: any[]) => mockGetCanvasPricingModels(...args)
}))

import CanvasModelPricingEditor from '../CanvasModelPricingEditor.vue'

const i18n = createI18n({
  legacy: false,
  locale: 'zh',
  missingWarn: false,
  fallbackWarn: false,
  messages: { zh: {} }
})

async function mountEditor(modelValue: ModelPricingConfig | null = null, props: Record<string, unknown> = {}) {
  const wrapper = mount(CanvasModelPricingEditor, {
    props: { modelValue, ...props },
    global: { plugins: [i18n] }
  })
  await flushPromises()
  return wrapper
}

function lastEmitted(wrapper: any): ModelPricingConfig | null {
  const events = wrapper.emitted('update:modelValue')
  if (!events || events.length === 0) return null
  return events[events.length - 1][0]
}

// mock 返回：2 个视频模型 + 2 个图片模型 + 1 个音频模型
// mount 后：
//   inputs[0] = veo-3.1 价格, inputs[1] = kling-o3-omni 价格
//   inputs[2..4] = gpt-image-2 (1k/2k/4k), inputs[5..7] = nano-banana-2 (1k/2k/4k)
//   selects[0] = veo-3.1 计费方式, selects[1] = kling-o3-omni 计费方式
beforeEach(() => {
  mockGetCanvasPricingModels.mockReset()
  mockGetCanvasPricingModels.mockResolvedValue({
    video: ['veo-3.1', 'kling-o3-omni'],
    image: ['gpt-image-2', 'nano-banana-2'],
    audio: ['music-v1']
  })
})

describe('CanvasModelPricingEditor', () => {
  it('mount 时立即请求模型列表', async () => {
    await mountEditor()
    expect(mockGetCanvasPricingModels).toHaveBeenCalledTimes(1)
  })

  it('按媒体类型渲染模型行，音频模型不参与定价', async () => {
    const wrapper = await mountEditor()

    const text = wrapper.text()
    expect(text).toContain('veo-3.1')
    expect(text).toContain('kling-o3-omni')
    expect(text).toContain('gpt-image-2')
    expect(text).not.toContain('music-v1')
  })

  it('有定价时回填正确值', async () => {
    const wrapper = await mountEditor({
      video: { 'veo-3.1': { per_second: 0.02 } },
      image: { 'gpt-image-2': { '2k': 0.03 } }
    })

    const values = wrapper.findAll('input').map((i) => (i.element as HTMLInputElement).value)
    expect(values).toContain('0.02')
    expect(values).toContain('0.03')
  })

  it('默认按次模式填入价格只发出 per_count', async () => {
    const wrapper = await mountEditor()

    await wrapper.findAll('input')[0].setValue('0.3')

    expect(lastEmitted(wrapper)).toEqual({ video: { 'veo-3.1': { per_count: 0.3 } } })
    expect(lastEmitted(wrapper)?.video?.['veo-3.1']).not.toHaveProperty('per_second')
  })

  it('切换到按秒后填入价格只发出 per_second', async () => {
    const wrapper = await mountEditor()

    await wrapper.findAll('select')[0].setValue('per_second')
    await wrapper.findAll('input')[0].setValue('0.02')

    expect(lastEmitted(wrapper)).toEqual({ video: { 'veo-3.1': { per_second: 0.02 } } })
    expect(lastEmitted(wrapper)?.video?.['veo-3.1']).not.toHaveProperty('per_count')
  })

  it('切换计费方式后已填价格被清空，防止 8 倍误收费', async () => {
    const wrapper = await mountEditor()

    await wrapper.findAll('input')[0].setValue('0.3')
    expect(lastEmitted(wrapper)?.video?.['veo-3.1']).toEqual({ per_count: 0.3 })

    await wrapper.findAll('select')[0].setValue('per_second')
    expect(lastEmitted(wrapper)).toEqual({})
  })

  it('清空输入后该模型条目被移除而非置 0', async () => {
    const wrapper = await mountEditor()

    await wrapper.findAll('input')[0].setValue('0.3')
    expect(lastEmitted(wrapper)?.video?.['veo-3.1']).toBeDefined()

    await wrapper.findAll('input')[0].setValue('')
    expect(lastEmitted(wrapper)).toEqual({})
    expect(lastEmitted(wrapper)?.video).toBeUndefined()
  })

  it('负数视为未配置，不产生负价', async () => {
    const wrapper = await mountEditor()

    await wrapper.findAll('input')[0].setValue('-5')
    expect(lastEmitted(wrapper)).toEqual({})
  })

  it('价格 0 是合法配置（免费模型）', async () => {
    const wrapper = await mountEditor()

    await wrapper.findAll('input')[0].setValue('0')
    expect(lastEmitted(wrapper)).toEqual({ video: { 'veo-3.1': { per_count: 0 } } })
  })

  it('不同模型互不干扰', async () => {
    const wrapper = await mountEditor()

    await wrapper.findAll('input')[0].setValue('0.3')  // veo-3.1
    await wrapper.findAll('input')[1].setValue('0.5')  // kling-o3-omni

    const payload = lastEmitted(wrapper)
    expect(payload?.video?.['veo-3.1']).toEqual({ per_count: 0.3 })
    expect(payload?.video?.['kling-o3-omni']).toEqual({ per_count: 0.5 })
  })

  it('图像模型按 1K/2K/4K 档位独立定价', async () => {
    const wrapper = await mountEditor()

    // 视频 2 行各 1 个 input → gpt-image-2 从 inputs[2] 开始
    const inputs = wrapper.findAll('input')
    await inputs[2].setValue('0.01')  // gpt-image-2 1k
    await inputs[4].setValue('0.09')  // gpt-image-2 4k（跳过 2k）

    const payload = lastEmitted(wrapper)
    expect(payload?.image?.['gpt-image-2']).toEqual({ '1k': 0.01, '4k': 0.09 })
    expect(payload?.image?.['gpt-image-2']).not.toHaveProperty('2k')
  })

  it('清空全部按钮发出空配置以清除后端定价', async () => {
    const wrapper = await mountEditor({ video: { 'veo-3.1': { per_second: 0.02 } } })

    const clearButton = wrapper
      .findAll('button')
      .find((b) => b.classes().some((c) => c.includes('text-red-600')))
    expect(clearButton).toBeDefined()
    await clearButton!.trigger('click')

    expect(lastEmitted(wrapper)).toEqual({})
  })

  it('模型列表加载失败时展示错误而非静默空表', async () => {
    mockGetCanvasPricingModels.mockRejectedValue(new Error('boom'))
    const wrapper = await mountEditor()

    expect(wrapper.text()).toContain('boom')
    expect(wrapper.findAll('input')).toHaveLength(0)
  })

  // 回归：父级 editForm 是 reactive()，v-model 会把 emit 出的 payload 包成 proxy 再回灌。
  // 若 syncFromProp 不 toRaw 拆包比引用，就会把这次回灌当成「外部换分组」而清空
  // videoModeDraft —— 用户刚选的「按秒」被重置回「按次」。这正是线上报的 bug。
  it('已有按次价切按秒后父级 reactive 回灌，计费方式不被重置回按次', async () => {
    // 模拟父级 editForm = reactive(...)：v-model 把 emit 出的 payload 包成 proxy 再回灌。
    // 场景复现线上 bug：原本按次 0.2 → 切按秒（此时值被清空、emit 空配置）→
    // 父级回灌空配置。若不 toRaw 拆包比引用，syncFromProp 会清空 videoModeDraft，
    // 而 videoDraft 已空 → videoMode() 无字段可反推 → select 掉回按次。
    const parent = reactive<{ mp: ModelPricingConfig | null }>({
      mp: { video: { 'veo-3.1': { per_count: 0.2 } } }
    })
    const wrapper = mount(CanvasModelPricingEditor, {
      props: {
        modelValue: parent.mp,
        'onUpdate:modelValue': (v: ModelPricingConfig | null) => {
          parent.mp = v
        }
      },
      global: { plugins: [i18n] }
    })
    await flushPromises()
    expect((wrapper.findAll('select')[0].element as HTMLSelectElement).value).toBe('per_count')

    // 切到按秒：原有 0.2 被清空，emit 空配置
    await wrapper.findAll('select')[0].setValue('per_second')
    expect(lastEmitted(wrapper)).toEqual({})

    // 父级把 reactive 包装后的空配置回灌（真实 v-model 行为）
    await wrapper.setProps({ modelValue: parent.mp })
    await flushPromises()

    // select 必须仍停在 per_second，而不是掉回 per_count
    expect((wrapper.findAll('select')[0].element as HTMLSelectElement).value).toBe('per_second')
  })

  it('视频输入框 placeholder 使用内置默认值', async () => {
    const wrapper = await mountEditor()

    const inputs = wrapper.findAll('input')
    // 默认模式 per_count → placeholder 为 0.05
    const perCountPlaceholders = inputs.slice(0, 2).map((i) => i.attributes('placeholder'))
    expect(perCountPlaceholders).toContain('0.05')

    // 切换到 per_second → placeholder 变为 0.01
    await wrapper.findAll('select')[0].setValue('per_second')
    expect(wrapper.findAll('input')[0].attributes('placeholder')).toBe('0.01')
  })
})

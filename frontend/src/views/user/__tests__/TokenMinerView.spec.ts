import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { nextTick } from 'vue'
import TokenMinerView from '../TokenMinerView.vue'
import { createPinia } from 'pinia'
import zh from '@/i18n/locales/zh/arcade'
import en from '@/i18n/locales/en/arcade'
import { WORLD, type GameState } from '@/games/token-miner/engine'
import * as artwork from '@/games/token-miner/renderer'

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn() }),
  RouterLink: { template: '<a><slot /></a>' },
}))

// Vitest aliases vue-i18n to its runtime-only build; use the compiler for real messages here.
vi.mock('vue-i18n', () =>
  vi.importActual(
    '../../../../node_modules/vue-i18n/dist/vue-i18n.esm-bundler.js',
  ),
)
vi.mock('@/components/layout/AppLayout.vue', () => ({
  default: { template: '<main><slot /></main>' },
}))
vi.mock('@/games/token-miner/renderer', () => ({
  drawGame: vi.fn(),
  loadMinerArtwork: async () => {},
}))

let wrapper: VueWrapper | undefined
let frameCallback: FrameRequestCallback
let time: number
async function frame(ms: number) {
  time += ms
  frameCallback(time)
  await nextTick()
}
function render(locale = 'zh') {
  wrapper = mount(TokenMinerView, {
    attachTo: document.body,
    global: {
      plugins: [
        createPinia(),
        createI18n({ legacy: false, locale, messages: { zh, en } }),
      ],
    },
  })
  return wrapper
}
beforeEach(() => {
  localStorage.clear()
  time = 1000
  vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue({
    scale: vi.fn(),
  } as unknown as CanvasRenderingContext2D)
  vi.stubGlobal(
    'requestAnimationFrame',
    vi.fn((callback: FrameRequestCallback) => {
      frameCallback = callback
      return 77
    }),
  )
  vi.stubGlobal('cancelAnimationFrame', vi.fn())
})
afterEach(() => {
  wrapper?.unmount()
  wrapper = undefined
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

describe('Arcade practice page', () => {
  it('waits for the real artwork before enabling gameplay', async () => {
    let ready!: () => void
    vi.spyOn(artwork, 'loadMinerArtwork').mockImplementationOnce(
      () =>
        new Promise<void>((resolve) => {
          ready = resolve
        }),
    )
    const w = render()
    expect(w.find('[data-testid="start"]').attributes('disabled')).toBeDefined()
    expect(w.find('[data-testid="start"]').text()).toContain('正在准备矿场')
    await w.find('.game-area').trigger('keydown', { code: 'Space' })
    expect(w.find('.game-overlay').exists()).toBe(true)
    ready()
    await flushPromises()
    expect(
      w.find('[data-testid="start"]').attributes('disabled'),
    ).toBeUndefined()
    await w.find('[data-testid="start"]').trigger('click')
    expect(w.find('.game-overlay').exists()).toBe(false)
  })

  it('offers a working retry after artwork fails instead of starting an empty game', async () => {
    vi.spyOn(artwork, 'loadMinerArtwork').mockRejectedValueOnce(
      new Error('offline'),
    )
    const w = render()
    await flushPromises()
    expect(w.find('[data-testid="start"]').text()).toContain('素材加载失败')
    await w.find('[data-testid="start"]').trigger('click')
    await flushPromises()
    expect(w.find('[data-testid="start"]').text()).toContain('开始挖矿')
    await w.find('[data-testid="start"]').trigger('click')
    expect(w.find('.game-overlay').exists()).toBe(false)
  })

  it('clearly labels practice points and does not expose an API credit claim', async () => {
    const w = render()
    await flushPromises()
    expect(w.text()).toContain('练习积分仅供娱乐，不兑换 API 额度')
    expect(w.text()).toContain('关卡记录保存在本机浏览器')
    expect(w.find('.launch-button').attributes('disabled')).toBeDefined()
    expect(w.find('[data-testid="timer"]').text()).toContain('60')
  })

  it('starts, focuses keyboard controls, prevents repeated shots, pauses and resumes', async () => {
    const w = render()
    await flushPromises()
    await w.find('[data-testid="start"]').trigger('click')
    expect(w.find('.game-overlay').exists()).toBe(false)
    expect(document.activeElement).toBe(w.find('.game-area').element)
    await frame(16)
    await frame(1100)
    expect(w.find('[data-testid="timer"]').text()).toContain('59')
    await w
      .find('.game-area')
      .trigger('keydown', { code: 'Space', repeat: true })
    expect(w.find('.launch-button').attributes('disabled')).toBeUndefined()
    await w.find('.game-area').trigger('keydown', { code: 'Space' })
    expect(w.find('.launch-button').attributes('disabled')).toBeDefined()
    await w.find('.game-area').trigger('keydown', { code: 'KeyP' })
    const timer = w.find('[data-testid="timer"]').text()
    await frame(5000)
    expect(w.text()).toContain('矿场等你回来')
    expect(w.find('[data-testid="timer"]').text()).toBe(timer)
    await w.find('[data-testid="start"]').trigger('click')
    await frame(16)
    await frame(1000)
    expect(w.find('[data-testid="timer"]').text()).not.toBe(timer)
  })

  it('auto-pauses on tab hiding and requires an explicit resume', async () => {
    const w = render()
    await flushPromises()
    await w.find('[data-testid="start"]').trigger('click')
    vi.spyOn(document, 'hidden', 'get').mockReturnValue(true)
    document.dispatchEvent(new Event('visibilitychange'))
    await nextTick()
    expect(w.text()).toContain('矿场等你回来')
    expect(w.find('[role="status"]').text()).toContain('自动暂停')
    vi.spyOn(document, 'hidden', 'get').mockReturnValue(false)
    document.dispatchEvent(new Event('visibilitychange'))
    await frame(60000)
    expect(w.find('[data-testid="timer"]').text()).toContain('60')
    expect(w.find('.game-overlay').exists()).toBe(true)
  })

  it('settles a timeout and retries with a fresh timer and score', async () => {
    const w = render()
    await flushPromises()
    await w.find('[data-testid="start"]').trigger('click')
    await frame(16)
    await frame(61000)
    expect(w.text()).toContain('差一点，再挖一局')
    expect(w.find('[data-testid="timer"]').text()).toContain('0')
    await w.find('[data-testid="start"]').trigger('click')
    expect(w.find('.game-overlay').exists()).toBe(false)
    expect(w.find('[data-testid="timer"]').text()).toContain('60')
    expect(w.find('[data-testid="score"]').text()).toContain('0')
  })

  it('requires an upgrade before advancing and preserves earned gear on retry', async () => {
    const w = render()
    await flushPromises()
    await w.find('[data-testid="start"]').trigger('click')
    await frame(16)
    // Arrange the last ore in a nearly cleared shaft; the real hook still has to return it.
    const state = vi.mocked(artwork.drawGame).mock.calls.at(-1)![1] as GameState
    state.level = 9
    state.gems = 3
    state.score = 0
    state.total = 0
    state.angle = 0
    state.ores = [{ id: 0, kind: 'token', x: WORLD.originX, y: 300, radius: 19, value: 10000, weight: 1, collected: false }]
    await w.find('.launch-button').trigger('click')
    await frame(1500)
    expect(w.find('.upgrade-picker').exists()).toBe(true)
    expect(w.find('[data-testid="start"]').attributes('disabled')).toBeDefined()
    await w.find('input[value="clock"]').setValue()
    await w.find('[data-testid="start"]').trigger('click')
    expect(w.find('.level-chip').text()).toContain('第 11 关 / 100')
    expect(w.find('[data-testid="timer"]').text()).toContain('95')
    expect(w.find('.mine-condition').text()).toContain('时钟 1')
    await frame(16)
    await frame(100000)
    await w.find('[data-testid="start"]').trigger('click')
    expect(w.find('[data-testid="timer"]').text()).toContain('95')
    expect(w.find('.mine-condition').text()).toContain('时钟 1')
    expect(w.find('[data-testid="score"]').text()).toContain('0')
  })

  it('restores a completed tenth shaft and applies exactly one upgrade before continuing', async () => {
    localStorage.setItem('token-park-miner-campaign-v1:0', JSON.stringify({ version: 1, id: 'restored-miner', seed: 42, level: 9, score: 45000, cleared: true, lives: 3, equipment: { winch: 0, clock: 0, assay: 0 } }))
    const w = render()
    await flushPromises()
    expect(w.find('.upgrade-picker').exists()).toBe(true)
    await w.find('input[value="clock"]').setValue()
    await w.find('[data-testid="start"]').trigger('click')
    expect(w.find('.level-chip').text()).toContain('第 11 关 / 100')
    expect(w.find('[data-testid="timer"]').text()).toContain('95')
    const saved = JSON.parse(localStorage.getItem('token-park-miner-campaign-v1:0')!)
    expect(saved.level).toBe(10)
    expect(saved.score).toBe(45000)
    expect(saved.equipment.clock).toBe(1)
    expect(saved.cleared).toBe(false)
  })

  it('renders the English practice flow and releases its animation on navigation', () => {
    const w = render('en')
    expect(w.text()).toContain('Dig up something good.')
    expect(w.text()).toContain(
      'Practice points cannot be exchanged for API credit.',
    )
    w.unmount()
    wrapper = undefined
    expect(cancelAnimationFrame).toHaveBeenCalledWith(77)
  })
})

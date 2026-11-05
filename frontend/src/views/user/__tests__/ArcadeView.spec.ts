import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { createPinia } from 'pinia'
import ArcadeView from '../ArcadeView.vue'
import zh from '@/i18n/locales/zh/arcade'
import en from '@/i18n/locales/en/arcade'
import { useArcadeStore } from '@/stores/arcade'

vi.mock('vue-i18n', () =>
  vi.importActual(
    '../../../../node_modules/vue-i18n/dist/vue-i18n.esm-bundler.js',
  ),
)
const mocks = vi.hoisted(() => ({ push: vi.fn(), back: vi.fn() }))
const tokenParkMocks = vi.hoisted(() => ({
  getTokenParkStatus: vi.fn(async () => ({
    eligible: true,
    reward_enabled: true,
    balance_usd: 50,
    threshold_usd: 50,
    rewards: { miner: { claimed: false, amount_usd: 1 }, ocean: { claimed: false, amount_usd: 1 }, tower: { claimed: false, amount_usd: 1 } },
  })),
  getTokenParkLeaderboard: vi.fn(async () => ({ entries: [], my_rank: 0, total_players: 0 })),
}))
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: mocks.push, back: mocks.back }),
  RouterLink: {
    props: ['to'],
    template: '<a :href="to" @click.prevent><slot /></a>',
  },
}))
vi.mock('@/components/layout/AppLayout.vue', () => ({
  default: { template: '<main><slot /></main>' },
}))
vi.mock('@/api/tokenPark', () => ({
  ...tokenParkMocks,
}))

let wrapper: VueWrapper | undefined
function render(locale = 'zh') {
  wrapper = mount(ArcadeView, {
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
  vi.stubGlobal(
    'requestAnimationFrame',
    vi.fn(() => 11),
  )
  vi.stubGlobal('cancelAnimationFrame', vi.fn())
  mocks.push.mockClear()
  mocks.back.mockClear()
})
afterEach(() => {
  wrapper?.unmount()
  wrapper = undefined
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

describe('TOKEN Park library', () => {
  it('introduces the TOKEN Park library without the retired 2048 entry', async () => {
    const w = render()
    await flushPromises()
    expect(w.find('h1').text()).toContain('TOKEN')
    expect(w.findAll('.library-game')).toHaveLength(3)
    expect(
      w.findAll('.game-cover img').map((img) => img.attributes('src')),
    ).toEqual([
      '/games/library/miner.webp',
      '/games/library/ocean.webp',
      '/games/token-tower/classic/cover.webp',
    ])
    expect(w.text()).toContain('键盘 / 触屏 / 手柄')
    expect(w.text()).toContain('本次游玩')
    expect(w.findAll('.featured-art')).toHaveLength(3)
    expect(w.text()).not.toContain('2048')
    expect(w.find('.library-tabs').text()).not.toContain('益智')
  })

  it('routes to the selected game and highlights the game card with keyboard focus', async () => {
    const w = render()
    await flushPromises()
    await w.find('.play-now').trigger('click')
    expect(w.find('.play-now').attributes('href')).toBe('/arcade/miner')
    const second = w.findAll('.library-game')[1]!
    await second.trigger('focus')
    expect(w.find('.featured-copy h2').text()).toBe('TOKEN 大鱼吃小鱼')
    await second.trigger('click')
    expect(mocks.push).toHaveBeenLastCalledWith('/arcade/ocean')
  })

  it('filters the collection and recovers from an empty recent filter', async () => {
    const w = render()
    await flushPromises()
    await w.findAll('.library-tabs button')[1]!.trigger('click')
    expect(w.findAll('.library-game')).toHaveLength(2)
    expect(w.find('.featured-copy h2').text()).toBe('TOKEN 矿工')
    await w.findAll('.library-tabs button')[3]!.trigger('click')
    expect(w.find('.library-empty').exists()).toBe(true)
    await w.find('.library-empty button').trigger('click')
    expect(w.findAll('.library-game')).toHaveLength(3)
  })
  it('filters relaxing games and launches Feed and Grow', async () => {
    const w = render()
    await flushPromises()
    await w.findAll('.library-tabs button')[2]!.trigger('click')
    expect(w.findAll('.library-game')).toHaveLength(1)
    expect(w.find('.featured-copy h2').text()).toBe('TOKEN 大鱼吃小鱼')
    expect(w.find('.play-now').attributes('href')).toBe('/arcade/ocean')
    await w.find('.library-game').trigger('click')
    expect(mocks.push).toHaveBeenLastCalledWith('/arcade/ocean')
  })

  it('records aggregate session stats and opens the last played game', async () => {
    const w = render()
    await flushPromises()
    const store = useArcadeStore()
    store.record({ id: 'one', game: 'miner', score: 700, won: true })
    store.record({ id: 'two', game: 'ocean', score: 512, won: false })
    await flushPromises()
    expect(w.text()).toContain('1,212')
    expect(w.text()).toContain('1')
    await w.find('.session-recent a').trigger('click')
    expect(w.find('.session-recent a').attributes('href')).toBe('/arcade/ocean')
  })

  it('enters and exits the immersive console library and restores card focus', async () => {
    const w = render()
    await w.find('.console-toggle').trigger('click')
    expect(w.find('.console-mode').exists()).toBe(true)
    await w.find('.console-mode').trigger('keydown', { code: 'Escape' })
    expect(w.find('.console-mode').exists()).toBe(false)
    w.unmount()
    wrapper = undefined
    expect(cancelAnimationFrame).toHaveBeenCalledWith(11)
  })

  it('uses keyboard arrows to move focus between game cards', async () => {
    const w = render()
    w.vm.$.setupState.onKeydown({
      code: 'ArrowRight',
      repeat: false,
      target: w.find('.featured-copy h2').element,
      preventDefault: vi.fn(),
    })
    await flushPromises()
    expect(w.find('.featured-copy h2').text()).toBe('TOKEN 大鱼吃小鱼')
    w.vm.$.setupState.onKeydown({
      code: 'ArrowDown',
      repeat: false,
      target: w.find('.featured-copy h2').element,
      preventDefault: vi.fn(),
    })
    await flushPromises()
    expect(document.activeElement).toBe(w.findAll('.library-game')[1]!.element)
  })

  it('renders the translated controller-friendly library', () => {
    const w = render('en')
    expect(w.find('.featured-copy h2').text()).toBe('Token Miner')
    expect(w.text()).toContain('Console mode')
    expect(w.text()).toContain('Select / play')
  })
})

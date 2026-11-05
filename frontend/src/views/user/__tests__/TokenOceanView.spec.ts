import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { createPinia } from 'pinia'
import TokenOceanView from '../TokenOceanView.vue'
import OceanScene from '@/components/games/OceanScene.vue'
import zh from '@/i18n/locales/zh/arcade'
import en from '@/i18n/locales/en/arcade'
import { useArcadeStore } from '@/stores/arcade'
import { useAuthStore } from '@/stores/auth'
import type { User } from '@/types'
import { beginFinale, tailPosition } from '@/games/token-ocean/challenge'
vi.mock('vue-i18n', () =>
  vi.importActual(
    '../../../../node_modules/vue-i18n/dist/vue-i18n.esm-bundler.js',
  ),
)
vi.mock('vue-router', () => ({
  RouterLink: { props: ['to'], template: '<a :href="to"><slot /></a>' },
}))
vi.mock('@/components/layout/AppLayout.vue', () => ({
  default: { template: '<main><slot /></main>' },
}))
vi.mock('@/components/games/OceanScene.vue', () => ({
  default: {
    props: ['state', 'label', 'labels'],
    emits: ['ready', 'error'],
    template: '<canvas />',
  },
}))
let wrapper: VueWrapper | undefined, animate: FrameRequestCallback
function render(locale = 'zh') {
  wrapper = mount(TokenOceanView, {
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
async function ready(w: VueWrapper) {
  w.findComponent(OceanScene).vm.$emit('ready')
  await flushPromises()
}
async function start(w: VueWrapper) {
  await ready(w)
  await w.find('.ocean-primary').trigger('click')
  w.vm.game.fish = []
  w.vm.game.spawnTimer = 100
}
function key(type: string, code: string) {
  window.dispatchEvent(
    new KeyboardEvent(type, { code, bubbles: true, cancelable: true }),
  )
}
beforeEach(() => {
  localStorage.clear()
  vi.stubGlobal(
    'requestAnimationFrame',
    vi.fn((cb: FrameRequestCallback) => {
      if (cb.name === 'animate') animate = cb
      return 22
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
describe('Feed and grow page', () => {
  it('gates starting on assets and retries errors', async () => {
    const w = render()
    key('keydown', 'Enter')
    expect(w.vm.game.phase).toBe('ready')
    w.findComponent(OceanScene).vm.$emit('error')
    await flushPromises()
    expect(w.text()).toContain('加载失败')
    const uid = w.findComponent(OceanScene).vm.$.uid
    await w.find('.ocean-primary').trigger('click')
    expect(w.findComponent(OceanScene).vm.$.uid).not.toBe(uid)
    await start(w)
    expect(w.vm.game.phase).toBe('playing')
    expect(w.find('.ocean-overlay').exists()).toBe(false)
  })
  it('offers one sixty-stage challenge with no mode selector and retains the full field guide', async () => {
    const w = render()
    await ready(w)
    expect(w.find('.ocean-modes').exists()).toBe(false)
    expect(w.find('.ocean-dialog p').text()).toContain('60 关')
    expect(w.find('.ocean-campaign-card').text()).toContain('第 1 / 60 关')
    key('keydown', 'ArrowRight')
    expect(w.vm.game.round).toBe(0)
    key('keydown', 'Enter')
    await flushPromises()
    expect(w.vm.game.phase).toBe('playing')
    expect(w.vm.game.challenge).not.toBeNull()
    expect(w.findAll('.ocean-species-grid article')).toHaveLength(23)
  })
  it('restores a chapter clear and continues to stage seven without recording a campaign win', async () => {
    localStorage.setItem('token-park-ocean-campaign-v1:0', JSON.stringify({ version: 1, id: 'restored-ocean', seed: 42, level: 5, score: 20000, cleared: true, lives: 2, equipment: { winch: 0, clock: 0, assay: 0 } }))
    const w = render()
    await ready(w)
    expect(w.vm.game.round).toBe(5)
    expect(w.vm.game.phase).toBe('cleared')
    expect(w.find('.ocean-primary').text()).toContain('挑战下一关')
    await w.find('.ocean-primary').trigger('click')
    expect(w.vm.game.round).toBe(6)
    expect(w.vm.game.phase).toBe('playing')
    expect(w.vm.game.score).toBe(20000)
    expect(w.vm.game.lives).toBe(3)
    expect(useArcadeStore().wins).toBe(0)
    const saved = JSON.parse(localStorage.getItem('token-park-ocean-campaign-v1:0')!)
    expect(saved.level).toBe(6)
    expect(saved.cleared).toBe(false)
  })
  it('shows regional objectives and tide countdowns, freezes on pause, and resets on retry', async () => {
    const w = render()
    await start(w)
    expect(w.find('.ocean-expedition').text()).toContain('第 1/60 关')
    expect(w.find('.ocean-expedition').text()).toContain('日光浅礁')
    w.vm.game.voyage.weather = 'golden'
    w.vm.game.voyage.weatherRemaining = 12
    w.vm.game.voyage.legs[0].caught = 3
    animate(100)
    await flushPromises()
    expect(w.find('.ocean-expedition').text()).toContain('流金鱼潮')
    await w.find('.ocean-pause:not(.ocean-sound)').trigger('click')
    const remaining = w.vm.game.voyage.weatherRemaining
    animate(9000)
    expect(w.vm.game.voyage.weatherRemaining).toBe(remaining)
    await w.find('.ocean-primary').trigger('click')
    expect(w.vm.game.voyage.legs[0].caught).toBe(3)
    w.vm.game.lives = 1
    w.vm.game.invincible = 0
    w.vm.game.dash = 0
    w.vm.game.fish = [{
      id: 900, x: w.vm.game.player.x, y: w.vm.game.player.y,
      vx: 0, vy: 0, radius: 90, species: 'perch', golden: false, turn: 0,
    }]
    animate(9100)
    animate(9200)
    await flushPromises()
    await w.find('.ocean-primary').trigger('click')
    expect(w.vm.game.voyage.stage).toBe(0)
    expect(w.vm.game.voyage.legs.every((l: { caught: number }) => l.caught === 0)).toBe(true)
    expect(w.vm.game.voyage.weather).toBeNull()
    expect(w.find('.ocean-expedition').text()).toContain('第 1/60 关')
  })
  it('shows hunter warnings and a three-star result without exposing internal state names', async () => {
    const w = render()
    await start(w)
    const c = w.vm.game.challenge
    c.finale = true
    c.hunter = { phase: 'aim' }
    animate(100)
    await flushPromises()
    expect(w.find('.ocean-event').text()).toContain('瞄准')
    c.finale = false
    w.vm.game.round = 59
    beginFinale(w.vm.game)
    c.bossHealth = 1
    c.tailBites = 2
    c.dodges = 2
    c.hunter.phase = 'recover'
    c.hunter.remaining = 2
    const tail = tailPosition(c.hunter)
    w.vm.game.player.x = tail.x
    w.vm.game.player.y = tail.y
    w.vm.game.dash = 0.3
    w.vm.game.time = 100
    animate(200)
    await flushPromises()
    expect(w.findAll('.ocean-result-stars .earned')).toHaveLength(3)
  })
  it('supports held keyboard movement and clears it on blur/pause', async () => {
    const w = render()
    await start(w)
    const x = w.vm.game.player.x
    key('keydown', 'KeyD')
    animate(100)
    animate(200)
    expect(w.vm.game.player.x).toBeGreaterThan(x)
    window.dispatchEvent(new Event('blur'))
    await flushPromises()
    const time = w.vm.game.time
    animate(300)
    expect(w.vm.game.time).toBe(time)
    await w.find('.ocean-primary').trigger('click')
    animate(400)
    animate(500)
    animate(600)
    expect(w.vm.game.player.vx).toBeLessThan(30)
  })
  it('keeps the active round when Enter is pressed again', async () => {
    const w = render()
    await start(w)
    const s = w.vm.game
    s.score = 99
    key('keydown', 'Enter')
    expect(w.vm.game).toBe(s)
    expect(w.vm.game.score).toBe(99)
  })
  it('dashes using keyboard without firing on key repeats', async () => {
    const w = render()
    await start(w)
    key('keydown', 'Space')
    expect(w.vm.game.stamina).toBe(62)
    window.dispatchEvent(
      new KeyboardEvent('keydown', { code: 'Space', repeat: true }),
    )
    expect(w.vm.game.stamina).toBe(62)
  })
  it('reads held gamepad axes continuously and ignores the dead zone', async () => {
    const w = render()
    await start(w)
    vi.spyOn(document, 'hasFocus').mockReturnValue(true)
    const pad = {
      connected: true,
      mapping: 'standard',
      axes: [1, 0],
      buttons: [],
    }
    Object.defineProperty(navigator, 'getGamepads', {
      configurable: true,
      value: vi.fn(() => [pad]),
    })
    const x = w.vm.game.player.x
    animate(100)
    animate(200)
    animate(300)
    expect(w.vm.game.player.x).toBeGreaterThan(x + 15)
    pad.axes = [0.1, 0]
    animate(400)
    animate(500)
    animate(600)
    expect(w.vm.game.player.vx).toBeLessThan(10)
    delete (navigator as unknown as Record<string, unknown>).getGamepads
  })
  it('records one run, retries cleanly, and does not duplicate on unmount', async () => {
    const w = render()
    await start(w)
    w.vm.game.score = 123
    w.vm.game.lives = 1
    w.vm.game.invincible = 0
    w.vm.game.fish = [
      {
        id: 999,
        x: w.vm.game.player.x,
        y: w.vm.game.player.y,
        vx: 0,
        vy: 0,
        radius: 45,
        species: 'perch',
        golden: false,
        turn: 0,
      },
    ]
    animate(100)
    animate(200)
    await flushPromises()
    await w.find('.ocean-primary').trigger('click')
    expect(w.vm.game.phase).toBe('playing')
    expect(w.vm.game.score).toBe(0)
    const store = useArcadeStore()
    expect(store.runs).toHaveLength(1)
    expect(store.best.ocean).toBe(123)
    w.unmount()
    wrapper = undefined
    expect(store.runs).toHaveLength(1)
    expect(cancelAnimationFrame).toHaveBeenCalledWith(22)
  })
  it('resets the run on user changes without crediting the next user', async () => {
    const w = render()
    await start(w)
    w.vm.game.score = 500
    useAuthStore().user = { id: 88 } as User
    await flushPromises()
    expect(w.vm.game.phase).toBe('ready')
    expect(useArcadeStore().runs).toEqual([])
    w.unmount()
    wrapper = undefined
    expect(useArcadeStore().runs).toEqual([])
  })
  it('opens audio only after the sound button is pressed and closes it on exit', async () => {
    const resume = vi.fn().mockResolvedValue(undefined),
      close = vi.fn().mockResolvedValue(undefined),
      created = vi.fn()
    vi.stubGlobal(
      'AudioContext',
      class {
        state = 'running'
        constructor() {
          created()
        }
        resume = resume
        close = close
      },
    )
    const w = render()
    await ready(w)
    expect(created).not.toHaveBeenCalled()
    await w.find('.ocean-sound').trigger('click')
    await flushPromises()
    expect(resume).toHaveBeenCalledTimes(1)
    expect(w.find('.ocean-sound').attributes('aria-pressed')).toBe('true')
    await w.find('.ocean-sound').trigger('click')
    expect(w.find('.ocean-sound').attributes('aria-pressed')).toBe('false')
    w.unmount()
    wrapper = undefined
    expect(close).toHaveBeenCalledTimes(1)
  })
  it('keeps the game playable when browser audio is unavailable', async () => {
    vi.stubGlobal(
      'AudioContext',
      class {
        constructor() {
          throw new Error('Unavailable')
        }
      },
    )
    const w = render()
    await start(w)
    await w.find('.ocean-sound').trigger('click')
    await flushPromises()
    expect(w.find('.ocean-sound').attributes('aria-pressed')).toBe('false')
    expect(w.vm.game.phase).toBe('playing')
  })
  it('uses English labels and exits immersive mode on Escape', async () => {
    const w = render('en')
    await ready(w)
    expect(w.text()).toContain('Small fish. Big appetite.')
    await w.find('.ocean-secondary').trigger('click')
    expect(w.find('.immersive').exists()).toBe(true)
    key('keydown', 'Escape')
    await flushPromises()
    expect(w.find('.immersive').exists()).toBe(false)
  })
})

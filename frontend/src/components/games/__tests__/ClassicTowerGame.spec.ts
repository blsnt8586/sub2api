import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { createPinia, setActivePinia } from 'pinia'
import ClassicTowerGame from '../ClassicTowerGame.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import zh from '@/i18n/locales/zh/arcade'
import en from '@/i18n/locales/en/arcade'
import { useAuthStore } from '@/stores/auth'
import { useArcadeStore } from '@/stores/arcade'
import { loadClassic, previewBattle, saveClassic, type State } from '@/games/token-tower/expedition/engine'
import { runExpedition } from '@/games/token-tower/expedition/__tests__/campaign'
import oldCampaign from '@/games/token-tower/expedition/legacy-50.json'
import { createCampaignEngine, type CampaignDefinition } from '@/games/token-tower/campaign-engine'
import type { User } from '@/types'

vi.mock('vue-i18n', () => vi.importActual('../../../../node_modules/vue-i18n/dist/vue-i18n.esm-bundler.js'))
vi.mock('vue-router', () => ({ RouterLink: { props: ['to'], template: '<a :href="to"><slot /></a>' } }))
vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<main><slot /></main>' } }))

let wrapper: VueWrapper | undefined
let images: TestImage[]
class TestImage {
  onload: (() => void) | null = null
  onerror: (() => void) | null = null
  set src(_value: string) { images.push(this) }
}
const state = (w: VueWrapper) => w.vm.$.setupState.state as State
const key = (value: string) => window.dispatchEvent(new KeyboardEvent('keydown', { key: value, bubbles: true, cancelable: true }))
function render(locale = 'zh') {
  wrapper = mount(ClassicTowerGame, { attachTo: document.body, global: { plugins: [createI18n({ legacy: false, locale, messages: { zh, en } })] } })
  return wrapper
}
async function artwork() { for (const image of images.splice(0)) image.onload?.(); await flushPromises() }
async function start(w: VueWrapper) { await artwork(); await w.find('.ct-start').trigger('click') }
beforeEach(() => {
  localStorage.clear(); setActivePinia(createPinia()); useAuthStore().user = { id: 1 } as User
  images = []; vi.stubGlobal('Image', TestImage)
  vi.stubGlobal('requestAnimationFrame', vi.fn(() => 82)); vi.stubGlobal('cancelAnimationFrame', vi.fn())
})
afterEach(() => {
  wrapper?.unmount(); wrapper = undefined; vi.restoreAllMocks(); vi.unstubAllGlobals(); vi.useRealTimers()
  document.body.innerHTML = ''
})

describe('hundred-floor tower browser UI', () => {
  it('presents one hundred-floor game without campaign switching', async () => {
    const w = render(); await artwork()
    expect(w.text()).toContain('第 1 / 100 层')
    expect(w.find('.ct-screen').text()).toContain('100 FLOORS / TEN REALMS')
    expect(w.findAll('.ct-header-actions a')).toHaveLength(0)
    expect(w.findAll('a').some(a => a.attributes('href')?.includes('campaign='))).toBe(false)
    expect(w.findAll('.ct-seals > span')).toHaveLength(10)
    expect(w.text()).not.toContain('经典十层')
  })

  it('migrates the existing fifty-floor save once, keeps its original copy and prefers new progress', async () => {
    const old = createCampaignEngine(oldCampaign as CampaignDefinition)
    const previous = old.createClassic('scout', 7)
    old.startClassic(previous); old.moveClassic(previous, 0, -1)
    const raw = old.saveClassic(previous)
    localStorage.setItem('token-tower-50-v1:1', raw)
    const w = render(); await artwork()
    expect(state(w).role).toBe('scout')
    expect(state(w).position).toEqual(previous.position)
    expect(state(w).phase).toBe('paused')
    expect(localStorage.getItem('token-tower-50-v1:1')).toBe(raw)
    expect(loadClassic(localStorage.getItem('token-tower-100-v1:1'))?.position).toEqual(previous.position)
    w.unmount(); wrapper = undefined
    const next = loadClassic(localStorage.getItem('token-tower-100-v1:1'))!
    next.stats.gold = 777
    localStorage.setItem('token-tower-100-v1:1', saveClassic(next))
    const again = render(); await artwork()
    expect(state(again).stats.gold).toBe(777)
  })
  it('renders the hundredth floor and all ten seals without missing translations', async () => {
    const w = render('en'); await start(w)
    state(w).floor = 99; state(w).discovered.push(99)
    await flushPromises()
    expect(w.find('.ct-floor').text()).toContain('100 / 100')
    expect(w.findAll('.ct-floor-track > span').at(-1)!.text()).toBe('100')
    expect(w.findAll('.ct-seals > span')).toHaveLength(10)
    expect(w.text()).toContain('Evernight')
    expect(w.text()).not.toContain('tower.expedition.')
  })
  it('renders late-floor progress, ten seals and the actual regional shop quote', async () => {
    const w = render('en'); await start(w)
    state(w).floor = 42; state(w).discovered.push(42)
    await flushPromises()
    expect(w.find('.ct-floor').text()).toContain('43 / 100')
    expect(w.find('.ct-floor h2').text()).toBe('Last Armory')
    expect(w.findAll('.ct-floor-track > span').map(s => s.text())).toEqual(['41', '42', '43', '44', '45', '46', '47', '48', '49', '50'])
    expect(w.findAll('.ct-shop button')[1]!.text()).toContain('Attack +20')
    expect(w.findAll('.ct-shop button')[1]!.text()).toContain('150 G')
    expect(w.text()).not.toContain('tower.expedition.')
  })

  it('groups attributes beside the map and retains encounter and objective controls in the other rail', async () => {
    const w = render(); await start(w)
    const console = w.find('.ct-layout')
    expect(console.element.children[0]!.classList.contains('ct-stats')).toBe(true)
    expect(console.element.children[1]!.classList.contains('ct-game')).toBe(true)
    expect(console.element.children[2]!.classList.contains('ct-aside')).toBe(true)
    expect(w.find('.ct-stats .ct-keys').exists()).toBe(true)
    expect(w.find('.ct-stats .ct-tools').exists()).toBe(true)
    expect(w.find('.ct-stats [data-testid="classic-hp"]').text()).toBe('600')
    const tabs = w.findAll('.ct-panel-tabs button')
    await tabs[2]!.trigger('click')
    expect(w.find('.ct-aside').classes()).toContain('ct-panel-shop')
    await tabs[1]!.trigger('click')
    expect(w.find('.ct-aside').classes()).toContain('ct-panel-inventory')
    expect(w.find('.ct-immersive').exists()).toBe(false)
  })
  it('keeps the embedded layout on start and resume, entering console mode only on explicit request', async () => {
    const w = render()
    expect(w.find('.ct-immersive').exists()).toBe(false)
    await start(w)
    expect(w.find('.ct-immersive').exists()).toBe(false)
    key('p'); await flushPromises()
    await w.find('.ct-resume').trigger('click')
    expect(w.find('.ct-immersive').exists()).toBe(false)
    await w.find('.ct-header-actions button').trigger('click')
    expect(w.find('.ct-immersive').exists()).toBe(true)
    await w.find('.ct-header-actions button').trigger('click')
    expect(w.find('.ct-immersive').exists()).toBe(false)
  })
  it('gates input on atlas loading and retries after a failed load', async () => {
    const w = render(); key('Enter'); expect(state(w).phase).toBe('ready')
    images[0]!.onerror?.(); await flushPromises()
    expect(w.text()).toContain('加载失败')
    await w.find('.ct-screen button').trigger('click'); await start(w)
    expect(state(w).phase).toBe('playing'); expect(w.findAll('.ct-cell')).toHaveLength(196)
  })
  it('supports both starting roles and keyboard/touch movement with undo', async () => {
    const w = render(); await artwork(); await w.findAll('[role="radio"]')[1]!.trigger('click')
    expect(state(w).stats.hp).toBe(680)
    await w.find('.ct-start').trigger('click'); key('w'); await flushPromises()
    expect(state(w).position).toEqual({ x: 1, y: 10 })
    await w.find('.ct-dpad .up').trigger('click'); expect(state(w).stats.keys.yellow).toBe(1)
    key('z'); expect(state(w).stats.keys.yellow).toBe(0)
    expect(state(w).position).toEqual({ x: 1, y: 10 })
  })
  it('blocks movement in the journal and restart confirmation', async () => {
    const w = render(); await start(w)
    key('w'); key('b'); await flushPromises()
    expect(w.findAllComponents(BaseDialog)[0]!.props('show')).toBe(true)
    key('w'); expect(state(w).position).toEqual({ x: 1, y: 10 })
    w.findAllComponents(BaseDialog)[0]!.vm.$emit('close'); await flushPromises()
    await w.find('.ct-tools button:last-child').trigger('click')
    key('w'); expect(state(w).position).toEqual({ x: 1, y: 10 })
    expect(w.findAllComponents(BaseDialog)[1]!.props('show')).toBe(true)
  })
  it('pauses on blur, persists and restores as paused with owner isolation', async () => {
    const w = render(); await start(w); key('w'); window.dispatchEvent(new Event('blur')); await flushPromises()
    expect(state(w).phase).toBe('paused'); expect(loadClassic(localStorage.getItem('token-tower-100-v1:1'))?.position.y).toBe(10)
    useAuthStore().user = { id: 2 } as User; await flushPromises()
    expect(state(w).phase).toBe('ready'); expect(state(w).position.y).toBe(11)
    useAuthStore().user = { id: 1 } as User; await flushPromises()
    expect(state(w).phase).toBe('paused'); expect(state(w).position.y).toBe(10)
    await w.find('.ct-resume').trigger('click'); expect(state(w).phase).toBe('playing')
  })
  it('cancels a click path when a dialog opens and preserves previous browser saves', async () => {
    const original = '{"legacy":"untouched"}'; localStorage.setItem('token-tower-v2:1', original)
    const w = render(); await start(w); vi.useFakeTimers()
    state(w).position = { x: 3, y: 11 }
    await w.findAll('.ct-cell')[11 * 14 + 9]!.trigger('click')
    expect(state(w).position.x).toBe(4)
    await w.find('.ct-tools button:last-child').trigger('click'); await vi.advanceTimersByTimeAsync(500)
    expect(state(w).position.x).toBe(4)
    document.querySelector<HTMLButtonElement>('.ct-confirm-restart')!.click(); await flushPromises()
    expect(state(w).phase).toBe('ready'); expect(state(w).position).toEqual({ x: 1, y: 11 })
    expect(localStorage.getItem('token-tower-v2:1')).toBe(original)
  })
  it('keeps gameplay usable when browser storage is unavailable', async () => {
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new Error('quota') })
    const w = render(); await start(w); key('w'); window.dispatchEvent(new Event('blur')); await flushPromises()
    expect(state(w).position.y).toBe(10); expect(w.find('.ct-save-error').exists()).toBe(true)
  })
  it('restores finished games without duplicate records and renders translated English controls', async () => {
    const { state: s } = runExpedition()
    localStorage.setItem('token-tower-100-v1:1', saveClassic(s))
    const record = vi.spyOn(useArcadeStore(), 'record'); const w = render('en'); await artwork()
    expect(w.text()).toContain('dawn back to the throne'); expect(record).not.toHaveBeenCalled()
    expect(w.find('.ct-dpad .up').attributes('aria-label')).toBe('Move up')
    expect(w.text()).not.toContain('tower.expedition.')
  })
  it('records an actual rescue once and blocks undo after victory', async () => {
    const { state: s } = runExpedition()
    s.phase = 'playing'; s.position = { x: 11, y: 2 }
    localStorage.setItem('token-tower-100-v1:1', saveClassic(s))
    const record = vi.spyOn(useArcadeStore(), 'record'), w = render()
    await artwork(); await w.find('.ct-resume').trigger('click')
    key('w'); await flushPromises()
    expect(state(w).phase).toBe('won'); expect(record).toHaveBeenCalledTimes(1)
    expect(useArcadeStore().runs).toHaveLength(1)
    key('z'); key('w'); await flushPromises()
    expect(state(w).phase).toBe('won'); expect(record).toHaveBeenCalledTimes(1)
  })
  it('plays attacks before returning to the map, locks input and permits undo only afterward', async () => {
    const w = render(); await start(w); vi.useFakeTimers()
    state(w).position = { x: 3, y: 7 }
    const preview = previewBattle(state(w), 200)!
    key('w'); await flushPromises()
    expect(w.find('.ct-battle').exists()).toBe(true)
    expect(state(w).stats.hp).toBe(600 - preview.damage)
    const steps = state(w).steps, seed = state(w).seed
    key('w'); key('z'); key('b'); await flushPromises()
    expect(state(w).steps).toBe(steps); expect(state(w).seed).toBe(seed)
    await vi.advanceTimersByTimeAsync(400)
    expect(w.find('.ct-fighter.attacking').exists()).toBe(true)
    await vi.advanceTimersByTimeAsync(4000)
    expect(w.find('.ct-battle').exists()).toBe(false)
    key('z'); expect(state(w).stats.hp).toBe(600); expect(state(w).stats.gold).toBe(0)
  })
  it('skips animation or pauses on blur without double rewards or orphaned timers', async () => {
    const w = render(); await start(w); vi.useFakeTimers()
    state(w).position = { x: 3, y: 7 }
    key('w'); await flushPromises()
    await w.find('.ct-battle-bottom button').trigger('click')
    expect(w.find('.ct-battle').exists()).toBe(false); expect(state(w).stats.gold).toBe(12)
    key('z'); key('w'); await flushPromises()
    window.dispatchEvent(new Event('blur')); await flushPromises()
    expect(state(w).phase).toBe('paused'); expect(w.find('.ct-battle').exists()).toBe(false)
    const hp = state(w).stats.hp
    await vi.advanceTimersByTimeAsync(5000)
    expect(state(w).stats.hp).toBe(hp); expect(state(w).stats.gold).toBe(12)
    await w.find('.ct-resume').trigger('click'); key('z'); expect(state(w).stats.gold).toBe(0)
    key('w'); await flushPromises(); key('p'); await flushPromises()
    expect(state(w).phase).toBe('paused'); expect(w.find('.ct-battle').exists()).toBe(false)
    expect(state(w).stats.gold).toBe(12)
  })
})

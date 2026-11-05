import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { createPinia, setActivePinia } from 'pinia'
import TokenTowerView from '../TokenTowerOriginalView.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import zh from '@/i18n/locales/zh/arcade'
import en from '@/i18n/locales/en/arcade'
import { useAuthStore } from '@/stores/auth'
import { useArcadeStore } from '@/stores/arcade'
import { createTower, loadTower, saveTower, type TowerState } from '@/games/token-tower/engine'
import { runCampaign } from '@/games/token-tower/__tests__/campaign'
import type { User } from '@/types'

vi.mock('vue-i18n', () => vi.importActual('../../../../node_modules/vue-i18n/dist/vue-i18n.esm-bundler.js'))
vi.mock('vue-router', () => ({ RouterLink: { props: ['to'], template: '<a :href="to"><slot /></a>' } }))
vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<main><slot /></main>' } }))

let wrapper: VueWrapper | undefined
let pending: TestImage[]
class TestImage {
  onload: (() => void) | null = null
  onerror: (() => void) | null = null
  set src(_value: string) { pending.push(this) }
}
function render(locale = 'zh') {
  wrapper = mount(TokenTowerView, { attachTo: document.body, global: { plugins: [createI18n({ legacy: false, locale, messages: { zh, en } })] } })
  return wrapper
}
const state = (w: VueWrapper) => w.vm.$.setupState.state as TowerState
async function artwork() { for (const image of pending.splice(0)) image.onload?.(); await flushPromises() }
async function start(w: VueWrapper) { await artwork(); await w.find('.tower-start').trigger('click') }
const key = (value: string, target: EventTarget = window) => target.dispatchEvent(new KeyboardEvent('keydown', { key: value, bubbles: true, cancelable: true }))

beforeEach(() => {
  localStorage.clear()
  setActivePinia(createPinia())
  useAuthStore().user = { id: 1 } as User
  pending = []
  vi.stubGlobal('Image', TestImage)
  vi.stubGlobal('requestAnimationFrame', vi.fn(() => 82))
  vi.stubGlobal('cancelAnimationFrame', vi.fn())
})
afterEach(() => {
  wrapper?.unmount(); wrapper = undefined
  vi.restoreAllMocks(); vi.unstubAllGlobals()
  document.body.innerHTML = ''
})

describe('TOKEN Tower page', () => {
  it('waits for every sprite, reports an artwork error and supports retry before starting', async () => {
    const w = render()
    key('Enter')
    expect(state(w).phase).toBe('ready')
    expect(w.text()).toContain('正在打开')
    pending[0]!.onerror?.(); await flushPromises()
    expect(w.text()).toContain('加载失败')
    await w.find('.tower-screen button').trigger('click')
    await start(w)
    expect(state(w).phase).toBe('playing')
    expect(w.findAll('.tower-cell')).toHaveLength(121)
    expect(w.find('.tower-screen').exists()).toBe(false)
    expect(w.find('.tower-player img').attributes('src')).toBe('/games/token-tower/tiles/hero.png')
  })

  it('supports keyboard and touch controls, blocking input when the journal is open', async () => {
    const w = render(); await start(w)
    key('d'); await flushPromises()
    expect(state(w).position).toEqual({ x: 2, y: 9 })
    await w.find('.tower-dpad .right').trigger('click')
    expect(state(w).position.x).toBe(3)
    key('b'); await flushPromises()
    expect(w.findComponent(BaseDialog).props('show')).toBe(true)
    key('d')
    expect(state(w).position.x).toBe(3)
    w.findComponent(BaseDialog).vm.$emit('close'); await flushPromises()
    key('d'); await flushPromises()
    expect(state(w).stats.potions).toBe(2)
    expect(state(w).stats.score).toBe(20)
  })

  it('pauses on blur and hidden pages and resumes only through an explicit action', async () => {
    const w = render(); await start(w)
    window.dispatchEvent(new Event('blur')); await flushPromises()
    expect(state(w).phase).toBe('paused')
    key('d'); expect(state(w).position.x).toBe(1)
    await w.find('.tower-resume').trigger('click')
    expect(state(w).phase).toBe('playing')
    key('p'); await flushPromises()
    expect(state(w).phase).toBe('paused')
    key('p'); await flushPromises()
    expect(state(w).phase).toBe('playing')
    vi.spyOn(document, 'hidden', 'get').mockReturnValue(true)
    document.dispatchEvent(new Event('visibilitychange')); await flushPromises()
    expect(state(w).phase).toBe('paused')
  })

  it('does not move from editable controls and restores inventory on undo', async () => {
    const w = render(); await start(w)
    const input = document.createElement('input'); document.body.append(input)
    key('d', input); expect(state(w).position.x).toBe(1)
    key('d'); key('d'); key('d'); await flushPromises()
    expect(state(w).stats.potions).toBe(2)
    key('z'); await flushPromises()
    expect(state(w).position.x).toBe(3)
    expect(state(w).stats.potions).toBe(1)
  })

  it('saves on exit and restores the same account into a paused expedition', async () => {
    const w = render(); await start(w)
    key('d'); key('d'); key('d'); await flushPromises()
    const id = state(w).id
    w.unmount(); wrapper = undefined
    expect(loadTower(localStorage.getItem('token-tower-v2:1'))?.stats.score).toBe(20)
    const restored = render(); await artwork()
    expect(state(restored).id).toBe(id)
    expect(state(restored).phase).toBe('paused')
    expect(state(restored).position.x).toBe(4)
    expect(restored.find('.tower-resume').exists()).toBe(true)
  })

  it('isolates saves and practice records when switching accounts', async () => {
    const w = render(); await start(w)
    key('d'); key('d'); key('d'); await flushPromises()
    const id = state(w).id
    expect(useArcadeStore().runs).toHaveLength(1)
    useAuthStore().user = { id: 2 } as User
    await flushPromises()
    expect(state(w).id).not.toBe(id)
    expect(state(w).stats.score).toBe(0)
    expect(useArcadeStore().runs).toHaveLength(0)
    expect(loadTower(localStorage.getItem('token-tower-v2:1'))?.id).toBe(id)
    useAuthStore().user = { id: 1 } as User
    await flushPromises()
    expect(state(w).id).toBe(id)
    expect(state(w).phase).toBe('paused')
    expect(useArcadeStore().runs).toHaveLength(1)
  })

  it('keeps play available when localStorage is unavailable', async () => {
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new Error('unavailable') })
    const w = render(); await start(w)
    window.dispatchEvent(new Event('blur')); await flushPromises()
    expect(w.text()).toContain('无法保存')
    await w.find('.tower-resume').trigger('click')
    key('d'); expect(state(w).position.x).toBe(2)
  })

  it('requires the restart dialog and resets progress while keeping the previous practice record', async () => {
    const w = render(); await start(w)
    key('d'); key('d'); key('d'); await flushPromises()
    const id = state(w).id
    await w.find('[aria-label="重新冒险"]').trigger('click')
    expect(state(w).id).toBe(id)
    const confirm = document.querySelector<HTMLButtonElement>('[data-testid="tower-confirm-restart"]')!
    confirm.click(); await flushPromises()
    expect(state(w).id).not.toBe(id)
    expect(state(w).phase).toBe('ready')
    expect(state(w).stats.score).toBe(0)
    expect(useArcadeStore().runs).toHaveLength(1)
  })

  it('renders a real completed campaign, records one victory and keeps the result after reloading', async () => {
    const { state: complete } = runCampaign()
    localStorage.setItem('token-tower-v2:1', saveTower(complete))
    const w = render(); await artwork()
    expect(w.text()).toContain('最后的 TOKEN，属于你了')
    expect(w.findAll('.tower-result-stars .earned')).toHaveLength(3)
    expect(useArcadeStore().wins).toBe(1)
    key('d'); key('z'); await flushPromises()
    expect(useArcadeStore().runs).toHaveLength(1)
    expect(state(w).stats.score).toBe(complete.stats.score)
  })

  it('translates floor names, encounter hints, instructions and notices into English', async () => {
    localStorage.setItem('token-tower-v2:1', saveTower(createTower()))
    const w = render('en'); await artwork()
    expect(w.text()).toContain('Forgotten Courtyard')
    expect(w.text()).toContain('No timer')
    expect(w.text()).toContain('Save restored')
    await w.find('.tower-start').trigger('click')
    expect(w.text()).not.toMatch(/tower\.(floors|notices|monsters|tiles)\./)
    w.unmount(); wrapper = undefined
    expect(cancelAnimationFrame).toHaveBeenCalledWith(82)
  })
})

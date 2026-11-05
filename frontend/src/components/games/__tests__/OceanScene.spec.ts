import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import OceanScene from '../OceanScene.vue'
import { createOcean } from '@/games/token-ocean/engine'
const mocks = vi.hoisted(() => ({ render: vi.fn() }))
vi.mock('@/games/token-ocean/renderer', () => ({
  renderOcean: mocks.render,
}))
let wrapper: VueWrapper | undefined
let frame: FrameRequestCallback
let resize: () => void
let disconnect: ReturnType<typeof vi.fn>
let images: Array<{ onload: () => void; onerror: () => void }>
beforeEach(() => {
  vi.clearAllMocks()
  images = []
  disconnect = vi.fn()
  vi.stubGlobal(
    'Image',
    class {
      onload = () => {}
      onerror = () => {}
      set src(_: string) {
        images.push(this)
      }
    },
  )
  vi.stubGlobal(
    'ResizeObserver',
    class {
      constructor(callback: () => void) {
        resize = callback
      }
      observe = vi.fn()
      disconnect = disconnect
    },
  )
  vi.stubGlobal(
    'requestAnimationFrame',
    vi.fn((cb: FrameRequestCallback) => {
      frame = cb
      return 71
    }),
  )
  vi.stubGlobal('cancelAnimationFrame', vi.fn())
  vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue({
    setTransform: vi.fn(),
  } as unknown as CanvasRenderingContext2D)
})
afterEach(() => {
  wrapper?.unmount()
  wrapper = undefined
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})
function render() {
  wrapper = mount(OceanScene, {
    props: {
      state: createOcean(),
      label: '2D cove',
      labels: { you: 'you', eat: 'eat', danger: 'danger' },
    },
  })
  return wrapper
}
async function loaded() {
  images.forEach((i) => i.onload())
  await flushPromises()
}
describe('Ocean scene lifecycle', () => {
  it('repaints after a resize notification even while the simulation clock is frozen', async () => {
    render()
    await loaded()
    const count = mocks.render.mock.calls.length
    resize()
    frame(100)
    expect(mocks.render).toHaveBeenCalledTimes(count + 1)
  })
  it('waits for all art, avoids repainting a stationary scene, then cleans up', async () => {
    const w = render()
    expect(w.emitted('ready')).toBeUndefined()
    await loaded()
    expect(w.emitted('ready')).toHaveLength(1)
    const count = mocks.render.mock.calls.length
    frame(100)
    frame(200)
    expect(mocks.render).toHaveBeenCalledTimes(count)
    const s = createOcean()
    s.time = 1
    await w.setProps({ state: s })
    frame(300)
    expect(mocks.render).toHaveBeenCalledTimes(count + 1)
    w.unmount()
    wrapper = undefined
    expect(disconnect).toHaveBeenCalledTimes(1)
    expect(cancelAnimationFrame).toHaveBeenCalledWith(71)
  })
  it('does not start rendering if the page is closed while artwork loads', async () => {
    const w = render()
    w.unmount()
    wrapper = undefined
    await loaded()
    expect(mocks.render).not.toHaveBeenCalled()
    expect(requestAnimationFrame).not.toHaveBeenCalled()
  })
  it('reports failed artwork and unsupported canvas without starting the loop', async () => {
    const w = render()
    images[0]!.onerror()
    await flushPromises()
    expect(w.emitted('error')).toHaveLength(1)
    expect(requestAnimationFrame).not.toHaveBeenCalled()
    w.unmount()
    wrapper = undefined
    vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(null)
    const noCanvas = render()
    await loaded()
    expect(noCanvas.emitted('error')).toHaveLength(1)
  })
})

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { defineComponent, h } from 'vue'
import { useArcadeGamepad } from '../useArcadeGamepad'

let callback: FrameRequestCallback
let pad: Gamepad | null
let hidden = false
function makePad() {
  return { connected: true, mapping: 'standard', axes: [0, 0], buttons: Array.from({ length: 16 }, () => ({ pressed: false })) } as unknown as Gamepad
}
function press(button: number, pressed: boolean) {
  (pad!.buttons[button] as { pressed: boolean }).pressed = pressed
}
function harness() {
  const action = vi.fn()
  const component = mount(defineComponent({ setup() { useArcadeGamepad(action); return () => h('div') } }))
  return { action, component }
}
beforeEach(() => {
  pad = makePad(); hidden = false
  vi.spyOn(document, 'hasFocus').mockReturnValue(true)
  vi.spyOn(document, 'hidden', 'get').mockImplementation(() => hidden)
  Object.defineProperty(navigator, 'getGamepads', { configurable: true, value: () => [pad] })
  vi.stubGlobal('requestAnimationFrame', vi.fn((cb: FrameRequestCallback) => { callback = cb; return 31 }))
  vi.stubGlobal('cancelAnimationFrame', vi.fn())
})
afterEach(() => { delete (navigator as Partial<Navigator>).getGamepads; vi.restoreAllMocks(); vi.unstubAllGlobals() })

describe('Arcade controller input', () => {
  it('fires confirmation on the button edge and does not repeat a held A', () => {
    const { action, component } = harness()
    callback(0); press(0, true); callback(10); callback(500)
    expect(action.mock.calls).toEqual([['confirm']])
    press(0, false); callback(510); press(0, true); callback(520)
    expect(action).toHaveBeenCalledTimes(2)
    component.unmount(); expect(cancelAnimationFrame).toHaveBeenCalledWith(31)
  })
  it('repeats direction input after its initial delay and applies an analog deadzone', () => {
    const { action, component } = harness()
    callback(0); pad!.axes[0] = .4; callback(10)
    expect(action).not.toHaveBeenCalled()
    pad!.axes[0] = .9; callback(20); callback(300)
    expect(action.mock.calls).toEqual([['right']])
    callback(380); callback(550)
    expect(action.mock.calls).toEqual([['right'], ['right'], ['right']])
    component.unmount()
  })
  it('suppresses carried button presses after route mount and while a tab is hidden', () => {
    press(0, true)
    const { action, component } = harness()
    callback(0); callback(100)
    expect(action).not.toHaveBeenCalled()
    hidden = true; press(1, true); callback(200)
    hidden = false; callback(300); callback(400)
    expect(action).not.toHaveBeenCalled()
    press(1, false); callback(450); press(1, true); callback(500)
    expect(action.mock.calls).toEqual([['back']])
    component.unmount()
  })
})

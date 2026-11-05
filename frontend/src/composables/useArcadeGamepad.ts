import { onMounted, onBeforeUnmount, ref } from 'vue'

export type PadAction =
  | 'up'
  | 'down'
  | 'left'
  | 'right'
  | 'confirm'
  | 'back'
  | 'menu'
  | 'aux'
/** Standard mapping only; edge-triggered buttons and repeat-delayed directional navigation. */
export function useArcadeGamepad(onAction: (action: PadAction) => void) {
  const connected = ref(false)
  let frame = 0
  let previous = new Set<PadAction>()
  let nextRepeat = 0
  let primed = false
  function poll(time: number) {
    const pad = [...(navigator.getGamepads?.() ?? [])].find(
      (p) => p?.connected && p.mapping === 'standard',
    )
    connected.value = Boolean(pad)
    const held = new Set<PadAction>()
    if (pad && !document.hidden && document.hasFocus()) {
      const pressed = (i: number) => Boolean(pad.buttons[i]?.pressed)
      if (pressed(12) || (pad.axes[1] ?? 0) < -0.6) held.add('up')
      if (pressed(13) || (pad.axes[1] ?? 0) > 0.6) held.add('down')
      if (pressed(14) || (pad.axes[0] ?? 0) < -0.6) held.add('left')
      if (pressed(15) || (pad.axes[0] ?? 0) > 0.6) held.add('right')
      if (pressed(0)) held.add('confirm')
      if (pressed(1)) held.add('back')
      if (pressed(2)) held.add('aux')
      if (pressed(9)) held.add('menu')
    }
    if (!pad || document.hidden || !document.hasFocus()) {
      previous = held
      primed = false
    } else if (!primed) {
      previous = held
      primed = true
    } else {
      for (const action of held) {
        const directional = ['up', 'down', 'left', 'right'].includes(action)
        if (!previous.has(action)) {
          onAction(action)
          if (directional) nextRepeat = time + 350
        } else if (directional && time >= nextRepeat) {
          onAction(action)
          nextRepeat = time + 160
        }
      }
      previous = held
    }
    frame = requestAnimationFrame(poll)
  }
  onMounted(() => {
    frame = requestAnimationFrame(poll)
  })
  onBeforeUnmount(() => cancelAnimationFrame(frame))
  return { connected }
}

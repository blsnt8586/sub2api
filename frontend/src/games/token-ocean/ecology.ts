import { roundFor } from './campaign'
import { fishRelation, type OceanState, type Swimmer } from './engine'
import { isGentleGiant } from './exploration'

/** Progress or time raises pressure; it stays bounded and the finale remains a duel. */
export function reefPressure(s: OceanState): 0 | 1 | 2 {
  if (!s.challenge || s.challenge.finale) return 0
  return s.level >= 4 || s.roundTime >= 70 || roundFor(s).chapter >= 7 ? 2 : s.level >= 2 || s.roundTime >= 35 || roundFor(s).chapter >= 3 ? 1 : 0
}
export function pufferPhase(
  s: OceanState,
  f: Pick<Swimmer, 'species' | 'turn'>,
) {
  if (f.species !== 'pufferfish') return 'calm'
  const cycle = (s.time + f.turn) % 8
  return cycle >= 4.8 && cycle < 7.2
    ? 'inflated'
    : cycle >= 4 && cycle < 4.8
      ? 'warning'
      : 'calm'
}
export function swimmerRadius(
  s: OceanState,
  f: Pick<Swimmer, 'species' | 'turn' | 'radius'>,
) {
  return f.radius * (pufferPhase(s, f) === 'inflated' ? 1.45 : 1)
}
export function swimmerRelation(
  s: OceanState,
  f: Pick<Swimmer, 'species' | 'turn' | 'radius'>,
) {
  if (isGentleGiant(f.species)) return 'peer'
  if (pufferPhase(s, f) === 'inflated') return 'danger'
  // Venom can only be overcome during frenzy, and only within its normal size limit.
  if (
    f.species === 'lionfish' &&
    !(s.frenzy > 0 && f.radius <= s.player.radius * 1.6)
  )
    return 'danger'
  return fishRelation(s, f.radius)
}
/** Barracuda telegraph, lock, sprint straight, then recover. No mid-charge homing. */
export function tickAmbush(
  s: OceanState,
  f: Swimmer,
  dt: number,
  danger: boolean,
) {
  if (
    !['barracuda', 'viperfish'].includes(f.species) ||
    s.challenge?.finale
  )
    return false
  const dx = s.player.x - f.x,
    dy = s.player.y - f.y,
    distance = Math.hypot(dx, dy)
  const a = (f.ambush ??= { phase: 'cruise', remaining: 1.5, x: 0, y: 0 })
  a.remaining -= dt
  if (a.phase === 'cruise') {
    if (
      !danger ||
      s.invincible > 0 ||
      distance > 430 ||
      distance < 95 ||
      a.remaining > 0
    )
      return false
    a.phase = 'aim'
    a.remaining = 0.9
    a.x = s.player.x
    a.y = s.player.y
  }
  if (a.phase === 'aim') {
    f.vx *= Math.exp(-12 * dt)
    f.vy *= Math.exp(-12 * dt)
    // Last 0.45s stays locked even if the player changes direction.
    if (a.remaining > 0.45) {
      a.x = s.player.x
      a.y = s.player.y
    }
    if (a.remaining <= 0) {
      const x = a.x - f.x,
        y = a.y - f.y,
        d = Math.max(1, Math.hypot(x, y))
      f.vx = (x / d) * 390
      f.vy = (y / d) * 390
      a.phase = 'charge'
      a.remaining = 0.7
    }
  } else if (a.phase === 'charge' && a.remaining <= 0) {
    a.phase = 'recover'
    a.remaining = 1.5
    f.vx *= 0.12
    f.vy *= 0.12
  } else if (a.phase === 'recover' && a.remaining <= 0) {
    a.phase = 'cruise'
    a.remaining = 2.3
  }
  return a.phase !== 'cruise'
}

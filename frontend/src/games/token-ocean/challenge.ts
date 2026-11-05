import { completeRound, roundFor } from './campaign'
import { reefPressure } from './ecology'
import type { OceanState } from './engine'
export interface Hunter {
  x: number
  y: number
  radius: number
  vx: number
  vy: number
  aimX: number
  aimY: number
  facing: number
  phase: 'stalk' | 'aim' | 'charge' | 'recover'
  remaining: number
  bitten: boolean
  closest: number
}
export interface Shoal {
  id: number
  remaining: number
  caught: number
  total: number
  x: number
  y: number
}
export interface OceanChallenge {
  pressure: number
  hunter: Hunter | null
  huntAt: number
  shoalAt: number
  shoal: Shoal | null
  finale: boolean
  bossHealth: number
  tailBites: number
  dodges: number
  shoals: number
  banner:
    | 'pressure'
    | 'none'
    | 'shoal'
    | 'shoalClear'
    | 'hunter'
    | 'tail'
    | 'dodge'
    | 'finale'
  bannerTime: number
}
export const createChallenge = (): OceanChallenge => ({
  pressure: 0,
  hunter: null,
  huntAt: 18,
  shoalAt: 6,
  shoal: null,
  finale: false,
  bossHealth: 3,
  tailBites: 0,
  dodges: 0,
  shoals: 0,
  banner: 'none',
  bannerTime: 0,
})
const clamp = (n: number, min: number, max: number) =>
  Math.max(min, Math.min(max, n))
export function challengeBanner(
  s: OceanState,
  banner: OceanChallenge['banner'],
) {
  if (!s.challenge) return
  s.challenge.banner = banner
  s.challenge.bannerTime = 3
  s.event++
}
export function beginFinale(s: OceanState) {
  const c = s.challenge
  if (!c || c.finale) return
  c.finale = true
  // The finale is a readable duel: ordinary predators leave instead of piling onto its charge.
  s.fish = s.fish.filter(
    (f) =>
      f.species !== 'pufferfish' &&
      f.species !== 'lionfish' &&
      f.radius * 1.15 <= s.player.radius,
  )
  c.bossHealth = 3
  s.invincible = Math.max(s.invincible, 2)
  // Reset into a full telegraph, including when growth happens during a charge.
  c.hunter = makeHunter(s)
  c.hunter.phase = 'aim'
  c.hunter.remaining = 1.8
  challengeBanner(s, 'finale')
}
function makeHunter(s: OceanState): Hunter {
  const radius = Math.max(58, s.player.radius * 1.4)
  const x = clamp(
    s.player.x + (s.player.x > 1300 ? -440 : 440),
    radius + 20,
    2600 - radius - 20,
  )
  const y = clamp(s.player.y - 180, radius + 20, 1600 - radius - 20)
  return {
    x,
    y,
    radius,
    vx: 0,
    vy: 0,
    aimX: s.player.x,
    aimY: s.player.y,
    facing: x < s.player.x ? 1 : -1,
    phase: 'stalk',
    remaining: 2.2,
    bitten: false,
    closest: Infinity,
  }
}
export function tailPosition(h: Hunter) {
  // The head follows the horizontal direction of the illustrated predator.
  return { x: h.x - h.facing * h.radius * 1.05, y: h.y }
}
export function tickChallenge(s: OceanState, dt: number, random: () => number) {
  const c = s.challenge
  if (!c || s.phase !== 'playing') return
  c.bannerTime = Math.max(0, c.bannerTime - dt)
  const pressure = reefPressure(s)
  if (pressure > c.pressure) {
    c.pressure = pressure
    challengeBanner(s, 'pressure')
  }
  if (c.shoal) {
    c.shoal.remaining = Math.max(0, c.shoal.remaining - dt)
    if (c.shoal.remaining <= 0) {
      const id = c.shoal.id
      s.fish = s.fish.filter((f) => f.shoal !== id)
      c.shoal = null
      c.shoalAt = s.time + 14 + random() * 12
    }
  }
  if (!c.finale && !c.shoal && s.time >= c.shoalAt) {
    const id = ++s.nextId,
      angle = random() * Math.PI * 2,
      cx = clamp(s.player.x + Math.cos(angle) * 300, 180, 2420),
      cy = clamp(s.player.y + Math.sin(angle) * 300, 180, 1420)
    c.shoal = { id, remaining: 11 + random() * 3, caught: 0, total: 6, x: cx, y: cy }
    for (let i = 0; i < 6; i++) {
      const a = (i * Math.PI) / 3
      s.fish.push({
        id: s.nextId++,
        x: cx + Math.cos(a) * 66,
        y: cy + Math.sin(a) * 66,
        vx: 0,
        vy: 0,
        radius: Math.max(9, s.player.radius * 0.53),
        species: 'bluegill',
        golden: true,
        turn: i,
        shoal: id,
      })
    }
    challengeBanner(s, 'shoal')
  }
  if (!c.hunter && s.time >= c.huntAt) {
    c.hunter = makeHunter(s)
    challengeBanner(s, 'hunter')
  }
  const h = c.hunter
  if (!h) return
  // A larger body grows only in the stalking phase, never during a telegraphed attack.
  if (h.phase === 'stalk') h.radius = Math.max(58, s.player.radius * 1.4)
  h.remaining -= dt
  if (h.phase === 'stalk') {
    const desiredX = clamp(
        s.player.x + h.facing * -350,
        h.radius + 10,
        2590 - h.radius,
      ),
      desiredY = clamp(s.player.y - 100, h.radius + 10, 1590 - h.radius)
    const dx = desiredX - h.x,
      dy = desiredY - h.y,
      d = Math.max(1, Math.hypot(dx, dy))
    h.vx = (dx / d) * 95
    h.vy = (dy / d) * 95
    h.x += h.vx * dt
    h.y += h.vy * dt
    h.facing = s.player.x >= h.x ? 1 : -1
    if (h.remaining <= 0) {
      h.phase = 'aim'
      h.remaining = c.finale ? 1.25 : 1.5 - reefPressure(s) * 0.15
      h.vx = 0
      h.vy = 0
      h.aimX = s.player.x
      h.aimY = s.player.y
    }
  } else if (h.phase === 'aim') {
    // The last half second is locked, so every attack can be dodged by reading the line.
    if (h.remaining > 0.5) {
      h.aimX = s.player.x
      h.aimY = s.player.y
    }
    h.facing = h.aimX >= h.x ? 1 : -1
    if (h.remaining <= 0) {
      const dx = h.aimX - h.x,
        dy = h.aimY - h.y,
        d = Math.max(1, Math.hypot(dx, dy)),
        speed = c.finale ? 620 + roundFor(s).chapter * 8 : 510 + reefPressure(s) * 40
      h.vx = (dx / d) * speed
      h.vy = (dy / d) * speed
      h.phase = 'charge'
      h.remaining = 0.92
      h.closest = Infinity
      h.bitten = false
    }
  } else if (h.phase === 'charge') {
    h.x += h.vx * dt
    h.y += h.vy * dt
    const d = Math.hypot(s.player.x - h.x, s.player.y - h.y)
    h.closest = Math.min(h.closest, d)
    if (
      h.remaining <= 0 ||
      h.x < h.radius ||
      h.x > 2600 - h.radius ||
      h.y < h.radius ||
      h.y > 1600 - h.radius
    ) {
      h.phase = 'recover'
      h.remaining = c.finale ? 3.6 : 2.8 - reefPressure(s) * 0.25
      h.vx = 0
      h.vy = 0
      const contact = (h.radius + s.player.radius) * 0.72
      if (h.closest > contact && h.closest < contact + 100) {
        c.dodges++
        s.score += 120
        challengeBanner(s, 'dodge')
      }
    }
  } else {
    const tail = tailPosition(h)
    if (
      !h.bitten &&
      s.dash > 0 &&
      Math.hypot(s.player.x - tail.x, s.player.y - tail.y) <
        s.player.radius + h.radius * 0.48
    ) {
      h.bitten = true
      c.tailBites++
      s.score += c.finale ? 750 : 300
      if (!c.finale) s.mass += 18
      s.flash = 0.18
      s.popups.push({
        x: tail.x,
        y: tail.y - 25,
        text: c.finale ? '+750' : '+300',
        life: 1,
        color: '#ffe299',
      })
      if (c.finale) {
        c.bossHealth--
        if (c.bossHealth <= 0) {
          s.score += 2000
          completeRound(s)
          return
        }
      }
      challengeBanner(s, 'tail')
    }
    if (h.remaining <= 0) {
      h.phase = 'stalk'
      h.remaining = c.finale ? 1.2 : 2.2 - reefPressure(s) * 0.35
      h.bitten = false
    }
  }
  h.x = clamp(h.x, h.radius, 2600 - h.radius)
  h.y = clamp(h.y, h.radius, 1600 - h.radius)
}
export function catchShoal(s: OceanState, id: number | undefined) {
  const c = s.challenge
  if (!c?.shoal || id !== c.shoal.id) return
  c.shoal.caught++
  if (c.shoal.caught >= c.shoal.total) {
    s.score += 500
    s.lives = Math.min(3, s.lives + 1)
    c.shoals++
    c.shoal = null
    c.shoalAt = s.time + 16
    challengeBanner(s, 'shoalClear')
  }
}
export function challengeStars(s: OceanState) {
  if (s.phase !== 'won') return 0
  // One star for completion; extra stars reward efficient and skillful play.
  return (
    1 +
    Number(s.time <= 7200) +
    Number(
      s.lives >= 2 &&
        (s.challenge?.dodges ?? 0) + (s.challenge?.tailBites ?? 0) >= 5,
    )
  )
}

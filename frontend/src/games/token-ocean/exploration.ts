import type { OceanState, Swimmer } from './engine'
import type { Species } from './species'

export type OceanZone = 'reef' | 'twilight' | 'abyss'
export const zoneAt = (y: number): OceanZone =>
  y < 580 ? 'reef' : y < 1080 ? 'twilight' : 'abyss'
export const depthAt = (y: number) => Math.max(0, Math.round(y * 1.7))
export const isGentleGiant = (species: Species) =>
  ['manta', 'whale-shark', 'blue-whale', 'oarfish'].includes(species)
export const isLuminous = (species: Species) =>
  [
    'lanternfish',
    'hatchetfish',
    'anglerfish',
    'gulper-eel',
    'viperfish',
  ].includes(species)
export const habitatFloor = (species: Species) =>
  [
    'lanternfish',
    'hatchetfish',
    'viperfish',
    'gulper-eel',
    'giant-squid',
  ].includes(species)
    ? 590
    : 0
export interface Relic {
  id: number
  x: number
  y: number
  collected: boolean
}
export interface Migration {
  species: 'manta' | 'whale-shark' | 'blue-whale'
  x: number
  y: number
  facing: number
  radius: number
  remaining: number
  caught: number
  pearls: Array<{ x: number; y: number; caught: boolean }>
}
export interface Exploration {
  zone: OceanZone
  discovered: Species[]
  lastDiscovery: Species | null
  discoveryTime: number
  relics: Relic[]
  relicComplete: boolean
  migration: Migration | null
  migrationAt: number
  migrations: number
  message:
    'none' | 'relic' | 'relicComplete' | 'migration' | 'migrationClear' | 'zone'
  messageTime: number
}
export const createExploration = (): Exploration => ({
  zone: 'reef',
  discovered: [],
  lastDiscovery: null,
  discoveryTime: 0,
  relics: [
    { id: 0, x: 580, y: 800, collected: false },
    { id: 1, x: 1990, y: 1230, collected: false },
    { id: 2, x: 740, y: 1430, collected: false },
  ],
  relicComplete: false,
  migration: null,
  migrationAt: 12,
  migrations: 0,
  message: 'none',
  messageTime: 0,
})
function announce(s: OceanState, message: Exploration['message']) {
  s.exploration.message = message
  s.exploration.messageTime = 3.5
  s.event++
}
export function discover(s: OceanState, species: Species) {
  const e = s.exploration
  if (e.discovered.includes(species)) return
  e.discovered.push(species)
  e.lastDiscovery = species
  e.discoveryTime = 2.4
  s.event++
}
/** Distance checks use visible bodies; ambient giants are never edible or harmful. */
export function tickExploration(
  s: OceanState,
  dt: number,
  random: () => number,
) {
  if (s.phase !== 'playing') return
  const e = s.exploration,
    p = s.player
  e.messageTime = Math.max(0, e.messageTime - dt)
  e.discoveryTime = Math.max(0, e.discoveryTime - dt)
  const zone = zoneAt(p.y)
  if (zone !== e.zone) {
    e.zone = zone
    announce(s, 'zone')
  }
  for (const f of s.fish)
    if (Math.hypot(f.x - p.x, f.y - p.y) < p.radius + f.radius + 180)
      discover(s, f.species)
  const hunter = s.challenge?.hunter
  if (
    hunter &&
    Math.hypot(hunter.x - p.x, hunter.y - p.y) < hunter.radius + p.radius + 180
  )
    discover(s, 'hammerhead')
  if (s.challenge?.finale) {
    e.migration = null
    return
  }
  for (const r of e.relics) {
    if (!r.collected && Math.hypot(r.x - p.x, r.y - p.y) < p.radius + 24) {
      r.collected = true
      s.score += 600
      s.stamina = 100
      announce(s, 'relic')
    }
  }
  if (!e.relicComplete && e.relics.every((r) => r.collected)) {
    e.relicComplete = true
    s.score += 1800
    s.lives = Math.min(3, s.lives + 1)
    announce(s, 'relicComplete')
  }
  if (!e.migration && s.time >= e.migrationAt) {
    const facing = p.x > 1300 ? -1 : 1
    const species =
      zone === 'abyss'
        ? 'blue-whale'
        : zone === 'twilight'
          ? 'whale-shark'
          : 'manta'
    const x = Math.max(360, Math.min(2240, p.x + facing * 300)),
      y = Math.max(180, Math.min(1400, p.y + 110))
    e.migration = {
      species,
      x,
      y,
      facing,
      radius:
        species === 'blue-whale' ? 220 : species === 'whale-shark' ? 170 : 125,
      remaining: 18,
      caught: 0,
      pearls: Array.from({ length: 6 }, (_, i) => ({
        x: Math.max(90, Math.min(2510, x + facing * (i - 2) * 100)),
        y: Math.max(130, Math.min(1470, y + Math.sin(i * 1.2) * 72)),
        caught: false,
      })),
    }
    announce(s, 'migration')
  }
  const m = e.migration
  if (!m) return
  m.remaining = Math.max(0, m.remaining - dt)
  // Slow migration is readable; pearls stay on their route, never teleport after collection.
  m.x += m.facing * 28 * dt
  if (Math.hypot(m.x - p.x, m.y - p.y) < m.radius + p.radius + 200)
    discover(s, m.species)
  for (const pearl of m.pearls) {
    if (
      !pearl.caught &&
      Math.hypot(pearl.x - p.x, pearl.y - p.y) < p.radius + 22
    ) {
      pearl.caught = true
      m.caught++
      s.score += 90
      s.stamina = Math.min(100, s.stamina + 12)
    }
  }
  if (m.caught === m.pearls.length) {
    s.score += 700
    s.lives = Math.min(3, s.lives + 1)
    e.migrations++
    e.migration = null
    e.migrationAt = s.time + 22 + random() * 6
    announce(s, 'migrationClear')
  } else if (m.remaining <= 0) {
    e.migration = null
    e.migrationAt = s.time + 18
  }
}
/** Seeded by run RNG. Large residents complement schools; they are not surprise predators. */
export function makeResident(
  id: number,
  species: Species,
  x: number,
  y: number,
  radius: number,
  facing: number,
): Swimmer {
  return {
    id,
    species,
    x,
    y,
    radius,
    vx: facing * 35,
    vy: 0,
    golden: false,
    turn: id * 0.71,
  }
}

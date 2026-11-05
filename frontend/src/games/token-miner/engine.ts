// Local practice rules only. Scores from this engine are never a billing authority.
export const WORLD = {
  width: 960,
  height: 600,
  originX: 480,
  originY: 132,
  restLength: 30,
}
import { LEVELS, earnsUpgrade } from './levels'
export { LEVELS, earnsUpgrade } from './levels'

export const UPGRADE_KINDS = ['winch', 'clock', 'assay'] as const
export type Upgrade = (typeof UPGRADE_KINDS)[number]
export type Equipment = Record<Upgrade, number>
export type MineCondition = 'stable' | 'crystal' | 'dense' | 'loose'
export type OreKind = 'token' | 'large' | 'gem' | 'rock'
export type GameStatus =
  | 'ready'
  | 'playing'
  | 'paused'
  | 'won'
  | 'lost'
  | 'complete'
export interface Ore {
  id: number
  kind: OreKind
  x: number
  y: number
  radius: number
  value: number
  weight: number
  collected: boolean
  lucky?: boolean
}
export interface GameState {
  status: GameStatus
  level: number
  equipment: Equipment
  condition: MineCondition
  gems: number
  score: number
  total: number
  remaining: number
  bombs: number
  angle: number
  direction: number
  length: number
  hook: 'swinging' | 'extending' | 'retracting'
  grabbed: Ore | null
  ores: Ore[]
  feedback: { value: number; x: number; y: number; age: number } | null
}

function randomSource(seed: number) {
  let n = seed >>> 0
  return () => {
    n += 0x6d2b79f5
    let t = Math.imul(n ^ (n >>> 15), 1 | n)
    t ^= t + Math.imul(t ^ (t >>> 7), 61 | t)
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296
  }
}

export function createGame(
  seed: number,
  level = 0,
  carriedTotal = 0,
  equipment: Equipment = { winch: 0, clock: 0, assay: 0 },
): GameState {
  const random = randomSource(seed)
  const safeLevel = Math.max(0, Math.min(LEVELS.length - 1, Math.trunc(level) || 0))
  const gear: Equipment = { winch: 0, clock: 0, assay: 0 }
  for (const kind of UPGRADE_KINDS)
    gear[kind] = Math.max(0, Math.min(3, Math.trunc(equipment[kind]) || 0))
  const condition: MineCondition = safeLevel === 0 ? 'stable'
    : (['stable', 'crystal', 'dense', 'loose'] as const)[Math.floor(random() * 4)]!
  const shaft = LEVELS[safeLevel]!
  const ores: Ore[] = []
  // Jittered cells keep the field varied without overlapping or unreachable ores.
  const kinds: OreKind[] = [
    'token',
    'rock',
    'large',
    'token',
    'rock',
    'token',
    'large',
    'gem',
    'token',
    'rock',
    'large',
    'gem',
    'token',
    'large',
    'large',
    'gem',
    'rock',
    'token',
    'large',
    'rock',
    'gem',
  ]
  for (let i = kinds.length - 1; i > 0; i--) {
    const j = Math.floor(random() * (i + 1))
    ;[kinds[i], kinds[j]] = [kinds[j]!, kinds[i]!]
  }
  for (let i = 0; i < kinds.length; i++) {
    const row = Math.floor(i / 7), col = i % 7
    // All patterns use separated cells, but their value and obstruction routes differ.
    const kind: OreKind = safeLevel > 0 && [15, 17, 19].includes(i) ? 'gem' : safeLevel === 0 ? kinds[i]!
      : shaft.layout === 'veins' ? (col % 3 === 0 ? 'rock' : row === 2 ? 'gem' : 'large')
      : shaft.layout === 'deep' ? (row === 0 ? 'rock' : row === 2 ? 'gem' : kinds[i]!)
      : shaft.layout === 'walls' ? ((col + row) % 3 === 0 ? 'rock' : kinds[i]!)
      : shaft.layout === 'crystals' ? (i % 4 === 0 ? 'gem' : kinds[i]!) : kinds[i]!

    const lucky = safeLevel > 0 && kind !== 'rock' && random() < 0.12
    const stats = {
      token: { radius: 19, value: 100, weight: 1 },
      large: { radius: 32, value: 250, weight: 2.4 },
      gem: { radius: 18, value: 400, weight: 0.8 },
      rock: { radius: 27, value: 20, weight: 3.5 },
    }[kind]
    ores.push({
      id: i,
      kind,
      ...stats,
      lucky,
      value: Math.round(stats.value * (safeLevel > 0 ? 0.9 + random() * 0.2 : 1)
        * (lucky ? 1.7 : 1)
        * (kind === 'gem' ? 1 + gear.assay * 0.2 : 1)
        * (condition === 'crystal' && kind === 'gem' ? 1.35 : condition === 'dense' && kind === 'large' ? 1.3 : 1)),
      weight: stats.weight * (condition === 'dense' && kind === 'large' ? 1.25 : condition === 'loose' && kind === 'rock' ? 0.5 : 1),
      x: 120 + (i % 7) * 120 + (random() - 0.5) * 32,
      y: 285 + Math.floor(i / 7) * 110 + (random() - 0.5) * 25,
      collected: false,
    })
  }
  // Keep a meaningful but feasible value quota across all condition/layout combinations.
  const available = ores.reduce((sum, o) => sum + o.value, 0)
  const required = shaft.target * 1.28
  if (available < required) {
    const factor = required / available
    for (const ore of ores) ore.value = Math.ceil(ore.value * factor)
  }
  return {
    status: 'ready',
    level: safeLevel,
    equipment: gear,
    condition,
    score: 0,
    gems: 0,
    total: carriedTotal,
    remaining: LEVELS[safeLevel]!.seconds + gear.clock * 10,
    bombs: 2,
    angle: -0.6,
    direction: 1,
    length: WORLD.restLength,
    hook: 'swinging',
    grabbed: null,
    ores,
    feedback: null,
  }
}

export function launch(state: GameState) {
  if (state.status !== 'playing' || state.hook !== 'swinging') return false
  state.hook = 'extending'
  return true
}

export function discard(state: GameState) {
  if (state.status !== 'playing' || !state.grabbed || state.bombs <= 0)
    return false
  state.grabbed.collected = true
  state.grabbed = null
  state.bombs--
  return true
}

export function hookPosition(state: GameState) {
  return {
    x: WORLD.originX + Math.sin(state.angle) * state.length,
    y: WORLD.originY + Math.cos(state.angle) * state.length,
  }
}

function finish(state: GameState) {
  state.remaining = Math.max(0, state.remaining)
  state.status =
    state.score >= LEVELS[state.level]!.target && state.gems >= LEVELS[state.level]!.gems
      ? state.level === LEVELS.length - 1
        ? 'complete'
        : 'won'
      : 'lost'
}

function physics(state: GameState, dt: number) {
  if (state.feedback) {
    state.feedback.age += dt
    if (state.feedback.age > 1.3) state.feedback = null
  }
  if (state.hook === 'swinging') {
    state.angle += state.direction * LEVELS[state.level]!.swingSpeed * dt
    const limit = 1.22
    if (Math.abs(state.angle) >= limit) {
      state.angle = Math.sign(state.angle) * limit
      state.direction *= -1
    }
    return
  }
  if (state.hook === 'extending') {
    state.length += 470 * dt
    const tip = hookPosition(state)
    const ore = state.ores.find(
      (o) =>
        !o.collected && Math.hypot(o.x - tip.x, o.y - tip.y) < o.radius + 8,
    )
    if (ore) {
      state.grabbed = ore
      state.hook = 'retracting'
    } else if (
      tip.x < 24 ||
      tip.x > WORLD.width - 24 ||
      tip.y > WORLD.height - 24
    ) {
      state.hook = 'retracting'
    }
    return
  }
  state.length -= (state.grabbed ? 340 * (1 + state.equipment.winch * 0.22) / state.grabbed.weight : 650) * dt
  if (state.length > WORLD.restLength) return
  state.length = WORLD.restLength
  state.hook = 'swinging'
  if (state.grabbed) {
    const value = state.grabbed.value
    if (state.grabbed.kind === 'gem') state.gems++
    state.grabbed.collected = true
    state.score += value
    state.total += value
    state.feedback = { value, x: WORLD.originX, y: WORLD.originY + 5, age: 0 }
    state.grabbed = null
    if (state.score >= LEVELS[state.level]!.target && state.gems >= LEVELS[state.level]!.gems) finish(state)
  }
  if (state.ores.every((o) => o.collected)) finish(state)
}

/** Small substeps prevent collision tunneling; timeouts never award a dangling ore. */
export function advance(state: GameState, seconds: number) {
  if (state.status !== 'playing' || !Number.isFinite(seconds) || seconds <= 0)
    return
  let remaining = Math.min(seconds, state.remaining)
  while (remaining > 0.000001 && state.status === 'playing') {
    const dt = Math.min(remaining, 1 / 120)
    state.remaining = Math.max(0, state.remaining - dt)
    remaining -= dt
    if (state.remaining <= 0.000001) {
      state.remaining = 0
      finish(state)
      break
    }
    physics(state, dt)
  }
}

/** A single upgrade is earned per cleared shaft. Retries retain earned gear. */
export function nextShaft(state: GameState, seed: number, upgrade: Upgrade | null = null): GameState | null {
  if (state.status !== 'won') return null
  const equipment = { ...state.equipment }
  if (earnsUpgrade(state.level)) {
    if (!upgrade || !UPGRADE_KINDS.includes(upgrade) || equipment[upgrade] >= 3) return null
    equipment[upgrade]++
  } else if (upgrade !== null) return null
  return createGame(seed, state.level + 1, state.total, equipment)
}

import { ROUNDS, ROUND_COUNT, roundFor } from './campaign'
import { createVoyage, catchVoyage, tickVoyage, voyageComplete, type Voyage } from './voyage'
import { type Species } from './species'
import {
  createExploration,
  tickExploration,
  zoneAt,
  isGentleGiant,
  makeResident,
  habitatFloor,
  type Exploration,
} from './exploration'
import {
  reefPressure,
  swimmerRadius,
  swimmerRelation,
  tickAmbush,
} from './ecology'
export type { Species } from './species'
import {
  createChallenge,
  tickChallenge,
  beginFinale,
  catchShoal,
  type OceanChallenge,
} from './challenge'
/** Pure, local arcade simulation; scores never enter API billing. */
export const WORLD = { width: 2600, height: 1600 }
export const evolutions = [
  { id: 'minnow', mass: 0, radius: 19 },
  { id: 'bluegill', mass: 35, radius: 31 },
  { id: 'perch', mass: 105, radius: 43 },
  { id: 'koi', mass: 240, radius: 58 },
  { id: 'tuna', mass: 480, radius: 75 },
  { id: 'moonfish', mass: 850, radius: 94 },
] as const
export type OceanPhase = 'ready' | 'playing' | 'over' | 'won' | 'cleared'
export interface Swimmer {
  id: number
  x: number
  y: number
  vx: number
  vy: number
  radius: number
  species: Species
  golden: boolean
  shoal?: number
  school?: number
  turn: number
  ambush?: {
    phase: 'cruise' | 'aim' | 'charge' | 'recover'
    remaining: number
    x: number
    y: number
  }
}
export interface Particle {
  x: number
  y: number
  vx: number
  vy: number
  life: number
  maxLife: number
  color: string
  radius: number
}
export interface Popup {
  x: number
  y: number
  text: string
  life: number
  color: string
  value?: number
}
export interface Gulp {
  x: number
  y: number
  radius: number
  species: Species
  facing: number
  life: number
}
export interface OceanState {
  phase: OceanPhase
  round: number
  roundTime: number
  challenge: OceanChallenge | null
  exploration: Exploration
  voyage: Voyage
  time: number
  score: number
  eaten: number
  mass: number
  level: number
  lives: number
  combo: number
  bestCombo: number
  comboTime: number
  frenzy: number
  frenzyCooldown: number
  frenzyCharge: number
  stamina: number
  bite: number
  feast: number
  feastCount: number
  gulpChain: number
  gulpTime: number
  schoolTimer: number
  gulps: Gulp[]
  dash: number
  invincible: number
  shake: number
  flash: number
  player: {
    x: number
    y: number
    vx: number
    vy: number
    radius: number
    facing: number
  }
  fish: Swimmer[]
  particles: Particle[]
  popups: Popup[]
  spawnTimer: number
  nextId: number
  event: number
  notice: 'welcome' | 'eat' | 'evolve' | 'hurt' | 'frenzy' | 'won' | 'over' | 'clear'
}
export interface OceanInput {
  x: number
  y: number
  target?: { x: number; y: number }
  dash?: boolean
}
const clamp = (n: number, min: number, max: number) =>
  Math.max(min, Math.min(max, n))
export const nextEvolution = (s: OceanState) => evolutions[s.level + 1] ?? null
export const growthProgress = (s: OceanState) => {
  const next = nextEvolution(s),
    base = evolutions[s.level]!.mass
  return next ? clamp((s.mass - base) / (next.mass - base), 0, 1) : 1
}
export function relation(
  playerRadius: number,
  fishRadius: number,
): 'prey' | 'peer' | 'danger' {
  return playerRadius >= fishRadius * 1.15
    ? 'prey'
    : fishRadius >= playerRadius * 1.15
      ? 'danger'
      : 'peer'
}
/** Frenzy is a short power reversal against ordinary fish, never the hunter. */
export function fishRelation(s: OceanState, radius: number) {
  return s.frenzy > 0 && radius <= s.player.radius * 1.6
    ? 'prey'
    : relation(s.player.radius, radius)
}
export function createOcean(seed = Date.now(), round = 0, carriedScore = 0, lives = 3): OceanState {
  const safeRound = Math.max(0, Math.min(ROUND_COUNT - 1, Math.trunc(round) || 0))
  const form = Math.min(4, safeRound % 6)
  const y = { reef: 300, twilight: 830, abyss: 1300 }[ROUNDS[safeRound]!.zone]
  return {
    phase: 'ready',
    round: safeRound,
    roundTime: 0,
    challenge: createChallenge(),
    exploration: createExploration(),
    voyage: createVoyage(seed, safeRound),
    time: 0,
    score: carriedScore,
    eaten: 0,
    mass: evolutions[form]!.mass,
    level: form,
    lives,
    combo: 0,
    bestCombo: 0,
    comboTime: 0,
    frenzy: 0,
    frenzyCooldown: 0,
    frenzyCharge: 0,
    stamina: 100,
    bite: 0,
    feast: 0,
    feastCount: 0,
    gulpChain: 0,
    gulpTime: 0,
    schoolTimer: 5,
    gulps: [],
    dash: 0,
    invincible: 2.5,
    shake: 0,
    flash: 0,
    player: {
      x: WORLD.width / 2,
      y,
      vx: 0,
      vy: 0,
      radius: evolutions[form]!.radius,
      facing: 1,
    },
    fish: [],
    particles: [],
    popups: [],
    spawnTimer: 0,
    nextId: 0,
    event: 0,
    notice: 'welcome',
  }
}
function randomRange(random: () => number, lo: number, hi: number) {
  return lo + random() * (hi - lo)
}
function spawn(s: OceanState, random: () => number, opening = false) {
  const pressure = reefPressure(s),
    edible = s.challenge?.finale ? 1 : 0.63 - pressure * 0.045,
    neutral = 0.84 - pressure * 0.025,
    roll = random(),
    radius = s.player.radius
  const tier = Math.min(
    evolutions.length - 1,
    Math.max(0, s.level + (roll < edible ? -1 : roll < neutral ? 0 : 1)),
  )
  let size =
    roll < edible
      ? randomRange(random, Math.max(9, radius * 0.42), radius * 0.76)
      : roll < neutral
        ? radius * randomRange(random, 0.94, 1.08)
        : radius * randomRange(random, 1.35, 1.8)
  const prey: Species[] =
    s.level < 2
      ? ['minnow', 'clownfish', 'sardine', 'bluegill', 'butterflyfish']
      : [
          'clownfish',
          'butterflyfish',
          'sardine',
          evolutions[tier]!.id,
          'koi',
          'moonfish',
        ]
  const predators: Species[] =
    pressure > 0
      ? ['perch', 'tuna', 'barracuda', 'anglerfish']
      : ['perch', 'tuna']
  const pool =
    roll < edible
      ? prey
      : roll < neutral
        ? (['clownfish', 'butterflyfish', evolutions[tier]!.id] as Species[])
        : predators
  let species =
    pool[Math.min(pool.length - 1, Math.floor(random() * pool.length))]!
  // Distinct hazards arrive after the opening; small silhouettes do not mean safe food.
  if (s.challenge && !s.challenge.finale && s.roundTime >= 10) {
    const special = random()
    if (special < 0.12) {
      species = 'pufferfish'
      size = radius * randomRange(random, 0.58, 0.72)
    } else if (pressure > 0 && special < 0.19) {
      species = 'lionfish'
      size = radius * randomRange(random, 0.65, 0.95)
    }
  }
  const angle = random() * Math.PI * 2
  // Spawn beyond the visible close neighborhood. No enemies materialize on the player.
  const distance =
    opening && roll < edible
      ? randomRange(random, 130, 440)
      : randomRange(random, 600, 1050)
  let x = clamp(
    s.player.x + Math.cos(angle) * distance,
    size + 15,
    WORLD.width - size - 15,
  )
  let y = clamp(
    s.player.y + Math.sin(angle) * distance,
    size + 15,
    WORLD.height - size - 15,
  )
  const zone = zoneAt(y)
  if (zone !== 'reef' && species !== 'pufferfish' && species !== 'lionfish') {
    const deepPrey: Species[] =
      zone === 'abyss'
        ? ['lanternfish', 'hatchetfish', 'lanternfish', 'moonfish']
        : ['sardine', 'hatchetfish', 'lanternfish', 'butterflyfish']
    const deepThreats: Species[] =
      zone === 'abyss'
        ? ['viperfish', 'gulper-eel', 'giant-squid', 'anglerfish', 'hammerhead']
        : ['barracuda', 'tuna', 'viperfish']
    const local =
      roll < edible
        ? deepPrey
        : roll < neutral
          ? (['hatchetfish', 'lanternfish'] as Species[])
          : deepThreats
    species =
      local[Math.min(local.length - 1, Math.floor(random() * local.length))]!
    if (roll >= neutral && !s.challenge?.finale)
      size = Math.min(170, size * (zone === 'abyss' ? 1.45 : 1.12))
  }
  x = clamp(x, size + 15, WORLD.width - size - 15)
  y = clamp(
    y,
    Math.max(size + 15, habitatFloor(species)),
    WORLD.height - size - 15,
  )
  if (Math.hypot(x - s.player.x, y - s.player.y) < s.player.radius + size + 110)
    return
  const direction = Math.cos(angle) < 0 ? 1 : -1
  s.fish.push({
    id: s.nextId++,
    x,
    y,
    vx: direction * randomRange(random, 35, 78),
    vy: randomRange(random, -18, 18),
    radius: size,
    species,
    golden:
      species !== 'pufferfish' &&
      species !== 'lionfish' &&
      roll < edible &&
      random() < (s.voyage.weather === 'golden' ? 0.32 : 0.09),
    turn: random() * Math.PI * 2,
  })
}
export function startOcean(s: OceanState, random: () => number = Math.random) {
  if (s.phase !== 'ready') return false
  s.phase = 'playing'
  for (let i = 0; i < 100; i++) spawn(s, random, true)
  // A few large residents establish scale. Gentle giants never join collision combat.
  for (const [species, x, y, radius] of [
    ['manta', 480, 440, 110],
    ['whale-shark', 2120, 840, 160],
    ['oarfish', 430, 1190, 150],
    ['blue-whale', 2040, 1310, 220],
  ] as const)
    s.fish.push(
      makeResident(s.nextId++, species, x, y, radius, x < 1300 ? 1 : -1),
    )
  // Deep waters already contain prey and readable distant threats before the first descent.
  for (let i = 0; i < 26; i++) {
    const x = 120 + random() * 2360,
      y = 1100 + random() * 330
    const species = (
      [
        'lanternfish',
        'hatchetfish',
        'lanternfish',
        'viperfish',
        'gulper-eel',
        'giant-squid',
      ] as const
    )[i % 6]!
    const radius =
      i % 6 < 3
        ? 10 + random() * 8
        : 58 + random() * (species === 'giant-squid' ? 80 : 35)
    s.fish.push(makeResident(s.nextId++, species, x, y, radius, i % 2 ? 1 : -1))
  }
  s.fish.push(makeResident(s.nextId++, 'hammerhead', 1780, 1030, 90, -1))
  // Two compact schools make the first dash a deliberate multi-fish catch.
  for (let group = 0; group < 2; group++)
    for (let i = 0; i < 5; i++) {
      const x = s.player.x + 175 + (i % 3) * 38,
        y = s.player.y + (group ? 90 : 0) + Math.floor(i / 3) * 30
      s.fish.push({
        id: s.nextId++,
        x,
        y,
        vx: 38,
        vy: 0,
        radius: 10 + (i % 3),
        species: group === 0 ? 'clownfish' : 'sardine',
        golden: group === 1 && i === 4,
        turn: group,
        school: -1 - group,
      })
    }
  return true
}
export function dashOcean(s: OceanState) {
  if (s.phase !== 'playing' || s.stamina < 38 || s.dash > 0) return false
  s.stamina -= 38
  s.dash = 0.38
  return true
}
function burst(
  s: OceanState,
  x: number,
  y: number,
  color: string,
  count: number,
  random: () => number,
) {
  for (let i = 0; i < count; i++) {
    const angle = random() * Math.PI * 2,
      speed = randomRange(random, 35, 145)
    const life = randomRange(random, 0.3, 0.7)
    s.particles.push({
      x,
      y,
      vx: Math.cos(angle) * speed,
      vy: Math.sin(angle) * speed,
      life,
      maxLife: life,
      color,
      radius: randomRange(random, 2, 5),
    })
  }
  if (s.particles.length > 160) s.particles.splice(0, s.particles.length - 160)
}
function event(s: OceanState, notice: OceanState['notice']) {
  s.notice = notice
  s.event++
}
function consume(s: OceanState, fish: Swimmer, random: () => number) {
  const revenge = relation(s.player.radius, fish.radius) === 'danger',
    risky = fish.species === 'pufferfish' || fish.species === 'lionfish',
    reward = (revenge ? 2 : 1) * (risky ? 2 : 1)
  if (fish.species === 'sardine') s.stamina = Math.min(100, s.stamina + 16)
  s.gulps.push({
    x: fish.x,
    y: fish.y,
    radius: fish.radius,
    species: fish.species,
    facing: fish.vx >= 0 ? 1 : -1,
    life: 0.22,
  })
  if (s.gulps.length > 16) s.gulps.shift()
  s.bite = 0.26
  if (s.gulpTime > 0) s.gulpChain++
  else {
    s.gulpChain = 1
    s.gulpTime = 0.55
  }
  if (s.gulpChain >= 3) {
    s.feast = 0.85
    s.feastCount = s.gulpChain
  }
  s.eaten++
  s.combo++
  s.bestCombo = Math.max(s.combo, s.bestCombo)
  s.comboTime = 3.4
  const multiplier = 1 + Math.min(4, Math.floor(s.combo / 3))
  const value = Math.round(
    (8 + fish.radius * 1.6) * multiplier * (fish.golden ? 3 : 1),
  )
  s.score += value * reward
  s.mass +=
    Math.max(
      4,
      Math.round(
        fish.radius * (s.level > 0 ? 0.34 : 0.72),
      ),
    ) * (fish.golden ? 2 : 1)
  catchShoal(s, fish.shoal)
  catchVoyage(s, fish)
  const earned = value * reward,
    last = s.popups[s.popups.length - 1]
  if (
    last?.value !== undefined &&
    last.life > 0.3 &&
    Math.hypot(last.x - fish.x, last.y - fish.y) < 130
  ) {
    last.value += earned
    last.text = `+${last.value}`
    last.life = 0.7
    last.x = (last.x + fish.x) * 0.5
    last.y = Math.min(last.y, fish.y - fish.radius)
  } else
    s.popups.push({
      x: fish.x,
      y: fish.y - fish.radius,
      text: `+${earned}`,
      value: earned,
      life: 0.7,
      color: fish.golden ? '#ffe095' : '#caffdf',
    })
  burst(
    s,
    fish.x,
    fish.y,
    fish.golden ? '#ffe299' : '#b4f5d5',
    fish.golden ? 20 : 10,
    random,
  )
  if (s.popups.length > 5) s.popups.shift()
  if (s.phase === 'playing') event(s, 'eat')
  if (s.frenzy <= 0 && s.frenzyCooldown <= 0) {
    s.frenzyCharge++
    if (s.frenzyCharge >= 8) {
      s.frenzy = 4.5
      s.frenzyCooldown = 10.5
      s.frenzyCharge = 0
      s.flash = 0.22
      event(s, 'frenzy')
    }
  }
  evolve(s, random)
}
function evolve(s: OceanState, random: () => number) {
  while (
    s.level < evolutions.length - 1 &&
    s.mass >= evolutions[s.level + 1]!.mass
  ) {
    s.level++
    s.player.radius = evolutions[s.level]!.radius
    s.invincible = Math.max(s.invincible, 0.65)
    s.flash = 0.32
    s.stamina = 100
    burst(s, s.player.x, s.player.y, '#ffdc91', 32, random)
    event(s, 'evolve')
  }
  if (s.mass >= evolutions[evolutions.length - 1]!.mass) {
    if (roundFor(s).boss && voyageComplete(s)) beginFinale(s)
  }

}
function hurt(s: OceanState, dx: number, dy: number, random: () => number) {
  if (s.invincible > 0 || s.dash > 0) return
  s.lives--
  s.invincible = 2.5
  s.combo = 0
  s.comboTime = 0
  s.frenzyCharge = 0
  s.frenzy = 0
  s.shake = 0.3
  s.player.vx = Math.sign(dx || 1) * 180
  s.player.vy = Math.sign(dy || 1) * 120
  burst(s, s.player.x, s.player.y, '#ffb5a2', 22, random)
  event(s, 'hurt')
  if (s.lives <= 0) {
    s.phase = 'over'
    event(s, 'over')
  }
}
function step(
  s: OceanState,
  dt: number,
  input: OceanInput,
  random: () => number,
) {
  if (s.phase !== 'playing') return
  s.time += dt
  s.roundTime += dt
  if (s.roundTime >= roundFor(s).seconds) {
    s.phase = 'over'
    event(s, 'over')
    return
  }
  tickVoyage(s, dt, random)
  if (s.phase !== 'playing') return
  s.bite = Math.max(0, s.bite - dt)
  s.feast = Math.max(0, s.feast - dt)
  s.gulpTime = Math.max(0, s.gulpTime - dt)
  for (const g of s.gulps) {
    g.life -= dt
    const pull = 1 - Math.exp(-19 * dt)
    g.x += (s.player.x + s.player.facing * s.player.radius * 0.55 - g.x) * pull
    g.y += (s.player.y - g.y) * pull
  }
  s.gulps = s.gulps.filter((g) => g.life > 0)
  s.dash = Math.max(0, s.dash - dt)
  s.invincible = Math.max(0, s.invincible - dt)
  s.frenzy = Math.max(0, s.frenzy - dt)
  s.frenzyCooldown = Math.max(0, s.frenzyCooldown - dt)
  s.shake = Math.max(0, s.shake - dt)
  s.flash = Math.max(0, s.flash - dt)
  s.stamina = Math.min(
    100,
    s.stamina + (s.voyage.weather === 'current' ? 36 : s.frenzy > 0 ? 32 : 19) * dt,
  )
  s.comboTime = Math.max(0, s.comboTime - dt)
  if (!s.comboTime && s.combo > 0) {
    s.combo = 0
    s.frenzyCharge = 0
  }
  let x = Number.isFinite(input.x) ? input.x : 0,
    y = Number.isFinite(input.y) ? input.y : 0
  if (
    input.target &&
    Number.isFinite(input.target.x) &&
    Number.isFinite(input.target.y)
  ) {
    const dx = input.target.x - s.player.x,
      dy = input.target.y - s.player.y,
      distance = Math.hypot(dx, dy)
    x = distance > 10 ? dx / Math.max(90, distance) : 0
    y = distance > 10 ? dy / Math.max(90, distance) : 0
  }
  const length = Math.max(1, Math.hypot(x, y))
  x /= length
  y /= length
  const speed =
    (235 - s.level * 6) * (s.dash > 0 ? 2.25 : s.frenzy > 0 ? 1.28 : 1) * (s.voyage.weather === 'current' ? 1.18 : 1)
  const blend = 1 - Math.exp(-12 * dt)
  s.player.vx += (x * speed - s.player.vx) * blend
  s.player.vy += (y * speed - s.player.vy) * blend
  s.player.x = clamp(
    s.player.x + s.player.vx * dt,
    s.player.radius,
    WORLD.width - s.player.radius,
  )
  s.player.y = clamp(
    s.player.y + s.player.vy * dt,
    s.player.radius,
    WORLD.height - s.player.radius,
  )
  if (Math.abs(s.player.vx) > 8) s.player.facing = s.player.vx > 0 ? 1 : -1
  s.spawnTimer -= dt
  const fishCap = s.challenge?.finale ? 80 : 144 + reefPressure(s) * 12
  if (s.spawnTimer <= 0) {
    // Recycle only distant non-event fish; a full world must not stop new species arriving.
    const distant = s.fish
      .filter(
        (f) =>
          f.shoal === undefined &&
          !isGentleGiant(f.species) &&
          Math.hypot(f.x - s.player.x, f.y - s.player.y) > 1150,
      )
      .slice(0, 3)
    const ids = new Set(distant.map((f) => f.id))
    s.fish = s.fish.filter((f) => !ids.has(f.id))
    const count = Math.max(0, Math.min(5, fishCap - s.fish.length))
    for (let i = 0; i < count; i++) spawn(s, random)
    s.spawnTimer = 0.45
  }
  s.schoolTimer -= dt
  if (s.schoolTimer <= 0 && s.fish.length < fishCap - 6) {
    const group = s.nextId++,
      direction = random() < 0.5 ? -1 : 1
    const cx = clamp(s.player.x + direction * 460, 120, WORLD.width - 120),
      cy = clamp(s.player.y + (random() - 0.5) * 360, 120, WORLD.height - 120)
    for (let i = 0; i < 9 && s.fish.length < fishCap; i++) {
      const radius = Math.max(9, s.player.radius * (0.4 + random() * 0.2)),
        x = cx + (i % 3) * 30,
        y = cy + Math.floor(i / 3) * 32
      if (
        Math.hypot(x - s.player.x, y - s.player.y) >
        s.player.radius + radius + 110
      )
        s.fish.push({
          id: s.nextId++,
          x,
          y,
          vx: -direction * 52,
          vy: 0,
          radius,
          species: (zoneAt(y) === 'reef'
            ? (['clownfish', 'sardine', 'butterflyfish'] as const)
            : (['lanternfish', 'hatchetfish', 'sardine'] as const))[
            Math.floor(random() * 3)
          ]!,
          golden: false,
          turn: direction === 1 ? Math.PI : 0,
          school: group,
        })
    }
    s.schoolTimer = s.voyage.weather === 'bloom' ? 2.5 : 4.5 + random() * 3
  }
  tickExploration(s, dt, random)
  tickChallenge(s, dt, random)
  evolve(s, random)
  if (s.phase !== 'playing') return
  const hunter = s.challenge?.hunter
  if (
    hunter &&
    hunter.phase !== 'recover' &&
    Math.hypot(s.player.x - hunter.x, s.player.y - hunter.y) <
      (hunter.radius + s.player.radius) * 0.72
  ) {
    hurt(s, s.player.x - hunter.x, s.player.y - hunter.y, random)
    if (s.phase !== 'playing') return
  }
  // Keep schools cohesive until startled; chasing a shoal should offer a route through it.
  const groups = new Map<number, { x: number; y: number; count: number }>()
  for (const f of s.fish)
    if (f.school !== undefined) {
      const g = groups.get(f.school) ?? { x: 0, y: 0, count: 0 }
      g.x += f.x
      g.y += f.y
      g.count++
      groups.set(f.school, g)
    }
  // Resolve threats before prey: eating and evolving cannot bypass a same-frame hit.
  const ordered = [...s.fish].sort(
    (a, b) =>
      Number(swimmerRelation(s, b) === 'danger') -
        Number(swimmerRelation(s, a) === 'danger') || b.radius - a.radius,
  )
  const removed = new Set<number>()
  for (const fish of ordered) {
    if (isGentleGiant(fish.species)) {
      fish.x += fish.vx * dt
      const preferredDepth =
        fish.species === 'blue-whale'
          ? 1310
          : fish.species === 'oarfish'
            ? 1190
            : fish.species === 'whale-shark'
              ? 840
              : 440
      fish.y = preferredDepth + Math.sin(s.time * 0.35 + fish.turn) * 45
      if (fish.x < fish.radius || fish.x > WORLD.width - fish.radius) {
        fish.x = clamp(fish.x, fish.radius, WORLD.width - fish.radius)
        fish.vx *= -1
      }
      continue
    }
    const dx = s.player.x - fish.x,
      dy = s.player.y - fish.y,
      distance = Math.hypot(dx, dy)
    const kind = swimmerRelation(s, fish),
      radius = swimmerRadius(s, fish),
      passive = fish.species === 'pufferfish' || fish.species === 'lionfish',
      pressure = reefPressure(s)
    const chase =
      kind === 'danger' &&
      !passive &&
      distance < (fish.species === 'anglerfish' ? 210 : 330 + pressure * 45) &&
      s.invincible <= 0
    const sucking =
      kind === 'prey' &&
      (s.dash > 0 || s.frenzy > 0) &&
      distance < (s.frenzy > 0 ? 35 : 18) + s.player.radius * 1.3 &&
      (dx * s.player.facing < s.player.radius * 0.5 || s.frenzy > 0)
    const flee =
      !sucking && kind === 'prey' && distance < (fish.shoal ? 75 : 115)
    const swimSpeed =
      kind === 'danger'
        ? passive
          ? 35
          : fish.species === 'anglerfish'
            ? 170 + pressure * 10
            : 115 + s.level * 4 + pressure * 12
        : flee
          ? fish.species === 'sardine'
            ? 158
            : 100
          : 60
    if (tickAmbush(s, fish, dt, kind === 'danger')) {
      // Locked ambush motion overrides ordinary chasing and school steering.
    } else if (sucking) {
      const mix = 1 - Math.exp(-18 * dt),
        speed = 440 + s.player.radius * 3
      fish.vx += ((dx / Math.max(1, distance)) * speed - fish.vx) * mix
      fish.vy += ((dy / Math.max(1, distance)) * speed - fish.vy) * mix
    } else if (chase || flee) {
      const sign = chase ? 1 : -1,
        mix = 1 - Math.exp(-2 * dt)
      fish.vx +=
        ((dx / Math.max(1, distance)) * swimSpeed * sign - fish.vx) * mix
      fish.vy +=
        ((dy / Math.max(1, distance)) * swimSpeed * sign - fish.vy) * mix
    } else if (fish.shoal && s.challenge?.shoal?.id === fish.shoal) {
      // Timed fish regroup after fleeing; the event marker stays useful throughout the chase.
      const shoal = s.challenge.shoal,
        tx = shoal.x + Math.cos(s.time * 0.6 + fish.turn) * 60,
        ty = shoal.y + Math.sin(s.time * 0.6 + fish.turn) * 55
      const dx = tx - fish.x,
        dy = ty - fish.y,
        d = Math.max(35, Math.hypot(dx, dy)),
        mix = 1 - Math.exp(-2 * dt)
      fish.vx += ((dx / d) * 65 - fish.vx) * mix
      fish.vy += ((dy / d) * 65 - fish.vy) * mix
    } else if (fish.school !== undefined && groups.has(fish.school)) {
      const group = groups.get(fish.school)!,
        gx = group.x / group.count,
        gy = group.y / group.count,
        mix = 1 - Math.exp(-1.6 * dt)
      const flow = Math.cos(fish.turn) > 0 ? 52 : -52
      fish.vx += (flow + (gx - fish.x) * 0.7 - fish.vx) * mix
      fish.vy +=
        ((gy - fish.y) * 0.7 +
          Math.sin(s.time * 0.8 + fish.turn) * 14 -
          fish.vy) *
        mix
    } else fish.vy += (Math.sin(s.time * 0.9 + fish.turn) * 22 - fish.vy) * dt
    fish.x += fish.vx * dt
    fish.y += fish.vy * dt
    if (fish.x < fish.radius || fish.x > WORLD.width - fish.radius) {
      fish.x = clamp(fish.x, fish.radius, WORLD.width - fish.radius)
      fish.vx *= -1
    }
    const minDepth = Math.max(fish.radius, habitatFloor(fish.species))
    if (fish.y < minDepth || fish.y > WORLD.height - fish.radius) {
      fish.y = clamp(fish.y, minDepth, WORLD.height - fish.radius)
      fish.vy *= -1
    }
    const contact = Math.hypot(s.player.x - fish.x, s.player.y - fish.y)
    if (
      kind === 'danger' &&
      s.invincible <= 0 &&
      s.dash <= 0 &&
      contact < (radius + s.player.radius) * 0.72
    ) {
      hurt(s, dx, dy, random)
      fish.x = clamp(
        fish.x - Math.sign(dx || 1) * 120,
        fish.radius,
        WORLD.width - fish.radius,
      )
      if (s.phase !== 'playing') break
    } else if (
      kind === 'prey' &&
      contact < s.player.radius + fish.radius * 0.35
    ) {
      removed.add(fish.id)
      consume(s, fish, random)
      if (s.phase !== 'playing') break
    }
  }
  s.fish = s.fish.filter((f) => !removed.has(f.id))
  for (const particle of s.particles) {
    particle.life -= dt
    particle.x += particle.vx * dt
    particle.y += particle.vy * dt
    particle.vx *= Math.exp(-2 * dt)
    particle.vy *= Math.exp(-2 * dt)
  }
  s.particles = s.particles.filter((p) => p.life > 0)
  for (const popup of s.popups) {
    popup.life -= dt
    popup.y -= dt * 40
  }
  s.popups = s.popups.filter((p) => p.life > 0)
}
export function tickOcean(
  s: OceanState,
  dt: number,
  input: OceanInput,
  random: () => number = Math.random,
) {
  if (s.phase !== 'playing' || !Number.isFinite(dt) || dt <= 0) return
  if (input.dash) dashOcean(s)
  let left = Math.min(0.2, dt)
  while (left > 0.00001) {
    const delta = Math.min(left, 1 / 90)
    step(s, delta, input, random)
    left -= delta
  }
}

export function nextRound(s: OceanState, seed = Date.now()): OceanState | null {
  if (s.phase !== 'cleared' || s.round >= ROUND_COUNT - 1) return null
  const next = createOcean(seed, s.round + 1, s.score, Math.min(3, s.lives + 1))
  next.time = s.time
  next.challenge!.huntAt += next.time
  next.challenge!.shoalAt += next.time
  next.voyage.weatherAt += next.time
  next.exploration.migrationAt += next.time
  return next
}

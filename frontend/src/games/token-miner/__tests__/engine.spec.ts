import { describe, expect, it } from 'vitest'
import {
  advance,
  createGame,
  discard,
  hookPosition,
  launch,
  LEVELS,
  nextShaft,
  WORLD,
  type GameState,
  type Ore,
} from '../engine'

function corridor(kind: Ore['kind'] = 'token'): GameState {
  const state = createGame(42)
  const original = state.ores.find((o) => o.kind === kind)!
  state.ores = [{ ...original, x: WORLD.originX, y: 300 }]
  state.angle = 0
  state.status = 'playing'
  return state
}

describe('Token Miner practice rules', () => {
  it('requires actual crystal retrieval even when the score quota has already been exceeded', () => {
    const s = corridor('token')
    s.level = 9
    s.score = LEVELS[9]!.target + 1000
    s.ores.push({ ...s.ores[0]!, id: 900, kind: 'gem', x: 700, y: 500 })
    launch(s)
    advance(s, 1.5)
    expect(s.gems).toBe(0)
    expect(s.status).toBe('playing')
    s.remaining = 0.1
    advance(s, 1)
    expect(s.status).toBe('lost')

    const finalCrystal = corridor('gem')
    finalCrystal.level = 9
    finalCrystal.gems = 2
    finalCrystal.score = LEVELS[9]!.target
    launch(finalCrystal)
    advance(finalCrystal, 1)
    expect(finalCrystal.gems).toBe(3)
    expect(finalCrystal.status).toBe('won')
  })
  it('offers varied but repeatable conditions and keeps all hundred shafts funded and reachable', () => {
    const conditions = new Set<string>()
    for (let level = 0; level < LEVELS.length; level++) {
      for (let seed = 0; seed < 10; seed++) {
        const s = createGame(seed, level)
        expect(s).toEqual(createGame(seed, level))
        conditions.add(s.condition)
        expect(s.ores.reduce((sum, o) => sum + o.value, 0)).toBeGreaterThan(LEVELS[level]!.target * 1.15)
        for (const o of s.ores) {
          expect(Math.abs(Math.atan2(o.x - WORLD.originX, o.y - WORLD.originY))).toBeLessThan(1.22)
          for (const other of s.ores)
            if (o.id !== other.id) expect(Math.hypot(o.x - other.x, o.y - other.y)).toBeGreaterThan(o.radius + other.radius)
        }
      }
    }
    expect(conditions.size).toBe(4)
  })

  it('earns exactly one capped upgrade per clear and carries progress to the next shaft', () => {
    const s = createGame(42)
    expect(nextShaft(s, 77, 'winch')).toBeNull()
    s.status = 'won'
    s.level = 9
    s.total = 900
    const next = nextShaft(s, 77, 'clock')!
    expect(next.level).toBe(10)
    expect(next.total).toBe(900)
    expect(next.remaining).toBe(LEVELS[10]!.seconds + 10)
    expect(next.equipment).toEqual({ winch: 0, clock: 1, assay: 0 })
    expect(s.equipment.clock).toBe(0)
    s.equipment.clock = 3
    expect(nextShaft(s, 77, 'clock')).toBeNull()
    s.status = 'complete'
    expect(nextShaft(s, 77, 'assay')).toBeNull()
  })

  it('applies equipment to real retrieval and crystal rewards', () => {
    const slow = corridor('rock'), fast = corridor('rock')
    fast.equipment.winch = 3
    launch(slow); launch(fast)
    advance(slow, 1.2); advance(fast, 1.2)
    expect(slow.grabbed).not.toBeNull()
    expect(fast.grabbed).toBeNull()
    const plain = createGame(7, 4)
    const assay = createGame(7, 4, 0, { winch: 0, clock: 0, assay: 3 })
    for (const gem of plain.ores.filter((o) => o.kind === 'gem'))
      expect(assay.ores[gem.id]!.value).toBeCloseTo(gem.value * 1.6, -1)
  })

  it('generates repeatable, separated and reachable fields with enough value for every target', () => {
    expect(createGame(42)).toEqual(createGame(42))
    expect(createGame(42).ores).not.toEqual(createGame(43).ores)
    for (let seed = 0; seed < 100; seed++) {
      const state = createGame(seed)
      expect(state.ores.reduce((n, o) => n + o.value, 0)).toBeGreaterThan(
        LEVELS[2].target,
      )
      for (const o of state.ores) {
        expect(
          Math.abs(Math.atan2(o.x - WORLD.originX, o.y - WORLD.originY)),
        ).toBeLessThan(1.22)
        expect(o.y + o.radius).toBeLessThan(WORLD.height - 24)
        for (const other of state.ores) {
          if (other.id !== o.id)
            expect(Math.hypot(other.x - o.x, other.y - o.y)).toBeGreaterThan(
              other.radius + o.radius,
            )
        }
      }
    }
  })

  it('ignores launch attempts outside a running game or during retrieval', () => {
    const state = createGame(42)
    expect(launch(state)).toBe(false)
    state.status = 'playing'
    expect(launch(state)).toBe(true)
    expect(launch(state)).toBe(false)
    expect(state.hook).toBe('extending')
  })

  it('catches ore without awarding points until it reaches the surface', () => {
    const state = corridor()
    launch(state)
    advance(state, 0.35)
    expect(state.grabbed?.kind).toBe('token')
    expect(state.score).toBe(0)
    expect(state.ores[0]!.collected).toBe(false)
    advance(state, 1)
    expect(state.score).toBe(100)
    expect(state.total).toBe(100)
    expect(state.ores[0]!.collected).toBe(true)
    expect(state.grabbed).toBeNull()
    advance(state, 10)
    expect(state.total).toBe(100)
  })

  it('retrieves heavy rocks more slowly than crystals', () => {
    const crystal = corridor('gem')
    const rock = corridor('rock')
    launch(crystal)
    launch(rock)
    advance(crystal, 1)
    advance(rock, 1)
    expect(crystal.score).toBe(400)
    expect(rock.score).toBe(0)
    expect(rock.grabbed?.kind).toBe('rock')
  })

  it('discards once, consumes a charge, and never awards the destroyed ore', () => {
    const state = corridor('rock')
    expect(discard(state)).toBe(false)
    launch(state)
    advance(state, 0.35)
    expect(discard(state)).toBe(true)
    expect(discard(state)).toBe(false)
    expect(state.bombs).toBe(1)
    advance(state, 2)
    expect(state.score).toBe(0)
    expect(state.grabbed).toBeNull()
    expect(state.ores[0]!.collected).toBe(true)
  })

  it('prevents use of dynamite when the inventory is empty', () => {
    const state = corridor('rock')
    launch(state)
    advance(state, 0.35)
    state.bombs = 0
    expect(discard(state)).toBe(false)
    expect(state.grabbed).not.toBeNull()
  })

  it('freezes physics and countdown when paused', () => {
    const state = corridor()
    launch(state)
    advance(state, 0.1)
    state.status = 'paused'
    const snapshot = structuredClone(state)
    advance(state, 90)
    expect(state).toEqual(snapshot)
    expect(launch(state)).toBe(false)
    expect(discard(state)).toBe(false)
  })

  it('expires without crediting a dangling ore, even after a long frame', () => {
    const state = corridor('rock')
    state.remaining = 0.5
    launch(state)
    advance(state, 120)
    expect(state.status).toBe('lost')
    expect(state.remaining).toBe(0)
    expect(state.score).toBe(0)
    expect(state.total).toBe(0)
  })

  it('advances only after a settled target and completes the final expedition', () => {
    for (const level of [0, 1, LEVELS.length - 1]) {
      const state = corridor('gem')
      state.level = level
      state.gems = LEVELS[level]!.gems
      state.score = LEVELS[level]!.target - 100
      state.total = state.score + 1000
      launch(state)
      advance(state, 1)
      expect(state.status).toBe(level === LEVELS.length - 1 ? 'complete' : 'won')
      expect(state.total).toBe(LEVELS[level]!.target + 1300)
    }
  })

  it('returns an empty hook from the edge and can launch again', () => {
    const state = corridor()
    state.ores[0]!.x = 100
    state.angle = 0.5
    launch(state)
    advance(state, 3)
    expect(state.hook).toBe('swinging')
    expect(state.length).toBe(WORLD.restLength)
    expect(state.score).toBe(0)
    expect(launch(state)).toBe(true)
  })

  it('keeps a 60 Hz and a long-frame simulation equivalent', () => {
    const a = corridor('rock')
    const b = corridor('rock')
    launch(a)
    launch(b)
    for (let i = 0; i < 120; i++) advance(a, 1 / 60)
    advance(b, 2)
    expect(a.score).toBe(b.score)
    expect(a.hook).toBe(b.hook)
    expect(hookPosition(a).y).toBeCloseTo(hookPosition(b).y, 4)
    expect(a.remaining).toBeCloseTo(b.remaining, 4)
  })

  it('ignores invalid frame deltas and clamps the requested level', () => {
    const state = corridor()
    const before = structuredClone(state)
    for (const dt of [NaN, Infinity, -1, 0]) advance(state, dt)
    expect(state).toEqual(before)
    expect(createGame(42, 99).level).toBe(LEVELS.length - 1)
    expect(createGame(42, -5).level).toBe(0)
  })
})

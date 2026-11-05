import { describe, expect, it } from 'vitest'
import {
  createOcean,
  startOcean,
  tickOcean,
  type OceanState,
  WORLD,
} from '../engine'
import {
  createExploration,
  discover,
  tickExploration,
  zoneAt,
  makeResident,
  isGentleGiant,
} from '../exploration'
import { swimmerRelation } from '../ecology'
import { beginFinale } from '../challenge'
import { fieldGuide, fishSprites } from '../species'

function playing() {
  const s = createOcean(42)
  startOcean(s, () => 0.5)
  s.fish = []
  s.spawnTimer = s.schoolTimer = 1000
  s.exploration.migrationAt = 1000
  if (s.challenge) s.challenge.huntAt = s.challenge.shoalAt = 1000
  return s
}
const step = (s: OceanState) => tickExploration(s, 0.02, () => 0.5)

describe('Deep ocean exploration', () => {
  it('provides three distinct layers and a complete 23-species field guide', () => {
    expect([zoneAt(420), zoneAt(700), zoneAt(1300)]).toEqual([
      'reef',
      'twilight',
      'abyss',
    ])
    expect(fieldGuide).toHaveLength(23)
    expect(new Set(fieldGuide).size).toBe(23)
    expect(fieldGuide.every((id) => fishSprites.includes(id))).toBe(true)
    const s = playing()
    s.player.y = 1250
    step(s)
    expect(s.exploration.zone).toBe('abyss')
    expect(s.exploration.message).toBe('zone')
  })
  it('discovers nearby species only once and freezes all exploration while not playing', () => {
    const s = playing()
    const f = makeResident(
      s.nextId++,
      'lanternfish',
      s.player.x,
      s.player.y,
      12,
      1,
    )
    s.fish.push(f)
    step(s)
    step(s)
    expect(s.exploration.discovered).toEqual(['lanternfish'])
    const event = s.event
    discover(s, 'lanternfish')
    expect(s.event).toBe(event)
    s.phase = 'over'
    const before = JSON.stringify(s.exploration)
    step(s)
    expect(JSON.stringify(s.exploration)).toBe(before)
  })
  it('makes huge residents harmless and inedible even during frenzy', () => {
    const s = playing()
    s.invincible = 0
    s.frenzy = 3
    for (const species of [
      'blue-whale',
      'whale-shark',
      'manta',
      'oarfish',
    ] as const) {
      const f = makeResident(
        s.nextId++,
        species,
        s.player.x,
        s.player.y,
        220,
        1,
      )
      s.fish.push(f)
      expect(isGentleGiant(species)).toBe(true)
      expect(swimmerRelation(s, f)).toBe('peer')
    }
    tickOcean(s, 0.1, { x: 0, y: 0 }, () => 0.5)
    expect(s.lives).toBe(3)
    expect(s.eaten).toBe(0)
    expect(s.fish).toHaveLength(4)
  })
  it('awards each relic once and the complete collection bonus once', () => {
    const s = playing()
    s.lives = 1
    for (const r of s.exploration.relics) {
      s.stamina = 1
      s.player.x = r.x
      s.player.y = r.y
      step(s)
      expect(s.stamina).toBe(100)
      const score = s.score
      step(s)
      expect(s.score).toBe(score)
    }
    expect(s.score).toBe(600 * 3 + 1800)
    expect(s.lives).toBe(2)
    expect(s.exploration.relicComplete).toBe(true)
    step(s)
    expect(s.lives).toBe(2)
  })
  it('selects the giant appropriate to each layer and starts an attainable six-crystal route', () => {
    for (const [y, species, size] of [
      [420, 'manta', 125],
      [800, 'whale-shark', 170],
      [1300, 'blue-whale', 220],
    ] as const) {
      const s = playing()
      s.player.y = y
      s.time = 12
      s.exploration.migrationAt = 12
      step(s)
      const m = s.exploration.migration!
      expect(m.species).toBe(species)
      expect(m.radius).toBe(size)
      expect(m.pearls).toHaveLength(6)
      expect(
        m.pearls.every(
          (p) => p.x >= 90 && p.x <= 2510 && p.y >= 130 && p.y <= 1470,
        ),
      ).toBe(true)
      expect(m.remaining).toBeGreaterThan(17)
    }
  })
  it('collects crystals once, restores stamina, and awards a complete migration only once', () => {
    const s = playing()
    s.time = 12
    s.lives = 1
    s.exploration.migrationAt = 12
    step(s)
    const m = s.exploration.migration!
    for (const pearl of m.pearls) {
      s.player.x = pearl.x
      s.player.y = pearl.y
      s.stamina = 10
      step(s)
      expect(s.stamina).toBe(22)
      const score = s.score
      step(s)
      expect(s.score).toBe(score)
    }
    expect(s.score).toBe(6 * 90 + 700)
    expect(s.lives).toBe(2)
    expect(s.exploration.migrations).toBe(1)
    expect(s.exploration.migration).toBeNull()
    expect(s.exploration.migrationAt).toBeGreaterThan(30)
  })
  it('expires an incomplete route without awarding completion or restarting immediately', () => {
    const s = playing()
    s.time = 12
    s.exploration.migrationAt = 12
    step(s)
    s.exploration.migration!.remaining = 0.01
    step(s)
    expect(s.exploration.migration).toBeNull()
    expect(s.exploration.migrations).toBe(0)
    expect(s.score).toBe(0)
    expect(s.exploration.migrationAt).toBeGreaterThan(s.time)
  })
  it('keeps exploration running beyond the last evolution without bypassing the hunting objective', () => {
    const s = playing()
    s.mass = 10000
    tickOcean(s, 0.02, { x: 0, y: 0 }, () => 0.5)
    expect(s.level).toBe(5)
    expect(s.phase).toBe('playing')
    expect(s.player.radius).toBe(94)
    const score = s.score
    tickOcean(s, 0.02, { x: 0, y: 0 }, () => 0.5)
    expect(s.score).toBe(score)
    const relaxed = playing()
    relaxed.mass = 10000
    tickOcean(relaxed, 0.02, { x: 0, y: 0 }, () => 0.5)
    expect(relaxed.phase).toBe('playing')
  })
  it('clears migration for the boss duel and keeps run resets independent', () => {
    const s = playing()
    s.time = 12
    s.exploration.migrationAt = 12
    step(s)
    expect(s.exploration.migration).not.toBeNull()
    beginFinale(s)
    step(s)
    expect(s.exploration.migration).toBeNull()
    const a = createExploration(),
      b = createExploration()
    a.relics[0]!.collected = true
    a.discovered.push('blue-whale')
    expect(b.relics[0]!.collected).toBe(false)
    expect(b.discovered).toEqual([])
  })
  it('keeps deep species in their habitat with bounded fish populations and finite positions', () => {
    const s = createOcean(42)
    let seed = 284
    const random = () =>
      (seed = (seed * 1664525 + 1013904223) >>> 0) / 4294967296
    startOcean(s, random)
    expect(s.fish.length).toBeGreaterThan(100)
    expect(s.fish.filter((f) => isGentleGiant(f.species)).length).toBe(4)
    expect(Math.max(...s.fish.map((f) => f.radius))).toBeGreaterThanOrEqual(220)
    s.invincible = 1000
    for (let i = 0; i < 60 * 12; i++)
      tickOcean(s, 1 / 60, { x: 0, y: -1 }, random)
    const deep = s.fish.filter((f) =>
      [
        'lanternfish',
        'hatchetfish',
        'viperfish',
        'gulper-eel',
        'giant-squid',
      ].includes(f.species),
    )
    expect(deep.length).toBeGreaterThan(0)
    expect(deep.every((f) => f.y >= 590)).toBe(true)
    expect(s.fish.length).toBeLessThanOrEqual(174)
    expect(
      s.fish.every(
        (f) =>
          Number.isFinite(f.x + f.y) &&
          f.x >= 0 &&
          f.x <= WORLD.width &&
          f.y >= 0 &&
          f.y <= WORLD.height,
      ),
    ).toBe(true)
  })
})

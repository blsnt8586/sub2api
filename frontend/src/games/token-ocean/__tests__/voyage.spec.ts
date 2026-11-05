import { describe, expect, it } from 'vitest'
import { createOcean, startOcean, tickOcean, nextRound, type OceanState } from '../engine'
import { createVoyage, tickVoyage } from '../voyage'
import { tailPosition, tickChallenge } from '../challenge'
import { ROUNDS, ROUND_COUNT, roundFor } from '../campaign'
function playing(round = 0, seed = 42) {
  const s = createOcean(seed, round)
  startOcean(s, () => 0.5)
  s.fish = []
  s.spawnTimer = s.schoolTimer = s.voyage.weatherAt = 10000
  s.challenge!.huntAt = s.challenge!.shoalAt = 10000
  return s
}
function eat(s: OceanState, dash = false, golden = false) {
  s.dash = dash ? 0.3 : 0
  s.fish.push({ id: s.nextId++, x: s.player.x, y: s.player.y, radius: 10, species: 'minnow', golden, vx: 0, vy: 0, turn: 0 })
  tickOcean(s, 0.02, { x: 0, y: 0 }, () => 0.5)
}
function counter(s: OceanState) {
  const h = s.challenge!.hunter!
  h.phase = 'recover'; h.remaining = 2; h.bitten = false
  Object.assign(s.player, tailPosition(h)); s.dash = 0.3
  tickChallenge(s, 0.01, () => 0.5)
}
describe('Sixty-stage ocean campaign', () => {
  it('defines ten complete chapters, varied goals and exactly ten boss stages', () => {
    expect(ROUNDS).toHaveLength(60)
    expect(ROUNDS.filter((r) => r.boss)).toHaveLength(10)
    expect(new Set(ROUNDS.map((r) => r.goal)).size).toBe(3)
    for (let i = 0; i < ROUND_COUNT; i++) {
      expect(ROUNDS[i]!.chapter).toBe(Math.floor(i / 6))
      const v = createVoyage(42, i)
      expect(v).toEqual(createVoyage(42, i))
      expect(v.legs[0]!.target).toBeGreaterThanOrEqual(ROUNDS[i]!.target)
    }
    expect(ROUNDS[54]!.seconds).toBeLessThan(ROUNDS[0]!.seconds)
  })
  it('counts only catches in the correct region, respecting dash and golden objectives', () => {
    const s = playing(1), leg = s.voyage.legs[0]!
    expect(leg.kind).toBe('dash')
    s.player.y = 830; eat(s, true)
    expect(leg.caught).toBe(0)
    s.player.y = 300; eat(s)
    expect(leg.caught).toBe(0)
    eat(s, true)
    expect(leg.caught).toBe(1)
    const gold = playing(3)
    eat(gold, true)
    expect(gold.voyage.legs[0]!.caught).toBe(0)
    eat(gold, false, true)
    expect(gold.voyage.legs[0]!.caught).toBe(1)
  })
  it('advances all sixty stages without treating an intermediate clear or boss as campaign victory', () => {
    let carried = 0
    for (let round = 0; round < ROUND_COUNT; round++) {
      const s = playing(round)
      s.score = carried
      const leg = s.voyage.legs[0]!
      for (let i = 0; i < leg.target; i++) eat(s, true, true)
      expect(leg.caught).toBe(leg.target)
      if (roundFor(s).boss) {
        expect(s.phase).toBe('playing')
        s.mass = Math.max(s.mass, 850)
        tickOcean(s, 0.02, { x: 0, y: 0 }, () => 0.5)
        expect(s.challenge!.finale).toBe(true)
        for (let hit = 0; hit < 3; hit++) {
          counter(s)
          expect(s.challenge!.bossHealth).toBe(2 - hit)
        }
      }
      expect(s.phase).toBe(round === 59 ? 'won' : 'cleared')
      carried = s.score
      const frozen = s.score
      eat(s, true, true)
      expect(s.score).toBe(frozen)
      const next = nextRound(s, 99)
      if (round === 59) expect(next).toBeNull()
      else {
        expect(next!.round).toBe(round + 1)
        expect(next!.score).toBe(s.score)
        expect(next!.roundTime).toBe(0)
        expect(next!.voyage.legs[0]!.caught).toBe(0)
      }
    }
  })
  it('requires stage objectives even at max growth, and enforces each stage timer', () => {
    const s = playing(5)
    s.mass = 850
    tickOcean(s, 0.02, { x: 0, y: 0 }, () => 0.5)
    expect(s.challenge!.finale).toBe(false)
    expect(nextRound(s)).toBeNull()
    s.roundTime = roundFor(s).seconds - 0.01
    tickOcean(s, 0.02, { x: 0, y: 0 }, () => 0.5)
    expect(s.phase).toBe('over')
    expect(s.round).toBe(5)
  })
  it('spawns safe event schools, rotates tides, and provides a golden tide for golden objectives', () => {
    const s = playing(), seen = new Set<string>()
    for (const roll of [0, 0, 0.99]) {
      s.time = s.voyage.weatherAt; tickVoyage(s, 0.02, () => roll)
      seen.add(s.voyage.weather!)
      for (const f of s.fish) {
        expect(f.radius * 1.15).toBeLessThan(s.player.radius)
        expect(Math.hypot(f.x - s.player.x, f.y - s.player.y)).toBeGreaterThan(s.player.radius + f.radius + 110)
      }
      s.time += s.voyage.weatherRemaining; tickVoyage(s, 25, () => roll)
      expect(s.voyage.weather).toBeNull()
    }
    expect(seen.size).toBe(3)
    const gold = playing(3)
    gold.time = gold.voyage.weatherAt; tickVoyage(gold, 0.01, () => 0)
    expect(gold.voyage.weather).toBe('golden')
    expect(gold.fish.some((f) => f.golden)).toBe(true)
    gold.challenge!.finale = true; tickVoyage(gold, 0.1, () => 0.5)
    expect(gold.voyage.weather).toBeNull()
  })
  it('changes real movement and stamina in currents and freezes completed states', () => {
    const normal = playing(), current = playing()
    current.voyage.weather = 'current'; current.voyage.weatherRemaining = 10
    for (const s of [normal, current]) s.stamina = 0
    for (let i = 0; i < 60; i++) {
      tickOcean(normal, 1 / 60, { x: 1, y: 0 }, () => 0.5)
      tickOcean(current, 1 / 60, { x: 1, y: 0 }, () => 0.5)
    }
    expect(current.player.x).toBeGreaterThan(normal.player.x + 20)
    expect(current.stamina).toBeGreaterThan(normal.stamina + 10)
    current.phase = 'cleared'
    const snapshot = structuredClone(current)
    tickVoyage(current, 10, () => 0.5); tickOcean(current, 1, { x: 1, y: 0 }, () => 0.5)
    expect(current).toEqual(snapshot)
  })
})

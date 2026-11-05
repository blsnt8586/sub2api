import { describe, expect, it } from 'vitest'
import {
  createOcean,
  startOcean,
  tickOcean,
  dashOcean,
  type OceanState,
  type Swimmer,
} from '../engine'
import {
  pufferPhase,
  reefPressure,
  swimmerRelation,
  swimmerRadius,
  tickAmbush,
} from '../ecology'
import { beginFinale, tickChallenge } from '../challenge'

function playing() {
  const s = createOcean(42)
  startOcean(s, () => 0.5)
  s.fish = []
  s.spawnTimer = s.schoolTimer = 1000
  s.challenge!.huntAt = s.challenge!.shoalAt = 1000
  s.invincible = 0
  return s
}
function fish(
  s: OceanState,
  species: Swimmer['species'],
  extra: Partial<Swimmer> = {},
) {
  const f: Swimmer = {
    id: s.nextId++,
    species,
    x: s.player.x,
    y: s.player.y,
    vx: 0,
    vy: 0,
    radius: 10,
    golden: false,
    turn: 0,
    ...extra,
  }
  s.fish.push(f)
  return f
}
const tick = (s: OceanState, dt = 1 / 60) =>
  tickOcean(s, dt, { x: 0, y: 0 }, () => 0.5)

describe('Reef species and escalating pressure', () => {
  it('warns before puffer inflation and keeps spikes dangerous even during frenzy', () => {
    const s = playing(),
      f = fish(s, 'pufferfish')
    s.time = 4.2
    expect(pufferPhase(s, f)).toBe('warning')
    expect(swimmerRelation(s, f)).toBe('prey')
    s.time = 5
    s.frenzy = 3
    expect(pufferPhase(s, f)).toBe('inflated')
    expect(swimmerRadius(s, f)).toBe(14.5)
    expect(swimmerRelation(s, f)).toBe('danger')
    tick(s)
    expect(s.eaten).toBe(0)
    expect(s.lives).toBe(2)
  })
  it('lets dash pass through spikes without swallowing them, then allows a deflated catch for double score', () => {
    const s = playing(),
      f = fish(s, 'pufferfish')
    s.time = 5
    dashOcean(s)
    tick(s)
    expect(s.lives).toBe(3)
    expect(s.eaten).toBe(0)
    s.time = 8
    s.dash = 0
    f.x = s.player.x
    f.y = s.player.y
    tick(s)
    expect(s.eaten).toBe(1)
    expect(s.score).toBe(48)
  })
  it('treats small lionfish as hazards and resolves them before a same-frame evolution', () => {
    const s = playing()
    s.mass = 34
    fish(s, 'minnow', { radius: 13 })
    const venom = fish(s, 'lionfish', { radius: 9 })
    expect(swimmerRelation(s, venom)).toBe('danger')
    tick(s)
    expect(s.lives).toBe(2)
    expect(s.level).toBe(1)
    expect(s.fish.some((f) => f.species === 'lionfish')).toBe(true)
    s.frenzy = 3
    venom.x = s.player.x
    venom.y = s.player.y
    tick(s)
    expect(s.eaten).toBe(2)
  })
  it('restores stamina for catching fast sardines without exceeding the cap', () => {
    const s = playing()
    s.stamina = 50
    fish(s, 'sardine')
    tick(s)
    expect(s.stamina).toBeGreaterThanOrEqual(66)
    s.stamina = 99
    fish(s, 'sardine')
    tick(s)
    expect(s.stamina).toBe(100)
  })
  it('locks barracuda aim before a straight sprint and exposes a recovery interval', () => {
    const s = playing(),
      f = fish(s, 'barracuda', { x: s.player.x - 300, radius: 30 })
    f.ambush = { phase: 'cruise', remaining: 0, x: 0, y: 0 }
    expect(tickAmbush(s, f, 0.01, true)).toBe(true)
    expect(f.ambush.phase).toBe('aim')
    tickAmbush(s, f, 0.5, true)
    const y = f.ambush.y
    s.player.y += 150
    tickAmbush(s, f, 0.1, true)
    expect(f.ambush.y).toBe(y)
    tickAmbush(s, f, 0.31, true)
    expect(f.ambush.phase).toBe('charge')
    const vx = f.vx,
      vy = f.vy
    s.player.y -= 250
    tickAmbush(s, f, 0.3, true)
    expect([f.vx, f.vy]).toEqual([vx, vy])
    tickAmbush(s, f, 0.41, true)
    expect(f.ambush.phase).toBe('recover')
    expect(Math.hypot(f.vx, f.vy)).toBeLessThan(60)
  })
  it('raises pressure by growth or time, caps it, and preserves half a second of hunter lock', () => {
    const s = playing()
    expect(reefPressure(s)).toBe(0)
    s.level = 2
    expect(reefPressure(s)).toBe(1)
    s.level = 0
    s.roundTime = 70
    expect(reefPressure(s)).toBe(2)
    tickChallenge(s, 0.01, () => 0.5)
    expect(s.challenge!.banner).toBe('pressure')
    beginFinale(s)
    s.challenge!.finale = false
    const h = s.challenge!.hunter!
    h.phase = 'stalk'
    h.remaining = 0
    tickChallenge(s, 0.01, () => 0.5)
    expect(h.phase).toBe('aim')
    expect(h.remaining).toBeGreaterThanOrEqual(1.1)
    h.remaining = 0.49
    const aimY = h.aimY
    s.player.y += 100
    tickChallenge(s, 0.1, () => 0.5)
    expect(h.aimY).toBe(aimY)
    expect(reefPressure(createOcean())).toBe(0)
  })
  it('leaves a real vulnerable interval between challenge frenzies and limits evolution protection', () => {
    const s = playing()
    for (let i = 0; i < 8; i++) fish(s, 'clownfish')
    tick(s)
    expect(s.frenzy).toBeGreaterThan(4)
    expect(s.frenzyCooldown).toBeGreaterThan(10)
    expect(s.invincible).toBeLessThanOrEqual(0.65)
    s.frenzy = 0
    s.mass = 40
    for (let i = 0; i < 9; i++) fish(s, 'clownfish')
    tick(s)
    expect(s.frenzyCharge).toBe(0)
    expect(s.frenzy).toBe(0)
    s.frenzyCooldown = 0
    for (let i = 0; i < 8; i++) fish(s, 'clownfish')
    tick(s)
    expect(s.frenzy).toBeGreaterThan(4)
  })
  it('removes spiny and venomous hazards for the final duel, even when frenzy had made them edible', () => {
    const s = playing()
    s.frenzy = 3
    fish(s, 'lionfish')
    fish(s, 'pufferfish')
    fish(s, 'clownfish')
    fish(s, 'barracuda', { radius: 28 })
    beginFinale(s)
    expect(s.fish.map((f) => f.species)).toEqual(['clownfish'])
    expect(reefPressure(s)).toBe(0)
    expect(s.challenge!.bossHealth).toBe(3)
  })
  it('populates a more varied reef without spawning dangerous fish on the player', () => {
    let seed = 14
    const random = () =>
      (seed = (seed * 1664525 + 1013904223) >>> 0) / 4294967296
    const s = createOcean(42)
    startOcean(s, random)
    expect(s.fish.length).toBeGreaterThanOrEqual(75)
    expect(new Set(s.fish.map((f) => f.species)).size).toBeGreaterThanOrEqual(6)
    expect(
      s.fish.every(
        (f) =>
          Math.hypot(f.x - s.player.x, f.y - s.player.y) >
          f.radius + s.player.radius + 100,
      ),
    ).toBe(true)
    s.invincible = 1000
    s.time = 75
    s.challenge!.huntAt = s.challenge!.shoalAt = 1000
    for (let i = 0; i < 60 * 50; i++)
      tickOcean(s, 1 / 60, { x: 0, y: 0 }, random)
    expect(s.fish.length).toBeLessThanOrEqual(168)
    expect(
      s.fish.some(
        (f) => f.species === 'pufferfish' || f.species === 'lionfish',
      ),
    ).toBe(true)
    expect(
      s.fish.some(
        (f) => f.species === 'barracuda' || f.species === 'anglerfish',
      ),
    ).toBe(true)
  })
})

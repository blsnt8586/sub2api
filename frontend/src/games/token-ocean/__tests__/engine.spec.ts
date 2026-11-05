import { describe, expect, it } from 'vitest'
import {
  createOcean,
  startOcean,
  tickOcean,
  dashOcean,
  relation,
  fishRelation,
  growthProgress,
  WORLD,
  type OceanState,
  type Swimmer,
} from '../engine'
import { cameraFor, screenToWorld } from '../renderer'
function playing() {
  const s = createOcean()
  startOcean(s, () => 0.5)
  s.fish = []
  s.spawnTimer = 100
  return s
}
function fish(s: OceanState, radius = 10, extra: Partial<Swimmer> = {}) {
  const f: Swimmer = {
    id: s.nextId++,
    x: s.player.x,
    y: s.player.y,
    vx: 0,
    vy: 0,
    radius,
    species: 'minnow',
    golden: false,
    turn: 0,
    ...extra,
  }
  s.fish.push(f)
  return f
}
function advance(s: OceanState, seconds: number, x = 0, y = 0) {
  for (let i = 0; i < seconds * 60; i++)
    tickOcean(s, 1 / 60, { x, y }, () => 0.5)
}
describe('Feed and grow simulation', () => {
  it('starts once with safe spawns and five nearby edible targets', () => {
    const s = createOcean()
    expect(startOcean(s)).toBe(true)
    expect(startOcean(s)).toBe(false)
    expect(s.fish.length).toBeGreaterThan(30)
    expect(
      s.fish.filter(
        (f) =>
          relation(s.player.radius, f.radius) === 'prey' &&
          Math.hypot(f.x - s.player.x, f.y - s.player.y) < 450,
      ).length,
    ).toBeGreaterThanOrEqual(5)
    expect(
      s.fish.every(
        (f) =>
          Math.hypot(f.x - s.player.x, f.y - s.player.y) >
          s.player.radius + f.radius + 100,
      ),
    ).toBe(true)
  })
  it('allows the opening school to be caught with normal swimming', () => {
    const s = createOcean()
    startOcean(s, () => 0.5)
    advance(s, 2.4, 1, 0)
    expect(s.eaten).toBeGreaterThan(0)
    expect(s.score).toBeGreaterThan(0)
  })
  it('normalizes diagonals and slows smoothly on releasing movement', () => {
    const a = playing(),
      b = playing(),
      x = a.player.x,
      y = a.player.y
    advance(a, 1, 1, 0)
    advance(b, 1, 1, 1)
    expect(Math.hypot(b.player.x - x, b.player.y - y)).toBeCloseTo(
      a.player.x - x,
      3,
    )
    advance(a, 0.5)
    expect(Math.abs(a.player.vx)).toBeLessThan(1)
  })
  it('moves toward pointer targets and clamps to the reef', () => {
    const s = playing()
    tickOcean(s, 0.2, {
      x: 0,
      y: 0,
      target: { x: s.player.x + 100, y: s.player.y },
    })
    expect(s.player.vx).toBeGreaterThan(0)
    s.player.x = WORLD.width - 1
    advance(s, 1, 1)
    expect(s.player.x).toBe(WORLD.width - s.player.radius)
  })
  it('requires a meaningful size difference; peers cannot hurt or feed', () => {
    expect(relation(19, 10)).toBe('prey')
    expect(relation(19, 19)).toBe('peer')
    expect(relation(19, 30)).toBe('danger')
    const s = playing()
    s.invincible = 0
    fish(s, 19)
    advance(s, 0.1)
    expect(s.eaten).toBe(0)
    expect(s.lives).toBe(3)
  })
  it('eats each prey once and golden prey grants extra score and growth', () => {
    const a = playing(),
      b = playing()
    fish(a)
    fish(b, 10, { golden: true })
    advance(a, 0.1)
    advance(b, 0.1)
    expect(a.eaten).toBe(1)
    expect(a.fish).toHaveLength(0)
    expect(b.score).toBe(a.score * 3)
    expect(b.mass).toBe(a.mass * 2)
    advance(a, 0.2)
    expect(a.eaten).toBe(1)
  })
  it('grows into its former predator and can then eat it', () => {
    const s = playing()
    s.mass = 34
    const enemy = fish(s, 22, { x: s.player.x + 120 })
    expect(relation(s.player.radius, enemy.radius)).toBe('danger')
    fish(s)
    advance(s, 0.02)
    expect(s.level).toBe(1)
    expect(s.player.radius).toBe(31)
    enemy.x = s.player.x
    enemy.y = s.player.y
    advance(s, 0.02)
    expect(s.eaten).toBe(2)
    expect(growthProgress(s)).toBeGreaterThan(0)
    expect(growthProgress(s)).toBeLessThan(1)
  })
  it('protects against repeated contact and ends after three distinct hits', () => {
    const s = playing()
    for (let i = 0; i < 3; i++) {
      s.invincible = 0
      fish(s, 40)
      advance(s, 0.02)
      expect(s.lives).toBe(2 - i)
      const hp = s.lives
      advance(s, 0.1)
      expect(s.lives).toBe(hp)
      s.fish = []
    }
    expect(s.phase).toBe('over')
    const time = s.time
    advance(s, 1)
    expect(s.time).toBe(time)
  })
  it('dash costs stamina, prevents contact damage and cannot be spammed', () => {
    const s = playing()
    s.invincible = 0
    fish(s, 40)
    expect(dashOcean(s)).toBe(true)
    expect(s.stamina).toBe(62)
    expect(dashOcean(s)).toBe(false)
    advance(s, 0.2, 1)
    expect(s.lives).toBe(3)
    s.stamina = 37
    s.dash = 0
    expect(dashOcean(s)).toBe(false)
    advance(s, 2)
    expect(s.stamina).toBeCloseTo(75, 5)
  })
  it('builds multipliers and frenzy, then expires broken chains', () => {
    const s = playing()
    for (let i = 0; i < 8; i++) {
      fish(s)
      advance(s, 0.02)
    }
    expect(s.bestCombo).toBe(8)
    expect(s.frenzy).toBeGreaterThan(4)
    expect(s.score).toBeGreaterThan(8 * 24)
    advance(s, 4)
    expect(s.combo).toBe(0)
    expect(s.frenzyCharge).toBe(0)
  })
  it('pulls nearby prey into a forward dash, but does not vacuum fish from behind', () => {
    const s = playing()
    fish(s, 10, { x: s.player.x + 35, y: s.player.y + 10 })
    fish(s, 10, { x: s.player.x - 38, y: s.player.y })
    expect(dashOcean(s)).toBe(true)
    advance(s, 0.2)
    expect(s.eaten).toBe(1)
    expect(s.fish).toHaveLength(1)
  })
  it('tracks a short multi-gulp window, merges nearby score text, and expires visual feedback', () => {
    const s = playing()
    for (let i = 0; i < 4; i++)
      fish(s, 10, { x: s.player.x + i, y: s.player.y })
    advance(s, 0.02)
    expect(s.feastCount).toBe(4)
    expect(s.feast).toBeGreaterThan(0)
    expect(s.gulps).toHaveLength(4)
    expect(s.popups).toHaveLength(1)
    advance(s, 0.7)
    fish(s)
    advance(s, 0.02)
    expect(s.gulpChain).toBe(1)
    advance(s, 1.2)
    expect(s.gulps).toEqual([])
    expect(s.bite).toBe(0)
    expect(s.feast).toBe(0)
  })
  it('reverses ordinary predator relations during frenzy, with a size limit and an expiry', () => {
    const s = playing()
    s.invincible = 0
    s.frenzy = 1
    expect(fishRelation(s, 28)).toBe('prey')
    expect(fishRelation(s, 40)).toBe('danger')
    fish(s, 28)
    advance(s, 0.02)
    expect(s.eaten).toBe(1)
    expect(s.lives).toBe(3)
    expect(s.score).toBeGreaterThan(100)
    advance(s, 1.1)
    expect(fishRelation(s, 28)).toBe('danger')
  })
  it('requires the stage objective even at the final form, then freezes scoring after a clear', () => {
    const s = playing()
    s.mass = 849
    fish(s)
    advance(s, 0.02)
    expect(s.phase).toBe('playing')
    expect(s.level).toBe(5)
    s.phase = 'cleared'
    const score = s.score
    fish(s)
    advance(s, 1)
    expect(s.score).toBe(score)
  })
  it('ignores invalid time and caps suspended-tab catch-up', () => {
    const s = playing()
    for (const dt of [NaN, Infinity, -1, 0]) tickOcean(s, dt, { x: 1, y: 0 })
    expect(s.time).toBe(0)
    tickOcean(s, 60, { x: 1, y: 0 })
    expect(s.time).toBeCloseTo(0.2)
  })
  it('maps portrait/landscape pointers to the same player camera center', () => {
    const s = playing()
    for (const [w, h] of [
      [1200, 750],
      [360, 450],
      [844, 326],
    ]) {
      const p = screenToWorld(s, w! / 2, h! / 2, w!, h!)
      expect(p.x).toBe(s.player.x)
      expect(p.y).toBe(s.player.y)
      s.player.x = 20
      const c = cameraFor(s, w! / h!)
      expect(c.x).toBe(0)
      expect(c.width / c.height).toBeCloseTo(w! / h!, 8)
      s.player.x = WORLD.width / 2
    }
  })
})

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
  beginFinale,
  tickChallenge,
  tailPosition,
  challengeStars,
} from '../challenge'
function playing() {
  const s = createOcean(42)
  startOcean(s, () => 0.5)
  s.fish = []
  s.schoolTimer = 1000
  s.spawnTimer = 1000
  s.challenge!.shoalAt = 1000
  s.challenge!.huntAt = 1000
  return s
}
function advance(s: OceanState, seconds: number, x = 0, y = 0) {
  for (let i = 0; i < Math.ceil(seconds * 90); i++)
    tickOcean(s, 1 / 90, { x, y }, () => 0.5)
}
function hunter(s: OceanState) {
  beginFinale(s)
  s.challenge!.finale = false
  return s.challenge!.hunter!
}
function prey(s: OceanState, extra: Partial<Swimmer> = {}) {
  const f: Swimmer = {
    id: s.nextId++,
    x: s.player.x,
    y: s.player.y,
    vx: 0,
    vy: 0,
    radius: 10,
    species: 'minnow',
    golden: false,
    turn: 0,
    ...extra,
  }
  s.fish.push(f)
  return f
}
describe('Hunter challenge', () => {
  it('always enables the hunter challenge and gives the first six seconds a safe introduction', () => {
    expect(createOcean().challenge).not.toBeNull()
    const s = playing()
    s.challenge!.huntAt = 10
    s.challenge!.shoalAt = 6
    advance(s, 5.9)
    expect(s.challenge!.shoal).toBeNull()
    expect(s.challenge!.hunter).toBeNull()
    advance(s, 0.2)
    expect(s.challenge!.shoal).not.toBeNull()
    expect(s.challenge!.hunter).toBeNull()
  })
  it('locks its target before charging and never steers a charge toward the player', () => {
    const s = playing(),
      h = hunter(s)
    h.remaining = 0.6
    h.x = s.player.x - 350
    h.y = s.player.y
    tickChallenge(s, 0.05, () => 0.5)
    const locked = h.aimY
    s.player.y += 150
    tickChallenge(s, 0.1, () => 0.5)
    expect(h.aimY).toBe(locked)
    tickChallenge(s, 0.5, () => 0.5)
    expect(h.phase).toBe('charge')
    const vx = h.vx,
      vy = h.vy
    s.player.y += 100
    tickChallenge(s, 0.1, () => 0.5)
    expect(h.vx).toBe(vx)
    expect(h.vy).toBe(vy)
  })
  it('exposes a real counterattack window after a charge and awards close dodges only once', () => {
    const s = playing(),
      h = hunter(s)
    h.phase = 'charge'
    h.remaining = 0.01
    h.x = s.player.x - 60
    h.y = s.player.y - 90
    h.vx = 510
    h.vy = 0
    tickChallenge(s, 0.02, () => 0.5)
    expect(h.phase).toBe('recover')
    expect(h.remaining).toBeGreaterThan(2)
    expect(s.challenge!.dodges).toBe(1)
    expect(s.score).toBe(120)
    tickChallenge(s, 0.1, () => 0.5)
    expect(s.score).toBe(120)
  })
  it('damages on hunter contact, honors dash and injury protection, and is safe during exhaustion', () => {
    const s = playing(),
      h = hunter(s)
    h.phase = 'charge'
    h.remaining = 0.5
    h.x = s.player.x
    h.y = s.player.y
    h.vx = 0
    h.vy = 0
    s.invincible = 0
    advance(s, 0.02)
    expect(s.lives).toBe(2)
    advance(s, 0.1)
    expect(s.lives).toBe(2)
    s.invincible = 0
    dashOcean(s)
    advance(s, 0.05)
    expect(s.lives).toBe(2)
    s.dash = 0
    h.phase = 'recover'
    h.remaining = 2
    advance(s, 0.1)
    expect(s.lives).toBe(2)
  })
  it('requires an active dash at the tail, not the head, and scores once per recovery', () => {
    const s = playing(),
      h = hunter(s)
    h.phase = 'recover'
    h.remaining = 2
    h.facing = 1
    const tail = tailPosition(h)
    s.player.x = tail.x
    s.player.y = tail.y
    tickChallenge(s, 0.01, () => 0.5)
    expect(s.challenge!.tailBites).toBe(0)
    s.player.x = h.x + h.radius
    s.dash = 0.3
    tickChallenge(s, 0.01, () => 0.5)
    expect(s.challenge!.tailBites).toBe(0)
    s.player.x = tail.x
    tickChallenge(s, 0.01, () => 0.5)
    expect(s.challenge!.tailBites).toBe(1)
    expect(s.score).toBe(300)
    tickChallenge(s, 0.1, () => 0.5)
    expect(s.score).toBe(300)
  })
  it('starts the final battle at full growth, then needs three distinct counterattacks to win', () => {
    const s = playing()
    s.round = 59
    s.voyage.stage = 1
    s.mass = 849
    prey(s)
    advance(s, 0.02)
    expect(s.phase).toBe('playing')
    expect(s.challenge!.finale).toBe(true)
    expect(s.level).toBe(5)
    for (let i = 0; i < 3; i++) {
      const h = s.challenge!.hunter!
      h.phase = 'recover'
      h.remaining = 2
      h.bitten = false
      const tail = tailPosition(h)
      s.player.x = tail.x
      s.player.y = tail.y
      s.dash = 0.3
      tickChallenge(s, 0.01, () => 0.5)
      expect(s.challenge!.bossHealth).toBe(2 - i)
      if (i < 2) {
        beginFinale(s)
        expect(s.challenge!.bossHealth).toBe(2 - i)
      }
    }
    expect(s.phase).toBe('won')
    expect(s.score).toBeGreaterThanOrEqual(4250)
    const score = s.score
    tickChallenge(s, 1, () => 0.5)
    advance(s, 1)
    expect(s.score).toBe(score)
  })
  it('creates six edible golden targets and heals only after clearing the whole timed school', () => {
    const s = playing()
    s.challenge!.shoalAt = 0
    s.lives = 1
    advance(s, 0.02)
    const id = s.challenge!.shoal!.id,
      school = s.fish.filter((f) => f.shoal === id)
    expect(school).toHaveLength(6)
    expect(
      school.every((f) => f.golden && f.radius * 1.15 < s.player.radius),
    ).toBe(true)
    for (const f of school.slice(0, 5)) {
      f.x = s.player.x
      f.y = s.player.y
      advance(s, 0.02)
    }
    expect(s.lives).toBe(1)
    expect(s.challenge!.shoal?.caught).toBe(5)
    school[5]!.x = s.player.x
    school[5]!.y = s.player.y
    advance(s, 0.02)
    expect(s.lives).toBe(2)
    expect(s.challenge!.shoals).toBe(1)
    expect(s.challenge!.shoal).toBeNull()
    const score = s.score
    advance(s, 0.1)
    expect(s.score).toBe(score)
  })
  it('regroups escaped golden fish around the event marker', () => {
    const s = playing()
    s.challenge!.shoalAt = 0
    advance(s, 0.02)
    const shoal = s.challenge!.shoal!,
      f = s.fish[0]!
    f.x = shoal.x + 250
    f.vx = 100
    s.player.x = 2200
    s.player.y = 1300
    advance(s, 3)
    expect(Math.hypot(f.x - shoal.x, f.y - shoal.y)).toBeLessThan(150)
  })
  it('lets an early tail counterattack contribute to immediate evolution', () => {
    const s = playing(),
      h = hunter(s)
    s.mass = 34
    h.phase = 'recover'
    h.remaining = 2
    const tail = tailPosition(h)
    s.player.x = tail.x
    s.player.y = tail.y
    s.dash = 0.3
    advance(s, 0.02)
    expect(s.challenge!.tailBites).toBe(1)
    expect(s.mass).toBe(52)
    expect(s.level).toBe(1)
  })
  it('expires only event fish, gives no partial clear reward, and cannot trigger a school after the finale', () => {
    const s = playing()
    s.challenge!.shoalAt = 0
    advance(s, 0.02)
    const normal = prey(s, { x: 100, y: 100 }),
      c = s.challenge!
    c.shoal!.remaining = 0.01
    tickChallenge(s, 0.02, () => 0.5)
    expect(c.shoal).toBeNull()
    expect(s.fish).toEqual([normal])
    expect(c.shoals).toBe(0)
    expect(s.score).toBe(0)
    beginFinale(s)
    c.shoalAt = 0
    advance(s, 0.02)
    expect(c.shoal).toBeNull()
  })
  it('makes the finale a duel and spawns only edible background fish', () => {
    const s = playing()
    s.level = 5
    s.mass = 850
    s.player.radius = 79
    const small = prey(s, { x: 100, y: 100 }),
      large = prey(s, { radius: 140, x: 200, y: 200 })
    beginFinale(s)
    expect(s.fish).toContain(small)
    expect(s.fish).not.toContain(large)
    s.spawnTimer = 0
    tickOcean(s, 0.1, { x: 0, y: 0 }, () => 0.99)
    expect(s.fish.length).toBeGreaterThan(1)
    expect(s.fish.every((f) => f.radius * 1.15 <= s.player.radius)).toBe(true)
  })
  it('freezes event timers when paused or after a run ends', () => {
    const s = playing(),
      h = hunter(s)
    s.phase = 'over'
    const time = h.remaining
    tickOcean(s, 0.2, { x: 0, y: 0 })
    tickChallenge(s, 0.2, () => 0.5)
    expect(h.remaining).toBe(time)
  })
  it('awards stars for completion, speed and skilled survival', () => {
    const s = playing()
    expect(challengeStars(s)).toBe(0)
    s.phase = 'won'
    s.time = 7300
    s.lives = 1
    expect(challengeStars(s)).toBe(1)
    s.time = 7100
    expect(challengeStars(s)).toBe(2)
    s.lives = 2
    s.challenge!.tailBites = 3
    s.challenge!.dodges = 2
    expect(challengeStars(s)).toBe(3)
  })
})

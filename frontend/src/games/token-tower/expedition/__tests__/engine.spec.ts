import { describe, expect, it } from 'vitest'
import * as engine from '../engine'
import { runExpedition } from './campaign'
import * as classic from '../../classic/engine'
import { combatFrames } from '../../classic/combat'
import type { BattleStrike } from '../../campaign-engine'

describe('100-floor expedition', () => {
  it.each([['knight', 1], ['scout', 1], ['knight', 7], ['scout', 7], ['knight', 99], ['scout', 99]] as const)('completes all hundred floors by legal moves and purchases as %s seed %i', (role, seed) => {
    const { state, actions, checkpoints } = runExpedition(role, seed)
    console.log('Expedition legal route', JSON.stringify({ role, actions: actions.length, checkpoints: checkpoints.filter(c => c.floor % 10 === 0), hp: state.stats.hp, damage: state.damage, battles: engine.battleCount(state) }))
    expect(state.phase).toBe('won')
    expect(state.discovered).toHaveLength(100)
    expect(engine.bossesDefeated(state)).toBe(true)
  }, 30000)

  it('contains hundred different connected maps and ten compulsory realm lords', () => {
    expect(engine.FLOORS).toHaveLength(100)
    expect(new Set(engine.FLOORS.map(map => JSON.stringify(map.map(t => engine.wall(t))))).size).toBe(100)
    expect(engine.MONSTERS).toHaveLength(80)
    expect(engine.campaign.floorRules.filter(r => r.guardian)).toHaveLength(10)
    for (const [floor, map] of engine.FLOORS.entries()) {
      expect(map).toHaveLength(196)
      const queue = [engine.campaign.start], seen = new Set<string>()
      for (const p of queue) {
        const key = `${p.x}:${p.y}`
        if (seen.has(key)) continue
        seen.add(key)
        for (const n of engine.neighbors(p)) if (engine.inside(n) && !engine.wall(map[n.y * 14 + n.x]!) && map[n.y * 14 + n.x] !== 51 && !seen.has(`${n.x}:${n.y}`)) queue.push(n)
      }
      expect(seen.has('11:1'), `exit floor ${floor + 1}`).toBe(true)
      for (const [index, tile] of map.entries()) if (tile !== 0 && tile !== 51 && !engine.wall(tile)) expect(seen.has(`${index % 14}:${Math.floor(index / 14)}`), `reachable tile ${tile} on floor ${floor + 1}`).toBe(true)
      expect(map.filter(t => t === 99)).toHaveLength(floor === 99 ? 1 : 0)
      if (floor % 10 === 2 || floor % 10 === 6) expect(map).toContain(51)
    }
  })

  it('blocks bypassing a realm lord and requires all ten seals for rescue', () => {
    const s = engine.createClassic(); engine.startClassic(s)
    s.floor = 9; s.discovered.push(9); s.position = { x: 11, y: 2 }
    expect(engine.moveClassic(s, 0, -1)).toBe(false)
    expect(s.notice.key).toBe('guardian')
    expect(s.floor).toBe(9); expect(s.phase).toBe('playing')
    s.floor = 99; s.discovered.push(99)
    const guardian = engine.campaign.floorRules[99]!.guardian!
    s.removed[engine.keyFor(99, guardian)] = true
    expect(engine.moveClassic(s, 0, -1)).toBe(false)
    expect(s.notice.key).toBe('seals')
    for (const [f, rule] of engine.campaign.floorRules.entries()) if (rule.guardian) s.removed[engine.keyFor(f, rule.guardian)] = true
    expect(engine.moveClassic(s, 0, -1)).toBe(true)
    expect(s.phase).toBe('won')
    expect(engine.loadClassic(engine.saveClassic(s))?.phase).toBe('won')
    delete s.removed[engine.keyFor(9, engine.campaign.floorRules[9]!.guardian!)]
    expect(engine.loadClassic(engine.saveClassic(s))).toBeNull()
  })

  it('isolates old saves, validates floor limits and supports late-floor undo history', () => {
    const s = engine.createClassic(); engine.startClassic(s)
    expect(engine.loadClassic(classic.saveClassic(classic.createClassic()))).toBeNull()
    expect(classic.loadClassic(engine.saveClassic(s))).toBeNull()
    s.floor = 99; s.discovered.push(99); s.position = { x: 1, y: 11 }
    expect(engine.moveClassic(s, 0, -1)).toBe(true)
    const saved = engine.saveClassic(s), loaded = engine.loadClassic(saved)!
    expect(loaded).not.toBeNull(); expect(loaded.phase).toBe('paused')
    expect(loaded.floor).toBe(99)
    expect(loaded.removed).toEqual(s.removed)
    expect(engine.undoClassic(loaded)).toBe(true)
    expect(loaded.position).toEqual({ x: 1, y: 11 })
    for (const floor of [-1, 100, 1.5]) {
      const invalid = JSON.parse(saved); invalid.state.floor = floor
      expect(engine.loadClassic(JSON.stringify(invalid))).toBeNull()
    }
    const invalid = JSON.parse(saved); invalid.state.removed['100:1:10'] = true
    expect(engine.loadClassic(JSON.stringify(invalid))).toBeNull()
  })

  it('uses regional shop prices, requires a nearby shrine and refunds purchases on undo', () => {
    const s = engine.createClassic(); engine.startClassic(s); s.stats.gold = 1000
    expect(engine.buyClassic(s, 'attack')).toBe(false)
    s.floor = 42; s.discovered.push(42)
    s.position = engine.pointsOf(42, 51)[0]!
    const before = structuredClone(s.stats)
    expect(engine.shopOffers(s).find(o => o.kind === 'attack')).toMatchObject({ cost: 150, gain: 20 })
    expect(engine.buyClassic(s, 'attack')).toBe(true)
    expect(s.stats.attack).toBe(before.attack + 20); expect(s.stats.gold).toBe(before.gold - 150)
    expect(engine.undoClassic(s)).toBe(true); expect(s.stats).toEqual(before)
  })

  it('shows every point of first strikes and double retaliation in the animation', () => {
    for (const m of engine.MONSTERS) for (const role of ['knight', 'scout'] as const) {
      const s = engine.createClassic(role, 7)
      s.stats.hp = s.stats.maxHp = 1_000_000
      s.stats.attack = m.defense + Math.ceil(m.hp / 7); s.stats.defense = m.attack - 10
      const strikes: BattleStrike[] = [], preview = engine.previewBattle(s, m.id, strikes)!
      const frames = combatFrames(s, m.id, engine.previewBattle)
      expect(preview.canWin).toBe(true); expect(s.seed).toBe(7)
      expect(frames.length).toBeLessThanOrEqual(16)
      expect(frames.filter(f => f.attacker === 'monster').reduce((n, f) => n + f.damage, 0)).toBe(preview.damage)
      expect(frames.at(-1)).toMatchObject({ attacker: 'hero', monsterHp: 0, heroHp: s.stats.hp - preview.damage })
      if (m.ability === 'first') expect(frames[0]).toMatchObject({ attacker: 'monster', round: 0 })
      if (m.ability === 'double') expect(strikes.filter(f => f.round === 1 && f.attacker === 'monster')).toHaveLength(2)
      let hp = s.stats.hp, enemyHp = m.hp
      for (const frame of frames) {
        expect(frame.heroHp).toBeLessThanOrEqual(hp); expect(frame.monsterHp).toBeLessThanOrEqual(enemyHp)
        if (frame.attacker === 'monster') expect(hp - frame.heroHp).toBe(frame.damage)
        hp = frame.heroHp; enemyHp = frame.monsterHp
      }
    }
  })
})

import { describe, expect, it } from 'vitest'
import oldCampaign from '../legacy-50.json'
import { createCampaignEngine, type CampaignDefinition } from '../../campaign-engine'
import * as engine from '../engine'
const legacy = createCampaignEngine(oldCampaign as CampaignDefinition)

describe('50 to 100 floor save migration', () => {
  it('retains the first fifty maps and monsters except the old final stair', () => {
    expect(engine.MONSTERS.slice(0, 40)).toEqual(oldCampaign.monsters)
    for (let floor = 0; floor < 50; floor++) {
      const map = [...oldCampaign.floors[floor]!]
      if (floor === 49) map[25] = 80
      expect(engine.FLOORS[floor]).toEqual(map)
    }
  })
  it('validates original progress and preserves stats, cleared tiles and identity', () => {
    const s = legacy.createClassic('scout', 7)
    legacy.startClassic(s)
    legacy.moveClassic(s, 0, -1)
    const raw = legacy.saveClassic(s), next = engine.migrateLegacy50(raw)!
    expect(next.phase).toBe('paused')
    expect(next.id).toBe(s.id)
    expect(next.stats).toEqual(s.stats)
    expect(next.position).toEqual(s.position)
    expect(next.removed).toEqual(s.removed)
    expect(next.seed).toBe(s.seed)
    expect(next.score).toBe(s.score)
    expect(next.history).toEqual([])
    expect(next.notice.key).toBe('migrated')
    expect(engine.loadClassic(raw)).toBeNull()
    expect(engine.loadClassic(engine.saveClassic(next))).not.toBeNull()
    expect(legacy.loadClassic(raw)).not.toBeNull()
  })
  it('reopens a fifty-floor victory and removes its obsolete final score bonus once', () => {
    const s = legacy.createClassic()
    s.floor = 49; s.discovered = Array.from({ length: 50 }, (_, i) => i)
    s.phase = 'won'; s.position = { x: 11, y: 1 }; s.stats.hp = s.stats.maxHp = 8000
    s.score = 20000 + 2000 + s.stats.hp
    for (const [floor, rule] of oldCampaign.floorRules.entries())
      if ('guardian' in rule && rule.guardian) s.removed[legacy.keyFor(floor, rule.guardian)] = true
    const migrated = engine.migrateLegacy50(legacy.saveClassic(s))!
    expect(migrated.phase).toBe('paused')
    expect(migrated.score).toBe(20000)
    expect(migrated.notice.key).toBe('extended')
    expect(engine.bossesDefeated(migrated)).toBe(false)
    engine.startClassic(migrated)
    expect(engine.moveClassic(migrated, 0, 1)).toBe(true)
    expect(engine.moveClassic(migrated, 0, -1)).toBe(true)
    expect(migrated.floor).toBe(50)
    expect(migrated.phase).toBe('playing')
    expect(engine.loadClassic(engine.saveClassic(migrated))!.score).toBe(20000)
    expect(engine.migrateLegacy50(engine.saveClassic(migrated))).toBeNull()
  })
  it('rejects corrupt legacy saves without interpreting them as new progress', () => {
    const raw = JSON.parse(legacy.saveClassic(legacy.createClassic()))
    for (const floor of [-1, 50, 99]) {
      raw.state.floor = floor
      expect(engine.migrateLegacy50(JSON.stringify(raw))).toBeNull()
    }
    expect(engine.migrateLegacy50('{')).toBeNull()
  })
})

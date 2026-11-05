import { describe, expect, it } from 'vitest'
import { combatFrames } from '../combat'
import { createClassic, MONSTERS, previewBattle } from '../engine'

describe('classic combat presentation', () => {
  it('preserves exact preview damage, HP and RNG for both roles, including grouped long fights', () => {
    for (const role of ['knight', 'scout'] as const) for (const seed of [1, 7, 99, 1234]) for (const monster of MONSTERS) {
      const s = createClassic(role, seed); s.stats.hp = 100000; s.stats.maxHp = 100000
      const preview = previewBattle(s, monster.id)!, frames = combatFrames(s, monster.id)
      expect(s.seed).toBe(seed)
      expect(frames.length).toBeLessThanOrEqual(15)
      expect(frames.filter(f => f.attacker === 'monster').reduce((sum, f) => sum + f.damage, 0)).toBe(preview.damage)
      expect(frames[frames.length - 1]).toMatchObject({ attacker: 'hero', monsterHp: 0, heroHp: s.stats.hp - preview.damage })
      let hp = s.stats.hp, enemyHp = monster.hp
      for (const frame of frames) {
        expect(frame.heroHp).toBeLessThanOrEqual(hp); expect(frame.monsterHp).toBeLessThanOrEqual(enemyHp)
        if (frame.attacker === 'monster') expect(hp - frame.heroHp).toBe(frame.damage)
        hp = frame.heroHp; enemyHp = frame.monsterHp
      }
    }
  })
  it('does not animate walls or battles that cannot be survived', () => {
    const s = createClassic(); s.stats.hp = 1
    expect(combatFrames(s, 2)).toEqual([])
    expect(combatFrames(s, 23)).toEqual([])
  })
})

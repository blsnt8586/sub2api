import { describe, expect, it } from 'vitest'
import campaign from '../campaign.json'
import { battleCount, buyClassic, createClassic, FLOORS, keyFor, loadClassic, moveClassic, pointsOf, previewBattle, rawTile, saveClassic, startClassic, tileAt, undoClassic } from '../engine'
import { runClassicCampaign } from './campaign'

const playing = () => { const s = createClassic('knight', 1); startClassic(s); return s }
describe('classic imported campaign', () => {
  it('keeps all ten original 14 by 14 maps and thirteen original enemy tables', () => {
    expect(campaign.sourceRevision).toBe('a32ee738485fa210e3a2b32cf57f506d51446c13')
    expect(FLOORS).toHaveLength(10)
    expect(FLOORS.every(f => f.length === 196)).toBe(true)
    expect(campaign.monsters).toHaveLength(13)
    expect(campaign.monsters[0]).toMatchObject({ id: 11, hp: 50, attack: 12, defense: 9, gold: 3, experience: 1 })
    expect(pointsOf(4, 80)).toHaveLength(2)
    expect(pointsOf(5, 90)).toHaveLength(2)
  })
  it('pairs both fire-floor stairs and repairs the missing ninth-floor return', () => {
    const s = playing()
    for (const p of pointsOf(4, 80)) {
      s.floor = 4; s.position = { x: p.x + 1, y: p.y }
      expect(moveClassic(s, -1, 0)).toBe(true)
      expect(s.floor).toBe(5); expect(s.position).toEqual(p)
      // Walk off then back to the arrival stair without retriggering on landing.
      s.position = { x: p.x + 1, y: p.y }
      expect(moveClassic(s, -1, 0)).toBe(true)
      expect(s.floor).toBe(4); expect(s.position).toEqual(p)
    }
    expect(FLOORS[8]![12 * 14 + 12]).toBe(60)
    expect(rawTile(8, { x: 12, y: 12 })).toBe(90)
  })
})
describe('classic browser rules', () => {
  it('supports both original starting stat profiles', () => {
    expect(createClassic().stats).toMatchObject({ hp: 150, attack: 14, defense: 10 })
    expect(createClassic('scout').stats).toMatchObject({ hp: 200, attack: 13, defense: 10 })
  })
  it('does not consume RNG in previews, and matches battle cost including dodges and criticals', () => {
    const s = playing(); s.position = { x: 5, y: 1 }
    const preview = previewBattle(s, 11)!
    expect(previewBattle(s, 11)).toEqual(preview); expect(s.seed).toBe(1)
    expect(moveClassic(s, 0, 1)).toBe(true)
    expect(s.stats.hp).toBe(150 - preview.damage)
    expect(s.seed).toBe(preview.seed); expect(s.stats.gold).toBe(3)
    undoClassic(s)
    expect(s.stats.hp).toBe(150); expect(s.seed).toBe(1)
    expect(previewBattle(s, 11)).toEqual(preview)
  })
  it('blocks lethal combat and ignores invalid movement without creating undo entries', () => {
    const s = playing(); s.stats.hp = 1; s.position = { x: 5, y: 1 }
    expect(moveClassic(s, 0, 1)).toBe(false)
    expect(s.stats.hp).toBe(1); expect(s.history).toHaveLength(0)
    for (const d of [[1, 1], [0, 0], [0.5, 0], [NaN, 0], [99, 0]]) expect(moveClassic(s, d[0]!, d[1]!)).toBe(false)
  })
  it('collects the formerly inaccessible book once and undoes the pickup', () => {
    const s = playing(); s.position = { x: 1, y: 11 }
    expect(moveClassic(s, 0, 1)).toBe(true); expect(s.stats.book).toBe(true)
    undoClassic(s); expect(s.stats.book).toBe(false)
    expect(tileAt(s, { x: 1, y: 12 })).toBe(35)
  })
  it('consumes one key for each original door and does not farm cleared pickups', () => {
    const s = playing(); s.position = { x: 5, y: 3 }; s.stats.keys.yellow = 1
    expect(moveClassic(s, 0, 1)).toBe(true); expect(s.stats.keys.yellow).toBe(0)
    s.removed[keyFor(0, { x: 5, y: 3 })] = true
    moveClassic(s, 0, -1); moveClassic(s, 0, 1)
    expect(s.stats.keys.yellow).toBe(0)
  })
  it('requires adjacency at the original shrine and accepts exactly the red-key price', () => {
    const s = playing(); s.stats.gold = 300
    expect(buyClassic(s, 'red')).toBe(false)
    s.floor = 2; s.position = { x: 6, y: 2 }
    expect(buyClassic(s, 'red')).toBe(true); expect(s.stats.gold).toBe(0); expect(s.stats.keys.red).toBe(1)
    undoClassic(s); expect(s.stats.gold).toBe(300); expect(s.stats.keys.red).toBe(0)
  })
  it('requires explicit curse acceptance and restores cursed stats with undo', () => {
    const s = playing(); s.floor = 9; s.position = { x: 7, y: 10 }
    expect(moveClassic(s, 0, -1)).toBe(false); expect(s.stats.attack).toBe(14)
    expect(moveClassic(s, 0, -1, true)).toBe(true); expect(s.stats.attack).toBe(4)
    undoClassic(s); expect(s.stats.attack).toBe(14)
  })
  it('round-trips saves, rejects corrupt values and bounds undo history', () => {
    const s = playing(); moveClassic(s, 1, 0)
    const restored = loadClassic(saveClassic(s))!
    expect(restored.phase).toBe('paused'); expect(restored.stats).toEqual(s.stats); expect(restored.seed).toBe(s.seed)
    expect(undoClassic(restored)).toBe(true)
    for (const raw of ['null', '{}', 'broken']) expect(loadClassic(raw)).toBeNull()
    const original = JSON.parse(saveClassic(s))
    for (const patch of [{ floor: 99 }, { position: { x: 0, y: 0 } }, { seed: -1 }, { phase: 'won' }, { removed: { '0:99:99': true } }]) expect(loadClassic(JSON.stringify({ ...original, state: { ...original.state, ...patch } }))).toBeNull()
  })
  it('triggers rescue only at the summit princess and does not permit victory undo', () => {
    const s = playing(); s.floor = 9; s.discovered.push(9); s.position = { x: 7, y: 2 }
    expect(moveClassic(s, 0, -1)).toBe(true); expect(s.phase).toBe('won')
    expect(undoClassic(s)).toBe(false); expect(loadClassic(saveClassic(s))?.phase).toBe('won')
  })
})
describe('legal classic campaign', () => {
  it('rescues the princess by walking, buying and accepting curses without modifying gameplay stats', () => {
    const result = runClassicCampaign()
    expect(result.state.phase).toBe('won')
    expect(result.state.discovered).toHaveLength(10)
    expect(battleCount(result.state)).toBeGreaterThan(30)
    expect(result.state.removed[keyFor(9, { x: 7, y: 3 })]).toBe(true)
    console.info('Classic campaign', JSON.stringify({ actions: result.actions.length, damage: result.state.damage, stats: result.state.stats, score: result.state.score }))
  }, 30_000)
})

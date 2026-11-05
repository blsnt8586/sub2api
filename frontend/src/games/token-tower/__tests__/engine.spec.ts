import { describe, expect, it } from 'vitest'
import { DOORS } from '../data'
import { battleCount, buyUpgrade, createTower, findTile, FLOORS, keyFor, loadTower, monsterFor, moveTower, pauseTower, previewBattle, rawTile, relicCount, saveTower, shopCost, SIZE, stars, startTower, tileAt, undoTower, usePotion, type Monster } from '../engine'
import { runCampaign } from './campaign'

const playing = () => { const s = createTower(); startTower(s); return s }
const enemy = (overrides: Partial<Monster> = {}): Monster => ({ id: 'slime', hp: 50, attack: 20, defense: 5, reward: 8, ability: 'normal', ...overrides })

describe('TOKEN Tower maps', () => {
  it('has ten bordered, connected maps with correct stairs and exactly three runes', () => {
    let runes = 0
    FLOORS.forEach((floor, f) => {
      expect(floor.map).toHaveLength(SIZE)
      expect(floor.map.every(row => row.length === SIZE)).toBe(true)
      expect(floor.map[0]).toBe('#'.repeat(SIZE))
      expect(floor.map[SIZE - 1]).toBe('#'.repeat(SIZE))
      expect(floor.map.slice(1, -1).every(row => row.startsWith('#') && row.endsWith('#'))).toBe(true)
      expect(floor.map.join('').split(f === 0 ? 'S' : '<')).toHaveLength(2)
      expect(floor.map.join('').split('>')).toHaveLength(f === 9 ? 1 : 2)
      runes += floor.map.join('').split('o').length - 1
      const start = findTile(f, f === 0 ? 'S' : '<'), seen = new Set([`${start.x}:${start.y}`]), queue = [start]
      for (const p of queue) for (const q of [{ x: p.x + 1, y: p.y }, { x: p.x - 1, y: p.y }, { x: p.x, y: p.y + 1 }, { x: p.x, y: p.y - 1 }]) {
        const key = `${q.x}:${q.y}`
        if (rawTile(f, q) !== '#' && !seen.has(key)) { seen.add(key); queue.push(q) }
      }
      expect(seen.size, floor.id).toBe(floor.map.join('').split('').filter(t => t !== '#').length)
      for (const [tile, color] of Object.entries(DOORS)) expect(floor.map.join('').split(tile).length - 1).toBeLessThanOrEqual(floor.map.join('').split(color === 'red' ? 'r' : color === 'blue' ? 'b' : 'y').length - 1)
    })
    expect(runes).toBe(3)
    expect(FLOORS).toHaveLength(10)
  })
})

describe('TOKEN Tower combat', () => {
  it('lets the hero strike first and removes retaliation on the killing blow', () => {
    expect(previewBattle(playing(), enemy())).toMatchObject({ hits: 2, damageTaken: 5, canWin: true })
    expect(previewBattle(playing(), enemy({ hp: 1 }))).toMatchObject({ hits: 1, damageTaken: 0 })
  })
  it('accounts for first strike, double attacks and defense-ignoring magic', () => {
    const s = playing()
    expect(previewBattle(s, enemy({ ability: 'first' })).damageTaken).toBe(10)
    expect(previewBattle(s, enemy({ ability: 'double' })).damageTaken).toBe(10)
    expect(previewBattle(s, enemy({ ability: 'magic' })).damageTaken).toBe(20)
  })
  it('handles unbreakable armor, zero enemy damage and lethal exact-health results', () => {
    const s = playing()
    expect(previewBattle(s, enemy({ defense: 35 }))).toMatchObject({ hits: null, damageTaken: null, canWin: false })
    expect(previewBattle(s, enemy({ attack: 10 })).damageTaken).toBe(0)
    s.stats.hp = 5
    expect(previewBattle(s, enemy()).canWin).toBe(false)
  })
  it('requires all three runes to remove the final seal', () => {
    const s = playing(), boss = monsterFor(9, 'X')!
    s.stats.attack = 1000; s.stats.defense = 1000
    expect(previewBattle(s, boss).canWin).toBe(false)
    for (const f of [2, 5, 7]) s.removed[keyFor(f, findTile(f, 'o'))] = true
    expect(relicCount(s)).toBe(3)
    expect(previewBattle(s, boss).canWin).toBe(true)
  })
})

describe('TOKEN Tower actions', () => {
  it('blocks movement before start, through walls, during pause and with invalid directions', () => {
    const s = createTower()
    expect(moveTower(s, 1, 0)).toBe(false)
    startTower(s)
    expect(moveTower(s, -1, 0)).toBe(false)
    for (const d of [[9, 0], [1, 1], [0, 0], [NaN, 0], [0.5, 0]]) expect(moveTower(s, d[0]!, d[1]!)).toBe(false)
    pauseTower(s)
    expect(moveTower(s, 1, 0)).toBe(false)
    expect(s.history).toHaveLength(0)
  })
  it('collects gems and equipment once, and updates battle predictions', () => {
    const s = playing()
    s.position = { x: 3, y: 1 }
    moveTower(s, 1, 0)
    expect(s.stats.attack).toBe(38)
    moveTower(s, -1, 0); moveTower(s, 1, 0)
    expect(s.stats.attack).toBe(38)
    s.floor = 1; s.position = { x: 2, y: 1 }; s.stats.attack = 100; s.stats.defense = 100
    moveTower(s, -1, 0)
    expect(s.stats.swords).toBe(1)
    expect(s.stats.attack).toBe(112)
  })
  it('charges exactly one key per locked door and lets the hero return through it', () => {
    const s = playing(); s.position = { x: 5, y: 1 }; s.stats.keys.yellow = 1
    expect(moveTower(s, 1, 0)).toBe(true)
    expect(s.stats.keys.yellow).toBe(0)
    moveTower(s, -1, 0)
    expect(moveTower(s, 1, 0)).toBe(true)
    expect(s.stats.keys.yellow).toBe(0)
  })
  it('refuses doors without keys and preserves inventory on a blocked move', () => {
    const s = playing(); s.position = { x: 5, y: 1 }; s.stats.keys.yellow = 0
    expect(moveTower(s, 1, 0)).toBe(false)
    expect(s.position).toEqual({ x: 5, y: 1 })
    expect(s.notice.key).toBe('yellowKey')
    expect(s.history).toHaveLength(0)
  })
  it('awards combat gold once, undoes the whole battle and prevents duplicate farming', () => {
    const s = playing(); s.position = { x: 1, y: 1 }
    moveTower(s, 1, 0)
    expect(s.stats.hp).toBe(1195)
    expect(s.stats.gold).toBe(8)
    expect(undoTower(s)).toBe(true)
    expect(s.stats.hp).toBe(1200)
    expect(s.stats.gold).toBe(0)
    moveTower(s, 1, 0); moveTower(s, -1, 0); moveTower(s, 1, 0)
    expect(s.stats.gold).toBe(8)
  })
  it('grants experience on kills, levels deterministically and restores progression on undo', () => {
    const s = playing(); s.position = { x: 1, y: 1 }; s.stats.experience = 39
    expect(moveTower(s, 1, 0)).toBe(true)
    expect(s.stats).toMatchObject({ experience: 7, level: 2, nextLevelExperience: 60, attack: 39, defense: 18, maxHp: 1300 })
    expect(s.notice).toMatchObject({ key: 'levelUp', values: { n: 2 } })
    expect(undoTower(s)).toBe(true)
    expect(s.stats).toMatchObject({ experience: 39, level: 1, nextLevelExperience: 40, attack: 35, defense: 15, maxHp: 1200 })
  })
  it('refuses lethal combat without consuming resources or moving the player', () => {
    const s = playing(); s.position = { x: 1, y: 1 }; s.stats.hp = 5
    expect(moveTower(s, 1, 0)).toBe(false)
    expect(s.stats.hp).toBe(5)
    expect(s.stats.gold).toBe(0)
    expect(s.encounter).toEqual({ x: 2, y: 1 })
    expect(s.history).toHaveLength(0)
  })
  it('stores potions without healing on pickup, heals only when needed and supports undo', () => {
    const s = playing(); s.position = { x: 3, y: 9 }; s.stats.hp = 1100
    moveTower(s, 1, 0)
    expect(s.stats.potions).toBe(2)
    expect(s.stats.hp).toBe(1100)
    expect(usePotion(s)).toBe(true)
    expect(s.stats.hp).toBe(1200)
    expect(s.stats.potions).toBe(1)
    expect(usePotion(s)).toBe(false)
    undoTower(s)
    expect(s.stats.hp).toBe(1100)
    expect(s.stats.potions).toBe(2)
  })
  it('restricts purchases to a merchant and raises prices after every purchase', () => {
    const s = playing(); s.stats.gold = 100
    expect(buyUpgrade(s, 'attack')).toBe(false)
    s.position = findTile(0, 'C')
    expect(buyUpgrade(s, 'attack')).toBe(true)
    expect(s.stats.gold).toBe(60)
    expect(shopCost(s)).toBe(60)
    expect(buyUpgrade(s, 'defense')).toBe(true)
    expect(s.stats.gold).toBe(0)
    expect(buyUpgrade(s, 'health')).toBe(false)
    undoTower(s)
    expect(shopCost(s)).toBe(60)
    expect(s.stats.defense).toBe(15)
  })
  it('keeps cleared items and enemies when traveling downstairs and upstairs', () => {
    const s = playing(); s.position = { x: 3, y: 1 }; moveTower(s, 1, 0)
    s.position = { x: 8, y: 1 }; moveTower(s, 1, 0)
    expect(s.floor).toBe(1)
    expect(s.position).toEqual(findTile(1, '<'))
    moveTower(s, 1, 0); moveTower(s, -1, 0)
    expect(s.floor).toBe(0)
    expect(tileAt(s, { x: 4, y: 1 })).toBe('.')
    expect(s.stats.attack).toBe(38)
    expect(undoTower(s)).toBe(true)
    expect(s.floor).toBe(1)
  })
  it('blocks the midboss exit until the Warden is defeated', () => {
    const s = playing(); s.floor = 4; s.position = { x: 8, y: 1 }
    expect(moveTower(s, 1, 0)).toBe(false)
    expect(s.notice.key).toBe('bossGate')
    s.removed[keyFor(4, findTile(4, 'X'))] = true
    expect(moveTower(s, 1, 0)).toBe(true)
    expect(s.floor).toBe(5)
  })
})

describe('TOKEN Tower saves', () => {
  it('round-trips an active game into paused state with undo and collectibles preserved', () => {
    const s = playing(); moveTower(s, 1, 0); moveTower(s, 1, 0)
    const restored = loadTower(saveTower(s))!
    expect(restored).not.toBeNull()
    expect(restored.phase).toBe('paused')
    expect(restored.id).toBe(s.id)
    expect(restored.stats).toEqual(s.stats)
    expect(undoTower(restored)).toBe(true)
    expect(restored.position).toEqual({ x: 2, y: 9 })
  })
  it('migrates version-two saves into the experience progression', () => {
    const s = playing(); moveTower(s, 1, 0)
    const source = JSON.parse(saveTower(s))
    source.version = 2
    for (const snapshot of [source.state, ...source.history]) {
      delete snapshot.stats.experience
      delete snapshot.stats.level
      delete snapshot.stats.nextLevelExperience
    }
    const restored = loadTower(JSON.stringify(source))!
    expect(restored.stats).toMatchObject({ experience: 0, level: 1, nextLevelExperience: 40 })
    expect(undoTower(restored)).toBe(true)
    expect(restored.stats.level).toBe(1)
  })
  it('rejects corrupt, old, out-of-bounds and non-finite saves', () => {
    expect(loadTower('broken')).toBeNull()
    expect(loadTower('null')).toBeNull()
    expect(loadTower('{"version":1}')).toBeNull()
    const source = JSON.parse(saveTower(playing()))
    for (const patch of [{ floor: 99 }, { position: { x: 0, y: 0 } }, { stats: { hp: -1 } }, { removed: { '__proto__': true, '9:99:99': true } }, { phase: 'won' }]) expect(loadTower(JSON.stringify({ ...source, state: { ...source.state, ...patch } }))).toBeNull()
  })

  it('ignores unrecognized runtime fields and removes undo entries from a different run', () => {
    const source = JSON.parse(saveTower(playing()))
    source.state.event = null
    source.state.notice = null
    source.history = [{ ...source.state, id: 'tower-other-game' }]
    const loaded = loadTower(JSON.stringify(source))!
    expect(loaded.event.id).toBe(0)
    expect(loaded.notice.key).toBe('restored')
    expect(loaded.history).toHaveLength(0)
  })
})

describe('complete legal TOKEN Tower campaign', () => {
  it('collects all three runes and clears all ten floors using only legal moves, potions and shop purchases', () => {
    const result = runCampaign()
    expect(result.state.phase).toBe('won')
    expect(result.checkpoints).toHaveLength(9)
    expect(result.state.discovered).toHaveLength(10)
    expect(relicCount(result.state)).toBe(3)
    expect(battleCount(result.state)).toBeGreaterThanOrEqual(30)
    expect(result.state.stats.hp).toBeGreaterThan(0)
    expect(stars(result.state)).toBeGreaterThanOrEqual(2)
    expect(loadTower(saveTower(result.state))?.phase).toBe('won')
    const score = result.state.stats.score
    expect(moveTower(result.state, 1, 0)).toBe(false)
    expect(undoTower(result.state)).toBe(false)
    expect(result.state.stats.score).toBe(score)
    console.info('TOKEN Tower campaign:', JSON.stringify({ checkpoints: result.checkpoints, result: result.state.stats, actions: result.actions.length, damage: result.state.damage }))
  })
})

import { DOORS } from '../data'
import { buyUpgrade, createTower, FLOORS, monsterFor, moveTower, previewBattle, shopCost, SIZE, startTower, tileAt, usePotion, type Point, type TowerState } from '../engine'

export type CampaignAction = { kind: 'move'; dx: number; dy: number } | { kind: 'heal' } | { kind: 'shop'; upgrade: 'attack' | 'defense' | 'health' }
const neighbors = (p: Point) => [{ x: p.x + 1, y: p.y }, { x: p.x - 1, y: p.y }, { x: p.x, y: p.y + 1 }, { x: p.x, y: p.y - 1 }]
export function reachable(s: TowerState) {
  const paths = new Map<string, Point[]>([[`${s.position.x}:${s.position.y}`, []]])
  const queue = [{ point: s.position, path: [] as Point[] }]
  const targets: { point: Point; path: Point[] }[] = []
  for (const node of queue) for (const point of neighbors(node.point)) {
    if (point.x < 0 || point.y < 0 || point.x >= SIZE || point.y >= SIZE) continue
    const key = `${point.x}:${point.y}`
    if (paths.has(key)) continue
    const tile = tileAt(s, point)
    if (tile === '#' || tile === '<') continue
    const path = [...node.path, point]
    paths.set(key, path)
    if (['.', 'S', 'C', 'n'].includes(tile)) queue.push({ point, path })
    if (tile !== '.') targets.push({ point, path })
  }
  if (tileAt(s) === 'C') targets.push({ point: s.position, path: [] })
  return targets
}

// A reproducible legal-input playthrough, also reusable for browser verification.
export function runCampaign() {
  const s = createTower(), actions: CampaignAction[] = [], checkpoints: { floor: number; hp: number; attack: number; defense: number; damage: number; potions: number }[] = []
  startTower(s)
  const walk = (path: Point[]) => { for (const p of path) { const action = { kind: 'move' as const, dx: p.x - s.position.x, dy: p.y - s.position.y }; if (!moveTower(s, action.dx, action.dy)) throw new Error(`Blocked legal route on floor ${s.floor + 1}: ${JSON.stringify(s.notice)}`); actions.push(action) } }
  let loops = 0
  while (s.phase !== 'won' && loops++ < 3000) {
    if (s.stats.potions && s.stats.hp <= s.stats.maxHp - 300) { if (usePotion(s)) actions.push({ kind: 'heal' }); continue }
    const options = reachable(s)
    const items = options.filter(n => 'rbyadp$sho'.includes(tileAt(s, n.point)))
    if (items.length) { items.sort((a, b) => a.path.length - b.path.length); walk(items[0]!.path); continue }
    const door = options.filter(n => { const color = DOORS[tileAt(s, n.point)]; return color && s.stats.keys[color] > 0 }).sort((a, b) => a.path.length - b.path.length)[0]
    if (door) { walk(door.path); continue }
    const merchant = options.find(n => tileAt(s, n.point) === 'C')
    if (merchant && s.stats.gold >= shopCost(s)) {
      walk(merchant.path)
      while (s.stats.gold >= shopCost(s)) {
        const enemies = [s.floor, Math.min(9, s.floor + 1)].flatMap(f => FLOORS[f]!.map.flatMap(row => [...row].flatMap(tile => { const m = monsterFor(f, tile as Parameters<typeof monsterFor>[1]); return m ? [m] : [] })))
        const estimate = () => enemies.reduce((sum, m) => sum + Math.min(3000, previewBattle(s, m).damageTaken ?? 3000), 0)
        const base = estimate()
        s.stats.attack += 5; const attackBenefit = base - estimate(); s.stats.attack -= 5
        s.stats.defense += 5; const defenseBenefit = base - estimate(); s.stats.defense -= 5
        const upgrade = s.stats.hp < 400 && s.stats.potions === 0 ? 'health' : attackBenefit >= defenseBenefit ? 'attack' : 'defense'
        if (!buyUpgrade(s, upgrade)) throw new Error('Shop failed')
        actions.push({ kind: 'shop', upgrade })
      }
      continue
    }
    const fights = options.flatMap(n => { const monster = monsterFor(s.floor, tileAt(s, n.point)); const preview = monster ? previewBattle(s, monster) : null; return preview?.canWin ? [{ ...n, damage: preview.damageTaken ?? Infinity }] : [] }).sort((a, b) => a.damage - b.damage || a.path.length - b.path.length)
    if (fights.length) { walk(fights[0]!.path); continue }
    const exit = options.find(n => tileAt(s, n.point) === '>')
    if (exit) {
      checkpoints.push({ floor: s.floor + 1, hp: s.stats.hp, attack: s.stats.attack, defense: s.stats.defense, damage: s.damage, potions: s.stats.potions })
      walk(exit.path)
      continue
    }
    throw new Error(`Campaign stuck at floor ${s.floor + 1}: ${JSON.stringify({ stats: s.stats, notice: s.notice, targets: options.map(o => tileAt(s, o.point)) })}`)
  }
  if (s.phase !== 'won') throw new Error('Campaign iteration limit')
  return { state: s, actions, checkpoints }
}

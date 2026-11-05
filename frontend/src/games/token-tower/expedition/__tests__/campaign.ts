import * as engine from '../engine'
import type { Point, Role, State, Upgrade } from '../../campaign-engine'

/** Legal inputs only: collect reachable loot, open affordable doors, then fight
 * the cheapest remaining battle. Never walks through a battle or stairs to pathfind. */
export function reachable(state: State) {
  const queue = [{ point: state.position, path: [] as Point[] }]
  const seen = new Set([`${state.position.x}:${state.position.y}`])
  const targets: { point: Point; path: Point[]; tile: number }[] = []
  for (const node of queue) for (const point of engine.neighbors(node.point)) {
    const key = `${point.x}:${point.y}`, tile = engine.tileAt(state, point)
    if (seen.has(key) || engine.wall(tile)) continue
    seen.add(key)
    const path = [...node.path, point]
    if (tile === 0) queue.push({ point, path })
    else targets.push({ point, path, tile })
  }
  return { targets, clear: queue }
}

// Return trips use only already-cleared ground and real paired stairs.
function clearedRoute(state: State, match = engine.nearShop): [number, number][] | null {
  const queue = [{ floor: state.floor, point: state.position, path: [] as [number, number][] }]
  const seen = new Set<string>()
  for (const node of queue) {
    const key = `${node.floor}:${node.point.x}:${node.point.y}`
    if (seen.has(key)) continue
    seen.add(key)
    const view = { ...state, floor: node.floor, position: node.point }
    if (match(view)) return node.path
    for (const point of engine.neighbors(node.point)) {
      const tile = engine.tileAt(view, point), path = [...node.path, [point.x - node.point.x, point.y - node.point.y] as [number, number]]
      if (tile === 0) queue.push({ floor: node.floor, point, path })
      else if (tile === 80 || tile === 90) {
        const floor = node.floor + (tile === 80 ? 1 : -1)
        if (!state.discovered.includes(floor)) continue
        const landing = engine.pointsOf(floor, tile === 80 ? 90 : 80)[0]
        if (landing) queue.push({ floor, point: landing, path })
      }
    }
  }
  return null
}

export function runExpedition(role: Role = 'knight', seed = 1, untilFloor = engine.FLOORS.length + 1) {
  let state = engine.createClassic(role, seed)
  const actions: ([number, number] | Upgrade)[] = []
  const checkpoints: { floor: number; hp: number; attack: number; defense: number; damage: number; gold: number }[] = []
  engine.startClassic(state)
  const walk = (path: Point[]) => { for (const p of path) {
    const dx = p.x - state.position.x, dy = p.y - state.position.y
    if (!engine.moveClassic(state, dx, dy)) throw new Error(`Blocked ${state.floor + 1}: ${state.notice.key}`)
    actions.push([dx, dy])
  } }
  for (let tries = 0; tries < 24000 && state.phase !== 'won'; tries++) {
    if (state.floor + 1 >= untilFloor && untilFloor <= engine.FLOORS.length) break
    const { targets, clear } = reachable(state)
    const loot = targets.filter(t => engine.KEYS[t.tile] || t.tile >= 28 && t.tile <= 35).sort((a, b) => a.path.length - b.path.length)[0]
    if (loot) { walk(loot.path); continue }
    const door = targets.find(t => engine.DOORS[t.tile] && state.stats.keys[engine.DOORS[t.tile]!] > 0)
    if (door) { walk(door.path); continue }
    const battles = targets.filter(t => engine.monsterFor(t.tile)).map(t => ({ ...t, battle: engine.previewBattle(state, t.tile)! })).sort((a, b) => a.battle.damage - b.battle.damage)
    // Spend earned gold at reachable shrines, balancing growth with survival.
    const shrine = clear.find(node => [node.point, ...engine.neighbors(node.point)].some(p => engine.tileAt(state, p) === 51))
    if (shrine && state.stats.gold >= engine.shopOffers(state)[0]!.cost) {
      walk(shrine.path)
      const upgrade: Upgrade = state.stats.hp < 250 * engine.zoneOf(state) ? 'health' : state.stats.attack < state.stats.defense * 1.2 ? 'attack' : 'defense'
      if (!engine.buyClassic(state, upgrade)) throw new Error('Shop purchase failed')
      actions.push(upgrade); continue
    }
    const battle = battles.find(t => t.battle.canWin)
    if (!battle && battles.length && state.stats.gold >= 50) {
      const home = { floor: state.floor, position: { ...state.position } }
      const route = clearedRoute(state, s => engine.nearShop(s) && engine.shopOffers(s)[0]!.cost <= s.stats.gold)
      if (route) {
        for (const [dx, dy] of route) {
          if (!engine.moveClassic(state, dx, dy)) throw new Error('Return trip blocked')
          actions.push([dx, dy])
        }
        // Recover enough to survive the blocked encounter, then grow attack.
        const requiredHp = battles[0]!.battle.damage + 150 * engine.zoneOf(state)
        while (state.stats.gold >= engine.shopOffers(state)[0]!.cost) {
          const upgrade = state.stats.hp < requiredHp ? 'health' : 'attack'
          engine.buyClassic(state, upgrade); actions.push(upgrade)
        }
        const outward = clearedRoute(state, s => s.floor === home.floor && engine.same(s.position, home.position))
        if (!outward) throw new Error('No outward trip')
        for (const [dx, dy] of outward) {
          if (!engine.moveClassic(state, dx, dy)) throw new Error('Outward trip blocked')
          actions.push([dx, dy])
        }
        continue
      }
    }
    if (battle) { walk(battle.path); continue }
    const up = targets.find(t => t.tile === 80 || t.tile === 99)
    if (up) {
      const before = state.floor
      walk(up.path)
      if (state.floor !== before || state.phase === 'won') {
        checkpoints.push({ floor: before + 1, hp: state.stats.hp, attack: state.stats.attack, defense: state.stats.defense, damage: state.damage, gold: state.stats.gold })
        // The full legal route also exercises real save validation beyond floor ten.
        const restored = engine.loadClassic(engine.saveClassic(state))
        if (!restored) throw new Error(`Save rejected on floor ${state.floor + 1}`)
        state = restored; engine.startClassic(state)
      }
      continue
    }
    throw new Error(`Stuck floor ${state.floor + 1} HP=${state.stats.hp} ATK=${state.stats.attack} DEF=${state.stats.defense} gold=${state.stats.gold} targets=${JSON.stringify(battles.map(t => [t.tile, t.battle.damage]))}`)
  }
  return { state, actions, checkpoints }
}

import { DOORS, EQUIPMENT_GAIN, FLOORS, GEM_GAIN, KEYS, SIZE, monsterFor, type Monster, type Tile } from './data'
export { FLOORS, monsterFor, SIZE } from './data'
export type { Monster, Tile } from './data'

export interface Point { x: number; y: number }
export interface TowerStats {
  hp: number
  maxHp: number
  attack: number
  defense: number
  gold: number
  score: number
  experience: number
  level: number
  nextLevelExperience: number
  keys: { red: number; blue: number; yellow: number }
  potions: number
  gems: number
  swords: number
  shields: number
}
export type TowerPhase = 'ready' | 'playing' | 'paused' | 'won'
export interface Notice { key: string; values?: Record<string, string | number> }
export interface TowerSnapshot {
  id: string
  phase: TowerPhase
  floor: number
  position: Point
  stats: TowerStats
  removed: Record<string, true>
  discovered: number[]
  steps: number
  upgrades: number
  damage: number
}
export interface TowerState extends TowerSnapshot {
  history: TowerSnapshot[]
  notice: Notice
  event: { id: number; kind: 'move' | 'loot' | 'fight' | 'door' | 'floor' | 'blocked' | 'heal' | 'shop' | 'undo' | 'won'; at: Point; value: number }
  encounter: Point | null
}
export interface BattlePreview {
  monster: Monster
  canWin: boolean
  hits: number | null
  damageTaken: number | null
  reward: number
  sealed: boolean
}
export const keyFor = (floor: number, point: Point) => `${floor}:${point.x}:${point.y}`
export const rawTile = (floor: number, p: Point): Tile => (FLOORS[floor]?.map[p.y]?.[p.x] ?? '#') as Tile
export const tileAt = (s: TowerState, p = s.position): Tile => s.removed[keyFor(s.floor, p)] ? '.' : rawTile(s.floor, p)
export const relicCount = (s: TowerSnapshot) => Object.keys(s.removed).filter(key => {
  const [floor, x, y] = key.split(':').map(Number)
  return rawTile(floor!, { x: x!, y: y! }) === 'o'
}).length
export const battleCount = (s: TowerSnapshot) => Object.keys(s.removed).filter(key => {
  const [floor, x, y] = key.split(':').map(Number)
  return Boolean(monsterFor(floor!, rawTile(floor!, { x: x!, y: y! })))
}).length
export const shopCost = (s: TowerSnapshot) => 40 + s.upgrades * 20
export const stars = (s: TowerSnapshot) => s.phase === 'won' ? 1 + Number(s.damage < 3500) + Number(battleCount(s) >= 30) : 0

export function findTile(floor: number, tile: Tile): Point {
  for (let y = 0; y < SIZE; y++) {
    const x = FLOORS[floor]?.map[y]?.indexOf(tile) ?? -1
    if (x >= 0) return { x, y }
  }
  return { x: 1, y: SIZE - 2 }
}

export function createTower(): TowerState {
  return {
    id: `tower-${Date.now()}-${Math.random().toString(36).slice(2, 9)}`,
    phase: 'ready', floor: 0, position: findTile(0, 'S'),
    stats: { hp: 1200, maxHp: 1200, attack: 35, defense: 15, gold: 0, score: 0, experience: 0, level: 1, nextLevelExperience: 40, keys: { red: 0, blue: 0, yellow: 1 }, potions: 1, gems: 0, swords: 0, shields: 0 },
    removed: {}, discovered: [0], steps: 0, upgrades: 0, damage: 0, history: [],
    notice: { key: 'welcome' }, encounter: null,
    event: { id: 0, kind: 'move', at: { x: 1, y: 9 }, value: 0 },
  }
}

export function previewBattle(s: TowerState, monster: Monster): BattlePreview {
  const dealt = s.stats.attack - monster.defense
  const hits = dealt > 0 ? Math.ceil(monster.hp / dealt) : null
  const sealed = monster.ability === 'seal' && relicCount(s) < 3
  const incoming = monster.ability === 'magic' ? monster.attack : Math.max(0, monster.attack - s.stats.defense)
  const retaliations = hits === null ? null : Math.max(0, hits - 1) + Number(monster.ability === 'first')
  const damageTaken = retaliations === null ? null : retaliations * incoming * (monster.ability === 'double' ? 2 : 1)
  return { monster, hits, damageTaken, canWin: !sealed && damageTaken !== null && damageTaken < s.stats.hp, reward: monster.reward, sealed }
}

function snapshot(s: TowerSnapshot): TowerSnapshot {
  const { id, phase, floor, position, stats, removed, discovered, steps, upgrades, damage } = s
  return JSON.parse(JSON.stringify({ id, phase, floor, position, stats, removed, discovered, steps, upgrades, damage })) as TowerSnapshot
}
function remember(s: TowerState) {
  s.history.push(snapshot(s))
  if (s.history.length > 40) s.history.shift()
}
function emit(s: TowerState, kind: TowerState['event']['kind'], at: Point, value = 0) {
  s.event = { id: s.event.id + 1, kind, at: { ...at }, value }
}
function blocked(s: TowerState, key: string, at: Point, values?: Notice['values']) {
  s.notice = { key, values }
  emit(s, 'blocked', at)
  return false
}
export function startTower(s: TowerState) { if (s.phase === 'ready' || s.phase === 'paused') s.phase = 'playing' }
export function pauseTower(s: TowerState) { if (s.phase === 'playing') s.phase = 'paused' }

export function moveTower(s: TowerState, dx: number, dy: number): boolean {
  if (s.phase !== 'playing' || !Number.isInteger(dx) || !Number.isInteger(dy) || Math.abs(dx) + Math.abs(dy) !== 1) return false
  const p = { x: s.position.x + dx, y: s.position.y + dy }, tile = tileAt(s, p)
  if (tile === '#') return blocked(s, 'wall', p)
  const color = DOORS[tile]
  if (color && s.stats.keys[color] < 1) return blocked(s, `${color}Key`, p)
  if (tile === '>' && FLOORS[s.floor]!.boss) {
    const boss = findTile(s.floor, 'X')
    if (!s.removed[keyFor(s.floor, boss)]) return blocked(s, 'bossGate', p)
  }
  const monster = monsterFor(s.floor, tile)
  const battle = monster ? previewBattle(s, monster) : null
  if (battle && !battle.canWin) {
    s.encounter = p
    return blocked(s, battle.sealed ? 'sealed' : battle.hits === null ? 'noAttack' : 'danger', p, battle.damageTaken === null ? undefined : { n: battle.damageTaken })
  }
  remember(s)
  s.encounter = null
  s.position = p
  s.steps++
  s.notice = { key: 'explore' }
  emit(s, 'move', p)
  if (color) {
    s.stats.keys[color]--
    s.removed[keyFor(s.floor, p)] = true
    s.notice = { key: 'door' }
    emit(s, 'door', p)
  }
  if (battle && monster) {
    const damage = battle.damageTaken ?? 0
    s.stats.hp -= damage
    s.damage += damage
    s.stats.gold += monster.reward
    s.stats.score += monster.reward * 10
    s.stats.experience += monster.reward
    let leveledUp = false
    while (s.stats.experience >= s.stats.nextLevelExperience && s.stats.level < 8) {
      s.stats.experience -= s.stats.nextLevelExperience
      s.stats.level++
      s.stats.nextLevelExperience = Math.ceil(s.stats.nextLevelExperience * 1.5)
      s.stats.attack += 4
      s.stats.defense += 3
      s.stats.maxHp += 100
      s.stats.hp += 100
      leveledUp = true
    }
    s.removed[keyFor(s.floor, p)] = true
    s.notice = leveledUp
      ? { key: 'levelUp', values: { n: s.stats.level, exp: monster.reward } }
      : { key: 'battle', values: { n: damage, gold: monster.reward, exp: monster.reward } }
    emit(s, 'fight', p, damage)
    if (s.stats.level === 8) s.stats.experience = Math.min(s.stats.experience, s.stats.nextLevelExperience - 1)
    if (tile === 'X' && s.floor === FLOORS.length - 1) {
      s.phase = 'won'
      s.stats.score += 2000 + s.stats.hp
      s.notice = { key: 'won' }
      emit(s, 'won', p, s.stats.score)
      return true
    }
  }
  collect(s, tile, p)
  if (tile === '>') changeFloor(s, s.floor + 1, '<')
  else if (tile === '<' && s.floor > 0) changeFloor(s, s.floor - 1, '>')
  else if (tile === 'C') s.notice = { key: 'shop' }
  else if (tile === 'n') s.notice = { key: 'mentor' }
  return true
}

function changeFloor(s: TowerState, floor: number, arrival: Tile) {
  if (floor < 0 || floor >= FLOORS.length) return
  s.floor = floor
  s.position = findTile(floor, arrival)
  if (!s.discovered.includes(floor)) s.discovered.push(floor)
  s.notice = { key: 'floor', values: { n: floor + 1 } }
  emit(s, 'floor', s.position)
}
function collect(s: TowerState, tile: Tile, at: Point) {
  const color = KEYS[tile]
  if (!color && !['a', 'd', 'p', '$', 's', 'h', 'o'].includes(tile)) return
  s.removed[keyFor(s.floor, at)] = true
  s.stats.score += 20
  let value = 0
  if (color) { s.stats.keys[color]++; s.notice = { key: 'key' } }
  if (tile === 'a' || tile === 'd') {
    value = GEM_GAIN(s.floor)
    s.stats[tile === 'a' ? 'attack' : 'defense'] += value
    s.stats.gems++
    s.notice = { key: tile === 'a' ? 'attackGem' : 'defenseGem', values: { n: value } }
  }
  if (tile === 'p') { s.stats.potions++; s.notice = { key: 'potion' } }
  if (tile === '$') { value = 25 + s.floor * 5; s.stats.gold += value; s.notice = { key: 'gold', values: { n: value } } }
  if (tile === 's' || tile === 'h') {
    value = EQUIPMENT_GAIN(s.floor)
    s.stats[tile === 's' ? 'attack' : 'defense'] += value
    s.stats[tile === 's' ? 'swords' : 'shields']++
    s.notice = { key: tile === 's' ? 'sword' : 'shield', values: { n: value } }
  }
  if (tile === 'o') { s.stats.score += 300; s.notice = { key: 'rune', values: { n: relicCount(s) } } }
  emit(s, 'loot', at, value)
}

export function usePotion(s: TowerState): boolean {
  if (s.phase !== 'playing' || s.stats.potions < 1 || s.stats.hp === s.stats.maxHp) return false
  remember(s)
  const gain = Math.min(300, s.stats.maxHp - s.stats.hp)
  s.stats.potions--
  s.stats.hp += gain
  s.notice = { key: 'heal', values: { n: gain } }
  emit(s, 'heal', s.position, gain)
  return true
}
export function buyUpgrade(s: TowerState, kind: 'attack' | 'defense' | 'health'): boolean {
  if (s.phase !== 'playing' || tileAt(s) !== 'C' || !['attack', 'defense', 'health'].includes(kind) || s.stats.gold < shopCost(s)) return false
  remember(s)
  s.stats.gold -= shopCost(s)
  s.upgrades++
  if (kind === 'health') { s.stats.maxHp += 200; s.stats.hp = Math.min(s.stats.maxHp, s.stats.hp + 300) }
  else s.stats[kind] += 5
  s.notice = { key: 'upgrade' }
  emit(s, 'shop', s.position)
  return true
}
export function undoTower(s: TowerState): boolean {
  if (!['playing', 'paused'].includes(s.phase)) return false
  const previous = s.history.pop()
  if (!previous) return false
  Object.assign(s, previous, { phase: 'playing', encounter: null, notice: { key: 'undone' } })
  emit(s, 'undo', s.position)
  return true
}
export function resetTower(s: TowerState) { Object.assign(s, createTower()) }

export function saveTower(s: TowerState): string {
  return JSON.stringify({ version: 3, state: snapshot(s), history: s.history.slice(-12) })
}
function validSnapshot(value: unknown): value is TowerSnapshot {
  if (!value || typeof value !== 'object') return false
  const s = value as TowerSnapshot
  const integer = (n: unknown, min = 0, max = 1_000_000) => typeof n === 'number' && Number.isSafeInteger(n) && n >= min && n <= max
  if (typeof s.id !== 'string' || s.id.length > 100 || !s.id.startsWith('tower-') || !['ready', 'playing', 'paused', 'won'].includes(s.phase)) return false
  if (!integer(s.floor, 0, FLOORS.length - 1) || !s.position || !integer(s.position.x, 0, SIZE - 1) || !integer(s.position.y, 0, SIZE - 1) || rawTile(s.floor, s.position) === '#') return false
  if (!s.stats || !s.stats.keys || !['red', 'blue', 'yellow'].every(k => integer(s.stats.keys[k as keyof TowerStats['keys']], 0, 999))) return false
  if (!['hp', 'maxHp', 'attack', 'defense', 'gold', 'score', 'experience', 'level', 'nextLevelExperience', 'potions', 'gems', 'swords', 'shields'].every(k => integer(s.stats[k as Exclude<keyof TowerStats, 'keys'>]))) return false
  if (!integer(s.stats.level, 1, 8) || s.stats.nextLevelExperience < 1 || s.stats.experience >= s.stats.nextLevelExperience) return false
  if (s.stats.maxHp < 1 || s.stats.hp < 1 || s.stats.hp > s.stats.maxHp || s.stats.attack < 1 || !integer(s.steps) || !integer(s.upgrades, 0, 999) || !integer(s.damage)) return false
  if (!Array.isArray(s.discovered) || s.discovered.length > FLOORS.length || !s.discovered.includes(s.floor) || !s.discovered.every(f => integer(f, 0, FLOORS.length - 1))) return false
  if (!s.removed || typeof s.removed !== 'object' || Array.isArray(s.removed) || Object.keys(s.removed).length > SIZE * SIZE * FLOORS.length) return false
  for (const [key, removed] of Object.entries(s.removed)) {
    if (!/^\d:\d{1,2}:\d{1,2}$/.test(key) || removed !== true) return false
    const [f, x, y] = key.split(':').map(Number)
    if (!integer(f, 0, FLOORS.length - 1) || !integer(x, 0, SIZE - 1) || !integer(y, 0, SIZE - 1)) return false
    if (!'rbyRBYadp$sho123456X'.includes(rawTile(f!, { x: x!, y: y! }))) return false
  }
  if (s.phase === 'won' && (s.floor !== FLOORS.length - 1 || !s.removed[keyFor(s.floor, findTile(s.floor, 'X'))] || relicCount(s) < 3)) return false
  return true
}
export function loadTower(raw: string | null): TowerState | null {
  if (!raw || raw.length > 700_000) return null
  try {
    const parsed = JSON.parse(raw)
    if (![2, 3].includes(parsed.version)) return null
    const normalize = (value: unknown): unknown => {
      if (!value || typeof value !== 'object') return value
      const entry = value as TowerSnapshot
      if (parsed.version === 2) entry.stats = { ...entry.stats, experience: 0, level: 1, nextLevelExperience: 40 }
      return entry
    }
    const restored = normalize(parsed.state)
    if (!validSnapshot(restored)) return null
    const s = createTower()
    Object.assign(s, snapshot(restored))
    s.phase = s.phase === 'playing' ? 'paused' : s.phase
    s.history = Array.isArray(parsed.history) ? parsed.history.map(normalize).filter((entry: unknown): entry is TowerSnapshot => validSnapshot(entry) && entry.id === s.id && entry.phase === 'playing').slice(-12).map(snapshot) : []
    s.notice = { key: s.phase === 'won' ? 'won' : 'restored' }
    return s
  } catch { return null }
}

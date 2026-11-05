// Browser port of Armour/Magic-Tower-Qt, Copyright (c) 2014 Chong Guo (MIT).
// Source: a32ee738485fa210e3a2b32cf57f506d51446c13. See public/games/token-tower/classic/LICENSE.txt.


export type Point = { x: number; y: number }
export type Role = 'knight' | 'scout'
export type Phase = 'ready' | 'playing' | 'paused' | 'won'
export type KeyColor = 'yellow' | 'blue' | 'red'
export type Upgrade = 'health' | 'attack' | 'defense' | KeyColor
export interface Stats {
  hp: number; maxHp: number; attack: number; defense: number; gold: number
  experience: number; level: number; next: number; book: boolean; sword: boolean; shield: boolean
  keys: Record<KeyColor, number>
}
export interface Snapshot {
  id: string; role: Role; phase: Phase; floor: number; position: Point; facing: number; stats: Stats
  removed: Record<string, true>; discovered: number[]; steps: number; damage: number; seed: number; score: number
}
export interface State extends Snapshot {
  history: Snapshot[]
  notice: { key: string; values?: Record<string, string | number> }
  event: { id: number; kind: 'move' | 'fight' | 'loot' | 'floor' | 'blocked' | 'shop' | 'curse' | 'undo' | 'won'; at: Point; amount: number }
}
export const DOORS: Partial<Record<number, KeyColor>> = { 3: 'yellow', 4: 'blue', 5: 'red' }
export const KEYS: Partial<Record<number, KeyColor>> = { 6: 'yellow', 7: 'blue', 8: 'red' }
export const SHOP: { kind: Upgrade; tile: number; cost: number; gain: number }[] = [
  { kind: 'health', tile: 30, cost: 50, gain: 30 },
  { kind: 'attack', tile: 32, cost: 50, gain: 2 },
  { kind: 'defense', tile: 34, cost: 50, gain: 1 },
  { kind: 'yellow', tile: 6, cost: 40, gain: 1 },
  { kind: 'blue', tile: 7, cost: 120, gain: 1 },
  { kind: 'red', tile: 8, cost: 300, gain: 1 },
]
export interface Battle {
  damage: number; rounds: number; canWin: boolean; criticals: number; dodges: number; seed: number
}
export interface BattleStrike {
  round: number; attacker: 'hero' | 'monster'; damage: number; critical: boolean; missed: boolean
  heroHp: number; monsterHp: number
}

export interface CampaignMonster {
  id: number; hp: number; attack: number; defense: number; gold: number; experience: number; miss: number; criticalThreshold: number
  sprite?: number; name?: string; ability?: 'normal' | 'magic' | 'first' | 'double'
}
export interface CampaignDefinition {
  sourceRevision: string; floors: number[][]; monsters: CampaignMonster[]
  start?: Point; long?: boolean
  floorRules?: { zone: number; guardian?: Point }[]
}

export function createCampaignEngine(campaign: CampaignDefinition) {
const SIZE = 14, FLOORS = campaign.floors, MONSTERS = campaign.monsters
const long = Boolean(campaign.long)
const revision = campaign.sourceRevision
const same = (a: Point, b: Point) => a.x === b.x && a.y === b.y
const keyFor = (floor: number, p: Point) => `${floor}:${p.x}:${p.y}`
const monsterFor = (tile: number) => MONSTERS.find(m => m.id === tile)
const wall = (tile: number) => [2, 50, 52, 60, 61, 62, 63, 64].includes(tile)
const curse = (tile: number) => tile >= 24 && tile <= 26
const neighbors = (p: Point) => [{ x: p.x, y: p.y - 1 }, { x: p.x - 1, y: p.y }, { x: p.x + 1, y: p.y }, { x: p.x, y: p.y + 1 }]
const inside = (p: Point) => p.x >= 0 && p.x < SIZE && p.y >= 0 && p.y < SIZE
function rawTile(floor: number, p: Point): number {
  if (!inside(p)) return 2
  // The upstream ninth floor omits the return stairs at the arrival point.
  if (!long && floor === 8 && p.x === 12 && p.y === 12) return 90
  return FLOORS[floor]?.[p.y * SIZE + p.x] ?? 2
}
const tileAt = (s: Snapshot, p = s.position) => s.removed[keyFor(s.floor, p)] ? 0 : rawTile(s.floor, p)
const pointsOf = (floor: number, tile: number): Point[] => Array.from({ length: SIZE * SIZE }, (_, i) => ({ x: i % SIZE, y: Math.floor(i / SIZE) })).filter(p => rawTile(floor, p) === tile)
const battleCount = (s: Snapshot) => Object.keys(s.removed).filter(key => {
  const [f, x, y] = key.split(':').map(Number)
  return Boolean(monsterFor(rawTile(f!, { x: x!, y: y! })))
}).length
const nearShop = (s: Snapshot) => [s.position, ...neighbors(s.position)].some(p => tileAt(s, p) === 51)
const heroSprite = (s: Snapshot) => (s.role === 'knight' ? 110 : 100) + s.facing
const stars = (s: Snapshot) => s.phase !== 'won' ? 0 : 1 + Number(s.damage < (long ? 1000 * FLOORS.length : 1800)) + Number(battleCount(s) >= (long ? Math.floor(2.8 * FLOORS.length) : 60))

function createClassic(role: Role = 'knight', seed = Math.floor(Math.random() * 0xffff_ffff) || 1): State {
  return {
    id: `tower-classic-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`, role,
    phase: 'ready', floor: 0, position: { ...(campaign.start ?? { x: 6, y: 12 }) }, facing: 3,
    stats: { hp: long ? (role === 'knight' ? 600 : 680) : role === 'knight' ? 150 : 200, maxHp: long ? (role === 'knight' ? 600 : 680) : role === 'knight' ? 150 : 200, attack: long ? (role === 'knight' ? 18 : 17) : role === 'knight' ? 14 : 13, defense: long ? 12 : 10, gold: 0, experience: 0, level: 1, next: 5, book: false, sword: false, shield: false, keys: { yellow: 0, blue: 0, red: 0 } },
    removed: {}, discovered: [0], steps: 0, damage: 0, seed: seed >>> 0 || 1, score: 0,
    history: [], notice: { key: 'welcome' }, event: { id: 0, kind: 'move', at: { ...(campaign.start ?? { x: 6, y: 12 }) }, amount: 0 },
  }
}

function previewBattle(s: Snapshot, tile: number, strikes?: BattleStrike[]): Battle | null {
  const m = monsterFor(tile)
  if (!m) return null
  // Pure simulation: preview never consumes RNG, so undo cannot reroll a battle.
  let seed = s.seed, hp = m.hp, damage = 0, rounds = 0, criticals = 0, dodges = 0
  const roll = () => { seed = (Math.imul(seed, 1664525) + 1013904223) >>> 0; return seed % 99 + 1 }
  const enemyStrike = (round: number) => {
    const response = roll(), enemyBase = Math.max(1, m.attack - (m.ability === 'magic' ? 0 : s.stats.defense))
    const received = response > (s.role === 'knight' ? 8 : 18) ? enemyBase + (response >= m.criticalThreshold ? Math.floor(enemyBase / 2) : 0) : 0
    if (received > 0) damage += received
    else dodges++
    strikes?.push({ round, attacker: 'monster', damage: received, critical: response >= m.criticalThreshold && received > 0, missed: received === 0, heroHp: Math.max(0, s.stats.hp - damage), monsterHp: Math.max(0, hp) })
  }
  if (m.ability === 'first') enemyStrike(0)
  while (hp > 0 && rounds < 6000) {
    rounds++
    const hit = roll(), base = Math.max(1, s.stats.attack - m.defense)
    const crit = hit >= (s.role === 'knight' ? 93 : 99)
    const dealt = hit > m.miss ? base + (crit ? Math.floor(base / 2) : 0) : 0
    if (hit > m.miss) {
      hp -= dealt
      criticals += Number(crit)
    }
    strikes?.push({ round: rounds, attacker: 'hero', damage: dealt, critical: crit && dealt > 0, missed: dealt === 0, heroHp: Math.max(0, s.stats.hp - damage), monsterHp: Math.max(0, hp) })
    if (hp <= 0) break
    enemyStrike(rounds)
    if (m.ability === 'double') enemyStrike(rounds)
  }
  return { damage, rounds, canWin: hp <= 0 && damage < s.stats.hp, criticals, dodges, seed }
}
function snapshot(s: Snapshot): Snapshot {
  const { id, role, phase, floor, position, facing, stats, removed, discovered, steps, damage, seed, score } = s
  return JSON.parse(JSON.stringify({ id, role, phase, floor, position, facing, stats, removed, discovered, steps, damage, seed, score })) as Snapshot
}
function remember(s: State) { s.history.push(snapshot(s)); if (s.history.length > 60) s.history.shift() }
function emit(s: State, kind: State['event']['kind'], at = s.position, amount = 0) { s.event = { id: s.event.id + 1, kind, at: { ...at }, amount } }
function blocked(s: State, key: string, values?: State['notice']['values']) { s.notice = { key, values }; emit(s, 'blocked'); return false }
function startClassic(s: State) { if (s.phase === 'ready' || s.phase === 'paused') s.phase = 'playing' }
function pauseClassic(s: State) { if (s.phase === 'playing') s.phase = 'paused' }

function moveClassic(s: State, dx: number, dy: number, confirmCurse = false): boolean {
  if (s.phase !== 'playing' || !Number.isInteger(dx) || !Number.isInteger(dy) || Math.abs(dx) + Math.abs(dy) !== 1) return false
  const p = { x: s.position.x + dx, y: s.position.y + dy }, tile = tileAt(s, p)
  if (wall(tile)) return blocked(s, 'wall')
  const color = DOORS[tile]
  if (color && s.stats.keys[color] < 1) return blocked(s, 'needKey', { color })
  if (tile === 51) { s.notice = { key: 'shop' }; emit(s, 'shop'); return false }
  if (tile === 10 || tile === 27) { s.notice = { key: tile === 10 ? 'mentor' : 'witch' }; emit(s, 'blocked'); return false }
  if (curse(tile) && !confirmCurse) return blocked(s, 'curseWarning')
  const battle = previewBattle(s, tile), monster = monsterFor(tile)
  if (battle && !battle.canWin) return blocked(s, 'danger', { n: battle.damage })
  const guardian = campaign.floorRules?.[s.floor]?.guardian
  if ((tile === 80 || tile === 99) && guardian && !s.removed[keyFor(s.floor, guardian)]) return blocked(s, 'guardian')
  if (tile === 99 && long && !bossesDefeated(s)) return blocked(s, 'seals')
  remember(s)
  s.position = p; s.facing = dx === 1 ? 4 : dx === -1 ? 2 : dy === 1 ? 3 : 1
  s.steps++; s.notice = { key: 'explore' }; emit(s, 'move')
  if (tile === 9) { s.removed[keyFor(s.floor, p)] = true; s.notice = { key: 'secret' }; emit(s, 'loot') }
  if (color) { s.stats.keys[color]--; s.removed[keyFor(s.floor, p)] = true; s.notice = { key: 'door' }; emit(s, 'loot') }
  if (battle && monster) {
    s.stats.hp -= battle.damage; s.damage += battle.damage; s.seed = battle.seed
    s.stats.gold += monster.gold; s.stats.experience += monster.experience; s.score += monster.gold * 10 + 30
    const before = s.stats.level
    while (s.stats.experience >= s.stats.next) {
      s.stats.experience -= s.stats.next; s.stats.level++; s.stats.next = Math.floor(s.stats.next * 1.5) + 1
      s.stats.hp += 10; s.stats.attack++; s.stats.defense++
    }
    s.stats.maxHp = Math.max(s.stats.hp, s.stats.maxHp)
    s.removed[keyFor(s.floor, p)] = true
    s.notice = { key: before !== s.stats.level ? 'levelUp' : 'battle', values: { n: battle.damage, gold: monster.gold, exp: monster.experience, level: s.stats.level, crit: battle.criticals, dodge: battle.dodges } }
    emit(s, 'fight', p, battle.damage)
  }
  const item = KEYS[tile]
  if (item || [28, 29, 30, 31, 32, 33, 34, 35].includes(tile)) {
    s.removed[keyFor(s.floor, p)] = true; s.score += 20
    if (item) { s.stats.keys[item]++; s.notice = { key: 'key', values: { color: item } } }
    else if (tile === 28) { s.stats.gold += long ? 25 * zoneOf(s) : 15; s.notice = { key: 'gold', values: { n: long ? 25 * zoneOf(s) : 15 } } }
    else if (tile === 29 || tile === 30) { const n = long ? (tile === 29 ? 45 : 105) * zoneOf(s) : tile === 29 ? 18 : 30; s.stats.hp += n; s.stats.maxHp = Math.max(s.stats.maxHp, s.stats.hp); s.notice = { key: 'heal', values: { n } } }
    else if (tile === 31 || tile === 32) { const n = long ? (tile === 31 ? 10 : 3) * zoneOf(s) : tile === 31 ? 10 : 3; s.stats.attack += n; s.stats.sword ||= tile === 31; s.notice = { key: 'attack', values: { n } } }
    else if (tile === 33 || tile === 34) { const n = long ? (tile === 33 ? 8 : 2) * zoneOf(s) : tile === 33 ? 8 : 2; s.stats.defense += n; s.stats.shield ||= tile === 33; s.notice = { key: 'defense', values: { n } } }
    else { s.stats.book = true; s.notice = { key: 'book' } }
    emit(s, 'loot')
  }
  if (curse(tile)) {
    const stat = tile === 24 ? 'hp' : tile === 25 ? 'defense' : 'attack'
    s.stats[stat] = Math.max(1, Math.floor(s.stats[stat] / 3))
    s.removed[keyFor(s.floor, p)] = true
    s.notice = { key: `curse${tile}` }; emit(s, 'curse')
  }
  if (tile === 80 || tile === 90) {
    const targetFloor = s.floor + (tile === 80 ? 1 : -1)
    if (targetFloor >= 0 && targetFloor < FLOORS.length) {
      // Paired stairs occupy the same coordinate in the original maps.
      const matches = pointsOf(targetFloor, tile === 80 ? 90 : 80)
      const landing = matches.find(q => same(q, p)) ?? matches[0]
      if (landing) {
        s.floor = targetFloor; s.position = { ...landing }
        if (!s.discovered.includes(s.floor)) s.discovered.push(s.floor)
        s.notice = { key: 'floor', values: { n: s.floor + 1 } }; emit(s, 'floor')
      }
    }
  }
  if (tile === 99) {
    s.phase = 'won'; s.score += 2000 + s.stats.hp
    s.notice = { key: 'won' }; emit(s, 'won', p, s.score)
  }
  return true
}
function buyClassic(s: State, kind: Upgrade): boolean {
  const offer = shopOffers(s).find(o => o.kind === kind)
  if (s.phase !== 'playing' || !nearShop(s) || !offer || s.stats.gold < offer.cost) return false
  remember(s); s.stats.gold -= offer.cost
  if (kind === 'health') { s.stats.hp += offer.gain; s.stats.maxHp = Math.max(s.stats.maxHp, s.stats.hp) }
  else if (kind === 'attack' || kind === 'defense') s.stats[kind] += offer.gain
  else s.stats.keys[kind]++
  s.notice = { key: 'purchase' }; emit(s, 'shop'); return true
}
function undoClassic(s: State): boolean {
  if (!['playing', 'paused'].includes(s.phase)) return false
  const previous = s.history.pop()
  if (!previous) return false
  Object.assign(s, previous, { phase: 'playing', notice: { key: 'undone' } }); emit(s, 'undo'); return true
}
function saveClassic(s: State): string { return JSON.stringify({ version: 1, campaign: revision, state: snapshot(s), history: s.history.slice(-20) }) }
function valid(value: unknown): value is Snapshot {
  if (!value || typeof value !== 'object') return false
  const s = value as Snapshot
  const int = (v: unknown, min = 0, max = 1_000_000) => typeof v === 'number' && Number.isSafeInteger(v) && v >= min && v <= max
  if (typeof s.id !== 'string' || !s.id.startsWith('tower-classic-') || s.id.length > 100 || !['knight', 'scout'].includes(s.role) || !['ready', 'playing', 'paused', 'won'].includes(s.phase)) return false
  if (!int(s.floor, 0, FLOORS.length - 1) || !s.position || !int(s.position.x, 0, SIZE - 1) || !int(s.position.y, 0, SIZE - 1) || wall(rawTile(s.floor, s.position)) || !int(s.facing, 1, 4)) return false
  if (!s.stats || !s.stats.keys || !(['yellow', 'blue', 'red'] as const).every(c => int(s.stats.keys[c], 0, 999))) return false
  if (!(['hp', 'maxHp', 'attack', 'defense', 'gold', 'experience', 'level', 'next'] as const).every(k => int(s.stats[k]))) return false
  if (s.stats.hp < 1 || s.stats.hp > s.stats.maxHp || s.stats.attack < 1 || s.stats.level < 1 || s.stats.next < 1 || s.stats.experience >= s.stats.next || !['book', 'sword', 'shield'].every(k => typeof s.stats[k as keyof Stats] === 'boolean')) return false
  if (!int(s.steps) || !int(s.damage) || !int(s.score) || !int(s.seed, 0, 0xffff_ffff)) return false
  if (!Array.isArray(s.discovered) || s.discovered.length > FLOORS.length || !s.discovered.includes(s.floor) || !s.discovered.every(f => int(f, 0, FLOORS.length - 1))) return false
  if (!s.removed || typeof s.removed !== 'object' || Array.isArray(s.removed) || Object.keys(s.removed).length > SIZE * SIZE * FLOORS.length) return false
  for (const [key, removed] of Object.entries(s.removed)) {
    if (!/^\d{1,2}:\d{1,2}:\d{1,2}$/.test(key) || removed !== true) return false
    const [f, x, y] = key.split(':').map(Number)
    if (!int(f, 0, FLOORS.length - 1) || !int(x, 0, 13) || !int(y, 0, 13)) return false
    const tile = rawTile(f!, { x: x!, y: y! })
    if (!DOORS[tile] && !KEYS[tile] && !monsterFor(tile) && !curse(tile) && tile !== 9 && !(tile >= 28 && tile <= 35)) return false
  }
  if (s.phase === 'won' && (s.floor !== FLOORS.length - 1 || rawTile(s.floor, s.position) !== 99 || long && !bossesDefeated(s))) return false
  return true
}
function loadClassic(raw: string | null): State | null {
  if (!raw || raw.length > (long ? 3_000_000 : 500_000)) return null
  try {
    const parsed = JSON.parse(raw)
    if (parsed?.version !== 1 || parsed.campaign !== revision || !valid(parsed.state)) return null
    const s = createClassic()
    Object.assign(s, snapshot(parsed.state))
    if (s.phase === 'playing') s.phase = 'paused'
    s.history = Array.isArray(parsed.history) ? parsed.history.filter((v: unknown): v is Snapshot => valid(v) && v.id === s.id && v.phase === 'playing').slice(-20).map(snapshot) : []
    s.notice = { key: s.phase === 'won' ? 'won' : 'restored' }
    return s
  } catch { return null }
}

function zoneOf(s: Snapshot) { return (campaign.floorRules?.[s.floor]?.zone ?? 0) + 1 }
function shopOffers(s: Snapshot) {
  if (!long) return SHOP
  const z = zoneOf(s)
  return SHOP.map(offer => ({ ...offer, cost: offer.kind === 'yellow' || offer.kind === 'blue' || offer.kind === 'red' ? offer.cost : 50 + (z - 1) * 25, gain: offer.kind === 'health' ? 100 * z : offer.kind === 'attack' ? 4 * z : offer.kind === 'defense' ? 3 * z : 1 }))
}
function bossesDefeated(s: Snapshot) { return (campaign.floorRules ?? []).every((rule, floor) => !rule.guardian || Boolean(s.removed[keyFor(floor, rule.guardian)])) }
const spriteTile = (tile: number) => monsterFor(tile)?.sprite ?? tile
return { SIZE, FLOORS, MONSTERS, DOORS, KEYS, SHOP, same, keyFor, monsterFor, wall, curse, neighbors, inside, rawTile, tileAt, pointsOf, battleCount, nearShop, heroSprite, stars, createClassic, previewBattle, startClassic, pauseClassic, moveClassic, buyClassic, undoClassic, saveClassic, loadClassic, shopOffers, spriteTile, zoneOf, bossesDefeated }
}

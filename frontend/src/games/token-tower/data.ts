export const SIZE = 11
export type KeyColor = 'red' | 'blue' | 'yellow'
export type Tile = '#' | '.' | 'S' | '<' | '>' | 'r' | 'b' | 'y' | 'R' | 'B' | 'Y' | 'a' | 'd' | 'p' | '$' | 's' | 'h' | 'o' | 'C' | 'n' | '1' | '2' | '3' | '4' | '5' | '6' | 'X'
export type MonsterId = 'slime' | 'bat' | 'skeleton' | 'guard' | 'mage' | 'beast' | 'warden' | 'demon'
export type Ability = 'normal' | 'double' | 'first' | 'magic' | 'seal'
export interface Monster {
  id: MonsterId
  hp: number
  attack: number
  defense: number
  reward: number
  ability: Ability
}
export interface TowerFloor { id: string; map: string[]; boss?: MonsterId; theme: 'keep' | 'forge' | 'abyss' }

// Authored, deterministic rooms. Coordinates, keys and monsters survive revisits.
export const FLOORS: TowerFloor[] = [
  { id: 'gate', theme: 'keep', map: [
    '###########',
    '#p1.a.Y..>#',
    '#.#.###.#.#',
    '#y#...#2#b#',
    '#.#d#d#.#.#',
    '#..1#..p..#',
    '#.###.###.#',
    '#d..B.3.a.#',
    '#.#.#.#.#.#',
    '#Sn.p.yC.$#',
    '###########',
  ] },
  { id: 'armory', theme: 'keep', map: [
    '###########',
    '#s3.Y.1..>#',
    '###.#.#.#.#',
    '#d1a#p#d#y#',
    '#.#.#.#.#.#',
    '#b..2..B..#',
    '#.###.###.#',
    '#p..#..a..#',
    '#.#Y#.###.#',
    '#<.$y...C.#',
    '###########',
  ] },
  { id: 'archive', theme: 'keep', map: [
    '###########',
    '#o2.b.Y..>#',
    '#.#.###.#.#',
    '#d#p1.#3#a#',
    '#.#.#.#.#.#',
    '#y..#a.Y..#',
    '###.#.###.#',
    '#p1.B...d.#',
    '#.#.#.#.#.#',
    '#<.$rypC..#',
    '###########',
  ] },
  { id: 'forge', theme: 'forge', map: [
    '###########',
    '#h3.Y...4>#',
    '###.#.###.#',
    '#a2.#p..#b#',
    '#.#.#.#.#.#',
    '#d..1..B..#',
    '#.###.###.#',
    '#p.Y..a.2d#',
    '#.#.#.###.#',
    '#<.yrypC.$#',
    '###########',
  ] },
  { id: 'warden', theme: 'forge', boss: 'warden', map: [
    '###########',
    '#r..R.X..>#',
    '#.#.###.#.#',
    '#a#p4.#d#b#',
    '#.#.#.#.#.#',
    '#y..#a.Y..#',
    '###.#.###.#',
    '#d2.B..3.p#',
    '#.#.#.#.#.#',
    '#<.$y.pC..#',
    '###########',
  ] },
  { id: 'catacombs', theme: 'forge', map: [
    '###########',
    '#s4.Y.a.5>#',
    '###.#.#.#.#',
    '#o2d#p#b#y#',
    '#.#.#.#.#.#',
    '#a..3..B..#',
    '#.###.###.#',
    '#p..#..d..#',
    '#.#Y#.###.#',
    '#<.$r.pC.y#',
    '###########',
  ] },
  { id: 'rift', theme: 'forge', map: [
    '###########',
    '#a3.b.Y..>#',
    '#.#.###.#.#',
    '#d#p5.#4#a#',
    '#.#.#.#.#.#',
    '#y..#d.Y..#',
    '###.#.###.#',
    '#p2.B..3.p#',
    '#.#.#.#.#.#',
    '#<.$rypC..#',
    '###########',
  ] },
  { id: 'observatory', theme: 'abyss', map: [
    '###########',
    '#h4.Y.a.5>#',
    '###.#.###.#',
    '#o3.#p..#b#',
    '#.#.#.#.#.#',
    '#d..2..B..#',
    '#.###.###.#',
    '#p.Y..a.3d#',
    '#.#.#.###.#',
    '#<.yrypC.$#',
    '###########',
  ] },
  { id: 'vault', theme: 'abyss', map: [
    '###########',
    '#a4.R.d.6>#',
    '#.#.###.#.#',
    '#r#p5.#4#b#',
    '#.#.#.#.#.#',
    '#y..#a.Y..#',
    '###.#.###.#',
    '#d3.B..5.p#',
    '#.#.#.#.#.#',
    '#<.$y.pC..#',
    '###########',
  ] },
  { id: 'core', theme: 'abyss', boss: 'demon', map: [
    '###########',
    '#d..R.X...#',
    '#.#.###.#.#',
    '#r#p6.#a#b#',
    '#.#.#.#.#.#',
    '#y..#d.Y..#',
    '###.#.###.#',
    '#a3.B..4.p#',
    '#.#.#.#.#.#',
    '#<.$y.pC..#',
    '###########',
  ] },
]

const baseMonsters: Monster[] = [
  { id: 'slime', hp: 50, attack: 20, defense: 5, reward: 8, ability: 'normal' },
  { id: 'bat', hp: 70, attack: 26, defense: 10, reward: 12, ability: 'double' },
  { id: 'skeleton', hp: 120, attack: 36, defense: 17, reward: 18, ability: 'normal' },
  { id: 'guard', hp: 175, attack: 45, defense: 22, reward: 26, ability: 'first' },
  { id: 'mage', hp: 110, attack: 24, defense: 15, reward: 30, ability: 'magic' },
  { id: 'beast', hp: 270, attack: 64, defense: 35, reward: 40, ability: 'normal' },
]

export function monsterFor(floor: number, tile: Tile): Monster | null {
  if (tile === 'X') return floor === 9
    ? { id: 'demon', hp: 1500, attack: 225, defense: 100, reward: 500, ability: 'seal' }
    : { id: 'warden', hp: 500, attack: 100, defense: 42, reward: 150, ability: 'first' }
  const base = baseMonsters[Number(tile) - 1]
  return base ? { ...base, hp: base.hp + floor * 45, attack: base.attack + floor * (base.ability === 'magic' ? 4 : 12), defense: base.defense + floor * 7, reward: base.reward + floor * 3 } : null
}

export const GEM_GAIN = (floor: number) => 3 + Math.floor(floor / 3)
export const EQUIPMENT_GAIN = (floor: number) => floor < 5 ? 12 : 24
export const DOORS: Partial<Record<Tile, KeyColor>> = { R: 'red', B: 'blue', Y: 'yellow' }
export const KEYS: Partial<Record<Tile, KeyColor>> = { r: 'red', b: 'blue', y: 'yellow' }
export const SPRITES: Partial<Record<Tile, string>> = {
  S: 'stairs-down', '<': 'stairs-down', '>': 'stairs-up',
  r: 'key-red', b: 'key-blue', y: 'key-yellow', R: 'door-red', B: 'door-blue', Y: 'door-yellow',
  a: 'gem-red', d: 'gem-blue', p: 'potion', '$': 'chest', s: 'sword', h: 'shield', o: 'rune', C: 'merchant', n: 'merchant',
}

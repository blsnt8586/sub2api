/** Local bitmap assets. Player evolution and reef biodiversity are separate. */
export const originalSpecies = [
  'minnow',
  'bluegill',
  'perch',
  'koi',
  'tuna',
  'moonfish',
] as const
export const reefSpecies = [
  'clownfish',
  'butterflyfish',
  'sardine',
  'pufferfish',
  'lionfish',
  'barracuda',
  'hammerhead',
  'anglerfish',
] as const
export const deepSpecies = [
  'lanternfish',
  'hatchetfish',
  'viperfish',
  'gulper-eel',
  'oarfish',
  'manta',
  'giant-squid',
  'whale-shark',
  'blue-whale',
] as const
export type Species =
  | (typeof originalSpecies)[number]
  | (typeof reefSpecies)[number]
  | (typeof deepSpecies)[number]
export type FishSprite = Species | 'pufferfish-inflated'
export const fishSprites: FishSprite[] = [
  ...originalSpecies,
  ...reefSpecies,
  ...deepSpecies,
  'pufferfish-inflated',
]
export const spritePath = (species: FishSprite) =>
  (originalSpecies as readonly string[]).includes(species)
    ? `/games/token-fishing/${species}-sprite.webp`
    : `/games/token-ocean/${species}.webp`

export const fieldGuide = [
  ...originalSpecies,
  ...reefSpecies,
  ...deepSpecies,
] as const

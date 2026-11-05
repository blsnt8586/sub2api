/** Browser-only practice checkpoints. These values never authorize credit or rewards. */
export interface ArcadeCheckpoint {
  version: 1
  id: string
  seed: number
  level: number
  score: number
  cleared: boolean
  lives: number
  equipment: { winch: number; clock: number; assay: number }
}
const key = (game: 'miner' | 'ocean', user: number) => `token-park-${game}-campaign-v1:${user}`
export function readCheckpoint(game: 'miner' | 'ocean', user: number): ArcadeCheckpoint | null {
  try {
    const p = JSON.parse(localStorage.getItem(key(game, user)) ?? 'null') as ArcadeCheckpoint | null
    if (!p || p.version !== 1 || typeof p.id !== 'string' || p.id.length > 100 || !p.id
      || !Number.isSafeInteger(p.seed) || p.seed < 0
      || !Number.isInteger(p.level) || p.level < 0 || p.level >= (game === 'miner' ? 100 : 60)
      || !Number.isSafeInteger(p.score) || p.score < 0 || p.score > 1e9
      || typeof p.cleared !== 'boolean' || !Number.isInteger(p.lives) || p.lives < 1 || p.lives > 3
      || !p.equipment || !['winch', 'clock', 'assay'].every((k) => {
        const n = p.equipment[k as keyof typeof p.equipment]
        return Number.isInteger(n) && n >= 0 && n <= 3
      })) return null
    return p
  } catch { return null }
}
export function writeCheckpoint(game: 'miner' | 'ocean', user: number, value: ArcadeCheckpoint) {
  try { localStorage.setItem(key(game, user), JSON.stringify(value)); return true } catch { return false }
}
export function clearCheckpoint(game: 'miner' | 'ocean', user: number) {
  try { localStorage.removeItem(key(game, user)) } catch { /* Storage can be unavailable. */ }
}

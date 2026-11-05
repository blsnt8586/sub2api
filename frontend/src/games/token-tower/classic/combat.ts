import { previewBattle, type BattleStrike, type Snapshot } from './engine'

export interface CombatFrame extends BattleStrike { hits: number }
// Long fights play in at most seven groups plus the final blow and optional first strike. The two beats retain
// every point of damage and the final HP, without a timer for every simulated hit.
export function combatFrames(state: Snapshot, tile: number, simulate = previewBattle): CombatFrame[] {
  const strikes: BattleStrike[] = []
  const result = simulate(state, tile, strikes)
  if (!result?.canWin) return []
  const size = Math.max(1, Math.ceil(result.rounds / 7)), frames: CombatFrame[] = []
  let heroHp = state.stats.hp
  for (const strike of strikes.filter(s => s.round === 0)) {
    frames.push({ ...strike, hits: 1 })
    heroHp = strike.heroHp
  }
  for (let round = 1; round < result.rounds; round += size) {
    const group = strikes.filter(s => s.round >= round && s.round < Math.min(round + size, result.rounds))
    for (const attacker of ['hero', 'monster'] as const) {
      const hits = group.filter(s => s.attacker === attacker)
      const last = hits[hits.length - 1]
      if (!last) continue
      const damage = hits.reduce((n, s) => n + s.damage, 0)
      frames.push({ ...last, heroHp: attacker === 'hero' ? heroHp : last.heroHp, damage, critical: hits.some(s => s.critical), missed: damage === 0, hits: hits.length })
      if (attacker === 'monster') heroHp = last.heroHp
    }
  }
  const final = strikes[strikes.length - 1]!
  frames.push({ ...final, hits: 1 })
  return frames
}

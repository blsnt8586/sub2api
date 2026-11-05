import type { OceanState } from './engine'
import type { OceanZone } from './exploration'
export const ROUND_COUNT = 60
export const ROUNDS_PER_CHAPTER = 6
export type HuntGoal = 'eat' | 'dash' | 'golden'
export interface OceanRound {
  chapter: number
  zone: OceanZone
  goal: HuntGoal
  target: number
  seconds: number
  boss: boolean
}
const goals: HuntGoal[] = ['eat', 'dash', 'eat', 'golden', 'dash', 'eat']
export const ROUNDS: OceanRound[] = Array.from({ length: ROUND_COUNT }, (_, index) => {
  const chapter = Math.floor(index / ROUNDS_PER_CHAPTER), step = index % ROUNDS_PER_CHAPTER
  const goal = goals[(step + chapter) % goals.length]!
  return {
    chapter, zone: (['reef', 'twilight', 'abyss'] as const)[Math.floor(step / 2)]!,
    goal, target: goal === 'eat' ? 24 + chapter * 2 + step * 2 : goal === 'dash' ? 14 + chapter + step : 5 + Math.floor(chapter / 2),
    seconds: Math.max(120, 190 - chapter * 5) + (step === 5 ? 60 : 0),
    boss: step === ROUNDS_PER_CHAPTER - 1,
  }
})
export function roundFor(s: OceanState) { return ROUNDS[s.round]! }
/** A stage clear is distinct from finishing the entire campaign. */
export function completeRound(s: OceanState) {
  if (s.phase !== 'playing') return
  s.score += 800 + roundFor(s).chapter * 100
  s.phase = s.round === ROUND_COUNT - 1 ? 'won' : 'cleared'
  s.notice = s.phase === 'won' ? 'won' : 'clear'
  s.event++
}

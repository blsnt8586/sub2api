export const SHAFT_COUNT = 100
export const SHAFTS_PER_CHAPTER = 10
export type MineLayout = 'scattered' | 'veins' | 'deep' | 'walls' | 'crystals'
export interface Shaft {
  target: number
  seconds: number
  swingSpeed: number
  chapter: number
  layout: MineLayout
  gems: number
}
const layouts: MineLayout[] = ['scattered', 'veins', 'deep', 'walls', 'crystals']
/** Each chapter introduces harder routes; targets remain bounded by that field's value. */
export const LEVELS: Shaft[] = Array.from({ length: SHAFT_COUNT }, (_, level) => {
  const chapter = Math.floor(level / SHAFTS_PER_CHAPTER)
  const step = level % SHAFTS_PER_CHAPTER
  return {
    target: Math.round((650 + Math.min(level, 7) * 290 + chapter * 65 + Math.max(0, step - 4) * 45) / 10) * 10,
    seconds: level === 0 ? 60 : Math.max(65, 86 - chapter + (step === 9 ? 12 : 0)),
    swingSpeed: Math.min(1.48, 0.95 + chapter * 0.035 + step * 0.025),
    chapter,
    layout: layouts[(step + chapter * 2) % layouts.length]!,
    gems: level === 0 ? 0 : step === 9 ? 3 : step % 3 === 2 ? 2 : 0,
  }
})
export const earnsUpgrade = (level: number) => level < SHAFT_COUNT - 1 && (level + 1) % SHAFTS_PER_CHAPTER === 0

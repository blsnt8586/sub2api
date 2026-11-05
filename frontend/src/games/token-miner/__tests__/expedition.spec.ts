import { describe, expect, it } from 'vitest'
import { advance, createGame, discard, launch, nextShaft, earnsUpgrade, LEVELS, WORLD, type GameState, type Ore, type Upgrade } from '../engine'

/** Aim at visible, unobstructed ore, using only normal shots and dynamite. */
function chooseOre(s: GameState): Ore | undefined {
  const candidates = s.ores.filter((o) => !o.collected)
  const visible = candidates.filter((o) => {
    const a = Math.atan2(o.x - WORLD.originX, o.y - WORLD.originY)
    const distance = Math.hypot(o.x - WORLD.originX, o.y - WORLD.originY)
    return !candidates.some((block) => {
      if (block === o) return false
      const dx = block.x - WORLD.originX, dy = block.y - WORLD.originY
      const along = dx * Math.sin(a) + dy * Math.cos(a)
      const across = Math.abs(dx * Math.cos(a) - dy * Math.sin(a))
      return along > 0 && along < distance - o.radius && across < block.radius + 8
    })
  })
  return visible.sort((a, b) => ((s.gems < LEVELS[s.level]!.gems && b.kind === 'gem') ? 10000 : b.value / b.weight) - ((s.gems < LEVELS[s.level]!.gems && a.kind === 'gem') ? 10000 : a.value / a.weight))[0]
}
describe('Full mining expedition balance', () => {
  it('can complete one hundred generated shafts with legal actions and earned gear', () => {
    for (const seed of [1, 7, 42, 99, 2026]) {
      let s = createGame(seed)
      for (let level = 0; level < LEVELS.length; level++) {
        s.status = 'playing'
        let target: Ore | undefined
        for (let frame = 0; frame < 18000 && s.status === 'playing'; frame++) {
          if (s.hook === 'swinging') {
            target ??= chooseOre(s)
            if (target && Math.abs(s.angle - Math.atan2(target.x - WORLD.originX, target.y - WORLD.originY)) < 0.013) {
              launch(s)
              target = undefined
            }
          }
          if (s.grabbed?.kind === 'rock' && s.bombs > 0) discard(s)
          advance(s, 1 / 120)
        }
        expect(s.status, `seed ${seed}, shaft ${level + 1}, score ${s.score}`).toBe(level === LEVELS.length - 1 ? 'complete' : 'won')
        if (level < LEVELS.length - 1) {
          const upgrade = earnsUpgrade(level) ? (['winch', 'assay', 'clock'] as Upgrade[]).find((k) => s.equipment[k] < 3)! : null
          s = nextShaft(s, seed + (level + 1) * 7919, upgrade)!
        }
      }
    }
  })
})

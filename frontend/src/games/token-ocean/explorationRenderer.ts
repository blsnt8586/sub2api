import type { OceanState } from './engine'

export interface ExplorationLabels {
  relic?: string
  migration?: string
  gentle?: string
}
export function drawExploration(
  ctx: CanvasRenderingContext2D,
  s: OceanState,
  crystal: HTMLImageElement,
  drawGiant: (
    species: 'manta' | 'whale-shark' | 'blue-whale',
    x: number,
    y: number,
    radius: number,
    facing: number,
  ) => void,
  reduced: boolean,
) {
  if (s.phase === 'ready' || s.challenge?.finale) return
  const t = reduced ? 0 : s.time
  for (const r of s.exploration.relics) {
    if (r.collected) continue
    const y = r.y + Math.sin(t * 1.7 + r.id) * 5
    ctx.save()
    ctx.shadowColor = '#9dffff'
    ctx.shadowBlur = 24
    ctx.drawImage(crystal, r.x - 23, y - 25, 46, 50)
    ctx.restore()
  }
  const m = s.exploration.migration
  if (!m) return
  drawGiant(m.species, m.x, m.y, m.radius, m.facing)
  for (const pearl of m.pearls) {
    if (pearl.caught) continue
    ctx.save()
    ctx.shadowColor = '#ffedb0'
    ctx.shadowBlur = 16
    ctx.drawImage(
      crystal,
      pearl.x - 10,
      pearl.y - 11 + Math.sin(t * 2 + pearl.x) * 3,
      20,
      22,
    )
    ctx.restore()
  }
}
export function drawExplorationIndicators(
  ctx: CanvasRenderingContext2D,
  s: OceanState,
  c: { x: number; y: number; width: number; height: number },
  labels: ExplorationLabels,
) {
  if (s.phase !== 'playing' || s.challenge?.finale) return
  const m = s.exploration.migration,
    target = m
      ? m.pearls.find((p) => !p.caught)
      : [...s.exploration.relics]
          .filter((r) => !r.collected)
          .sort(
            (a, b) =>
              Math.hypot(a.x - s.player.x, a.y - s.player.y) -
              Math.hypot(b.x - s.player.x, b.y - s.player.y),
          )[0]
  if (!target) return
  const x = target.x - c.x,
    y = target.y - c.y,
    px = Math.max(70, Math.min(c.width - 70, x)),
    py = Math.max(145, Math.min(c.height - 135, y))
  ctx.save()
  ctx.fillStyle = m ? '#ffe7ad' : '#b2edf3'
  ctx.translate(px, py)
  if (x !== px || y !== py) {
    ctx.rotate(Math.atan2(y - c.height / 2, x - c.width / 2))
    ctx.beginPath()
    ctx.moveTo(8, 0)
    ctx.lineTo(-5, -5)
    ctx.lineTo(-5, 5)
    ctx.closePath()
    ctx.fill()
  }
  ctx.restore()
  ctx.fillStyle = m ? '#ffe7ad' : '#b2edf3'
  ctx.font = '600 10px system-ui'
  ctx.textAlign = 'center'
  const distance = Math.round(
    Math.hypot(target.x - s.player.x, target.y - s.player.y),
  )
  ctx.fillText(
    `${m ? (labels.migration ?? 'MIGRATION') : (labels.relic ?? 'RELIC')} · ${m ? `${m.caught}/6` : `${distance}m`}`,
    px,
    py + 21,
  )
}

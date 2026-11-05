import { tailPosition } from './challenge'
import type { OceanState } from './engine'
export interface ChallengeLabels {
  hunter?: string
  tail?: string
  shoal?: string
}
export function drawChallenge(
  ctx: CanvasRenderingContext2D,
  s: OceanState,
  labels: ChallengeLabels,
  drawFish: (x: number, y: number, r: number, facing: number) => void,
  _time: number,
) {
  const c = s.challenge
  if (!c) return
  const h = c.hunter
  if (h) {
    if (h.phase === 'aim') {
      const dx = h.aimX - h.x,
        dy = h.aimY - h.y,
        d = Math.max(1, Math.hypot(dx, dy)),
        length = d + 220
      ctx.save()
      ctx.translate(h.x, h.y)
      ctx.rotate(Math.atan2(dy, dx))
      ctx.fillStyle = h.remaining < 0.5 ? '#ff775c30' : '#ffbc7020'
      ctx.fillRect(0, -h.radius * 0.8, length, h.radius * 1.6)
      ctx.strokeStyle = '#ffe4ba88'
      ctx.lineWidth = 1.5
      for (let x = 70; x < length; x += 70) {
        ctx.beginPath()
        ctx.moveTo(x - 8, -6)
        ctx.lineTo(x, 0)
        ctx.lineTo(x - 8, 6)
        ctx.stroke()
      }
      ctx.restore()
    }
    if (h.phase === 'charge') {
      ctx.strokeStyle = '#ffbb9a88'
      ctx.lineWidth = h.radius * 0.65
      ctx.lineCap = 'round'
      ctx.beginPath()
      ctx.moveTo(h.x - h.vx * 0.2, h.y - h.vy * 0.2)
      ctx.lineTo(h.x, h.y)
      ctx.stroke()
    }
    ctx.save()
    ctx.shadowColor = h.phase === 'recover' ? '#a9f9cb' : '#ff855e'
    ctx.shadowBlur = 20
    drawFish(h.x, h.y, h.radius, h.facing)
    ctx.restore()
    ctx.font = '700 11px system-ui'
    ctx.textAlign = 'center'
    ctx.fillStyle = '#d4eddc'
    if (h.phase === 'recover' && !h.bitten) {
      const tail = tailPosition(h)
      ctx.fillStyle = '#edffc7'
      ctx.beginPath()
      ctx.moveTo(tail.x - 8, tail.y)
      ctx.lineTo(tail.x, tail.y - 11)
      ctx.lineTo(tail.x + 8, tail.y)
      ctx.lineTo(tail.x, tail.y + 11)
      ctx.closePath()
      ctx.fill()
      ctx.fillText(
        `${Math.ceil(h.remaining)}s`,
        tail.x,
        tail.y - h.radius * 0.6,
      )
    } else if (h.phase === 'aim') {
      ctx.fillStyle = '#ffd4ad'
      ctx.fillText('!', h.x, h.y - h.radius - 12)
    }
    if (c.finale) {
      for (let i = 0; i < 3; i++) {
        ctx.fillStyle = i < c.bossHealth ? '#ffa881' : '#4c8b8e88'
        ctx.fillRect(h.x - 46 + i * 33, h.y - h.radius - 9, 27, 5)
      }
    }
  }
  if (c.shoal) {
    const q = c.shoal
    ctx.font = '700 13px system-ui'
    ctx.textAlign = 'center'
    ctx.fillStyle = '#fff0b5'
    ctx.fillText(
      `◆ ${q.caught}/${q.total} · ${Math.ceil(q.remaining)}s`,
      q.x,
      q.y - 107,
    )
  }
}
export function drawChallengeIndicators(
  ctx: CanvasRenderingContext2D,
  s: OceanState,
  c: { x: number; y: number; width: number; height: number },
  labels: ChallengeLabels,
) {
  const state = s.challenge
  if (!state) return
  const targets = [
    ...(state.hunter
      ? [
          {
            x: state.hunter.x,
            y: state.hunter.y,
            color: '#ffd0ae',
            text: `! ${labels.hunter ?? 'HUNTER'}`,
          },
        ]
      : []),
    ...(state.shoal
      ? [
          {
            x: state.shoal.x,
            y: state.shoal.y,
            color: '#ffecb2',
            text: `◆ ${Math.ceil(state.shoal.remaining)}s`,
          },
        ]
      : []),
  ]
  for (const target of targets) {
    const x = target.x - c.x,
      y = target.y - c.y
    if (x > 35 && x < c.width - 35 && y > 80 && y < c.height - 80) continue
    const px = Math.max(60, Math.min(c.width - 60, x)),
      py = Math.max(95, Math.min(c.height - 100, y)),
      angle = Math.atan2(y - c.height / 2, x - c.width / 2)
    ctx.save()
    ctx.translate(px, py)
    ctx.rotate(angle)
    ctx.fillStyle = target.color
    ctx.beginPath()
    ctx.moveTo(12, 0)
    ctx.lineTo(-6, -7)
    ctx.lineTo(-6, 7)
    ctx.closePath()
    ctx.fill()
    ctx.restore()
    ctx.fillStyle = target.color
    ctx.font = '700 11px system-ui'
    ctx.textAlign = 'center'
    ctx.fillText(target.text, px, py + 24)
  }
}

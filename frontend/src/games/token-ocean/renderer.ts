import {
  drawChallenge,
  drawChallengeIndicators,
  type ChallengeLabels,
} from './challengeRenderer'
import { WORLD, evolutions, type OceanState, type Swimmer } from './engine'
import { pufferPhase, swimmerRadius, swimmerRelation } from './ecology'
import type { FishSprite } from './species'
import { isGentleGiant, isLuminous } from './exploration'
import {
  drawExploration,
  drawExplorationIndicators,
  type ExplorationLabels,
} from './explorationRenderer'
export interface OceanArt {
  cove: HTMLImageElement
  abyss: HTMLImageElement
  crystal: HTMLImageElement
  fish: Record<FishSprite, HTMLImageElement>
}
export function cameraFor(s: OceanState, aspect: number) {
  const height = Math.min(580, 1280 / Math.max(0.4, aspect)),
    width = height * Math.max(0.4, aspect)
  return {
    width,
    height,
    x: Math.max(0, Math.min(WORLD.width - width, s.player.x - width / 2)),
    y: Math.max(0, Math.min(WORLD.height - height, s.player.y - height / 2)),
  }
}
export function screenToWorld(
  s: OceanState,
  x: number,
  y: number,
  width: number,
  height: number,
) {
  const c = cameraFor(s, width / Math.max(1, height))
  return {
    x: c.x + (x / Math.max(1, width)) * c.width,
    y: c.y + (y / Math.max(1, height)) * c.height,
  }
}
function sparkle(
  ctx: CanvasRenderingContext2D,
  x: number,
  y: number,
  size: number,
  color: string,
) {
  ctx.fillStyle = color
  ctx.beginPath()
  ctx.moveTo(x, y - size)
  ctx.quadraticCurveTo(x + size * 0.2, y - size * 0.2, x + size, y)
  ctx.quadraticCurveTo(x + size * 0.2, y + size * 0.2, x, y + size)
  ctx.quadraticCurveTo(x - size * 0.2, y + size * 0.2, x - size, y)
  ctx.quadraticCurveTo(x - size * 0.2, y - size * 0.2, x, y - size)
  ctx.fill()
}
function fish(
  ctx: CanvasRenderingContext2D,
  image: HTMLImageElement,
  x: number,
  y: number,
  r: number,
  facing: number,
  time: number,
  options: { tilt?: number; bite?: number; glow?: string; alpha?: number } = {},
) {
  const w = r * 2.7,
    h = (w * image.height) / Math.max(1, image.width),
    chomp = options.bite
      ? Math.sin(Math.min(1, options.bite / 0.26) * Math.PI)
      : 0
  ctx.save()
  ctx.translate(x, y + Math.sin(time * 5) * Math.min(1.5, r * 0.02))
  ctx.rotate((options.tilt ?? 0) + Math.sin(time * 4) * 0.018)
  ctx.scale(
    -facing * (1 + chomp * 0.12 + Math.sin(time * 7) * 0.012),
    1 - chomp * 0.1 + Math.sin(time * 7 + 1) * 0.008,
  )
  if (options.alpha !== undefined) ctx.globalAlpha *= options.alpha
  ctx.shadowColor = options.glow ?? '#01324370'
  ctx.shadowBlur = options.glow ? 14 : 5
  ctx.shadowOffsetY = options.glow ? 0 : 3
  // Keep the painted silhouette continuous, including when giant fish fill the viewport.
  ctx.drawImage(image, -w / 2, -h / 2, w, h)
  ctx.restore()
}
export function renderOcean(
  ctx: CanvasRenderingContext2D,
  s: OceanState,
  art: OceanArt,
  width: number,
  height: number,
  labels: {
    you: string
    danger: string
    eat: string
    spines?: string
    venom?: string
    ambush?: string
    stamina?: string
  } & ChallengeLabels &
    ExplorationLabels,
  reduced = false,
) {
  const c = cameraFor(s, width / height),
    t = reduced ? 0 : s.time,
    p = s.player
  ctx.save()
  ctx.scale(width / c.width, height / c.height)
  ctx.drawImage(
    art.cove,
    0,
    310,
    art.cove.width,
    art.cove.height - 310,
    -c.x * 0.08,
    -c.y * 0.06,
    c.width + WORLD.width * 0.08,
    c.height + WORLD.height * 0.06,
  )
  const depth = Math.max(0, Math.min(1, (p.y - 480) / 640))
  if (depth > 0) {
    ctx.globalAlpha = depth
    ctx.drawImage(
      art.abyss,
      -c.x * 0.06,
      -c.y * 0.025,
      c.width + WORLD.width * 0.06,
      c.height + WORLD.height * 0.025,
    )
    ctx.globalAlpha = 1
  }
  const tint = ctx.createLinearGradient(0, 0, c.width * 0.6, c.height)
  tint.addColorStop(0, '#04495b08')
  tint.addColorStop(1, `rgba(8, 20, 52, ${0.2 + depth * 0.17})`)
  ctx.fillStyle = tint
  ctx.fillRect(0, 0, c.width, c.height)
  // Soft shafts, near-camera plankton, and background fish give the painted reef depth.
  for (let i = 0; i < 5; i++) {
    const x = (i * c.width) / 4 - 100 + Math.sin(t * 0.14 + i) * 22
    const beam = ctx.createLinearGradient(x, 0, x + 190, c.height * 0.85)
    beam.addColorStop(0, '#d8fff40a')
    beam.addColorStop(1, '#b0fff100')
    ctx.fillStyle = beam
    ctx.beginPath()
    ctx.moveTo(x, 0)
    ctx.lineTo(x + 45, 0)
    ctx.lineTo(x + 260, c.height)
    ctx.lineTo(x + 120, c.height)
    ctx.fill()
  }
  for (let i = 0; i < 20; i++) {
    const x =
        (((i * 157 + t * (4 + (i % 3)) - c.x * 0.13) % (c.width + 90)) +
          c.width +
          90) %
        (c.width + 90),
      y =
        (((i * 89 - t * (3 + (i % 5)) - c.y * 0.1) % c.height) + c.height) %
        c.height
    ctx.globalAlpha = 0.15 + (i % 4) * 0.035
    ctx.fillStyle = '#c9f8ee'
    ctx.beginPath()
    ctx.arc(x, y, 1 + (i % 3) * 0.55, 0, Math.PI * 2)
    ctx.fill()
  }
  ctx.globalAlpha = 1
  ctx.save()
  if (s.shake > 0 && !reduced)
    ctx.translate(Math.sin(t * 90) * 3, Math.cos(t * 75) * 2)
  ctx.translate(-c.x, -c.y)
  drawExploration(
    ctx,
    s,
    art.crystal,
    (species, x, y, r, facing) =>
      fish(ctx, art.fish[species], x, y, r, facing, t, { alpha: 0.9 }),
    reduced,
  )
  const school: Swimmer[] =
    s.phase === 'ready'
      ? [
          {
            x: p.x + 170,
            y: p.y - 70,
            radius: 12,
            species: 'clownfish',
            id: -1,
            vy: 0,
            vx: -1,
            golden: false,
            turn: 1,
          },
          {
            x: p.x + 260,
            y: p.y + 50,
            radius: 13,
            species: 'butterflyfish',
            id: -2,
            vy: 0,
            vx: -1,
            golden: true,
            turn: 2,
          },
          {
            x: p.x - 280,
            y: p.y + 80,
            radius: 40,
            species: 'anglerfish',
            id: -3,
            vy: 0,
            vx: 1,
            golden: false,
            turn: 3,
          },
        ]
      : s.fish
  for (const f of [...school].sort(
    (a, b) =>
      Number(isGentleGiant(b.species)) - Number(isGentleGiant(a.species)) ||
      a.radius - b.radius,
  )) {
    if (
      f.x + f.radius * 2 < c.x ||
      f.x - f.radius * 2 > c.x + c.width ||
      f.y + f.radius * 2 < c.y ||
      f.y - f.radius * 2 > c.y + c.height
    )
      continue
    const kind = swimmerRelation(s, f),
      puff = pufferPhase(s, f),
      radius = swimmerRadius(s, f),
      near = Math.hypot(f.x - p.x, f.y - p.y)
    const ambush = f.ambush
    if (ambush?.phase === 'aim') {
      const dx = ambush.x - f.x,
        dy = ambush.y - f.y,
        d = Math.max(1, Math.hypot(dx, dy)),
        nx = (-dy / d) * 7,
        ny = (dx / d) * 7
      ctx.save()
      ctx.fillStyle = ambush.remaining <= 0.45 ? '#ffb18250' : '#ffcf9c25'
      ctx.beginPath()
      ctx.moveTo(f.x + nx, f.y + ny)
      ctx.lineTo(ambush.x + nx * 0.4, ambush.y + ny * 0.4)
      ctx.lineTo(ambush.x - nx * 0.4, ambush.y - ny * 0.4)
      ctx.lineTo(f.x - nx, f.y - ny)
      ctx.closePath()
      ctx.fill()
      ctx.restore()
    }
    fish(
      ctx,
      art.fish[puff === 'inflated' ? 'pufferfish-inflated' : f.species],
      f.x,
      f.y,
      radius,
      f.vx >= 0 ? 1 : -1,
      t + f.turn,
      {
        glow: f.golden
          ? '#ffe5a5'
          : puff === 'warning'
            ? '#ffba70'
            : f.species === 'lionfish'
              ? '#d9a1ee'
              : isLuminous(f.species)
                ? '#78e9ee'
                : undefined,
        tilt:
          Math.max(
            -0.23,
            Math.min(
              0.23,
              Math.atan2(
                (f as { vy?: number }).vy ?? 0,
                Math.max(70, Math.abs(f.vx)),
              ),
            ),
          ) * (f.vx >= 0 ? 1 : -1),
      },
    )
    if (f.golden) {
      for (let i = 0; i < 2; i++) {
        const a = t * 1.5 + i * 3 + f.turn
        sparkle(
          ctx,
          f.x + Math.cos(a) * f.radius * 1.35,
          f.y + Math.sin(a) * f.radius * 0.85,
          2.5 + Math.sin(t * 3 + i),
          '#ffe7b2',
        )
      }
    }
    // Close-range words explain dangerous exceptions without outlining the fish.
    const hint =
      near < p.radius + radius + 120
        ? puff !== 'calm'
          ? (labels.spines ?? 'WAIT FOR SPINES')
          : f.species === 'lionfish' && kind === 'danger'
            ? (labels.venom ?? 'VENOM')
            : ambush?.phase === 'aim'
              ? (labels.ambush ?? 'AMBUSH')
              : ''
        : ''
    if (hint) {
      ctx.fillStyle = '#ffdfbd'
      ctx.font = '600 11px system-ui'
      ctx.textAlign = 'center'
      ctx.fillText(hint, f.x, f.y - radius - 27)
    }
    if (isGentleGiant(f.species) && near < p.radius + radius + 70) {
      ctx.fillStyle = '#b5ddd6'
      ctx.font = '500 10px system-ui'
      ctx.textAlign = 'center'
      ctx.fillText(
        labels.gentle ?? 'GENTLE GIANT',
        f.x,
        f.y - Math.min(radius, 100) - 12,
      )
    }
    // A tiny warning appears only for an immediate threat. Fish are never enclosed in outlines.
    if (kind === 'danger' && near < p.radius + f.radius + 85) {
      ctx.fillStyle = '#ffbfa7'
      ctx.beginPath()
      ctx.moveTo(f.x, f.y - f.radius - 13)
      ctx.lineTo(f.x - 4, f.y - f.radius - 20)
      ctx.lineTo(f.x + 4, f.y - f.radius - 20)
      ctx.fill()
    }
  }
  drawChallenge(
    ctx,
    s,
    labels,
    (x, y, r, facing) =>
      fish(ctx, art.fish.hammerhead, x, y, r, facing, t, {
        glow:
          s.challenge?.hunter?.phase === 'recover' ? '#b3dfab80' : undefined,
      }),
    t,
  )
  for (const g of s.gulps) {
    const life = g.life / 0.22
    fish(
      ctx,
      art.fish[g.species],
      g.x,
      g.y,
      g.radius * Math.max(0.04, life),
      g.facing,
      t,
      { alpha: Math.min(1, life * 1.5), tilt: (1 - life) * 0.65 },
    )
  }
  const tilt = Math.max(-0.32, Math.min(0.32, p.vy / 600)) * p.facing
  fish(
    ctx,
    art.fish[evolutions[s.level]!.id],
    p.x,
    p.y,
    p.radius,
    p.facing,
    t,
    {
      tilt,
      bite: reduced ? 0 : s.bite,
      glow: s.frenzy > 0 ? '#ffd78c' : s.invincible > 0 ? '#d7fff4' : undefined,
      alpha: s.invincible > 0 && Math.floor(t * 8) % 2 ? 0.8 : 1,
    },
  )
  // Small crest above the player replaces the former selection ring and permanent YOU label.
  ctx.fillStyle = s.frenzy > 0 ? '#ffe5a5' : '#e4fff1'
  ctx.beginPath()
  ctx.moveTo(p.x - 4, p.y - p.radius - 11)
  ctx.lineTo(p.x, p.y - p.radius - 6)
  ctx.lineTo(p.x + 4, p.y - p.radius - 11)
  ctx.closePath()
  ctx.fill()
  for (const q of s.particles) {
    ctx.globalAlpha = q.life / q.maxLife
    sparkle(ctx, q.x, q.y, (q.radius * q.life) / q.maxLife, q.color)
  }
  for (const q of s.popups) {
    ctx.globalAlpha = Math.min(1, q.life * 3)
    ctx.fillStyle = q.color
    ctx.font = '700 16px system-ui'
    ctx.textAlign = 'center'
    ctx.fillText(q.text, q.x, q.y)
  }
  ctx.globalAlpha = 1
  ctx.restore()
  drawChallengeIndicators(ctx, s, c, labels)
  drawExplorationIndicators(ctx, s, c, labels)
  const vignette = ctx.createRadialGradient(
    c.width / 2,
    c.height * 0.45,
    c.height * 0.25,
    c.width / 2,
    c.height * 0.5,
    Math.max(c.width, c.height) * 0.7,
  )
  vignette.addColorStop(0, '#001c2f00')
  vignette.addColorStop(1, '#001e3c55')
  ctx.fillStyle = vignette
  ctx.fillRect(0, 0, c.width, c.height)
  if (s.flash > 0 && !reduced) {
    ctx.fillStyle = `rgba(255,236,174,${s.flash * 0.32})`
    ctx.fillRect(0, 0, c.width, c.height)
  }
  ctx.restore()
}

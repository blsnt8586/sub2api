import { hookPosition, WORLD, type GameState, type Ore } from './engine'

const { width: W, height: H, originX: OX, originY: OY } = WORLD
const sprites = new Map<string, HTMLImageElement>()
let pending: Promise<void> | undefined
export function loadMinerArtwork(): Promise<void> {
  if (!pending) {
    pending = Promise.all(
      [
        'cavern',
        'rig',
        'token',
        'large',
        'gem',
        'rock',
        'claw',
        'dynamite',
      ].map(
        (name) =>
          new Promise<void>((resolve, reject) => {
            const image = new Image()
            image.onload = () => {
              sprites.set(name, image)
              resolve()
            }
            image.onerror = () =>
              reject(new Error(`Unable to load mining artwork: ${name}`))
            image.src = `/games/token-miner/${name}.webp`
          }),
      ),
    )
      .then(() => {})
      .catch((error) => {
        pending = undefined
        throw error
      })
  }
  return pending
}
function sprite(
  c: CanvasRenderingContext2D,
  name: string,
  x: number,
  y: number,
  width: number,
  height?: number,
) {
  const image = sprites.get(name)
  if (!image) return
  const h = height ?? (width * image.naturalHeight) / image.naturalWidth
  c.drawImage(image, x - width / 2, y - h / 2, width, h)
}
function ore(
  c: CanvasRenderingContext2D,
  o: Ore,
  x = o.x,
  y = o.y,
  time = 0,
  reducedMotion = false,
) {
  c.save()
  if (o.kind === 'gem') {
    const glow = c.createRadialGradient(x, y, 0, x, y, 50)
    glow.addColorStop(0, 'rgba(66,255,200,.18)')
    glow.addColorStop(1, 'rgba(66,255,200,0)')
    c.fillStyle = glow
    c.fillRect(x - 50, y - 50, 100, 100)
  }
  c.shadowColor = o.lucky ? '#ffcc58' : '#0009'
  c.shadowBlur = 12
  c.shadowOffsetY = 5
  const size = o.radius * 2 + 14
  // Maintain the asset's proportions inside its collision envelope.
  const image = sprites.get(o.kind)
  if (image) {
    const scale = size / Math.max(image.naturalWidth, image.naturalHeight)
    sprite(c, o.kind, x, y, image.naturalWidth * scale)
  }
  c.shadowColor = 'transparent'
  if (o.lucky) {
    c.fillStyle = '#ffe7a3'
    c.font = '700 14px ui-sans-serif, system-ui'
    c.textAlign = 'center'
    c.fillText('★', x, y - o.radius - 8)
  }
  if ((o.kind === 'gem' || o.kind === 'large') && !reducedMotion) {
    c.globalAlpha = Math.max(0, Math.sin(time * 2.5 + o.id * 2)) * 0.8
    c.strokeStyle = '#eaffd5'
    c.lineWidth = 1.5
    const sx = x + o.radius * 0.75,
      sy = y - o.radius * 0.7
    c.beginPath()
    c.moveTo(sx - 5, sy)
    c.lineTo(sx + 5, sy)
    c.moveTo(sx, sy - 5)
    c.lineTo(sx, sy + 5)
    c.stroke()
  }
  c.restore()
}

export function drawGame(
  c: CanvasRenderingContext2D,
  state: GameState,
  options: { dark: boolean; reducedMotion: boolean; time: number },
) {
  c.save()
  c.fillStyle = '#062c30'
  c.fillRect(0, 0, W, H)
  const cavern = sprites.get('cavern')
  if (cavern) c.drawImage(cavern, 0, 0, W, H)
  // The winch outlet is positioned at the physics origin; the robot sits on the deck.
  sprite(c, 'rig', OX + 35, 83, 235)
  for (const o of state.ores) {
    if (o.collected || o === state.grabbed) continue
    ore(c, o, o.x, o.y, options.time, options.reducedMotion)
  }
  if (!options.reducedMotion) {
    // Tiny ambient motes, kept subdued so the moving claw remains the focal point.
    for (let i = 0; i < 14; i++) {
      const x = 90 + ((i * 157.3) % 780)
      const y = 180 + ((i * 73.7 + options.time * (3 + (i % 3))) % 370)
      c.fillStyle = `rgba(255,219,151,${0.08 + 0.08 * Math.sin(i + options.time)})`
      c.beginPath()
      c.arc(x, y, i % 2 ? 1 : 1.5, 0, Math.PI * 2)
      c.fill()
    }
  }
  const tip = hookPosition(state)
  if (state.hook === 'swinging' && state.status === 'playing') {
    c.save()
    c.setLineDash([2, 12])
    c.strokeStyle = '#c6ebcf24'
    c.lineWidth = 1
    c.beginPath()
    c.moveTo(OX, OY)
    c.lineTo(OX + Math.sin(state.angle) * 450, OY + Math.cos(state.angle) * 450)
    c.stroke()
    c.restore()
  }
  c.lineCap = 'round'
  c.strokeStyle = '#081b1c'
  c.lineWidth = 5
  c.beginPath()
  c.moveTo(OX, OY)
  c.lineTo(tip.x, tip.y)
  c.stroke()
  const cable = c.createLinearGradient(OX - 2, OY, tip.x + 2, tip.y)
  cable.addColorStop(0, '#f4d092')
  cable.addColorStop(0.4, '#b5b6a1')
  cable.addColorStop(1, '#ece5bf')
  c.strokeStyle = cable
  c.lineWidth = 2.4
  c.beginPath()
  c.moveTo(OX, OY)
  c.lineTo(tip.x, tip.y)
  c.stroke()
  if (state.grabbed)
    ore(
      c,
      state.grabbed,
      tip.x,
      tip.y + state.grabbed.radius * 0.65,
      options.time,
      true,
    )
  c.save()
  c.translate(tip.x, tip.y + 6)
  c.rotate(-state.angle)
  sprite(c, 'claw', 0, 6, state.grabbed ? 32 : 41, 43)
  c.restore()
  if (state.feedback) {
    const f = state.feedback
    c.globalAlpha = Math.max(0, 1 - f.age / 1.3)
    c.fillStyle = '#ffe5a6'
    c.font = '800 28px ui-sans-serif, system-ui'
    c.textAlign = 'center'
    c.shadowColor = '#041b1d'
    c.shadowBlur = 8
    c.fillText(
      `+${f.value}`,
      f.x,
      f.y + 42 - (options.reducedMotion ? 0 : f.age * 40),
    )
  }
  c.restore()
}

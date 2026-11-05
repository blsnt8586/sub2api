import { roundFor, ROUNDS, completeRound, type HuntGoal } from './campaign'
import type { OceanState, Swimmer } from './engine'
import { zoneAt, type OceanZone } from './exploration'

export type Weather = 'bloom' | 'current' | 'golden'
export interface VoyageLeg {
  zone: OceanZone
  kind: HuntGoal
  target: number
  caught: number
}
export interface Voyage {
  legs: VoyageLeg[]
  stage: number
  weather: Weather | null
  previousWeather: Weather | null
  weatherRemaining: number
  weatherAt: number
  clearedTime: number
}
/** Seeded objectives are chosen once per run; catching fish cannot reroll a contract. */
export function createVoyage(seed: number, round = 0): Voyage {
  let n = seed >>> 0
  const random = () => {
    n = (Math.imul(n, 1664525) + 1013904223) >>> 0
    return n / 4294967296
  }
  return {
    legs: [{ zone: ROUNDS[round]!.zone, kind: ROUNDS[round]!.goal,
      target: ROUNDS[round]!.target + Math.floor(random() * 5), caught: 0 }],
    stage: 0,
    weather: null,
    previousWeather: null,
    weatherRemaining: 0,
    weatherAt: 22 + random() * 12,
    clearedTime: 0,
  }
}
export function catchVoyage(s: OceanState, fish: Swimmer) {
  if (s.phase !== 'playing' || s.challenge?.finale) return
  const v = s.voyage, leg = v.legs[v.stage]
  // The catch location determines the region; crossing a boundary after a catch cannot count it twice.
  if (!leg || zoneAt(fish.y) !== leg.zone || (leg.kind === 'dash' && s.dash <= 0) || (leg.kind === 'golden' && !fish.golden)) return
  leg.caught++
  if (leg.caught < leg.target) return
  v.stage++
  v.clearedTime = 4
  s.score += 300
  s.stamina = 100
  s.event++
  if (!roundFor(s).boss) completeRound(s)
}
export const voyageComplete = (s: OceanState) => s.voyage.stage >= s.voyage.legs.length
export function tickVoyage(s: OceanState, dt: number, random: () => number) {
  if (s.phase !== 'playing') return
  const v = s.voyage
  v.clearedTime = Math.max(0, v.clearedTime - dt)
  if (s.challenge?.finale) {
    v.weather = null
    v.weatherRemaining = 0
    return
  }
  v.weatherRemaining = Math.max(0, v.weatherRemaining - dt)
  if (v.weather && v.weatherRemaining === 0) v.weather = null
  if (v.weather || s.time < v.weatherAt) return
  const leg = v.legs[v.stage]
  const options = (['bloom', 'current', 'golden'] as const).filter((w) => w !== v.previousWeather)
  v.weather = leg?.kind === 'golden' && v.previousWeather !== 'golden' ? 'golden' : options[Math.min(options.length - 1, Math.floor(random() * options.length))]!
  v.previousWeather = v.weather
  v.weatherRemaining = 14 + random() * 6
  v.weatherAt = s.time + v.weatherRemaining + 18 + random() * 16
  s.event++
  if (v.weather === 'current') return
  // A visible fresh school announces the event. Every body starts clear of the player.
  const direction = random() < 0.5 ? -1 : 1
  const cx = Math.max(200, Math.min(2400, s.player.x + direction * 360))
  const cy = Math.max(180, Math.min(1360, s.player.y + (random() - 0.5) * 220))
  const school = s.nextId++
  for (let i = 0; i < 12 && s.fish.length < 168; i++) {
    const radius = Math.max(9, s.player.radius * (0.4 + random() * 0.15))
    const x = cx + (i % 4) * 28, y = cy + Math.floor(i / 4) * 32
    if (Math.hypot(x - s.player.x, y - s.player.y) < s.player.radius + radius + 110) continue
    s.fish.push({ id: s.nextId++, x, y, vx: -direction * 50, vy: 0, radius,
      species: zoneAt(y) === 'reef' ? 'sardine' : 'lanternfish',
      golden: v.weather === 'golden', turn: i, school })
  }
}

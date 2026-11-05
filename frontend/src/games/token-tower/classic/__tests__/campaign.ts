import route from './legal-route.json'
import { buyClassic, createClassic, moveClassic, startClassic, type Upgrade } from '../engine'

// Recorded legal inputs, not an in-game solver. No stats, coordinates or map state are written.
// Lowercase is movement; uppercase explicitly accepts a curse; digits purchase shrine upgrades.
export function runClassicCampaign() {
  const s = createClassic('knight', route.seed)
  const directions: Record<string, [number, number]> = { u: [0, -1], d: [0, 1], l: [-1, 0], r: [1, 0] }
  const upgrades: Upgrade[] = ['health', 'attack', 'defense', 'yellow', 'blue', 'red']
  startClassic(s)
  for (const [index, input] of [...route.moves].entries()) {
    const direction = directions[input.toLowerCase()]
    const accepted = direction ? moveClassic(s, ...direction, input !== input.toLowerCase()) : buyClassic(s, upgrades[Number(input)]!)
    if (!accepted) throw new Error(`Illegal replay input ${index}: ${input}, ${JSON.stringify(s.notice)}`)
  }
  return { state: s, actions: [...route.moves] }
}

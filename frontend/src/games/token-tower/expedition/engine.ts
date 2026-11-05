import campaign from './campaign.json'
import legacyCampaign from './legacy-50.json'
import { createCampaignEngine, type CampaignDefinition } from '../campaign-engine'
export { campaign }
export type { Point, Role, Phase, KeyColor, Upgrade, Stats, Snapshot, State, Battle, BattleStrike } from '../campaign-engine'
export const { SIZE, FLOORS, MONSTERS, DOORS, KEYS, SHOP, same, keyFor, monsterFor, wall, curse, neighbors, inside, rawTile, tileAt, pointsOf, battleCount, nearShop, heroSprite, stars, createClassic, previewBattle, startClassic, pauseClassic, moveClassic, buyClassic, undoClassic, saveClassic, loadClassic: loadCurrent, shopOffers, spriteTile, zoneOf, bossesDefeated } = createCampaignEngine(campaign as CampaignDefinition)

const legacyEngine = createCampaignEngine(legacyCampaign as CampaignDefinition)
/** Validate the original save against its own maps before adapting the extended campaign. */
export function migrateLegacy50(raw: string | null): ReturnType<typeof loadCurrent> {
  const previous = legacyEngine.loadClassic(raw)
  if (!previous) return null
  const wasWon = previous.phase === 'won'
  if (wasWon) {
    previous.phase = 'paused'
    previous.score = Math.max(0, previous.score - 2000 - previous.stats.hp)
  }
  previous.history = []
  const parsed = JSON.parse(legacyEngine.saveClassic(previous))
  parsed.campaign = campaign.sourceRevision
  const migrated = loadCurrent(JSON.stringify(parsed))
  if (migrated) migrated.notice = { key: wasWon ? 'extended' : 'migrated' }
  return migrated
}
export const loadClassic = loadCurrent

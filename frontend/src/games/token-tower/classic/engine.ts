// MIT classic campaign adapter. See public/games/token-tower/classic/LICENSE.txt.
import campaign from './campaign.json'
import { createCampaignEngine } from '../campaign-engine'
export type { Point, Role, Phase, KeyColor, Upgrade, Stats, Snapshot, State, Battle, BattleStrike, CampaignMonster } from '../campaign-engine'
export const { SIZE, FLOORS, MONSTERS, DOORS, KEYS, SHOP, same, keyFor, monsterFor, wall, curse, neighbors, inside, rawTile, tileAt, pointsOf, battleCount, nearShop, heroSprite, stars, createClassic, previewBattle, startClassic, pauseClassic, moveClassic, buyClassic, undoClassic, saveClassic, loadClassic, shopOffers, spriteTile, zoneOf, bossesDefeated } = createCampaignEngine(campaign)

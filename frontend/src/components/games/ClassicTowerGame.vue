<template>
  <component :is="immersive ? 'div' : AppLayout">
    <main ref="gameRoot" class="classic-tower" :style="{ '--ct-board-size': `${boardSize}px` }" :class="{ 'ct-immersive': immersive, 'ct-mobile': narrow }">
      <header class="ct-header">
        <div>
          <RouterLink to="/arcade" class="ct-back"><Icon name="arrowLeft" size="sm" />{{ t('park.back') }}</RouterLink>
          <div class="ct-wordmark"><h1>{{ t('tower.title') }}</h1><span>{{ ct('edition') }}</span></div>
          <p>{{ ct('subtitle') }}</p>
        </div>
        <div class="ct-header-actions">
          <button class="ct-button" :aria-pressed="immersive" @click="toggleImmersive"><Icon :name="immersive ? 'x' : 'arrowsUpDown'" size="sm" /><span>{{ t(immersive ? 'park.exitConsole' : 'park.consoleMode') }}</span></button>
        </div>
      </header>

      <div class="ct-layout">
        <section class="ct-stats" :aria-label="t('tower.hero')">
          <div class="ct-stage-header"><div class="ct-floor"><strong>{{ String(state.floor + 1).padStart(2, '0') }}<small>F</small></strong><div><span>{{ t('tower.floor', { n: state.floor + 1, total: FLOORS.length }) }}</span><h2>{{ floorTitle }}</h2></div></div><div class="ct-tools"><button :title="t('tower.bookShortcut')" :aria-label="t('tower.book')" @click="openBook"><Icon name="grid" size="sm" /></button><button :title="t('tower.undoShortcut')" :aria-label="t('tower.undo')" :disabled="!state.history.length || state.phase === 'won' || Boolean(combat)" @click="undo"><Icon name="arrowLeft" size="sm" /></button><button :aria-label="t(state.phase === 'paused' ? 'tower.resume' : 'tower.pause')" :disabled="state.phase === 'ready' || state.phase === 'won'" @click="togglePause"><Icon :name="state.phase === 'paused' ? 'play' : 'pause'" size="sm" /></button><button :aria-label="t('tower.restart')" @click="askRestart()"><Icon name="refresh" size="sm" /></button></div></div>
        <div class="ct-vitals"><span class="ct-sprite ct-portrait" :style="sprite(heroSprite(state))" aria-hidden="true"></span><div><span>{{ t('tower.hp') }} <strong data-testid="classic-hp">{{ combat?.displayHp ?? state.stats.hp }}</strong></span><progress :value="combat?.displayHp ?? state.stats.hp" :max="state.stats.maxHp" :aria-label="t('tower.hp')"></progress></div></div>
        <div v-for="stat in statItems" :key="stat.key" class="ct-stat"><span class="ct-sprite" :style="sprite(stat.tile)" aria-hidden="true"></span><span>{{ t(`tower.${stat.key}`) }}<strong :data-testid="`classic-${stat.key}`">{{ state.stats[stat.key] }}</strong></span></div>
        <div class="ct-xp"><span>{{ t('tower.level', { n: state.stats.level }) }} <b>{{ state.stats.experience }}/{{ state.stats.next }} XP</b></span><progress :value="state.stats.experience" :max="state.stats.next" :aria-label="t('tower.experience')"></progress></div>
        <div class="ct-keys"><span v-for="(tile, i) in [6, 7, 8]" :key="tile"><span class="ct-sprite" :style="sprite(tile)" role="img" :aria-label="t(`tower.colors.${colors[i]}`)"></span><b>{{ state.stats.keys[colors[i]!] }}</b></span></div>
        <div class="ct-left-equipment"><span v-for="item in equipment" :key="item.key" :class="{ found: state.stats[item.key] }"><span class="ct-sprite" :style="sprite(item.tile)" aria-hidden="true"></span><small>{{ ct(`equipment.${item.key}`) }}</small></span></div>
      </section>

        <section class="ct-game">
          <div ref="stage" class="ct-stage" tabindex="0" :aria-label="ct('board')">
            <div class="ct-board" role="grid" :aria-label="floorTitle">
              <button v-for="cell in cells" :key="cell.index" class="ct-cell" role="gridcell" :class="{ 'ct-selected': selected && same(selected, cell.point), 'ct-hero-cell': same(combat?.from ?? state.position, cell.point), 'ct-stair': cell.tile === 80 || cell.tile === 90, 'ct-danger-tile': curse(cell.tile) }" :disabled="!artReady || state.phase !== 'playing' || Boolean(combat) || wall(cell.tile)" :tabindex="same(combat?.from ?? state.position, cell.point) ? 0 : -1" :aria-label="label(cell.tile, cell.point)" :title="label(cell.tile, cell.point)" @click="selectCell(cell.point)" @pointerenter="hover(cell.point)">
                <span class="ct-sprite" :style="sprite(combat && same(combat.at, cell.point) ? combat.tile : cell.tile)" aria-hidden="true"></span>
                <span v-if="cell.preview && showDamage" class="ct-damage" :class="{ danger: !cell.preview.canWin }">−{{ cell.preview.damage }}</span>
                <span v-if="same(combat?.from ?? state.position, cell.point)" class="ct-sprite ct-hero" :class="{ 'ct-hit': effect?.kind === 'fight', 'ct-loot': effect?.kind === 'loot' }" :style="sprite(heroSprite(state))" aria-hidden="true"></span>
                <span v-if="effect && same(effect.at, cell.point) && effect.kind === 'fight'" :key="effect.id" class="ct-pop">−{{ effect.amount }}</span>
              </button>
            </div>
            <ClassicTowerBattle v-if="combat" :frames="combat.frames" :hero="combat.hero" :hero-name="ct(`roles.${state.role}`)" :hero-hp="combat.heroHp" :monster="spriteTile(combat.tile)" :monster-name="tileName(combat.tile)" :monster-hp="monsterFor(combat.tile)!.hp" :gold="monsterFor(combat.tile)!.gold" :exp="monsterFor(combat.tile)!.experience" @complete="finishCombat" @health="hp => { if (combat) combat.displayHp = hp }" />
            <div v-if="!combat && (state.phase !== 'playing' || !artReady)" class="ct-screen">
              <template v-if="artError"><Icon name="exclamationTriangle" size="lg" /><h3>{{ t('tower.artError') }}</h3><button class="ct-primary" @click="loadArt">{{ t('tower.retry') }}</button></template>
              <template v-else-if="!artReady"><span class="ct-sprite ct-screen-hero" :style="sprite(113)" aria-hidden="true"></span><h3>{{ t('tower.loading') }}</h3></template>
              <template v-else-if="state.phase === 'won'"><span class="ct-sprite ct-screen-hero" :style="sprite(99)" aria-hidden="true"></span><span class="ct-eyebrow">{{ ct('complete') }}</span><h3>{{ t('tower.won') }}</h3><p>{{ ct('wonHint') }}</p><strong class="ct-result">{{ state.score.toLocaleString() }} <small>{{ '★'.repeat(stars(state)) }}</small></strong><p>{{ t('tower.resultDetail', { n: battleCount(state), steps: state.steps }) }}</p><button class="ct-primary" @click="restartPrompt = true">{{ t('tower.restart') }}</button></template>
              <template v-else-if="state.phase === 'ready'"><span class="ct-eyebrow">100 FLOORS / TEN REALMS</span><h3>{{ ct('startTitle') }}</h3><p>{{ ct('startHint') }}</p><div class="ct-role-select" role="radiogroup" :aria-label="ct('chooseRole')"><button v-for="role in roles" :key="role" role="radio" :aria-checked="state.role === role" @click="chooseRole(role)"><span class="ct-sprite" :style="sprite(role === 'knight' ? 113 : 103)" aria-hidden="true"></span><span><b>{{ ct(`roles.${role}`) }}</b><small>{{ ct(`roles.${role}Hint`) }}</small></span></button></div><button class="ct-primary ct-start" @click="start"><Icon name="play" size="sm" />{{ t('tower.start') }}</button></template>
              <template v-else><span class="ct-sprite ct-screen-hero" :style="sprite(heroSprite(state))" aria-hidden="true"></span><h3>{{ t('tower.pausedTitle') }}</h3><p>{{ t('tower.pausedHint') }}</p><button class="ct-primary ct-resume" @click="start"><Icon name="play" size="sm" />{{ t('tower.resume') }}</button></template>
            </div>
          </div>
          <div class="ct-notice" aria-live="polite"><Icon :name="state.event.kind === 'blocked' ? 'informationCircle' : 'checkCircle'" size="sm" /><p>{{ combat ? ct('combat.exchanging') : ct(`notices.${state.notice.key}`, noticeValues) }}</p></div>
          <div class="ct-controls"><div class="ct-dpad" :aria-label="t('tower.move')"><button v-for="d in directions" :key="d.key" :class="d.key" :disabled="!canMove" :aria-label="ct(`directions.${d.key}`)" @click="move(d.dx, d.dy)"><Icon :name="d.icon" size="sm" /></button></div><p>{{ ct('controls') }}<small v-if="connected">{{ t('park.controllerConnected') }}</small></p><label class="ct-damage-toggle"><input v-model="showDamage" type="checkbox" />{{ t('tower.damageLabels') }}</label></div>
        </section>

        <aside class="ct-aside" :class="[`ct-panel-${activePanel}`, { 'ct-details-open': detailsOpen }]">
          <nav class="ct-panel-tabs" :aria-label="ct('panels.label')"><button v-for="panel in panels" :key="panel" :aria-pressed="activePanel === panel" :aria-expanded="compactPortrait ? activePanel === panel && detailsOpen : undefined" @click="selectPanel(panel)">{{ ct(`panels.${panel}`) }}</button></nav>
          <section class="ct-inspect"><div class="ct-section-label"><span>01 / {{ t('tower.preview') }}</span><Icon name="eye" size="sm" /></div><template v-if="inspectedMonster && inspectedBattle"><div class="ct-enemy-title"><span class="ct-sprite ct-enemy" :style="sprite(inspectedMonster.id)" aria-hidden="true"></span><div><h3>{{ tileName(inspectedMonster.id) }}</h3><span>{{ ct('rewards', { gold: inspectedMonster.gold, exp: inspectedMonster.experience }) }}</span></div></div><dl class="ct-enemy-stats"><div><dt>HP</dt><dd>{{ inspectedMonster.hp }}</dd></div><div><dt>{{ t('tower.attack') }}</dt><dd>{{ inspectedMonster.attack }}</dd></div><div><dt>{{ t('tower.defense') }}</dt><dd>{{ inspectedMonster.defense }}</dd></div></dl><div class="ct-battle-cost" :class="{ danger: !inspectedBattle.canWin }"><span>{{ t('tower.expectedDamage') }}<small>{{ ct('rounds', { n: inspectedBattle.rounds }) }}</small></span><strong>−{{ inspectedBattle.damage }} <small>HP</small></strong></div><p class="ct-ability"><b>{{ t(`tower.abilities.${inspectedMonster.ability}`) }}</b> · {{ t(`tower.abilityHints.${inspectedMonster.ability}`) }}</p><p>{{ ct('randomHint') }}</p></template><template v-else-if="selected && curse(tileAt(state, selected))"><span class="ct-sprite ct-enemy" :style="sprite(tileAt(state, selected))" aria-hidden="true"></span><h3>{{ tileName(tileAt(state, selected)) }}</h3><p>{{ ct(`notices.curse${tileAt(state, selected)}`) }}</p><button v-if="adjacent(selected)" class="ct-primary" :disabled="!canMove" @click="cursePrompt = { ...selected }; stopPath()">{{ ct('faceCurse') }}</button></template><p v-else>{{ ct('inspectHint') }}</p><button class="ct-text-button" @click="openBook"><Icon name="grid" size="sm" />{{ state.stats.book ? t('tower.book') : ct('bookLocked') }}</button></section>
          <section class="ct-inventory"><div class="ct-section-label"><span>02 / {{ t('tower.objective') }}</span><Icon name="flag" size="sm" /></div><h3>{{ ct('quest') }}</h3><p>{{ ct(`hints.${state.floor % 10}`) }}</p><div class="ct-equipment"><span v-for="item in equipment" :key="item.key" :class="{ found: state.stats[item.key] }"><span class="ct-sprite" :style="sprite(item.tile)" aria-hidden="true"></span><small>{{ ct(`equipment.${item.key}`) }}</small><Icon v-if="state.stats[item.key]" name="checkCircle" size="xs" /></span></div><p class="ct-zone-name">{{ ct(`zones.${zoneOf(state) - 1}`) }} · {{ ct('sealProgress', { n: seals.filter(s => s.cleared).length }) }}</p><div class="ct-seals" :aria-label="ct('sealProgress', { n: seals.filter(s => s.cleared).length })"><span v-for="seal in seals" :key="seal.floor" :class="{ cleared: seal.cleared, current: Math.floor(state.floor / 10) === Math.floor(seal.floor / 10) }"><Icon :name="seal.cleared ? 'checkCircle' : 'lock'" size="xs" />{{ seal.floor + 1 }}F</span></div><div class="ct-floor-track"><span v-for="n in floorTrack" :key="n" :class="{ current: state.floor === n - 1, visited: state.discovered.includes(n - 1) }">{{ String(n).padStart(2, '0') }}</span></div></section>
          <section class="ct-shop" :class="{ available: nearShop(state) }"><div class="ct-section-label"><span>03 / {{ t('tower.shop') }}</span><span class="ct-sprite" :style="sprite(51)" aria-hidden="true"></span></div><p>{{ ct(nearShop(state) ? 'shopHere' : 'shopHint') }}</p><button v-for="offer in offers" :key="offer.kind" :disabled="!canMove || !nearShop(state) || state.stats.gold < offer.cost" @click="buy(offer.kind)"><span class="ct-sprite" :style="sprite(offer.tile)" aria-hidden="true"></span><span>{{ ct(`offers.${offer.kind}`, { n: offer.gain }) }}</span><b>{{ offer.cost }} G</b></button></section>
        </aside>
      </div>
      <footer class="ct-footer"><span :class="{ 'ct-save-error': storageError }"><Icon :name="storageError ? 'exclamationTriangle' : 'checkCircle'" size="sm" />{{ storageError ? t('tower.saveError') : ct('local') }}</span><span>{{ t('tower.score') }} <b>{{ state.score.toLocaleString() }}</b></span><a href="/games/token-tower/classic/LICENSE.txt" target="_blank" rel="noopener noreferrer">Magic-Tower-Qt · MIT <Icon name="externalLink" size="xs" /></a></footer>

      <BaseDialog :show="bookOpen" :title="t('tower.book')" width="wide" @close="bookOpen = false"><p class="ct-dialog-copy">{{ ct('bookHint') }}</p><div class="ct-book"><article v-for="m in floorMonsters" :key="m.id"><span class="ct-sprite ct-enemy" :style="sprite(m.id)" aria-hidden="true"></span><div><h4>{{ tileName(m.id) }}</h4><p>HP {{ m.hp }} · {{ t('tower.attack') }} {{ m.attack }} · {{ t('tower.defense') }} {{ m.defense }}</p><small>{{ t(`tower.abilities.${m.ability}`) }}</small><small>{{ ct('monsterChance', { dodge: m.miss, crit: 100 - m.criticalThreshold }) }}</small></div><strong :class="{ danger: !previewBattle(state, m.id)?.canWin }">−{{ previewBattle(state, m.id)?.damage }}<small>HP</small></strong></article></div><template #footer><button class="ct-button" @click="bookOpen = false">{{ t('tower.close') }}</button></template></BaseDialog>
      <BaseDialog :show="restartPrompt" :title="t('tower.restart')" width="narrow" @close="restartPrompt = false"><p class="ct-dialog-copy">{{ ct('restartHint') }}</p><template #footer><button class="ct-button" @click="restartPrompt = false">{{ t('tower.cancel') }}</button><button class="ct-primary ct-confirm-restart" @click="restart">{{ t('tower.restart') }}</button></template></BaseDialog>
      <BaseDialog :show="Boolean(cursePrompt)" :title="ct('curseTitle')" width="narrow" @close="cursePrompt = null"><p class="ct-dialog-copy">{{ ct(`notices.curse${cursePrompt ? tileAt(state, cursePrompt) : 24}`) }} {{ ct('curseConfirm') }}</p><template #footer><button class="ct-button" @click="cursePrompt = null">{{ t('tower.cancel') }}</button><button class="ct-primary ct-confirm-curse" @click="confirmCurse">{{ ct('faceCurse') }}</button></template></BaseDialog>
    </main>
  </component>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import ClassicTowerBattle from './ClassicTowerBattle.vue'
import { combatFrames, type CombatFrame } from '@/games/token-tower/classic/combat'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAuthStore } from '@/stores/auth'
import { useArcadeStore } from '@/stores/arcade'
import { useArcadeGamepad } from '@/composables/useArcadeGamepad'
import { claimCompletedTokenParkGame, loadTokenParkProgress, saveTokenParkProgress } from '@/composables/useTokenParkReward'
import * as expeditionEngine from '@/games/token-tower/expedition/engine'
import type { Battle, Point, Role, State, Upgrade } from '@/games/token-tower/campaign-engine'

const { battleCount, buyClassic, createClassic, curse, FLOORS, heroSprite, loadClassic, monsterFor, moveClassic, nearShop, neighbors, pauseClassic, previewBattle, same, saveClassic, shopOffers, spriteTile, zoneOf, SIZE, stars, startClassic, tileAt, undoClassic, wall } = expeditionEngine

const { t, locale } = useI18n(), auth = useAuthStore(), records = useArcadeStore()
const ct = (key: string, values: Record<string, string | number> = {}) => t(`tower.expedition.${key}`, values)
const immersive = ref(false), stage = ref<HTMLElement | null>(null)
const gameRoot = ref<HTMLElement | null>(null), boardSize = ref(448), narrow = ref(false)
const panels = ['inspect', 'inventory', 'shop'] as const
const activePanel = ref<typeof panels[number]>('inspect'), detailsOpen = ref(false), compactPortrait = ref(false)
function selectPanel(panel: typeof panels[number]) { detailsOpen.value = activePanel.value !== panel || !detailsOpen.value; activePanel.value = panel }
const combat = ref<{ from: Point; at: Point; tile: number; preview: Battle; hero: number; heroHp: number; displayHp: number; frames: CombatFrame[] } | null>(null)
let resizeObserver: ResizeObserver | undefined
function fitBoard() {
  if (!stage.value || !gameRoot.value) return
  const rootStyle = getComputedStyle(gameRoot.value)
  const width = gameRoot.value.clientWidth - parseFloat(rootStyle.paddingLeft || '0') - parseFloat(rootStyle.paddingRight || '0')
  narrow.value = width < 720
  compactPortrait.value = narrow.value && window.innerHeight <= 690 && window.innerHeight > window.innerWidth
  const style = getComputedStyle(stage.value)
  const availableWidth = narrow.value ? width - 18 : width - 398
  boardSize.value = Math.max(1, Math.floor(Math.min(availableWidth, stage.value.clientHeight - parseFloat(style.paddingTop || '0') - parseFloat(style.paddingBottom || '0'))))
}
function toggleImmersive() { stopPath(); combat.value = null; immersive.value = !immersive.value; void nextTick(observeStage) }
function observeStage() { resizeObserver?.disconnect(); if (stage.value) resizeObserver?.observe(stage.value); if (gameRoot.value) resizeObserver?.observe(gameRoot.value); fitBoard() }
function finishCombat() { combat.value = null; void nextTick(() => stage.value?.focus({ preventScroll: true })) }
function askRestart() { stopPath(); finishCombat(); restartPrompt.value = true }
const bookOpen = ref(false), restartPrompt = ref(false), cursePrompt = ref<Point | null>(null)
const artReady = ref(false), artError = ref(false), storageError = ref(false), showDamage = ref(true)
const selected = ref<Point | null>(null), effect = ref<State['event'] | null>(null)
const colors = ['yellow', 'blue', 'red'] as const, roles: Role[] = ['knight', 'scout']
const statItems = [{ key: 'attack', tile: 31 }, { key: 'defense', tile: 33 }, { key: 'gold', tile: 28 }] as const
const equipment = [{ key: 'book', tile: 35 }, { key: 'sword', tile: 31 }, { key: 'shield', tile: 33 }] as const
const directions = [{ key: 'up', dx: 0, dy: -1, icon: 'chevronUp' }, { key: 'left', dx: -1, dy: 0, icon: 'chevronLeft' }, { key: 'down', dx: 0, dy: 1, icon: 'chevronDown' }, { key: 'right', dx: 1, dy: 0, icon: 'chevronRight' }] as const
let owner = auth.user?.id ?? 0, disposed = false, generation = 0
let saveTimer: ReturnType<typeof setTimeout> | undefined, effectTimer: ReturnType<typeof setTimeout> | undefined
let pathTimer: ReturnType<typeof setTimeout> | undefined
const storageKey = () => `token-tower-100-v1:${owner}`
function readSave() {
  try {
    const current = localStorage.getItem(storageKey())
    if (current !== null) return loadClassic(current) ?? createClassic()
    const legacy = expeditionEngine.migrateLegacy50(localStorage.getItem(`token-tower-50-v1:${owner}`))
    if (legacy) {
      try { localStorage.setItem(storageKey(), saveClassic(legacy)) } catch { storageError.value = true }
      return legacy
    }
    return createClassic()
  } catch { storageError.value = true; return createClassic() }
}
const state = ref<State>(readSave())
void loadTokenParkProgress('tower').then((raw) => {
  if (!raw) return
  const remote = loadClassic(raw)
  if (remote) state.value = remote
})
const floorTitle = computed(() => expeditionEngine.campaign.floorRules[state.value.floor]![locale.value.startsWith('zh') ? 'name' : 'nameEn'])
const floorTrack = computed(() => Array.from({ length: 10 }, (_, i) => (Math.floor(state.value.floor / 10) * 10) + i + 1))
const offers = computed(() => shopOffers(state.value))
const seals = computed(() => expeditionEngine.campaign.floorRules.flatMap((r, floor) => r.guardian ? [{ floor, cleared: Boolean(state.value.removed[`${floor}:${r.guardian.x}:${r.guardian.y}`]) }] : []))
const canMove = computed(() => state.value.phase === 'playing' && artReady.value && !bookOpen.value && !restartPrompt.value && !cursePrompt.value && !combat.value)
const battlePreviews = computed(() => new Map([...new Set(FLOORS[state.value.floor])].filter(tile => monsterFor(tile)).map(tile => [tile, previewBattle(state.value, tile)])))
const cells = computed(() => Array.from({ length: SIZE * SIZE }, (_, index) => { const point = { x: index % SIZE, y: Math.floor(index / SIZE) }, tile = tileAt(state.value, point); return { index, point, tile, preview: battlePreviews.value.get(tile) } }))
const floorMonsters = computed(() => [...new Set(cells.value.map(c => c.tile))].flatMap(tile => { const m = monsterFor(tile); return m ? [m] : [] }).sort((a, b) => a.id - b.id))
const inspectedTile = computed(() => combat.value?.tile ?? (selected.value ? tileAt(state.value, selected.value) : neighbors(state.value.position).map(p => tileAt(state.value, p)).find(tile => monsterFor(tile)) ?? 0))
const inspectedMonster = computed(() => monsterFor(inspectedTile.value))
const inspectedBattle = computed(() => combat.value?.preview ?? battlePreviews.value.get(inspectedTile.value))
const noticeValues = computed(() => { const v = state.value.notice.values ?? {}; return { ...v, ...(v.color ? { color: t(`tower.colors.${v.color}`) } : {}) } })
const sprite = (id: number) => { const tile = spriteTile(id); return { backgroundPosition: `${tile % 16 / 15 * 100}% ${Math.floor(tile / 16) / 7 * 100}%` } }
const adjacent = (p: Point) => Math.abs(p.x - state.value.position.x) + Math.abs(p.y - state.value.position.y) === 1
const tileName = (tile: number) => monsterFor(tile) ? monsterFor(tile)!.name === 'boss' ? ct(`bosses.${Math.floor((tile - 200) / 8)}`) : ct(`monsters.${monsterFor(tile)!.name}`, { zone: ct(`zones.${Math.floor((tile - 200) / 8)}`) }) : wall(tile) ? t('tower.tiles.wall') : ct(`tiles.${tile}`)
function label(tile: number, point: Point) { return same(state.value.position, point) ? t('tower.hero') : `${tileName(tile)}${monsterFor(tile) && state.value.stats.book ? ` · −${battlePreviews.value.get(tile)?.damage} HP` : ''}` }
function hover(point: Point) { if (!combat.value && (monsterFor(tileAt(state.value, point)) || curse(tileAt(state.value, point)))) selected.value = point }
function stopPath() { clearTimeout(pathTimer); pathTimer = undefined }
function move(dx: number, dy: number) {
  stopPath()
  if (!canMove.value) return
  const target = { x: state.value.position.x + dx, y: state.value.position.y + dy }
  if (curse(tileAt(state.value, target))) { selected.value = target; cursePrompt.value = target; return }
  const tile = tileAt(state.value, target), frames = combatFrames(state.value, tile, previewBattle)
  const encounter = frames.length ? { from: { ...state.value.position }, at: target, tile, preview: previewBattle(state.value, tile)!, hero: state.value.role === 'knight' ? 113 : 103, heroHp: state.value.stats.hp, displayHp: state.value.stats.hp, frames } : null
  if (moveClassic(state.value, dx, dy) && encounter) { combat.value = encounter; selected.value = null; activePanel.value = 'inspect'; effect.value = null }
}
function selectCell(point: Point) {
  selected.value = point; stopPath()
  if (!canMove.value || same(point, state.value.position)) return
  if (adjacent(point)) { move(point.x - state.value.position.x, point.y - state.value.position.y); return }
  // Click-to-walk crosses cleared ground only. Battles, doors and stairs require a deliberate step.
  if (tileAt(state.value, point) !== 0) return
  const queue = [{ point: state.value.position, path: [] as Point[] }], seen = new Set([`${state.value.position.x}:${state.value.position.y}`])
  for (const node of queue) {
    if (same(node.point, point)) {
      const route = node.path
      const step = () => {
        if (!canMove.value || !route.length) return
        const next = route.shift()!
        if (!moveClassic(state.value, next.x - state.value.position.x, next.y - state.value.position.y)) return
        if (route.length) pathTimer = setTimeout(step, 85)
      }
      step(); return
    }
    for (const next of neighbors(node.point)) {
      const key = `${next.x}:${next.y}`
      if (!seen.has(key) && tileAt(state.value, next) === 0) { seen.add(key); queue.push({ point: next, path: [...node.path, next] }) }
    }
  }
}
function confirmCurse() { const point = cursePrompt.value; cursePrompt.value = null; if (point && canMove.value && adjacent(point)) moveClassic(state.value, point.x - state.value.position.x, point.y - state.value.position.y, true) }
function start() { if (!artReady.value || bookOpen.value || restartPrompt.value || cursePrompt.value) return; startClassic(state.value); void nextTick(() => stage.value?.focus({ preventScroll: true })) }
function chooseRole(role: Role) { if (state.value.phase === 'ready') state.value = createClassic(role) }
function togglePause() { stopPath(); finishCombat(); if (state.value.phase === 'playing') pauseClassic(state.value); else if (state.value.phase === 'paused') start() }
function undo() { stopPath(); if (artReady.value && !bookOpen.value && !restartPrompt.value && !cursePrompt.value && !combat.value) { undoClassic(state.value); selected.value = null } }
function buy(kind: Upgrade) { stopPath(); if (canMove.value) buyClassic(state.value, kind) }
function openBook() { stopPath(); if (combat.value) return; if (state.value.stats.book) bookOpen.value = true; else state.value.notice = { key: 'bookLocked' } }
function restart() { stopPath(); combat.value = null; state.value = createClassic(state.value.role); selected.value = null; effect.value = null; restartPrompt.value = false; persist() }
function persist() {
  clearTimeout(saveTimer)
  const serialized = saveClassic(state.value)
  try { localStorage.setItem(storageKey(), serialized); storageError.value = false } catch { storageError.value = true }
  void saveTokenParkProgress('tower', serialized)
}
function autoPause() { stopPath(); combat.value = null; pauseClassic(state.value); persist() }
function visibility() { if (document.hidden) autoPause() }
async function loadArt() {
  const attempt = ++generation
  artReady.value = false; artError.value = false
  try {
    await new Promise<void>((resolve, reject) => { const image = new Image(); image.onload = () => resolve(); image.onerror = () => reject(new Error('classic-art')); image.src = '/games/token-tower/classic/sprites.webp' })
    if (!disposed && generation === attempt) artReady.value = true
  } catch { if (!disposed && generation === attempt) artError.value = true }
}
function keydown(event: KeyboardEvent) {
  if (event.ctrlKey || event.metaKey || event.altKey || (event.target instanceof HTMLElement && event.target.closest('input,textarea,select,[contenteditable="true"]'))) return
  if (combat.value) {
    if (event.key === 'Escape' || event.key.toLowerCase() === 'p') { event.preventDefault(); autoPause() }
    else if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); finishCombat() }
    else if (event.key.startsWith('Arrow')) event.preventDefault()
    return
  }
  if (bookOpen.value || restartPrompt.value || cursePrompt.value) return
  const key = event.key.toLowerCase()
  if (key === 'escape') { event.preventDefault(); if (immersive.value) toggleImmersive(); autoPause(); return }
  if (key === 'b') { event.preventDefault(); if (!event.repeat) openBook(); return }
  if (key === 'p') { event.preventDefault(); if (!event.repeat) togglePause(); return }
  if (key === 'z') { event.preventDefault(); if (!event.repeat) undo(); return }
  if ((key === 'enter' || key === ' ') && ['ready', 'paused'].includes(state.value.phase) && !(event.target instanceof HTMLElement && event.target.closest('button,a'))) { event.preventDefault(); start(); return }
  const direction = ({ arrowup: [0, -1], w: [0, -1], arrowleft: [-1, 0], a: [-1, 0], arrowdown: [0, 1], s: [0, 1], arrowright: [1, 0], d: [1, 0] } as Record<string, [number, number]>)[key]
  if (direction && canMove.value) { event.preventDefault(); move(...direction) }
}
const { connected } = useArcadeGamepad(action => {
  if (combat.value) { if (action === 'confirm') finishCombat(); else if (action === 'back' || action === 'menu') autoPause(); return }
  if (restartPrompt.value || cursePrompt.value) return
  if (bookOpen.value) { if (action === 'confirm' || action === 'back') bookOpen.value = false; return }
  if (action === 'menu') togglePause()
  else if (action === 'back') autoPause()
  else if (action === 'confirm' && state.value.phase !== 'playing') start()
  else if (action === 'aux') openBook()
  else { const d = directions.find(d => d.key === action); if (d) move(d.dx, d.dy) }
})
watch(() => [state.value.event.id, state.value.phase, state.value.id], () => {
  clearTimeout(saveTimer); saveTimer = setTimeout(persist, 220)
  if (state.value.phase === 'won' && owner === (auth.user?.id ?? 0)) {
    records.record({ id: state.value.id, game: 'tower', score: state.value.score, won: true })
    const serialized = saveClassic(state.value)
    void saveTokenParkProgress('tower', serialized).then((saved) => { if (saved) return claimCompletedTokenParkGame('tower', { level: state.value.floor + 1 }); return false })
  }
})
watch(() => state.value.event.id, () => {
  const event = state.value.event
  if (!combat.value && ['fight', 'loot', 'curse'].includes(event.kind)) { clearTimeout(effectTimer); effect.value = { ...event, at: { ...event.at } }; effectTimer = setTimeout(() => { effect.value = null }, 350) }
  if (event.kind === 'floor') { stopPath(); selected.value = null }
})
watch(() => auth.user?.id ?? 0, id => { stopPath(); combat.value = null; persist(); owner = id; state.value = readSave(); selected.value = null; effect.value = null; bookOpen.value = false; restartPrompt.value = false; cursePrompt.value = null })
watch(() => nearShop(state.value), near => { if (near) activePanel.value = 'shop' })
onMounted(() => { if (typeof ResizeObserver !== 'undefined') resizeObserver = new ResizeObserver(fitBoard); observeStage(); window.addEventListener('resize', fitBoard); void loadArt(); window.addEventListener('keydown', keydown); window.addEventListener('blur', autoPause); window.addEventListener('pagehide', autoPause); document.addEventListener('visibilitychange', visibility) })
onBeforeUnmount(() => { resizeObserver?.disconnect(); window.removeEventListener('resize', fitBoard); disposed = true; generation++; stopPath(); persist(); clearTimeout(effectTimer); window.removeEventListener('keydown', keydown); window.removeEventListener('blur', autoPause); window.removeEventListener('pagehide', autoPause); document.removeEventListener('visibilitychange', visibility) })
</script>

<style scoped src="@/games/token-tower/classic/classic.css"></style>

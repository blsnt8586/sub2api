<template>
  <component :is="immersive ? 'div' : AppLayout">
    <main :class="['tower-page', { 'tower-immersive': immersive }]">
      <header class="tower-header">
        <div>
          <RouterLink class="tower-back" to="/arcade"><Icon name="arrowLeft" size="sm" />{{ t('park.back') }}</RouterLink>
          <RouterLink class="tower-back" to="/arcade/tower">{{ t('tower.classic.edition') }}</RouterLink>
          <div class="tower-wordmark"><h1>{{ t('tower.title') }}</h1><span>THE LAST TOKEN</span></div>
          <p>{{ t('tower.subtitle') }}</p>
        </div>
        <div class="tower-header-actions">
          <span class="tower-save" :class="{ error: storageError }"><Icon :name="storageError ? 'exclamationTriangle' : 'checkCircle'" size="sm" />{{ t(storageError ? 'tower.saveError' : 'tower.saved') }}</span>
          <button class="tower-button" :aria-pressed="immersive" @click="immersive = !immersive"><Icon :name="immersive ? 'x' : 'arrowsUpDown'" size="sm" />{{ t(immersive ? 'park.exitConsole' : 'park.consoleMode') }}</button>
        </div>
      </header>

      <section class="tower-status" :aria-label="t('tower.hero')">
        <div class="tower-health"><img :src="asset('hero')" alt="" width="48" height="48" /><div><span>{{ t('tower.hp') }} <b>{{ state.stats.hp.toLocaleString() }} <small>/ {{ state.stats.maxHp.toLocaleString() }}</small></b></span><progress :value="state.stats.hp" :max="state.stats.maxHp" :aria-label="t('tower.hp')"></progress></div></div>
        <div class="tower-level" :aria-label="t('tower.level', { n: state.stats.level })"><span>{{ t('tower.level', { n: state.stats.level }) }} <b>{{ state.stats.experience }} / {{ state.stats.nextLevelExperience }} XP</b></span><progress :value="state.stats.experience" :max="state.stats.nextLevelExperience" :aria-label="t('tower.experience')"></progress></div>
        <div class="tower-stat"><img :src="asset('sword')" alt="" /><span>{{ t('tower.attack') }}<strong data-testid="tower-attack">{{ state.stats.attack }}</strong></span></div>
        <div class="tower-stat"><img :src="asset('shield')" alt="" /><span>{{ t('tower.defense') }}<strong>{{ state.stats.defense }}</strong></span></div>
        <div class="tower-stat"><img :src="asset('chest')" alt="" /><span>{{ t('tower.gold') }}<strong>{{ state.stats.gold }}</strong></span></div>
        <div class="tower-keys"><span v-for="color in keyColors" :key="color"><img :src="asset(`key-${color}`)" :alt="t(`tower.colors.${color}`)" />{{ state.stats.keys[color] }}</span></div>
      </section>

      <div class="tower-layout">
        <section class="tower-stage" :aria-label="t('tower.title')">
          <div class="tower-stage-header"><div><span class="tower-floor-number">{{ String(state.floor + 1).padStart(2, '0') }}</span><div><span>{{ t('tower.floor', { n: state.floor + 1, total: floors.length }) }}</span><h2>{{ t(`tower.floors.${floor.id}`) }}</h2></div></div><div class="tower-tools"><button class="tower-tool" :aria-label="t('tower.book')" :title="t('tower.bookShortcut')" @click="openBook"><Icon name="grid" size="sm" /></button><button class="tower-tool" :disabled="!state.history.length || state.phase === 'won'" :aria-label="t('tower.undo')" :title="t('tower.undoShortcut')" @click="undo"><Icon name="arrowLeft" size="sm" /></button><button class="tower-tool tower-pause" :disabled="state.phase === 'ready' || state.phase === 'won'" :aria-label="t(state.phase === 'paused' ? 'tower.resume' : 'tower.pause')" @click="togglePause"><Icon :name="state.phase === 'paused' ? 'play' : 'pause'" size="sm" /></button><button class="tower-tool" :aria-label="t('tower.restart')" @click="restartPrompt = true"><Icon name="refresh" size="sm" /></button></div></div>

          <div ref="stage" class="tower-map-area" :data-theme="floor.theme" tabindex="0" :aria-label="t('tower.boardLabel')">
            <div :key="state.floor" class="tower-map" :class="{ 'tower-enter': state.event.kind === 'floor' }">
              <div class="tower-grid" role="grid" :aria-label="t(`tower.floors.${floor.id}`)">
                <button v-for="cell in cells" :key="cell.index" role="gridcell" class="tower-cell" :class="{ 'tower-wall': cell.tile === '#', 'tower-worn': cell.index % 7 === 0, 'tower-adjacent': adjacent(cell.point) && cell.tile !== '#', 'tower-selected': samePoint(selected, cell.point), 'tower-boss-cell': cell.tile === 'X' }" :tabindex="cell.tile === '#' ? -1 : 0" :disabled="cell.tile === '#' || state.phase !== 'playing' || !artReady" :aria-label="tileLabel(cell.tile, cell.point)" :title="tileLabel(cell.tile, cell.point)" @click="selectCell(cell.point)" @pointerenter="inspect(cell.point)" @focus="inspect(cell.point)">
                  <img v-if="cell.sprite" class="tower-sprite" :class="{ 'tower-creature': cell.monster, 'tower-rune': cell.tile === 'o', 'tower-door': 'RBY'.includes(cell.tile) }" :src="asset(cell.sprite)" alt="" draggable="false" />
                  <img v-else-if="cell.tile === '#' && cell.point.y === 0 && [2, 8].includes(cell.point.x)" class="tower-torch" :src="asset('torch')" alt="" />
                  <span v-if="cell.monster && showDamage" class="tower-cell-damage" :class="{ dangerous: !cell.battle?.canWin, free: cell.battle?.damageTaken === 0 }">{{ cell.battle?.sealed ? t('tower.sealBadge') : damageText(cell.battle?.damageTaken ?? null) }}</span>
                </button>
              </div>
              <div class="tower-player" :class="{ 'tower-player-hit': effect?.kind === 'fight' }" :style="{ transform: `translate(${state.position.x * 100}%, ${state.position.y * 100}%)` }" aria-hidden="true"><img :src="asset('hero')" alt="" /></div>
              <div v-if="effect" :key="effect.id" class="tower-effect" :class="`tower-effect-${effect.kind}`" :style="{ left: `${effect.at.x / SIZE * 100}%`, top: `${effect.at.y / SIZE * 100}%` }" aria-hidden="true"><img v-if="effect.kind === 'fight'" :src="asset('slash')" alt="" /><b v-if="effect.value">{{ effect.kind === 'fight' ? '−' : '+' }}{{ effect.value }}</b></div>
            </div>

            <div v-if="!artReady || state.phase !== 'playing'" class="tower-screen">
              <template v-if="!artReady"><Icon :name="artError ? 'exclamationTriangle' : 'clock'" size="xl" /><h3>{{ t(artError ? 'tower.artError' : 'tower.loading') }}</h3><button v-if="artError" class="tower-primary" @click="loadArtwork">{{ t('tower.retry') }}</button></template>
              <template v-else-if="state.phase === 'ready'"><img class="tower-screen-hero" :src="asset('hero')" alt="" /><span class="tower-eyebrow">A TEN-FLOOR ADVENTURE</span><h3>{{ t('tower.startTitle') }}</h3><p>{{ t('tower.startHint') }}</p><ul><li><img :src="asset('key-yellow')" alt="" />{{ t('tower.ruleKeys') }}</li><li><img :src="asset('sword')" alt="" />{{ t('tower.ruleBattle') }}</li><li><img :src="asset('rune')" alt="" />{{ t('tower.ruleRunes') }}</li></ul><button class="tower-primary tower-start" @click="start">{{ t('tower.start') }}<Icon name="arrowRight" size="sm" /></button><small>{{ t('tower.practice') }}</small></template>
              <template v-else-if="state.phase === 'paused'"><Icon name="pause" size="xl" /><span class="tower-eyebrow">TAKE YOUR TIME</span><h3>{{ t('tower.pausedTitle') }}</h3><p>{{ t('tower.pausedHint') }}</p><button class="tower-primary tower-resume" @click="start">{{ t('tower.resume') }}<Icon name="play" size="sm" /></button><button class="tower-text-button" @click="openBook">{{ t('tower.book') }}</button></template>
              <template v-else><img class="tower-screen-hero" :src="asset('altar')" alt="" /><span class="tower-eyebrow">THE LAST TOKEN IS YOURS</span><h3>{{ t('tower.won') }}</h3><p>{{ t('tower.wonHint') }}</p><div class="tower-result-stars" :aria-label="t('tower.starResult', { n: stars(state) })"><Icon v-for="n in 3" :key="n" name="star" size="lg" :class="{ earned: n <= stars(state) }" /></div><strong class="tower-result-score" data-testid="tower-result">{{ state.stats.score.toLocaleString() }}<small>{{ t('tower.score') }}</small></strong><p>{{ t('tower.resultDetail', { n: battleCount(state), steps: state.steps }) }}</p><button class="tower-primary" @click="restartPrompt = true">{{ t('tower.restart') }}<Icon name="refresh" size="sm" /></button><RouterLink class="tower-text-button" to="/arcade">{{ t('park.back') }}</RouterLink></template>
            </div>
          </div>

          <div class="tower-notice" role="status" aria-live="polite"><span :class="{ warn: state.event.kind === 'blocked' }"></span>{{ t(`tower.notices.${state.notice.key}`, state.notice.values ?? {}) }}</div>
          <div class="tower-control-bar"><div class="tower-dpad" :aria-label="t('tower.move')"><button class="up" :aria-label="t('merge.up')" :disabled="!canMove" @click="move(0,-1)"><Icon name="arrowUp" size="sm" /></button><button class="left" :aria-label="t('merge.left')" :disabled="!canMove" @click="move(-1,0)"><Icon name="arrowLeft" size="sm" /></button><span></span><button class="right" :aria-label="t('merge.right')" :disabled="!canMove" @click="move(1,0)"><Icon name="arrowRight" size="sm" /></button><button class="down" :aria-label="t('merge.down')" :disabled="!canMove" @click="move(0,1)"><Icon name="arrowDown" size="sm" /></button></div><div class="tower-key-help"><p><kbd>W A S D</kbd> {{ t('tower.move') }}</p><span>{{ t('tower.keyHelp') }}</span><small v-if="connected"><Icon name="gamepad" size="xs" />{{ t('park.controllerConnected') }}</small></div><button class="tower-heal" :disabled="!canMove || !state.stats.potions || state.stats.hp >= state.stats.maxHp" @click="heal"><img :src="asset('potion')" alt="" /><span>{{ t('tower.heal') }}<small>+300 HP · E</small></span><b>×{{ state.stats.potions }}</b></button></div>
        </section>

        <aside class="tower-aside">
          <section class="tower-encounter"><div class="tower-section-label"><span>01 / {{ t('tower.preview') }}</span><label><input v-model="showDamage" type="checkbox" />{{ t('tower.damageLabels') }}</label></div><template v-if="battle"><div class="tower-enemy"><img :src="asset(battle.monster.id)" alt="" /><div><h3>{{ t(`tower.monsters.${battle.monster.id}`) }}</h3><span>{{ t(`tower.abilities.${battle.monster.ability}`) }}</span></div></div><div class="tower-enemy-stats"><span>HP<b>{{ battle.monster.hp }}</b></span><span>{{ t('tower.attack') }}<b>{{ battle.monster.attack }}</b></span><span>{{ t('tower.defense') }}<b>{{ battle.monster.defense }}</b></span></div><div class="tower-damage-preview" :class="{ danger: !battle.canWin }"><span>{{ t('tower.expectedDamage') }}</span><strong>{{ battle.sealed ? t('tower.sealBadge') : damageText(battle.damageTaken) }}</strong><small>{{ t(battle.canWin ? 'tower.safe' : 'tower.danger') }}</small></div><p class="tower-ability-hint">{{ t(`tower.abilityHints.${battle.monster.ability}`) }}</p></template><div v-else class="tower-encounter-empty"><img :src="asset('skeleton')" alt="" /><p>{{ t('tower.inspectHint') }}</p></div><button class="tower-book-link" @click="openBook">{{ t('tower.book') }}<span>B<Icon name="chevronRight" size="sm" /></span></button></section>

          <section class="tower-quest"><div class="tower-section-label"><span>02 / {{ t('tower.objective') }}</span><Icon name="flag" size="sm" /></div><h3>{{ t('tower.questTitle') }}</h3><p>{{ t('tower.questHint') }}</p><div class="tower-relics"><span v-for="n in 3" :key="n" :class="{ found: n <= relicCount(state) }"><img :src="asset('rune')" :alt="t('tower.rune', { n })" /></span><strong>{{ relicCount(state) }} <small>/ 3</small></strong></div><div class="tower-milestone" :class="{ done: wardenDefeated }"><Icon :name="wardenDefeated ? 'checkCircle' : 'flag'" size="sm" /><span>{{ t('tower.midBoss') }}</span><b>05F</b></div><div class="tower-milestone" :class="{ done: state.phase === 'won' }"><Icon :name="state.phase === 'won' ? 'checkCircle' : 'flag'" size="sm" /><span>{{ t('tower.finalBoss') }}</span><b>10F</b></div></section>

          <section class="tower-shop" :class="{ available: currentTile === 'C' }"><div class="tower-section-label"><span>03 / {{ t('tower.shop') }}</span><img :src="asset('merchant')" alt="" /></div><p>{{ t(currentTile === 'C' ? 'tower.shopHint' : 'tower.shopLocate') }}</p><button v-for="kind in upgrades" :key="kind" class="tower-shop-upgrade" :disabled="!canMove || currentTile !== 'C' || state.stats.gold < shopCost(state)" @click="buy(kind)"><span><img :src="asset(kind === 'attack' ? 'sword' : kind === 'defense' ? 'shield' : 'potion')" alt="" />{{ t(`tower.upgrades.${kind}`) }}</span><b>{{ shopCost(state) }} T</b></button><small>{{ t('tower.shopInflation') }}</small></section>
        </aside>
      </div>

      <footer class="tower-footer"><span><Icon name="checkCircle" size="sm" />{{ t('tower.practice') }}</span><span>{{ t('tower.score') }} <strong>{{ state.stats.score.toLocaleString() }}</strong> · {{ t('park.best') }} <strong>{{ records.best.tower.toLocaleString() }}</strong></span><a href="https://kenney.nl/assets/tiny-dungeon" target="_blank" rel="noopener noreferrer">Art: Kenney · CC0<Icon name="externalLink" size="xs" /></a></footer>
    </main>

    <BaseDialog :show="bookOpen" :title="t('tower.book')" width="wide" @close="bookOpen = false"><p class="tower-book-intro">{{ t('tower.bookHint') }}</p><div class="tower-book-list"><article v-for="enemy in floorEnemies" :key="enemy.index" :class="['tower-book-enemy', { cleared: enemy.cleared }]"><img :src="asset(enemy.monster.id)" alt="" /><div><h4>{{ t(`tower.monsters.${enemy.monster.id}`) }}<small>{{ t(`tower.abilities.${enemy.monster.ability}`) }}</small></h4><p>HP {{ enemy.monster.hp }} · {{ t('tower.attack') }} {{ enemy.monster.attack }} · {{ t('tower.defense') }} {{ enemy.monster.defense }}</p><small>{{ t(`tower.abilityHints.${enemy.monster.ability}`) }}</small></div><strong :class="{ dangerous: !enemy.preview.canWin }">{{ enemy.cleared ? t('tower.defeated') : enemy.preview.sealed ? t('tower.sealBadge') : damageText(enemy.preview.damageTaken) }}<small v-if="!enemy.cleared">{{ t('tower.expectedDamage') }}</small></strong></article></div><p class="tower-book-footnote">{{ t('tower.battleRule') }}</p><template #footer><button class="tower-button" @click="bookOpen = false">{{ t('tower.close') }}</button></template></BaseDialog>
    <BaseDialog :show="restartPrompt" :title="t('tower.restart')" width="narrow" @close="restartPrompt = false"><p>{{ t('tower.restartHint') }}</p><template #footer><button class="tower-button" @click="restartPrompt = false">{{ t('tower.cancel') }}</button><button class="tower-primary" data-testid="tower-confirm-restart" @click="restart">{{ t('tower.restart') }}</button></template></BaseDialog>
  </component>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { useArcadeStore } from '@/stores/arcade'
import { useAuthStore } from '@/stores/auth'
import { useArcadeGamepad } from '@/composables/useArcadeGamepad'
import { DOORS, SPRITES, type KeyColor } from '@/games/token-tower/data'
import { battleCount, buyUpgrade, createTower, findTile, FLOORS, keyFor, loadTower, monsterFor, moveTower, pauseTower, previewBattle, relicCount, saveTower, shopCost, SIZE, stars, startTower, tileAt, undoTower, usePotion, type Point, type Tile, type TowerState } from '@/games/token-tower/engine'

const { t } = useI18n()
const auth = useAuthStore(), records = useArcadeStore()
const immersive = ref(false), stage = ref<HTMLElement | null>(null)
const bookOpen = ref(false), restartPrompt = ref(false), showDamage = ref(true)
const storageError = ref(false), artReady = ref(false), artError = ref(false)
const selected = ref<Point | null>(null), effect = ref<TowerState['event'] | null>(null)
const keyColors: KeyColor[] = ['red', 'blue', 'yellow']
const upgrades = ['attack', 'defense', 'health'] as const
let owner = auth.user?.id ?? 0, saveTimer: ReturnType<typeof setTimeout> | undefined, effectTimer: ReturnType<typeof setTimeout> | undefined
let artGeneration = 0, disposed = false
const storageKey = () => `token-tower-v2:${owner}`
function readSave() { try { return loadTower(localStorage.getItem(storageKey())) ?? createTower() } catch { storageError.value = true; return createTower() } }
const state = ref<TowerState>(readSave())
const floors = FLOORS
const floor = computed(() => FLOORS[state.value.floor]!)
const currentTile = computed(() => tileAt(state.value))
const canMove = computed(() => state.value.phase === 'playing' && artReady.value && !bookOpen.value && !restartPrompt.value)
const wardenDefeated = computed(() => Boolean(state.value.removed[keyFor(4, findTile(4, 'X'))]))
const cells = computed(() => floor.value.map.flatMap((row, y) => [...row].map((_tile, x) => {
  const point = { x, y }, tile = tileAt(state.value, point), monster = monsterFor(state.value.floor, tile)
  return { point, index: y * SIZE + x, tile, monster, sprite: monster?.id ?? SPRITES[tile], battle: monster ? previewBattle(state.value, monster) : null }
})))
const floorEnemies = computed(() => floor.value.map.flatMap((row, y) => [...row].flatMap((tile, x) => {
  const monster = monsterFor(state.value.floor, tile as Tile)
  return monster ? [{ index: y * SIZE + x, monster, preview: previewBattle(state.value, monster), cleared: Boolean(state.value.removed[keyFor(state.value.floor, { x, y })]) }] : []
})))
const battle = computed(() => {
  const target = selected.value ?? state.value.encounter
  if (target) { const monster = monsterFor(state.value.floor, tileAt(state.value, target)); if (monster) return previewBattle(state.value, monster) }
  const nearby = cells.value.find(cell => cell.monster && adjacent(cell.point))
  return nearby?.battle ?? null
})
const asset = (name: string) => `/games/token-tower/tiles/${name}.png`
const samePoint = (a: Point | null, b: Point) => a?.x === b.x && a?.y === b.y
const adjacent = (p: Point) => Math.abs(p.x - state.value.position.x) + Math.abs(p.y - state.value.position.y) === 1
const damageText = (damage: number | null) => damage === null ? '∞' : damage === 0 ? '0' : `−${damage}`
function tileLabel(tile: Tile, point: Point) {
  if (samePoint(state.value.position, point)) return t('tower.hero')
  const monster = monsterFor(state.value.floor, tile)
  if (monster) { const preview = previewBattle(state.value, monster); return `${t(`tower.monsters.${monster.id}`)} · ${t('tower.expectedDamage')} ${preview.sealed ? t('tower.sealBadge') : damageText(preview.damageTaken)}` }
  return t(`tower.tiles.${DOORS[tile] ? `door${tile}` : tile === '#' ? 'wall' : tile === '.' ? 'floor' : tile === '<' || tile === 'S' ? 'down' : tile === '>' ? 'up' : tile}`)
}
function inspect(point: Point) { if (monsterFor(state.value.floor, tileAt(state.value, point))) selected.value = point }
function selectCell(point: Point) { selected.value = point; if (adjacent(point)) move(point.x - state.value.position.x, point.y - state.value.position.y) }
function move(dx: number, dy: number) { if (!canMove.value) return; if (moveTower(state.value, dx, dy)) selected.value = null }
function start() { if (!artReady.value || bookOpen.value || restartPrompt.value) return; startTower(state.value); void nextTick(() => stage.value?.focus({ preventScroll: true })) }
function togglePause() { if (state.value.phase === 'playing') pauseTower(state.value); else if (state.value.phase === 'paused') start() }
function heal() { if (canMove.value) usePotion(state.value) }
function undo() { if (artReady.value && !bookOpen.value && !restartPrompt.value) { undoTower(state.value); selected.value = null } }
function buy(kind: typeof upgrades[number]) { if (canMove.value) buyUpgrade(state.value, kind) }
function openBook() { bookOpen.value = true }
function restart() { state.value = createTower(); selected.value = null; effect.value = null; restartPrompt.value = false; persist() }
function persist() { clearTimeout(saveTimer); try { localStorage.setItem(storageKey(), saveTower(state.value)); storageError.value = false } catch { storageError.value = true } }
function queueSave() { clearTimeout(saveTimer); saveTimer = setTimeout(persist, 180) }
function record() { if (owner !== (auth.user?.id ?? 0) || state.value.stats.score < 1) return; records.record({ id: state.value.id, game: 'tower', score: state.value.stats.score, won: state.value.phase === 'won' }) }

async function loadArtwork() {
  const generation = ++artGeneration
  artError.value = false
  artReady.value = false
  const names = [...new Set([...Object.values(SPRITES), 'wall', 'wall-carved', 'floor', 'floor-worn', 'hero', 'slime', 'bat', 'skeleton', 'guard', 'mage', 'beast', 'warden', 'demon', 'torch', 'altar', 'slash'])]
  try {
    await Promise.all(names.map(name => new Promise<void>((resolve, reject) => {
      const image = new Image()
      image.onload = () => resolve()
      image.onerror = () => reject(new Error('tower-asset'))
      image.src = asset(name!)
    })))
    if (!disposed && generation === artGeneration) artReady.value = true
  } catch { if (!disposed && generation === artGeneration) artError.value = true }
}
function autoPause() { pauseTower(state.value); persist() }
function visibility() { if (document.hidden) autoPause() }
function keydown(event: KeyboardEvent) {
  if (event.altKey || event.ctrlKey || event.metaKey || (event.target instanceof HTMLElement && event.target.closest('input,textarea,select,[contenteditable="true"]'))) return
  if (bookOpen.value || restartPrompt.value) return
  const key = event.key.toLowerCase()
  if (key === 'escape') { event.preventDefault(); immersive.value = false; autoPause(); return }
  if (key === 'b') { event.preventDefault(); if (!event.repeat) openBook(); return }
  if (key === 'p') { event.preventDefault(); if (!event.repeat) togglePause(); return }
  if (key === 'e') { event.preventDefault(); if (!event.repeat) heal(); return }
  if (key === 'z') { event.preventDefault(); if (!event.repeat) undo(); return }
  if ((key === 'enter' || key === ' ') && ['ready', 'paused'].includes(state.value.phase)) {
    // Native buttons retain their keyboard behavior, including opening the book.
    if (!(event.target instanceof HTMLElement && event.target.closest('button,a'))) { event.preventDefault(); start() }
    return
  }
  const direction = ({ arrowup: [0, -1], w: [0, -1], arrowdown: [0, 1], s: [0, 1], arrowleft: [-1, 0], a: [-1, 0], arrowright: [1, 0], d: [1, 0] } as Record<string, [number, number]>)[key]
  if (direction && canMove.value) { event.preventDefault(); move(...direction) }
}
const { connected } = useArcadeGamepad(action => {
  if (restartPrompt.value) return
  if (bookOpen.value) { if (action === 'back' || action === 'confirm') bookOpen.value = false; return }
  if (action === 'menu') togglePause()
  else if (action === 'back') autoPause()
  else if (action === 'confirm' && state.value.phase !== 'playing') start()
  else if (action === 'aux') heal()
  else { const direction = ({ up: [0, -1], down: [0, 1], left: [-1, 0], right: [1, 0] } as Record<string, [number, number]>)[action]; if (direction) move(...direction) }
})
watch(() => state.value.event.id, () => {
  const event = state.value.event
  if (!['move', 'blocked', 'floor', 'undo'].includes(event.kind)) {
    clearTimeout(effectTimer)
    effect.value = { ...event }
    effectTimer = setTimeout(() => { effect.value = null }, 500)
  }
  if (event.kind === 'floor') selected.value = null
})
watch(() => [state.value.id, state.value.event.id, state.value.phase], () => { queueSave(); record() })
watch(() => auth.user?.id ?? 0, id => { persist(); owner = id; storageError.value = false; state.value = readSave(); selected.value = null; effect.value = null; bookOpen.value = false; restartPrompt.value = false })
onMounted(() => { void loadArtwork(); window.addEventListener('keydown', keydown); window.addEventListener('blur', autoPause); window.addEventListener('pagehide', autoPause); document.addEventListener('visibilitychange', visibility); record() })
onBeforeUnmount(() => { disposed = true; artGeneration++; persist(); record(); clearTimeout(effectTimer); window.removeEventListener('keydown', keydown); window.removeEventListener('blur', autoPause); window.removeEventListener('pagehide', autoPause); document.removeEventListener('visibilitychange', visibility) })
</script>

<style scoped src="@/games/token-tower/tower.css"></style>

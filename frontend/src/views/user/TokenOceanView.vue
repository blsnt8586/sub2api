<template>
  <AppLayout>
    <div :class="['ocean-page', { immersive }]">
      <header class="ocean-header">
        <div>
          <RouterLink to="/arcade" class="ocean-back"
            ><Icon name="arrowLeft" size="sm" />{{ t('park.back') }}</RouterLink
          >
          <h1>{{ t('ocean.title') }}<span> / FEED & GROW</span></h1>
          <p>{{ t('ocean.subtitle') }}</p>
        </div>
        <button
          class="ocean-secondary"
          :aria-pressed="immersive"
          @click="immersive = !immersive"
        >
          <Icon :name="immersive ? 'x' : 'arrowsUpDown'" size="sm" />{{
            t(immersive ? 'park.exitConsole' : 'park.consoleMode')
          }}
        </button>
      </header>
      <section class="ocean-game">
        <div class="ocean-statusbar">
          <div class="ocean-score">
            <span>{{ t('ocean.score') }}</span
            ><strong data-testid="ocean-score">{{
              game.score.toLocaleString()
            }}</strong>
          </div>
          <div class="ocean-growth">
            <div>
              <strong>{{
                t(`fishing.fish.${evolutions[game.level]!.id}`)
              }}</strong
              ><span>{{ t('ocean.stage', { n: game.level + 1 }) }}</span>
            </div>
            <progress
              :value="growthProgress(game)"
              max="1"
              :aria-label="t('ocean.growth')"
            ></progress>
          </div>
          <div class="ocean-best">
            <span>{{ t('ocean.campaign.remaining') }}</span
            ><strong>{{ Math.max(0, Math.ceil(roundFor(game).seconds - game.roundTime)) }}s</strong>
          </div>
        </div>
        <div
          ref="stage"
          class="ocean-stage"
          tabindex="0"
          :aria-label="t('ocean.stageLabel')"
          @pointermove="point"
          @pointerdown="pointDown"
          @pointerup="pointUp"
          @pointercancel="clearPointer"
          @pointerleave="clearPointer"
          @contextmenu.prevent
        >
          <OceanScene
            :key="artKey"
            :state="game"
            :label="t('ocean.sceneLabel')"
            :labels="{
              you: t('ocean.you'),
              eat: t('ocean.eat'),
              danger: t('ocean.danger'),
              hunter: t('ocean.challenge.hunter'),
              tail: t('ocean.challenge.tailTarget'),
              shoal: t('ocean.challenge.shoal'),
              spines: t('ocean.ecology.spines'),
              venom: t('ocean.ecology.venom'),
              ambush: t('ocean.ecology.ambush'),
              relic: t('ocean.exploration.relic'),
              migration: t('ocean.exploration.migration'),
              gentle: t('ocean.exploration.gentle'),
            }"
            @ready="artReady = true"
            @error="artError = true"
          />
          <div class="ocean-top-hud">
            <span class="ocean-scene-caption"
              >{{ t(`ocean.exploration.zones.${game.exploration.zone}`) }} ·
              {{ depthAt(game.player.y) }}m <i></i>
              {{ t('ocean.campaign.round', { n: game.round + 1, total: ROUND_COUNT }) }}
              <span
                class="ocean-pressure"
                :data-pressure="reefPressure(game)"
              >
                {{
                  t(
                    game.challenge?.finale
                      ? 'ocean.ecology.finale'
                      : `ocean.ecology.pressure.${reefPressure(game)}`,
                  )
                }}
              </span></span
            >
            <div class="ocean-hearts" aria-hidden="true">
              <span v-for="i in 3" :key="i" :class="{ lost: i > game.lives }"
                >♥</span
              >
            </div>
            <div v-if="game.frenzy > 0" class="ocean-frenzy">
              {{ t('ocean.frenzy') }} · {{ Math.ceil(game.frenzy) }}s
            </div>
            <button
              class="ocean-pause ocean-sound"
              :aria-label="t(soundEnabled ? 'arcade.mute' : 'arcade.unmute')"
              :aria-pressed="soundEnabled"
              @pointerdown.stop
              @click.stop="toggleSound"
            >
              <Icon :name="soundEnabled ? 'speaker' : 'speakerOff'" size="sm" />
            </button>
            <button
              class="ocean-pause"
              :disabled="game.phase !== 'playing'"
              :aria-label="t(paused ? 'ocean.resume' : 'arcade.pause')"
              @pointerdown.stop
              @click.stop="togglePause"
            >
              <Icon :name="paused ? 'play' : 'pause'" size="sm" />
            </button>
            <button
              v-if="immersive"
              class="ocean-pause"
              :aria-label="t('park.exitConsole')"
              @pointerdown.stop
              @click.stop="immersive = false"
            >
              <Icon name="x" size="sm" />
            </button>
          </div>
          <div
            v-if="
              game.phase === 'playing' && !paused && !game.challenge?.finale
            "
            class="ocean-expedition"
            aria-live="polite"
          >
            <span>{{ voyageObjective }}</span>
            <strong v-if="game.voyage.clearedTime > 0">{{ t('ocean.voyage.cleared') }}</strong>
            <strong v-else-if="game.voyage.weather">{{ t(`ocean.voyage.weather.${game.voyage.weather}`, { seconds: Math.ceil(game.voyage.weatherRemaining) }) }}</strong>
            <strong v-else-if="game.exploration.migration">{{
              t('ocean.exploration.escort', {
                n: game.exploration.migration.caught,
                seconds: Math.ceil(game.exploration.migration.remaining),
              })
            }}</strong>
            <small v-else-if="game.exploration.messageTime > 0">{{
              t(`ocean.exploration.messages.${game.exploration.message}`)
            }}</small>
            <small
              v-else-if="
                game.exploration.discoveryTime > 0 &&
                game.exploration.lastDiscovery
              "
              >{{
                t('ocean.exploration.discovery', {
                  name: speciesName(game.exploration.lastDiscovery),
                })
              }}</small
            >
            <small v-else>{{ t('ocean.exploration.diveHint') }}</small>
          </div>
          <Transition name="feast"
            ><div
              v-if="game.feast > 0 && !paused && game.phase === 'playing'"
              class="ocean-feast"
            >
              <strong>{{ t('ocean.feast', { n: game.feastCount }) }}</strong
              ><span>{{ t('ocean.feastHint') }}</span>
            </div></Transition
          >
          <div
            v-if="
              game.challenge &&
              game.phase === 'playing' &&
              !paused &&
              (challengeHint || game.challenge.finale || game.challenge.shoal)
            "
            class="ocean-event"
            :class="{ finale: game.challenge.finale }"
          >
            <span>{{ challengeHint }}</span>
            <strong v-if="game.challenge.finale">{{
              t('ocean.challenge.bossHealth', { n: game.challenge.bossHealth })
            }}</strong>
            <small v-else-if="game.challenge.shoal">{{
              t('ocean.challenge.shoalProgress', {
                n: game.challenge.shoal.caught,
                total: game.challenge.shoal.total,
                seconds: Math.ceil(game.challenge.shoal.remaining),
              })
            }}</small>
          </div>
          <span class="sr-only">{{ t('ocean.lives', { n: game.lives }) }}</span>
          <div
            v-if="game.phase === 'playing' && !paused && artReady"
            class="ocean-bottom-hud"
          >
            <div class="ocean-chain">
              <strong v-if="game.combo > 1"
                >{{ game.combo }} <small>COMBO</small></strong
              ><span v-else>{{ t('ocean.chainHint') }}</span>
              <small
                v-if="game.frenzy <= 0 && game.frenzyCooldown > 0"
                class="ocean-frenzy-cooldown"
                >{{
                  t('ocean.ecology.cooldown', {
                    n: Math.ceil(game.frenzyCooldown),
                  })
                }}</small
              >
              <div class="ocean-chain-dots">
                <i
                  v-for="i in 8"
                  :key="i"
                  :class="{
                    charged: game.frenzy > 0 || i <= game.frenzyCharge,
                  }"
                ></i>
              </div>
            </div>
            <button
              class="ocean-dash"
              :disabled="game.stamina < 38 || game.dash > 0"
              @pointerdown.stop
              @pointerup.stop
              @click.stop="dash"
            >
              <span>↯ {{ t('ocean.dash') }} <kbd>SPACE</kbd></span
              ><progress
                :value="game.stamina"
                max="100"
                :aria-label="t('ocean.stamina')"
              ></progress>
            </button>
          </div>
          <div
            v-if="!artReady || paused || game.phase !== 'playing'"
            class="ocean-overlay"
          >
            <div v-if="!artReady" class="ocean-dialog">
              <span class="ocean-kicker">INTO THE BLUE</span>
              <h2>{{ t(artError ? 'ocean.artError' : 'ocean.loading') }}</h2>
              <button v-if="artError" class="ocean-primary" @click="retryArt">
                {{ t('fishing.retryArt') }}
              </button>
            </div>
            <div v-else class="ocean-dialog">
              <span class="ocean-kicker">{{
                paused
                  ? 'TAKE A BREATH'
                  : game.phase === 'ready'
                    ? 'SMALL FISH. BIG APPETITE.'
                    : game.phase === 'won'
                      ? 'KING OF THE REEF'
                      : 'ONE MORE BITE?'
              }}</span>
              <h2>
                {{ t(paused ? 'ocean.paused' : `ocean.${game.phase}Title`) }}
              </h2>
              <p>
                {{
                  t(
                    paused
                      ? 'ocean.pauseHint'
                      : game.phase === 'ready'
                        ? 'ocean.challenge.intro'
                        : 'ocean.result',
                    {
                      score: game.score.toLocaleString(),
                      count: game.eaten,
                      combo: game.bestCombo,
                    },
                  )
                }}
              </p>
              <div v-if="game.phase === 'ready' || game.phase === 'cleared'" class="ocean-campaign-card">
                <strong>{{ t('ocean.campaign.chapter', { n: roundFor(game).chapter + 1, total: 10 }) }}</strong>
                <span>{{ t('ocean.campaign.round', { n: game.round + 1, total: ROUND_COUNT }) }}</span>
                <p>{{ game.phase === 'cleared' ? t('ocean.campaign.clearedHint') : voyageObjective }}</p>
                <small>{{ t('ocean.campaign.time', { n: roundFor(game).seconds, boss: roundFor(game).boss ? t('ocean.campaign.boss') : t('ocean.campaign.hunt') }) }}</small>
              </div>
              <div
                v-if="game.phase === 'won' && game.challenge"
                class="ocean-result-stars"
                :aria-label="
                  t('ocean.challenge.stars', { n: challengeStars(game) })
                "
              >
                <span
                  v-for="n in 3"
                  :key="n"
                  :class="{ earned: n <= challengeStars(game) }"
                  >★</span
                ><small>{{
                  t('ocean.challenge.runStats', {
                    seconds: Math.round(game.time),
                    dodges: game.challenge.dodges,
                    bites: game.challenge.tailBites,
                  })
                }}</small>
              </div>
              <small
                v-if="game.phase === 'won' && game.challenge"
                class="ocean-star-hint"
                >{{ t('ocean.challenge.starHint') }}</small
              >
              <button class="ocean-primary" @click="primary">
                <Icon name="play" size="sm" />{{
                  t(
                    paused
                      ? 'ocean.resume'
                      : game.phase === 'ready'
                        ? 'ocean.start'
                        : game.phase === 'cleared'
                          ? 'ocean.campaign.next'
                          : 'ocean.retry',
                  )
                }}<kbd>↵ / A</kbd>
              </button>
              <small v-if="game.phase === 'ready'">{{
                t(checkpointRestored ? 'ocean.campaign.savedHint' : 'ocean.pointerHint')
              }}</small>
            </div>
          </div>
        </div>
        <div class="ocean-evolutions" :aria-label="t('ocean.evolutionPath')">
          <div
            v-for="(f, i) in evolutions"
            :key="f.id"
            :class="{ reached: i <= game.level, current: i === game.level }"
          >
            <img
              :src="`/games/token-fishing/${f.id}-sprite.webp`"
              alt=""
            /><span>{{ t(`fishing.fish.${f.id}`) }}</span
            ><small>{{
              i === game.level ? t('ocean.current') : `${i + 1}`
            }}</small>
          </div>
        </div>
      </section>
      <details class="ocean-field-guide">
        <summary>
          <span>{{ t('ocean.ecology.guideTitle') }}</span
          ><small>{{
            t('ocean.exploration.collection', {
              n: game.exploration.discovered.length,
            })
          }}</small>
        </summary>
        <div class="ocean-species-grid">
          <article
            v-for="id in fieldGuide"
            :key="id"
            :class="{ discovered: game.exploration.discovered.includes(id) }"
          >
            <img :src="spritePath(id)" alt="" loading="lazy" />
            <div>
              <h3>
                {{ speciesName(id) }}
                <small v-if="game.exploration.discovered.includes(id)">{{
                  t('ocean.exploration.seen')
                }}</small>
              </h3>
              <p>{{ t(`ocean.ecology.hints.${id}`) }}</p>
            </div>
          </article>
        </div>
      </details>
      <footer class="ocean-footer">
        <p>
          <span>{{ t('ocean.moveHint') }}</span
          ><span>{{ t('ocean.keyHint') }}</span>
        </p>
        <span>{{ t('arcade.rewardNotice') }}</span>
      </footer>
      <p class="sr-only" aria-live="polite">{{ notice }} {{ challengeHint }}</p>
    </div>
  </AppLayout>
</template>
<script setup lang="ts">
import {
  computed,
  onBeforeUnmount,
  onMounted,
  ref,
  shallowRef,
  triggerRef,
  watch,
} from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import OceanScene from '@/components/games/OceanScene.vue'
import {
  createOcean,
  startOcean,
  tickOcean,
  dashOcean,
  evolutions,
  growthProgress,
} from '@/games/token-ocean/engine'
import { reefPressure } from '@/games/token-ocean/ecology'
import { fieldGuide, spritePath } from '@/games/token-ocean/species'
import { originalSpecies, type Species } from '@/games/token-ocean/species'
import { depthAt } from '@/games/token-ocean/exploration'
import { createOceanSound } from '@/games/token-ocean/sound'
import { challengeStars } from '@/games/token-ocean/challenge'
import { ROUND_COUNT, roundFor } from '@/games/token-ocean/campaign'
import { nextRound } from '@/games/token-ocean/engine'
import { screenToWorld } from '@/games/token-ocean/renderer'
import { readCheckpoint, writeCheckpoint, clearCheckpoint } from '@/games/checkpoints'
import { useArcadeStore } from '@/stores/arcade'
import { useAuthStore } from '@/stores/auth'
import { useArcadeGamepad } from '@/composables/useArcadeGamepad'
import { claimCompletedTokenParkGame, clearTokenParkProgress, loadTokenParkProgress, saveTokenParkProgress } from '@/composables/useTokenParkReward'
const { t } = useI18n(),
  records = useArcadeStore(),
  auth = useAuthStore()
let roundSeed = Date.now()
const game = shallowRef(createOcean(roundSeed)),
  paused = ref(false),
  immersive = ref(false),
  artReady = ref(false),
  artError = ref(false),
  artKey = ref(0),
  stage = ref<HTMLElement>(),
  soundEnabled = ref(false)
const sound = createOceanSound()

function speciesName(id: Species) {
  return t(
    (originalSpecies as readonly string[]).includes(id)
      ? `fishing.fish.${id}`
      : `ocean.ecology.names.${id}`,
  )
}
async function toggleSound() {
  if (soundEnabled.value) {
    sound.mute()
    soundEnabled.value = false
  } else soundEnabled.value = await sound.enable()
}
const held = new Set<string>()
let pointer: { x: number; y: number } | undefined,
  frame = 0,
  lastTime = 0,
  intersection: IntersectionObserver | undefined
let roundId = '',
  owner = auth.user?.id
let entryScore = 0
const checkpointRestored = ref(false)
function restoreCheckpoint() {
  const p = readCheckpoint('ocean', auth.user?.id ?? 0)
  if (!p) return
  roundSeed = p.seed; roundId = p.id; entryScore = p.score
  game.value = createOcean(p.seed, p.level, p.score, p.lives)
  if (p.cleared) game.value.phase = p.level < ROUND_COUNT - 1 ? 'cleared' : 'won'
  checkpointRestored.value = true
}
function persistCheckpoint() {
  const s = game.value
  if (s.phase === 'won') {
    const completion = { version: 1 as const, id: roundId, seed: roundSeed, level: s.round, score: s.score, cleared: true, lives: Math.max(1, s.lives), equipment: { winch: 0, clock: 0, assay: 0 } }
    void saveTokenParkProgress('ocean', JSON.stringify(completion)).then((saved) => { if (saved) return claimCompletedTokenParkGame('ocean', { level: s.round + 1 }); return false }).then((claimed) => { if (claimed) { clearCheckpoint('ocean', auth.user?.id ?? 0); void clearTokenParkProgress('ocean') } })
    return
  }
  const checkpoint = {
    version: 1, id: roundId, seed: roundSeed, level: s.round,
    score: s.phase === 'cleared' ? s.score : entryScore,
    cleared: s.phase === 'cleared', lives: Math.max(1, s.lives),
    equipment: { winch: 0, clock: 0, assay: 0 },
  } as const
  writeCheckpoint('ocean', auth.user?.id ?? 0, checkpoint)
  void saveTokenParkProgress('ocean', JSON.stringify(checkpoint))
}
restoreCheckpoint()
void loadTokenParkProgress('ocean').then((raw) => {
  if (!raw) return
  try {
    writeCheckpoint('ocean', auth.user?.id ?? 0, JSON.parse(raw) as import('@/games/checkpoints').ArcadeCheckpoint)
    restoreCheckpoint()
  } catch { /* Keep the validated local checkpoint when the remote state is malformed. */ }
})
const notice = computed(() =>
  game.value.notice === 'eat' ? '' : t(`ocean.notices.${game.value.notice}`),
)
const voyageObjective = computed(() => {
  const leg = game.value.voyage.legs[game.value.voyage.stage]
  if (!leg) return t('ocean.campaign.final')
  return t(`ocean.voyage.${leg.kind}`, {
    stage: game.value.round + 1,
    totalRounds: ROUND_COUNT,
    zone: t(`ocean.exploration.zones.${leg.zone}`),
    n: leg.caught, total: leg.target,
    direction: game.value.exploration.zone === leg.zone ? '' : game.value.player.y < (leg.zone === 'reef' ? 300 : leg.zone === 'twilight' ? 830 : 1300) ? '↓' : '↑',
  })
})
const challengeHint = computed(() => {
  const c = game.value.challenge
  if (!c) return ''
  const h = c.hunter
  if (h?.phase === 'aim') return t('ocean.challenge.aim')
  if (h?.phase === 'charge') return t('ocean.challenge.charge')
  if (h?.phase === 'recover' && !h.bitten) return t('ocean.challenge.recover')
  if (game.value.frenzy > 0) return t('ocean.frenzyHint')
  if (c.bannerTime > 0 && c.banner !== 'none')
    return t(`ocean.challenge.banners.${c.banner}`)
  if (c.finale) return t('ocean.challenge.finalObjective')
  return ''
})
function clearPointer() {
  pointer = undefined
}
function clearInput() {
  held.clear()
  clearPointer()
  lastTime = 0
}
function point(e: PointerEvent) {
  if (
    e.target instanceof Element &&
    e.target.closest('button, a, .ocean-overlay')
  )
    return
  if (game.value.phase !== 'playing' || paused.value || !artReady.value) return
  if (e.pointerType === 'touch' && !e.buttons) return
  const r = stage.value?.getBoundingClientRect()
  if (!r) return
  pointer = { x: e.clientX - r.left, y: e.clientY - r.top }
}
function pointDown(e: PointerEvent) {
  if (
    e.target instanceof Element &&
    e.target.closest('button, a, .ocean-overlay')
  )
    return
  stage.value?.focus({ preventScroll: true })
  stage.value?.setPointerCapture?.(e.pointerId)
  point(e)
}
function pointUp(e: PointerEvent) {
  if (e.pointerType !== 'mouse') clearPointer()
}
function dash() {
  if (!paused.value && artReady.value) {
    if (dashOcean(game.value)) sound.dash()
    triggerRef(game)
  }
}
function saveRun() {
  if (roundId && owner === auth.user?.id && game.value.score > 0)
    records.record({
      id: roundId,
      game: 'ocean',
      score: game.value.score,
      won: game.value.phase === 'won',
    })
}
function primary() {
  if (!artReady.value || artError.value) return
  if (game.value.phase === 'playing' && !paused.value) return
  if (paused.value) {
    paused.value = false
    clearInput()
  } else {
    if (game.value.phase === 'cleared') {
      roundSeed = Date.now()
      const next = nextRound(game.value, roundSeed)
      if (!next) return
      game.value = next
      entryScore = next.score
    } else if (game.value.phase !== 'ready') {
      saveRun()
      const old = game.value
      if (old.phase === 'over') game.value = createOcean(roundSeed, old.round, entryScore)
      else { roundSeed = Date.now(); entryScore = 0; game.value = createOcean(roundSeed); roundId = '' }
    }
    roundId ||= `ocean-${crypto.randomUUID()}`
    owner = auth.user?.id
    clearInput()
    startOcean(game.value)
    checkpointRestored.value = false
    persistCheckpoint()
    triggerRef(game)
  }
  stage.value?.focus({ preventScroll: true })
}
function retryArt() {
  artReady.value = false
  artError.value = false
  artKey.value++
}
function togglePause() {
  if (game.value.phase !== 'playing' || !artReady.value) return
  paused.value = !paused.value
  clearInput()
  if (!paused.value) stage.value?.focus({ preventScroll: true })
}
function autoPause() {
  if (game.value.phase === 'playing') paused.value = true
  clearInput()
}
function visibility() {
  if (document.hidden) autoPause()
}
const movementKeys = [
  'ArrowLeft',
  'ArrowRight',
  'ArrowUp',
  'ArrowDown',
  'KeyW',
  'KeyA',
  'KeyS',
  'KeyD',
]
function onKeydown(e: KeyboardEvent) {
  if (
    e.target instanceof Element &&
    e.target.closest('input, textarea, select, [contenteditable="true"]')
  )
    return
  if (e.code === 'Escape') {
    immersive.value = false
    autoPause()
    return
  }
  if (
    e.target instanceof Element &&
    e.target.closest('button, a') &&
    ['Space', 'Enter'].includes(e.code)
  )
    return
  if (![...movementKeys, 'Space', 'KeyE', 'KeyP', 'Enter'].includes(e.code))
    return
  e.preventDefault()
  if (movementKeys.includes(e.code)) {
    if (!paused.value) {
      held.add(e.code)
      clearPointer()
    }
    return
  }
  if (e.repeat) return
  if (e.code === 'KeyP') togglePause()
  else if (
    e.code === 'Enter' ||
    (e.code === 'Space' && (paused.value || game.value.phase !== 'playing'))
  )
    primary()
  else dash()
}
function onKeyup(e: KeyboardEvent) {
  held.delete(e.code)
}
useArcadeGamepad((action) => {
  if (action === 'menu') togglePause()
  if (action === 'back') {
    immersive.value = false
    autoPause()
  }
  if (action === 'confirm') {
    if (paused.value || game.value.phase !== 'playing') primary()
    else dash()
  }
  if (action === 'aux') dash()
})
function animate(time: number) {
  const dt = lastTime ? (time - lastTime) / 1000 : 0
  lastTime = time
  if (!paused.value && artReady.value && game.value.phase === 'playing') {
    let x =
      Number(held.has('KeyD') || held.has('ArrowRight')) -
      Number(held.has('KeyA') || held.has('ArrowLeft'))
    let y =
      Number(held.has('KeyS') || held.has('ArrowDown')) -
      Number(held.has('KeyW') || held.has('ArrowUp'))
    const pad =
      document.hasFocus() && !document.hidden
        ? [...(navigator.getGamepads?.() ?? [])].find(
            (p) => p?.connected && p.mapping === 'standard',
          )
        : undefined
    if (pad) {
      const px =
        (Math.abs(pad.axes[0] ?? 0) > 0.2 ? pad.axes[0]! : 0) +
        Number(!!pad.buttons[15]?.pressed) -
        Number(!!pad.buttons[14]?.pressed)
      const py =
        (Math.abs(pad.axes[1] ?? 0) > 0.2 ? pad.axes[1]! : 0) +
        Number(!!pad.buttons[13]?.pressed) -
        Number(!!pad.buttons[12]?.pressed)
      if (px || py) {
        x = px
        y = py
        clearPointer()
      }
    }
    const r = stage.value?.getBoundingClientRect()
    const target =
      pointer && r && !x && !y
        ? screenToWorld(game.value, pointer.x, pointer.y, r.width, r.height)
        : undefined
    const previousEaten = game.value.eaten,
      previousLevel = game.value.level,
      previousLives = game.value.lives
    tickOcean(game.value, dt, { x, y, target })
    if (game.value.lives < previousLives) sound.hurt()
    else if (game.value.level > previousLevel) sound.evolve()
    else if (game.value.eaten > previousEaten) sound.bite(game.value.combo)
    triggerRef(game)
    if (game.value.phase !== 'playing') {
      if (game.value.phase !== 'over') persistCheckpoint()
      saveRun()
      clearInput()
    }
  }
  frame = requestAnimationFrame(animate)
}
watch(
  () => auth.user?.id,
  () => {
    clearInput()
    roundId = ''
    roundSeed = Date.now(); entryScore = 0; checkpointRestored.value = false
    game.value = createOcean(roundSeed)
    restoreCheckpoint()
    paused.value = false
  },
  { flush: 'sync' },
)
onMounted(() => {
  window.addEventListener('keydown', onKeydown)
  window.addEventListener('keyup', onKeyup)
  window.addEventListener('blur', autoPause)
  document.addEventListener('visibilitychange', visibility)
  if (typeof IntersectionObserver !== 'undefined' && stage.value) {
    intersection = new IntersectionObserver((entries) => {
      if (entries.some((e) => !e.isIntersecting)) autoPause()
    })
    intersection.observe(stage.value)
  }
  frame = requestAnimationFrame(animate)
})
onBeforeUnmount(() => {
  saveRun()
  sound.close()
  cancelAnimationFrame(frame)
  intersection?.disconnect()
  window.removeEventListener('keydown', onKeydown)
  window.removeEventListener('keyup', onKeyup)
  window.removeEventListener('blur', autoPause)
  document.removeEventListener('visibilitychange', visibility)
})
</script>
<style scoped>
.ocean-campaign-card { display: grid; gap: 6px; padding: 14px; border: 1px solid #9dcebd44; border-radius: 12px; background: #073a4266; margin: 14px 0; }
.ocean-campaign-card strong { color: #ffe4a8; font-size: 13px; }
.ocean-campaign-card span { font-size: 22px; font-weight: 800; }
.ocean-campaign-card p { margin: 2px 0 !important; font-size: 12px !important; }
.ocean-campaign-card small { color: #c0dfd8; font-size: 11px; }

.ocean-expedition {
  position: absolute;
  left: 24px;
  bottom: 102px;
  z-index: 2;
  max-width: min(420px, calc(100% - 48px));
  display: grid;
  gap: 5px;
  pointer-events: none;
  color: #d2f0e9;
  font-size: 11px;
  line-height: 1.5;
  text-shadow: 0 1px 5px #001e35;
}
.ocean-expedition span {
  font-weight: 700;
}
.ocean-expedition small {
  color: #acd6d5;
  font-size: 10px;
}
.ocean-expedition strong {
  color: #ffe5ab;
  font-size: 11px;
}
.ocean-species-grid h3 small {
  color: #21866b;
  font-size: 9px;
  margin-left: 5px;
}
:global(.dark .ocean-species-grid h3 small) {
  color: #91dabd;
}
@media (max-width: 600px) {
  .ocean-expedition {
    left: 14px;
    bottom: 92px;
    max-width: calc(100% - 28px);
    font-size: 10px;
  }
  .ocean-expedition small,
  .ocean-expedition strong {
    font-size: 9px;
  }
}
@media (max-height: 500px) and (orientation: landscape) {
  .immersive .ocean-expedition {
    left: 18px;
    bottom: 76px;
    max-width: 340px;
  }
  .immersive .ocean-expedition small {
    display: none;
  }
}
.ocean-frenzy-cooldown {
  display: block;
  margin-top: 5px;
  color: #d4e8dc;
  font-size: 10px;
}
.ocean-pressure {
  margin-left: 12px;
  color: #c9ebd0;
}
.ocean-pressure[data-pressure='1'] {
  color: #ffe0a6;
}
.ocean-pressure[data-pressure='2'] {
  color: #ffb5a1;
}
.ocean-field-guide {
  margin-top: 16px;
  border: 1px solid #b6d1cc55;
  border-radius: 16px;
  background: #f5fbf8b0;
  color: #315a5d;
}
.ocean-field-guide summary {
  cursor: pointer;
  padding: 16px 20px;
  font-size: 13px;
}
.ocean-field-guide summary span {
  font-weight: 700;
  margin-right: 12px;
}
.ocean-field-guide summary small {
  color: #658487;
}
.ocean-species-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 14px 24px;
  padding: 4px 20px 20px;
}
.ocean-species-grid article {
  display: flex;
  align-items: center;
  gap: 12px;
}
.ocean-species-grid img {
  width: 72px;
  height: 64px;
  object-fit: contain;
  flex: 0 0 72px;
}
.ocean-species-grid h3 {
  font-size: 13px;
  font-weight: 700;
}
.ocean-species-grid p {
  margin-top: 4px;
  font-size: 11px;
  line-height: 1.65;
  color: #638080;
}
:global(.dark .ocean-field-guide) {
  background: #133136;
  color: #d2e7df;
  border-color: #7da49a33;
}
:global(.dark .ocean-species-grid p),
:global(.dark .ocean-field-guide summary small) {
  color: #9db8b4;
}
.immersive .ocean-field-guide {
  display: none;
}
@media (max-width: 900px) {
  .ocean-species-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
@media (max-width: 520px) {
  .ocean-species-grid {
    grid-template-columns: 1fr;
  }
  .ocean-pressure {
    display: block;
    margin: 4px 0 0;
  }
  .ocean-field-guide summary small {
    display: block;
    margin-top: 6px;
  }
}

.ocean-page {
  max-width: 1280px;
  margin: 0 auto;
  color: #173b43;
}
.ocean-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 22px;
}
.ocean-back {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: #668284;
  margin-bottom: 14px;
}
.ocean-header h1 {
  font-weight: 850;
  font-size: clamp(22px, 2.5vw, 32px);
  letter-spacing: -0.04em;
}
.ocean-header h1 span {
  font:
    10px ui-monospace,
    monospace;
  letter-spacing: 0.12em;
  color: #7a9595;
  margin-left: 10px;
}
.ocean-header p {
  font-size: 13px;
  color: #6c8587;
  margin-top: 6px;
}
.ocean-secondary {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 7px;
  font-size: 12px;
  border: 1px solid #d9e5e3;
  background: #fff;
  border-radius: 10px;
  padding: 10px 14px;
  flex-shrink: 0;
}
.ocean-game {
  overflow: hidden;
  border-radius: 22px;
  background: #fff;
  border: 1px solid #d5e5e1;
  box-shadow: 0 14px 50px #163b4210;
}
.ocean-statusbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24px;
  padding: 14px 24px;
}
.ocean-score,
.ocean-best {
  display: grid;
  gap: 2px;
  min-width: 85px;
}
.ocean-score span,
.ocean-best span {
  font-size: 10px;
  color: #7c9291;
}
.ocean-score strong {
  font:
    800 27px ui-monospace,
    monospace;
  letter-spacing: -0.06em;
}
.ocean-best {
  text-align: right;
}
.ocean-best strong {
  font:
    700 17px ui-monospace,
    monospace;
}
.ocean-growth {
  width: 38%;
  max-width: 340px;
}
.ocean-growth > div {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 7px;
  font-size: 12px;
}
.ocean-growth span {
  color: #7c9291;
  font-size: 10px;
}
progress {
  appearance: none;
  display: block;
  border: 0;
  width: 100%;
  height: 4px;
  border-radius: 10px;
  overflow: hidden;
  background: #e6eeeb;
}
progress::-webkit-progress-bar {
  background: #e6eeeb;
}
progress::-webkit-progress-value {
  background: #43ae99;
  border-radius: 10px;
}
progress::-moz-progress-bar {
  background: #43ae99;
  border-radius: 10px;
}
.ocean-stage {
  position: relative;
  width: 100%;
  aspect-ratio: 16 / 9;
  max-height: 700px;
  background: #076084;
  overflow: hidden;
  touch-action: none;
  cursor: default;
  outline: none;
  outline-offset: -3px;
}
.ocean-stage:focus-visible {
  outline: 2px solid #c5e2d099;
}
.ocean-top-hud {
  position: absolute;
  z-index: 2;
  top: 20px;
  left: 24px;
  right: 24px;
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
  pointer-events: none;
}
.ocean-hearts {
  margin-right: auto;
  display: flex;
  gap: 5px;
  font-size: 19px;
  color: #ffccaf;
  text-shadow: 0 2px 7px #173c4990;
}
.ocean-hearts .lost {
  color: #d3eced36;
}
.ocean-pause {
  width: 44px;
  height: 44px;
  display: grid;
  place-items: center;
  border: 1px solid #d6fff020;
  background: #073e5738;
  color: #e8fff6;
  border-radius: 12px;
  pointer-events: auto;
  backdrop-filter: blur(8px);
}
.ocean-frenzy {
  padding: 8px 18px;
  background: #f8d28ae6;
  color: #674a1b;
  border-radius: 8px;
  font-weight: 800;
  font-size: 13px;
  box-shadow: 0 0 30px #ffe09b55;
}
.ocean-bottom-hud {
  position: absolute;
  bottom: 20px;
  left: 24px;
  right: 24px;
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  pointer-events: none;
  color: #eefff6;
}
.ocean-scene-caption {
  position: absolute;
  left: 0;
  top: 54px;
  font:
    9px ui-monospace,
    monospace;
  letter-spacing: 0.16em;
  color: #ddf5e6a3;
  display: flex;
  align-items: center;
  gap: 8px;
}
.ocean-scene-caption i {
  height: 1px;
  width: 16px;
  background: #d1f5e357;
}
.ocean-feast {
  position: absolute;
  top: 44%;
  left: 0;
  right: 0;
  text-align: center;
  pointer-events: none;
  transform: translateY(-52px);
  text-shadow: 0 4px 18px #063f46;
}
.ocean-feast strong {
  display: block;
  font-weight: 850;
  letter-spacing: -0.03em;
  color: #ffe6ae;
  font-size: clamp(23px, 3vw, 38px);
}
.ocean-feast span {
  font-size: 10px;
  color: #fff0c8;
  letter-spacing: 0.16em;
}
.feast-enter-active,
.feast-leave-active {
  transition:
    opacity 0.2s,
    transform 0.2s;
}
.feast-enter-from {
  opacity: 0;
  transform: translateY(-38px) scale(0.9);
}
.feast-leave-to {
  opacity: 0;
  transform: translateY(-66px);
}
.ocean-event {
  position: absolute;
  top: 70px;
  left: 24%;
  right: 24%;
  display: grid;
  gap: 3px;
  text-align: center;
  pointer-events: none;
  color: #efffdb;
  font-size: 12px;
  font-weight: 700;
  text-shadow: 0 2px 6px #002d43;
}
.ocean-event.finale {
  color: #ffdab7;
}
.ocean-event small {
  font-size: 10px;
  color: #e4ebd3;
}
.ocean-event strong {
  font-size: 11px;
  letter-spacing: 0.08em;
}





.ocean-result-stars {
  margin: 0 0 20px;
  color: #b8d1c33b;
  font-size: 34px;
  letter-spacing: 5px;
}
.ocean-result-stars .earned {
  color: #ffe4a1;
}
.ocean-dialog .ocean-star-hint {
  font-size: 10px;
  line-height: 1.7;
  color: #b9d7c9;
  margin: -8px auto 18px;
  max-width: 360px;
}
.ocean-result-stars small {
  display: block;
  font-size: 11px;
  color: #c0d8ca;
  letter-spacing: 0;
  margin-top: 8px;
}
.ocean-chain {
  text-shadow: 0 2px 8px #173c49;
}
.ocean-chain > strong {
  font:
    800 32px ui-monospace,
    monospace;
}
.ocean-chain small {
  font-size: 12px;
}
.ocean-chain > span {
  font-size: 11px;
}
.ocean-chain-dots {
  display: flex;
  gap: 4px;
  margin-top: 7px;
}
.ocean-chain-dots i {
  width: 15px;
  height: 4px;
  background: #bdf7e852;
  border-radius: 4px;
}
.ocean-chain-dots .charged {
  background: #ffe2a3;
  box-shadow: 0 0 7px #ffdc8d50;
}
.ocean-dash {
  pointer-events: auto;
  width: 135px;
  padding: 12px 14px;
  background: #063d54bb;
  border: 1px solid #d6fff044;
  border-radius: 12px;
  backdrop-filter: blur(8px);
}
.ocean-dash > span {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 13px;
  font-weight: 700;
  margin-bottom: 9px;
}
.ocean-dash kbd {
  font-size: 9px;
  color: #a8d9cf;
}
.ocean-dash:disabled {
  opacity: 0.5;
}
.ocean-dash progress,
.ocean-dash progress::-webkit-progress-bar {
  background: #fff2;
}
.ocean-dash progress::-webkit-progress-value {
  background: #b6e9d6;
}
.ocean-dash progress::-moz-progress-bar {
  background: #b6e9d6;
}
.ocean-overlay {
  position: absolute;
  inset: 0;
  display: grid;
  place-items: center;
  background: linear-gradient(145deg, #07394e8a, #05364eae);
  backdrop-filter: blur(2px);
  cursor: default;
  padding: 24px;
}
.ocean-dialog {
  max-width: 480px;
  text-align: center;
  color: #eafff5;
}
.ocean-kicker {
  color: #9ecdc2;
  font:
    10px ui-monospace,
    monospace;
  letter-spacing: 0.24em;
}
.ocean-dialog h2 {
  font-size: clamp(25px, 3.3vw, 43px);
  font-weight: 850;
  letter-spacing: -0.04em;
  margin: 16px 0 12px;
}
.ocean-dialog p {
  font-size: 13px;
  line-height: 1.9;
  color: #b9ddd5;
  max-width: 380px;
  margin: 0 auto 22px;
  text-wrap: balance;
}
.ocean-rules {
  display: flex;
  justify-content: center;
  gap: 20px;
  margin-bottom: 28px;
  font-size: 11px;
  color: #d4eae0;
}
.ocean-rules span {
  display: grid;
  gap: 8px;
}
.ocean-rules b {
  font-size: 24px;
  font-weight: 500;
  color: #c4edca;
}
.ocean-rules span:nth-child(2) b {
  color: #ffbd9b;
}
.ocean-rules span:nth-child(3) b {
  color: #ffe1a0;
}
.ocean-primary {
  display: inline-flex;
  gap: 12px;
  align-items: center;
  justify-content: center;
  background: #e0f3c4;
  color: #244a3f;
  border-radius: 12px;
  padding: 14px 24px;
  font-size: 14px;
  font-weight: 800;
  box-shadow: 0 6px 24px #001d3d35;
  min-height: 48px;
  transition:
    background 0.15s,
    transform 0.15s;
}
.ocean-primary:hover {
  background: #eeffda;
  transform: translateY(-1px);
}
.ocean-primary kbd {
  font:
    10px ui-monospace,
    monospace;
  opacity: 0.5;
  margin-left: 10px;
}
.ocean-dialog > small {
  display: block;
  font-size: 11px;
  color: #a7cbc4;
  margin-top: 18px;
}
.ocean-evolutions {
  display: grid;
  grid-template-columns: repeat(6, 1fr);
  gap: 8px;
  padding: 14px 20px;
}
.ocean-evolutions > div {
  display: flex;
  align-items: center;
  gap: 8px;
  opacity: 0.38;
  min-width: 0;
}
.ocean-evolutions .reached {
  opacity: 0.65;
}
.ocean-evolutions .current {
  opacity: 1;
}
.ocean-evolutions img {
  object-fit: contain;
  width: 37px;
  height: 30px;
  transform: scaleX(-1);
}
.ocean-evolutions span {
  font-size: 10px;
  white-space: nowrap;
}
.ocean-evolutions small {
  font-size: 9px;
  color: #63918a;
}
.ocean-evolutions .current small {
  color: #29836e;
}
.ocean-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  margin-top: 16px;
  font-size: 10px;
  color: #809591;
}
.ocean-footer p {
  display: flex;
  gap: 18px;
}
.immersive {
  position: fixed;
  inset: 0;
  z-index: 100;
  max-width: none;
  background: #edf4f0;
  padding: 16px clamp(12px, 5vw, 90px);
  overflow: auto;
  display: flex;
  flex-direction: column;
}
.immersive .ocean-header {
  margin-bottom: 12px;
}
.immersive .ocean-back,
.immersive .ocean-header p {
  display: none;
}
.immersive .ocean-header h1 {
  font-size: 20px;
}
.immersive .ocean-game {
  min-height: 0;
  display: flex;
  flex-direction: column;
  flex: 1;
}
.immersive .ocean-stage {
  aspect-ratio: auto;
  flex: 1;
  min-height: 240px;
  max-height: none;
}
.immersive .ocean-footer {
  margin-top: 10px;
}
:global(.dark .ocean-page) {
  color: #e1eee7;
}
:global(.dark .ocean-game),
:global(.dark .ocean-secondary) {
  background: #16292d;
  border-color: #294044;
}
:global(.dark .ocean-header p),
:global(.dark .ocean-back) {
  color: #91aaa6;
}
:global(.dark .ocean-page.immersive) {
  background: #101f24;
}
:global(.dark .ocean-growth progress),
:global(.dark .ocean-growth progress::-webkit-progress-bar) {
  background: #2d4446;
}
button:focus-visible,
a:focus-visible {
  outline: 3px solid #69b4a2;
  outline-offset: 3px;
}
@media (max-width: 850px) {
  .ocean-header h1 span {
    display: none;
  }
  .ocean-evolutions > div {
    flex-direction: column;
    gap: 2px;
  }
  .ocean-evolutions span {
    font-size: 9px;
  }
  .ocean-evolutions small {
    display: none;
  }
  .ocean-footer {
    flex-direction: column;
    gap: 7px;
    align-items: flex-start;
  }
}
@media (max-width: 560px) {
  .ocean-scene-caption {
    display: none;
  }
  .ocean-frenzy {
    position: absolute;
    top: 52px;
    left: 50%;
    transform: translateX(-50%);
    white-space: nowrap;
  }
  .ocean-event {
    top: 104px;
    left: 12%;
    right: 12%;
    font-size: 11px;
  }
  .ocean-event small {
    font-size: 9px;
  }
  .ocean-feast {
    top: 40%;
  }
  .ocean-sound {
    width: 40px;
    height: 44px;
  }
  .ocean-header {
    align-items: flex-start;
    gap: 8px;
    margin-bottom: 16px;
  }
  .ocean-header h1 {
    font-size: 23px;
  }
  .ocean-header p {
    font-size: 11px;
    max-width: 210px;
    line-height: 1.7;
  }
  .ocean-secondary {
    font-size: 0;
    padding: 10px;
  }
  .ocean-header .ocean-back {
    margin-bottom: 10px;
  }
  .ocean-game {
    border-radius: 15px;
  }
  .ocean-stage {
    aspect-ratio: 4 / 5;
    max-height: none;
  }
  .ocean-statusbar {
    padding: 12px 16px;
    gap: 12px;
  }
  .ocean-score,
  .ocean-best {
    min-width: 52px;
  }
  .ocean-score strong {
    font-size: 22px;
  }
  .ocean-growth {
    width: 48%;
  }
  .ocean-growth strong {
    font-size: 11px;
  }
  .ocean-growth span {
    font-size: 9px;
  }
  .ocean-top-hud {
    top: 14px;
    left: 16px;
    right: 16px;
  }
  .ocean-bottom-hud {
    bottom: 16px;
    left: 16px;
    right: 16px;
  }
  .ocean-frenzy {
    font-size: 11px;
    padding: 7px 10px;
  }
  .ocean-dash {
    width: 110px;
  }
  .ocean-dash kbd {
    display: none;
  }
  .ocean-evolutions {
    padding: 10px 8px;
    gap: 1px;
  }
  .ocean-evolutions img {
    width: 30px;
    height: 24px;
  }
  .ocean-evolutions span {
    font-size: 8px;
  }
  .ocean-overlay {
    padding: 18px;
  }
  .ocean-dialog p {
    font-size: 12px;
  }




  .ocean-rules {
    gap: 16px;
    font-size: 10px;
  }
  .ocean-dialog > small {
    font-size: 10px;
  }
  .ocean-footer p {
    gap: 8px;
    flex-wrap: wrap;
  }
}
@media (max-height: 500px) and (orientation: landscape) {
  .immersive {
    padding: 6px 12px;
  }
  .immersive .ocean-header,
  .immersive .ocean-footer,
  .immersive .ocean-evolutions {
    display: none;
  }
  .immersive .ocean-statusbar {
    padding: 6px 16px;
  }
  .immersive .ocean-score strong {
    font-size: 18px;
  }
  .immersive .ocean-stage {
    min-height: 180px;
  }
  .immersive .ocean-dialog h2 {
    font-size: 24px;
    margin: 8px 0;
  }
  .immersive .ocean-dialog p {
    margin-bottom: 8px;
  }



  .immersive .ocean-event {
    top: 12px;
    left: 20%;
    right: 20%;
    font-size: 11px;
  }
  .immersive .ocean-rules {
    margin-bottom: 10px;
  }
  .immersive .ocean-rules b,
  .immersive .ocean-dialog > small {
    display: none;
  }
  .immersive .ocean-primary {
    min-height: 40px;
    padding: 10px 20px;
  }
}
@media (prefers-reduced-motion: reduce) {
  * {
    transition: none !important;
  }
}
</style>

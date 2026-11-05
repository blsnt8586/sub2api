<template>
  <AppLayout>
    <div class="arcade-page">
      <RouterLink class="miner-back" to="/arcade"
        ><Icon name="arrowLeft" size="sm" />{{ t('park.back') }}</RouterLink
      >
      <header class="arcade-heading">
        <div>
          <div class="eyebrow">
            <span class="status-dot"></span> TOKEN PARK
            <span class="practice-badge">{{ t('arcade.practice') }}</span>
          </div>
          <h1>{{ t('arcade.gameTitle') }}<span class="title-dot">.</span></h1>
          <p>{{ t('arcade.subtitle') }}</p>
        </div>
        <div class="best-score">
          <Icon name="star" size="md" />
          <div>
            <span>{{ t('arcade.sessionBest') }}</span
            ><strong>{{ best.toLocaleString() }} <small>T</small></strong>
          </div>
        </div>
      </header>

      <div class="arcade-grid">
        <section class="game-panel" :aria-label="t('arcade.gameTitle')">
          <div class="game-toolbar">
            <div class="game-brand">
              <span class="game-number">01</span>
              <div>
                <h2>{{ t('arcade.gameTitle') }}</h2>
                <span>TOKEN MINER</span>
              </div>
            </div>
            <div class="game-tools">
              <span class="level-chip"
                >{{ t('arcade.level', { n: game.level + 1 }) }} / {{ LEVELS.length }}</span
              >
              <button
                class="tool-button"
                :aria-label="t(soundEnabled ? 'arcade.mute' : 'arcade.unmute')"
                :aria-pressed="soundEnabled"
                @click="toggleSound"
              >
                <Icon
                  :name="soundEnabled ? 'speaker' : 'speakerOff'"
                  size="sm"
                />
              </button>
              <button
                class="tool-button"
                :aria-label="
                  t(game.status === 'paused' ? 'arcade.resume' : 'arcade.pause')
                "
                :disabled="!['playing', 'paused'].includes(game.status)"
                @click="togglePause"
              >
                <Icon
                  :name="game.status === 'paused' ? 'play' : 'pause'"
                  size="sm"
                />
              </button>
            </div>
          </div>

          <div
            ref="gameArea"
            class="game-area"
            tabindex="0"
            :aria-label="t('arcade.canvasLabel')"
            @keydown="onKeydown"
          >
            <div class="game-hud" aria-live="off">
              <div>
                <span>{{ t('arcade.haul') }}</span
                ><strong data-testid="score"
                  >{{ game.score.toLocaleString() }}<small> T</small></strong
                >
              </div>
              <div class="target-hud">
                <span>{{ t('arcade.target') }}</span
                ><strong>{{ target.toLocaleString() }}<small> T</small></strong>
              </div>
              <div :class="['timer-hud', { urgent: game.remaining <= 10 }]">
                <span>{{ t('arcade.remaining') }}</span
                ><strong data-testid="timer"
                  ><Icon name="clock" size="sm" />{{ Math.ceil(game.remaining)
                  }}<small>s</small></strong
                >
              </div>
            </div>
            <canvas
              ref="canvas"
              class="mine-canvas"
              width="960"
              height="600"
              role="img"
              :aria-label="t('arcade.canvasLabel')"
              @click="shoot"
            >
              {{ t('arcade.canvasFallback') }}
            </canvas>

            <div v-if="game.status !== 'playing'" class="game-overlay">
              <div class="overlay-card" :class="{ 'has-upgrades': game.status === 'won' && earnsUpgrade(game.level) }">
                <div class="overlay-emblem">
                  <img
                    src="/games/token-miner/gem.webp"
                    alt=""
                    width="64"
                    height="64"
                  />
                </div>
                <span class="overlay-kicker">{{
                  game.status === 'ready'
                    ? 'READY TO DIG?'
                    : game.status === 'paused'
                      ? 'TAKE A BREATHER'
                      : game.status === 'lost'
                        ? 'ONE MORE TRY?'
                        : 'NICE HAUL!'
                }}</span>
                <h3>{{ overlayTitle }}</h3>
                <p>{{ overlayDescription }}</p>
                <div
                  v-if="['won', 'lost', 'complete'].includes(game.status)"
                  class="result-score"
                >
                  <strong>{{ game.score.toLocaleString() }}</strong
                  ><span>{{ t('arcade.practicePoints') }}</span>
                </div>
                <fieldset v-if="game.status === 'won' && earnsUpgrade(game.level)" class="upgrade-picker">
                  <legend>{{ t('arcade.upgrades.choose') }}</legend>
                  <label v-for="kind in UPGRADE_KINDS" :key="kind" :class="{ selected: selectedUpgrade === kind, capped: game.equipment[kind] >= 3 }">
                    <input v-model="selectedUpgrade" type="radio" name="upgrade" :value="kind" :disabled="game.equipment[kind] >= 3" />
                    <span><strong>{{ t(`arcade.upgrades.${kind}`) }} <small>{{ game.equipment[kind] }}/3</small></strong><small>{{ t(`arcade.upgrades.${kind}Hint`) }}</small></span>
                  </label>
                </fieldset>
                <button
                  class="primary-button overlay-start"
                  :disabled="(!artworkReady && !artworkFailed) || (game.status === 'won' && earnsUpgrade(game.level) && !selectedUpgrade)"
                  data-testid="start"
                  @click="primaryAction"
                >
                  <Icon
                    :name="
                      game.status === 'lost' || game.status === 'complete'
                        ? 'refresh'
                        : 'play'
                    "
                    size="sm"
                  />{{
                    !artworkReady
                      ? t(
                          artworkFailed
                            ? 'arcade.artworkRetry'
                            : 'arcade.artworkLoading',
                        )
                      : primaryLabel
                  }}<Icon name="arrowRight" size="sm" />
                </button>
                <span class="overlay-footnote">{{
                  t('arcade.rewardNotice')
                }}</span>
              </div>
            </div>
          </div>

          <div class="mine-condition">
            <span>{{ t(`arcade.conditions.${game.condition}`) }}</span>
            <small>{{ t('arcade.gear', { winch: game.equipment.winch, clock: game.equipment.clock, assay: game.equipment.assay }) }}</small>
          </div>
          <p class="shaft-objective">{{ LEVELS[game.level]!.gems ? t('arcade.shaftGoal', { value: target, gems: game.gems, required: LEVELS[game.level]!.gems }) : t('arcade.targetValue', { n: target }) }} · {{ t(`arcade.layouts.${LEVELS[game.level]!.layout}`) }}</p>
          <div class="game-progress">
            <div :style="{ width: `${progress}%` }"></div>
          </div>
          <div class="control-bar">
            <div class="control-hint">
              <kbd>Space</kbd><span>{{ t('arcade.launchHint') }}</span
              ><span class="desktop-hint"
                >· <kbd>P</kbd> {{ t('arcade.pause') }}</span
              >
            </div>
            <div class="control-actions">
              <button
                class="discard-button"
                :disabled="
                  game.status !== 'playing' || !game.grabbed || game.bombs === 0
                "
                :aria-label="t('arcade.discardLabel', { n: game.bombs })"
                @click="useBomb"
              >
                <img
                  class="dynamite-art"
                  src="/games/token-miner/dynamite.webp"
                  alt=""
                  width="27"
                  height="27"
                />{{ t('arcade.discard') }}
                <span>×{{ game.bombs }}</span>
              </button>
              <button
                class="primary-button launch-button"
                :disabled="
                  game.status !== 'playing' || game.hook !== 'swinging'
                "
                @click="shoot"
              >
                <Icon name="arrowDown" size="sm" />{{
                  t(
                    game.hook === 'swinging'
                      ? 'arcade.launch'
                      : 'arcade.retrieving',
                  )
                }}
              </button>
            </div>
          </div>
          <p class="game-announcement" role="status" aria-live="polite">
            {{ announcement }}
          </p>
        </section>

        <aside class="arcade-aside">
          <section class="mission-card">
            <div class="section-eyebrow">
              <Icon name="flag" size="sm" />{{ t('arcade.expedition') }}
            </div>
            <h2>{{ t('arcade.missionTitle') }}</h2>
            <p>{{ t('arcade.missionDescription') }}</p>
            <div class="chapter-caption">{{ t(`arcade.levelNames.${Math.floor(game.level / 10)}`) }} · {{ t('arcade.chapter', { n: Math.floor(game.level / 10) + 1 }) }}</div>
            <ol class="mission-list">
              <li
                v-for="level in chapterLevels"
                :key="level.index"
                :class="{
                  current: level.index === game.level,
                  cleared:
                    level.index < game.level ||
                    (level.index === game.level &&
                      ['won', 'complete'].includes(game.status)),
                }"
              >
                <span class="mission-number"
                  ><Icon
                    v-if="
                      level.index < game.level ||
                      (level.index === game.level &&
                        ['won', 'complete'].includes(game.status))
                    "
                    name="check"
                    size="sm"
                  /><template v-else>{{ String(level.index + 1).padStart(2, '0') }}</template></span
                >
                <div>
                  <strong>{{ t('arcade.level', { n: level.index + 1 }) }} · {{ t(`arcade.layouts.${level.layout}`) }}</strong
                  ><span>{{
                    t('arcade.targetValue', { n: level.target })
                  }}</span>
                </div>
                <Icon
                  v-if="level.index === game.level"
                  name="arrowLeft"
                  size="sm"
                  class="mission-arrow"
                />
              </li>
            </ol>
            <div class="run-total">
              <span>{{ t('arcade.total') }}</span
              ><strong
                >{{ game.total.toLocaleString() }} <small>T</small></strong
              >
            </div>
          </section>

          <section class="ore-card">
            <div class="section-eyebrow">{{ t('arcade.fieldGuide') }}</div>
            <h2>{{ t('arcade.knowYourOre') }}</h2>
            <div
              v-for="kind in oreKinds"
              :key="kind.name"
              class="ore-guide-row"
            >
              <img
                class="ore-art"
                :src="`/games/token-miner/${kind.name}.webp`"
                alt=""
                width="42"
                height="42"
              />
              <div>
                <strong>{{ t(`arcade.ores.${kind.name}`) }}</strong
                ><span>{{ t(`arcade.oreHints.${kind.name}`) }}</span>
              </div>
              <b :class="{ 'gem-value': kind.name === 'gem' }"
                >+{{ kind.value }}</b
              >
            </div>
          </section>

          <div class="practice-note">
            <Icon name="informationCircle" size="md" />
            <p>
              <strong>{{ t('arcade.practiceNoteTitle') }}</strong
              >{{ t('arcade.practiceNote') }}
            </p>
          </div>
        </aside>
      </div>

      <section class="how-to-play">
        <div class="how-heading">
          <span class="section-eyebrow">HOW TO PLAY</span>
          <h2>{{ t('arcade.howTitle') }}</h2>
        </div>
        <div v-for="index in [0, 1, 2]" :key="index" class="how-step">
          <span class="how-number">0{{ index + 1 }}</span>
          <div>
            <h3>{{ t(`arcade.steps.${index}.title`) }}</h3>
            <p>{{ t(`arcade.steps.${index}.description`) }}</p>
          </div>
        </div>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import {
  computed,
  watch,
  onBeforeUnmount,
  onMounted,
  ref,
  shallowRef,
  triggerRef,
} from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { readCheckpoint, writeCheckpoint, clearCheckpoint } from '@/games/checkpoints'
import { useArcadeStore } from '@/stores/arcade'
import { useArcadeGamepad } from '@/composables/useArcadeGamepad'
import { claimCompletedTokenParkGame, clearTokenParkProgress, loadTokenParkProgress, saveTokenParkProgress } from '@/composables/useTokenParkReward'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import {
  advance,
  createGame,
  discard,
  launch,
  LEVELS,
  earnsUpgrade,
  nextShaft,
  UPGRADE_KINDS,
  type Upgrade,
  WORLD,
} from '@/games/token-miner/engine'
import { drawGame, loadMinerArtwork } from '@/games/token-miner/renderer'

const { t } = useI18n()
const router = useRouter()
const records = useArcadeStore()
let roundId: string = crypto.randomUUID()
useArcadeGamepad((action) => {
  if (game.value.status === 'won' && earnsUpgrade(game.value.level) && ['up', 'down', 'left', 'right'].includes(action)) {
    const options = UPGRADE_KINDS.filter((k) => game.value.equipment[k] < 3)
    const direction = action === 'up' || action === 'left' ? -1 : 1
    selectedUpgrade.value = options[(options.indexOf(selectedUpgrade.value!) + direction + options.length) % options.length]!
    return
  }
  if (action === 'back') {
    void router.push('/arcade')
    return
  }
  if (action === 'menu') {
    togglePause()
    return
  }
  if (action === 'aux') {
    useBomb()
    return
  }
  if (action === 'confirm') {
    if (game.value.status === 'playing') shoot()
    else primaryAction()
  }
})
const auth = useAuthStore()
let shaftSeed = Date.now()
const game = shallowRef(createGame(shaftSeed))
const restored = ref(false)
function restoreCheckpoint() {
  const p = readCheckpoint('miner', auth.user?.id ?? 0)
  if (!p) return
  roundId = p.id
  shaftSeed = p.seed
  game.value = createGame(p.seed, p.level, p.score, p.equipment)
  if (p.cleared) game.value.status = p.level < LEVELS.length - 1 ? 'won' : 'complete'
  restored.value = true
}
function persistCheckpoint() {
  const s = game.value
  if (s.status === 'complete') {
    const completion = { version: 1 as const, id: roundId, seed: shaftSeed, level: s.level, score: s.total, cleared: true, lives: 3, equipment: { ...s.equipment } }
    void saveTokenParkProgress('miner', JSON.stringify(completion)).then((saved) => { if (saved) return claimCompletedTokenParkGame('miner', { level: s.level + 1 }); return false }).then((claimed) => { if (claimed) { clearCheckpoint('miner', auth.user?.id ?? 0); void clearTokenParkProgress('miner') } })
    return
  }
  const checkpoint = {
    version: 1, id: roundId, seed: shaftSeed, level: s.level,
    score: s.total - (s.status === 'won' ? 0 : s.score), cleared: s.status === 'won', lives: 3,
    equipment: { ...s.equipment },
  } as const
  writeCheckpoint('miner', auth.user?.id ?? 0, checkpoint)
  void saveTokenParkProgress('miner', JSON.stringify(checkpoint))
}
restoreCheckpoint()
void loadTokenParkProgress('miner').then((raw) => {
  if (!raw) return
  try {
    writeCheckpoint('miner', auth.user?.id ?? 0, JSON.parse(raw) as import('@/games/checkpoints').ArcadeCheckpoint)
    restoreCheckpoint()
  } catch { /* Keep the validated local checkpoint when the remote state is malformed. */ }
})
watch(() => auth.user?.id, () => {
  roundId = crypto.randomUUID(); shaftSeed = Date.now()
  game.value = createGame(shaftSeed); restored.value = false; selectedUpgrade.value = null
  restoreCheckpoint()
})
const selectedUpgrade = ref<Upgrade | null>(null)
const canvas = ref<HTMLCanvasElement | null>(null)
const gameArea = ref<HTMLElement | null>(null)
const best = ref(0)
const announcement = ref('')
const soundEnabled = ref(false)
const artworkReady = ref(false)
const artworkFailed = ref(false)
function loadArtwork() {
  artworkFailed.value = false
  void loadMinerArtwork()
    .then(() => {
      artworkReady.value = true
    })
    .catch(() => {
      artworkFailed.value = true
    })
}
const chapterStart = computed(() => Math.floor(game.value.level / 10) * 10)
const chapterLevels = computed(() => LEVELS.slice(chapterStart.value, chapterStart.value + 10).map((level, i) => ({ ...level, index: chapterStart.value + i })))
const target = computed(() => LEVELS[game.value.level]!.target)
const progress = computed(() =>
  Math.min(100, (game.value.score / target.value) * 100),
)
const oreKinds = [
  { name: 'token', value: 100 },
  { name: 'large', value: 250 },
  { name: 'gem', value: 400 },
  { name: 'rock', value: 20 },
]
const overlayTitle = computed(() =>
  t(`arcade.overlays.${game.value.status}.title`),
)
const overlayDescription = computed(() => restored.value && game.value.status === 'ready' ? t('arcade.savedHint') :
  t(`arcade.overlays.${game.value.status}.description`, {
    target: target.value,
    seconds: Math.ceil(game.value.remaining),
  }),
)
const primaryLabel = computed(() =>
  t(
    game.value.status === 'paused'
      ? 'arcade.resume'
      : game.value.status === 'won'
        ? 'arcade.nextLevel'
        : game.value.status === 'lost'
          ? 'arcade.retry'
          : game.value.status === 'complete'
            ? 'arcade.playAgain'
            : 'arcade.start',
  ),
)
let animationFrame = 0
let previousTime = 0
let lastUIUpdate = 0
let context: CanvasRenderingContext2D | null = null
let audio: AudioContext | null = null
let dark = false
let reducedMotion = false
let themeObserver: MutationObserver | null = null
let motionPreference: MediaQueryList | null = null

function focusGame() {
  gameArea.value?.focus({ preventScroll: true })
}
function primaryAction() {
  if (!artworkReady.value) {
    if (artworkFailed.value) loadArtwork()
    return
  }
  const state = game.value
  if (state.status === 'paused') {
    togglePause()
    return
  }
  if (state.status === 'won') {
    if (earnsUpgrade(state.level) && !selectedUpgrade.value) return
    shaftSeed = Date.now()
    const next = nextShaft(state, shaftSeed, earnsUpgrade(state.level) ? selectedUpgrade.value : null)
    if (!next) return
    game.value = next
    selectedUpgrade.value = null
  } else if (state.status === 'lost') {
    game.value = createGame(shaftSeed, state.level, state.total - state.score, state.equipment)
  } else if (state.status === 'complete') {
    shaftSeed = Date.now()
    game.value = createGame(shaftSeed)
    roundId = crypto.randomUUID()
  }
  game.value.status = 'playing'
  restored.value = false
  persistCheckpoint()
  previousTime = 0
  triggerRef(game)
  announcement.value = t('arcade.started', { n: game.value.level + 1 })
  focusGame()
}
function shoot() {
  if (launch(game.value)) {
    triggerRef(game)
    beep(260, 0.05)
  }
  focusGame()
}
function useBomb() {
  if (discard(game.value)) {
    triggerRef(game)
    beep(100, 0.12)
    announcement.value = t('arcade.discarded')
  }
  focusGame()
}
function togglePause() {
  if (game.value.status === 'playing') game.value.status = 'paused'
  else if (game.value.status === 'paused') {
    game.value.status = 'playing'
    previousTime = 0
  } else return
  triggerRef(game)
  announcement.value = t(
    game.value.status === 'paused' ? 'arcade.paused' : 'arcade.resumed',
  )
  focusGame()
}
function onKeydown(event: KeyboardEvent) {
  if (
    (event.target as HTMLElement)?.closest('button, input, select, textarea, a')
  )
    return
  if (
    event.repeat ||
    event.altKey ||
    event.ctrlKey ||
    event.metaKey ||
    event.shiftKey
  )
    return
  if (event.code === 'Space' || event.code === 'ArrowDown') {
    event.preventDefault()
    if (game.value.status === 'ready') primaryAction()
    else shoot()
  } else if (event.code === 'KeyP' || event.code === 'Escape') {
    event.preventDefault()
    if (event.code === 'Escape') void router.push('/arcade')
    else togglePause()
  } else if (event.code === 'KeyE') {
    event.preventDefault()
    useBomb()
  }
}
function onVisibilityChange() {
  if (document.hidden && game.value.status === 'playing') {
    game.value.status = 'paused'
    triggerRef(game)
    announcement.value = t('arcade.autoPaused')
  }
  previousTime = 0
}
function toggleSound() {
  try {
    audio ??= new AudioContext()
    void audio.resume().catch(() => {
      soundEnabled.value = false
    })
    soundEnabled.value = !soundEnabled.value
    beep(560, 0.08)
  } catch {
    soundEnabled.value = false
  }
}
function beep(frequency: number, duration: number) {
  if (!soundEnabled.value || !audio || audio.state !== 'running') return
  const oscillator = audio.createOscillator()
  const gain = audio.createGain()
  oscillator.type = 'sine'
  oscillator.frequency.value = frequency
  gain.gain.setValueAtTime(0.06, audio.currentTime)
  gain.gain.exponentialRampToValueAtTime(0.001, audio.currentTime + duration)
  oscillator.connect(gain)
  gain.connect(audio.destination)
  oscillator.start()
  oscillator.stop(audio.currentTime + duration)
  oscillator.onended = () => {
    oscillator.disconnect()
    gain.disconnect()
  }
}
function frame(time: number) {
  const dt = previousTime ? (time - previousTime) / 1000 : 0
  previousTime = time
  const oldScore = game.value.score
  const oldStatus = game.value.status
  advance(game.value, dt)
  if (game.value.score !== oldScore) {
    best.value = Math.max(best.value, game.value.total)
    beep(650, 0.13)
    announcement.value = t('arcade.collected', {
      n: game.value.score - oldScore,
      total: game.value.score,
    })
  }
  if (game.value.status !== oldStatus) {
    persistCheckpoint()
    if (['won', 'lost', 'complete'].includes(game.value.status)) {
      records.record({
        id: roundId,
        game: 'miner',
        score: game.value.total,
        won: game.value.status === 'complete',
      })
    }
    announcement.value = `${t(`arcade.overlays.${game.value.status}.title`)}. ${t('arcade.result', { n: game.value.score })}`
  }
  if (
    time - lastUIUpdate > 80 ||
    game.value.score !== oldScore ||
    game.value.status !== oldStatus
  ) {
    triggerRef(game)
    lastUIUpdate = time
  }
  if (context && !document.hidden)
    drawGame(context, game.value, { dark, reducedMotion, time: time / 1000 })
  animationFrame = requestAnimationFrame(frame)
}
function onMotionChange(event: MediaQueryListEvent) {
  reducedMotion = event.matches
}
onMounted(() => {
  loadArtwork()
  context = canvas.value?.getContext('2d') ?? null
  if (canvas.value && context) {
    const scale = Math.min(window.devicePixelRatio || 1, 2)
    canvas.value.width = WORLD.width * scale
    canvas.value.height = WORLD.height * scale
    context.scale(scale, scale)
  }
  dark = document.documentElement.classList.contains('dark')
  themeObserver = new MutationObserver(() => {
    dark = document.documentElement.classList.contains('dark')
  })
  themeObserver.observe(document.documentElement, {
    attributes: true,
    attributeFilter: ['class'],
  })
  motionPreference = window.matchMedia('(prefers-reduced-motion: reduce)')
  reducedMotion = motionPreference.matches
  motionPreference.addEventListener('change', onMotionChange)
  document.addEventListener('visibilitychange', onVisibilityChange)
  animationFrame = requestAnimationFrame(frame)
})
onBeforeUnmount(() => {
  if (game.value.score > 0)
    records.record({
      id: roundId,
      game: 'miner',
      score: game.value.total,
      won: game.value.status === 'complete',
    })
  cancelAnimationFrame(animationFrame)
  themeObserver?.disconnect()
  motionPreference?.removeEventListener('change', onMotionChange)
  document.removeEventListener('visibilitychange', onVisibilityChange)
  if (audio) void audio.close().catch(() => {})
})
</script>

<style scoped>
.shaft-objective { padding: 10px 18px; font-size: 12px; color: var(--arcade-ink); background: var(--arcade-soft); margin: 0; }
.chapter-caption { color: var(--arcade-accent); font-size: 12px; font-weight: 700; margin: 12px 0; }
.mine-condition { display: flex; flex-wrap: wrap; justify-content: space-between; gap: 6px 16px; padding: 10px 18px; color: var(--arcade-accent); background: var(--arcade-soft); font-size: 12px; }
.mine-condition small { color: var(--arcade-muted); font-size: 11px; }
.has-upgrades .overlay-emblem, .has-upgrades .overlay-kicker, .has-upgrades .result-score, .has-upgrades .overlay-footnote { display: none; }
.upgrade-picker { display: grid; gap: 6px; margin: 12px 0; text-align: left; }
.upgrade-picker legend { margin-bottom: 6px; font-weight: 700; font-size: 12px; }
.upgrade-picker label { display: flex; gap: 10px; align-items: center; border: 1px solid #b7975d55; border-radius: 8px; padding: 8px 12px; cursor: pointer; }
.upgrade-picker label.selected { border-color: #e7c478; background: #285748; }
.upgrade-picker label.capped { opacity: .45; cursor: default; }
.upgrade-picker label span { display: grid; gap: 2px; }
.upgrade-picker strong { font-size: 12px; }
.upgrade-picker small { font-size: 10px; color: #b8c9bc; }
.upgrade-picker input { accent-color: #edcb81; }

.miner-back {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  min-height: 44px;
  color: #79877e;
  font-size: 12px;
  margin-bottom: 8px;
}
.arcade-page {
  --arcade-accent: #b9651b;
  --arcade-ink: #263d36;
  --arcade-muted: #6a7871;
  --arcade-line: #e3e7df;
  --arcade-surface: #fff;
  --arcade-soft: #f6f7f2;
  max-width: 1480px;
  margin: 0 auto;
  color: var(--arcade-ink);
}
:global(.dark .arcade-page) {
  --arcade-accent: #f1b966;
  --arcade-ink: #e4eee5;
  --arcade-muted: #a1b1aa;
  --arcade-line: #34433c;
  --arcade-surface: #1c2825;
  --arcade-soft: #222f29;
}
.arcade-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24px;
  margin: 6px 0 28px;
}
.eyebrow,
.section-eyebrow {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--arcade-muted);
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 1.8px;
}
.status-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: #5b9274;
}
.practice-badge {
  padding: 4px 8px;
  border: 1px solid var(--arcade-line);
  border-radius: 5px;
  letter-spacing: 1px;
  font-size: 10px;
}
.arcade-heading h1 {
  margin: 12px 0 8px;
  font-size: clamp(28px, 3vw, 38px);
  font-weight: 800;
  letter-spacing: -1.5px;
  line-height: 1.2;
}
.title-dot {
  color: var(--arcade-accent);
  margin-left: 5px;
}
.arcade-heading p {
  font-size: 14px;
  color: var(--arcade-muted);
}
.best-score {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 13px 20px;
  background: var(--arcade-surface);
  border: 1px solid var(--arcade-line);
  border-radius: 12px;
  color: var(--arcade-accent);
  flex-shrink: 0;
}
.best-score span {
  display: block;
  font-size: 11px;
  color: var(--arcade-muted);
  margin-bottom: 3px;
}
.best-score strong {
  font-size: 21px;
  font-variant-numeric: tabular-nums;
}
.best-score small {
  font-size: 12px;
}
.arcade-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 284px;
  gap: 24px;
  align-items: start;
}
.game-panel {
  position: relative;
  min-width: 0;
  border: 1px solid var(--arcade-line);
  background: var(--arcade-surface);
  border-radius: 18px;
  overflow: hidden;
  box-shadow: 0 8px 32px #243a2a06;
}
.game-toolbar {
  min-height: 84px;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  padding: 18px 24px;
  gap: 12px;
}
.game-brand {
  display: flex;
  align-items: center;
  gap: 12px;
}
.game-number {
  display: grid;
  place-items: center;
  width: 38px;
  height: 42px;
  background: var(--arcade-soft);
  color: var(--arcade-accent);
  border-radius: 9px;
  font:
    700 15px ui-monospace,
    monospace;
}
.game-brand h2 {
  font-size: 17px;
  font-weight: 750;
}
.game-brand div > span {
  font:
    10px ui-monospace,
    monospace;
  letter-spacing: 2px;
  color: var(--arcade-muted);
}
.game-tools {
  display: flex;
  align-items: center;
  gap: 8px;
}
.level-chip {
  background: var(--arcade-soft);
  border: 1px solid var(--arcade-line);
  border-radius: 6px;
  padding: 6px 9px;
  margin-right: 4px;
  font-size: 11px;
  white-space: nowrap;
}
.tool-button {
  display: grid;
  place-items: center;
  width: 44px;
  height: 44px;
  border: 1px solid var(--arcade-line);
  border-radius: 8px;
  color: var(--arcade-muted);
}
button {
  cursor: pointer;
  transition:
    background 0.18s,
    transform 0.18s;
  touch-action: manipulation;
}
button:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}
button:focus-visible,
.game-area:focus-visible {
  outline: 3px solid #20bba4;
  outline-offset: -3px;
}
.tool-button:hover:enabled {
  background: var(--arcade-soft);
}
.game-area {
  position: relative;
  background: #49382f;
  aspect-ratio: 8 / 5;
  overflow: hidden;
}
.mine-canvas {
  width: 100%;
  height: 100%;
  display: block;
  cursor: crosshair;
  touch-action: manipulation;
}
.game-hud {
  position: absolute;
  inset: 14px 16px auto;
  display: flex;
  gap: 10px;
  pointer-events: none;
  z-index: 1;
  color: #f9e7c3;
}
:global(.dark .arcade-page .game-hud) {
  color: #f9e7c3;
}
.game-hud > div {
  background: #061f27b3;
  border: 1px solid #cbb28133;
  border-radius: 9px;
  padding: 9px 13px;
  box-shadow: 0 4px 12px #031c2333;
  min-width: 80px;
}
.game-hud span {
  display: block;
  font-size: 11px;
  font-weight: 650;
  margin-bottom: 1px;
  opacity: 0.85;
}
.game-hud strong {
  display: flex;
  align-items: center;
  gap: 4px;
  font:
    800 clamp(16px, 2.2vw, 26px) ui-monospace,
    monospace;
}
.game-hud small {
  font-size: 11px;
}
.timer-hud {
  margin-left: auto;
  text-align: right;
}
.timer-hud strong {
  justify-content: flex-end;
}
.timer-hud.urgent {
  color: #c13f25;
}
:global(.dark .arcade-page .timer-hud.urgent) {
  color: #ffae89;
}
.game-overlay {
  position: absolute;
  inset: 0;
  display: flex;
  justify-content: flex-end;
  align-items: center;
  z-index: 2;
  background: linear-gradient(90deg, transparent 20%, #031b2190);
  padding: 25px;
}
.overlay-card {
  max-height: calc(100% - 16px);
  overflow-y: auto;
  width: min(330px, 100%);
  padding: 25px;
  text-align: left;
  background: #0b2826ed;
  color: #f4eedb;
  border: 1px solid #b7975d66;
  border-radius: 15px;
  box-shadow:
    0 20px 60px #001b2080,
    inset 0 1px 0 #dfc18533;
  backdrop-filter: blur(12px);
}
:global(.dark .arcade-page .overlay-card) {
  background: #0b2826ed;
  color: #f4eedb;
  border-color: #b7975d66;
}
.overlay-emblem {
  width: 57px;
  height: 57px;
  display: grid;
  place-items: center;
  margin-bottom: 12px;
  background: #22554855;
  border: 1px solid #b7975d44;
  border-radius: 14px;
}
.overlay-emblem img {
  width: 43px;
  height: 43px;
  object-fit: contain;
  filter: drop-shadow(0 0 12px #46eacb44);
}
.overlay-kicker {
  color: #edcb81 !important;
  font:
    600 10px ui-monospace,
    monospace;
  letter-spacing: 2px;
  color: var(--arcade-accent);
}
.overlay-card h3 {
  margin: 12px 0 8px;
  font-weight: 800;
  font-size: 26px;
  letter-spacing: -1px;
}
.overlay-card p {
  font-size: 13px;
  line-height: 1.8;
  opacity: 0.8;
}
.overlay-start {
  margin: 22px auto 16px;
  width: 100%;
}
.overlay-footnote {
  display: block;
  font-size: 11px;
  opacity: 0.7;
  line-height: 1.6;
}
.primary-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 9px;
  min-height: 46px;
  padding: 10px 18px;
  border: 1px solid #d0932d;
  border-radius: 8px;
  background: #f1bc52;
  color: #382b14;
  font-weight: 750;
  font-size: 13px;
  box-shadow: 0 3px 0 #c28b2e;
}
.primary-button:hover:enabled {
  background: #ffce6c;
}
.primary-button:active:enabled {
  transform: translateY(2px);
  box-shadow: 0 1px 0 #c28b2e;
}
.result-score {
  display: flex;
  flex-direction: column;
  gap: 2px;
  margin-top: 12px;
}
.result-score strong {
  font:
    800 36px ui-monospace,
    monospace;
  color: var(--arcade-accent);
}
.result-score span {
  font-size: 11px;
  opacity: 0.7;
}
.game-progress {
  height: 4px;
  background: var(--arcade-soft);
}
.game-progress > div {
  height: 100%;
  background: #6ca48b;
  transition: width 0.25s;
}
.control-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 16px 20px;
}
.control-hint {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 7px;
  color: var(--arcade-muted);
  font-size: 11px;
}
.control-hint kbd {
  border: 1px solid var(--arcade-line);
  border-bottom-width: 2px;
  border-radius: 4px;
  padding: 3px 5px;
  background: var(--arcade-soft);
  font:
    10px ui-monospace,
    monospace;
}
.desktop-hint {
  white-space: nowrap;
}
.control-actions {
  display: flex;
  gap: 12px;
  flex-shrink: 0;
}
.discard-button {
  display: flex;
  align-items: center;
  gap: 6px;
  min-height: 46px;
  padding: 8px 12px;
  font-size: 12px;
  border: 1px solid var(--arcade-line);
  border-radius: 8px;
}
.discard-button > span {
  color: var(--arcade-muted);
  font:
    12px ui-monospace,
    monospace;
}
.launch-button {
  min-width: 120px;
}
.game-announcement {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip-path: inset(50%);
  white-space: nowrap;
}
.arcade-aside {
  display: flex;
  flex-direction: column;
  gap: 18px;
}
.mission-card,
.ore-card {
  border: 1px solid var(--arcade-line);
  border-radius: 14px;
  padding: 23px 21px;
  background: var(--arcade-surface);
}
.arcade-aside h2 {
  font-size: 18px;
  font-weight: 750;
  margin: 11px 0 7px;
  letter-spacing: -0.5px;
}
.mission-card > p {
  font-size: 12px;
  color: var(--arcade-muted);
  line-height: 1.8;
}
.mission-list {
  margin-top: 22px;
  display: flex;
  flex-direction: column;
  gap: 18px;
  list-style: none;
}
.mission-list li {
  display: flex;
  align-items: center;
  gap: 12px;
  color: var(--arcade-muted);
}
.mission-number {
  display: grid;
  place-items: center;
  width: 31px;
  height: 34px;
  border: 1px solid var(--arcade-line);
  border-radius: 7px;
  font:
    600 12px ui-monospace,
    monospace;
}
.mission-list strong {
  display: block;
  font-size: 12px;
  font-weight: 650;
}
.mission-list li div > span {
  display: block;
  margin-top: 3px;
  font-size: 11px;
}
.mission-list .current {
  color: var(--arcade-ink);
}
.mission-list .current .mission-number {
  background: #f6e7c5;
  color: #84551c;
  border-color: #ecd5a0;
}
.mission-arrow {
  margin-left: auto;
  color: var(--arcade-accent);
}
.mission-list .cleared .mission-number {
  background: #d8ece0;
  color: #366c50;
}
.run-total {
  display: flex;
  justify-content: space-between;
  align-items: center;
  border-top: 1px dashed var(--arcade-line);
  padding-top: 19px;
  margin-top: 24px;
}
.run-total > span {
  font-size: 11px;
  color: var(--arcade-muted);
}
.run-total strong {
  font:
    700 19px ui-monospace,
    monospace;
}
.run-total small {
  font-size: 11px;
  color: var(--arcade-muted);
}
.ore-guide-row {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-top: 18px;
}
.ore-art {
  width: 42px;
  height: 42px;
  object-fit: contain;
  filter: drop-shadow(0 3px 4px #0002);
}
.dynamite-art {
  width: 25px;
  height: 29px;
  object-fit: contain;
}
.ore-guide-row strong {
  display: block;
  font-size: 12px;
  font-weight: 650;
}
.ore-guide-row div > span {
  display: block;
  color: var(--arcade-muted);
  font-size: 11px;
  margin-top: 3px;
}
.ore-guide-row b {
  margin-left: auto;
  font:
    650 13px ui-monospace,
    monospace;
  color: var(--arcade-accent);
}
.ore-guide-row .gem-value {
  color: #0d9488;
}
:global(.dark .arcade-page .ore-guide-row .gem-value) {
  color: #5eead4;
}
.practice-note {
  display: flex;
  align-items: flex-start;
  gap: 9px;
  padding: 0 5px;
  color: var(--arcade-muted);
}
.practice-note svg {
  flex-shrink: 0;
  margin-top: 2px;
}
.practice-note p {
  font-size: 11px;
  line-height: 1.9;
}
.practice-note strong {
  display: block;
  font-weight: 600;
  color: var(--arcade-ink);
}
.how-to-play {
  display: grid;
  grid-template-columns: 1fr 1.3fr 1.3fr 1.3fr;
  gap: 30px;
  border-top: 1px solid var(--arcade-line);
  margin-top: 30px;
  padding: 26px 0 6px;
}
.how-heading h2 {
  margin-top: 8px;
  font-weight: 750;
  font-size: 17px;
}
.how-step {
  display: flex;
  gap: 12px;
  align-items: flex-start;
}
.how-number {
  font:
    500 23px ui-monospace,
    monospace;
  color: var(--arcade-accent);
  opacity: 0.7;
}
.how-step h3 {
  font-size: 13px;
  font-weight: 700;
  margin: 1px 0 6px;
}
.how-step p {
  font-size: 12px;
  line-height: 1.8;
  color: var(--arcade-muted);
}
@media (min-width: 1600px) {
  .arcade-grid {
    grid-template-columns: minmax(0, 1fr) 310px;
  }
}
@media (max-width: 1200px) {
  .arcade-grid {
    grid-template-columns: minmax(0, 1fr);
  }
  .arcade-aside {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 20px;
  }
  .practice-note {
    grid-column: 1 / -1;
  }
  .mission-list {
    gap: 13px;
  }
  .game-area {
    max-height: 640px;
  }
  .how-to-play {
    gap: 20px;
  }
}
@media (max-width: 640px) {
  .arcade-heading {
    align-items: flex-start;
    gap: 12px;
    margin-bottom: 20px;
  }
  .eyebrow {
    letter-spacing: 1px;
    font-size: 9px;
    gap: 5px;
  }
  .practice-badge {
    font-size: 9px;
    padding: 2px 5px;
  }
  .arcade-heading h1 {
    font-size: 29px;
  }
  .arcade-heading p {
    font-size: 12px;
    max-width: 220px;
    line-height: 1.7;
  }
  .best-score {
    padding: 10px;
    gap: 6px;
  }
  .best-score > svg {
    display: none;
  }
  .best-score span {
    font-size: 10px;
  }
  .best-score strong {
    font-size: 18px;
  }
  .game-toolbar {
    padding: 12px;
    gap: 8px;
    min-height: 72px;
  }
  .game-brand {
    gap: 8px;
  }
  .game-number {
    width: 28px;
    height: 36px;
    font-size: 12px;
  }
  .game-brand h2 {
    font-size: 14px;
  }
  .game-brand div > span {
    font-size: 8px;
    letter-spacing: 1px;
  }
  .game-tools {
    gap: 5px;
  }
  .level-chip {
    font-size: 10px;
    padding: 5px;
    margin-right: 0;
  }
  .tool-button {
    width: 44px;
    height: 44px;
  }
  .game-area {
    aspect-ratio: auto;
  }
  .mine-canvas {
    height: auto;
    aspect-ratio: 8 / 5;
  }
  .game-hud {
    inset: 10px 12px auto;
    gap: 14px;
  }
  .game-hud > div {
    background: #061f27b3;
    border: 1px solid #cbb28133;
    border-radius: 9px;
    padding: 9px 13px;
    box-shadow: 0 4px 12px #031c2333;
    min-width: auto;
  }
  .game-hud strong {
    font-size: 19px;
  }
  .game-hud span {
    font-size: 10px;
  }
  .overlay-card {
    width: 275px;
    padding: 15px 17px;
  }
  .overlay-card h3 {
    font-size: 21px;
    margin-top: 6px;
  }
  .overlay-card p {
    font-size: 11px;
    line-height: 1.6;
  }
  .overlay-start {
    margin: 12px auto 9px;
  }
  .overlay-footnote {
    font-size: 10px;
  }
  .control-bar {
    flex-wrap: wrap;
    padding: 12px;
  }
  .control-hint {
    justify-content: center;
    width: 100%;
    font-size: 10px;
  }
  .desktop-hint {
    display: none;
  }
  .control-actions {
    width: 100%;
  }
  .launch-button {
    flex: 1;
  }
  .discard-button {
    flex: 1;
    justify-content: center;
  }
  .arcade-aside {
    grid-template-columns: 1fr;
    gap: 16px;
  }
  .mission-card,
  .ore-card {
    padding: 20px;
  }
  .mission-list {
    flex-direction: row;
    flex-wrap: wrap;
    gap: 16px;
  }
  .mission-list li {
    flex: 1;
    min-width: 110px;
    gap: 8px;
  }
  .mission-arrow {
    display: none;
  }
  .ore-card {
    display: grid;
    grid-template-columns: 1fr 1fr;
    column-gap: 20px;
  }
  .ore-card .section-eyebrow,
  .ore-card h2 {
    grid-column: 1 / -1;
  }
  .ore-guide-row {
    gap: 9px;
  }
  .ore-guide-row b {
    font-size: 12px;
  }
  .ore-guide-row strong {
    font-size: 11px;
  }
  .ore-guide-row div > span {
    font-size: 10px;
  }
  .how-to-play {
    grid-template-columns: 1fr;
    gap: 18px;
    margin-top: 22px;
  }
  .how-heading {
    margin-bottom: 3px;
  }
  .result-score {
    margin-top: 6px;
  }
  .result-score strong {
    font-size: 25px;
    line-height: 1.2;
  }
  .overlay-emblem {
    display: none;
  }
  .game-overlay {
    position: relative;
    inset: auto;
    padding: 15px;
    justify-content: center;
    background: linear-gradient(#0b302d, #072522);
  }
  .overlay-card {
    width: 100%;
    padding: 5px;
    background: transparent;
    border: 0;
    box-shadow: none;
    backdrop-filter: none;
  }
  :global(.dark .arcade-page .overlay-card) {
    background: transparent;
  }
  .game-hud > div {
    padding: 6px 8px;
  }
  .overlay-footnote {
    font-size: 9px;
  }
}
@media (prefers-reduced-motion: reduce) {
  *,
  *::before,
  *::after {
    transition: none !important;
  }
}
</style>

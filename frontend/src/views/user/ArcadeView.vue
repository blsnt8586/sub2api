<template>
  <component :is="immersive ? 'div' : AppLayout">
    <div
      ref="library"
      :class="['park-library', { 'console-mode': immersive }]"
      @keydown="onKeydown"
    >
      <div class="library-inner">
        <header class="library-header">
          <div class="park-wordmark">
            <span class="park-symbol"><Icon name="gamepad" size="lg" /></span>
            <div>
              <h1>
                TOKEN<span>{{ t('park.wordmark') }}</span>
              </h1>
              <p>YOUR LITTLE PLAYGROUND</p>
            </div>
          </div>
          <div class="library-header-actions">
            <span v-if="connected" class="controller-connected"
              ><span></span>{{ t('park.controllerConnected') }}</span
            ><button
              ref="modeButton"
              class="console-toggle"
              :aria-pressed="immersive"
              @click="toggleMode"
            >
              <Icon :name="immersive ? 'x' : 'arrowsUpDown'" size="sm" />{{
                t(immersive ? 'park.exitConsole' : 'park.consoleMode')
              }}<kbd>{{ immersive ? 'Esc' : 'F' }}</kbd>
            </button>
          </div>
        </header>

        <section class="featured-game" :aria-label="t(selected.title)">
          <img
            v-for="item in games"
            v-show="selected.id === item.id"
            :key="item.id"
            class="featured-art"
            :src="item.cover"
            alt=""
            width="1536"
            height="1024"
            fetchpriority="high"
          />
          <div class="featured-shade"></div>
          <div class="featured-copy">
            <div class="featured-kicker">
              <span></span>{{ t('park.featured')
              }}<span class="kicker-separator">/</span>0{{ selectedIndex + 1 }}
            </div>
            <div class="featured-genres">
              <span>{{ t(selected.genre) }}</span
              ><span>{{ t('park.singlePlayer') }}</span>
            </div>
            <h2>{{ t(selected.title) }}</h2>
            <p>{{ t(selected.description) }}</p>
            <div class="featured-buttons">
              <RouterLink class="play-now" :to="selected.path"
                ><Icon name="play" size="sm" />{{ t('park.play')
                }}<span class="pad-a">A</span></RouterLink
              ><button class="more-button" @click="focusCard">
                <Icon name="grid" size="sm" />{{ t('park.browse') }}
              </button>
            </div>
            <div class="featured-meta">
              <span
                ><Icon name="clock" size="sm" />{{ t(selected.minutes) }}</span
              ><span
                ><Icon name="gamepad" size="sm" />{{ t('park.controls') }}</span
              >
            </div>
          </div>
          <div class="featured-position">
            <span>{{ t('park.collection') }}</span
            ><strong
              >0{{ selectedIndex + 1
              }}<small> / 0{{ games.length }}</small></strong
            >
          </div>
        </section>

        <section v-if="parkStatus?.eligible" class="library-collection" :aria-label="t('park.library')">
          <div class="collection-heading">
            <div>
              <span class="library-eyebrow">PICK YOUR NEXT ADVENTURE</span>
              <h2>
                {{ t('park.library') }}<span>{{ games.length }}</span>
              </h2>
            </div>
            <div
              class="library-tabs"
              role="group"
              :aria-label="t('park.filter')"
            >
              <button
                v-for="(filter, index) in filters"
                :key="filter"
                :ref="
                  (el) => {
                    filterButtons[index] = el as HTMLButtonElement
                  }
                "
                :class="{ active: activeFilter === filter }"
                :aria-pressed="activeFilter === filter"
                @click="setFilter(filter)"
              >
                {{ t(`park.${filter}`) }}
              </button>
            </div>
          </div>
          <div v-if="visibleGames.length" class="game-shelf">
            <button
              v-for="item in visibleGames"
              :key="item.id"
              :ref="(el) => setCardRef(item.id, el)"
              :class="['library-game', { selected: selected.id === item.id }]"
              :tabindex="selected.id === item.id ? 0 : -1"
              :aria-label="`${t(item.title)} — ${t('park.play')}`"
              @focus="selectGame(item.id)"
              @pointerenter="selectGame(item.id)"
              @click="openGame(item.id)"
            >
              <div class="game-cover">
                <img :src="item.cover" alt="" width="720" height="480" /><span
                  class="game-genre"
                  >{{ t(item.genre) }}</span
                ><span class="cover-launch"><Icon name="play" size="sm" /></span
                ><span class="cover-title"
                  >{{ item.coverTitle
                  }}<small>{{ item.coverSubtitle }}</small></span
                >
              </div>
              <div class="game-caption">
                <div>
                  <h3>{{ t(item.title) }}</h3>
                  <span
                    >{{ t(item.minutes) }}<i>·</i
                    >{{ t('park.singlePlayer') }}</span
                  >
                </div>
                <span v-if="records.best[item.id]" class="game-best"
                  ><Icon name="trophy" size="sm" />{{
                    records.best[item.id].toLocaleString()
                  }}</span
                ><Icon v-else name="arrowRight" size="sm" />
              </div>
              <span class="selected-line"></span>
            </button>
          </div>
          <div v-else class="library-empty">
            <Icon name="gamepad" size="xl" />
            <h3>{{ t('park.emptyTitle') }}</h3>
            <p>{{ t('park.emptyDescription') }}</p>
            <button class="console-toggle" @click="setFilter('all')">
              {{ t('park.all') }}<Icon name="arrowRight" size="sm" />
            </button>
          </div>
        </section>

        <section v-else class="park-access-gate" aria-live="polite">
          <Icon name="lock" size="xl" />
          <template v-if="statusLoading">
            <h2>正在加载 TOKEN 乐园</h2>
          </template>
          <template v-else-if="parkStatus">
            <h2>充值后解锁 TOKEN 乐园</h2>
            <p>总充值达到 {{ parkStatus.threshold_usd }} 美元即可解锁游戏。</p>
            <RouterLink class="console-toggle" to="/purchase">去充值 <Icon name="arrowRight" size="sm" /></RouterLink>
          </template>
          <template v-else>
            <h2>加载失败</h2>
            <p>请刷新页面后重试。</p>
          </template>
        </section>

        <TokenParkAdminLeaderboard v-if="auth.isAdmin" class="mt-6" />

        <section v-else-if="parkStatus?.eligible && leaderboard" class="park-leaderboard" aria-label="TOKEN 乐园排行榜">
          <div class="leaderboard-heading">
            <div>
              <span class="library-eyebrow">TOKEN PARK RANKING</span>
              <h2>通关排行榜</h2>
            </div>
            <strong v-if="leaderboard.my_rank">我的排名 #{{ leaderboard.my_rank }}</strong>
          </div>
          <div class="leaderboard-list">
            <div v-for="entry in leaderboard.entries.slice(0, 10)" :key="`${entry.rank}-${entry.display_name}`" class="leaderboard-row" :class="{ mine: entry.is_current_user }">
              <b>#{{ entry.rank }}</b>
              <span>{{ entry.is_current_user ? '我' : entry.display_name }}</span>
              <small>{{ entry.games_completed }} 款通关 · ${{ entry.total_reward_usd.toFixed(2) }}</small>
            </div>
          </div>
        </section>

        <section class="session-strip" :aria-label="t('park.session')">
          <div class="session-intro">
            <span class="session-icon"><Icon name="trophy" size="lg" /></span>
            <div>
              <span class="library-eyebrow">JUST FOR THE JOY OF IT</span>
              <h2>{{ t('park.session') }}</h2>
            </div>
          </div>
          <div class="session-stat">
            <strong>{{ records.runs.length }}</strong
            ><span>{{ t('park.rounds') }}</span>
          </div>
          <div class="session-stat">
            <strong>{{ records.wins }}</strong
            ><span>{{ t('park.wins') }}</span>
          </div>
          <div class="session-stat">
            <strong>{{ records.total.toLocaleString() }}</strong
            ><span>{{ t('park.practiceScore') }}</span>
          </div>
          <div class="session-recent">
            <template v-if="records.recent[0]"
              ><span>{{ t('park.lastPlayed') }}</span
              ><RouterLink :to="gameById(records.recent[0].game).path"
                >{{ t(gameById(records.recent[0].game).title)
                }}<Icon name="arrowRight" size="sm" /></RouterLink></template
            ><template v-else
              ><span>{{ t('park.sessionEmpty') }}</span
              ><strong>{{ t('park.firstAdventure') }}</strong></template
            >
          </div>
        </section>

        <footer class="library-footer">
          <div class="controller-legend">
            <span
              ><kbd class="legend-confirm">A</kbd
              >{{ t('park.selectPlay') }}</span
            ><span><kbd>← →</kbd>{{ t('park.navigate') }}</span
            ><span><kbd>☰</kbd>{{ t('park.consoleMode') }}</span>
          </div>
          <p>{{ t('park.practiceNote') }}</p>
        </footer>
      </div>
    </div>
  </component>
</template>
<script setup lang="ts">
import { computed, nextTick, onMounted, ref, type ComponentPublicInstance } from 'vue'
import { RouterLink, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import TokenParkAdminLeaderboard from '@/components/games/TokenParkAdminLeaderboard.vue'
import { useAuthStore } from '@/stores/auth'
import Icon from '@/components/icons/Icon.vue'
import { games, type GameId } from '@/games/catalog'
import { useArcadeStore } from '@/stores/arcade'
import { useArcadeGamepad } from '@/composables/useArcadeGamepad'
import { getTokenParkLeaderboard, getTokenParkStatus, type TokenParkLeaderboard, type TokenParkStatus } from '@/api/tokenPark'

const { t } = useI18n()
const router = useRouter()
const records = useArcadeStore()
const auth = useAuthStore()
const parkStatus = ref<TokenParkStatus | null>(null)
const leaderboard = ref<TokenParkLeaderboard | null>(null)
const statusLoading = ref(true)
const selectedIndex = ref(0)
const selected = computed(() => games[selectedIndex.value]!)
const immersive = ref(false)
const library = ref<HTMLElement | null>(null)
const modeButton = ref<HTMLButtonElement | null>(null)
const filters = ['all', 'action', 'relax', 'recent'] as const
type Filter = (typeof filters)[number]
const activeFilter = ref<Filter>('all')
const filterButtons = ref<HTMLButtonElement[]>([])
const cards = new Map<GameId, HTMLButtonElement>()
const visibleGames = computed(() =>
  games.filter(
    (game) =>
      activeFilter.value === 'all' ||
      activeFilter.value === game.category ||
      (activeFilter.value === 'recent' &&
        records.runs.some((run) => run.game === game.id)),
  ),
)
onMounted(async () => {
  try {
    parkStatus.value = await getTokenParkStatus()
    if (parkStatus.value.eligible && !auth.isAdmin) leaderboard.value = await getTokenParkLeaderboard()
  } catch (error) {
    console.warn('TOKEN Park access check failed', error)
  } finally {
    statusLoading.value = false
  }
})
function gameById(id: GameId) {
  return games.find((game) => game.id === id)!
}
function setCardRef(
  id: GameId,
  element: Element | ComponentPublicInstance | null,
) {
  if (element instanceof HTMLButtonElement) cards.set(id, element)
  else cards.delete(id)
}
function selectGame(id: GameId) {
  selectedIndex.value = games.findIndex((game) => game.id === id)
}
function focusCard() {
  cards.get(selected.value.id)?.focus({ preventScroll: true })
  cards.get(selected.value.id)?.scrollIntoView?.({
    block: 'nearest',
    inline: 'nearest',
    behavior: 'auto',
  })
}
function setFilter(filter: Filter) {
  activeFilter.value = filter
  if (
    !visibleGames.value.some((game) => game.id === selected.value.id) &&
    visibleGames.value[0]
  )
    selectGame(visibleGames.value[0].id)
}
function navigate(delta: number) {
  const index = visibleGames.value.findIndex(
    (game) => game.id === selected.value.id,
  )
  const item =
    visibleGames.value[
      (index + delta + visibleGames.value.length) % visibleGames.value.length
    ]
  if (item) {
    selectGame(item.id)
    void nextTick(focusCard)
  }
}
function openGame(id = selected.value.id) {
  void router.push(gameById(id).path)
}
function toggleMode() {
  immersive.value = !immersive.value
  void nextTick(focusCard)
}
function onKeydown(event: KeyboardEvent) {
  if (
    event.altKey ||
    event.ctrlKey ||
    event.metaKey ||
    event.shiftKey ||
    event.repeat ||
    (event.target as HTMLElement).closest('input, textarea, select')
  )
    return
  if (event.code === 'ArrowLeft' || event.code === 'ArrowRight') {
    event.preventDefault()
    if ((event.target as HTMLElement).closest('.library-tabs')) {
      const delta = event.code === 'ArrowRight' ? 1 : -1
      const index =
        (filters.indexOf(activeFilter.value) + delta + filters.length) %
        filters.length
      setFilter(filters[index]!)
      filterButtons.value[index]?.focus()
    } else navigate(event.code === 'ArrowRight' ? 1 : -1)
  } else if (event.code === 'ArrowDown') {
    event.preventDefault()
    focusCard()
  } else if (event.code === 'ArrowUp') {
    event.preventDefault()
    filterButtons.value[filters.indexOf(activeFilter.value)]?.focus()
  } else if (event.code === 'KeyF') {
    event.preventDefault()
    toggleMode()
  } else if (event.code === 'Escape' && immersive.value) {
    event.preventDefault()
    toggleMode()
  }
}
const { connected } = useArcadeGamepad((action) => {
  if (action === 'left' || action === 'right')
    navigate(action === 'right' ? 1 : -1)
  else if (action === 'down') focusCard()
  else if (action === 'up')
    filterButtons.value[filters.indexOf(activeFilter.value)]?.focus()
  else if (action === 'confirm') {
    const active = document.activeElement
    if (
      active instanceof HTMLButtonElement &&
      library.value?.contains(active) &&
      !active.classList.contains('library-game')
    )
      active.click()
    else openGame()
  } else if (action === 'menu') toggleMode()
  else if (action === 'back') {
    if (immersive.value) toggleMode()
    else router.back()
  }
})
</script>
<style scoped>
.park-library {
  --lib-bg: #f7f8fb;
  --lib-card: #fff;
  --lib-soft: #eef0f6;
  --lib-text: #242b40;
  --lib-muted: #747c91;
  --lib-line: #e4e7ef;
  max-width: 1480px;
  margin: 0 auto;
  color: var(--lib-text);
}
:global(.dark .park-library),
.park-library.console-mode {
  --lib-bg: #111625;
  --lib-card: #191f30;
  --lib-soft: #21293b;
  --lib-text: #ecf0fc;
  --lib-muted: #9ba8c4;
  --lib-line: #2b354c;
}
.library-inner {
  width: 100%;
}
.park-access-gate {
  min-height: 360px;
  display: grid;
  place-items: center;
  align-content: center;
  gap: 12px;
  padding: 48px 24px;
  text-align: center;
  background: var(--lib-card);
  border: 1px solid var(--lib-line);
  border-radius: 14px;
}
.park-access-gate > svg { color: #8695c3; }
.park-access-gate h2 { font-size: 22px; font-weight: 800; }
.park-access-gate p { max-width: 520px; color: var(--lib-muted); line-height: 1.6; }
.park-leaderboard {
  margin-top: 28px;
  padding: 22px;
  background: var(--lib-card);
  border: 1px solid var(--lib-line);
  border-radius: 14px;
}
.leaderboard-heading { display: flex; justify-content: space-between; align-items: end; gap: 16px; }
.leaderboard-heading h2 { margin-top: 4px; font-size: 22px; font-weight: 800; }
.leaderboard-heading strong { color: #b27d27; font-size: 13px; }
.leaderboard-list { display: grid; gap: 8px; margin-top: 16px; }
.leaderboard-row { display: grid; grid-template-columns: 52px minmax(0, 1fr) auto; align-items: center; gap: 12px; min-height: 42px; padding: 8px 10px; border-bottom: 1px solid var(--lib-line); }
.leaderboard-row.mine { background: color-mix(in srgb, #e8c36b 18%, transparent); border-radius: 8px; }
.leaderboard-row b { color: var(--lib-muted); }
.leaderboard-row span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-weight: 700; }
.leaderboard-row small { color: var(--lib-muted); }
.library-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
  margin: 1px 0 26px;
}
.park-wordmark {
  display: flex;
  align-items: center;
  gap: 12px;
}
.park-symbol {
  display: grid;
  place-items: center;
  width: 47px;
  height: 47px;
  color: #8695c3;
  background: var(--lib-card);
  border: 1px solid var(--lib-line);
  border-radius: 14px;
}
.park-wordmark h1 {
  font-size: 21px;
  font-weight: 850;
  letter-spacing: 1px;
}
.park-wordmark h1 span {
  font-weight: 500;
  letter-spacing: -0.5px;
  margin-left: 7px;
}
.park-wordmark p {
  margin-top: 3px;
  font:
    500 8px ui-monospace,
    monospace;
  letter-spacing: 2.6px;
  color: var(--lib-muted);
}
.library-header-actions {
  display: flex;
  align-items: center;
  gap: 18px;
}
.console-toggle {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 9px;
  border: 1px solid var(--lib-line);
  border-radius: 9px;
  padding: 9px 13px;
  min-height: 44px;
  background: var(--lib-card);
  font-size: 11px;
  font-weight: 600;
}
.console-toggle kbd {
  font:
    10px ui-monospace,
    monospace;
  border: 1px solid var(--lib-line);
  border-radius: 4px;
  padding: 1px 4px;
  color: var(--lib-muted);
}
.controller-connected {
  display: flex;
  gap: 7px;
  align-items: center;
  color: var(--lib-muted);
  font-size: 11px;
}
.controller-connected > span {
  width: 5px;
  height: 5px;
  background: #88c5a9;
  border-radius: 50%;
}
.featured-game {
  position: relative;
  overflow: hidden;
  min-height: 375px;
  border-radius: 20px;
  isolation: isolate;
  background: #0d2530;
  box-shadow: 0 18px 40px #101f3617;
}
.featured-art {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  object-fit: cover;
  object-position: 60% 53%;
  z-index: -3;
}
.featured-shade {
  position: absolute;
  inset: 0;
  z-index: -2;
  background:
    linear-gradient(
      90deg,
      #0c1829fa 0%,
      #0e1b2af0 27%,
      #0d1a29a6 48%,
      #0e1b2a08 83%
    ),
    linear-gradient(0deg, #0b17286e, transparent 60%);
}
.featured-copy {
  max-width: 660px;
  container-type: inline-size;
  padding: 35px 42px;
  color: #f5f4f9;
}
.featured-kicker {
  display: flex;
  align-items: center;
  gap: 8px;
  color: #d7dded;
  font-size: 10px;
  font-weight: 650;
  letter-spacing: 1.8px;
}
.featured-kicker > span:first-child {
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: #9dceba;
}
.kicker-separator {
  margin: 0 5px;
  color: #818fa9;
}
.featured-genres {
  display: flex;
  gap: 8px;
  margin: 27px 0 10px;
}
.featured-genres span {
  color: #c3cee2;
  font-size: 10px;
  border: 1px solid #ffffff26;
  background: #ffffff08;
  border-radius: 4px;
  padding: 3px 7px;
}
.featured-copy h2 {
  font-size: clamp(16px, 9.4cqi, 56px);
  font-weight: 800;
  letter-spacing: -1.6px;
  line-height: 1.2;
  white-space: nowrap;
}
.featured-copy > p {
  max-width: 355px;
  font-size: 12px;
  color: #c2cde0;
  line-height: 1.9;
  margin-top: 13px;
}
.featured-buttons {
  display: flex;
  gap: 12px;
  align-items: center;
  margin-top: 24px;
}
.play-now {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
  min-height: 45px;
  padding: 11px 17px;
  background: #e7edf6;
  color: #1c2b40;
  border: 1px solid #ffffff45;
  border-radius: 8px;
  font-size: 12px;
  font-weight: 750;
  box-shadow: 0 4px 15px #0014243b;
}
.pad-a {
  display: grid;
  place-items: center;
  width: 17px;
  height: 17px;
  border: 1px solid #2333475c;
  border-radius: 50%;
  margin-left: 7px;
  font:
    700 9px ui-monospace,
    monospace;
  opacity: 0.7;
}
.more-button {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  min-height: 44px;
  color: #cbd7ed;
  font-size: 11px;
  padding: 8px 10px;
  border-radius: 7px;
}
.more-button:hover {
  background: #ffffff12;
}
.featured-meta {
  display: flex;
  align-items: center;
  gap: 20px;
  margin-top: 23px;
  font-size: 10px;
  color: #aabbd3;
}
.featured-meta > span {
  display: flex;
  gap: 6px;
  align-items: center;
}
.featured-meta svg {
  width: 13px;
  height: 13px;
}
.featured-position {
  position: absolute;
  right: 28px;
  bottom: 25px;
  color: #d0dbee;
  text-align: right;
}
.featured-position > span {
  display: block;
  font:
    500 9px ui-monospace,
    monospace;
  letter-spacing: 2px;
  margin-bottom: 4px;
}
.featured-position strong {
  font-size: 24px;
  font-weight: 400;
  font-variant-numeric: tabular-nums;
}
.featured-position small {
  color: #acbcd5;
  font-size: 12px;
}
.collection-heading {
  display: flex;
  align-items: end;
  justify-content: space-between;
  gap: 20px;
  margin: 33px 0 18px;
}
.library-eyebrow {
  font:
    500 8px ui-monospace,
    monospace;
  letter-spacing: 1.8px;
  color: var(--lib-muted);
}
.collection-heading h2 {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 21px;
  font-weight: 750;
  letter-spacing: -0.5px;
  margin-top: 5px;
}
.collection-heading h2 > span {
  font:
    500 10px ui-monospace,
    monospace;
  border: 1px solid var(--lib-line);
  border-radius: 5px;
  padding: 2px 6px;
  color: var(--lib-muted);
}
.library-tabs {
  display: flex;
  gap: 5px;
  background: var(--lib-soft);
  padding: 4px;
  border-radius: 9px;
}
.library-tabs button {
  min-height: 36px;
  padding: 6px 15px;
  color: var(--lib-muted);
  font-size: 11px;
  font-weight: 600;
  border-radius: 6px;
}
.library-tabs button.active {
  background: var(--lib-card);
  color: var(--lib-text);
  box-shadow: 0 2px 5px #1928460d;
}
.game-shelf {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 23px;
  padding: 5px;
  margin: -5px;
}
.library-game {
  display: block;
  min-width: 0;
  position: relative;
  text-align: left;
  background: var(--lib-card);
  border: 1px solid var(--lib-line);
  border-radius: 13px;
  transition:
    transform 0.18s,
    box-shadow 0.18s;
}
.library-game.selected {
  border-color: #90a4d1;
  box-shadow:
    0 0 0 3px #a9bce038,
    0 12px 30px #19284610;
}
.library-game:hover {
  transform: translateY(-3px);
}
.game-cover {
  position: relative;
  aspect-ratio: 1.9 / 1;
  border-radius: 12px 12px 0 0;
  overflow: hidden;
  background: #1e2442;
  isolation: isolate;
}
.game-cover::after {
  content: '';
  position: absolute;
  inset: 0;
  z-index: -1;
  background:
    linear-gradient(90deg, #0b1b308f, transparent 60%),
    linear-gradient(0deg, #0a162bad, transparent 75%);
}
.game-cover img {
  position: absolute;
  width: 100%;
  height: 100%;
  object-fit: cover;
  z-index: -2;
}
.game-genre {
  position: absolute;
  top: 12px;
  left: 14px;
  font-size: 9px;
  padding: 4px 8px;
  border-radius: 5px;
  border: 1px solid #ffffff30;
  color: #f2effc;
  background: #0e1b3180;
  backdrop-filter: blur(8px);
}
.cover-launch {
  position: absolute;
  right: 14px;
  bottom: 16px;
  display: grid;
  place-items: center;
  width: 32px;
  height: 32px;
  color: #fff;
  border: 1px solid #ffffff45;
  background: #ffffff14;
  border-radius: 50%;
}
.cover-title {
  position: absolute;
  left: 18px;
  bottom: 17px;
  color: #fff;
  font-size: 29px;
  font-weight: 800;
  letter-spacing: 2px;
  text-shadow: 0 2px 9px #071625;
}
.cover-title small {
  display: block;
  font:
    500 8px ui-monospace,
    monospace;
  letter-spacing: 5px;
  margin-top: 3px;
}
.game-caption {
  display: flex;
  align-items: center;
  gap: 12px;
  justify-content: space-between;
  padding: 15px 18px 17px;
}
.game-caption h3 {
  font-size: 14px;
  font-weight: 700;
}
.game-caption div > span {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--lib-muted);
  font-size: 10px;
  margin-top: 5px;
}
.game-caption i {
  font-style: normal;
}
.game-caption > svg {
  color: var(--lib-muted);
}
.game-best {
  display: flex;
  align-items: center;
  gap: 6px;
  font:
    600 13px ui-monospace,
    monospace;
  color: var(--lib-muted);
}
.selected-line {
  position: absolute;
  width: 30px;
  height: 2px;
  background: #a5b4d8;
  bottom: 0;
  left: 18px;
  opacity: 0;
}
.selected .selected-line {
  opacity: 1;
}
.session-strip {
  display: grid;
  grid-template-columns: 1.3fr 0.55fr 0.55fr 0.65fr 1.1fr;
  align-items: center;
  gap: 15px;
  padding: 23px 26px;
  background: var(--lib-card);
  border: 1px solid var(--lib-line);
  border-radius: 13px;
  margin-top: 27px;
}
.session-intro {
  display: flex;
  align-items: center;
  gap: 13px;
}
.session-icon {
  display: grid;
  place-items: center;
  width: 43px;
  height: 43px;
  background: var(--lib-soft);
  color: #9c91ab;
  border-radius: 12px;
  flex-shrink: 0;
}
.session-intro h2 {
  font-size: 14px;
  font-weight: 700;
  margin-top: 5px;
}
.session-stat {
  border-left: 1px solid var(--lib-line);
  padding-left: 17px;
}
.session-stat strong {
  display: block;
  font-size: 24px;
  font-weight: 650;
  font-variant-numeric: tabular-nums;
}
.session-stat > span {
  display: block;
  font-size: 10px;
  color: var(--lib-muted);
  margin-top: 3px;
}
.session-recent {
  border-left: 1px solid var(--lib-line);
  padding-left: 17px;
}
.session-recent > span {
  display: block;
  color: var(--lib-muted);
  font-size: 10px;
}
.session-recent strong,
.session-recent a {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 7px;
  font-size: 12px;
  font-weight: 650;
}
.library-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
  padding: 23px 0 5px;
}
.controller-legend {
  display: flex;
  gap: 20px;
  color: var(--lib-muted);
  font-size: 10px;
}
.controller-legend > span {
  display: flex;
  gap: 7px;
  align-items: center;
}
.controller-legend kbd {
  display: inline-grid;
  place-items: center;
  min-width: 20px;
  min-height: 20px;
  border: 1px solid var(--lib-line);
  border-radius: 5px;
  padding: 1px 4px;
  font:
    9px ui-monospace,
    monospace;
}
.controller-legend .legend-confirm {
  border-radius: 50%;
  color: #72a889;
}
.library-footer > p {
  color: var(--lib-muted);
  font-size: 10px;
}
.library-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  padding: 35px 20px;
  min-height: 215px;
  border: 1px dashed var(--lib-line);
  border-radius: 14px;
  color: var(--lib-muted);
}
.library-empty h3 {
  font-weight: 650;
  font-size: 15px;
  color: var(--lib-text);
}
.library-empty p {
  font-size: 12px;
}
.park-library button {
  cursor: pointer;
  touch-action: manipulation;
}
.park-library :is(button, a):focus-visible {
  outline: 3px solid #a8bce5;
  outline-offset: 4px;
}
.console-mode {
  min-height: 100dvh;
  max-width: none;
  background:
    radial-gradient(ellipse at 60% 0, #2c315e55, transparent 60%), #111625;
  padding: clamp(25px, 4vw, 60px) clamp(20px, 6vw, 110px);
}
.console-mode .library-inner {
  max-width: 1480px;
  margin: auto;
}
.console-mode .featured-game {
  min-height: 420px;
}
.console-mode .game-cover {
  aspect-ratio: 2 / 1;
}
@media (min-width: 1600px) {
  .featured-game {
    min-height: 440px;
  }
  .featured-copy {
    padding: 45px 50px;
  }
  .game-cover {
    aspect-ratio: 2.2 / 1;
  }
}
@media (max-width: 1000px) {
  .game-shelf {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .featured-copy {
    padding: 30px;
  }
  .session-strip {
    grid-template-columns: 1.2fr 0.5fr 0.5fr 0.6fr;
    gap: 12px;
  }
  .session-recent {
    display: none;
  }
  .library-footer {
    flex-wrap: wrap;
    gap: 15px;
  }
  .library-tabs button {
    padding-inline: 11px;
  }
  .controller-connected {
    display: none;
  }
}
@media (max-width: 640px) {
  .library-header {
    gap: 12px;
    margin-bottom: 18px;
  }
  .park-symbol {
    width: 39px;
    height: 41px;
    border-radius: 11px;
  }
  .park-wordmark {
    gap: 9px;
  }
  .park-wordmark h1 {
    font-size: 16px;
    letter-spacing: 0.5px;
  }
  .park-wordmark p {
    font-size: 6px;
    letter-spacing: 1.8px;
  }
  .console-toggle {
    font-size: 10px;
    padding: 8px 10px;
    gap: 6px;
  }
  .console-toggle kbd {
    display: none;
  }
  .featured-game {
    min-height: 420px;
    border-radius: 15px;
  }
  .featured-art {
    object-position: 65% 10%;
    height: 70%;
  }
  .featured-shade {
    background:
      linear-gradient(
        0deg,
        #0c1829 12%,
        #0c1829e8 41%,
        #0c182944 75%,
        #0c182911
      ),
      linear-gradient(90deg, #0c182957, transparent);
  }
  .featured-copy {
    position: relative;
    padding: 25px;
    padding-top: 165px;
    max-width: none;
  }
  .featured-kicker {
    position: absolute;
    top: 22px;
    font-size: 8px;
    letter-spacing: 1.5px;
  }
  .featured-genres {
    margin: 0 0 9px;
  }
  .featured-copy h2 {
    letter-spacing: -1px;
  }
  .featured-copy > p {
    font-size: 11px;
    line-height: 1.8;
    max-width: 295px;
    margin-top: 10px;
  }
  .featured-buttons {
    margin-top: 18px;
    gap: 10px;
  }
  .play-now {
    font-size: 11px;
    padding: 10px 13px;
  }
  .pad-a {
    display: none;
  }
  .more-button {
    font-size: 10px;
    padding-inline: 6px;
  }
  .featured-meta {
    gap: 15px;
    margin-top: 17px;
    font-size: 9px;
  }
  .featured-position {
    display: none;
  }
  .collection-heading {
    align-items: start;
    flex-direction: column;
    gap: 14px;
    margin-top: 27px;
  }
  .collection-heading h2 {
    font-size: 20px;
  }
  .library-tabs {
    width: 100%;
    justify-content: space-between;
  }
  .library-tabs button {
    min-height: 39px;
    padding: 6px 13px;
  }
  .game-shelf {
    display: flex;
    gap: 15px;
    overflow-x: auto;
    padding: 7px;
    margin: -7px;
    scroll-snap-type: x mandatory;
    scrollbar-width: thin;
    scrollbar-color: var(--lib-line) transparent;
  }
  .library-game {
    flex: 0 0 86%;
    scroll-snap-align: start;
  }
  .game-cover {
    aspect-ratio: 1.9 / 1;
  }
  .game-caption {
    padding: 14px;
  }
  .game-caption h3 {
    font-size: 13px;
  }
  .game-caption div > span {
    font-size: 9px;
  }
  .session-strip {
    grid-template-columns: 1fr 1fr;
    gap: 17px;
    padding: 19px;
    margin-top: 25px;
  }
  .session-intro {
    grid-column: 1/-1;
  }
  .session-stat {
    padding-left: 0;
    border-left: 0;
    border-top: 1px solid var(--lib-line);
    padding-top: 15px;
  }
  .session-stat strong {
    font-size: 25px;
  }
  .controller-legend {
    gap: 12px;
    font-size: 9px;
  }
  .library-footer > p {
    font-size: 9px;
    line-height: 1.8;
  }
  .console-mode {
    padding: 24px 16px;
  }
  .console-mode .featured-game {
    min-height: 420px;
  }
  .console-mode .game-cover {
    aspect-ratio: 1.9 / 1;
  }
}
@media (prefers-reduced-motion: reduce) {
  .library-game {
    transition: none;
  }
  .library-game:hover {
    transform: none;
  }
}
</style>

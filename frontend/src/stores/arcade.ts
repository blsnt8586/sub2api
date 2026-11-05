import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { useAuthStore } from './auth'
import { games, type GameId } from '@/games/catalog'

export interface PracticeRun {
  id: string
  game: GameId
  score: number
  won: boolean
  playedAt: number
}
// Session-only practice data, isolated by user. Never used for billing.
export const useArcadeStore = defineStore('arcade', () => {
  const auth = useAuthStore()
  const sessions = ref<Record<number, PracticeRun[]>>({})
  const runs = computed(() => sessions.value[auth.user?.id ?? 0] ?? [])
  const recent = computed(() =>
    [...runs.value].reverse().sort((a, b) => b.playedAt - a.playedAt),
  )
  const best = computed(
    () =>
      Object.fromEntries(
        games.map((game) => [
          game.id,
          runs.value.reduce(
            (score, run) =>
              run.game === game.id ? Math.max(score, run.score) : score,
            0,
          ),
        ]),
      ) as Record<GameId, number>,
  )
  const total = computed(() => runs.value.reduce((sum, r) => sum + r.score, 0))
  const wins = computed(() => runs.value.filter((r) => r.won).length)
  function record(run: Omit<PracticeRun, 'playedAt'>) {
    if (!Number.isSafeInteger(run.score) || run.score < 0) return
    const userId = auth.user?.id ?? 0
    const list = (sessions.value[userId] ??= [])
    const existing = list.find((r) => r.id === run.id)
    if (existing) {
      existing.score = Math.max(existing.score, run.score)
      existing.won ||= run.won
      existing.playedAt = Date.now()
    } else list.push({ ...run, playedAt: Date.now() })
  }
  return { runs, recent, best, total, wins, record }
})

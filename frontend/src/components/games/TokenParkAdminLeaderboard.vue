<template>
  <section class="space-y-6" aria-label="通关排行榜">
      <div class="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h2 class="text-xl font-bold text-gray-900 dark:text-white">通关排行榜</h2>
        </div>
        <button class="btn btn-secondary" :disabled="loading" @click="load">
          <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
          刷新
        </button>
      </div>

      <div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
        <div class="card p-4"><span class="text-xs text-gray-500 dark:text-gray-400">参与用户</span><strong class="mt-2 block text-2xl text-gray-900 dark:text-white">{{ leaderboard?.total_players ?? 0 }}</strong></div>
        <div class="card p-4"><span class="text-xs text-gray-500 dark:text-gray-400">已发放游戏奖励</span><strong class="mt-2 block text-2xl text-gray-900 dark:text-white">{{ totalGames }}</strong></div>
        <div class="card p-4"><span class="text-xs text-gray-500 dark:text-gray-400">已发放额度</span><strong class="mt-2 block text-2xl text-gray-900 dark:text-white">${{ totalReward.toFixed(2) }}</strong></div>
      </div>

      <div class="card overflow-hidden">
        <div v-if="error" class="p-6 text-sm text-red-600 dark:text-red-400">{{ error }}</div>
        <div v-else-if="loading" class="p-6 text-sm text-gray-500 dark:text-gray-400">正在加载排行榜...</div>
        <div v-else-if="!leaderboard?.entries.length" class="p-10 text-center text-sm text-gray-500 dark:text-gray-400">暂无通关记录。</div>
        <div v-else class="overflow-x-auto">
          <table class="min-w-full divide-y divide-gray-200 dark:divide-dark-700">
            <thead class="bg-gray-50 dark:bg-dark-800/60"><tr>
              <th class="px-5 py-3 text-left text-xs font-semibold uppercase text-gray-500">排名</th>
              <th class="px-5 py-3 text-left text-xs font-semibold uppercase text-gray-500">用户邮箱</th>
              <th class="px-5 py-3 text-right text-xs font-semibold uppercase text-gray-500">通关游戏</th>
              <th class="px-5 py-3 text-right text-xs font-semibold uppercase text-gray-500">奖励额度</th>
              <th class="px-5 py-3 text-right text-xs font-semibold uppercase text-gray-500">最后通关</th>
            </tr></thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-800">
              <tr v-for="entry in leaderboard.entries" :key="entry.user_id ?? entry.rank" class="hover:bg-gray-50 dark:hover:bg-dark-800/50">
                <td class="whitespace-nowrap px-5 py-3 font-semibold text-gray-900 dark:text-white">#{{ entry.rank }}</td>
                <td class="whitespace-nowrap px-5 py-3 text-sm text-gray-700 dark:text-gray-200">{{ entry.email || entry.display_name }}</td>
                <td class="whitespace-nowrap px-5 py-3 text-right text-sm text-gray-700 dark:text-gray-200">{{ entry.games_completed }} / 3</td>
                <td class="whitespace-nowrap px-5 py-3 text-right text-sm text-gray-700 dark:text-gray-200">${{ entry.total_reward_usd.toFixed(2) }}</td>
                <td class="whitespace-nowrap px-5 py-3 text-right text-sm text-gray-500 dark:text-gray-400">{{ formatDate(entry.last_completed_at) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import { getTokenParkLeaderboard } from '@/api/admin/tokenPark'
import type { TokenParkLeaderboard } from '@/api/tokenPark'

const leaderboard = ref<TokenParkLeaderboard | null>(null)
const loading = ref(false)
const error = ref('')
const totalGames = computed(() => leaderboard.value?.entries.reduce((sum, entry) => sum + entry.games_completed, 0) ?? 0)
const totalReward = computed(() => leaderboard.value?.entries.reduce((sum, entry) => sum + entry.total_reward_usd, 0) ?? 0)
function formatDate(value: string) {
  return new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value))
}
async function load() {
  loading.value = true
  error.value = ''
  try { leaderboard.value = await getTokenParkLeaderboard() } catch { error.value = '排行榜加载失败，请稍后重试。' } finally { loading.value = false }
}
onMounted(load)
</script>

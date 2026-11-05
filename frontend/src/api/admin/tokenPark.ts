import { apiClient } from '../client'
import type { TokenParkLeaderboard } from '@/api/tokenPark'

export async function getTokenParkLeaderboard(): Promise<TokenParkLeaderboard> {
  const response = await apiClient.get<TokenParkLeaderboard>('/admin/token-park/leaderboard')
  return response.data
}

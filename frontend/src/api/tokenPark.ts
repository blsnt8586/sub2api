import { apiClient } from './client'

export interface TokenParkProof {
  level?: number
}

export interface TokenParkGameStatus {
  claimed: boolean
  amount_usd: number
}

export interface TokenParkStatus {
  eligible: boolean
  reward_enabled: boolean
  balance_usd: number
  credited_usd: number
  threshold_usd: number
  rewards: Record<string, TokenParkGameStatus>
}

export interface TokenParkProgress {
  game_id: string
  campaign_version: string
  state: string
  updated_at: string
}

export interface TokenParkLeaderboardEntry {
  rank: number
  user_id?: number
  display_name: string
  email?: string
  games_completed: number
  total_reward_usd: number
  last_completed_at: string
  is_current_user: boolean
}

export interface TokenParkLeaderboard {
  entries: TokenParkLeaderboardEntry[]
  my_rank: number
  total_players: number
}

export async function getTokenParkStatus(): Promise<TokenParkStatus> {
  const response = await apiClient.get<TokenParkStatus>('/token-park/status')
  return response.data
}

export async function getTokenParkLeaderboard(): Promise<TokenParkLeaderboard> {
  const response = await apiClient.get<TokenParkLeaderboard>('/token-park/leaderboard')
  return response.data
}

export async function claimTokenParkReward(game: string, proof: TokenParkProof): Promise<{ amount_usd: number; balance: number; claimed: boolean }> {
  const response = await apiClient.post(`/token-park/games/${encodeURIComponent(game)}/claim`, { proof })
  return response.data
}

export async function getTokenParkProgress(game: string): Promise<TokenParkProgress | null> {
  const response = await apiClient.get<TokenParkProgress | null>(`/token-park/games/${encodeURIComponent(game)}/progress`)
  return response.data
}

export async function saveTokenParkProgress(game: string, state: string): Promise<TokenParkProgress> {
  const response = await apiClient.put<TokenParkProgress>(`/token-park/games/${encodeURIComponent(game)}/progress`, { state: JSON.parse(state) })
  return response.data
}

export async function deleteTokenParkProgress(game: string): Promise<void> {
  await apiClient.delete(`/token-park/games/${encodeURIComponent(game)}/progress`)
}

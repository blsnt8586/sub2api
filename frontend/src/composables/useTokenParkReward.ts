import { claimTokenParkReward, deleteTokenParkProgress, getTokenParkProgress, saveTokenParkProgress as saveTokenParkProgressRequest, type TokenParkProof } from '@/api/tokenPark'
import { useAuthStore } from '@/stores/auth'

export async function claimCompletedTokenParkGame(game: string, proof: TokenParkProof): Promise<boolean> {
  try {
    await claimTokenParkReward(game, proof)
    await useAuthStore().refreshUser()
    return true
  } catch (error: any) {
    // A duplicate claim is an expected result after a reload; the server remains authoritative.
    if (error?.status !== 409) console.warn('TOKEN Park reward claim failed', error)
    return false
  }
}

export async function loadTokenParkProgress(game: 'miner' | 'ocean' | 'tower'): Promise<string | null> {
  if (!useAuthStore().user?.id) return null
  try {
    return (await getTokenParkProgress(game))?.state ?? null
  } catch (error) {
    console.warn('TOKEN Park progress load failed', error)
    return null
  }
}

export async function saveTokenParkProgress(game: 'miner' | 'ocean' | 'tower', state: string): Promise<boolean> {
  if (!useAuthStore().user?.id) return false
  try {
    await saveTokenParkProgressRequest(game, state)
    return true
  } catch (error) {
    console.warn('TOKEN Park progress save failed', error)
    return false
  }
}

export async function clearTokenParkProgress(game: 'miner' | 'ocean' | 'tower'): Promise<boolean> {
  if (!useAuthStore().user?.id) return false
  try {
    await deleteTokenParkProgress(game)
    return true
  } catch (error) {
    console.warn('TOKEN Park progress delete failed', error)
    return false
  }
}

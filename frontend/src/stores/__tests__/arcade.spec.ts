import { beforeEach, describe, expect, it } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useArcadeStore } from '../arcade'
import { useAuthStore } from '../auth'
import type { User } from '@/types'

beforeEach(() => setActivePinia(createPinia()))
describe('Practice library records', () => {
  it('updates one round idempotently and computes separate game bests', () => {
    const store = useArcadeStore()
    store.record({ id: 'a', game: 'miner', score: 700, won: true })
    store.record({ id: 'a', game: 'miner', score: 650, won: false })
    store.record({ id: 'b', game: 'ocean', score: 128, won: false })
    store.record({ id: 'b', game: 'ocean', score: 2048, won: true })
    expect(store.runs).toHaveLength(2)
    expect(store.best).toEqual({ miner: 700, ocean: 2048, tower: 0 })
    expect(store.wins).toBe(2)
  })
  it('updates ocean bests and recent rounds without duplicating a trip', () => {
    const store = useArcadeStore()
    store.record({ id: 'ocean-run', game: 'ocean', score: 150, won: false })
    store.record({ id: 'ocean-run', game: 'ocean', score: 650, won: true })
    expect(store.best.ocean).toBe(650)
    expect(store.total).toBe(650)
    expect(store.wins).toBe(1)
    expect(store.recent[0]?.game).toBe('ocean')
    expect(store.runs).toHaveLength(1)
  })
  it('isolates records across logout and account switches', () => {
    const store = useArcadeStore(),
      auth = useAuthStore()
    auth.user = { id: 1 } as User
    store.record({ id: 'a', game: 'miner', score: 700, won: true })
    auth.user = { id: 2 } as User
    expect(store.runs).toEqual([])
    store.record({ id: 'b', game: 'tower', score: 16, won: false })
    auth.user = null
    expect(store.runs).toEqual([])
    auth.user = { id: 1 } as User
    expect(store.best.miner).toBe(700)
    expect(store.best.tower).toBe(0)
  })
  it('rejects non-finite, negative and fractional practice scores', () => {
    const store = useArcadeStore()
    for (const score of [NaN, Infinity, -3, 1.1])
      store.record({ id: String(score), game: 'miner', score, won: false })
    expect(store.runs).toEqual([])
  })
})

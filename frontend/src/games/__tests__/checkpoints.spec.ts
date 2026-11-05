import { beforeEach, describe, expect, it, vi } from 'vitest'
import { clearCheckpoint, readCheckpoint, writeCheckpoint, type ArcadeCheckpoint } from '../checkpoints'
const valid: ArcadeCheckpoint = { version: 1, id: 'test-run', seed: 42, level: 49, score: 10000, cleared: true, lives: 2, equipment: { winch: 2, clock: 1, assay: 0 } }
beforeEach(() => { localStorage.clear(); vi.restoreAllMocks() })
describe('Local campaign checkpoints', () => {
  it('isolates users and games and restores progress without a server call', () => {
    expect(writeCheckpoint('miner', 1, valid)).toBe(true)
    expect(readCheckpoint('miner', 1)).toEqual(valid)
    expect(readCheckpoint('ocean', 1)).toBeNull()
    expect(readCheckpoint('miner', 2)).toBeNull()
    writeCheckpoint('ocean', 1, { ...valid, level: 59 })
    clearCheckpoint('miner', 1)
    expect(readCheckpoint('miner', 1)).toBeNull()
    expect(readCheckpoint('ocean', 1)!.level).toBe(59)
  })
  it('rejects malformed, out-of-range and incompatible saves while retaining the raw data', () => {
    const bad = [null, {}, { ...valid, version: 2 }, { ...valid, level: 100 }, { ...valid, score: -1 }, { ...valid, seed: null }, { ...valid, equipment: { winch: 4, clock: 0, assay: 0 } }]
    for (const value of bad) {
      localStorage.setItem('token-park-miner-campaign-v1:1', JSON.stringify(value))
      expect(readCheckpoint('miner', 1)).toBeNull()
      expect(localStorage.getItem('token-park-miner-campaign-v1:1')).not.toBeNull()
    }
    localStorage.setItem('token-park-ocean-campaign-v1:1', JSON.stringify({ ...valid, level: 60 }))
    expect(readCheckpoint('ocean', 1)).toBeNull()
  })
  it('keeps play available when storage is blocked or full', () => {
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => { throw new Error('blocked') })
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new Error('full') })
    vi.spyOn(Storage.prototype, 'removeItem').mockImplementation(() => { throw new Error('blocked') })
    expect(readCheckpoint('miner', 1)).toBeNull()
    expect(writeCheckpoint('miner', 1, valid)).toBe(false)
    expect(() => clearCheckpoint('ocean', 1)).not.toThrow()
  })
})

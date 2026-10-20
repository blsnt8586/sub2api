import { describe, it, expect } from 'vitest'
import { parseRadarBench } from '../radarBench'
describe('RadarBench contract', () => {
 it('accepts the source 150-point score including zero and missing observations', () => {
  const intelligence = { score_max: 150, points: [{model:'gpt-6-astra',effort:'high',score:0},{model:'gpt-6-luna',effort:'low',score:null}] }
  expect(parseRadarBench({intelligence})).toBe(intelligence)
 })
 it('rejects legacy IQ and invalid scales instead of inventing radar scores', () => {
  expect(parseRadarBench({intelligence:{points:[{iq:100}]}})).toBeNull()
  expect(parseRadarBench({intelligence:{score_max:150,points:[{model:'x',effort:'low',score:151}]}})).toBeNull()
  expect(parseRadarBench(null)).toBeNull()
 })
})

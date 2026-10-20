import { describe, it, expect, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import CodexRadarView from '../CodexRadarView.vue'
import point from './fixtures/radarBenchSummary.json'
const mocks = vi.hoisted(() => ({ get: vi.fn() }))
vi.mock('@/api/codexradar', () => ({ getCodexRadarSummary: mocks.get }))
vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<main><slot /></main>' } }))
describe('RadarBench display', () => {
 it('renders real public scores and switches dimensions without fetching or synthesizing IQ', async () => {
  mocks.get.mockResolvedValue({data:{intelligence:{score_max:150,expected:1,fresh:1,points:[{...point,stale:false,observed_at:'2026-10-15T07:00:00Z'}]}}})
  const w=mount(CodexRadarView); await flushPromises()
  expect(w.text()).toContain('100.5')
  expect(w.find('.radar-matrix').exists()).toBe(true)
  expect(w.find('.scroll').exists()).toBe(false)
  expect(w.findAll('.matrix-heading span')).toHaveLength(7)
  expect(w.text()).toContain('70 题次')
  expect(w.text()).toContain('$0.50')
  expect(w.text()).toContain('15.9 分钟')
  await w.findAll('nav button')[2].trigger('click')
  expect(w.text()).toContain('125.2')
  expect(w.text()).toContain('13 题次')
  expect(w.find('.metrics').exists()).toBe(false)
  expect(mocks.get).toHaveBeenCalledTimes(1)
  w.unmount()
 })
 it('keeps missing values distinct from zero and offers refresh on total failure', async () => {
  mocks.get.mockResolvedValue({data:null})
  const w=mount(CodexRadarView);await flushPromises()
  expect(w.text()).toContain('原站数据暂不可用')
  expect(w.find('.toolbar-button').text()).toContain('刷新')
  w.unmount()
 })
 it('marks aged observations stale and preserves scores when refresh fails', async () => {
  mocks.get.mockResolvedValue({data:{intelligence:{score_max:150,expected:1,fresh:1,points:[{...point,stale:false,observed_at:new Date(Date.now()-7200000).toISOString()}]}}})
  const w=mount(CodexRadarView)
  await flushPromises()
  expect(w.find('article.stale').exists()).toBe(true)
  expect(w.text()).toContain('待更新')
  mocks.get.mockRejectedValueOnce(new Error('offline'))
  await w.find('.toolbar-button').trigger('click')
  await flushPromises()
  expect(w.text()).toContain('100.5')
  expect(w.find('[role="alert"]').text()).toContain('保留上次显示的数据')
  w.unmount()
 })

})

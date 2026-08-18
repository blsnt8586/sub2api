import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'
import Sub2APIOptimizeScheduleModal from '../Sub2APIOptimizeScheduleModal.vue'

const api = vi.hoisted(() => ({
  getOptimizeSchedule: vi.fn(),
  getLinkedAccounts: vi.fn(),
  listOptimizeLogs: vi.fn(),
  runOptimizeNow: vi.fn(),
  upsertOptimizeSchedule: vi.fn(),
  deleteOptimizeSchedule: vi.fn(),
}))

vi.mock('@/api/admin/sub2apiProviders', async () => {
  const actual = await vi.importActual<typeof import('@/api/admin/sub2apiProviders')>(
    '@/api/admin/sub2apiProviders'
  )
  return { ...actual, ...api }
})

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) =>
        params ? `${key}:${JSON.stringify(params)}` : key,
      locale: ref('zh-CN'),
    }),
  }
})

describe('Sub2APIOptimizeScheduleModal', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    api.getOptimizeSchedule.mockReset()
    api.getLinkedAccounts.mockReset()
    api.listOptimizeLogs.mockReset()
    api.runOptimizeNow.mockReset()
    api.upsertOptimizeSchedule.mockReset()
    api.deleteOptimizeSchedule.mockReset()

    api.getOptimizeSchedule.mockResolvedValue({ schedule: null, logs: [] })
    api.getLinkedAccounts.mockResolvedValue([
      { id: 42, name: 'Primary Account', platform: 'openai' },
    ])
    api.listOptimizeLogs.mockResolvedValue({
      items: [{
        id: 99,
        provider_id: 7,
        schedule_id: null,
        trigger: 'manual_account',
        status: 'success',
        total: 1,
        optimized: 1,
        skipped: 0,
        failed: 0,
        created_at: '2026-08-16T11:00:00Z',
        started_at: '2026-08-16T11:00:00Z',
        finished_at: '2026-08-16T11:00:03Z',
        detail: [{
          account_id: 42,
          account_name: 'Primary Account',
          status: 'optimized',
          old_group: 'economy',
          new_group: 'standard',
          switch_events: [
            { action: 'switch', from_group: 'economy', to_group: 'discount', status: 'success', test_status: 'failed', occurred_at: '2026-08-16T11:00:01Z' },
            { action: 'rollback', from_group: 'discount', to_group: 'economy', status: 'success', occurred_at: '2026-08-16T11:00:02Z' },
            { action: 'switch', from_group: 'economy', to_group: 'standard', status: 'success', test_status: 'passed', occurred_at: '2026-08-16T11:00:03Z' },
          ],
        }],
      }],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1,
    })
  })

  it('shows Provider logs without a schedule and renders every switch event in order', async () => {
    const wrapper = mount(Sub2APIOptimizeScheduleModal, {
      props: { show: true, providerId: 7 },
      global: { stubs: { teleport: true } },
    })
    await flushPromises()

    expect(wrapper.text()).toContain('admin.sub2apiProviders.logTrigger.manual_account')
    expect(wrapper.text()).not.toContain('admin.sub2apiProviders.runNow')

    const logButton = wrapper.findAll('button').find(button => button.text().includes('#99'))
    expect(logButton).toBeDefined()
    await logButton!.trigger('click')

    const events = wrapper.findAll('[data-test="optimize-switch-event"]')
    expect(events).toHaveLength(3)
    expect(events[0].text()).toContain('economy')
    expect(events[0].text()).toContain('discount')
    expect(events[1].text()).toContain('admin.sub2apiProviders.switchAction.rollback')
    expect(events[1].text()).toContain('discount')
    expect(events[1].text()).toContain('economy')
    expect(events[2].text()).toContain('standard')
    const text = wrapper.text()
    expect(text).toContain('admin.sub2apiProviders.switchTestStatus.failed')
    expect(text).toContain('admin.sub2apiProviders.switchTestStatus.passed')

    wrapper.unmount()
  })
})

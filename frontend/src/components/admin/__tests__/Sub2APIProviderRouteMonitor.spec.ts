import { ref } from 'vue'
import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import type { Sub2APIProviderProbeTargetHealth } from '@/api/admin/sub2apiProviders'
import Sub2APIProviderRouteMonitor from '../Sub2APIProviderRouteMonitor.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key, locale: ref('zh-CN') }),
  }
})

vi.mock('@/utils/format', async () => {
  const actual = await vi.importActual<typeof import('@/utils/format')>('@/utils/format')
  return { ...actual, formatRelativeTime: (value: string) => `relative:${value}` }
})

const route: Sub2APIProviderProbeTargetHealth = {
  id: 77,
  provider_id: 7,
  account_id: 5,
  account_name: 'Claude route',
  provider_api_key_id: 272,
  remote_group_id: 44,
  remote_group_name: 'Claude-Kiro-90 cache',
  remote_group_multiplier: 0.4,
  sub2api_optimize_enabled: true,
  sub2api_min_multiplier: 0.3,
  sub2api_max_multiplier: 0.8,
  platform: 'anthropic',
  enabled: true,
  interval_seconds: 1800,
  adaptive_interval_enabled: true,
  healthy_interval_seconds: 3600,
  healthy_interval_threshold: 2,
  stable_healthy_interval_seconds: 7200,
  stable_healthy_threshold: 6,
  consecutive_healthy: 4,
  current_interval_seconds: 3600,
  test_model: 'claude-sonnet-4-6',
  allow_media_probe: false,
  timeout_seconds: 15,
  degraded_latency_ms: 2000,
  degraded_optimize_threshold: 3,
  cost_optimize_enabled: true,
  cost_optimize_interval_seconds: 21600,
  cost_optimize_healthy_threshold: 3,
  last_cost_optimize_at: null,
  failure_threshold: 3,
  recovery_threshold: 2,
  status: 'unhealthy',
  latency_ms: 421,
  traffic_request_count: 8,
  traffic_success_rate: 87.5,
  traffic_p95_latency_ms: 1780,
  error_category: 'timeout',
  error_message: 'upstream private failure details must remain hidden',
  last_checked_at: '2026-08-15T03:00:00Z',
  last_run_at: '2026-08-15T03:00:00Z',
  route_changed_at: null,
  consecutive_failures: 1,
  buckets: Array.from({ length: 60 }, (_, index) => ({
    started_at: new Date(Date.UTC(2026, 7, 15, 0, index * 5)).toISOString(),
    ended_at: new Date(Date.UTC(2026, 7, 15, 0, (index + 1) * 5)).toISOString(),
    status: index === 59 ? 'unhealthy' as const : 'healthy' as const,
    sample_count: 1,
    healthy_samples: index === 59 ? 0 : 1,
    degraded_samples: 0,
    unhealthy_samples: index === 59 ? 1 : 0,
    max_health_latency_ms: index === 59 ? 421 : 180,
  })),
}

describe('Sub2APIProviderRouteMonitor', () => {
  it('keeps route identity visible while keeping raw upstream failures out of the default pane', () => {
    const wrapper = mount(Sub2APIProviderRouteMonitor, {
      props: { routes: [route], historyByTarget: {} },
    })

    expect(wrapper.text()).toContain(route.account_name)
    expect(wrapper.text()).toContain(route.remote_group_name!)
    expect(wrapper.text()).toContain(route.test_model!)
    expect(wrapper.text()).toContain('×0.4')
    expect(wrapper.text()).toContain('421 ms')
    expect(wrapper.get('[data-test="route-multiplier-77"]').classes()).toContain('text-emerald-700')
    expect(wrapper.get('[data-test="route-multiplier-77"]').attributes('title')).toBe('admin.sub2apiProviders.multiplierRangeWithinDetail')
    expect(wrapper.get('.route-platform').classes()).toContain('bg-orange-100')
    expect(wrapper.get('[data-test="route-optimize-77"]').attributes('aria-label')).toBe('admin.sub2apiProviders.joinScheduleOn')
    expect(wrapper.text()).not.toContain(route.error_message!)
    expect(wrapper.findAll('[data-test="route-timeline-bucket"]')).toHaveLength(60)
    expect(wrapper.get('[data-test="route-timeline-detail"]').text()).toContain('421 ms')
  })

  it('requests isolated history and runs only the chosen route', async () => {
    const wrapper = mount(Sub2APIProviderRouteMonitor, {
      props: { routes: [route], historyByTarget: {} },
    })

    await wrapper.get('[data-test="route-history-77"]').trigger('click')
    await wrapper.get('[data-test="route-run-77"]').trigger('click')

    expect(wrapper.emitted('history')?.[0]).toEqual([77])
    expect(wrapper.emitted('run')?.[0]).toEqual([77])
  })

  it('shows the account test model as inherited read-only state', async () => {
    const wrapper = mount(Sub2APIProviderRouteMonitor, {
      props: { routes: [route], historyByTarget: {} },
    })

    await wrapper.get('[data-test="route-toggle-77"]').trigger('click')

    const model = wrapper.get('[data-test="route-account-model-77"]')
    expect(model.text()).toContain(route.test_model!)
    expect(model.text()).toContain('admin.sub2apiProviders.health.routes.modelSourceAccount')
    expect(model.find('input').exists()).toBe(false)
  })

  it('offers the 30-minute lower-cost check interval', async () => {
    const wrapper = mount(Sub2APIProviderRouteMonitor, {
      props: { routes: [route], historyByTarget: {} },
    })

    await wrapper.get('[data-test="route-toggle-77"]').trigger('click')

    const interval = wrapper.get('select')
    expect(interval.find('option[value="1800"]').exists()).toBe(true)
  })

  it('shows and updates the adaptive healthy cadence independently', async () => {
    const wrapper = mount(Sub2APIProviderRouteMonitor, {
      props: { routes: [route], historyByTarget: {} },
    })

    await wrapper.get('[data-test="route-toggle-77"]').trigger('click')
    const cadence = wrapper.get('[data-test="route-adaptive-cadence-77"]')
    expect(cadence.text()).toContain('admin.sub2apiProviders.health.routes.currentInterval')
    expect(cadence.text()).toContain('admin.sub2apiProviders.health.routes.healthyStreak')

    const healthyInput = cadence.find('input[value="3600"]')
    await healthyInput.setValue('4200')
    await healthyInput.trigger('change')
    expect(wrapper.emitted('update')).toContainEqual([77, { healthy_interval_seconds: 4200 }])
  })

  it('marks staged route changes as unsaved and prevents a probe run', async () => {
    const wrapper = mount(Sub2APIProviderRouteMonitor, {
      props: { routes: [route], historyByTarget: {}, dirtyTargetIds: [route.id] },
    })

    expect(wrapper.text()).toContain('admin.sub2apiProviders.health.routes.unsaved')
    expect(wrapper.get('[data-test="route-run-77"]').attributes('disabled')).toBeDefined()
  })
})

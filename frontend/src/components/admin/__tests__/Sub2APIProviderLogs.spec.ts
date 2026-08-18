import { ref } from 'vue'
import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import type { OptimizeLogInfo, Sub2APIProviderHealth, Sub2APIProviderProbeTargetHealth } from '@/api/admin/sub2apiProviders'
import Sub2APIProviderLogs from '../Sub2APIProviderLogs.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key, locale: ref('zh-CN') }),
  }
})

vi.mock('@/utils/format', () => ({ formatDateTime: (value: string) => `date:${value}` }))

const buckets = Array.from({ length: 60 }, () => ({
  started_at: '2026-08-15T00:00:00Z',
  ended_at: '2026-08-15T00:24:00Z',
  status: 'unknown' as const,
  sample_count: 0,
  healthy_samples: 0,
  degraded_samples: 0,
  unhealthy_samples: 0,
}))

const control: Sub2APIProviderHealth = {
  provider_id: 1,
  status: 'unhealthy',
  control_status: 'unhealthy',
  data_status: 'unknown',
  traffic_status: 'unknown',
  consecutive_failures: 2,
  data_probe_count: 0,
  data_probe_success: 0,
  data_probe_failed: 0,
  data_probe_enabled: false,
  data_probe_interval_seconds: 1800,
  probe_account_count: 0,
  account_probes: [],
  traffic_request_count: 0,
  error_category: 'authentication',
  error_message: 'secret upstream response must not be shown',
  details: { login_error: 'raw private login details' },
  last_checked_at: '2026-08-15T12:00:00Z',
}

const route: Sub2APIProviderProbeTargetHealth = {
  id: 7,
  provider_id: 1,
  account_id: 9,
  account_name: 'Claude account',
  remote_group_name: 'Claude group',
  remote_group_multiplier: 0.4,
  platform: 'anthropic',
  enabled: true,
  interval_seconds: 1800,
  test_model: 'claude-sonnet-4-6',
  allow_media_probe: false,
  timeout_seconds: 15,
  degraded_latency_ms: 2000,
  failure_threshold: 3,
  recovery_threshold: 2,
  status: 'healthy',
  latency_ms: 420,
  traffic_request_count: 8,
  traffic_success_rate: 100,
  consecutive_failures: 0,
  buckets,
}

const optimizationLog: OptimizeLogInfo = {
  id: 268,
  provider_id: 1,
  trigger: 'probe_unhealthy',
  status: 'success',
  total: 1,
  optimized: 0,
  skipped: 1,
  failed: 0,
  created_at: '2026-08-17T12:48:45Z',
  detail: [{
    account_id: 9,
    account_name: 'Claude account',
    status: 'skipped',
    old_group: 'Stable group',
    new_group: 'Stable group',
    old_multiplier: 0.05,
    new_multiplier: 0.05,
    probe_error_category: 'timeout',
    probe_error_message: 'secret probe error must not be shown',
    switch_events: [{
      action: 'switch',
      from_group_id: 44,
      from_group: 'Stable group',
      from_multiplier: 0.05,
      to_group_id: 59,
      to_group: 'Candidate group',
      to_multiplier: 0.035,
      status: 'success',
      test_status: 'failed',
      reason: 'secret candidate error must not be shown',
      occurred_at: '2026-08-17T12:45:56Z',
    }, {
      action: 'rollback',
      from_group_id: 59,
      from_group: 'Candidate group',
      from_multiplier: 0.035,
      to_group_id: 44,
      to_group: 'Stable group',
      to_multiplier: 0.05,
      status: 'success',
      reason: 'secret rollback detail must not be shown',
      occurred_at: '2026-08-17T12:48:18Z',
    }],
  }],
}

const recoveredOptimizationLog: OptimizeLogInfo = {
  id: 270,
  provider_id: 1,
  trigger: 'probe_unhealthy',
  status: 'success',
  total: 1,
  optimized: 1,
  skipped: 0,
  failed: 0,
  created_at: '2026-08-17T13:15:00Z',
  detail: [{
    account_id: 9,
    account_name: 'Claude account',
    status: 'optimized',
    old_group: 'Stable group',
    new_group: 'Available group',
    old_multiplier: 0.05,
    new_multiplier: 0.08,
    switch_events: [{
      action: 'switch',
      from_group_id: 44,
      from_group: 'Stable group',
      from_multiplier: 0.05,
      to_group_id: 59,
      to_group: 'Rejected group',
      to_multiplier: 0.04,
      status: 'success',
      test_status: 'failed',
      occurred_at: '2026-08-17T13:12:00Z',
    }, {
      action: 'rollback',
      from_group_id: 59,
      from_group: 'Rejected group',
      from_multiplier: 0.04,
      to_group_id: 44,
      to_group: 'Stable group',
      to_multiplier: 0.05,
      status: 'success',
      occurred_at: '2026-08-17T13:12:30Z',
    }, {
      action: 'switch',
      from_group_id: 44,
      from_group: 'Stable group',
      from_multiplier: 0.05,
      to_group_id: 61,
      to_group: 'Available group',
      to_multiplier: 0.08,
      status: 'success',
      test_status: 'passed',
      occurred_at: '2026-08-17T13:14:30Z',
    }],
  }],
}

describe('Sub2APIProviderLogs', () => {
  it('shows diagnostic stages and categories without exposing raw upstream errors', () => {
    const wrapper = mount(Sub2APIProviderLogs, {
      props: { controlHistory: [control], routes: [route], routeHistory: { 7: [route] }, optimizationLogs: [] },
    })

    expect(wrapper.text()).toContain('admin.sub2apiProviders.health.logs.stages.login')
    expect(wrapper.text()).toContain('authentication')
    expect(wrapper.text()).toContain('Claude account')
    expect(wrapper.text()).toContain('×0.4')
    expect(wrapper.text()).not.toContain(control.error_message!)
    expect(wrapper.text()).not.toContain('raw private login details')
  })

  it('shows probe-triggered group switching and rollback without raw upstream errors', () => {
    const wrapper = mount(Sub2APIProviderLogs, {
      props: { controlHistory: [], routes: [route], routeHistory: {}, optimizationLogs: [optimizationLog] },
    })

    expect(wrapper.text()).toContain('admin.sub2apiProviders.health.logs.scopes.optimization')
    expect(wrapper.text()).toContain('admin.sub2apiProviders.health.logs.optimization.triggers.probe_unhealthy')
    expect(wrapper.text()).toContain('Candidate group ×0.04')
    expect(wrapper.text()).toContain('Stable group ×0.05')
    expect(wrapper.text()).toContain('admin.sub2apiProviders.health.logs.optimization.outcomes.rolledBack')
    expect(wrapper.text()).not.toContain('secret probe error must not be shown')
    expect(wrapper.text()).not.toContain('secret candidate error must not be shown')
    expect(wrapper.text()).not.toContain('secret rollback detail must not be shown')
  })

  it('uses the final successful candidate as the outcome after an earlier rollback', () => {
    const wrapper = mount(Sub2APIProviderLogs, {
      props: { controlHistory: [], routes: [route], routeHistory: {}, optimizationLogs: [recoveredOptimizationLog] },
    })

    const optimizedLabel = wrapper.findAll('span').find(node => (
      node.text() === 'admin.sub2apiProviders.health.logs.optimization.outcomes.optimized'
    ))

    expect(optimizedLabel?.classes()).toContain('text-green-600')
    expect(wrapper.text()).toContain('admin.sub2apiProviders.health.logs.optimization.summaries.optimized')
    expect(wrapper.text()).not.toContain('admin.sub2apiProviders.health.logs.optimization.summaries.rolledBack')
    expect(wrapper.text()).toContain('Rejected group ×0.04')
    expect(wrapper.text()).toContain('admin.sub2apiProviders.health.logs.optimization.events.testFailed')
    expect(wrapper.text()).toContain('admin.sub2apiProviders.health.logs.optimization.events.rollbackSucceeded')
    expect(wrapper.text()).toContain('Available group ×0.08')
    expect(wrapper.text()).toContain('admin.sub2apiProviders.health.logs.optimization.events.testPassed')
  })
})

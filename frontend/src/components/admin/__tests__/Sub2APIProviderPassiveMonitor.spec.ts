import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import type { Sub2APIProviderProbeTargetHealth } from '@/api/admin/sub2apiProviders'
import Sub2APIProviderPassiveMonitor from '../Sub2APIProviderPassiveMonitor.vue'

vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({ t: (key: string) => key, locale: { value: 'zh' } }),
}))
function route(): Sub2APIProviderProbeTargetHealth {
  return {
    id: 7, provider_id: 18, account_id: 396, account_name: 'User traffic account', platform: 'openai',
    monitoring_source: 'user_requests', enabled: true, interval_seconds: 120, adaptive_interval_enabled: true,
    healthy_interval_seconds: 320, healthy_interval_threshold: 2, stable_healthy_interval_seconds: 600, stable_healthy_threshold: 6,
    consecutive_healthy: 0, current_interval_seconds: 120, allow_media_probe: false, timeout_seconds: 60,
    degraded_latency_ms: 5000, degraded_optimize_threshold: 3, cost_optimize_enabled: false, cost_optimize_interval_seconds: 3600,
    cost_optimize_healthy_threshold: 3, failure_threshold: 3, recovery_threshold: 2, status: 'unknown', traffic_request_count: 0,
    consecutive_failures: 0, buckets: [],
  }
}

describe('Sub2APIProviderPassiveMonitor', () => {
  it('shows user traffic and has no active probe or cadence controls', () => {
    const wrapper = mount(Sub2APIProviderPassiveMonitor, { props: { routes: [route()] } })
    expect(wrapper.text()).toContain('health.routes.hybridDescription')
    expect(wrapper.text()).toContain('health.routes.noRequestResults')
    expect(wrapper.find('[data-test^="route-timeline-"]').exists()).toBe(true)
    expect(wrapper.text()).not.toContain('health.routes.userHistory1h')
    expect(wrapper.find('input').exists()).toBe(false)
    expect(wrapper.find('[data-test^="route-run-"]').exists()).toBe(false)
    expect(wrapper.emitted()).toEqual({})
  })

  it('expands compact account policy without loading duplicate request history', async () => {
    const account = { ...route(), hybrid_health: { account_id: 396, provider_id: 18, policy: { enabled: true, window_seconds: 300, minimum_samples: 3, degraded_latency_ms: 25000, timeout_ms: 60000, failure_threshold: 2, account_failure_threshold: 3, session_avoid_seconds: 300, recovery_delay_seconds: 300, probe_interval_seconds: 60, probe_timeout_seconds: 60, recovery_successes: 2 }, phase: 'observing' as const, revision: 1, observe_since: '', reason: '', recovery_successes: 0, last_probe_status: '' } }
    const wrapper = mount(Sub2APIProviderPassiveMonitor, { props: { routes: [account] } })
    await wrapper.get('[data-test="passive-account-7"]').trigger('click')
    expect(wrapper.text()).toContain('health.routes.accountPolicy')
    expect(wrapper.text()).not.toContain('health.routes.userHistory1h')
    expect(wrapper.emitted('run')).toBeUndefined()
    await wrapper.get('[data-test="passive-account-7"]').trigger('click')
    expect(wrapper.emitted('history')).toBeUndefined()
  })

  it('keeps account settings independent and emits a draft without running a probe', async () => {
    const policy = { enabled: true, window_seconds: 300, minimum_samples: 3, degraded_latency_ms: 25000, timeout_ms: 60000, failure_threshold: 2, account_failure_threshold: 3, session_avoid_seconds: 300, recovery_delay_seconds: 300, probe_interval_seconds: 60, probe_timeout_seconds: 60, recovery_successes: 2 }
    const first = { ...route(), hybrid_health: { account_id: 396, provider_id: 18, policy, phase: 'observing' as const, revision: 1, observe_since: '', reason: '', recovery_successes: 0, last_probe_status: '' } }
    const second = { ...first, id: 8, account_id: 397, account_name: 'Second account', hybrid_health: { ...first.hybrid_health, account_id: 397, policy: { ...policy, failure_threshold: 1 } } }
    const wrapper = mount(Sub2APIProviderPassiveMonitor, { props: { routes: [first, second] } })
    await wrapper.get('[data-test="passive-account-7"]').trigger('click')
    const input = wrapper.findAll('input[type="number"]')[0]!
    await input.setValue(5)
    expect(wrapper.emitted('update-policy')).toEqual([[396, { ...policy, failure_threshold: 5 }]])
    expect(second.hybrid_health.policy.failure_threshold).toBe(1)
    expect(wrapper.emitted('run')).toBeUndefined()
    await wrapper.setProps({ draftPolicies: { 396: { ...policy, failure_threshold: 5 } } })
    expect(wrapper.text()).toContain('health.routes.unsaved')
    expect((wrapper.findAll('input[type="number"]')[0]!.element as HTMLInputElement).value).toBe('5')
  })

  it('distinguishes recovery progress from user request history', () => {
    const account = { ...route(), monitoring_source: 'recovery_probes', status: 'unhealthy' as const,
      hybrid_health: { account_id: 396, provider_id: 18, policy: { enabled: true, window_seconds: 300, minimum_samples: 3, degraded_latency_ms: 25000, timeout_ms: 60000, failure_threshold: 2, account_failure_threshold: 3, session_avoid_seconds: 300, recovery_delay_seconds: 300, probe_interval_seconds: 60, probe_timeout_seconds: 60, recovery_successes: 2 }, phase: 'recovering' as const, revision: 1, observe_since: '', reason: 'failures', recovery_successes: 1, next_probe_at: '2026-11-03T08:10:00Z', last_probe_status: 'healthy' } }
    const wrapper = mount(Sub2APIProviderPassiveMonitor, { props: { routes: [account] } })
    expect(wrapper.text()).toContain('health.routes.recovering')
    expect(wrapper.text()).toContain('health.routes.recoveryProgress')
    expect(wrapper.text()).toContain('health.routes.nextRecoveryProbe')
    expect(wrapper.findAll('dd').map(item => item.text())).toEqual(['0', '0', '—', '—'])
  })

  it('shows the waiting period before the first recovery probe', () => {
    const policy = { enabled: true, window_seconds: 300, minimum_samples: 3, degraded_latency_ms: 25000, timeout_ms: 60000, failure_threshold: 2, account_failure_threshold: 3, session_avoid_seconds: 300, recovery_delay_seconds: 300, probe_interval_seconds: 60, probe_timeout_seconds: 60, recovery_successes: 2 }
    const account = { ...route(), status: 'unhealthy' as const, hybrid_health: { account_id: 396, provider_id: 18, policy, phase: 'recovering' as const, revision: 1, observe_since: '', reason: 'failures', recovery_successes: 0, next_probe_at: '2026-11-03T08:10:00Z', last_probe_status: '' } }
    const wrapper = mount(Sub2APIProviderPassiveMonitor, { props: { routes: [account] } })
    expect(wrapper.text()).toContain('health.routes.paused')
    expect(wrapper.text()).toContain('health.routes.firstRecoveryProbe')
    expect(wrapper.text()).not.toContain('health.routes.recoveryProgress')
  })
})

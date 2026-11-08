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
    answer_probe_interval_seconds: 300,
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
    const account = { ...route(), hybrid_health: { account_id: 396, provider_id: 18, policy: { enabled: true, window_seconds: 300, minimum_samples: 3, degraded_latency_ms: 25000, timeout_ms: 60000, failure_threshold: 2, account_failure_threshold: 3, degraded_threshold: 2, account_degraded_threshold: 5, session_avoid_seconds: 300, recovery_delay_seconds: 300, probe_interval_seconds: 60, probe_timeout_seconds: 60, recovery_successes: 2 }, phase: 'observing' as const, revision: 1, observe_since: '', reason: '', recovery_successes: 0, last_probe_status: '' } }
    const wrapper = mount(Sub2APIProviderPassiveMonitor, { props: { routes: [account] } })
    await wrapper.get('[data-test="passive-account-7"]').trigger('click')
    expect(wrapper.text()).toContain('health.routes.accountPolicy')
    expect(wrapper.text()).not.toContain('health.routes.userHistory1h')
    expect(wrapper.emitted('run')).toBeUndefined()
    await wrapper.get('[data-test="passive-account-7"]').trigger('click')
    expect(wrapper.emitted('history')).toBeUndefined()
  })

  it('keeps account settings independent and emits a draft without running a probe', async () => {
    const policy = { enabled: true, window_seconds: 300, minimum_samples: 3, degraded_latency_ms: 25000, timeout_ms: 60000, failure_threshold: 2, account_failure_threshold: 3, degraded_threshold: 2, account_degraded_threshold: 5, session_avoid_seconds: 300, recovery_delay_seconds: 300, probe_interval_seconds: 60, probe_timeout_seconds: 60, recovery_successes: 2 }
    const first = { ...route(), hybrid_health: { account_id: 396, provider_id: 18, policy, phase: 'observing' as const, revision: 1, observe_since: '', reason: '', recovery_successes: 0, last_probe_status: '' } }
    const second = { ...first, id: 8, account_id: 397, account_name: 'Second account', hybrid_health: { ...first.hybrid_health, account_id: 397, policy: { ...policy, failure_threshold: 1 } } }
    const wrapper = mount(Sub2APIProviderPassiveMonitor, { props: { routes: [first, second] } })
    await wrapper.get('[data-test="passive-account-7"]').trigger('click')
    const input = wrapper.get('[data-test="health-policy-396-failure_threshold"]')
    await input.setValue(5)
    expect(wrapper.emitted('update-policy')).toEqual([[396, { ...policy, failure_threshold: 5 }]])
    expect(second.hybrid_health.policy.failure_threshold).toBe(1)
    expect(wrapper.emitted('run')).toBeUndefined()
    await wrapper.setProps({ draftPolicies: { 396: { ...policy, failure_threshold: 5 } } })
    expect(wrapper.text()).toContain('health.routes.unsaved')
    expect((wrapper.get('[data-test="health-policy-396-failure_threshold"]').element as HTMLInputElement).value).toBe('5')
  })

  it('distinguishes recovery progress from user request history', () => {
    const account = { ...route(), monitoring_source: 'recovery_probes', status: 'unhealthy' as const,
      hybrid_health: { account_id: 396, provider_id: 18, policy: { enabled: true, window_seconds: 300, minimum_samples: 3, degraded_latency_ms: 25000, timeout_ms: 60000, failure_threshold: 2, account_failure_threshold: 3, degraded_threshold: 2, account_degraded_threshold: 5, session_avoid_seconds: 300, recovery_delay_seconds: 300, probe_interval_seconds: 60, probe_timeout_seconds: 60, recovery_successes: 2 }, phase: 'recovering' as const, revision: 1, observe_since: '', reason: 'failures', recovery_successes: 1, next_probe_at: '2026-11-03T08:10:00Z', last_probe_status: 'healthy' } }
    const wrapper = mount(Sub2APIProviderPassiveMonitor, { props: { routes: [account] } })
    expect(wrapper.text()).toContain('health.routes.recovering')
    expect(wrapper.text()).toContain('health.routes.recoveryProgress')
    expect(wrapper.text()).toContain('health.routes.nextRecoveryProbe')
    expect(wrapper.findAll('dd').map(item => item.text())).toEqual(['0', '0', '—', '—'])
  })

  it('shows an account pause separately from healthy request evidence', () => {
    const wrapper = mount(Sub2APIProviderPassiveMonitor, {
      props: { routes: [{ ...route(), account_status: 'disabled', status: 'healthy' }] },
    })
    expect(wrapper.text()).toContain('admin.accounts.status.paused')
    expect(wrapper.text()).toContain('health.status.healthy')
  })

  it('edits and disables slow-request rules independently from error rules', async () => {
    const policy = { enabled: true, window_seconds: 300, minimum_samples: 3, degraded_latency_ms: 25000, timeout_ms: 60000, failure_threshold: 2, account_failure_threshold: 3, degraded_threshold: 2, account_degraded_threshold: 5, session_avoid_seconds: 300, recovery_delay_seconds: 300, probe_interval_seconds: 60, probe_timeout_seconds: 60, recovery_successes: 2 }
    const account = { ...route(), hybrid_health: { account_id: 396, provider_id: 18, policy, phase: 'observing' as const, revision: 1, observe_since: '', reason: '', recovery_successes: 0, last_probe_status: '' } }
    const wrapper = mount(Sub2APIProviderPassiveMonitor, { props: { routes: [account] } })
    await wrapper.get('[data-test="passive-account-7"]').trigger('click')
    const session = wrapper.get('[data-test="health-policy-396-degraded_threshold"]')
    const total = wrapper.get('[data-test="health-policy-396-account_degraded_threshold"]')
    expect((session.element as HTMLInputElement).value).toBe('2')
    expect((total.element as HTMLInputElement).value).toBe('5')
    await session.setValue(3)
    expect(wrapper.emitted('update-policy')?.at(-1)).toEqual([396, { ...policy, degraded_threshold: 3 }])
    await wrapper.setProps({ draftPolicies: { 396: { ...policy, degraded_threshold: 3 } } })
    await total.setValue(0)
    expect(wrapper.emitted('update-policy')?.at(-1)).toEqual([396, { ...policy, degraded_threshold: 3, account_degraded_threshold: 0 }])
    expect(policy.account_failure_threshold).toBe(3)
    expect(policy.degraded_threshold).toBe(2)
  })

  it('shows the waiting period before the first recovery probe', () => {
    const policy = { enabled: true, window_seconds: 300, minimum_samples: 3, degraded_latency_ms: 25000, timeout_ms: 60000, failure_threshold: 2, account_failure_threshold: 3, degraded_threshold: 2, account_degraded_threshold: 5, session_avoid_seconds: 300, recovery_delay_seconds: 300, probe_interval_seconds: 60, probe_timeout_seconds: 60, recovery_successes: 2 }
    const account = { ...route(), status: 'unhealthy' as const, hybrid_health: { account_id: 396, provider_id: 18, policy, phase: 'recovering' as const, revision: 1, observe_since: '', reason: 'failures', recovery_successes: 0, next_probe_at: '2026-11-03T08:10:00Z', last_probe_status: '' } }
    const wrapper = mount(Sub2APIProviderPassiveMonitor, { props: { routes: [account] } })
    expect(wrapper.text()).toContain('health.routes.paused')
    expect(wrapper.text()).toContain('health.routes.firstRecoveryProbe')
    expect(wrapper.text()).not.toContain('health.routes.recoveryProgress')
  })

  it('configures a prompt and shows raw upstream answer history without judging it', async () => {
    const account = {
      ...route(),
      answer_probe_enabled: true,
      answer_probe_prompt: 'Configured question',
      answer_text: 'Raw upstream answer',
      answer_at: '2026-11-07T00:05:00Z',
      answer_history: [
        { at: '2026-11-07T00:05:00Z', text: 'A longer answer that is clipped in the single-line tile but remains available in the hover title.', latency_ms: 1300 },
        { at: '2026-11-07T00:00:00Z', text: 'Raw upstream answer', latency_ms: 1200 },
      ],
    }
    const wrapper = mount(Sub2APIProviderPassiveMonitor, { props: { routes: [account] } })
    const answerTiles = wrapper.findAll('[data-test="answer-timeline-item"]')
    expect(answerTiles).toHaveLength(2)
    expect(answerTiles[0].attributes('title')).toContain('Raw upstream answer')
    expect(answerTiles[1].classes()).toContain('truncate')
    expect(answerTiles[1].attributes('title')).toContain('A longer answer that is clipped in the single-line tile')
    expect(wrapper.get('[data-test="answer-timeline-latest-time"]').text()).toContain('11/07')
    await wrapper.get('[data-test="passive-account-7"]').trigger('click')
    expect(wrapper.text()).toContain('Raw upstream answer')
    expect(wrapper.find('[data-test="answerProbeHistory"]').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('correct')
    await wrapper.get('textarea').setValue('New configured question')
    await wrapper.get('[data-test="answer-probe-interval"]').setValue('60')
    expect(wrapper.emitted('update-answer')).toBeUndefined()
    await wrapper.get('[data-test="answer-probe-save"]').trigger('click')
    expect(wrapper.emitted('update-answer')?.at(-1)).toEqual([7, { answer_probe_enabled: true, answer_probe_prompt: 'New configured question', answer_probe_interval_seconds: 60 }])
  })

  it('saves immediately when the answer test is disabled from the switch', async () => {
    const account = { ...route(), answer_probe_enabled: true, answer_probe_prompt: 'Configured question' }
    const wrapper = mount(Sub2APIProviderPassiveMonitor, { props: { routes: [account] } })
    await wrapper.get('[data-test="passive-account-7"]').trigger('click')
    const checkbox = wrapper.find('input[type="checkbox"]')
    await checkbox.setValue(false)
    expect(wrapper.emitted('update-answer')).toBeUndefined()
    await wrapper.get('[data-test="answer-probe-save"]').trigger('click')
    expect(wrapper.emitted('update-answer')?.at(-1)).toEqual([7, { answer_probe_enabled: false, answer_probe_prompt: 'Configured question', answer_probe_interval_seconds: 300 }])
  })

  it('keeps only the six newest answer tiles in one row', () => {
    const history = Array.from({ length: 8 }, (_, index) => ({
      at: `2026-11-07T00:0${index}:00Z`,
      text: `Answer ${index}`,
    }))
    const wrapper = mount(Sub2APIProviderPassiveMonitor, { props: { routes: [{ ...route(), answer_probe_enabled: true, answer_history: history }] } })
    const tiles = wrapper.findAll('[data-test="answer-timeline-item"]')
    expect(tiles).toHaveLength(6)
    expect(tiles[0].text()).toBe('Answer 2')
    expect(tiles[5].text()).toBe('Answer 7')
    expect(wrapper.find('[data-test="answer-timeline"] .overflow-x-auto').exists()).toBe(false)
  })

  it('marks failed answer probes separately from returned answers', () => {
    const wrapper = mount(Sub2APIProviderPassiveMonitor, {
      props: {
        routes: [{
          ...route(),
          answer_probe_enabled: true,
          answer_history: [
            { at: '2026-11-07T00:00:00Z', text: 'Returned answer' },
            { at: '2026-11-07T00:01:00Z', error: 'upstream timeout' },
          ],
        }],
      },
    })
    const tiles = wrapper.findAll('[data-test="answer-timeline-item"]')
    expect(tiles[0].classes()).toContain('text-blue-700')
    expect(tiles[1].classes()).toContain('text-red-700')
  })
})

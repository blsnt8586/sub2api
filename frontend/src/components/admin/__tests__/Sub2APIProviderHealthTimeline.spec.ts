import { ref } from 'vue'
import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import type { Sub2APIProviderHealthOverview } from '@/api/admin/sub2apiProviders'
import Sub2APIProviderHealthTimeline from '../Sub2APIProviderHealthTimeline.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
      locale: ref('zh-CN'),
    }),
  }
})

const windowStart = new Date('2026-08-13T12:00:00Z')
const buckets = Array.from({ length: 60 }, (_, index) => ({
  started_at: new Date(windowStart.getTime() + index * 24 * 60 * 1000).toISOString(),
  ended_at: new Date(windowStart.getTime() + (index + 1) * 24 * 60 * 1000).toISOString(),
  status: index === 1 ? 'degraded' as const : index === 2 ? 'unhealthy' as const : 'unknown' as const,
  sample_count: index === 1 || index === 2 ? 1 : 0,
  healthy_samples: 0,
  degraded_samples: index === 1 ? 1 : 0,
  unhealthy_samples: index === 2 ? 1 : 0,
  max_health_latency_ms: index === 1 ? 2400 : null,
  last_error: index === 2 ? 'health endpoint failed' : null,
}))

const overview: Sub2APIProviderHealthOverview = {
  provider_id: 7,
  availability_status: 'unhealthy',
  evidence_status: 'unhealthy',
  window_started_at: windowStart.toISOString(),
  window_ended_at: new Date(windowStart.getTime() + 24 * 60 * 60 * 1000).toISOString(),
  bucket_seconds: 1440,
  buckets,
  summary: { healthy: 0, degraded: 1, unhealthy: 1, unknown: 58 },
  routes: [],
}

describe('Sub2APIProviderHealthTimeline', () => {
  it('renders a fixed 60-bucket timeline with a textual summary', () => {
    const wrapper = mount(Sub2APIProviderHealthTimeline, { props: { overview } })

    expect(wrapper.findAll('[data-test="timeline-bucket"]')).toHaveLength(60)
    expect(wrapper.get('[data-test="timeline-focus-target"]').attributes('aria-label')).toContain(
      'admin.sub2apiProviders.health.timelineAriaLabel'
    )
    expect(wrapper.text()).toContain('58 admin.sub2apiProviders.health.bucketShort.unknown')
    expect(wrapper.get('[data-test="timeline-metrics"]').text()).toContain('2/60')
    expect(wrapper.get('[data-test="timeline-metrics"]').text()).toContain('0%')
    expect(wrapper.get('[data-test="timeline-metrics"]').text()).toContain('2')
    expect(wrapper.get('[data-test="timeline-bucket"]').attributes('title')).toContain(
      'admin.sub2apiProviders.health.status.unknown'
    )
    expect(wrapper.get('[data-test="timeline-detail"]').text()).toContain('admin.sub2apiProviders.health.anomalyRecorded')
    expect(wrapper.text()).not.toContain('health endpoint failed')
    expect(wrapper.findAll('[data-test="timeline-bucket"]')[2].classes()).not.toContain('ring-gray-900')
  })

  it('uses one focus target and supports arrow, Home, and End navigation', async () => {
    const wrapper = mount(Sub2APIProviderHealthTimeline, { props: { overview } })
    const focusTarget = wrapper.get('[data-test="timeline-focus-target"]')

    expect(wrapper.findAll('[tabindex="0"]')).toHaveLength(1)
    expect(wrapper.get('[data-test="timeline-detail"]').text()).toContain('admin.sub2apiProviders.health.anomalyRecorded')

    await focusTarget.trigger('keydown', { key: 'ArrowLeft' })
    expect(wrapper.get('[data-test="timeline-detail"]').text()).toContain('admin.sub2apiProviders.health.status.degraded')

    await focusTarget.trigger('keydown', { key: 'Home' })
    expect(wrapper.get('[data-test="timeline-detail"]').text()).toContain('admin.sub2apiProviders.health.status.unknown')

    await focusTarget.trigger('keydown', { key: 'End' })
    expect(wrapper.get('[data-test="timeline-detail"]').text()).toContain('admin.sub2apiProviders.health.status.unknown')
  })
})

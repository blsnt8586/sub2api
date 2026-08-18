import { ref } from 'vue'
import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import type {
  ProviderHealthStatus,
  Sub2APIProviderHealthBucket,
  Sub2APIProviderProbeTargetHealth,
} from '@/api/admin/sub2apiProviders'
import Sub2APIProviderRouteTimeline from '../Sub2APIProviderRouteTimeline.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => params?.count == null ? key : `${key}:${params.count}`,
      locale: ref('zh-CN'),
    }),
  }
})

const probeStart = new Date('2026-08-17T10:00:00Z')
const probeTimeFormatter = new Intl.DateTimeFormat('zh-CN', {
  month: '2-digit',
  day: '2-digit',
  hour: '2-digit',
  minute: '2-digit',
  second: '2-digit',
})

const makeBucket = (
  index: number,
  status: ProviderHealthStatus = 'healthy',
  latency = 842,
  sampleCount = 1,
): Sub2APIProviderHealthBucket => ({
  started_at: new Date(probeStart.getTime() + index * 5 * 60 * 1000).toISOString(),
  ended_at: new Date(probeStart.getTime() + index * 5 * 60 * 1000 + latency).toISOString(),
  status,
  sample_count: sampleCount,
  healthy_samples: status === 'healthy' && sampleCount > 0 ? 1 : 0,
  degraded_samples: status === 'degraded' && sampleCount > 0 ? 1 : 0,
  unhealthy_samples: status === 'unhealthy' && sampleCount > 0 ? 1 : 0,
  max_health_latency_ms: sampleCount > 0 ? latency : null,
})

const makeRoute = (
  buckets: Sub2APIProviderHealthBucket[],
  status: Sub2APIProviderProbeTargetHealth['status'] = 'healthy',
): Sub2APIProviderProbeTargetHealth => ({
  id: 77,
  provider_id: 7,
  account_id: 5,
  account_name: 'OpenAI account',
  platform: 'openai',
  enabled: status !== 'disabled',
  interval_seconds: 300,
  test_model: 'gpt-5',
  allow_media_probe: false,
  timeout_seconds: 30,
  degraded_latency_ms: 5000,
  failure_threshold: 3,
  recovery_threshold: 2,
  status,
  consecutive_failures: 0,
  traffic_request_count: 0,
  buckets,
})

describe('Sub2APIProviderRouteTimeline', () => {
  it('renders one status block per real probe and filters legacy empty slots', () => {
    const wrapper = mount(Sub2APIProviderRouteTimeline, {
      props: {
        route: makeRoute([
          makeBucket(0, 'unknown', 0, 0),
          makeBucket(1, 'healthy', 620),
          makeBucket(2, 'unknown', 0, 0),
          makeBucket(3, 'degraded', 6200),
        ]),
      },
    })

    expect(wrapper.findAll('[data-test="route-timeline-bucket"]')).toHaveLength(2)
    expect(wrapper.text()).toContain('admin.sub2apiProviders.health.routes.recentProbeResults:2')
    expect(wrapper.text()).not.toContain('waitingNextProbe')
    expect(wrapper.text()).not.toContain('noProbeThisCycle')
  })

  it('shows the selected real probe status, timestamp, and latency', async () => {
    const selectedBucket = makeBucket(0, 'healthy', 730)
    const wrapper = mount(Sub2APIProviderRouteTimeline, {
      props: { route: makeRoute([selectedBucket, makeBucket(1, 'degraded', 6200)]) },
    })

    await wrapper.findAll('[data-test="route-timeline-bucket"]')[0].trigger('pointerenter')

    const detail = wrapper.get('[data-test="route-timeline-detail"]').text()
    expect(detail).toContain('admin.sub2apiProviders.health.status.healthy')
    expect(detail).toContain('730 ms')
    expect(detail).toContain(probeTimeFormatter.format(new Date(selectedBucket.ended_at)))
    expect(detail).not.toContain(' - ')
    expect(wrapper.findAll('[data-test="route-timeline-bucket"]')[0].attributes('title')).not.toContain(' - ')
    expect(wrapper.get('[role="img"]').attributes('aria-label')).not.toContain(' - ')
  })

  it('falls back to the probe start time when the completion time is unavailable', () => {
    const bucket = makeBucket(0, 'healthy', 730)
    bucket.ended_at = ''

    const wrapper = mount(Sub2APIProviderRouteTimeline, {
      props: { route: makeRoute([bucket]) },
    })

    const detail = wrapper.get('[data-test="route-timeline-detail"]').text()
    expect(detail).toContain(probeTimeFormatter.format(new Date(bucket.started_at)))
    expect(detail).not.toContain('admin.sub2apiProviders.health.neverChecked')
    expect(detail).not.toContain(' - ')
  })

  it('keeps compact card timelines to status blocks without redundant headings', () => {
    const wrapper = mount(Sub2APIProviderRouteTimeline, {
      props: { route: makeRoute([makeBucket(0)]), compact: true },
    })

    expect(wrapper.findAll('[data-test="route-timeline-bucket"]')).toHaveLength(1)
    expect(wrapper.text()).not.toContain('admin.sub2apiProviders.health.routes.probeHistory')
    expect(wrapper.text()).not.toContain('admin.sub2apiProviders.health.routes.recentProbeResults')
    expect(wrapper.find('[data-test="route-timeline-detail"]').exists()).toBe(false)
  })

  it('keeps keyboard navigation across real probes only', async () => {
    const wrapper = mount(Sub2APIProviderRouteTimeline, {
      props: { route: makeRoute([makeBucket(0, 'healthy', 111), makeBucket(1, 'unhealthy', 222)]) },
    })
    const focusTarget = wrapper.get('[role="img"]')

    expect(wrapper.get('[data-test="route-timeline-detail"]').text()).toContain('222 ms')
    await focusTarget.trigger('keydown', { key: 'ArrowLeft' })
    expect(wrapper.get('[data-test="route-timeline-detail"]').text()).toContain('111 ms')
  })

  it('uses a separate empty state instead of synthetic unknown blocks', () => {
    const wrapper = mount(Sub2APIProviderRouteTimeline, { props: { route: makeRoute([], 'unknown') } })

    expect(wrapper.findAll('[data-test="route-timeline-bucket"]')).toHaveLength(0)
    expect(wrapper.find('[role="img"]').exists()).toBe(false)
    expect(wrapper.get('[data-test="route-timeline-empty"]').text()).toContain(
      'admin.sub2apiProviders.health.routes.noProbeResults'
    )
  })

  it('keeps real history visible after the account probe is disabled', () => {
    const wrapper = mount(Sub2APIProviderRouteTimeline, {
      props: { route: makeRoute([makeBucket(0, 'healthy', 842)], 'disabled') },
    })

    expect(wrapper.findAll('[data-test="route-timeline-bucket"]')).toHaveLength(1)
    expect(wrapper.get('[data-test="route-timeline-detail"]').text()).toContain('842 ms')
    expect(wrapper.text()).toContain('admin.sub2apiProviders.health.routes.recentProbeResults:1')
  })

  it('shows disabled as an empty state when no historical probe exists', () => {
    const wrapper = mount(Sub2APIProviderRouteTimeline, { props: { route: makeRoute([], 'disabled') } })

    expect(wrapper.get('[data-test="route-timeline-empty"]').text()).toContain(
      'admin.sub2apiProviders.health.status.disabled'
    )
  })
})

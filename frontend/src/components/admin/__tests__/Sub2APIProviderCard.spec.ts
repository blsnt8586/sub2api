import { ref } from 'vue'
import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import type { Sub2APIProvider, Sub2APIProviderHealthOverview, Sub2APIProviderProbeTargetHealth, Sub2APIProviderRemoteOverview } from '@/api/admin/sub2apiProviders'
import Sub2APIProviderCard from '../Sub2APIProviderCard.vue'

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

vi.mock('@/utils/format', async () => {
  const actual = await vi.importActual<typeof import('@/utils/format')>('@/utils/format')
  return {
    ...actual,
    formatRelativeTime: (value: string) => `relative:${value}`,
  }
})

const provider: Sub2APIProvider = {
  id: 7,
  name: 'Production Provider',
  base_url: 'https://provider.example.com',
  provider_type: 'sub2api',
  status: 'active',
  notes: 'Primary upstream',
  email: 'admin@example.com',
  api_path_keys: '/api/v1/keys',
  api_path_groups: '/api/v1/groups',
  last_sync_at: '2026-08-14T05:00:00Z',
  last_sync_status: 'success',
  last_sync_error: null,
  created_at: '2026-08-14T04:00:00Z',
  updated_at: '2026-08-14T06:00:00Z',
  accounts_count: 3,
}

const bucketStart = new Date('2026-08-13T06:00:00Z')
const buckets = Array.from({ length: 60 }, (_, index) => ({
  started_at: new Date(bucketStart.getTime() + index * 24 * 60 * 1000).toISOString(),
  ended_at: new Date(bucketStart.getTime() + (index + 1) * 24 * 60 * 1000).toISOString(),
  status: index === 59 ? 'healthy' as const : 'unknown' as const,
  sample_count: index === 59 ? 1 : 0,
  healthy_samples: index === 59 ? 1 : 0,
  degraded_samples: 0,
  unhealthy_samples: 0,
}))
const route = (id: number, accountName: string, platform: string): Sub2APIProviderProbeTargetHealth => ({
  id,
  provider_id: provider.id,
  account_id: id + 100,
  account_name: accountName,
  provider_api_key_id: id + 200,
  remote_group_id: id + 300,
  remote_group_name: `${platform} group`,
  remote_group_multiplier: id === 1 ? 0.4 : 1,
  sub2api_optimize_enabled: true,
  sub2api_min_multiplier: 0.3,
  sub2api_max_multiplier: 0.8,
  platform,
  enabled: true,
  interval_seconds: 1800,
  test_model: `${platform}-test-model`,
  allow_media_probe: false,
  timeout_seconds: 15,
  degraded_latency_ms: 2000,
  failure_threshold: 3,
  recovery_threshold: 2,
  status: id === 1 ? 'healthy' : 'unknown',
  latency_ms: id === 1 ? 240 : null,
  traffic_request_count: 0,
  last_checked_at: id === 1 ? '2026-08-14T05:00:00Z' : null,
  last_run_at: id === 1 ? '2026-08-14T05:00:00Z' : null,
  route_changed_at: null,
  consecutive_failures: 0,
  buckets,
})
const overview: Sub2APIProviderHealthOverview = {
  provider_id: provider.id,
  availability_status: 'healthy',
  evidence_status: 'unknown',
  latest: {
    provider_id: provider.id,
    status: 'unknown',
    control_status: 'healthy',
    data_status: 'unknown',
    traffic_status: 'unknown',
    consecutive_failures: 0,
    data_probe_count: 0,
    data_probe_success: 0,
    data_probe_failed: 0,
    data_probe_enabled: false,
    data_probe_interval_seconds: 1800,
    probe_account_count: 0,
    account_probes: [],
    traffic_request_count: 0,
    last_checked_at: '2026-08-14T05:00:00Z',
  },
  latest_control: {
    provider_id: provider.id,
    status: 'unknown',
    control_status: 'healthy',
    data_status: 'unknown',
    traffic_status: 'unknown',
    consecutive_failures: 0,
    health_latency_ms: 240,
    data_probe_count: 0,
    data_probe_success: 0,
    data_probe_failed: 0,
    data_probe_enabled: false,
    data_probe_interval_seconds: 1800,
    probe_account_count: 0,
    account_probes: [],
    traffic_request_count: 0,
    last_checked_at: '2026-08-14T05:00:00Z',
  },
  window_started_at: '2026-08-13T06:00:00Z',
  window_ended_at: '2026-08-14T06:00:00Z',
  bucket_seconds: 1440,
  buckets,
  summary: { healthy: 1, degraded: 0, unhealthy: 0, unknown: 59 },
  routes: [
    route(1, 'Codex Account', 'openai'),
    route(2, 'Claude Account', 'anthropic'),
  ],
}

const remoteOverview: Sub2APIProviderRemoteOverview = {
  provider_id: provider.id,
  available: true,
  balance: 128.5,
  rate_overrides_available: true,
  sampled_at: '2026-08-14T05:30:00Z',
  source: 'control_probe',
  last_attempted_at: '2026-08-14T05:30:00Z',
  last_attempt_source: 'control_probe',
  groups: [
    {
      id: 10,
      name: 'Economy',
      platform: 'openai',
      default_multiplier: 0.5,
      effective_multiplier: 0.35,
      has_custom_rate: true,
    },
    {
      id: 11,
      name: 'Standard',
      platform: 'anthropic',
      default_multiplier: 1,
      effective_multiplier: 1,
      has_custom_rate: false,
    },
  ],
}

describe('Sub2APIProviderCard', () => {
  it('renders the provider summary as a bounded panel', () => {
    const wrapper = mount(Sub2APIProviderCard, { props: { provider, overview } })

    expect(wrapper.get('article').classes()).toEqual(expect.arrayContaining([
      'min-h-[360px]',
      'rounded-lg',
      'border',
    ]))
    expect(wrapper.get('h2').text()).toBe(provider.name)
    expect(wrapper.text()).toContain('provider.example.com')
    expect(wrapper.text()).toContain(provider.notes)
    expect(wrapper.text()).toContain('relative:2026-08-14T05:00:00Z')
    expect(wrapper.text()).toContain('admin.sub2apiProviders.health.availabilityStatus.healthy')
    expect(wrapper.text()).toContain('admin.sub2apiProviders.apiPathStatus.ready')
    expect(wrapper.text()).toContain('Codex Account')
    expect(wrapper.text()).toContain('Claude Account')
    expect(wrapper.text()).toContain('openai')
    expect(wrapper.text()).toContain('anthropic')
    expect(wrapper.text()).toContain('openai-test-model')
    expect(wrapper.text()).toContain('anthropic-test-model')
    expect(wrapper.text()).toContain('×0.4')
    expect(wrapper.text()).toContain('×1')
    expect(wrapper.get('[data-test="route-multiplier-1"]').classes()).toContain('text-gray-600')
    expect(wrapper.get('[data-test="route-multiplier-1"]').find('svg').exists()).toBe(false)
    expect(wrapper.get('[data-test="route-multiplier-2"]').classes()).toContain('text-red-700')
    expect(wrapper.get('[data-test="route-multiplier-2"]').find('svg').exists()).toBe(true)
    expect(wrapper.get('[data-test="route-multiplier-2"]').attributes('title')).toBe('admin.sub2apiProviders.multiplierRangeAboveDetail')
    // The legacy fixture still contains 60 fixed slots per account, but only
    // one slot per account is backed by a real probe sample.
    expect(wrapper.findAll('[data-test="route-timeline-bucket"]')).toHaveLength(2)
    expect(wrapper.text()).not.toContain('admin.sub2apiProviders.health.routes.probeHistory')
    expect(wrapper.text()).not.toContain('admin.sub2apiProviders.health.routes.recentProbeResults')
    expect(wrapper.find('[data-test="provider-traffic-status"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="provider-health-timeline"]').exists()).toBe(false)
    expect(wrapper.get('article').classes()).toContain('provider-card')
  })

  it('opens account-level probe management without exposing a provider-wide run action', async () => {
    const wrapper = mount(Sub2APIProviderCard, { props: { provider, overview } })

    await wrapper.findAll('button')[0].trigger('click')
    await wrapper.get('[data-test="provider-control-status"]').trigger('click')
    await wrapper.get('[data-test="provider-route-probe-1"]').trigger('click')
    await wrapper.get('[data-test="provider-view-accounts"]').trigger('click')
    await wrapper.get('[data-test="provider-view-logs"]').trigger('click')
    await wrapper.get('[data-test="provider-manage-probes"]').trigger('click')
    await wrapper.get('[data-test="provider-remote-overview"]').trigger('click')

    expect(wrapper.emitted('more')).toHaveLength(1)
    expect(wrapper.emitted('view-health')).toHaveLength(3)
    expect(wrapper.emitted('view-accounts')).toHaveLength(1)
    expect(wrapper.emitted('view-logs')).toHaveLength(1)
    expect(wrapper.emitted('view-remote-overview')).toHaveLength(1)
    expect(wrapper.emitted('run-probe')).toBeUndefined()
    expect(provider.status).toBe('active')
  })

  it('shows a compact remote balance and rate summary after an explicit read', () => {
    const preciseOverview: Sub2APIProviderRemoteOverview = {
      ...remoteOverview,
      groups: remoteOverview.groups.map((group, index) => index === 0
        ? { ...group, effective_multiplier: 0.035 }
        : group),
    }
    const wrapper = mount(Sub2APIProviderCard, { props: { provider, overview, remoteOverview: preciseOverview } })

    const summary = wrapper.get('[data-test="provider-remote-overview"]')
    expect(summary.text()).toContain('128.5')
    expect(summary.text()).toContain('2')
    expect(summary.text()).toContain('×0.035 - ×1')
    expect(wrapper.find('[data-test="provider-remote-overview-status"]').exists()).toBe(false)
    expect(summary.attributes('disabled')).toBeUndefined()
  })

  it('keeps remote overview loading and retry states explicit', () => {
    const loading = mount(Sub2APIProviderCard, {
      props: { provider, overview, remoteOverviewLoading: true },
    })
    expect(loading.get('[data-test="provider-remote-overview"]').attributes('disabled')).toBeDefined()
    expect(loading.text()).toContain('admin.sub2apiProviders.remoteOverview.loading')

    const failed = mount(Sub2APIProviderCard, {
      props: { provider, overview, remoteOverviewError: 'upstream unavailable' },
    })
    expect(failed.text()).toContain('admin.sub2apiProviders.remoteOverview.loadFailed')
    expect(failed.text()).toContain('admin.sub2apiProviders.remoteOverview.retry')
  })

  it('keeps the last successful asset data visible after a later refresh failure', () => {
    const staleOverview: Sub2APIProviderRemoteOverview = {
      ...remoteOverview,
      last_attempted_at: '2026-08-14T05:45:00Z',
      last_attempt_source: 'manual',
      last_error: 'wallet unavailable',
      last_error_at: '2026-08-14T05:45:00Z',
    }
    const wrapper = mount(Sub2APIProviderCard, {
      props: { provider, overview, remoteOverview: staleOverview },
    })

    const summary = wrapper.get('[data-test="provider-remote-overview"]')
    expect(summary.text()).toContain('128.5')
    expect(summary.text()).toContain('×0.35 - ×1')
    expect(wrapper.find('[data-test="provider-remote-overview-status"]').exists()).toBe(false)
    expect(summary.text()).not.toContain('wallet unavailable')
  })

  it('shows refresh progress without replacing an existing asset snapshot', () => {
    const wrapper = mount(Sub2APIProviderCard, {
      props: { provider, overview, remoteOverview, remoteOverviewLoading: true },
    })

    expect(wrapper.text()).toContain('128.5')
    expect(wrapper.text()).not.toContain('admin.sub2apiProviders.remoteOverview.refreshing')
    expect(wrapper.get('[data-test="provider-remote-overview"]').attributes('disabled')).toBeDefined()
  })

  it('prioritizes a compact account preview and links to the full account panel', () => {
    const routes = Array.from({ length: 12 }, (_, index) => route(index + 1, `Account ${index + 1}`, index % 2 ? 'anthropic' : 'openai'))
    const wrapper = mount(Sub2APIProviderCard, {
      props: {
        provider: { ...provider, accounts_count: routes.length },
        overview: { ...overview, routes },
      },
    })

    expect(wrapper.find('[data-test="provider-route-scroll"]').exists()).toBe(false)
    expect(wrapper.findAll('[data-test^="provider-route-probe-"]')).toHaveLength(3)
    expect(wrapper.get('[data-test="provider-view-all-routes"]').exists()).toBe(true)
  })

  it('keeps multiplier metadata neutral unless an enabled optimization range is exceeded', () => {
    const disabledOptimization = {
      ...route(3, 'Disabled optimization', 'openai'),
      sub2api_optimize_enabled: false,
      remote_group_multiplier: 1.2,
    }
    const unconfiguredRange = {
      ...route(4, 'Unconfigured range', 'anthropic'),
      sub2api_min_multiplier: null,
      sub2api_max_multiplier: null,
      remote_group_multiplier: 1.2,
    }
    const wrapper = mount(Sub2APIProviderCard, {
      props: { provider, overview: { ...overview, routes: [disabledOptimization, unconfiguredRange] } },
    })

    for (const id of [3, 4]) {
      const badge = wrapper.get(`[data-test="route-multiplier-${id}"]`)
      expect(badge.classes()).toContain('text-gray-600')
      expect(badge.find('svg').exists()).toBe(false)
    }
    expect(wrapper.get('[data-test="route-multiplier-3"]').attributes('title')).toBe('admin.sub2apiProviders.multiplierOptimizationDisabled')
    expect(wrapper.get('[data-test="route-multiplier-4"]').attributes('title')).toBe('admin.sub2apiProviders.multiplierRangeUnconfigured')
  })

  it('shows fallback states when route and control data are unavailable', () => {
    const wrapper = mount(Sub2APIProviderCard, {
      props: {
        provider: {
          ...provider,
          status: 'inactive',
          notes: null,
          api_path_keys: null,
          api_path_groups: null,
          last_sync_at: null,
          last_sync_status: null,
        },
      },
    })

    expect(wrapper.text()).not.toContain('admin.sub2apiProviders.noNotes')
    expect(wrapper.text()).toContain('admin.sub2apiProviders.apiPathStatus.notDetected')
    expect(wrapper.text()).toContain('admin.sub2apiProviders.syncStatus.never')
    expect(wrapper.text()).toContain('admin.sub2apiProviders.health.routes.empty')
    expect(wrapper.get('[data-test="provider-probe-paused"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="provider-control-status"]').exists()).toBe(false)
  })
})

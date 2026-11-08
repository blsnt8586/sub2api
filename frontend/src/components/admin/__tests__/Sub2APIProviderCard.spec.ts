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
  average_duration_ms: id === 1 ? 1250 : null,
  cache_hit_rate: id === 1 ? 0.501 : null,
  cache_hit_rate_available: id === 1,
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
  usage: {
    total_recharged: 500,
    order_recharged: 450,
    redeem_recharged: 50,
    total_consumed: 491.5,
    total_requests: 100,
    total_input_tokens: 10000,
    total_output_tokens: 2000,
    total_cache_creation_tokens: 500,
    total_cache_read_tokens: 1500,
    total_tokens: 14000,
    total_cost: 20,
    total_actual_cost: 18.5,
    today_requests: 4,
    today_input_tokens: 400,
    today_output_tokens: 100,
    today_cache_creation_tokens: 20,
    today_cache_read_tokens: 80,
    today_tokens: 600,
    today_cost: 1.2,
    today_actual_cost: 1,
    average_duration_ms: 1250,
    cache_hit_rate: 0.501,
    cache_hit_rate_available: true,
    funding_summary_available: true,
    funding_summary_source: 'orders_and_redeems',
    dashboard_available: true,
  },
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
    expect(wrapper.get('h2').text()).toContain(provider.name)
    expect(wrapper.get('[data-test="provider-upstream-link"]').text()).toContain(provider.name)
    expect(wrapper.text()).not.toContain('provider.example.com')
    expect(wrapper.text()).toContain(provider.notes)
    expect(wrapper.text()).toContain('admin.sub2apiProviders.health.availabilityStatus.healthy')
    expect(wrapper.get('[data-test="provider-control-status"]').text()).not.toContain('relative:')
    expect(wrapper.text()).toContain('admin.sub2apiProviders.apiPathStatus.ready')
    expect(wrapper.text()).not.toContain('admin.sub2apiProviders.accountSyncLabel')
    expect(wrapper.text()).toContain('Codex Account')
    expect(wrapper.text()).toContain('Claude Account')
    expect(wrapper.text()).toContain('openai')
    expect(wrapper.text()).toContain('anthropic')
    expect(wrapper.text()).not.toContain('openai-test-model')
    expect(wrapper.text()).not.toContain('anthropic-test-model')
    expect(wrapper.text()).toContain('×0.4')
    expect(wrapper.text()).toContain('×1')
    expect(wrapper.get('[data-test="route-multiplier-1"]').classes()).toContain('text-emerald-700')
    expect(wrapper.get('[data-test="route-multiplier-1"]').find('svg').exists()).toBe(false)
    expect(wrapper.get('[data-test="route-multiplier-2"]').classes()).toContain('text-red-700')
    expect(wrapper.get('[data-test="route-multiplier-2"]').find('svg').exists()).toBe(true)
    expect(wrapper.get('[data-test="route-multiplier-2"]').attributes('title')).toBe('admin.sub2apiProviders.multiplierRangeAboveDetail')
    expect(wrapper.get('[data-test="provider-route-probe-1"] .route-platform').classes()).toContain('bg-emerald-100')
    expect(wrapper.get('[data-test="provider-route-probe-2"] .route-platform').classes()).toContain('bg-orange-100')
    expect(wrapper.get('[data-test="route-optimize-1"]').attributes('aria-label')).toBe('admin.sub2apiProviders.joinScheduleOn')
    const controlStatus = wrapper.get('[data-test="provider-control-status"]').element
    expect(wrapper.get('header').element.contains(controlStatus)).toBe(true)
    expect(wrapper.get('[data-test="provider-control-status"]').classes()).toContain('h-8')
    expect(wrapper.get('[data-test="provider-type-tag"]').classes()).toContain('h-8')
    expect(wrapper.get('[aria-label="Production Provider common.more"]').classes()).toContain('h-8')
    // The legacy fixture still contains 60 fixed slots per account, but only
    // one slot per account is backed by a real probe sample.
    expect(wrapper.findAll('[data-test="route-timeline-bucket"]')).toHaveLength(2)
    expect(wrapper.text()).not.toContain('admin.sub2apiProviders.health.routes.probeHistory')
    expect(wrapper.text()).not.toContain('admin.sub2apiProviders.health.routes.recentProbeResults')
    expect(wrapper.find('[data-test="provider-traffic-status"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="provider-health-timeline"]').exists()).toBe(false)
    expect(wrapper.get('article').classes()).toContain('provider-card')
  })

  it('renders the upstream host as a safe external navigation button', () => {
    const wrapper = mount(Sub2APIProviderCard, { props: { provider, overview } })
    const link = wrapper.get('[data-test="provider-upstream-link"]')

    expect(link.attributes('href')).toBe(provider.base_url + '/')
    expect(link.attributes('target')).toBe('_blank')
    expect(link.attributes('rel')).toContain('noopener')
    expect(link.find('svg').exists()).toBe(true)
  })

  it('shows account status separately from route health', () => {
    const unhealthy = { ...route(1, 'Paused account', 'openai'), account_status: 'error', status: 'healthy' as const }
    const wrapper = mount(Sub2APIProviderCard, {
      props: { provider, overview: { ...overview, routes: [unhealthy] } },
    })
    expect(wrapper.text()).toContain('admin.accounts.status.error')
    expect(wrapper.text()).toContain('240 ms')
  })

  it('does not create an external link for an invalid provider URL', () => {
    const wrapper = mount(Sub2APIProviderCard, {
      props: { provider: { ...provider, base_url: 'javascript:alert(1)' }, overview },
    })

    expect(wrapper.find('[data-test="provider-upstream-link"]').exists()).toBe(false)
  })

  it('opens account-level probe management without exposing a provider-wide run action', async () => {
    const wrapper = mount(Sub2APIProviderCard, { props: { provider, overview } })

    await wrapper.get('[aria-label="Production Provider common.more"]').trigger('click')
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

  it('shows the remote usage summary without groups or multiplier range', () => {
    const preciseOverview: Sub2APIProviderRemoteOverview = {
      ...remoteOverview,
      groups: remoteOverview.groups.map((group, index) => index === 0
        ? { ...group, effective_multiplier: 0.035 }
        : group),
    }
    const wrapper = mount(Sub2APIProviderCard, { props: { provider, overview, remoteOverview: preciseOverview } })

    const summary = wrapper.get('[data-test="provider-remote-overview"]')
    expect(summary.text()).toContain('128.5')
    expect(summary.text()).toContain('500.00')
    expect(summary.text()).not.toContain('admin.sub2apiProviders.remoteOverview.fundingBreakdown')
    expect(summary.findAll('[data-test="remote-metric-detail"]')).toHaveLength(0)
    expect(summary.text()).not.toContain('admin.sub2apiProviders.remoteOverview.totalConsumed')
    expect(summary.text()).toContain('4')
    expect(summary.text()).toContain('1.00')
    expect(summary.text()).toContain('600')
    expect(summary.text()).toContain('14.0K')
    expect(summary.find('[data-test="remote-metric-cache-hit-rate"]').exists()).toBe(false)
    expect(summary.find('[data-test="remote-metric-average-response"]').exists()).toBe(false)
    expect(wrapper.findAll('[data-test="route-traffic-metrics"]')).toHaveLength(1)
    expect(wrapper.get('[data-test="route-traffic-metrics"]').text()).toContain('50.1%')
    expect(wrapper.get('[data-test="route-traffic-metrics"]').text()).toContain('1.25s')
    expect(summary.text()).not.toContain('admin.sub2apiProviders.remoteOverview.groups')
    expect(summary.text()).not.toContain('admin.sub2apiProviders.remoteOverview.rateRange')
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

  it('shows local request profit in three fixed rows even without remote wallet data', () => {
    const localOverview: Sub2APIProviderRemoteOverview = {
      ...remoteOverview,
      available: false,
      balance: 999999,
      usage: null,
      local_usage: { ...remoteOverview.usage!, usage_source: 'local', funding_summary_available: false },
      profit: {
        available: true, cost_source: 'local_request_logs', range_start: '', range_end: '', sampled_at: '',
        total_revenue: 30, total_remote_cost: 0, total_estimated_cost: 15,
        total_gross_profit: 15, total_gross_margin: 0.5,
        today_revenue: 2, today_remote_cost: 0, today_estimated_cost: 1,
        today_gross_profit: 1, today_gross_margin: 0.5,
        month_gross_profit: -2.5, month_gross_margin: -0.25, month_profit_available: true,
        cumulative_gross_profit: 15, cumulative_gross_margin: 0.5, cumulative_profit_available: true,
        accounts: [],
      },
    }
    const wrapper = mount(Sub2APIProviderCard, { props: { provider, remoteOverview: localOverview } })
    const metrics = wrapper.get('[data-test="provider-remote-metrics"]')
    expect(metrics.find('span.grid').classes()).toContain('grid-cols-4')
    expect(metrics.findAll('[data-test^="remote-metric-"]').filter(item => item.attributes('data-test') !== 'remote-metric-detail')
      .map(item => item.attributes('data-test'))).toEqual([
      'remote-metric-today-consumed', 'remote-metric-today-tokens', 'remote-metric-total-recharged', 'remote-metric-total-tokens',
      'remote-metric-balance',
      'remote-metric-today-profit', 'remote-metric-month-profit', 'remote-metric-cumulative-profit',
    ])
    expect(metrics.get('[data-test="remote-metric-balance"]').text()).not.toContain('999,999')
    expect(metrics.get('[data-test="remote-metric-total-recharged"]').text()).toContain('-')
    expect(metrics.get('[data-test="remote-metric-today-profit"]').text()).toContain('$1.00')
    expect(metrics.get('[data-test="remote-metric-month-profit"]').text()).toContain('$-2.50')
    expect(metrics.get('[data-test="remote-metric-cumulative-profit"]').text()).toContain('$15.00')
    expect(metrics.text()).not.toContain('admin.sub2apiProviders.remoteOverview.localUsageSource')
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
    expect(summary.text()).toContain('500.00')
    expect(summary.text()).not.toContain('admin.sub2apiProviders.remoteOverview.groups')
    expect(summary.text()).not.toContain('admin.sub2apiProviders.remoteOverview.rateRange')
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

  it('shows every account probe in the provider pane while keeping abnormal accounts first', () => {
    const routes = Array.from({ length: 12 }, (_, index) => route(index + 1, `Account ${index + 1}`, index % 2 ? 'anthropic' : 'openai'))
    routes[7] = { ...routes[7], status: 'unhealthy', latency_ms: 4800 }
    const wrapper = mount(Sub2APIProviderCard, {
      props: {
        provider: { ...provider, accounts_count: routes.length },
        overview: { ...overview, routes },
      },
    })

    expect(wrapper.find('[data-test="provider-route-scroll"]').exists()).toBe(false)
    expect(wrapper.findAll('[data-test^="provider-route-probe-"]')).toHaveLength(routes.length)
    expect(wrapper.find('[data-test="provider-view-all-routes"]').exists()).toBe(false)
    expect(wrapper.findAll('[data-test^="provider-route-probe-"]')[0].attributes('data-test')).toBe('provider-route-probe-8')
    for (const item of routes) {
      expect(wrapper.find(`[data-test="provider-route-probe-${item.id}"]`).exists()).toBe(true)
    }
  })

  it('uses semantic multiplier colors for disabled and unconfigured optimization ranges', () => {
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
      expect(badge.classes()).toContain(id === 3 ? 'text-slate-600' : 'text-sky-700')
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
    expect(wrapper.text()).not.toContain('admin.sub2apiProviders.accountSyncLabel')
    expect(wrapper.text()).toContain('admin.sub2apiProviders.health.routes.empty')
    expect(wrapper.get('[data-test="provider-probe-paused"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="provider-control-status"]').exists()).toBe(false)
  })
})

it('shows New API identity and leaves unknown funding unavailable', () => {
  const wrapper = mount(Sub2APIProviderCard, {
    props: {
      provider: { ...provider, provider_type: 'newapi', auth_mode: 'access_token', has_access_token: true },
      remoteOverview: { ...remoteOverview, usage: { ...remoteOverview.usage!, funding_summary_available: false, total_recharged: 0, total_consumed: 1.5 } },
    },
  })
  expect(wrapper.text()).toContain('New API')
  expect(wrapper.text()).toContain('admin.sub2apiProviders.form.authManagementToken')
  expect(wrapper.get('[data-test="remote-metric-total-recharged"]').text()).toContain('-')
  expect(wrapper.find('[data-test="remote-metric-total-consumed"]').exists()).toBe(false)
})

import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import type { Sub2APIProviderRemoteOverview } from '@/api/admin/sub2apiProviders'
import zh from '@/i18n/locales/zh/admin/sub2apiProviders'
import Sub2APIProviderRemoteDetails from '../Sub2APIProviderRemoteDetails.vue'

function overview(): Sub2APIProviderRemoteOverview {
  return {
    provider_id: 18, available: true, balance: 12.3,
    rate_overrides_available: true, sampled_at: '2026-11-03T08:00:00Z', source: 'manual',
    last_attempted_at: '2026-11-03T08:00:00Z', last_attempt_source: 'manual',
    groups: [
      { id: 1, name: 'Claude', platform: 'anthropic', description: 'Fast', effective_multiplier: 0.049, default_multiplier: 0.1, has_custom_rate: true },
      { id: 2, name: 'Codex', platform: 'openai', effective_multiplier: 0.2, default_multiplier: 0.2, has_custom_rate: false },
    ],
    profit: {
      available: true, cost_source: 'local_request_logs', range_start: '', range_end: '', sampled_at: '2026-11-03T08:02:00Z',
      total_revenue: 100, total_remote_cost: 0, total_estimated_cost: 50, total_gross_profit: 50, total_gross_margin: 0.5,
      today_revenue: 10, today_remote_cost: 0, today_estimated_cost: 12, today_gross_profit: -2, today_gross_margin: -0.2,
      month_gross_profit: 20, month_gross_margin: 0.4, cumulative_gross_profit: 50, cumulative_gross_margin: 0.5,
      month_profit_available: true, cumulative_profit_available: true,
      accounts: [{
        account_id: 396, account_name: '乔然 AI', revenue: 100, remote_cost: 0, estimated_cost: 50, gross_profit: 50, gross_margin: 0.5,
        today_revenue: 10, today_remote_cost: 0, today_estimated_cost: 12, today_gross_profit: -2, today_gross_margin: -0.2,
        remote_cost_available: false, estimated_cost_available: true, cost_source: 'local_request_logs',
      }],
    },
  }
}
function render(data = overview()) {
  return mount(Sub2APIProviderRemoteDetails, {
    props: { show: true, providerId: 18, providerName: '乔然', overview: data, localLoading: false, remoteLoading: false, upstreamEnabled: true },
    global: {
      plugins: [createI18n({ legacy: false, locale: 'zh', messages: { zh: { admin: zh, common: { close: '关闭' } } } })],
      stubs: { BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' } },
    },
  })
}

describe('Sub2APIProviderRemoteDetails', () => {
  it('shows local profit and losses without a remote asset snapshot', () => {
    const data = overview()
    data.available = false
    const wrapper = render(data)
    expect(wrapper.text()).toContain('$-2.00')
    expect(wrapper.text()).toContain('$50.00')
    expect(wrapper.text()).toContain('尚无上游资产快照')
    expect(wrapper.get('tbody').text()).toContain('乔然 AI')
    expect(wrapper.emitted('sync-upstream')).toBeUndefined()
  })

  it('switches all account metrics together between today and cumulative', async () => {
    const wrapper = render()
    const cells = () => wrapper.findAll('tbody td').map(cell => cell.text())
    expect(cells()).toEqual(['$10.00', '$12.00', '$-2.00', '-20.0%'])
    const cumulative = wrapper.findAll('button').find(button => button.text() === '累计')!
    await cumulative.trigger('click')
    expect(cells()).toEqual(['$100.00', '$50.00', '$50.00', '50.0%'])
    expect(cumulative.attributes('aria-pressed')).toBe('true')
    await wrapper.setProps({ providerId: 19 })
    expect(cells()).toEqual(['$10.00', '$12.00', '$-2.00', '-20.0%'])
  })

  it('keeps unknown cost distinct from zero while preserving known revenue', () => {
    const data = overview()
    data.profit!.available = false
    data.profit!.accounts[0]!.estimated_cost_available = false
    const wrapper = render(data)
    expect(wrapper.text()).toContain('利润汇总暂不可用')
    expect(wrapper.findAll('tbody td').map(cell => cell.text())).toEqual(['$10.00', '—', '—', '—'])
    expect(wrapper.get('tbody').text()).toContain('缺少上游倍率')
    expect(wrapper.text()).not.toContain('$50.00')
  })

  it('filters groups by platform and description and keeps multiplier precision', async () => {
    const wrapper = render()
    await wrapper.get('input[type="search"]').setValue(' ANTHROPIC ')
    expect(wrapper.text()).toContain('×0.049')
    expect(wrapper.text()).not.toContain('Codex')
    await wrapper.get('input[type="search"]').setValue('fast')
    expect(wrapper.text()).toContain('Claude')
    await wrapper.get('input[type="search"]').setValue('absent')
    expect(wrapper.text()).toContain('没有匹配的分组')
  })

  it('separates local refresh from upstream sync and disables busy actions', async () => {
    const wrapper = render()
    const local = wrapper.findAll('button').find(button => button.text() === '刷新利润')!
    const remote = wrapper.findAll('button').find(button => button.text() === '同步上游资产')!
    await local.trigger('click')
    expect(wrapper.emitted('refresh-local')).toHaveLength(1)
    expect(wrapper.emitted('sync-upstream')).toBeUndefined()
    await remote.trigger('click')
    expect(wrapper.emitted('sync-upstream')).toHaveLength(1)
    await wrapper.setProps({ localLoading: true, remoteLoading: true })
    expect(local.attributes('disabled')).toBeDefined()
    expect(remote.attributes('disabled')).toBeDefined()
    await wrapper.setProps({ remoteLoading: false, upstreamEnabled: false })
    expect(remote.attributes('disabled')).toBeDefined()
  })

  it('retains local profit and distinct timestamps when upstream sync fails', async () => {
    const wrapper = render()
    await wrapper.setProps({ remoteError: 'remote timeout' })
    expect(wrapper.get('[role="status"]').text()).toContain('上游资产最近同步失败')
    expect(wrapper.get('tbody').text()).toContain('$-2.00')
    expect(wrapper.text()).toContain('统计更新于')
    expect(wrapper.text()).toContain('资产同步于')
  })
})

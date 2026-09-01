import { ref } from 'vue'
import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import Sub2APIProviderActionMenu from '../Sub2APIProviderActionMenu.vue'
import type { Sub2APIProvider } from '@/api/admin/sub2apiProviders'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
    locale: ref('zh-CN'),
  }),
}))

const provider: Sub2APIProvider = {
  id: 7,
  name: 'Provider',
  base_url: 'https://provider.example.com',
  provider_type: 'sub2api',
  status: 'active',
  notes: null,
  proxy_id: null,
  email: 'admin@example.com',
  auth_mode: 'password',
  has_access_token: false,
  has_refresh_token: false,
  created_at: '2026-08-26T00:00:00Z',
  updated_at: '2026-08-26T00:00:00Z',
  accounts_count: 0,
}

describe('Sub2APIProviderActionMenu', () => {
  it('exposes every menu action through a typed event', async () => {
    const wrapper = mount(Sub2APIProviderActionMenu, {
      props: { show: true, provider, position: { top: 10, left: 10 } },
      attachTo: document.body,
    })

    const buttons = Array.from(document.body.querySelectorAll('button'))
    expect(buttons).toHaveLength(6)
    for (const button of buttons) button.click()

    expect(wrapper.emitted('edit')).toHaveLength(1)
    expect(wrapper.emitted('toggle-status')).toHaveLength(1)
    expect(wrapper.emitted('detect-paths')).toHaveLength(1)
    expect(wrapper.emitted('probe-settings')).toHaveLength(1)
    expect(wrapper.emitted('optimize-all')).toHaveLength(1)
    expect(wrapper.emitted('delete')).toHaveLength(1)
  })
})

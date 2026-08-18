import { createPinia, setActivePinia } from 'pinia'
import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import Sub2APICredentialBundleImport from '../Sub2APICredentialBundleImport.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) =>
        params ? `${key}:${JSON.stringify(params)}` : key,
    }),
  }
})

function credentialBundle(origin = 'https://mdkj.lol') {
  return JSON.stringify({
    format: 'sub2api-browser-credentials',
    version: 1,
    source: { origin, captured_at: '2026-08-17T03:00:00.000Z' },
    local_storage: {
      auth_token: 'browser-access-token',
      refresh_token: 'browser-refresh-token',
      auth_user: { email: 'browser@example.com' },
    },
    cookie_capture: {
      method: 'GM_cookie.list',
      items: [
        { name: 'session', value: 'sensitive-cookie', httpOnly: true },
        { name: 'theme', value: 'dark', httpOnly: false },
      ],
    },
  })
}

function credentialBundleWithoutCookies(origin = 'https://mdkj.lol') {
  const bundle = JSON.parse(credentialBundle(origin))
  bundle.cookie_capture.items = []
  return JSON.stringify(bundle)
}

describe('Sub2APICredentialBundleImport', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('emits only normalized token data and clears the raw cookie bundle', async () => {
    const wrapper = mount(Sub2APICredentialBundleImport)

    await wrapper.get('button').trigger('click')
    const input = wrapper.get<HTMLTextAreaElement>('[data-test="credential-bundle-input"]')
    await input.setValue(credentialBundle())
    await wrapper.get('[data-test="credential-bundle-apply"]').trigger('click')

    const imported = wrapper.emitted('imported')?.[0]?.[0] as Record<string, unknown>
    expect(imported).toMatchObject({
      accessToken: 'browser-access-token',
      refreshToken: 'browser-refresh-token',
      email: 'browser@example.com',
      sourceOrigin: 'https://mdkj.lol',
      cookieCount: 2,
      httpOnlyCookieCount: 1,
    })
    expect(imported).not.toHaveProperty('cookies')
    expect(imported).not.toHaveProperty('cookieItems')
    expect(input.element.value).toBe('')
    expect(wrapper.get('[data-test="credential-bundle-summary"]').text()).toContain(
      'admin.sub2apiProviders.form.credentialCookieSummary:{"count":2,"httpOnly":1}'
    )
  })

  it('keeps the bundle for correction when its origin does not match the provider', async () => {
    const wrapper = mount(Sub2APICredentialBundleImport, {
      props: { expectedBaseUrl: 'https://o10.top/keys' },
    })

    await wrapper.get('button').trigger('click')
    const input = wrapper.get<HTMLTextAreaElement>('[data-test="credential-bundle-input"]')
    await input.setValue(credentialBundle())
    await wrapper.get('[data-test="credential-bundle-apply"]').trigger('click')

    expect(wrapper.emitted('imported')).toBeUndefined()
    expect(input.element.value).toContain('sub2api-browser-credentials')
    expect(wrapper.find('[data-test="credential-bundle-summary"]').exists()).toBe(false)
  })

  it('imports the token pair when no readable cookies were captured', async () => {
    const wrapper = mount(Sub2APICredentialBundleImport)

    await wrapper.get('button').trigger('click')
    await wrapper.get<HTMLTextAreaElement>('[data-test="credential-bundle-input"]')
      .setValue(credentialBundleWithoutCookies())
    await wrapper.get('[data-test="credential-bundle-apply"]').trigger('click')

    expect(wrapper.emitted('imported')?.[0]?.[0]).toMatchObject({
      accessToken: 'browser-access-token',
      refreshToken: 'browser-refresh-token',
      cookieCount: 0,
      httpOnlyCookieCount: 0,
    })
    expect(wrapper.get('[data-test="credential-cookie-none"]').text()).toContain(
      'admin.sub2apiProviders.form.credentialCookieNone'
    )
  })
})

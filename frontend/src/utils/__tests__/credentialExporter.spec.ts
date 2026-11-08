import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises } from '@vue/test-utils'
import { parseSub2APICredentialBundle } from '../sub2apiCredentialBundle'

const script = readFileSync(resolve('public/sub2api-credential-exporter.user.js'), 'utf8')
const fetchMock = vi.fn()
const clipboard = vi.fn()
const menu = vi.fn()
const cookies = vi.fn()

function startScript() {
  new Function('GM_addStyle', 'GM_cookie', 'GM_notification', 'GM_registerMenuCommand', 'GM_setClipboard', 'fetch', script)(
    vi.fn(), { list: cookies }, vi.fn(), menu, clipboard, fetchMock
  )
}

async function openNewAPI() {
  let refreshCookie = 'old-refresh-cookie'
  cookies.mockImplementation((_options, callback) => {
    callback([
      { name: 'new_api_refresh', value: 'wrong-path-cookie', httpOnly: true, domain: window.location.hostname, hostOnly: false, path: '/', secure: false },
      { name: 'new_api_refresh', value: refreshCookie, httpOnly: true, domain: window.location.hostname, hostOnly: false, path: '/api/user/auth', secure: false },
    ])
  })
  fetchMock.mockImplementation(async (path: string) => {
    if (path === '/api/user/auth/refresh') refreshCookie = 'refresh-cookie'
    return {
      ok: true, json: async () => path === '/api/status'
        ? { success: true, data: { quota_per_unit: 500000 } }
        : path === '/api/user/auth/refresh'
          ? { success: true, data: { access_token: 'browser-session-token', user: { id: 42 } } }
          : { success: true, data: { id: 42 } },
    }
  })
  localStorage.setItem('user', JSON.stringify({ id: 42 }))
  vi.spyOn(window, 'confirm').mockReturnValue(true)
  startScript()
  await flushPromises()
}

describe('credential exporter userscript', () => {
  beforeEach(() => {
    document.body.innerHTML = ''
    localStorage.clear()
    vi.useFakeTimers()
    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.clearAllTimers()
    vi.useRealTimers()
    vi.restoreAllMocks()
    document.body.innerHTML = ''
  })

  it('exports the current New API browser session without asking for a management token', async () => {
    await openNewAPI()
    document.querySelector<HTMLButtonElement>('#sub2api-credential-exporter-button')!.click()
    await flushPromises()

    expect(fetchMock).toHaveBeenCalledWith('/api/user/auth/refresh', expect.objectContaining({
      method: 'POST', credentials: 'include',
    }))
    expect(fetchMock).toHaveBeenLastCalledWith('/api/user/self', expect.objectContaining({
      credentials: 'omit', redirect: 'error',
      headers: { Accept: 'application/json', Authorization: 'Bearer browser-session-token', 'New-Api-User': '42' },
    }))
    const raw = document.querySelector<HTMLTextAreaElement>('.sub2api-export-json')!.value
    expect(parseSub2APICredentialBundle(raw, window.location.origin, 'newapi')).toMatchObject({
      providerType: 'newapi', userID: '42', accessToken: 'browser-session-token', refreshToken: 'refresh-cookie',
    })
    expect(cookies).toHaveBeenCalledWith({ url: `${window.location.origin}/api/user/auth/refresh` }, expect.any(Function))
    expect(document.body.textContent).not.toContain('browser-session-token')
    document.querySelector<HTMLButtonElement>('.sub2api-copy-button')!.click()
    await flushPromises()
    expect(clipboard).toHaveBeenCalledWith(raw, 'text')
  })

  it('supports Promise-based GM_cookie.list implementations', async () => {
    await openNewAPI()
    const originalFetch = fetchMock.getMockImplementation()!
    let currentCookie = 'old-refresh-cookie'
    cookies.mockImplementation((_options: unknown, _callback: unknown) => Promise.resolve([
      { name: 'new_api_refresh', value: currentCookie, httpOnly: true, domain: window.location.hostname, hostOnly: false, path: '/api/user/auth', secure: false },
    ]))
    fetchMock.mockImplementation((path: string, options: unknown) => {
      if (path === '/api/user/auth/refresh') currentCookie = 'fresh-refresh-cookie'
      return originalFetch(path, options)
    })
    document.querySelector<HTMLButtonElement>('#sub2api-credential-exporter-button')!.click()
    await flushPromises()
    await vi.advanceTimersByTimeAsync(1200)
    await flushPromises()
    expect(document.querySelector('.sub2api-export-json')).not.toBeNull()
  })

  it('retries a New API refresh race and accepts a successful refresh without cookie rotation', async () => {
    await openNewAPI()
    let refreshCalls = 0
    cookies.mockImplementation((_options: unknown, callback: (cookies: unknown[]) => void) => callback([
      { name: 'new_api_refresh', value: 'stable-refresh-cookie', httpOnly: true, domain: window.location.hostname, hostOnly: false, path: '/api/user/auth', secure: false },
    ]))
    fetchMock.mockImplementation(async (path: string) => {
      if (path === '/api/status') return { ok: true, status: 200, json: async () => ({ success: true, data: { quota_per_unit: 500000 } }) }
      if (path === '/api/user/auth/refresh') {
        refreshCalls += 1
        if (refreshCalls === 1) {
          return { ok: false, status: 409, json: async () => ({ success: false, code: 'AUTH_REFRESH_RACE' }) }
        }
        return {
          ok: true, status: 200,
          json: async () => ({ success: true, data: { access_token: 'browser-session-token', access_expires_at: 1_800_000_000, user: { id: 42 } } }),
        }
      }
      return { ok: true, status: 200, json: async () => ({ success: true, data: { id: 42 } }) }
    })
    document.querySelector<HTMLButtonElement>('#sub2api-credential-exporter-button')!.click()
    await flushPromises()
    await vi.advanceTimersByTimeAsync(1600)
    await flushPromises()

    expect(refreshCalls).toBe(2)
    const raw = document.querySelector<HTMLTextAreaElement>('.sub2api-export-json')!.value
    expect(parseSub2APICredentialBundle(raw, window.location.origin, 'newapi')).toMatchObject({
      userID: '42', accessToken: 'browser-session-token', refreshToken: 'stable-refresh-cookie',
    })
  })

  it.each([
    { ok: true, success: true, data: { access_token: 'token', user: { id: 43 } } },
    { ok: true, success: false, data: { access_token: 'token', user: { id: 42 } } },
    { ok: false, success: true, data: { access_token: 'token', user: { id: 42 } } },
  ])('refuses rejected browser sessions or another user identity: %j', async result => {
    await openNewAPI()
    fetchMock.mockResolvedValue({ ok: result.ok, json: async () => result })
    document.querySelector<HTMLButtonElement>('#sub2api-credential-exporter-button')!.click()
    await flushPromises()
    expect(document.querySelector('.sub2api-export-json')).toBeNull()
    expect(document.querySelector<HTMLButtonElement>('#sub2api-credential-exporter-button')!.disabled).toBe(false)
  })

  it('refuses a non-renewable export when HttpOnly cookies are unreadable', async () => {
    await openNewAPI()
    cookies.mockImplementation((_options, callback) => callback([]))
    document.querySelector<HTMLButtonElement>('#sub2api-credential-exporter-button')!.click()
    await flushPromises()
    expect(document.querySelector('.sub2api-export-json')).toBeNull()
    expect(document.querySelector<HTMLButtonElement>('#sub2api-credential-exporter-button')!.disabled).toBe(false)
  })

  it('keeps the existing Sub2API token-pair and cookie export compatible', async () => {
    localStorage.setItem('auth_token', 'access-token')
    localStorage.setItem('refresh_token', 'refresh-token')
    localStorage.setItem('auth_user', JSON.stringify({ email: 'test@example.com' }))
    fetchMock.mockResolvedValue({ ok: true, json: async () => ({ code: 0, data: { version: '0.2.16' } }) })
    cookies.mockImplementation((_options, callback) => callback([{ name: 'session', value: 'cookie-value', httpOnly: true }]))
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    startScript()
    await flushPromises()
    document.querySelector<HTMLButtonElement>('#sub2api-credential-exporter-button')!.click()
    await flushPromises()
    const raw = document.querySelector<HTMLTextAreaElement>('.sub2api-export-json')!.value
    expect(parseSub2APICredentialBundle(raw, window.location.origin, 'sub2api')).toMatchObject({
      accessToken: 'access-token', refreshToken: 'refresh-token', cookieCount: 1,
    })
  })

  it('does not mount an export button on an unrelated site', async () => {
    fetchMock.mockResolvedValue({ ok: true, json: async () => ({ success: true, data: {} }) })
    startScript()
    await flushPromises()
    expect(document.querySelector('#sub2api-credential-exporter-button')).toBeNull()
  })
})

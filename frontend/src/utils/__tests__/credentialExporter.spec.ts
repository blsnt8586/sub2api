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
  fetchMock.mockImplementation(async (path: string) => ({
    ok: true, json: async () => path === '/api/status'
      ? { success: true, data: { quota_per_unit: 500000 } }
      : { success: true, data: { id: 42 } },
  }))
  localStorage.setItem('user', JSON.stringify({ id: 42 }))
  startScript()
  await flushPromises()
  document.querySelector<HTMLButtonElement>('#sub2api-credential-exporter-button')!.click()
  await flushPromises()
  return document.querySelector<HTMLFormElement>('form')!
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

  it('verifies the New API management token without browser cookies and produces an importable bundle', async () => {
    const form = await openNewAPI()
    expect((form.elements.namedItem('user_id') as HTMLInputElement).value).toBe('42')
    ;(form.elements.namedItem('access_token') as HTMLInputElement).value = 'management-token'
    form.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }))
    await flushPromises()

    expect(fetchMock).toHaveBeenLastCalledWith('/api/user/self', expect.objectContaining({
      credentials: 'omit', redirect: 'error',
      headers: { Accept: 'application/json', Authorization: 'Bearer management-token', 'New-Api-User': '42' },
    }))
    const raw = document.querySelector<HTMLTextAreaElement>('.sub2api-export-json')!.value
    expect(parseSub2APICredentialBundle(raw, window.location.origin, 'newapi')).toMatchObject({
      providerType: 'newapi', userID: '42', accessToken: 'management-token', refreshToken: '',
    })
    expect(cookies).not.toHaveBeenCalled()
    expect(document.body.textContent).not.toContain('management-token')
    document.querySelector<HTMLButtonElement>('.sub2api-copy-button')!.click()
    await flushPromises()
    expect(clipboard).toHaveBeenCalledWith(raw, 'text')
  })

  it.each([
    { ok: true, success: true, id: 43 },
    { ok: true, success: false, id: 42 },
    { ok: false, success: true, id: 42 },
  ])('refuses rejected credentials or another user identity: %j', async result => {
    const form = await openNewAPI()
    fetchMock.mockResolvedValue({ ok: result.ok, json: async () => ({ success: result.success, data: { id: result.id } }) })
    ;(form.elements.namedItem('access_token') as HTMLInputElement).value = 'bad-token'
    form.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }))
    await flushPromises()
    expect(document.querySelector('.sub2api-export-json')).toBeNull()
    expect(form.querySelector('[role="status"]')?.textContent).toContain('验证失败')
    expect(form.querySelector<HTMLButtonElement>('[type="submit"]')!.disabled).toBe(false)
  })

  it('does not reopen the export result when verification finishes after cancellation', async () => {
    const form = await openNewAPI()
    let respond!: (response: unknown) => void
    fetchMock.mockImplementation(() => new Promise(resolve => { respond = resolve }))
    ;(form.elements.namedItem('access_token') as HTMLInputElement).value = 'management-token'
    form.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }))
    document.querySelector<HTMLButtonElement>('.sub2api-export-close')!.click()
    respond({ ok: true, json: async () => ({ success: true, data: { id: 42 } }) })
    await flushPromises()
    expect(document.querySelector('#sub2api-credential-exporter-panel')).toBeNull()
    expect(fetchMock.mock.calls.at(-1)?.[1].signal.aborted).toBe(true)
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

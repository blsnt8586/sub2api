import { describe, expect, it } from 'vitest'
import {
  parseSub2APICredentialBundle,
  Sub2APICredentialBundleError,
} from '@/utils/sub2apiCredentialBundle'

function bundle(overrides: Record<string, unknown> = {}) {
  return JSON.stringify({
    format: 'sub2api-browser-credentials',
    version: 1,
    source: {
      origin: 'https://mdkj.lol',
      captured_at: '2026-08-17T03:00:00.000Z',
    },
    local_storage: {
      auth_token: 'access-token',
      refresh_token: 'refresh-token',
      token_expires_at: '1786939200000',
      auth_user: { email: 'admin@example.com', role: 'admin' },
    },
    cookie_capture: {
      method: 'GM_cookie.list',
      items: [
        { name: 'session', value: 'one', httpOnly: true },
        { name: 'theme', value: 'dark', httpOnly: false },
      ],
    },
    ...overrides,
  })
}

function expectCode(run: () => unknown, code: string) {
  try {
    run()
    throw new Error('expected parser to throw')
  } catch (error) {
    expect(error).toBeInstanceOf(Sub2APICredentialBundleError)
    expect((error as Sub2APICredentialBundleError).code).toBe(code)
  }
}

describe('parseSub2APICredentialBundle', () => {
  it('extracts the token pair and only returns a cookie summary', () => {
    const parsed = parseSub2APICredentialBundle(bundle(), 'https://mdkj.lol/keys')
    expect(parsed).toMatchObject({
      accessToken: 'access-token',
      refreshToken: 'refresh-token',
      email: 'admin@example.com',
      sourceOrigin: 'https://mdkj.lol',
      tokenExpiresAt: 1786939200000,
      cookieCount: 2,
      httpOnlyCookieCount: 1,
      cookieCaptureMethod: 'GM_cookie.list',
    })
    expect(parsed).not.toHaveProperty('cookies')
  })

  it('accepts auth_user serialized by older clients', () => {
    const parsed = parseSub2APICredentialBundle(bundle({
      local_storage: {
        auth_token: 'access-token',
        refresh_token: 'refresh-token',
        auth_user: JSON.stringify({ email: 'legacy@example.com' }),
      },
    }))
    expect(parsed.email).toBe('legacy@example.com')
  })

  it('rejects a bundle captured from another origin', () => {
    expectCode(() => parseSub2APICredentialBundle(bundle(), 'https://o10.top'), 'originMismatch')
  })

  it('rejects malformed and incomplete bundles', () => {
    expectCode(() => parseSub2APICredentialBundle('{'), 'invalidJson')
    expectCode(() => parseSub2APICredentialBundle(JSON.stringify({ format: 'other', version: 1 })), 'invalidFormat')
    expectCode(() => parseSub2APICredentialBundle(bundle({ version: 2 })), 'unsupportedVersion')
    expectCode(() => parseSub2APICredentialBundle(bundle({
      local_storage: { auth_token: 'access-only' },
    })), 'tokenPairMissing')
  })
})

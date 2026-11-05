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

describe('New API credential bundles', () => {
  const newAPIBundle = (credentials: Record<string, unknown> = { user_id: '42', access_token: 'management-token' }) =>
    JSON.stringify({
      format: 'newapi-browser-credentials', version: 1,
      source: { origin: 'https://newapi.example.com' }, credentials,
    })

  it.each(['42', 42])('imports user ID %s and a management token without a refresh token', userID => {
    const parsed = parseSub2APICredentialBundle(
      newAPIBundle({ user_id: userID, access_token: ' management-token ' }),
      'https://newapi.example.com/console', 'newapi'
    )
    expect(parsed).toMatchObject({
      providerType: 'newapi', userID: '42', accessToken: 'management-token',
      refreshToken: '', sourceOrigin: 'https://newapi.example.com', cookieCount: 0,
    })
  })

  it('rejects bundles for the wrong provider or origin', () => {
    expectCode(() => parseSub2APICredentialBundle(newAPIBundle(), undefined, 'sub2api'), 'providerMismatch')
    expectCode(() => parseSub2APICredentialBundle(bundle(), undefined, 'newapi'), 'providerMismatch')
    expectCode(() => parseSub2APICredentialBundle(newAPIBundle(), 'https://other.example.com', 'newapi'), 'originMismatch')
  })

  it.each([0, -1, 1.5, '01', '1e3', 'abc', '9007199254740992', null])('rejects invalid user ID %s', userID => {
    expectCode(() => parseSub2APICredentialBundle(newAPIBundle({ user_id: userID, access_token: 'token' })), 'invalidUserID')
  })

  it('requires a bounded management token', () => {
    expectCode(() => parseSub2APICredentialBundle(newAPIBundle({ user_id: '42' })), 'managementTokenMissing')
    expectCode(() => parseSub2APICredentialBundle(newAPIBundle({ user_id: '42', access_token: 'x'.repeat(128 * 1024 + 1) })), 'tokenTooLarge')
  })
})

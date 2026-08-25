import { describe, expect, it } from 'vitest'

import { extractErrorMessage } from '../errorHandler'

describe('extractErrorMessage', () => {
  it('reads normalized top-level API errors from the response interceptor', () => {
    expect(extractErrorMessage({
      status: 503,
      reason: 'PROVIDER_TOKEN_KEY_REQUIRED',
      message: 'configure security.provider_token_key before importing provider tokens',
    }, 'fallback')).toBe('configure security.provider_token_key before importing provider tokens')
  })

  it('keeps compatibility with raw Axios response errors', () => {
    expect(extractErrorMessage({
      response: { data: { message: 'raw API error' } },
      message: 'Request failed with status code 503',
    }, 'fallback')).toBe('raw API error')
  })

  it('falls back to reason when no message is available', () => {
    expect(extractErrorMessage({ reason: 'PROVIDER_TOKEN_KEY_REQUIRED' }, 'fallback'))
      .toBe('PROVIDER_TOKEN_KEY_REQUIRED')
  })

  it('uses the supplied fallback for unrelated errors', () => {
    expect(extractErrorMessage(new Error(''), 'fallback')).toBe('fallback')
  })
})

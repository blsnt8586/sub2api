export const SUB2API_CREDENTIAL_BUNDLE_FORMAT = 'sub2api-browser-credentials'
export const SUB2API_CREDENTIAL_BUNDLE_VERSION = 1

const MAX_BUNDLE_BYTES = 256 * 1024
const MAX_TOKEN_LENGTH = 128 * 1024

export type Sub2APICredentialBundleErrorCode =
  | 'empty'
  | 'tooLarge'
  | 'invalidJson'
  | 'invalidFormat'
  | 'unsupportedVersion'
  | 'invalidOrigin'
  | 'originMismatch'
  | 'tokenPairMissing'
  | 'tokenTooLarge'

export class Sub2APICredentialBundleError extends Error {
  constructor(public readonly code: Sub2APICredentialBundleErrorCode) {
    super(code)
    this.name = 'Sub2APICredentialBundleError'
  }
}

export interface Sub2APICredentialBundle {
  accessToken: string
  refreshToken: string
  email?: string
  sourceOrigin: string
  capturedAt?: string
  tokenExpiresAt?: number
  cookieCount: number
  httpOnlyCookieCount: number
  cookieCaptureMethod?: string
  rawSize: number
}

type UnknownRecord = Record<string, unknown>

function asRecord(value: unknown): UnknownRecord | null {
  return value !== null && typeof value === 'object' && !Array.isArray(value)
    ? value as UnknownRecord
    : null
}

function optionalString(value: unknown): string | undefined {
  return typeof value === 'string' && value.trim() ? value.trim() : undefined
}

function normalizeOrigin(value: unknown): string {
  const raw = optionalString(value)
  if (!raw) throw new Sub2APICredentialBundleError('invalidOrigin')
  try {
    const parsed = new URL(raw)
    if (!['http:', 'https:'].includes(parsed.protocol) || parsed.username || parsed.password) {
      throw new Error('invalid origin')
    }
    return parsed.origin
  } catch {
    throw new Sub2APICredentialBundleError('invalidOrigin')
  }
}

function extractEmail(value: unknown): string | undefined {
  let user: unknown = value
  if (typeof user === 'string') {
    try {
      user = JSON.parse(user)
    } catch {
      return undefined
    }
  }
  const record = asRecord(user)
  if (!record) return undefined
  const direct = optionalString(record.email)
  if (direct) return direct
  const nested = asRecord(record.user)
  return nested ? optionalString(nested.email) : undefined
}

function parseOptionalTimestamp(value: unknown): number | undefined {
  if (typeof value === 'number' && Number.isFinite(value) && value > 0) return value
  if (typeof value === 'string' && value.trim()) {
    const parsed = Number(value)
    if (Number.isFinite(parsed) && parsed > 0) return parsed
  }
  return undefined
}

export function parseSub2APICredentialBundle(
  raw: string,
  expectedBaseURL?: string
): Sub2APICredentialBundle {
  const trimmed = raw.trim()
  if (!trimmed) throw new Sub2APICredentialBundleError('empty')
  const rawSize = new TextEncoder().encode(trimmed).byteLength
  if (rawSize > MAX_BUNDLE_BYTES) throw new Sub2APICredentialBundleError('tooLarge')

  let decoded: unknown
  try {
    decoded = JSON.parse(trimmed)
  } catch {
    throw new Sub2APICredentialBundleError('invalidJson')
  }
  const bundle = asRecord(decoded)
  if (!bundle || bundle.format !== SUB2API_CREDENTIAL_BUNDLE_FORMAT) {
    throw new Sub2APICredentialBundleError('invalidFormat')
  }
  if (bundle.version !== SUB2API_CREDENTIAL_BUNDLE_VERSION) {
    throw new Sub2APICredentialBundleError('unsupportedVersion')
  }

  const source = asRecord(bundle.source)
  const sourceOrigin = normalizeOrigin(source?.origin)
  if (expectedBaseURL?.trim()) {
    const expectedOrigin = normalizeOrigin(expectedBaseURL)
    if (sourceOrigin !== expectedOrigin) {
      throw new Sub2APICredentialBundleError('originMismatch')
    }
  }

  const storage = asRecord(bundle.local_storage)
  const accessToken = optionalString(storage?.auth_token)
  const refreshToken = optionalString(storage?.refresh_token)
  if (!accessToken || !refreshToken) {
    throw new Sub2APICredentialBundleError('tokenPairMissing')
  }
  if (accessToken.length > MAX_TOKEN_LENGTH || refreshToken.length > MAX_TOKEN_LENGTH) {
    throw new Sub2APICredentialBundleError('tokenTooLarge')
  }

  const cookieCapture = asRecord(bundle.cookie_capture)
  const cookieItems = Array.isArray(cookieCapture?.items) ? cookieCapture.items : []
  const httpOnlyCookieCount = cookieItems.reduce((count, item) => {
    return count + (asRecord(item)?.httpOnly === true ? 1 : 0)
  }, 0)

  return {
    accessToken,
    refreshToken,
    email: extractEmail(storage?.auth_user),
    sourceOrigin,
    capturedAt: optionalString(source?.captured_at),
    tokenExpiresAt: parseOptionalTimestamp(storage?.token_expires_at),
    cookieCount: cookieItems.length,
    httpOnlyCookieCount,
    cookieCaptureMethod: optionalString(cookieCapture?.method),
    rawSize,
  }
}

import { beforeEach, describe, expect, it, vi } from 'vitest'

const { getLocaleMock, hasTranslationMock, translateMock } = vi.hoisted(() => ({
  getLocaleMock: vi.fn<() => 'en' | 'zh'>(() => 'zh'),
  hasTranslationMock: vi.fn<(key: string, locale?: string) => boolean>(() => false),
  translateMock: vi.fn<(key: string, params?: Record<string, number>) => string>((key: string) => key),
}))

vi.mock('@/i18n', () => ({
  getLocale: getLocaleMock,
  i18n: {
    global: {
      te: hasTranslationMock,
      t: translateMock,
    },
  },
}))

import { formatRelativeTime } from '../format'

describe('formatRelativeTime', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-08-16T02:20:00+08:00'))
    getLocaleMock.mockReturnValue('zh')
    hasTranslationMock.mockReturnValue(false)
    translateMock.mockImplementation((key: string) => key)
  })

  it('uses locale messages when the requested key exists', () => {
    hasTranslationMock.mockImplementation((key: string) => key === 'common.time.minutesAgo')
    translateMock.mockImplementation((key: string, params?: Record<string, number>) => (
      key === 'common.time.minutesAgo' ? `${params?.n}分钟前` : key
    ))

    expect(formatRelativeTime('2026-08-16T02:15:00+08:00')).toBe('5分钟前')
  })

  it('falls back to Intl instead of exposing a missing translation key', () => {
    expect(formatRelativeTime('2026-08-16T02:15:00+08:00')).toBe('5分钟前')
    expect(formatRelativeTime('2026-08-16T02:15:00+08:00')).not.toContain('common.time')
    expect(translateMock).not.toHaveBeenCalled()
  })

  it('uses the active locale for the fallback', () => {
    getLocaleMock.mockReturnValue('en')

    expect(formatRelativeTime('2026-08-16T02:15:00+08:00')).toBe('5 min. ago')
  })
})

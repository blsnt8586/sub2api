/**
 * Sub2API 优化配置校验工具
 */

export interface OptimizeConfig {
  sub2api_optimize_enabled?: boolean
  sub2api_max_multiplier?: number | null
  sub2api_min_multiplier?: number | null
  sub2api_test_model?: string | null
}

/**
 * 检查账号是否配置了优化所需的三个必填字段
 * @param config 账号配置对象
 * @returns true=配置完整, false=配置不完整
 */
export function isOptimizeConfigComplete(config: OptimizeConfig): boolean {
  return (
    config.sub2api_max_multiplier != null &&
    config.sub2api_min_multiplier != null &&
    config.sub2api_test_model != null &&
    config.sub2api_test_model.trim() !== ''
  )
}

/**
 * 找出配置不完整的账号
 * @param accounts 账号列表
 * @returns 配置不完整的账号数组
 */
export function findIncompleteAccounts<T extends OptimizeConfig & { name: string }>(
  accounts: T[]
): T[] {
  return accounts.filter(acc => !isOptimizeConfigComplete(acc))
}

/** 只返回已经开启参与、但三项必填配置不完整的账号。 */
export function findIncompleteParticipatingAccounts<T extends OptimizeConfig & { name: string }>(
  accounts: T[]
): T[] {
  return accounts.filter(
    acc => acc.sub2api_optimize_enabled === true && !isOptimizeConfigComplete(acc)
  )
}

export type MultiplierRangeState = 'unknown' | 'unbounded' | 'within' | 'below' | 'above'

/** 按账号自己的倍率上下限判断当前远端倍率是否越界。 */
export function getMultiplierRangeState(
  current: number | null | undefined,
  min: number | null | undefined,
  max: number | null | undefined
): MultiplierRangeState {
  if (current == null || !Number.isFinite(current)) return 'unknown'
  if (min != null && current < min) return 'below'
  if (max != null && current > max) return 'above'
  if (min != null || max != null) return 'within'
  return 'unbounded'
}

/**
 * 返回单个账号首个缺失字段对应的必填错误提示，配置完整时返回 null。
 * 用于「开启参与」等需要给出具体缺哪个字段的入口。
 * @param config 账号配置对象
 * @param t i18n翻译函数
 * @returns 错误提示文案，配置完整时为 null
 */
export function getIncompleteConfigError(
  config: OptimizeConfig,
  t: (key: string) => string
): string | null {
  if (config.sub2api_max_multiplier == null) {
    return t('admin.sub2apiProviders.maxMultiplierRequired')
  }
  if (config.sub2api_min_multiplier == null) {
    return t('admin.sub2apiProviders.minMultiplierRequired')
  }
  if (config.sub2api_test_model == null || config.sub2api_test_model.trim() === '') {
    return t('admin.sub2apiProviders.testModelRequired')
  }
  return null
}

/**
 * 倍率验证结果
 */
export interface MultiplierValidation {
  valid: boolean
  value?: number
  error?: string
}

/**
 * 验证倍率上限输入
 * @param raw 原始输入字符串
 * @param minMultiplier 当前下限（用于检查上限不得小于下限）
 * @param t i18n翻译函数
 */
export function validateMaxMultiplier(
  raw: string,
  minMultiplier: number | null | undefined,
  t: (key: string) => string
): MultiplierValidation {
  const trimmed = raw.trim()
  const n = Number(trimmed)

  if (trimmed === '' || Number.isNaN(n) || n <= 0) {
    return { valid: false, error: t('admin.sub2apiProviders.maxMultiplierInvalid') }
  }

  if (minMultiplier != null && n < minMultiplier) {
    return { valid: false, error: t('admin.sub2apiProviders.maxMultiplierInvalid') }
  }

  return { valid: true, value: n }
}

/**
 * 验证倍率下限输入
 * @param raw 原始输入字符串
 * @param maxMultiplier 当前上限（用于检查下限不得大于上限）
 * @param t i18n翻译函数
 */
export function validateMinMultiplier(
  raw: string,
  maxMultiplier: number | null | undefined,
  t: (key: string) => string
): MultiplierValidation {
  const trimmed = raw.trim()

  // 关闭参与时可以清除；开启参与时由调用方阻止清空。
  if (trimmed === '') {
    return { valid: true, value: undefined }
  }

  const n = Number(trimmed)

  if (Number.isNaN(n) || n < 0) {
    return { valid: false, error: t('admin.sub2apiProviders.minMultiplierInvalid') }
  }

  if (maxMultiplier != null && n > maxMultiplier) {
    return { valid: false, error: t('admin.sub2apiProviders.minMultiplierExceedsMax') }
  }

  return { valid: true, value: n }
}

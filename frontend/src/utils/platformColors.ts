/**
 * Centralized platform color definitions.
 *
 * All components that need platform-specific styling should import from here
 * instead of defining their own color mappings.
 */

// 注：'composite' 是上游的复合分组平台（一个分组挂多平台账号），仅作为 group.platform
// 出现，不是账号平台；因此它有配色但不在 ALL_PLATFORMS（账号/配额平台枚举）里。
export type Platform =
  | 'anthropic'
  | 'openai'
  | 'antigravity'
  | 'gemini'
  | 'grok'
  | 'canvas'
  | 'kimi'
  | 'zhipu'
  | 'deepseek'
  | 'composite'

// ── Badge (bg + text + border, for inline badges with border) ───────
const BADGE: Record<Platform, string> = {
  anthropic: 'bg-orange-500/10 text-orange-600 border-orange-500/30 dark:text-orange-400',
  openai: 'bg-green-500/10 text-green-600 border-green-500/30 dark:text-green-400',
  antigravity: 'bg-purple-500/10 text-purple-600 border-purple-500/30 dark:text-purple-400',
  gemini: 'bg-blue-500/10 text-blue-600 border-blue-500/30 dark:text-blue-400',
  grok: 'bg-zinc-800/10 text-zinc-800 border-zinc-800/30 dark:bg-zinc-500/10 dark:text-zinc-200 dark:border-zinc-500/30',
  canvas: 'bg-rose-500/10 text-rose-600 border-rose-500/30 dark:bg-rose-500/10 dark:text-rose-300 dark:border-rose-500/30',
  kimi: 'bg-pink-500/10 text-pink-600 border-pink-500/30 dark:text-pink-400',
  zhipu: 'bg-indigo-500/10 text-indigo-600 border-indigo-500/30 dark:text-indigo-400',
  deepseek: 'bg-teal-500/10 text-teal-600 border-teal-500/30 dark:text-teal-400',
  composite: 'bg-cyan-500/10 text-cyan-700 border-cyan-500/30 dark:text-cyan-300',
}
const BADGE_DEFAULT = 'bg-slate-500/10 text-slate-600 border-slate-500/30 dark:text-slate-400'

// ── Light badge (softer bg, no border) ──────────────────────────────
const BADGE_LIGHT: Record<Platform, string> = {
  anthropic: 'bg-orange-500/10 text-orange-600 dark:bg-orange-500/10 dark:text-orange-300',
  openai: 'bg-green-500/10 text-green-600 dark:bg-green-500/10 dark:text-green-300',
  antigravity: 'bg-purple-500/10 text-purple-600 dark:bg-purple-500/10 dark:text-purple-300',
  gemini: 'bg-blue-500/10 text-blue-600 dark:bg-blue-500/10 dark:text-blue-300',
  grok: 'bg-zinc-800/10 text-zinc-800 dark:bg-zinc-500/10 dark:text-zinc-200',
  canvas: 'bg-rose-500/10 text-rose-600 dark:bg-rose-500/10 dark:text-rose-300',
  kimi: 'bg-pink-500/10 text-pink-600 dark:bg-pink-500/10 dark:text-pink-300',
  zhipu: 'bg-indigo-500/10 text-indigo-600 dark:bg-indigo-500/10 dark:text-indigo-300',
  deepseek: 'bg-teal-500/10 text-teal-600 dark:bg-teal-500/10 dark:text-teal-300',
  composite: 'bg-cyan-500/10 text-cyan-700 dark:bg-cyan-500/10 dark:text-cyan-300',
}

// ── Border ──────────────────────────────────────────────────────────
const BORDER: Record<Platform, string> = {
  anthropic: 'border-orange-500/20 dark:border-orange-500/20',
  openai: 'border-green-500/20 dark:border-green-500/20',
  antigravity: 'border-purple-500/20 dark:border-purple-500/20',
  gemini: 'border-blue-500/20 dark:border-blue-500/20',
  grok: 'border-zinc-800/20 dark:border-zinc-500/20',
  canvas: 'border-rose-500/20 dark:border-rose-500/20',
  kimi: 'border-pink-500/20 dark:border-pink-500/20',
  zhipu: 'border-indigo-500/20 dark:border-indigo-500/20',
  deepseek: 'border-teal-500/20 dark:border-teal-500/20',
  composite: 'border-cyan-500/20 dark:border-cyan-500/20',
}
const BORDER_DEFAULT = 'border-gray-200 dark:border-dark-700'

// ── Border strong (higher-contrast platform tint, e.g. plaza group cards) ──
const BORDER_STRONG: Record<Platform, string> = {
  anthropic: 'border-orange-500/35 dark:border-orange-500/30',
  openai: 'border-green-500/35 dark:border-green-500/30',
  antigravity: 'border-purple-500/35 dark:border-purple-500/30',
  gemini: 'border-blue-500/35 dark:border-blue-500/30',
  grok: 'border-zinc-800/35 dark:border-zinc-500/35',
  canvas: 'border-rose-500/35 dark:border-rose-500/30', // [CUSTOM]
  kimi: 'border-pink-500/35 dark:border-pink-500/30',
  zhipu: 'border-indigo-500/35 dark:border-indigo-500/30',
  deepseek: 'border-teal-500/35 dark:border-teal-500/30',
  composite: 'border-cyan-500/35 dark:border-cyan-500/30',
}
const BORDER_STRONG_DEFAULT = 'border-gray-300 dark:border-dark-600'

// ── Accent (single raw color per platform; consumers derive washes/tints
//    from it via CSS color-mix, e.g. plaza paid-price zone) ──
const ACCENT: Record<Platform, string> = {
  anthropic: '#f97316', // orange-500
  openai: '#22c55e', // green-500
  antigravity: '#a855f7', // purple-500
  gemini: '#3b82f6', // blue-500
  grok: '#71717a', // zinc-500
  canvas: '#f43f5e', // rose-500 [CUSTOM]
  kimi: '#ec4899', // pink-500
  zhipu: '#6366f1', // indigo-500
  deepseek: '#14b8a6', // teal-500
  composite: '#06b6d4', // cyan-500
}
const ACCENT_DEFAULT = '#14b8a6' // primary-500 (teal)

// ── Accent bar (gradient) ───────────────────────────────────────────
const ACCENT_BAR: Record<Platform, string> = {
  anthropic: 'bg-gradient-to-r from-orange-400 to-orange-500',
  openai: 'bg-gradient-to-r from-emerald-400 to-emerald-500',
  antigravity: 'bg-gradient-to-r from-purple-400 to-purple-500',
  gemini: 'bg-gradient-to-r from-blue-400 to-blue-500',
  grok: 'bg-gradient-to-r from-zinc-700 to-zinc-900',
  canvas: 'bg-gradient-to-r from-rose-400 to-rose-500',
  kimi: 'bg-gradient-to-r from-pink-400 to-pink-500',
  zhipu: 'bg-gradient-to-r from-indigo-400 to-indigo-500',
  deepseek: 'bg-gradient-to-r from-teal-400 to-teal-500',
  composite: 'bg-gradient-to-r from-slate-500 to-cyan-500',
}
const ACCENT_BAR_DEFAULT = 'bg-gradient-to-r from-primary-400 to-primary-500'

// ── Text (price, icon) ─────────────────────────────────────────────
const TEXT: Record<Platform, string> = {
  anthropic: 'text-orange-600 dark:text-orange-400',
  openai: 'text-emerald-600 dark:text-emerald-400',
  antigravity: 'text-purple-600 dark:text-purple-400',
  gemini: 'text-blue-600 dark:text-blue-400',
  grok: 'text-zinc-800 dark:text-zinc-200',
  canvas: 'text-rose-600 dark:text-rose-400',
  kimi: 'text-pink-600 dark:text-pink-400',
  zhipu: 'text-indigo-600 dark:text-indigo-400',
  deepseek: 'text-teal-600 dark:text-teal-400',
  composite: 'text-cyan-700 dark:text-cyan-300',
}
const TEXT_DEFAULT = 'text-primary-600 dark:text-primary-400'

// ── Strong text (text-700，用于模型 tag 文字/分组标题等需更深文字色的场景) ──
const TEXT_STRONG: Record<Platform, string> = {
  anthropic: 'text-orange-700 dark:text-orange-400',
  openai: 'text-emerald-700 dark:text-emerald-400',
  antigravity: 'text-purple-700 dark:text-purple-400',
  gemini: 'text-blue-700 dark:text-blue-400',
  grok: 'text-zinc-700 dark:text-zinc-300',
  canvas: 'text-rose-700 dark:text-rose-400',
  kimi: 'text-pink-700 dark:text-pink-400',
  zhipu: 'text-indigo-700 dark:text-indigo-400',
  deepseek: 'text-teal-700 dark:text-teal-400',
  composite: 'text-cyan-700 dark:text-cyan-300',
}
const TEXT_STRONG_DEFAULT = 'text-blue-700 dark:text-blue-400'

// ── Icon (check mark etc.) ──────────────────────────────────────────
const ICON: Record<Platform, string> = {
  anthropic: 'text-orange-500 dark:text-orange-400',
  openai: 'text-emerald-500 dark:text-emerald-400',
  antigravity: 'text-purple-500 dark:text-purple-400',
  gemini: 'text-blue-500 dark:text-blue-400',
  grok: 'text-zinc-800 dark:text-zinc-200',
  canvas: 'text-rose-500 dark:text-rose-400',
  kimi: 'text-pink-500 dark:text-pink-400',
  zhipu: 'text-indigo-500 dark:text-indigo-400',
  deepseek: 'text-teal-500 dark:text-teal-400',
  composite: 'text-cyan-600 dark:text-cyan-300',
}
const ICON_DEFAULT = 'text-primary-500 dark:text-primary-400'

// ── Button (solid bg) ───────────────────────────────────────────────
const BUTTON: Record<Platform, string> = {
  anthropic: 'bg-orange-500 text-white hover:bg-orange-600 active:bg-orange-700 dark:bg-orange-500/80 dark:hover:bg-orange-500',
  openai: 'bg-green-600 text-white hover:bg-green-700 active:bg-green-800 dark:bg-green-600/80 dark:hover:bg-green-600',
  antigravity: 'bg-purple-500 text-white hover:bg-purple-600 active:bg-purple-700 dark:bg-purple-500/80 dark:hover:bg-purple-500',
  gemini: 'bg-blue-500 text-white hover:bg-blue-600 active:bg-blue-700 dark:bg-blue-500/80 dark:hover:bg-blue-500',
  grok: 'bg-zinc-800 text-white hover:bg-zinc-900 active:bg-black dark:bg-zinc-700 dark:hover:bg-zinc-600',
  canvas: 'bg-rose-500 text-white hover:bg-rose-600 active:bg-rose-700 dark:bg-rose-500/80 dark:hover:bg-rose-500',
  kimi: 'bg-pink-500 text-white hover:bg-pink-600 active:bg-pink-700 dark:bg-pink-500/80 dark:hover:bg-pink-500',
  zhipu: 'bg-indigo-500 text-white hover:bg-indigo-600 active:bg-indigo-700 dark:bg-indigo-500/80 dark:hover:bg-indigo-500',
  deepseek: 'bg-teal-500 text-white hover:bg-teal-600 active:bg-teal-700 dark:bg-teal-500/80 dark:hover:bg-teal-500',
  composite: 'bg-cyan-700 text-white hover:bg-cyan-800 active:bg-cyan-900 dark:bg-cyan-600 dark:hover:bg-cyan-500',
}
const BUTTON_DEFAULT = 'bg-primary-500 text-white hover:bg-primary-600 dark:bg-primary-600 dark:hover:bg-primary-500'

// ── Discount badge ──────────────────────────────────────────────────
const DISCOUNT: Record<Platform, string> = {
  anthropic: 'bg-orange-100 text-orange-700 dark:bg-orange-900/40 dark:text-orange-300',
  openai: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300',
  antigravity: 'bg-purple-100 text-purple-700 dark:bg-purple-900/40 dark:text-purple-300',
  gemini: 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300',
  grok: 'bg-zinc-100 text-zinc-800 dark:bg-zinc-800 dark:text-zinc-200',
  canvas: 'bg-rose-100 text-rose-700 dark:bg-rose-900/40 dark:text-rose-300',
  kimi: 'bg-pink-100 text-pink-700 dark:bg-pink-900/40 dark:text-pink-300',
  zhipu: 'bg-indigo-100 text-indigo-700 dark:bg-indigo-900/40 dark:text-indigo-300',
  deepseek: 'bg-teal-100 text-teal-700 dark:bg-teal-900/40 dark:text-teal-300',
  composite: 'bg-cyan-100 text-cyan-800 dark:bg-cyan-900/40 dark:text-cyan-300',
}
const DISCOUNT_DEFAULT = 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300'

// ── Solid tag (bg-100 + text-700，用于模型/平台标签徽章) ─────────────
const TAG: Record<Platform, string> = {
  anthropic: 'bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-400',
  openai: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400',
  antigravity: 'bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-400',
  gemini: 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400',
  grok: 'bg-zinc-100 text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300',
  canvas: 'bg-rose-100 text-rose-700 dark:bg-rose-900/30 dark:text-rose-400',
  kimi: 'bg-pink-100 text-pink-700 dark:bg-pink-900/30 dark:text-pink-400',
  zhipu: 'bg-indigo-100 text-indigo-700 dark:bg-indigo-900/30 dark:text-indigo-400',
  deepseek: 'bg-teal-100 text-teal-700 dark:bg-teal-900/30 dark:text-teal-400',
  composite: 'bg-cyan-100 text-cyan-700 dark:bg-cyan-900/30 dark:text-cyan-400',
}
const TAG_DEFAULT = 'bg-gray-100 text-gray-700 dark:bg-gray-900/30 dark:text-gray-400'

// ── Soft tag (bg-100 + text-600，文字浅一档，用于 type/plan 副徽章) ──
const TAG_SOFT: Record<Platform, string> = {
  anthropic: 'bg-orange-100 text-orange-600 dark:bg-orange-900/30 dark:text-orange-400',
  openai: 'bg-emerald-100 text-emerald-600 dark:bg-emerald-900/30 dark:text-emerald-400',
  antigravity: 'bg-purple-100 text-purple-600 dark:bg-purple-900/30 dark:text-purple-400',
  gemini: 'bg-blue-100 text-blue-600 dark:bg-blue-900/30 dark:text-blue-400',
  grok: 'bg-zinc-100 text-zinc-600 dark:bg-zinc-800 dark:text-zinc-300',
  canvas: 'bg-rose-100 text-rose-600 dark:bg-rose-900/30 dark:text-rose-400',
  kimi: 'bg-pink-100 text-pink-600 dark:bg-pink-900/30 dark:text-pink-400',
  zhipu: 'bg-indigo-100 text-indigo-600 dark:bg-indigo-900/30 dark:text-indigo-400',
  deepseek: 'bg-teal-100 text-teal-600 dark:bg-teal-900/30 dark:text-teal-400',
  composite: 'bg-cyan-100 text-cyan-600 dark:bg-cyan-900/30 dark:text-cyan-400',
}
const TAG_SOFT_DEFAULT = 'bg-gray-100 text-gray-600 dark:bg-gray-900/30 dark:text-gray-400'

// ── Badge standard variant (bg-50 淡色背景，用于 GroupBadge standard 类型) ──
// 注意：色系与 subscription 略有不同（如 anthropic sub=orange，std=amber）
const BADGE_STANDARD: Record<Platform, string> = {
  anthropic: 'bg-amber-50 text-amber-700 dark:bg-amber-900/20 dark:text-amber-400',
  openai: 'bg-green-50 text-green-700 dark:bg-green-900/20 dark:text-green-400',
  antigravity: 'bg-fuchsia-50 text-fuchsia-700 dark:bg-fuchsia-900/20 dark:text-fuchsia-400',
  gemini: 'bg-sky-50 text-sky-700 dark:bg-sky-900/20 dark:text-sky-400',
  grok: 'bg-zinc-100 text-zinc-700 dark:bg-zinc-800 dark:text-zinc-200',
  canvas: 'bg-rose-50 text-rose-700 dark:bg-rose-900/20 dark:text-rose-400',
  kimi: 'bg-pink-50 text-pink-700 dark:bg-pink-900/20 dark:text-pink-400',
  zhipu: 'bg-indigo-50 text-indigo-700 dark:bg-indigo-900/20 dark:text-indigo-400',
  deepseek: 'bg-teal-50 text-teal-700 dark:bg-teal-900/20 dark:text-teal-400',
  composite: 'bg-cyan-50 text-cyan-800 dark:bg-cyan-900/20 dark:text-cyan-300',
}
const BADGE_STANDARD_DEFAULT = 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400'

// ── Label subscription (bg-200/60 深色背景，用于 GroupBadge labelClass 订阅正常状态) ──
const LABEL_SUBSCRIPTION: Record<Platform, string> = {
  anthropic: 'bg-orange-200/60 text-orange-800 dark:bg-orange-800/40 dark:text-orange-300',
  openai: 'bg-emerald-200/60 text-emerald-800 dark:bg-emerald-800/40 dark:text-emerald-300',
  antigravity: 'bg-purple-200/60 text-purple-800 dark:bg-purple-800/40 dark:text-purple-300',
  gemini: 'bg-blue-200/60 text-blue-800 dark:bg-blue-800/40 dark:text-blue-300',
  grok: 'bg-zinc-300/70 text-zinc-800 dark:bg-zinc-700/60 dark:text-zinc-200',
  canvas: 'bg-pink-200/60 text-pink-800 dark:bg-pink-800/40 dark:text-pink-300',
  kimi: 'bg-pink-200/60 text-pink-800 dark:bg-pink-800/40 dark:text-pink-300',
  zhipu: 'bg-indigo-200/60 text-indigo-800 dark:bg-indigo-800/40 dark:text-indigo-300',
  deepseek: 'bg-teal-200/60 text-teal-800 dark:bg-teal-800/40 dark:text-teal-300',
  composite: 'bg-cyan-200/70 text-cyan-900 dark:bg-cyan-900/50 dark:text-cyan-300',
}
const LABEL_SUBSCRIPTION_DEFAULT = 'bg-violet-200/60 text-violet-800 dark:bg-violet-800/40 dark:text-violet-300'

// ── Badge subscription variant (bg-100，用于 GroupBadge subscription 类型主徽章) ──
// 注意 grok/canvas 与 TAG 色系略有差异（grok 更深，canvas 用 pink）
const BADGE_SUBSCRIPTION: Record<Platform, string> = {
  anthropic: 'bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-400',
  openai: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400',
  antigravity: 'bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-400',
  gemini: 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400',
  grok: 'bg-zinc-200 text-zinc-800 dark:bg-zinc-700 dark:text-zinc-100',
  canvas: 'bg-pink-100 text-pink-700 dark:bg-pink-900/30 dark:text-pink-400',
  kimi: 'bg-pink-100 text-pink-700 dark:bg-pink-900/30 dark:text-pink-400',
  zhipu: 'bg-indigo-100 text-indigo-700 dark:bg-indigo-900/30 dark:text-indigo-400',
  deepseek: 'bg-teal-100 text-teal-700 dark:bg-teal-900/30 dark:text-teal-400',
  composite: 'bg-cyan-100 text-cyan-800 dark:bg-cyan-900/30 dark:text-cyan-300',
}
const BADGE_SUBSCRIPTION_DEFAULT = 'bg-violet-100 text-violet-700 dark:bg-violet-900/30 dark:text-violet-400'

// ── Header gradient (subscription confirm) ─────────────────────────
const GRADIENT: Record<Platform, string> = {
  anthropic: 'from-orange-500 to-orange-600',
  openai: 'from-emerald-500 to-emerald-600',
  antigravity: 'from-purple-500 to-purple-600',
  gemini: 'from-blue-500 to-blue-600',
  grok: 'from-zinc-700 to-zinc-900',
  canvas: 'from-rose-500 to-rose-600',
  kimi: 'from-pink-500 to-pink-600',
  zhipu: 'from-indigo-500 to-indigo-600',
  deepseek: 'from-teal-500 to-teal-600',
  composite: 'from-slate-600 to-cyan-600',
}
const GRADIENT_DEFAULT = 'from-primary-500 to-primary-600'

// ── Header text (light text on gradient bg) ────────────────────────
const GRADIENT_TEXT: Record<Platform, string> = {
  anthropic: 'text-orange-100',
  openai: 'text-emerald-100',
  antigravity: 'text-purple-100',
  gemini: 'text-blue-100',
  grok: 'text-zinc-100',
  canvas: 'text-rose-100',
  kimi: 'text-pink-100',
  zhipu: 'text-indigo-100',
  deepseek: 'text-teal-100',
  composite: 'text-cyan-100',
}
const GRADIENT_TEXT_DEFAULT = 'text-primary-100'

const GRADIENT_SUBTEXT: Record<Platform, string> = {
  anthropic: 'text-orange-200',
  openai: 'text-emerald-200',
  antigravity: 'text-purple-200',
  gemini: 'text-blue-200',
  grok: 'text-zinc-300',
  canvas: 'text-rose-200',
  kimi: 'text-pink-200',
  zhipu: 'text-indigo-200',
  deepseek: 'text-teal-200',
  composite: 'text-cyan-200',
}
const GRADIENT_SUBTEXT_DEFAULT = 'text-primary-200'

// ── Public API ──────────────────────────────────────────────────────

function isPlatform(p: string): p is Platform {
  return (
    p === 'anthropic' ||
    p === 'openai' ||
    p === 'antigravity' ||
    p === 'gemini' ||
    p === 'grok' ||
    p === 'canvas' ||
    p === 'kimi' ||
    p === 'zhipu' ||
    p === 'deepseek' ||
    p === 'composite'
  )
}

export function platformBadgeClass(p: string): string {
  return isPlatform(p) ? BADGE[p] : BADGE_DEFAULT
}

export function platformBadgeLightClass(p: string): string {
  return isPlatform(p) ? BADGE_LIGHT[p] : BADGE_DEFAULT
}

export function platformBorderClass(p: string): string {
  return isPlatform(p) ? BORDER[p] : BORDER_DEFAULT
}

export function platformBorderStrongClass(p: string): string {
  return isPlatform(p) ? BORDER_STRONG[p] : BORDER_STRONG_DEFAULT
}

export function platformAccentColor(p: string): string {
  return isPlatform(p) ? ACCENT[p] : ACCENT_DEFAULT
}

export function platformAccentBarClass(p: string): string {
  return isPlatform(p) ? ACCENT_BAR[p] : ACCENT_BAR_DEFAULT
}

export function platformTextClass(p: string): string {
  return isPlatform(p) ? TEXT[p] : TEXT_DEFAULT
}

export function platformStrongTextClass(p: string, fallback: string = TEXT_STRONG_DEFAULT): string {
  return isPlatform(p) ? TEXT_STRONG[p] : fallback
}

export function platformIconClass(p: string): string {
  return isPlatform(p) ? ICON[p] : ICON_DEFAULT
}

export function platformButtonClass(p: string): string {
  return isPlatform(p) ? BUTTON[p] : BUTTON_DEFAULT
}

export function platformDiscountClass(p: string): string {
  return isPlatform(p) ? DISCOUNT[p] : DISCOUNT_DEFAULT
}

// platformTagClass 返回实心标签样式（bg-100 + text-700）。
// fallback 允许调用方覆盖未知平台的默认色（如 PlatformTypeBadge 历史上未知走蓝色）。
export function platformTagClass(p: string, fallback: string = TAG_DEFAULT): string {
  return isPlatform(p) ? TAG[p] : fallback
}

// platformTagSoftClass 同 platformTagClass 但文字浅一档（text-600），用于副徽章。
export function platformTagSoftClass(p: string, fallback: string = TAG_SOFT_DEFAULT): string {
  return isPlatform(p) ? TAG_SOFT[p] : fallback
}

// platformBadgeStandardClass 返回 GroupBadge standard 类型的淡色背景徽章样式。
export function platformBadgeStandardClass(p: string, fallback: string = BADGE_STANDARD_DEFAULT): string {
  return isPlatform(p) ? BADGE_STANDARD[p] : fallback
}

// platformLabelSubscriptionClass 返回 GroupBadge 订阅分组正常状态下右侧 label 的样式。
export function platformLabelSubscriptionClass(p: string, fallback: string = LABEL_SUBSCRIPTION_DEFAULT): string {
  return isPlatform(p) ? LABEL_SUBSCRIPTION[p] : fallback
}

// platformBadgeSubscriptionClass 返回 GroupBadge 订阅类型主徽章样式（bg-100）。
export function platformBadgeSubscriptionClass(p: string, fallback: string = BADGE_SUBSCRIPTION_DEFAULT): string {
  return isPlatform(p) ? BADGE_SUBSCRIPTION[p] : fallback
}

export function platformGradientClass(p: string): string {
  return isPlatform(p) ? GRADIENT[p] : GRADIENT_DEFAULT
}

export function platformGradientTextClass(p: string): string {
  return isPlatform(p) ? GRADIENT_TEXT[p] : GRADIENT_TEXT_DEFAULT
}

export function platformGradientSubtextClass(p: string): string {
  return isPlatform(p) ? GRADIENT_SUBTEXT[p] : GRADIENT_SUBTEXT_DEFAULT
}

export function platformLabel(p: string): string {
  switch (p) {
    case 'anthropic': return 'Anthropic'
    case 'openai': return 'OpenAI'
    case 'antigravity': return 'Antigravity'
    case 'gemini': return 'Gemini'
    case 'grok': return 'Grok'
    case 'canvas': return 'Canvas'
    case 'kimi': return 'Kimi'
    case 'zhipu': return 'Zhipu GLM'
    case 'deepseek': return 'DeepSeek'
    case 'composite': return 'Composite'
    default: return p || 'API'
  }
}

// ── 平台权威枚举列表 ────────────────────────────────────────
//
// 单一权威来源：新增平台只需在此追加一项（并补上文各 Record 的对应条目），
// 所有平台下拉/多选/配额矩阵会自动包含新平台，无需逐个文件改。
// 顺序即 UI 展示顺序。后端权威列表见 service/domain_constants.go AllowedQuotaPlatforms。
// 不含 'composite'：它是分组级的复合平台，不能作为账号平台或配额维度。
export const ALL_PLATFORMS: Platform[] = [
  'anthropic',
  'openai',
  'gemini',
  'antigravity',
  'grok',
  'canvas',
  'kimi',
  'zhipu',
  'deepseek',
]

// PlatformSelectOption 是下拉/筛选组件使用的 { value, label } 结构。
export interface PlatformSelectOption {
  value: Platform
  label: string
}

// platformSelectOptions 返回全部平台的 { value, label } 列表（label 走 platformLabel）。
export function platformSelectOptions(): PlatformSelectOption[] {
  return ALL_PLATFORMS.map((p) => ({ value: p, label: platformLabel(p) }))
}

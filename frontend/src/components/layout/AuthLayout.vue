<template>
  <div class="auth-page">
    <!-- ── Background layers（与首页同族的设计语言）── -->
    <div class="auth-glow" aria-hidden="true" />
    <div class="auth-aurora auth-aurora-1" aria-hidden="true" />
    <div class="auth-aurora auth-aurora-2" aria-hidden="true" />
    <div class="auth-grid" aria-hidden="true" />
    <SplitGlobe />

    <!-- Content Container（上下 auto margin 垂直居中）-->
    <div class="auth-content relative z-10 w-full max-w-md">
      <!-- Logo/Brand -->
      <div class="mb-8 text-center">
        <template v-if="settingsLoaded">
          <div class="auth-logo">
            <img :src="siteLogo || '/logo.svg'" alt="Logo" class="h-full w-full object-contain" />
          </div>
          <h1 class="auth-title">{{ siteName }}</h1>
          <p class="auth-subtitle">{{ siteSubtitle }}</p>
        </template>
      </div>

      <!-- Card -->
      <div class="auth-card">
        <div class="auth-card-topline" aria-hidden="true" />
        <slot />
      </div>

      <!-- Footer Links -->
      <div class="mt-6 text-center text-sm">
        <slot name="footer" />
      </div>
    </div>

    <!-- Page Footer: Disclaimer + Copyright（贴页面底部）-->
    <footer class="auth-footer relative z-10">
      <p class="auth-disclaimer">{{ t('home.footer.disclaimer', { siteName }) }}</p>
      <div class="auth-copy">&copy; {{ currentYear }} {{ siteName }}</div>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores'
import { sanitizeUrl } from '@/utils/url'
import SplitGlobe from '@/components/common/SplitGlobe.vue'

const appStore = useAppStore()
const { t, locale } = useI18n()

const siteName = computed(() => appStore.siteName || 'Sub2API')
const siteLogo = computed(() => sanitizeUrl(appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))

// site_subtitle 支持纯文本或 JSON 多语言映射（与 HomeView 同约定）
const siteSubtitle = computed(() => {
  const raw = (appStore.cachedPublicSettings?.site_subtitle || '').trim()
  const fallback = 'Subscription to API Conversion Platform'
  if (!raw) return fallback
  if (raw.startsWith('{')) {
    try {
      const map = JSON.parse(raw) as Record<string, string>
      const lang = locale.value.slice(0, 2)
      return map[lang] || map.zh || map.en || Object.values(map)[0] || fallback
    } catch {
      // 非法 JSON 按纯文本处理
    }
  }
  return raw
})

const settingsLoaded = computed(() => appStore.publicSettingsLoaded)
const currentYear = computed(() => new Date().getFullYear())

onMounted(() => {
  appStore.fetchPublicSettings()
})
</script>

<style scoped>
/* ═══ PAGE ═══════════════════════════════ */
.auth-page {
  position: relative;
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  align-items: center;
  overflow: hidden;
  padding: 16px;
  background: #f6f8fb;
}
/* 上下 auto margin：内容不足一屏时垂直居中，页脚自然贴底 */
.auth-content { margin-top: auto; margin-bottom: auto; padding-top: 24px; }
:global(html.dark .auth-page) { background: #030308; }

/* ═══ BACKGROUND LAYERS ══════════════════ */
.auth-glow {
  position: absolute; inset: 0; pointer-events: none;
  background:
    radial-gradient(ellipse 80% 60% at 50% -15%, rgba(34, 211, 238, 0.16), transparent 62%),
    radial-gradient(ellipse 50% 42% at 85% 15%, rgba(167, 139, 250, 0.12), transparent 65%),
    radial-gradient(ellipse 46% 40% at 10% 30%, rgba(244, 114, 182, 0.08), transparent 65%),
    radial-gradient(ellipse 60% 50% at 50% 110%, rgba(34, 211, 238, 0.08), transparent 60%);
}
:global(html.dark .auth-page .auth-glow) {
  background:
    radial-gradient(ellipse 80% 60% at 50% -15%, rgba(34, 211, 238, 0.22), transparent 62%),
    radial-gradient(ellipse 50% 42% at 85% 15%, rgba(167, 139, 250, 0.14), transparent 65%),
    radial-gradient(ellipse 46% 40% at 10% 30%, rgba(244, 114, 182, 0.1), transparent 65%),
    radial-gradient(ellipse 60% 50% at 50% 110%, rgba(34, 211, 238, 0.12), transparent 60%);
}
.auth-aurora {
  position: absolute; pointer-events: none;
  border-radius: 50%;
  filter: blur(100px);
  will-change: transform;
}
.auth-aurora-1 {
  width: 46vw; height: 46vw;
  top: -16vw; left: -12vw;
  background: radial-gradient(circle, rgba(34,211,238,0.12) 0%, rgba(59,130,246,0.06) 45%, transparent 70%);
  animation: auth-drift-1 26s ease-in-out infinite alternate;
}
.auth-aurora-2 {
  width: 40vw; height: 40vw;
  bottom: -14vw; right: -10vw;
  background: radial-gradient(circle, rgba(167,139,250,0.10) 0%, rgba(244,114,182,0.05) 45%, transparent 70%);
  animation: auth-drift-2 32s ease-in-out infinite alternate;
}
:global(html.dark .auth-page .auth-aurora-1) {
  background: radial-gradient(circle, rgba(34,211,238,0.18) 0%, rgba(59,130,246,0.1) 45%, transparent 70%);
}
:global(html.dark .auth-page .auth-aurora-2) {
  background: radial-gradient(circle, rgba(167,139,250,0.16) 0%, rgba(244,114,182,0.08) 45%, transparent 70%);
}
@keyframes auth-drift-1 {
  0%   { transform: translate(0, 0) scale(1); }
  100% { transform: translate(10vw, 8vh) scale(1.15); }
}
@keyframes auth-drift-2 {
  0%   { transform: translate(0, 0) scale(1.08); }
  100% { transform: translate(-9vw, -9vh) scale(0.94); }
}
.auth-grid {
  position: absolute; inset: 0; pointer-events: none;
  background-image: radial-gradient(circle, rgba(15,23,42,0.16) 1px, transparent 1px);
  background-size: 34px 34px;
  opacity: 0.55;
  -webkit-mask-image: radial-gradient(ellipse 85% 70% at 50% 20%, black 30%, transparent 80%);
  mask-image: radial-gradient(ellipse 85% 70% at 50% 20%, black 30%, transparent 80%);
}
:global(html.dark .auth-page .auth-grid) {
  background-image: radial-gradient(circle, rgba(255,255,255,0.22) 1px, transparent 1px);
  opacity: 0.6;
}

/* ═══ BRAND ══════════════════════════════ */
.auth-logo {
  display: inline-flex;
  width: 64px; height: 64px;
  margin-bottom: 16px;
  border-radius: 18px;
  overflow: hidden;
  border: 1px solid rgba(34, 211, 238, 0.35);
  box-shadow:
    0 0 24px rgba(34, 211, 238, 0.25),
    0 8px 24px rgba(15, 23, 42, 0.08);
}
.auth-title {
  font-size: 1.9rem;
  font-weight: 800;
  letter-spacing: -0.03em;
  margin-bottom: 6px;
  background: linear-gradient(135deg, #0f172a 30%, #0e7490 70%, #7c3aed 100%);
  -webkit-background-clip: text; background-clip: text;
  -webkit-text-fill-color: transparent;
}
:global(html.dark .auth-page .auth-title) {
  background: linear-gradient(135deg, #fff 30%, #5eead4 70%, #a78bfa 100%);
  -webkit-background-clip: text; background-clip: text;
  -webkit-text-fill-color: transparent;
}
.auth-subtitle { font-size: 13px; color: rgba(15, 23, 42, 0.45); }
:global(html.dark .auth-page .auth-subtitle) { color: rgba(255, 255, 255, 0.4); }

/* ═══ CARD ═══════════════════════════════ */
.auth-card {
  position: relative;
  border-radius: 20px;
  padding: 32px;
  background: rgba(255, 255, 255, 0.85);
  backdrop-filter: blur(16px);
  border: 1px solid rgba(15, 23, 42, 0.08);
  box-shadow:
    0 1px 2px rgba(15, 23, 42, 0.04),
    0 24px 60px rgba(15, 23, 42, 0.1);
  overflow: hidden;
}
:global(html.dark .auth-page .auth-card) {
  background: rgba(255, 255, 255, 0.03);
  border-color: rgba(255, 255, 255, 0.1);
  box-shadow: 0 24px 70px rgba(0, 0, 0, 0.55);
}
/* 顶部青→紫渐变细线（呼应首页特性卡） */
.auth-card-topline {
  position: absolute; top: 0; left: 0; right: 0; height: 2px;
  background: linear-gradient(90deg, transparent, #22d3ee 30%, #a78bfa 70%, transparent);
  opacity: 0.75;
}

/* ═══ PAGE FOOTER: DISCLAIMER + COPYRIGHT ═ */
.auth-footer {
  width: 100%;
  padding: 28px 16px 8px;
  flex-shrink: 0;
}
.auth-disclaimer {
  text-align: center;
  font-size: 11px;
  line-height: 1.8;
  color: rgba(15, 23, 42, 0.32);
  max-width: 520px;
  margin-left: auto;
  margin-right: auto;
}
:global(html.dark .auth-page .auth-disclaimer) { color: rgba(255, 255, 255, 0.25); }
.auth-copy {
  margin-top: 8px;
  margin-bottom: 8px;
  text-align: center;
  font-size: 11px;
  color: rgba(15, 23, 42, 0.3);
}
:global(html.dark .auth-page .auth-copy) { color: rgba(255, 255, 255, 0.25); }

/* ═══ REDUCED MOTION ═════════════════════ */
@media (prefers-reduced-motion: reduce) {
  .auth-aurora { animation: none; }
}
</style>

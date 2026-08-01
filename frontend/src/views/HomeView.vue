  <!-- Custom Home Content: Full Page Mode -->
  <div v-if="hasHomeContent" class="min-h-screen">
    <iframe v-if="isHomeContentUrl" :src="homeContent.trim()" class="h-screen w-full border-0"
      allow="clipboard-write; clipboard-read; fullscreen" allowfullscreen />
    <div v-else v-html="homeContent" />
  </div>

  <!-- Compact Home Page (upstream feature) -->
  <div
    v-else-if="compactHomeEnabled"
    data-testid="compact-home"
    class="flex min-h-screen flex-col bg-gray-50 text-gray-900 dark:bg-dark-950 dark:text-white"
  >
    <header class="border-b border-gray-200 px-4 py-4 sm:px-6 dark:border-dark-800">
      <nav class="mx-auto flex max-w-5xl flex-wrap items-center justify-between gap-3 sm:gap-4">
        <div class="flex min-w-0 flex-1 items-center gap-3">
          <img :src="siteLogo || '/logo.svg'" alt="Logo" class="h-9 w-9 shrink-0 rounded-lg object-contain" />
          <span class="min-w-0 truncate text-base font-semibold">{{ siteName }}</span>
        </div>
        <div class="flex max-w-full shrink-0 flex-wrap items-center justify-end gap-2">
          <LocaleSwitcher />
          <a v-if="docUrl" :href="docUrl" target="_blank" rel="noopener noreferrer"
            class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg text-gray-500 hover:bg-gray-100 dark:text-dark-400 dark:hover:bg-dark-800"
            :title="t('home.viewDocs')">
            <Icon name="book" size="md" />
          </a>
          <button
            class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg text-gray-500 hover:bg-gray-100 dark:text-dark-400 dark:hover:bg-dark-800"
            :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
            @click="toggleTheme">
            <Icon v-if="isDark" name="sun" size="md" />
            <Icon v-else name="moon" size="md" />
          </button>
          <router-link :to="isAuthenticated ? dashboardPath : '/login'"
            class="inline-flex min-h-10 shrink-0 items-center justify-center rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white hover:bg-gray-800 dark:bg-white dark:text-gray-900 dark:hover:bg-gray-200">
            {{ isAuthenticated ? t('home.dashboard') : t('home.login') }}
          </router-link>
        </div>
      </nav>
    </header>
    <main class="flex min-w-0 flex-1 items-center justify-center px-4 py-16 sm:px-6">
      <div class="min-w-0 max-w-2xl text-center">
        <img :src="siteLogo || '/logo.svg'" alt="Logo" class="mx-auto mb-6 h-20 w-20 rounded-2xl object-contain" />
        <h1 class="[overflow-wrap:anywhere] text-3xl font-bold md:text-4xl">{{ siteName }}</h1>
        <p class="mt-4 whitespace-pre-wrap [overflow-wrap:anywhere] text-base text-gray-600 dark:text-dark-300">{{ siteSubtitle }}</p>
        <router-link :to="isAuthenticated ? dashboardPath : '/login'"
          class="mt-8 inline-flex min-h-10 items-center justify-center rounded-lg bg-primary-600 px-5 py-2.5 text-sm font-medium text-white hover:bg-primary-700">
          {{ isAuthenticated ? t('home.goToDashboard') : t('home.login') }}
        </router-link>
      </div>
    </main>
    <footer class="min-w-0 border-t border-gray-200 px-4 py-5 text-center text-sm text-gray-500 [overflow-wrap:anywhere] sm:px-6 dark:border-dark-800 dark:text-dark-400">
      &copy; {{ currentYear }} {{ siteName }}
    </footer>
  </div>

  <!-- ════════ ElevenLabs-style: pure black · neon accents · data flow ════════ -->
  <div v-else class="el-page" :class="{ light: !isDark }">

    <!-- ── BACKGROUND LAYERS ── -->
    <div class="bg-glow" aria-hidden="true" />
    <div class="bg-aurora bg-aurora-1" aria-hidden="true" />
    <div class="bg-aurora bg-aurora-2" aria-hidden="true" />
    <div class="bg-grid" aria-hidden="true" />
    <canvas ref="starRef" class="bg-stars" aria-hidden="true" />
    <SplitGlobe />

    <!-- ── NAV ── -->
    <header class="el-nav">
      <nav class="el-nav-in">
        <a class="el-logo" href="#">
          <img :src="siteLogo || '/logo.svg'" class="el-logo-img" alt="logo" />
          <span class="el-logo-txt">{{ siteName }}</span>
        </a>
        <div class="el-nav-r">
          <LocaleSwitcher />
          <a v-if="docUrl" :href="docUrl" target="_blank" rel="noopener noreferrer" class="el-nav-lnk">
            {{ t('home.viewDocs') }}
          </a>
          <button
            @click="toggleTheme"
            class="el-theme-btn"
            :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
          >
            <Icon v-if="isDark" name="sun" size="sm" />
            <Icon v-else name="moon" size="sm" />
          </button>
          <router-link v-if="isAuthenticated" :to="dashboardPath" class="el-btn-dark el-btn-sm">
            {{ t('home.dashboard') }}
          </router-link>
          <router-link v-else to="/login" class="el-btn-dark el-btn-sm">{{ t('home.login') }}</router-link>
        </div>
      </nav>
    </header>

    <main class="el-main">
      <!-- ══ HERO ══ -->
      <section class="el-hero">
        <p class="el-kicker rv">AI API GATEWAY PLATFORM</p>
        <h1 class="el-h1 rv">
          <span class="h1-line1">{{ siteName }}</span>
          <span class="h1-line2">{{ t('home.heroTaglinePrefix') }}<span class="h1-accent">{{ t('home.heroTaglineAccent') }}</span></span>
        </h1>
        <p class="el-sub rv">{{ siteSubtitle }}</p>
        <!-- Base URL swap widget -->
        <div class="el-baseurl rv">
          <p class="el-baseurl-label">{{ t('home.baseUrlLabel') }}</p>
          <div class="el-baseurl-box">
            <span class="el-baseurl-origin">{{ gatewayOrigin }}</span>
            <Transition name="path-swap" mode="out-in">
              <span class="el-baseurl-path" :key="apiPathIdx">{{ apiPaths[apiPathIdx] }}</span>
            </Transition>
            <button class="el-baseurl-copy" @click="copyBaseUrl" :title="t('home.baseUrlCopied')">
              <svg v-if="!urlCopied" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="h-4 w-4">
                <rect x="9" y="9" width="13" height="13" rx="2" /><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
              </svg>
              <svg v-else viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" class="h-4 w-4 text-emerald-400">
                <path d="M20 6 9 17l-5-5" />
              </svg>
            </button>
          </div>
        </div>
        <div class="el-cta-row rv">
          <router-link :to="isAuthenticated ? dashboardPath : '/login'" class="el-btn-white">
            {{ isAuthenticated ? t('home.goToDashboard') : t('home.getStarted') }}
          </router-link>
          <a v-if="docUrl" :href="docUrl" target="_blank" rel="noopener noreferrer" class="el-btn-dark">
            {{ t('home.viewDocs') }}
          </a>
        </div>
        <!-- waveform -->
        <div class="el-wave-wrap rv">
          <canvas ref="waveRef" class="el-wave-cv" aria-hidden="true" />
          <div class="el-wave-caption">
            <span class="el-live-dot" /> GATEWAY TRAFFIC
          </div>
        </div>
      </section>

      <!-- ══ GATEWAY FLOW ══ -->
      <section class="el-flow-sec rv">
        <p class="el-kicker center">HOW IT WORKS</p>
        <h2 class="el-h2">{{ t('home.flowTitle') }}</h2>
        <div class="el-flow-wrap">
          <svg class="el-flow-svg" viewBox="0 0 900 440" fill="none">
            <!-- paths: platform -> hub -->
            <path v-for="(_, i) in platforms" :key="'pt'+i" :id="'fp'+i"
              :d="flowPath(i)" class="el-fpath" />
            <!-- path: hub -> app -->
            <path id="fout" d="M 492 220 C 600 220, 660 220, 760 220" class="el-fpath el-fpath-out" />

            <!-- flowing particles (larger, brighter) -->
            <g v-for="(p, i) in platforms" :key="'pp'+i">
              <circle v-for="k in 3" :key="k" r="4.5" :fill="p.color" :opacity="0.95">
                <animateMotion :dur="`${2 + i * 0.25}s`" repeatCount="indefinite"
                  :begin="`${(k - 1) * ((2 + i * 0.25) / 3)}s`">
                  <mpath :href="'#fp' + i" />
                </animateMotion>
              </circle>
            </g>
            <circle v-for="k in 5" :key="'out'+k" r="4.5" fill="#22d3ee" opacity="1">
              <animateMotion dur="1.5s" repeatCount="indefinite" :begin="`${(k - 1) * 0.3}s`">
                <mpath href="#fout" />
              </animateMotion>
            </circle>

            <!-- platform nodes -->
            <g v-for="(p, i) in platforms" :key="'pn'+i">
              <rect :x="30" :y="nodeY(i) - 22" width="130" height="44" rx="10" class="el-node-box" />
              <circle :cx="56" :cy="nodeY(i)" r="9" :fill="p.color" opacity="0.92" />
              <text :x="74" :y="nodeY(i) + 4.5" class="el-node-txt">{{ p.name }}</text>
            </g>

            <!-- hub -->
            <circle cx="450" cy="220" r="58" class="el-hub-pulse" />
            <circle cx="450" cy="220" r="44" class="el-hub-core" />
            <text x="450" y="215" text-anchor="middle" class="el-hub-txt">{{ siteName }}</text>
            <text x="450" y="233" text-anchor="middle" class="el-hub-sub">GATEWAY</text>

            <!-- app node -->
            <rect x="760" y="192" width="112" height="56" rx="12" class="el-node-box el-node-app" />
            <text x="816" y="215" text-anchor="middle" class="el-node-txt">{{ t('home.flowYourApp') }}</text>
            <text x="816" y="233" text-anchor="middle" class="el-node-sub">ONE API KEY</text>
          </svg>
        </div>
      </section>

      <!-- ══ COMPAT STRIP ══ -->
      <section class="el-compat rv">
        <p class="el-kicker center">{{ t('home.compat.title') }}</p>
        <div class="el-compat-row">
          <div v-for="c in compatItems" :key="c.label" class="el-compat-chip">
            <svg class="el-compat-check" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
              <path d="M20 6 9 17l-5-5" />
            </svg>
            <div class="el-compat-txt">
              <span class="el-compat-label">{{ c.label }}</span>
              <span class="el-compat-sub">{{ c.sub }}</span>
            </div>
          </div>
        </div>
      </section>

      <!-- ══ STATS ══ -->
      <section class="el-stats rv">
        <div class="el-stat">
          <span class="el-stat-v">{{ n1 }}+</span>
          <span class="el-stat-l">AI PLATFORMS</span>
        </div>
        <div class="el-stat-sep" />
        <div class="el-stat">
          <span class="el-stat-v">{{ n2 }}<span class="el-stat-unit">%</span></span>
          <span class="el-stat-l">UPTIME</span>
        </div>
        <div class="el-stat-sep" />
        <div class="el-stat">
          <span class="el-stat-v">{{ n3 }}<span class="el-stat-unit">ms</span></span>
          <span class="el-stat-l">AVG LATENCY</span>
        </div>
        <div class="el-stat-sep" />
        <div class="el-stat">
          <span class="el-stat-v">24/7</span>
          <span class="el-stat-l">REALTIME BILLING</span>
        </div>
      </section>

      <!-- ══ FEATURES ══ -->
      <section class="el-feat-sec">
        <p class="el-kicker center rv">CAPABILITIES</p>
        <h2 class="el-h2 rv">{{ t('home.capabilitiesTitle') }}</h2>
        <div class="el-feat-grid">
          <div v-for="(f, i) in features" :key="i" class="el-feat rv"
            :style="`--fc:${f.color}; transition-delay:${i * 0.08}s`">
            <div class="el-feat-num">0{{ i + 1 }}</div>
            <h3 class="el-feat-h">{{ f.title }}</h3>
            <p class="el-feat-p">{{ f.desc }}</p>
            <div class="el-feat-line" :style="`background:${f.color}`" />
          </div>
        </div>
      </section>

      <!-- ══ TERMINAL ══ -->
      <section class="el-term-sec rv">
        <p class="el-kicker center">QUICK START</p>
        <h2 class="el-h2">{{ t('home.quickstartTitle') }}</h2>
        <div class="el-term">
          <div class="el-term-hd">
            <div class="el-term-tabs">
              <button
                v-for="tab in termTabs" :key="tab.id"
                class="el-term-tab" :class="{ active: termTab === tab.id }"
                @click="termTab = tab.id"
              >{{ tab.label }}</button>
            </div>
            <span class="el-term-live"><span class="el-live-dot" />CONNECTED</span>
          </div>
          <div class="el-term-bd" :key="termTab">
            <div v-for="(line, i) in activeTermLines" :key="i" class="el-tl" :style="`animation-delay:${i * 0.12 + 0.1}s`">
              <span v-for="(seg, j) in line" :key="j" :class="seg.cls">{{ seg.txt }}</span>
            </div>
            <div v-if="termTab === 'curl'" class="el-tl" style="animation-delay:1.3s"><span class="tp">$</span> <span class="el-cursor" /></div>
          </div>
        </div>
      </section>

      <!-- ══ FAQ ══ -->
      <section class="el-faq-sec rv">
        <p class="el-kicker center">FAQ</p>
        <h2 class="el-h2">{{ t('home.faq.title') }}</h2>
        <div class="el-faq-list">
          <div v-for="n in 6" :key="n" class="el-faq-item" :class="{ open: faqOpen === n }">
            <button class="el-faq-q" @click="faqOpen = faqOpen === n ? 0 : n">
              <span>{{ t(`home.faq.q${n}`) }}</span>
              <svg class="el-faq-arrow" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="m6 9 6 6 6-6" />
              </svg>
            </button>
            <div class="el-faq-a-wrap">
              <p class="el-faq-a">{{ t(`home.faq.a${n}`) }}</p>
            </div>
          </div>
        </div>
      </section>

      <!-- ══ FINAL CTA ══ -->
      <section class="el-final rv">
        <h2 class="el-final-h">{{ t('home.finalCtaTitle') }}</h2>
        <p class="el-final-p">{{ t('home.finalCtaDesc') }}</p>
        <router-link :to="isAuthenticated ? dashboardPath : '/login'" class="el-btn-white el-btn-lg">
          {{ isAuthenticated ? t('home.goToDashboard') : t('home.getStarted') }} →
        </router-link>
      </section>
    </main>

    <!-- ── FOOTER ── -->
    <footer class="el-footer">
      <p class="el-ft-disclaimer">{{ t('home.footer.disclaimer', { siteName }) }}</p>
      <div class="el-ft-row">
        <p class="el-ft-c">© {{ year }} {{ siteName }}</p>
        <div class="el-ft-r">
          <a v-if="docUrl" :href="docUrl" target="_blank" rel="noopener noreferrer" class="el-ft-a">{{ t('home.docs') }}</a>
        </div>
      </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore, useAppStore } from '@/stores'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'
import { sanitizeUrl } from '@/utils/url'
import SplitGlobe from '@/components/common/SplitGlobe.vue'

const { t, locale } = useI18n()
const authStore = useAuthStore()
const appStore = useAppStore()

const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || 'Sub2API')
const siteLogo = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
// site_subtitle 支持两种配置：纯文本（原样显示）或 JSON 多语言映射
// 例如 {"zh":"专业的 AI 网关平台","en":"Professional AI Gateway"}
const siteSubtitle = computed(() => {
  const raw = (appStore.cachedPublicSettings?.site_subtitle || '').trim()
  if (!raw) return t('home.heroSubtitle')
  if (raw.startsWith('{')) {
    try {
      const map = JSON.parse(raw) as Record<string, string>
      const lang = locale.value.slice(0, 2)
      return map[lang] || map.zh || map.en || Object.values(map)[0] || t('home.heroSubtitle')
    } catch {
      // 非法 JSON 按纯文本处理
    }
  }
  return raw
})
const docUrl = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.doc_url || appStore.docUrl || ''))
const homeContent = computed(() => appStore.cachedPublicSettings?.home_content || '')
const isHomeContentUrl = computed(() => { const c = homeContent.value.trim(); return c.startsWith('http://') || c.startsWith('https://') })
const hasHomeContent = computed(() => homeContent.value.trim().length > 0)
const compactHomeEnabled = computed(() => appStore.cachedPublicSettings?.compact_home_enabled === true)
const isAuthenticated = computed(() => authStore.isAuthenticated)
const isAdmin = computed(() => authStore.isAdmin)
const dashboardPath = computed(() => isAdmin.value ? '/admin/dashboard' : '/dashboard')
const year = new Date().getFullYear()

// ── gateway flow diagram ──
const platforms = [
  { name: 'Claude',      color: '#f97316' },
  { name: 'GPT',         color: '#22c55e' },
  { name: 'Gemini',      color: '#3b82f6' },
  { name: 'Antigravity', color: '#ec4899' },
  { name: 'Grok',        color: '#a78bfa' },
]
function nodeY(i: number) { return 60 + i * 80 }
function flowPath(i: number) {
  const y = nodeY(i)
  return `M 160 ${y} C 280 ${y}, 320 220, 406 220`
}

// ── counters ──
const n1 = ref(0), n2 = ref(0), n3 = ref(0)
function counter(target: number, set: (v: number) => void, ms = 1500, decimals = 0) {
  const s = performance.now()
  const tick = (now: number) => {
    const p = Math.min((now - s) / ms, 1)
    set(Number((p * target).toFixed(decimals)))
    if (p < 1) requestAnimationFrame(tick)
  }
  requestAnimationFrame(tick)
}

// ── base url widget ──
// 演示数据；网关地址取当前站点真实域名，访客可直接复制使用
const gatewayOrigin = window.location.origin
const apiPaths = ['/v1/messages', '/v1/chat/completions', '/v1/responses', '/v1/images/generations']
const apiPathIdx = ref(0)
let pathTimer = 0
const urlCopied = ref(false)
async function copyBaseUrl() {
  try {
    await navigator.clipboard.writeText(gatewayOrigin)
    urlCopied.value = true
    setTimeout(() => { urlCopied.value = false }, 2000)
  } catch {
    // clipboard unavailable (non-secure context); ignore
  }
}

// ── compat strip ──
const compatItems = computed(() => [
  { label: t('home.compat.anthropicSdk'), sub: t('home.compat.anthropicSdkSub') },
  { label: t('home.compat.openaiSdk'), sub: t('home.compat.openaiSdkSub') },
  { label: t('home.compat.geminiNative'), sub: t('home.compat.geminiNativeSub') },
  { label: t('home.compat.streaming'), sub: t('home.compat.streamingSub') },
  { label: t('home.compat.functionCalling'), sub: t('home.compat.functionCallingSub') },
  { label: t('home.compat.promptCache'), sub: t('home.compat.promptCacheSub') },
])

// ── FAQ ──
const faqOpen = ref(0)

// ── terminal（curl/Python/Node.js 三 tab）──
const termTabs = [
  { id: 'curl', label: 'curl' },
  { id: 'python', label: 'Python' },
  { id: 'node', label: 'Node.js' },
] as const
const termTab = ref<'curl' | 'python' | 'node'>('curl')

const termLinesCurl = [
  [{ cls: 'tp', txt: '$' }, { cls: 'tc', txt: ' curl' }, { cls: 'tf', txt: ' -X POST' }, { cls: 'tu', txt: ` ${gatewayOrigin}/v1/messages` }],
  [{ cls: 'tk', txt: '    -H' }, { cls: 'ts', txt: ' "Authorization: Bearer sk-..."' }],
  [{ cls: 'tk', txt: '    -d' }, { cls: 'ts', txt: ' \'{"model":"claude-opus-5","messages":[...]}\'' }],
  [{ cls: 'tcm', txt: '→ routing · claude-pro-007 · load 15%' }],
  [{ cls: 'tok', txt: '200 OK' }, { cls: 'tms', txt: '  138ms · 1.2k tokens' }],
  [{ cls: 'tj', txt: '{ "content": [{"type":"text","text":"Hello!"}] }' }],
]
const termLinesPython = [
  [{ cls: 'tf', txt: 'import' }, { cls: 'tk', txt: ' anthropic' }],
  [{ cls: 'tk', txt: '' }],
  [{ cls: 'tk', txt: 'client = anthropic.' }, { cls: 'tc', txt: 'Anthropic' }, { cls: 'tk', txt: '(' }],
  [{ cls: 'tk', txt: '    api_key=' }, { cls: 'ts', txt: '"sk-..."' }, { cls: 'tk', txt: ',' }],
  [{ cls: 'tk', txt: '    base_url=' }, { cls: 'ts', txt: `"${gatewayOrigin}"` }],
  [{ cls: 'tk', txt: ')' }],
  [{ cls: 'tk', txt: 'msg = client.messages.' }, { cls: 'tc', txt: 'create' }, { cls: 'tk', txt: '(' }],
  [{ cls: 'tk', txt: '    model=' }, { cls: 'ts', txt: '"claude-opus-5"' }, { cls: 'tk', txt: ', messages=[...]' }],
  [{ cls: 'tk', txt: ')' }],
  [{ cls: 'tc', txt: 'print' }, { cls: 'tk', txt: '(msg.content[0].text)' }],
]
const termLinesNode = [
  [{ cls: 'tf', txt: 'import' }, { cls: 'tk', txt: ' Anthropic ' }, { cls: 'tf', txt: 'from' }, { cls: 'ts', txt: " '@anthropic-ai/sdk'" }],
  [{ cls: 'tk', txt: '' }],
  [{ cls: 'tf', txt: 'const' }, { cls: 'tk', txt: ' client = ' }, { cls: 'tf', txt: 'new' }, { cls: 'tc', txt: ' Anthropic' }, { cls: 'tk', txt: '({' }],
  [{ cls: 'tk', txt: '  apiKey: ' }, { cls: 'ts', txt: "'sk-...'" }, { cls: 'tk', txt: ',' }],
  [{ cls: 'tk', txt: '  baseURL: ' }, { cls: 'ts', txt: `'${gatewayOrigin}'` }],
  [{ cls: 'tk', txt: '})' }],
  [{ cls: 'tf', txt: 'const' }, { cls: 'tk', txt: ' msg = ' }, { cls: 'tf', txt: 'await' }, { cls: 'tk', txt: ' client.messages.' }, { cls: 'tc', txt: 'create' }, { cls: 'tk', txt: '({' }],
  [{ cls: 'tk', txt: '  model: ' }, { cls: 'ts', txt: "'claude-opus-5'" }, { cls: 'tk', txt: ', messages: [...]' }],
  [{ cls: 'tk', txt: '})' }],
  [{ cls: 'tk', txt: 'console.' }, { cls: 'tc', txt: 'log' }, { cls: 'tk', txt: '(msg.content[0].text)' }],
]
const activeTermLines = computed(() => {
  if (termTab.value === 'python') return termLinesPython
  if (termTab.value === 'node') return termLinesNode
  return termLinesCurl
})

// ── features ──
const features = computed(() => [
  { title: t('home.features.unifiedGateway'), desc: t('home.features.unifiedGatewayDesc'), color: '#22d3ee' },
  { title: t('home.features.multiAccount'),   desc: t('home.features.multiAccountDesc'),   color: '#a78bfa' },
  { title: t('home.features.balanceQuota'),   desc: t('home.features.balanceQuotaDesc'),   color: '#f472b6' },
])

// ── theme ──
// 默认亮色；用户切换后记忆到 localStorage
const isDark = ref(localStorage.getItem('theme') === 'dark')
function applyTheme() {
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
}
function toggleTheme() {
  isDark.value = !isDark.value
  applyTheme()
  // reduced-motion 下 canvas 不循环，主题切换后手动重画一帧
  if (reducedMotion) { redrawWave?.(); redrawStars?.() }
}

// 系统开启「减弱动态效果」时：canvas 只画静态首帧，轮换/循环动画不启动
const reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches

// ── starfield background ──
const starRef = ref<HTMLCanvasElement | null>(null)
let starRaf = 0
let redrawStars: (() => void) | null = null
let redrawWave: (() => void) | null = null
function initStars(cv: HTMLCanvasElement) {
  const ctx = cv.getContext('2d')!
  const fit = () => { cv.width = innerWidth; cv.height = innerHeight }
  fit(); addEventListener('resize', fit)
  const stars = Array.from({ length: 110 }, () => ({
    x: Math.random() * cv.width,
    y: Math.random() * cv.height,
    r: Math.random() * 1.4 + 0.4,
    a: Math.random() * 0.55 + 0.18,
    vy: Math.random() * 0.08 + 0.02,
    tw: Math.random() * Math.PI * 2,
  }))
  let tk = 0
  const draw = () => {
    ctx.clearRect(0, 0, cv.width, cv.height)
    stars.forEach(s => {
      const twinkle = 0.6 + Math.sin(tk * 0.02 + s.tw) * 0.4
      ctx.fillStyle = isDark.value
        ? `rgba(180, 230, 255, ${s.a * twinkle})`
        : `rgba(37, 99, 235, ${s.a * twinkle * 0.35})`
      ctx.beginPath()
      ctx.arc(s.x, s.y, s.r, 0, Math.PI * 2)
      ctx.fill()
      s.y -= s.vy
      if (s.y < -2) { s.y = cv.height + 2; s.x = Math.random() * cv.width }
    })
    tk++
    if (!reducedMotion) starRaf = requestAnimationFrame(draw)
  }
  redrawStars = draw
  draw()
}

// ── waveform bars ──
const waveRef = ref<HTMLCanvasElement | null>(null)
let waveRaf = 0
function initWave(cv: HTMLCanvasElement) {
  const ctx = cv.getContext('2d')!
  const fit = () => {
    cv.width = cv.offsetWidth * devicePixelRatio
    cv.height = cv.offsetHeight * devicePixelRatio
    ctx.setTransform(devicePixelRatio, 0, 0, devicePixelRatio, 0, 0)
  }
  fit(); addEventListener('resize', fit)
  let tk = 0
  const draw = () => {
    const W = cv.offsetWidth, H = cv.offsetHeight
    ctx.clearRect(0, 0, W, H)
    const bw = 4, gap = 2, n = Math.floor(W / (bw + gap))
    const cy = H / 2
    for (let i = 0; i < n; i++) {
      const x = i * (bw + gap)
      const env = Math.pow(Math.sin((i / n) * Math.PI), 0.7)
      const amp = Math.abs(
        Math.sin(i * 0.16 + tk * 0.042) * 0.6 +
        Math.sin(i * 0.06 - tk * 0.028) * 0.4
      )
      const h = (amp * env * H * 0.88 + 3)
      // 色相沿条形位置 + 时间流动：青(185) → 蓝 → 紫 → 粉(325)
      const hue = 185 + ((Math.sin(i * 0.045 + tk * 0.012) + 1) / 2) * 140
      const alpha = 0.55 + env * 0.4
      if (isDark.value) {
        ctx.shadowBlur = 8
        ctx.shadowColor = `hsla(${hue}, 95%, 65%, 0.6)`
        ctx.fillStyle = `hsla(${hue}, 95%, 70%, ${alpha})`
      } else {
        ctx.shadowBlur = 4
        ctx.shadowColor = `hsla(${hue}, 80%, 45%, 0.35)`
        ctx.fillStyle = `hsla(${hue}, 80%, 46%, ${alpha * 0.9})`
      }
      ctx.fillRect(x, cy - h / 2, bw, h)
      ctx.shadowBlur = 0
    }
    tk++
    if (!reducedMotion) waveRaf = requestAnimationFrame(draw)
  }
  redrawWave = draw
  draw()
}

// ── scroll reveal ──
function initReveal() {
  const ob = new IntersectionObserver(es => {
    es.forEach(e => { if (e.isIntersecting) { e.target.classList.add('on'); ob.unobserve(e.target) } })
  }, { threshold: 0.12 })
  document.querySelectorAll('.rv').forEach(el => ob.observe(el))
}

onMounted(() => {
  applyTheme()
  authStore.checkAuth()
  if (!appStore.publicSettingsLoaded) appStore.fetchPublicSettings()
  if (waveRef.value) initWave(waveRef.value)
  if (starRef.value) initStars(starRef.value)
  // 两半相位一致 —— 是同一颗地球被剖开分置左右边界
  initReveal()
  if (!reducedMotion) {
    pathTimer = window.setInterval(() => {
      apiPathIdx.value = (apiPathIdx.value + 1) % apiPaths.length
    }, 2600)
  }
  setTimeout(() => counter(5, v => n1.value = v), 300)
  setTimeout(() => counter(99.9, v => n2.value = v, 1500, 1), 450)
  setTimeout(() => counter(138, v => n3.value = v), 600)
})
onUnmounted(() => {
  cancelAnimationFrame(waveRaf)
  cancelAnimationFrame(starRaf)
  clearInterval(pathTimer)
})
</script>

<style scoped>
/* ═══ BASE ═══════════════════════════════ */
.el-page {
  min-height: 100vh;
  display: flex; flex-direction: column;
  background: #000;
  color: #fff;
  font-feature-settings: 'ss01';
  overflow-x: hidden;
}
.el-main { flex: 1; position: relative; z-index: 1; }
.el-footer { position: relative; z-index: 1; }

/* ═══ BACKGROUND LAYERS ═══════════════════ */
.bg-glow {
  position: fixed; inset: 0; z-index: 0; pointer-events: none;
  background:
    radial-gradient(ellipse 80% 60% at 50% -15%, rgba(34, 211, 238, 0.30), transparent 62%),
    radial-gradient(ellipse 50% 42% at 85% 15%, rgba(167, 139, 250, 0.18), transparent 65%),
    radial-gradient(ellipse 46% 40% at 10% 30%, rgba(244, 114, 182, 0.12), transparent 65%),
    radial-gradient(ellipse 60% 50% at 50% 110%, rgba(34, 211, 238, 0.14), transparent 60%);
}
/* aurora drifting blobs */
.bg-aurora {
  position: fixed; z-index: 0; pointer-events: none;
  border-radius: 50%;
  filter: blur(110px);
  will-change: transform;
}
.bg-aurora-1 {
  width: 55vw; height: 55vw;
  top: -18vw; left: -12vw;
  background: radial-gradient(circle,
    rgba(34, 211, 238, 0.22) 0%,
    rgba(59, 130, 246, 0.12) 45%,
    transparent 70%);
  animation: aurora-drift-1 26s ease-in-out infinite alternate;
}
.bg-aurora-2 {
  width: 48vw; height: 48vw;
  bottom: -15vw; right: -10vw;
  background: radial-gradient(circle,
    rgba(167, 139, 250, 0.20) 0%,
    rgba(244, 114, 182, 0.10) 45%,
    transparent 70%);
  animation: aurora-drift-2 32s ease-in-out infinite alternate;
}
@keyframes aurora-drift-1 {
  0%   { transform: translate(0, 0) scale(1); }
  100% { transform: translate(14vw, 10vh) scale(1.18); }
}
@keyframes aurora-drift-2 {
  0%   { transform: translate(0, 0) scale(1.1); }
  100% { transform: translate(-12vw, -12vh) scale(0.92); }
}
.bg-grid {
  position: fixed; inset: 0; z-index: 0; pointer-events: none;
  background-image: radial-gradient(circle, rgba(255,255,255,0.28) 1px, transparent 1px);
  background-size: 34px 34px;
  opacity: 0.85;
  -webkit-mask-image: radial-gradient(ellipse 85% 65% at 50% 8%, black 30%, transparent 80%);
  mask-image: radial-gradient(ellipse 85% 65% at 50% 8%, black 30%, transparent 80%);
}
.bg-stars {
  position: fixed; inset: 0; z-index: 0; pointer-events: none;
  width: 100vw; height: 100vh;
}

/* kicker — small caps label, ElevenLabs signature */
.el-kicker {
  font-size: 11px; font-weight: 600;
  letter-spacing: 0.28em;
  color: rgba(255,255,255,0.4);
  text-transform: uppercase;
  margin-bottom: 20px;
}
.el-kicker.center { text-align: center; }

/* ═══ NAV ════════════════════════════════ */
.el-nav {
  position: sticky; top: 0; z-index: 50;
  background: rgba(0,0,0,0.8);
  backdrop-filter: blur(16px);
  border-bottom: 1px solid rgba(255,255,255,0.08);
}
.el-nav-in {
  max-width: 1120px; margin: 0 auto;
  padding: 14px 32px;
  display: flex; align-items: center; justify-content: space-between;
}
.el-logo { display: flex; align-items: center; gap: 10px; text-decoration: none; }
.el-logo-img { width: 30px; height: 30px; border-radius: 8px; }
.el-logo-txt { font-size: 15px; font-weight: 700; letter-spacing: -0.01em; color: #fff; }
.el-nav-r { display: flex; align-items: center; gap: 18px; }
.el-nav-lnk { font-size: 13px; color: rgba(255,255,255,0.55); text-decoration: none; transition: color .15s; }
.el-nav-lnk:hover { color: #fff; }
.el-theme-btn {
  display: inline-flex; align-items: center; justify-content: center;
  width: 32px; height: 32px;
  border-radius: 8px; border: none; background: transparent;
  color: rgba(255,255,255,0.5); cursor: pointer;
  transition: color .15s, background .15s;
}
.el-theme-btn:hover { color: #fff; background: rgba(255,255,255,0.08); }

/* CTA — 引入紫色第二强调色 */
.el-btn-white {
  display: inline-flex; align-items: center; justify-content: center;
  background: #fff; color: #000;
  border-radius: 999px;
  font-weight: 700; text-decoration: none;
  padding: 10px 24px; font-size: 14px;
  transition: transform .15s ease, box-shadow .2s ease, background .15s;
  position: relative; overflow: hidden;
}
.el-btn-white::before {
  content: '';
  position: absolute; inset: -1px;
  border-radius: inherit;
  background: linear-gradient(135deg, #22d3ee, #a78bfa);
  opacity: 0;
  transition: opacity .25s;
  z-index: -1;
}
.el-btn-white:hover { transform: translateY(-1px); box-shadow: 0 0 24px rgba(167,139,250,0.4), 0 0 48px rgba(34,211,238,0.2); background: #f5f0ff; }
.el-btn-dark {
  display: inline-flex; align-items: center; justify-content: center;
  background: transparent; color: rgba(255,255,255,0.85);
  border: 1px solid rgba(255,255,255,0.18);
  border-radius: 999px;
  font-weight: 500; text-decoration: none;
  padding: 10px 24px; font-size: 14px;
  transition: border-color .15s, background .15s;
}
.el-btn-dark:hover { border-color: rgba(255,255,255,0.45); background: rgba(255,255,255,0.05); }
/* 尺寸修饰类放在颜色类之后，保证同特异性下覆盖生效 */
.el-btn-sm { padding: 7px 16px; font-size: 13px; }
.el-btn-lg { padding: 14px 36px; font-size: 16px; }

/* ═══ HERO ═══════════════════════════════ */
.el-hero {
  max-width: 1120px; margin: 0 auto;
  padding: 110px 32px 40px;
  text-align: center;
  position: relative;
}
.el-h1 {
  font-size: clamp(3rem, 8vw, 6.5rem);
  font-weight: 800;
  letter-spacing: -0.02em;
  line-height: 1.04;
  margin-bottom: 26px;
  text-align: center;
}
.h1-line1 {
  display: block;
  background: linear-gradient(180deg, #fff 45%, rgba(255,255,255,0.6));
  -webkit-background-clip: text; background-clip: text; -webkit-text-fill-color: transparent;
  letter-spacing: -0.04em;
  margin-bottom: 6px;
}
.h1-line2 {
  display: block;
  font-size: 0.42em;
  font-weight: 650;
  letter-spacing: 0.01em;
  color: rgba(255,255,255,0.75);
}
.h1-accent {
  background: linear-gradient(135deg, #22d3ee 0%, #a78bfa 60%, #f472b6 100%);
  -webkit-background-clip: text; background-clip: text; -webkit-text-fill-color: transparent;
  animation: accent-hue 8s linear infinite;
}
@keyframes accent-hue { 0%{filter:hue-rotate(0deg)} 100%{filter:hue-rotate(60deg)} }
.el-sub {
  font-size: 1.1rem; line-height: 1.7;
  color: rgba(255,255,255,0.45);
  max-width: 520px; margin: 0 auto 40px;
}
.el-cta-row { display: flex; justify-content: center; gap: 14px; flex-wrap: wrap; margin-bottom: 70px; }

/* base url swap widget */
.el-baseurl { margin: 0 auto 36px; max-width: 560px; }
.el-baseurl-label {
  font-size: 10px; font-weight: 600; letter-spacing: 0.24em;
  text-transform: uppercase;
  color: rgba(255,255,255,0.35);
  margin-bottom: 10px; text-align: center;
}
.el-baseurl-box {
  display: flex; align-items: center;
  border: 1px solid rgba(34,211,238,0.25);
  border-radius: 12px;
  background: rgba(255,255,255,0.03);
  backdrop-filter: blur(8px);
  padding: 12px 14px 12px 18px;
  font-family: ui-monospace, 'Fira Code', monospace;
  font-size: 14px;
  box-shadow: 0 0 30px rgba(34,211,238,0.08);
  transition: border-color .2s, box-shadow .2s;
}
.el-baseurl-box:hover { border-color: rgba(34,211,238,0.45); box-shadow: 0 0 40px rgba(34,211,238,0.15); }
.el-baseurl-origin { color: #fff; font-weight: 600; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.el-baseurl-path { color: #22d3ee; flex: 1; text-align: left; white-space: nowrap; }
.el-baseurl-copy {
  display: inline-flex; align-items: center; justify-content: center;
  width: 32px; height: 32px; flex-shrink: 0;
  border-radius: 8px; border: 1px solid rgba(255,255,255,0.12);
  background: rgba(255,255,255,0.05);
  color: rgba(255,255,255,0.6);
  cursor: pointer;
  transition: color .15s, border-color .15s, background .15s;
}
.el-baseurl-copy:hover { color: #22d3ee; border-color: rgba(34,211,238,0.4); background: rgba(34,211,238,0.08); }
.path-swap-enter-active, .path-swap-leave-active { transition: opacity .25s, transform .25s; }
.path-swap-enter-from { opacity: 0; transform: translateY(6px); }
.path-swap-leave-to { opacity: 0; transform: translateY(-6px); }

/* waveform */
.el-wave-wrap { position: relative; max-width: 860px; margin: 0 auto; }
.el-wave-cv { width: 100%; height: 180px; display: block; }
.el-wave-caption {
  display: flex; align-items: center; justify-content: center; gap: 8px;
  margin-top: 14px;
  font-size: 10px; font-weight: 600; letter-spacing: 0.22em;
  color: rgba(255,255,255,0.3);
}
.el-live-dot {
  width: 6px; height: 6px; border-radius: 50%;
  background: #22d3ee;
  box-shadow: 0 0 8px #22d3ee;
  animation: el-blink 1.4s ease-in-out infinite;
}
@keyframes el-blink { 0%,100%{opacity:1} 50%{opacity:.3} }
/* ═══ SECTION HEADINGS ═══════════════════ */
.el-h2 {
  font-size: clamp(1.8rem, 4vw, 2.6rem);
  font-weight: 750;
  letter-spacing: -0.03em;
  text-align: center;
  margin-bottom: 48px;
  color: #fff;
}

/* ═══ GATEWAY FLOW ═══════════════════════ */
.el-flow-sec {
  max-width: 1120px; margin: 0 auto;
  padding: 65px 32px 40px;
}
.el-flow-wrap {
  border: 1px solid rgba(255,255,255,0.08);
  border-radius: 20px;
  background:
    radial-gradient(ellipse 50% 60% at 50% 50%, rgba(34,211,238,0.06), transparent),
    #030303;
  padding: 20px;
  overflow: hidden;
}
.el-flow-svg { width: 100%; height: auto; display: block; }
.el-fpath { stroke: rgba(255,255,255,0.15); stroke-width: 1.6; fill: none; filter: drop-shadow(0 0 3px rgba(255,255,255,0.12)); }
.el-fpath-out { stroke: rgba(34,211,238,0.5); stroke-width: 2; filter: drop-shadow(0 0 6px rgba(34,211,238,0.4)); }
.el-node-box {
  fill: rgba(255,255,255,0.04);
  stroke: rgba(255,255,255,0.15);
  stroke-width: 1;
}
.el-node-app { stroke: rgba(34,211,238,0.55); fill: rgba(34,211,238,0.07); filter: drop-shadow(0 0 8px rgba(34,211,238,0.3)); }
.el-node-txt { fill: rgba(255,255,255,0.9); font-size: 15px; font-weight: 600; }
.el-node-sub { fill: rgba(34,211,238,0.6); font-size: 8.5px; font-weight: 600; letter-spacing: 0.18em; }
.el-hub-core {
  fill: #050505;
  stroke: rgba(34,211,238,0.85);
  stroke-width: 2;
  filter: drop-shadow(0 0 24px rgba(34,211,238,0.7)) drop-shadow(0 0 48px rgba(34,211,238,0.3));
}
.el-hub-pulse {
  fill: none;
  stroke: rgba(34,211,238,0.5);
  stroke-width: 1.2;
  transform-origin: 450px 220px;
  animation: el-hub-ring 2.6s ease-out infinite;
}
@keyframes el-hub-ring {
  0% { transform: scale(0.75); opacity: 1; }
  100% { transform: scale(1.5); opacity: 0; }
}
.el-hub-txt { fill: #fff; font-size: 16px; font-weight: 800; letter-spacing: -0.01em; }
.el-hub-sub { fill: #22d3ee; font-size: 9px; font-weight: 700; letter-spacing: 0.28em; }

/* ═══ COMPAT STRIP ═══════════════════════ */
.el-compat { max-width: 1120px; margin: 0 auto; padding: 20px 32px 60px; }
.el-compat-row {
  display: grid; gap: 12px;
  grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
  margin-top: 24px;
}
.el-compat-chip {
  display: flex; align-items: center; gap: 10px;
  padding: 14px 16px;
  border: 1px solid rgba(255,255,255,0.08);
  border-radius: 12px;
  background: rgba(255,255,255,0.02);
  transition: border-color .2s, background .2s, transform .2s;
}
.el-compat-chip:hover {
  border-color: rgba(34,211,238,0.35);
  background: rgba(34,211,238,0.04);
  transform: translateY(-2px);
}
.el-compat-check {
  width: 16px; height: 16px; flex-shrink: 0;
  color: #22d3ee;
}
.el-compat-txt { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
.el-compat-label { font-size: 13px; font-weight: 700; color: #fff; white-space: nowrap; }
.el-compat-sub { font-size: 11px; color: rgba(255,255,255,0.35); white-space: nowrap; }

/* ═══ FAQ ════════════════════════════════ */
.el-faq-sec { max-width: 760px; margin: 0 auto; padding: 40px 32px 80px; }
.el-faq-list { display: flex; flex-direction: column; gap: 10px; }
.el-faq-item {
  border: 1px solid rgba(255,255,255,0.08);
  border-radius: 12px;
  background: rgba(255,255,255,0.02);
  overflow: hidden;
  transition: border-color .2s;
}
.el-faq-item.open { border-color: rgba(34,211,238,0.3); }
.el-faq-q {
  width: 100%;
  display: flex; align-items: center; justify-content: space-between; gap: 16px;
  padding: 16px 20px;
  border: none; background: transparent;
  font-size: 14px; font-weight: 600; color: #fff;
  text-align: left; cursor: pointer;
}
.el-faq-arrow {
  width: 16px; height: 16px; flex-shrink: 0;
  color: rgba(255,255,255,0.4);
  transition: transform .25s ease;
}
.el-faq-item.open .el-faq-arrow { transform: rotate(180deg); color: #22d3ee; }
.el-faq-a-wrap {
  display: grid;
  grid-template-rows: 0fr;
  transition: grid-template-rows .3s ease;
}
.el-faq-item.open .el-faq-a-wrap { grid-template-rows: 1fr; }
.el-faq-a {
  overflow: hidden;
  padding: 0 20px;
  font-size: 13px; line-height: 1.8;
  color: rgba(255,255,255,0.45);
  transition: padding .3s ease;
}
.el-faq-item.open .el-faq-a { padding: 0 20px 18px; }

/* ═══ STATS ══════════════════════════════ */
.el-stats {
  max-width: 1120px; margin: 0 auto;
  padding: 70px 32px;
  display: flex; align-items: center; justify-content: center;
  gap: clamp(24px, 6vw, 72px);
  flex-wrap: wrap;
  border-top: 1px solid rgba(255,255,255,0.06);
  border-bottom: 1px solid rgba(255,255,255,0.06);
}
.el-stat { display: flex; flex-direction: column; align-items: center; gap: 8px; }
.el-stat-v {
  font-size: clamp(2rem, 5vw, 3.2rem);
  font-weight: 800; letter-spacing: -0.03em;
  font-variant-numeric: tabular-nums;
  color: #22d3ee;
  text-shadow: 0 0 20px rgba(34,211,238,0.5);
  line-height: 1;
}
.el-stat-unit { font-size: 0.5em; font-weight: 700; color: rgba(255,255,255,0.5); margin-left: 2px; }
.el-stat-l { font-size: 10px; font-weight: 600; letter-spacing: 0.24em; color: rgba(255,255,255,0.32); }
.el-stat-sep { width: 1px; height: 48px; background: rgba(255,255,255,0.08); }

/* ═══ FEATURES ═══════════════════════════ */
.el-feat-sec { max-width: 1120px; margin: 0 auto; padding: 65px 32px; }
.el-feat-grid { display: grid; gap: 1px; grid-template-columns: repeat(auto-fit, minmax(280px, 1fr)); background: rgba(255,255,255,0.07); border: 1px solid rgba(255,255,255,0.08); border-radius: 16px; overflow: hidden; }
.el-feat { position: relative; background: #000; padding: 40px 32px 52px; transition: background .25s; overflow: hidden; }
.el-feat::before {
  content: '';
  position: absolute; top: 0; left: 0; right: 0; height: 2px;
  background: var(--fc, #22d3ee);
  opacity: 0.6;
  transition: opacity .25s;
}
.el-feat:hover { background: #060606; }
.el-feat:hover::before { opacity: 1; }
.el-feat::after {
  content: '';
  position: absolute; top: 0; left: 0; right: 0; height: 120px;
  background: linear-gradient(180deg, var(--fc, #22d3ee) 0%, transparent 100%);
  opacity: 0;
  transition: opacity .3s;
  pointer-events: none;
}
.el-feat:hover::after { opacity: 0.04; }
.el-feat-num {
  font-size: 11px; font-weight: 700; letter-spacing: 0.2em;
  color: var(--fc, #22d3ee);
  opacity: 0.7;
  margin-bottom: 32px;
  font-variant-numeric: tabular-nums;
}
.el-feat-h { font-size: 1.15rem; font-weight: 700; letter-spacing: -0.01em; color: #fff; margin-bottom: 12px; }
.el-feat-p { font-size: 13.5px; line-height: 1.75; color: rgba(255,255,255,0.42); }
.el-feat-line {
  position: absolute; left: 32px; bottom: 0;
  width: 40px; height: 2px;
  opacity: 0.7;
  transition: width .35s ease;
}
.el-feat:hover .el-feat-line { width: calc(100% - 64px); }
/* ═══ TERMINAL ═══════════════════════════ */
.el-term-sec { max-width: 960px; margin: 0 auto; padding: 40px 32px 80px; }
.el-term {
  border: 1px solid rgba(255,255,255,0.1);
  border-radius: 14px;
  background: #030303;
  overflow: hidden;
  box-shadow: 0 0 60px rgba(34,211,238,0.06);
}
.el-term-hd {
  display: flex; align-items: center; justify-content: space-between;
  padding: 12px 20px;
  border-bottom: 1px solid rgba(255,255,255,0.06);
  background: rgba(255,255,255,0.02);
}
.el-term-title { font-size: 11px; font-family: ui-monospace, monospace; color: rgba(255,255,255,0.3); letter-spacing: 0.08em; }
.el-term-tabs { display: flex; gap: 4px; }
.el-term-tab {
  padding: 4px 14px;
  border: none; border-radius: 6px;
  background: transparent;
  font-size: 12px; font-weight: 600;
  font-family: ui-monospace, monospace;
  color: rgba(255,255,255,0.35);
  cursor: pointer;
  transition: color .15s, background .15s;
}
.el-term-tab:hover { color: rgba(255,255,255,0.7); }
.el-term-tab.active { background: rgba(34,211,238,0.12); color: #22d3ee; }
.el-term-live { display: flex; align-items: center; gap: 6px; font-size: 10px; font-weight: 700; letter-spacing: 0.18em; color: #22d3ee; }
.el-term-bd { padding: 22px 24px; font-family: ui-monospace, 'Fira Code', monospace; font-size: 13px; line-height: 2.1; }
.el-tl { display: flex; flex-wrap: wrap; opacity: 0; animation: el-tl-in .35s ease forwards; white-space: pre; }
@keyframes el-tl-in { from { opacity: 0; transform: translateY(3px); } to { opacity: 1; transform: none; } }
.tp { color: #22d3ee; font-weight: 700; }
.tc { color: #7dd3fc; }
.tf { color: #c4b5fd; }
.tu { color: #5eead4; }
.tk { color: #475569; }
.ts { color: #86efac; }
.tcm { color: #3f4a5a; font-style: italic; }
.tok { color: #22c55e; background: rgba(34,197,94,0.1); padding: 0 8px; border-radius: 4px; font-weight: 700; }
.tms { color: #475569; font-size: 11px; }
.tj { color: #fcd34d; }
.el-cursor {
  display: inline-block; width: 8px; height: 15px;
  background: #22d3ee; box-shadow: 0 0 8px #22d3ee;
  animation: el-cur 1s step-end infinite;
  margin-left: 6px; vertical-align: middle;
}
@keyframes el-cur { 0%,50%{opacity:1} 51%,100%{opacity:0} }

/* ═══ FINAL CTA ══════════════════════════ */
.el-final {
  text-align: center;
  padding: 100px 32px 120px;
  background: radial-gradient(ellipse 55% 70% at 50% 100%, rgba(34,211,238,0.09), transparent);
}
.el-final-h { font-size: clamp(2.2rem, 5vw, 3.4rem); font-weight: 800; letter-spacing: -0.035em; margin-bottom: 14px; }
.el-final-p { font-size: 1rem; color: rgba(255,255,255,0.45); margin-bottom: 40px; }

/* ═══ FOOTER ═════════════════════════════ */
.el-footer {
  border-top: 1px solid rgba(255,255,255,0.06);
  padding: 26px 32px;
  display: flex; flex-direction: column; gap: 16px;
  max-width: 1120px; margin: 0 auto; width: 100%;
}
.el-ft-disclaimer {
  font-size: 11px;
  line-height: 1.8;
  color: rgba(255,255,255,0.22);
  text-align: center;
  max-width: 760px;
  margin: 0 auto;
}
.el-ft-row {
  display: flex; align-items: center; justify-content: space-between;
  flex-wrap: wrap; gap: 12px;
}
.el-ft-c { font-size: 12px; color: rgba(255,255,255,0.25); }
.el-ft-r { display: flex; gap: 20px; }
.el-ft-a { font-size: 12px; color: rgba(255,255,255,0.3); text-decoration: none; transition: color .15s; }
.el-ft-a:hover { color: #fff; }

/* ═══ SCROLL REVEAL ══════════════════════ */
.rv { opacity: 0; transform: translateY(26px); transition: opacity .7s ease, transform .7s ease; }
.rv.on { opacity: 1; transform: none; }

/* ═══════════════════════════════════════════
   LIGHT THEME（浅色模式整套覆盖）
═══════════════════════════════════════════ */
.el-page.light { background: #f6f8fb; color: #0f172a; }

/* 背景层 */
.light .bg-glow {
  background:
    radial-gradient(ellipse 80% 60% at 50% -15%, rgba(34, 211, 238, 0.16), transparent 62%),
    radial-gradient(ellipse 50% 42% at 85% 15%, rgba(167, 139, 250, 0.12), transparent 65%),
    radial-gradient(ellipse 46% 40% at 10% 30%, rgba(244, 114, 182, 0.08), transparent 65%),
    radial-gradient(ellipse 60% 50% at 50% 110%, rgba(34, 211, 238, 0.08), transparent 60%);
}
.light .bg-aurora-1 {
  background: radial-gradient(circle, rgba(34,211,238,0.12) 0%, rgba(59,130,246,0.06) 45%, transparent 70%);
}
.light .bg-aurora-2 {
  background: radial-gradient(circle, rgba(167,139,250,0.10) 0%, rgba(244,114,182,0.05) 45%, transparent 70%);
}
.light .bg-grid {
  background-image: radial-gradient(circle, rgba(15,23,42,0.16) 1px, transparent 1px);
  opacity: 0.6;
}

/* 导航 */
.light .el-nav { background: rgba(246,248,251,0.8); border-bottom-color: rgba(15,23,42,0.08); }
.light .el-logo-txt { color: #0f172a; }
.light .el-nav-lnk { color: rgba(15,23,42,0.55); }
.light .el-nav-lnk:hover { color: #0f172a; }
.light .el-theme-btn { color: rgba(15,23,42,0.45); }
.light .el-theme-btn:hover { color: #0f172a; background: rgba(15,23,42,0.06); }

/* Hero */
.light .el-kicker { color: rgba(15,23,42,0.4); }
.light .h1-line1 {
  background: linear-gradient(180deg, #0f172a 45%, rgba(15,23,42,0.65));
  -webkit-background-clip: text; background-clip: text; -webkit-text-fill-color: transparent;
}
.light .h1-line2 { color: rgba(15,23,42,0.72); }
.light .el-sub { color: rgba(15,23,42,0.55); }

/* Base URL 组件 */
.light .el-baseurl-label { color: rgba(15,23,42,0.4); }
.light .el-baseurl-box {
  border-color: rgba(8,145,178,0.3);
  background: #fff;
  box-shadow: 0 4px 24px rgba(15,23,42,0.06);
}
.light .el-baseurl-box:hover { border-color: rgba(8,145,178,0.5); box-shadow: 0 4px 30px rgba(8,145,178,0.12); }
.light .el-baseurl-origin { color: #0f172a; }
.light .el-baseurl-path { color: #0e7490; }
.light .el-baseurl-copy { border-color: rgba(15,23,42,0.12); background: rgba(15,23,42,0.03); color: rgba(15,23,42,0.5); }
.light .el-baseurl-copy:hover { color: #0e7490; border-color: rgba(8,145,178,0.4); background: rgba(8,145,178,0.06); }

/* 按钮（浅色下主 CTA 反转为黑底白字） */
.light .el-btn-white { background: #0f172a; color: #fff; }
.light .el-btn-white:hover { background: #1e293b; box-shadow: 0 8px 30px rgba(15,23,42,0.25); }
.light .el-btn-dark { border-color: rgba(15,23,42,0.2); color: rgba(15,23,42,0.75); }
.light .el-btn-dark:hover { border-color: rgba(15,23,42,0.45); color: #0f172a; background: rgba(15,23,42,0.04); }

/* 波形标注 */
.light .el-wave-caption { color: rgba(15,23,42,0.35); }

/* 区块标题 */
.light .el-h2 { color: #0f172a; }

/* 流程图 */
.light .el-flow-wrap {
  border-color: rgba(15,23,42,0.08);
  background:
    radial-gradient(ellipse 50% 60% at 50% 50%, rgba(34,211,238,0.05), transparent),
    #fff;
}
.light .el-fpath { stroke: rgba(15,23,42,0.15); filter: none; }
.light .el-fpath-out { stroke: rgba(8,145,178,0.5); filter: none; }
.light .el-node-box { fill: #f8fafc; stroke: rgba(15,23,42,0.14); }
.light .el-node-app { stroke: rgba(8,145,178,0.5); fill: rgba(34,211,238,0.06); filter: none; }
.light .el-node-txt { fill: #0f172a; }
.light .el-node-sub { fill: #0e7490; }
.light .el-hub-core {
  fill: #fff;
  stroke: rgba(8,145,178,0.7);
  filter: drop-shadow(0 0 14px rgba(34,211,238,0.35));
}
.light .el-hub-pulse { stroke: rgba(8,145,178,0.35); }
.light .el-hub-txt { fill: #0f172a; }
.light .el-hub-sub { fill: #0e7490; }

/* Stats */
.light .el-stats { border-color: rgba(15,23,42,0.08); }
.light .el-stat-v { color: #0e7490; text-shadow: none; }
.light .el-stat-unit { color: rgba(15,23,42,0.45); }
.light .el-stat-l { color: rgba(15,23,42,0.4); }
.light .el-stat-sep { background: rgba(15,23,42,0.1); }

/* Features */
.light .el-feat-grid { background: rgba(15,23,42,0.08); border-color: rgba(15,23,42,0.08); }
.light .el-feat { background: #fff; }
.light .el-feat:hover { background: #f8fafc; }
.light .el-feat-h { color: #0f172a; }
.light .el-feat-p { color: rgba(15,23,42,0.5); }

/* Compat strip 亮色适配 */
.light .el-compat-chip { border-color: rgba(15,23,42,0.1); background: #fff; }
.light .el-compat-chip:hover { border-color: rgba(8,145,178,0.4); background: rgba(34,211,238,0.04); }
.light .el-compat-check { color: #0e7490; }
.light .el-compat-label { color: #0f172a; }
.light .el-compat-sub { color: rgba(15,23,42,0.45); }

/* FAQ 亮色适配 */
.light .el-faq-item { border-color: rgba(15,23,42,0.1); background: #fff; }
.light .el-faq-item.open { border-color: rgba(8,145,178,0.4); }
.light .el-faq-q { color: #0f172a; }
.light .el-faq-arrow { color: rgba(15,23,42,0.35); }
.light .el-faq-item.open .el-faq-arrow { color: #0e7490; }
.light .el-faq-a { color: rgba(15,23,42,0.55); }

/* Terminal tabs 亮色适配 */
.light .el-term-tab { color: rgba(15,23,42,0.4); }
.light .el-term-tab:hover { color: rgba(15,23,42,0.75); }
.light .el-term-tab.active { background: rgba(8,145,178,0.1); color: #0e7490; }

/* Terminal 亮色适配 */
.light .el-term {
  border-color: rgba(15,23,42,0.1);
  background: #fff;
  box-shadow: 0 8px 40px rgba(15,23,42,0.08);
}
.light .el-term-hd {
  border-bottom-color: rgba(15,23,42,0.06);
  background: rgba(15,23,42,0.02);
}
.light .el-term-title { color: rgba(15,23,42,0.35); }
.light .el-term-live { color: #0e7490; }
.light .tp { color: #0e7490; }
.light .tc { color: #0369a1; }
.light .tf { color: #7c3aed; }
.light .tu { color: #0f766e; }
.light .tk { color: rgba(15,23,42,0.4); }
.light .ts { color: #15803d; }
.light .tcm { color: rgba(15,23,42,0.38); }
.light .tok { color: #15803d; background: rgba(34,197,94,0.12); }
.light .tms { color: rgba(15,23,42,0.4); }
.light .tj { color: #b45309; }
.light .el-cursor { background: #0e7490; box-shadow: 0 0 6px rgba(14,116,144,0.5); }

/* Final CTA */
.light .el-final { background: radial-gradient(ellipse 55% 70% at 50% 100%, rgba(34,211,238,0.1), transparent); }
.light .el-final-h { color: #0f172a; }
.light .el-final-p { color: rgba(15,23,42,0.5); }

/* Footer */
.light .el-footer { border-top-color: rgba(15,23,42,0.08); }
.light .el-ft-disclaimer { color: rgba(15,23,42,0.35); }
.light .el-ft-c { color: rgba(15,23,42,0.35); }
.light .el-ft-a { color: rgba(15,23,42,0.4); }
.light .el-ft-a:hover { color: #0f172a; }

/* ═══ REDUCED MOTION ═════════════════════ */
@media (prefers-reduced-motion: reduce) {
  .bg-aurora, .el-hub-pulse, .el-live-dot, .el-cursor,
  .el-tl, .tline, .rv {
    animation: none !important;
    transition: none !important;
  }
  .el-tl, .tline { opacity: 1; }
  .rv { opacity: 1; transform: none; }
}

/* ═══ RESPONSIVE ═════════════════════════ */
@media (max-width: 768px) {
  .el-hero { padding: 70px 20px 30px; }
  .el-flow-sec, .el-feat-sec { padding-left: 20px; padding-right: 20px; }
  .el-term-sec { padding-left: 16px; padding-right: 16px; }
  .el-term-bd { font-size: 11px; padding: 16px; }
  .el-stat-sep { display: none; }
  .el-node-txt { font-size: 16px; }
}
</style>

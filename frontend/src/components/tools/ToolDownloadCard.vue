<template>
  <article class="tool-card" :aria-label="displayName">
    <header class="tool-heading">
      <div class="tool-mark" :class="{ 'tool-mark-official': logo }" aria-hidden="true">
        <img v-if="logo" :src="logo" alt="" width="52" height="52" decoding="async" />
        <Icon v-else name="terminal" size="xl" />
      </div>
      <div class="tool-identity">
        <div class="tool-name-row"><h3>{{ displayName }}</h3><span v-if="tool.version" class="tool-version">{{ tool.version }}</span></div>
        <span class="tool-category">{{ t(`toolDownloads.${tool.id === 'cc-switch' ? 'switchCategory' : 'codingCategory'}`) }}</span>
      </div>
      <a :href="tool.repository_url" target="_blank" rel="noopener noreferrer" class="project-link" :aria-label="`${displayName} · ${t('toolDownloads.project')}`" :title="t('toolDownloads.project')"><Icon name="externalLink" size="sm" /></a>
    </header>

    <p class="tool-description">{{ description }}</p>
    <div class="tool-tags"><span v-for="tag in tags" :key="tag">{{ tag }}</span></div>

    <div v-if="tool.available" class="tool-downloads">
      <div class="platform-tabs" role="group" :aria-label="`${displayName} · ${t('toolDownloads.chooseSystem')}`">
        <button v-for="platform in supportedPlatforms" :key="platform" type="button" :aria-pressed="selectedPlatform === platform" :class="{ selected: selectedPlatform === platform }" @click="selectedPlatform = platform">
          <svg v-if="platform === 'windows'" aria-hidden="true" viewBox="0 0 24 24" fill="currentColor"><path d="M2 4.8 10.5 3.6v8H2zm9.8-1.4L22 2v9.6H11.8zM2 12.9h8.5v8L2 19.7zm9.8 0H22v9.6L11.8 21z" /></svg>
          <svg v-else-if="platform === 'macos'" aria-hidden="true" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6"><rect x="3" y="3" width="18" height="14" rx="2" /><path d="M8 21h8m-4-4v4" /></svg>
          <Icon v-else name="terminal" size="sm" />
          {{ t(`toolDownloads.${platform}`) }}
        </button>
      </div>
      <div class="platform-detail">
        <span>{{ selectedPlatform === 'macos' ? t('toolDownloads.macosHint') : t('toolDownloads.architecture') }}</span>
        <label v-if="architectures.length > 1" class="architecture-select">
          <span class="sr-only">{{ displayName }} · {{ t('toolDownloads.architecture') }}</span>
          <select v-model="selectedArchitecture"><option v-for="architecture in architectures" :key="architecture" :value="architecture">{{ architectureLabel(architecture) }}</option></select>
        </label>
        <span v-else-if="selectedPlatform !== 'macos'" class="architecture-label">{{ architectureLabel(architectures[0] || '') }}</span>
        <span v-else class="universal-label">Universal</span>
      </div>
      <div class="download-options">
        <a v-for="(asset, index) in visibleAssets" :key="asset.url" :href="buildApiUrl(asset.url)" :download="asset.name" class="download-option" :class="{ 'download-primary': index === 0 }" :aria-label="`${t('toolDownloads.download')} ${asset.name}`">
          <Icon name="download" size="md" />
          <span class="download-copy"><strong>{{ assetLabel(asset) }}</strong><span>{{ asset.format.toUpperCase() }}<span class="size-separator">·</span>{{ formatSize(asset.size) }}</span></span>
          <Icon name="arrowDown" size="sm" class="download-arrow" />
        </a>
      </div>
      <p v-if="tool.stale" role="status" class="tool-notice"><Icon name="informationCircle" size="sm" />{{ t('toolDownloads.staleCompact') }}</p>
    </div>
    <div v-else class="tool-pending" role="status"><Icon :name="tool.syncing ? 'refresh' : 'clock'" size="md" :class="{ 'animate-spin motion-reduce:animate-none': tool.syncing }" /><strong>{{ t(tool.syncing ? 'toolDownloads.syncingTitle' : 'toolDownloads.pendingTitle') }}</strong><p>{{ t(tool.syncing ? 'toolDownloads.syncing' : 'toolDownloads.unavailable') }}</p></div>

    <footer class="tool-footer">
      <span v-if="tool.available" class="ready-indicator"><span aria-hidden="true"></span>{{ t('toolDownloads.hosted') }}</span>
      <span v-else class="pending-indicator">{{ t('toolDownloads.daily') }}</span>
      <a v-if="tool.available" :href="tool.release_url" target="_blank" rel="noopener noreferrer" class="release-link">{{ t('toolDownloads.release') }}<Icon name="externalLink" size="xs" /></a>
      <span v-if="tool.published_at" class="release-date">{{ shortDate(tool.published_at) }}</span>
    </footer>
  </article>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { buildApiUrl } from '@/api/client'
import type { DownloadableTool, ToolDownloadAsset, ToolPlatform } from '@/api/toolDownloads'

const props = defineProps<{ tool: DownloadableTool; preferredPlatform?: ToolPlatform | 'all' }>()
const { t, locale } = useI18n()
const logos: Record<string, string> = { codexplusplus: '/tools/logos/codexplusplus.webp', 'cc-switch': '/tools/logos/cc-switch.webp' }
const logo = computed(() => logos[props.tool.id])
const displayName = computed(() => props.tool.id === 'codexplusplus' ? 'Codex++' : props.tool.name)
const description = computed(() => props.tool.id === 'codexplusplus' ? t('toolDownloads.description') : props.tool.id === 'cc-switch' ? t('toolDownloads.switchDescription') : props.tool.description || props.tool.name)
const tags = computed(() => props.tool.id === 'codexplusplus' ? ['Codex', 'Claude Code'] : props.tool.id === 'cc-switch' ? ['Claude Code', 'Codex', 'Gemini CLI'] : [])
const supportedPlatforms = computed(() => (['windows', 'macos', 'linux'] as const).filter(platform => props.tool.assets.some(asset => asset.platform === platform)))
const selectedPlatform = ref<ToolPlatform>('windows')
const selectedArchitecture = ref('x64')
const nativePlatform = (): ToolPlatform => /Mac|iPhone|iPad/i.test(navigator.userAgent) ? 'macos' : /Linux/i.test(navigator.userAgent) && !/Android/i.test(navigator.userAgent) ? 'linux' : 'windows'
watch([() => supportedPlatforms.value.join(','), () => props.preferredPlatform], (_values, previous) => {
  const preferenceChanged = previous && props.preferredPlatform !== previous[1]
  const explicitPreference = props.preferredPlatform && props.preferredPlatform !== 'all' ? props.preferredPlatform : undefined
  if (explicitPreference && supportedPlatforms.value.includes(explicitPreference)) {
    selectedPlatform.value = explicitPreference
  } else if (preferenceChanged || !previous || !supportedPlatforms.value.includes(selectedPlatform.value)) {
    const preferred = nativePlatform()
    selectedPlatform.value = supportedPlatforms.value.includes(preferred) ? preferred : supportedPlatforms.value[0] || 'windows'
  }
}, { immediate: true })
const architectures = computed(() => [...new Set(props.tool.assets.filter(asset => asset.platform === selectedPlatform.value).map(asset => asset.architecture))].sort((a, b) => Number(b === 'x64') - Number(a === 'x64')))
watch(architectures, values => { if (!values.includes(selectedArchitecture.value)) selectedArchitecture.value = values[0] || 'x64' }, { immediate: true })
const rank = (asset: ToolDownloadAsset) => asset.format === 'zip' || asset.format === 'deb' ? 1 : 0
const visibleAssets = computed(() => props.tool.assets.filter(asset => asset.platform === selectedPlatform.value && asset.architecture === selectedArchitecture.value).sort((a, b) => rank(a) - rank(b)))
const architectureLabel = (value: string) => value === 'arm64' ? 'ARM64' : value === 'universal' ? 'Universal' : value === 'x64' ? 'x64' : value
const assetLabel = (asset: ToolDownloadAsset) => t(`toolDownloads.${asset.format === 'zip' ? 'portableDownload' : asset.format === 'appimage' ? 'appImageDownload' : asset.format === 'deb' ? 'debDownload' : 'installerDownload'}`)
const formatSize = (bytes: number) => `${(bytes / 1024 / 1024).toFixed(1)} MB`
const shortDate = (value: string) => new Intl.DateTimeFormat(locale.value, { month: 'short', day: 'numeric' }).format(new Date(value))
</script>

<style scoped>
.tool-card { display:flex; flex-direction:column; min-width:0; min-height:540px; height:100%; padding:24px; border:1px solid rgb(226 232 240); border-radius:20px; background:rgb(255 255 255 / .96); box-shadow:0 3px 12px -7px rgb(15 23 42 / .15); transition:border-color .2s,box-shadow .2s; }
.tool-card:hover { border-color:rgb(148 163 184 / .7); box-shadow:0 10px 28px -18px rgb(15 23 42 / .25); }
.tool-heading { display:flex; align-items:center; gap:14px; }
.tool-mark { display:flex; flex-shrink:0; align-items:center; justify-content:center; width:52px; height:52px; border:1px solid rgb(153 246 228 / .6); border-radius:15px; color:#0f766e; background:linear-gradient(140deg,#f0fdfa,#ccfbf1); }
.tool-mark svg { width:29px; height:29px; }
.tool-mark.tool-mark-official { border:0; border-radius:0; background:transparent; }
.tool-mark img { display:block; width:52px; height:52px; object-fit:contain; }
.tool-identity { min-width:0; flex:1; }
.tool-name-row { display:flex; align-items:center; flex-wrap:wrap; gap:9px; }
.tool-name-row h3 { color:#0f172a; font-size:20px; font-weight:700; letter-spacing:-.5px; line-height:1.4; overflow-wrap:anywhere; }
.tool-version { padding:3px 7px; border:1px solid #e2e8f0; border-radius:6px; font-size:11px; font-weight:600; color:#64748b; font-variant-numeric:tabular-nums; }
.tool-category { display:block; margin-top:3px; color:#64748b; font-size:12px; }
.project-link { display:flex; align-items:center; justify-content:center; width:44px; height:44px; margin-right:-8px; color:#64748b; border-radius:10px; }
.project-link:hover { color:#0f766e; background:#f0fdfa; }
.tool-description { min-height:48px; margin-top:18px; color:#64748b; font-size:13px; line-height:24px; }
.tool-tags { display:flex; gap:6px; flex-wrap:wrap; margin:12px 0 20px; }
.tool-tags span { padding:3px 8px; color:#64748b; background:#f8fafc; border:1px solid #f1f5f9; border-radius:5px; font-size:11px; }
.tool-downloads { flex:1; display:flex; flex-direction:column; border-top:1px solid #f1f5f9; padding-top:18px; }
.platform-tabs { display:flex; gap:4px; padding:4px; border-radius:10px; background:#f1f5f9; }
.platform-tabs button { display:flex; flex:1; align-items:center; justify-content:center; gap:7px; min-height:44px; padding:4px 7px; border:1px solid transparent; border-radius:7px; font-size:12px; font-weight:500; color:#64748b; cursor:pointer; transition:color .15s,background .15s,box-shadow .15s; }
.platform-tabs button svg { width:15px; height:15px; flex-shrink:0; }
.platform-tabs button.selected { color:#0f172a; background:white; border-color:#e2e8f0; box-shadow:0 1px 3px rgb(15 23 42 / .05); }
.platform-detail { display:flex; align-items:center; justify-content:space-between; gap:8px; min-height:46px; color:#64748b; font-size:11px; }
.architecture-label,.universal-label { color:#64748b; font-size:11px; font-weight:500; }
.architecture-select select { min-height:44px; padding:4px 27px 4px 10px; border:1px solid #e2e8f0; border-radius:7px; background-color:#fff; color:#475569; font-size:11px; cursor:pointer; }
.download-options { display:flex; flex-direction:column; min-height:122px; gap:8px; }
.download-option { display:flex; align-items:center; gap:11px; min-height:57px; padding:10px 14px; border:1px solid #e2e8f0; border-radius:10px; background:white; color:#475569; transition:background .15s,border-color .15s; }
.download-option:hover { background:#f8fafc; border-color:#94a3b8; }
.download-option.download-primary { color:white; border-color:#0d9488; background:linear-gradient(115deg,#0f9488,#0b827c); box-shadow:0 3px 8px -5px rgb(13 148 136 / .6); }
.download-option.download-primary:hover { border-color:#0f766e; background:#0f766e; }
.download-copy { display:flex; flex:1; min-width:0; align-items:center; justify-content:space-between; gap:10px; }
.download-copy strong { font-size:13px; font-weight:600; }
.download-copy>span { font-size:11px; opacity:.85; white-space:nowrap; }
.size-separator { margin:0 6px; }
.download-arrow { opacity:.65; }
.tool-notice { display:flex; gap:6px; align-items:center; margin-top:12px; color:#a16207; font-size:11px; line-height:1.6; }
.tool-notice svg { flex-shrink:0; }
.tool-pending { flex:1; display:flex; min-height:170px; align-items:center; justify-content:center; flex-direction:column; gap:10px; padding:20px; border:1px dashed #cbd5e1; border-radius:12px; color:#64748b; background:#f8fafc; text-align:center; }
.tool-pending strong { font-size:13px; font-weight:600; }
.tool-pending p { max-width:260px; font-size:12px; line-height:1.8; }
.tool-footer { display:flex; flex-shrink:0; align-items:center; gap:12px; min-height:44px; margin-top:17px; padding-top:14px; border-top:1px solid #f1f5f9; font-size:11px; color:#64748b; }
.ready-indicator { display:flex; align-items:center; gap:6px; margin-right:auto; color:#0f766e; }
.ready-indicator>span { width:5px; height:5px; background:#10b981; border-radius:50%; }
.pending-indicator { margin-right:auto; }
.release-link { display:flex; align-items:center; gap:4px; min-height:44px; color:#64748b; }
.release-link:hover { color:#0f766e; }
.release-date { white-space:nowrap; }
.tool-card :is(a,button,select):focus-visible { outline:2px solid #14b8a6; outline-offset:3px; }
:global(.dark .tool-card) { background:#111c2d; border-color:#2a3648; box-shadow:0 4px 14px -10px rgb(0 0 0 / .5); }
:global(.dark .tool-card:hover) { border-color:#475569; }
:global(.dark .tool-card .tool-mark) { background:linear-gradient(140deg,#113632,#12483f); color:#5eead4; border-color:#175b50; }
:global(.dark .tool-card .tool-mark-official) { background:transparent; border:0; }
:global(.dark .tool-card .tool-name-row h3) { color:#f1f5f9; }
:global(.dark .tool-card .tool-version) { color:#94a3b8; border-color:#334155; }
:global(.dark .tool-card :is(.tool-category,.tool-description,.project-link,.platform-detail,.architecture-label,.universal-label,.release-link)) { color:#94a3b8; }
:global(.dark .tool-card .tool-tags span) { color:#a9b7ca; background:#19263a; border-color:#243248; }
:global(.dark .tool-card :is(.tool-downloads,.tool-footer)) { border-color:#253246; }
:global(.dark .tool-card .platform-tabs) { background:#0b1423; }
:global(.dark .tool-card .platform-tabs button) { color:#94a3b8; }
:global(.dark .tool-card .platform-tabs button.selected) { color:#e2e8f0; background:#253247; border-color:#344259; }
:global(.dark .tool-card .architecture-select select) { color:#cbd5e1; background-color:#172538; border-color:#334155; }
:global(.dark .tool-card .download-option:not(.download-primary)) { color:#cbd5e1; background:#172538; border-color:#334155; }
:global(.dark .tool-card .download-option:not(.download-primary):hover) { background:#23344c; }
:global(.dark .tool-card .tool-notice) { color:#fcd34d; }
:global(.dark .tool-card .tool-pending) { background:#101b2c; color:#94a3b8; border-color:#334155; }
:global(.dark .tool-card .ready-indicator) { color:#5eead4; }
:global(.dark .tool-card .project-link:hover) { color:#5eead4; background:#123a36; }
@media(max-width:480px) { .tool-card { padding:20px; min-height:560px; border-radius:16px; } .tool-heading { gap:11px; } .tool-description { min-height:72px; } .download-copy { align-items:flex-start; flex-direction:column; gap:2px; } .platform-tabs button { min-height:44px; } .release-date { display:none; } }
@media(prefers-reduced-motion:reduce) { .tool-card,.tool-card * { transition:none!important; animation:none!important; } }
</style>

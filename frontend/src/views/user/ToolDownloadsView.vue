<template>
  <AppLayout>
    <section class="tool-library" :aria-busy="loading">
      <header class="library-header">
        <div class="header-copy"><div class="library-eyebrow"><span></span>{{ t('toolDownloads.collection') }}</div><h1>{{ t('toolDownloads.title') }}<span class="title-dot">.</span></h1><p>{{ t('toolDownloads.subtitle') }}</p></div>
      </header>

      <div class="library-toolbar">
        <label class="tool-search"><Icon name="search" size="md" /><span class="sr-only">{{ t('toolDownloads.search') }}</span><input v-model="search" type="search" maxlength="200" :placeholder="t('toolDownloads.search')" /></label>
        <div class="filter-group" role="group" :aria-label="t('toolDownloads.chooseSystem')"><button v-for="platform in filters" :key="platform" type="button" :aria-pressed="selectedPlatform === platform" :class="{ active: selectedPlatform === platform }" @click="selectedPlatform = platform">{{ t(`toolDownloads.${platform === 'all' ? 'allPlatforms' : platform}`) }}</button></div>
        <button class="refresh-button" type="button" :disabled="loading" :aria-label="t('toolDownloads.refresh')" :title="t('toolDownloads.refresh')" @click="load"><Icon name="refresh" size="sm" :class="{ 'animate-spin motion-reduce:animate-none': loading }" /><span>{{ t('toolDownloads.refresh') }}</span></button>
      </div>

      <p v-if="error" role="alert" class="library-error">{{ t('toolDownloads.failed') }}</p>
      <div class="collection-heading"><h2>{{ t('toolDownloads.library') }}<span>{{ filteredTools.length.toString().padStart(2, '0') }}</span></h2><span class="daily-note"><Icon name="clock" size="sm" />{{ t('toolDownloads.daily') }}</span></div>
      <div v-if="loading && !tools.length" role="status" class="tool-grid skeleton-grid" :aria-label="t('toolDownloads.loading')"><div v-for="i in 2" :key="i" class="tool-skeleton"><div></div><span></span><span></span><p></p><p></p></div></div>
      <div v-else-if="filteredTools.length" class="tool-grid"><ToolDownloadCard v-for="tool in filteredTools" :key="tool.id" :tool="tool" :preferred-platform="selectedPlatform" /></div>
      <div v-else-if="!error" class="library-empty" role="status"><span><Icon name="search" size="lg" /></span><h3>{{ t(search || selectedPlatform !== 'all' ? 'toolDownloads.noResults' : 'toolDownloads.noTools') }}</h3><p>{{ t('toolDownloads.noResultsHint') }}</p><button v-if="search || selectedPlatform !== 'all'" type="button" @click="clearFilters">{{ t('toolDownloads.clearFilters') }}</button></div>

      <footer class="library-footer"><span><Icon name="shield" size="sm" />{{ t('toolDownloads.originalPackages') }}</span><span><Icon name="download" size="sm" />{{ t('toolDownloads.hosted') }}</span><span><Icon name="refresh" size="sm" />{{ t('toolDownloads.automaticUpdates') }}</span></footer>
    </section>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import ToolDownloadCard from '@/components/tools/ToolDownloadCard.vue'
import { getToolDownloads, type DownloadableTool, type ToolPlatform } from '@/api/toolDownloads'

const { t } = useI18n()
const tools = ref<DownloadableTool[]>([])
const loading = ref(false)
const error = ref(false)
const search = ref('')
const filters = ['all', 'windows', 'macos', 'linux'] as const
const selectedPlatform = ref<ToolPlatform | 'all'>('all')
const filteredTools = computed(() => {
  const query = search.value.trim().toLowerCase()
  return tools.value.filter(tool => {
    const matchesPlatform = selectedPlatform.value === 'all' || tool.assets.some(asset => asset.platform === selectedPlatform.value)
    const description = tool.id === 'codexplusplus' ? t('toolDownloads.description') : tool.id === 'cc-switch' ? t('toolDownloads.switchDescription') : tool.description || ''
    const keywords = [tool.name, tool.id === 'codexplusplus' ? 'Codex++' : '', description, tool.repository_url].join(' ').toLowerCase()
    return matchesPlatform && (!query || keywords.includes(query))
  })
})
const clearFilters = () => { search.value = ''; selectedPlatform.value = 'all' }
let controller: AbortController | undefined
let syncPoll: ReturnType<typeof setTimeout> | undefined
async function load() {
  if (loading.value) return
  clearTimeout(syncPoll)
  controller = new AbortController()
  loading.value = true
  error.value = false
  try {
    tools.value = (await getToolDownloads(controller.signal)).tools
    // Only poll while files are actively syncing; ready catalogs are static.
    if (tools.value.some(tool => tool.syncing)) syncPoll = setTimeout(load, 5000)
  } catch {
    if (!controller.signal.aborted) error.value = true
  } finally {
    loading.value = false
  }
}
onMounted(load)
onUnmounted(() => { controller?.abort(); clearTimeout(syncPoll) })
</script>

<style scoped>
.tool-library { width:100%; max-width:1280px; margin:0 auto; padding:3px 0 20px; }
.library-header { display:flex; align-items:center; justify-content:space-between; gap:30px; padding:12px 0 30px; }
.library-eyebrow { display:flex; align-items:center; gap:7px; color:#0f766e; font-size:11px; letter-spacing:2px; font-weight:600; }
.library-eyebrow>span { display:block; width:16px; height:2px; background:#0d9488; }
.header-copy h1 { margin:10px 0 7px; color:#0f172a; font-size:32px; font-weight:700; letter-spacing:-1px; line-height:1.3; }
.title-dot { color:#0d9488; margin-left:2px; }
.header-copy>p { color:#64748b; font-size:13px; line-height:1.8; }
.library-toolbar { display:flex; align-items:center; gap:16px; padding:10px 12px; border:1px solid #e2e8f0; border-radius:13px; background:rgb(255 255 255 / .85); box-shadow:0 2px 5px -3px rgb(15 23 42 / .08); }
.tool-search { display:flex; align-items:center; gap:10px; flex:1; min-width:120px; padding-left:5px; color:#94a3b8; }
.tool-search input { width:100%; min-width:0; height:40px; padding:0; border:0; outline:none; background:transparent; box-shadow:none; font-size:12px; color:#334155; }
.tool-search input::placeholder { color:#94a3b8; }
.tool-search:focus-within { color:#0d9488; }
.filter-group { display:flex; flex-shrink:0; gap:3px; padding:3px; border-radius:8px; background:#f1f5f9; }
.filter-group button { min-height:44px; padding:5px 14px; border-radius:6px; font-size:11px; font-weight:500; color:#64748b; cursor:pointer; transition:background .15s,color .15s,box-shadow .15s; }
.filter-group button.active { color:#0f766e; background:white; box-shadow:0 1px 3px rgb(15 23 42 / .08); }
.refresh-button { display:flex; flex-shrink:0; align-items:center; justify-content:center; gap:6px; min-width:44px; height:44px; padding:0 10px; border-left:1px solid #e2e8f0; color:#64748b; font-size:11px; cursor:pointer; }
.refresh-button:disabled { opacity:.5; cursor:wait; }
.refresh-button:hover { color:#0f766e; }
.collection-heading { display:flex; align-items:center; justify-content:space-between; gap:12px; margin:29px 0 15px; }
.collection-heading h2 { display:flex; align-items:center; gap:9px; font-size:14px; font-weight:600; color:#334155; }
.collection-heading h2>span { display:flex; align-items:center; justify-content:center; min-width:24px; height:21px; padding:0 5px; border:1px solid #e2e8f0; border-radius:5px; background:rgb(255 255 255 / .5); color:#64748b; font-size:10px; font-weight:500; font-variant-numeric:tabular-nums; }
.daily-note { display:flex; align-items:center; gap:5px; font-size:11px; color:#64748b; }
.tool-grid { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); grid-auto-rows:1fr; align-items:stretch; gap:20px; }
.library-footer { display:flex; align-items:center; justify-content:center; flex-wrap:wrap; gap:12px 28px; margin-top:27px; color:#64748b; font-size:11px; }
.library-footer>span { display:flex; align-items:center; gap:6px; }
.library-error { margin-top:14px; padding:12px 16px; border:1px solid #fecaca; border-radius:10px; background:#fef2f2; color:#b91c1c; font-size:13px; }
.library-empty { display:flex; align-items:center; flex-direction:column; gap:10px; min-height:290px; padding:48px 20px; border:1px dashed #cbd5e1; border-radius:16px; text-align:center; background:rgb(255 255 255 / .6); color:#64748b; }
.library-empty>span { display:flex; align-items:center; justify-content:center; width:50px; height:50px; border-radius:14px; background:#f1f5f9; color:#94a3b8; }
.library-empty h3 { font-size:15px; font-weight:600; color:#334155; }
.library-empty p { font-size:12px; }
.library-empty button { min-height:44px; margin-top:4px; padding:8px 18px; border:1px solid #99f6e4; border-radius:8px; background:#f0fdfa; color:#0f766e; font-size:12px; cursor:pointer; }
.tool-skeleton { min-height:350px; padding:24px; border:1px solid #e2e8f0; border-radius:20px; background:rgb(255 255 255 / .8); }
.tool-skeleton div,.tool-skeleton span,.tool-skeleton p { display:block; border-radius:8px; background:#f1f5f9; }
.tool-skeleton div { width:52px; height:52px; margin-bottom:22px; }.tool-skeleton span { height:13px; margin:14px 0; }.tool-skeleton span:nth-child(3) { width:70%; }.tool-skeleton p { height:55px; margin-top:25px; }
.tool-library :is(button,input):focus-visible { outline:2px solid #14b8a6; outline-offset:3px; }
:global(.dark .tool-library .header-copy h1) { color:#f1f5f9; }
:global(.dark .tool-library .library-eyebrow) { color:#5eead4; }
:global(.dark .tool-library :is(.header-copy>p,.daily-note,.library-footer)) { color:#94a3b8; }
:global(.dark .tool-library .library-toolbar) { border-color:#2a3648; background:rgb(17 28 45 / .9); }
:global(.dark .tool-library .tool-search input) { color:#e2e8f0; }
:global(.dark .tool-library .filter-group) { background:#0b1423; }
:global(.dark .tool-library .filter-group button) { color:#94a3b8; }
:global(.dark .tool-library .filter-group button.active) { color:#5eead4; background:#233348; }
:global(.dark .tool-library .refresh-button) { color:#94a3b8; border-color:#334155; }
:global(.dark .tool-library .collection-heading h2) { color:#cbd5e1; }
:global(.dark .tool-library .collection-heading h2>span) { color:#94a3b8; border-color:#334155; background:#152236; }
:global(.dark .tool-library .library-error) { border-color:#7f1d1d; background:#2f1820; color:#fca5a5; }
:global(.dark .tool-library :is(.library-empty,.tool-skeleton)) { background:#111c2d; border-color:#334155; color:#94a3b8; }
:global(.dark .tool-library .library-empty h3) { color:#cbd5e1; }
:global(.dark .tool-library :is(.library-empty>span,.tool-skeleton div,.tool-skeleton span,.tool-skeleton p)) { background:#233348; }
@media(min-width:1700px) { .tool-grid { grid-template-columns:repeat(3,minmax(0,1fr)); } }
@media(max-width:1100px) { .refresh-button span { display:none; } .filter-group button { padding:5px 10px; } }
@media(max-width:760px) { .tool-grid { grid-template-columns:minmax(0,1fr); gap:16px; }.library-header { gap:14px; padding:5px 0 22px; }.header-copy h1 { font-size:28px; }.library-toolbar { flex-wrap:wrap; gap:9px; padding:10px; }.tool-search { flex-basis:calc(100% - 52px); }.refresh-button { order:0; border:0; padding:0; }.filter-group { order:1; width:100%; }.filter-group button { flex:1; min-height:44px; padding:4px 6px; }.collection-heading { margin-top:23px; }.library-footer { gap:10px 18px; font-size:10px; } }
@media(prefers-reduced-motion:reduce) { .tool-library,.tool-library * { transition:none!important; animation:none!important; } }
</style>

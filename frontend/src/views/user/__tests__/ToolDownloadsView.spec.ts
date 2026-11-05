import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ToolDownloadsView from '../ToolDownloadsView.vue'
import { getToolDownloads, type DownloadableTool } from '@/api/toolDownloads'

vi.mock('@/api/toolDownloads', () => ({ getToolDownloads: vi.fn() }))
vi.mock('vue-i18n', async importOriginal => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key, locale: { value: 'en' } }),
}))
vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<main><slot /></main>' } }))

const fixture: DownloadableTool = {
  id: 'codexplusplus', name: 'CodexPlusPlus', version: 'v2.3.0', available: true, stale: false,
  repository_url: 'https://github.com/BigPizzaV3/CodexPlusPlus',
  release_url: 'https://github.com/BigPizzaV3/CodexPlusPlus/releases/tag/v2.3.0', refresh_interval_seconds: 86400,
  assets: [
    { name: 'windows-setup.exe', platform: 'windows', architecture: 'x64', format: 'exe', size: 29014931, url: '/api/v1/tool-downloads/files/release-id/windows-setup.exe' },
    { name: 'macos.dmg', platform: 'macos', architecture: 'universal', format: 'dmg', size: 98836022, url: '/api/v1/tool-downloads/files/release-id/macos.dmg' },
    { name: 'portable.zip', platform: 'windows', architecture: 'x64', format: 'zip', size: 40000, url: '/api/v1/tool-downloads/files/release-id/portable.zip' },
  ],
}
const ccSwitch: DownloadableTool = {
  ...fixture, id: 'cc-switch', name: 'CC Switch', version: 'v4.0.25',
  repository_url: 'https://github.com/farion1231/cc-switch',
  assets: [
    { name: 'cc-x64.msi', platform: 'windows', architecture: 'x64', format: 'msi', size: 1000, url: '/api/v1/tool-downloads/files/cc-id/cc-x64.msi' },
    { name: 'cc-arm64.msi', platform: 'windows', architecture: 'arm64', format: 'msi', size: 1000, url: '/api/v1/tool-downloads/files/cc-id/cc-arm64.msi' },
    { name: 'cc-linux.deb', platform: 'linux', architecture: 'x64', format: 'deb', size: 1000, url: '/api/v1/tool-downloads/files/cc-id/cc-linux.deb' },
  ],
}
const render = () => mount(ToolDownloadsView, { global: { stubs: { Icon: true } } })
const refresh = 'button[aria-label="toolDownloads.refresh"]'

describe('ToolDownloadsView', () => {
  beforeEach(() => { vi.resetAllMocks() })
  it('offers same-site downloads and switches between operating systems', async () => {
    vi.mocked(getToolDownloads).mockResolvedValue({ tools: [fixture] })
    const wrapper = render()
    await flushPromises()
    const installer = wrapper.get('a[download="windows-setup.exe"]')
    expect(installer.attributes('href')).toBe(fixture.assets[0]!.url)
    expect(installer.attributes('target')).toBeUndefined()
    expect(wrapper.find('a[download="macos.dmg"]').exists()).toBe(false)
    await wrapper.get('.platform-tabs button:nth-child(2)').trigger('click')
    expect(wrapper.get('a[download="macos.dmg"]').attributes('href')).toBe(fixture.assets[1]!.url)
    expect(wrapper.find('a[download="windows-setup.exe"]').exists()).toBe(false)
    wrapper.unmount()
  })
  it('retains hosted downloads when update sync fails', async () => {
    vi.mocked(getToolDownloads).mockResolvedValue({ tools: [{ ...fixture, stale: true }] })
    const wrapper = render()
    await flushPromises()
    expect(wrapper.text()).toContain('toolDownloads.staleCompact')
    expect(wrapper.findAll('a[download]')).toHaveLength(2)
    wrapper.unmount()
  })
  it('keeps the chosen system when refreshed metadata has the same installers', async () => {
    vi.mocked(getToolDownloads).mockResolvedValue({ tools: [fixture] })
    const wrapper = render()
    await flushPromises()
    await wrapper.get('.platform-tabs button:nth-child(2)').trigger('click')
    vi.mocked(getToolDownloads).mockResolvedValue({ tools: [{ ...fixture, assets: fixture.assets.map(asset => ({ ...asset })) }] })
    await wrapper.get(refresh).trigger('click')
    await flushPromises()
    expect(wrapper.get('a[download]').attributes('download')).toBe('macos.dmg')
    wrapper.unmount()
  })
  it('allows retry when the API is unavailable without sending users to GitHub', async () => {
    vi.mocked(getToolDownloads).mockRejectedValue(new Error('offline'))
    const wrapper = render()
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('toolDownloads.failed')
    expect(wrapper.findAll('a')).toHaveLength(0)
    vi.mocked(getToolDownloads).mockResolvedValue({ tools: [fixture] })
    await wrapper.get(refresh).trigger('click')
    await flushPromises()
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('v2.3.0')
    wrapper.unmount()
  })
  it('keeps ready tools usable while another tool is syncing', async () => {
    vi.mocked(getToolDownloads).mockResolvedValue({ tools: [fixture, { ...ccSwitch, available: false, syncing: true, assets: [] }] })
    const wrapper = render()
    await flushPromises()
    expect(wrapper.findAll('article')).toHaveLength(2)
    expect(wrapper.get('article[aria-label="CC Switch"]').text()).toContain('toolDownloads.syncing')
    expect(wrapper.get('article[aria-label="CC Switch"]').findAll('a[download]')).toHaveLength(0)
    expect(wrapper.get('article[aria-label="Codex++"]').findAll('a[download]')).toHaveLength(2)
    wrapper.unmount()
  })
  it('searches multiple tools and clears an empty result', async () => {
    vi.mocked(getToolDownloads).mockResolvedValue({ tools: [fixture, ccSwitch] })
    const wrapper = render()
    await flushPromises()
    const input = wrapper.get('input[type="search"]')
    await input.setValue('CC Switch')
    expect(wrapper.findAll('article')).toHaveLength(1)
    expect(wrapper.get('article').attributes('aria-label')).toBe('CC Switch')
    await input.setValue('nonexistent')
    expect(wrapper.findAll('article')).toHaveLength(0)
    expect(wrapper.text()).toContain('toolDownloads.noResults')
    await wrapper.get('.library-empty button').trigger('click')
    expect(wrapper.findAll('article')).toHaveLength(2)
    await input.setValue('Codex++')
    expect(wrapper.get('article').attributes('aria-label')).toBe('Codex++')
    wrapper.unmount()
  })
  it('filters tools by OS and selects ARM64 installers independently per card', async () => {
    vi.mocked(getToolDownloads).mockResolvedValue({ tools: [fixture, ccSwitch] })
    const wrapper = render()
    await flushPromises()
    await wrapper.get('.filter-group button:nth-child(4)').trigger('click')
    expect(wrapper.findAll('article')).toHaveLength(1)
    expect(wrapper.get('a[download="cc-linux.deb"]').attributes('href')).toContain('/cc-id/')
    await wrapper.get('.filter-group button:nth-child(2)').trigger('click')
    expect(wrapper.findAll('article')).toHaveLength(2)
    const card = wrapper.get('article[aria-label="CC Switch"]')
    expect(card.get('a[download]').attributes('download')).toBe('cc-x64.msi')
    await card.get('select').setValue('arm64')
    expect(card.get('a[download]').attributes('download')).toBe('cc-arm64.msi')
    expect(wrapper.get('article[aria-label="Codex++"] a[download]').attributes('download')).toBe('windows-setup.exe')
    wrapper.unmount()
  })
})

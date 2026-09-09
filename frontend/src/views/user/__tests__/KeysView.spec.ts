import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { nextTick } from 'vue'

import type { ApiKey } from '@/types'
import KeysView from '../KeysView.vue'

const {
  listKeys,
  listSmartGroupLogs,
  createKey,
  updateKey,
  getPublicSettings,
  getDashboardApiKeysUsage,
  getAvailableGroups,
  getUserGroupRates,
  showError,
  showSuccess,
  copyToClipboard,
  isCurrentStep,
  nextStep,
} = vi.hoisted(() => ({
  listKeys: vi.fn(),
  listSmartGroupLogs: vi.fn(),
  createKey: vi.fn(),
  updateKey: vi.fn(),
  getPublicSettings: vi.fn(),
  getDashboardApiKeysUsage: vi.fn(),
  getAvailableGroups: vi.fn(),
  getUserGroupRates: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
  copyToClipboard: vi.fn(),
  isCurrentStep: vi.fn(),
  nextStep: vi.fn(),
}))

const messages: Record<string, string> = {
  'common.actions': 'Actions',
  'common.name': 'Name',
  'common.refresh': 'Refresh',
  'common.status': 'Status',
  'keys.apiKey': 'API Key',
  'keys.allGroups': 'All Groups',
  'keys.allStatus': 'All Status',
  'keys.columnSettings': 'Column Settings',
  'keys.createKey': 'Create API Key',
  'keys.created': 'Created',
  'keys.expiresAt': 'Expires',
  'keys.group': 'Group',
  'keys.smartGroupColumn': 'Routing group',
	'keys.routingModeLabel': 'Routing mode',
	'keys.fixedRouting': 'Fixed group',
	'keys.fixedGroupBadge': 'Fixed',
	'keys.smartRouting': 'Smart groups',
	'keys.fixedGroupDescription': 'Always use one group.',
	'keys.smartGroupDescription': 'Automatically fail over.',
	'keys.platformLabel': 'Platform',
	'keys.selectPlatform': 'Select a platform',
	'keys.selectPlatformFirst': 'Select a platform first',
	'keys.platformGroupCount': '{count} groups',
	'keys.smartGroupCandidates': 'Candidate groups',
	'keys.smartGroupCandidatesHint': 'Sorted by rate.',
	'keys.smartGroupFailureThreshold': 'Consecutive failures',
	'keys.smartGroupFailureThresholdHint': 'Failure threshold.',
	'keys.smartGroupRecoveryMinutes': 'Stable check interval',
	'keys.smartGroupRecoveryMinutesHint': 'Recovery interval.',
	'keys.smartGroupSettings': 'Smart group settings',
	'keys.smartGroupBadge': 'Smart {count}',
	'keys.smartGroupRuntimeHint': 'Smart runtime',
	'keys.smartGroupPrimaryLabel': 'Preferred route',
	'keys.smartGroupPrimaryHint': 'No. 1 is sorted by effective multiplier.',
	'keys.smartGroupSwitchLogs': 'Smart switch logs',
	'keys.smartGroupSwitchLogsShort': 'Switch logs',
	'keys.smartGroupSwitchLogsDescription': 'Only recent logs.',
	'keys.smartGroupNoSwitchLogs': 'No smart switches.',
	'keys.smartGroupSwitchLogsLoadFailed': 'Failed to load logs',
	'keys.smartGroupSwitchReasonFailure': 'Failure failover',
	'keys.smartGroupSwitchReasonCostRecovery': 'Cost recovery',
	'keys.keyUpdatedSuccess': 'API key updated successfully',
	'keys.clickToManageSmartGroup': 'Manage smart groups',
	'keys.clickToChangeGroup': 'Change group',
	'keys.fixedGroupLabel': 'Fixed group',
	'keys.selectGroup': 'Select a group',
	'keys.searchGroup': 'Search groups',
  'keys.id': 'ID',
  'keys.currentConcurrency': 'Current Concurrency',
  'keys.lastUsedAt': 'Last Used',
  'keys.lastUsedIP': 'Last Used IP',
  'keys.rateLimitColumn': 'Rate Limit',
  'keys.searchPlaceholder': 'Search name or key...',
  'keys.status.active': 'Active',
  'keys.status.expired': 'Expired',
  'keys.status.inactive': 'Inactive',
  'keys.status.quota_exhausted': 'Quota exhausted',
  'keys.usage': 'Usage',
}

vi.mock('@/api', () => ({
  keysAPI: {
    list: listKeys,
    listSmartGroupLogs,
    create: createKey,
    update: updateKey,
    delete: vi.fn(),
    toggleStatus: vi.fn(),
  },
  authAPI: {
    getPublicSettings,
  },
  usageAPI: {
    getDashboardApiKeysUsage,
  },
  userGroupsAPI: {
    getAvailable: getAvailableGroups,
    getUserGroupRates,
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess,
  }),
}))

vi.mock('@/stores/onboarding', () => ({
  useOnboardingStore: () => ({
    isCurrentStep,
    nextStep,
  }),
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copyToClipboard,
  }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => {
        let message = messages[key] ?? key
        for (const [name, value] of Object.entries(params ?? {})) {
          message = message.replace(`{${name}}`, String(value))
        }
        return message
      },
    }),
  }
})

const createApiKey = (): ApiKey => ({
  id: 1,
  user_id: 1,
  key: 'sk-test-key',
  name: 'test-key',
	  group_id: null,
	  status: 'active',
	  smart_group_enabled: false,
	  smart_group_ids: [],
	  smart_group_failure_threshold: 3,
	  smart_group_recovery_interval_seconds: 900,
	  smart_group_consecutive_failures: 0,
	  smart_group_healthy_since: null,
	  smart_group_last_probe_at: null,
	  smart_group_last_switch_at: null,
	  smart_group_last_switch_reason: '',
	  smart_group_last_error: '',
  ip_whitelist: [],
  ip_blacklist: [],
  last_used_at: null,
  last_used_ip: null,
  quota: 0,
  quota_used: 0,
  expires_at: null,
  created_at: '2026-06-27T00:00:00Z',
  updated_at: '2026-06-27T00:00:00Z',
  current_concurrency: 3,
  rate_limit_5h: 0,
  rate_limit_1d: 0,
  rate_limit_7d: 0,
  usage_5h: 0,
  usage_1d: 0,
  usage_7d: 0,
  window_5h_start: null,
  window_1d_start: null,
  window_7d_start: null,
  reset_5h_at: null,
  reset_1d_at: null,
  reset_7d_at: null,
})

const AppLayoutStub = {
  template: '<div><slot /></div>',
}

const TablePageLayoutStub = {
  template: `
    <div>
      <slot name="filters" />
      <slot name="actions" />
      <slot name="table" />
      <slot name="pagination" />
    </div>
  `,
}

const DataTableStub = {
  name: 'DataTable',
  props: ['columns', 'data'],
  emits: ['sort'],
  template: `
    <div>
      <div data-test="columns">{{ columns.map((col) => col.key).join(',') }}</div>
      <div data-test="columns-meta">{{ JSON.stringify(columns.map((col) => ({ key: col.key, sortable: !!col.sortable }))) }}</div>
      <button data-test="sort-current-concurrency" @click="$emit('sort', 'current_concurrency', 'asc')">
        Sort Current Concurrency
      </button>
      <div v-for="row in data" :key="row.id">
        <div
          v-if="columns.some((col) => col.key === 'id')"
          data-test="key-id"
        >
          <slot name="cell-id" :value="row.id" :row="row" />
        </div>
        <slot name="cell-name" :value="row.name" :row="row" />
		<div data-test="group-cell">
		  <slot name="cell-group" :value="row.group" :row="row" />
		</div>
        <div data-test="current-concurrency">
          <slot name="cell-current_concurrency" :value="row.current_concurrency" :row="row" />
        </div>
        <slot name="cell-actions" :row="row" />
        <div
          v-if="columns.some((col) => col.key === 'last_used_ip')"
          data-test="last-used-ip"
        >
          <slot name="cell-last_used_ip" :value="row.last_used_ip" :row="row" />
        </div>
      </div>
      <slot name="empty" />
    </div>
  `,
}

const SelectStub = {
  name: 'Select',
  props: ['modelValue', 'options'],
  emits: ['update:modelValue'],
  template: '<select :value="modelValue" @change="$emit(\'update:modelValue\', $event.target.value)"></select>',
}

const SearchInputStub = {
  name: 'SearchInput',
  props: ['modelValue'],
  emits: ['update:modelValue', 'search'],
  template: '<input :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />',
}

const PaginationStub = {
  name: 'Pagination',
  props: ['page', 'total', 'pageSize'],
  emits: ['update:page', 'update:pageSize'],
  template: `
    <div>
      <button data-test="page-size-50" @click="$emit('update:pageSize', 50)">50</button>
    </div>
  `,
}

const IconStub = {
  props: ['name'],
  template: '<span data-test="icon">{{ name }}</span>',
}

const BaseDialogStub = {
	name: 'BaseDialog',
	props: ['show', 'title'],
	template: `
		<section v-if="show" data-test="base-dialog">
			<h2>{{ title }}</h2>
			<slot />
			<slot name="footer" />
		</section>
	`,
}

const mountView = async () => {
  const wrapper = mount(KeysView, {
    global: {
      stubs: {
        AppLayout: AppLayoutStub,
        TablePageLayout: TablePageLayoutStub,
        DataTable: DataTableStub,
        Pagination: PaginationStub,
		BaseDialog: BaseDialogStub,
        ConfirmDialog: true,
        EmptyState: true,
        Select: SelectStub,
        SearchInput: SearchInputStub,
        Icon: IconStub,
        UseKeyModal: true,
        EndpointPopover: true,
        GroupBadge: true,
        GroupOptionItem: true,
        Teleport: true,
      },
    },
  })
  await flushPromises()
  await nextTick()
  return wrapper
}

const visibleColumnKeys = (wrapper: VueWrapper) =>
  wrapper.get('[data-test="columns"]').text().split(',').filter(Boolean)

const visibleColumnMeta = (wrapper: VueWrapper): Array<{ key: string; sortable: boolean }> =>
  JSON.parse(wrapper.get('[data-test="columns-meta"]').text())

const getButtonByText = (wrapper: VueWrapper, text: string) => {
  const button = wrapper.findAll('button').find((item) => item.text().includes(text))
  if (!button) {
    throw new Error(`Button not found: ${text}`)
  }
  return button
}

describe('user KeysView column settings', () => {
  beforeEach(() => {
    localStorage.clear()

    listKeys.mockReset()
    listSmartGroupLogs.mockReset()
    createKey.mockReset()
    updateKey.mockReset()
    getPublicSettings.mockReset()
    getDashboardApiKeysUsage.mockReset()
    getAvailableGroups.mockReset()
    getUserGroupRates.mockReset()
    showError.mockReset()
    showSuccess.mockReset()
    copyToClipboard.mockReset()
    isCurrentStep.mockReset()
    nextStep.mockReset()

    listKeys.mockResolvedValue({
      items: [createApiKey()],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1,
    })
	createKey.mockResolvedValue(createApiKey())
	updateKey.mockResolvedValue(createApiKey())
    getPublicSettings.mockResolvedValue({})
    getDashboardApiKeysUsage.mockResolvedValue({ stats: {} })
    getAvailableGroups.mockResolvedValue([])
    getUserGroupRates.mockResolvedValue({})
    listSmartGroupLogs.mockResolvedValue([])
    isCurrentStep.mockReturnValue(false)
  })

  it('uses the default API key columns with low-frequency columns hidden', async () => {
    const wrapper = await mountView()

    expect(visibleColumnKeys(wrapper)).toEqual([
      'name',
      'key',
      'group',
      'current_concurrency',
      'usage',
      'expires_at',
      'status',
      'created_at',
      'actions',
    ])
    expect(visibleColumnKeys(wrapper)).not.toContain('rate_limit')
    expect(visibleColumnKeys(wrapper)).not.toContain('last_used_at')
    expect(visibleColumnKeys(wrapper)).not.toContain('last_used_ip')
    expect(visibleColumnKeys(wrapper)).not.toContain('id')
  })

  it('shows a hidden column when toggled and persists the preference', async () => {
    const wrapper = await mountView()

    await wrapper.get('button[title="Column Settings"]').trigger('click')
    await getButtonByText(wrapper, 'Rate Limit').trigger('click')
    await nextTick()

    expect(visibleColumnKeys(wrapper)).toContain('rate_limit')
    expect(localStorage.getItem('api-key-hidden-columns')).toBe(
      JSON.stringify(['id', 'last_used_at', 'last_used_ip'])
    )
    expect(localStorage.getItem('api-key-column-settings-version')).toBe('3')
  })

  it('shows the API key ID column when toggled', async () => {
    const wrapper = await mountView()

    await wrapper.get('button[title="Column Settings"]').trigger('click')
    await getButtonByText(wrapper, 'ID').trigger('click')
    await nextTick()

    expect(visibleColumnKeys(wrapper)).toContain('id')
    expect(wrapper.get('[data-test="key-id"]').text()).toBe('#1')
    expect(visibleColumnMeta(wrapper).find((column) => column.key === 'id')?.sortable).toBe(true)
  })

  it('shows the last used IP column when toggled', async () => {
    listKeys.mockResolvedValueOnce({
      items: [{ ...createApiKey(), last_used_ip: '203.0.113.10' }],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1,
    })
    const wrapper = await mountView()

    await wrapper.get('button[title="Column Settings"]').trigger('click')
    await getButtonByText(wrapper, 'Last Used IP').trigger('click')
    await nextTick()

    expect(visibleColumnKeys(wrapper)).toContain('last_used_ip')
    expect(wrapper.get('[data-test="last-used-ip"]').text()).toBe('203.0.113.10')
  })

  it('restores column preferences from localStorage on mount', async () => {
    localStorage.setItem('api-key-hidden-columns', JSON.stringify(['group', 'created_at']))
    localStorage.setItem('api-key-column-settings-version', '1')

    const wrapper = await mountView()

    expect(visibleColumnKeys(wrapper)).toEqual([
      'name',
      'key',
      'current_concurrency',
      'usage',
      'rate_limit',
      'expires_at',
      'status',
      'last_used_at',
      'actions',
    ])
    expect(localStorage.getItem('api-key-hidden-columns')).toBe(
      JSON.stringify(['group', 'created_at', 'last_used_ip', 'id'])
    )
    expect(localStorage.getItem('api-key-column-settings-version')).toBe('3')
  })

  it('does not include always-visible columns in the toggleable menu', async () => {
    const wrapper = await mountView()

    await wrapper.get('button[title="Column Settings"]').trigger('click')
    await nextTick()

    const columnMenuText = wrapper.text()
    expect(columnMenuText).toContain('API Key')
    expect(columnMenuText).toContain('ID')
    expect(columnMenuText).toContain('Current Concurrency')
    expect(columnMenuText).toContain('Rate Limit')
    expect(columnMenuText).toContain('Last Used IP')
    expect(columnMenuText).not.toContain('Name')
    expect(columnMenuText).not.toContain('Actions')
  })

  it('renders the current concurrency value', async () => {
    const wrapper = await mountView()

    expect(wrapper.get('[data-test="current-concurrency"]').text()).toBe('3')
  })

  it('marks current concurrency as sortable', async () => {
    const wrapper = await mountView()

    const currentConcurrencyColumn = visibleColumnMeta(wrapper).find(
      (column) => column.key === 'current_concurrency'
    )
    expect(currentConcurrencyColumn?.sortable).toBe(true)
  })

  it('keeps filters and selected page size when sorting by current concurrency', async () => {
    getAvailableGroups.mockResolvedValue([{ id: 42, name: 'OpenAI' }])
    const wrapper = await mountView()

    await wrapper.get('[data-test="page-size-50"]').trigger('click')
    await flushPromises()

    await wrapper.findComponent({ name: 'SearchInput' }).vm.$emit('update:modelValue', 'target')
    await wrapper.findComponent({ name: 'SearchInput' }).vm.$emit('search')
    await flushPromises()

    const selects = wrapper.findAllComponents({ name: 'Select' })
    await selects[0].vm.$emit('update:modelValue', 42)
    await flushPromises()
    await selects[1].vm.$emit('update:modelValue', 'active')
    await flushPromises()

    listKeys.mockClear()

    await wrapper.get('[data-test="sort-current-concurrency"]').trigger('click')
    await flushPromises()

    expect(listKeys).toHaveBeenLastCalledWith(
      1,
      50,
      {
        search: 'target',
        status: 'active',
        group_id: 42,
        sort_by: 'current_concurrency',
        sort_order: 'asc',
      },
      expect.objectContaining({ signal: expect.any(AbortSignal) })
    )
  })

	it('selects one platform first and creates smart routing from its lowest-rate group', async () => {
		getAvailableGroups.mockResolvedValue([
			{
				id: 11,
				name: 'OpenAI standard',
				description: null,
				platform: 'openai',
				rate_multiplier: 0.2,
				peak_rate_enabled: false,
				peak_start: '',
				peak_end: '',
				peak_rate_multiplier: 1,
				subscription_type: 'standard',
			},
			{
				id: 12,
				name: 'OpenAI economy',
				description: null,
				platform: 'openai',
				rate_multiplier: 0.1,
				peak_rate_enabled: false,
				peak_start: '',
				peak_end: '',
				peak_rate_multiplier: 1,
				subscription_type: 'standard',
			},
			{
				id: 21,
				name: 'Claude',
				description: null,
				platform: 'anthropic',
				rate_multiplier: 0.05,
				peak_rate_enabled: false,
				peak_start: '',
				peak_end: '',
				peak_rate_multiplier: 1,
				subscription_type: 'standard',
			},
		])
		const wrapper = await mountView()

		await wrapper.get('button[data-tour="keys-create-btn"]').trigger('click')
		await nextTick()
		const platformSelect = wrapper.findAllComponents({ name: 'Select' }).find(
			(component) => component.attributes('data-test') === 'key-platform-select'
		)
		expect(platformSelect).toBeTruthy()
		await platformSelect!.vm.$emit('update:modelValue', 'openai')
		await nextTick()

		const fixedGroupSelect = wrapper.findAllComponents({ name: 'Select' }).find(
			(component) => component.attributes('data-test') === 'key-group-select'
		)
		expect(fixedGroupSelect?.props('options').map((option: { value: number }) => option.value)).toEqual([11, 12])

		await wrapper.get('[data-test="routing-mode-smart"]').trigger('click')
		await nextTick()
		const candidates = wrapper.findAll('[data-test="smart-group-option"]')
		expect(candidates).toHaveLength(2)
		expect(candidates.map((candidate) => candidate.attributes('data-platform'))).toEqual(['openai', 'openai'])

		await wrapper.get('[data-test="smart-group-option"][data-group-id="11"]').trigger('click')
		await wrapper.get('[data-test="smart-group-option"][data-group-id="12"]').trigger('click')
		await wrapper.get('input[required]').setValue('smart-openai')
		await wrapper.get('form#key-form').trigger('submit')
		await flushPromises()

		expect(createKey).toHaveBeenCalledTimes(1)
		const createArgs = createKey.mock.calls[0]
		expect(createArgs[1]).toBe(12)
		expect(createArgs[8]).toEqual({
			smart_group_enabled: true,
			smart_group_ids: [12, 11],
			smart_group_failure_threshold: 3,
			smart_group_recovery_interval_seconds: 900,
		})
	})

	it('opens smart routing settings instead of the fixed-group dropdown', async () => {
		const openAIGroup = {
			id: 11,
			name: 'OpenAI standard',
			description: null,
			platform: 'openai',
			rate_multiplier: 0.2,
			peak_rate_enabled: false,
			peak_start: '',
			peak_end: '',
			peak_rate_multiplier: 1,
			subscription_type: 'standard',
		}
		getAvailableGroups.mockResolvedValue([
			openAIGroup,
			{ ...openAIGroup, id: 12, name: 'OpenAI economy', rate_multiplier: 0.1 },
		])
		listKeys.mockResolvedValue({
			items: [{
				...createApiKey(),
				group_id: 11,
				group: openAIGroup,
				smart_group_enabled: true,
				smart_group_ids: [12, 11],
			}],
			total: 1,
			page: 1,
			page_size: 20,
			pages: 1,
		})
		const wrapper = await mountView()

		await wrapper.get('[data-test="key-group-control"][data-routing-mode="smart"]').trigger('click')
		await nextTick()

		expect(wrapper.get('[data-test="base-dialog"] h2').text()).toBe('Smart group settings')
		expect(wrapper.find('[data-test="fixed-group-quick-option"]').exists()).toBe(false)
		expect(wrapper.find('input[required]').exists()).toBe(false)
		expect(wrapper.findAll('[data-test="smart-group-option"]')).toHaveLength(2)
		expect(wrapper.get('[data-test="smart-group-option"][data-group-id="11"]').attributes('disabled')).toBeUndefined()
		expect(wrapper.get('[data-test="smart-group-primary-label"]').text()).toContain('Preferred route')
	})

	it('updates the active route to the new cheapest candidate without probing', async () => {
		const openAIGroup = {
			id: 11,
			name: 'OpenAI standard',
			description: null,
			platform: 'openai',
			rate_multiplier: 0.2,
			peak_rate_enabled: false,
			peak_start: '',
			peak_end: '',
			peak_rate_multiplier: 1,
			subscription_type: 'standard',
		}
		getAvailableGroups.mockResolvedValue([
			openAIGroup,
			{ ...openAIGroup, id: 12, name: 'OpenAI economy', rate_multiplier: 0.1 },
			{ ...openAIGroup, id: 13, name: 'OpenAI premium', rate_multiplier: 0.3 },
		])
		listKeys.mockResolvedValue({
			items: [{
				...createApiKey(),
				group_id: 11,
				group: openAIGroup,
				smart_group_enabled: true,
				smart_group_ids: [11, 13],
			}],
			total: 1,
			page: 1,
			page_size: 20,
			pages: 1,
		})
		listKeys.mockResolvedValueOnce({
			items: [{
				...createApiKey(),
				group_id: 11,
				group: openAIGroup,
				smart_group_enabled: true,
				smart_group_ids: [11, 13],
			}],
			total: 1,
			page: 1,
			page_size: 20,
			pages: 1,
		})
		listKeys.mockResolvedValue({
			items: [{
				...createApiKey(),
				group_id: 12,
				group: { ...openAIGroup, id: 12, name: 'OpenAI economy', rate_multiplier: 0.1 },
				smart_group_enabled: true,
				smart_group_ids: [12, 11, 13],
			}],
			total: 1,
			page: 1,
			page_size: 20,
			pages: 1,
		})
		const wrapper = await mountView()

		await wrapper.get('[data-test="key-group-control"][data-routing-mode="smart"]').trigger('click')
		await nextTick()
		await wrapper.get('[data-test="smart-group-option"][data-group-id="12"]').trigger('click')
		await wrapper.get('form#key-form').trigger('submit')
		await flushPromises()

		expect(updateKey).toHaveBeenCalledWith(1, expect.objectContaining({
			group_id: 12,
			smart_group_enabled: true,
			smart_group_ids: [12, 11, 13],
		}))
		expect(showSuccess).toHaveBeenCalledWith('API key updated successfully')
		expect(wrapper.findComponent({ name: 'GroupBadge' }).attributes('name')).toBe('OpenAI economy')
	})

	it('shows smart switch logs only for smart keys and loads the recent history', async () => {
		listKeys.mockResolvedValue({
			items: [{
				...createApiKey(),
				smart_group_enabled: true,
				smart_group_ids: [12, 11],
			}],
			total: 1,
			page: 1,
			page_size: 20,
			pages: 1,
		})
		listSmartGroupLogs.mockResolvedValue([
			{
				id: 9,
				api_key_id: 1,
				from_group_id: 11,
				from_group_name: 'OpenAI standard',
				to_group_id: 12,
				to_group_name: 'OpenAI economy',
				reason: 'failure',
				switched_at: '2026-09-05T12:00:00Z',
			},
		])
		const wrapper = await mountView()

		expect(wrapper.get('[data-test="smart-group-logs-button"]').text()).toContain('Switch logs')
		await wrapper.get('[data-test="smart-group-logs-button"]').trigger('click')
		await flushPromises()

		expect(listSmartGroupLogs).toHaveBeenCalledWith(1)
		expect(wrapper.get('[data-test="base-dialog"]').text()).toContain('OpenAI standard')
		expect(wrapper.get('[data-test="base-dialog"]').text()).toContain('OpenAI economy')
		expect(wrapper.get('[data-test="base-dialog"]').text()).toContain('Failure failover')
	})
})

<template>
  <AppLayout>
    <TablePageLayout>
      <!-- 筛选栏 -->
      <template #filters>
        <div class="flex flex-wrap items-center gap-3">
          <div class="relative w-full sm:w-64">
            <Icon name="search" size="md" class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400 dark:text-gray-500" />
            <input
              v-model="searchQuery"
              type="text"
              :placeholder="t('admin.sub2apiProviders.searchProviders')"
              class="input pl-10"
              @input="handleSearch"
            />
          </div>
          <div class="w-full sm:w-40">
            <Select v-model="filters.status" :options="statusFilterOptions" :placeholder="t('admin.sub2apiProviders.allStatus')" @change="loadProviders" />
          </div>
          <div class="flex flex-1 flex-wrap items-center justify-end gap-2">
            <button @click="loadProviders" :disabled="loading" class="btn btn-secondary" :title="t('admin.sub2apiProviders.refresh')">
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </button>
            <button @click="openCreateDialog" class="btn btn-primary">
              <Icon name="plus" size="md" class="mr-1" />
              {{ t('admin.sub2apiProviders.createProvider') }}
            </button>
          </div>
        </div>
      </template>

      <!-- 表格 -->
      <template #table>
        <DataTable :columns="columns" :data="providers" :loading="loading">

          <!-- 上游名称列 -->
          <template #cell-name="{ row }">
            <div class="min-w-0">
              <span class="font-medium text-gray-900 dark:text-white">{{ row.name }}</span>
              <div class="mt-0.5 truncate text-xs text-gray-500 dark:text-dark-400" :title="row.base_url">{{ row.base_url }}</div>
              <div v-if="row.notes" class="mt-0.5 truncate text-xs text-gray-400 dark:text-dark-500" :title="row.notes">{{ row.notes }}</div>
            </div>
          </template>

          <!-- 上游平台列 -->
          <template #cell-provider_type="{ row }">
            <span
              class="inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-medium"
              :class="providerTypeBadgeTagClass(row.provider_type)"
            >
              <span class="h-1.5 w-1.5 rounded-full" :class="providerTypeDotClass(row.provider_type)"></span>
              {{ providerTypeLabel(row.provider_type) }}
            </span>
          </template>

          <!-- 路径探测列 -->
          <template #cell-api_paths="{ row }">
            <span
              v-if="row.api_path_keys"
              class="inline-flex items-center gap-1 rounded-full bg-green-50 px-2 py-0.5 text-xs font-medium text-green-600 dark:bg-green-900/20 dark:text-green-400"
              :title="`Keys: ${row.api_path_keys}\nGroups: ${row.api_path_groups || '未探测'}`"
            >
              <span class="h-1.5 w-1.5 rounded-full bg-green-500"></span>
              {{ t('admin.sub2apiProviders.pathsReady') }}
            </span>
            <span
              v-else
              class="inline-flex items-center gap-1 rounded-full bg-amber-50 px-2 py-0.5 text-xs font-medium text-amber-600 dark:bg-amber-900/20 dark:text-amber-400"
              :title="t('admin.sub2apiProviders.pathsNotDetectedHint')"
            >
              <span class="h-1.5 w-1.5 rounded-full bg-amber-500"></span>
              {{ t('admin.sub2apiProviders.pathsNotDetected') }}
            </span>
          </template>

          <!-- 启用状态列 -->
          <template #cell-status="{ value }">
            <span :class="['badge', value === 'active' ? 'badge-success' : 'badge-gray']">
              {{ t(`admin.sub2apiProviders.statusLabels.${value}`) }}
            </span>
          </template>

          <!-- 连通状态列 -->
          <template #cell-last_sync_status="{ row }">
            <div class="flex flex-col gap-0.5">
              <span
                v-if="row.last_sync_status"
                :class="['badge text-xs', row.last_sync_status === 'success' ? 'badge-success' : 'badge-danger', row.last_sync_status === 'failed' && row.last_sync_error ? 'cursor-help' : '']"
                :title="row.last_sync_status === 'failed' ? (row.last_sync_error || '') : ''"
              >
                {{ t(`admin.sub2apiProviders.syncStatus.${row.last_sync_status}`) }}
              </span>
              <span v-else class="text-xs text-gray-400 dark:text-dark-500">{{ t('admin.sub2apiProviders.syncStatus.never') }}</span>
              <span v-if="row.last_sync_at" class="text-xs text-gray-400 dark:text-dark-500">{{ formatDateTime(row.last_sync_at) }}</span>
            </div>
          </template>

          <!-- 关联账号数列 -->
          <template #cell-accounts_count="{ row }">
            <button
              @click="openAccountsPanel(row)"
              class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium transition-colors"
              :class="(row.accounts_count ?? 0) > 0
                ? 'bg-indigo-50 text-indigo-600 hover:bg-indigo-100 dark:bg-indigo-900/20 dark:text-indigo-400 dark:hover:bg-indigo-900/40'
                : 'bg-gray-100 text-gray-400 hover:bg-gray-200 dark:bg-dark-700 dark:text-dark-500 dark:hover:bg-dark-600'"
              :title="t('admin.sub2apiProviders.viewAccounts')"
            >
              <Icon name="users" size="sm" />
              {{ row.accounts_count ?? 0 }}
            </button>
          </template>

          <!-- 更新时间列 -->
          <template #cell-updated_at="{ row }">
            <span class="text-xs text-gray-500 tabular-nums dark:text-dark-400" :title="row.updated_at">
              {{ row.updated_at ? formatDateTime(row.updated_at) : '—' }}
            </span>
          </template>

          <!-- 操作列 -->
          <template #cell-actions="{ row }">
            <div class="flex items-center gap-1">
              <!-- 👁️ 查看关联账号 -->
              <button
                @click="openAccountsPanel(row)"
                class="inline-flex items-center gap-1 rounded px-2 py-1 text-xs text-gray-600 transition-colors hover:bg-indigo-50 hover:text-indigo-600 dark:text-gray-300 dark:hover:bg-indigo-900/20 dark:hover:text-indigo-400"
              >
                <Icon name="eye" size="sm" />
                <span>{{ t('admin.sub2apiProviders.viewAccounts') }}</span>
              </button>
              <!-- ▶ 测试连接 -->
              <button
                @click="handleTestConnection(row)"
                :disabled="testingId === row.id"
                class="inline-flex items-center gap-1 rounded px-2 py-1 text-xs text-gray-600 transition-colors hover:bg-blue-50 hover:text-blue-600 disabled:opacity-40 dark:text-gray-300 dark:hover:bg-blue-900/20 dark:hover:text-blue-400"
              >
                <Icon :name="testingId === row.id ? 'refresh' : 'play'" size="sm" :class="testingId === row.id ? 'animate-spin' : ''" />
                <span>{{ t('admin.sub2apiProviders.testConnection') }}</span>
              </button>
              <!-- ✏️ 编辑 -->
              <button
                @click="openEditDialog(row)"
                class="inline-flex items-center gap-1 rounded px-2 py-1 text-xs text-gray-600 transition-colors hover:bg-gray-100 hover:text-gray-700 dark:text-gray-300 dark:hover:bg-dark-600 dark:hover:text-gray-200"
              >
                <Icon name="edit" size="sm" />
                <span>{{ t('common.edit') }}</span>
              </button>
              <!-- ⏸/▶ 启用/停用 -->
              <button
                @click="handleToggleStatus(row)"
                :disabled="togglingId === row.id"
                class="inline-flex items-center gap-1 rounded px-2 py-1 text-xs transition-colors disabled:opacity-40"
                :class="row.status === 'active'
                  ? 'text-red-500 hover:bg-red-50 hover:text-red-600 dark:text-red-400 dark:hover:bg-red-900/20'
                  : 'text-green-600 hover:bg-green-50 hover:text-green-700 dark:text-green-400 dark:hover:bg-green-900/20'"
                :title="row.status === 'active' ? t('admin.sub2apiProviders.disableProvider') : t('admin.sub2apiProviders.enableProvider')"
              >
                <Icon :name="togglingId === row.id ? 'refresh' : (row.status === 'active' ? 'ban' : 'play')" size="sm" :class="togglingId === row.id ? 'animate-spin' : ''" />
                <span>{{ row.status === 'active' ? t('admin.sub2apiProviders.disableProvider') : t('admin.sub2apiProviders.enableProvider') }}</span>
              </button>
              <!-- ⋯ 更多（低频：探测路径 / 批量优化 / 删除） -->
              <button
                @click="openActionMenu(row, $event)"
                class="inline-flex items-center gap-1 rounded px-2 py-1 text-xs text-gray-600 transition-colors hover:bg-gray-100 hover:text-gray-700 dark:text-gray-300 dark:hover:bg-dark-600 dark:hover:text-gray-200"
              >
                <Icon name="more" size="sm" />
                <span>{{ t('common.more') }}</span>
              </button>
            </div>
          </template>

          <template #empty>
            <EmptyState
              :title="t('admin.sub2apiProviders.noProviders')"
              :description="t('admin.sub2apiProviders.noProvidersDesc')"
              :action-text="t('admin.sub2apiProviders.createProvider')"
              @action="openCreateDialog"
            />
          </template>
        </DataTable>
      </template>

      <!-- 分页 -->
      <template #pagination>
        <Pagination
          v-if="pagination.total > 0"
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          @update:page="handlePageChange"
          @update:pageSize="handlePageSizeChange"
        />
      </template>
    </TablePageLayout>

    <!-- ============================================================ -->
    <!-- 👁️ 绑定账户详情面板（新增）                                    -->
    <!-- ============================================================ -->
    <BaseDialog
      :show="showAccountsPanel"
      :title="`${accountsPanelProvider?.name || ''} — ${t('admin.sub2apiProviders.viewAccounts')}`"
      width="full"
      @close="showAccountsPanel = false"
    >
      <div class="space-y-4">
        <!-- 说明区：API 路径解释 -->
        <!-- 未探测路径时的精简警示（探测好后完全不显示） -->
        <div v-if="accountsPanelProvider && !accountsPanelProvider.api_path_keys" class="rounded-lg bg-amber-50 px-3 py-2 text-xs text-amber-700 dark:bg-amber-900/20 dark:text-amber-400">
          ⚠️ 尚未探测 API 路径，关联账号与分组优化可能失败，请先在列表页点 🔍 探测路径
        </div>

        <!-- 加载中 -->
        <div v-if="loadingLinked" class="flex items-center justify-center py-8 text-gray-400">
          <Icon name="refresh" size="md" class="animate-spin mr-2" />
          加载中...
        </div>

        <!-- 账户列表区域（loading 结束后始终显示） -->
        <div v-else>
          <!-- 始终显示的头部工具栏 -->
          <div class="mb-3 flex items-center justify-between">
            <span class="text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.sub2apiProviders.linkedAccounts') }} ({{ panelLinkedAccounts.length }})
            </span>
            <div class="flex items-center gap-2">
              <!-- 关联账号（始终显示） -->
              <button
                v-if="accountsPanelProvider"
                @click="openLinkDialog(accountsPanelProvider)"
                class="btn btn-primary text-xs py-1 px-2"
              >
                <Icon name="link" size="sm" class="mr-1" />
                {{ t('admin.sub2apiProviders.linkAccount') }}
              </button>
              <!-- 刷新分组 -->
              <button
                v-if="accountsPanelProvider && panelLinkedAccounts.length > 0"
                @click="refreshLinkedAccounts(accountsPanelProvider.id, true)"
                :disabled="loadingLinked"
                class="btn btn-secondary text-xs py-1 px-2"
                :title="t('admin.sub2apiProviders.refreshRemote')"
              >
                <Icon name="refresh" size="sm" class="mr-1" :class="loadingLinked ? 'animate-spin' : ''" />
                {{ t('admin.sub2apiProviders.refreshRemote') }}
              </button>
              <!-- 批量优化 -->
              <button
                v-if="accountsPanelProvider && panelLinkedAccounts.length > 0"
                @click="handleOptimizeAll(accountsPanelProvider)"
                :disabled="optimizingAllId === accountsPanelProvider?.id"
                class="btn btn-secondary text-xs py-1 px-2"
              >
                <Icon :name="optimizingAllId === accountsPanelProvider?.id ? 'refresh' : 'bolt'" size="sm" class="mr-1" :class="optimizingAllId === accountsPanelProvider?.id ? 'animate-spin' : ''" />
                {{ t('admin.sub2apiProviders.optimizeAll') }}
              </button>
            </div>
          </div>

          <!-- 有账号：显示卡片列表（窄屏横向滚动，避免 8 列固定宽挤压变形） -->
          <div v-if="panelLinkedAccounts.length > 0" class="overflow-x-auto">
           <div class="min-w-[880px] space-y-2">
            <!-- 列表头 -->
            <div class="grid grid-cols-[1.4fr_72px_1.2fr_56px_88px_92px_92px_1.4fr_100px] gap-4 px-3 text-xs font-medium text-gray-400 dark:text-dark-500 uppercase tracking-wide">
              <span>{{ t('admin.sub2apiProviders.colAccountName') }}</span>
              <span>{{ t('admin.sub2apiProviders.colPlatform') }}</span>
              <span>{{ t('admin.sub2apiProviders.colCurrentGroup') }}</span>
              <span class="text-center">{{ t('admin.sub2apiProviders.colMultiplier') }}</span>
              <span class="text-center" :title="t('admin.sub2apiProviders.joinScheduleHint')">{{ t('admin.sub2apiProviders.joinSchedule') }}</span>
              <span class="text-center" :title="t('admin.sub2apiProviders.maxMultiplierHint')">{{ t('admin.sub2apiProviders.maxMultiplier') }}</span>
              <span class="text-center" :title="t('admin.sub2apiProviders.minMultiplierHint')">{{ t('admin.sub2apiProviders.minMultiplier') }}</span>
              <span :title="t('admin.sub2apiProviders.testModelHint')">{{ t('admin.sub2apiProviders.testModel') }}</span>
              <span class="text-right">{{ t('admin.sub2apiProviders.colActions') }}</span>
            </div>
            <!-- 数据行 -->
            <div
              v-for="acc in panelLinkedAccounts"
              :key="acc.id"
              class="grid grid-cols-[1.4fr_72px_1.2fr_56px_88px_92px_92px_1.4fr_100px] gap-4 items-center rounded-lg border border-gray-100 dark:border-dark-700 bg-white dark:bg-dark-800 px-3 py-3 hover:border-gray-200 dark:hover:border-dark-600 hover:shadow-sm transition-all"
            >
              <!-- 账号名称（KeyID 并入 title） -->
              <div class="min-w-0">
                <span :title="`${acc.name}${acc.provider_api_key_id ? ' · KeyID ' + acc.provider_api_key_id : ''}`" class="block truncate font-medium text-gray-900 dark:text-white text-sm">{{ acc.name }}</span>
              </div>
              <!-- 平台 -->
              <div>
                <span :class="['badge text-xs', acc.platform === 'anthropic' ? 'badge-warning' : acc.platform === 'openai' ? 'badge-success' : 'badge-info']">
                  {{ acc.platform }}
                </span>
              </div>
              <!-- 当前分组 -->
              <div class="min-w-0">
                <span v-if="acc.remote_group_name" :title="acc.remote_group_name" class="block truncate text-sm text-gray-700 dark:text-gray-300">{{ acc.remote_group_name }}</span>
                <span v-else class="text-xs text-gray-400 dark:text-dark-500 italic">未同步</span>
              </div>
              <!-- 倍率 -->
              <div class="text-center">
                <span
                  v-if="acc.remote_group_multiplier != null"
                  :class="[
                    'inline-block rounded-full px-2 py-0.5 text-xs font-bold font-mono',
                    acc.remote_group_multiplier <= 0.5
                      ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400'
                      : acc.remote_group_multiplier <= 1.0
                        ? 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400'
                        : 'bg-red-100 text-red-600 dark:bg-red-900/30 dark:text-red-400'
                  ]"
                >×{{ acc.remote_group_multiplier }}</span>
                <span v-else class="text-gray-400 text-xs">—</span>
              </div>
              <!-- 参与定时开关（独立列） -->
              <div class="flex items-center justify-center">
                <button
                  type="button"
                  @click="handleToggleParticipate(acc)"
                  :disabled="savingSettingsId === acc.id"
                  class="relative inline-flex h-5 w-9 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 focus:outline-none disabled:opacity-40"
                  :class="acc.sub2api_optimize_enabled ? 'bg-blue-600' : 'bg-gray-200 dark:bg-dark-600'"
                  :title="acc.sub2api_optimize_enabled ? t('admin.sub2apiProviders.joinScheduleOn') : t('admin.sub2apiProviders.joinScheduleOff')"
                >
                  <span
                    class="pointer-events-none inline-block h-4 w-4 transform rounded-full bg-white shadow ring-0 transition duration-200"
                    :class="acc.sub2api_optimize_enabled ? 'translate-x-4' : 'translate-x-0'"
                  />
                </button>
              </div>
              <!-- 倍率上限输入（独立列，不参与时置灰但保留值） -->
              <div class="flex items-center justify-center">
                <input
                  type="number"
                  step="0.1"
                  min="0"
                  :value="acc.sub2api_max_multiplier ?? ''"
                  :placeholder="t('admin.sub2apiProviders.maxMultiplier')"
                  @change="handleUpdateMaxMultiplier(acc, ($event.target as HTMLInputElement).value)"
                  :disabled="savingSettingsId === acc.id"
                  :title="t('admin.sub2apiProviders.maxMultiplierHint')"
                  class="w-16 rounded border border-gray-200 bg-white px-1.5 py-1 text-center text-xs font-mono text-gray-700 focus:border-blue-400 focus:outline-none disabled:cursor-not-allowed disabled:bg-gray-100 disabled:text-gray-400 dark:border-dark-600 dark:bg-dark-700 dark:text-gray-300 dark:disabled:bg-dark-800 dark:disabled:text-dark-500"
                />
              </div>
              <!-- 倍率下限输入（独立列，不参与时置灰但保留值；留空=无下限） -->
              <div class="flex items-center justify-center">
                <input
                  type="number"
                  step="0.1"
                  min="0"
                  :value="acc.sub2api_min_multiplier ?? ''"
                  :placeholder="t('admin.sub2apiProviders.minMultiplier')"
                  @change="handleUpdateMinMultiplier(acc, ($event.target as HTMLInputElement).value)"
                  :disabled="savingSettingsId === acc.id"
                  :title="t('admin.sub2apiProviders.minMultiplierHint')"
                  class="w-16 rounded border border-gray-200 bg-white px-1.5 py-1 text-center text-xs font-mono text-gray-700 focus:border-blue-400 focus:outline-none disabled:cursor-not-allowed disabled:bg-gray-100 disabled:text-gray-400 dark:border-dark-600 dark:bg-dark-700 dark:text-gray-300 dark:disabled:bg-dark-800 dark:disabled:text-dark-500"
                />
              </div>
              <!-- 测试模型下拉（不参与时置灰但保留值，焦点时懒加载模型列表） -->
              <div class="min-w-0">
                <select
                  :value="acc.sub2api_test_model ?? ''"
                  @focus="loadAccountModels(acc)"
                  @change="handleUpdateTestModel(acc, ($event.target as HTMLSelectElement).value)"
                  :disabled="savingSettingsId === acc.id"
                  :title="acc.sub2api_test_model ?? t('admin.sub2apiProviders.defaultModel')"
                  class="w-full rounded border border-gray-200 bg-white px-1.5 py-1 text-xs text-gray-700 focus:border-blue-400 focus:outline-none disabled:cursor-not-allowed disabled:bg-gray-100 disabled:text-gray-400 dark:border-dark-600 dark:bg-dark-700 dark:text-gray-300 dark:disabled:bg-dark-800 dark:disabled:text-dark-500"
                >
                  <option value="">{{ t('admin.sub2apiProviders.defaultModel') }}</option>
                  <!-- 已保存但不在列表中的模型也要能显示 -->
                  <option
                    v-if="acc.sub2api_test_model && !(accountModels[acc.id] || []).some(m => m.id === acc.sub2api_test_model)"
                    :value="acc.sub2api_test_model"
                  >{{ acc.sub2api_test_model }}</option>
                  <option v-for="m in accountModels[acc.id] || []" :key="m.id" :value="m.id">{{ m.id }}</option>
                </select>
              </div>
              <!-- 操作 -->
              <div class="flex items-center justify-end gap-1">
                <!-- 模型测试 -->
                <button
                  @click="openModelTest(acc)"
                  class="rounded p-1.5 text-blue-500 hover:bg-blue-50 dark:hover:bg-blue-900/20 transition-colors"
                  :title="t('admin.sub2apiProviders.testModel')"
                >
                  <Icon name="play" size="sm" />
                </button>
                <!-- 立即优化 -->
                <button
                  @click="handleOptimizeAccount(accountsPanelProvider!, acc)"
                  :disabled="optimizingAccountId === acc.id"
                  class="rounded p-1.5 text-orange-500 hover:bg-orange-50 dark:hover:bg-orange-900/20 disabled:opacity-40 transition-colors"
                  :title="t('admin.sub2apiProviders.optimizeAccount')"
                >
                  <Icon :name="optimizingAccountId === acc.id ? 'refresh' : 'bolt'" size="sm" :class="optimizingAccountId === acc.id ? 'animate-spin' : ''" />
                </button>
                <!-- 解除关联 -->
                <button
                  @click="handleUnlinkAccount(accountsPanelProvider!, acc)"
                  :disabled="unlinkingAccountId === acc.id"
                  class="rounded p-1.5 text-red-400 hover:bg-red-50 dark:hover:bg-red-900/20 disabled:opacity-40 transition-colors"
                  :title="t('admin.sub2apiProviders.unlinkAccount')"
                >
                  <Icon :name="unlinkingAccountId === acc.id ? 'refresh' : 'x'" size="sm" :class="unlinkingAccountId === acc.id ? 'animate-spin' : ''" />
                </button>
              </div>
            </div>
           </div>
          </div>

          <!-- 无账号时的空状态 -->
          <div v-else class="rounded-lg border border-dashed border-gray-200 p-8 text-center text-sm text-gray-400 dark:border-dark-600 dark:text-dark-500">
            {{ t('admin.sub2apiProviders.noLinkedAccounts') }}
          </div>
        </div>
      </div>
    </BaseDialog>

    <!-- ============================================================ -->
    <!-- 🔗 关联账号对话框（仅关联新账号）                               -->
    <!-- ============================================================ -->
    <BaseDialog
      :show="showLinkDialog"
      :title="`${t('admin.sub2apiProviders.linkAccountTitle')} — ${currentProvider?.name || ''}`"
      @close="closeLinkDialog"
    >
      <div class="space-y-4">
        <p class="text-sm text-gray-600 dark:text-dark-400">{{ t('admin.sub2apiProviders.linkAccountDesc') }}</p>

        <!-- 选择账号 -->
        <div>
          <label class="input-label">{{ t('admin.sub2apiProviders.selectAccount') }}</label>

          <!-- 加载中提示 -->
          <div v-if="loadingAccounts" class="flex items-center gap-2 rounded-lg border border-gray-200 px-3 py-2 text-sm text-gray-400 dark:border-dark-600 dark:text-dark-500">
            <Icon name="refresh" size="sm" class="animate-spin" />
            正在加载账号列表...
          </div>

          <!-- 账号选择（支持搜索，options>5 自动启用） -->
          <Select
            v-else
            v-model="selectedAccountId"
            :options="availableAccountOptions"
            :searchable="true"
            search-placeholder="搜索账号名称或平台…"
            :placeholder="availableAccountOptions.length === 0 ? '所有账号均已关联' : t('admin.sub2apiProviders.selectAccountPlaceholder')"
          />

          <p class="input-hint">
            显示"账号管理"里所有账号（Anthropic / OpenAI / Gemini），关联后系统会用账号的 api_key 在远端匹配对应 APIKey
          </p>

          <!-- 无可用账号时的提示 -->
          <div v-if="!loadingAccounts && availableAccountOptions.length === 0 && allAccounts.length > 0" class="mt-2 rounded-lg bg-amber-50 p-2 text-xs text-amber-700 dark:bg-amber-900/20 dark:text-amber-300">
            ⚠️ 所有账号均已关联到此代理，可在"👁️查看绑定账户"面板中管理
          </div>
        </div>

        <!-- 已关联账号数量提示 -->
        <div v-if="panelLinkedAccounts.length > 0" class="text-xs text-gray-500 dark:text-dark-400">
          当前已关联 {{ panelLinkedAccounts.length }} 个账号，
          <button @click="closeLinkDialog(); openAccountsPanel(currentProvider!)" class="text-primary-500 hover:underline">点此查看详情</button>
        </div>
      </div>

      <template #footer>
        <div class="flex justify-end gap-3">
          <button type="button" class="btn btn-secondary" @click="closeLinkDialog">{{ t('common.cancel') }}</button>
          <button
            @click="handleLinkAccount"
            :disabled="!selectedAccountId || linking"
            class="btn btn-primary"
          >
            {{ linking ? t('admin.sub2apiProviders.linking') : t('admin.sub2apiProviders.linkAccount') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <!-- ============================================================ -->
    <!-- ✏️ 创建/编辑 Provider 对话框                                  -->
    <!-- ============================================================ -->
    <BaseDialog
      :show="showEditDialog"
      :title="isEditing ? t('admin.sub2apiProviders.editProvider') : t('admin.sub2apiProviders.createProvider')"
      width="wide"
      @close="closeEditDialog"
    >
      <form id="provider-form" @submit.prevent="handleSave" class="space-y-4">
        <div>
          <label class="input-label">{{ t('admin.sub2apiProviders.form.name') }}</label>
          <input v-model="form.name" type="text" class="input" :placeholder="t('admin.sub2apiProviders.form.namePlaceholder')" required />
        </div>

        <div>
          <label class="input-label">{{ t('admin.sub2apiProviders.form.baseUrl') }}</label>
          <input v-model="form.base_url" type="url" class="input" :placeholder="t('admin.sub2apiProviders.form.baseUrlPlaceholder')" required />
          <p class="input-hint">例如 https://direct.jinnyapi.com（同一实例可管理多个平台的账号）</p>
        </div>

        <!-- 上游平台仅创建时选择（当前仅 sub2api）；编辑时隐藏，避免切换上游类型导致接口逻辑与历史数据不匹配 -->
        <div v-if="!isEditing">
          <label class="input-label">{{ t('admin.sub2apiProviders.form.providerType') }}</label>
          <div class="flex flex-wrap gap-3">
            <button
              v-for="opt in providerTypeOptions"
              :key="opt.value"
              type="button"
              @click="form.provider_type = opt.value"
              class="group relative flex items-center gap-3 overflow-hidden rounded-xl border px-4 py-3 text-left transition-all duration-200 focus:outline-none"
              :class="providerTypeCardClass(opt.color, form.provider_type === opt.value)"
            >
              <!-- 选中态柔和光晕背景 -->
              <span
                v-if="form.provider_type === opt.value"
                class="pointer-events-none absolute inset-0 opacity-60"
                :class="providerTypeGlowClass(opt.color)"
              />
              <!-- 图标徽章 -->
              <span
                class="relative flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-lg transition-colors"
                :class="providerTypeBadgeClass(opt.color, form.provider_type === opt.value)"
              >
                <Icon name="grid" size="sm" />
              </span>
              <!-- 名称 -->
              <span
                class="relative text-sm font-semibold transition-colors"
                :class="form.provider_type === opt.value ? providerTypeTextClass(opt.color) : 'text-gray-600 dark:text-dark-300'"
              >
                {{ opt.label }}
              </span>
              <!-- 选中勾选角标 -->
              <span
                v-if="form.provider_type === opt.value"
                class="relative flex h-5 w-5 flex-shrink-0 items-center justify-center rounded-full text-white shadow-sm"
                :class="providerTypeCheckClass(opt.color)"
              >
                <Icon name="check" size="xs" :stroke-width="3" />
              </span>
            </button>
          </div>
          <p class="input-hint">{{ t('admin.sub2apiProviders.form.providerTypeHint') }}</p>
        </div>

        <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
          <div>
            <label class="input-label">{{ t('admin.sub2apiProviders.form.email') }}</label>
            <input v-model="form.email" type="email" class="input" :placeholder="t('admin.sub2apiProviders.form.emailPlaceholder')" required />
          </div>
          <div>
            <label class="input-label">{{ t('admin.sub2apiProviders.form.password') }}</label>
            <input v-model="form.password" type="password" class="input" :placeholder="isEditing ? '留空则不修改密码' : t('admin.sub2apiProviders.form.passwordPlaceholder')" :required="!isEditing" />
          </div>
        </div>

        <div v-if="isEditing">
          <label class="input-label">{{ t('admin.sub2apiProviders.form.status') }}</label>
          <Select v-model="form.status" :options="statusOptions" />
        </div>

        <div>
          <label class="input-label">{{ t('admin.sub2apiProviders.form.notes') }}</label>
          <textarea v-model="form.notes" rows="2" class="input" :placeholder="t('admin.sub2apiProviders.form.notesPlaceholder')"></textarea>
        </div>
      </form>

      <template #footer>
        <div class="flex justify-end gap-3">
          <button type="button" class="btn btn-secondary" @click="closeEditDialog">{{ t('common.cancel') }}</button>
          <button type="submit" form="provider-form" class="btn btn-primary" :disabled="saving">
            {{ saving ? t('common.saving') : t('common.save') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <!-- ============================================================ -->
    <!-- 🗑️ 删除确认                                                   -->
    <!-- ============================================================ -->
    <ConfirmDialog
      :show="showDeleteDialog"
      :title="t('admin.sub2apiProviders.deleteProvider')"
      :message="(deletingProvider?.accounts_count ?? 0) > 0
        ? `${t('admin.sub2apiProviders.deleteConfirmMessage', { name: deletingProvider?.name ?? '' })}\n\n⚠️ 该上游已关联 ${deletingProvider?.accounts_count} 个账号，删除后账号将自动解绑。`
        : t('admin.sub2apiProviders.deleteConfirmMessage', { name: deletingProvider?.name ?? '' })"
      :confirm-text="t('common.delete')"
      :cancel-text="t('common.cancel')"
      danger
      @confirm="confirmDelete"
      @cancel="showDeleteDialog = false"
    />

    <!-- ============================================================ -->
    <!-- ⋯ 更多操作下拉菜单（探测路径 / 批量优化 / 删除）               -->
    <!-- ============================================================ -->
    <Sub2APIProviderActionMenu
      :show="actionMenu.show"
      :provider="actionMenu.provider"
      :position="actionMenu.position"
      :detecting="detectingId === actionMenu.provider?.id"
      :optimizing="optimizingAllId === actionMenu.provider?.id"
      @close="actionMenu.show = false"
      @detect-paths="handleDetectPaths"
      @optimize-all="handleOptimizeAll"
      @schedule-optimize="handleScheduleOptimize"
      @delete="handleDeleteClick"
    />

    <!-- ============================================================ -->
    <!-- ⏰ 定时优化配置                                               -->
    <!-- ============================================================ -->
    <Sub2APIOptimizeScheduleModal
      v-if="scheduleModal.providerId"
      :show="scheduleModal.show"
      :provider-id="scheduleModal.providerId"
      @close="scheduleModal.show = false"
    />

    <!-- ============================================================ -->
    <!-- ▶️ 账号模型测试（复用账号测试弹窗）                            -->
    <!-- ============================================================ -->
    <AccountTestModal
      :show="modelTestModal.show"
      :account="modelTestModal.account"
      @close="modelTestModal.show = false"
    />

    <!-- ============================================================ -->
    <!-- ⚡ 批量优化结果                                                -->
    <!-- ============================================================ -->
    <BaseDialog
      :show="showOptimizeResultDialog"
      :title="t('admin.sub2apiProviders.optimizeAllTitle')"
      @close="showOptimizeResultDialog = false"
    >
      <div v-if="optimizeAllResult" class="space-y-3">
        <div class="grid grid-cols-3 gap-2 text-center">
          <div class="rounded-lg bg-green-50 p-3 dark:bg-green-900/20">
            <div class="text-2xl font-bold text-green-600 dark:text-green-400">{{ optimizeAllResult.optimized }}</div>
            <div class="text-xs text-green-700 dark:text-green-300">{{ t('admin.sub2apiProviders.optimizeStatOptimized') }}</div>
          </div>
          <div class="rounded-lg bg-blue-50 p-3 dark:bg-blue-900/20">
            <div class="text-2xl font-bold text-blue-600 dark:text-blue-400">{{ optimizeAllResult.skipped }}</div>
            <div class="text-xs text-blue-700 dark:text-blue-300">{{ t('admin.sub2apiProviders.optimizeStatSkipped') }}</div>
          </div>
          <div class="rounded-lg bg-red-50 p-3 dark:bg-red-900/20">
            <div class="text-2xl font-bold text-red-500">{{ optimizeAllResult.failed }}</div>
            <div class="text-xs text-red-600">{{ t('admin.sub2apiProviders.optimizeStatFailed') }}</div>
          </div>
        </div>

        <div v-if="optimizeAllResult.results.length > 0" class="max-h-60 overflow-y-auto space-y-1">
          <div
            v-for="r in optimizeAllResult.results" :key="r.account_id"
            class="rounded p-2 text-sm"
            :class="r.status === 'optimized' ? 'bg-green-50 dark:bg-green-900/10' : r.status === 'skipped' ? 'bg-blue-50 dark:bg-blue-900/10' : 'bg-red-50 dark:bg-red-900/10'"
          >
            <div class="flex items-center justify-between">
              <span class="font-medium">{{ r.account_name }}</span>
              <div class="text-xs font-mono">
                <span v-if="r.status === 'optimized'" class="text-green-600">×{{ r.old_multiplier ?? '-' }} → <strong>×{{ r.new_multiplier ?? '-' }}</strong></span>
                <span v-else-if="r.status === 'skipped'" class="text-blue-500">{{ r.new_multiplier != null ? '×' + r.new_multiplier : t('admin.sub2apiProviders.optimizeStatSkipped') }}</span>
                <span v-else class="text-red-500">{{ t('admin.sub2apiProviders.optimizeStatFailed') }}</span>
              </div>
            </div>
            <div v-if="r.reason" class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">{{ r.reason }}</div>
          </div>
        </div>
      </div>
    </BaseDialog>

  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { adminAPI } from '@/api/admin'
import type { Sub2APIProvider, OptimizeAllResult, LinkedAccountInfo } from '@/api/admin/sub2apiProviders'
import type { Account, ClaudeModel } from '@/types'
import { formatDateTime } from '@/utils/format'
import type { Column } from '@/components/common/types'
import { findIncompleteAccounts, validateMaxMultiplier, validateMinMultiplier, getIncompleteConfigError } from '@/utils/sub2apiValidation'
import { extractErrorMessage } from '@/utils/errorHandler'

import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Sub2APIProviderActionMenu from '@/components/admin/Sub2APIProviderActionMenu.vue'
import Sub2APIOptimizeScheduleModal from '@/components/admin/Sub2APIOptimizeScheduleModal.vue'
import AccountTestModal from '@/components/account/AccountTestModal.vue'
import Select from '@/components/common/Select.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()
const appStore = useAppStore()

// ==================== 列表状态 ====================
const providers = ref<Sub2APIProvider[]>([])
const loading = ref(false)
const searchQuery = ref('')
const filters = reactive({ status: '' })
const pagination = reactive({ page: 1, page_size: getPersistedPageSize(), total: 0, pages: 0 })

const columns = computed((): Column[] => [
  { key: 'name', label: '上游名称', sortable: false },
  { key: 'provider_type', label: '上游平台', sortable: false },
  { key: 'status', label: '启用', sortable: false },
  { key: 'last_sync_status', label: '连通状态', sortable: false },
  { key: 'api_paths', label: '路径探测', sortable: false },
  { key: 'accounts_count', label: '关联账号', sortable: false },
  { key: 'updated_at', label: '更新时间', sortable: false },
  { key: 'actions', label: '操作', sortable: false }
])

const statusFilterOptions = computed(() => [
  { value: '', label: t('admin.sub2apiProviders.allStatus') },
  { value: 'active', label: t('admin.sub2apiProviders.statusLabels.active') },
  { value: 'inactive', label: t('admin.sub2apiProviders.statusLabels.inactive') }
])

const statusOptions = computed(() => [
  { value: 'active', label: t('admin.sub2apiProviders.statusLabels.active') },
  { value: 'inactive', label: t('admin.sub2apiProviders.statusLabels.inactive') }
])

// 上游平台（provider_type）：当前仅支持 sub2api，后续扩展其他上游协议时在此追加选项。
// color 用于单选按钮组的选中态配色，新增平台时给一个区分色即可。
const providerTypeOptions = [
  { value: 'sub2api', label: 'Sub2API', color: 'blue' },
]

// 卡片选择器配色（Tailwind 需静态字面量，故用查表而非拼接）。
// 新增平台时给一个 color（blue/green/purple/orange）即可自动套用整套配色。
const PROVIDER_TYPE_PALETTE: Record<string, {
  card: string           // 选中态卡片：边框 + 底色 + 阴影
  glow: string           // 选中态柔和光晕背景（渐变）
  badge: string          // 选中态图标徽章
  badgeOff: string       // 未选中图标徽章
  text: string           // 选中态文字
  check: string          // 勾选角标底色
}> = {
  blue: {
    card: 'border-blue-400 bg-blue-50/60 shadow-md shadow-blue-500/10 dark:border-blue-500/60 dark:bg-blue-900/20',
    glow: 'bg-gradient-to-br from-blue-100/70 via-transparent to-transparent dark:from-blue-800/20',
    badge: 'bg-blue-500 text-white shadow-sm shadow-blue-500/30',
    badgeOff: 'bg-gray-100 text-gray-400 group-hover:bg-blue-100 group-hover:text-blue-500 dark:bg-dark-700 dark:text-dark-400',
    text: 'text-blue-700 dark:text-blue-300',
    check: 'bg-blue-500',
  },
  green: {
    card: 'border-green-400 bg-green-50/60 shadow-md shadow-green-500/10 dark:border-green-500/60 dark:bg-green-900/20',
    glow: 'bg-gradient-to-br from-green-100/70 via-transparent to-transparent dark:from-green-800/20',
    badge: 'bg-green-500 text-white shadow-sm shadow-green-500/30',
    badgeOff: 'bg-gray-100 text-gray-400 group-hover:bg-green-100 group-hover:text-green-500 dark:bg-dark-700 dark:text-dark-400',
    text: 'text-green-700 dark:text-green-300',
    check: 'bg-green-500',
  },
  purple: {
    card: 'border-purple-400 bg-purple-50/60 shadow-md shadow-purple-500/10 dark:border-purple-500/60 dark:bg-purple-900/20',
    glow: 'bg-gradient-to-br from-purple-100/70 via-transparent to-transparent dark:from-purple-800/20',
    badge: 'bg-purple-500 text-white shadow-sm shadow-purple-500/30',
    badgeOff: 'bg-gray-100 text-gray-400 group-hover:bg-purple-100 group-hover:text-purple-500 dark:bg-dark-700 dark:text-dark-400',
    text: 'text-purple-700 dark:text-purple-300',
    check: 'bg-purple-500',
  },
  orange: {
    card: 'border-orange-400 bg-orange-50/60 shadow-md shadow-orange-500/10 dark:border-orange-500/60 dark:bg-orange-900/20',
    glow: 'bg-gradient-to-br from-orange-100/70 via-transparent to-transparent dark:from-orange-800/20',
    badge: 'bg-orange-500 text-white shadow-sm shadow-orange-500/30',
    badgeOff: 'bg-gray-100 text-gray-400 group-hover:bg-orange-100 group-hover:text-orange-500 dark:bg-dark-700 dark:text-dark-400',
    text: 'text-orange-700 dark:text-orange-300',
    check: 'bg-orange-500',
  },
}
const providerTypePalette = (color: string) => PROVIDER_TYPE_PALETTE[color] ?? PROVIDER_TYPE_PALETTE.blue

// 表格里「上游平台」列的徽章配色（浅底 + 圆点），按 provider_type 查表
const PROVIDER_TYPE_TAG: Record<string, { tag: string; dot: string }> = {
  blue: { tag: 'bg-blue-50 text-blue-600 dark:bg-blue-900/20 dark:text-blue-400', dot: 'bg-blue-500' },
  green: { tag: 'bg-green-50 text-green-600 dark:bg-green-900/20 dark:text-green-400', dot: 'bg-green-500' },
  purple: { tag: 'bg-purple-50 text-purple-600 dark:bg-purple-900/20 dark:text-purple-400', dot: 'bg-purple-500' },
  orange: { tag: 'bg-orange-50 text-orange-600 dark:bg-orange-900/20 dark:text-orange-400', dot: 'bg-orange-500' },
}
// provider_type 值 → 配色 key（未知类型回退 blue）
const providerTypeTagColor = (v: string) =>
  providerTypeOptions.find((o) => o.value === v)?.color ?? 'blue'
const providerTypeBadgeTagClass = (v: string) =>
  (PROVIDER_TYPE_TAG[providerTypeTagColor(v)] ?? PROVIDER_TYPE_TAG.blue).tag
const providerTypeDotClass = (v: string) =>
  (PROVIDER_TYPE_TAG[providerTypeTagColor(v)] ?? PROVIDER_TYPE_TAG.blue).dot

// 卡片整体：选中态用配色，未选中用中性描边 + hover 轻微抬升
const providerTypeCardClass = (color: string, active: boolean): string =>
  active
    ? providerTypePalette(color).card
    : 'border-gray-200 bg-white hover:border-gray-300 hover:shadow-sm dark:border-dark-600 dark:bg-dark-800 dark:hover:border-dark-500'
const providerTypeGlowClass = (color: string) => providerTypePalette(color).glow
const providerTypeBadgeClass = (color: string, active: boolean): string =>
  active ? providerTypePalette(color).badge : providerTypePalette(color).badgeOff
const providerTypeTextClass = (color: string) => providerTypePalette(color).text
const providerTypeCheckClass = (color: string) => providerTypePalette(color).check
// 编辑态只读展示用：把 provider_type 值映射为展示名，未知值原样回退
const providerTypeLabel = (v: string) =>
  providerTypeOptions.find((o) => o.value === v)?.label ?? v

// ==================== 加载数据 ====================
let searchTimer: ReturnType<typeof setTimeout> | null = null
const handleSearch = () => {
  if (searchTimer) clearTimeout(searchTimer)
  searchTimer = setTimeout(() => { pagination.page = 1; loadProviders() }, 300)
}

const loadProviders = async () => {
  loading.value = true
  try {
    const result = await adminAPI.sub2apiProviders.list(
      pagination.page, pagination.page_size,
      { status: filters.status || undefined, search: searchQuery.value || undefined }
    )
    providers.value = result.items
    pagination.total = result.total
    pagination.pages = result.pages
  } catch {
    appStore.showError(t('admin.sub2apiProviders.loadFailed'))
  } finally { loading.value = false }
}

const handlePageChange = (page: number) => { pagination.page = page; loadProviders() }
const handlePageSizeChange = (size: number) => { pagination.page_size = size; pagination.page = 1; loadProviders() }

// ==================== 创建/编辑 ====================
const showEditDialog = ref(false)
const isEditing = ref(false)
const editingId = ref<number | null>(null)
const saving = ref(false)
const form = reactive({ name: '', base_url: '', provider_type: 'sub2api', email: '', password: '', status: 'active' as 'active'|'inactive', notes: '' })
const resetForm = () => Object.assign(form, { name:'', base_url:'', provider_type:'sub2api', email:'', password:'', status:'active', notes:'' })

const openCreateDialog = () => { isEditing.value=false; editingId.value=null; resetForm(); showEditDialog.value=true }
const openEditDialog = (row: Sub2APIProvider) => {
  isEditing.value=true; editingId.value=row.id
  Object.assign(form, { name:row.name, base_url:row.base_url, provider_type:row.provider_type||'sub2api', email:row.email, password:'', status:row.status, notes:row.notes??'' })
  showEditDialog.value=true
}
const closeEditDialog = () => { showEditDialog.value=false }

const handleSave = async () => {
  saving.value=true
  try {
    if (isEditing.value && editingId.value) {
      const payload: Record<string,unknown> = { name:form.name, base_url:form.base_url, email:form.email, status:form.status, notes:form.notes||null }
      if (form.password) payload.password = form.password
      const updated = await adminAPI.sub2apiProviders.update(editingId.value, payload as any)
      // 局部替换该行，不刷整列表
      updateProviderInList(editingId.value, updated)
      appStore.showSuccess(t('admin.sub2apiProviders.updateSuccess'))
      closeEditDialog()
    } else {
      const created = await adminAPI.sub2apiProviders.create({
        name:form.name, base_url:form.base_url, provider_type:form.provider_type,
        email:form.email, password:form.password, notes:form.notes||null
      })
      appStore.showSuccess(t('admin.sub2apiProviders.createSuccess'))
      closeEditDialog()
      // 新建：追加到列表头部，刷新总数
      providers.value = [created, ...providers.value]
      pagination.total += 1
      // 后台自动探测路径，探测完毕局部更新该行
      appStore.showInfo('正在自动探测 API 路径…')
      adminAPI.sub2apiProviders.detectPaths(created.id)
        .then(r => {
          updateProviderInList(created.id, {
            api_path_keys: r.keys_path,
            api_path_groups: r.groups_path,
            last_sync_status: 'success',
            last_sync_at: new Date().toISOString(),
          })
          appStore.showSuccess('路径探测成功')
        })
        .catch(() => {
          updateProviderInList(created.id, { last_sync_status: 'failed' })
          appStore.showError('路径探测失败，请稍后手动探测')
        })
    }
  } catch (e:any) {
    appStore.showError(extractErrorMessage(e, t('admin.sub2apiProviders.saveFailed')))
  }
  finally { saving.value=false }
}

// ==================== 局部更新工具 ====================
// 按 id 就地替换 providers 数组中的某一行，避免全量重新加载导致页面刷新。
// accounts_count 由 List 接口通过 eager-load 计算，update/testConnection 接口
// 不返回该值（会返回 0），此处保留列表里的原始值，避免误清零。
const updateProviderInList = (id: number, updates: Partial<Sub2APIProvider>) => {
  const idx = providers.value.findIndex(p => p.id === id)
  if (idx !== -1) {
    const current = providers.value[idx]
    providers.value[idx] = {
      ...current,
      ...updates,
      accounts_count: updates.accounts_count || current.accounts_count,
    }
  }
}

// ==================== 删除 ====================
const showDeleteDialog = ref(false)
const deletingProvider = ref<Sub2APIProvider|null>(null)
const handleDeleteClick = (row: Sub2APIProvider) => { deletingProvider.value=row; showDeleteDialog.value=true }
const confirmDelete = async () => {
  if (!deletingProvider.value) return
  try {
    await adminAPI.sub2apiProviders.delete(deletingProvider.value.id)
    appStore.showSuccess(t('admin.sub2apiProviders.deleteSuccess'))
    // 局部移除，不重刷整列表
    const id = deletingProvider.value.id
    providers.value = providers.value.filter(p => p.id !== id)
    pagination.total = Math.max(0, pagination.total - 1)
    showDeleteDialog.value = false
  } catch (e:any) {
    appStore.showError(extractErrorMessage(e, t('admin.sub2apiProviders.deleteFailed')))
  }
}

// ==================== 启用 / 停用 ====================
const togglingId = ref<number|null>(null)
const handleToggleStatus = async (row: Sub2APIProvider) => {
  togglingId.value = row.id
  const next = row.status === 'active' ? 'inactive' : 'active'
  try {
    const updated = await adminAPI.sub2apiProviders.update(row.id, { status: next } as any)
    // 局部替换该行，不刷新整列表
    updateProviderInList(row.id, updated)
    appStore.showSuccess(
      next === 'active'
        ? t('admin.sub2apiProviders.enableProvider')
        : t('admin.sub2apiProviders.disableProvider')
    )
  } catch (e: any) {
    appStore.showError(extractErrorMessage(e, t('common.error')))
  } finally {
    togglingId.value = null
  }
}

// ==================== 测试连接 ====================
const testingId = ref<number|null>(null)
const handleTestConnection = async (row: Sub2APIProvider) => {
  testingId.value = row.id
  try {
    await adminAPI.sub2apiProviders.testConnection(row.id)
    appStore.showSuccess(t('admin.sub2apiProviders.connectionSuccess'))
    // testConnection 只返回 message，用 getById 拉取最新 last_sync_status/last_sync_at
    const res = await adminAPI.sub2apiProviders.getById(row.id)
    updateProviderInList(row.id, res.provider)
  } catch (e:any) {
    appStore.showError(extractErrorMessage(e, t('admin.sub2apiProviders.connectionFailed')))
    // 失败时也局部刷新，更新 last_sync_status=failed
    adminAPI.sub2apiProviders.getById(row.id)
      .then(res => updateProviderInList(row.id, res.provider))
      .catch(() => {})
  } finally {
    testingId.value = null
  }
}

// ==================== 探测路径 ====================
const detectingId = ref<number|null>(null)
const handleDetectPaths = async (row: Sub2APIProvider) => {
  detectingId.value = row.id
  try {
    const r = await adminAPI.sub2apiProviders.detectPaths(row.id)
    // 局部更新路径字段和连通状态，不刷整列表
    updateProviderInList(row.id, {
      api_path_keys: r.keys_path,
      api_path_groups: r.groups_path,
      last_sync_status: 'success',
      last_sync_at: new Date().toISOString(),
    })
    appStore.showSuccess(t('admin.sub2apiProviders.pathsDetected', { keys: r.keys_path, groups: r.groups_path }))
  } catch (e:any) {
    appStore.showError(extractErrorMessage(e, t('admin.sub2apiProviders.pathsDetectFailed')))
    updateProviderInList(row.id, { last_sync_status: 'failed' })
  } finally {
    detectingId.value = null
  }
}

// ==================== ⋯ 更多操作菜单 ====================
const actionMenu = reactive<{
  show: boolean
  provider: Sub2APIProvider | null
  position: { top: number; left: number } | null
}>({ show: false, provider: null, position: null })

const openActionMenu = (row: Sub2APIProvider, event: MouseEvent) => {
  const btn = (event.currentTarget as HTMLElement).getBoundingClientRect()
  // 菜单宽 176px（w-44），右对齐到按钮右缘，向下 4px
  actionMenu.position = { top: btn.bottom + 4, left: btn.right - 176 }
  actionMenu.provider = row
  actionMenu.show = true
}

// ==================== ⏰ 定时优化配置弹窗 ====================
const scheduleModal = reactive<{
  show: boolean
  providerId: number | null
}>({ show: false, providerId: null })

const handleScheduleOptimize = (row: Sub2APIProvider) => {
  scheduleModal.providerId = row.id
  scheduleModal.show = true
}

// ==================== 👁️ 查看绑定账户面板（新增）====================
const showAccountsPanel = ref(false)
const accountsPanelProvider = ref<Sub2APIProvider|null>(null)
const panelLinkedAccounts = ref<LinkedAccountInfo[]>([])
const loadingLinked = ref(false)
const optimizingAccountId = ref<number|null>(null)
const unlinkingAccountId = ref<number|null>(null)
const savingSettingsId = ref<number|null>(null)
// 每账号懒加载的模型列表（key=账号ID）
const accountModels = reactive<Record<number, ClaudeModel[]>>({})
const loadingModelsId = ref<number|null>(null)

// 全量覆盖保存：始终把「是否参与 + 倍率上限 + 测试模型」三元组一起提交，
// 倍率上限/测试模型的值与参与状态解耦，关闭参与后照常保留。
const saveAccountOptimizeSettings = async (
  acc: LinkedAccountInfo,
  next: { enabled: boolean; min_multiplier: number | null; max_multiplier: number | null; test_model: string | null }
) => {
  const provider = accountsPanelProvider.value
  if (!provider) return
  savingSettingsId.value = acc.id
  try {
    await adminAPI.sub2apiProviders.updateAccountOptimizeSettings(provider.id, acc.id, {
      enabled: next.enabled,
      min_multiplier: next.min_multiplier,
      max_multiplier: next.max_multiplier,
      test_model: next.test_model,
    })
    // 本地更新，避免整表刷新
    const idx = panelLinkedAccounts.value.findIndex(a => a.id === acc.id)
    if (idx !== -1) {
      panelLinkedAccounts.value[idx] = {
        ...panelLinkedAccounts.value[idx],
        sub2api_optimize_enabled: next.enabled,
        sub2api_min_multiplier: next.min_multiplier ?? undefined,
        sub2api_max_multiplier: next.max_multiplier ?? undefined,
        sub2api_test_model: next.test_model ?? undefined,
      }
    }
    appStore.showSuccess(t('admin.sub2apiProviders.settingsSaved'))
  } catch (e: any) {
    appStore.showError(extractErrorMessage(e, t('admin.sub2apiProviders.settingsSaveFailed')))
  } finally {
    savingSettingsId.value = null
  }
}

// 切换「是否参与定时优化」。
// 开启参与的前提:倍率上限、倍率下限、测试模型三者必须已填写;未填则提示先设置。
// 三个字段的值始终保留，仅切换 enabled。
const handleToggleParticipate = async (acc: LinkedAccountInfo) => {
  const enabling = !acc.sub2api_optimize_enabled
  if (enabling) {
    // 校验:开启参与时,上限、下限、测试模型必填
    const err = getIncompleteConfigError(acc, t)
    if (err) {
      appStore.showError(err)
      return
    }
  }
  await saveAccountOptimizeSettings(acc, {
    enabled: enabling,
    min_multiplier: acc.sub2api_min_multiplier ?? null,
    max_multiplier: acc.sub2api_max_multiplier ?? null,
    test_model: acc.sub2api_test_model ?? null,
  })
}

// 更新账号的「倍率上限」（非法值还原显示），参与状态保持不变
const handleUpdateMaxMultiplier = async (acc: LinkedAccountInfo, raw: string) => {
  const validation = validateMaxMultiplier(raw, acc.sub2api_min_multiplier, t)
  if (!validation.valid) {
    appStore.showError(validation.error!)
    panelLinkedAccounts.value = [...panelLinkedAccounts.value] // 还原输入框
    return
  }
  await saveAccountOptimizeSettings(acc, {
    enabled: acc.sub2api_optimize_enabled ?? false,
    min_multiplier: acc.sub2api_min_multiplier ?? null,
    max_multiplier: validation.value!,
    test_model: acc.sub2api_test_model ?? null,
  })
}

// 更新账号的「倍率下限」。空=清除下限（传 null，从最便宜候选开始）；
// 非空须 ≥0 且不超过已设的倍率上限，否则候选区间为空。参与状态保持不变。
const handleUpdateMinMultiplier = async (acc: LinkedAccountInfo, raw: string) => {
  const validation = validateMinMultiplier(raw, acc.sub2api_max_multiplier, t)
  if (!validation.valid) {
    appStore.showError(validation.error!)
    panelLinkedAccounts.value = [...panelLinkedAccounts.value] // 还原输入框
    return
  }
  await saveAccountOptimizeSettings(acc, {
    enabled: acc.sub2api_optimize_enabled ?? false,
    min_multiplier: validation.value ?? null,
    max_multiplier: acc.sub2api_max_multiplier ?? null,
    test_model: acc.sub2api_test_model ?? null,
  })
}

// 更新账号的「测试模型」（空=按平台默认，回写 null），参与状态保持不变
const handleUpdateTestModel = async (acc: LinkedAccountInfo, raw: string) => {
  const trimmed = raw.trim()
  await saveAccountOptimizeSettings(acc, {
    enabled: acc.sub2api_optimize_enabled ?? false,
    min_multiplier: acc.sub2api_min_multiplier ?? null,
    max_multiplier: acc.sub2api_max_multiplier ?? null,
    test_model: trimmed === '' ? null : trimmed,
  })
}

// 懒加载账号可用模型（下拉框聚焦时触发，已加载则跳过）
const loadAccountModels = async (acc: LinkedAccountInfo) => {
  if (accountModels[acc.id] || loadingModelsId.value === acc.id) return
  loadingModelsId.value = acc.id
  try {
    accountModels[acc.id] = await adminAPI.accounts.getAvailableModels(acc.id)
  } catch {
    accountModels[acc.id] = []
  } finally {
    loadingModelsId.value = null
  }
}

// ==================== 🧪 账号模型测试（复用 AccountTestModal）====================
const modelTestModal = reactive<{ show: boolean; account: Account | null }>({ show: false, account: null })

// LinkedAccountInfo → 最小 Account（AccountTestModal 仅用到 id/name/platform/type/status）
const openModelTest = (acc: LinkedAccountInfo) => {
  modelTestModal.account = {
    id: acc.id,
    name: acc.name,
    platform: acc.platform,
    type: (acc as any).type ?? 'apikey',
    status: acc.status,
  } as unknown as Account
  modelTestModal.show = true
}

const openAccountsPanel = async (row: Sub2APIProvider) => {
  accountsPanelProvider.value = row
  showAccountsPanel.value = true
  // 打开面板时实时同步上游当前分组
  await refreshLinkedAccounts(row.id, true)
}

const refreshLinkedAccounts = async (providerId: number, sync = false) => {
  loadingLinked.value = true
  try {
    // 专用接口，直接返回关联到该 Provider 的账号（含远端分组信息）
    // sync=true 时后端会实时登录上游拉取当前分组
    panelLinkedAccounts.value = await adminAPI.sub2apiProviders.getLinkedAccounts(providerId, sync)
  } catch {
    panelLinkedAccounts.value = []
  } finally { loadingLinked.value = false }
}

const handleOptimizeAccount = async (provider: Sub2APIProvider, account: any) => {
  // 校验:手动优化前必须填写上限、下限、测试模型
  const err = getIncompleteConfigError(account, t)
  if (err) {
    appStore.showError(err)
    return
  }
  optimizingAccountId.value = account.id
  try {
    const r = await adminAPI.sub2apiProviders.optimizeAccount(provider.id, account.id)
    if (r.status === 'optimized') {
      appStore.showSuccess(t('admin.sub2apiProviders.optimizeSuccess', { old: r.old_group ?? '-', new: r.new_group ?? '-', multiplier: r.new_multiplier ?? '-' }))
    } else if (r.status === 'skipped') {
      appStore.showInfo(r.reason || t('admin.sub2apiProviders.alreadyOptimal'))
    } else {
      appStore.showError(r.reason || t('admin.sub2apiProviders.optimizeFailed'))
    }
    await refreshLinkedAccounts(provider.id)
  } catch (e:any) {
    appStore.showError(extractErrorMessage(e, t('admin.sub2apiProviders.optimizeFailed')))
  }
  finally { optimizingAccountId.value = null }
}

const handleUnlinkAccount = async (provider: Sub2APIProvider, account: any) => {
  unlinkingAccountId.value = account.id
  try {
    await adminAPI.sub2apiProviders.unlinkAccount(provider.id, account.id)
    appStore.showSuccess(t('admin.sub2apiProviders.unlinked'))
    await refreshLinkedAccounts(provider.id)
  } catch (e:any) {
    appStore.showError(extractErrorMessage(e, t('admin.sub2apiProviders.unlinkFailed')))
  }
  finally { unlinkingAccountId.value = null }
}

// ==================== 🔗 关联账号对话框 ====================
const showLinkDialog = ref(false)
const currentProvider = ref<Sub2APIProvider|null>(null)
const allAccounts = ref<any[]>([])
// 专门给关联对话框用的已关联账号 ID 集合，与"查看面板"独立
const linkDialogLinkedIds = ref<Set<number>>(new Set())
const selectedAccountId = ref<number|string>('')
const linking = ref(false)
const loadingAccounts = ref(false)

const availableAccountOptions = computed(() => {
  return allAccounts.value
    .filter((a: any) => !linkDialogLinkedIds.value.has(a.id) && a.status === 'active')
    .map((a: any) => ({
      value: a.id,
      label: `${a.name} [${a.platform}]`
    }))
})

const openLinkDialog = async (row: Sub2APIProvider) => {
  currentProvider.value = row
  // 重置状态，防止残留上次数据
  allAccounts.value = []
  linkDialogLinkedIds.value = new Set()
  selectedAccountId.value = ''
  showLinkDialog.value = true
  loadingAccounts.value = true

  try {
    // 并行加载：全部账号 + 已关联账号（用专用接口）
    const [accountsRes, linkedList] = await Promise.all([
      adminAPI.accounts.list(1, 500),
      adminAPI.sub2apiProviders.getLinkedAccounts(row.id)
    ])
    allAccounts.value = accountsRes.items
    linkDialogLinkedIds.value = new Set(linkedList.map((a) => a.id))
    // 同时更新查看面板里的关联账号数据
    panelLinkedAccounts.value = linkedList
  } catch {
    allAccounts.value = []
    appStore.showError('加载账号列表失败')
  } finally {
    loadingAccounts.value = false
  }
}

const closeLinkDialog = () => { showLinkDialog.value = false; selectedAccountId.value = '' }

const handleLinkAccount = async () => {
  if (!currentProvider.value || !selectedAccountId.value) return
  linking.value = true
  try {
    await adminAPI.sub2apiProviders.linkAccount(currentProvider.value.id, Number(selectedAccountId.value))
    appStore.showSuccess(t('admin.sub2apiProviders.linked'))
    selectedAccountId.value = ''
    // 关联后立即同步分组信息（sync=true），让新账号即刻显示远端分组
    const linkedList = await adminAPI.sub2apiProviders.getLinkedAccounts(currentProvider.value.id, true)
    panelLinkedAccounts.value = linkedList
    linkDialogLinkedIds.value = new Set(linkedList.map((a) => a.id))
    // 关闭关联对话框，直接打开账号面板展示结果（数据已是最新，无需重复 sync）
    const provider = currentProvider.value
    closeLinkDialog()
    accountsPanelProvider.value = provider
    showAccountsPanel.value = true
  } catch (e:any) {
    appStore.showError(extractErrorMessage(e, t('admin.sub2apiProviders.linkFailed')))
  }
  finally { linking.value = false }
}

// ==================== ⚡ 批量优化 ====================
const optimizingAllId = ref<number|null>(null)
const showOptimizeResultDialog = ref(false)
const optimizeAllResult = ref<OptimizeAllResult|null>(null)

const handleOptimizeAll = async (row: Sub2APIProvider) => {
  // 校验:批量优化前,检查该 provider 下所有账号是否都填写了上限、下限、测试模型
  if (showAccountsPanel.value && accountsPanelProvider.value?.id === row.id && panelLinkedAccounts.value.length > 0) {
    const incomplete = findIncompleteAccounts(panelLinkedAccounts.value)
    if (incomplete.length > 0) {
      const names = incomplete.map(a => a.name).join('、')
      appStore.showError(t('admin.sub2apiProviders.optimizeAllIncomplete', { accounts: names }))
      return
    }
  }
  optimizingAllId.value = row.id
  try {
    const r = await adminAPI.sub2apiProviders.optimizeAll(row.id)
    optimizeAllResult.value = r
    showOptimizeResultDialog.value = true
    // 刷新面板内账号信息
    if (showAccountsPanel.value && accountsPanelProvider.value?.id === row.id) {
      await refreshLinkedAccounts(row.id)
    }
  } catch (e:any) {
    appStore.showError(extractErrorMessage(e, t('admin.sub2apiProviders.optimizeFailed')))
  }
  finally { optimizingAllId.value = null }
}

// ==================== 初始化 ====================
onMounted(() => loadProviders())
</script>

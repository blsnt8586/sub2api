<template>
  <AppLayout>
    <div class="flex min-h-0 flex-col gap-6">
      <!-- 筛选栏 -->
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
            <button @click="loadProviders" :disabled="loading" class="btn btn-secondary" :title="t('admin.sub2apiProviders.refresh')" :aria-label="t('admin.sub2apiProviders.refresh')">
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </button>
            <button @click="openCreateDialog" class="btn btn-primary">
              <Icon name="plus" size="md" class="mr-1" />
              {{ t('admin.sub2apiProviders.createProvider') }}
            </button>
          </div>
      </div>

      <div
        v-if="!loading && providers.length"
        class="grid grid-cols-3 gap-2 rounded-lg border border-gray-200 bg-white p-2 shadow-sm dark:border-dark-700 dark:bg-dark-800 sm:grid-cols-6"
        :aria-label="t('admin.sub2apiProviders.health.summaryTitle')"
        role="group"
      >
        <button
          v-for="item in providerHealthSummaryItems"
          :key="item.value"
          type="button"
          class="flex min-h-12 cursor-pointer items-center gap-2 rounded-md px-2.5 text-left transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500"
          :class="healthFilter === item.value ? item.activeClass : 'hover:bg-gray-50 dark:hover:bg-dark-700/60'"
          @click="healthFilter = item.value"
        >
          <span class="h-2 w-2 flex-shrink-0 rounded-full" :class="item.dotClass"></span>
          <span class="min-w-0">
            <span class="block truncate text-[10px] text-gray-500 dark:text-dark-400">{{ item.label }}</span>
            <span class="block text-base font-semibold tabular-nums" :class="item.textClass">{{ item.count }}</span>
          </span>
        </button>
      </div>

      <!-- 上游面板 -->
      <div v-if="loading" class="grid grid-cols-1 gap-4 lg:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4" aria-busy="true" :aria-label="t('common.loading')">
        <div
          v-for="index in 3"
          :key="index"
          class="min-h-[340px] rounded-lg border border-gray-200 bg-white p-4 shadow-sm motion-safe:animate-pulse dark:border-dark-700 dark:bg-dark-800"
        >
          <div class="h-5 w-2/5 rounded bg-gray-200 dark:bg-dark-600"></div>
          <div class="mt-2 h-4 w-3/5 rounded bg-gray-100 dark:bg-dark-700"></div>
          <div class="mt-7 h-4 w-2/5 rounded bg-gray-100 dark:bg-dark-700"></div>
          <div class="mt-3 h-4 rounded-sm bg-gray-100 dark:bg-dark-700"></div>
          <div class="mt-2 h-3 w-4/5 rounded bg-gray-100 dark:bg-dark-700"></div>
          <div class="mt-6 h-4 w-1/2 rounded bg-gray-100 dark:bg-dark-700"></div>
          <div class="mt-7 h-11 rounded bg-gray-100 dark:bg-dark-700"></div>
        </div>
      </div>

      <div v-else-if="displayedProviders.length > 0" class="grid grid-cols-1 items-stretch gap-4 lg:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
        <Sub2APIProviderCard
          v-for="(provider, index) in displayedProviders"
          :key="provider.id"
          :provider="provider"
          :overview="providerHealthOverview[provider.id]"
          :remote-overview="providerRemoteOverviews[provider.id]"
          :remote-overview-loading="providerRemoteOverviewLoading[provider.id]"
          :remote-overview-error="providerRemoteOverviewErrors[provider.id]"
          :animation-index="index"
          :now-tick="relativeTimeTick"
          @view-accounts="openAccountsPanel(provider)"
          @view-health="openProbeDialog(provider)"
          @view-logs="openProbeLogs(provider)"
          @view-remote-overview="openRemoteOverview(provider)"
          @more="openActionMenu(provider, $event)"
        />
      </div>

      <div v-else-if="providers.length > 0" class="rounded-lg border border-dashed border-gray-200 bg-white py-10 text-center dark:border-dark-700 dark:bg-dark-800">
        <p class="text-sm font-medium text-gray-700 dark:text-dark-200">{{ t('admin.sub2apiProviders.health.noFilterResults') }}</p>
        <button type="button" class="mt-2 cursor-pointer text-xs font-medium text-primary-600 hover:underline dark:text-primary-400" @click="healthFilter = 'all'">
          {{ t('admin.sub2apiProviders.health.clearHealthFilter') }}
        </button>
      </div>

      <div v-else class="rounded-lg border border-dashed border-gray-200 bg-white py-8 dark:border-dark-700 dark:bg-dark-800">
        <EmptyState
          :title="t('admin.sub2apiProviders.noProviders')"
          :description="t('admin.sub2apiProviders.noProvidersDesc')"
          :action-text="t('admin.sub2apiProviders.createProvider')"
          @action="openCreateDialog"
        />
      </div>

      <!-- 分页 -->
      <Pagination
        v-if="pagination.total > 0"
        :page="pagination.page"
        :total="pagination.total"
        :page-size="pagination.page_size"
        @update:page="handlePageChange"
        @update:pageSize="handlePageSizeChange"
      />
    </div>

    <BaseDialog
      :show="showRemoteOverviewDialog"
      :title="`${remoteOverviewProvider?.name || ''} · ${t('admin.sub2apiProviders.remoteOverview.dialogTitle')}`"
      width="wide"
      @close="showRemoteOverviewDialog = false"
    >
      <div class="min-h-52">
        <div
          v-if="remoteOverviewProvider && providerRemoteOverviewLoading[remoteOverviewProvider.id] && !remoteDialogHasSnapshot"
          class="flex min-h-52 items-center justify-center gap-2 text-sm text-gray-500 dark:text-dark-300"
          aria-live="polite"
        >
          <Icon name="refresh" size="md" class="animate-spin text-blue-500" />
          {{ t('admin.sub2apiProviders.remoteOverview.loading') }}
        </div>

        <div
          v-else-if="remoteDialogError && !remoteDialogHasSnapshot"
          class="flex min-h-52 flex-col items-center justify-center rounded-md border border-red-200 bg-red-50 px-6 text-center dark:border-red-800 dark:bg-red-900/10"
          role="alert"
        >
          <Icon name="exclamationCircle" size="lg" class="text-red-500" />
          <p class="mt-3 text-sm font-semibold text-red-700 dark:text-red-300">{{ t('admin.sub2apiProviders.remoteOverview.loadFailed') }}</p>
          <p class="mt-1 max-w-xl text-sm leading-6 text-red-600 dark:text-red-400">{{ remoteDialogError }}</p>
          <button type="button" class="btn btn-secondary mt-4 min-h-11" @click="refreshRemoteOverview()">
            <Icon name="refresh" size="sm" class="mr-1.5" />
            {{ t('admin.sub2apiProviders.remoteOverview.retry') }}
          </button>
        </div>

        <div v-else-if="remoteDialogHasSnapshot && remoteDialogOverview" class="space-y-4">
          <div
            v-if="remoteDialogError"
            class="flex flex-col items-stretch gap-3 rounded-md border border-amber-200 bg-amber-50 px-3 py-2.5 text-sm text-amber-800 sm:flex-row sm:items-start sm:justify-between dark:border-amber-800 dark:bg-amber-900/10 dark:text-amber-300"
            role="status"
          >
            <span class="flex min-w-0 items-start gap-2">
              <Icon name="exclamationTriangle" size="sm" class="mt-0.5 flex-shrink-0" />
              <span class="min-w-0">
                <strong class="block font-semibold">{{ t('admin.sub2apiProviders.remoteOverview.staleSnapshot') }}</strong>
                <span class="mt-0.5 block break-words text-xs leading-5">{{ remoteDialogError }}</span>
              </span>
            </span>
            <button type="button" class="btn btn-secondary min-h-11 flex-shrink-0 sm:self-start" @click="refreshRemoteOverview()">
              <Icon name="refresh" size="sm" class="mr-1.5" />
              {{ t('admin.sub2apiProviders.remoteOverview.retry') }}
            </button>
          </div>

          <div
            v-else-if="remoteOverviewProvider && providerRemoteOverviewLoading[remoteOverviewProvider.id]"
            class="flex items-center gap-2 rounded-md border border-blue-200 bg-blue-50 px-3 py-2.5 text-sm text-blue-700 dark:border-blue-800 dark:bg-blue-900/10 dark:text-blue-300"
            aria-live="polite"
          >
            <Icon name="refresh" size="sm" class="animate-spin" />
            {{ t('admin.sub2apiProviders.remoteOverview.refreshingWithSnapshot') }}
          </div>
          <div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
            <div class="rounded-md border border-gray-200 bg-gray-50 px-4 py-3 dark:border-dark-700 dark:bg-dark-900/30">
              <span class="flex items-center gap-2 text-sm text-gray-600 dark:text-dark-300">
                <Icon name="creditCard" size="sm" class="text-blue-500" />
                {{ t('admin.sub2apiProviders.remoteOverview.balance') }}
              </span>
              <strong class="mt-1 block text-2xl font-semibold tabular-nums text-gray-900 dark:text-white">{{ formatRemoteNumber(remoteDialogOverview.balance) }}</strong>
            </div>
            <div class="rounded-md border border-gray-200 bg-gray-50 px-4 py-3 dark:border-dark-700 dark:bg-dark-900/30">
              <span class="flex items-center gap-2 text-sm text-gray-600 dark:text-dark-300">
                <Icon name="database" size="sm" class="text-indigo-500" />
                {{ t('admin.sub2apiProviders.remoteOverview.visibleGroups') }}
              </span>
              <strong class="mt-1 block text-2xl font-semibold tabular-nums text-gray-900 dark:text-white">{{ remoteDialogOverview.groups.length }}</strong>
            </div>
            <div class="rounded-md border border-gray-200 bg-gray-50 px-4 py-3 dark:border-dark-700 dark:bg-dark-900/30">
              <span class="flex items-center gap-2 text-sm text-gray-600 dark:text-dark-300">
                <Icon name="chart" size="sm" class="text-amber-500" />
                {{ t('admin.sub2apiProviders.remoteOverview.customRates') }}
              </span>
              <strong class="mt-1 block text-2xl font-semibold tabular-nums text-gray-900 dark:text-white">{{ remoteDialogCustomRateCount }}</strong>
            </div>
          </div>

          <div
            v-if="!remoteDialogOverview.rate_overrides_available"
            class="flex items-start gap-2 rounded-md border border-amber-200 bg-amber-50 px-3 py-2.5 text-sm leading-6 text-amber-800 dark:border-amber-800 dark:bg-amber-900/10 dark:text-amber-300"
          >
            <Icon name="exclamationTriangle" size="sm" class="mt-1 flex-shrink-0" />
            {{ t('admin.sub2apiProviders.remoteOverview.rateFallback') }}
          </div>

          <div class="overflow-hidden rounded-md border border-gray-200 dark:border-dark-700">
            <div class="flex items-center justify-between gap-3 border-b border-gray-200 bg-gray-50 px-4 py-2.5 dark:border-dark-700 dark:bg-dark-900/30">
              <h3 class="text-sm font-semibold text-gray-800 dark:text-dark-100">{{ t('admin.sub2apiProviders.remoteOverview.groupRates') }}</h3>
              <span class="text-xs tabular-nums text-gray-500 dark:text-dark-400">{{ formatDateTime(remoteDialogOverview.sampled_at) }}</span>
            </div>
            <div v-if="remoteDialogOverview.groups.length" class="max-h-[48vh] divide-y divide-gray-100 overflow-y-auto dark:divide-dark-700">
              <div v-for="group in remoteDialogOverview.groups" :key="group.id" class="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-4 px-4 py-3">
                <div class="min-w-0">
                  <div class="flex min-w-0 flex-wrap items-center gap-2">
                    <span class="truncate text-sm font-medium text-gray-900 dark:text-white" :title="group.name">{{ group.name }}</span>
                    <span v-if="group.platform" class="rounded border border-gray-200 px-1.5 py-0.5 text-xs text-gray-600 dark:border-dark-600 dark:text-dark-300">{{ group.platform }}</span>
                    <span v-if="group.has_custom_rate" class="rounded bg-blue-50 px-1.5 py-0.5 text-xs font-medium text-blue-700 dark:bg-blue-900/20 dark:text-blue-300">{{ t('admin.sub2apiProviders.remoteOverview.customRate') }}</span>
                  </div>
                  <p v-if="group.description" class="mt-1 truncate text-xs text-gray-500 dark:text-dark-400" :title="group.description">{{ group.description }}</p>
                </div>
                <div class="text-right tabular-nums">
                  <strong class="block text-sm text-gray-900 dark:text-white">×{{ formatRemoteMultiplier(group.effective_multiplier) }}</strong>
                  <span v-if="group.has_custom_rate" class="mt-0.5 block text-xs text-gray-500 line-through dark:text-dark-400">×{{ formatRemoteMultiplier(group.default_multiplier) }}</span>
                  <span v-else class="mt-0.5 block text-xs text-gray-500 dark:text-dark-400">{{ t('admin.sub2apiProviders.remoteOverview.defaultRate') }}</span>
                </div>
              </div>
            </div>
            <div v-else class="px-4 py-10 text-center text-sm text-gray-500 dark:text-dark-400">{{ t('admin.sub2apiProviders.remoteOverview.noGroups') }}</div>
          </div>
        </div>
      </div>

      <template #footer>
        <div class="flex w-full flex-col items-stretch gap-3 sm:flex-row sm:items-center sm:justify-between">
          <span v-if="remoteDialogHasSnapshot && remoteDialogOverview" class="min-w-0 text-xs leading-5 text-gray-500 dark:text-dark-400">
            {{ t('admin.sub2apiProviders.remoteOverview.snapshotMeta', { source: remoteDialogSourceLabel, time: formatDateTime(remoteDialogOverview.sampled_at) }) }}
            <span class="block sm:inline">{{ t('admin.sub2apiProviders.remoteOverview.cacheHint') }}</span>
          </span>
          <span v-else></span>
          <div class="grid gap-2 sm:flex sm:flex-shrink-0 sm:items-center" :class="remoteDialogHasSnapshot ? 'grid-cols-2' : 'grid-cols-1'">
            <button type="button" class="btn btn-secondary min-h-11 w-full whitespace-nowrap sm:w-auto" @click="showRemoteOverviewDialog = false">{{ t('common.close') }}</button>
            <button
              v-if="remoteDialogHasSnapshot"
              type="button"
              class="btn btn-primary min-h-11 w-full whitespace-nowrap sm:w-auto"
              :disabled="!!(remoteOverviewProvider && providerRemoteOverviewLoading[remoteOverviewProvider.id])"
              @click="refreshRemoteOverview()"
            >
              <Icon name="refresh" size="sm" class="mr-1.5" />
              {{ t('admin.sub2apiProviders.remoteOverview.refresh') }}
            </button>
          </div>
        </div>
      </template>
    </BaseDialog>

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
        <div v-if="accountsPanelProvider && !accountsPanelProvider.api_path_keys" class="flex items-start gap-2 rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-700 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-400">
          <Icon name="exclamationCircle" size="sm" class="mt-0.5 flex-shrink-0" />
          <span>{{ t('admin.sub2apiProviders.pathsNotDetectedAccountHint') }}</span>
        </div>

        <!-- 加载中 -->
        <div v-if="loadingLinked" class="flex items-center justify-center py-8 text-gray-400">
          <Icon name="refresh" size="md" class="animate-spin mr-2" />
          加载中...
        </div>

        <!-- 账户列表区域（loading 结束后始终显示） -->
        <div v-else>
          <!-- 始终显示的头部工具栏 -->
          <div class="mb-3 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <span class="text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.sub2apiProviders.linkedAccounts') }} ({{ panelLinkedAccounts.length }})
            </span>
            <div class="grid grid-cols-2 gap-2 sm:flex sm:items-center">
              <button
                v-if="accountsPanelProvider"
                type="button"
                class="btn btn-secondary min-h-11 min-w-0 px-2 text-xs"
                @click="openProbeFromAccountsPanel"
              >
                <Icon name="shield" size="sm" class="mr-1" />
                {{ t('admin.sub2apiProviders.health.settings') }}
              </button>
              <!-- 关联账号（始终显示） -->
              <button
                v-if="accountsPanelProvider"
                @click="openLinkDialog(accountsPanelProvider)"
                class="btn btn-primary min-h-11 min-w-0 px-2 text-xs"
              >
                <Icon name="link" size="sm" class="mr-1" />
                {{ t('admin.sub2apiProviders.linkAccount') }}
              </button>
              <!-- 刷新分组 -->
              <button
                v-if="accountsPanelProvider && panelLinkedAccounts.length > 0"
                @click="refreshLinkedAccounts(accountsPanelProvider.id, true)"
                :disabled="loadingLinked"
                class="btn btn-secondary min-h-11 min-w-0 px-2 text-xs"
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
                class="btn btn-secondary min-h-11 min-w-0 px-2 text-xs"
              >
                <Icon :name="optimizingAllId === accountsPanelProvider?.id ? 'refresh' : 'bolt'" size="sm" class="mr-1" :class="optimizingAllId === accountsPanelProvider?.id ? 'animate-spin' : ''" />
                {{ t('admin.sub2apiProviders.optimizeAll') }}
              </button>
            </div>
          </div>

          <div v-if="panelLinkedAccounts.length > 0">
            <div class="space-y-2 md:hidden">
              <article
                v-for="acc in panelLinkedAccounts"
                :key="`mobile-${acc.id}`"
                class="overflow-hidden rounded-md border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800"
              >
                <button
                  type="button"
                  class="flex min-h-14 w-full cursor-pointer items-start gap-3 px-3 py-3 text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 focus-visible:ring-inset"
                  :aria-expanded="mobileExpandedAccountID === acc.id"
                  @click="mobileExpandedAccountID = mobileExpandedAccountID === acc.id ? null : acc.id"
                >
                  <span class="mt-1.5 h-2 w-2 flex-shrink-0 rounded-full" :class="probeStatusDotClass(panelAccountProbeStatus(acc))"></span>
                  <span class="min-w-0 flex-1">
                    <span class="flex min-w-0 items-center gap-2">
                      <span class="min-w-0 flex-1 truncate text-sm font-medium text-gray-900 dark:text-white">{{ acc.name }}</span>
                      <span :class="['badge flex-shrink-0 text-[10px]', acc.platform === 'anthropic' ? 'badge-warning' : acc.platform === 'openai' ? 'badge-success' : 'badge-info']">{{ acc.platform }}</span>
                      <span
                        v-if="acc.remote_group_multiplier != null"
                        class="inline-flex items-center gap-0.5 rounded border px-1.5 text-[10px] font-semibold tabular-nums"
                        :class="accountMultiplierBadgeClass(acc)"
                        :title="accountMultiplierTitle(acc)"
                        :aria-label="accountMultiplierTitle(acc)"
                      ><Icon v-if="accountMultiplierOutOfRange(acc)" name="exclamationCircle" size="xs" class="flex-shrink-0" />×{{ acc.remote_group_multiplier }}</span>
                    </span>
                    <span class="mt-1 flex min-w-0 items-center gap-2 text-[11px] text-gray-500 dark:text-dark-400">
                      <span class="min-w-0 flex-1 truncate">{{ acc.remote_group_name || t('admin.sub2apiProviders.health.routes.unboundGroup') }}</span>
                      <span :class="probeStatusTextClass(panelAccountProbeStatus(acc))">{{ probeAccountStatusLabel(panelAccountProbeStatus(acc), panelAccountProbeIsSelected(acc)) }}</span>
                    </span>
                  </span>
                  <Icon name="chevronDown" size="sm" class="mt-1 flex-shrink-0 text-gray-400 transition-transform" :class="mobileExpandedAccountID === acc.id ? 'rotate-180' : ''" />
                </button>

                <div v-if="mobileExpandedAccountID === acc.id" class="border-t border-gray-100 px-3 py-3 dark:border-dark-700">
                  <div class="grid grid-cols-2 gap-3">
                    <label class="text-xs text-gray-500 dark:text-dark-400">
                      {{ t('admin.sub2apiProviders.minMultiplier') }}
                      <input
                        type="number"
                        step="0.1"
                        min="0"
                        :value="acc.sub2api_min_multiplier ?? ''"
                        :disabled="savingSettingsId === acc.id"
                        class="input mt-1 min-h-11"
                        @change="handleUpdateMinMultiplier(acc, ($event.target as HTMLInputElement).value)"
                      />
                    </label>
                    <label class="text-xs text-gray-500 dark:text-dark-400">
                      {{ t('admin.sub2apiProviders.maxMultiplier') }}
                      <input
                        type="number"
                        step="0.1"
                        min="0"
                        :value="acc.sub2api_max_multiplier ?? ''"
                        :disabled="savingSettingsId === acc.id"
                        class="input mt-1 min-h-11"
                        @change="handleUpdateMaxMultiplier(acc, ($event.target as HTMLInputElement).value)"
                      />
                    </label>
                    <label class="col-span-2 text-xs text-gray-500 dark:text-dark-400">
                      {{ t('admin.sub2apiProviders.testModel') }}
                      <select
                        :value="acc.sub2api_test_model ?? ''"
                        :disabled="savingSettingsId === acc.id"
                        class="input mt-1 min-h-11"
                        @focus="loadAccountModels(acc)"
                        @change="handleUpdateTestModel(acc, ($event.target as HTMLSelectElement).value)"
                      >
                        <option value="" :disabled="acc.sub2api_optimize_enabled">{{ t('admin.sub2apiProviders.selectTestModel') }}</option>
                        <option v-if="acc.sub2api_test_model && !(accountModels[acc.id] || []).some(m => m.id === acc.sub2api_test_model)" :value="acc.sub2api_test_model">{{ acc.sub2api_test_model }}</option>
                        <option v-for="m in accountModels[acc.id] || []" :key="m.id" :value="m.id">{{ m.id }}</option>
                      </select>
                    </label>
                  </div>

                  <div class="mt-3 flex min-h-11 items-center justify-between border-y border-gray-100 py-2 dark:border-dark-700">
                    <div>
                      <p class="text-xs font-medium text-gray-700 dark:text-dark-200">{{ t('admin.sub2apiProviders.joinSchedule') }}</p>
                      <p class="mt-0.5 text-[11px] text-gray-400 dark:text-dark-500">{{ acc.sub2api_optimize_enabled ? t('admin.sub2apiProviders.joinScheduleOn') : t('admin.sub2apiProviders.joinScheduleOff') }}</p>
                    </div>
                    <button
                      type="button"
                      class="relative inline-flex h-7 w-12 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 disabled:opacity-40"
                      :class="acc.sub2api_optimize_enabled ? 'bg-blue-600' : 'bg-gray-200 dark:bg-dark-600'"
                      :disabled="savingSettingsId === acc.id"
                      @click="handleToggleParticipate(acc)"
                    >
                      <span class="pointer-events-none inline-block h-6 w-6 rounded-full bg-white shadow transition-transform" :class="acc.sub2api_optimize_enabled ? 'translate-x-5' : 'translate-x-0'" />
                    </button>
                  </div>

                  <div class="mt-3 grid grid-cols-3 gap-2">
                    <button type="button" class="btn btn-secondary min-h-11 min-w-0 px-1 text-[11px]" @click="openModelTest(acc)">
                      <Icon name="play" size="sm" class="mr-1" />{{ t('admin.sub2apiProviders.testModel') }}
                    </button>
                    <button
                      type="button"
                      class="btn btn-secondary min-h-11 min-w-0 px-1 text-[11px]"
                      :disabled="optimizingAccountId === acc.id || !acc.sub2api_optimize_enabled"
                      :title="acc.sub2api_optimize_enabled ? t('admin.sub2apiProviders.optimizeAccount') : t('admin.sub2apiProviders.optimizeNotEnabled')"
                      @click="handleOptimizeAccount(accountsPanelProvider!, acc)"
                    >
                      <Icon :name="optimizingAccountId === acc.id ? 'refresh' : 'bolt'" size="sm" class="mr-1" :class="optimizingAccountId === acc.id ? 'animate-spin' : ''" />{{ t('admin.sub2apiProviders.optimizeAccount') }}
                    </button>
                    <button type="button" class="btn min-h-11 min-w-0 border-red-200 px-1 text-[11px] text-red-600 hover:bg-red-50 dark:border-red-800 dark:text-red-400 dark:hover:bg-red-900/20" :disabled="unlinkingAccountId === acc.id" @click="handleUnlinkAccount(accountsPanelProvider!, acc)">
                      <Icon :name="unlinkingAccountId === acc.id ? 'refresh' : 'x'" size="sm" class="mr-1" :class="unlinkingAccountId === acc.id ? 'animate-spin' : ''" />{{ t('admin.sub2apiProviders.unlinkAccount') }}
                    </button>
                  </div>
                </div>
              </article>
            </div>

            <div class="hidden overflow-x-auto md:block">
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
                <span class="mt-1 inline-flex items-center gap-1 text-[11px]" :class="probeStatusTextClass(panelAccountProbeStatus(acc))">
                  <span class="h-1.5 w-1.5 rounded-full" :class="probeStatusDotClass(panelAccountProbeStatus(acc))"></span>
                  {{ probeAccountStatusLabel(panelAccountProbeStatus(acc), panelAccountProbeIsSelected(acc)) }}
                  <span v-if="panelAccountProbe(acc)?.latency_ms != null" class="tabular-nums text-gray-400 dark:text-dark-500">· {{ panelAccountProbe(acc)?.latency_ms }} ms</span>
                </span>
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
                  :class="['inline-flex items-center gap-0.5 rounded-full border px-2 py-0.5 text-xs font-bold font-mono', accountMultiplierBadgeClass(acc)]"
                  :title="accountMultiplierTitle(acc)"
                  :aria-label="accountMultiplierTitle(acc)"
                ><Icon v-if="accountMultiplierOutOfRange(acc)" name="exclamationCircle" size="xs" class="flex-shrink-0" />×{{ acc.remote_group_multiplier }}</span>
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
                  <option value="" :disabled="acc.sub2api_optimize_enabled">{{ t('admin.sub2apiProviders.selectTestModel') }}</option>
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
                  :disabled="optimizingAccountId === acc.id || !acc.sub2api_optimize_enabled"
                  class="rounded p-1.5 text-orange-500 hover:bg-orange-50 dark:hover:bg-orange-900/20 disabled:opacity-40 transition-colors"
                  :title="acc.sub2api_optimize_enabled ? t('admin.sub2apiProviders.optimizeAccount') : t('admin.sub2apiProviders.optimizeNotEnabled')"
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
	          <p class="input-hint">{{ t('admin.sub2apiProviders.form.baseUrlHint') }}</p>
	        </div>

	        <div>
	          <label class="input-label" for="provider-proxy">{{ t('admin.sub2apiProviders.form.proxy') }}</label>
	          <Select
	            id="provider-proxy"
	            v-model="form.proxy_id"
	            :options="providerProxyOptions"
	            :disabled="loadingProviderProxies"
	            :aria-label="t('admin.sub2apiProviders.form.proxy')"
	            aria-describedby="provider-proxy-hint"
	            searchable="auto"
	          />
	          <p id="provider-proxy-hint" class="input-hint">
	            {{ loadingProviderProxies ? t('admin.sub2apiProviders.form.proxyLoading') : t('admin.sub2apiProviders.form.proxyHint') }}
	          </p>
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

        <div>
          <label class="input-label">{{ t('admin.sub2apiProviders.form.authMode') }}</label>
          <div class="grid grid-cols-1 gap-2 md:grid-cols-2" role="radiogroup" :aria-label="t('admin.sub2apiProviders.form.authMode')">
            <button
              v-for="option in authModeOptions"
              :key="option.value"
              type="button"
              role="radio"
              :aria-checked="form.auth_method === option.value"
              class="flex min-h-12 cursor-pointer items-center gap-3 rounded-md border px-3 text-left transition-colors duration-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500"
              :class="form.auth_method === option.value
                ? 'border-primary-400 bg-primary-50 text-primary-800 dark:border-primary-600 dark:bg-primary-900/20 dark:text-primary-200'
                : 'border-gray-200 bg-white text-gray-600 hover:border-gray-300 hover:bg-gray-50 dark:border-dark-600 dark:bg-dark-800 dark:text-dark-300 dark:hover:bg-dark-700'"
              @click="selectAuthMethod(option.value)"
            >
              <span class="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-md" :class="form.auth_method === option.value ? 'bg-primary-100 text-primary-700 dark:bg-primary-900/40 dark:text-primary-300' : 'bg-gray-100 text-gray-500 dark:bg-dark-700 dark:text-dark-300'">
                <Icon :name="option.icon" size="sm" />
              </span>
              <span class="min-w-0">
                <span class="flex flex-wrap items-center gap-1.5 text-sm font-semibold">
                  {{ option.label }}
                  <span v-if="option.recommended" class="rounded border border-primary-200 bg-primary-100 px-1.5 py-0.5 text-[10px] font-semibold text-primary-700 dark:border-primary-800 dark:bg-primary-900/40 dark:text-primary-300">{{ t('admin.sub2apiProviders.form.recommended') }}</span>
                </span>
                <span class="block text-xs leading-4 opacity-75">{{ option.description }}</span>
              </span>
            </button>
          </div>
        </div>

        <div>
          <label class="input-label">{{ t('admin.sub2apiProviders.form.email') }}</label>
          <input v-model="form.email" type="email" autocomplete="username" class="input" :placeholder="t('admin.sub2apiProviders.form.emailPlaceholder')" required />
          <p v-if="form.auth_method === 'token_pair'" class="input-hint">{{ t('admin.sub2apiProviders.form.emailTokenHint') }}</p>
        </div>

        <div v-if="form.auth_method === 'password'">
          <label class="input-label">{{ t('admin.sub2apiProviders.form.password') }}</label>
          <input
            v-model="form.password"
            type="password"
            autocomplete="new-password"
            class="input"
            :placeholder="isEditing && originalAuthMode === 'password' ? t('admin.sub2apiProviders.form.keepSecretPlaceholder') : t('admin.sub2apiProviders.form.passwordPlaceholder')"
            :required="!isEditing || originalAuthMode !== 'password'"
          />
        </div>

        <div v-else-if="form.auth_method === 'token_pair'" class="space-y-3 rounded-md border border-gray-200 bg-gray-50 p-3 dark:border-dark-600 dark:bg-dark-900/30">
          <div class="flex flex-wrap items-start justify-between gap-2">
            <div class="flex min-w-0 items-start gap-2">
              <Icon name="shield" size="sm" class="mt-0.5 flex-shrink-0 text-primary-600 dark:text-primary-400" />
              <p class="text-xs leading-5 text-gray-600 dark:text-dark-300">{{ t('admin.sub2apiProviders.form.tokenPairHint') }}</p>
            </div>
            <div v-if="isEditing && originalAuthMode === 'token_pair'" class="flex flex-shrink-0 items-center gap-1.5 text-[11px]">
              <span class="rounded border px-1.5 py-0.5" :class="editingTokenStatus.hasAccess ? 'border-green-200 bg-green-50 text-green-700 dark:border-green-800 dark:bg-green-900/20 dark:text-green-300' : 'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-300'">
                AT {{ editingTokenStatus.hasAccess ? t('admin.sub2apiProviders.form.configured') : t('admin.sub2apiProviders.form.missing') }}
              </span>
              <span class="rounded border px-1.5 py-0.5" :class="editingTokenStatus.hasRefresh ? 'border-green-200 bg-green-50 text-green-700 dark:border-green-800 dark:bg-green-900/20 dark:text-green-300' : 'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-300'">
                RT {{ editingTokenStatus.hasRefresh ? t('admin.sub2apiProviders.form.configured') : t('admin.sub2apiProviders.form.missing') }}
              </span>
            </div>
          </div>
          <Sub2APICredentialBundleImport
            :key="credentialImportKey"
            :expected-base-url="form.base_url"
            @imported="handleCredentialBundleImported"
          />
          <div>
            <label class="input-label">{{ t('admin.sub2apiProviders.form.accessToken') }}</label>
            <input v-model="form.access_token" type="password" autocomplete="off" spellcheck="false" class="input font-mono text-xs" :placeholder="tokenInputPlaceholder" :required="tokenPairRequired" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.sub2apiProviders.form.refreshToken') }}</label>
            <input v-model="form.refresh_token" type="password" autocomplete="off" spellcheck="false" class="input font-mono text-xs" :placeholder="tokenInputPlaceholder" :required="tokenPairRequired" />
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
        ? `${t('admin.sub2apiProviders.deleteConfirmMessage', { name: deletingProvider?.name ?? '' })}\n\n${t('admin.sub2apiProviders.deleteLinkedAccountsWarning', { count: deletingProvider?.accounts_count ?? 0 })}`
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
      :toggling="togglingId === actionMenu.provider?.id"
      :testing="testingId === actionMenu.provider?.id"
      @close="actionMenu.show = false"
      @edit="openEditDialog"
      @toggle-status="handleToggleStatus"
      @detect-paths="handleDetectPaths"
      @test-connection="handleTestConnection"
      @probe-settings="openProbeDialog"
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

    <BaseDialog
      :show="showProbeDialog"
      :title="`${probeProvider?.name || ''} — ${t('admin.sub2apiProviders.health.title')}`"
      width="wide"
      @close="closeProbeDialog"
    >
      <div v-if="loadingProbeDialog" class="flex min-h-48 items-center justify-center text-gray-400 dark:text-dark-400">
        <Icon name="refresh" size="md" class="mr-2 animate-spin" />
        {{ t('common.loading') }}
      </div>
      <div v-else>
        <div
          v-if="probeProvider?.status === 'inactive'"
          class="mb-5 flex items-start gap-3 rounded-md border border-amber-200 bg-amber-50 px-3 py-2.5 text-sm text-amber-800 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-300"
          role="status"
        >
          <Icon name="shield" size="sm" class="mt-0.5 flex-shrink-0" />
          <div>
            <p class="font-medium">{{ t('admin.sub2apiProviders.health.probePaused') }}</p>
            <p class="mt-1 text-sm leading-5 text-amber-700 dark:text-amber-400">{{ t('admin.sub2apiProviders.health.probePausedHint') }}</p>
          </div>
        </div>
        <section class="pb-5">
          <div class="flex flex-wrap items-center justify-between gap-3">
            <div class="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1">
              <Icon name="shield" size="md" :class="probeStatusTextClass(probeDialogHealth?.control_status)" />
              <span class="text-base font-semibold text-gray-900 dark:text-white">
                {{ t('admin.sub2apiProviders.health.controlConnection') }} · {{ t(`admin.sub2apiProviders.health.status.${probeDialogHealth?.control_status || 'unknown'}`) }}
              </span>
              <span class="text-sm text-gray-500 dark:text-dark-400">
                {{ probeDialogHealth?.last_checked_at ? formatDateTime(probeDialogHealth.last_checked_at) : t('admin.sub2apiProviders.health.neverChecked') }}
              </span>
            </div>
            <button class="btn btn-secondary" :disabled="probingId === probeProvider?.id || probeProvider?.status === 'inactive'" @click="probeProvider && handleRunControlProbe(probeProvider)">
              <Icon :name="probingId === probeProvider?.id ? 'refresh' : 'play'" size="sm" class="mr-1" :class="probingId === probeProvider?.id ? 'animate-spin' : ''" />
              {{ t('admin.sub2apiProviders.health.runControlNow') }}
            </button>
          </div>
          <div class="mt-4 flex flex-wrap items-center gap-2 border-y border-gray-100 py-3 text-sm dark:border-dark-700">
            <span class="inline-flex items-center gap-1.5 text-gray-600 dark:text-dark-300">
              <span class="h-1.5 w-1.5 rounded-full" :class="probeProvider?.status === 'active' && probeForm.control_enabled ? 'bg-green-500' : 'bg-gray-300 dark:bg-dark-500'"></span>
              {{ probeProvider?.status === 'inactive' ? t('admin.sub2apiProviders.health.probePaused') : t('admin.sub2apiProviders.health.controlCadence', { interval: formatProbeInterval(probeForm.control_interval_seconds) }) }}
            </span>
            <span class="text-gray-300 dark:text-dark-400" aria-hidden="true">/</span>
            <span class="inline-flex items-center gap-1.5 text-gray-600 dark:text-dark-300">
              <span class="h-1.5 w-1.5 rounded-full" :class="enabledProbeTargetCount ? 'bg-blue-500' : 'bg-gray-300 dark:bg-dark-500'"></span>
              {{ t('admin.sub2apiProviders.health.routes.activeCount', { count: enabledProbeTargetCount }) }}
            </span>
          </div>
          <div v-if="currentControlHasAnomaly" class="mt-3 flex items-center gap-2 text-sm text-amber-700 dark:text-amber-400">
            <Icon name="exclamationCircle" size="sm" />
            {{ t('admin.sub2apiProviders.health.controlAnomalySummary') }}
          </div>
          <div class="mt-4 border-y border-gray-100 py-4 dark:border-dark-700">
            <p class="text-sm font-semibold text-gray-700 dark:text-dark-200">{{ t('admin.sub2apiProviders.health.controlDetails') }}</p>
            <dl class="mt-2 grid grid-cols-2 gap-x-4 gap-y-3 sm:grid-cols-4">
              <div><dt class="text-sm text-gray-500 dark:text-dark-400">{{ t('admin.sub2apiProviders.health.loginStage') }}</dt><dd class="mt-1 text-sm font-medium text-gray-700 dark:text-dark-200">{{ formatProbeStage(probeDialogHealth?.login_latency_ms) }}</dd></div>
              <div><dt class="text-sm text-gray-500 dark:text-dark-400">/health</dt><dd class="mt-1 text-sm font-medium text-gray-700 dark:text-dark-200">{{ formatProbeStage(probeDialogHealth?.health_latency_ms) }}</dd></div>
              <div><dt class="text-sm text-gray-500 dark:text-dark-400">Keys API</dt><dd class="mt-1 text-sm font-medium text-gray-700 dark:text-dark-200">{{ formatProbeStage(probeDialogHealth?.keys_latency_ms, probeDetailNumber('key_count')) }}</dd></div>
              <div><dt class="text-sm text-gray-500 dark:text-dark-400">Groups API</dt><dd class="mt-1 text-sm font-medium text-gray-700 dark:text-dark-200">{{ formatProbeStage(probeDialogHealth?.groups_latency_ms, probeDetailNumber('group_count')) }}</dd></div>
            </dl>
          </div>
        </section>

        <section class="border-t border-gray-100 py-5 dark:border-dark-700">
          <div class="mb-4">
            <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.sub2apiProviders.health.connectionSettingsTitle') }}</h3>
            <p class="mt-1 max-w-3xl text-sm leading-5 text-gray-500 dark:text-dark-400">{{ t('admin.sub2apiProviders.health.connectionSettingsDescription') }}</p>
          </div>
          <div class="grid gap-4 md:max-w-3xl md:grid-cols-2">
            <label class="flex min-h-11 items-center gap-3 md:col-span-2">
              <input v-model="probeForm.control_enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
              <span><span class="block text-sm font-semibold text-gray-800 dark:text-dark-100">{{ t('admin.sub2apiProviders.health.controlEnabled') }}</span><span class="mt-1 block text-sm leading-5 text-gray-500 dark:text-dark-400">{{ t('admin.sub2apiProviders.health.controlHint') }}</span></span>
            </label>
            <label class="text-sm font-medium text-gray-700 dark:text-dark-200"><span>{{ t('admin.sub2apiProviders.health.controlInterval') }}</span><span class="mt-1 block text-xs font-normal text-gray-500 dark:text-dark-400">{{ t('admin.sub2apiProviders.health.controlIntervalHint') }}</span><input v-model.number="probeForm.control_interval_seconds" type="number" min="60" max="86400" class="input mt-2 min-h-11" /></label>
            <label class="text-sm font-medium text-gray-700 dark:text-dark-200"><span>{{ t('admin.sub2apiProviders.health.controlTimeout') }}</span><span class="mt-1 block text-xs font-normal text-gray-500 dark:text-dark-400">{{ t('admin.sub2apiProviders.health.controlTimeoutHint') }}</span><input v-model.number="probeForm.timeout_seconds" type="number" min="3" max="120" class="input mt-2 min-h-11" /></label>
          </div>
        </section>

        <Sub2APIProviderRouteMonitor
          :routes="probeTargets"
          :history-by-target="probeTargetHistory"
          :running-target-id="runningProbeTargetID"
          :dirty-target-ids="pendingProbeTargetIDs"
          :now-tick="relativeTimeTick"
          @update="handleUpdateProbeTarget"
          @run="handleRunProbeTarget"
          @history="loadProbeTargetHistory"
        />
      </div>
      <template #footer>
        <div class="flex w-full items-center justify-between gap-3">
          <span v-if="pendingProbeTargetIDs.length" class="text-xs font-medium text-amber-600 dark:text-amber-400">
            {{ t('admin.sub2apiProviders.health.routes.unsavedCount', { count: pendingProbeTargetIDs.length }) }}
          </span>
          <span v-else></span>
          <div class="flex items-center gap-3">
            <button class="btn btn-secondary" @click="closeProbeDialog">{{ t('common.cancel') }}</button>
            <button class="btn btn-primary" :disabled="savingProbeConfig || loadingProbeDialog" @click="saveProbeConfig">
              <Icon v-if="savingProbeConfig" name="refresh" size="sm" class="mr-1 animate-spin" />
              {{ t('common.save') }}
            </button>
          </div>
        </div>
      </template>
    </BaseDialog>

    <BaseDialog
      :show="showProbeLogsDialog"
      :title="`${probeLogsProvider?.name || ''} — ${t('admin.sub2apiProviders.health.logs.title')}`"
      width="wide"
      @close="closeProbeLogs"
    >
      <div v-if="loadingProbeLogs" class="flex min-h-56 items-center justify-center text-gray-400 dark:text-dark-400">
        <Icon name="refresh" size="md" class="mr-2 animate-spin" />
        {{ t('common.loading') }}
      </div>
      <template v-else>
        <div class="mb-3 flex items-center justify-between gap-3">
          <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.sub2apiProviders.health.logs.windowHint') }}</p>
          <button type="button" class="btn btn-secondary" :disabled="loadingProbeLogs" @click="reloadProbeLogs()">
            <Icon name="refresh" size="sm" class="mr-1" />
            {{ t('admin.sub2apiProviders.refresh') }}
          </button>
        </div>
        <Sub2APIProviderLogs
          :control-history="probeLogsControlHistory"
          :routes="probeLogsTargets"
          :route-history="probeLogsTargetHistory"
          :optimization-logs="probeLogsOptimizationHistory"
        />
      </template>
    </BaseDialog>

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
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { adminAPI } from '@/api/admin'
import type {
  ProviderAccountProbeStatus,
  Sub2APIProvider,
  Sub2APIProviderHealth,
  Sub2APIProviderHealthOverview,
  Sub2APIProviderProbeConfig,
  Sub2APIProviderProbeTargetHealth,
  Sub2APIProviderRemoteOverview,
  UpdateProviderProbeTargetRequest,
  OptimizeAllResult,
  OptimizeLogInfo,
  LinkedAccountInfo,
} from '@/api/admin/sub2apiProviders'
import type { Account, ClaudeModel, Proxy } from '@/types'
import {
  findIncompleteParticipatingAccounts,
  getIncompleteConfigError,
  getMultiplierRangeState,
  validateMaxMultiplier,
  validateMinMultiplier,
} from '@/utils/sub2apiValidation'
import { applyOptimizeResultToAccounts } from '@/utils/sub2apiOptimization'
import { extractErrorMessage } from '@/utils/errorHandler'
import { formatDateTime } from '@/utils/format'
import type { Sub2APICredentialBundle } from '@/utils/sub2apiCredentialBundle'

import AppLayout from '@/components/layout/AppLayout.vue'
import Pagination from '@/components/common/Pagination.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Sub2APIProviderCard from '@/components/admin/Sub2APIProviderCard.vue'
import Sub2APIProviderLogs from '@/components/admin/Sub2APIProviderLogs.vue'
import Sub2APIProviderRouteMonitor from '@/components/admin/Sub2APIProviderRouteMonitor.vue'
import Sub2APICredentialBundleImport from '@/components/admin/Sub2APICredentialBundleImport.vue'
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
const providerProxies = ref<Proxy[]>([])
const loadingProviderProxies = ref(false)
const relativeTimeTick = ref(Date.now())
const providerHealth = reactive<Record<number, Sub2APIProviderHealth | null>>({})
const providerHealthOverview = reactive<Record<number, Sub2APIProviderHealthOverview | null>>({})
const providerRemoteOverviews = reactive<Record<number, Sub2APIProviderRemoteOverview | null>>({})
const providerRemoteOverviewLoading = reactive<Record<number, boolean>>({})
const providerRemoteOverviewErrors = reactive<Record<number, string | null>>({})
const showRemoteOverviewDialog = ref(false)
const remoteOverviewProvider = ref<Sub2APIProvider | null>(null)
const loading = ref(false)
const searchQuery = ref('')
const filters = reactive({ status: '' })
type ProviderHealthFilter = 'all' | 'unhealthy' | 'degraded' | 'unknown' | 'healthy' | 'paused'
const healthFilter = ref<ProviderHealthFilter>('all')
const pagination = reactive({ page: 1, page_size: getPersistedPageSize(), total: 0, pages: 0 })

const remoteDialogOverview = computed(() => remoteOverviewProvider.value
  ? providerRemoteOverviews[remoteOverviewProvider.value.id] ?? null
  : null)
const remoteDialogHasSnapshot = computed(() => remoteDialogOverview.value?.available === true)
const remoteDialogError = computed(() => {
  const providerID = remoteOverviewProvider.value?.id
  if (!providerID) return null
  return providerRemoteOverviewErrors[providerID] || remoteDialogOverview.value?.last_error || null
})
const remoteDialogSourceLabel = computed(() => remoteDialogOverview.value?.source === 'control_probe'
  ? t('admin.sub2apiProviders.remoteOverview.sources.controlProbe')
  : t('admin.sub2apiProviders.remoteOverview.sources.manual'))
const remoteDialogCustomRateCount = computed(() => remoteDialogOverview.value?.groups.filter(group => group.has_custom_rate).length ?? 0)
const formatRemoteNumber = (value: number) => new Intl.NumberFormat(undefined, {
  minimumFractionDigits: 0,
  maximumFractionDigits: 2,
}).format(value)
const formatRemoteMultiplier = (value: number) => Number.isInteger(value)
  ? value.toFixed(0)
  : String(Number(value.toFixed(4)))

const refreshRemoteOverview = async (provider = remoteOverviewProvider.value) => {
  if (!provider || providerRemoteOverviewLoading[provider.id]) return
  providerRemoteOverviewLoading[provider.id] = true
  providerRemoteOverviewErrors[provider.id] = null
  try {
    mergeRemoteOverview(await adminAPI.sub2apiProviders.getRemoteOverview(provider.id))
  } catch (error) {
    providerRemoteOverviewErrors[provider.id] = extractErrorMessage(
      error,
      t('admin.sub2apiProviders.remoteOverview.loadFailed')
    )
    await loadCachedRemoteOverviews([provider.id])
  } finally {
    providerRemoteOverviewLoading[provider.id] = false
  }
}

const openRemoteOverview = (provider: Sub2APIProvider) => {
  remoteOverviewProvider.value = provider
  showRemoteOverviewDialog.value = true
  if (providerRemoteOverviews[provider.id]?.available !== true) {
    void refreshRemoteOverview(provider)
  }
}

const providerOperationalStatus = (provider: Sub2APIProvider): Exclude<ProviderHealthFilter, 'all'> => {
  if (provider.status === 'inactive') return 'paused'
  return providerHealthOverview[provider.id]?.availability_status ?? 'unknown'
}

const providerHealthSummaryItems = computed(() => {
  const count = (status?: Exclude<ProviderHealthFilter, 'all'>) => status
    ? providers.value.filter(provider => providerOperationalStatus(provider) === status).length
    : providers.value.length
  return [
    { value: 'all' as const, label: t('admin.sub2apiProviders.health.summary.all'), count: count(), dotClass: 'bg-blue-500', textClass: 'text-gray-900 dark:text-white', activeClass: 'bg-blue-50 ring-1 ring-blue-200 dark:bg-blue-900/20 dark:ring-blue-800' },
    { value: 'unhealthy' as const, label: t('admin.sub2apiProviders.health.summary.unhealthy'), count: count('unhealthy'), dotClass: 'bg-red-500', textClass: 'text-red-600 dark:text-red-400', activeClass: 'bg-red-50 ring-1 ring-red-200 dark:bg-red-900/20 dark:ring-red-800' },
    { value: 'degraded' as const, label: t('admin.sub2apiProviders.health.summary.degraded'), count: count('degraded'), dotClass: 'bg-amber-400', textClass: 'text-amber-600 dark:text-amber-400', activeClass: 'bg-amber-50 ring-1 ring-amber-200 dark:bg-amber-900/20 dark:ring-amber-800' },
    { value: 'unknown' as const, label: t('admin.sub2apiProviders.health.summary.unknown'), count: count('unknown'), dotClass: 'bg-gray-300 dark:bg-dark-500', textClass: 'text-gray-600 dark:text-dark-300', activeClass: 'bg-gray-100 ring-1 ring-gray-200 dark:bg-dark-700 dark:ring-dark-600' },
    { value: 'healthy' as const, label: t('admin.sub2apiProviders.health.summary.healthy'), count: count('healthy'), dotClass: 'bg-green-500', textClass: 'text-green-600 dark:text-green-400', activeClass: 'bg-green-50 ring-1 ring-green-200 dark:bg-green-900/20 dark:ring-green-800' },
    { value: 'paused' as const, label: t('admin.sub2apiProviders.health.summary.paused'), count: count('paused'), dotClass: 'bg-slate-400', textClass: 'text-gray-600 dark:text-dark-300', activeClass: 'bg-gray-100 ring-1 ring-gray-200 dark:bg-dark-700 dark:ring-dark-600' },
  ]
})

const providerHealthSeverity: Record<Exclude<ProviderHealthFilter, 'all'>, number> = {
  unhealthy: 5,
  degraded: 4,
  unknown: 3,
  healthy: 2,
  paused: 1,
}

const displayedProviders = computed(() => providers.value
  .filter(provider => healthFilter.value === 'all' || providerOperationalStatus(provider) === healthFilter.value)
  .slice()
  .sort((a, b) => providerHealthSeverity[providerOperationalStatus(b)] - providerHealthSeverity[providerOperationalStatus(a)] || a.name.localeCompare(b.name)))

const statusFilterOptions = computed(() => [
  { value: '', label: t('admin.sub2apiProviders.allStatus') },
  { value: 'active', label: t('admin.sub2apiProviders.statusLabels.active') },
  { value: 'inactive', label: t('admin.sub2apiProviders.statusLabels.inactive') }
])

const statusOptions = computed(() => [
  { value: 'active', label: t('admin.sub2apiProviders.statusLabels.active') },
  { value: 'inactive', label: t('admin.sub2apiProviders.statusLabels.inactive') }
])

const providerProxyOptions = computed(() => [
  { value: null, label: t('admin.sub2apiProviders.form.proxyDirect') },
  ...providerProxies.value.map(proxy => ({
    value: proxy.id,
    label: `${proxy.name} · ${proxy.protocol}://${proxy.host}:${proxy.port}`,
  })),
])

const loadProviderProxies = async () => {
  if (loadingProviderProxies.value) return
  loadingProviderProxies.value = true
  try {
    providerProxies.value = await adminAPI.proxies.getAll()
  } catch (error) {
    providerProxies.value = []
    appStore.showError(extractErrorMessage(error, t('admin.sub2apiProviders.form.proxyLoadFailed')))
  } finally {
    loadingProviderProxies.value = false
  }
}

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
    const providerIDs = result.items.map(provider => provider.id)
    await Promise.all([
      loadHealthOverviews(providerIDs),
      loadCachedRemoteOverviews(providerIDs),
    ])
  } catch {
    appStore.showError(t('admin.sub2apiProviders.loadFailed'))
  } finally { loading.value = false }
}

const HEALTH_REFRESH_INTERVAL_MS = 15_000
let healthRefreshTimer: ReturnType<typeof setInterval> | null = null
let relativeTimeTimer: ReturnType<typeof setInterval> | null = null
let healthRefreshInFlight = false
let healthRequestSequence = 0
const latestHealthRequestByProvider = new Map<number, number>()

const remoteOverviewVersion = (overview: Sub2APIProviderRemoteOverview | null | undefined) => Math.max(
  Date.parse(overview?.last_attempted_at || '') || 0,
  Date.parse(overview?.sampled_at || '') || 0
)

const mergeRemoteOverview = (overview: Sub2APIProviderRemoteOverview) => {
  const current = providerRemoteOverviews[overview.provider_id]
  if (!current || remoteOverviewVersion(overview) >= remoteOverviewVersion(current)) {
    providerRemoteOverviews[overview.provider_id] = overview
    providerRemoteOverviewErrors[overview.provider_id] = null
  }
}

const loadCachedRemoteOverviews = async (providerIDs: number[]) => {
  if (providerIDs.length === 0) return
  try {
    const overviews = await adminAPI.sub2apiProviders.getCachedRemoteOverviews(providerIDs)
    overviews.forEach(mergeRemoteOverview)
  } catch {
    // Cache polling is supplemental. Preserve the current screen and health status.
  }
}

const loadHealthOverviews = async (providerIds: number[], showError = true, preserveHealth = false) => {
  if (providerIds.length === 0) return
  const requestTokens = new Map(providerIds.map(providerId => {
    const token = ++healthRequestSequence
    latestHealthRequestByProvider.set(providerId, token)
    return [providerId, token] as const
  }))
  try {
    const overviews = await adminAPI.sub2apiProviders.getHealthOverview(providerIds)
    const overviewByProvider = new Map(overviews.map(overview => [overview.provider_id, overview]))
    for (const providerId of providerIds) {
      if (latestHealthRequestByProvider.get(providerId) !== requestTokens.get(providerId)) continue
      const overview = overviewByProvider.get(providerId) ?? null
      providerHealthOverview[providerId] = overview
      providerHealth[providerId] = overview?.latest ?? null
      if (overview && showAccountsPanel.value && accountsPanelProvider.value?.id === providerId) {
        accountsPanelProbeTargets.value = overview.routes
      }
      if (overview && showProbeDialog.value && probeProvider.value?.id === providerId) {
        probeDialogHealth.value = overview.latest ?? probeDialogHealth.value
        probeTargets.value = overview.routes.map(target => ({
          ...target,
          ...(pendingProbeTargetUpdates[target.id] ?? {}),
        }))
      }
    }
  } catch {
    for (const providerId of providerIds) {
      if (latestHealthRequestByProvider.get(providerId) !== requestTokens.get(providerId)) continue
      if (!preserveHealth) {
        providerHealthOverview[providerId] = null
        providerHealth[providerId] = null
      }
    }
    if (showError) appStore.showError(t('admin.sub2apiProviders.health.overviewLoadFailed'))
  }
}

const refreshHealthSilently = async () => {
  if (document.hidden || healthRefreshInFlight || providers.value.length === 0) return
  healthRefreshInFlight = true
  try {
    const providerIDs = providers.value.map(provider => provider.id)
    await Promise.all([
      loadHealthOverviews(providerIDs, false, true),
      loadCachedRemoteOverviews(providerIDs),
    ])
  } finally {
    healthRefreshInFlight = false
  }
}

const handleHealthVisibilityChange = () => {
  if (!document.hidden) void refreshHealthSilently()
}

const startHealthPolling = () => {
  if (healthRefreshTimer) return
  document.addEventListener('visibilitychange', handleHealthVisibilityChange)
  healthRefreshTimer = setInterval(() => { void refreshHealthSilently() }, HEALTH_REFRESH_INTERVAL_MS)
  relativeTimeTimer = setInterval(() => { relativeTimeTick.value = Date.now() }, 30_000)
}

const stopHealthPolling = () => {
  if (healthRefreshTimer) {
    clearInterval(healthRefreshTimer)
    healthRefreshTimer = null
  }
  if (relativeTimeTimer) {
    clearInterval(relativeTimeTimer)
    relativeTimeTimer = null
  }
  document.removeEventListener('visibilitychange', handleHealthVisibilityChange)
}

const probingId = ref<number | null>(null)
const handleRunControlProbe = async (row: Sub2APIProvider) => {
  probingId.value = row.id
  try {
    const health = await adminAPI.sub2apiProviders.runProbe(row.id)
    providerHealth[row.id] = health
    await Promise.all([
      loadHealthOverviews([row.id], false, true),
      loadCachedRemoteOverviews([row.id]),
    ])
    if (probeProvider.value?.id === row.id) {
      probeDialogHealth.value = health
    }
    if (probeLogsProvider.value?.id === row.id) await reloadProbeLogs()
    appStore.showSuccess(t('admin.sub2apiProviders.health.controlRunSuccess'))
  } catch (e: any) {
    appStore.showError(extractErrorMessage(e, t('admin.sub2apiProviders.health.controlRunFailed')))
  } finally {
    probingId.value = null
  }
}

const handlePageChange = (page: number) => { pagination.page = page; loadProviders() }
const handlePageSizeChange = (size: number) => { pagination.page_size = size; pagination.page = 1; loadProviders() }

// ==================== 创建/编辑 ====================
const showEditDialog = ref(false)
const isEditing = ref(false)
const editingId = ref<number | null>(null)
const saving = ref(false)
type ProviderAuthMode = 'password' | 'token_pair'
const form = reactive({
  name: '', base_url: '', provider_type: 'sub2api', email: '', password: '',
  auth_method: 'token_pair' as ProviderAuthMode, auth_mode: 'token_pair' as ProviderAuthMode, access_token: '', refresh_token: '',
  status: 'active' as 'active'|'inactive', notes: '', proxy_id: null as number | null,
})
const originalProxyID = ref<number | null>(null)
const originalAuthMode = ref<ProviderAuthMode>('token_pair')
const editingTokenStatus = reactive({ hasAccess: false, hasRefresh: false })
const authModeOptions = computed(() => [
  { value: 'token_pair' as const, icon: 'key' as const, label: t('admin.sub2apiProviders.form.authTokenPair'), description: t('admin.sub2apiProviders.form.authTokenPairDesc'), recommended: true },
  { value: 'password' as const, icon: 'lock' as const, label: t('admin.sub2apiProviders.form.authPassword'), description: t('admin.sub2apiProviders.form.authPasswordDesc'), recommended: false },
])
const tokenPairRequired = computed(() => !isEditing.value || originalAuthMode.value !== 'token_pair' || !editingTokenStatus.hasAccess || !editingTokenStatus.hasRefresh)
const tokenInputPlaceholder = computed(() => tokenPairRequired.value ? t('admin.sub2apiProviders.form.tokenRequiredPlaceholder') : t('admin.sub2apiProviders.form.keepSecretPlaceholder'))
const credentialImportKey = ref(0)

const selectAuthMethod = (method: ProviderAuthMode) => {
  if (form.auth_method === method) return
  form.auth_method = method
  form.auth_mode = method
}

const handleCredentialBundleImported = (bundle: Sub2APICredentialBundle) => {
  form.auth_method = 'token_pair'
  form.auth_mode = 'token_pair'
  form.password = ''
  form.access_token = bundle.accessToken
  form.refresh_token = bundle.refreshToken
  if (!form.base_url.trim()) form.base_url = bundle.sourceOrigin
  if (bundle.email) form.email = bundle.email
}

const resetForm = () => {
  credentialImportKey.value += 1
  Object.assign(form, { name:'', base_url:'', provider_type:'sub2api', email:'', password:'', auth_method:'token_pair', auth_mode:'token_pair', access_token:'', refresh_token:'', status:'active', notes:'', proxy_id:null })
  originalProxyID.value = null
  originalAuthMode.value = 'token_pair'
  Object.assign(editingTokenStatus, { hasAccess: false, hasRefresh: false })
}

const openCreateDialog = () => { isEditing.value=false; editingId.value=null; resetForm(); showEditDialog.value=true; void loadProviderProxies() }
const openEditDialog = (row: Sub2APIProvider) => {
  credentialImportKey.value += 1
  isEditing.value=true; editingId.value=row.id
  const authMode: ProviderAuthMode = row.auth_mode || 'password'
  originalAuthMode.value = authMode
  originalProxyID.value = row.proxy_id ?? null
  Object.assign(editingTokenStatus, { hasAccess: row.has_access_token, hasRefresh: row.has_refresh_token })
  Object.assign(form, { name:row.name, base_url:row.base_url, provider_type:row.provider_type||'sub2api', email:row.email, password:'', auth_method:authMode, auth_mode:authMode, access_token:'', refresh_token:'', status:row.status, notes:row.notes??'', proxy_id:row.proxy_id??null })
  showEditDialog.value=true
  void loadProviderProxies()
}
const closeEditDialog = () => {
  credentialImportKey.value += 1
  showEditDialog.value=false
}

const handleSave = async () => {
  saving.value=true
  try {
    if (isEditing.value && editingId.value) {
      const normalizedNotes = form.notes.trim()
      const payload: Record<string,unknown> = { name:form.name, base_url:form.base_url, email:form.email, auth_mode:form.auth_mode, status:form.status, notes:normalizedNotes }
	  if (form.proxy_id !== originalProxyID.value) payload.proxy_id = form.proxy_id
      if (form.auth_mode === 'password' && form.password) payload.password = form.password
      if (form.auth_mode === 'token_pair' && form.access_token) payload.access_token = form.access_token
      if (form.auth_mode === 'token_pair' && form.refresh_token) payload.refresh_token = form.refresh_token
      const updated = await adminAPI.sub2apiProviders.update(editingId.value, payload as any)
      // 局部替换该行，不刷整列表
      updateProviderInList(editingId.value, { ...updated, notes: normalizedNotes || null })
      appStore.showSuccess(t('admin.sub2apiProviders.updateSuccess'))
      closeEditDialog()
    } else {
      const created = await adminAPI.sub2apiProviders.create({
        name:form.name, base_url:form.base_url, provider_type:form.provider_type,
        email:form.email, auth_mode:form.auth_mode,
	    proxy_id:form.proxy_id,
        ...(form.auth_mode === 'password' ? { password:form.password } : { access_token:form.access_token, refresh_token:form.refresh_token }),
        notes:form.notes||null
      })
      appStore.showSuccess(t('admin.sub2apiProviders.createSuccess'))
      closeEditDialog()
      // 新建：追加到列表头部，刷新总数
      providers.value = [created, ...providers.value]
      pagination.total += 1
      void loadHealthOverviews([created.id], false, true)
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
      // Provider update/test APIs do not include the eager-loaded account count.
      // Preserve the list value here; explicit link/unlink flows update it below.
      accounts_count: current.accounts_count,
    }
  }
}

const updateProviderAccountCount = (id: number, count: number) => {
  const idx = providers.value.findIndex(p => p.id === id)
  if (idx !== -1) {
    providers.value[idx] = { ...providers.value[idx], accounts_count: Math.max(0, count) }
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
    await loadHealthOverviews([row.id], false, true)
  } catch (e:any) {
    appStore.showError(extractErrorMessage(e, t('admin.sub2apiProviders.connectionFailed')))
    // 失败时也局部刷新，更新 last_sync_status=failed
    adminAPI.sub2apiProviders.getById(row.id)
      .then(async res => {
        updateProviderInList(row.id, res.provider)
        await loadHealthOverviews([row.id], false, true)
      })
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
  const menuWidth = 192
  const menuHeight = 380
  const viewportPadding = 8
  const left = Math.min(
    Math.max(viewportPadding, btn.right - menuWidth),
    window.innerWidth - menuWidth - viewportPadding
  )
  const belowTop = btn.bottom + 4
  const top = belowTop + menuHeight <= window.innerHeight - viewportPadding
    ? belowTop
    : Math.max(viewportPadding, btn.top - menuHeight - 4)
  actionMenu.position = { top, left }
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

// ==================== Provider 健康探针 ====================
const showProbeDialog = ref(false)
const loadingProbeDialog = ref(false)
const savingProbeConfig = ref(false)
const probeProvider = ref<Sub2APIProvider | null>(null)
const probeDialogHealth = ref<Sub2APIProviderHealth | null>(null)
const probeTargets = ref<Sub2APIProviderProbeTargetHealth[]>([])
const probeTargetHistory = reactive<Record<number, Sub2APIProviderProbeTargetHealth[]>>({})
const pendingProbeTargetUpdates = reactive<Record<number, UpdateProviderProbeTargetRequest>>({})
const runningProbeTargetID = ref<number | null>(null)
const probeForm = reactive({
  control_enabled: true,
  control_interval_seconds: 1800,
  timeout_seconds: 15,
  degraded_latency_ms: 2000,
})

const applyProbeConfig = (config: Sub2APIProviderProbeConfig) => {
  Object.assign(probeForm, {
    control_enabled: config.control_enabled,
    control_interval_seconds: config.control_interval_seconds,
    timeout_seconds: config.timeout_seconds,
    degraded_latency_ms: config.degraded_latency_ms,
  })
}

const probeDetailNumber = (key: string) => {
  const value = probeDialogHealth.value?.details?.[key]
  return typeof value === 'number' && Number.isFinite(value) ? value : null
}

const formatProbeStage = (latency?: number | null, count?: number | null) => {
  const parts: string[] = []
  if (count != null) parts.push(t('admin.sub2apiProviders.health.itemCount', { count }))
  if (latency != null) parts.push(`${latency} ms`)
  return parts.length ? parts.join(' · ') : '—'
}

const formatProbeInterval = (seconds: number) => {
  if (seconds >= 3600 && seconds % 3600 === 0) return t('admin.sub2apiProviders.health.intervalHours', { count: seconds / 3600 })
  if (seconds >= 60 && seconds % 60 === 0) return t('admin.sub2apiProviders.health.intervalMinutes', { count: seconds / 60 })
  return t('admin.sub2apiProviders.health.intervalSeconds', { count: seconds })
}

const probeStatusDotClass = (status?: ProviderAccountProbeStatus | null) => ({
  healthy: 'bg-green-500',
  degraded: 'bg-amber-400',
  unhealthy: 'bg-red-500',
  unknown: 'border border-gray-300 bg-transparent dark:border-dark-500',
  disabled: 'bg-gray-300 dark:bg-dark-500',
}[status || 'unknown'])

const enabledProbeTargetCount = computed(() => probeTargets.value.filter(target => target.enabled).length)
const pendingProbeTargetIDs = computed(() => Object.keys(pendingProbeTargetUpdates).map(Number))

const currentControlHasAnomaly = computed(() => {
  const health = probeDialogHealth.value
  if (!health) return false
  if (health.error_message) return true
  return ['health_error', 'keys_error', 'groups_error'].some(key => typeof health.details?.[key] === 'string')
})

const clearPendingProbeTargets = () => {
  for (const targetID of Object.keys(pendingProbeTargetUpdates)) delete pendingProbeTargetUpdates[Number(targetID)]
}

const closeProbeDialog = () => {
  showProbeDialog.value = false
  probeProvider.value = null
  clearPendingProbeTargets()
}

const openProbeDialog = async (row: Sub2APIProvider) => {
  probeProvider.value = row
  showProbeDialog.value = true
  loadingProbeDialog.value = true
  clearPendingProbeTargets()
  try {
    const [config, health, targets] = await Promise.all([
      adminAPI.sub2apiProviders.getProbeConfig(row.id),
      adminAPI.sub2apiProviders.getHealth(row.id),
      adminAPI.sub2apiProviders.getProbeTargets(row.id, true),
    ])
    if (probeProvider.value?.id !== row.id) return
    applyProbeConfig(config)
    probeDialogHealth.value = health
    probeTargets.value = targets
    providerHealth[row.id] = health
    await loadHealthOverviews([row.id], false, true)
  } catch (e: any) {
    appStore.showError(extractErrorMessage(e, t('admin.sub2apiProviders.health.loadFailed')))
  } finally {
    if (probeProvider.value?.id === row.id) loadingProbeDialog.value = false
  }
}

const saveProbeConfig = async () => {
  if (!probeProvider.value) return
  savingProbeConfig.value = true
  try {
    const providerID = probeProvider.value.id
    const [config, ...savedTargets] = await Promise.all([
      adminAPI.sub2apiProviders.updateProbeConfig(providerID, { ...probeForm }),
      ...Object.entries(pendingProbeTargetUpdates).map(([targetID, payload]) =>
        adminAPI.sub2apiProviders.updateProbeTarget(providerID, Number(targetID), payload)
      ),
    ])
    applyProbeConfig(config)
    for (const target of savedTargets) replaceProbeTarget(target)
    clearPendingProbeTargets()
    await loadHealthOverviews([providerID], false, true)
    appStore.showSuccess(t('admin.sub2apiProviders.health.settingsSaved'))
  } catch (e: any) {
    appStore.showError(extractErrorMessage(e, t('admin.sub2apiProviders.health.settingsSaveFailed')))
  } finally {
    savingProbeConfig.value = false
  }
}

const replaceProbeTarget = (target: Sub2APIProviderProbeTargetHealth) => {
  const index = probeTargets.value.findIndex(item => item.id === target.id)
  if (index === -1) probeTargets.value = [...probeTargets.value, target]
  else probeTargets.value.splice(index, 1, target)
}

const handleUpdateProbeTarget = (targetID: number, payload: UpdateProviderProbeTargetRequest) => {
  const target = probeTargets.value.find(item => item.id === targetID)
  if (!target) return
  replaceProbeTarget({ ...target, ...payload })
  pendingProbeTargetUpdates[targetID] = { ...pendingProbeTargetUpdates[targetID], ...payload }
}

const loadProbeTargetHistory = async (targetID: number) => {
  if (!probeProvider.value) return
  try {
    probeTargetHistory[targetID] = await adminAPI.sub2apiProviders.getProbeTargetHistory(probeProvider.value.id, targetID, 100)
  } catch (e: any) {
    appStore.showError(extractErrorMessage(e, t('admin.sub2apiProviders.health.routes.historyLoadFailed')))
  }
}

const handleRunProbeTarget = async (targetID: number) => {
  if (!probeProvider.value || pendingProbeTargetUpdates[targetID]) return
  runningProbeTargetID.value = targetID
  try {
    const target = await adminAPI.sub2apiProviders.runProbeTarget(probeProvider.value.id, targetID)
    replaceProbeTarget(target)
    await Promise.all([
      loadProbeTargetHistory(targetID),
      loadHealthOverviews([probeProvider.value.id], false, true),
    ])
    appStore.showSuccess(t('admin.sub2apiProviders.health.routes.runSuccess'))
  } catch (e: any) {
    appStore.showError(extractErrorMessage(e, t('admin.sub2apiProviders.health.routes.runFailed')))
  } finally {
    runningProbeTargetID.value = null
  }
}

// ==================== 只读诊断日志 ====================
const showProbeLogsDialog = ref(false)
const loadingProbeLogs = ref(false)
const probeLogsProvider = ref<Sub2APIProvider | null>(null)
const probeLogsControlHistory = ref<Sub2APIProviderHealth[]>([])
const probeLogsTargets = ref<Sub2APIProviderProbeTargetHealth[]>([])
const probeLogsTargetHistory = ref<Record<number, Sub2APIProviderProbeTargetHealth[]>>({})
const probeLogsOptimizationHistory = ref<OptimizeLogInfo[]>([])
let probeLogsPollTimer: ReturnType<typeof setInterval> | null = null
let probeLogsRefreshInFlight = false

const loadDailyOptimizeLogs = async (providerID: number, from: string) => {
  const first = await adminAPI.sub2apiProviders.listOptimizeLogs(providerID, {
    from,
    page: 1,
    page_size: 100,
  })
  if (first.pages <= 1) return first.items

  const remaining = await Promise.all(
    Array.from({ length: first.pages - 1 }, (_, index) => adminAPI.sub2apiProviders.listOptimizeLogs(providerID, {
      from,
      page: index + 2,
      page_size: 100,
    }))
  )
  return [first.items, ...remaining.map(page => page.items)].flat()
}

const reloadProbeLogs = async (silent = false) => {
  const provider = probeLogsProvider.value
  if (!provider || probeLogsRefreshInFlight) return
  probeLogsRefreshInFlight = true
  if (!silent) loadingProbeLogs.value = true
  try {
    const from = new Date(Date.now() - 24 * 60 * 60 * 1000).toISOString()
    const [history, targets, optimizationLogs] = await Promise.all([
      adminAPI.sub2apiProviders.getProbeHistory(provider.id, 2000, 86400),
      adminAPI.sub2apiProviders.getProbeTargets(provider.id),
      loadDailyOptimizeLogs(provider.id, from),
    ])
    if (probeLogsProvider.value?.id !== provider.id) return
    probeLogsControlHistory.value = history
    probeLogsTargets.value = targets
    probeLogsOptimizationHistory.value = optimizationLogs

    const routeHistory: Record<number, Sub2APIProviderProbeTargetHealth[]> = {}
    for (let start = 0; start < targets.length; start += 6) {
      const batch = targets.slice(start, start + 6)
      const records = await Promise.all(batch.map(async target => {
        try {
          return [target.id, await adminAPI.sub2apiProviders.getProbeTargetHistory(provider.id, target.id, 2000, 86400)] as const
        } catch {
          return [target.id, [] as Sub2APIProviderProbeTargetHealth[]] as const
        }
      }))
      for (const [targetID, items] of records) routeHistory[targetID] = items
    }
    if (probeLogsProvider.value?.id === provider.id) probeLogsTargetHistory.value = routeHistory
  } catch (e: any) {
    if (!silent) appStore.showError(extractErrorMessage(e, t('admin.sub2apiProviders.health.logs.loadFailed')))
  } finally {
    probeLogsRefreshInFlight = false
    if (!silent && probeLogsProvider.value?.id === provider.id) loadingProbeLogs.value = false
  }
}

const stopProbeLogsPolling = () => {
  if (probeLogsPollTimer) {
    clearInterval(probeLogsPollTimer)
    probeLogsPollTimer = null
  }
}

const startProbeLogsPolling = () => {
  stopProbeLogsPolling()
  probeLogsPollTimer = setInterval(() => {
    if (showProbeLogsDialog.value && !document.hidden) void reloadProbeLogs(true)
  }, 30_000)
}

const openProbeLogs = (row: Sub2APIProvider) => {
  probeLogsProvider.value = row
  probeLogsControlHistory.value = []
  probeLogsTargets.value = []
  probeLogsTargetHistory.value = {}
  probeLogsOptimizationHistory.value = []
  showProbeLogsDialog.value = true
  void reloadProbeLogs()
  startProbeLogsPolling()
}

const closeProbeLogs = () => {
  stopProbeLogsPolling()
  showProbeLogsDialog.value = false
  probeLogsProvider.value = null
}

const probeStatusTextClass = (status?: string | null) => ({
  healthy: 'text-green-600 dark:text-green-400',
  degraded: 'text-amber-600 dark:text-amber-400',
  unhealthy: 'text-red-600 dark:text-red-400',
  unknown: 'text-gray-500 dark:text-dark-400',
  disabled: 'text-gray-400 dark:text-dark-500',
}[status || 'unknown'])

// ==================== 👁️ 查看绑定账户面板（新增）====================
const showAccountsPanel = ref(false)
const accountsPanelProvider = ref<Sub2APIProvider|null>(null)
const panelLinkedAccounts = ref<LinkedAccountInfo[]>([])
const accountsPanelProbeTargets = ref<Sub2APIProviderProbeTargetHealth[]>([])
const mobileExpandedAccountID = ref<number | null>(null)
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
    // The probe model is derived from the account setting. Reflect the saved
    // value locally so both monitoring surfaces update without a full reload.
    const updateProbeModel = (targets: Sub2APIProviderProbeTargetHealth[]) => {
      const targetIndex = targets.findIndex(target => target.account_id === acc.id)
      if (targetIndex === -1) return
      targets.splice(targetIndex, 1, {
        ...targets[targetIndex],
        test_model: next.test_model,
        sub2api_optimize_enabled: next.enabled,
        sub2api_min_multiplier: next.min_multiplier,
        sub2api_max_multiplier: next.max_multiplier,
      })
    }
    updateProbeModel(accountsPanelProbeTargets.value)
    if (probeProvider.value?.id === provider.id) updateProbeModel(probeTargets.value)
    appStore.showSuccess(t('admin.sub2apiProviders.settingsSaved'))
  } catch (e: any) {
    appStore.showError(extractErrorMessage(e, t('admin.sub2apiProviders.settingsSaveFailed')))
    panelLinkedAccounts.value = [...panelLinkedAccounts.value]
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

// 更新账号的「倍率下限」。关闭参与时可以清空；开启参与时不能为空。
// 非空须 >=0 且不超过已设的倍率上限，否则候选区间为空。参与状态保持不变。
const handleUpdateMinMultiplier = async (acc: LinkedAccountInfo, raw: string) => {
  if (acc.sub2api_optimize_enabled && raw.trim() === '') {
    appStore.showError(t('admin.sub2apiProviders.minMultiplierRequired'))
    panelLinkedAccounts.value = [...panelLinkedAccounts.value]
    return
  }
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

// 更新账号的「测试模型」。关闭参与时可以清空；开启参与时不能为空。
const handleUpdateTestModel = async (acc: LinkedAccountInfo, raw: string) => {
  const trimmed = raw.trim()
  if (acc.sub2api_optimize_enabled && trimmed === '') {
    appStore.showError(t('admin.sub2apiProviders.testModelRequired'))
    panelLinkedAccounts.value = [...panelLinkedAccounts.value]
    return
  }
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
  accountsPanelProbeTargets.value = []
  mobileExpandedAccountID.value = null
  showAccountsPanel.value = true
  // 打开面板时实时同步上游当前分组
  await refreshLinkedAccounts(row.id, true)
}

const refreshLinkedAccounts = async (providerId: number, sync = false) => {
  loadingLinked.value = true
  try {
    // 专用接口，直接返回关联到该 Provider 的账号（含远端分组信息）
    // sync=true 时后端会实时登录上游拉取当前分组
    const [accounts, targets] = await Promise.all([
      adminAPI.sub2apiProviders.getLinkedAccounts(providerId, sync),
      // Keep the route monitor aligned with the account panel after a manual
      // upstream group sync. The target owns the route snapshot used by probes.
      adminAPI.sub2apiProviders.getProbeTargets(providerId, sync),
    ])
    panelLinkedAccounts.value = accounts
    if (accountsPanelProvider.value?.id === providerId) {
      accountsPanelProbeTargets.value = targets
    }
  } catch {
    panelLinkedAccounts.value = []
    accountsPanelProbeTargets.value = []
  } finally { loadingLinked.value = false }
}

const panelAccountProbe = (account: LinkedAccountInfo): Sub2APIProviderProbeTargetHealth | undefined =>
  accountsPanelProbeTargets.value.find(target => target.account_id === account.id)

const accountMultiplierRangeState = (account: LinkedAccountInfo) => account.sub2api_optimize_enabled
  ? getMultiplierRangeState(
    account.remote_group_multiplier,
    account.sub2api_min_multiplier,
    account.sub2api_max_multiplier
  )
  : 'unbounded'

const accountMultiplierBadgeClass = (account: LinkedAccountInfo): string => {
  const state = accountMultiplierRangeState(account)
  if (state === 'above' || state === 'below') {
    return 'border-red-200 bg-red-50 text-red-700 dark:border-red-800 dark:bg-red-900/20 dark:text-red-300'
  }
  return 'border-gray-200 bg-gray-50 text-gray-600 dark:border-dark-600 dark:bg-dark-700 dark:text-dark-300'
}

const accountMultiplierOutOfRange = (account: LinkedAccountInfo): boolean => {
  const state = accountMultiplierRangeState(account)
  return state === 'above' || state === 'below'
}

const accountMultiplierTitle = (account: LinkedAccountInfo): string => {
  const current = account.remote_group_multiplier ?? '-'
  if (!account.sub2api_optimize_enabled) {
    return t('admin.sub2apiProviders.multiplierOptimizationDisabled', { current })
  }
  const state = accountMultiplierRangeState(account)
  if (state === 'above') {
    return t('admin.sub2apiProviders.multiplierRangeAboveDetail', { current, max: account.sub2api_max_multiplier })
  }
  if (state === 'below') {
    return t('admin.sub2apiProviders.multiplierRangeBelowDetail', { current, min: account.sub2api_min_multiplier })
  }
  if (state === 'within') {
    return t('admin.sub2apiProviders.multiplierRangeWithinDetail', {
      current,
      min: account.sub2api_min_multiplier,
      max: account.sub2api_max_multiplier,
    })
  }
  return t('admin.sub2apiProviders.multiplierRangeUnconfigured', { current })
}

const panelAccountProbeIsSelected = (account: LinkedAccountInfo) => Boolean(panelAccountProbe(account)?.enabled)

const panelAccountProbeStatus = (account: LinkedAccountInfo): ProviderAccountProbeStatus => {
  if (!panelAccountProbeIsSelected(account)) return 'disabled'
  return panelAccountProbe(account)?.status ?? 'unknown'
}

const probeAccountStatusLabel = (status: ProviderAccountProbeStatus, selected: boolean) => {
  if (!selected) return t('admin.sub2apiProviders.health.accountProbeDisabled')
  if (status === 'disabled') return t('admin.sub2apiProviders.health.mediaProbeSkipped')
  return t(`admin.sub2apiProviders.health.status.${status}`)
}

const openProbeFromAccountsPanel = () => {
  const provider = accountsPanelProvider.value
  if (!provider) return
  showAccountsPanel.value = false
  void openProbeDialog(provider)
}

const handleOptimizeAccount = async (provider: Sub2APIProvider, account: LinkedAccountInfo) => {
  if (!account.sub2api_optimize_enabled) {
    appStore.showError(t('admin.sub2apiProviders.optimizeNotEnabled'))
    return
  }
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

    // 单账号接口已经返回最终分组与倍率，只替换命中的账号对象。
    // 保持列表、其他账号和探针状态不变，避免整张表进入 loading 后重新渲染。
    applyOptimizeResultToAccounts(panelLinkedAccounts.value, r)
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
    panelLinkedAccounts.value = panelLinkedAccounts.value.filter(item => item.id !== account.id)
    accountsPanelProbeTargets.value = accountsPanelProbeTargets.value.filter(target => target.account_id !== account.id)
    linkDialogLinkedIds.value = new Set([...linkDialogLinkedIds.value].filter(id => id !== account.id))
    updateProviderAccountCount(provider.id, panelLinkedAccounts.value.length)
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
    const providerID = currentProvider.value.id
    const [linkedList, targets] = await Promise.all([
      adminAPI.sub2apiProviders.getLinkedAccounts(providerID, true),
      adminAPI.sub2apiProviders.getProbeTargets(providerID, true),
    ])
    panelLinkedAccounts.value = linkedList
    accountsPanelProbeTargets.value = targets
    linkDialogLinkedIds.value = new Set(linkedList.map((a) => a.id))
    updateProviderAccountCount(providerID, linkedList.length)
    void loadHealthOverviews([providerID], false, true)
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
    const incomplete = findIncompleteParticipatingAccounts(panelLinkedAccounts.value)
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
    // 批量接口已经返回每个账号的最终分组与倍率，只局部合并受影响行。
    if (showAccountsPanel.value && accountsPanelProvider.value?.id === row.id) {
      for (const result of r.results) {
        applyOptimizeResultToAccounts(panelLinkedAccounts.value, result)
        const targetIndex = accountsPanelProbeTargets.value.findIndex(target => target.account_id === result.account_id)
        if (targetIndex !== -1 && result.status !== 'failed') {
          const target = accountsPanelProbeTargets.value[targetIndex]
          accountsPanelProbeTargets.value[targetIndex] = {
            ...target,
            ...(result.new_group !== undefined ? { remote_group_name: result.new_group } : {}),
            ...(result.new_multiplier !== undefined ? { remote_group_multiplier: result.new_multiplier } : {}),
          }
        }
      }
    }
  } catch (e:any) {
    appStore.showError(extractErrorMessage(e, t('admin.sub2apiProviders.optimizeFailed')))
  }
  finally { optimizingAllId.value = null }
}

// ==================== 初始化 ====================
onMounted(() => {
  void loadProviders()
  void loadProviderProxies()
  startHealthPolling()
})

onBeforeUnmount(() => {
  stopHealthPolling()
  stopProbeLogsPolling()
  if (searchTimer) clearTimeout(searchTimer)
})
</script>

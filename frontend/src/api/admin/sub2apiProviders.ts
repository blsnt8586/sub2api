/**
 * Admin Sub2API Provider API endpoints
 * Handles third-party Sub2API instance management
 */

import { apiClient } from '../client'
import type { PaginatedResponse } from '@/types'

// ==================== Types ====================

export interface Sub2APIProvider {
  id: number
  name: string
  base_url: string
  /** 上游类型：当前仅 sub2api，后续可扩展其他上游协议 */
  provider_type: string
  status: 'active' | 'inactive'
  notes?: string | null
  proxy_id: number | null
  proxy_name?: string | null
  email: string
  auth_mode: 'password' | 'token_pair'
  has_access_token: boolean
  has_refresh_token: boolean
  access_token_expires_at?: string | null
  last_token_refresh_at?: string | null
  last_auth_error?: string | null
  api_path_keys?: string | null
  api_path_groups?: string | null
  last_sync_at?: string | null
  last_sync_status?: 'success' | 'failed' | null
  last_sync_error?: string | null
  created_at: string
  updated_at: string
  accounts_count: number
}

export interface Sub2APIProviderWithAccounts {
  provider: Sub2APIProvider
  accounts_count: number
}

export interface CreateProviderRequest {
  name: string
  base_url: string
  /** 上游类型：不传则后端默认 sub2api */
  provider_type?: string
  email: string
  password?: string
  auth_mode?: 'password' | 'token_pair'
  access_token?: string
  refresh_token?: string
  notes?: string | null
  proxy_id?: number | null
}

export interface UpdateProviderRequest {
  name?: string
  base_url?: string
  email?: string
  password?: string
  auth_mode?: 'password' | 'token_pair'
  access_token?: string
  refresh_token?: string
  status?: 'active' | 'inactive'
  notes?: string | null
  proxy_id?: number | null
}

export interface PathDetectionResult {
  keys_path: string
  groups_path: string
}

export interface Sub2APIProviderRemoteGroupRate {
  id: number
  name: string
  description?: string
  platform?: string
  status?: string
  default_multiplier: number
  effective_multiplier: number
  has_custom_rate: boolean
}

export interface Sub2APIProviderRemoteOverview {
  provider_id: number
  available: boolean
  balance: number
  groups: Sub2APIProviderRemoteGroupRate[]
  rate_overrides_available: boolean
  sampled_at: string
  source: 'manual' | 'control_probe' | ''
  last_attempted_at: string
  last_attempt_source: 'manual' | 'control_probe' | ''
  last_error?: string | null
  last_error_at?: string | null
}

export interface AccountProviderLink {
  account_id: number
  account_name: string
  account_platform: string
  provider_id: number
  provider_api_key_id?: number | null
  remote_group_name?: string | null
  remote_group_multiplier?: number | null
  remote_group_synced_at?: string | null
}

export type ProviderHealthStatus = 'healthy' | 'degraded' | 'unhealthy' | 'unknown'

export type ProviderAccountProbeStatus = ProviderHealthStatus | 'disabled'

export interface Sub2APIProviderAccountProbe {
  account_id: number
  account_name?: string
  platform?: string
  status: ProviderAccountProbeStatus
  latency_ms?: number | null
  error_category?: string | null
  error_message?: string | null
  checked_at?: string | null
}

export interface Sub2APIProviderHealth {
  provider_id: number
  status: ProviderHealthStatus
  control_status: ProviderHealthStatus
  data_status: ProviderHealthStatus
  traffic_status: ProviderHealthStatus
  consecutive_failures: number
  login_latency_ms?: number | null
  health_latency_ms?: number | null
  keys_latency_ms?: number | null
  groups_latency_ms?: number | null
  data_probe_count: number
  data_probe_success: number
  data_probe_failed: number
  data_probe_enabled: boolean
  data_probe_interval_seconds: number
  probe_account_count: number
  account_probes: Sub2APIProviderAccountProbe[]
  traffic_request_count: number
  traffic_success_rate?: number | null
  traffic_p95_latency_ms?: number | null
  error_category?: string | null
  error_message?: string | null
  details?: Record<string, unknown>
  last_checked_at?: string | null
}

export interface Sub2APIProviderHealthBucket {
  started_at: string
  ended_at: string
  status: ProviderHealthStatus
  sample_count: number
  healthy_samples: number
  degraded_samples: number
  unhealthy_samples: number
  max_health_latency_ms?: number | null
  last_error?: string | null
}

export interface Sub2APIProviderHealthOverview {
  provider_id: number
  availability_status: ProviderHealthStatus
  evidence_status: ProviderHealthStatus
  latest?: Sub2APIProviderHealth | null
  latest_control?: Sub2APIProviderHealth | null
  window_started_at: string
  window_ended_at: string
  bucket_seconds: number
  buckets: Sub2APIProviderHealthBucket[]
  summary: {
    healthy: number
    degraded: number
    unhealthy: number
    unknown: number
  }
  routes: Sub2APIProviderProbeTargetHealth[]
}

export interface Sub2APIProviderProbeTargetHealth {
  id: number
  provider_id: number
  account_id: number
  account_name: string
  provider_api_key_id?: number | null
  remote_group_id?: number | null
  remote_group_name?: string | null
  remote_group_multiplier?: number | null
  sub2api_optimize_enabled?: boolean
  sub2api_min_multiplier?: number | null
  sub2api_max_multiplier?: number | null
  platform: string
  enabled: boolean
  interval_seconds: number
  test_model?: string | null
  allow_media_probe: boolean
  timeout_seconds: number
  degraded_latency_ms: number
  failure_threshold: number
  recovery_threshold: number
  status: ProviderAccountProbeStatus
  latency_ms?: number | null
  traffic_request_count: number
  traffic_success_rate?: number | null
  traffic_p95_latency_ms?: number | null
  error_category?: string | null
  error_message?: string | null
  last_checked_at?: string | null
  last_run_at?: string | null
  route_changed_at?: string | null
  consecutive_failures: number
  buckets: Sub2APIProviderHealthBucket[]
}

export type UpdateProviderProbeTargetRequest = Partial<Pick<
  Sub2APIProviderProbeTargetHealth,
  | 'enabled'
  | 'interval_seconds'
  | 'allow_media_probe'
  | 'timeout_seconds'
  | 'degraded_latency_ms'
  | 'failure_threshold'
  | 'recovery_threshold'
>>

export interface Sub2APIProviderProbeConfig {
  id: number
  provider_id: number
  control_enabled: boolean
  control_interval_seconds: number
  data_enabled: boolean
  data_interval_seconds: number
  selected_account_ids: number[]
  allow_media_probe: boolean
  timeout_seconds: number
  degraded_latency_ms: number
  failure_threshold: number
  recovery_threshold: number
  last_control_run_at?: string | null
  last_data_run_at?: string | null
}

export type UpdateProviderProbeConfigRequest = Partial<Omit<
  Sub2APIProviderProbeConfig,
  'id' | 'provider_id' | 'last_control_run_at' | 'last_data_run_at'
>>

/** 用于"查看绑定账号"面板的详细信息（字段与后端 LinkedAccountInfo 对齐） */
export interface LinkedAccountInfo {
  id: number
  name: string
  platform: string
  status: string
  provider_id: number
  provider_api_key_id?: number | null
  remote_group_name?: string | null
  remote_group_multiplier?: number | null
  remote_group_synced_at?: string | null
  sub2api_optimize_enabled?: boolean
  sub2api_min_multiplier?: number | null
  sub2api_max_multiplier?: number | null
  sub2api_test_model?: string | null
}

// 手动优化（单个/批量）返回结构，与定时任务日志明细 OptimizeLogDetail 共用同一契约：
// 后端走同一套智能引擎（倍率上限 + 连通测试 + 回滚），逐账号给出 optimized/skipped/failed 结论。
export interface OptimizeResult {
  account_id: number
  account_name: string
  status: 'optimized' | 'skipped' | 'failed'
  old_group?: string
  new_group?: string
  old_multiplier?: number
  new_multiplier?: number
  reason?: string
  switch_events?: OptimizeGroupSwitchEvent[]
}

export interface OptimizeAllResult {
  results: OptimizeResult[]
  total: number
  optimized: number
  skipped: number
  failed: number
}

// ==================== API Functions ====================

/**
 * 列出所有 Provider（分页）
 */
export async function list(
  page: number = 1,
  pageSize: number = 20,
  filters?: {
    status?: string
    search?: string
  },
  options?: { signal?: AbortSignal }
): Promise<PaginatedResponse<Sub2APIProvider>> {
  const { data } = await apiClient.get<PaginatedResponse<Sub2APIProvider>>('/admin/sub2api-providers', {
    params: { page, page_size: pageSize, ...filters },
    signal: options?.signal
  })
  return data
}

/**
 * 获取所有活跃 Provider（不分页，用于下拉选择）
 */
export async function listAll(
  filters?: { status?: string }
): Promise<Sub2APIProvider[]> {
  const { data } = await apiClient.get<Sub2APIProvider[]>('/admin/sub2api-providers/all', {
    params: filters
  })
  return data
}

/**
 * 根据 ID 获取 Provider 详情（含关联账号数）
 */
export async function getById(id: number): Promise<Sub2APIProviderWithAccounts> {
  const { data } = await apiClient.get<Sub2APIProviderWithAccounts>(`/admin/sub2api-providers/${id}`)
  return data
}

/**
 * 创建 Provider
 */
export async function create(payload: CreateProviderRequest): Promise<Sub2APIProvider> {
  const { data } = await apiClient.post<Sub2APIProvider>('/admin/sub2api-providers', payload)
  return data
}

/**
 * 更新 Provider
 */
export async function update(id: number, payload: UpdateProviderRequest): Promise<Sub2APIProvider> {
  const { data } = await apiClient.put<Sub2APIProvider>(`/admin/sub2api-providers/${id}`, payload)
  return data
}

/**
 * 删除 Provider（软删除）
 */
export async function deleteProvider(id: number): Promise<{ message: string }> {
  const { data } = await apiClient.delete<{ message: string }>(`/admin/sub2api-providers/${id}`)
  return data
}

/**
 * 测试 Provider 连接
 */
export async function testConnection(id: number): Promise<{ message: string }> {
  const { data } = await apiClient.post<{ message: string }>(`/admin/sub2api-providers/${id}/test-connection`)
  return data
}

/**
 * 探测并更新 API 路径
 */
export async function detectPaths(id: number): Promise<PathDetectionResult> {
  const { data } = await apiClient.post<PathDetectionResult>(`/admin/sub2api-providers/${id}/detect-paths`)
  return data
}

/**
 * 实时读取上游登录账号的余额和远程分组倍率，并刷新 Redis 最新快照。
 */
export async function getRemoteOverview(id: number): Promise<Sub2APIProviderRemoteOverview> {
  const { data } = await apiClient.get<Sub2APIProviderRemoteOverview>(
    `/admin/sub2api-providers/${id}/remote-overview`
  )
  return data
}

/**
 * 批量读取 Redis 中的平台资产快照，不会向任何上游发起请求。
 */
export async function getCachedRemoteOverviews(ids: number[]): Promise<Sub2APIProviderRemoteOverview[]> {
  if (ids.length === 0) return []
  const { data } = await apiClient.get<Sub2APIProviderRemoteOverview[]>(
    '/admin/sub2api-providers/remote-overviews',
    { params: { ids: ids.join(',') } }
  )
  return data
}

export async function getHealth(id: number): Promise<Sub2APIProviderHealth> {
  const { data } = await apiClient.get<Sub2APIProviderHealth>(`/admin/sub2api-providers/${id}/health`)
  return data
}

export async function getHealthOverview(ids: number[]): Promise<Sub2APIProviderHealthOverview[]> {
  if (ids.length === 0) return []
  const { data } = await apiClient.get<Sub2APIProviderHealthOverview[]>('/admin/sub2api-providers/health-overview', {
    params: { ids: ids.join(',') }
  })
  return data
}

export async function runProbe(id: number): Promise<Sub2APIProviderHealth> {
  const { data } = await apiClient.post<Sub2APIProviderHealth>(`/admin/sub2api-providers/${id}/probe/run`)
  return data
}

export async function getProbeConfig(id: number): Promise<Sub2APIProviderProbeConfig> {
  const { data } = await apiClient.get<Sub2APIProviderProbeConfig>(`/admin/sub2api-providers/${id}/probe-config`)
  return data
}

export async function updateProbeConfig(id: number, payload: UpdateProviderProbeConfigRequest): Promise<Sub2APIProviderProbeConfig> {
  const { data } = await apiClient.put<Sub2APIProviderProbeConfig>(`/admin/sub2api-providers/${id}/probe-config`, payload)
  return data
}

export async function getProbeHistory(id: number, limit = 100, sinceSeconds = 3600): Promise<Sub2APIProviderHealth[]> {
  const { data } = await apiClient.get<Sub2APIProviderHealth[]>(`/admin/sub2api-providers/${id}/probe/history`, {
    params: { limit, since_seconds: sinceSeconds }
  })
  return data
}

export async function getProbeTargets(id: number, sync = false): Promise<Sub2APIProviderProbeTargetHealth[]> {
  const { data } = await apiClient.get<Sub2APIProviderProbeTargetHealth[]>(
    `/admin/sub2api-providers/${id}/probe-targets`,
    { params: sync ? { sync: 'true' } : undefined }
  )
  return data
}

export async function updateProbeTarget(
  providerId: number,
  targetId: number,
  payload: UpdateProviderProbeTargetRequest
): Promise<Sub2APIProviderProbeTargetHealth> {
  const { data } = await apiClient.put<Sub2APIProviderProbeTargetHealth>(
    `/admin/sub2api-providers/${providerId}/probe-targets/${targetId}`,
    payload
  )
  return data
}

export async function runProbeTarget(providerId: number, targetId: number): Promise<Sub2APIProviderProbeTargetHealth> {
  const { data } = await apiClient.post<Sub2APIProviderProbeTargetHealth>(
    `/admin/sub2api-providers/${providerId}/probe-targets/${targetId}/run`
  )
  return data
}

export async function getProbeTargetHistory(
  providerId: number,
  targetId: number,
  limit = 100,
  sinceSeconds = 3600
): Promise<Sub2APIProviderProbeTargetHealth[]> {
  const { data } = await apiClient.get<Sub2APIProviderProbeTargetHealth[]>(
    `/admin/sub2api-providers/${providerId}/probe-targets/${targetId}/history`,
    { params: { limit, since_seconds: sinceSeconds } }
  )
  return data
}

/**
 * 关联 Account 到 Provider（自动匹配远程 APIKey）
 */
export async function linkAccount(
  providerId: number,
  accountId: number
): Promise<AccountProviderLink> {
  const { data } = await apiClient.post<AccountProviderLink>(
    `/admin/sub2api-providers/${providerId}/link-account`,
    { account_id: accountId }
  )
  return data
}

/**
 * 获取 Provider 下所有关联账号（含远端 Key ID、分组、倍率）
 * @param sync 为 true 时后端会实时登录上游拉取当前分组并刷新缓存
 */
export async function getLinkedAccounts(providerId: number, sync = false): Promise<LinkedAccountInfo[]> {
  const { data } = await apiClient.get<LinkedAccountInfo[]>(
    `/admin/sub2api-providers/${providerId}/accounts`,
    { params: sync ? { sync: 'true' } : undefined }
  )
  return data
}

/**
 * 解除 Account 关联
 */
export async function unlinkAccount(
  providerId: number,
  accountId: number
): Promise<{ message: string }> {
  const { data } = await apiClient.delete<{ message: string }>(
    `/admin/sub2api-providers/${providerId}/accounts/${accountId}`
  )
  return data
}

/**
 * 优化单个 Account 的分组（切换到最低倍率）
 */
export async function optimizeAccount(
  providerId: number,
  accountId: number
): Promise<OptimizeResult> {
  const { data } = await apiClient.post<OptimizeResult>(
    `/admin/sub2api-providers/${providerId}/accounts/${accountId}/optimize`
  )
  return data
}

/**
 * 批量优化该 Provider 下所有关联 Account
 */
export async function optimizeAll(providerId: number): Promise<OptimizeAllResult> {
  const { data } = await apiClient.post<OptimizeAllResult>(
    `/admin/sub2api-providers/${providerId}/optimize-all`
  )
  return data
}

// ==================== 定时优化 Types ====================

export interface OptimizeScheduleInfo {
  id: number
  provider_id: number
  cron_expr: string
  enabled: boolean
  last_run_at?: string | null
  next_run_at?: string | null
  created_at: string
  updated_at: string
  recent_logs?: OptimizeLogInfo[]
}

/** 单个账号的优化明细（后端 detail 数组的元素） */
export interface OptimizeLogDetail {
  account_id?: number
  account_name?: string
  status?: 'optimized' | 'skipped' | 'failed'
  old_group?: string
  new_group?: string
  old_multiplier?: number
  new_multiplier?: number
  reason?: string
  trigger?: OptimizeLogTrigger
  switch_events?: OptimizeGroupSwitchEvent[]
  probe_target_id?: number
  probe_run_id?: number
  failure_threshold?: number
  probe_error_category?: string
  probe_error_message?: string
  execution_disposition?: 'executed' | 'coalesced' | 'deferred'
  coalesced_from_log_id?: number
  coalesced_from_trigger?: OptimizeLogTrigger
}

export type OptimizeLogTrigger =
  | 'cron'
  | 'schedule_now'
  | 'probe_unhealthy'
  | 'manual_account'
  | 'manual_all'
  | 'legacy'

export interface OptimizeGroupSwitchEvent {
  action: 'switch' | 'rollback'
  from_group_id?: number
  from_group?: string
  from_multiplier?: number
  to_group_id?: number
  to_group?: string
  to_multiplier?: number
  status: 'success' | 'failed'
  test_status?: 'passed' | 'failed'
  reason?: string
  occurred_at: string
}

export interface OptimizeLogInfo {
  id: number
  provider_id: number
  schedule_id?: number | null
  trigger: OptimizeLogTrigger
  status: 'success' | 'failed' | 'partial' | 'skipped'
  total: number
  optimized: number
  skipped: number
  failed: number
  detail?: OptimizeLogDetail[] | null
  started_at?: string | null
  finished_at?: string | null
  created_at: string
}

export interface OptimizeLogFilters {
  trigger?: OptimizeLogTrigger | ''
  status?: OptimizeLogInfo['status'] | ''
  account_id?: number
  keyword?: string
  from?: string
  to?: string
  page?: number
  page_size?: number
}

export interface UpsertOptimizeScheduleRequest {
  cron_expr: string
  enabled: boolean
}

export interface UpdateAccountOptimizeSettingsRequest {
  enabled: boolean
  min_multiplier?: number | null
  max_multiplier?: number | null
  test_model?: string | null
}

// ==================== 定时优化 API Functions ====================

/**
 * 获取 Provider 的定时优化配置（含最近5条日志）。
 * 后端返回扁平的 OptimizeScheduleInfo（日志在 recent_logs），未配置时返回 null。
 */
export async function getOptimizeSchedule(
  providerId: number
): Promise<{ schedule: OptimizeScheduleInfo | null; logs: OptimizeLogInfo[] }> {
  const { data } = await apiClient.get<OptimizeScheduleInfo | null>(
    `/admin/sub2api-providers/${providerId}/optimize-schedule`
  )
  return { schedule: data, logs: data?.recent_logs ?? [] }
}

/**
 * 创建或更新定时优化配置
 */
export async function upsertOptimizeSchedule(
  providerId: number,
  payload: UpsertOptimizeScheduleRequest
): Promise<OptimizeScheduleInfo> {
  const { data } = await apiClient.put<OptimizeScheduleInfo>(
    `/admin/sub2api-providers/${providerId}/optimize-schedule`,
    payload
  )
  return data
}

/**
 * 删除定时优化配置
 */
export async function deleteOptimizeSchedule(
  providerId: number
): Promise<{ message: string }> {
  const { data } = await apiClient.delete<{ message: string }>(
    `/admin/sub2api-providers/${providerId}/optimize-schedule`
  )
  return data
}

/**
 * 立即执行一次定时优化
 */
export async function runOptimizeNow(
  providerId: number
): Promise<OptimizeScheduleInfo> {
  const { data } = await apiClient.post<OptimizeScheduleInfo>(
    `/admin/sub2api-providers/${providerId}/optimize-schedule/run`
  )
  return data
}

/**
 * 查询 Provider 级优化审计日志。日志不依赖定时计划，删除计划后仍可查询。
 */
export async function listOptimizeLogs(
  providerId: number,
  filters: OptimizeLogFilters = {}
): Promise<PaginatedResponse<OptimizeLogInfo>> {
  const params: Record<string, string | number> = {}
  if (filters.trigger) params.trigger = filters.trigger
  if (filters.status) params.status = filters.status
  if (filters.account_id) params.account_id = filters.account_id
  if (filters.keyword?.trim()) params.keyword = filters.keyword.trim()
  if (filters.from) params.from = filters.from
  if (filters.to) params.to = filters.to
  params.page = filters.page ?? 1
  params.page_size = filters.page_size ?? 20

  const { data } = await apiClient.get<PaginatedResponse<OptimizeLogInfo>>(
    `/admin/sub2api-providers/${providerId}/optimize-logs`,
    { params }
  )
  return data
}

/**
 * 更新账号的定时优化设置（倍率上限、测试模型）
 */
export async function updateAccountOptimizeSettings(
  providerId: number,
  accountId: number,
  payload: UpdateAccountOptimizeSettingsRequest
): Promise<{ message: string }> {
  const { data } = await apiClient.put<{ message: string }>(
    `/admin/sub2api-providers/${providerId}/accounts/${accountId}/optimize-settings`,
    payload
  )
  return data
}

export const sub2apiProvidersAPI = {
  list,
  listAll,
  getById,
  create,
  update,
  delete: deleteProvider,
  testConnection,
  detectPaths,
  getRemoteOverview,
  getCachedRemoteOverviews,
  getHealth,
  getHealthOverview,
  runProbe,
  getProbeConfig,
  updateProbeConfig,
  getProbeHistory,
  getProbeTargets,
  updateProbeTarget,
  runProbeTarget,
  getProbeTargetHistory,
  getLinkedAccounts,
  linkAccount,
  unlinkAccount,
  optimizeAccount,
  optimizeAll,
  getOptimizeSchedule,
  upsertOptimizeSchedule,
  deleteOptimizeSchedule,
  runOptimizeNow,
  listOptimizeLogs,
  updateAccountOptimizeSettings,
}

export default sub2apiProvidersAPI

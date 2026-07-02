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
  email: string
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
  password: string
  notes?: string | null
}

export interface UpdateProviderRequest {
  name?: string
  base_url?: string
  email?: string
  password?: string
  status?: 'active' | 'inactive'
  notes?: string | null
}

export interface PathDetectionResult {
  keys_path: string
  groups_path: string
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
}

export interface OptimizeLogInfo {
  id: number
  schedule_id: number
  status: 'success' | 'failed' | 'partial'
  total: number
  optimized: number
  skipped: number
  failed: number
  detail?: OptimizeLogDetail[] | null
  started_at: string
  finished_at?: string | null
  created_at: string
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
): Promise<{ message: string }> {
  const { data } = await apiClient.post<{ message: string }>(
    `/admin/sub2api-providers/${providerId}/optimize-schedule/run`
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
  getLinkedAccounts,
  linkAccount,
  unlinkAccount,
  optimizeAccount,
  optimizeAll,
  getOptimizeSchedule,
  upsertOptimizeSchedule,
  deleteOptimizeSchedule,
  runOptimizeNow,
  updateAccountOptimizeSettings,
}

export default sub2apiProvidersAPI

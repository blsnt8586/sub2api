/**
 * Codex 雷达 API 客户端（二开新增）。
 *
 * 数据来源为第三方社区站点 codexradar.com，本平台仅做代理缓存 + 署名转载。
 * 图片接口需 JWT 鉴权（Bearer 头），故用 axios 拉 blob 再转 objectURL，
 * 而非直接 <img src>（后者无法携带 Authorization 头）。
 */

import { apiClient } from './client'

export interface CodexRadarRecommendationItem {
  model?: string
  effort?: string
  iq?: number
  passed?: number
  samples?: number
  average_cost_usd?: number
  average_duration_minutes?: number
  [key: string]: unknown
}

export interface CodexRadarRecommendationGroup {
  key?: string
  title?: string
  rule?: string
  items?: CodexRadarRecommendationItem[]
  [key: string]: unknown
}

export interface CodexRadarIntelligencePoint {
  model?: string
  effort?: string
  iq?: number
  passed?: number
  total?: number
  average_price_usd?: number
  average_minutes?: number
  runs_24h?: number
  runs_48h?: number
  runs_total?: number
  [key: string]: unknown
}

export interface CodexRadarData {
  recommendations?: {
    recommendations?: CodexRadarRecommendationGroup[]
    [key: string]: unknown
  }
  intelligence?: {
    points?: CodexRadarIntelligencePoint[]
    [key: string]: unknown
  }
  [key: string]: unknown
}

/** 第三方摘要响应：两个结构化接口 + 本平台附加的来源元信息。 */
export interface CodexRadarSummary {
  enabled: boolean
  available: boolean
  /** 第三方来源站点首页（前端「详情查看」跳转）。 */
  source: string
  /** 第三方数据署名。 */
  attribution: string
  /** 本平台缓存时间（RFC3339），空 = 尚无数据。 */
  fetched_at: string
  refresh_interval_seconds: number
  data: CodexRadarData | Record<string, unknown> | null
}

/** 拉取缓存的第三方状态摘要。 */
export async function getCodexRadarSummary(): Promise<CodexRadarSummary> {
  const { data } = await apiClient.get<CodexRadarSummary>('/codexradar/summary')
  return data
}

/**
 * 拉取漫画摘要图并转为可用于 <img> 的 objectURL。
 * 调用方负责在替换/卸载时 URL.revokeObjectURL 释放。
 */
export async function fetchCodexRadarImageObjectURL(): Promise<string> {
  const response = await apiClient.get('/codexradar/image', { responseType: 'blob' })
  return URL.createObjectURL(response.data as Blob)
}

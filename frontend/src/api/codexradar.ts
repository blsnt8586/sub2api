/**
 * Codex 雷达 API 客户端（二开新增）。
 *
 * 数据来源为第三方社区站点 codexradar.com，本平台仅做代理缓存 + 署名转载。
 * 图片接口需 JWT 鉴权（Bearer 头），故用 axios 拉 blob 再转 objectURL，
 * 而非直接 <img src>（后者无法携带 Authorization 头）。
 */

import { apiClient } from './client'

/** 第三方摘要响应：原始 current.json 数据 + 本平台附加的来源/署名元信息。 */
export interface CodexRadarSummary {
  enabled: boolean
  available: boolean
  /** 第三方来源站点首页（前端「详情查看」跳转）。 */
  source: string
  /** 第三方数据署名。 */
  attribution: string
  /** 本平台缓存时间（RFC3339），空 = 尚无数据。 */
  fetched_at: string
  /** 第三方 current.json 原样透传；结构以对方为准，弱类型消费。 */
  data: Record<string, unknown> | null
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

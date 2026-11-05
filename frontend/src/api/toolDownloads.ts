import { apiClient } from './client'

export type ToolPlatform = 'windows' | 'macos' | 'linux'

export interface ToolDownloadAsset {
  name: string
  platform: ToolPlatform
  architecture: string
  format: 'exe' | 'msi' | 'zip' | 'dmg' | 'appimage' | 'deb'
  size: number
  url: string
  digest?: string
}

export interface DownloadableTool {
  id: string
  name: string
  description?: string
  repository_url: string
  release_url: string
  version: string
  published_at?: string
  checked_at?: string
  next_check_at?: string
  available: boolean
  stale: boolean
  syncing?: boolean
  refresh_interval_seconds: number
  assets: ToolDownloadAsset[]
}

export async function getToolDownloads(signal?: AbortSignal): Promise<{ tools: DownloadableTool[] }> {
  const { data } = await apiClient.get<{ tools: DownloadableTool[] }>('/tool-downloads', { signal })
  return data
}

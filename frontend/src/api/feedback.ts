import { apiClient } from './client'

export interface FeedbackAttachment { id: number; filename: string; content_type: string; size_bytes: number; data_url?: string }
export interface FeedbackMessage { id: number; author_type: 'user' | 'admin'; author_id: number; content: string; created_at: string; attachments: FeedbackAttachment[] }
export interface FeedbackThread { id: number; user_id: number; username?: string; email?: string; subject: string; status: 'open' | 'replied' | 'closed'; created_at: string; updated_at: string; messages: FeedbackMessage[] }
export interface FeedbackThreadPage { items: FeedbackThread[]; has_more: boolean }

export interface FeedbackInboxUser {
  user_id: number
  username: string
  email: string
  thread_count: number
  unread_count: number
  updated_at: string
  preview: string
}

export interface FeedbackThreadSummary {
  id: number
  user_id: number
  subject: string
  status: FeedbackThread['status']
  created_at: string
  updated_at: string
  unread_count: number
  last_message_id: number
  preview: string
}

export interface FeedbackAdminThread extends FeedbackThread {
  unread_count: number
  last_user_message_id: number
}

export interface FeedbackPage<T> { items: T[]; has_more: boolean }

export async function listFeedbackUsers(offset = 0): Promise<FeedbackPage<FeedbackInboxUser>> {
  const { data } = await apiClient.get<FeedbackPage<FeedbackInboxUser>>('/admin/feedback/users', { params: { limit: 20, offset } })
  return { items: data?.items ?? [], has_more: data?.has_more === true }
}

export async function listUserFeedback(userID: number, status = '', offset = 0): Promise<FeedbackPage<FeedbackThreadSummary>> {
  const { data } = await apiClient.get<FeedbackPage<FeedbackThreadSummary>>(`/admin/feedback/users/${userID}/threads`, { params: { status, limit: 20, offset } })
  return { items: data?.items ?? [], has_more: data?.has_more === true }
}

export async function getFeedbackThread(id: number): Promise<FeedbackAdminThread> {
  const { data } = await apiClient.get<FeedbackAdminThread>(`/admin/feedback/${id}`)
  return data
}

export async function markFeedbackRead(id: number, lastReadMessageID: number): Promise<void> {
  await apiClient.post(`/admin/feedback/${id}/read`, { last_read_message_id: lastReadMessageID })
}

export async function listMyFeedback(offset = 0): Promise<FeedbackThreadPage> {
	const { data } = await apiClient.get<FeedbackThreadPage>('/feedback', { params: { limit: 10, offset } })
  return {
    items: Array.isArray(data?.items) ? data.items : [],
    has_more: data?.has_more === true,
  }
}

export async function createFeedback(form: FormData): Promise<FeedbackThread> {
	const { data } = await apiClient.post<FeedbackThread>('/feedback', form, { headers: { 'Content-Type': 'multipart/form-data' } })
  return data
}

export async function replyMyFeedback(id: number, form: FormData): Promise<FeedbackThread> {
  const { data } = await apiClient.post<FeedbackThread>(`/feedback/${id}/reply`, form, { headers: { 'Content-Type': 'multipart/form-data' } })
  return data
}

export async function listFeedback(status = ''): Promise<FeedbackThread[]> {
	const { data } = await apiClient.get<FeedbackThread[]>('/admin/feedback', { params: { status } })
  return data ?? []
}

export async function replyFeedback(id: number, form: FormData): Promise<FeedbackThread> {
	const { data } = await apiClient.post<FeedbackThread>(`/admin/feedback/${id}/reply`, form, { headers: { 'Content-Type': 'multipart/form-data' } })
  return data
}

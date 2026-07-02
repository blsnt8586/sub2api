/**
 * API 错误处理工具
 */

/**
 * 从 axios 错误对象中提取错误消息
 * @param error 错误对象（通常来自 axios catch）
 * @param fallback 备用消息（当无法提取到服务端消息时使用）
 * @returns 错误消息字符串
 */
export function extractErrorMessage(error: any, fallback: string): string {
  return error?.response?.data?.message ?? fallback
}

/**
 * 通用 API 错误处理包装器
 * 自动提取并展示错误消息，无需手动写 try-catch
 *
 * @example
 * await handleApiError(
 *   () => adminAPI.sub2apiProviders.delete(id),
 *   appStore.showSuccess.bind(appStore, t('deleteSuccess')),
 *   appStore.showError.bind(appStore),
 *   t('deleteFailed')
 * )
 */
export async function handleApiError<T>(
  apiCall: () => Promise<T>,
  onSuccess?: (result: T) => void,
  showError?: (msg: string) => void,
  fallbackError?: string
): Promise<T | null> {
  try {
    const result = await apiCall()
    if (onSuccess) onSuccess(result)
    return result
  } catch (error: any) {
    if (showError && fallbackError) {
      showError(extractErrorMessage(error, fallbackError))
    }
    return null
  }
}

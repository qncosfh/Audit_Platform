/**
 * 通用工具函数库
 */

export {}

/**
 * 根据进度百分比返回进度条颜色
 * @param percentage 进度百分比 (0-100)
 * @returns 颜色值
 */
export const getProgressColor = (percentage: number): string => {
  if (percentage < 30) return '#3b82f6'
  if (percentage < 70) return '#10b981'
  return '#f59e0b'
}

/**
 * 格式化日期为本地化日期字符串
 * @param dateString 日期字符串
 * @param locale 语言环境，默认为 'zh-CN'
 * @returns 格式化后的日期字符串
 */
export const formatDate = (dateString: string | undefined | null, locale: string = 'zh-CN'): string => {
  if (!dateString) return '-'
  const date = new Date(dateString)
  if (isNaN(date.getTime())) return '-'
  return date.toLocaleString(locale === 'en' ? 'en-US' : 'zh-CN')
}

/**
 * 格式化日期为相对时间（几分钟前、几小时前等）
 * @param dateString 日期字符串
 * @param t 国际化函数
 * @returns 相对时间字符串
 */
export const formatRelativeDate = (dateString: string | undefined | null, t?: (key: string) => string): string => {
  if (!dateString) return '-'
  const date = new Date(dateString)
  if (isNaN(date.getTime())) return '-'
  
  const now = new Date()
  const diff = now.getTime() - date.getTime()
  const minutes = Math.floor(diff / (1000 * 60))
  const hours = Math.floor(diff / (1000 * 60 * 60))
  const days = Math.floor(diff / (1000 * 60 * 60 * 24))
  
  // 如果传入了国际化函数，使用国际化
  if (t) {
    if (minutes < 1) return t('task.justNow')
    if (minutes < 60) return `${minutes}${t('task.minutesAgo')}`
    if (hours < 24) return `${hours}${t('task.hoursAgo')}`
    if (days === 1) return t('task.yesterday')
  }
  
  // 默认中文
  if (minutes < 1) return '刚刚'
  if (minutes < 60) return `${minutes}分钟前`
  if (hours < 24) return `${hours}小时前`
  if (days === 1) return '昨天'
  
  return formatDate(dateString)
}

/**
 * 格式化时长（秒）为人类可读格式
 * @param seconds 秒数
 * @returns 格式化后的时长字符串
 */
export const formatDuration = (seconds: number | undefined | null): string => {
  if (!seconds) return '-'
  const hours = Math.floor(seconds / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  const secs = Math.floor(seconds % 60)
  
  if (hours > 0) return `${hours}h ${minutes}m`
  if (minutes > 0) return `${minutes}m`
  return `${secs}s`
}

/**
 * 格式化文件大小
 * @param bytes 字节数
 * @returns 格式化后的文件大小字符串
 */
export const formatSize = (bytes: number): string => {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}
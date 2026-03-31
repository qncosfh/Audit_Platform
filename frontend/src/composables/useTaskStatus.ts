/**
 * 任务状态相关的组合式函数
 */
import { useI18n } from 'vue-i18n'

/**
 * 任务状态处理组合式函数
 * 提供任务状态相关的类型转换和文本获取功能
 */
export const useTaskStatus = () => {
  const { t } = useI18n()

  /**
   * 获取任务状态对应的 Element Plus 标签类型
   * @param status 任务状态字符串
   * @returns Element Plus 标签类型
   */
  const getTaskStatusType = (status: string): string => {
    switch (status) {
      case 'running':
        return 'primary'
      case 'completed':
        return 'success'
      case 'failed':
        return 'danger'
      case 'paused':
        return 'warning'
      case 'pending':
        return 'info'
      default:
        return 'info'
    }
  }

  /**
   * 获取任务状态对应的文本
   * @param status 任务状态字符串
   * @returns 本地化的状态文本
   */
  const getTaskStatusText = (status: string): string => {
    switch (status) {
      case 'pending':
        return t('task.pending')
      case 'running':
        return t('task.running')
      case 'completed':
        return t('task.completed')
      case 'failed':
        return t('task.failed')
      case 'paused':
        return t('task.paused')
      default:
        return t('task.unknownType')
    }
  }

  /**
   * 获取安全评分对应的样式类名
   * @param score 安全评分 (0-100)
   * @returns 样式类名
   */
  const getScoreClass = (score: number): string => {
    if (score >= 90) return 'score-high'
    if (score >= 70) return 'score-medium'
    if (score >= 50) return 'score-low'
    return 'score-critical'
  }

  /**
   * 获取安全评分对应的 Element Plus 标签类型
   * @param score 安全评分 (0-100)
   * @returns Element Plus 标签类型
   */
  const getScoreTagType = (score: number): '' | 'success' | 'warning' | 'danger' => {
    if (score >= 90) return 'success'
    if (score >= 70) return 'warning'
    return 'danger'
  }

  /**
   * 获取风险等级对应的 Element Plus 标签类型
   * @param riskLevel 风险等级字符串
   * @returns Element Plus 标签类型
   */
  const getRiskLevelType = (riskLevel: string | undefined): string => {
    if (!riskLevel) return 'info'
    switch (riskLevel) {
      case '低风险':
        return 'success'
      case '中风险':
        return 'warning'
      case '高风险':
      case '严重':
        return 'danger'
      default:
        return 'info'
    }
  }

  return {
    getTaskStatusType,
    getTaskStatusText,
    getScoreClass,
    getScoreTagType,
    getRiskLevelType
  }
}
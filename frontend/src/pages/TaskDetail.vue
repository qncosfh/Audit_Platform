<template>
  <div class="task-detail">
    <div class="task-header">
      <div class="header-left">
        <el-page-header @back="$router.push('/tasks')" :title="task?.name || '任务详情'" />
        <el-button :icon="Refresh" @click="refreshTask" :loading="refreshing" circle />
      </div>
      
      <div class="task-actions" v-if="task">
        <el-button 
          v-if="task.status === 'pending' || task.status === 'paused' || task.status === 'failed'"
          type="primary" 
          :icon="VideoPlay"
          @click="startTask"
          :loading="loading"
        >
          {{ task.status === 'failed' ? '重新审计' : (task.status === 'paused' ? '继续审计' : '开始审计') }}
        </el-button>
        
        <el-button 
          v-if="task.status === 'running'"
          type="danger" 
          :icon="Close"
          @click="stopTask"
          :loading="loading"
        >
          停止审计
        </el-button>
        
        <el-button 
          v-if="task.status === 'completed'"
          type="success" 
          :icon="Download"
          @click="exportReport"
          :loading="loading"
        >
          导出报告
        </el-button>
        
        <el-button 
          v-if="task.status === 'paused' || task.status === 'failed' || task.status === 'completed'"
          type="danger" 
          :icon="Delete"
          @click="deleteTask"
          :loading="loading"
        >
          删除任务
        </el-button>
      </div>
    </div>

    <div class="task-content" v-if="task">
      <!-- 任务基本信息 -->
      <el-card class="task-card">
        <template #header>
          <div class="card-header">
            <span>任务信息</span>
          </div>
        </template>
        
        <div class="task-info">
          <div class="info-item">
            <span class="label">任务名称：</span>
            <span class="value">{{ task.name }}</span>
          </div>
          
          <div class="info-item">
            <span class="label">任务描述：</span>
            <span class="value">{{ task.description || '无' }}</span>
          </div>
          
          <div class="info-item">
            <span class="label">代码源：</span>
            <span class="value">{{ task.codeSource?.name || '-' }}</span>
          </div>
          
          <div class="info-item">
            <span class="label">模型配置：</span>
            <span class="value">{{ task.modelConfig?.name || '-' }}</span>
          </div>
          
          <div class="info-item">
            <span class="label">状态：</span>
            <el-tag :type="getTaskStatusType(task.status)">
              {{ getTaskStatusText(task.status) }}
            </el-tag>
          </div>
          
          <div class="info-item">
            <span class="label">进度：</span>
            <el-progress 
              :percentage="task.progress" 
              :color="getProgressColor(task.progress)"
              :status="task.status === 'completed' ? 'success' : (task.status === 'failed' ? 'exception' : 'active')"
            />
          </div>
          
          <div class="info-item">
            <span class="label">创建时间：</span>
            <span class="value">{{ formatDate(task.createdAt) }}</span>
          </div>
          
          <div class="info-item">
            <span class="label">更新时间：</span>
            <span class="value">{{ formatDate(task.updatedAt) }}</span>
          </div>
        </div>
      </el-card>

      <!-- 自定义提示词 -->
      <el-card class="prompt-card">
        <template #header>
          <div class="card-header">
            <span>自定义提示词</span>
          </div>
        </template>
        
        <div class="prompt-content">
          <el-input
            v-model="task.prompt"
            type="textarea"
            :rows="8"
            placeholder="请输入AI审计的自定义提示词"
            :disabled="task.status !== 'pending'"
          />
          
          <div class="prompt-actions" v-if="task.status === 'pending'">
            <el-button @click="updateTask">保存提示词</el-button>
          </div>
        </div>
      </el-card>

      <!-- 漏洞统计和安全评分 -->
      <el-card class="stats-card" v-if="task.status === 'completed'">
        <template #header>
          <div class="card-header">
            <span>审计统计</span>
          </div>
        </template>
        
        <div class="stats-content">
          <!-- 安全评分 -->
          <div class="security-score" v-if="task.securityScore !== undefined">
            <div class="score-circle" :class="getScoreClass(task.securityScore)">
              <span class="score-value">{{ task.securityScore }}</span>
              <span class="score-label">安全评分</span>
            </div>
            <el-tag :type="getRiskLevelType(task.riskLevel)" size="large">{{ task.riskLevel || '未知' }}</el-tag>
          </div>
          
          <!-- 漏洞统计 -->
          <div class="vuln-stats" v-if="task.vulnerabilityCount > 0">
            <div class="stat-item critical">
              <span class="stat-value">{{ task.criticalVulns || 0 }}</span>
              <span class="stat-label">严重</span>
            </div>
            <div class="stat-item high">
              <span class="stat-value">{{ task.highVulns || 0 }}</span>
              <span class="stat-label">高危</span>
            </div>
            <div class="stat-item medium">
              <span class="stat-value">{{ task.mediumVulns || 0 }}</span>
              <span class="stat-label">中危</span>
            </div>
            <div class="stat-item low">
              <span class="stat-value">{{ task.lowVulns || 0 }}</span>
              <span class="stat-label">低危</span>
            </div>
          </div>
          
          <!-- 基本统计 -->
          <div class="basic-stats">
            <div class="stat-item">
              <span class="stat-value">{{ task.scannedFiles || 0 }}</span>
              <span class="stat-label">扫描文件</span>
            </div>
            <div class="stat-item">
              <span class="stat-value">{{ task.vulnerabilityCount || 0 }}</span>
              <span class="stat-label">漏洞总数</span>
            </div>
            <div class="stat-item">
              <span class="stat-value">{{ task.duration ? (task.duration / 1000).toFixed(1) + 's' : '-' }}</span>
              <span class="stat-label">耗时</span>
            </div>
            <div class="stat-item" v-if="task.detectedLanguage">
              <span class="stat-value">{{ task.detectedLanguage }}</span>
              <span class="stat-label">语言</span>
            </div>
          </div>
        </div>
      </el-card>

      <!-- 审计结果 -->
      <el-card class="result-card" v-if="(task.status === 'completed' || task.status === 'failed') && task.result">
        <template #header>
          <div class="card-header">
            <span>{{ task.status === 'completed' ? '审计结果' : '失败原因' }}</span>
          </div>
        </template>
        
        <div class="result-content">
          <el-input
            v-model="task.result"
            type="textarea"
            :rows="15"
            readonly
            :placeholder="task.status === 'completed' ? '审计结果将在这里显示' : '失败原因将在这里显示'"
          />
        </div>
      </el-card>

      <!-- 审计日志 + AI交互日志 -->
      <el-card class="log-card" v-if="task.status === 'running' || (task.log && task.log.length > 0) || (task.aiLog && task.aiLog.length > 0)">
        <template #header>
          <div class="card-header">
            <span>审计日志</span>
            <div class="header-right">
              <el-button :icon="Refresh" size="small" @click="refreshLog" :loading="logLoading" circle />
              <el-switch v-model="autoRefresh" v-if="task.status === 'running'" active-text="自动刷新" />
            </div>
          </div>
        </template>
        
        <div class="log-content">
          <pre>{{ displayLog }}</pre>
        </div>
      </el-card>

      <!-- 调用图分析 - 使用新的CallGraph组件 -->
      <el-card class="callgraph-card" v-if="task.status === 'completed'">
        <template #header>
          <div class="card-header">
            <span>调用图分析</span>
            <div class="header-right">
              <el-button :icon="Refresh" size="small" @click="reloadCallGraph" :loading="callGraphLoading" circle />
            </div>
          </div>
        </template>
        
        <CallGraph 
          :task-id="taskId" 
          :initial-func="selectedFunc"
          :depth="3"
        />
      </el-card>
    </div>

    <!-- 加载状态 -->
    <div v-if="!task" class="loading-container">
      <el-skeleton :rows="8" animated />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useTaskStore } from '@/stores/task'
import { useTaskStatus } from '@/composables/useTaskStatus'
import { getProgressColor, formatDate } from '@/utils/common'
import {
  VideoPlay,
  Close,
  Download,
  Delete,
  Refresh,
  ZoomIn,
  Plus,
  Minus
} from '@element-plus/icons-vue'
import { ElMessageBox } from 'element-plus'

const route = useRoute()
const router = useRouter()
const taskStore = useTaskStore()
const { getTaskStatusType, getTaskStatusText, getScoreClass, getScoreTagType, getRiskLevelType } = useTaskStatus()

const loading = ref(false)
const refreshing = ref(false)
const logLoading = ref(false)
const autoRefresh = ref(true)
const taskId = computed(() => route.params.id as string)

// 定时刷新
let refreshInterval: ReturnType<typeof setInterval> | null = null

// WebSocket 连接
let ws: WebSocket | null = null

const auditSteps = ref<{ time: string; type: string; content: string }[]>([])

// 连接 WebSocket 接收进度更新
const connectWebSocket = () => {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const wsUrl = `${protocol}//${window.location.host}/ws/progress`
  
  ws = new WebSocket(wsUrl)
  
  ws.onmessage = (event) => {
    try {
      const data = JSON.parse(event.data)
      // 只处理当前任务的消息
      if (String(data.task_id) === taskId.value) {
        // 更新进度
        if (task.value) {
          task.value.progress = data.progress
          task.value.status = data.status
      // 更新 AI 日志
      if (data.aiLog) {
        task.value.aiLog = data.aiLog
      }
        }
        
        // 添加新的日志步骤
        const now = new Date()
        const timeStr = `${now.getHours().toString().padStart(2, '0')}:${now.getMinutes().toString().padStart(2, '0')}:${now.getSeconds().toString().padStart(2, '0')}`
        
        // 根据消息内容确定类型
        let type = 'info'
        if (data.message.includes('完成') || data.message.includes('成功')) {
          type = 'success'
        } else if (data.message.includes('失败') || data.message.includes('错误')) {
          type = 'danger'
        } else if (data.message.includes('正在分析') || data.message.includes('正在扫描')) {
          type = 'primary'
        }
        
        // 检查是否已存在相同消息
        const exists = auditSteps.value.some(step => step.content === data.message)
        if (!exists) {
          auditSteps.value.push({
            time: timeStr,
            type: type,
            content: data.message
          })
        }
        
        // 如果任务完成或失败，刷新任务获取最终结果
        if (data.status === 'completed' || data.status === 'failed') {
          refreshTask()
        }
      }
    } catch (error) {
      console.error('解析 WebSocket 消息失败:', error)
    }
  }
  
  ws.onclose = () => {
    console.log('WebSocket 连接关闭')
  }
  
  ws.onerror = (error) => {
    console.error('WebSocket 错误:', error)
  }
}

// 断开 WebSocket
const disconnectWebSocket = () => {
  if (ws) {
    ws.close()
    ws = null
  }
}

// 使用 ref 来确保响应式更新
const task = ref<any>(null)

// 初始化 task
onMounted(async () => {
  await refreshTask()
})

  // 合并审计日志和AI交互日志
  const displayLog = computed(() => {
    if (!task.value) return ''
    // 使用驼峰命名，后端返回的是 aiLog
    let log = task.value.log || ''
    const aiLog = task.value.aiLog || ''
    if (aiLog) {
      log += '\n\n' + aiLog
    }
    return log || '日志将在审计过程中显示...'
  })

// 刷新日志（仅刷新日志内容，不刷新整个任务）
const refreshLog = async () => {
  logLoading.value = true
  try {
    await taskStore.loadTask(taskId.value)
    if (taskStore.currentTask) {
      task.value = taskStore.currentTask
    }
  } catch (error) {
    console.error('刷新日志失败:', error)
  } finally {
    logLoading.value = false
  }
}

const startTask = async () => {
  loading.value = true
  try {
    await taskStore.startTask(taskId.value)
    ElMessage.success('任务已开始')
    // 重新加载任务状态
    await taskStore.loadTask(taskId.value)
    // 刷新页面以更新状态
    window.location.reload()
  } catch (error) {
    ElMessage.error('启动任务失败')
  } finally {
    loading.value = false
  }
}

const stopTask = async () => {
  loading.value = true
  try {
    await taskStore.stopTask(taskId.value)
    ElMessage.success('任务已停止')
    // 重新加载任务状态
    await taskStore.loadTask(taskId.value)
    // 刷新页面以更新状态
    window.location.reload()
  } catch (error) {
    ElMessage.error('停止任务失败')
  } finally {
    loading.value = false
  }
}

const exportReport = async () => {
  loading.value = true
  try {
    // 优先使用 reportPath 字段（如果存在，说明报告过大已保存到文件）
    const reportPath = task.value?.reportPath
    const content = task.value?.result
    
    if (reportPath) {
      // 报告已保存到文件，提供文件下载链接
      // 假设文件在服务器上的路径可以通过 API 下载
      const fileName = reportPath.split('/').pop() || `审计报告_${task.value?.name || 'task'}.md`
      const downloadUrl = `/api/tasks/${taskId.value}/download-report`
      
      // 使用 fetch 下载文件
      const response = await fetch(downloadUrl)
      if (response.ok) {
        const blob = await response.blob()
        const url = URL.createObjectURL(blob)
        const a = document.createElement('a')
        a.href = url
        a.download = fileName
        document.body.appendChild(a)
        a.click()
        document.body.removeChild(a)
        URL.revokeObjectURL(url)
        ElMessage.success('完整报告下载成功')
      } else {
        // 如果无法下载文件，使用 result 字段的内容
        if (content && content.trim() !== '' && !content.includes('报告内容过大，已保存到文件')) {
          downloadContent(content, fileName)
        } else {
          ElMessage.warning('无法下载完整报告，请查看数据库中的摘要内容')
        }
      }
    } else if (content && content.trim() !== '' && !content.includes('报告内容过大，已保存到文件')) {
      // 直接使用 task.result 作为报告内容
      downloadContent(content, `审计报告_${task.value?.name || 'task'}_${new Date().toISOString().slice(0,10)}.md`)
      ElMessage.success('报告导出成功')
    } else {
      ElMessage.error('报告内容为空或已保存到文件')
    }
  } catch (error) {
    console.error('导出报告失败:', error)
    ElMessage.error('导出报告失败')
  } finally {
    loading.value = false
  }
}

// 下载内容到文件
const downloadContent = (content: string, fileName: string) => {
  const blob = new Blob([content], { type: 'text/markdown;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = fileName
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
}

const updateTask = async () => {
  if (!task.value) return
  
  loading.value = true
  try {
    await taskStore.updateTask(taskId.value, { prompt: task.value.prompt })
    ElMessage.success('提示词已保存')
  } catch (error) {
    ElMessage.error('保存失败')
  } finally {
    loading.value = false
  }
}

const refreshTask = async () => {
  refreshing.value = true
  try {
    await taskStore.loadTask(taskId.value)
    
    // 直接使用 store 中的 currentTask（这是 loadTask 返回的最新数据）
    if (taskStore.currentTask) {
      task.value = taskStore.currentTask
    }
    
    // 如果任务正在运行，重新连接 WebSocket 获取最新日志
    if (task.value?.status === 'running') {
      // 断开旧连接
      disconnectWebSocket()
      // 重新连接
      connectWebSocket()
    }
  } catch (error) {
    console.error('刷新任务失败:', error)
  } finally {
    refreshing.value = false
  }
}

const deleteTask = async () => {
  try {
    await ElMessageBox.confirm('确定要删除这个任务吗？', '警告', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    })
    
    loading.value = true
    await taskStore.deleteTask(taskId.value)
    ElMessage.success('删除成功')
    router.push('/tasks')
  } catch (error) {
    if (error !== 'cancel') {
      console.error('删除任务失败:', error)
      ElMessage.error('删除失败')
    }
  } finally {
    loading.value = false
  }
}

// 监听自动刷新开关
watch([autoRefresh, () => task.value?.status], ([newAutoRefresh, newStatus]) => {
  if (newAutoRefresh && newStatus === 'running') {
    // 启动定时刷新
    if (refreshInterval) {
      clearInterval(refreshInterval)
    }
    refreshInterval = setInterval(() => {
      refreshTask()
    }, 3000) // 每3秒刷新一次
  } else {
    // 停止定时刷新
    if (refreshInterval) {
      clearInterval(refreshInterval)
      refreshInterval = null
    }
  }
})

// 监听任务状态，启动定时刷新
watch(() => task.value?.status, (newStatus) => {
  if (newStatus === 'running') {
    connectWebSocket()
    // 启动定时刷新
    if (refreshInterval) {
      clearInterval(refreshInterval)
    }
    refreshInterval = setInterval(() => {
      if (autoRefresh.value) {
        refreshTask()
      }
    }, 3000) // 每3秒刷新一次
  } else {
    // 停止定时刷新
    if (refreshInterval) {
      clearInterval(refreshInterval)
      refreshInterval = null
    }
  }
})

// 组件卸载时断开 WebSocket
onUnmounted(() => {
  disconnectWebSocket()
  if (refreshInterval) {
    clearInterval(refreshInterval)
  }
})

// ==================== 调用图相关功能 ====================
import CallGraph from '@/components/CallGraph.vue'

const selectedFunc = ref('')
const callGraphLoading = ref(false)

// 重新加载调用图
const reloadCallGraph = () => {
  // CallGraph 组件内部会处理重新加载
  // 触发组件重新渲染
  const callGraphEl = document.querySelector('.callgraph-card')
  if (callGraphEl) {
    callGraphEl.scrollIntoView({ behavior: 'smooth' })
  }
}
</script>

<style scoped>
.task-detail {
  padding: 24px;
}

.task-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24px;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 12px;
}

.task-actions {
  display: flex;
  gap: 12px;
}

.task-content {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.log-card, .ai-log-card {
  margin-top: 16px;
}

.log-content, .ai-log-content {
  max-height: 80vh;
  min-height: 500px;
  overflow-y: auto;
  background-color: #1e1e1e;
  border-radius: 4px;
  padding: 12px;
}

.prompt-card {
  max-height: 150px;
}

.prompt-card :deep(.el-card__body) {
  padding: 12px;
}

.prompt-card :deep(.el-textarea__inner) {
  max-height: 80px;
}

.log-content pre, .ai-log-content pre {
  margin: 0;
  white-space: pre-wrap;
  word-wrap: break-word;
  color: #d4d4d4;
  font-family: 'Courier New', Courier, monospace;
  font-size: 12px;
  line-height: 1.5;
}

.ai-log-content :deep(.el-textarea__inner) {
  background-color: #1e1e1e;
  color: #d4d4d4;
  font-family: 'Courier New', Courier, monospace;
  font-size: 12px;
  line-height: 1.5;
  min-height: 300px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.header-right {
  display: flex;
  align-items: center;
  gap: 12px;
}

.task-info {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
  gap: 16px;
}

.info-item {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.label {
  font-size: 12px;
  color: #6b7280;
}

.value {
  font-size: 16px;
  font-weight: 500;
  color: #1f2937;
}

.prompt-content {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.prompt-actions {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}

.result-content, .process-content {
  min-height: 200px;
}

.loading-container {
  padding: 24px;
}

/* 审计统计卡片样式 */
.stats-content {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.security-score {
  display: flex;
  align-items: center;
  gap: 20px;
}

.score-circle {
  width: 80px;
  height: 80px;
  border-radius: 50%;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  color: white;
}

.score-circle.score-high {
  background: linear-gradient(135deg, #67c23a 0%, #85ce61 100%);
}

.score-circle.score-medium {
  background: linear-gradient(135deg, #e6a23c 0%, #ebb563 100%);
}

.score-circle.score-low {
  background: linear-gradient(135deg, #e6a23c 0%, #f56c6c 100%);
}

.score-circle.score-critical {
  background: linear-gradient(135deg, #f56c6c 0%, #fa5555 100%);
}

.score-value {
  font-size: 24px;
  font-weight: bold;
}

.score-label {
  font-size: 10px;
  opacity: 0.9;
}

.vuln-stats, .basic-stats {
  display: flex;
  gap: 16px;
  flex-wrap: wrap;
}

.stat-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 12px 20px;
  border-radius: 8px;
  background-color: #f5f7fa;
  min-width: 80px;
}

.stat-item.critical {
  background-color: #fef0f0;
}

.stat-item.critical .stat-value {
  color: #f56c6c;
}

.stat-item.high {
  background-color: #fef0f0;
}

.stat-item.high .stat-value {
  color: #e6a23c;
}

.stat-item.medium {
  background-color: #fdf6ec;
}

.stat-item.medium .stat-value {
  color: #e6a23c;
}

.stat-item.low {
  background-color: #f0f9eb;
}

.stat-item.low .stat-value {
  color: #67c23a;
}

.stat-value {
  font-size: 24px;
  font-weight: bold;
  color: #303133;
}

.stat-label {
  font-size: 12px;
  color: #909399;
  margin-top: 4px;
}

/* 调用图样式 - 由 CallGraph 组件内部处理 */
.callgraph-card {
  margin-top: 16px;
}

@media (max-width: 768px) {
  .task-content {
    grid-template-columns: 1fr;
  }
  
  .task-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 16px;
  }
  
  .task-actions {
    width: 100%;
    justify-content: flex-end;
  }
}
</style>

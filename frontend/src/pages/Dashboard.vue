<template>
  <div class="dashboard">
    <!-- 漏洞态势 -->
    <div class="vulnerability-section">
      <el-card class="vuln-overview-card">
        <template #header>
          <div class="card-header">
            <div class="header-left">
              <el-icon class="header-icon"><Warning /></el-icon>
              <span>{{ $t('dashboard.vulnerabilityOverview') }}</span>
            </div>
            <el-tag type="danger" effect="dark" round>
              {{ totalVulnerabilities }} {{ $t('analysis.vulnerabilities') }}
            </el-tag>
          </div>
        </template>
        
        <div class="vuln-stats-grid">
          <div class="vuln-stat-item critical">
            <div class="vuln-count">{{ vulnerabilityStats.critical }}</div>
            <div class="vuln-label">
              <el-icon><WarningFilled /></el-icon>
              {{ $t('analysis.critical') }}
            </div>
            <div class="vuln-bar">
              <div class="vuln-bar-fill" :style="{ width: getVulnPercent('critical') + '%' }"></div>
            </div>
          </div>
          
          <div class="vuln-stat-item high">
            <div class="vuln-count">{{ vulnerabilityStats.high }}</div>
            <div class="vuln-label">
              <el-icon><WarningFilled /></el-icon>
              {{ $t('analysis.high') }}
            </div>
            <div class="vuln-bar">
              <div class="vuln-bar-fill" :style="{ width: getVulnPercent('high') + '%' }"></div>
            </div>
          </div>
          
          <div class="vuln-stat-item medium">
            <div class="vuln-count">{{ vulnerabilityStats.medium }}</div>
            <div class="vuln-label">
              <el-icon><WarningFilled /></el-icon>
              {{ $t('analysis.medium') }}
            </div>
            <div class="vuln-bar">
              <div class="vuln-bar-fill" :style="{ width: getVulnPercent('medium') + '%' }"></div>
            </div>
          </div>
          
          <div class="vuln-stat-item low">
            <div class="vuln-count">{{ vulnerabilityStats.low }}</div>
            <div class="vuln-label">
              <el-icon><InfoFilled /></el-icon>
              {{ $t('analysis.low') }}
            </div>
            <div class="vuln-bar">
              <div class="vuln-bar-fill" :style="{ width: getVulnPercent('low') + '%' }"></div>
            </div>
          </div>
        </div>
      </el-card>
    </div>

    <!-- 任务视图 -->
    <div class="cosmos-section">
      <el-card class="cosmos-card">
        <template #header>
          <div class="card-header">
            <div class="header-left">
              <el-icon class="header-icon"><Sunrise /></el-icon>
              <span>{{ $t('dashboard.taskOverview') }}</span>
            </div>
            <span class="cosmos-hint">{{ $t('dashboard.clickToView') }}</span>
          </div>
        </template>
        
        <div class="cosmos-container" ref="cosmosRef">
          <!-- 宇宙背景 -->
          <div class="cosmos-bg">
            <div 
              v-for="i in 100" 
              :key="i" 
              class="star"
              :style="{
                left: Math.random() * 100 + '%',
                top: Math.random() * 100 + '%',
                animationDelay: Math.random() * 3 + 's',
                opacity: Math.random() * 0.5 + 0.3
              }"
            ></div>
          </div>
          
          <!-- 原子/任务 -->
          <div 
            v-for="(task, index) in taskAtoms" 
            :key="task.id"
            class="atom"
            :class="[task.status, getAtomSizeClass(task)]"
            :style="getAtomPosition(index)"
            @click="showTaskDetail(task)"
          >
            <div class="atom-core">
              <div class="atom-glow"></div>
            </div>
            <div class="atom-orbit" v-if="task.status === 'running'">
              <div class="orbit-electron"></div>
            </div>
            <div class="atom-label">{{ task.name }}</div>
          </div>
          
          <!-- 中心太阳 -->
          <div class="sun">
            <div class="sun-core"></div>
            <div class="sun-glow"></div>
          </div>
        </div>
      </el-card>
    </div>

    <!-- 系统监控 -->
    <div class="monitor-section">
      <el-row :gutter="16">
        <el-col :span="6">
          <div class="monitor-card">
            <div class="monitor-header">
              <el-icon><Monitor /></el-icon>
              <span>{{ $t('dashboard.cpu') }}</span>
            </div>
            <div class="monitor-value">{{ cpuUsage.toFixed(2) }}%</div>
            <el-progress 
              :percentage="cpuUsage" 
              :stroke-width="6"
              :color="getProgressColor(cpuUsage)"
              :show-text="false"
            />
          </div>
        </el-col>
        <el-col :span="6">
          <div class="monitor-card">
            <div class="monitor-header">
              <el-icon><CircleCheck /></el-icon>
              <span>{{ $t('dashboard.disk') }}</span>
            </div>
            <div class="monitor-value">{{ diskUsage.toFixed(2) }}%</div>
            <el-progress 
              :percentage="diskUsage" 
              :stroke-width="6"
              :color="getProgressColor(diskUsage)"
              :show-text="false"
            />
          </div>
        </el-col>
        <el-col :span="6">
          <div class="monitor-card">
            <div class="monitor-header">
              <el-icon><Odometer /></el-icon>
              <span>{{ $t('dashboard.memory') }}</span>
            </div>
            <div class="monitor-value">{{ memoryUsage.toFixed(2) }}%</div>
            <el-progress 
              :percentage="memoryUsage" 
              :stroke-width="6"
              :color="getProgressColor(memoryUsage)"
              :show-text="false"
            />
          </div>
        </el-col>
        <el-col :span="6">
          <div class="monitor-card">
            <div class="monitor-header">
              <el-icon><Connection /></el-icon>
              <span>{{ $t('dashboard.network') }}</span>
            </div>
            <div class="monitor-value">{{ networkTraffic.toFixed(2) }} MB/s</div>
            <div class="network-graph">
              <div 
                v-for="(value, index) in networkHistory" 
                :key="index"
                class="graph-bar"
                :style="{ height: value + '%' }"
              ></div>
            </div>
          </div>
        </el-col>
      </el-row>
    </div>

    <!-- 任务详情对话框 -->
    <el-dialog
      v-model="showTaskDialog"
      :title="selectedTask?.name"
      width="500px"
      class="task-detail-dialog"
    >
      <div class="task-detail-content" v-if="selectedTask">
        <div class="detail-row">
          <span class="detail-label">{{ $t('task.status') }}</span>
          <el-tag :type="getTaskStatusType(selectedTask.status)" size="small">
            {{ getTaskStatusText(selectedTask.status) }}
          </el-tag>
        </div>
        <div class="detail-row">
          <span class="detail-label">{{ $t('dashboard.startDate') }}</span>
          <span class="detail-value">{{ formatDate(selectedTask.createdAt) }}</span>
        </div>
        <div class="detail-row">
          <span class="detail-label">{{ $t('dashboard.endDate') }}</span>
          <span class="detail-value">{{ selectedTask.completedAt ? formatDate(selectedTask.completedAt) : '-' }}</span>
        </div>
        <div class="detail-row">
          <span class="detail-label">{{ $t('analysis.vulnerabilityCount') }}</span>
          <span class="detail-value">
            <el-tag type="danger" size="small" v-if="selectedTask.criticalVulns">{{ selectedTask.criticalVulns }} {{ $t('analysis.critical') }}</el-tag>
            <el-tag type="warning" size="small" v-if="selectedTask.highVulns">{{ selectedTask.highVulns }} {{ $t('analysis.high') }}</el-tag>
            <el-tag type="info" size="small" v-if="selectedTask.mediumVulns">{{ selectedTask.mediumVulns }} {{ $t('analysis.medium') }}</el-tag>
            <el-tag size="small" v-if="selectedTask.lowVulns">{{ selectedTask.lowVulns }} {{ $t('analysis.low') }}</el-tag>
            <span v-if="!selectedTask.criticalVulns && !selectedTask.highVulns">-</span>
          </span>
        </div>
        <div class="detail-row">
          <span class="detail-label">{{ $t('codeSource.title') }}</span>
          <span class="detail-value">{{ selectedTask.codeSource?.name || '-' }}</span>
        </div>
        <div class="detail-row">
          <span class="detail-label">{{ $t('model.title') }}</span>
          <span class="detail-value">{{ selectedTask.modelConfig?.name || '-' }}</span>
        </div>
      </div>
      
      <template #footer>
        <el-button @click="showTaskDialog = false">{{ $t('common.cancel') }}</el-button>
        <el-button type="primary" @click="viewTaskDetail">
          {{ $t('task.view') }}
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useTaskStore } from '@/stores/task'
import { useTaskStatus } from '@/composables/useTaskStatus'
import { getProgressColor, formatDate } from '@/utils/common'
import {
  Monitor,
  CircleCheck,
  Odometer,
  Connection,
  Warning,
  WarningFilled,
  InfoFilled,
  Sunrise
} from '@element-plus/icons-vue'

const { t } = useI18n()
const { getTaskStatusType, getTaskStatusText } = useTaskStatus()
const router = useRouter()
const taskStore = useTaskStore()

// 系统监控数据
const cpuUsage = ref(35)
const diskUsage = ref(62)
const memoryUsage = ref(48)
const networkTraffic = ref(12.5)
const networkHistory = ref<number[]>(Array(20).fill(30))

// 漏洞态势数据
const vulnerabilityStats = computed(() => {
  let critical = 0
  let high = 0
  let medium = 0
  let low = 0
  
  taskStore.tasks.forEach(task => {
    if (task.status === 'completed') {
      critical += task.criticalVulns || 0
      high += task.highVulns || 0
      medium += task.mediumVulns || 0
      low += task.lowVulns || 0
    }
  })
  
  return { critical, high, medium, low }
})

const totalVulnerabilities = computed(() => {
  return vulnerabilityStats.value.critical + vulnerabilityStats.value.high + 
         vulnerabilityStats.value.medium + vulnerabilityStats.value.low
})

const getVulnPercent = (severity: string) => {
  const total = totalVulnerabilities.value
  if (total === 0) return 0
  return Math.round((vulnerabilityStats.value[severity as keyof typeof vulnerabilityStats.value] / total) * 100)
}

// 任务原子数据
const taskAtoms = computed(() => {
  return taskStore.tasks.slice(0, 20).map(task => ({
    ...task,
    x: Math.random() * 80 + 10,
    y: Math.random() * 80 + 10
  }))
})

// 任务详情对话框
const showTaskDialog = ref(false)
const selectedTask = ref<any>(null)

const getAtomPosition = (index: number) => {
  const task = taskAtoms.value[index]
  if (!task) return {}
  return {
    left: task.x + '%',
    top: task.y + '%'
  }
}

const getAtomSizeClass = (task: any) => {
  const vulns = (task.criticalVulns || 0) + (task.highVulns || 0) + 
                (task.mediumVulns || 0) + (task.lowVulns || 0)
  if (vulns > 10) return 'large'
  if (vulns > 5) return 'medium'
  return 'small'
}

const showTaskDetail = (task: any) => {
  selectedTask.value = task
  showTaskDialog.value = true
}

const viewTaskDetail = () => {
  showTaskDialog.value = false
  if (selectedTask.value) {
    router.push(`/tasks/${selectedTask.value.id}`)
  }
}

// 模拟系统监控数据更新
let monitorInterval: number

const updateMonitorData = () => {
  cpuUsage.value = Math.max(10, Math.min(90, cpuUsage.value + Math.random() * 20 - 10))
  memoryUsage.value = Math.max(20, Math.min(90, memoryUsage.value + Math.random() * 15 - 7.5))
  diskUsage.value = Math.max(30, Math.min(95, diskUsage.value + Math.random() * 5 - 2.5))
  networkTraffic.value = Math.max(1, networkTraffic.value + Math.random() * 10 - 5)
  
  networkHistory.value = [...networkHistory.value.slice(1), Math.random() * 80 + 20]
}

const loadDashboardData = async () => {
  try {
    await Promise.all([
      taskStore.loadTasks(1, 20),
      taskStore.loadCodeSources(),
      taskStore.loadModels()
    ])
  } catch (error) {
    console.error('加载数据失败:', error)
  }
}

onMounted(() => {
  loadDashboardData()
  monitorInterval = window.setInterval(updateMonitorData, 3000)
})

onUnmounted(() => {
  if (monitorInterval) {
    clearInterval(monitorInterval)
  }
})
</script>

<style scoped>
.dashboard {
  padding: 24px;
  max-width: 1600px;
  margin: 0 auto;
}

/* 系统监控 */
.monitor-section {
  margin-bottom: 24px;
}

.monitor-card {
  background: #ffffff;
  border-radius: 12px;
  padding: 20px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
}

.monitor-header {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  color: #6b7280;
  margin-bottom: 12px;
}

.monitor-header .el-icon {
  font-size: 18px;
  color: #3b82f6;
}

.monitor-value {
  font-size: 28px;
  font-weight: 700;
  color: #111827;
  margin-bottom: 12px;
}

.network-graph {
  display: flex;
  align-items: flex-end;
  gap: 2px;
  height: 40px;
}

.graph-bar {
  flex: 1;
  background: linear-gradient(to top, #3b82f6, #60a5fa);
  border-radius: 2px 2px 0 0;
  min-height: 4px;
  transition: height 0.3s ease;
}

/* 漏洞态势 */
.vulnerability-section {
  margin-bottom: 24px;
}

.vuln-overview-card {
  border-radius: 12px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 8px;
}

.header-icon {
  font-size: 18px;
  color: #3b82f6;
}

.vuln-stats-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 16px;
}

.vuln-stat-item {
  text-align: center;
  padding: 16px;
  border-radius: 10px;
}

.vuln-stat-item.critical { background: #fef2f2; }
.vuln-stat-item.high { background: #fff7ed; }
.vuln-stat-item.medium { background: #fefce8; }
.vuln-stat-item.low { background: #f0fdf4; }

.vuln-count {
  font-size: 32px;
  font-weight: 700;
}

.vuln-stat-item.critical .vuln-count { color: #dc2626; }
.vuln-stat-item.high .vuln-count { color: #ea580c; }
.vuln-stat-item.medium .vuln-count { color: #ca8a04; }
.vuln-stat-item.low .vuln-count { color: #16a34a; }

.vuln-label {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 4px;
  font-size: 14px;
  color: #6b7280;
  margin: 8px 0;
}

.vuln-bar {
  height: 4px;
  background: #e5e7eb;
  border-radius: 2px;
  overflow: hidden;
}

.vuln-bar-fill {
  height: 100%;
  border-radius: 2px;
  transition: width 0.5s ease;
}

.vuln-stat-item.critical .vuln-bar-fill { background: #dc2626; }
.vuln-stat-item.high .vuln-bar-fill { background: #ea580c; }
.vuln-stat-item.medium .vuln-bar-fill { background: #eab308; }
.vuln-stat-item.low .vuln-bar-fill { background: #22c55e; }

/* 宇宙视图 */
.cosmos-section {
  margin-bottom: 24px;
}

.cosmos-card {
  border-radius: 12px;
}

.cosmos-hint {
  font-size: 12px;
  color: #9ca3af;
}

.cosmos-container {
  position: relative;
  height: 500px;
  background: radial-gradient(ellipse at center, #1a1a2e 0%, #0f0f1a 50%, #000000 100%);
  border-radius: 12px;
  overflow: hidden;
  cursor: default;
  z-index: 1;
}

/* 星空背景 */
.cosmos-bg {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
}

.star {
  position: absolute;
  width: 2px;
  height: 2px;
  background: #ffffff;
  border-radius: 50%;
  animation: twinkle 3s ease-in-out infinite;
}

@keyframes twinkle {
  0%, 100% { opacity: 0.3; transform: scale(1); }
  50% { opacity: 1; transform: scale(1.5); }
}

/* 中心太阳 */
.sun {
  position: absolute;
  left: 50%;
  top: 50%;
  transform: translate(-50%, -50%);
  width: 80px;
  height: 80px;
}

.sun-core {
  position: absolute;
  width: 40px;
  height: 40px;
  background: radial-gradient(circle, #ffd700 0%, #ff8c00 50%, #ff4500 100%);
  border-radius: 50%;
  left: 50%;
  top: 50%;
  transform: translate(-50%, -50%);
  box-shadow: 0 0 30px #ff8c00, 0 0 60px #ff4500;
}

.sun-glow {
  position: absolute;
  width: 80px;
  height: 80px;
  background: radial-gradient(circle, rgba(255, 200, 0, 0.3) 0%, transparent 70%);
  border-radius: 50%;
  left: 50%;
  top: 50%;
  transform: translate(-50%, -50%);
  animation: sunPulse 4s ease-in-out infinite;
}

@keyframes sunPulse {
  0%, 100% { transform: translate(-50%, -50%) scale(1); opacity: 0.5; }
  50% { transform: translate(-50%, -50%) scale(1.2); opacity: 0.8; }
}

/* 原子/任务 */
.atom {
  position: absolute;
  transform: translate(-50%, -50%);
  cursor: pointer;
  z-index: 10;
  transition: all 0.3s ease;
}

.atom:hover {
  z-index: 20;
  transform: translate(-50%, -50%) scale(1.2);
}

.atom-core {
  width: 24px;
  height: 24px;
  border-radius: 50%;
  position: relative;
}

.atom.small .atom-core {
  width: 16px;
  height: 16px;
}

.atom.medium .atom-core {
  width: 24px;
  height: 24px;
}

.atom.large .atom-core {
  width: 32px;
  height: 32px;
}

.atom.pending .atom-core {
  background: radial-gradient(circle, #9ca3af 0%, #6b7280 100%);
  box-shadow: 0 0 10px rgba(156, 163, 175, 0.5);
}

.atom.running .atom-core {
  background: radial-gradient(circle, #60a5fa 0%, #3b82f6 100%);
  box-shadow: 0 0 15px rgba(59, 130, 246, 0.7);
}

.atom.completed .atom-core {
  background: radial-gradient(circle, #34d399 0%, #10b981 100%);
  box-shadow: 0 0 15px rgba(16, 185, 129, 0.7);
}

.atom.failed .atom-core {
  background: radial-gradient(circle, #f87171 0%, #ef4444 100%);
  box-shadow: 0 0 15px rgba(239, 68, 68, 0.7);
}

.atom-glow {
  position: absolute;
  width: 100%;
  height: 100%;
  border-radius: 50%;
  animation: atomGlow 2s ease-in-out infinite;
}

.atom.pending .atom-glow { background: rgba(156, 163, 175, 0.3); }
.atom.running .atom-glow { background: rgba(59, 130, 246, 0.3); }
.atom.completed .atom-glow { background: rgba(16, 185, 129, 0.3); }
.atom.failed .atom-glow { background: rgba(239, 68, 68, 0.3); }

@keyframes atomGlow {
  0%, 100% { transform: scale(1); opacity: 0.5; }
  50% { transform: scale(1.5); opacity: 0; }
}

/* 电子轨道 */
.atom-orbit {
  position: absolute;
  width: 40px;
  height: 40px;
  border: 1px dashed rgba(59, 130, 246, 0.3);
  border-radius: 50%;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  animation: orbitRotate 3s linear infinite;
}

.atom.large .atom-orbit {
  width: 50px;
  height: 50px;
}

.orbit-electron {
  position: absolute;
  width: 6px;
  height: 6px;
  background: #60a5fa;
  border-radius: 50%;
  top: -3px;
  left: 50%;
  transform: translateX(-50%);
  box-shadow: 0 0 6px #60a5fa;
}

@keyframes orbitRotate {
  from { transform: translate(-50%, -50%) rotate(0deg); }
  to { transform: translate(-50%, -50%) rotate(360deg); }
}

.atom-label {
  position: absolute;
  top: 100%;
  left: 50%;
  transform: translateX(-50%);
  margin-top: 8px;
  white-space: nowrap;
  font-size: 11px;
  color: #e5e7eb;
  text-shadow: 0 1px 2px rgba(0, 0, 0, 0.5);
  max-width: 80px;
  overflow: hidden;
  text-overflow: ellipsis;
}

/* 任务详情对话框 */
.task-detail-content {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.detail-row {
  display: flex;
  align-items: center;
  gap: 16px;
}

.detail-label {
  width: 80px;
  font-size: 14px;
  color: #6b7280;
}

.detail-value {
  flex: 1;
  font-size: 14px;
  color: #111827;
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

/* 响应式 */
@media (max-width: 1200px) {
  .vuln-stats-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (max-width: 768px) {
  .cosmos-container {
    height: 400px;
  }
}
</style>
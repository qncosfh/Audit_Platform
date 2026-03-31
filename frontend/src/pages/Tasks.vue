<template>
  <div class="tasks-page">
    <!-- 页面头部 -->
    <div class="page-header">
      <div class="header-left">
        <h1>{{ $t('task.title') }}</h1>
        <p>{{ $t('task.pageDesc') }}</p>
      </div>
      <div class="header-actions">
        <el-button type="primary" size="large" @click="$router.push('/tasks/create')">
          <el-icon><Plus /></el-icon>
          {{ $t('task.create') }}
        </el-button>
      </div>
    </div>

    <!-- 统计卡片 -->
    <div class="stats-row">
      <div class="stat-card">
        <div class="stat-icon pending">
          <el-icon><Clock /></el-icon>
        </div>
        <div class="stat-content">
          <div class="stat-value">{{ statusCounts.pending }}</div>
          <div class="stat-label">{{ $t('task.pending') }}</div>
        </div>
      </div>
      
      <div class="stat-card">
        <div class="stat-icon running">
          <el-icon class="animate-spin"><Loading /></el-icon>
        </div>
        <div class="stat-content">
          <div class="stat-value">{{ statusCounts.running }}</div>
          <div class="stat-label">{{ $t('task.running') }}</div>
        </div>
      </div>
      
      <div class="stat-card">
        <div class="stat-icon completed">
          <el-icon><CircleCheck /></el-icon>
        </div>
        <div class="stat-content">
          <div class="stat-value">{{ statusCounts.completed }}</div>
          <div class="stat-label">{{ $t('task.completed') }}</div>
        </div>
      </div>
      
      <div class="stat-card">
        <div class="stat-icon failed">
          <el-icon><CircleClose /></el-icon>
        </div>
        <div class="stat-content">
          <div class="stat-value">{{ statusCounts.failed }}</div>
          <div class="stat-label">{{ $t('task.failed') }}</div>
        </div>
      </div>
    </div>

    <!-- 筛选栏 -->
    <el-card class="filter-card">
      <div class="filter-bar">
        <el-input
          v-model="searchQuery"
          :placeholder="$t('task.searchPlaceholder')"
          :prefix-icon="Search"
          class="search-input"
          clearable
          @input="handleSearch"
        />
        
        <el-select v-model="filterStatus" :placeholder="$t('task.statusFilter')" clearable class="status-select">
          <el-option :label="$t('task.allStatus')" value="" />
          <el-option :label="$t('task.pending')" value="pending" />
          <el-option :label="$t('task.running')" value="running" />
          <el-option :label="$t('task.paused')" value="paused" />
          <el-option :label="$t('task.completed')" value="completed" />
          <el-option :label="$t('task.failed')" value="failed" />
        </el-select>
        
        <el-button type="primary" :icon="Refresh" @click="loadTasks" :loading="loading">
          {{ $t('common.refresh') }}
        </el-button>
        
        <el-button :icon="Select" @click="selectAll" v-if="filteredTasks.length > 0">
          {{ selectedTasks.length === paginatedTasks.length && paginatedTasks.length > 0 ? $t('task.deselectAll') : $t('task.selectAll') }}
        </el-button>
        
        <!-- 批量操作按钮 -->
        <div class="batch-actions" v-if="selectedTasks.length > 0">
          <span class="selected-count">{{ $t('task.selectedCount').replace('{count}', String(selectedTasks.length)) }}</span>
          <el-button type="danger" size="small" :icon="Delete" @click="batchDelete">
            {{ $t('task.batchDelete') }}
          </el-button>
          <el-button size="small" @click="clearSelection">{{ $t('common.cancel') }}</el-button>
        </div>
      </div>
    </el-card>

    <!-- 任务列表 -->
    <el-card class="tasks-card">
      <div v-if="loading" class="loading-state">
        <el-skeleton :rows="5" animated />
      </div>
      
      <div v-else-if="filteredTasks.length === 0" class="empty-state">
        <el-empty :description="$t('task.noTasks')" :image-size="140">
          <el-button type="primary" @click="$router.push('/tasks/create')">
            {{ $t('task.createFirst') }}
          </el-button>
        </el-empty>
      </div>
      
      <div v-else class="tasks-table">
        <el-table :data="paginatedTasks" style="width: 100%" row-class-name="table-row" :header-cell-style="headerCellStyle" :cell-style="cellStyle" @row-click="handleRowClick">
          <el-table-column type="selection" width="40" />
          
          <el-table-column prop="name" :label="$t('task.name')" min-width="150" show-overflow-tooltip />
          
          <el-table-column prop="description" :label="$t('task.description')" min-width="150" show-overflow-tooltip>
            <template #default="{ row }">
              <span>{{ row.description || '-' }}</span>
            </template>
          </el-table-column>
          
          <el-table-column :label="$t('nav.codeSources')" min-width="100">
            <template #default="{ row }">
              <div class="cell-with-icon">
                <el-icon><Folder /></el-icon>
                <span>{{ row.codeSource?.name || '-' }}</span>
              </div>
            </template>
          </el-table-column>
          
          <el-table-column :label="$t('model.title')" min-width="100">
            <template #default="{ row }">
              <div class="cell-with-icon">
                <el-icon><Cpu /></el-icon>
                <span>{{ row.modelConfig?.name || '-' }}</span>
              </div>
            </template>
          </el-table-column>
          
          <el-table-column prop="status" :label="$t('task.status')" width="100">
            <template #default="{ row }">
              <el-tag :type="getStatusType(row.status)" size="small" effect="dark" round>
                {{ getStatusText(row.status) }}
              </el-tag>
            </template>
          </el-table-column>
          
          <el-table-column :label="$t('task.progress')" width="100">
            <template #default="{ row }">
              <el-progress 
                v-if="row.status === 'running'"
                :percentage="row.progress" 
                :stroke-width="6"
                :color="getProgressColor(row.progress)"
              />
              <span v-else class="progress-text">{{ row.progress }}%</span>
            </template>
          </el-table-column>
          
          <el-table-column :label="$t('analysis.vulnerabilities')" min-width="150">
            <template #default="{ row }">
              <div class="vuln-cell">
                <template v-if="row.status === 'completed'">
                  <el-tag v-if="row.securityScore !== undefined" size="small" :type="getScoreTagType(row.securityScore)">
                    {{ row.securityScore }}{{ $t('task.scoreUnit') }}
                  </el-tag>
                  <el-tag v-if="row.criticalVulns > 0" type="danger" size="small">{{ row.criticalVulns }}{{ $t('analysis.critical') }}</el-tag>
                  <el-tag v-if="row.highVulns > 0" type="warning" size="small">{{ row.highVulns }}{{ $t('analysis.high') }}</el-tag>
                  <el-tag v-if="row.mediumVulns > 0" type="info" size="small">{{ row.mediumVulns }}{{ $t('analysis.medium') }}</el-tag>
                  <el-tag v-if="row.lowVulns > 0" size="small">{{ row.lowVulns }}{{ $t('analysis.low') }}</el-tag>
                  <span v-if="!row.securityScore && !row.criticalVulns && !row.highVulns" class="vuln-count">
                    {{ row.vulnerabilityCount || 0 }}{{ $t('task.vulnCount') }}
                  </span>
                </template>
                <template v-else>
                  <span class="vuln-count">{{ row.vulnerabilityCount || 0 }}{{ $t('task.vulnCount') }}</span>
                </template>
              </div>
            </template>
          </el-table-column>
          
          <el-table-column :label="$t('task.files')" width="70">
            <template #default="{ row }">
              <span>{{ row.scannedFiles || 0 }}{{ $t('task.filesUnit') }}</span>
            </template>
          </el-table-column>
          
          <el-table-column :label="$t('task.duration')" width="70">
            <template #default="{ row }">
              <span>{{ formatDuration(row.duration) }}</span>
            </template>
          </el-table-column>
          
          <el-table-column prop="createdAt" :label="$t('task.createdAt')" width="90">
            <template #default="{ row }">
              <span>{{ formatDateUtil(row.createdAt) }}</span>
            </template>
          </el-table-column>
          
          <el-table-column :label="$t('common.actions')" width="80" fixed="right">
            <template #default="{ row }">
              <el-dropdown trigger="click" @command="(cmd: string) => handleCommand(cmd, row)">
                <el-button text size="small">
                  <el-icon><MoreFilled /></el-icon>
                </el-button>
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item command="start" v-if="row.status === 'pending'">
                      <el-icon><VideoPlay /></el-icon> {{ $t('task.start') }}
                    </el-dropdown-item>
                    <el-dropdown-item command="stop" v-if="row.status === 'running'">
                      <el-icon><VideoPause /></el-icon> {{ $t('task.stop') }}
                    </el-dropdown-item>
                    <el-dropdown-item command="export" v-if="row.status === 'completed'">
                      <el-icon><Download /></el-icon> {{ $t('task.export') }}
                    </el-dropdown-item>
                    <el-dropdown-item command="delete" divided>
                      <el-icon><Delete /></el-icon> {{ $t('task.delete') }}
                    </el-dropdown-item>
                  </el-dropdown-menu>
                </template>
              </el-dropdown>
            </template>
          </el-table-column>
        </el-table>
      </div>
      
      <!-- 分页 -->
      <div class="pagination-wrapper" v-if="filteredTasks.length > 0">
        <div class="pagination-info">
          <span>{{ $t('task.pageInfo').replace('{current}', String(currentPage)).replace('{total}', String(totalPages)) }}</span>
          <span class="total-count">{{ $t('task.totalCount').replace('{count}', String(filteredTasks.length)) }}</span>
        </div>
        <el-pagination
          v-model:current-page="currentPage"
          v-model:page-size="pageSize"
          :page-sizes="[10, 20, 50]"
          :total="filteredTasks.length"
          layout="sizes, prev, pager, next, jumper"
          background
        />
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useTaskStore } from '@/stores/task'
import { useTaskStatus } from '@/composables/useTaskStatus'
import { getProgressColor, formatDuration, formatDate as formatDateUtil } from '@/utils/common'
import {
  Plus,
  Search,
  Refresh,
  Clock,
  Loading,
  CircleCheck,
  CircleClose,
  Folder,
  Cpu,
  VideoPlay,
  VideoPause,
  Download,
  Delete,
  MoreFilled,
  Select
} from '@element-plus/icons-vue'

const { t } = useI18n()
const { getTaskStatusType: getStatusType, getTaskStatusText: getStatusText, getScoreTagType } = useTaskStatus()
const router = useRouter()
const taskStore = useTaskStore()

const loading = ref(false)
const searchQuery = ref('')
const filterStatus = ref('')
const currentPage = ref(1)
const pageSize = ref(10)
const selectedTasks = ref<number[]>([])

const tasks = computed(() => taskStore.tasks)

// 表头样式 - 居中显示
const headerCellStyle = () => {
  return {
    textAlign: 'center',
    backgroundColor: '#f5f7fa',
    color: '#606266',
    fontWeight: '600',
    fontSize: '14px'
  }
}

// 单元格样式 - 居中显示
const cellStyle = () => {
  return {
    textAlign: 'center',
    verticalAlign: 'middle'
  }
}

// 处理行点击
const handleRowClick = (row: any) => {
  router.push(`/tasks/${row.id}`)
}

// 全选当前页
const selectAll = () => {
  if (selectedTasks.value.length === paginatedTasks.value.length) {
    selectedTasks.value = []
  } else {
    selectedTasks.value = paginatedTasks.value.map(t => t.id)
  }
}

// 清除选择
const clearSelection = () => {
  selectedTasks.value = []
}

// 批量删除
const batchDelete = async () => {
  try {
    await ElMessageBox.confirm(
      t('task.batchDeleteConfirm').replace('{count}', String(selectedTasks.value.length)), 
      t('task.batchDelete'), 
      { type: 'warning', confirmButtonText: t('common.confirm'), cancelButtonText: t('common.cancel') }
    )
    
    loading.value = true
    for (const taskId of selectedTasks.value) {
      await taskStore.deleteTask(taskId)
    }
    ElMessage.success(t('task.deleteSuccess'))
    selectedTasks.value = []
    await loadTasks()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error(t('task.deleteError') || t('common.error'))
    }
  } finally {
    loading.value = false
  }
}

const statusCounts = computed(() => ({
  pending: tasks.value.filter(t => t.status === 'pending').length,
  running: tasks.value.filter(t => t.status === 'running').length,
  paused: tasks.value.filter(t => t.status === 'paused').length,
  completed: tasks.value.filter(t => t.status === 'completed').length,
  failed: tasks.value.filter(t => t.status === 'failed').length
}))

const filteredTasks = computed(() => {
  let result = tasks.value
  
  if (searchQuery.value) {
    const query = searchQuery.value.toLowerCase()
    result = result.filter(task => 
      task.name.toLowerCase().includes(query) ||
      (task.description && task.description.toLowerCase().includes(query))
    )
  }
  
  if (filterStatus.value) {
    result = result.filter(task => task.status === filterStatus.value)
  }
  
  return result.sort((a, b) => 
    new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime()
  )
})

const paginatedTasks = computed(() => {
  const start = (currentPage.value - 1) * pageSize.value
  return filteredTasks.value.slice(start, start + pageSize.value)
})

const totalPages = computed(() => {
  return Math.ceil(filteredTasks.value.length / pageSize.value) || 1
})

const handleSearch = () => {
  currentPage.value = 1
}

const loadTasks = async () => {
  loading.value = true
  try {
    await taskStore.loadTasks(1, 100)
  } catch (error) {
    ElMessage.error(t('task.loadFailed'))
  } finally {
    loading.value = false
  }
}

const handleCommand = async (command: string, task: any) => {
  switch (command) {
    case 'start':
      try {
        await taskStore.startTask(task.id)
        ElMessage.success(t('task.taskStarted'))
        await loadTasks()
      } catch (error) {
        ElMessage.error(t('task.startFailed'))
      }
      break
    case 'stop':
      try {
        await ElMessageBox.confirm(t('task.stopConfirm'), t('common.info'))
        await taskStore.stopTask(task.id)
        ElMessage.success(t('task.taskStopped'))
        await loadTasks()
      } catch (error) {
        // cancel
      }
      break
    case 'export':
      try {
        const content = await taskStore.exportReport(task.id)
        if (content) {
          const blob = new Blob([content], { type: 'text/markdown' })
          const url = URL.createObjectURL(blob)
          const a = document.createElement('a')
          a.href = url
          a.download = `report_${task.id}.md`
          a.click()
          URL.revokeObjectURL(url)
          ElMessage.success(t('task.exportSuccess'))
        }
      } catch (error) {
        ElMessage.error(t('task.exportFailed'))
      }
      break
    case 'delete':
      try {
        await ElMessageBox.confirm(t('task.deleteConfirm'), t('common.warning'), { type: 'warning' })
        await taskStore.deleteTask(task.id)
        ElMessage.success(t('task.deleteSuccess'))
        await loadTasks()
      } catch (error) {
        // cancel
      }
      break
  }
}

onMounted(() => {
  loadTasks()
})
</script>

<style scoped>
.tasks-page {
  padding: 24px;
  max-width: 1400px;
  margin: 0 auto;
}

/* 页面头部 */
.page-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 24px;
}

.header-left h1 {
  margin: 0 0 8px 0;
  font-size: 24px;
  font-weight: 600;
  color: #111827;
}

.header-left p {
  margin: 0;
  font-size: 14px;
  color: #6b7280;
}

/* 统计卡片 */
.stats-row {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 16px;
  margin-bottom: 24px;
}

.stat-card {
  background: #ffffff;
  border-radius: 12px;
  padding: 20px;
  display: flex;
  align-items: center;
  gap: 16px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
}

.stat-icon {
  width: 48px;
  height: 48px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 20px;
}

.stat-icon.pending { background: #f3f4f6; color: #6b7280; }
.stat-icon.running { background: #eff6ff; color: #3b82f6; }
.stat-icon.completed { background: #ecfdf5; color: #10b981; }
.stat-icon.failed { background: #fef2f2; color: #ef4444; }

.stat-value {
  font-size: 28px;
  font-weight: 700;
  color: #111827;
}

.stat-label {
  font-size: 14px;
  color: #6b7280;
}

/* 筛选栏 */
.filter-card {
  margin-bottom: 24px;
  border-radius: 12px;
}

.filter-bar {
  display: flex;
  gap: 12px;
  align-items: center;
}

.search-input {
  width: 280px;
}

.status-select {
  width: 140px;
}

/* 批量操作 */
.batch-actions {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-left: auto;
  padding-left: 12px;
  border-left: 1px solid #e5e7eb;
}

.selected-count {
  font-size: 14px;
  color: #3b82f6;
  font-weight: 500;
}

/* 任务卡片 */
.tasks-card {
  border-radius: 12px;
}

.loading-state,
.empty-state {
  padding: 60px 0;
}

/* 横向任务列表样式 */
.tasks-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

/* 表头样式 */
.task-header-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 16px;
  background: #f9fafb;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  font-size: 13px;
  font-weight: 600;
  color: #6b7280;
}

.task-checkbox-header {
  width: 24px;
  flex-shrink: 0;
}

.task-name-header {
  flex: 1;
  min-width: 150px;
}

.task-cell-header {
  min-width: 100px;
  text-align: center;
}

.task-actions-header {
  width: 40px;
  flex-shrink: 0;
}

.task-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 16px;
  background: #ffffff;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s ease;
}

.task-row:hover {
  border-color: #3b82f6;
  background: #f8fafc;
}

.task-row.is-selected {
  border-color: #3b82f6;
  background: #f0f7ff;
}

.task-checkbox {
  flex-shrink: 0;
}

.task-name-cell {
  flex: 1;
  min-width: 150px;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.task-name-cell .task-name {
  font-size: 14px;
  font-weight: 600;
  color: #111827;
}

.task-name-cell .task-desc {
  font-size: 12px;
  color: #6b7280;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.task-cell {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: #374151;
  min-width: 100px;
}

.task-cell .el-icon {
  color: #9ca3af;
}

.task-progress-cell {
  min-width: 90px;
}

.vuln-cell {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-wrap: wrap;
  min-width: 150px;
}

.vuln-count {
  font-size: 12px;
  color: #6b7280;
}

.cell-with-icon {
  display: flex;
  align-items: center;
  gap: 6px;
  justify-content: center;
}

.cell-with-icon .el-icon {
  color: #9ca3af;
  font-size: 14px;
}

.task-actions {
  flex-shrink: 0;
}

/* 分页样式 */
.pagination-wrapper {
  margin-top: 24px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding-top: 16px;
  border-top: 1px solid #e5e7eb;
}

.pagination-info {
  display: flex;
  gap: 16px;
  font-size: 14px;
  color: #6b7280;
}

.pagination-info .total-count {
  color: #374151;
}

.task-card {
  background: #ffffff;
  border: 1px solid #e5e7eb;
  border-radius: 12px;
  padding: 16px;
  cursor: pointer;
  transition: all 0.3s ease;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.task-card:hover {
  border-color: #3b82f6;
  box-shadow: 0 4px 12px rgba(59, 130, 246, 0.15);
  transform: translateY(-2px);
}

.task-card.is-selected {
  border-color: #3b82f6;
  background: #f0f7ff;
}

/* 复选框 */
.task-checkbox {
  display: flex;
  align-items: center;
  justify-content: center;
}

/* 任务主要内容区域 */
.task-main {
  cursor: pointer;
}

.task-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}

.status-badge {
  padding: 4px 12px;
  border-radius: 20px;
  font-size: 12px;
  font-weight: 500;
}

.status-badge.pending { background: #f3f4f6; color: #6b7280; }
.status-badge.running { background: #eff6ff; color: #3b82f6; }
.status-badge.completed { background: #ecfdf5; color: #10b981; }
.status-badge.failed { background: #fef2f2; color: #ef4444; }

.task-body {
  margin-bottom: 16px;
}

.task-name {
  margin: 0 0 8px 0;
  font-size: 16px;
  font-weight: 600;
  color: #111827;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.task-desc {
  margin: 0 0 12px 0;
  font-size: 13px;
  color: #6b7280;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.task-progress {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 12px;
}

.task-progress :deep(.el-progress) {
  flex: 1;
}

.progress-text {
  font-size: 13px;
  font-weight: 500;
  color: #3b82f6;
  min-width: 40px;
}

.task-info {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.info-row {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  color: #6b7280;
}

.task-footer {
  display: flex;
  justify-content: space-between;
  padding-top: 12px;
  border-top: 1px solid #f3f4f6;
}

.task-footer .stat-item {
  text-align: center;
}

.task-footer .label {
  display: block;
  font-size: 11px;
  color: #9ca3af;
}

.task-footer .value {
  display: block;
  font-size: 14px;
  font-weight: 600;
  color: #374151;
}

.task-footer .value.score-high {
  color: #10b981;
}

.task-footer .value.score-medium {
  color: #f59e0b;
}

.task-footer .value.score-low {
  color: #f97316;
}

.task-footer .value.score-critical {
  color: #ef4444;
}

.task-footer .stat-item.vuln-critical .value {
  color: #ef4444;
}

.task-footer .stat-item.vuln-high .value {
  color: #f97316;
}

.pagination-wrapper {
  margin-top: 24px;
  display: flex;
  justify-content: center;
}

.animate-spin {
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

/* 响应式 */
@media (max-width: 1200px) {
  .tasks-grid {
    grid-template-columns: repeat(3, 1fr);
  }
}

@media (max-width: 992px) {
  .tasks-grid {
    grid-template-columns: repeat(2, 1fr);
  }
  
  .stats-row {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (max-width: 768px) {
  .page-header {
    flex-direction: column;
    gap: 16px;
  }
  
  .tasks-grid {
    grid-template-columns: 1fr;
  }
  
  .filter-bar {
    flex-wrap: wrap;
  }
  
  .search-input {
    width: 100%;
  }
  
  .stats-row {
    grid-template-columns: 1fr;
  }
}
</style>
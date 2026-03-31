<template>
  <div class="reports-page">
    <!-- 页面头部 -->
    <div class="page-header">
      <div class="header-left">
        <h1>{{ $t('report.title') }}</h1>
        <p>{{ $t('report.pageDesc') }}</p>
      </div>
      <div class="header-actions">
        <el-button type="primary" @click="refreshReports">
          <el-icon><Refresh /></el-icon>
          {{ $t('common.refresh') }}
        </el-button>
      </div>
    </div>

    <!-- 统计概览 -->
    <div class="stats-overview">
      <div class="stat-item">
        <div class="stat-icon blue">
          <el-icon><Document /></el-icon>
        </div>
        <div class="stat-info">
          <div class="stat-value">{{ reports.length }}</div>
          <div class="stat-label">{{ $t('report.totalReports') }}</div>
        </div>
      </div>
      
      <div class="stat-item">
        <div class="stat-icon red">
          <el-icon><Warning /></el-icon>
        </div>
        <div class="stat-info">
          <div class="stat-value">{{ totalVulnerabilities }}</div>
          <div class="stat-label">{{ $t('report.foundVulns') }}</div>
        </div>
      </div>
      
      <div class="stat-item">
        <div class="stat-icon green">
          <el-icon><CircleCheck /></el-icon>
        </div>
        <div class="stat-info">
          <div class="stat-value">{{ completedCount }}</div>
          <div class="stat-label">{{ $t('task.completed') }}</div>
        </div>
      </div>
      
      <div class="stat-item">
        <div class="stat-icon purple">
          <el-icon><Files /></el-icon>
        </div>
        <div class="stat-info">
          <div class="stat-value">{{ totalFiles }}</div>
          <div class="stat-label">{{ $t('report.scannedFiles') }}</div>
        </div>
      </div>
    </div>

    <!-- 筛选栏 -->
    <el-card class="filter-card">
      <div class="filter-bar">
        <el-input
          v-model="searchQuery"
          :placeholder="$t('report.searchPlaceholder')"
          :prefix-icon="Search"
          class="search-input"
          clearable
          @input="handleSearch"
        />
        
        <el-select v-model="filterStatus" :placeholder="$t('task.statusFilter')" clearable class="status-select">
          <el-option :label="$t('task.allStatus')" value="" />
          <el-option :label="$t('task.completed')" value="completed" />
          <el-option :label="$t('task.running')" value="running" />
          <el-option :label="$t('task.failed')" value="failed" />
        </el-select>
        
        <el-date-picker
          v-model="dateRange"
          type="daterange"
          :range-separator="$t('report.to')"
          :start-placeholder="$t('report.startDate')"
          :end-placeholder="$t('report.endDate')"
          format="YYYY-MM-DD"
          value-format="YYYY-MM-DD"
          class="date-picker"
        />
      </div>
    </el-card>

    <!-- 报告列表 -->
    <el-card class="reports-card">
      <div v-if="loading" class="loading-state">
        <el-skeleton :rows="6" animated />
      </div>
      
      <div v-else-if="filteredReports.length === 0" class="empty-state">
        <el-empty :description="$t('report.noReport')" :image-size="160">
          <el-button type="primary" @click="$router.push('/tasks')">
            {{ $t('report.createTask') }}
          </el-button>
        </el-empty>
      </div>
      
      <div v-else class="reports-table">
        <el-table :data="paginatedReports" style="width: 100%" row-class-name="table-row" :header-cell-style="headerCellStyle" :cell-style="cellStyle">
          <el-table-column prop="name" :label="$t('task.name')" min-width="180" show-overflow-tooltip />
          
          <el-table-column prop="description" :label="$t('task.description')" min-width="200" show-overflow-tooltip>
            <template #default="{ row }">
              <span>{{ row.description || '-' }}</span>
            </template>
          </el-table-column>
          
          <el-table-column prop="status" :label="$t('task.status')" width="100">
            <template #default="{ row }">
              <el-tag :type="getStatusType(row.status)" size="small" effect="dark" round>
                {{ getStatusText(row.status) }}
              </el-tag>
            </template>
          </el-table-column>
          
          <el-table-column :label="$t('report.vulnStats')" min-width="200">
            <template #default="{ row }">
              <div class="vuln-stats-cell">
                <span class="vuln-item critical" v-if="getVulnCount(row, 'critical') > 0">
                  {{ getVulnCount(row, 'critical') }} {{ $t('analysis.critical') }}
                </span>
                <span class="vuln-item high" v-if="getVulnCount(row, 'high') > 0">
                  {{ getVulnCount(row, 'high') }} {{ $t('analysis.high') }}
                </span>
                <span class="vuln-item medium" v-if="getVulnCount(row, 'medium') > 0">
                  {{ getVulnCount(row, 'medium') }} {{ $t('analysis.medium') }}
                </span>
                <span class="vuln-item low" v-if="getVulnCount(row, 'low') > 0">
                  {{ getVulnCount(row, 'low') }} {{ $t('analysis.low') }}
                </span>
                <span v-if="getVulnCount(row, 'critical') === 0 && getVulnCount(row, 'high') === 0 && getVulnCount(row, 'medium') === 0 && getVulnCount(row, 'low') === 0" class="no-vuln">
                  {{ $t('analysis.noVulnerabilities') }}
                </span>
              </div>
            </template>
          </el-table-column>
          
          <el-table-column prop="createdAt" :label="$t('task.createdAt')" width="170">
            <template #default="{ row }">
              <span>{{ formatDate(row.createdAt) }}</span>
            </template>
          </el-table-column>
          
          <el-table-column :label="$t('common.action')" width="180" fixed="right">
            <template #default="{ row }">
              <div class="action-buttons">
                <el-button 
                  type="primary" 
                  size="small" 
                  link
                  @click="viewReport(row)"
                >
                  <el-icon><View /></el-icon>
                  {{ $t('report.view') }}
                </el-button>
                <el-button 
                  type="success" 
                  size="small" 
                  link
                  @click="downloadReport(row)"
                  :disabled="row.status !== 'completed'"
                >
                  <el-icon><Download /></el-icon>
                  {{ $t('report.download') }}
                </el-button>
                <el-button 
                  type="danger" 
                  size="small" 
                  link
                  @click="confirmDelete(row)"
                >
                  <el-icon><Delete /></el-icon>
                </el-button>
              </div>
            </template>
          </el-table-column>
        </el-table>
        
        <!-- 分页 -->
        <div class="pagination-wrapper" v-if="filteredReports.length > pageSize">
          <el-pagination
            v-model:current-page="currentPage"
            v-model:page-size="pageSize"
            :page-sizes="[10, 20, 50]"
            :total="filteredReports.length"
            layout="total, sizes, prev, pager, next"
            background
          />
        </div>
      </div>
    </el-card>

    <!-- 报告详情对话框 -->
    <el-dialog
      v-model="showDialog"
      :title="currentReport?.name"
      width="85%"
      class="report-dialog"
      :z-index="3000"
      destroy-on-close
      modal-class="report-modal"
      append-to-body
    >
      <div class="dialog-content" v-if="currentReport">
        <!-- 报告头部信息 -->
        <div class="report-header-info">
          <div class="info-item">
            <span class="label">{{ $t('task.status') }}:</span>
            <el-tag :type="getStatusType(currentReport.status)" size="small">
              {{ getStatusText(currentReport.status) }}
            </el-tag>
          </div>
          <div class="info-item">
            <span class="label">{{ $t('task.createdAt') }}:</span>
            <span>{{ formatDate(currentReport.createdAt) }}</span>
          </div>
          <div class="info-item" v-if="currentReport.codeSource">
            <span class="label">{{ $t('codeSource.name') }}:</span>
            <span>{{ currentReport.codeSource.name }}</span>
          </div>
          <div class="info-item">
            <span class="label">{{ $t('report.preview') }}:</span>
            <el-radio-group v-model="previewMode" size="small">
              <el-radio-button value="markdown">Markdown</el-radio-button>
              <el-radio-button value="html">HTML</el-radio-button>
            </el-radio-group>
          </div>
        </div>
        
        <!-- 报告内容 -->
        <div class="report-body" v-if="currentReport.result">
          <!-- Markdown 原始内容 -->
          <pre v-if="previewMode === 'markdown'" class="report-content">{{ currentReport.result }}</pre>
          <!-- HTML 渲染内容 -->
          <div 
            v-else 
            class="markdown-body" 
            v-html="renderMarkdown(currentReport.result)"
          ></div>
        </div>
        <div class="empty-report" v-else>
          <el-empty :description="$t('report.noContent')" />
        </div>
      </div>
      
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="showDialog = false">{{ $t('common.cancel') }}</el-button>
          <el-button 
            type="primary" 
            @click="downloadReport(currentReport)"
            :disabled="currentReport?.status !== 'completed'"
          >
            <el-icon><Download /></el-icon>
            {{ $t('report.downloadReport') }}
          </el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useTaskStore } from '@/stores/task'
import { taskApi } from '@/api'
import { formatDate } from '@/utils/common'
import { marked } from 'marked'
import hljs from 'highlight.js'
import {
  Refresh,
  Search,
  Document,
  Warning,
  CircleCheck,
  Files,
  Clock,
  View,
  Download,
  Delete
} from '@element-plus/icons-vue'

const { t } = useI18n()

// 配置 marked 和 highlight.js
marked.setOptions({
  highlight: function(code: string, lang: string) {
    if (lang && hljs.getLanguage(lang)) {
      try {
        return hljs.highlight(code, { language: lang }).value
      } catch (e) {
        console.error(e)
      }
    }
    return hljs.highlightAuto(code).value
  },
  breaks: true,
  gfm: true
})

const taskStore = useTaskStore()

const loading = ref(false)
const searchQuery = ref('')
const filterStatus = ref('')
const dateRange = ref([])
const currentPage = ref(1)
const pageSize = ref(10)
const showDialog = ref(false)
const currentReport = ref<any>(null)
const previewMode = ref<'markdown' | 'html'>('markdown')

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

// 渲染 Markdown 为 HTML
const renderMarkdown = (content: string) => {
  if (!content) return ''
  try {
    return marked(content) as string
  } catch (e) {
    console.error('Markdown 解析失败:', e)
    return content
  }
}

const reports = computed(() =>
  taskStore.tasks.filter(task => task.status === 'completed')
)

const filteredReports = computed(() => {
  let result = reports.value
  
  if (searchQuery.value) {
    const query = searchQuery.value.toLowerCase()
    result = result.filter(report => 
      report.name.toLowerCase().includes(query)
    )
  }
  
  if (filterStatus.value) {
    result = result.filter(report => report.status === filterStatus.value)
  }
  
  if (dateRange.value && dateRange.value.length === 2) {
    const [start, end] = dateRange.value
    result = result.filter(report => {
      const date = new Date(report.createdAt)
      return date >= new Date(start) && date <= new Date(end)
    })
  }
  
  return result.sort((a, b) => 
    new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime()
  )
})

const paginatedReports = computed(() => {
  const start = (currentPage.value - 1) * pageSize.value
  return filteredReports.value.slice(start, start + pageSize.value)
})

const totalVulnerabilities = computed(() => {
  let total = 0
  reports.value.forEach(task => {
    // 使用 Task 模型中的精确漏洞统计字段（与 Dashboard 和 Tasks 保持一致）
    total += (task.criticalVulns || 0) + (task.highVulns || 0) + 
             (task.mediumVulns || 0) + (task.lowVulns || 0)
  })
  return total
})

const completedCount = computed(() => reports.value.length)

const totalFiles = computed(() => {
  let total = 0
  reports.value.forEach(task => {
    if (task.scannedFiles) {
      total += task.scannedFiles
    }
  })
  return total
})

const getVulnCount = (task: any, severity: string) => {
  // 使用 Task 模型中的精确漏洞统计字段（与 Dashboard 和 Tasks 保持一致）
  switch (severity) {
    case 'critical': return task.criticalVulns || 0
    case 'high': return task.highVulns || 0
    case 'medium': return task.mediumVulns || 0
    case 'low': return task.lowVulns || 0
    default: return 0
  }
}

const getStatusType = (status: string) => {
  switch (status) {
    case 'completed': return 'success'
    case 'running': return 'primary'
    case 'failed': return 'danger'
    default: return 'info'
  }
}

const getStatusText = (status: string) => {
  switch (status) {
    case 'pending': return t('task.pending')
    case 'running': return t('task.running')
    case 'completed': return t('task.completed')
    case 'failed': return t('task.failed')
    default: return t('task.unknownType')
  }
}

const handleSearch = () => {
  currentPage.value = 1
}

const loadReports = async () => {
  loading.value = true
  try {
    await taskStore.loadTasks(1, 100)
  } catch (error) {
    console.error('Failed to load reports:', error)
    ElMessage.error(t('report.loadFailed') || 'Failed to load reports')
  } finally {
    loading.value = false
  }
}

const refreshReports = async () => {
  await loadReports()
  ElMessage.success(t('common.success'))
}

const viewReport = (report: any) => {
  currentReport.value = report
  showDialog.value = true
}

const downloadReport = async (report: any) => {
  try {
    if (!report.result) {
      ElMessage.warning(t('report.noContent'))
      return
    }
    
    const blob = new Blob([report.result], { type: 'text/markdown;charset=utf-8' })
    const url = window.URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = `audit_report_${report.id}_${Date.now()}.md`
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    window.URL.revokeObjectURL(url)
    
    ElMessage.success(t('report.exportSuccess'))
  } catch (error) {
    console.error('Download failed:', error)
    ElMessage.error(t('report.exportFailed') || t('common.error'))
  }
}

const confirmDelete = async (report: any) => {
  try {
    await ElMessageBox.confirm(
      `${t('report.deleteConfirm') || t('common.confirm')}: "${report.name}"?`,
      t('report.delete') || t('common.delete'),
      {
        confirmButtonText: t('common.confirm'),
        cancelButtonText: t('common.cancel'),
        type: 'warning'
      }
    )
    
    // 删除对应的任务
    await taskApi.delete(report.id)
    ElMessage.success(t('report.deleteSuccess'))
    await loadReports()
  } catch (error) {
    // User cancelled or API error
  }
}

onMounted(() => {
  loadReports()
})
</script>

<style scoped>
.reports-page {
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

/* 统计概览 */
.stats-overview {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 20px;
  margin-bottom: 24px;
}

.stat-item {
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

.stat-icon.blue { background: #eff6ff; color: #3b82f6; }
.stat-icon.red { background: #fef2f2; color: #ef4444; }
.stat-icon.green { background: #ecfdf5; color: #10b981; }
.stat-icon.purple { background: #f5f3ff; color: #8b5cf6; }

.stat-value {
  font-size: 24px;
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
  gap: 16px;
  align-items: center;
}

.search-input {
  width: 300px;
}

.status-select {
  width: 140px;
}

.date-picker {
  width: 280px;
}

/* 报告列表 */
.reports-card {
  border-radius: 12px;
}

.loading-state,
.empty-state {
  padding: 60px 0;
}

.reports-table {
  min-height: 400px;
}

.table-row {
  cursor: pointer;
}

.task-info-cell {
  padding: 8px 0;
}

.task-name {
  font-size: 14px;
  font-weight: 500;
  color: #111827;
}

.task-desc {
  font-size: 12px;
  color: #6b7280;
  margin-top: 4px;
}

.vuln-stats-cell {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.vuln-item {
  font-size: 12px;
  padding: 2px 8px;
  border-radius: 4px;
}

.vuln-item.critical { background: #fef2f2; color: #dc2626; }
.vuln-item.high { background: #fff7ed; color: #ea580c; }
.vuln-item.medium { background: #fefce8; color: #ca8a04; }
.vuln-item.low { background: #f0fdf4; color: #16a34a; }

.time-cell {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: #6b7280;
}

.action-buttons {
  display: flex;
  gap: 8px;
}

.pagination-wrapper {
  margin-top: 20px;
  display: flex;
  justify-content: flex-end;
}

/* 报告对话框 */
.report-dialog {
  margin-top: 5vh !important;
}

.report-dialog :deep(.el-dialog) {
  z-index: 3000 !important;
  margin-left: 280px !important;
}

.report-dialog :deep(.el-overlay) {
  z-index: 2999 !important;
}

.report-header-info {
  display: flex;
  gap: 24px;
  padding: 16px;
  background: #f9fafb;
  border-radius: 8px;
  margin-bottom: 20px;
}

.report-header-info .info-item {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
}

.report-header-info .label {
  color: #6b7280;
}

.report-body {
  max-height: 60vh;
  overflow-y: auto;
}

.report-content {
  background: #f8fafc;
  padding: 20px;
  border-radius: 8px;
  font-size: 13px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-wrap: break-word;
  margin: 0;
}

.empty-report {
  padding: 40px 0;
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}

/* Markdown 渲染样式 */
.markdown-body {
  padding: 20px;
  background: #ffffff;
  border-radius: 8px;
  line-height: 1.7;
  color: #24292e;
}

.markdown-body :deep(h1) {
  font-size: 24px;
  font-weight: 600;
  margin: 0 0 16px 0;
  padding-bottom: 8px;
  border-bottom: 1px solid #eaecef;
}

.markdown-body :deep(h2) {
  font-size: 20px;
  font-weight: 600;
  margin: 24px 0 12px 0;
  padding-bottom: 6px;
  border-bottom: 1px solid #eaecef;
}

.markdown-body :deep(h3) {
  font-size: 16px;
  font-weight: 600;
  margin: 20px 0 10px 0;
}

.markdown-body :deep(h4),
.markdown-body :deep(h5),
.markdown-body :deep(h6) {
  font-size: 14px;
  font-weight: 600;
  margin: 16px 0 8px 0;
}

.markdown-body :deep(p) {
  margin: 0 0 12px 0;
}

.markdown-body :deep(ul),
.markdown-body :deep(ol) {
  margin: 0 0 12px 0;
  padding-left: 24px;
}

.markdown-body :deep(li) {
  margin: 4px 0;
}

.markdown-body :deep(code) {
  padding: 2px 6px;
  background: #f6f8fa;
  border-radius: 4px;
  font-family: 'SFMono-Regular', Consolas, 'Liberation Mono', Menlo, monospace;
  font-size: 13px;
}

.markdown-body :deep(pre) {
  padding: 16px;
  background: #f6f8fa;
  border-radius: 6px;
  overflow-x: auto;
  margin: 0 0 12px 0;
}

.markdown-body :deep(pre code) {
  padding: 0;
  background: transparent;
}

.markdown-body :deep(blockquote) {
  margin: 0 0 12px 0;
  padding: 0 16px;
  border-left: 4px solid #dfe2e5;
  color: #6a737d;
}

.markdown-body :deep(table) {
  width: 100%;
  border-collapse: collapse;
  margin: 0 0 12px 0;
}

.markdown-body :deep(th),
.markdown-body :deep(td) {
  padding: 8px 12px;
  border: 1px solid #dfe2e5;
}

.markdown-body :deep(th) {
  background: #f6f8fa;
  font-weight: 600;
}

.markdown-body :deep(a) {
  color: #0366d6;
  text-decoration: none;
}

.markdown-body :deep(a:hover) {
  text-decoration: underline;
}

.markdown-body :deep(hr) {
  height: 1px;
  background: #eaecef;
  border: none;
  margin: 24px 0;
}

/* 漏洞严重程度高亮样式 */
.markdown-body :deep(.critical),
.markdown-body :deep(.severity-critical) {
  color: #d73a49;
  font-weight: 600;
}

.markdown-body :deep(.high),
.markdown-body :deep(.severity-high) {
  color: #e36209;
  font-weight: 600;
}

.markdown-body :deep(.medium),
.markdown-body :deep(.severity-medium) {
  color: #b08800;
}

.markdown-body :deep(.low),
.markdown-body :deep(.severity-low) {
  color: #22863a;
}

/* 响应式 */
@media (max-width: 1200px) {
  .stats-overview {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (max-width: 768px) {
  .page-header {
    flex-direction: column;
    gap: 16px;
  }
  
  .stats-overview {
    grid-template-columns: 1fr;
  }
  
  .filter-bar {
    flex-direction: column;
    align-items: stretch;
  }
  
  .search-input,
  .status-select,
  .date-picker {
    width: 100%;
  }
}
</style>
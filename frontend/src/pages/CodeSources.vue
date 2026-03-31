<template>
  <div class="code-sources-page">
    <!-- 页面头部 -->
    <div class="page-header">
      <div class="header-left">
        <h1>{{ $t('codeSource.title') }}</h1>
        <p>{{ $t('codeSource.pageDesc') || $t('codeSource.title') }}</p>
      </div>
      <div class="header-actions">
        <el-button type="primary" :icon="Plus" @click="showUploadDialog = true">
          {{ $t('codeSource.upload') }}
        </el-button>
      </div>
    </div>

    <!-- 统计概览 -->
    <div class="stats-overview">
      <div class="stat-item">
        <div class="stat-icon blue">
          <el-icon><Folder /></el-icon>
        </div>
        <div class="stat-info">
          <div class="stat-value">{{ sources.length }}</div>
          <div class="stat-label">{{ $t('codeSource.title') }}</div>
        </div>
      </div>
      
      <div class="stat-item">
        <div class="stat-icon green">
          <el-icon><CircleCheck /></el-icon>
        </div>
        <div class="stat-info">
          <div class="stat-value">{{ activeCount }}</div>
          <div class="stat-label">{{ $t('codeSource.active') }}</div>
        </div>
      </div>
      
      <div class="stat-item">
        <div class="stat-icon yellow">
          <el-icon><Loading /></el-icon>
        </div>
        <div class="stat-info">
          <div class="stat-value">{{ processingCount }}</div>
          <div class="stat-label">{{ $t('codeSource.processing') }}</div>
        </div>
      </div>
      
      <div class="stat-item">
        <div class="stat-icon red">
          <el-icon><CircleClose /></el-icon>
        </div>
        <div class="stat-info">
          <div class="stat-value">{{ errorCount }}</div>
          <div class="stat-label">{{ $t('codeSource.error') }}</div>
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
        />
        
        <el-select v-model="filterType" :placeholder="$t('codeSource.type')" clearable class="status-select">
          <el-option :label="$t('codeSource.zip')" value="zip" />
          <el-option :label="$t('codeSource.jar')" value="jar" />
          <el-option :label="$t('codeSource.git')" value="git" />
        </el-select>
        
        <el-button type="primary" :icon="Refresh" @click="loadCodeSources" :loading="loading">
          {{ $t('common.refresh') }}
        </el-button>
      </div>
    </el-card>

    <!-- 代码源列表 -->
    <el-card class="sources-card">
      
      <div v-if="loading" class="loading-container">
        <el-skeleton :rows="6" animated />
      </div>
      
      <div v-else-if="filteredSources.length === 0" class="empty-container">
        <el-empty :description="$t('common.noData')">
          <el-button type="primary" @click="showUploadDialog = true">{{ $t('codeSource.upload') }}</el-button>
        </el-empty>
      </div>
      
      <div v-else class="sources-grid">
        <div 
          v-for="source in filteredSources" 
          :key="source.id || source.ID" 
          class="source-card"
          @click="viewSource(source.id || source.ID)"
        >
          <div class="source-header">
            <div class="source-icon">
              <el-icon v-if="source.type === 'zip'"><Folder /></el-icon>
              <el-icon v-else-if="source.type === 'jar'"><Document /></el-icon>
              <el-icon v-else-if="source.type === 'git'"><Link /></el-icon>
              <el-icon v-else><FolderOpened /></el-icon>
            </div>
            <div class="source-title">
              <h3>{{ source.name }}</h3>
              <span class="source-type">{{ getSourceTypeText(source.type) }}</span>
            </div>
            <el-tag :type="getSourceStatusType(source.status)" size="small">
              {{ getSourceStatusText(source.status) }}
            </el-tag>
          </div>
          
          <div class="source-meta">
            <div class="meta-item">
              <el-icon><Clock /></el-icon>
              <span>{{ formatDateUtil(source.createdAt) }}</span>
            </div>
            <div class="meta-item">
              <el-icon><InfoFilled /></el-icon>
              <span>{{ formatSizeUtil(source.size) }}</span>
            </div>
            <div v-if="source.language" class="meta-item language-tag">
              <el-tag size="small" type="info">{{ source.language }}</el-tag>
            </div>
          </div>
          
          <div class="source-path">
            <el-icon><Location /></el-icon>
            <span>{{ source.path }}</span>
          </div>
          
          <div class="source-actions">
            <el-button size="small" @click.stop="$router.push(`/tasks/create?source=${source.id || source.ID}`)">
              {{ $t('codeSource.createTask') }}
            </el-button>
            <el-button size="small" type="danger" @click.stop="deleteSource(source.id || source.ID)">
              {{ $t('common.delete') }}
            </el-button>
          </div>
        </div>
      </div>
    </el-card>

    <!-- 上传对话框 -->
    <el-dialog 
      v-model="showUploadDialog" 
      :title="$t('codeSource.upload')" 
      width="600px"
      :before-close="handleUploadClose"
    >
      <el-form :model="uploadForm" :rules="uploadRules" ref="uploadFormRef" label-width="100px">
        <el-form-item :label="$t('codeSource.name')" prop="name">
          <el-input v-model="uploadForm.name" :placeholder="$t('codeSource.name')" />
        </el-form-item>
        
        <el-form-item :label="$t('codeSource.type')" prop="type">
          <el-radio-group v-model="uploadForm.type" @change="onUploadTypeChange">
            <el-radio label="zip">{{ $t('codeSource.zip') }}</el-radio>
            <el-radio label="jar">{{ $t('codeSource.jar') }}</el-radio>
            <el-radio label="git">{{ $t('codeSource.git') }}</el-radio>
          </el-radio-group>
        </el-form-item>
        
        <!-- ZIP/JAR 文件上传 -->
        <el-form-item v-if="uploadForm.type === 'zip' || uploadForm.type === 'jar'" :label="$t('codeSource.file')" prop="file">
          <el-upload
            ref="uploadRef"
            :auto-upload="false"
            :on-change="handleFileChange"
            :on-remove="handleFileRemove"
            :limit="1"
            accept=".zip,.jar"
            drag
          >
            <el-icon class="el-icon--upload"><upload-filled /></el-icon>
            <div class="el-upload__text">
              {{ $t('codeSource.dragUpload') }}
            </div>
            <template #tip>
              <div class="el-upload__tip">
                {{ $t('codeSource.uploadTip') }}
              </div>
            </template>
          </el-upload>
        </el-form-item>
        
        <!-- Git 仓库 -->
        <el-form-item v-if="uploadForm.type === 'git'" :label="$t('codeSource.url')" prop="gitUrl">
          <el-input v-model="uploadForm.gitUrl" :placeholder="$t('codeSource.gitUrlPlaceholder')" />
        </el-form-item>
      </el-form>
      
      <template #footer>
        <span class="dialog-footer">
          <el-button @click="showUploadDialog = false">{{ $t('common.cancel') }}</el-button>
          <el-button type="primary" @click="submitUpload" :loading="uploading">
            {{ $t('common.submit') }}
          </el-button>
        </span>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { useTaskStore } from '@/stores/task'
import { formatDate as formatDateUtil, formatSize as formatSizeUtil } from '@/utils/common'
import {
  Plus,
  Search,
  Refresh,
  Folder,
  Document,
  Link,
  FolderOpened,
  Clock,
  InfoFilled,
  Location,
  UploadFilled,
  CircleCheck,
  CircleClose,
  Loading
} from '@element-plus/icons-vue'

const { t, locale } = useI18n()
const router = useRouter()
const taskStore = useTaskStore()

const loading = ref(false)
const uploading = ref(false)
const showUploadDialog = ref(false)
const searchQuery = ref('')
const filterType = ref('')

const uploadForm = ref({
  name: '',
  type: 'zip',
  file: null as File | null,
  gitUrl: ''
})

const uploadFormRef = ref<FormInstance>()
const uploadRef = ref()

const uploadRules: FormRules = {
  name: [
    { required: true, message: t('codeSource.name') + t('auth.usernameRequired'), trigger: 'blur' }
  ],
  type: [
    { required: true, message: t('codeSource.type') + t('auth.usernameRequired'), trigger: 'change' }
  ],
  gitUrl: [
    { required: true, message: t('codeSource.gitUrlPlaceholder'), trigger: 'blur' },
    { type: 'url', message: 'Please enter a valid URL', trigger: 'blur' }
  ]
}

const sources = computed(() => taskStore.codeSources)

// 统计计算属性
const activeCount = computed(() => sources.value.filter(s => s.status === 'active').length)
const processingCount = computed(() => sources.value.filter(s => s.status === 'processing').length)
const errorCount = computed(() => sources.value.filter(s => s.status === 'error').length)

const filteredSources = computed(() => {
  let result = sources.value
  
  if (searchQuery.value) {
    const query = searchQuery.value.toLowerCase()
    result = result.filter(source => 
      source.name.toLowerCase().includes(query)
    )
  }
  
  if (filterType.value) {
    result = result.filter(source => source.type === filterType.value)
  }
  
  return result.sort((a, b) => new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime())
})

const getSourceTypeText = (type: string) => {
  switch (type) {
    case 'zip': return t('codeSource.zip')
    case 'jar': return t('codeSource.jar')
    case 'git': return t('codeSource.git')
    default: return t('codeSource.unknownType')
  }
}

const getSourceStatusType = (status: string) => {
  switch (status) {
    case 'active': return 'success'
    case 'processing': return 'warning'
    case 'error': return 'danger'
    default: return 'info'
  }
}

const getSourceStatusText = (status: string) => {
  switch (status) {
    case 'active': return t('codeSource.active')
    case 'processing': return t('codeSource.processing')
    case 'error': return t('codeSource.error')
    default: return t('codeSource.unknown')
  }
}

const loadCodeSources = async () => {
  loading.value = true
  try {
    await taskStore.loadCodeSources()
  } catch (error) {
    console.error('Load code source failed:', error)
    ElMessage.error(t('codeSource.loadFailed') || t('common.error'))
  } finally {
    loading.value = false
  }
}

const onUploadTypeChange = (type: string) => {
  uploadForm.value.file = null
  uploadForm.value.gitUrl = ''
  if (uploadRef.value) {
    uploadRef.value.clearFiles()
  }
}

const handleFileChange = (file: any) => {
  uploadForm.value.file = file.raw
}

const handleFileRemove = () => {
  uploadForm.value.file = null
}

const handleUploadClose = (done: () => void) => {
  if (uploading.value) {
    ElMessage.warning(t('codeSource.uploadInProgress'))
    return
  }
  done()
}

const submitUpload = async () => {
  if (!uploadFormRef.value) return
  
  await uploadFormRef.value.validate(async (valid) => {
    if (valid) {
      uploading.value = true
      try {
        if (uploadForm.value.type === 'zip' || uploadForm.value.type === 'jar') {
          if (!uploadForm.value.file) {
            ElMessage.error(t('codeSource.selectFile'))
            uploading.value = false
            return
          }
          // Upload file
          if (uploadForm.value.type === 'zip') {
            await taskStore.uploadZip(uploadForm.value.file)
          } else {
            await taskStore.uploadJar(uploadForm.value.file)
          }
        } else if (uploadForm.value.type === 'git') {
          // Git repository
          await taskStore.addGitRepo(uploadForm.value.gitUrl)
        }
        
        ElMessage.success(t('codeSource.uploadSuccess'))
        showUploadDialog.value = false
        await loadCodeSources()
        resetUploadForm()
      } catch (error) {
        console.error('Upload failed:', error)
        ElMessage.error(t('codeSource.uploadFailed') || t('common.error'))
      } finally {
        uploading.value = false
      }
    }
  })
}

const viewSource = (sourceId: string | number) => {
  router.push(`/code-sources/${sourceId}`)
}

const deleteSource = async (sourceId: string) => {
  try {
    await ElMessageBox.confirm(t('codeSource.deleteConfirm'), t('common.warning'), {
      confirmButtonText: t('common.confirm'),
      cancelButtonText: t('common.cancel'),
      type: 'warning'
    })
    
    await taskStore.deleteCodeSource(sourceId)
    ElMessage.success(t('codeSource.deleteSuccess'))
    await loadCodeSources()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error(t('codeSource.deleteFailed') || t('common.error'))
    }
  }
}

const resetUploadForm = () => {
  uploadForm.value = {
    name: '',
    type: 'zip',
    file: null,
    gitUrl: ''
  }
  if (uploadFormRef.value) {
    uploadFormRef.value.resetFields()
  }
  if (uploadRef.value) {
    uploadRef.value.clearFiles()
  }
}

onMounted(() => {
  loadCodeSources()
})
</script>

<style scoped>
.code-sources-page {
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
.stat-icon.green { background: #ecfdf5; color: #10b981; }
.stat-icon.yellow { background: #fefce8; color: #eab308; }
.stat-icon.red { background: #fef2f2; color: #ef4444; }

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
  gap: 12px;
  align-items: center;
}

.search-input {
  width: 280px;
}

.status-select {
  width: 140px;
}

/* 代码源卡片 */
.sources-card {
  border-radius: 12px;
  margin-bottom: 24px;
}

.loading-container,
.empty-container {
  padding: 60px 0;
}

.sources-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: 16px;
}

.source-card {
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  padding: 16px;
  cursor: pointer;
  transition: all 0.2s;
  background: #fff;
}

.source-card:hover {
  border-color: #3b82f6;
  box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.1);
}

.source-header {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 12px;
}

.source-icon {
  width: 40px;
  height: 40px;
  border-radius: 8px;
  background-color: #eff6ff;
  color: #3b82f6;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 20px;
}

.source-title {
  flex: 1;
}

.source-title h3 {
  margin: 0 0 4px 0;
  font-size: 16px;
  color: #1f2937;
}

.source-type {
  font-size: 12px;
  color: #6b7280;
}

.source-meta {
  display: flex;
  gap: 16px;
  margin-bottom: 8px;
}

.meta-item {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: #6b7280;
}

.source-path {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 16px;
  font-size: 12px;
  color: #6b7280;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.source-actions {
  display: flex;
  gap: 8px;
  justify-content: flex-end;
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}

@media (max-width: 768px) {
  .page-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 16px;
  }
  
  .filter-container {
    flex-direction: column;
    align-items: stretch;
  }
  
  .search-input {
    min-width: auto;
  }
  
  .sources-grid {
    grid-template-columns: 1fr;
  }
}
</style>
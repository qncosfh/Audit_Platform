<template>
  <div class="models-page">
    <!-- 页面头部 -->
    <div class="page-header">
      <div class="header-left">
        <h1>{{ $t('model.title') }}</h1>
        <p>{{ $t('model.pageDesc') || $t('model.title') }}</p>
      </div>
      <div class="header-actions">
        <el-button type="primary" :icon="Plus" @click="showCreateDialog = true">
          {{ $t('model.create') }}
        </el-button>
      </div>
    </div>

    <!-- 统计概览 -->
    <div class="stats-overview">
      <div class="stat-item">
        <div class="stat-icon blue">
          <el-icon><Cpu /></el-icon>
        </div>
        <div class="stat-info">
          <div class="stat-value">{{ models.length }}</div>
          <div class="stat-label">{{ $t('model.title') }}</div>
        </div>
      </div>
      
      <div class="stat-item">
        <div class="stat-icon green">
          <el-icon><CircleCheck /></el-icon>
        </div>
        <div class="stat-info">
          <div class="stat-value">{{ activeCount }}</div>
          <div class="stat-label">{{ $t('model.active') }}</div>
        </div>
      </div>
      
      <div class="stat-item">
        <div class="stat-icon yellow">
          <el-icon><Loading /></el-icon>
        </div>
        <div class="stat-info">
          <div class="stat-value">{{ testingCount }}</div>
          <div class="stat-label">{{ $t('model.testing') }}</div>
        </div>
      </div>
      
      <div class="stat-item">
        <div class="stat-icon red">
          <el-icon><CircleClose /></el-icon>
        </div>
        <div class="stat-info">
          <div class="stat-value">{{ errorCount }}</div>
          <div class="stat-label">{{ $t('model.error') }}</div>
        </div>
      </div>
    </div>

    <!-- 筛选栏 -->
    <el-card class="filter-card">
      <div class="filter-bar">
        <el-input
          v-model="searchQuery"
          :placeholder="$t('model.searchPlaceholder')"
          :prefix-icon="Search"
          class="search-input"
          clearable
        />
        
        <el-select v-model="filterProvider" :placeholder="$t('model.providerFilter')" clearable class="status-select">
          <el-option :label="$t('model.openai')" value="openai" />
          <el-option :label="$t('model.anthropic')" value="anthropic" />
          <el-option :label="$t('model.google')" value="google" />
          <el-option :label="$t('model.local')" value="local" />
        </el-select>
        
        <el-button type="primary" :icon="Refresh" @click="loadModels" :loading="loading">
          {{ $t('common.refresh') }}
        </el-button>
      </div>
    </el-card>

    <!-- 模型列表 -->
    <el-card class="models-card">
      
      <div v-if="loading" class="loading-container">
        <el-skeleton :rows="6" animated />
      </div>
      
      <div v-else-if="filteredModels.length === 0" class="empty-container">
        <el-empty :description="$t('model.noModels')">
          <el-button type="primary" @click="showCreateDialog = true">{{ $t('model.create') }}</el-button>
        </el-empty>
      </div>
      
      <div v-else class="models-grid">
        <div 
          v-for="model in filteredModels" 
          :key="model.id" 
          class="model-card"
          :class="{ 'model-active': model.isActive }"
        >
          <div class="model-header">
            <div class="model-icon">
              <el-icon v-if="model.provider === 'openai'"><ChatDotRound /></el-icon>
              <el-icon v-else-if="model.provider === 'anthropic'"><Document /></el-icon>
              <el-icon v-else-if="model.provider === 'google'"><ChromeFilled /></el-icon>
              <el-icon v-else><Cpu /></el-icon>
            </div>
            <div class="model-title">
              <h3>{{ model.name }}</h3>
              <span class="model-provider">{{ getProviderText(model.provider) }}</span>
            </div>
            <el-tag :type="getModelStatusType(model.status)" size="small">
              {{ getModelStatusText(model.status) }}
            </el-tag>
          </div>
          
          <div class="model-meta">
            <div class="meta-item">
              <el-icon><Document /></el-icon>
              <span>{{ model.model || $t('model.notSet') }}</span>
            </div>
            <div class="meta-item">
              <el-icon><Clock /></el-icon>
              <span>{{ formatDate(model.createdAt) }}</span>
            </div>
          </div>
          
          <div class="model-config">
            <div class="config-item">
              <span class="config-label">{{ $t('model.baseUrl') }}:</span>
              <span class="config-value">{{ model.baseUrl || $t('model.notSet') }}</span>
            </div>
            <div class="config-item">
              <span class="config-label">{{ $t('model.apiKey') }}:</span>
              <span class="config-value">{{ model.apiKey ? '******' + model.apiKey.slice(-4) : $t('model.notSet') }}</span>
            </div>
            <div class="config-item">
              <span class="config-label">{{ $t('model.maxTokens') }}:</span>
              <span class="config-value">{{ model.maxTokens === 0 ? $t('model.unlimited') : model.maxTokens }}</span>
            </div>
          </div>
          
          <div class="model-actions">
            <el-button size="small" :loading="testingModelId === model.id" :disabled="testingModelId !== null" @click="testModel(model.id)">
              {{ testingModelId === model.id ? $t('model.testing') : $t('model.test') }}
            </el-button>
            <el-button size="small" @click="editModel(model)">
              {{ $t('common.edit') }}
            </el-button>
            <el-button 
              size="small" 
              type="danger" 
              @click="deleteModel(model.id)"
            >
              {{ $t('common.delete') }}
            </el-button>
          </div>
          
          <div class="model-footer">
            <el-switch
              v-model="model.isActive"
              :active-text="$t('model.enabled')"
              :inactive-text="$t('model.disabled')"
              @change="(value) => toggleModelStatus(model.id, value)"
            />
          </div>
        </div>
      </div>
    </el-card>

    <!-- 创建/编辑模型对话框 -->
    <el-dialog 
      v-model="showCreateDialog" 
      :title="editingModel ? $t('model.edit') : $t('model.create')" 
      width="600px"
    >
      <el-form :model="modelForm" :rules="modelRules" ref="modelFormRef" label-width="100px">
        <el-form-item :label="$t('model.name')" prop="name">
          <el-input v-model="modelForm.name" :placeholder="$t('model.namePlaceholder')" />
        </el-form-item>
        
        <el-form-item :label="$t('model.provider')" prop="provider">
          <el-select v-model="modelForm.provider" :placeholder="$t('model.selectProvider')" class="full-width" @change="handleProviderChange">
            <el-option :label="$t('model.openai')" value="openai" />
            <el-option :label="$t('model.anthropic')" value="anthropic" />
            <el-option :label="$t('model.google')" value="google" />
            <el-option :label="$t('model.local')" value="local" />
          </el-select>
        </el-form-item>
        
        <el-form-item :label="$t('model.model')" prop="model">
          <el-input v-model="modelForm.model" :placeholder="$t('model.modelPlaceholder')" />
        </el-form-item>
        
        <el-form-item :label="$t('model.baseUrl')" prop="base_url">
          <el-input v-model="modelForm.base_url" :placeholder="apiEndpointPlaceholder" />
        </el-form-item>
        
        <el-form-item :label="$t('model.apiKey')" prop="api_key">
          <el-input 
            v-model="modelForm.api_key" 
            type="password"
            :placeholder="$t('model.apiKeyPlaceholder')"
            show-password
          />
        </el-form-item>
        
        <el-form-item :label="$t('model.maxTokens')" prop="max_tokens">
          <div class="token-input-container">
            <el-input-number 
              v-model="modelForm.max_tokens" 
              :min="100" 
              :max="8000"
              controls-position="right"
              :disabled="modelForm.unlimited_tokens"
              class="full-width"
            />
            <el-checkbox 
              v-model="modelForm.unlimited_tokens" 
              class="unlimited-checkbox"
            >
              {{ $t('model.unlimited') }}
            </el-checkbox>
          </div>
        </el-form-item>
        
        <el-form-item :label="$t('model.status')">
          <el-switch
            v-model="modelForm.is_active"
            :active-text="$t('model.enabled')"
            :inactive-text="$t('model.disabled')"
          />
        </el-form-item>
      </el-form>
      
      <template #footer>
        <span class="dialog-footer">
          <el-button @click="showCreateDialog = false">{{ $t('common.cancel') }}</el-button>
          <el-button type="primary" @click="submitModel" :loading="submitting">
            {{ editingModel ? $t('common.update') : $t('common.create') }}
          </el-button>
        </span>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useTaskStore } from '@/stores/task'
import { formatDate } from '@/utils/common'
import {
  Plus,
  Search,
  Refresh,
  ChatDotRound,
  Document,
  ChromeFilled,
  Cpu,
  CircleCheck,
  CircleClose,
  Loading
} from '@element-plus/icons-vue'

const { t } = useI18n()
const taskStore = useTaskStore()

const loading = ref(false)
const submitting = ref(false)
const testingModelId = ref<string | null>(null)
const showCreateDialog = ref(false)
const searchQuery = ref('')
const filterProvider = ref('')
const editingModel = ref<any>(null)

const modelForm = ref({
  name: '',
  provider: 'openai',
  model: '',
  base_url: '',
  api_key: '',
  max_tokens: 4000,
  unlimited_tokens: false,
  is_active: true
})

const modelFormRef = ref()

// API端点placeholder
const apiEndpointPlaceholder = computed(() => {
  switch (modelForm.value.provider) {
    case 'openai': return t('model.openaiPlaceholder')
    case 'anthropic': return t('model.anthropicPlaceholder')
    case 'google': return t('model.googlePlaceholder')
    case 'local': return t('model.localPlaceholder')
    default: return t('model.baseUrlPlaceholder')
  }
})

// 处理提供者变更
const handleProviderChange = () => {
  // 根据提供者自动填充API端点
  switch (modelForm.value.provider) {
    case 'openai':
      modelForm.value.base_url = 'https://api.openai.com/v1'
      break
    case 'anthropic':
      modelForm.value.base_url = 'https://api.anthropic.com'
      break
    case 'google':
      modelForm.value.base_url = 'https://generativelanguage.googleapis.com/v1'
      break
    case 'local':
      modelForm.value.base_url = 'http://localhost:3000/v1'
      break
  }
}

const modelRules = computed(() => ({
  name: [
    { required: true, message: t('model.nameRequired'), trigger: 'blur' }
  ],
  provider: [
    { required: true, message: t('model.providerRequired'), trigger: 'change' }
  ],
  model: [
    { required: true, message: t('model.modelRequired'), trigger: 'blur' }
  ]
}))

const models = computed(() => taskStore.models)

// 统计计算属性
const activeCount = computed(() => models.value.filter(m => m.status === 'active' || m.isActive).length)
const testingCount = computed(() => models.value.filter(m => m.status === 'testing').length)
const errorCount = computed(() => models.value.filter(m => m.status === 'error').length)

const filteredModels = computed(() => {
  let result = models.value
  
  if (searchQuery.value) {
    const query = searchQuery.value.toLowerCase()
    result = result.filter(model => 
      model.name.toLowerCase().includes(query) ||
      model.provider.toLowerCase().includes(query) ||
      model.model.toLowerCase().includes(query)
    )
  }
  
  if (filterProvider.value) {
    result = result.filter(model => model.provider === filterProvider.value)
  }
  
  return result.sort((a, b) => new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime())
})

const getProviderText = (provider: string) => {
  switch (provider) {
    case 'openai': return t('model.openai')
    case 'anthropic': return t('model.anthropic')
    case 'google': return t('model.google')
    case 'local': return t('model.local')
    default: return provider
  }
}

const getModelStatusType = (status: string) => {
  switch (status) {
    case 'active': return 'success'
    case 'inactive': return 'info'
    case 'error': return 'danger'
    default: return 'warning'
  }
}

const getModelStatusText = (status: string) => {
  switch (status) {
    case 'active': return t('model.active')
    case 'inactive': return t('model.disabled')
    case 'error': return t('model.error')
    default: return t('model.unknown')
  }
}

const loadModels = async () => {
  loading.value = true
  try {
    await taskStore.loadModels()
  } catch (error) {
    console.error('Load models failed:', error)
    ElMessage.error(t('model.loadFailed'))
  } finally {
    loading.value = false
  }
}

const testModel = async (modelId: string) => {
  testingModelId.value = modelId
  try {
    await taskStore.testModel(modelId)
    ElMessage.success(t('model.testSuccess'))
  } catch (error) {
    console.error('Model test failed:', error)
    ElMessage.error(t('model.testError'))
  } finally {
    testingModelId.value = null
  }
}

const editModel = (model: any) => {
  editingModel.value = model
  modelForm.value = {
    name: model.name,
    provider: model.provider,
    model: model.model,
    base_url: model.baseUrl || model.base_url || '',
    api_key: model.apiKey || model.api_key || '',
    max_tokens: model.maxTokens || model.max_tokens || 4000,
    unlimited_tokens: model.maxTokens === 0,
    is_active: model.isActive !== false
  }
  showCreateDialog.value = true
}

const toggleModelStatus = async (modelId: string, isActive: boolean) => {
  try {
    await taskStore.updateModel(modelId, { isActive: isActive })
    ElMessage.success(t('model.statusUpdateSuccess'))
  } catch (error) {
    console.error('Update model status failed:', error)
    ElMessage.error(t('model.statusUpdateFailed'))
  }
}

const submitModel = async () => {
  if (!modelFormRef.value) return
  
  await modelFormRef.value.validate(async (valid) => {
    if (valid) {
      submitting.value = true
      try {
        // 如果选择了不限制token，则将max_tokens设为0
        const submitData = {
          name: modelForm.value.name,
          provider: modelForm.value.provider,
          model: modelForm.value.model,
          base_url: modelForm.value.base_url,
          api_key: modelForm.value.api_key,
          max_tokens: modelForm.value.unlimited_tokens ? 0 : modelForm.value.max_tokens,
          is_active: modelForm.value.is_active
        }
        
        if (editingModel.value) {
          await taskStore.updateModel(editingModel.value.id, submitData)
          ElMessage.success(t('model.updateSuccess'))
        } else {
          await taskStore.createModel(submitData)
          ElMessage.success(t('model.createSuccess'))
        }
        showCreateDialog.value = false
        await loadModels()
        resetModelForm()
      } catch (error) {
        console.error('Model operation failed:', error)
        ElMessage.error(t('model.operationFailed'))
      } finally {
        submitting.value = false
      }
    }
  })
}

const deleteModel = async (modelId: string) => {
  try {
    await ElMessageBox.confirm(t('model.deleteConfirm'), t('common.warning'), {
      confirmButtonText: t('common.confirm'),
      cancelButtonText: t('common.cancel'),
      type: 'warning'
    })
    
    await taskStore.deleteModel(modelId)
    ElMessage.success(t('model.deleteSuccess'))
    await loadModels()
  } catch (error) {
    if (error !== 'cancel') {
      console.error('Delete model failed:', error)
      ElMessage.error(t('model.deleteFailed'))
    }
  }
}

const resetModelForm = () => {
  editingModel.value = null
  modelForm.value = {
    name: '',
    provider: 'openai',
    model: '',
    base_url: '',
    api_key: '',
    max_tokens: 4000,
    unlimited_tokens: false,
    is_active: true
  }
  if (modelFormRef.value) {
    modelFormRef.value.resetFields()
  }
}

onMounted(() => {
  loadModels()
})
</script>

<style scoped>
.models-page {
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

/* 模型卡片 */
.models-card {
  border-radius: 12px;
  margin-bottom: 24px;
}

.loading-container,
.empty-container {
  padding: 60px 0;
}

.models-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(350px, 1fr));
  gap: 16px;
}

.model-card {
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  padding: 16px;
  background: #fff;
  transition: all 0.2s;
}

.model-card:hover {
  border-color: #3b82f6;
  box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.1);
}

.model-active {
  border-color: #10b981;
  background-color: #f0fdf4;
}

.model-header {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 12px;
}

.model-icon {
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

.model-title {
  flex: 1;
}

.model-title h3 {
  margin: 0 0 4px 0;
  font-size: 16px;
  color: #1f2937;
}

.model-provider {
  font-size: 12px;
  color: #6b7280;
}

.model-meta {
  display: flex;
  gap: 16px;
  margin-bottom: 12px;
}

.meta-item {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: #6b7280;
}

.model-config {
  margin-bottom: 16px;
  padding: 12px;
  background-color: #f8fafc;
  border-radius: 6px;
}

.config-item {
  display: flex;
  justify-content: space-between;
  margin-bottom: 8px;
  font-size: 14px;
}

.config-label {
  color: #6b7280;
}

.config-value {
  font-weight: 500;
  color: #1f2937;
}

.model-actions {
  display: flex;
  gap: 8px;
  margin-bottom: 16px;
}

.model-footer {
  display: flex;
  justify-content: flex-end;
  border-top: 1px solid #e5e7eb;
  padding-top: 12px;
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}

.full-width {
  width: 100%;
}

.token-input-container {
  display: flex;
  align-items: center;
  gap: 12px;
}

.token-input-container .el-input-number {
  flex: 1;
}

.unlimited-checkbox {
  white-space: nowrap;
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
  
  .models-grid {
    grid-template-columns: 1fr;
  }
}
</style>
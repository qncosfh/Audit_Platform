<template>
  <div class="create-task">
    <el-page-header @back="$router.push('/tasks')" :title="$t('task.create')" />
    
    <div class="form-container">
      <el-form 
        ref="formRef" 
        :model="form" 
        :rules="rules" 
        label-width="120px"
        class="task-form"
      >
        <!-- 基本信息 -->
        <el-card class="form-card">
          <template #header>
            <div class="card-header">
              <span>{{ $t('task.basicInfo') }}</span>
            </div>
          </template>
          
          <el-form-item :label="$t('task.name')" prop="name">
            <el-input 
              v-model="form.name" 
              :placeholder="$t('task.namePlaceholder')"
              maxlength="100"
              show-word-limit
            />
          </el-form-item>
          
          <el-form-item :label="$t('task.description')" prop="description">
            <el-input 
              v-model="form.description" 
              type="textarea" 
              :rows="3"
              :placeholder="$t('task.descPlaceholder')"
              maxlength="500"
              show-word-limit
            />
          </el-form-item>
          
          <el-form-item :label="$t('codeSource.title')" prop="codeSourceId">
            <el-select 
              v-model="form.codeSourceId" 
              :placeholder="$t('task.selectCodeSource')"
              filterable
              class="full-width"
              @change="handleCodeSourceChange"
              clearable
            >
              <el-option
                v-for="item in codeSources"
                :key="item.id || item.ID"
                :label="item.name"
                :value="item.id || item.ID"
              />
            </el-select>
            <div class="form-tip">
              {{ $t('task.noCodeSource') }}<el-button type="text" @click="$router.push('/code-sources')">{{ $t('task.uploadNow') }}</el-button>
            </div>
          </el-form-item>
        </el-card>

        <!-- 模型配置 -->
        <el-card class="form-card">
          <template #header>
            <div class="card-header">
              <span>{{ $t('model.title') }}</span>
            </div>
          </template>
          
          <el-form-item :label="$t('model.model')" prop="modelConfigId">
            <el-select 
              v-model="form.modelConfigId" 
              :placeholder="$t('task.selectModel')"
              filterable
              class="full-width"
            >
              <el-option
                v-for="model in models"
                :key="model.id"
                :label="model.name"
                :value="model.id"
              >
                <div class="model-option">
                  <div class="model-name">{{ model.name }}</div>
                  <div class="model-desc">{{ model.provider }} - {{ model.model }}</div>
                </div>
              </el-option>
            </el-select>
            <div class="form-tip">
              {{ $t('task.noModel') }}<el-button type="text" @click="$router.push('/models')">{{ $t('task.configNow') }}</el-button>
            </div>
          </el-form-item>
          
          <el-form-item :label="$t('task.customPrompt')" prop="prompt">
            <el-input
              v-model="form.prompt"
              type="textarea"
              :rows="8"
              :placeholder="$t('task.promptPlaceholder')"
            />
            <div class="form-actions">
              <el-button @click="loadDefaultPrompt">{{ $t('task.loadDefaultPrompt') }}</el-button>
              <el-button @click="clearPrompt">{{ $t('task.clearPrompt') }}</el-button>
            </div>
          </el-form-item>
        </el-card>

        <!-- 高级设置 -->
        <el-card class="form-card">
          <template #header>
            <div class="card-header">
              <span>{{ $t('task.advancedSettings') }}</span>
            </div>
          </template>
          
          <el-form-item :label="$t('task.timeout')">
            <el-input-number 
              v-model="form.timeout" 
              :min="60" 
              :max="3600"
              :step="60"
              controls-position="right"
            />
            <span class="unit">{{ $t('task.seconds') }}</span>
          </el-form-item>
        </el-card>

        <!-- 提交按钮 -->
        <div class="form-actions">
          <el-button @click="$router.push('/tasks')">{{ $t('common.cancel') }}</el-button>
          <el-button type="primary" @click="submitForm" :loading="loading">
            {{ $t('task.create') }}
          </el-button>
        </div>
      </el-form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { useTaskStore } from '@/stores/task'
import { useAuthStore } from '@/stores/auth'

const { t } = useI18n()
const router = useRouter()
const taskStore = useTaskStore()
const authStore = useAuthStore()

const loading = ref(false)
const formRef = ref<FormInstance>()
const form = ref({
  name: '',
  description: '',
  codeSourceId: null as number | null,
  modelConfigId: null as number | null,
  prompt: '',
  timeout: 600
})

const rules: FormRules = {
  name: [
    { required: true, message: t('task.nameRequired'), trigger: 'blur' },
    { min: 2, max: 100, message: t('task.nameLength'), trigger: 'blur' }
  ],
  description: [
    { required: true, message: t('task.descRequired'), trigger: 'blur' }
  ],
  codeSourceId: [
    { required: true, message: t('task.selectCodeSource'), trigger: 'change' }
  ],
  modelConfigId: [
    { required: true, message: t('task.selectModel'), trigger: 'change' }
  ]
}

const codeSources = computed(() => taskStore.codeSources)
const models = computed(() => taskStore.models)

const getSourceTypeText = (type: string) => {
  switch (type) {
    case 'zip': return t('codeSource.zip')
    case 'jar': return t('codeSource.jar')
    case 'git': return t('codeSource.git')
    case 'path': return t('codeSource.localPath')
    default: return t('task.unknownType')
  }
}

// 处理代码源选择变化
const handleCodeSourceChange = (sourceId: any) => {
  console.log('Code source changed:', sourceId, typeof sourceId)
  // 如果用户还没有填写提示词，可以自动加载默认提示词
  if (!form.value.prompt) {
    loadDefaultPrompt()
  }
}

const loadDefaultPrompt = () => {
  const prompt = `
  你是一名资深代码安全审计专家，具备丰富的实战渗透经验和代码审计能力，精通 Java、Go、Python、C/C++ 等主流语言及常见框架（如 Spring、Gin、Django 等）。

  【审计目标】
  对提供的代码进行严格安全审计，仅识别“可被直接利用的高危漏洞”，并输出标准化漏洞报告。

  【漏洞范围（仅允许以下类型）】
  仅允许报告以下高危且可利用漏洞：
  	•	SQL注入（SQL Injection）
  	•	命令执行 / 命令注入（RCE）
  	•	不安全反序列化
  	•	路径遍历 / 任意文件读写
  	•	硬编码敏感信息（密码 / Token / API Key）
  	•	SSRF（必须可控目标）
  	•	任意文件上传（可导致RCE）

  【严格禁止报告（避免误报）】
  禁止报告以下内容：
  	•	XSS（除非明确存在完整利用链）
  	•	CSRF
  	•	信息泄露（如版本号、注释等）
  	•	中低危漏洞
  	•	依赖漏洞（未结合实际利用链）
  	•	仅“可能存在”但无法验证的问题

  【严格判定标准（必须全部满足）】
  只有同时满足以下条件才允许报告漏洞：
  	1.	存在明确的用户输入点（如 HTTP 参数、JSON body、文件上传、Header 等）
  	2.	用户输入进入危险函数或敏感操作（如 SQL 执行、系统命令、文件操作等）
  	3.	没有有效安全防护（如未使用参数化查询、无白名单校验、无路径限制等）
  	4.	可以构造真实可执行的攻击 Payload

  【关键要求】
  	•	不允许基于猜测或假设报告漏洞
  	•	不确定的漏洞一律忽略
  	•	必须基于真实代码逻辑分析
  	•	必须体现完整数据流（输入 → 处理 → 危险点）

  【输出格式（必须严格遵守）】
  漏洞报告
  	•	威胁名称:
  	•	严重等级: Critical / High
  	•	位置: 文件名:行号 或 方法名
  	•	漏洞描述:
  （必须说明数据流：输入 → 处理 → 危险点）
  	•	危险代码:
  	•	利用条件:
  	•	POC / EXP:
  	•	修复建议:

  【无漏洞情况】
  如果未发现符合条件的高危漏洞，仅输出：无高危安全漏洞
  不得输出任何其他内容。

  【审计重点提示】
  优先分析以下路径：
  	•	Controller → Service → DAO
  	•	用户输入 → SQL拼接
  	•	用户输入 → 命令执行
  	•	用户输入 → 文件路径操作
  	•	用户输入 → 反序列化入口
  `

  form.value.prompt = prompt
}
const clearPrompt = () => {
  form.value.prompt = ''
}

const submitForm = async () => {
  if (!formRef.value) return
  
  await formRef.value.validate(async (valid) => {
    if (valid) {
      loading.value = true
      try {
        const codeSourceId = form.value.codeSourceId
        const modelConfigId = form.value.modelConfigId
        
        console.log('表单 codeSourceId:', codeSourceId, typeof codeSourceId)
        console.log('表单 modelConfigId:', modelConfigId, typeof modelConfigId)
        
        const taskData = {
          name: form.value.name,
          description: form.value.description,
          codeSourceId: String(codeSourceId),
          modelConfigId: String(modelConfigId),
          prompt: form.value.prompt,
          timeout: form.value.timeout
        }
        console.log('Submitting task data:', taskData)
        await taskStore.createTask(taskData)
        ElMessage.success(t('task.createSuccess'))
        router.push('/tasks')
      } catch (error: any) {
        console.error('Failed to create task:', error)
        ElMessage.error(error?.message || t('task.createError'))
      } finally {
        loading.value = false
      }
    }
  })
}

onMounted(async () => {
  try {
    await Promise.all([
      taskStore.loadCodeSources(),
      taskStore.loadModels()
    ])
  } catch (error) {
    console.error('Failed to load data:', error)
  }
})
</script>

<style scoped>
.create-task {
  padding: 24px;
}

.form-container {
  max-width: 800px;
  margin: 0 auto;
}

.form-card {
  margin-bottom: 24px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.source-option, .model-option {
  display: flex;
  flex-direction: column;
}

.source-name, .model-name {
  font-weight: 500;
}

.source-desc, .model-desc {
  font-size: 12px;
  color: #666;
  margin-top: 2px;
}

.form-tip {
  margin-top: 8px;
  font-size: 12px;
  color: #999;
}

.form-actions {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  margin-top: 16px;
}

.unit {
  margin-left: 8px;
  color: #666;
}

.full-width {
  width: 100%;
}

@media (max-width: 768px) {
  .form-container {
    padding: 0 16px;
  }
  
  .form-card {
    margin-bottom: 16px;
  }
  
  .form-actions {
    flex-direction: column;
    align-items: flex-end;
  }
}
</style>

<template>
  <div class="register-container">
    <div class="register-card">
      <div class="register-header">
        <el-icon class="register-icon"><OfficeBuilding /></el-icon>
        <h1>{{ $t('auth.registerTitle') }}</h1>
        <p>{{ $t('auth.registerDesc') }}</p>
      </div>

      <el-form 
        ref="registerFormRef" 
        :model="registerForm" 
        :rules="registerRules" 
        class="register-form"
        label-position="top"
      >
        <el-row :gutter="20">
          <el-col :span="12">
            <el-form-item :label="$t('auth.applicantName')" prop="username">
              <el-input 
                v-model="registerForm.username" 
                :placeholder="$t('auth.enterName')"
                :prefix-icon="User"
                size="large"
              />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item :label="$t('auth.phone')" prop="phone">
              <el-input 
                v-model="registerForm.phone" 
                :placeholder="$t('auth.enterPhone')"
                :prefix-icon="Phone"
                size="large"
              />
            </el-form-item>
          </el-col>
        </el-row>

        <el-form-item :label="$t('auth.companyName')" prop="company">
          <el-input 
            v-model="registerForm.company" 
            :placeholder="$t('auth.enterCompany')"
            :prefix-icon="OfficeBuilding"
            size="large"
          />
        </el-form-item>

        <el-form-item :label="$t('auth.industry')" prop="industry">
          <el-select 
            v-model="registerForm.industry" 
            :placeholder="$t('auth.selectIndustry')"
            size="large"
            style="width: 100%"
          >
            <el-option :label="$t('auth.industryInternet')" value="互联网/软件" />
            <el-option :label="$t('auth.industryFinance')" value="金融/银行" />
            <el-option :label="$t('auth.industryEcommerce')" value="电商/零售" />
            <el-option :label="$t('auth.industryManufacturing')" value="制造业" />
            <el-option :label="$t('auth.industryEducation')" value="教育/科研" />
            <el-option :label="$t('auth.industryHealthcare')" value="医疗健康" />
            <el-option :label="$t('auth.industryGovernment')" value="政府/公共服务" />
            <el-option :label="$t('auth.industryOther')" value="其他" />
          </el-select>
        </el-form-item>

        <el-form-item :label="$t('auth.email')" prop="email">
          <el-input 
            v-model="registerForm.email" 
            type="email"
            :placeholder="$t('auth.enterEmail')"
            :prefix-icon="Message"
            size="large"
          />
        </el-form-item>

        <el-form-item :label="$t('auth.userCount')" prop="userCount">
          <el-select 
            v-model="registerForm.userCount" 
            :placeholder="$t('auth.selectUserCount')"
            size="large"
            style="width: 100%"
          >
            <el-option :label="$t('auth.userCount1')" value="1-5" />
            <el-option :label="$t('auth.userCount2')" value="6-20" />
            <el-option :label="$t('auth.userCount3')" value="21-50" />
            <el-option :label="$t('auth.userCount4')" value="50+" />
          </el-select>
        </el-form-item>

        <el-form-item :label="$t('auth.useCase')" prop="description">
          <el-input 
            v-model="registerForm.description" 
            type="textarea"
            :rows="3"
            :placeholder="$t('auth.enterUseCase')"
            size="large"
          />
        </el-form-item>

        <el-form-item>
          <el-button 
            type="primary" 
            size="large" 
            :loading="loading"
            @click="handleRegister"
            class="register-button"
          >
            {{ $t('auth.submitApplication') }}
          </el-button>
        </el-form-item>

        <div class="form-footer">
          <span>{{ $t('auth.hasAccount') }}</span>
          <el-button type="text" @click="$router.push('/login')">{{ $t('auth.loginNow') }}</el-button>
        </div>
      </el-form>
    </div>

    <!-- 申请成功对话框 -->
    <el-dialog
      v-model="showSuccessDialog"
      :title="$t('auth.applicationSuccess')"
      width="500px"
      :close-on-click-modal="false"
      :show-close="false"
    >
      <div class="success-content">
        <el-icon class="success-icon" color="#67c23a"><CircleCheckFilled /></el-icon>
        <h3>{{ $t('auth.thankYou') }}</h3>
        <p>{{ $t('auth.applicationReceived') }}</p>
        <div class="success-info">
          <p><strong>{{ $t('auth.resultWillSend') }}</strong></p>
          <p class="email">{{ registerForm.email }}</p>
        </div>
        <p class="tip">{{ $t('auth.checkEmailTip') }}</p>
      </div>
      <template #footer>
        <el-button type="primary" @click="goToLogin">{{ $t('auth.goToLogin') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import {
  OfficeBuilding,
  User,
  Phone,
  Message,
  Lock,
  CircleCheckFilled
} from '@element-plus/icons-vue'
import { authApi } from '@/api'

const { t } = useI18n()
const router = useRouter()

const loading = ref(false)
const registerFormRef = ref<FormInstance>()
const showSuccessDialog = ref(false)

const registerForm = reactive({
  username: '',
  phone: '',
  company: '',
  industry: '',
  email: '',
  userCount: '',
  description: ''
})

const registerRules: FormRules = {
  username: [
    { required: true, message: t('auth.enterName'), trigger: 'blur' },
    { min: 2, max: 50, message: t('auth.nameMinLength'), trigger: 'blur' },
    { pattern: /^[\u4e00-\u9fa5a-zA-Z0-9_]+$/, message: '用户名只能包含中文、字母、数字和下划线', trigger: 'blur' }
  ],
  phone: [
    { required: true, message: t('auth.enterPhone'), trigger: 'blur' },
    { pattern: /^1[3-9]\d{9}$/, message: '请输入正确的手机号格式', trigger: 'blur' }
  ],
  company: [
    { required: true, message: t('auth.enterCompany'), trigger: 'blur' },
    { min: 2, max: 100, message: '公司名称长度为2-100个字符', trigger: 'blur' }
  ],
  industry: [
    { required: true, message: t('auth.selectIndustry'), trigger: 'change' }
  ],
  email: [
    { required: true, message: t('auth.enterEmail'), trigger: 'blur' },
    { type: 'email', message: t('auth.emailInvalid'), trigger: 'blur' }
  ],
  userCount: [
    { required: true, message: t('auth.selectUserCount'), trigger: 'change' }
  ],
  description: [
    { required: true, message: t('auth.enterUseCase'), trigger: 'blur' },
    { min: 10, max: 500, message: '使用描述长度为10-500个字符', trigger: 'blur' }
  ]
}

const handleRegister = async () => {
  if (!registerFormRef.value) return

  try {
    await registerFormRef.value.validate()
    loading.value = true

    // 生成随机密码（8位，包含大小写字母、数字和特殊字符）
    const defaultPassword = 'Pass1234!'

    // 调用后端注册API，发送完整申请信息
    const response = await authApi.register({
      username: registerForm.username,
      email: registerForm.email,
      password: defaultPassword,
      phone: registerForm.phone,
      company: registerForm.company,
      industry: registerForm.industry,
      userCount: registerForm.userCount,
      description: registerForm.description
    })
    
    // 保存token
    if (response.data.Data?.token) {
      localStorage.setItem('auth_token', response.data.Data.token)
      localStorage.setItem('auth_token_expiry', String(Date.now() + 24 * 60 * 60 * 1000))
      localStorage.setItem('user', JSON.stringify(response.data.Data.user))
    }
    
    // 显示成功对话框
    showSuccessDialog.value = true
    
  } catch (error: any) {
    ElMessage.error(error.message || t('auth.registerFailed'))
  } finally {
    loading.value = false
  }
}

const goToLogin = () => {
  showSuccessDialog.value = false
  router.push('/login')
}
</script>

<style scoped>
.register-container {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  padding: 20px;
}

.register-card {
  width: 100%;
  max-width: 700px;
  background: #ffffff;
  padding: 40px;
  border-radius: 16px;
  box-shadow: 0 10px 30px rgba(0, 0, 0, 0.2);
  animation: slideUp 0.3s ease-out;
}

@keyframes slideUp {
  from {
    opacity: 0;
    transform: translateY(20px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.register-header {
  text-align: center;
  margin-bottom: 30px;
}

.register-icon {
  font-size: 48px;
  color: #667eea;
  margin-bottom: 16px;
}

.register-header h1 {
  margin: 0 0 8px 0;
  font-size: 24px;
  color: #1f2937;
  font-weight: 600;
}

.register-header p {
  margin: 0;
  color: #6b7280;
  font-size: 14px;
}

.register-form {
  margin-top: 20px;
}

.register-form :deep(.el-form-item__label) {
  font-weight: 500;
  color: #374151;
}

.register-button {
  width: 100%;
  height: 44px;
  font-size: 16px;
  margin-top: 10px;
}

.form-footer {
  text-align: center;
  margin-top: 20px;
  color: #6b7280;
  font-size: 14px;
}

.form-footer span {
  margin-right: 8px;
}

/* 成功对话框样式 */
.success-content {
  text-align: center;
  padding: 20px 0;
}

.success-icon {
  font-size: 64px;
  margin-bottom: 20px;
}

.success-content h3 {
  font-size: 20px;
  color: #1f2937;
  margin: 0 0 16px 0;
}

.success-content p {
  color: #6b7280;
  margin: 0 0 12px 0;
  line-height: 1.6;
}

.success-info {
  background: #f0f9ff;
  padding: 16px;
  border-radius: 8px;
  margin: 20px 0;
}

.success-info p {
  margin: 0;
  color: #1f2937;
}

.success-info .email {
  font-size: 18px;
  font-weight: 600;
  color: #3b82f6;
  margin-top: 8px;
}

.tip {
  font-size: 13px;
  color: #9ca3af;
  padding: 12px;
  background: #fef3c7;
  border-radius: 8px;
  margin-top: 20px;
}
</style>
<template>
  <div class="login-container">
    <div class="login-card">
      <!-- 语言切换 -->
      <div class="language-switch">
        <LanguageSwitch />
      </div>

      <div class="login-header">
        <el-icon class="login-icon"><Aim /></el-icon>
        <h1>{{ $t('app.title') }}</h1>
        <p>{{ $t('auth.welcomeBack') }}</p>
      </div>

      <el-form 
        ref="loginFormRef" 
        :model="loginForm" 
        :rules="loginRules" 
        class="login-form"
        @keyup.enter="handleLogin"
      >
        <el-form-item prop="username">
          <el-input 
            v-model="loginForm.username" 
            :placeholder="$t('auth.username')"
            :prefix-icon="User"
            size="large"
            @keyup.enter="handleLogin"
          />
        </el-form-item>

        <el-form-item prop="password">
          <el-input 
            v-model="loginForm.password" 
            type="password" 
            :placeholder="$t('auth.password')"
            :prefix-icon="Lock"
            size="large"
            show-password
            @keyup.enter="handleLogin"
          />
        </el-form-item>

        <el-form-item>
          <el-button 
            type="primary" 
            size="large" 
            :loading="loading"
            @click="handleLogin"
            class="login-button"
          >
            {{ $t('auth.login') }}
          </el-button>
        </el-form-item>

        <div class="form-footer">
          <span>{{ $t('auth.noAccount') }}</span>
          <el-button type="text" @click="$router.push('/register')">{{ $t('auth.registerNow') }}</el-button>
        </div>
      </el-form>

      <div class="security-notice">
        <el-icon><WarningFilled /></el-icon>
        <span>{{ $t('auth.securityNotice') }}</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { useAuthStore } from '@/stores/auth'
import LanguageSwitch from '@/components/LanguageSwitch.vue'
import {
  Aim,
  User,
  Lock,
  WarningFilled
} from '@element-plus/icons-vue'

const { t } = useI18n()
const router = useRouter()
const authStore = useAuthStore()

const loading = ref(false)
const loginFormRef = ref<FormInstance>()

const loginForm = reactive({
  username: '',
  password: ''
})

// 密码强度验证 - 与后端保持一致
const validatePassword = (rule: any, value: string, callback: any) => {
  if (!value) {
    callback(new Error(t('auth.passwordRequired')))
    return
  }
  
  // 验证密码长度
  if (value.length < 8) {
    callback(new Error('密码长度至少8个字符'))
    return
  }
  
  // 验证密码复杂度
  const hasUpper = /[A-Z]/.test(value)
  const hasLower = /[a-z]/.test(value)
  const hasDigit = /[0-9]/.test(value)
  const hasSpecial = /[!@#$%^&*]/.test(value)
  
  if (!hasUpper) {
    callback(new Error('密码必须包含大写字母'))
    return
  }
  if (!hasLower) {
    callback(new Error('密码必须包含小写字母'))
    return
  }
  if (!hasDigit) {
    callback(new Error('密码必须包含数字'))
    return
  }
  if (!hasSpecial) {
    callback(new Error('密码必须包含特殊字符(!@#$%^&*)'))
    return
  }
  
  callback()
}

const loginRules: FormRules = {
  username: [
    { required: true, message: t('auth.usernameRequired'), trigger: 'blur' },
    { min: 3, message: t('auth.usernameMinLength'), trigger: 'blur' }
  ],
  password: [
    { required: true, message: t('auth.passwordRequired'), trigger: 'blur' },
    { validator: validatePassword, trigger: 'blur' }
  ]
}

const handleLogin = async () => {
  if (!loginFormRef.value) return

  try {
    await loginFormRef.value.validate()
    loading.value = true

    const result = await authStore.login(loginForm)
    
    if (result.success) {
      ElMessage.success(t('auth.loginSuccess'))
      router.push('/dashboard')
    } else {
      ElMessage.error(t('auth.loginError'))
    }
  } catch (error) {
    ElMessage.error(t('auth.loginError'))
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-container {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  padding: 20px;
}

.language-switch {
  position: absolute;
  top: 16px;
  right: 16px;
}

.login-card {
  width: 100%;
  max-width: 400px;
  background: #ffffff;
  padding: 40px;
  border-radius: 16px;
  box-shadow: 0 10px 30px rgba(0, 0, 0, 0.2);
  animation: slideUp 0.3s ease-out;
  position: relative;
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

.login-header {
  text-align: center;
  margin-bottom: 30px;
}

.login-icon {
  font-size: 48px;
  color: #667eea;
  margin-bottom: 16px;
}

.login-header h1 {
  margin: 0 0 8px 0;
  font-size: 24px;
  color: #1f2937;
  font-weight: 600;
}

.login-header p {
  margin: 0;
  color: #6b7280;
  font-size: 14px;
}

.login-form {
  margin-top: 20px;
}

.login-button {
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

.security-notice {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  margin-top: 20px;
  padding: 12px;
  background-color: #fef3c7;
  border-radius: 8px;
  color: #92400e;
  font-size: 12px;
}
</style>
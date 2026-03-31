<template>
  <div class="layout">
    <!-- 顶部导航栏 -->
    <el-header class="header">
      <div class="header-content">
        <div class="logo">
          <img src="/logo.png" alt="logo" class="logo-img" />
          <span class="logo-text">{{ $t('app.title') }}</span>
        </div>
        
        <div class="header-actions">
          <LanguageSwitch />
          <el-dropdown trigger="click" @command="handleCommand">
            <div class="user-avatar">
              <el-avatar :size="36" :style="avatarStyle">
                {{ userInitial }}
              </el-avatar>
            </div>
            <template #dropdown>
              <el-dropdown-menu class="user-dropdown-menu">
                <div class="user-info">
                  <el-avatar :size="48" :style="avatarStyle">{{ userInitial }}</el-avatar>
                  <div class="user-detail">
                    <div class="username">{{ authStore.user?.username }}</div>
                    <div class="email">{{ authStore.user?.email }}</div>
                  </div>
                </div>
                <el-dropdown-item divided command="password">
                  <el-icon><Lock /></el-icon>
                  {{ $t('auth.changePassword') }}
                </el-dropdown-item>
                <el-dropdown-item divided command="logout">
                  <el-icon><SwitchButton /></el-icon>
                  {{ $t('auth.logout') }}
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </div>
    </el-header>

    <!-- 主要内容区域 -->
    <el-container class="main-container">
      <!-- 侧边栏 -->
      <el-aside class="sidebar" width="240px">
        <el-menu
          :default-active="$route.path"
          class="sidebar-menu"
          background-color="#1f2937"
          text-color="#e5e7eb"
          active-text-color="#60a5fa"
        >
          <el-menu-item index="/dashboard" @click="router.push('/dashboard')">
            <el-icon><House /></el-icon>
            <span>{{ $t('nav.dashboard') }}</span>
          </el-menu-item>
          
          <el-menu-item index="/tasks" @click="router.push('/tasks')">
            <el-icon><Document /></el-icon>
            <span>{{ $t('nav.tasks') }}</span>
          </el-menu-item>
          
          <el-menu-item index="/code-sources" @click="router.push('/code-sources')">
            <el-icon><Folder /></el-icon>
            <span>{{ $t('nav.codeSources') }}</span>
          </el-menu-item>
          
          <el-menu-item index="/models" @click="router.push('/models')">
            <el-icon><Cpu /></el-icon>
            <span>{{ $t('nav.models') }}</span>
          </el-menu-item>
          
          <el-menu-item index="/reports" @click="router.push('/reports')">
            <el-icon><DataAnalysis /></el-icon>
            <span>{{ $t('nav.reports') }}</span>
          </el-menu-item>
        </el-menu>
      </el-aside>

      <!-- 内容区域 -->
      <el-main class="content">
        <router-view />
      </el-main>
    </el-container>

    <!-- 修改密码对话框 -->
    <el-dialog
      v-model="showPasswordDialog"
      :title="$t('auth.changePassword')"
      width="420px"
      :close-on-click-modal="false"
    >
      <el-form 
        ref="passwordFormRef"
        :model="passwordForm" 
        :rules="passwordRules"
        label-position="top"
      >
        <el-form-item :label="$t('auth.currentPassword')" prop="oldPassword">
          <el-input 
            v-model="passwordForm.oldPassword" 
            type="password" 
            :placeholder="$t('auth.enterCurrentPassword')"
            show-password
          />
        </el-form-item>
        <el-form-item :label="$t('auth.newPassword')" prop="newPassword">
          <el-input 
            v-model="passwordForm.newPassword" 
            type="password" 
            :placeholder="$t('auth.newPasswordPlaceholder')"
            show-password
          />
        </el-form-item>
        <el-form-item :label="$t('auth.confirmNewPassword')" prop="confirmPassword">
          <el-input 
            v-model="passwordForm.confirmPassword" 
            type="password" 
            :placeholder="$t('auth.confirmNewPasswordPlaceholder')"
            show-password
          />
        </el-form-item>
      </el-form>
      
      <template #footer>
        <el-button @click="showPasswordDialog = false">{{ $t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="passwordLoading" @click="handleChangePassword">
          {{ $t('common.confirm') }}
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { authApi } from '@/api'
import { setLoggingOut } from '@/router'
import { ElMessage, FormInstance, FormRules } from 'element-plus'
import {
  Aim,
  Plus,
  House,
  Document,
  Folder,
  Cpu,
  DataAnalysis,
  User,
  Lock,
  SwitchButton
} from '@element-plus/icons-vue'
import LanguageSwitch from './LanguageSwitch.vue'

const { t } = useI18n()

const router = useRouter()
const authStore = useAuthStore()
const { locale } = useI18n()

// 头像颜色数组
const avatarColors = [
  { bg: '#409eff', text: '#ffffff' },
  { bg: '#67c23a', text: '#ffffff' },
  { bg: '#e6a23c', text: '#ffffff' },
  { bg: '#f56c6c', text: '#ffffff' },
  { bg: '#909399', text: '#ffffff' },
  { bg: '#c985f5', text: '#ffffff' },
  { bg: '#36b5c9', text: '#ffffff' },
  { bg: '#7dbf6f', text: '#ffffff' }
]

// 根据用户名生成固定颜色
const avatarStyle = computed(() => {
  const username = authStore.user?.username || 'U'
  const index = username.charCodeAt(0) % avatarColors.length
  const color = avatarColors[index]
  return {
    backgroundColor: color.bg,
    color: color.text,
    fontSize: '16px',
    fontWeight: '600'
  }
})

const userInitial = computed(() => {
  return authStore.user?.username?.charAt(0).toUpperCase() || 'U'
})

// 修改密码相关
const showPasswordDialog = ref(false)
const passwordLoading = ref(false)
const passwordFormRef = ref<FormInstance>()

const passwordForm = reactive({
  oldPassword: '',
  newPassword: '',
  confirmPassword: ''
})

const validateConfirmPassword = (rule: any, value: string, callback: any) => {
  if (value !== passwordForm.newPassword) {
    callback(new Error(t('auth.passwordMismatch')))
  } else {
    callback()
  }
}

const passwordRules: FormRules = {
  oldPassword: [
    { required: true, message: t('auth.enterCurrentPassword'), trigger: 'blur' }
  ],
  newPassword: [
    { required: true, message: t('auth.newPassword'), trigger: 'blur' },
    { min: 8, message: t('auth.passwordMinLength'), trigger: 'blur' }
  ],
  confirmPassword: [
    { required: true, message: t('auth.confirmNewPassword'), trigger: 'blur' },
    { validator: validateConfirmPassword, trigger: 'blur' }
  ]
}

const handleCommand = (command: string) => {
  switch (command) {
    case 'profile':
      router.push('/dashboard')
      break
    case 'password':
      showPasswordDialog.value = true
      break
    case 'dashboard':
      router.push('/dashboard')
      break
    case 'tasks':
      router.push('/tasks')
      break
    case 'sources':
      router.push('/code-sources')
      break
    case 'models':
      router.push('/models')
      break
    case 'reports':
      router.push('/reports')
      break
    case 'logout':
      handleLogout()
      break
  }
}

const handleChangePassword = async () => {
  if (!passwordFormRef.value) return
  
  await passwordFormRef.value.validate(async (valid) => {
    if (!valid) return
    
    passwordLoading.value = true
    try {
      await authApi.changePassword(passwordForm.oldPassword, passwordForm.newPassword)
      ElMessage.success(t('auth.passwordChangedSuccess'))
      showPasswordDialog.value = false
      
      // 清空表单
      passwordForm.oldPassword = ''
      passwordForm.newPassword = ''
      passwordForm.confirmPassword = ''
      
      // 退出登录
      await handleLogout()
    } catch (error: any) {
      ElMessage.error(error.message || t('auth.passwordChangeFailed'))
    } finally {
      passwordLoading.value = false
    }
  })
}

const handleLogout = async () => {
  // 设置退出登录状态，防止路由守卫触发认证检查
  setLoggingOut(true)
  
  // 先清理本地状态，确保任何pending的API调用不会再触发错误提示
  const token = localStorage.getItem('auth_token')
  localStorage.removeItem('auth_token')
  localStorage.removeItem('auth_token_expiry')
  authStore.$patch({ token: null, user: null })
  
  try {
    // 尝试调用后端登出API（忽略结果，因为本地已经清理了）
    await authApi.logout().catch(() => {})
  } catch (error) {
    // 忽略API错误
  }
  
  ElMessage.success(t('auth.logoutSuccess'))
  router.push('/login')
}
</script>

<style scoped>
.layout {
  min-height: 100vh;
  background-color: #f3f4f6;
}

.header {
  background: #ffffff;
  border-bottom: 1px solid #e5e7eb;
  padding: 0;
  height: 64px;
  line-height: 64px;
  z-index: 1000;
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
}

.header-content {
  display: flex;
  justify-content: space-between;
  align-items: center;
  height: 100%;
  padding: 0 24px;
  width: 100%;
}

.logo {
  display: flex;
  align-items: center;
  gap: 12px;
}

.logo-img {
  width: 56px;
  height: 56px;
  object-fit: contain;
}

.logo-icon {
  font-size: 24px;
  color: #60a5fa;
}

.logo-text {
  font-size: 20px;
  font-weight: 600;
  color: #111827;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 16px;
}

.user-avatar {
  cursor: pointer;
  padding: 4px;
  border-radius: 50%;
  transition: background-color 0.2s;
}

.user-avatar:hover {
  background-color: #f3f4f6;
}

.main-container {
  display: flex;
  padding: 24px;
  padding-left: 264px;
  padding-top: 88px; /* 64px header height + 24px padding */
  gap: 24px;
  width: 100%;
  max-width: 100%;
  box-sizing: border-box;
  position: relative;
}

.sidebar {
  background: #1f2937;
  border-radius: 12px;
  overflow: hidden;
  z-index: 10;
  height: calc(100vh - 64px - 48px);
  max-height: calc(100vh - 64px - 48px);
  display: flex;
  flex-direction: column;
  position: fixed;
  left: 24px;
  top: 88px; /* header height 64px + main-container padding-top 24px */
}

.sidebar-menu {
  border-right: none;
  height: calc(100vh - 64px - 48px);
  min-height: calc(100vh - 64px - 48px);
  padding: 16px 8px;
  display: flex;
  flex-direction: column;
}

.sidebar-menu :deep(.el-menu-item) {
  flex-shrink: 0;
}

/* 菜单项样式优化 */
:deep(.el-menu-item) {
  height: 48px;
  line-height: 48px;
  margin: 4px 0;
  border-radius: 8px;
  font-size: 14px;
  transition: all 0.3s ease;
}

:deep(.el-menu-item:hover) {
  background-color: rgba(96, 165, 250, 0.15) !important;
}

:deep(.el-menu-item.is-active) {
  background-color: rgba(96, 165, 250, 0.2) !important;
  color: #60a5fa !important;
}

:deep(.el-menu-item.is-active::before) {
  content: '';
  position: absolute;
  left: 0;
  top: 50%;
  transform: translateY(-50%);
  width: 4px;
  height: 24px;
  background: #60a5fa;
  border-radius: 0 4px 4px 0;
}

:deep(.el-menu-item .el-icon) {
  font-size: 18px;
  margin-right: 12px;
}

.content {
  flex: 1;
  padding: 0;
  background: #ffffff;
  border-radius: 12px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
  position: relative;
  z-index: 1;
}

/* 用户信息下拉 */
.user-dropdown-menu {
  padding: 0;
  min-width: 180px;
}

.user-info {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 16px;
  margin: 0;
  border-bottom: 1px solid #f3f4f6;
  background: #f9fafb;
}

.user-detail {
  flex: 1;
  min-width: 0;
}

.username {
  font-size: 14px;
  font-weight: 600;
  color: #111827;
}

.email {
  font-size: 12px;
  color: #6b7280;
  margin-top: 2px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* 下拉菜单项样式 */
:deep(.el-dropdown-menu__item) {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 16px;
}

:deep(.el-dropdown-menu__item--divided) {
  margin-top: 0;
  border-top: 1px solid #f3f4f6;
}
</style>

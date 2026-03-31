<template>
  <div class="not-found-page">
    <div class="not-found-content">
      <div class="error-code">
        404
        <img src="../../hhh.jpeg" alt="hhh" class="hhh-image" />
      </div>
      <h1>{{ $t('common.notFound') }}</h1>
      <p>{{ $t('common.notFoundDesc') }}</p>
      <div class="actions">
        <el-button type="primary" size="large" @click="goHome">{{ $t('common.backHome') }}</el-button>
        <el-button size="large" @click="goBack">{{ $t('common.backPrev') }}</el-button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'

const { t } = useI18n()
const router = useRouter()
const authStore = useAuthStore()

// 判断是否已登录
const isAuthenticated = computed(() => authStore.isAuthenticated)

// 根据登录状态决定返回哪里
const goHome = () => {
  if (isAuthenticated.value) {
    // 已登录，跳转到仪表盘
    router.push('/dashboard')
  } else {
    // 未登录，跳转到登录页
    router.push('/login')
  }
}

const goBack = () => {
  router.back()
}
</script>

<style scoped>
.not-found-page {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  display: flex;
  justify-content: center;
  align-items: center;
  background-image: url('../../hhh.jpeg');
  background-size: cover;
  background-position: center;
  background-repeat: no-repeat;
}

.not-found-page::before {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.5);
}

.not-found-content {
  position: relative;
  z-index: 1;
  text-align: center;
  color: white;
  padding: 40px;
}

.error-code {
  font-size: 150px;
  font-weight: bold;
  line-height: 1;
  margin-bottom: 20px;
  text-shadow: 2px 2px 4px rgba(0, 0, 0, 0.5);
  letter-spacing: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 20px;
}

.hhh-image {
  width: 100px;
  height: auto;
  border-radius: 8px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
}

h1 {
  font-size: 36px;
  margin-bottom: 16px;
  font-weight: 500;
}

p {
  font-size: 18px;
  margin-bottom: 40px;
  opacity: 0.9;
}

.actions {
  display: flex;
  gap: 24px;
  justify-content: center;
}

.actions .el-button {
  padding: 16px 40px;
  font-size: 16px;
  border-radius: 8px;
}

.actions .el-button--primary {
  background: rgba(59, 130, 246, 0.9);
  border-color: rgba(59, 130, 246, 0.9);
}

.actions .el-button--default {
  background: rgba(255, 255, 255, 0.2);
  border-color: rgba(255, 255, 255, 0.5);
  color: white;
}

.actions .el-button:hover {
  opacity: 0.9;
  transform: translateY(-2px);
  transition: all 0.3s;
}
</style>
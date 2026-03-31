<template>
  <div id="app">
    <el-config-provider :locale="elementLocale">
      <!-- 需要隐藏Layout的页面（如404）直接显示 -->
      <router-view v-if="$route.meta.hideLayout" />
      <!-- 已登录用户显示 Layout（Layout内部有router-view） -->
      <Layout v-else-if="authStore.isAuthenticated" />
      <!-- 未登录用户显示登录/注册页面 -->
      <router-view v-else />
    </el-config-provider>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElConfigProvider } from 'element-plus'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import en from 'element-plus/es/locale/lang/en'
import { useAuthStore } from './stores/auth'
import Layout from './components/Layout.vue'

const route = useRoute()
const authStore = useAuthStore()
const { locale } = useI18n()

// Element Plus locale
const elementLocale = computed(() => {
  return locale.value === 'en' ? en : zhCn
})

// 页面加载时检查认证状态
onMounted(async () => {
  await authStore.checkAuth()
})
</script>

<style>
/* 全局样式 */
#app {
  min-height: 100vh;
}

* {
  box-sizing: border-box;
}

body {
  margin: 0;
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
  background-color: #f3f4f6;
  color: #1f2937;
}

/* Element Plus 暗色主题样式 */
:root {
  --el-color-primary: #3b82f6;
  --el-color-primary-light-3: #93c5fd;
  --el-color-primary-light-7: #e0f2fe;
  --el-color-primary-dark-2: #1d4ed8;
}

/* 滚动条样式 */
::-webkit-scrollbar {
  width: 8px;
  height: 8px;
}

::-webkit-scrollbar-track {
  background: #f1f5f9;
}

::-webkit-scrollbar-thumb {
  background: #cbd5e1;
  border-radius: 4px;
}

::-webkit-scrollbar-thumb:hover {
  background: #94a3b8;
}
</style>
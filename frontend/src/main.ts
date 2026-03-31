import { createApp, watch } from 'vue'
import { createPinia } from 'pinia'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
import 'element-plus/theme-chalk/dark/css-vars.css'
import * as ElementPlusIconsVue from '@element-plus/icons-vue'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import en from 'element-plus/es/locale/lang/en'

import App from './App.vue'
import router from './router'
import i18n from './locales'

const app = createApp(App)

// 注册所有图标
for (const [key, component] of Object.entries(ElementPlusIconsVue)) {
  app.component(key, component)
}

// Element Plus locale映射
const elementLocales: Record<string, any> = {
  'zh-CN': zhCn,
  'en': en
}

// 获取当前语言设置
const getElementLocale = () => {
  const lang = localStorage.getItem('language') || 'zh-CN'
  return elementLocales[lang] || zhCn
}

// 设置动态标题
const setDocumentTitle = () => {
  const lang = localStorage.getItem('language') || 'zh-CN'
  const title = lang === 'en' ? 'AI Code Audit Platform' : '熔炉——AI智审平台'
  document.title = title
  const titleEl = document.getElementById('app-title')
  if (titleEl) {
    titleEl.textContent = title
  }
}

// 更新Element Plus的locale
const updateElementLocale = () => {
  const lang = localStorage.getItem('language') || 'zh-CN'
  const locale = elementLocales[lang] || zhCn
  // 更新Element Plus的全局locale配置
  ;(ElementPlus as any)._context?.appContext?.config?.globalProperties?.$message?.locale?.()
}

app.use(createPinia())
app.use(router)
app.use(i18n)
app.use(ElementPlus, {
  locale: getElementLocale(),
})

// 初始化标题
setDocumentTitle()

// 监听语言变化更新标题和Element Plus locale
watch(() => i18n.global.locale.value, (newLocale) => {
  setDocumentTitle()
  // 动态更新Element Plus的locale
  const elLocale = elementLocales[newLocale] || zhCn
  // 重新配置Element Plus的locale
  const el = ElementPlus as any
  if (el._context && el._context.globalProperties) {
    const instance = app.config.globalProperties.$ELEMENT
    if (instance) {
      instance.locale = elLocale
    }
  }
})

app.mount('#app')

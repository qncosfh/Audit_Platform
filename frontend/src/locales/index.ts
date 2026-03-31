import { createI18n } from 'vue-i18n'
import zhCN from './zh-CN'
import en from './en'

// 获取保存的语言设置，默认为中文
const getStoredLanguage = () => {
  const stored = localStorage.getItem('language')
  if (stored === 'en' || stored === 'zh-CN') {
    return stored
  }
  // 尝试使用浏览器语言
  const browserLang = navigator.language.toLowerCase()
  if (browserLang.startsWith('en')) {
    return 'en'
  }
  return 'zh-CN'
}

const i18n = createI18n({
  legacy: false,
  locale: getStoredLanguage(),
  fallbackLocale: 'zh-CN',
  messages: {
    'zh-CN': zhCN,
    'en': en
  }
})

export default i18n
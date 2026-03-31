<template>
  <el-dropdown @command="handleCommand" trigger="click">
    <el-button text>
      <el-icon><Monitor /></el-icon>
      <span class="lang-text">{{ currentLanguageLabel }}</span>
    </el-button>
    <template #dropdown>
      <el-dropdown-menu>
        <el-dropdown-item command="zh-CN">
          <span>{{ $t('common.chinese') }}</span>
          <el-icon v-if="locale === 'zh-CN'" class="check-icon"><Check /></el-icon>
        </el-dropdown-item>
        <el-dropdown-item command="en">
          <span>{{ $t('common.english') }}</span>
          <el-icon v-if="locale === 'en'" class="check-icon"><Check /></el-icon>
        </el-dropdown-item>
      </el-dropdown-menu>
    </template>
  </el-dropdown>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Check, Monitor } from '@element-plus/icons-vue'

const { locale, t } = useI18n()

const currentLanguageLabel = computed(() => {
  return locale.value === 'en' ? t('common.english') : t('common.chinese')
})

const handleCommand = (command: string) => {
  locale.value = command
  localStorage.setItem('language', command)
}
</script>

<style scoped>
.lang-text {
  margin-left: 4px;
}

.check-icon {
  margin-left: 8px;
  color: #409eff;
}

:deep(.el-dropdown-menu__item) {
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-width: 120px;
}
</style>
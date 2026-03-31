<template>
  <div class="code-source-detail">
    <!-- 页面头部 -->
    <div class="page-header">
      <div class="header-left">
        <el-page-header @back="$router.push('/code-sources')" :title="codeSource?.name || t('codeSource.codeSourceDetail')" />
      </div>
      <div class="header-actions">
        <el-button type="primary" @click="createTask" :disabled="!codeSource">
          <el-icon><Plus /></el-icon>
          {{ t('codeSource.createTask') }}
        </el-button>
        <el-button type="danger" @click="deleteCodeSource" :disabled="loading">
          <el-icon><Delete /></el-icon>
          {{ t('common.delete') }}
        </el-button>
      </div>
    </div>

    <div class="content-wrapper">
      <!-- 左侧：目录树 -->
      <el-card class="tree-card">
        <template #header>
          <div class="card-header">
            <span>{{ t('codeSource.projectStructure') }}</span>
            <el-button :icon="Refresh" @click="loadCodeSource" :loading="loading">{{ t('common.refresh') }}</el-button>
          </div>
        </template>

        <div v-if="loading" class="loading-container">
          <el-skeleton :rows="8" animated />
        </div>

        <div v-else-if="!fileTree || fileTree.length === 0" class="empty-tree">
          <el-empty :description="t('codeSource.noDirectoryStructure')" />
        </div>

        <div v-else class="tree-container">
          <el-tree
            :data="fileTree"
            :props="treeProps"
            default-expand-all
            :expand-on-click-node="false"
            node-key="path"
            class="file-tree"
            @node-click="handleNodeClick"
          >
            <template #default="{ node, data }">
              <div class="tree-node">
                <div class="node-icon">
                  <el-icon v-if="data.type === 'directory'"><Folder /></el-icon>
                  <el-icon v-else-if="isImage(data.ext)"><Picture /></el-icon>
                  <el-icon v-else><Document /></el-icon>
                </div>
                <span class="node-label">{{ node.label }}</span>
                <span class="node-size" v-if="data.size">{{ formatSizeUtil(data.size) }}</span>
              </div>
            </template>
          </el-tree>
        </div>
      </el-card>

      <!-- 右侧：文件预览 -->
      <el-card class="preview-card">
        <template #header>
          <div class="card-header">
            <div class="header-info">
              <span>{{ t('codeSource.filePreview') }} - {{ currentFile?.name || t('codeSource.selectFileToPreview') }}</span>
              <div class="header-tags" v-if="currentFile">
                <el-tag v-if="currentFile.language" type="info">{{ currentFile.language }}</el-tag>
                <el-tag type="info">{{ lineCount }} {{ t('codeSource.lines') || '行' }}</el-tag>
              </div>
            </div>
          </div>
        </template>

        <div v-if="previewLoading" class="loading-container">
          <el-skeleton :rows="10" animated />
        </div>

        <div v-else-if="!currentFile" class="empty-preview">
          <el-empty :description="t('codeSource.clickToPreview')" />
        </div>

        <div v-else-if="isImage(currentFile.ext)" class="image-preview">
          <img :src="imagePreviewUrl" :alt="currentFile.name" />
        </div>

        <div v-else class="code-preview" ref="previewContainerRef" @scroll="syncScroll">
          <div class="code-wrapper" v-if="currentFile.content">
            <!-- 行号 -->
            <div class="line-numbers" ref="lineNumbersRef">
              <div 
                v-for="n in lineCount" 
                :key="n" 
                class="line-number"
                :class="{ 'highlighted': highlightedLine === n }"
                @click="handleLineClick(n)"
              >{{ n }}</div>
            </div>
            <!-- 代码内容 - 使用 v-html 渲染高亮代码 -->
            <div class="code-content-wrapper" ref="codeContentRef">
              <pre class="code-block" v-html="highlightedCode"></pre>
            </div>
          </div>
        </div>
      </el-card>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, nextTick, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import { codeSourceApi } from '@/api'
import { formatSize as formatSizeUtil } from '@/utils/common'
import { Plus, Delete, Refresh, Folder, Document, Picture } from '@element-plus/icons-vue'
import hljs from 'highlight.js'
import 'highlight.js/styles/github.css'

// 导入 highlight.js 支持的语言
import go from 'highlight.js/lib/languages/go'
import java from 'highlight.js/lib/languages/java'
import python from 'highlight.js/lib/languages/python'
import javascript from 'highlight.js/lib/languages/javascript'
import typescript from 'highlight.js/lib/languages/typescript'
import csharp from 'highlight.js/lib/languages/csharp'
import cpp from 'highlight.js/lib/languages/cpp'
import c from 'highlight.js/lib/languages/c'
import ruby from 'highlight.js/lib/languages/ruby'
import php from 'highlight.js/lib/languages/php'
import swift from 'highlight.js/lib/languages/swift'
import kotlin from 'highlight.js/lib/languages/kotlin'
import scala from 'highlight.js/lib/languages/scala'
import rust from 'highlight.js/lib/languages/rust'
import sql from 'highlight.js/lib/languages/sql'
import bash from 'highlight.js/lib/languages/bash'
import shell from 'highlight.js/lib/languages/shell'
import xml from 'highlight.js/lib/languages/xml'
import json from 'highlight.js/lib/languages/json'
import yaml from 'highlight.js/lib/languages/yaml'
import css from 'highlight.js/lib/languages/css'
import scss from 'highlight.js/lib/languages/scss'
import less from 'highlight.js/lib/languages/less'
import markdown from 'highlight.js/lib/languages/markdown'
import dockerfile from 'highlight.js/lib/languages/dockerfile'
import powershell from 'highlight.js/lib/languages/powershell'
import objectivec from 'highlight.js/lib/languages/objectivec'
import plaintext from 'highlight.js/lib/languages/plaintext'
import ini from 'highlight.js/lib/languages/ini'
import haskell from 'highlight.js/lib/languages/haskell'
import erlang from 'highlight.js/lib/languages/erlang'
import elixir from 'highlight.js/lib/languages/elixir'
import perl from 'highlight.js/lib/languages/perl'
import lua from 'highlight.js/lib/languages/lua'
import r from 'highlight.js/lib/languages/r'
import dart from 'highlight.js/lib/languages/dart'
import groovy from 'highlight.js/lib/languages/groovy'
import clojure from 'highlight.js/lib/languages/clojure'
import fsharp from 'highlight.js/lib/languages/fsharp'
import ocaml from 'highlight.js/lib/languages/ocaml'
import makefile from 'highlight.js/lib/languages/makefile'
import dockerfileLang from 'highlight.js/lib/languages/dockerfile'

// 注册语言
hljs.registerLanguage('go', go)
hljs.registerLanguage('java', java)
hljs.registerLanguage('python', python)
hljs.registerLanguage('javascript', javascript)
hljs.registerLanguage('typescript', typescript)
hljs.registerLanguage('csharp', csharp)
hljs.registerLanguage('cpp', cpp)
hljs.registerLanguage('c', c)
hljs.registerLanguage('ruby', ruby)
hljs.registerLanguage('php', php)
hljs.registerLanguage('swift', swift)
hljs.registerLanguage('kotlin', kotlin)
hljs.registerLanguage('scala', scala)
hljs.registerLanguage('rust', rust)
hljs.registerLanguage('sql', sql)
hljs.registerLanguage('bash', bash)
hljs.registerLanguage('shell', shell)
hljs.registerLanguage('sh', shell)
hljs.registerLanguage('zsh', shell)
hljs.registerLanguage('xml', xml)
hljs.registerLanguage('html', xml)
hljs.registerLanguage('json', json)
hljs.registerLanguage('yaml', yaml)
hljs.registerLanguage('css', css)
hljs.registerLanguage('scss', scss)
hljs.registerLanguage('less', less)
hljs.registerLanguage('markdown', markdown)
hljs.registerLanguage('dockerfile', dockerfileLang)
hljs.registerLanguage('powershell', powershell)
hljs.registerLanguage('objectivec', objectivec)
hljs.registerLanguage('plaintext', plaintext)
hljs.registerLanguage('ini', ini)
hljs.registerLanguage('haskell', haskell)
hljs.registerLanguage('erlang', erlang)
hljs.registerLanguage('elixir', elixir)
hljs.registerLanguage('perl', perl)
hljs.registerLanguage('lua', lua)
hljs.registerLanguage('r', r)
hljs.registerLanguage('dart', dart)
hljs.registerLanguage('groovy', groovy)
hljs.registerLanguage('clojure', clojure)
hljs.registerLanguage('fsharp', fsharp)
hljs.registerLanguage('ocaml', ocaml)
hljs.registerLanguage('makefile', makefile)
hljs.registerLanguage('vue', xml)
hljs.registerLanguage('svelte', xml)
hljs.registerLanguage('jsx', javascript)
hljs.registerLanguage('tsx', typescript)

const route = useRoute()
const router = useRouter()
const { t } = useI18n()

const loading = ref(false)
const previewLoading = ref(false)
const codeSource = ref<any>(null)
const fileTree = ref<any[]>([])
const currentFile = ref<any>(null)
const imagePreviewUrl = ref('')
const highlightedLine = ref<number | null>(null)
const previewContainerRef = ref<HTMLElement | null>(null)
const lineNumbersRef = ref<HTMLElement | null>(null)
const codeContentRef = ref<HTMLElement | null>(null)
// 高亮后的代码内容
const highlightedCode = ref('')

const treeProps = {
  children: 'children',
  label: 'name'
}

// 计算行数
const lineCount = computed(() => {
  if (!currentFile.value?.content) return 0
  return currentFile.value.content.split('\n').length
})

const codeSourceId = () => route.params.id as string

const loadCodeSource = async () => {
  loading.value = true
  try {
    const res = await codeSourceApi.get(codeSourceId())
    const data = res.data.Data?.data || res.data.data
    codeSource.value = data
    fileTree.value = data.fileTree || []
  } catch (error) {
    console.error(t('codeSource.loadFailed') + ':', error)
    ElMessage.error(t('codeSource.loadFailed'))
  } finally {
    loading.value = false
  }
}

const handleNodeClick = async (data: any) => {
  // 只处理文件，不处理目录
  if (data.type !== 'file') return
  
  // 检查是否是可预览的文件类型
  if (!isPreviewable(data.ext)) {
    ElMessage.warning(t('codeSource.fileTypeNotSupported'))
    return
  }

  previewLoading.value = true
  currentFile.value = null
  highlightedLine.value = null
  
  try {
    const res = await codeSourceApi.getFile(codeSourceId(), data.path)
    const fileData = res.data.Data?.data || res.data.data
    currentFile.value = fileData
    
    // 如果是图片，获取预览URL
    if (isImage(data.ext)) {
      const ext = data.ext.toLowerCase()
      const mimeType = getMimeType(ext)
      imagePreviewUrl.value = `data:${mimeType};base64,${btoa(fileData.content)}`
    }
  } catch (error) {
    console.error(t('codeSource.loadFailed') + ':', error)
    ElMessage.error(t('codeSource.loadFailed'))
  } finally {
    previewLoading.value = false
  }
}

// 监听文件内容变化，应用语法高亮
watch(() => currentFile.value, async (newFile) => {
  if (newFile && newFile.content) {
    await nextTick()
    applyHighlighting()
  }
})

// 应用语法高亮
const applyHighlighting = () => {
  if (!currentFile.value) return
  
  const content = currentFile.value.content
  if (!content) {
    highlightedCode.value = ''
    return
  }
  
  // 优先使用后端返回的语言，如果没有则自动检测
  let language = currentFile.value.language
  
  // 如果没有语言信息，根据文件扩展名推断
  if (!language && currentFile.value.name) {
    const ext = currentFile.value.name.substring(currentFile.value.name.lastIndexOf('.'))
    language = getLanguageFromExt(ext)
  }
  
  // 如果还是没有，使用 plaintext
  if (!language) {
    language = 'plaintext'
  }
  
  try {
    let result: string
    if (hljs.getLanguage(language)) {
      const hljsResult = hljs.highlight(content, { language })
      result = hljsResult.value
    } else {
      // 语言不支持时自动检测
      const hljsResult = hljs.highlightAuto(content)
      result = hljsResult.value
    }
    
    highlightedCode.value = result
  } catch (e) {
    console.warn('语法高亮失败:', e)
    // 回退：使用纯文本模式
    highlightedCode.value = escapeHtml(content)
  }
}

// HTML 转义（仅在回退模式使用）
const escapeHtml = (text: string): string => {
  const div = document.createElement('div')
  div.textContent = text
  return div.innerHTML
}

// 同步滚动 - 行号跟随代码内容滚动
const syncScroll = () => {
  if (lineNumbersRef.value && previewContainerRef.value) {
    lineNumbersRef.value.scrollTop = previewContainerRef.value.scrollTop
  }
}

// 点击行号高亮
const handleLineClick = (lineNum: number) => {
  highlightedLine.value = lineNum
  // 滚动到该行
  setTimeout(() => {
    const codeElement = document.querySelector('.code-content code')
    if (!codeElement || !previewContainerRef.value) return
    
    // 获取所有文本节点并计算位置
    const walker = document.createTreeWalker(codeElement, NodeFilter.SHOW_TEXT)
    let charCount = 0
    let lineStart = 0
    let currentLine = 1
    
    while (walker.nextNode()) {
      const node = walker.currentNode as Text
      const text = node.textContent || ''
      
      for (let i = 0; i < text.length; i++) {
        if (text[i] === '\n') {
          currentLine++
          lineStart = charCount + i + 1
          if (currentLine === lineNum) {
            // 滚动到该行
            const range = document.createRange()
            range.setStart(node, i + 1)
            range.collapse(true)
            const rect = range.getBoundingClientRect()
            if (rect) {
              previewContainerRef.value.scrollTop = previewContainerRef.value.scrollTop + rect.top - 100
            }
            return
          }
        }
      }
      charCount += text.length
    }
  }, 50)
}

// 获取文件扩展名对应的语言
const getLanguageFromExt = (ext: string): string => {
  const extMap: Record<string, string> = {
    '.go': 'go',
    '.java': 'java',
    '.py': 'python',
    '.js': 'javascript',
    '.jsx': 'javascript',
    '.ts': 'typescript',
    '.tsx': 'typescript',
    '.vue': 'vue',
    '.html': 'html',
    '.htm': 'html',
    '.css': 'css',
    '.scss': 'scss',
    '.less': 'less',
    '.json': 'json',
    '.xml': 'xml',
    '.yaml': 'yaml',
    '.yml': 'yaml',
    '.md': 'markdown',
    '.sql': 'sql',
    '.sh': 'bash',
    '.bash': 'bash',
    '.zsh': 'bash',
    '.c': 'c',
    '.h': 'c',
    '.cpp': 'cpp',
    '.hpp': 'cpp',
    '.cs': 'csharp',
    '.rb': 'ruby',
    '.php': 'php',
    '.swift': 'swift',
    '.kt': 'kotlin',
    '.scala': 'scala',
    '.r': 'r',
    '.lua': 'lua',
    '.pl': 'perl',
    '.hs': 'haskell',
    '.erl': 'erlang',
    '.ex': 'elixir',
    '.txt': 'plaintext',
    '.log': 'plaintext',
    '.ini': 'ini',
    '.conf': 'ini',
    '.cfg': 'ini',
    '.properties': 'properties',
    '.env': 'bash',
    '.gitignore': 'plaintext',
    '.dockerfile': 'dockerfile',
    '.toml': 'ini',
    '.makefile': 'makefile',
    '.mk': 'makefile',
    '.svelte': 'xml',
    '.dart': 'dart',
    '.groovy': 'groovy',
    '.clj': 'clojure',
    '.exs': 'elixir',
    '.ml': 'ocaml',
    '.fs': 'fsharp',
    '.fsx': 'fsharp',
  }
  return extMap[ext?.toLowerCase()] || 'plaintext'
}

const isPreviewable = (ext: string) => {
  const previewableExts = [
    // 编程语言
    '.go', '.java', '.py', '.js', '.jsx', '.ts', '.tsx', '.vue',
    '.html', '.htm', '.css', '.scss', '.less', '.json', '.xml',
    '.yaml', '.yml', '.md', '.sql', '.sh', '.bash', '.zsh',
    '.c', '.h', '.cpp', '.hpp', '.cs', '.rb', '.php', '.swift',
    '.kt', '.scala', '.r', '.lua', '.pl', '.hs', '.erl', '.ex', '.exs',
    '.dart', '.groovy', '.clj', '.ml', '.fs', '.fsx', '.nim', '.zig',
    // 配置文件
    '.txt', '.log', '.ini', '.conf', '.cfg', '.properties',
    '.env', '.gitignore', '.gitattributes', '.dockerignore',
    '.dockerfile', 'dockerfile', '.toml', '.makefile', '.mk',
    '.editorconfig', '.prettierrc', '.eslintrc', '.babelrc',
    '.npmrc', '.nvmrc', '.node-version',
    // 前端框架/库
    '.svelte',
    // 图片
    '.png', '.jpg', '.jpeg', '.gif', '.bmp', '.webp', '.svg', '.ico'
  ]
  return previewableExts.includes(ext?.toLowerCase()) || ext?.toLowerCase() === 'dockerfile'
}

const isImage = (ext: string) => {
  const imageExts = ['.png', '.jpg', '.jpeg', '.gif', '.bmp', '.webp', '.svg', '.ico']
  return imageExts.includes(ext?.toLowerCase())
}

const getMimeType = (ext: string) => {
  const mimeTypes: Record<string, string> = {
    '.png': 'image/png',
    '.jpg': 'image/jpeg',
    '.jpeg': 'image/jpeg',
    '.gif': 'image/gif',
    '.bmp': 'image/bmp',
    '.webp': 'image/webp',
    '.svg': 'image/svg+xml',
    '.ico': 'image/x-icon'
  }
  return mimeTypes[ext] || 'application/octet-stream'
}

const createTask = () => {
  router.push(`/tasks/create?source=${codeSourceId()}`)
}

const deleteCodeSource = async () => {
  try {
    await ElMessageBox.confirm(t('codeSource.confirmDelete'), t('common.warning'), { type: 'warning' })
    await codeSourceApi.delete(codeSourceId())
    ElMessage.success(t('codeSource.deleteSuccess'))
    router.push('/code-sources')
  } catch (error) {
    // cancel
  }
}

onMounted(() => {
  loadCodeSource().then(() => {
    const file = route.query.file as string
    const line = route.query.line as string
    if (file) {
      setTimeout(() => {
        navigateToFile(file, line ? parseInt(line) : 1)
      }, 500)
    }
  })
})

// 导航到指定文件并高亮行
const navigateToFile = async (filePath: string, line: number) => {
  const findNode = (nodes: any[], path: string): any => {
    for (const node of nodes) {
      if (node.path === path) return node
      if (node.children) {
        const found = findNode(node.children, path)
        if (found) return found
      }
    }
    return null
  }
  
  const fileNode = findNode(fileTree.value, filePath)
  if (fileNode) {
    await handleNodeClick(fileNode)
    setTimeout(() => {
      if (line > 1) {
        highlightedLine.value = line
        handleLineClick(line)
      }
    }, 300)
  } else {
    console.warn('文件未找到:', filePath)
  }
}

// 监听路由参数变化
watch(() => route.query, (newQuery) => {
  const file = newQuery.file as string
  const line = newQuery.line as string
  if (file && codeSource.value) {
    navigateToFile(file, line ? parseInt(line) : 1)
  }
})
</script>

<style scoped>
.code-source-detail {
  padding: 24px;
  height: calc(100vh - 120px);
  display: flex;
  flex-direction: column;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}

.header-actions {
  display: flex;
  gap: 12px;
}

.content-wrapper {
  display: flex;
  gap: 16px;
  flex: 1;
  min-height: 0;
}

.tree-card {
  width: 350px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
}

.tree-card :deep(.el-card__body) {
  flex: 1;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.preview-card {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.preview-card :deep(.el-card__body) {
  flex: 1;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.header-info {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.header-tags {
  display: flex;
  gap: 8px;
}

.loading-container,
.empty-tree,
.empty-preview {
  padding: 40px 0;
  text-align: center;
}

.tree-container {
  flex: 1;
  min-height: 0;
  overflow-x: auto;
  overflow-y: auto;
  position: relative;
  scrollbar-gutter: stable;
}

.file-tree {
  background: transparent;
  width: max-content;
  min-width: 100%;
  padding-bottom: 8px;
}

.file-tree :deep(.el-tree-node) {
  width: max-content;
  min-width: 100%;
}

.tree-node {
  display: flex;
  align-items: center;
  gap: 8px;
  width: max-content;
  min-width: 100%;
  padding: 4px 0;
}

.node-icon {
  display: flex;
  align-items: center;
  color: #6b7280;
  flex-shrink: 0;
}

.node-label {
  font-size: 13px;
  white-space: nowrap;
}

.node-size {
  font-size: 12px;
  color: #9ca3af;
  flex-shrink: 0;
  margin-left: 8px;
}

/* 滚动条样式 */
.tree-container::-webkit-scrollbar {
  width: 12px;
  height: 12px;
}

.tree-container::-webkit-scrollbar-track {
  background: #f1f1f1;
  border-radius: 6px;
}

.tree-container::-webkit-scrollbar-thumb {
  background: #c1c1c1;
  border-radius: 6px;
  border: 2px solid #f1f1f1;
}

.tree-container::-webkit-scrollbar-thumb:hover {
  background: #a1a1a1;
}

/* 代码预览区域 - 唯一滚动容器 */
.code-preview {
  flex: 1;
  min-height: 0;
  background: #f6f8fa;
  border-radius: 6px;
  overflow: auto;
  scrollbar-width: thin;
  scrollbar-color: #aaa #f0f0f0;
}

/* 代码包装器 - flex 横向排列 */
.code-wrapper {
  display: flex;
  align-items: flex-start;
  min-width: max-content;
}

/* 行号区域 */
.line-numbers {
  flex-shrink: 0;
  width: 50px;
  background: #eff1f3;
  border-right: 1px solid #ddd;
  padding: 12px 0;
  user-select: none;
}

.line-number {
  height: 21px;
  line-height: 21px;
  padding-right: 12px;
  text-align: right;
  font-family: 'Fira Code', 'Consolas', 'Monaco', monospace;
  font-size: 13px;
  color: #6b7280;
  cursor: pointer;
  transition: background-color 0.15s;
}

.line-number:hover {
  background-color: #e1e4e8;
}

.line-number.highlighted {
  background-color: #fff3cd;
  color: #856404;
  font-weight: 600;
}

/* 代码内容区域 */
.code-content-wrapper {
  flex: 1;
  min-width: 0;
}

/* 代码块 - 继承 highlight.js 的 github.css 样式 */
.code-block {
  margin: 0;
  padding: 12px 16px !important;
  background: #f6f8fa !important;
  font-family: 'Fira Code', 'Consolas', 'Monaco', monospace !important;
  font-size: 13px !important;
  line-height: 1.5 !important;
  white-space: pre !important;
}

/* 确保 highlight.js 的样式正确应用 */
.code-block .hljs {
  background: transparent !important;
}

/* 语法高亮颜色 - 使用与 github.css 一致的颜色 */
.code-block .hljs { color: #24292e; }
.code-block .hljs-comment, .code-block .hljs-quote { color: #6a737d; font-style: italic; }
.code-block .hljs-keyword, .code-block .hljs-selector-tag { color: #d73a49; }
.code-block .hljs-string, .code-block .hljs-addition { color: #032f62; }
.code-block .hljs-number, .code-block .hljs-literal { color: #005cc5; }
.code-block .hljs-attribute, .code-block .hljs-template-tag { color: #6f42c1; }
.code-block .hljs-type, .code-block .hljs-selector-class { color: #6f42c1; }
.code-block .hljs-built_in, .code-block .hljs-builtin-name { color: #e36209; }
.code-block .hljs-title, .hljs-section { color: #6f42c1; font-weight: bold; }
.code-block .hljs-class .hljs-title { color: #22863a; }
.code-block .hljs-variable, .hljs-template-variable { color: #e36209; }
.code-block .hljs-tag { color: #22863a; }
.code-block .hljs-name { color: #22863a; }
.code-block .hljs-attr { color: #005cc5; }
.code-block .hljs-symbol, .hljs-bullet { color: #005cc5; }
.code-block .hljs-meta { color: #e36209; }
.code-block .hljs-selector-id, .hljs-selector-class { color: #005cc5; }
.code-block .hljs-subst { color: #24292e; font-weight: bold; }
.code-block .hljs-doctag { color: #6a737d; }
.code-block .hljs-params { color: #24292e; }
.code-block .hljs-operator { color: #24292e; }
.code-block .hljs-regexp { color: #032f62; }
.code-block .hljs-deletion { color: #b31d28; background: #ffeef0; }
.code-block .hljs-emphasis { font-style: italic; }
.code-block .hljs-strong { font-weight: bold; }

/* 滚动条样式 */
.code-preview::-webkit-scrollbar {
  width: 14px;
  height: 14px;
}

.code-preview::-webkit-scrollbar-track {
  background: #f0f0f0;
}

.code-preview::-webkit-scrollbar-thumb {
  background: #aaa;
  border-radius: 7px;
}

.code-preview::-webkit-scrollbar-thumb:hover {
  background: #888;
}

.code-preview::-webkit-scrollbar-corner {
  background: #f0f0f0;
}

.image-preview {
  display: flex;
  justify-content: center;
  align-items: center;
  padding: 20px;
  flex: 1;
}

.image-preview img {
  max-width: 100%;
  max-height: 100%;
  object-fit: contain;
}

@media (max-width: 1024px) {
  .content-wrapper {
    flex-direction: column;
  }
  
  .tree-card {
    width: 100%;
    max-height: 300px;
  }
}
</style>

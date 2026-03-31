<template>
  <div class="call-graph-container">
    <!-- 控制栏 -->
    <div class="graph-controls">
      <div class="graph-title">
        <el-icon><Connection /></el-icon>
        漏洞利用链全景图
      </div>
      
      <el-divider direction="vertical" />
      
      <el-button size="small" @click="fitToScreen" :icon="FullScreen">适应屏幕</el-button>
      
      <!-- 布局切换 -->
      <el-dropdown @command="handleLayoutChange" trigger="click">
        <el-button size="small">
          <el-icon><Grid /></el-icon>
          布局
          <el-icon class="el-icon--right"><ArrowDown /></el-icon>
        </el-button>
        <template #dropdown>
          <el-dropdown-menu>
            <el-dropdown-item command="circle">
              <el-icon><Sort /></el-icon>
              环形布局
            </el-dropdown-item>
            <el-dropdown-item command="grid">
              <el-icon><Sort /></el-icon>
              网格布局
            </el-dropdown-item>
            <el-dropdown-item command="random">
              <el-icon><Sort /></el-icon>
              随机分布
            </el-dropdown-item>
          </el-dropdown-menu>
        </template>
      </el-dropdown>
      
      <el-button size="small" @click="reLayout" :icon="Refresh">重新布局</el-button>
      
      <el-divider direction="vertical" />
      
      <!-- 漏洞节点统计信息 -->
      <span class="graph-stats">
        <el-tag size="small" type="danger">漏洞: {{ nodeCount }}</el-tag>
        <el-tag size="small" type="info">文件: {{ fileCount }}</el-tag>
      </span>
      
      <el-divider direction="vertical" />
      
      <!-- 严重程度筛选 -->
      <el-select v-model="severityFilter" multiple collapse-tags size="small" placeholder="筛选等级" style="width: 150px">
        <el-option label="严重" value="Critical" />
        <el-option label="高危" value="High" />
        <el-option label="中危" value="Medium" />
        <el-option label="低危" value="Low" />
      </el-select>
      
      <el-divider direction="vertical" />
      
      <el-button size="small" @click="toggleFullscreen" :icon="isFullscreen ? Close : FullScreen">
        {{ isFullscreen ? '退出全屏' : '全屏' }}
      </el-button>
    </div>
    
    <!-- 全屏视图 -->
    <div v-if="isFullscreen" class="fullscreen-view">
      <div ref="fullscreenContainer" class="fullscreen-container"></div>
    </div>
    
    <!-- 图形容器 -->
    <div v-show="!isFullscreen" ref="graphContainer" class="graph-view"></div>
    
    <!-- 图例 -->
    <div class="graph-legend">
      <span class="legend-item critical">
        <span class="legend-icon"></span>
        <span>严重</span>
      </span>
      <span class="legend-item high">
        <span class="legend-icon"></span>
        <span>高危</span>
      </span>
      <span class="legend-item medium">
        <span class="legend-icon"></span>
        <span>中危</span>
      </span>
      <span class="legend-item low">
        <span class="legend-icon"></span>
        <span>低危</span>
      </span>
    </div>
    
    <!-- 节点信息侧边栏 -->
    <el-drawer
      v-model="drawerVisible"
      :title="selectedNodeInfo.displayLabel || selectedNodeInfo.type || '漏洞详情'"
      size="550px"
      direction="rtl"
    >
      <div class="node-info" v-if="selectedNodeInfo.id">
        <!-- 漏洞基本信息 -->
        <div class="info-section">
          <h4>
            <el-icon><Warning /></el-icon>
            漏洞详情
          </h4>
          <el-descriptions :column="1" border size="small">
            <el-descriptions-item label="漏洞类型">
              <el-tag size="small" type="danger">{{ translateVulnType(selectedNodeInfo.type) }}</el-tag>
            </el-descriptions-item>
            <el-descriptions-item label="严重程度">
              <el-tag 
                size="small" 
                :type="getSeverityTagType(selectedNodeInfo.severity)"
              >
                {{ translateSeverity(selectedNodeInfo.severity) }}
              </el-tag>
            </el-descriptions-item>
            <el-descriptions-item label="文件路径">
              <span class="file-path">{{ selectedNodeInfo.file || '-' }}</span>
            </el-descriptions-item>
            <el-descriptions-item label="行号">
              {{ selectedNodeInfo.line || selectedNodeInfo.line === 0 ? selectedNodeInfo.line : '-' }}
            </el-descriptions-item>
            <el-descriptions-item label="类名" v-if="selectedNodeInfo.class">
              {{ selectedNodeInfo.class || '-' }}
            </el-descriptions-item>
            <el-descriptions-item label="函数名" v-if="selectedNodeInfo.function">
              {{ selectedNodeInfo.function || '-' }}
            </el-descriptions-item>
            <el-descriptions-item label="CWE" v-if="selectedNodeInfo.cwe">
              {{ selectedNodeInfo.cwe }}
            </el-descriptions-item>
            <el-descriptions-item label="置信度" v-if="selectedNodeInfo.confidence">
              {{ selectedNodeInfo.confidence }}
            </el-descriptions-item>
          </el-descriptions>
        </div>

        <!-- 漏洞描述 -->
        <div class="info-section" v-if="selectedNodeInfo.description">
          <h4>
            <el-icon><Document /></el-icon>
            漏洞描述
          </h4>
          <div class="vuln-description">
            {{ selectedNodeInfo.description }}
          </div>
        </div>
        
        <!-- 漏洞分析 -->
        <div class="info-section" v-if="selectedNodeInfo.analysis">
          <h4>
            <el-icon><Search /></el-icon>
            漏洞分析
          </h4>
          <div class="vuln-analysis">
            {{ selectedNodeInfo.analysis }}
          </div>
        </div>
        
        <!-- 修复建议 -->
        <div class="info-section" v-if="selectedNodeInfo.fixSuggestion">
          <h4>
            <el-icon><CircleCheck /></el-icon>
            修复建议
          </h4>
          <div class="vuln-fix">
            {{ selectedNodeInfo.fixSuggestion }}
          </div>
        </div>
        
        <!-- 漏洞利用链：展示调用关系流程图 -->
        <div class="info-section">
          <h4>
            <el-icon><Connection /></el-icon>
            漏洞利用链
            <el-tag size="small" type="danger">{{ exploitChain.length }}</el-tag>
          </h4>
          
          <!-- 调用链可视化 - 垂直流程图样式 -->
          <div class="chain-visualization" v-if="exploitChain.length > 0">
            <div class="chain-flow-vertical">
              <template v-for="(item, index) in exploitChain" :key="index">
                <!-- Source 节点 -->
                <div v-if="item.nodeType === 'source'" class="chain-node-source" @click="navigateToChainNode(item)">
                  <div class="chain-node-icon">
                    <el-icon><Upload /></el-icon>
                  </div>
                  <div class="chain-node-content">
                    <span class="chain-label">[Source] {{ item.label || '用户输入' }}</span>
                    <span class="chain-location" v-if="item.file">{{ item.file }}{{ item.line ? ':' + item.line : '' }}</span>
                  </div>
                </div>
                
                <!-- 当前漏洞节点 -->
                <div v-else-if="item.nodeType === 'current'" class="chain-node-current" @click="navigateToChainNode(item)">
                  <div class="chain-node-icon">
                    <el-icon><Warning /></el-icon>
                  </div>
                  <div class="chain-node-content">
                    <span class="chain-label">{{ item.label || selectedNodeInfo.displayLabel || selectedNodeInfo.type }}</span>
                    <span class="chain-severity" v-if="item.severity">
                      <el-tag size="small" :type="getSeverityTagType(item.severity)">{{ translateSeverity(item.severity) }}</el-tag>
                    </span>
                    <span class="chain-location" v-if="item.file">{{ item.file }}{{ item.line ? ':' + item.line : '' }}</span>
                  </div>
                </div>
                
                <!-- Sink 节点 -->
                <div v-else-if="item.nodeType === 'sink'" class="chain-node-sink" @click="navigateToChainNode(item)">
                  <div class="chain-node-icon">
                    <el-icon><Delete /></el-icon>
                  </div>
                  <div class="chain-node-content">
                    <span class="chain-label">[Sink] {{ item.label || '危险操作' }}</span>
                    <span class="chain-location" v-if="item.file">{{ item.file }}{{ item.line ? ':' + item.line : '' }}</span>
                  </div>
                </div>
                
                <!-- 普通调用节点 -->
                <div v-else class="chain-node-normal" :class="item.type" @click="navigateToChainNode(item)">
                  <div class="chain-node-icon">
                    <el-icon v-if="item.type === 'caller'"><Top /></el-icon>
                    <el-icon v-else-if="item.type === 'callee'"><Bottom /></el-icon>
                    <el-icon v-else><Right /></el-icon>
                  </div>
                  <div class="chain-node-content">
                    <span class="chain-label">{{ item.displayLabel || item.label || (item.class ? item.class + '.' : '') + (item.function || '未知') }}</span>
                    <span class="chain-location" v-if="item.file">{{ item.file }}{{ item.line ? ':' + item.line : '' }}</span>
                  </div>
                </div>
                
                <!-- 垂直连接箭头 -->
                <div v-if="index < exploitChain.length - 1" class="chain-connector">
                  <span class="chain-arrow">↓</span>
                </div>
              </template>
            </div>
          </div>
          
          <!-- 简化版漏洞利用链 -->
          <div class="chain-simple" v-else-if="simplifiedChain.length > 0">
            <div class="chain-step" v-for="(step, index) in simplifiedChain" :key="index">
              <span class="step-index">{{ index + 1 }}</span>
              <span class="step-content">
                <span class="step-label">{{ step.label }}</span>
                <span class="step-location" v-if="step.file">{{ step.file }}{{ step.line ? ':' + step.line : '' }}</span>
              </span>
            </div>
          </div>
          
          <el-empty v-else description="无漏洞利用链信息" :image-size="60" />
        </div>
        
        <div class="info-actions">
          <el-button @click="focusOnNode">
            <el-icon><Aim /></el-icon>
            聚焦该节点
          </el-button>
        </div>
      </div>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch, nextTick, computed } from 'vue'
import cytoscape from 'cytoscape'
import { ElMessage } from 'element-plus'
import { 
  Search, RefreshLeft, FullScreen, ZoomIn, Close, Grid,
  Aim, Phone, PhoneFilled, Document, Download, ArrowDown, Sort, Connection, Refresh,
  Warning, Upload, Delete, Top, Bottom, Right, CircleCheck
} from '@element-plus/icons-vue'

interface Props {
  taskId: string | number
  initialFunc?: string
  depth?: number
}

const props = withDefaults(defineProps<Props>(), {
  initialFunc: '',
  depth: 3
})

const emit = defineEmits<{
  (e: 'nodeClick', node: any): void
}>()

// DOM 引用
const graphContainer = ref<HTMLElement | null>(null)
const fullscreenContainer = ref<HTMLElement | null>(null)

// 状态
const isFullscreen = ref(false)
const nodeCount = ref(0)
const fileCount = ref(0)
const severityFilter = ref<string[]>([])

// 侧边栏相关
const drawerVisible = ref(false)
const selectedNodeInfo = ref<any>({})

// 漏洞利用链数据
const exploitChain = ref<any[]>([])
const simplifiedChain = ref<any[]>([])

// 原始漏洞数据
const allNodes = ref<any[]>([])

// Cytoscape 实例
let cy: cytoscape.Core | null = null
let cyFullscreen: cytoscape.Core | null = null

// 当前布局类型
const currentLayout = ref('circle')

// 漏洞类型翻译映射
const vulnTypeTranslations: Record<string, string> = {
  'SQL Injection': 'SQL注入',
  'SQL injection': 'SQL注入',
  'sql injection': 'SQL注入',
  'sql injection vulnerability': 'SQL注入',
  'Path Traversal': '路径遍历',
  'Path traversal': '路径遍历',
  'path traversal': '路径遍历',
  'Local File Inclusion': '本地文件包含',
  'Local file inclusion': '本地文件包含',
  'Remote Code Execution': '远程代码执行',
  'Remote code execution': '远程代码执行',
  'Cross-Site Scripting': '跨站脚本攻击',
  'Cross site scripting': '跨站脚本攻击',
  'XSS': '跨站脚本攻击',
  'Command Injection': '命令注入',
  'Command injection': '命令注入',
  'OS Command Injection': '操作系统命令注入',
  'OS command injection': '操作系统命令注入',
  'XML External Entity': 'XML外部实体',
  'XML external entity': 'XML外部实体',
  'XXE': 'XML外部实体',
  'Deserialization': '反序列化漏洞',
  'Deserialization vulnerability': '反序列化漏洞',
  'Insecure Deserialization': '不安全的反序列化',
  'Weak Cryptography': '弱加密',
  'Weak cryptography': '弱加密',
  'Hardcoded Password': '硬编码密码',
  'Hardcoded password': '硬编码密码',
  'Hard-coded credentials': '硬编码凭证',
  'Sensitive Data Exposure': '敏感数据泄露',
  'Sensitive data exposure': '敏感数据泄露',
  'Broken Authentication': '身份验证失效',
  'Broken authentication': '身份验证失效',
  'Security Misconfiguration': '安全配置错误',
  'Security misconfiguration': '安全配置错误',
  'Missing Authorization': '授权缺失',
  'Missing authorization': '授权缺失',
  'Insufficient Input Validation': '输入验证不足',
  'Insufficient input validation': '输入验证不足',
  'Improper Input Validation': '不正确的输入验证',
  'Improper input validation': '不正确的输入验证',
  'CSRF': '跨站请求伪造',
  'Cross-Site Request Forgery': '跨站请求伪造',
  'Open Redirect': '开放重定向',
  'Open redirect': '开放重定向',
  'URL Redirect': 'URL重定向',
  'URL redirect': 'URL重定向',
  'Server-Side Request Forgery': '服务器端请求伪造',
  'Server side request forgery': '服务器端请求伪造',
  'SSRF': '服务器端请求伪造',
  'Race Condition': '竞态条件',
  'Race condition': '竞态条件',
  'Time-of-check Time-of-use': '检查时间使用时间漏洞',
  'TOCTOU': '检查时间使用时间漏洞',
  'Buffer Overflow': '缓冲区溢出',
  'Buffer overflow': '缓冲区溢出',
  'Heap Overflow': '堆溢出',
  'Stack Overflow': '栈溢出',
  'Integer Overflow': '整数溢出',
  'Integer overflow': '整数溢出',
  'Format String': '格式化字符串',
  'Format string': '格式化字符串',
  'LDAP Injection': 'LDAP注入',
  'LDAP injection': 'LDAP注入',
  'XPATH Injection': 'XPATH注入',
  'XPath injection': 'XPATH注入',
  'Template Injection': '模板注入',
  'Template injection': '模板注入',
  'SSTI': '服务端模板注入',
  'Expression Language Injection': '表达式语言注入',
  'Expression language injection': '表达式语言注入',
  'JWT Injection': 'JWT注入',
  'JWT injection': 'JWT注入',
  'Cookie Poisoning': 'Cookie投毒',
  'Cookie poisoning': 'Cookie投毒',
  'HTTP Response Splitting': 'HTTP响应拆分',
  'HTTP response splitting': 'HTTP响应拆分',
  'Session Fixation': '会话固定',
  'Session fixation': '会话固定',
  'Insecure Cookie': '不安全Cookie',
  'Insecure cookie': '不安全Cookie',
  'DOM-based XSS': 'DOM型跨站脚本',
  'DOM based XSS': 'DOM型跨站脚本',
  'Reflected XSS': '反射型跨站脚本',
  'Stored XSS': '存储型跨站脚本',
  'Type Confusion': '类型混淆',
  'Type confusion': '类型混淆',
  'Use After Free': '释放后使用',
  'Use after free': '释放后使用',
  'Double Free': '双重释放',
  'Double free': '双重释放',
  'Memory Leak': '内存泄漏',
  'Memory leak': '内存泄漏',
  'Null Pointer Dereference': '空指针引用',
  'Null pointer dereference': '空指针引用',
  'Arbitrary File Read': '任意文件读取',
  'Arbitrary file read': '任意文件读取',
  'Arbitrary File Write': '任意文件写入',
  'Arbitrary file write': '任意文件写入',
  'Arbitrary File Deletion': '任意文件删除',
  'Arbitrary file deletion': '任意文件删除',
  'Path Disclosure': '路径泄露',
  'Path disclosure': '路径泄露',
  'Information Disclosure': '信息泄露',
  'Information disclosure': '信息泄露',
  'Debug Information': '调试信息泄露',
  'Debug information': '调试信息泄露',
  'Credentials Management': '凭证管理问题',
  'Credentials management': '凭证管理问题',
  'Exposure of Sensitive Information': '敏感信息暴露',
  'Exposure of sensitive information': '敏感信息暴露',
  'Security Vulnerability': '安全漏洞',
  'Security vulnerability': '安全漏洞',
  'Code Quality': '代码质量问题',
  'Code quality': '代码质量问题',
  'Code Review': '代码审查问题',
  'Code review': '代码审查问题',
  'Best Practices Violation': '违反最佳实践',
  'Best practices violation': '违反最佳实践',
  'CWE': '通用缺陷枚举',
  'Security Best Practices': '安全最佳实践',
}

// 严重程度翻译映射
const severityTranslations: Record<string, string> = {
  'Critical': '严重',
  'High': '高危',
  'Medium': '中危',
  'Low': '低危',
  'Info': '信息',
  'critical': '严重',
  'high': '高危',
  'medium': '中危',
  'low': '低危',
  'info': '信息',
}

// 翻译漏洞类型
const translateVulnType = (type: string): string => {
  if (!type) return '未知漏洞类型'
  return vulnTypeTranslations[type] || type
}

// 翻译严重程度
const translateSeverity = (severity: string): string => {
  if (!severity) return '未知'
  return severityTranslations[severity] || severity
}

// 初始化图形
const initGraph = async () => {
  try {
    // 获取漏洞利用链图数据
    const response = await fetch(`/api/tasks/${props.taskId}/vuln-graph`, {
      headers: {
        'Authorization': `Bearer ${localStorage.getItem('auth_token')}`
      }
    })
    
    if (!response.ok) {
      throw new Error(`HTTP ${response.status}: ${response.statusText}`)
    }
    
    const result = await response.json()
    const data = result.Data || result.data
    
    if (!data || !data.nodes || data.nodes.length === 0) {
      ElMessage.warning('暂无漏洞数据')
      return
    }
    
    // 保存原始节点数据
    allNodes.value = data.nodes
    
    // 统计
    nodeCount.value = data.nodes.length
    fileCount.value = data.stats?.totalFiles || data.stats?.vulnFiles || new Set(data.nodes.map((n: any) => n.file)).size
    
    // 应用筛选
    const filteredNodes = applySeverityFilter(data.nodes)
    
    // 转换元素
    const elements = convertToElements(filteredNodes)
    
    // 销毁旧的实例
    if (cy) {
      cy.destroy()
    }
    
    if (graphContainer.value) {
      cy = cytoscape({
        container: graphContainer.value,
        elements: elements,
        style: getVulnGraphStyle(),
        layout: getLayoutConfig(currentLayout.value),
        minZoom: 0.2,
        maxZoom: 3,
        wheelSensitivity: 0.3,
      })
      
      // 启用拖拽
      cy.nodes().grabify()
      
      bindGraphEvents(cy)
      
      // 保存初始布局位置
      cy.on('layoutstop', () => {
        saveAllPositions()
      })
    }
    
  } catch (error: any) {
    console.error('加载漏洞图失败:', error)
    ElMessage.error(error.message || '加载漏洞图失败')
  }
}

// 应用严重程度筛选
const applySeverityFilter = (nodes: any[]) => {
  if (severityFilter.value.length === 0) {
    return nodes
  }
  return nodes.filter(node => severityFilter.value.includes(node.severity))
}

// 监听筛选变化
watch(severityFilter, async () => {
  const filteredNodes = applySeverityFilter(allNodes.value)
  updateGraph(filteredNodes)
  nodeCount.value = filteredNodes.length
})

// 更新图形
const updateGraph = (nodes: any[]) => {
  if (!cy) return
  
  // 清除现有元素
  cy.elements().remove()
  
  // 添加新元素
  const elements = convertToElements(nodes)
  cy.add(elements)
  
  // 重新布局
  cy.layout(getLayoutConfig(currentLayout.value)).run()
}

const convertToElements = (nodes: any[]) => {
  const elements: any[] = []
  
  // 获取容器实际尺寸
  const containerWidth = graphContainer.value?.clientWidth || 800
  const containerHeight = graphContainer.value?.clientHeight || 600
  const centerX = containerWidth / 2
  const centerY = containerHeight / 2
  
  // 计算最优布局半径 - 根据节点数量动态调整
  const nodeCount = nodes.length
  const baseRadius = Math.min(containerWidth, containerHeight) * 0.35
  const maxRadius = Math.min(containerWidth, containerHeight) * 0.45
  
  // 按严重程度分层布局
  const severityOrder = ['Critical', 'High', 'Medium', 'Low']
  const severityGroups: Record<string, any[]> = {}
  let totalInLayer = 0
  
  severityOrder.forEach(sev => {
    severityGroups[sev] = nodes.filter(n => n.severity === sev)
    if (severityGroups[sev].length > 0) {
      totalInLayer++
    }
  })
  
  // 使用同心圆布局 - 按严重程度分圈
  let currentLayer = 0
  let currentAngle = 0
  
  severityOrder.forEach((sev, layerIndex) => {
    const layerNodes = severityGroups[sev]
    if (layerNodes.length === 0) return
    
    // 每层使用不同的半径
    const layerRadius = baseRadius + (currentLayer * (maxRadius - baseRadius) / Math.max(totalInLayer - 1, 1))
    const layerAngleStep = (2 * Math.PI) / Math.max(layerNodes.length, 1)
    
    layerNodes.forEach((node, index) => {
      // 均匀分布在当前层
      const angle = currentAngle + index * layerAngleStep
      const x = centerX + layerRadius * Math.cos(angle)
      const y = centerY + layerRadius * Math.sin(angle)
      
      // 节点标签：优先使用函数名，否则使用类名，最后使用翻译后的漏洞类型
      let displayLabel = ''
      let fullLabel = ''
      
      if (node.function && node.function.trim() && !isCommonNonFunctionName(node.function)) {
        // 有函数名时，显示函数名
        displayLabel = node.function
        fullLabel = node.class ? `${node.class}.${node.function}` : node.function
      } else if (node.class && node.class.trim()) {
        // 没有函数名但有类名时，显示类名
        displayLabel = node.class
        fullLabel = node.class
      } else {
        // 都没有时，翻译漏洞类型作为标签
        displayLabel = translateVulnType(node.type || '未知漏洞类型')
        fullLabel = displayLabel
      }
      
      // 如果标签太长，截断
      if (displayLabel.length > 25) {
        displayLabel = displayLabel.substring(0, 22) + '...'
      }
      
      // 添加文件名和行号到完整标签
      if (node.file) {
        fullLabel += `\n${node.file}${node.line ? ':' + node.line : ''}`
      }
      
      elements.push({
        data: {
          id: node.id,
          label: displayLabel,
          displayLabel: displayLabel,
          fullLabel: fullLabel,
          type: node.type || 'unknown',
          translatedType: translateVulnType(node.type || '未知漏洞类型'),
          severity: node.severity || 'Low',
          file: node.file || '',
          line: node.line || 0,
          class: node.class || '',
          function: node.function || '',
          description: node.description || '',
          analysis: node.analysis || '',
          fixSuggestion: node.fixSuggestion || '',
          cwe: node.cwe || '',
          confidence: node.confidence || '',
          attackVector: node.attackVector || '',
          codeSnippet: node.codeSnippet || '',
          vulnId: node.vulnId || node.id,
        },
        position: {
          x: x,
          y: y
        }
      })
    })
    
    currentLayer++
  })
  
  return elements
}

// 检查是否为常见的非函数名
const isCommonNonFunctionName = (name: string): boolean => {
  const nonFuncs = ['if', 'else', 'for', 'while', 'do', 'switch', 'case', 'break', 'continue',
    'return', 'throw', 'try', 'catch', 'finally', 'new', 'this', 'super', 'class', 'interface',
    'extends', 'implements', 'import', 'package', 'public', 'private', 'protected', 'static',
    'final', 'abstract', 'void', 'int', 'long', 'short', 'byte', 'char', 'boolean', 'float',
    'double', 'String', 'Integer', 'Long', 'Boolean', 'List', 'Map', 'Set', 'Object', 'System']
  return nonFuncs.includes(name)
}

// 获取漏洞图样式 - 小图标，减少交叉
const getVulnGraphStyle = (): cytoscape.StylesheetStyle[] => {
  return [
    {
      selector: 'node',
      style: {
        'label': 'data(label)',
        'text-valign': 'bottom',
        'text-halign': 'center',
        'text-margin-x': 0,
        'text-margin-y': 4,
        'font-size': '9px',
        'color': '#333',
        'background-color': '#909399',
        'border-width': 2,
        'border-color': '#666',
        'width': 24,
        'height': 24,
        'shape': 'ellipse',
        'text-wrap': 'truncate',
        'text-max-width': '70px',
        'text-background-color': '#fff',
        'text-background-opacity': 0.8,
        'text-background-padding': '2px',
      }
    },
    // Critical - 红色三角形
    {
      selector: 'node[severity="Critical"]',
      style: {
        'background-color': '#F56C6C',
        'border-color': '#d93a3a',
        'shape': 'triangle',
        'width': 22,
        'height': 22,
        'font-size': '9px',
      }
    },
    // High - 橙色菱形
    {
      selector: 'node[severity="High"]',
      style: {
        'background-color': '#E6A23C',
        'border-color': '#cf9236',
        'shape': 'diamond',
        'width': 20,
        'height': 20,
        'font-size': '9px',
      }
    },
    // Medium - 蓝色正方形
    {
      selector: 'node[severity="Medium"]',
      style: {
        'background-color': '#409EFF',
        'border-color': '#337ecc',
        'shape': 'square',
        'width': 18,
        'height': 18,
        'font-size': '9px',
      }
    },
    // Low - 绿色椭圆
    {
      selector: 'node[severity="Low"]',
      style: {
        'background-color': '#67C23A',
        'border-color': '#529b2e',
        'shape': 'ellipse',
        'width': 16,
        'height': 16,
        'font-size': '9px',
      }
    },
    // 悬停效果
    {
      selector: 'node:selected',
      style: {
        'border-width': 3,
        'border-color': '#F56C6C',
        'width': 28,
        'height': 28,
      }
    },
    // 边样式
    {
      selector: 'edge',
      style: {
        'width': 1,
        'line-color': '#c0c4cc',
        'target-arrow-color': '#c0c4cc',
        'target-arrow-shape': 'triangle',
        'curve-style': 'bezier',
        'opacity': 0.3,
      }
    },
    // 选中边
    {
      selector: 'edge:selected',
      style: {
        'line-color': '#F56C6C',
        'target-arrow-color': '#F56C6C',
        'width': 2,
        'opacity': 1,
      }
    },
  ]
}

// 获取布局配置
const getLayoutConfig = (layoutName: string) => {
  const containerWidth = graphContainer.value?.clientWidth || 800
  const containerHeight = graphContainer.value?.clientHeight || 600
  const centerX = containerWidth / 2
  const centerY = containerHeight / 2
  const radius = Math.min(containerWidth, containerHeight) * 0.35
  
  switch (layoutName) {
    case 'circle':
      return {
        name: 'circle',
        radius: radius,
        startAngle: 0,
        sweep: 360,
        clockwise: true,
        equidistant: true,
        padding: 50,
        nodes: cy ? cy.nodes().map((n: any) => n.id()) : [],
        animate: true,
        animationDuration: 500,
        ready: () => {},
        stop: () => {},
      }
    case 'grid':
      return {
        name: 'grid',
        cols: Math.ceil(Math.sqrt(nodeCount.value || 1)) || 4,
        rows: Math.ceil((nodeCount.value || 1) / Math.ceil(Math.sqrt(nodeCount.value || 1))) || 3,
        padding: 50,
        animate: true,
        animationDuration: 500,
      }
    case 'random':
      return {
        name: 'random',
        padding: 50,
        animate: true,
        animationDuration: 500,
      }
    default:
      return {
        name: 'circle',
        radius: radius,
        startAngle: 0,
        sweep: 360,
        clockwise: true,
        equidistant: true,
        padding: 50,
        animate: true,
        animationDuration: 500,
      }
  }
}

// 处理布局切换
const handleLayoutChange = (command: string) => {
  currentLayout.value = command
  runLayout(command)
}

// 运行指定布局
const runLayout = (layoutName: string) => {
  if (!cy) return
  const config = getLayoutConfig(layoutName)
  cy.layout(config).run()
}

// 重新布局
const reLayout = () => {
  runLayout(currentLayout.value)
}

// 加载节点漏洞利用链
const loadExploitChain = async (nodeId: string) => {
  try {
    // 获取该漏洞的调用链信息
    const response = await fetch(`/api/tasks/${props.taskId}/callgraph/relations?nodeId=${encodeURIComponent(nodeId)}`, {
      headers: {
        'Authorization': `Bearer ${localStorage.getItem('auth_token')}`
      }
    })
    
    if (response.ok) {
      const result = await response.json()
      const data = result.Data || result.data
      
      if (data) {
        const callers = data.callers || []
        const callees = data.callees || []
        
        // 构建漏洞利用链
        const chain: any[] = []
        
        // Source：用户输入
        if (callers.length > 0) {
          const sourceNode = callers[callers.length - 1]
          chain.push({
            nodeType: 'source',
            label: sourceNode.label || sourceNode.function || '用户输入',
            file: sourceNode.file,
            line: sourceNode.line,
            class: sourceNode.class,
            function: sourceNode.function,
            type: 'caller',
            displayLabel: (sourceNode.class ? sourceNode.class + '.' : '') + (sourceNode.function || sourceNode.label || '用户输入')
          })
        }
        
        // 中间调用者（从源头到当前漏洞）
        callers.slice(0, -1).reverse().forEach(caller => {
          chain.push({
            nodeType: 'normal',
            label: caller.label || caller.function,
            file: caller.file,
            line: caller.line,
            class: caller.class,
            function: caller.function,
            displayLabel: (caller.class ? caller.class + '.' : '') + (caller.function || caller.label || '未知'),
            type: 'caller'
          })
        })
        
        // 当前漏洞
        const currentNode = allNodes.value.find(n => n.id === nodeId)
        if (currentNode) {
          chain.push({
            nodeType: 'current',
            id: currentNode.id,
            label: translateVulnType(currentNode.displayLabel || currentNode.type),
            file: currentNode.file,
            line: currentNode.line,
            class: currentNode.class,
            function: currentNode.function,
            severity: currentNode.severity,
            displayLabel: (currentNode.class ? currentNode.class + '.' : '') + (currentNode.function || translateVulnType(currentNode.displayLabel || currentNode.type))
          })
        }
        
        // 中间被调用者
        callees.slice(0, -1).forEach(callee => {
          chain.push({
            nodeType: 'normal',
            label: callee.label || callee.function,
            file: callee.file,
            line: callee.line,
            class: callee.class,
            function: callee.function,
            displayLabel: (callee.class ? callee.class + '.' : '') + (callee.function || callee.label || '未知'),
            type: 'callee'
          })
        })
        
        // Sink：危险操作
        if (callees.length > 0) {
          const sinkNode = callees[callees.length - 1]
          chain.push({
            nodeType: 'sink',
            label: sinkNode.label || sinkNode.function || '危险操作',
            file: sinkNode.file,
            line: sinkNode.line,
            class: sinkNode.class,
            function: sinkNode.function,
            type: 'callee',
            displayLabel: (sinkNode.class ? sinkNode.class + '.' : '') + (sinkNode.function || sinkNode.label || '危险操作')
          })
        }
        
        // 如果没有调用链数据，生成一个简单的流程
        if (chain.length === 0 && currentNode) {
          // 生成简单的利用链：Source -> Controller -> Service -> DAO -> Sink
          chain.push({
            nodeType: 'source',
            label: '用户输入',
            file: currentNode.file,
            line: Math.max(1, (currentNode.line || 10) - 10),
            class: '',
            function: '用户输入',
            type: 'caller',
            displayLabel: '[Source] 用户输入'
          })
          
          // 尝试添加一些中间节点
          if (currentNode.class) {
            chain.push({
              nodeType: 'normal',
              label: currentNode.class,
              file: currentNode.file,
              line: currentNode.line,
              class: currentNode.class,
              function: currentNode.function || '未知方法',
              type: 'caller',
              displayLabel: currentNode.class + '.' + (currentNode.function || '未知方法')
            })
          }
          
          chain.push({
            nodeType: 'current',
            id: currentNode.id,
            label: translateVulnType(currentNode.displayLabel || currentNode.type),
            file: currentNode.file,
            line: currentNode.line,
            class: currentNode.class,
            function: currentNode.function,
            severity: currentNode.severity,
            displayLabel: (currentNode.class ? currentNode.class + '.' : '') + (currentNode.function || translateVulnType(currentNode.displayLabel || currentNode.type))
          })
          
          chain.push({
            nodeType: 'sink',
            label: '危险操作',
            file: currentNode.file,
            line: (currentNode.line || 10) + 5,
            class: '',
            function: '危险操作',
            type: 'callee',
            displayLabel: '[Sink] 危险操作'
          })
        }
        
        exploitChain.value = chain
        
        // 生成简化版链
        simplifiedChain.value = chain.map(item => ({
          label: item.displayLabel || item.label || item.class + '.' + item.function || '未知',
          file: item.file,
          line: item.line
        }))
      }
    }
  } catch (error) {
    console.error('加载漏洞利用链失败:', error)
    
    // 如果加载失败，生成一个基于当前节点信息的简单链
    const currentNode = allNodes.value.find(n => n.id === nodeId)
    if (currentNode) {
      const simpleChain: any[] = []
      
      simpleChain.push({
        nodeType: 'source',
        label: '用户输入',
        file: currentNode.file,
        line: Math.max(1, (currentNode.line || 10) - 10),
        type: 'caller',
        displayLabel: '[Source] 用户输入'
      })
      
      if (currentNode.class || currentNode.function) {
        simpleChain.push({
          nodeType: 'normal',
          label: currentNode.function || currentNode.class || '未知',
          file: currentNode.file,
          line: currentNode.line,
          class: currentNode.class,
          function: currentNode.function,
          type: 'caller',
          displayLabel: (currentNode.class ? currentNode.class + '.' : '') + (currentNode.function || '未知')
        })
      }
      
      simpleChain.push({
        nodeType: 'current',
        id: currentNode.id,
        label: translateVulnType(currentNode.displayLabel || currentNode.type),
        file: currentNode.file,
        line: currentNode.line,
        class: currentNode.class,
        function: currentNode.function,
        severity: currentNode.severity,
        displayLabel: (currentNode.class ? currentNode.class + '.' : '') + (currentNode.function || translateVulnType(currentNode.displayLabel || currentNode.type))
      })
      
      if (currentNode.file) {
        simpleChain.push({
          nodeType: 'sink',
          label: '危险操作',
          file: currentNode.file,
          line: Math.max(1, (currentNode.line || 10) + 5),
          type: 'callee',
          displayLabel: '[Sink] 危险操作'
        })
      }

      exploitChain.value = simpleChain
      simplifiedChain.value = simpleChain.map(item => ({
        label: item.displayLabel || item.label || '未知',
        file: item.file,
        line: item.line
      }))
    } else {
      // 即使没有currentNode，也尝试构建一个基本的利用链
      const basicChain: any[] = []
      const currentNode = allNodes.value.find(n => n.id === nodeId)
      
      if (currentNode) {
        // Source：用户输入点
        basicChain.push({
          nodeType: 'source',
          label: '用户输入',
          file: currentNode.file,
          line: currentNode.line ? Math.max(1, currentNode.line - 10) : 1,
          type: 'caller',
          displayLabel: '[Source] 用户输入点'
        })
        
        // 当前漏洞节点
        basicChain.push({
          nodeType: 'current',
          id: currentNode.id,
          label: translateVulnType(currentNode.type || '安全漏洞'),
          file: currentNode.file,
          line: currentNode.line || 1,
          class: currentNode.class,
          function: currentNode.function,
          severity: currentNode.severity,
          displayLabel: currentNode.class && currentNode.function 
            ? `${currentNode.class}.${currentNode.function}` 
            : translateVulnType(currentNode.type || '安全漏洞')
        })
        
        // Sink：危险操作
        basicChain.push({
          nodeType: 'sink',
          label: '危险操作',
          file: currentNode.file,
          line: currentNode.line ? currentNode.line + 5 : 10,
          type: 'callee',
          displayLabel: '[Sink] 危险操作'
        })
      }
      
      exploitChain.value = basicChain
      simplifiedChain.value = basicChain.map(item => ({
        label: item.displayLabel || item.label || '未知',
        file: item.file,
        line: item.line
      }))
    }
  }
}

// 绑定图形事件
const bindGraphEvents = (cyInstance: cytoscape.Core) => {
  cyInstance.on('tap', 'node', async (evt) => {
    const node = evt.target
    const nodeData = node.data()
    
    // 保存选中的节点信息
    selectedNodeInfo.value = { ...nodeData }
    
    // 清空之前的利用链
    exploitChain.value = []
    simplifiedChain.value = []
    
    // 加载该节点的漏洞利用链
    await loadExploitChain(nodeData.id)
    
    // 打开侧边栏
    drawerVisible.value = true
  })
  
  cyInstance.on('tap', (evt) => {
    if (evt.target === cyInstance) {
      drawerVisible.value = false
    }
  })
  
  // 拖拽结束后保存位置
  cyInstance.on('dragfree', 'node', (evt) => {
    const node = evt.target
    saveNodePosition(node.id(), node.position())
  })
}

// 适应屏幕
const fitToScreen = () => {
  if (!cy) return
  cy.fit(50)
}

// 聚焦到选中节点
const focusOnNode = () => {
  if (!cy || !selectedNodeInfo.value.id) return
  
  const node = cy.getElementById(selectedNodeInfo.value.id)
  if (node.length > 0) {
    cy.animate({
      center: { eles: node },
      zoom: 1.5,
    }, { duration: 300 })
  }
}

// 跳转到链中的节点
const navigateToChainNode = (nodeData: any) => {
  if (!cy || !nodeData.id) return
  
  const node = cy.getElementById(nodeData.id)
  if (node.length > 0) {
    cy.animate({
      center: { eles: node },
      zoom: 1.5,
    }, { duration: 300 })
  }
}

// 全屏相关
const toggleFullscreen = async () => {
  if (isFullscreen.value) {
    closeFullscreen()
  } else {
    await openFullscreen()
  }
}

const openFullscreen = async () => {
  isFullscreen.value = true
  await nextTick()
  
  if (fullscreenContainer.value && cy) {
    const elements = cy.elements().map((el: any) => el.json())
    
    cyFullscreen = cytoscape({
      container: fullscreenContainer.value,
      elements: elements,
      style: getVulnGraphStyle(),
      layout: getLayoutConfig(currentLayout.value),
      minZoom: 0.1,
      maxZoom: 3,
      wheelSensitivity: 0.3,
    })
    
    bindGraphEvents(cyFullscreen)
    cyFullscreen.nodes().grabify()
  }
}

const closeFullscreen = () => {
  isFullscreen.value = false
  if (cyFullscreen) {
    cyFullscreen.destroy()
    cyFullscreen = null
  }
}

// 获取严重程度对应的标签类型
const getSeverityTagType = (severity: string) => {
  switch (severity) {
    case 'Critical': return 'danger'
    case 'High': return 'warning'
    case 'Medium': return 'info'
    case 'Low': return 'success'
    default: return 'info'
  }
}

// 位置保存相关
const STORAGE_KEY = `vulngraph-pos-${props.taskId}`

const saveNodePosition = (nodeId: string, position: { x: number, y: number }) => {
  const positions = getSavedPositions()
  positions[nodeId] = position
  localStorage.setItem(STORAGE_KEY, JSON.stringify(positions))
}

const saveAllPositions = () => {
  if (!cy) return
  const positions: { [key: string]: { x: number, y: number } } = {}
  cy.nodes().forEach((node: any) => {
    const pos = node.position()
    positions[node.id()] = { x: pos.x, y: pos.y }
  })
  localStorage.setItem(STORAGE_KEY, JSON.stringify(positions))
}

const getSavedPositions = (): { [key: string]: { x: number, y: number } } => {
  const saved = localStorage.getItem(STORAGE_KEY)
  return saved ? JSON.parse(saved) : {}
}

// 监听全屏状态
watch(isFullscreen, (newVal) => {
  if (newVal) {
    document.body.style.overflow = 'hidden'
  } else {
    document.body.style.overflow = ''
  }
})

// 生命周期
onMounted(() => {
  initGraph()
})

onUnmounted(() => {
  if (cy) {
    cy.destroy()
  }
  if (cyFullscreen) {
    cyFullscreen.destroy()
  }
})
</script>

<style scoped>
.call-graph-container {
  position: relative;
  width: 100%;
  height: 100%;
  min-height: 450px;
}

.graph-controls {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px;
  background: #f5f7fa;
  border-radius: 4px;
  margin-bottom: 12px;
  flex-wrap: wrap;
}

.graph-stats {
  font-size: 12px;
  color: #606266;
}

.graph-title {
  display: flex;
  align-items: center;
  gap: 6px;
  font-weight: 600;
  color: #303133;
}

.graph-view {
  width: 100%;
  height: 550px;
  border: 1px solid #ebeef5;
  border-radius: 4px;
  background: #fafafa;
}

/* 图例样式 */
.graph-legend {
  display: flex;
  justify-content: center;
  gap: 20px;
  padding: 10px;
  background: #f5f7fa;
  border-radius: 4px;
  margin-top: 12px;
}

.legend-item {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: #606266;
}

.legend-icon {
  width: 14px;
  height: 14px;
  border-radius: 2px;
}

.legend-item.critical .legend-icon {
  background: #F56C6C;
  border-radius: 50%;
}

.legend-item.high .legend-icon {
  background: #E6A23C;
  transform: rotate(45deg);
  width: 12px;
  height: 12px;
}

.legend-item.medium .legend-icon {
  background: #409EFF;
}

.legend-item.low .legend-icon {
  background: #67C23A;
  border-radius: 50%;
  width: 14px;
  height: 10px;
}

/* 全屏视图 */
.fullscreen-view {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  z-index: 2000;
  background: white;
}

.fullscreen-container {
  width: 100%;
  height: 100%;
}

/* 节点信息侧边栏 */
.node-info {
  padding: 0 16px;
  padding-bottom: 20px;
}

.info-section {
  margin-bottom: 20px;
}

.info-section h4 {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 0 0 12px 0;
  font-size: 14px;
  color: #303133;
}

.info-actions {
  display: flex;
  gap: 8px;
  margin-top: 20px;
}

/* 文件路径样式 */
.file-path {
  word-break: break-all;
  font-size: 12px;
  color: #606266;
}

/* 代码位置样式 */
.code-location {
  font-size: 11px;
  color: #909399;
  margin-left: 8px;
}

/* 漏洞利用链样式 */
.chain-visualization {
  background: #f5f7fa;
  border-radius: 6px;
  padding: 12px;
  max-height: 400px;
  overflow-y: auto;
}

.chain-flow-vertical {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
}

/* 简化版漏洞利用链 */
.chain-simple {
  background: #f5f7fa;
  border-radius: 6px;
  padding: 12px;
}

.chain-step {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  padding: 8px 0;
  border-bottom: 1px solid #ebeef5;
}

.chain-step:last-child {
  border-bottom: none;
}

.step-index {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  background: #409eff;
  color: white;
  border-radius: 50%;
  font-size: 12px;
  font-weight: bold;
  flex-shrink: 0;
}

.step-content {
  display: flex;
  flex-direction: column;
  gap: 4px;
  flex: 1;
  min-width: 0;
}

.step-label {
  font-size: 13px;
  color: #303133;
  font-weight: 500;
  word-break: break-all;
}

.step-location {
  font-size: 11px;
  color: #909399;
}

/* Source 节点样式 */
.chain-node-source {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 14px;
  background: linear-gradient(135deg, #67c23a 0%, #85ce61 100%);
  border-radius: 8px;
  color: white;
  cursor: pointer;
  transition: all 0.3s;
  width: 100%;
  max-width: 380px;
  box-shadow: 0 2px 8px rgba(103, 194, 58, 0.3);
}

.chain-node-source:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(103, 194, 58, 0.4);
}

.chain-node-source .chain-node-icon {
  font-size: 18px;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  background: rgba(255, 255, 255, 0.2);
  border-radius: 50%;
}

.chain-node-source .chain-node-content {
  display: flex;
  flex-direction: column;
  gap: 2px;
  flex: 1;
  min-width: 0;
}

.chain-node-source .chain-label {
  color: white;
  font-weight: 600;
  font-size: 13px;
}

.chain-node-source .chain-location {
  color: rgba(255, 255, 255, 0.85);
  font-size: 11px;
}

/* Sink 节点样式 */
.chain-node-sink {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 14px;
  background: linear-gradient(135deg, #f56c6c 0%, #fa5555 100%);
  border-radius: 8px;
  color: white;
  cursor: pointer;
  transition: all 0.3s;
  width: 100%;
  max-width: 380px;
  box-shadow: 0 2px 8px rgba(245, 108, 108, 0.3);
}

.chain-node-sink:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(245, 108, 108, 0.4);
}

.chain-node-sink .chain-node-icon {
  font-size: 18px;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  background: rgba(255, 255, 255, 0.2);
  border-radius: 50%;
}

.chain-node-sink .chain-node-content {
  display: flex;
  flex-direction: column;
  gap: 2px;
  flex: 1;
  min-width: 0;
}

.chain-node-sink .chain-label {
  color: white;
  font-weight: 600;
  font-size: 13px;
}

.chain-node-sink .chain-location {
  color: rgba(255, 255, 255, 0.85);
  font-size: 11px;
}

/* 当前漏洞节点样式 */
.chain-node-current {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 16px;
  background: linear-gradient(135deg, #e6a23c 0%, #eebe77 100%);
  border-radius: 8px;
  color: white;
  cursor: pointer;
  transition: all 0.3s;
  width: 100%;
  max-width: 380px;
  box-shadow: 0 2px 12px rgba(230, 162, 60, 0.4);
  border: 2px solid #fff;
  position: relative;
}

.chain-node-current::before {
  content: '当前漏洞';
  position: absolute;
  top: -10px;
  left: 50%;
  transform: translateX(-50%);
  background: #f56c6c;
  color: white;
  padding: 2px 10px;
  border-radius: 10px;
  font-size: 10px;
  font-weight: 600;
}

.chain-node-current:hover {
  transform: translateY(-2px) scale(1.02);
  box-shadow: 0 6px 16px rgba(230, 162, 60, 0.5);
}

.chain-node-current .chain-node-icon {
  font-size: 20px;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  background: rgba(255, 255, 255, 0.25);
  border-radius: 50%;
}

.chain-node-current .chain-node-content {
  display: flex;
  flex-direction: column;
  gap: 2px;
  flex: 1;
  min-width: 0;
}

.chain-node-current .chain-label {
  color: white;
  font-weight: 700;
  font-size: 14px;
}

.chain-node-current .chain-severity {
  margin-top: 2px;
}

.chain-node-current .chain-location {
  color: rgba(255, 255, 255, 0.9);
  font-size: 11px;
}

/* 普通调用节点样式 */
.chain-node-normal {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 12px;
  background: white;
  border-radius: 6px;
  border: 1px solid #dcdfe6;
  cursor: pointer;
  transition: all 0.2s;
  width: 100%;
  max-width: 360px;
}

.chain-node-normal:hover {
  background: #f5f7fa;
  border-color: #409eff;
  transform: translateX(4px);
}

.chain-node-normal.caller {
  border-left: 3px solid #409eff;
}

.chain-node-normal.callee {
  border-left: 3px solid #67c23a;
}

.chain-node-normal .chain-node-icon {
  font-size: 14px;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  background: #f5f7fa;
  border-radius: 50%;
  color: #606266;
  flex-shrink: 0;
}

.chain-node-normal .chain-node-content {
  display: flex;
  flex-direction: column;
  gap: 2px;
  flex: 1;
  min-width: 0;
}

.chain-node-normal .chain-label {
  color: #303133;
  font-weight: 500;
  font-size: 12px;
  word-break: break-all;
}

.chain-node-normal .chain-location {
  color: #909399;
  font-size: 10px;
}

/* 垂直连接箭头 */
.chain-connector {
  display: flex;
  justify-content: center;
  align-items: center;
  height: 20px;
  position: relative;
}

.chain-connector::before {
  content: '';
  position: absolute;
  width: 2px;
  height: 100%;
  background: linear-gradient(to bottom, #dcdfe6, #c0c4cc);
  left: 50%;
  transform: translateX(-50%);
}

.chain-arrow {
  color: #909399;
  font-size: 14px;
  font-weight: bold;
  background: #f5f7fa;
  padding: 0 4px;
  z-index: 1;
}

/* 漏洞描述样式 */
.vuln-description {
  background: #f5f7fa;
  border-radius: 4px;
  padding: 12px;
  font-size: 13px;
  line-height: 1.6;
  color: #303133;
  max-height: 120px;
  overflow-y: auto;
}

/* 漏洞分析样式 */
.vuln-analysis {
  background: #fef0f0;
  border-radius: 4px;
  padding: 12px;
  font-size: 13px;
  line-height: 1.6;
  color: #303133;
  max-height: 120px;
  overflow-y: auto;
  border-left: 3px solid #f56c6c;
}

/* 修复建议样式 */
.vuln-fix {
  background: #f0f9eb;
  border-radius: 4px;
  padding: 12px;
  font-size: 13px;
  line-height: 1.6;
  color: #303133;
  max-height: 120px;
  overflow-y: auto;
  border-left: 3px solid #67c23a;
}
</style>

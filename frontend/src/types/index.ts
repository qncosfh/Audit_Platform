// 用户相关类型
export interface User {
  id: number
  username: string
  email: string
  role: string
  createdAt: string
}

export interface LoginRequest {
  username: string
  password: string
}

export interface RegisterRequest {
  username: string
  email: string
  password: string
}

// 代码源类型
export type CodeSourceType = 'zip' | 'jar' | 'git'

export interface CodeSource {
  id: number
  userId: number
  type: CodeSourceType
  name: string
  size?: number
  url?: string
  filePath?: string
  path?: string
  status: string
  language?: string
  createdAt: string
  updatedAt: string
}

// 模型配置类型
export interface ModelConfig {
  id: number
  userId: number
  name: string
  provider: string
  apiKey: string
  baseUrl?: string
  model: string
  maxTokens: number
  createdAt: string
  updatedAt: string
  isActive?: boolean
  status?: string
}

// 任务类型
export interface Task {
  id: number
  userId: number
  name: string
  description: string
  codeSourceId: number
  modelConfigId: number
  prompt: string
  status: 'pending' | 'running' | 'completed' | 'failed'
  progress: number
  result?: string
  reportPath?: string
  codeSource?: CodeSource
  modelConfig?: ModelConfig
  createdAt: string
  updatedAt: string
  vulnerabilityCount?: number
  scannedFiles?: number
  duration?: number
  log?: string
  aiLog?: string
  currentFile?: string
  // 漏洞统计（从后端直接获取的准确值）
  criticalVulns?: number
  highVulns?: number
  mediumVulns?: number
  lowVulns?: number
  // 安全评分
  securityScore?: number
  riskLevel?: string
  // 检测语言
  detectedLanguage?: string
}

// 审计结果类型
export interface AuditResult {
  vulnerabilities: Vulnerability[]
  summary: AuditSummary
  analysis: string
  recommendations: string[]
  reportUrl?: string
}

export interface Vulnerability {
  id: string
  type: string
  severity: 'critical' | 'high' | 'medium' | 'low'
  file: string
  line: number
  description: string
  code: string
  cwe?: string
  cvss?: number
}

export interface AuditSummary {
  totalFiles: number
  scannedLines: number
  vulnerabilitiesFound: number
  criticalCount: number
  highCount: number
  mediumCount: number
  lowCount: number
  scanTime: number
}

// API响应类型 - 同时支持大小写格式
export interface ApiResponse<T = any> {
  code?: number
  Code?: number
  message?: string
  Message?: string
  data?: T
  Data?: T
}

export interface PaginatedResponse<T> {
  items: T[]
  total: number
  page: number
  pageSize: number
}

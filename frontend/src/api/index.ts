import axios, { AxiosInstance, AxiosRequestConfig, AxiosResponse } from 'axios'
import { ElMessage } from 'element-plus'
import type { User, LoginRequest, RegisterRequest, ModelConfig, Task, CodeSource, ApiResponse } from '@/types'

// Token存储键名
const TOKEN_KEY = 'auth_token'
const TOKEN_EXPIRY_KEY = 'auth_token_expiry'

// 安全地存储Token（带过期时间）
const setToken = (token: string, expiresInHours: number = 24) => {
  const expiryTime = Date.now() + expiresInHours * 60 * 60 * 1000
  localStorage.setItem(TOKEN_KEY, token)
  localStorage.setItem(TOKEN_EXPIRY_KEY, expiryTime.toString())
}

// 安全地获取Token（带过期检查）
const getToken = (): string | null => {
  const token = localStorage.getItem(TOKEN_KEY)
  const expiryStr = localStorage.getItem(TOKEN_EXPIRY_KEY)
  
  if (!token || !expiryStr) {
    return null
  }
  
  const expiry = parseInt(expiryStr, 10)
  if (Date.now() > expiry) {
    // Token已过期，清理存储
    localStorage.removeItem(TOKEN_KEY)
    localStorage.removeItem(TOKEN_EXPIRY_KEY)
    return null
  }
  
  return token
}

// 清理Token
const clearToken = () => {
  localStorage.removeItem(TOKEN_KEY)
  localStorage.removeItem(TOKEN_EXPIRY_KEY)
}

// 创建axios实例
const api: AxiosInstance = axios.create({
  baseURL: '/api',
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json'
  }
})

// 文件上传专用axios实例 - 超时时间设置为5分钟
const uploadApi: AxiosInstance = axios.create({
  baseURL: '/api',
  timeout: 300000, // 5分钟
  headers: {
    'Content-Type': 'multipart/form-data'
  }
})

// 请求拦截器 - 使用安全的Token获取方式
api.interceptors.request.use(
  (config) => {
    const token = getToken()
    if (token) {
      config.headers.Authorization = `Bearer ${token}`
    }
    return config
  },
  (error) => {
    return Promise.reject(error)
  }
)

// 上传专用请求拦截器
uploadApi.interceptors.request.use(
  (config) => {
    const token = getToken()
    if (token) {
      config.headers.Authorization = `Bearer ${token}`
    }
    return config
  },
  (error) => {
    return Promise.reject(error)
  }
)

// 响应拦截器 - 处理后端返回的统一响应格式
api.interceptors.response.use(
  (response: AxiosResponse) => {
    // 后端返回的响应格式可能是 {Code, Message, Data} 或 {code, message, data}
    const data = response.data as any
    
    const code = data.Code || data.code
    const message = data.Message || data.message
    
    if (code !== 200 && code !== undefined) {
      ElMessage.error(message || '请求失败')
      return Promise.reject(new Error(message || '请求失败'))
    }
    
    return response
  },
  (error) => {
    if (error.response?.status === 401) {
      // 检查是否是刷新token的请求，避免无限循环
      const originalRequest = error.config
      if (!originalRequest._retry && !originalRequest.url?.includes('/auth/refresh')) {
        originalRequest._retry = true
        
        // 限制重试次数，避免无限循环
        if (originalRequest._retryCount && originalRequest._retryCount >= 2) {
          clearToken()
          window.location.href = '/login'
          return Promise.reject(error)
        }
        
        originalRequest._retryCount = (originalRequest._retryCount || 0) + 1
        
        // 尝试刷新token
        return api.post('/auth/refresh').then((res) => {
          const newToken = res.data.Data?.token || res.data.data?.token
          if (newToken) {
            // 使用安全的Token存储方式
            setToken(newToken, 24) // 24小时有效期
            originalRequest.headers.Authorization = `Bearer ${newToken}`
            return api(originalRequest)
          } else {
            throw new Error('Token刷新失败')
          }
        }).catch(() => {
          clearToken()
          // 跳转到登录页
          window.location.href = '/login'
          return Promise.reject(error)
        })
      }
      
      clearToken()
      window.location.href = '/login'
    } else {
      ElMessage.error(error.response?.data?.Message || error.response?.data?.message || error.message || '网络错误')
    }
    return Promise.reject(error)
  }
)

// 上传专用响应拦截器
uploadApi.interceptors.response.use(
  (response: AxiosResponse) => {
    const data = response.data as any
    
    const code = data.Code || data.code
    const message = data.Message || data.message
    
    if (code !== 200 && code !== undefined) {
      ElMessage.error(message || '上传失败')
      return Promise.reject(new Error(message || '上传失败'))
    }
    
    return response
  },
  (error) => {
    if (error.response?.status === 401) {
      const originalRequest = error.config
      if (!originalRequest._retry && !originalRequest.url?.includes('/auth/refresh')) {
        originalRequest._retry = true
        
        return uploadApi.post('/auth/refresh').then((res) => {
          const newToken = res.data.Data?.token || res.data.data?.token
          if (newToken) {
            setToken(newToken, 24)
            originalRequest.headers.Authorization = `Bearer ${newToken}`
            return uploadApi(originalRequest)
          } else {
            throw new Error('Token刷新失败')
          }
        }).catch(() => {
          clearToken()
          window.location.href = '/login'
          return Promise.reject(error)
        })
      }
      
      clearToken()
      window.location.href = '/login'
    } else {
      ElMessage.error(error.response?.data?.Message || error.response?.data?.message || error.message || '上传失败')
    }
    return Promise.reject(error)
  }
)

// 获取响应数据 - 兼容大小写
const getResponseData = (response: AxiosResponse) => {
  return response.data.Data || response.data.data
}

// 用户认证相关API
export const authApi = {
  login: (data: LoginRequest) => api.post<ApiResponse<{ token: string; user: User }>>('/auth/login', data),
  register: (data: RegisterRequest) => api.post<ApiResponse<{ token: string; user: User }>>('/auth/register', data),
  getCurrentUser: () => api.get<ApiResponse<User>>('/auth/me'),
  logout: async () => {
    try {
      await api.post<ApiResponse>('/auth/logout')
    } finally {
      // 无论请求成功与否，都清理本地token
      clearToken()
    }
  },
  refreshToken: () => api.post<ApiResponse<{ token: string; user: User }>>('/auth/refresh'),
  changePassword: (oldPassword: string, newPassword: string) => 
    api.post<ApiResponse>('/auth/change-password', { old_password: oldPassword, new_password: newPassword })
}

// 导出安全存储函数供其他模块使用
export const secureStorage = {
  setToken,
  getToken,
  clearToken
}

// 代码源管理API
export const codeSourceApi = {
  uploadZip: (file: File) => {
    const formData = new FormData()
    formData.append('file', file)
    return uploadApi.post<ApiResponse<CodeSource>>('/code-sources/upload/zip', formData)
  },
  uploadJar: (file: File) => {
    const formData = new FormData()
    formData.append('file', file)
    return uploadApi.post<ApiResponse<CodeSource>>('/code-sources/upload/jar', formData)
  },
  addGitRepo: (url: string) => api.post<ApiResponse<CodeSource>>('/code-sources/git', { url }),
  addLocalPath: (path: string) => api.post<ApiResponse<CodeSource>>('/code-sources/path', { path }),
  list: (page: number = 1, pageSize: number = 10) => 
    api.get<ApiResponse<{ items: CodeSource[]; total: number }>>('/code-sources', { params: { page, pageSize } }),
  get: (id: string | number) => api.get<ApiResponse<any>>(`/code-sources/${id}`),
  getFile: (id: string | number, path: string) => api.get<ApiResponse<any>>(`/code-sources/${id}/file`, { params: { path } }),
  delete: (id: string | number) => api.delete<ApiResponse>(`/code-sources/${id}`)
}

// 模型配置API
export const modelApi = {
  list: () => api.get<ApiResponse<ModelConfig[]>>('/models'),
  create: (data: any) => api.post<ApiResponse<ModelConfig>>('/models', data),
  update: (id: string | number, data: any) => api.put<ApiResponse<ModelConfig>>(`/models/${id}`, data),
  delete: (id: string | number) => api.delete<ApiResponse>(`/models/${id}`),
  test: (id: string | number) => api.post<ApiResponse<{ success: boolean; message: string; response: string }>>(`/models/${id}/test`)
}

// 任务管理API
export const taskApi = {
  list: (page: number = 1, pageSize: number = 10) => 
    api.get<ApiResponse<{ items: Task[]; total: number }>>('/tasks', { params: { page, pageSize } }),
  create: (data: any) => api.post<ApiResponse<Task>>('/tasks', data),
  get: (id: string | number) => api.get<ApiResponse<Task>>(`/tasks/${id}`),
  getDetail: (id: string | number) => api.get<ApiResponse<Task>>(`/tasks/${id}/detail`),
  update: (id: string | number, data: any) => api.put<ApiResponse<Task>>(`/tasks/${id}`, data),
  updateProgress: (id: string | number, data: any) => api.put<ApiResponse<Task>>(`/tasks/${id}/progress`, data),
  delete: (id: string | number) => api.delete<ApiResponse>(`/tasks/${id}`),
  start: (id: string | number) => api.post<ApiResponse>(`/tasks/${id}/start`),
  stop: (id: string | number) => api.post<ApiResponse>(`/tasks/${id}/stop`),
  exportReport: (id: string | number) => api.get<ApiResponse<{ url: string }>>(`/tasks/${id}/export`),
  // 调用图API
  getCallGraph: (id: string | number, func?: string, depth?: number) => 
    api.get<ApiResponse<any>>(`/tasks/${id}/callgraph`, { 
      params: { func, depth } 
    }),
  getCallGraphText: (id: string | number, func?: string) => 
    api.get<ApiResponse<{ text: string }>>(`/tasks/${id}/callgraph/text`, { 
      params: { func } 
    }),
  // 获取节点的Callees和Callers
  getNodeRelations: (id: string | number, nodeId: string) => 
    api.get<ApiResponse<{ callees: any[]; callers: any[] }>>(`/tasks/${id}/callgraph/relations`, { 
      params: { nodeId } 
    }),
  // 获取源代码API
  getSourceCode: (id: string | number, file: string, line?: number) => 
    api.get<ApiResponse<{ file: string; line: number; total: number; code: any[]; fullPath: string }>>(`/tasks/${id}/source`, { 
      params: { file, line: line || 1 } 
    })
}

// 报告管理API
export const reportApi = {
  list: () => api.get<ApiResponse<any[]>>('/reports'),
  get: (id: string | number) => api.get<ApiResponse<{ url: string; filename: string }>>(`/reports/${id}`),
  download: (id: string | number) => api.get<ApiResponse>(`/reports/${id}/download`, { responseType: 'blob' }),
  delete: (id: string | number) => api.delete<ApiResponse>(`/reports/${id}`)
}

// 分析API - 商业级功能
export const analysisApi = {
  // 漏洞相关
  getVulnerabilities: (taskId: string | number, severity?: string) => 
    api.get<ApiResponse<{ items: any[]; stats: any; total: number }>>(`/analysis/${taskId}/vulnerabilities`, { 
      params: severity ? { severity } : {} 
    }),
  getVulnerability: (taskId: string | number, vulnId: string | number) => 
    api.get<ApiResponse<any>>(`/analysis/${taskId}/vulnerabilities/${vulnId}`),
  
  // 利用链
  getExploitChains: (taskId: string | number) => 
    api.get<ApiResponse<{ items: any[]; total: number }>>(`/analysis/${taskId}/chains`),
  
  // 项目统计
  getProjectStats: (taskId: string | number) => 
    api.get<ApiResponse<any>>(`/analysis/${taskId}/stats`),
  
  // 跨文件分析
  getCrossFileAnalysis: (taskId: string | number) => 
    api.get<ApiResponse<any>>(`/analysis/${taskId}/crossfile`),
  
  // 依赖分析
  getDependencyAnalysis: (taskId: string | number) => 
    api.get<ApiResponse<any>>(`/analysis/${taskId}/dependency`),
  
  // 漏洞类型分布
  getVulnerabilityTypes: (taskId: string | number) => 
    api.get<ApiResponse<any[]>>(`/analysis/${taskId}/vuln-types`),
  
  // 按严重程度获取文件
  getFilesBySeverity: (taskId: string | number, severity: string) => 
    api.get<ApiResponse<any[]>>(`/analysis/${taskId}/files`, { params: { severity } }),
  
  // 导出报告
  exportReport: (taskId: string | number, format: string = 'markdown') => 
    api.get<ApiResponse<{ content: string; format: string; filename: string }>>(`/analysis/${taskId}/report`, { 
      params: { format } 
    })
}

export default api

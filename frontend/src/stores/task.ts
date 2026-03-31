import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { taskApi, codeSourceApi, modelApi } from '@/api'
import type { Task, CodeSource, ModelConfig, PaginatedResponse } from '@/types'

export const useTaskStore = defineStore('task', () => {
  const tasks = ref<Task[]>([])
  const codeSources = ref<CodeSource[]>([])
  const models = ref<ModelConfig[]>([])
  const loading = ref(false)
  const currentTask = ref<Task | null>(null)

  const taskCount = computed(() => tasks.value.length)
  const pendingTasks = computed(() => tasks.value.filter(t => String(t.status) === 'pending'))
  const runningTasks = computed(() => tasks.value.filter(t => String(t.status) === 'running'))
  const completedTasks = computed(() => tasks.value.filter(t => String(t.status) === 'completed'))

  const loadTasks = async (page: number = 1, pageSize: number = 10) => {
    loading.value = true
    try {
      const response = await taskApi.list(page, pageSize)
      tasks.value = response.data.Data?.items || response.data.data?.items || []
      return response.data.Data || response.data.data
    } catch (error) {
      console.error('Load tasks error:', error)
      throw error
    } finally {
      loading.value = false
    }
  }

  const loadTask = async (id: string | number) => {
    loading.value = true
    try {
      const response = await taskApi.get(String(id))
      currentTask.value = response.data.Data || response.data.data
      return currentTask.value
    } catch (error) {
      console.error('Load task error:', error)
      throw error
    } finally {
      loading.value = false
    }
  }

  const createTask = async (taskData: Omit<Task, 'id' | 'status' | 'progress' | 'createdAt' | 'updatedAt' | 'result'>) => {
    loading.value = true
    try {
      const response = await taskApi.create(taskData as any)
      const newTask = response.data.Data || response.data.data
      tasks.value.unshift(newTask)
      return newTask
    } catch (error) {
      console.error('Create task error:', error)
      throw error
    } finally {
      loading.value = false
    }
  }

  const updateTask = async (id: string | number, data: Partial<Task>) => {
    loading.value = true
    try {
      const response = await taskApi.update(String(id), data)
      const updatedTask = response.data.Data || response.data.data
      
      const index = tasks.value.findIndex(t => t.id === id || t.ID === id)
      if (index !== -1) {
        tasks.value[index] = updatedTask
      }
      
      if (currentTask.value?.id === id || currentTask.value?.ID === id) {
        currentTask.value = updatedTask
      }
      
      return updatedTask
    } catch (error) {
      console.error('Update task error:', error)
      throw error
    } finally {
      loading.value = false
    }
  }

  const deleteTask = async (id: string | number) => {
    loading.value = true
    try {
      await taskApi.delete(String(id))
      tasks.value = tasks.value.filter(t => t.id !== id && t.ID !== id)
      if (currentTask.value?.id === id || currentTask.value?.ID === id) {
        currentTask.value = null
      }
    } catch (error) {
      console.error('Delete task error:', error)
      throw error
    } finally {
      loading.value = false
    }
  }

  const startTask = async (id: string | number) => {
    loading.value = true
    try {
      const response = await taskApi.start(String(id))
      const updatedTask = response.data.Data || response.data.data
      
      // 更新 tasks 数组中的任务
      const index = tasks.value.findIndex(t => t.id === id || t.ID === id)
      if (index !== -1) {
        tasks.value[index] = updatedTask
      }
      
      // 更新 currentTask（如果当前查看的就是这个任务）
      if (currentTask.value?.id === id || currentTask.value?.ID === id) {
        currentTask.value = updatedTask
      }
      
      return updatedTask
    } catch (error) {
      console.error('Start task error:', error)
      throw error
    } finally {
      loading.value = false
    }
  }

  const stopTask = async (id: string | number) => {
    loading.value = true
    try {
      await taskApi.stop(String(id))
      const task = tasks.value.find(t => t.id === id || t.ID === id)
      if (task) {
        task.status = 'failed'
      }
    } catch (error) {
      console.error('Stop task error:', error)
      throw error
    } finally {
      loading.value = false
    }
  }

  const exportReport = async (id: string | number) => {
    loading.value = true
    try {
      const response = await taskApi.exportReport(String(id))
      // 返回 content 而不是 url，因为后端返回的是内容
      return response.data.Data?.content || response.data.data?.content
    } catch (error) {
      console.error('Export report error:', error)
      throw error
    } finally {
      loading.value = false
    }
  }

  const updateProgress = async (id: string | number, data: { progress?: number; status?: string; result?: string }) => {
    loading.value = true
    try {
      const response = await taskApi.updateProgress(String(id), data)
      const updatedTask = response.data.Data || response.data.data
      
      const index = tasks.value.findIndex(t => t.id === id || t.ID === id)
      if (index !== -1) {
        tasks.value[index] = updatedTask
      }
      
      if (currentTask.value?.id === id || currentTask.value?.ID === id) {
        currentTask.value = updatedTask
      }
      
      return updatedTask
    } catch (error) {
      console.error('Update progress error:', error)
      throw error
    } finally {
      loading.value = false
    }
  }

  // 代码源管理
  const loadCodeSources = async () => {
    loading.value = true
    try {
      const response = await codeSourceApi.list(1, 100)
      codeSources.value = response.data.Data?.items || response.data.data?.items || []
    } catch (error) {
      console.error('Load code sources error:', error)
      throw error
    } finally {
      loading.value = false
    }
  }

  const uploadCodeSource = async (data: FormData) => {
    loading.value = true
    try {
      const type = data.get('type') as string
      let response
      if (type === 'zip') {
        response = await codeSourceApi.uploadZip(data.get('file') as File)
      } else if (type === 'jar') {
        response = await codeSourceApi.uploadJar(data.get('file') as File)
      } else {
        throw new Error('不支持的文件类型')
      }
      const newSource = response.data.Data || response.data.data
      codeSources.value.unshift(newSource)
      return newSource
    } catch (error) {
      console.error('Upload code source error:', error)
      throw error
    } finally {
      loading.value = false
    }
  }

  const uploadZip = async (file: File) => {
    loading.value = true
    try {
      const response = await codeSourceApi.uploadZip(file)
      const newSource = response.data.Data || response.data.data
      codeSources.value.unshift(newSource)
      return newSource
    } catch (error) {
      console.error('Upload zip error:', error)
      throw error
    } finally {
      loading.value = false
    }
  }

  const uploadJar = async (file: File) => {
    loading.value = true
    try {
      const response = await codeSourceApi.uploadJar(file)
      const newSource = response.data.Data || response.data.data
      codeSources.value.unshift(newSource)
      return newSource
    } catch (error) {
      console.error('Upload jar error:', error)
      throw error
    } finally {
      loading.value = false
    }
  }

  const addGitRepo = async (url: string) => {
    loading.value = true
    try {
      const response = await codeSourceApi.addGitRepo(url)
      const newSource = response.data.Data || response.data.data
      codeSources.value.unshift(newSource)
      return newSource
    } catch (error) {
      console.error('Add git repo error:', error)
      throw error
    } finally {
      loading.value = false
    }
  }

  const addLocalPath = async (path: string) => {
    loading.value = true
    try {
      const response = await codeSourceApi.addLocalPath(path)
      const newSource = response.data.Data || response.data.data
      codeSources.value.unshift(newSource)
      return newSource
    } catch (error) {
      console.error('Add local path error:', error)
      throw error
    } finally {
      loading.value = false
    }
  }

  const testModel = async (modelId: string | number) => {
    loading.value = true
    try {
      const response = await modelApi.test(String(modelId))
      return response.data.Data || response.data.data
    } catch (error) {
      console.error('Test model error:', error)
      throw error
    } finally {
      loading.value = false
    }
  }

  const deleteCodeSource = async (id: string | number) => {
    loading.value = true
    try {
      await codeSourceApi.delete(String(id))
      codeSources.value = codeSources.value.filter(cs => cs.id !== id && cs.ID !== id)
    } catch (error) {
      console.error('Delete code source error:', error)
      throw error
    } finally {
      loading.value = false
    }
  }

  // 模型配置管理
  const loadModels = async () => {
    loading.value = true
    try {
      const response = await modelApi.list()
      models.value = response.data.Data || response.data.data || []
    } catch (error) {
      console.error('Load models error:', error)
      throw error
    } finally {
      loading.value = false
    }
  }

  const createModel = async (modelData: Omit<ModelConfig, 'id' | 'createdAt'>) => {
    loading.value = true
    try {
      const response = await modelApi.create(modelData as any)
      const newModel = response.data.Data || response.data.data
      models.value.unshift(newModel)
      return newModel
    } catch (error) {
      console.error('Create model error:', error)
      throw error
    } finally {
      loading.value = false
    }
  }

  const updateModel = async (id: string | number, data: Partial<ModelConfig>) => {
    loading.value = true
    try {
      const response = await modelApi.update(String(id), data)
      const updatedModel = response.data.Data || response.data.data
      
      const index = models.value.findIndex(m => m.id === id || m.ID === id)
      if (index !== -1) {
        models.value[index] = updatedModel
      }
      
      return updatedModel
    } catch (error) {
      console.error('Update model error:', error)
      throw error
    } finally {
      loading.value = false
    }
  }

  const deleteModel = async (id: string | number) => {
    loading.value = true
    try {
      await modelApi.delete(String(id))
      models.value = models.value.filter(m => m.id !== id && m.ID !== id)
    } catch (error) {
      console.error('Delete model error:', error)
      throw error
    } finally {
      loading.value = false
    }
  }

  return {
    tasks,
    codeSources,
    models,
    loading,
    currentTask,
    taskCount,
    pendingTasks,
    runningTasks,
    completedTasks,
    loadTasks,
    loadTask,
    createTask,
    updateTask,
    deleteTask,
    startTask,
    stopTask,
    exportReport,
    updateProgress,
    loadCodeSources,
    uploadCodeSource,
    uploadZip,
    uploadJar,
    addGitRepo,
    addLocalPath,
    testModel,
    deleteCodeSource,
    loadModels,
    createModel,
    updateModel,
    deleteModel
  }
})

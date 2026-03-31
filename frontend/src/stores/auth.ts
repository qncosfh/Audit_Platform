import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { authApi, secureStorage } from '@/api'
import type { User, LoginRequest, RegisterRequest } from '@/types'

export const useAuthStore = defineStore('auth', () => {
  const user = ref<User | null>(null)
  const token = ref<string | null>(secureStorage.getToken())
  const loading = ref(false)

  const isAuthenticated = computed(() => !!token.value)

  const login = async (credentials: LoginRequest) => {
    loading.value = true
    try {
      const response = await authApi.login(credentials)
      const { token: newToken, user: userData } = response.data.data || response.data.Data
      
      token.value = newToken
      user.value = userData
      // 使用安全的Token存储
      secureStorage.setToken(newToken, 24)
      
      return { success: true }
    } catch (error) {
      return { success: false, error }
    } finally {
      loading.value = false
    }
  }

  const register = async (userData: RegisterRequest) => {
    loading.value = true
    try {
      const response = await authApi.register(userData)
      const { token: newToken, user: newUser } = response.data.data || response.data.Data
      
      token.value = newToken
      user.value = newUser
      // 使用安全的Token存储
      secureStorage.setToken(newToken, 24)
      
      return { success: true }
    } catch (error) {
      return { success: false, error }
    } finally {
      loading.value = false
    }
  }

  const logout = async () => {
    try {
      await authApi.logout()
    } catch (error) {
      console.error('Logout error:', error)
    } finally {
      token.value = null
      user.value = null
      // 使用安全的Token清理
      secureStorage.clearToken()
    }
  }

  const getCurrentUser = async () => {
    if (!token.value) return null
    
    try {
      const response = await authApi.getCurrentUser()
      user.value = response.data.data || response.data.Data
      return user.value
    } catch (error) {
      logout()
      return null
    }
  }

  const checkAuth = async () => {
    if (token.value) {
      await getCurrentUser()
    }
  }

  return {
    user,
    token,
    loading,
    isAuthenticated,
    login,
    register,
    logout,
    getCurrentUser,
    checkAuth
  }
})

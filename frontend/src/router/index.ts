import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/',
      redirect: '/dashboard'
    },
    {
      path: '/login',
      name: 'login',
      component: () => import('@/pages/Login.vue'),
      meta: { requiresGuest: true }
    },
    {
      path: '/register',
      name: 'register',
      component: () => import('@/pages/Register.vue'),
      meta: { requiresGuest: true }
    },
    {
      path: '/dashboard',
      name: 'dashboard',
      component: () => import('@/pages/Dashboard.vue'),
      meta: { requiresAuth: true }
    },
    {
      path: '/tasks',
      name: 'tasks',
      component: () => import('@/pages/Tasks.vue'),
      meta: { requiresAuth: true }
    },
    {
      path: '/tasks/create',
      name: 'create-task',
      component: () => import('@/pages/CreateTask.vue'),
      meta: { requiresAuth: true }
    },
    {
      path: '/tasks/:id',
      name: 'task-detail',
      component: () => import('@/pages/TaskDetail.vue'),
      meta: { requiresAuth: true }
    },
    {
      path: '/code-sources',
      name: 'code-sources',
      component: () => import('@/pages/CodeSources.vue'),
      meta: { requiresAuth: true }
    },
    {
      path: '/code-sources/:id',
      name: 'code-source-detail',
      component: () => import('@/pages/CodeSourceDetail.vue'),
      meta: { requiresAuth: true }
    },
    {
      path: '/models',
      name: 'models',
      component: () => import('@/pages/Models.vue'),
      meta: { requiresAuth: true }
    },
    {
      path: '/reports',
      name: 'reports',
      component: () => import('@/pages/Reports.vue'),
      meta: { requiresAuth: true }
    },
    {
      path: '/:pathMatch(.*)*',
      name: 'not-found',
      component: () => import('@/pages/NotFound.vue'),
      meta: { hideLayout: true }
    }
  ]
})

// 路由守卫
let isLoggingOut = false

router.beforeEach(async (to, from, next) => {
  const authStore = useAuthStore()
  
  // 如果正在退出登录，跳过认证检查
  if (isLoggingOut) {
    next()
    return
  }
  
  // 检查是否需要认证
  if (to.meta.requiresAuth) {
    // 如果没有 token，直接跳转到登录页
    if (!authStore.token) {
      next({ path: '/login', replace: true })
      return
    }
    
    // 有 token 但未验证过，尝试验证
    if (!authStore.user) {
      await authStore.checkAuth()
    }
    
    // 验证后仍然没有用户信息，说明 token 无效
    if (!authStore.user) {
      next({ path: '/login', replace: true })
      return
    }
  }
  
  // 检查是否需要游客状态（已登录用户不能访问登录注册页）
  if (to.meta.requiresGuest) {
    if (authStore.isAuthenticated) {
      next({ path: '/dashboard', replace: true })
      return
    }
  }
  
  next()
})

// 标记退出登录状态
export const setLoggingOut = (value: boolean) => {
  isLoggingOut = value
}

export default router
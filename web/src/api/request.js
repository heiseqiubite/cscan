import axios from 'axios'
import { ElMessage } from 'element-plus'
import { useUserStore } from '@/stores/user'
import { useWorkspaceStore } from '@/stores/workspace'
import router from '@/router'

const request = axios.create({
  baseURL: '/api/v1',
  timeout: 30000
})

// 请求拦截器
request.interceptors.request.use(
  config => {
    const userStore = useUserStore()
    const workspaceStore = useWorkspaceStore()
    if (userStore.token) {
      config.headers['Authorization'] = `Bearer ${userStore.token}`
    }
    
    // 设置工作空间ID header
    // 确保工作空间ID的正确处理
    const currentWsId = workspaceStore.currentWorkspaceId
    if (!currentWsId || currentWsId === 'undefined' || currentWsId === 'null' || currentWsId === 'all') {
      config.headers['X-Workspace-Id'] = 'all'
    } else {
      config.headers['X-Workspace-Id'] = currentWsId
    }
    return config
  },
  error => {
    return Promise.reject(error)
  }
)

// 响应拦截器
request.interceptors.response.use(
  response => {
    const res = response.data
    if (res.code === 401) {
      const userStore = useUserStore()
      userStore.logout()
      router.push('/login')
      ElMessage.error('登录已过期，请重新登录')
      return Promise.reject(new Error('Unauthorized'))
    }
    return res
  },
  error => {
    if (error.response && error.response.status === 401) {
      const userStore = useUserStore()
      userStore.logout()
      router.push('/login')
      ElMessage.error('登录已过期，请重新登录')
    } else {
      // 优化网络错误和后端未启动时的弹窗提示
      const errorMsg = error.message || '请求失败'
      if (errorMsg === 'Network Error' || error.code === 'ECONNABORTED' || errorMsg.includes('timeout')) {
        ElMessage({
          message: '网络错误或后端服务未启动，请检查连接',
          type: 'error',
          grouping: true
        })
      } else {
        ElMessage({
          message: errorMsg,
          type: 'error',
          grouping: true
        })
      }
    }
    return Promise.reject(error)
  }
)

export default request

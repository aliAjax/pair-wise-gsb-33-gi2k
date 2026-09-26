import axios from 'axios'
import { ElMessage } from 'element-plus'
import { useAuthStore } from '@/stores/authStore'

const request = axios.create({
  baseURL: '/api/v1',
  timeout: 15000,
})

// 业务错误：除了弹提示，还把业务码挂在 error.bizCode 上，
// 页面可用它做差异化处理（例如修订冲突需引导重新合并）。
export class BizError extends Error {
  bizCode: number
  constructor(code: number, message: string) {
    super(message)
    this.name = 'BizError'
    this.bizCode = code
  }
}

request.interceptors.request.use((config) => {
  const auth = useAuthStore()
  if (auth.token) {
    config.headers.Authorization = `Bearer ${auth.token}`
  }
  return config
})

request.interceptors.response.use(
  (res) => {
    const body = res.data
    if (body && typeof body.code === 'number' && body.code !== 0) {
      ElMessage.error(body.message || '请求失败')
      return Promise.reject(new BizError(body.code, body.message))
    }
    return body?.data
  },
  (err) => {
    const status = err.response?.status
    const body = err.response?.data
    const msg = body?.message || '网络异常'
    if (status === 401) {
      const auth = useAuthStore()
      auth.logout()
    }
    ElMessage.error(msg)
    return Promise.reject(new BizError(body?.code ?? status ?? -1, msg))
  },
)

export default request

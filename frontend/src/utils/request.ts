import axios, { type AxiosError } from 'axios'
import { ElMessage } from 'element-plus'
import { useAuthStore } from '@/stores/authStore'

// Business code for a stale page revision: the page handles re-merging
// itself, so the global interceptor must not toast it.
const SILENT_CODES = new Set<number>([40901])

const request = axios.create({
  baseURL: '/api/v1',
  timeout: 15000,
})

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
      if (!SILENT_CODES.has(body.code)) {
        ElMessage.error(body.message || '请求失败')
      }
      return Promise.reject(new ApiBusinessError(body.message, body.code, body.details, body.data))
    }
    return body?.data
  },
  (err: AxiosError<{ code?: number; message?: string; details?: unknown }>) => {
    const status = err.response?.status
    const body = err.response?.data
    const code = body?.code
    const msg = body?.message || '网络异常'
    if (status === 401) {
      const auth = useAuthStore()
      auth.logout()
    }
    if (code !== undefined && SILENT_CODES.has(code)) {
      return Promise.reject(new ApiBusinessError(msg, code, body?.details))
    }
    ElMessage.error(msg)
    return Promise.reject(new ApiBusinessError(msg, code, body?.details))
  },
)

// ApiBusinessError carries the business code and structured details from
// the unified error envelope so pages can branch on revision conflicts.
export class ApiBusinessError extends Error {
  code?: number
  details?: unknown
  data?: unknown

  constructor(message: string, code?: number, details?: unknown, data?: unknown) {
    super(message)
    this.name = 'ApiBusinessError'
    this.code = code
    this.details = details
    this.data = data
  }
}

export default request

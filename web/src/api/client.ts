import axios from 'axios'
import router from '@/router'

export const http = axios.create({ baseURL: '/api', timeout: 60000 })

http.interceptors.request.use((config) => {
  const token = localStorage.getItem('aidash_token')
  if (token) config.headers.Authorization = `Bearer ${token}`
  return config
})

http.interceptors.response.use(
  (res) => {
    const body = res.data
    if (body && typeof body === 'object' && 'data' in body) return body.data
    return body
  },
  (error) => {
    const status = error?.response?.status
    const msg = error?.response?.data?.error || error?.message || '请求失败'
    if (status === 401) {
      localStorage.removeItem('aidash_token')
      localStorage.removeItem('aidash_user')
      if (router.currentRoute.value.name !== 'login') {
        router.push({ name: 'login' })
      }
    }
    return Promise.reject(new Error(msg))
  },
)

export async function upload(url: string, data: unknown): Promise<any> {
  const res = await http.post(url, data)
  return res
}

import axios from 'axios'
import type { ApiEnvelope } from '@/types/auth'

const http = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL ?? '',
  timeout: 15000,
})

http.interceptors.request.use((config) => {
  const token = localStorage.getItem('cabinet_access_token')
  if (token) config.headers.Authorization = `Bearer ${token}`
  return config
})

http.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      localStorage.removeItem('cabinet_access_token')
      window.dispatchEvent(new Event('cabinet:unauthorized'))
    }
    return Promise.reject(error)
  },
)

export async function request<T>(config: Parameters<typeof http.request>[0]): Promise<T> {
  const response = await http.request<ApiEnvelope<T>>(config)
  return response.data.data
}

export default http

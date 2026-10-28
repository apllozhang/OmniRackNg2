import http, { request } from './http'
import type { LoginResult, User } from '@/types/auth'

export function login(username: string, password: string) {
  return request<LoginResult>({ method: 'POST', url: '/api/v1/auth/login', data: { username, password } })
}

export function currentUser() {
  return request<User>({ method: 'GET', url: '/api/v1/auth/me' })
}

export function logout() {
  return http.post('/api/v1/auth/logout')
}

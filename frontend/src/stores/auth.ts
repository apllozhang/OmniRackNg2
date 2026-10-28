import { defineStore } from 'pinia'
import { currentUser, login, logout } from '@/api/auth'
import type { User } from '@/types/auth'

const TOKEN_KEY = 'cabinet_access_token'

export const useAuthStore = defineStore('auth', {
  state: () => ({
    token: localStorage.getItem(TOKEN_KEY) ?? '',
    user: null as User | null,
    loading: false,
  }),
  getters: {
    isAuthenticated: (state) => Boolean(state.token),
    isAdmin: (state) => Boolean(state.user?.roles?.some((role) => role.code === 'system_admin')),
  },
  actions: {
    async signIn(username: string, password: string) {
      this.loading = true
      try {
        const result = await login(username, password)
        this.token = result.token
        this.user = result.user
        localStorage.setItem(TOKEN_KEY, result.token)
      } finally {
        this.loading = false
      }
    },
    async loadUser() {
      if (!this.token) return
      try {
        this.user = await currentUser()
      } catch (error: any) {
        if (error?.response?.status === 401) this.clearSession()
        else throw error
      }
    },
    clearSession() {
      this.token = ''
      this.user = null
      localStorage.removeItem(TOKEN_KEY)
    },
    async signOut() {
      try {
        if (this.token) await logout()
      } finally {
        this.clearSession()
      }
    },
  },
})

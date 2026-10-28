import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { currentUser, logout } from '@/api/auth'
import { useAuthStore } from './auth'

vi.mock('@/api/auth', () => ({ currentUser: vi.fn(), login: vi.fn(), logout: vi.fn() }))
beforeEach(() => {
  vi.resetAllMocks()
  localStorage.setItem('cabinet_access_token', 'existing-token')
  setActivePinia(createPinia())
})
describe('session refresh failures', () => {
  it.each([{ response: { status: 503 } }, new Error('offline')])('retains credentials on temporary failure', async (failure) => {
    vi.mocked(currentUser).mockRejectedValue(failure)
    const auth = useAuthStore()
    await expect(auth.loadUser()).rejects.toBe(failure)
    expect(auth.token).toBe('existing-token')
    expect(localStorage.getItem('cabinet_access_token')).toBe('existing-token')
    expect(logout).not.toHaveBeenCalled()
  })
  it('clears an invalid session locally without revoking other sessions', async () => {
    vi.mocked(currentUser).mockRejectedValue({ response: { status: 401 } })
    const auth = useAuthStore()
    await auth.loadUser()
    expect(auth.token).toBe('')
    expect(localStorage.getItem('cabinet_access_token')).toBeNull()
    expect(logout).not.toHaveBeenCalled()
  })
  it('clears local state even when explicit logout is unavailable', async () => {
    vi.mocked(logout).mockRejectedValue(new Error('offline'))
    const auth = useAuthStore()
    await expect(auth.signOut()).rejects.toThrow('offline')
    expect(auth.token).toBe('')
    expect(localStorage.getItem('cabinet_access_token')).toBeNull()
  })
})

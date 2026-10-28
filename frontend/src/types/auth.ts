export interface Role {
  id: string
  code: string
  name: string
}

export interface User {
  id: string
  username: string
  displayName: string
  email?: string
  authSource: 'local' | 'ldap'
  enabled: boolean
  lastLoginAt?: string
  roles?: Role[]
}

export interface LoginResult {
  token: string
  user: User
}

export interface ApiEnvelope<T> {
  code: string
  message: string
  data: T
  details?: unknown
  requestId?: string
}

import type { Role, User } from '@/types/auth'
import type { ApprovalRecord, OperationPolicy } from '@/types/device'

export interface ManagedUser extends User { createdAt: string; updatedAt: string; version: number; roles?: Role[] }
export interface UserPayload { username?: string; displayName: string; email?: string; password?: string; authSource: 'local'|'ldap'; enabled?: boolean; roleCodes: string[] }
export interface LDAPGroupRoleMapping { group: string; roleCode: string }
export interface LDAPConfig {
  id?: string; version?: number; enabled: boolean; url: string; useTls: boolean; skipTlsVerify: boolean; bindDn: string; bindPassword?: string; hasBindPassword?: boolean; baseDn: string; userFilter: string; usernameAttribute: string; displayNameAttribute: string; emailAttribute: string; defaultRole: string; allowLocalFallback: boolean; groupSearchBaseDn: string; groupFilter: string; groupAttribute: string; groupRoleMappings: LDAPGroupRoleMapping[]
}
export type { Role, ApprovalRecord, OperationPolicy }

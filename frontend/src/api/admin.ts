import { request } from './http'
import type { LDAPConfig, ManagedUser, UserPayload } from '@/types/admin'
import type { ApprovalRecord, OperationPolicy } from '@/types/device'
import type { Role } from '@/types/auth'

export function listAdminUsers(params?: { search?: string; enabled?: boolean; authSource?: string }) { return request<{ items: ManagedUser[] }>({ method:'GET', url:'/api/v1/admin/users', params }) }
export function listAdminRoles() { return request<{ items: Role[] }>({ method:'GET', url:'/api/v1/admin/roles' }) }
export function createAdminUser(data: UserPayload) { return request<ManagedUser>({ method:'POST', url:'/api/v1/admin/users', data }) }
export function updateAdminUser(id: string, version: number, data: UserPayload) { return request<ManagedUser>({ method:'PUT', url:`/api/v1/admin/users/${id}`, params:{ version }, data }) }
export function resetAdminPassword(id: string, version: number, password: string) { return request<ManagedUser>({ method:'POST', url:`/api/v1/admin/users/${id}/reset-password`, params:{ version }, data:{ password } }) }
export function deleteAdminUser(id: string, version: number) { return request<{ deleted:boolean; id:string }>({ method:'DELETE', url:`/api/v1/admin/users/${id}`, params:{ version } }) }
export function getAdminApprovalPolicy() { return request<OperationPolicy>({ method:'GET', url:'/api/v1/admin/approval-policy' }) }
export function updateAdminApprovalPolicy(data: Pick<OperationPolicy,'version'|'assignApprovalEnabled'|'moveApprovalEnabled'|'decommissionApprovalEnabled'|'defaultApproverRole'>) { return request<OperationPolicy>({ method:'PUT', url:'/api/v1/admin/approval-policy', data }) }
export function listAdminApprovals(status?: string) { return request<{ items: ApprovalRecord[] }>({ method:'GET', url:'/api/v1/admin/approvals', params:{ status } }) }
export function approveAdminOperation(id:string, version:number, comment='') { return request<ApprovalRecord>({ method:'POST', url:`/api/v1/admin/approvals/${id}/approve`, data:{version,comment} }) }
export function rejectAdminOperation(id:string, version:number, comment='') { return request<ApprovalRecord>({ method:'POST', url:`/api/v1/admin/approvals/${id}/reject`, data:{version,comment} }) }
export function getLDAPConfig() { return request<LDAPConfig>({ method:'GET', url:'/api/v1/admin/ldap' }) }
export function updateLDAPConfig(data: LDAPConfig) { return request<LDAPConfig>({ method:'PUT', url:'/api/v1/admin/ldap', data }) }
export function testLDAPConfig(data: LDAPConfig) { return request<{ connected:boolean; message:string }>({ method:'POST', url:'/api/v1/admin/ldap/test', data }) }

import { request } from './http'
import type {
  ApprovalRecord,
  DecommissionPayload,
  Device,
  DeviceCreatePayload,
  DeviceListResult,
  DevicePayload,
  DeviceType,
  DeviceTypePayload,
  OperationPolicy,
  OperationResult,
  PositionChangePayload,
  PositionHistory,
  RackULayout,
} from '@/types/device'
import type {
  RackDiagramImportCommitPayload,
  RackDiagramImportCommitResult,
  RackDiagramImportValidatePayload,
  RackDiagramImportValidation,
} from '@/types/rackDiagramImport'

export interface DeviceQuery {
  sortBy?: string
  sortOrder?: 'asc' | 'desc'
	scope?: 'PC' | 'ROOM'
  page?: number
  pageSize?: number
  search?: string
  typeId?: string
  lifecycleStatus?: string
  rackId?: string
}

export function listDeviceTypes() {
  return request<{ items: DeviceType[] }>({ method: 'GET', url: '/api/v1/device-types' })
}
export function createDeviceType(data: DeviceTypePayload) {
  return request<DeviceType>({ method: 'POST', url: '/api/v1/device-types', data })
}
export function updateDeviceType(id: string, data: DeviceTypePayload) {
  return request<DeviceType>({ method: 'PUT', url: `/api/v1/device-types/${id}`, data })
}
export function deleteDeviceType(id: string, version: number) {
  return request<{ deleted: boolean; id: string }>({ method: 'DELETE', url: `/api/v1/device-types/${id}`, params: { version } })
}

export function listDevices(params: DeviceQuery) {
  return request<DeviceListResult>({ method: 'GET', url: '/api/v1/devices', params })
}
export function getDevice(id: string) {
  return request<Device>({ method: 'GET', url: `/api/v1/devices/${id}` })
}
export function createDevice(data: DeviceCreatePayload) {
  return request<Device>({ method: 'POST', url: '/api/v1/devices', data })
}
export function updateDevice(id: string, data: DevicePayload) {
  return request<Device>({ method: 'PUT', url: `/api/v1/devices/${id}`, data })
}
export function deleteDevice(id: string, version: number) {
  return request<{ deleted: boolean; id: string }>({ method: 'DELETE', url: `/api/v1/devices/${id}`, params: { version } })
}
export function assignDevice(id: string, data: PositionChangePayload) {
  return request<OperationResult>({ method: 'POST', url: `/api/v1/devices/${id}/assign`, data })
}
export function moveDevice(id: string, data: PositionChangePayload) {
  return request<OperationResult>({ method: 'POST', url: `/api/v1/devices/${id}/move`, data })
}
export function decommissionDevice(id: string, data: DecommissionPayload) {
  return request<OperationResult>({ method: 'POST', url: `/api/v1/devices/${id}/decommission`, data })
}
export function listDeviceHistory(id: string) {
  return request<{ items: PositionHistory[] }>({ method: 'GET', url: `/api/v1/devices/${id}/history` })
}
export function getRackLayout(rackId: string) {
  return request<RackULayout>({ method: 'GET', url: `/api/v1/racks/${rackId}/u-layout` })
}

export function validateRackDiagramImport(roomId: string, data: RackDiagramImportValidatePayload) {
  return request<RackDiagramImportValidation>({
    method: 'POST',
    url: `/api/v1/rooms/${roomId}/rack-diagram-import/validate`,
    data,
  })
}

export function commitRackDiagramImport(roomId: string, data: RackDiagramImportCommitPayload) {
  return request<RackDiagramImportCommitResult>({
    method: 'POST',
    url: `/api/v1/rooms/${roomId}/rack-diagram-import/commit`,
    data,
  })
}

export function getOperationPolicy() {
  return request<OperationPolicy>({ method: 'GET', url: '/api/v1/settings/operation-policy' })
}
export function updateOperationPolicy(data: Pick<OperationPolicy, 'version' | 'assignApprovalEnabled' | 'moveApprovalEnabled' | 'defaultApproverRole'>) {
  return request<OperationPolicy>({ method: 'PUT', url: '/api/v1/settings/operation-policy', data })
}
export function listApprovals(status?: string) {
  return request<{ items: ApprovalRecord[] }>({ method: 'GET', url: '/api/v1/approvals', params: { status } })
}
export function approveOperation(id: string, version: number, comment = '') {
  return request<ApprovalRecord>({ method: 'POST', url: `/api/v1/approvals/${id}/approve`, data: { version, comment } })
}
export function rejectOperation(id: string, version: number, comment = '') {
  return request<ApprovalRecord>({ method: 'POST', url: `/api/v1/approvals/${id}/reject`, data: { version, comment } })
}

export interface TerminalRegistration {
  status: 'IDLE' | 'IN_USE' | 'REPAIR' | 'RETIRED'
  assignee: string
  department: string
  siteId: string | null
  position: string
}
export function createTerminal(data: DeviceCreatePayload & { terminal: TerminalRegistration }) {
  return request<Device>({ method: 'POST', url: '/api/v1/asset-ops/terminal-devices', data })
}
export function getTerminalState(id: string) {
  return request<TerminalRegistration>({ method: 'GET', url: `/api/v1/asset-ops/devices/${id}/terminal-state` })
}
export function listTerminalSites() {
  return request<{ items: Array<{ id: string; name: string; region: string }> }>({ method: 'GET', url: '/api/v1/asset-ops/sites' })
}

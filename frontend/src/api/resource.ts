import { request } from './http'
import type { DataCenter, DataCenterCreatePayload, DataCenterPayload, Rack, RackCreatePayload, RackPayload, ResourceCopyPayload, ResourceCopyResult, ResourceMovePayload, ResourceTreeResponse, Room, RoomCreatePayload, RoomPayload } from '@/types/resource'

export function getResourceTree() {
  return request<ResourceTreeResponse>({ method: 'GET', url: '/api/v1/resource-tree' })
}

export function createDataCenter(data: DataCenterCreatePayload) {
  return request<DataCenter>({ method: 'POST', url: '/api/v1/data-centers', data })
}

export function updateDataCenter(id: string, data: DataCenterPayload) {
  return request<DataCenter>({ method: 'PUT', url: `/api/v1/data-centers/${id}`, data })
}

export function deleteDataCenter(id: string, version: number) {
  return request<{ deleted: boolean; id: string }>({ method: 'DELETE', url: `/api/v1/data-centers/${id}`, params: { version } })
}

export function createRoom(dataCenterId: string, data: RoomCreatePayload) {
  return request<Room>({ method: 'POST', url: `/api/v1/data-centers/${dataCenterId}/rooms`, data })
}

export function updateRoom(id: string, data: RoomPayload) {
  return request<Room>({ method: 'PUT', url: `/api/v1/rooms/${id}`, data })
}

export function deleteRoom(id: string, version: number) {
  return request<{ deleted: boolean; id: string }>({ method: 'DELETE', url: `/api/v1/rooms/${id}`, params: { version } })
}

export function createRack(roomId: string, data: RackCreatePayload) {
  return request<Rack>({ method: 'POST', url: `/api/v1/rooms/${roomId}/racks`, data })
}

export function updateRack(id: string, data: RackPayload) {
  return request<Rack>({ method: 'PUT', url: `/api/v1/racks/${id}`, data })
}

export function deleteRack(id: string, version: number) {
  return request<{ deleted: boolean; id: string }>({ method: 'DELETE', url: `/api/v1/racks/${id}`, params: { version } })
}


export function copyDataCenter(id: string, data: ResourceCopyPayload) {
  return request<ResourceCopyResult<DataCenter>>({ method: 'POST', url: `/api/v1/data-centers/${id}/copy`, data })
}

export function copyRoom(id: string, data: ResourceCopyPayload) {
  return request<ResourceCopyResult<Room>>({ method: 'POST', url: `/api/v1/rooms/${id}/copy`, data })
}

export function moveRoom(id: string, data: ResourceMovePayload) {
  return request<Room>({ method: 'POST', url: `/api/v1/rooms/${id}/move`, data })
}

export function copyRack(id: string, data: ResourceCopyPayload) {
  return request<ResourceCopyResult<Rack>>({ method: 'POST', url: `/api/v1/racks/${id}/copy`, data })
}

export function moveRack(id: string, data: ResourceMovePayload) {
  return request<Rack>({ method: 'POST', url: `/api/v1/racks/${id}/move`, data })
}

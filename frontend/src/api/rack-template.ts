import { request } from './http'
import type { RackTemplate, RackTemplateCreatePayload, RackTemplateSpecPayload, RackTemplateUpdatePayload, RackTemplateVersion } from '@/types/rack-template'

export function getRackTemplates() {
  return request<{ items: RackTemplate[] }>({ method: 'GET', url: '/api/v1/rack-templates' })
}

export function createRackTemplate(data: RackTemplateCreatePayload) {
  return request<RackTemplate>({ method: 'POST', url: '/api/v1/rack-templates', data })
}

export function updateRackTemplate(id: string, data: RackTemplateUpdatePayload) {
  return request<RackTemplate>({ method: 'PUT', url: `/api/v1/rack-templates/${id}`, data })
}

export function createRackTemplateVersion(id: string, version: number, data: RackTemplateSpecPayload) {
  return request<RackTemplateVersion>({ method: 'POST', url: `/api/v1/rack-templates/${id}/versions`, params: { version }, data })
}

export function deleteRackTemplate(id: string, version: number) {
  return request<{ deleted: boolean; id: string }>({ method: 'DELETE', url: `/api/v1/rack-templates/${id}`, params: { version } })
}
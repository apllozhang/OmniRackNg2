import { request } from './http'
import type { PDU, PDUSocket, PDUConnection, PDUCreatePayload, PDUSocketPayload, PDUConnectionPayload } from '@/types/pdu'

export function listPDUs(rackId: string) {
  return request<{ items: PDU[] }>({ method: 'GET', url: `/api/v1/racks/${rackId}/pdus` }).then((response) => response.items || [])
}
export function createPDU(rackId: string, data: PDUCreatePayload) {
  return request<PDU>({ method: 'POST', url: `/api/v1/racks/${rackId}/pdus`, data })
}
export function updatePDU(id: string, data: PDU) {
  return request<PDU>({ method: 'PUT', url: `/api/v1/pdus/${id}`, data })
}
export function deletePDU(id: string, version: number) {
  return request<{ deleted: boolean; id: string }>({ method: 'DELETE', url: `/api/v1/pdus/${id}`, params: { version } })
}
export function listPDUSockets(pduId: string) {
  return request<{ items: PDUSocket[] }>({ method: 'GET', url: `/api/v1/pdus/${pduId}/sockets` }).then((response) => response.items || [])
}
export function createPDUSocket(pduId: string, data: PDUSocketPayload) {
  return request<PDUSocket>({ method: 'POST', url: `/api/v1/pdus/${pduId}/sockets`, data })
}
export function updatePDUSocket(id: string, data: PDUSocket) {
  return request<PDUSocket>({ method: 'PUT', url: `/api/v1/pdu-sockets/${id}`, data })
}
export function deletePDUSocket(id: string, version: number) {
  return request<{ deleted: boolean; id: string }>({ method: 'DELETE', url: `/api/v1/pdu-sockets/${id}`, params: { version } })
}
export function listPDUConnections(rackId: string) {
  return request<{ items: PDUConnection[] }>({ method: 'GET', url: `/api/v1/racks/${rackId}/pdu-connections` }).then((response) => response.items || [])
}
export function connectPDUSocket(socketId: string, data: PDUConnectionPayload) {
  return request<PDUConnection>({ method: 'POST', url: `/api/v1/pdu-sockets/${socketId}/connection`, data })
}
export function disconnectPDUConnection(id: string, version: number) {
  return request<{ deleted: boolean; id: string }>({ method: 'DELETE', url: `/api/v1/pdu-connections/${id}`, data: { version } })
}


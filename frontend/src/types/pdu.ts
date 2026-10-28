import type { BaseModel, Device } from './device'

export type PDUStatus = 'ACTIVE' | 'DISABLED'
export type PDUSocketStatus = 'AVAILABLE' | 'CONNECTED' | 'DISABLED'
export type PDUSocketStandard = 'CN' | 'EU'
export type PDUConnectionRole = 'PRIMARY' | 'REDUNDANT'

export interface PDU extends BaseModel {
  rackId: string
  code: string
  name: string
  manufacturer?: string
  modelNumber?: string
  serialNumber?: string
  inputVoltage?: number
  ratedPowerW?: number
  ratedCurrentA?: number
  status: PDUStatus
  remarks?: string
  sockets?: PDUSocket[]
}
export interface PDUSocket extends BaseModel {
  pduId: string
  socketNo: number
  standard: PDUSocketStandard
  amperageA: 10 | 16
  label?: string
  status: PDUSocketStatus
  connection?: PDUConnection
}
export interface PDUConnection extends BaseModel {
  socketId: string
  deviceId: string
  powerW?: number
  circuit?: string
  redundancyRole: PDUConnectionRole
  connectedAt: string
  connectedBy?: string
  socket?: PDUSocket
  device?: Device
}
export type PDUCreatePayload = Omit<PDU, 'id' | 'createdAt' | 'updatedAt' | 'version' | 'sockets' | 'rackId'> & { autoGenerateCode?: boolean }
export type PDUSocketPayload = Omit<PDUSocket, 'id' | 'createdAt' | 'updatedAt' | 'version' | 'pduId' | 'connection'>
export interface PDUConnectionPayload {
  version?: number
  deviceId: string
  powerW?: number
  circuit?: string
  redundancyRole: PDUConnectionRole
}
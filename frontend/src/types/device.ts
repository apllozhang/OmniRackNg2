import type { AutoCodeCreateOption } from '@/features/code/autoCode'
import type { Rack } from './resource'

export type DeviceCategory = 'PC' | 'SERVER' | 'NETWORK' | 'STORAGE' | 'SECURITY' | 'POWER_ENVIRONMENT' | 'ACCESSORY' | 'OTHER'
export type DeviceLifecycleStatus = 'WAITING_RACK' | 'RUNNING' | 'MAINTENANCE' | 'PENDING_REMOVAL' | 'OFF_RACK' | 'SCRAPPED'
export type DeviceOrientation = 'NORMAL' | 'REVERSE'
export type PositionOperation = 'ASSIGN' | 'MOVE' | 'DECOMMISSION'
export type ApprovalStatus = 'PENDING' | 'APPROVED' | 'REJECTED' | 'CANCELLED'

export interface BaseModel {
  id: string
  createdAt: string
  updatedAt: string
  version: number
}

export interface DeviceType extends BaseModel {
  code: string
  name: string
  category: DeviceCategory
  status: 'ACTIVE' | 'DISABLED'
  defaultHeightU: number
  defaultWeightKg?: number
  defaultRatedPowerW?: number
  defaultPeakPowerW?: number
  defaultDualPower: boolean
  description?: string
  sortOrder: number
}

export interface RackDevicePosition extends BaseModel {
  deviceId: string
  rackId: string
  roomId: string
  dataCenterId: string
  startU: number
  heightU: number
  endU: number
  orientation: DeviceOrientation
  installedAt: string
  installedBy?: string
  reason?: string
  device?: Device
  rack?: Rack
}

export interface Device extends BaseModel {
  typeId: string
  type?: DeviceType
  code: string
  name: string
  assetNumber?: string
  serialNumber?: string
  manufacturer?: string
  modelNumber?: string
  specification?: string
  firmwareVersion?: string
  purchaseBatch?: string
  warrantyExpiresAt?: string
  organization?: string
  manager?: string
  contact?: string
  businessSystem?: string
  applicationName?: string
  lifecycleStatus: DeviceLifecycleStatus
  heightU: number
  widthMm?: number
  depthMm?: number
  heightMm?: number
  weightKg?: number
  ratedPowerW?: number
  peakPowerW?: number
  inputVoltage?: number
  dualPowerRequired: boolean
  managementIp?: string
  businessIp?: string
  macAddress?: string
  managementProtocol?: string
  monitoringStatus?: string
  externalQrCode?: string
  externalQrCodeUrl?: string
  tags?: string
  remarks?: string
  currentPosition?: RackDevicePosition
}

export type DevicePayload = Omit<Device, 'id' | 'createdAt' | 'updatedAt' | 'type' | 'currentPosition'> & { id?: string }
export type DeviceCreatePayload = DevicePayload & AutoCodeCreateOption
export type DeviceTypePayload = Omit<DeviceType, 'id' | 'createdAt' | 'updatedAt'> & { id?: string }

export interface DeviceListResult {
  items: Device[]
  total: number
  page: number
  pageSize: number
}

export interface PositionChangePayload {
  version: number
  rackId: string
  startU: number
  heightU: number
  orientation: DeviceOrientation
  reason?: string
}

export interface DecommissionPayload {
  version: number
  reason?: string
}

export interface OperationResult {
  executed: boolean
  device?: Device
  position?: RackDevicePosition
  approval?: ApprovalRecord
}

export interface PositionSnapshot {
  positionId?: string
  rackId?: string
  dataCenterId?: string
  roomId?: string
  startU?: number
  heightU?: number
  endU?: number
  orientation?: DeviceOrientation
}

export interface PositionHistory extends BaseModel {
  deviceId: string
  operation: PositionOperation
  fromPosition?: PositionSnapshot
  toPosition?: PositionSnapshot
  reason?: string
  actorId?: string
  requestId?: string
}

export interface FreeRange {
  startU: number
  endU: number
  heightU: number
}

export interface RackULayout {
  rack: Rack
  positions: RackDevicePosition[]
  free: FreeRange[]
}

export interface OperationPolicy extends BaseModel {
  assignApprovalEnabled: boolean
  moveApprovalEnabled: boolean
  decommissionApprovalEnabled: boolean
  approvalMode: 'SINGLE'
  defaultApproverRole: string
}

export interface ApprovalRecord extends BaseModel {
  deviceId: string
  device?: Device
  operation: 'ASSIGN' | 'MOVE' | 'DECOMMISSION'
  status: ApprovalStatus
  requestedBy?: string
  requestedAt: string
  requestedDeviceVersion: number
  sourcePosition?: PositionSnapshot
  targetRackId: string
  targetStartU: number
  targetHeightU: number
  targetOrientation: DeviceOrientation
  reason?: string
  decidedBy?: string
  decidedAt?: string
  decisionComment?: string
}

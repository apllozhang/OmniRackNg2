import type { RackTemplateSnapshot } from './rack-template'
import type { AutoCodeCreateOption } from '@/features/code/autoCode'

export type ResourceStatus = 'PLANNING' | 'OPERATING' | 'MAINTENANCE' | 'DISABLED' | 'ARCHIVED'
export type RackStatus = 'PLANNING' | 'AVAILABLE' | 'PARTIAL' | 'FULL' | 'MAINTENANCE' | 'DISABLED'

export interface BaseResource {
  id: string
  createdAt: string
  updatedAt: string
  version: number
  code: string
  name: string
  status: string
  sortOrder: number
  remarks?: string
}

export interface Rack extends BaseResource {
  templateId?: string
  templateVersionId?: string
  templateCode?: string
  templateName?: string
  templateRevision?: number
  templateSnapshot?: RackTemplateSnapshot
  dataCenterId: string
  roomId: string
  type: string
  manufacturer?: string
  modelNumber?: string
  serialNumber?: string
  assetNumber?: string
  uHeight: number
  widthMm: number
  depthMm: number
  heightMm: number
  loadCapacityKg?: number
  zone?: string
  row?: string
  column?: string
  aisle?: string
  xCoordinate?: number
  yCoordinate?: number
  rotation: number
  manager?: string
  department?: string
  purpose?: string
  dualPower: boolean
  inputCircuits: number
  ratedVoltage?: number
  ratedCurrent?: number
  ratedPowerKw?: number
  peakPowerKw?: number
  pduCount: number
}

export interface Room extends BaseResource {
  dataCenterId: string
  building?: string
  floor?: string
  roomNumber?: string
  areaSquareMeters?: number
  clearHeightMeters?: number
  purpose?: string
  environmentLevel?: string
  maxLoadKg?: number
  coolingCapacityKw?: number
  designPowerKw?: number
  availablePowerKw?: number
  usedPowerKw?: number
  redundancyPolicy?: string
  manager?: string
  contact?: string
  openHours?: string
  accessNotes?: string
  floorPlanEnabled: boolean
  floorPlanFormat: string
  racksPerRow: number
  racks?: Rack[]
}

export interface DataCenter extends BaseResource {
  address?: string
  longitude?: number
  latitude?: number
  manager?: string
  contact?: string
  serviceProvider?: string
  commissionedAt?: string
  rooms?: Room[]
}

export interface ResourceTreeResponse {
  items: DataCenter[]
}

export type DataCenterPayload = Omit<DataCenter, 'id' | 'createdAt' | 'updatedAt' | 'rooms'> & { id?: string }
export type DataCenterCreatePayload = DataCenterPayload & AutoCodeCreateOption
export type RoomPayload = Omit<Room, 'id' | 'createdAt' | 'updatedAt' | 'racks' | 'dataCenterId'> & { id?: string }
export type RoomCreatePayload = RoomPayload & AutoCodeCreateOption
export type RackPayload = Omit<Rack, 'id' | 'createdAt' | 'updatedAt' | 'dataCenterId' | 'roomId' | 'templateId' | 'templateVersionId' | 'templateCode' | 'templateName' | 'templateRevision' | 'templateSnapshot'> & { id?: string; templateVersionId?: string; templateCode?: string; templateName?: string; templateRevision?: number }
export type RackCreatePayload = RackPayload & AutoCodeCreateOption & { overrideTemplateSpec?: boolean }


export interface ResourceCopyPayload {
  version: number
  code?: string
  name?: string
  autoGenerateCode: boolean
  targetDataCenterId?: string
  targetRoomId?: string
}

export interface ResourceMovePayload {
  version: number
  code?: string
  autoGenerateCode: boolean
  targetDataCenterId?: string
  targetRoomId?: string
}

export interface ResourceCopyResult<T extends DataCenter | Room | Rack> {
  resource: T
  copiedRooms: number
  copiedRacks: number
}

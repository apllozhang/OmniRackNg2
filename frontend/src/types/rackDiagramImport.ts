import type { DeviceCategory, DeviceType } from './device'

export type RackDiagramImportItemKind = 'UNCHANGED' | 'CREATE_NEW' | 'MOVE_EXISTING' | 'NEEDS_DECISION' | 'REMOVE_MISSING' | 'ERROR'
export type RackDiagramImportDecision = 'UPDATE_EXISTING' | 'CREATE_NEW' | 'DECOMMISSION' | 'IGNORE'

export interface RackDiagramImportDeviceInput {
  clientId: string
  rackId: string
  rackCode: string
  startU: number
  endU: number
  name: string
  serialNumber?: string
  ratedPowerW?: number
  typeId?: string
  typeCategory?: DeviceCategory
  sourceDeviceId?: string
  sourceDeviceVersion?: number
  sourceDeviceCode?: string
  sourceRackId?: string
  sourceStartU?: number
  sourceEndU?: number
}

export interface RackDiagramImportValidatePayload {
  formatVersion: string
  dataCenterId: string
  roomId: string
  exportedAt?: string
  defaultTypeId: string
  coveredRackIds: string[]
  devices: RackDiagramImportDeviceInput[]
}

export interface RackDiagramImportItem {
  id: string
  kind: RackDiagramImportItemKind
  severity: 'info' | 'warning' | 'error'
  message: string
  rackId?: string
  rackCode?: string
  startU?: number
  endU?: number
  name: string
  serialNumber?: string
  ratedPowerW?: number
  sourceDeviceId?: string
  existingDeviceId?: string
  existingDeviceCode?: string
  existingDeviceName?: string
  requiresDecision: boolean
  allowedDecisions?: RackDiagramImportDecision[]
  defaultDecision?: RackDiagramImportDecision
}

export interface RackDiagramImportValidation {
  token: string
  expiresAt: string
  summary: { total: number; unchanged: number; create: number; move: number; decisions: number; removals: number; errors: number }
  items: RackDiagramImportItem[]
}

export interface RackDiagramImportCommitPayload {
  token: string
  decisions: Array<{ itemId: string; action: RackDiagramImportDecision }>
}

export interface RackDiagramImportCommitResult {
  created: number
  updated: number
  moved: number
  decommissioned: number
  unchanged: number
}

export interface ParsedRackDiagramImport {
  payload: Omit<RackDiagramImportValidatePayload, 'defaultTypeId'>
  fileName: string
}

export interface RackDiagramImportDialogContext {
  roomId: string
  dataCenterId: string
  deviceTypes: DeviceType[]
}

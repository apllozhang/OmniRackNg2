export interface RackTemplateSnapshot {
  templateId: string
  versionId: string
  code: string
  name: string
  revision: number
  type: string
  manufacturer?: string
  modelNumber?: string
  uHeight: number
  widthMm: number
  depthMm: number
  heightMm: number
  loadCapacityKg?: number
  dualPower: boolean
  inputCircuits: number
  ratedVoltage?: number
  ratedCurrent?: number
  ratedPowerKw?: number
  peakPowerKw?: number
  pduCount: number
}

export type RackTemplateStatus = 'ACTIVE' | 'DISABLED'

export interface RackTemplateVersion {
  id: string
  templateId: string
  revision: number
  version: number
  createdAt: string
  updatedAt: string
  type: string
  manufacturer?: string
  modelNumber?: string
  uHeight: number
  widthMm: number
  depthMm: number
  heightMm: number
  loadCapacityKg?: number
  dualPower: boolean
  inputCircuits: number
  ratedVoltage?: number
  ratedCurrent?: number
  ratedPowerKw?: number
  peakPowerKw?: number
  pduCount: number
  changeNote?: string
}

export interface RackTemplate {
  id: string
  code: string
  name: string
  description?: string
  status: RackTemplateStatus
  isSystem: boolean
  currentRevision: number
  remarks?: string
  version: number
  createdAt: string
  updatedAt: string
  versions: RackTemplateVersion[]
}

export interface RackTemplateSpecPayload {
  type: string
  manufacturer?: string
  modelNumber?: string
  uHeight: number
  widthMm: number
  depthMm: number
  heightMm: number
  loadCapacityKg?: number
  dualPower: boolean
  inputCircuits: number
  ratedVoltage?: number
  ratedCurrent?: number
  ratedPowerKw?: number
  peakPowerKw?: number
  pduCount: number
  changeNote?: string
}

export interface RackTemplateCreatePayload {
  code: string
  autoGenerateCode?: boolean
  name: string
  description?: string
  status: RackTemplateStatus
  remarks?: string
  spec: RackTemplateSpecPayload
}

export interface RackTemplateUpdatePayload {
  version: number
  code: string
  name: string
  description?: string
  status: RackTemplateStatus
  remarks?: string
}
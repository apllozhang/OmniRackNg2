import type { Rack, RackPayload } from '@/types/resource'
import type { RackTemplate, RackTemplateSpecPayload, RackTemplateVersion } from '@/types/rack-template'

/**
 * Single source of truth for cabinet defaults used by forms, templates and imports.
 * Keep these values aligned with backend/internal/model/rack_defaults.go.
 */
export const RACK_DEFAULTS = {
  status: 'AVAILABLE',
  type: 'STANDARD',
  uHeight: 42,
  widthMm: 600,
  depthMm: 1200,
  heightMm: 2000,
  rotation: 0,
  dualPower: false,
  inputCircuits: 0,
  pduCount: 0,
  sortOrder: 0,
} as const

export const RACK_STATUS_VALUES = ['PLANNING', 'AVAILABLE', 'PARTIAL', 'FULL', 'MAINTENANCE', 'DISABLED'] as const

export const RACK_TEMPLATE_SPEC_KEYS = [
  'type', 'manufacturer', 'modelNumber', 'uHeight', 'widthMm', 'depthMm', 'heightMm',
  'loadCapacityKg', 'dualPower', 'inputCircuits', 'ratedVoltage', 'ratedCurrent',
  'ratedPowerKw', 'peakPowerKw', 'pduCount',
] as const

type RackTemplateSpec = Pick<RackTemplateVersion, typeof RACK_TEMPLATE_SPEC_KEYS[number]>

export function createRackPayload(overrides: Partial<RackPayload> = {}): RackPayload {
  return {
    templateVersionId: undefined,
    code: '',
    name: '',
    version: 0,
    status: RACK_DEFAULTS.status,
    sortOrder: RACK_DEFAULTS.sortOrder,
    type: RACK_DEFAULTS.type,
    uHeight: RACK_DEFAULTS.uHeight,
    widthMm: RACK_DEFAULTS.widthMm,
    depthMm: RACK_DEFAULTS.depthMm,
    heightMm: RACK_DEFAULTS.heightMm,
    rotation: RACK_DEFAULTS.rotation,
    dualPower: RACK_DEFAULTS.dualPower,
    inputCircuits: RACK_DEFAULTS.inputCircuits,
    pduCount: RACK_DEFAULTS.pduCount,
    manufacturer: '',
    modelNumber: '',
    serialNumber: '',
    assetNumber: '',
    zone: '',
    row: '',
    column: '',
    aisle: '',
    manager: '',
    department: '',
    purpose: '',
    remarks: '',
    ...overrides,
  }
}

export function createRackTemplateSpec(overrides: Partial<RackTemplateSpecPayload> = {}): RackTemplateSpecPayload {
  return {
    type: RACK_DEFAULTS.type,
    manufacturer: '',
    modelNumber: '',
    uHeight: RACK_DEFAULTS.uHeight,
    widthMm: RACK_DEFAULTS.widthMm,
    depthMm: RACK_DEFAULTS.depthMm,
    heightMm: RACK_DEFAULTS.heightMm,
    dualPower: RACK_DEFAULTS.dualPower,
    inputCircuits: RACK_DEFAULTS.inputCircuits,
    pduCount: RACK_DEFAULTS.pduCount,
    changeNote: '',
    ...overrides,
  }
}

export function latestRackTemplateVersion(template: RackTemplate): RackTemplateVersion | undefined {
  return [...(template.versions ?? [])].sort((a, b) => b.revision - a.revision)[0]
}

/** Apply the same template-to-rack mapping in forms and batch imports. */
export function applyRackTemplateSpec(target: Partial<RackPayload>, spec: RackTemplateSpec): Partial<RackPayload> {
  Object.assign(target, {
    type: spec.type,
    manufacturer: spec.manufacturer ?? '',
    modelNumber: spec.modelNumber ?? '',
    uHeight: spec.uHeight,
    widthMm: spec.widthMm,
    depthMm: spec.depthMm,
    heightMm: spec.heightMm,
    loadCapacityKg: spec.loadCapacityKg,
    dualPower: spec.dualPower,
    inputCircuits: spec.inputCircuits,
    ratedVoltage: spec.ratedVoltage,
    ratedCurrent: spec.ratedCurrent,
    ratedPowerKw: spec.ratedPowerKw,
    peakPowerKw: spec.peakPowerKw,
    pduCount: spec.pduCount,
  })
  return target
}

export function rackPayloadFromRack(rack: Rack): RackPayload {
  const {
    dataCenterId: _dataCenterId, roomId: _roomId, createdAt: _createdAt, updatedAt: _updatedAt,
    templateId: _templateId, templateSnapshot: _templateSnapshot, ...payload
  } = rack
  return payload
}

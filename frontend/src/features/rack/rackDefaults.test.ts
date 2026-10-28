import { describe, expect, it } from 'vitest'
import {
  RACK_DEFAULTS,
  RACK_STATUS_VALUES,
  applyRackTemplateSpec,
  createRackPayload,
  createRackTemplateSpec,
  latestRackTemplateVersion,
} from '@/features/rack/rackDefaults'
import type { RackTemplate } from '@/types/rack-template'

describe('rack defaults and template mapping', () => {
  it('uses one complete default payload for rack forms and imports', () => {
    const payload = createRackPayload()

    expect(payload).toMatchObject({
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
    })
    expect(payload.code).toBe('')
    expect(payload.name).toBe('')
    expect(RACK_STATUS_VALUES).toContain(payload.status)
  })

  it('allows explicit overrides without changing shared defaults', () => {
    const payload = createRackPayload({ code: 'R-01', uHeight: 48, sortOrder: 12 })

    expect(payload).toMatchObject({ code: 'R-01', uHeight: 48, sortOrder: 12 })
    expect(RACK_DEFAULTS.uHeight).toBe(42)
    expect(RACK_DEFAULTS.sortOrder).toBe(0)
  })

  it('maps every template specification field to a rack payload', () => {
    const payload = createRackPayload({ code: 'R-01', name: '机柜 01' })
    applyRackTemplateSpec(payload, {
      type: 'HIGH_DENSITY',
      manufacturer: '厂商 A',
      modelNumber: 'M-42',
      uHeight: 48,
      widthMm: 800,
      depthMm: 1100,
      heightMm: 2200,
      loadCapacityKg: 1500,
      dualPower: true,
      inputCircuits: 2,
      ratedVoltage: 220,
      ratedCurrent: 32,
      ratedPowerKw: 7,
      peakPowerKw: 9,
      pduCount: 4,
    })

    expect(payload).toMatchObject({
      type: 'HIGH_DENSITY',
      manufacturer: '厂商 A',
      modelNumber: 'M-42',
      uHeight: 48,
      widthMm: 800,
      depthMm: 1100,
      heightMm: 2200,
      loadCapacityKg: 1500,
      dualPower: true,
      inputCircuits: 2,
      ratedVoltage: 220,
      ratedCurrent: 32,
      ratedPowerKw: 7,
      peakPowerKw: 9,
      pduCount: 4,
    })
  })

  it('selects the highest revision even when API versions are unordered', () => {
    const template = {
      versions: [
        { revision: 2 },
        { revision: 5 },
        { revision: 3 },
      ],
    } as RackTemplate

    expect(latestRackTemplateVersion(template)?.revision).toBe(5)
    expect(latestRackTemplateVersion({ versions: [] } as unknown as RackTemplate)).toBeUndefined()
  })

  it('creates template specifications with the same physical defaults', () => {
    expect(createRackTemplateSpec()).toMatchObject({
      type: RACK_DEFAULTS.type,
      uHeight: RACK_DEFAULTS.uHeight,
      widthMm: RACK_DEFAULTS.widthMm,
      depthMm: RACK_DEFAULTS.depthMm,
      heightMm: RACK_DEFAULTS.heightMm,
      dualPower: RACK_DEFAULTS.dualPower,
      inputCircuits: RACK_DEFAULTS.inputCircuits,
      pduCount: RACK_DEFAULTS.pduCount,
    })
  })
})

import { describe, expect, it } from 'vitest'
import { findRackSearchTarget, rackSearchableText } from './rackSearch'

const rack = {
  id: 'rack-1', name: '01号机柜', code: 'RACK-01', uHeight: 42,
  widthMm: 600, depthMm: 1000, heightMm: 2000, rotation: 0,
  dualPower: false, inputCircuits: 1, pduCount: 0, dataCenterId: 'dc-1', roomId: 'room-1', type: 'STANDARD',
} as any
const position = {
  id: 'position-1', rackId: rack.id, deviceId: 'device-1', roomId: rack.roomId, dataCenterId: rack.dataCenterId,
  startU: 20, endU: 21, heightU: 2, device: { name: '核心数据库服务器', code: 'DEV-001', serialNumber: 'SN-001' },
} as any

describe('rack screen search', () => {
  it('includes installed device fields in the rack filter text', () => {
    expect(rackSearchableText(rack, [position])).toContain('核心数据库服务器')
  })

  it('locates a device and returns its rack and U position', () => {
    const result = findRackSearchTarget([rack], { [rack.id]: { positions: [position] } }, '数据库服务器')
    expect(result?.rack.id).toBe(rack.id)
    expect(result?.position?.id).toBe(position.id)
  })

  it('prefers an exact device name when multiple devices match', () => {
    const exact = { ...position, id: 'position-2', device: { ...position.device, name: '数据库' } }
    const result = findRackSearchTarget([rack], { [rack.id]: { positions: [position, exact] } }, '数据库')
    expect(result?.position?.id).toBe(exact.id)
  })
})

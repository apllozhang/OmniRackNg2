import { describe, expect, it } from 'vitest'
import type { Device } from '@/types/device'
import {
  createDevicePayload,
  deviceEditableFieldKeys,
  deviceFieldGroups,
  deviceFieldLabels,
  toDeviceApiPayload,
  toDevicePayload,
  validateDevicePayload,
} from '@/features/device/deviceForm'

const timestamp = '2026-09-04T08:00:00Z'

function fullDevice(): Device {
  return {
    id: 'device-1', createdAt: timestamp, updatedAt: timestamp, version: 7,
    typeId: 'type-1', code: 'DEV-001', name: '核心服务器', assetNumber: 'ASSET-001',
    serialNumber: 'SN-001', manufacturer: '示例厂商', modelNumber: 'X1', specification: '双路 CPU',
    firmwareVersion: 'V1.2', purchaseBatch: '2026-Q3', warrantyExpiresAt: '2029-08-31T00:00:00Z',
    organization: '技术中心', manager: '张三', contact: '13800000000', businessSystem: '门户',
    applicationName: '门户应用', lifecycleStatus: 'RUNNING', heightU: 2, widthMm: 440, depthMm: 800,
    heightMm: 88, weightKg: 25.5, ratedPowerW: 800, peakPowerW: 1200, inputVoltage: 220,
    dualPowerRequired: true, managementIp: '10.0.0.1', businessIp: '172.16.0.1',
    macAddress: '00:11:22:33:44:55', managementProtocol: 'SSH', monitoringStatus: 'ONLINE',
    externalQrCode: 'QR-001', externalQrCodeUrl: 'https://example.test/qr/1', tags: '核心,生产', remarks: '备注',
  }
}

describe('device form backend contract', () => {
  it('covers the complete editable field contract of the backend Device model', () => {
    expect(deviceEditableFieldKeys).toEqual([
      'typeId', 'code', 'name', 'assetNumber', 'serialNumber', 'manufacturer', 'modelNumber',
      'specification', 'firmwareVersion', 'purchaseBatch', 'warrantyExpiresAt', 'organization',
      'manager', 'contact', 'businessSystem', 'applicationName', 'lifecycleStatus', 'heightU',
      'widthMm', 'depthMm', 'heightMm', 'weightKg', 'ratedPowerW', 'peakPowerW', 'inputVoltage',
      'dualPowerRequired', 'managementIp', 'businessIp', 'macAddress', 'managementProtocol',
      'monitoringStatus', 'externalQrCode', 'externalQrCodeUrl', 'tags', 'remarks',
    ])
  })

  it('keeps shared labels and detail groups aligned with the editable backend field contract', () => {
    expect(Object.keys(deviceFieldLabels)).toEqual([...deviceEditableFieldKeys])
    const groupedKeys = deviceFieldGroups.flatMap((section) => section.keys)
    expect(new Set(groupedKeys).size).toBe(groupedKeys.length)
    expect([...groupedKeys].sort()).toEqual([...deviceEditableFieldKeys].sort())
  })

  it('loads all backend fields and normalizes the warranty date for editing', () => {
    const source = fullDevice()
    const payload = toDevicePayload(source)
    expect(payload.warrantyExpiresAt).toBe('2029-08-31')
    for (const key of deviceEditableFieldKeys) {
      if (key === 'warrantyExpiresAt') continue
      expect(payload[key]).toEqual(source[key] ?? createDevicePayload()[key])
    }
  })

  it('converts the date back to RFC3339 before sending it to Go', () => {
    const payload = toDeviceApiPayload({ ...createDevicePayload(), warrantyExpiresAt: '2029-08-31' })
    expect(payload.warrantyExpiresAt).toBe('2029-08-31T00:00:00Z')
  })
})

describe('device form validation mirrors backend rules', () => {
  it('validates required fields, IP addresses, integer dimensions and numeric ranges', () => {
    const errors = validateDevicePayload({
      ...createDevicePayload(), typeId: '', code: '', name: '', heightU: 0,
      widthMm: 440.5, ratedPowerW: -1, managementIp: '999.1.1.1', warrantyExpiresAt: '2029-02-30',
    })
    expect(errors).toEqual(expect.arrayContaining([
      '请选择设备类型', '请填写设备编码', '请填写设备名称', '设备 U 高度必须是 1 到 100 的整数',
      '宽度必须是大于 0 的整数', '额定功率不能为负数', '管理 IP 格式无效', '保修到期日格式无效',
    ]))
  })

  it('enforces positioned-device lifecycle and U-height restrictions', () => {
    const errors = validateDevicePayload({
      ...createDevicePayload(), typeId: 'type-1', code: 'DEV-001', name: '设备', heightU: 3,
      lifecycleStatus: 'OFF_RACK',
    }, { positioned: true, originalHeightU: 2 })
    expect(errors).toContain('已上架设备不能直接修改 U 高度，请使用迁移功能')
    expect(errors).toContain('已上架设备不能设置为待上架、已下架或已报废')
  })
})


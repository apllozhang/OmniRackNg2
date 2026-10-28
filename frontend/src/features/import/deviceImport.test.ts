import { describe, expect, it } from 'vitest'
import { prepareDeviceImportRows, deviceImportFields } from '@/features/import/deviceImport'
import type { Device, DeviceType } from '@/types/device'
import type { ParsedExcelRow } from '@/utils/excelImport'

const timestamp = '2026-09-04T00:00:00Z'
const serverType = {
  id: 'type-1', code: 'SERVER', name: '服务器', category: 'SERVER', status: 'ACTIVE',
  defaultHeightU: 2, defaultDualPower: false, sortOrder: 1,
  createdAt: timestamp, updatedAt: timestamp, version: 1,
} as DeviceType

const existingDevice: Device = {
  id: 'device-1', typeId: serverType.id, type: serverType, code: 'DEV-001', name: '核心服务器',
  lifecycleStatus: 'RUNNING', heightU: 2, dualPowerRequired: true, serialNumber: 'SN-001', assetNumber: 'ASSET-001',
  managementIp: '10.0.0.1', businessIp: '172.16.0.1', manager: '原负责人', ratedPowerW: 800,
  createdAt: timestamp, updatedAt: timestamp, version: 7,
}

function row(rowNumber: number, values: Record<string, unknown>): ParsedExcelRow { return { rowNumber, values } }

describe('device import optional code', () => {

  it('accepts terminal inventory and repair labels and prevents importing unrecorded active custody', () => {
    const type={...serverType,category:'PC' as const,code:'TERMINAL-LAPTOP',name:'笔记本'}
    for(const [label,status] of [['在库','WAITING_RACK'],['维修中','MAINTENANCE'],['已报废','SCRAPPED']]) {
      const result=prepareDeviceImportRows([row(2,{typeCode:type.code,name:'办公笔记本',lifecycleStatus:label})],[type],[],0)[0]!
      expect(result.errors).toEqual([])
      expect(result.payload).toMatchObject({lifecycleStatus:status})
    }
    const result=prepareDeviceImportRows([row(2,{typeCode:type.code,name:'办公笔记本',lifecycleStatus:'使用中'})],[type],[],0)[0]!
    expect(result.errors.join('')).toContain('单条登记使用人、部门和位置')
    const existing={...existingDevice,type,typeId:type.id}
    const update=prepareDeviceImportRows([row(2,{code:existing.code,lifecycleStatus:'在库'})],[type],[existing],1)[0]!
    expect(update.errors.join('')).toContain('请清空使用状态列')
  })
  it('does not require a code column in the template', () => {
    expect(deviceImportFields.find((field) => field.key === 'code')?.required).toBe(false)
  })

  it('marks a blank code for server-side generation', () => {
    const rows = [
      row(2, { typeCode: 'server', name: '应用服务器 01' }),
      row(3, { typeCode: 'SERVER', code: 'dev-manual-01', name: '手工编码设备' }),
    ]
    const results = prepareDeviceImportRows(rows, [serverType], [], 0)
    expect(results[0].errors).toEqual([])
    expect(results[0].summary).toBe('自动编码 · 应用服务器 01')
    expect(results[0].operation).toBe('CREATE')
    expect(results[0].payload).toMatchObject({ code: '', autoGenerateCode: true })
    expect(results[1].errors).toEqual([])
    expect(results[1].payload).toMatchObject({ code: 'DEV-MANUAL-01', autoGenerateCode: false })
  })

  it('allows multiple blank codes while retaining duplicate checks for manual codes', () => {
    const rows = [
      row(2, { typeCode: 'SERVER', name: '设备 A' }), row(3, { typeCode: 'SERVER', name: '设备 B' }),
      row(4, { typeCode: 'SERVER', code: 'SAME', name: '设备 C' }), row(5, { typeCode: 'SERVER', code: 'same', name: '设备 D' }),
    ]
    const results = prepareDeviceImportRows(rows, [serverType], [], 0)
    expect(results[0].errors).toEqual([])
    expect(results[1].errors).toEqual([])
    expect(results[2].errors).toContain('Excel 内设备编码重复')
    expect(results[3].errors).toContain('Excel 内设备编码重复')
  })
})

describe('device export re-import update matching', () => {
  it('matches an exported row by device code and preserves version and position-independent fields', () => {
    const result = prepareDeviceImportRows([row(2, {
      typeCode: '服务器', code: 'dev-001', name: '核心服务器', lifecycleStatus: '运行中',
      serialNumber: 'SN-001', assetNumber: 'ASSET-001', managementIp: '10.0.0.99', businessIp: '172.16.0.99',
    })], [serverType], [existingDevice], 1)[0]
    expect(result.errors).toEqual([])
    expect(result.operation).toBe('UPDATE')
    expect(result.matchedDeviceId).toBe('device-1')
    expect(result.payload).toMatchObject({ version: 7, code: 'DEV-001', managementIp: '10.0.0.99', businessIp: '172.16.0.99', manager: '原负责人' })
  })

  it('matches by serial number or asset number when code is blank', () => {
    const bySerial = prepareDeviceImportRows([row(2, { typeCode: 'SERVER', name: '补充管理IP', serialNumber: 'SN-001', managementIp: '10.0.0.2' })], [serverType], [existingDevice], 1)[0]
    const byAsset = prepareDeviceImportRows([row(2, { typeCode: 'SERVER', name: '补充业务IP', assetNumber: 'ASSET-001', businessIp: '172.16.0.2' })], [serverType], [existingDevice], 1)[0]
    expect(bySerial).toMatchObject({ operation: 'UPDATE', matchedDeviceId: 'device-1' })
    expect(bySerial.payload).toMatchObject({ code: 'DEV-001', managementIp: '10.0.0.2', businessIp: '172.16.0.1' })
    expect(byAsset).toMatchObject({ operation: 'UPDATE', matchedDeviceId: 'device-1' })
    expect(byAsset.payload).toMatchObject({ code: 'DEV-001', businessIp: '172.16.0.2', managementIp: '10.0.0.1' })
  })

  it('creates a new device when no identifier matches and generates its code', () => {
    const result = prepareDeviceImportRows([row(2, { typeCode: 'SERVER', name: '新设备', serialNumber: 'SN-NEW' })], [serverType], [existingDevice], 1)[0]
    expect(result.errors).toEqual([])
    expect(result.operation).toBe('CREATE')
    expect(result.matchedDeviceId).toBeUndefined()
    expect(result.payload).toMatchObject({ code: '', autoGenerateCode: true, serialNumber: 'SN-NEW' })
  })

  it('blocks ambiguous identifiers and repeated matches instead of creating duplicates', () => {
    const repeated = prepareDeviceImportRows([
      row(2, { typeCode: 'SERVER', code: 'DEV-001', name: '核心服务器' }),
      row(3, { typeCode: 'SERVER', code: 'DEV-001', name: '核心服务器改名' }),
    ], [serverType], [existingDevice], 1)
    expect(repeated[0].errors).toContain('Excel 内设备编码重复')
    expect(repeated[0].errors).toContain('Excel 内多行匹配同一已有设备，已阻止批量更新')
    expect(repeated[1].errors).toContain('Excel 内多行匹配同一已有设备，已阻止批量更新')

    const changedCode = prepareDeviceImportRows([row(2, {
      typeCode: 'SERVER', code: 'DEV-NEW', name: '核心服务器', serialNumber: 'SN-001',
    })], [serverType], [existingDevice], 1)[0]
    expect(changedCode.errors.join('；')).toContain('请确认是新设备还是原设备改编码')
  })

  it('does not count updates toward the 3000-device create limit and keeps blank optional fields', () => {
    const result = prepareDeviceImportRows([row(2, {
      typeCode: '服务器', code: 'DEV-001', name: '核心服务器', serialNumber: 'SN-001',
      managementIp: '10.0.0.5',
    })], [serverType], [existingDevice], 3000)[0]
    expect(result.errors).toEqual([])
    expect(result.payload).toMatchObject({ managementIp: '10.0.0.5', businessIp: '172.16.0.1', ratedPowerW: 800 })
  })
})

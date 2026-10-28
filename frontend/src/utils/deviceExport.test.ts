import * as XLSX from 'xlsx-js-style'
import { describe, expect, it } from 'vitest'
import { buildDeviceExportWorkbook, deviceExportFileName, toDeviceExportRows } from '@/utils/deviceExport'
import type { DataCenter } from '@/types/resource'
import type { Device } from '@/types/device'

const timestamp = '2026-09-04T08:00:00Z'
const type = { id: 'type-1', code: 'SERVER', name: '服务器', category: 'SERVER' as const, status: 'ACTIVE' as const, defaultHeightU: 2, defaultDualPower: true, sortOrder: 0, createdAt: timestamp, updatedAt: timestamp, version: 1 }
const tree: DataCenter[] = [{ id: 'dc-1', code: 'DC-01', name: '一号数据中心', status: 'OPERATING', sortOrder: 0, createdAt: timestamp, updatedAt: timestamp, version: 1, rooms: [{ id: 'room-1', dataCenterId: 'dc-1', code: 'ROOM-01', name: '主机房', status: 'OPERATING', sortOrder: 0, createdAt: timestamp, updatedAt: timestamp, version: 1, floorPlanEnabled: true, floorPlanFormat: 'GRID', racksPerRow: 8, racks: [{ id: 'rack-1', dataCenterId: 'dc-1', roomId: 'room-1', code: 'RACK-01', name: '01号机柜', status: 'PARTIAL', sortOrder: 0, type: 'STANDARD', uHeight: 42, widthMm: 600, depthMm: 1200, heightMm: 2000, rotation: 0, dualPower: true, inputCircuits: 2, pduCount: 2, createdAt: timestamp, updatedAt: timestamp, version: 1 }] }] }]

const device: Device = { id: 'device-1', typeId: type.id, type, code: 'DEV-001', name: '核心服务器', lifecycleStatus: 'RUNNING', heightU: 2, dualPowerRequired: true, serialNumber: 'SN-001', ratedPowerW: 800, managementIp: '10.0.0.1', createdAt: timestamp, updatedAt: timestamp, version: 1, currentPosition: { id: 'pos-1', deviceId: 'device-1', rackId: 'rack-1', roomId: 'room-1', dataCenterId: 'dc-1', startU: 10, endU: 11, heightU: 2, orientation: 'NORMAL', installedAt: timestamp, createdAt: timestamp, updatedAt: timestamp, version: 1 } }

describe('device Excel export', () => {

  it('exports actual terminal usage states and omits inapplicable cabinet fields', () => {
    const terminal={...device,type:{...type,category:'PC' as const,name:'笔记本'},currentPosition:undefined,lifecycleStatus:'WAITING_RACK' as const}
    const row=toDeviceExportRows([terminal],[],{[device.id]:{status:'IN_USE',assignee:'张三',department:'研究部',siteId:null,position:''}})[0]
    expect(row).toMatchObject({生命周期状态:'使用中',设备分类:'终端',设备高度U:'',是否双路供电:'不适用',上架状态:'不适用'})
    expect(toDeviceExportRows([terminal])[0]?.生命周期状态).toBe('待核实')
    expect(toDeviceExportRows([{...terminal,lifecycleStatus:'SCRAPPED'}])[0]?.生命周期状态).toBe('已报废')
  })
  it('maps device and resolved rack location into a complete ledger row', () => {
    expect(toDeviceExportRows([device], tree)[0]).toMatchObject({
      设备编码: 'DEV-001', 设备名称: '核心服务器', 设备类型: '服务器', 设备分类: '服务器', 生命周期状态: '运行中',
      序列号: 'SN-001', 额定功耗W: 800, 数据中心: '一号数据中心', 机房: '主机房', 机柜编码: 'RACK-01', 机柜名称: '01号机柜', 起始U: 10, 结束U: 11, 上架状态: '已上架',
    })
  })

  it('leaves location blank for an uninstalled device and preserves optional blanks', () => {
    const uninstalled = { ...device, currentPosition: undefined, serialNumber: undefined, ratedPowerW: undefined }
    const row = toDeviceExportRows([uninstalled], tree)[0]
    expect(row).toMatchObject({ 序列号: '', 额定功耗W: '', 数据中心: '', 机房: '', 机柜编码: '', 起始U: '', 结束U: '', 上架状态: '未上架' })
  })

  it('creates styled ledger and explanation sheets with stable headers', () => {
    const workbook = buildDeviceExportWorkbook({ devices: [device], resourceTree: tree, generatedAt: new Date('2026-09-04T08:30:00Z') })
    expect(workbook.SheetNames).toEqual(['设备台账', '导出说明'])
    const sheet = workbook.Sheets['设备台账']
    expect(sheet.A1.v).toBe('设备编码')
    expect(sheet.A1.s.fill.fgColor.rgb).toBe('1F4E78')
    expect(XLSX.utils.sheet_to_json<Record<string, unknown>>(sheet)[0]).toMatchObject({ 设备名称: '核心服务器', 数据中心: '一号数据中心' })
    expect(deviceExportFileName(new Date('2026-09-04T08:30:00'), 1)).toBe('设备台账-20260904083000-1条.xlsx')
  })
})

it('round trips actual XLSX bytes with SN, configuration and location', () => {
  const workbook = buildDeviceExportWorkbook({ devices: [{ ...device, specification: 'CPU 32核 / 128GB / 2TB' }], resourceTree: tree })
  const bytes = XLSX.write(workbook, { bookType: 'xlsx', type: 'array' })
  const restored = XLSX.read(bytes, { type: 'array' })
  expect(XLSX.utils.sheet_to_json(restored.Sheets['设备台账'])).toEqual([expect.objectContaining({ 设备编码: 'DEV-001', 序列号: 'SN-001', 规格: 'CPU 32核 / 128GB / 2TB', 机柜名称: '01号机柜' })])
})
it('rejects empty workbooks', () => {
  expect(() => buildDeviceExportWorkbook({ devices: [] })).toThrow('没有可导出的设备')
})

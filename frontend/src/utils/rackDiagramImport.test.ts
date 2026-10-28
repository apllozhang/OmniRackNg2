import * as XLSX from 'xlsx-js-style'
import { describe, expect, it } from 'vitest'
import { buildRackDiagramWorkbook, RACK_DIAGRAM_META_SHEET } from '@/utils/rackDiagramExport'
import { parseRackDiagramExcel } from '@/utils/rackDiagramImport'
import type { RackDevicePosition, RackULayout } from '@/types/device'
import type { DataCenter, Rack, Room } from '@/types/resource'

const timestamp = '2026-09-02T08:00:00Z'
const dataCenter = {
  id: 'dc-1', code: 'DC-01', name: '数据中心', createdAt: timestamp, updatedAt: timestamp,
  version: 1, status: 'OPERATING', sortOrder: 0,
} as DataCenter
const room = {
  id: 'room-1', dataCenterId: 'dc-1', code: 'ROOM-01', name: '机房', createdAt: timestamp,
  updatedAt: timestamp, version: 1, status: 'OPERATING', sortOrder: 0,
  floorPlanEnabled: true, floorPlanFormat: 'GRID', racksPerRow: 2,
} as Room
function rack(id: string, code: string, name: string): Rack {
  return {
    id, code, name, uHeight: 4, createdAt: timestamp, updatedAt: timestamp, version: 2,
    status: 'PARTIAL', sortOrder: 0, dataCenterId: dataCenter.id, roomId: room.id, type: 'STANDARD',
    widthMm: 600, depthMm: 1200, heightMm: 2000, rotation: 0, dualPower: true,
    inputCircuits: 2, pduCount: 2,
  }
}
const firstRack = rack('rack-1', 'RACK-01', '01号机柜')
const secondRack = rack('rack-2', 'RACK-02', '02号机柜')
function sourcePosition(): RackDevicePosition {
  return {
    id: 'position-1', createdAt: timestamp, updatedAt: timestamp, version: 3,
    deviceId: 'device-1', rackId: firstRack.id, roomId: room.id, dataCenterId: dataCenter.id,
    startU: 2, endU: 3, heightU: 2, orientation: 'NORMAL', installedAt: timestamp,
    device: {
      id: 'device-1', createdAt: timestamp, updatedAt: timestamp, version: 5,
      typeId: 'type-server', code: 'DEV-001', name: '核心服务器', serialNumber: 'SN-001',
      lifecycleStatus: 'RUNNING', ratedPowerW: 800, heightU: 2, dualPowerRequired: true,
      type: {
        id: 'type-server', createdAt: timestamp, updatedAt: timestamp, version: 1,
        code: 'SERVER', name: '服务器', category: 'SERVER', status: 'ACTIVE',
        defaultHeightU: 2, defaultDualPower: true, sortOrder: 0,
      },
    },
  }
}
function layout(target: Rack, positions: RackDevicePosition[] = []): RackULayout {
  return { rack: target, positions, free: [] }
}
function workbook() {
  return buildRackDiagramWorkbook({
    dataCenter, room, racks: [firstRack, secondRack], racksPerRow: 2,
    layouts: { [firstRack.id]: layout(firstRack, [sourcePosition()]), [secondRack.id]: layout(secondRack) },
    generatedAt: new Date(timestamp),
  })
}
function asFile(book: XLSX.WorkBook, name = '机柜图.xlsx') {
  const output = XLSX.write(book, { bookType: 'xlsx', type: 'array', cellStyles: true }) as ArrayBuffer
  return { name, arrayBuffer: async () => output } as File
}

function moveMerge(sheet: XLSX.WorkSheet, from: string, to: string) {
  const fromRange = XLSX.utils.decode_range(from)
  const toRange = XLSX.utils.decode_range(to)
  sheet['!merges'] = (sheet['!merges'] ?? []).filter((item) => {
    const isSource = item.s.r === fromRange.s.r && item.s.c === fromRange.s.c && item.e.r === fromRange.e.r && item.e.c === fromRange.e.c
    const overlapsTarget = item.s.r <= toRange.e.r && item.e.r >= toRange.s.r && item.s.c <= toRange.e.c && item.e.c >= toRange.s.c
    return !isSource && !overlapsTarget
  })
  sheet['!merges']?.push(toRange)
}

describe('rack diagram Excel import parser', () => {
  it('round-trips the exported device identity, data and merged U range', async () => {
    const result = await parseRackDiagramExcel(asFile(workbook()), room.id, dataCenter.id)

    expect(result.payload.coveredRackIds).toEqual([firstRack.id, secondRack.id])
    expect(result.payload.devices).toHaveLength(1)
    expect(result.payload.devices[0]).toMatchObject({
      rackId: firstRack.id, startU: 2, endU: 3, name: '核心服务器', serialNumber: 'SN-001',
      ratedPowerW: 800, sourceDeviceId: 'device-1', sourceDeviceVersion: 5,
      sourceDeviceCode: 'DEV-001', sourceRackId: firstRack.id, sourceStartU: 2, sourceEndU: 3,
    })
  })

  it('keeps blank optional serial number and power undefined during an unchanged round trip', async () => {
    const position = sourcePosition()
    position.device!.serialNumber = undefined
    position.device!.ratedPowerW = undefined
    const book = buildRackDiagramWorkbook({
      dataCenter, room, racks: [firstRack, secondRack], racksPerRow: 2,
      layouts: { [firstRack.id]: layout(firstRack, [position]), [secondRack.id]: layout(secondRack) },
      generatedAt: new Date(timestamp),
    })

    const result = await parseRackDiagramExcel(asFile(book), room.id, dataCenter.id)
    expect(result.payload.devices[0]).toMatchObject({
      name: '核心服务器', sourceDeviceId: 'device-1', sourceDeviceCode: 'DEV-001',
    })
    expect(result.payload.devices[0].serialNumber).toBeUndefined()
    expect(result.payload.devices[0].ratedPowerW).toBeUndefined()
  })
  it('keeps source identity when a device is renamed in its original slot', async () => {
    const book = workbook()
    book.Sheets['机柜图'].D4.v = '核心数据库服务器/SN-001/800W'

    const result = await parseRackDiagramExcel(asFile(book), room.id, dataCenter.id)
    expect(result.payload.devices[0]).toMatchObject({ name: '核心数据库服务器', sourceDeviceId: 'device-1', sourceDeviceCode: 'DEV-001' })
  })

  it('treats a newly filled empty U cell as a new device without a source code', async () => {
    const book = workbook()
    book.Sheets['机柜图'].D3 = { t: 's', v: '新增交换机/SN-NEW/350W', s: book.Sheets['机柜图'].D4.s }

    const result = await parseRackDiagramExcel(asFile(book), room.id, dataCenter.id)
    const added = result.payload.devices.find((item) => item.name === '新增交换机')
    expect(added).toMatchObject({ rackId: firstRack.id, startU: 4, endU: 4, serialNumber: 'SN-NEW', ratedPowerW: 350 })
    expect(added?.sourceDeviceId).toBeUndefined()
    expect(added?.sourceDeviceCode).toBeUndefined()
  })

  it('matches an exported device moved to another rack by its complete original label', async () => {
    const book = workbook()
    const sheet = book.Sheets['机柜图']
    sheet.K4 = { ...sheet.D4 }
    delete sheet.D4
    moveMerge(sheet, 'D4:F5', 'K4:M5')

    const result = await parseRackDiagramExcel(asFile(book), room.id, dataCenter.id)
    expect(result.payload.devices).toHaveLength(1)
    expect(result.payload.devices[0]).toMatchObject({
      rackId: secondRack.id, startU: 2, endU: 3, sourceDeviceId: 'device-1', sourceRackId: firstRack.id,
    })
  })

  it('rejects files without hidden metadata and files from a different room', async () => {
    const withoutMetadata = workbook()
    delete withoutMetadata.Sheets[RACK_DIAGRAM_META_SHEET]
    withoutMetadata.SheetNames = withoutMetadata.SheetNames.filter((name) => name !== RACK_DIAGRAM_META_SHEET)
    await expect(parseRackDiagramExcel(asFile(withoutMetadata), room.id, dataCenter.id)).rejects.toThrow('系统元数据')

    await expect(parseRackDiagramExcel(asFile(workbook()), 'another-room', dataCenter.id)).rejects.toThrow('与当前选择不一致')
  })
})

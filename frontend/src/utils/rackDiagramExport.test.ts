import * as XLSX from 'xlsx-js-style'
import { describe, expect, it } from 'vitest'
import {
  buildRackDiagramWorkbook,
  RACK_DIAGRAM_META_SHEET,
  rackDiagramFileName,
  sanitizeRackDiagramFileName,
} from '@/utils/rackDiagramExport'
import type { RackDevicePosition, RackULayout } from '@/types/device'
import type { DataCenter, Rack, Room } from '@/types/resource'

const timestamp = '2026-09-01T08:00:00Z'

function rack(id: string, code: string, name: string, uHeight = 4): Rack {
  return {
    id, code, name, uHeight, createdAt: timestamp, updatedAt: timestamp, version: 1,
    status: 'PARTIAL', sortOrder: 0, dataCenterId: 'dc-1', roomId: 'room-1', type: 'STANDARD',
    widthMm: 600, depthMm: 1200, heightMm: 2000, rotation: 0, dualPower: true,
    inputCircuits: 2, pduCount: 2,
  }
}

const dataCenter = {
  id: 'dc-1', code: 'DC/01', name: '一号数据中心', createdAt: timestamp, updatedAt: timestamp,
  version: 1, status: 'OPERATING', sortOrder: 0,
} as DataCenter

const room = {
  id: 'room-1', dataCenterId: 'dc-1', code: 'ROOM:01', name: '主机房', createdAt: timestamp,
  updatedAt: timestamp, version: 1, status: 'OPERATING', sortOrder: 0,
  floorPlanEnabled: true, floorPlanFormat: 'GRID', racksPerRow: 2,
} as Room

function position(targetRack: Rack): RackDevicePosition {
  return {
    id: 'position-1', createdAt: timestamp, updatedAt: timestamp, version: 1,
    deviceId: 'device-1', rackId: targetRack.id, roomId: room.id, dataCenterId: dataCenter.id,
    startU: 2, endU: 3, heightU: 2, orientation: 'NORMAL', installedAt: timestamp,
    device: {
      id: 'device-1', createdAt: timestamp, updatedAt: timestamp, version: 1,
      typeId: 'server-type', code: 'DEV-001', name: '核心服务器', lifecycleStatus: 'RUNNING',
      serialNumber: 'SN-20260901-001', ratedPowerW: 800, heightU: 2, dualPowerRequired: true,
      type: {
        id: 'server-type', createdAt: timestamp, updatedAt: timestamp, version: 1,
        code: 'SERVER', name: '服务器', category: 'SERVER', status: 'ACTIVE', defaultHeightU: 2,
        defaultDualPower: true, sortOrder: 0,
      },
    },
  }
}

function layout(targetRack: Rack, positions: RackDevicePosition[] = []): RackULayout {
  return { rack: targetRack, positions, free: [] }
}

function hasMerge(sheet: XLSX.WorkSheet, range: string) {
  const expected = XLSX.utils.decode_range(range)
  return (sheet['!merges'] ?? []).some((merge) =>
    merge.s.r === expected.s.r && merge.s.c === expected.s.c &&
    merge.e.r === expected.e.r && merge.e.c === expected.e.c)
}

describe('rack diagram Excel export', () => {
  it('creates a styled rack diagram with descending U positions and merged multi-U devices', () => {
    const firstRack = rack('rack-1', 'RACK-A01', '01号机柜')
    const workbook = buildRackDiagramWorkbook({
      dataCenter, room, racks: [firstRack], layouts: { [firstRack.id]: layout(firstRack, [position(firstRack)]) },
      generatedAt: new Date('2026-09-01T08:30:00Z'),
    })
    const sheet = workbook.Sheets['机柜图']

    expect(workbook.SheetNames).toEqual(['机柜图', RACK_DIAGRAM_META_SHEET])
    expect(workbook.Workbook?.Sheets?.find((item) => item.name === RACK_DIAGRAM_META_SHEET)?.Hidden).toBe(2)
    const metadataRows = XLSX.utils.sheet_to_json<Record<string, unknown>>(workbook.Sheets[RACK_DIAGRAM_META_SHEET])
    expect(metadataRows).toContainEqual(expect.objectContaining({
      recordType: 'DEVICE', deviceId: 'device-1', deviceCode: 'DEV-001', deviceVersion: 1,
      rackId: 'rack-1', startU: 2, endU: 3,
    }))
    expect(sheet.D2.v).toBe('RACK-A01 01号机柜')
    expect(hasMerge(sheet, 'D2:F2')).toBe(true)
    expect([sheet.B3.v, sheet.B4.v, sheet.B5.v, sheet.B6.v]).toEqual([4, 3, 2, 1])
    expect(sheet.C3.v).toBe('1U')
    expect(hasMerge(sheet, 'D4:F5')).toBe(true)
    expect(sheet.D4.v).toBe('核心服务器/SN-20260901-001/800W')
    expect(sheet.D4.s.fill.fgColor.rgb).toBe('FFFF00')
    expect(sheet.D4.s.alignment).toMatchObject({ horizontal: 'center', vertical: 'center', wrapText: true })
    expect(sheet.D4.s.border.top.style).toBe('thin')
    expect(sheet.G3.s.fill.fgColor.rgb).toBe('00E5E5')
  })

  it('omits missing serial number and rated power without exporting the device code', () => {
    const firstRack = rack('rack-1', 'RACK-A01', '01号机柜')
    const installed = position(firstRack)
    installed.device!.serialNumber = ' '
    installed.device!.ratedPowerW = undefined
    const workbook = buildRackDiagramWorkbook({
      dataCenter, room, racks: [firstRack], layouts: { [firstRack.id]: layout(firstRack, [installed]) },
    })

    expect(workbook.Sheets['机柜图'].D4.v).toBe('核心服务器')
  })

  it('exports whichever optional SN or power value is present', () => {
    const firstRack = rack('rack-1', 'RACK-A01', '01号机柜')
    const withSerial = position(firstRack)
    withSerial.device!.ratedPowerW = undefined
    const serialWorkbook = buildRackDiagramWorkbook({
      dataCenter, room, racks: [firstRack], layouts: { [firstRack.id]: layout(firstRack, [withSerial]) },
    })
    expect(serialWorkbook.Sheets['机柜图'].D4.v).toBe('核心服务器/SN-20260901-001')

    const withPower = position(firstRack)
    withPower.device!.serialNumber = ''
    withPower.device!.ratedPowerW = 650.5
    const powerWorkbook = buildRackDiagramWorkbook({
      dataCenter, room, racks: [firstRack], layouts: { [firstRack.id]: layout(firstRack, [withPower]) },
    })
    expect(powerWorkbook.Sheets['机柜图'].D4.v).toBe('核心服务器/650.5W')
  })

  it('starts the next rack group below the first when racksPerRow is one', () => {
    const firstRack = rack('rack-1', 'RACK-A01', '01号机柜')
    const secondRack = rack('rack-2', 'RACK-A02', '02号机柜')
    const workbook = buildRackDiagramWorkbook({
      dataCenter, room, racks: [firstRack, secondRack], racksPerRow: 1,
      layouts: { [firstRack.id]: layout(firstRack), [secondRack.id]: layout(secondRack) },
    })
    const sheet = workbook.Sheets['机柜图']

    expect(sheet.D2.v).toContain('RACK-A01')
    expect(sheet.D10.v).toContain('RACK-A02')
    expect(sheet.K2).toBeUndefined()
  })

  it('keeps cell style data after xlsx serialization', () => {
    const firstRack = rack('rack-1', 'RACK-A01', '01号机柜')
    const workbook = buildRackDiagramWorkbook({
      dataCenter, room, racks: [firstRack], layouts: { [firstRack.id]: layout(firstRack, [position(firstRack)]) },
    })
    const output = XLSX.write(workbook, { bookType: 'xlsx', type: 'array', cellStyles: true })
    const parsed = XLSX.read(output, { type: 'array', cellStyles: true })
    const sheet = parsed.Sheets['机柜图']

    expect(output.byteLength).toBeGreaterThan(5000)
    expect(sheet.D4.s).toBeDefined()
    expect(sheet.D4.s.fill?.fgColor?.rgb ?? sheet.D4.s.fgColor?.rgb).toMatch(/FFFF00$/)
  })

  it('sanitizes invalid filename characters and appends xlsx', () => {
    expect(sanitizeRackDiagramFileName(' DC/01:主机房* ')).toBe('DC-01-主机房-')
    expect(rackDiagramFileName(dataCenter, room, new Date(2026, 8, 1, 16, 5, 6)))
      .toBe('DC-01-ROOM-01-机柜图-20260901160506.xlsx')
  })
})

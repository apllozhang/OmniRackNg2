import * as XLSX from 'xlsx-js-style'
import type { DataCenter, Rack, Room } from '@/types/resource'
import type { DeviceCategory, RackDevicePosition, RackULayout } from '@/types/device'

export interface RackDiagramExportOptions {
  dataCenter: DataCenter
  room: Room
  racks: Rack[]
  layouts: Record<string, RackULayout>
  racksPerRow?: number
  generatedAt?: Date
}

const RACK_WIDTH = 6
const RACK_GAP = 1
const START_ROW = 1
const START_COLUMN = 1
const MAX_RACKS_PER_ROW = 20
export const RACK_DIAGRAM_META_SHEET = '__CABINET_IMPORT_META__'
export const RACK_DIAGRAM_FORMAT_VERSION = '1'

interface RackDiagramMetadataRow {
  recordType: 'RACK' | 'DEVICE'
  rackId: string
  rackVersion: number
  rackCode: string
  rackName: string
  uHeight: number
  headerRow: number
  firstColumn: number
  deviceId?: string
  deviceVersion?: number
  deviceCode?: string
  deviceName?: string
  serialNumber?: string
  ratedPowerW?: number
  typeId?: string
  typeCategory?: DeviceCategory
  startU?: number
  endU?: number
  cellAddress?: string
}
const COLORS = {
  black: '000000',
  white: 'FFFFFF',
  header: 'FFC000',
  headerSide: 'F4B183',
  uBar: '00E5E5',
  SERVER: 'FFFF00',
  PC: 'DDEBFA',
  NETWORK: '92D050',
  STORAGE: '8EA9DB',
  SECURITY: 'F4B183',
  POWER_ENVIRONMENT: 'F8696B',
  ACCESSORY: 'D9EAD3',
  OTHER: 'D9D2E9',
} satisfies Record<string, string>

const thinBorder = {
  top: { style: 'thin' as const, color: { rgb: COLORS.black } },
  bottom: { style: 'thin' as const, color: { rgb: COLORS.black } },
  left: { style: 'thin' as const, color: { rgb: COLORS.black } },
  right: { style: 'thin' as const, color: { rgb: COLORS.black } },
}

function fill(rgb: string) {
  return { patternType: 'solid' as const, fgColor: { rgb } }
}

function baseStyle(background = COLORS.white): XLSX.CellStyle {
  return {
    font: { name: '微软雅黑', sz: 10, color: { rgb: COLORS.black } },
    alignment: { horizontal: 'center', vertical: 'center', wrapText: true },
    border: thinBorder,
    fill: fill(background),
  }
}

function cell(sheet: XLSX.WorkSheet, row: number, column: number, value: string | number, style: XLSX.CellStyle) {
  const address = XLSX.utils.encode_cell({ r: row, c: column })
  sheet[address] = { t: typeof value === 'number' ? 'n' : 's', v: value, s: style }
}

function addMerge(sheet: XLSX.WorkSheet, startRow: number, startColumn: number, endRow: number, endColumn: number) {
  const merges = sheet['!merges'] ?? (sheet['!merges'] = [])
  merges.push({ s: { r: startRow, c: startColumn }, e: { r: endRow, c: endColumn } })
}

function normalizedRacksPerRow(value: number | undefined, rackCount: number) {
  const numeric = Number(value)
  const fallback = Math.min(8, Math.max(1, rackCount))
  if (!Number.isFinite(numeric)) return fallback
  return Math.min(MAX_RACKS_PER_ROW, Math.max(1, Math.round(numeric)))
}

function safeHeight(rack: Rack) {
  const value = Math.round(Number(rack.uHeight))
  return Number.isFinite(value) ? Math.max(1, value) : 42
}

function positionBounds(position: RackDevicePosition, rackHeight: number) {
  const startU = Math.max(1, Math.min(rackHeight, Math.round(Number(position.startU))))
  const rawEnd = Number(position.endU) || (startU + Math.max(1, Number(position.heightU)) - 1)
  const endU = Math.max(startU, Math.min(rackHeight, Math.round(rawEnd)))
  return { startU, endU }
}

function categoryColor(category: DeviceCategory | undefined) {
  return COLORS[category ?? 'OTHER'] ?? COLORS.OTHER
}

function deviceLabel(position: RackDevicePosition) {
  const device = position.device
  const name = device?.name?.trim() || '未命名设备'
  const serialNumber = device?.serialNumber?.trim()
  const ratedPower = Number(device?.ratedPowerW)
  const powerLabel = Number.isFinite(ratedPower) && ratedPower > 0
    ? `${Number(ratedPower.toFixed(2))}W`
    : ''
  return [name, serialNumber, powerLabel].filter(Boolean).join('/')
}

function rackTitle(rack: Rack) {
  const code = rack.code?.trim()
  const name = rack.name?.trim()
  if (code && name && code !== name) return `${code} ${name}`
  return code || name || '未命名机柜'
}

function renderRack(sheet: XLSX.WorkSheet, rack: Rack, layout: RackULayout | undefined, headerRow: number, firstColumn: number) {
  const rackHeight = safeHeight(rack)
  const numberColumn = firstColumn
  const unitColumn = firstColumn + 1
  const deviceStartColumn = firstColumn + 2
  const deviceEndColumn = firstColumn + 4
  const barColumn = firstColumn + 5

  for (let column = firstColumn; column <= barColumn; column += 1) {
    const background = column >= deviceStartColumn && column <= deviceEndColumn ? COLORS.header : COLORS.headerSide
    cell(sheet, headerRow, column, '', {
      ...baseStyle(background),
      font: { name: '微软雅黑', sz: 11, bold: true, italic: true, color: { rgb: COLORS.black } },
    })
  }
  cell(sheet, headerRow, deviceStartColumn, rackTitle(rack), {
    ...baseStyle(COLORS.header),
    font: { name: '微软雅黑', sz: 11, bold: true, italic: true, color: { rgb: COLORS.black } },
  })
  addMerge(sheet, headerRow, deviceStartColumn, headerRow, deviceEndColumn)

  const positions = (layout?.positions ?? []).map((position) => ({ position, ...positionBounds(position, rackHeight) }))
  const positionByU = new Map<number, (typeof positions)[number]>()
  for (const item of positions) {
    for (let u = item.startU; u <= item.endU; u += 1) {
      if (!positionByU.has(u)) positionByU.set(u, item)
    }
  }

  for (let u = rackHeight; u >= 1; u -= 1) {
    const row = headerRow + 1 + (rackHeight - u)
    const occupied = positionByU.get(u)
    cell(sheet, row, numberColumn, u, baseStyle())
    cell(sheet, row, unitColumn, '1U', baseStyle(COLORS.uBar))
    for (let column = deviceStartColumn; column <= deviceEndColumn; column += 1) {
      cell(sheet, row, column, '', baseStyle(occupied ? categoryColor(occupied.position.device?.type?.category) : COLORS.white))
    }
    cell(sheet, row, barColumn, '', baseStyle(COLORS.uBar))

    if (!occupied) {
      addMerge(sheet, row, deviceStartColumn, row, deviceEndColumn)
      continue
    }
    if (u !== occupied.endU) continue

    const topRow = headerRow + 1 + (rackHeight - occupied.endU)
    const bottomRow = headerRow + 1 + (rackHeight - occupied.startU)
    cell(sheet, topRow, deviceStartColumn, deviceLabel(occupied.position), {
      ...baseStyle(categoryColor(occupied.position.device?.type?.category)),
      font: { name: '微软雅黑', sz: 10, italic: true, color: { rgb: COLORS.black } },
    })
    addMerge(sheet, topRow, deviceStartColumn, bottomRow, deviceEndColumn)
  }

  const footerRow = headerRow + rackHeight + 1
  for (let column = firstColumn; column <= barColumn; column += 1) {
    cell(sheet, footerRow, column, '', baseStyle(COLORS.uBar))
  }
  addMerge(sheet, footerRow, unitColumn, footerRow, barColumn)
  return footerRow
}

function setSheetRange(sheet: XLSX.WorkSheet, lastRow: number, lastColumn: number) {
  sheet['!ref'] = XLSX.utils.encode_range({ s: { r: 0, c: 0 }, e: { r: lastRow, c: lastColumn } })
}

export function buildRackDiagramWorkbook(options: RackDiagramExportOptions) {
  const workbook = XLSX.utils.book_new()
  const sheet: XLSX.WorkSheet = {}
  const racks = options.racks ?? []
  const perRow = normalizedRacksPerRow(options.racksPerRow ?? options.room.racksPerRow, racks.length)
  let headerRow = START_ROW
  let lastRow = START_ROW
  let lastColumn = START_COLUMN
  const metadataRows: RackDiagramMetadataRow[] = []

  for (let offset = 0; offset < racks.length; offset += perRow) {
    const group = racks.slice(offset, offset + perRow)
    const groupHeight = Math.max(...group.map(safeHeight))
    group.forEach((rack, index) => {
      const firstColumn = START_COLUMN + index * (RACK_WIDTH + RACK_GAP)
      const rackHeight = safeHeight(rack)
      const layout = options.layouts[rack.id]
      metadataRows.push({
        recordType: 'RACK', rackId: rack.id, rackVersion: rack.version, rackCode: rack.code, rackName: rack.name,
        uHeight: rackHeight, headerRow: headerRow + 1, firstColumn: firstColumn + 1,
      })
      for (const position of layout?.positions ?? []) {
        const bounds = positionBounds(position, rackHeight)
        const topRow = headerRow + 1 + (rackHeight - bounds.endU)
        metadataRows.push({
          recordType: 'DEVICE', rackId: rack.id, rackVersion: rack.version, rackCode: rack.code, rackName: rack.name,
          uHeight: rackHeight, headerRow: headerRow + 1, firstColumn: firstColumn + 1,
          deviceId: position.deviceId, deviceVersion: position.device?.version, deviceCode: position.device?.code,
          deviceName: position.device?.name, serialNumber: position.device?.serialNumber, ratedPowerW: position.device?.ratedPowerW,
          typeId: position.device?.typeId, typeCategory: position.device?.type?.category,
          startU: bounds.startU, endU: bounds.endU, cellAddress: XLSX.utils.encode_cell({ r: topRow, c: firstColumn + 2 }),
        })
      }
      lastRow = Math.max(lastRow, renderRack(sheet, rack, layout, headerRow, firstColumn))
      lastColumn = Math.max(lastColumn, firstColumn + RACK_WIDTH - 1)
    })
    headerRow += groupHeight + 4
  }

  if (!racks.length) {
    cell(sheet, START_ROW, START_COLUMN, '当前机房暂无机柜', baseStyle(COLORS.headerSide))
  }
  setSheetRange(sheet, lastRow, lastColumn)

  const columnCount = lastColumn + 1
  sheet['!cols'] = Array.from({ length: columnCount }, (_, column) => {
    if (column < START_COLUMN) return { wch: 2 }
    const relative = (column - START_COLUMN) % (RACK_WIDTH + RACK_GAP)
    if (relative === RACK_WIDTH) return { wch: 4 }
    if (relative === 0) return { wch: 5 }
    if (relative === 1 || relative === 5) return { wch: 5 }
    return { wch: 14 }
  })
  sheet['!rows'] = Array.from({ length: lastRow + 1 }, (_, row) => ({ hpt: row === START_ROW ? 24 : 18 }))
  sheet['!margins'] = { left: 0.25, right: 0.25, top: 0.35, bottom: 0.35, header: 0.1, footer: 0.1 }
  sheet['!pageSetup'] = { orientation: 'landscape', fitToWidth: 1, fitToHeight: 0, paperSize: 9 }

  workbook.Props = {
    Title: `${options.dataCenter.name}-${options.room.name}-机柜图`,
    Subject: '机柜 U 位与设备占用图',
    Author: 'IT 基础设施管理平台',
    CreatedDate: options.generatedAt ?? new Date(),
  }
  XLSX.utils.book_append_sheet(workbook, sheet, '机柜图')
  const metadata = XLSX.utils.json_to_sheet(metadataRows, {
    header: ['recordType', 'rackId', 'rackVersion', 'rackCode', 'rackName', 'uHeight', 'headerRow', 'firstColumn', 'deviceId', 'deviceVersion', 'deviceCode', 'deviceName', 'serialNumber', 'ratedPowerW', 'typeId', 'typeCategory', 'startU', 'endU', 'cellAddress'],
  })
  XLSX.utils.sheet_add_aoa(metadata, [
    ['formatVersion', RACK_DIAGRAM_FORMAT_VERSION],
    ['dataCenterId', options.dataCenter.id],
    ['roomId', options.room.id],
    ['generatedAt', (options.generatedAt ?? new Date()).toISOString()],
  ], { origin: 'U1' })
  XLSX.utils.book_append_sheet(workbook, metadata, RACK_DIAGRAM_META_SHEET)
  workbook.Workbook = workbook.Workbook ?? {}
  workbook.Workbook.Sheets = workbook.SheetNames.map((name) => ({ name, Hidden: name === RACK_DIAGRAM_META_SHEET ? 2 : 0 }))
  return workbook
}

export function sanitizeRackDiagramFileName(value: string) {
  const cleaned = value.replace(/[<>:"/\\|?*\u0000-\u001F]/g, '-').replace(/[. ]+$/g, '').trim()
  return cleaned || '机柜图'
}

function timestamp(date: Date) {
  const pad = (value: number) => String(value).padStart(2, '0')
  return `${date.getFullYear()}${pad(date.getMonth() + 1)}${pad(date.getDate())}${pad(date.getHours())}${pad(date.getMinutes())}${pad(date.getSeconds())}`
}

export function rackDiagramFileName(dataCenter: DataCenter, room: Room, generatedAt = new Date()) {
  return `${sanitizeRackDiagramFileName(`${dataCenter.code || dataCenter.name}-${room.code || room.name}-机柜图-${timestamp(generatedAt)}`)}.xlsx`
}

export function downloadRackDiagramExcel(options: RackDiagramExportOptions) {
  const generatedAt = options.generatedAt ?? new Date()
  const workbook = buildRackDiagramWorkbook({ ...options, generatedAt })
  const output = XLSX.write(workbook, { bookType: 'xlsx', type: 'array', cellStyles: true })
  const blob = new Blob([output], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' })
  const url = URL.createObjectURL(blob)
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = rackDiagramFileName(options.dataCenter, options.room, generatedAt)
  anchor.style.display = 'none'
  document.body.appendChild(anchor)
  anchor.click()
  anchor.remove()
  window.setTimeout(() => URL.revokeObjectURL(url), 0)
  return anchor.download
}

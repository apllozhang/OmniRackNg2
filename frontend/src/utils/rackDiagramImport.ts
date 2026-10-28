import * as XLSX from 'xlsx-js-style'
import { RACK_DIAGRAM_FORMAT_VERSION, RACK_DIAGRAM_META_SHEET } from './rackDiagramExport'
import type { DeviceCategory } from '@/types/device'
import type { ParsedRackDiagramImport, RackDiagramImportDeviceInput } from '@/types/rackDiagramImport'

interface MetadataRow {
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

const CATEGORY_BY_COLOR: Record<string, DeviceCategory> = {
  FFFF00: 'SERVER', '92D050': 'NETWORK', '8EA9DB': 'STORAGE', F4B183: 'SECURITY', F8696B: 'POWER_ENVIRONMENT', D9EAD3: 'ACCESSORY', D9D2E9: 'OTHER',
}

function text(value: unknown) { return String(value ?? '').trim() }
function numeric(value: unknown) {
  if (value === null || value === undefined || text(value) === '') return undefined
  const result = Number(value)
  return Number.isFinite(result) ? result : undefined
}
function metadataValue(sheet: XLSX.WorkSheet, row: number) { return text(sheet[XLSX.utils.encode_cell({ r: row, c: 21 })]?.v) }
function deviceLabel(row: MetadataRow) {
  const power = Number(row.ratedPowerW) > 0 ? `${Number(Number(row.ratedPowerW).toFixed(2))}W` : ''
  return [text(row.deviceName), text(row.serialNumber), power].filter(Boolean).join('/')
}
function fillColor(cell: XLSX.CellObject | undefined) {
  const style = (cell as any)?.s
  const raw = style?.fill?.fgColor?.rgb ?? style?.fgColor?.rgb ?? ''
  return text(raw).replace(/^FF(?=[0-9A-F]{6}$)/i, '').toUpperCase()
}
function parseLabel(raw: string, source?: MetadataRow) {
  if (source && raw === deviceLabel(source)) {
    return { name: text(source.deviceName), serialNumber: text(source.serialNumber), ratedPowerW: numeric(source.ratedPowerW) }
  }
  const parts = raw.split('/').map((item) => item.trim()).filter(Boolean)
  let ratedPowerW: number | undefined
  const power = parts[parts.length - 1]?.match(/^([0-9]+(?:\.[0-9]+)?)\s*[wW]$/)
  if (power) { ratedPowerW = Number(power[1]); parts.pop() }
  let serialNumber = ''
  if (parts.length > 1) serialNumber = parts.pop() ?? ''
  return { name: parts.join('/').trim(), serialNumber, ratedPowerW }
}
function mergeAt(sheet: XLSX.WorkSheet, row: number, column: number) {
  return (sheet['!merges'] ?? []).find((merge) => merge.s.r <= row && row <= merge.e.r && merge.s.c <= column && column <= merge.e.c)
}

export async function parseRackDiagramExcel(file: File, expectedRoomId: string, expectedDataCenterId: string): Promise<ParsedRackDiagramImport> {
  if (!/\.xlsx$/i.test(file.name)) throw new Error('请选择由“导出机柜图”生成的 .xlsx 文件')
  const workbook = XLSX.read(await file.arrayBuffer(), { type: 'array', cellStyles: true })
  const diagram = workbook.Sheets['机柜图']
  const metadata = workbook.Sheets[RACK_DIAGRAM_META_SHEET]
  if (!diagram || !metadata) throw new Error('文件缺少机柜图或系统元数据，必须使用本系统导出的机柜图')
  const formatVersion = metadataValue(metadata, 0)
  const dataCenterId = metadataValue(metadata, 1)
  const roomId = metadataValue(metadata, 2)
  const exportedAt = metadataValue(metadata, 3)
  if (formatVersion !== RACK_DIAGRAM_FORMAT_VERSION) throw new Error(`不支持的机柜图版本：${formatVersion || '未知'}`)
  if (roomId !== expectedRoomId || dataCenterId !== expectedDataCenterId) throw new Error('机柜图所属数据中心或机房与当前选择不一致')

  const rows = XLSX.utils.sheet_to_json<MetadataRow>(metadata, { defval: '' }).filter((row) => row.recordType === 'RACK' || row.recordType === 'DEVICE')
  const racks = rows.filter((row) => row.recordType === 'RACK')
  const originalDevices = rows.filter((row) => row.recordType === 'DEVICE' && row.deviceId)
  if (!racks.length) throw new Error('机柜图元数据中没有机柜记录')
  const usedSourceIds = new Set<string>()
  const devices: RackDiagramImportDeviceInput[] = []

  for (const rack of racks) {
    const headerRow = Number(rack.headerRow) - 1
    const firstColumn = Number(rack.firstColumn) - 1
    const uHeight = Number(rack.uHeight)
    if (!rack.rackId || !Number.isInteger(headerRow) || !Number.isInteger(firstColumn) || !Number.isInteger(uHeight) || uHeight < 1) throw new Error(`机柜 ${rack.rackCode || rack.rackName} 的元数据无效`)
    const deviceColumn = firstColumn + 2
    for (let row = headerRow + 1; row <= headerRow + uHeight; row += 1) {
      const address = XLSX.utils.encode_cell({ r: row, c: deviceColumn })
      const raw = text(diagram[address]?.v)
      if (!raw) continue
      const merge = mergeAt(diagram, row, deviceColumn)
      if (merge && (merge.s.r !== row || merge.s.c !== deviceColumn)) continue
      const topRow = merge?.s.r ?? row
      const bottomRow = merge?.e.r ?? row
      const endU = uHeight - (topRow - (headerRow + 1))
      const startU = uHeight - (bottomRow - (headerRow + 1))
      const exactSource = originalDevices.find((item) => item.rackId === rack.rackId && Number(item.startU) === startU && Number(item.endU) === endU)
      const sameLabel = originalDevices.filter((item) => !usedSourceIds.has(text(item.deviceId)) && deviceLabel(item) === raw)
      let source: MetadataRow | undefined
      if (exactSource && deviceLabel(exactSource) === raw && !usedSourceIds.has(text(exactSource.deviceId))) {
        source = exactSource
      } else if (sameLabel.length === 1) {
        source = sameLabel[0]
      } else if (exactSource && !usedSourceIds.has(text(exactSource.deviceId))) {
        // Same slot but edited label: keep the exported identity so validation can ask
        // whether this is a rename/update or a replacement with a newly generated code.
        source = exactSource
      }
      if (source?.deviceId) usedSourceIds.add(text(source.deviceId))
      const parsed = parseLabel(raw, source)
      if (!parsed.name) throw new Error(`机柜 ${rack.rackCode} 的 ${startU}-${endU}U 设备名称为空`)
      const colorCategory = CATEGORY_BY_COLOR[fillColor(diagram[address])]
      devices.push({
        clientId: `${rack.rackId}:${startU}:${endU}:${devices.length}`, rackId: text(rack.rackId), rackCode: text(rack.rackCode),
        startU, endU, name: parsed.name, serialNumber: parsed.serialNumber || undefined, ratedPowerW: parsed.ratedPowerW,
        typeId: text(source?.typeId) || undefined, typeCategory: source?.typeCategory || colorCategory,
        sourceDeviceId: text(source?.deviceId) || undefined, sourceDeviceVersion: numeric(source?.deviceVersion), sourceDeviceCode: text(source?.deviceCode) || undefined,
        sourceRackId: text(source?.rackId) || undefined, sourceStartU: numeric(source?.startU), sourceEndU: numeric(source?.endU),
      })
    }
  }
  return { fileName: file.name, payload: { formatVersion, dataCenterId, roomId, exportedAt: exportedAt || undefined, coveredRackIds: racks.map((rack) => text(rack.rackId)), devices } }
}

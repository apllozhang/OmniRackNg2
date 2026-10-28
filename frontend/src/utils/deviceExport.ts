import * as XLSX from 'xlsx-js-style'
import type { TerminalRegistration } from '@/api/device'
import type { DataCenter, Rack, Room } from '@/types/resource'
import type { Device, DeviceCategory, DeviceLifecycleStatus } from '@/types/device'

export interface DeviceExportOptions {
  terminalStates?: Record<string, TerminalRegistration>
  devices: Device[]
  resourceTree?: DataCenter[]
  generatedAt?: Date
}

export interface DeviceExportRow {
  设备编码: string
  设备名称: string
  设备类型: string
  设备分类: string
  生命周期状态: string
  资产编号: string
  序列号: string
  厂商: string
  型号: string
  规格: string
  固件版本: string
  采购批次: string
  保修到期日: string
  设备高度U: number | string
  宽度mm: number | string
  深度mm: number | string
  高度mm: number | string
  重量kg: number | string
  额定功耗W: number | string
  峰值功耗W: number | string
  输入电压V: number | string
  是否双路供电: string
  管理IP: string
  业务IP: string
  MAC地址: string
  管理协议: string
  监控状态: string
  外部二维码: string
  二维码链接: string
  标签: string
  组织: string
  负责人: string
  联系电话: string
  业务系统: string
  应用名称: string
  数据中心: string
  机房: string
  机柜编码: string
  机柜名称: string
  起始U: number | string
  结束U: number | string
  上架状态: string
  备注: string
}

const categoryLabels: Record<DeviceCategory, string> = {
	PC: '终端',
  SERVER: '服务器', NETWORK: '网络设备', STORAGE: '存储设备', SECURITY: '安全设备',
  POWER_ENVIRONMENT: '配电与环境设备', ACCESSORY: '机柜配件', OTHER: '其他',
}

const lifecycleLabels: Record<DeviceLifecycleStatus, string> = {
  WAITING_RACK: '待上架', RUNNING: '运行中', MAINTENANCE: '维护中',
  PENDING_REMOVAL: '待下架', OFF_RACK: '已下架', SCRAPPED: '已报废',
}

const headerStyle: XLSX.CellStyle = {
  font: { name: '微软雅黑', sz: 10, bold: true, color: { rgb: 'FFFFFF' } },
  alignment: { horizontal: 'center', vertical: 'center', wrapText: true },
  fill: { patternType: 'solid', fgColor: { rgb: '1F4E78' } },
  border: { top: { style: 'thin', color: { rgb: 'B4C7E7' } }, bottom: { style: 'thin', color: { rgb: 'B4C7E7' } }, left: { style: 'thin', color: { rgb: 'B4C7E7' } }, right: { style: 'thin', color: { rgb: 'B4C7E7' } } },
}
const cellStyle: XLSX.CellStyle = {
  font: { name: '微软雅黑', sz: 10, color: { rgb: '1F2937' } },
  alignment: { vertical: 'center', wrapText: true },
  border: { top: { style: 'thin', color: { rgb: 'D9E2F3' } }, bottom: { style: 'thin', color: { rgb: 'D9E2F3' } }, left: { style: 'thin', color: { rgb: 'D9E2F3' } }, right: { style: 'thin', color: { rgb: 'D9E2F3' } } },
}

function text(value: unknown) {
  return value === undefined || value === null ? '' : String(value).trim()
}
function numberOrBlank(value: unknown) {
  return value === undefined || value === null || value === '' || !Number.isFinite(Number(value)) ? '' : Number(value)
}
function locationIndex(resourceTree: DataCenter[] = []) {
  const result = new Map<string, { dataCenter: DataCenter; room: Room; rack: Rack }>()
  for (const dataCenter of resourceTree) {
    for (const room of dataCenter.rooms ?? []) {
      for (const rack of room.racks ?? []) result.set(rack.id, { dataCenter, room, rack })
    }
  }
  return result
}

export function toDeviceExportRows(devices: Device[], resourceTree: DataCenter[] = [], terminalStates: Record<string,TerminalRegistration> = {}): DeviceExportRow[] {
  const locations = locationIndex(resourceTree)
  return devices.map((device) => {
    const isTerminal = device.type?.category==='PC'
    const terminalLabels: Record<string,string> = { IDLE:'在库', IN_USE:'使用中', REPAIR:'维修中', RETIRED:'已报废' }
    const status = device.lifecycleStatus==='SCRAPPED' ? 'RETIRED' : terminalStates[device.id]?.status
    const position = device.currentPosition
    const location = position ? locations.get(position.rackId) : undefined
    return {
      设备编码: text(device.code), 设备名称: text(device.name), 设备类型: text(device.type?.name || device.typeId),
      设备分类: device.type?.category ? categoryLabels[device.type.category] ?? device.type.category : '',
      生命周期状态: isTerminal ? terminalLabels[status || ''] || '待核实' : lifecycleLabels[device.lifecycleStatus] ?? text(device.lifecycleStatus), 资产编号: text(device.assetNumber), 序列号: text(device.serialNumber),
      厂商: text(device.manufacturer), 型号: text(device.modelNumber), 规格: text(device.specification), 固件版本: text(device.firmwareVersion), 采购批次: text(device.purchaseBatch), 保修到期日: text(device.warrantyExpiresAt), 设备高度U: isTerminal && !position ? '' : numberOrBlank(device.heightU), 宽度mm: numberOrBlank(device.widthMm), 深度mm: numberOrBlank(device.depthMm), 高度mm: numberOrBlank(device.heightMm),
      重量kg: numberOrBlank(device.weightKg), 额定功耗W: numberOrBlank(device.ratedPowerW), 峰值功耗W: numberOrBlank(device.peakPowerW), 输入电压V: numberOrBlank(device.inputVoltage),
      是否双路供电: isTerminal && !position ? '不适用' : device.dualPowerRequired ? '是' : '否', 管理IP: text(device.managementIp), 业务IP: text(device.businessIp), MAC地址: text(device.macAddress),
      管理协议: text(device.managementProtocol), 监控状态: text(device.monitoringStatus), 外部二维码: text(device.externalQrCode), 二维码链接: text(device.externalQrCodeUrl), 标签: text(device.tags), 组织: text(device.organization), 负责人: text(device.manager), 联系电话: text(device.contact),
      业务系统: text(device.businessSystem), 应用名称: text(device.applicationName), 数据中心: text(location?.dataCenter.name), 机房: text(location?.room.name),
      机柜编码: text(location?.rack.code || position?.rackId), 机柜名称: text(location?.rack.name), 起始U: position ? numberOrBlank(position.startU) : '', 结束U: position ? numberOrBlank(position.endU) : '',
      上架状态: position ? '已上架' : isTerminal ? '不适用' : '未上架', 备注: text(device.remarks),
    }
  })
}

export function buildDeviceExportWorkbook(options: DeviceExportOptions) {
  if (!options.devices.length) throw new Error('没有可导出的设备')
  const rows = toDeviceExportRows(options.devices, options.resourceTree, options.terminalStates)
  const workbook = XLSX.utils.book_new()
  const sheet = XLSX.utils.json_to_sheet(rows)
  const headers = Object.keys(rows[0] ?? ({
    设备编码: '', 设备名称: '', 设备类型: '', 设备分类: '', 生命周期状态: '', 资产编号: '', 序列号: '', 厂商: '', 型号: '', 规格: '', 固件版本: '', 采购批次: '', 保修到期日: '',
    设备高度U: '', 宽度mm: '', 深度mm: '', 高度mm: '', 重量kg: '', 额定功耗W: '', 峰值功耗W: '', 输入电压V: '', 是否双路供电: '', 管理IP: '', 业务IP: '', MAC地址: '', 管理协议: '',
    监控状态: '', 外部二维码: '', 二维码链接: '', 标签: '', 组织: '', 负责人: '', 联系电话: '', 业务系统: '', 应用名称: '', 数据中心: '', 机房: '', 机柜编码: '', 机柜名称: '', 起始U: '', 结束U: '', 上架状态: '', 备注: '',
  }))
  const range = XLSX.utils.decode_range(sheet['!ref'] ?? 'A1:A1')
  for (let column = range.s.c; column <= range.e.c; column += 1) {
    const address = XLSX.utils.encode_cell({ r: 0, c: column })
    if (sheet[address]) sheet[address].s = headerStyle
  }
  for (let row = 1; row <= range.e.r; row += 1) {
    for (let column = range.s.c; column <= range.e.c; column += 1) {
      const address = XLSX.utils.encode_cell({ r: row, c: column })
      if (sheet[address]) sheet[address].s = cellStyle
    }
  }
  sheet['!cols'] = headers.map((header) => ({ wch: ['设备名称', '规格', '备注'].includes(header) ? 26 : ['数据中心', '机房', '机柜名称'].includes(header) ? 18 : 15 }))
  sheet['!rows'] = [{ hpt: 28 }]
  sheet['!autofilter'] = { ref: sheet['!ref'] ?? 'A1:A1' }
  sheet['!freeze'] = { xSplit: 2, ySplit: 1 }
  XLSX.utils.book_append_sheet(workbook, sheet, '设备台账')

  const note = XLSX.utils.aoa_to_sheet([
    ['设备批量导出说明'],
    ['导出时间', (options.generatedAt ?? new Date()).toLocaleString('zh-CN', { hour12: false })],
    ['导出数量', options.devices.length],
    ['说明', '本表导出所选范围及分类下的设备台账。未上架设备的位置字段为空，上架设备同步展示数据中心、机房、机柜和 U 位。'],
  ])
  note['!cols'] = [{ wch: 18 }, { wch: 100 }]
  note['A1'].s = headerStyle
  for (const address of ['A2', 'A3', 'A4', 'B2', 'B3', 'B4']) if (note[address]) note[address].s = cellStyle
  XLSX.utils.book_append_sheet(workbook, note, '导出说明')
  workbook.Props = { Title: '设备台账', Subject: '设备批量导出', Author: 'IT 基础设施管理平台', CreatedDate: options.generatedAt ?? new Date() }
  return workbook
}

export function sanitizeDeviceExportFileName(value: string) {
  const cleaned = value.replace(/[<>:"/\\|?*\u0000-\u001F]/g, '-').replace(/[. ]+$/g, '').trim()
  return cleaned || '设备台账导出'
}

function timestamp(date: Date) {
  const pad = (value: number) => String(value).padStart(2, '0')
  return `${date.getFullYear()}${pad(date.getMonth() + 1)}${pad(date.getDate())}${pad(date.getHours())}${pad(date.getMinutes())}${pad(date.getSeconds())}`
}

export function deviceExportFileName(generatedAt = new Date(), count?: number) {
  const suffix = count === undefined ? '' : `-${count}条`
  return `${sanitizeDeviceExportFileName(`设备台账-${timestamp(generatedAt)}${suffix}`)}.xlsx`
}

export function downloadDeviceExportExcel(options: DeviceExportOptions) {
  const generatedAt = options.generatedAt ?? new Date()
  const workbook = buildDeviceExportWorkbook({ ...options, generatedAt })
  const output = XLSX.write(workbook, { bookType: 'xlsx', type: 'array', cellStyles: true })
  const blob = new Blob([output], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' })
  const url = URL.createObjectURL(blob)
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = deviceExportFileName(generatedAt, options.devices.length)
  anchor.style.display = 'none'
  document.body.appendChild(anchor)
  anchor.click()
  anchor.remove()
  window.setTimeout(() => URL.revokeObjectURL(url), 0)
  return anchor.download
}



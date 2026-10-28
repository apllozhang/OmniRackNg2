import type { Device, DeviceCreatePayload, DeviceLifecycleStatus, DevicePayload, DeviceType } from '@/types/device'
import { cellBoolean, cellDate, cellInteger, cellText, assignExcelNumber, countExcelValues, isBlankCell, normalizeExcelCode } from '@/utils/excelImport'
import { withOptionalImportCode } from '@/features/code/autoCode'
import { deviceLifecycleValues, deviceNumberFieldKeys, deviceTextFieldKeys, isValidDeviceIp, toDevicePayload } from '@/features/device/deviceForm'
import type { ExcelFieldDefinition, ParsedExcelRow, PreparedImportRow } from '@/utils/excelImport'

export type DeviceImportOperation = 'CREATE' | 'UPDATE'

export interface PreparedDeviceImportRow extends PreparedImportRow {
  operation: DeviceImportOperation
  matchedDeviceId?: string
}

export const deviceImportFields: ExcelFieldDefinition[] = [
  { key:'typeCode', title:'设备类型编码', aliases:['设备类型'], required:true, type:'text', description:'必须与系统中启用的设备类型编码或名称匹配；导出的设备信息可直接再次导入', example:'SERVER', width:22 },
  { key:'code', title:'设备编码', required:false, type:'text', description:'可选；填写已有编码会更新已有设备，填写新编码会新增设备，留空时根据序列号/资产编号匹配，否则由系统自动生成', example:'SRV-APP-001', width:22 },
  { key:'name', title:'设备名称', required:true, type:'text', description:'最多 150 个字符；更新时留空保持原值', example:'应用服务器001', width:24 },
  { key:'lifecycleStatus', title:'生命周期状态', required:false, type:'enum', defaultValue:'WAITING_RACK', allowedValues:['WAITING_RACK','RUNNING','MAINTENANCE','PENDING_REMOVAL','OFF_RACK','SCRAPPED'], description:'支持英文状态值或系统显示名称；设备信息导入不会改变 U 位', example:'WAITING_RACK', width:24 },
  { key:'heightU', title:'设备高度U', required:false, type:'integer', description:'1-100 的整数；新增时留空使用设备类型默认高度，更新时留空保持原值', example:2 },
  { key:'assetNumber', title:'资产编号', required:false, type:'text', description:'建议设置为文本格式；可用于识别已有设备', example:'ASSET-2026-001', width:22 },
  { key:'serialNumber', title:'序列号', required:false, type:'text', description:'建议设置为文本格式；可用于识别已有设备', example:'SN20260831001', width:22 },
  { key:'manufacturer', title:'厂商', required:false, type:'text', description:'设备厂商', example:'示例厂商' },
  { key:'modelNumber', title:'型号', required:false, type:'text', description:'设备型号', example:'MODEL-X2' },
  { key:'specification', title:'规格描述', aliases:['规格'], required:false, type:'text', description:'CPU、内存、端口等规格', example:'2U/双路CPU/256GB' },
  { key:'firmwareVersion', title:'固件版本', required:false, type:'text', description:'固件或系统版本', example:'V1.2.0' },
  { key:'purchaseBatch', title:'采购批次', required:false, type:'text', description:'采购批次号', example:'2026-Q3' },
  { key:'warrantyExpiresAt', title:'保修到期日', required:false, type:'date', description:'YYYY-MM-DD', example:'2029-08-31', width:20 },
  { key:'organization', title:'所属组织', aliases:['组织'], required:false, type:'text', description:'资产归属组织', example:'技术中心' },
  { key:'manager', title:'负责人', required:false, type:'text', description:'设备负责人', example:'李四' },
  { key:'contact', title:'联系方式', aliases:['联系电话'], required:false, type:'text', description:'电话或邮箱', example:'13800000000' },
  { key:'businessSystem', title:'业务系统', required:false, type:'text', description:'所属业务系统', example:'统一门户' },
  { key:'applicationName', title:'应用名称', required:false, type:'text', description:'承载应用名称', example:'门户应用' },
  { key:'widthMm', title:'宽度mm', required:false, type:'number', description:'填写时必须大于 0', example:440 },
  { key:'depthMm', title:'深度mm', required:false, type:'number', description:'填写时必须大于 0', example:800 },
  { key:'heightMm', title:'高度mm', required:false, type:'number', description:'填写时必须大于 0', example:88 },
  { key:'weightKg', title:'重量kg', required:false, type:'number', description:'不能为负数；新增时留空可使用类型默认值，更新时留空保持原值', example:25 },
  { key:'ratedPowerW', title:'额定功率W', aliases:['额定功耗W'], required:false, type:'number', description:'不能为负数；新增时留空可使用类型默认值，更新时留空保持原值', example:800 },
  { key:'peakPowerW', title:'峰值功率W', aliases:['峰值功耗W'], required:false, type:'number', description:'不能为负数；更新时留空保持原值', example:1200 },
  { key:'inputVoltage', title:'输入电压V', required:false, type:'number', description:'不能为负数；更新时留空保持原值', example:220 },
  { key:'dualPowerRequired', title:'需要双路电源', aliases:['是否双路供电'], required:false, type:'boolean', description:'是/否、TRUE/FALSE、1/0；新增时留空使用类型默认值，更新时留空保持原值', example:'是', width:20 },
  { key:'managementIp', title:'管理IP', required:false, type:'text', description:'合法 IPv4 或 IPv6 地址', example:'192.168.10.21', width:20 },
  { key:'businessIp', title:'业务IP', required:false, type:'text', description:'合法 IPv4 或 IPv6 地址', example:'10.10.20.21', width:20 },
  { key:'macAddress', title:'MAC地址', required:false, type:'text', description:'建议格式 AA:BB:CC:DD:EE:FF', example:'00:11:22:33:44:55', width:22 },
  { key:'managementProtocol', title:'管理协议', required:false, type:'text', description:'如 SNMP、Redfish、SSH', example:'SNMP' },
  { key:'monitoringStatus', title:'监控状态', required:false, type:'text', description:'自定义监控状态', example:'已纳管' },
  { key:'externalQrCode', title:'外部二维码', required:false, type:'text', description:'已有外部标签的实际编码，选填；系统二维码自动生成，无需填写', example:'QR-2026-001' },
  { key:'externalQrCodeUrl', title:'二维码链接', required:false, type:'text', description:'已有外部标签实际包含的完整 URL，选填；不是本系统扫码地址', example:'https://example.com/qr/001', width:30 },
  { key:'tags', title:'标签', required:false, type:'text', description:'多个标签可使用逗号分隔', example:'生产,核心' },
  { key:'remarks', title:'备注', required:false, type:'text', description:'补充说明', example:'批量导入示例' },
]

const statuses = deviceLifecycleValues
const statusLabels: Record<string, DeviceLifecycleStatus> = {
  '待上架':'WAITING_RACK', '运行中':'RUNNING', '维护中':'MAINTENANCE', '待下架':'PENDING_REMOVAL', '已下架':'OFF_RACK', '已报废':'SCRAPPED',
}
const textKeys = deviceTextFieldKeys
const numberKeys = deviceNumberFieldKeys

function normalized(value: unknown) { return normalizeExcelCode(value) }
function findByValue(devices: Device[], key: 'code' | 'serialNumber' | 'assetNumber', value: unknown) {
  const target = normalized(value)
  return target ? devices.filter((item) => normalized(item[key]) === target) : []
}
function resolveType(raw: unknown, types: DeviceType[]) {
  const target = normalized(raw)
  if (!target) return undefined
  const active = types.filter((item) => item.status === 'ACTIVE')
  return active.find((item) => normalized(item.code) === target) ?? active.find((item) => normalized(item.name) === target)
}
function resolveStatus(raw: unknown, terminal = false) {
  const value = cellText(raw)
  if (!value) return undefined
  const upper = value.toUpperCase()
  if (terminal) { const terminalLabels: Record<string,DeviceLifecycleStatus> = { 在库:'WAITING_RACK', 使用中:'RUNNING', 维修中:'MAINTENANCE', 已报废:'SCRAPPED' }; if (terminalLabels[value]) return terminalLabels[value] }
  return statuses.includes(upper as DeviceLifecycleStatus) ? upper as DeviceLifecycleStatus : statusLabels[value]
}
function displayIdentity(device?: Device) { return device ? `${device.code} · ${device.name}` : '' }
function hasValue(value: unknown) { return !isBlankCell(value) }

function identifyExisting(value: Record<string, unknown>, existingDevices: Device[], errors: string[]) {
  const code = findByValue(existingDevices, 'code', value.code)
  const serial = findByValue(existingDevices, 'serialNumber', value.serialNumber)
  const asset = findByValue(existingDevices, 'assetNumber', value.assetNumber)
  if (code.length > 1) errors.push('系统中存在重复设备编码，无法安全更新')
  if (serial.length > 1) errors.push('系统中存在重复序列号，无法安全识别设备')
  if (asset.length > 1) errors.push('系统中存在重复资产编号，无法安全识别设备')

  const identity = [...new Map([...serial, ...asset].map((item) => [item.id, item])).values()]
  if (code.length === 1 && identity.some((item) => item.id !== code[0].id)) {
    errors.push(`设备编码匹配“${displayIdentity(code[0])}”，但序列号或资产编号匹配其他设备，请确认是新设备还是原设备改编码`)
    return undefined
  }
  if (code.length === 1) return code[0]
  if (hasValue(value.code) && identity.length) {
    errors.push('设备编码未匹配到已有设备，但序列号或资产编号已匹配已有设备，请确认是新设备还是原设备改编码')
    return undefined
  }
  if (identity.length > 1) {
    errors.push('序列号与资产编号分别匹配到不同设备，无法安全导入')
    return undefined
  }
  return identity[0]
}

function existingPayload(device: Device): DevicePayload {
  return toDevicePayload(device)
}

function mergeUpdatePayload(existing: Device, value: Record<string, unknown>, type: DeviceType | undefined, status: DeviceLifecycleStatus | undefined, parsed: Partial<DevicePayload>): DevicePayload {
  const current = existingPayload(existing)
  const merged: DevicePayload = { ...current, version: existing.version }
  if (type) merged.typeId = type.id
  if (hasValue(value.code)) merged.code = normalized(value.code)
  if (hasValue(value.name)) merged.name = cellText(value.name)
  if (status) merged.lifecycleStatus = status
  if (parsed.heightU !== undefined) merged.heightU = parsed.heightU
  if (parsed.dualPowerRequired !== undefined) merged.dualPowerRequired = parsed.dualPowerRequired
  for (const key of textKeys) if (hasValue(value[key])) merged[key] = cellText(value[key])
  for (const key of numberKeys) if (parsed[key] !== undefined) merged[key] = parsed[key] as never
  if (hasValue(value.warrantyExpiresAt)) merged.warrantyExpiresAt = cellDate(value.warrantyExpiresAt) ?? existing.warrantyExpiresAt
  return merged
}

export function prepareDeviceImportRows(rows: ParsedExcelRow[], types: DeviceType[], existingDevices: Device[], currentTotal: number): PreparedDeviceImportRow[] {
  const counts = countExcelValues(rows, 'code')
  const serialCounts = countExcelValues(rows, 'serialNumber')
  const assetCounts = countExcelValues(rows, 'assetNumber')
  const prepared = rows.map((row): PreparedDeviceImportRow => {
    const value = row.values
    const errors: string[] = []
    const existing = identifyExisting(value, existingDevices, errors)
    const operation: DeviceImportOperation = existing ? 'UPDATE' : 'CREATE'
    const typeRaw = cellText(value.typeCode)
    const type = resolveType(typeRaw, types)
    const code = normalized(value.code)
    const name = cellText(value.name)
    const terminal = (type?.category ?? existing?.type?.category)==='PC'
    const status = resolveStatus(value.lifecycleStatus,terminal)

    if (!typeRaw) { if (operation === 'CREATE') errors.push('设备类型编码不能为空') }
    else if (!type) errors.push(`启用的设备类型编码或名称 ${typeRaw} 不存在`)
    if (code) {
      if (code.length > 80) errors.push('设备编码不能超过 80 个字符')
      if ((counts.get(code) ?? 0) > 1) errors.push('Excel 内设备编码重复')
    }
    const serialNumber = normalized(value.serialNumber)
    const assetNumber = normalized(value.assetNumber)
    if (serialNumber && (serialCounts.get(serialNumber) ?? 0) > 1) errors.push('Excel 内序列号重复')
    if (assetNumber && (assetCounts.get(assetNumber) ?? 0) > 1) errors.push('Excel 内资产编号重复')
    if (!name) { if (operation === 'CREATE') errors.push('设备名称不能为空') }
    else if (name.length > 150) errors.push('设备名称不能超过 150 个字符')
    if (hasValue(value.lifecycleStatus) && !status) errors.push(`生命周期状态 ${cellText(value.lifecycleStatus)} 无效`)

    if (terminal && operation==='CREATE' && status==='RUNNING') errors.push('使用中终端请单条登记使用人、部门和位置，批量新增仅支持在库、维修中或已报废')
    if (terminal && operation==='UPDATE' && ['在库','使用中','维修中','已报废'].includes(cellText(value.lifecycleStatus))) errors.push('已有终端状态请通过领用、归还、维修或报废操作变更；导入基础档案时请清空使用状态列')
    const height = cellInteger(value.heightU)
    if (height !== undefined && (Number.isNaN(height) || height < 1 || height > 100)) errors.push('设备高度U必须是 1-100 的整数')
    const warranty = cellDate(value.warrantyExpiresAt)
    if (warranty === null) errors.push('保修到期日必须使用 YYYY-MM-DD 格式')
    const dualPower = cellBoolean(value.dualPowerRequired)
    if (dualPower === null) errors.push('需要双路电源只能填写是/否、TRUE/FALSE、1/0')

    const payload: DevicePayload = {
      version: existing?.version ?? 1, typeId: type?.id ?? existing?.typeId ?? '', code: code || existing?.code || '', name: name || existing?.name || '',
      lifecycleStatus: status ?? existing?.lifecycleStatus ?? 'WAITING_RACK', heightU: height ?? existing?.heightU ?? type?.defaultHeightU ?? 1,
      dualPowerRequired: dualPower ?? existing?.dualPowerRequired ?? type?.defaultDualPower ?? false,
    }
    for (const key of textKeys) payload[key] = cellText(value[key])
    payload.warrantyExpiresAt = warranty ?? undefined
    assignExcelNumber(payload, 'widthMm', value.widthMm, errors, '宽度mm', { min: 0, exclusiveMin: true })
    assignExcelNumber(payload, 'depthMm', value.depthMm, errors, '深度mm', { min: 0, exclusiveMin: true })
    assignExcelNumber(payload, 'heightMm', value.heightMm, errors, '高度mm', { min: 0, exclusiveMin: true })
    assignExcelNumber(payload, 'weightKg', value.weightKg, errors, '重量kg', { min: 0 })
    assignExcelNumber(payload, 'ratedPowerW', value.ratedPowerW, errors, '额定功率W', { min: 0 })
    assignExcelNumber(payload, 'peakPowerW', value.peakPowerW, errors, '峰值功率W', { min: 0 })
    assignExcelNumber(payload, 'inputVoltage', value.inputVoltage, errors, '输入电压V', { min: 0 })
    if (payload.managementIp && !isValidDeviceIp(payload.managementIp)) errors.push('管理IP格式无效')
    if (payload.businessIp && !isValidDeviceIp(payload.businessIp)) errors.push('业务IP格式无效')

    const finalPayload = existing ? mergeUpdatePayload(existing, value, type, status, payload) : withOptionalImportCode(payload)
    return {
      rowNumber: row.rowNumber, operation, matchedDeviceId: existing?.id,
      summary: existing ? `更新 · ${displayIdentity(existing)}` : code ? `${code} · ${name || '未填写名称'}` : `自动编码 · ${name || '未填写设备名称'}`,
      payload: finalPayload, errors,
    }
  })
  const matchedRows = new Map<string, PreparedDeviceImportRow[]>()
  for (const row of prepared) if (row.matchedDeviceId) matchedRows.set(row.matchedDeviceId, [...(matchedRows.get(row.matchedDeviceId) ?? []), row])
  for (const duplicateRows of matchedRows.values()) if (duplicateRows.length > 1) duplicateRows.forEach((row) => row.errors.push('Excel 内多行匹配同一已有设备，已阻止批量更新'))
  const availableSlots = Math.max(0, 3000 - currentTotal)
  let allowedCreates = 0
  for (const row of prepared) {
    if (row.operation === 'CREATE' && !row.errors.length) {
      if (allowedCreates >= availableSlots) row.errors.push(`超过设备总数 3000 台上限，当前仅剩 ${availableSlots} 个可用名额`)
      allowedCreates += 1
    }
  }
  return prepared
}

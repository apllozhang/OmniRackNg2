import type { Device, DeviceLifecycleStatus, DevicePayload } from '@/types/device'

export const deviceLifecycleOptions: Array<{ value: DeviceLifecycleStatus; label: string }> = [
  { value: 'WAITING_RACK', label: '待上架' },
  { value: 'RUNNING', label: '运行中' },
  { value: 'MAINTENANCE', label: '维护中' },
  { value: 'PENDING_REMOVAL', label: '待下架' },
  { value: 'OFF_RACK', label: '已下架' },
  { value: 'SCRAPPED', label: '已报废' },
]

export const deviceLifecycleValues = deviceLifecycleOptions.map((item) => item.value)

export const deviceTextFieldKeys = [
  'assetNumber', 'serialNumber', 'manufacturer', 'modelNumber', 'specification', 'firmwareVersion',
  'purchaseBatch', 'organization', 'manager', 'contact', 'businessSystem', 'applicationName',
  'managementIp', 'businessIp', 'macAddress', 'managementProtocol', 'monitoringStatus',
  'externalQrCode', 'externalQrCodeUrl', 'tags', 'remarks',
] as const

export const deviceNumberFieldKeys = [
  'widthMm', 'depthMm', 'heightMm', 'weightKg', 'ratedPowerW', 'peakPowerW', 'inputVoltage',
] as const

export const deviceEditableFieldKeys = [
  'typeId', 'code', 'name', 'assetNumber', 'serialNumber', 'manufacturer', 'modelNumber',
  'specification', 'firmwareVersion', 'purchaseBatch', 'warrantyExpiresAt', 'organization',
  'manager', 'contact', 'businessSystem', 'applicationName', 'lifecycleStatus', 'heightU',
  'widthMm', 'depthMm', 'heightMm', 'weightKg', 'ratedPowerW', 'peakPowerW', 'inputVoltage',
  'dualPowerRequired', 'managementIp', 'businessIp', 'macAddress', 'managementProtocol',
  'monitoringStatus', 'externalQrCode', 'externalQrCodeUrl', 'tags', 'remarks',
] as const satisfies ReadonlyArray<Exclude<keyof DevicePayload, 'version' | 'id'>>

export type DeviceEditableFieldKey = typeof deviceEditableFieldKeys[number]

export const deviceFieldLabels: Record<DeviceEditableFieldKey, string> = {
  typeId: '设备类型',
  code: '设备编码',
  name: '设备名称',
  assetNumber: '资产编号',
  serialNumber: '序列号',
  manufacturer: '厂商',
  modelNumber: '型号',
  specification: '规格',
  firmwareVersion: '固件版本',
  purchaseBatch: '采购批次',
  warrantyExpiresAt: '保修到期日',
  organization: '所属组织',
  manager: '负责人',
  contact: '联系方式',
  businessSystem: '业务系统',
  applicationName: '应用名称',
  lifecycleStatus: '生命周期',
  heightU: 'U 高度',
  widthMm: '宽度(mm)',
  depthMm: '深度(mm)',
  heightMm: '设备高度(mm)',
  weightKg: '重量(kg)',
  ratedPowerW: '额定功率(W)',
  peakPowerW: '峰值功率(W)',
  inputVoltage: '输入电压(V)',
  dualPowerRequired: '双路电源',
  managementIp: '管理 IP',
  businessIp: '业务 IP',
  macAddress: 'MAC 地址',
  managementProtocol: '管理协议',
  monitoringStatus: '监控状态',
  externalQrCode: '外部二维码',
  externalQrCodeUrl: '二维码链接',
  tags: '标签',
  remarks: '备注',
}

export const deviceFieldGroups: ReadonlyArray<{ title: string; keys: readonly DeviceEditableFieldKey[] }> = [
  { title: '基本信息', keys: ['typeId', 'code', 'name', 'lifecycleStatus'] },
  { title: '资产与规格', keys: ['assetNumber', 'serialNumber', 'manufacturer', 'modelNumber', 'specification', 'firmwareVersion', 'purchaseBatch', 'warrantyExpiresAt'] },
  { title: '尺寸与电力', keys: ['heightU', 'widthMm', 'depthMm', 'heightMm', 'weightKg', 'ratedPowerW', 'peakPowerW', 'inputVoltage', 'dualPowerRequired'] },
  { title: '组织与业务', keys: ['organization', 'manager', 'contact', 'businessSystem', 'applicationName'] },
  { title: '网络与管理', keys: ['managementIp', 'businessIp', 'macAddress', 'managementProtocol', 'monitoringStatus'] },
  { title: '扩展信息', keys: ['externalQrCode', 'externalQrCodeUrl', 'tags', 'remarks'] },
]

export function createDevicePayload(): DevicePayload {
  return {
    version: 1,
    typeId: '',
    code: '',
    name: '',
    assetNumber: '',
    serialNumber: '',
    manufacturer: '',
    modelNumber: '',
    specification: '',
    firmwareVersion: '',
    purchaseBatch: '',
    warrantyExpiresAt: undefined,
    organization: '',
    manager: '',
    contact: '',
    businessSystem: '',
    applicationName: '',
    lifecycleStatus: 'WAITING_RACK',
    heightU: 1,
    widthMm: undefined,
    depthMm: undefined,
    heightMm: undefined,
    weightKg: undefined,
    ratedPowerW: undefined,
    peakPowerW: undefined,
    inputVoltage: undefined,
    dualPowerRequired: false,
    managementIp: '',
    businessIp: '',
    macAddress: '',
    managementProtocol: '',
    monitoringStatus: '',
    externalQrCode: '',
    externalQrCodeUrl: '',
    tags: '',
    remarks: '',
  }
}

export function toDevicePayload(device: Device): DevicePayload {
  const { id: _id, createdAt: _createdAt, updatedAt: _updatedAt, type: _type, currentPosition: _currentPosition, ...payload } = device
  return {
    ...createDevicePayload(),
    ...payload,
    warrantyExpiresAt: toDateOnly(payload.warrantyExpiresAt),
  }
}

export function toDeviceApiPayload(payload: DevicePayload): DevicePayload {
  const warranty = payload.warrantyExpiresAt?.trim()
  return {
    ...payload,
    warrantyExpiresAt: warranty ? (/^\d{4}-\d{2}-\d{2}$/.test(warranty) ? `${warranty}T00:00:00Z` : warranty) : undefined,
  }
}

export interface DeviceValidationOptions {
  terminal?: boolean
  autoGenerateCode?: boolean
  positioned?: boolean
  originalHeightU?: number
}

export function validateDevicePayload(payload: DevicePayload, options: DeviceValidationOptions = {}) {
  const errors: string[] = []
  const code = payload.code.trim()
  const name = payload.name.trim()
  if (!payload.typeId) errors.push('请选择设备类型')
  if (!options.autoGenerateCode && !code) errors.push('请填写设备编码')
  if (code.length > 80) errors.push('设备编码不能超过 80 个字符')
  if (!name) errors.push('请填写设备名称')
  if (name.length > 150) errors.push('设备名称不能超过 150 个字符')
  if (!deviceLifecycleValues.includes(payload.lifecycleStatus)) errors.push('设备生命周期状态无效')
  if (!options.terminal && (!Number.isInteger(Number(payload.heightU)) || Number(payload.heightU) < 1 || Number(payload.heightU) > 100)) errors.push('设备 U 高度必须是 1 到 100 的整数')

  for (const [key, label] of [['widthMm', '宽度'], ['depthMm', '深度'], ['heightMm', '设备高度']] as const) {
    const value = payload[key]
    if (value !== undefined && value !== null && (!Number.isInteger(Number(value)) || Number(value) <= 0)) errors.push(`${label}必须是大于 0 的整数`)
  }
  for (const [key, label] of [['weightKg', '重量'], ['ratedPowerW', '额定功率'], ['peakPowerW', '峰值功率'], ['inputVoltage', '输入电压']] as const) {
    const value = payload[key]
    if (value !== undefined && value !== null && (!Number.isFinite(Number(value)) || Number(value) < 0)) errors.push(`${label}不能为负数`)
  }
  if (payload.managementIp?.trim() && !isValidDeviceIp(payload.managementIp)) errors.push('管理 IP 格式无效')
  if (payload.businessIp?.trim() && !isValidDeviceIp(payload.businessIp)) errors.push('业务 IP 格式无效')
  if (payload.warrantyExpiresAt && !isValidDateOnly(payload.warrantyExpiresAt)) errors.push('保修到期日格式无效')

  const lengthRules: Array<[keyof DevicePayload, string, number]> = [
    ['assetNumber', '资产编号', 100], ['serialNumber', '序列号', 120], ['manufacturer', '厂商', 100],
    ['modelNumber', '型号', 100], ['specification', '规格', 500], ['firmwareVersion', '固件版本', 100],
    ['purchaseBatch', '采购批次', 100], ['organization', '所属组织', 150], ['manager', '负责人', 100],
    ['contact', '联系方式', 100], ['businessSystem', '业务系统', 150], ['applicationName', '应用名称', 150],
    ['managementIp', '管理 IP', 64], ['businessIp', '业务 IP', 64], ['macAddress', 'MAC 地址', 64],
    ['managementProtocol', '管理协议', 100], ['monitoringStatus', '监控状态', 50],
    ['externalQrCode', '外部二维码', 255], ['externalQrCodeUrl', '二维码链接', 500], ['tags', '标签', 500],
  ]
  for (const [key, label, max] of lengthRules) {
    const value = payload[key]
    if (typeof value === 'string' && value.length > max) errors.push(`${label}不能超过 ${max} 个字符`)
  }

  if (options.positioned && options.originalHeightU !== undefined && Number(payload.heightU) !== Number(options.originalHeightU)) errors.push('已上架设备不能直接修改 U 高度，请使用迁移功能')
  if (options.positioned && ['WAITING_RACK', 'OFF_RACK', 'SCRAPPED'].includes(payload.lifecycleStatus)) errors.push('已上架设备不能设置为待上架、已下架或已报废')
  if (!options.terminal && !options.positioned && payload.lifecycleStatus === 'RUNNING') errors.push('未上架设备不能直接设置为运行中')
  return errors
}

export function isValidDeviceIp(value: string) {
  const normalized = value.trim()
  if (normalized.includes(':')) return /^[0-9a-fA-F:]+$/.test(normalized) && normalized.includes(':')
  const parts = normalized.split('.')
  return parts.length === 4 && parts.every((item) => /^\d{1,3}$/.test(item) && Number(item) >= 0 && Number(item) <= 255)
}

function toDateOnly(value?: string) {
  if (!value) return undefined
  const match = value.match(/^(\d{4}-\d{2}-\d{2})/)
  return match?.[1] ?? value
}

function isValidDateOnly(value: string) {
  const match = value.match(/^(\d{4})-(\d{2})-(\d{2})$/)
  if (!match) return false
  const year = Number(match[1]), month = Number(match[2]), day = Number(match[3])
  const date = new Date(Date.UTC(year, month - 1, day))
  return date.getUTCFullYear() === year && date.getUTCMonth() === month - 1 && date.getUTCDate() === day
}

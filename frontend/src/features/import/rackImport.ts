import type { DataCenter, Rack, RackCreatePayload } from '@/types/resource'
import type { RackTemplate } from '@/types/rack-template'
import { RACK_DEFAULTS, RACK_STATUS_VALUES, applyRackTemplateSpec, createRackPayload, latestRackTemplateVersion } from '@/features/rack/rackDefaults'
import { withOptionalImportCode } from '@/features/code/autoCode'
import { cellBoolean, cellInteger, cellText, assignExcelInteger, assignExcelNumber, countExcelValues, normalizeExcelCode } from '@/utils/excelImport'
import type { ExcelFieldDefinition, ParsedExcelRow, PreparedImportRow } from '@/utils/excelImport'

export interface RackImportPayload {
  roomId: string
  payload: RackCreatePayload
}

export const rackImportFields: ExcelFieldDefinition[] = [
  { key:'dataCenterCode', title:'数据中心编码', required:true, type:'text', description:'必须与系统中已有数据中心编码完全匹配', example:'DC-BJ-01', width:22 },
  { key:'roomCode', title:'机房编码', required:true, type:'text', description:'必须属于所填数据中心，且机房未停用/归档', example:'ROOM-A01', width:20 },
  { key:'code', title:'机柜编码', required:false, type:'text', description:'可选；填写时必须全局唯一且最多 50 个字符，留空由系统自动生成', example:'RACK-A-01', width:20 },
  { key:'name', title:'机柜名称', required:true, type:'text', description:'最多 150 个字符', example:'A区01号机柜', width:22 },
  { key:'templateCode', title:'机柜模板编码', required:false, type:'text', description:'填写时必须匹配启用的机柜模板；留空使用自定义参数', example:'RACK-TPL-42U', width:22 },
  { key:'status', title:'状态', required:false, type:'enum', defaultValue:RACK_DEFAULTS.status, allowedValues:[...RACK_STATUS_VALUES], description:'机柜状态', example:'AVAILABLE', width:20 },
  { key:'type', title:'机柜类型', required:false, type:'text', defaultValue:RACK_DEFAULTS.type, description:'自定义类型英文编码', example:'STANDARD' },
  { key:'uHeight', title:'U位高度', required:false, type:'integer', defaultValue:RACK_DEFAULTS.uHeight, description:'1-100 的整数', example:42 },
  { key:'widthMm', title:'宽度mm', required:false, type:'number', defaultValue:RACK_DEFAULTS.widthMm, description:'必须大于 0', example:600 },
  { key:'depthMm', title:'深度mm', required:false, type:'number', defaultValue:RACK_DEFAULTS.depthMm, description:'必须大于 0', example:1200 },
  { key:'heightMm', title:'高度mm', required:false, type:'number', defaultValue:RACK_DEFAULTS.heightMm, description:'必须大于 0', example:2000 },
  { key:'manufacturer', title:'厂商', required:false, type:'text', description:'机柜厂商', example:'示例厂商' },
  { key:'modelNumber', title:'型号', required:false, type:'text', description:'机柜型号', example:'CAB-42U' },
  { key:'serialNumber', title:'序列号', required:false, type:'text', description:'建议设置为文本格式', example:'SN20260831001', width:22 },
  { key:'assetNumber', title:'资产编号', required:false, type:'text', description:'建议设置为文本格式', example:'ASSET-RACK-001', width:22 },
  { key:'loadCapacityKg', title:'承重kg', required:false, type:'number', description:'不能为负数', example:1000 },
  { key:'zone', title:'区域', required:false, type:'text', description:'机房内区域', example:'A' },
  { key:'row', title:'排', required:false, type:'text', description:'机房内排号', example:'01' },
  { key:'column', title:'列', required:false, type:'text', description:'机房内列号', example:'01' },
  { key:'aisle', title:'通道', required:false, type:'text', description:'机房内通道', example:'冷通道1' },
  { key:'xCoordinate', title:'X坐标', required:false, type:'number', description:'大屏平面位置 X 坐标', example:100 },
  { key:'yCoordinate', title:'Y坐标', required:false, type:'number', description:'大屏平面位置 Y 坐标', example:100 },
  { key:'rotation', title:'旋转角度', required:false, type:'enum', defaultValue:RACK_DEFAULTS.rotation, allowedValues:['0','90','180','270'], description:'仅支持 0/90/180/270', example:0 },
  { key:'manager', title:'负责人', required:false, type:'text', description:'机柜负责人', example:'张三' },
  { key:'department', title:'所属部门', required:false, type:'text', description:'负责部门', example:'基础设施部' },
  { key:'purpose', title:'用途', required:false, type:'text', description:'机柜用途', example:'核心业务' },
  { key:'dualPower', title:'双路电源', required:false, type:'boolean', defaultValue:RACK_DEFAULTS.dualPower, description:'是/否、TRUE/FALSE、1/0', example:'是' },
  { key:'inputCircuits', title:'输入路数', required:false, type:'integer', defaultValue:RACK_DEFAULTS.inputCircuits, description:'大于等于 0 的整数', example:2 },
  { key:'ratedVoltage', title:'额定电压V', required:false, type:'number', description:'不能为负数', example:220 },
  { key:'ratedCurrent', title:'额定电流A', required:false, type:'number', description:'不能为负数', example:32 },
  { key:'ratedPowerKw', title:'额定功率kW', required:false, type:'number', description:'不能为负数', example:7.04 },
  { key:'peakPowerKw', title:'峰值功率kW', required:false, type:'number', description:'不能为负数', example:8 },
  { key:'pduCount', title:'PDU数量', required:false, type:'integer', defaultValue:RACK_DEFAULTS.pduCount, description:'大于等于 0 的整数', example:2 },
  { key:'sortOrder', title:'排序值', required:false, type:'integer', defaultValue:RACK_DEFAULTS.sortOrder, description:'大于等于 0 的整数', example:10 },
  { key:'remarks', title:'备注', required:false, type:'text', description:'补充说明', example:'批量导入示例' },
]

export function prepareRackImportRows(rows: ParsedExcelRow[], tree: DataCenter[], templates: RackTemplate[], existingRacks: Rack[]): PreparedImportRow[] {
  const dcMap = new Map(tree.map((item) => [normalizeExcelCode(item.code), item]))
  const templateMap = new Map(templates.filter((item) => item.status === 'ACTIVE').map((item) => [normalizeExcelCode(item.code), item]))
  const existingCodes = new Set(existingRacks.map((item) => normalizeExcelCode(item.code)))
  const counts = countExcelValues(rows, 'code')

  const prepared = rows.map((row) => {
    const value = row.values
    const errors: string[] = []
    const dataCenterCode = normalizeExcelCode(value.dataCenterCode)
    const roomCode = normalizeExcelCode(value.roomCode)
    const code = normalizeExcelCode(value.code)
    const name = cellText(value.name)
    const dataCenter = dcMap.get(dataCenterCode)
    const room = dataCenter?.rooms?.find((item) => normalizeExcelCode(item.code) === roomCode)
    if (!dataCenterCode) errors.push('数据中心编码不能为空')
    else if (!dataCenter) errors.push(`数据中心编码 ${dataCenterCode} 不存在`)
    else if (['DISABLED','ARCHIVED'].includes(dataCenter.status)) errors.push('所属数据中心已停用或归档')
    if (!roomCode) errors.push('机房编码不能为空')
    else if (dataCenter && !room) errors.push(`机房编码 ${roomCode} 不属于数据中心 ${dataCenterCode}`)
    else if (room && ['DISABLED','ARCHIVED'].includes(room.status)) errors.push('所属机房已停用或归档')
    if (code) {
      if (code.length > 50) errors.push('机柜编码不能超过 50 个字符')
      if ((counts.get(code) ?? 0) > 1) errors.push('Excel 内机柜编码重复')
      if (existingCodes.has(code)) errors.push('机柜编码已存在')
    }
    if (!name) errors.push('机柜名称不能为空')
    else if (name.length > 150) errors.push('机柜名称不能超过 150 个字符')

    const templateCode = normalizeExcelCode(value.templateCode)
    const template = templateCode ? templateMap.get(templateCode) : undefined
    const templateVersion = template ? latestRackTemplateVersion(template) : undefined
    if (templateCode && !template) errors.push(`启用的机柜模板编码 ${templateCode} 不存在`)
    else if (template && !templateVersion) errors.push(`机柜模板 ${templateCode} 没有可用版本`)

    const base = createRackPayload({ code, name })
    if (templateVersion) applyRackTemplateSpec(base, templateVersion)

    const status = normalizeExcelCode(value.status) || RACK_DEFAULTS.status
    if (!RACK_STATUS_VALUES.includes(status as typeof RACK_STATUS_VALUES[number])) errors.push(`状态 ${status} 无效`)
    base.status = status
    base.type = normalizeExcelCode(value.type) || base.type
    base.templateVersionId = templateVersion?.id

    assignExcelInteger(base, 'uHeight', value.uHeight, errors, 'U位高度', 1, 100)
    assignExcelNumber(base, 'widthMm', value.widthMm, errors, '宽度mm', { min: 0, exclusiveMin: true })
    assignExcelNumber(base, 'depthMm', value.depthMm, errors, '深度mm', { min: 0, exclusiveMin: true })
    assignExcelNumber(base, 'heightMm', value.heightMm, errors, '高度mm', { min: 0, exclusiveMin: true })
    assignExcelNumber(base, 'loadCapacityKg', value.loadCapacityKg, errors, '承重kg', { min: 0 })
    assignExcelNumber(base, 'xCoordinate', value.xCoordinate, errors, 'X坐标')
    assignExcelNumber(base, 'yCoordinate', value.yCoordinate, errors, 'Y坐标')
    const rotation = cellInteger(value.rotation)
    if (rotation !== undefined) {
      if (Number.isNaN(rotation) || ![0,90,180,270].includes(rotation)) errors.push('旋转角度仅支持 0、90、180、270')
      else base.rotation = rotation
    }
    const dualPower = cellBoolean(value.dualPower)
    if (dualPower === null) errors.push('双路电源只能填写是/否、TRUE/FALSE、1/0')
    else if (dualPower !== undefined) base.dualPower = dualPower
    assignExcelInteger(base, 'inputCircuits', value.inputCircuits, errors, '输入路数', 0)
    assignExcelNumber(base, 'ratedVoltage', value.ratedVoltage, errors, '额定电压V', { min: 0 })
    assignExcelNumber(base, 'ratedCurrent', value.ratedCurrent, errors, '额定电流A', { min: 0 })
    assignExcelNumber(base, 'ratedPowerKw', value.ratedPowerKw, errors, '额定功率kW', { min: 0 })
    assignExcelNumber(base, 'peakPowerKw', value.peakPowerKw, errors, '峰值功率kW', { min: 0 })
    assignExcelInteger(base, 'pduCount', value.pduCount, errors, 'PDU数量', 0)
    assignExcelInteger(base, 'sortOrder', value.sortOrder, errors, '排序值')

    Object.assign(base, {
      manufacturer:cellText(value.manufacturer) || base.manufacturer, modelNumber:cellText(value.modelNumber) || base.modelNumber, serialNumber:cellText(value.serialNumber), assetNumber:cellText(value.assetNumber),
      zone:cellText(value.zone), row:cellText(value.row), column:cellText(value.column), aisle:cellText(value.aisle), manager:cellText(value.manager),
      department:cellText(value.department), purpose:cellText(value.purpose), remarks:cellText(value.remarks),
    })
    const payload = withOptionalImportCode({ ...base, overrideTemplateSpec: Boolean(templateVersion) }) as RackCreatePayload
    return { rowNumber:row.rowNumber, summary:code ? `${code} · ${name || '未填写名称'}` : `自动编码 · ${name || '未填写机柜名称'}`, payload:room ? { roomId:room.id, payload } satisfies RackImportPayload : undefined, errors }
  })
  return prepared
}

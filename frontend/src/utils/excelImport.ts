import * as XLSX from 'xlsx-js-style'

export type ExcelFieldType = 'text' | 'integer' | 'number' | 'boolean' | 'date' | 'enum'

export interface ExcelFieldDefinition {
  key: string
  title: string
  aliases?: string[]
  required: boolean
  type: ExcelFieldType
  defaultValue?: string | number | boolean
  allowedValues?: string[]
  description: string
  example?: string | number | boolean
  width?: number
}

export interface ParsedExcelRow {
  rowNumber: number
  values: Record<string, unknown>
}

export interface PreparedImportRow {
  rowNumber: number
  summary: string
  payload?: unknown
  errors: string[]
  operation?: 'CREATE' | 'UPDATE'
  matchedDeviceId?: string
}

export function excelFieldHeader(field: ExcelFieldDefinition) {
  return `${field.title}（${field.required ? '必填' : '可选'}）`
}

function normalizeHeader(value: unknown) {
  return String(value ?? '').trim().replace(/[()]/g, (item) => item === '(' ? '（' : '）')
}

function workbookFileName(fileName: string) {
  return fileName.toLowerCase().endsWith('.xlsx') ? fileName : `${fileName}.xlsx`
}

function setWidths(sheet: XLSX.WorkSheet, widths: number[]) {
  sheet['!cols'] = widths.map((wch) => ({ wch }))
  sheet['!autofilter'] = sheet['!ref'] ? { ref: sheet['!ref'] } : undefined
}

export function buildExcelFieldIndexes(headers: unknown[], fields: ExcelFieldDefinition[]) {
  const normalizedHeaders = headers.map(normalizeHeader)
  const indexes = new Map<string, number>()
  for (const field of fields) {
    const candidates = [
      excelFieldHeader(field), field.title, field.key,
      `${field.title}（必填）`, `${field.title}（可选）`, ...(field.aliases ?? []),
    ].map(normalizeHeader)
    const index = normalizedHeaders.findIndex((header) => candidates.includes(header))
    if (index >= 0) indexes.set(field.key, index)
  }
  return indexes
}

function saveWorkbook(workbook: XLSX.WorkBook, fileName: string) {
  const output = XLSX.write(workbook, { bookType: 'xlsx', type: 'array' })
  const blob = new Blob([output], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' })
  const url = URL.createObjectURL(blob)
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = workbookFileName(fileName)
  anchor.click()
  URL.revokeObjectURL(url)
}

export function downloadExcelTemplate(fileName: string, subject: string, fields: ExcelFieldDefinition[]) {
  const headers = fields.map(excelFieldHeader)
  const workbook = XLSX.utils.book_new()

  const dataSheet = XLSX.utils.aoa_to_sheet([headers])
  setWidths(dataSheet, fields.map((field) => field.width ?? Math.max(14, Math.min(28, field.title.length * 2 + 8))))
  XLSX.utils.book_append_sheet(workbook, dataSheet, '导入数据')

  const exampleSheet = XLSX.utils.aoa_to_sheet([headers, fields.map((field) => field.example ?? field.defaultValue ?? '')])
  setWidths(exampleSheet, fields.map((field) => field.width ?? Math.max(14, Math.min(28, field.title.length * 2 + 8))))
  XLSX.utils.book_append_sheet(workbook, exampleSheet, '填写示例')

  const descriptionRows = fields.map((field) => ({
    字段名称: field.title,
    是否必填: field.required ? '必填' : '可选',
    类型格式: field.type === 'boolean' ? '布尔值' : field.type === 'integer' ? '整数' : field.type === 'number' ? '数字' : field.type === 'date' ? '日期 YYYY-MM-DD' : field.type === 'enum' ? '枚举值' : '文本',
    默认值: field.defaultValue ?? '',
    可选值或填写说明: field.allowedValues?.join(' / ') || field.description,
    示例: field.example ?? '',
  }))
  const descriptionSheet = XLSX.utils.json_to_sheet(descriptionRows)
  setWidths(descriptionSheet, [22, 12, 18, 16, 52, 24])
  XLSX.utils.book_append_sheet(workbook, descriptionSheet, '字段说明')

  const instructionSheet = XLSX.utils.aoa_to_sheet([
    [`${subject}批量导入模板使用说明`],
    ['1', '请只在“导入数据”工作表中填写待导入数据，“填写示例”工作表不会被导入。'],
    ['2', '表头包含“必填/可选”标识，请勿修改或删除必填字段表头。'],
    ['3', '机柜/设备编码为可选字段：留空时由系统在导入提交时自动生成；设备导出信息再次导入时，已有设备编码会更新原设备，不会重复创建。序列号、资产编号、IP、MAC 等字段建议在 Excel 中设置为文本格式，避免前导 0 丢失。'],
    ['4', '枚举字段必须按“字段说明”工作表中的英文值填写；布尔值可填写：是/否、TRUE/FALSE、1/0。'],
    ['5', '日期统一填写为 YYYY-MM-DD；空白可选字段将采用默认值或保持为空。'],
    ['6', '设备导入支持新增和更新：优先按设备编码匹配，编码为空时按序列号或资产编号匹配；系统会校验重复编码、序列号、资产编号及冲突匹配，错误行不会提交。'],
    ['7', '设备更新时，空白可选字段默认保持原值；导出的数据中心、机房、机柜和 U 位仅供查看，不会通过设备信息导入改变设备位置。'],
  ])
  setWidths(instructionSheet, [10, 100])
  XLSX.utils.book_append_sheet(workbook, instructionSheet, '使用说明')
  saveWorkbook(workbook, fileName)
}

export async function parseExcelFile(file: File, fields: ExcelFieldDefinition[]): Promise<ParsedExcelRow[]> {
  const workbook = XLSX.read(await file.arrayBuffer(), { type: 'array', cellDates: true })
  const sheet = workbook.Sheets['导入数据'] ?? workbook.Sheets[workbook.SheetNames[0]]
  if (!sheet) throw new Error('Excel 中没有可读取的工作表')

  const matrix = XLSX.utils.sheet_to_json<unknown[]>(sheet, { header: 1, defval: '', raw: true })
  if (!matrix.length) throw new Error('Excel 内容为空')
  const indexes = buildExcelFieldIndexes(matrix[0], fields)
  const missing = fields.filter((field) => field.required && !indexes.has(field.key))
  if (missing.length) throw new Error(`缺少必填字段表头：${missing.map((field) => field.title).join('、')}`)

  return matrix.slice(1).map((cells, index) => {
    const values: Record<string, unknown> = {}
    for (const field of fields) {
      const columnIndex = indexes.get(field.key)
      values[field.key] = columnIndex === undefined ? '' : cells[columnIndex]
    }
    return { rowNumber: index + 2, values }
  }).filter((row) => Object.values(row.values).some((value) => !isBlankCell(value)))
}

export function downloadImportResult(fileName: string, rows: Array<{ rowNumber: number; summary: string; status: string; errors: string }>) {
  const sheet = XLSX.utils.json_to_sheet(rows.map((row) => ({ Excel行号: row.rowNumber, 数据标识: row.summary, 导入状态: row.status, 错误原因: row.errors })))
  setWidths(sheet, [12, 30, 14, 80])
  const workbook = XLSX.utils.book_new()
  XLSX.utils.book_append_sheet(workbook, sheet, '导入结果')
  saveWorkbook(workbook, fileName)
}

export function isBlankCell(value: unknown) {
  return value === undefined || value === null || (typeof value === 'string' && value.trim() === '')
}

export function cellText(value: unknown) {
  if (isBlankCell(value)) return ''
  if (value instanceof Date) return formatDate(value)
  return String(value).trim()
}

export function cellNumber(value: unknown): number | undefined {
  if (isBlankCell(value)) return undefined
  const parsed = typeof value === 'number' ? value : Number(String(value).replace(/,/g, '').trim())
  return Number.isFinite(parsed) ? parsed : Number.NaN
}

export function cellInteger(value: unknown): number | undefined {
  const parsed = cellNumber(value)
  if (parsed === undefined || Number.isNaN(parsed)) return parsed
  return Number.isInteger(parsed) ? parsed : Number.NaN
}

export function cellBoolean(value: unknown): boolean | undefined | null {
  if (isBlankCell(value)) return undefined
  if (typeof value === 'boolean') return value
  if (typeof value === 'number') return value === 1 ? true : value === 0 ? false : null
  const normalized = String(value).trim().toLowerCase()
  if (['是', 'true', 'yes', 'y', '1'].includes(normalized)) return true
  if (['否', 'false', 'no', 'n', '0'].includes(normalized)) return false
  return null
}

export function cellDate(value: unknown): string | undefined | null {
  if (isBlankCell(value)) return undefined
  if (value instanceof Date && !Number.isNaN(value.getTime())) return formatDate(value)
  if (typeof value === 'number') {
    const parsed = XLSX.SSF.parse_date_code(value)
    if (parsed) return `${parsed.y}-${String(parsed.m).padStart(2, '0')}-${String(parsed.d).padStart(2, '0')}`
  }
  const text = String(value).trim()
  if (!/^\d{4}-\d{2}-\d{2}$/.test(text)) return null
  const date = new Date(`${text}T00:00:00`)
  return Number.isNaN(date.getTime()) ? null : text
}

function formatDate(value: Date) {
  return `${value.getFullYear()}-${String(value.getMonth() + 1).padStart(2, '0')}-${String(value.getDate()).padStart(2, '0')}`
}


export function normalizeExcelCode(value: unknown) {
  return cellText(value).toUpperCase()
}

export function countExcelValues(rows: ParsedExcelRow[], key: string) {
  const counts = new Map<string, number>()
  for (const row of rows) {
    const value = normalizeExcelCode(row.values[key])
    if (value) counts.set(value, (counts.get(value) ?? 0) + 1)
  }
  return counts
}

export interface ExcelNumberValidationOptions {
  min?: number
  max?: number
  exclusiveMin?: boolean
  integer?: boolean
}

export function assignExcelNumber(
  target: object,
  key: string,
  value: unknown,
  errors: string[],
  label: string,
  options: ExcelNumberValidationOptions = {},
) {
  const parsed = options.integer ? cellInteger(value) : cellNumber(value)
  if (parsed === undefined) return
  const invalid = Number.isNaN(parsed)
    || (options.min !== undefined && (options.exclusiveMin ? parsed <= options.min : parsed < options.min))
    || (options.max !== undefined && parsed > options.max)
  if (invalid) {
    if (options.integer) {
      const range = options.min !== undefined ? `不小于 ${options.min} 的` : ''
      errors.push(`${label}必须是${range}整数${options.max !== undefined ? `且不能大于 ${options.max}` : ''}`)
    } else {
      errors.push(`${label}数值无效`)
    }
    return
  }
  ;(target as Record<string, unknown>)[key] = parsed
}

export function assignExcelInteger(
  target: object,
  key: string,
  value: unknown,
  errors: string[],
  label: string,
  min?: number,
  max?: number,
) {
  assignExcelNumber(target, key, value, errors, label, { min, max, integer: true })
}





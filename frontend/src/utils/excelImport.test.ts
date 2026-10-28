import * as XLSX from 'xlsx-js-style'
import { describe, expect, it } from 'vitest'
import {
  assignExcelInteger,
  assignExcelNumber,
  buildExcelFieldIndexes,
  cellBoolean,
  cellInteger,
  cellNumber,
  countExcelValues,
  normalizeExcelCode,
  parseExcelFile,
} from '@/utils/excelImport'
import type { ExcelFieldDefinition, ParsedExcelRow } from '@/utils/excelImport'

const fields: ExcelFieldDefinition[] = [
  { key: 'code', title: '机柜编码', aliases: ['编码'], required: true, type: 'text', description: '唯一编码' },
  { key: 'uHeight', title: 'U位高度', required: false, type: 'integer', description: '1-100', defaultValue: 42 },
  { key: 'widthMm', title: '宽度mm', required: false, type: 'number', description: '大于 0', defaultValue: 600 },
]

describe('excel import field mapping and validation', () => {
  it('matches generated required/optional headers, keys and aliases', () => {
    const indexes = buildExcelFieldIndexes(
      ['机柜编码（必填）', 'uHeight', '宽度mm(可选)'],
      fields,
    )

    expect([...indexes.entries()]).toEqual([
      ['code', 0],
      ['uHeight', 1],
      ['widthMm', 2],
    ])
  })

  it('rejects a workbook without a required field header and parses mapped rows', async () => {
    const missingHeaderWorkbook = XLSX.utils.book_new()
    XLSX.utils.book_append_sheet(missingHeaderWorkbook, XLSX.utils.aoa_to_sheet([['U位高度']]), '导入数据')
    const missingBuffer = XLSX.write(missingHeaderWorkbook, { bookType: 'xlsx', type: 'array' })
    await expect(parseExcelFile(new File([missingBuffer], 'missing.xlsx'), fields)).rejects.toThrow('缺少必填字段表头：机柜编码')

    const workbook = XLSX.utils.book_new()
    XLSX.utils.book_append_sheet(workbook, XLSX.utils.aoa_to_sheet([
      ['编码', 'U位高度', '宽度mm（可选）'],
      [' rack-01 ', 48, 800],
    ]), '导入数据')
    const buffer = XLSX.write(workbook, { bookType: 'xlsx', type: 'array' })
    const rows = await parseExcelFile(new File([buffer], 'racks.xlsx'), fields)

    expect(rows).toEqual([{ rowNumber: 2, values: { code: ' rack-01 ', uHeight: 48, widthMm: 800 } }])
  })

  it('normalizes codes consistently and counts duplicates case-insensitively', () => {
    const rows: ParsedExcelRow[] = [
      { rowNumber: 2, values: { code: ' rack-01 ' } },
      { rowNumber: 3, values: { code: 'RACK-01' } },
      { rowNumber: 4, values: { code: '' } },
    ]

    expect(normalizeExcelCode(' rack-01 ')).toBe('RACK-01')
    expect(countExcelValues(rows, 'code').get('RACK-01')).toBe(2)
    expect(countExcelValues(rows, 'code').has('')).toBe(false)
  })

  it('keeps zero valid, rejects invalid numeric values and leaves blank optional values untouched', () => {
    expect(cellNumber(0)).toBe(0)
    expect(cellInteger(0)).toBe(0)
    expect(cellInteger(2.5)).toBe(Number.NaN)
    expect(cellBoolean('否')).toBe(false)

    const target: Record<string, unknown> = { inputCircuits: 7, widthMm: 600 }
    const errors: string[] = []
    assignExcelInteger(target, 'inputCircuits', 0, errors, '输入路数', 0)
    assignExcelNumber(target, 'widthMm', 0, errors, '宽度mm', { min: 0, exclusiveMin: true })
    assignExcelInteger(target, 'uHeight', 2.5, errors, 'U位高度', 1, 100)
    assignExcelNumber(target, 'depthMm', -1, errors, '深度mm', { min: 0, exclusiveMin: true })
    assignExcelNumber(target, 'heightMm', '', errors, '高度mm', { min: 0, exclusiveMin: true })

    expect(target.inputCircuits).toBe(0)
    expect(target.widthMm).toBe(600)
    expect(target.heightMm).toBeUndefined()
    expect(errors).toEqual(['宽度mm数值无效', 'U位高度必须是不小于 1 的整数且不能大于 100', '深度mm数值无效'])
  })
})

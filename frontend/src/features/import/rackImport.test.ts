import { describe, expect, it } from 'vitest'
import { prepareRackImportRows } from '@/features/import/rackImport'
import type { DataCenter, Rack } from '@/types/resource'
import type { RackTemplate } from '@/types/rack-template'
import type { ParsedExcelRow } from '@/utils/excelImport'

const dataCenter = {
  id: 'dc-1', code: 'DC-01', name: '数据中心 01', status: 'OPERATING', sortOrder: 0, version: 1,
  createdAt: '2026-09-01T00:00:00Z', updatedAt: '2026-09-01T00:00:00Z',
  rooms: [{
    id: 'room-1', dataCenterId: 'dc-1', code: 'ROOM-01', name: '机房 01', status: 'OPERATING', sortOrder: 0, version: 1,
    createdAt: '2026-09-01T00:00:00Z', updatedAt: '2026-09-01T00:00:00Z',
    floorPlanEnabled: false, floorPlanFormat: '', racksPerRow: 8,
  }],
} as DataCenter

const template = {
  id: 'template-1', code: 'TPL-42U', name: '标准模板', status: 'ACTIVE', isSystem: false,
  currentRevision: 2, version: 1, createdAt: '2026-09-01T00:00:00Z', updatedAt: '2026-09-01T00:00:00Z',
  versions: [{
    id: 'version-2', templateId: 'template-1', revision: 2, version: 1,
    createdAt: '2026-09-01T00:00:00Z', updatedAt: '2026-09-01T00:00:00Z',
    type: 'STANDARD', manufacturer: '模板厂商', modelNumber: 'TPL-MODEL',
    uHeight: 48, widthMm: 800, depthMm: 1200, heightMm: 2200,
    loadCapacityKg: 1800, dualPower: true, inputCircuits: 2,
    ratedVoltage: 220, ratedCurrent: 32, ratedPowerKw: 7, peakPowerKw: 9, pduCount: 4,
  }],
} as RackTemplate

describe('rack import integration', () => {
  it('keeps template physical defaults when optional spreadsheet overrides are blank', () => {
    const rows: ParsedExcelRow[] = [{
      rowNumber: 2,
      values: {
        dataCenterCode: ' dc-01 ', roomCode: 'room-01', code: 'rack-01', name: '机柜 01', templateCode: 'tpl-42u',
      },
    }]

    const [result] = prepareRackImportRows(rows, [dataCenter], [template], [])

    expect(result.errors).toEqual([])
    const importPayload = result.payload as { payload: Record<string, unknown> } | undefined
    expect(importPayload?.payload).toMatchObject({
      templateVersionId: 'version-2',
      overrideTemplateSpec: true,
      type: 'STANDARD',
      manufacturer: '模板厂商',
      modelNumber: 'TPL-MODEL',
      uHeight: 48,
      widthMm: 800,
      dualPower: true,
      pduCount: 4,
    })
  })

  it('keeps spreadsheet instance overrides after a template supplies defaults', () => {
    const [result] = prepareRackImportRows([{
      rowNumber: 2,
      values: {
        dataCenterCode: 'DC-01', roomCode: 'ROOM-01', code: 'RACK-02', name: '自定义尺寸机柜',
        templateCode: 'TPL-42U', widthMm: 900, depthMm: 1300, heightMm: 2300, dualPower: '否', inputCircuits: 0, pduCount: 0,
      },
    }], [dataCenter], [template], [])

    expect(result.errors).toEqual([])
    const importPayload = result.payload as { payload: Record<string, unknown> } | undefined
    expect(importPayload?.payload).toMatchObject({
      templateVersionId: 'version-2', overrideTemplateSpec: true,
      widthMm: 900, depthMm: 1300, heightMm: 2300,
      dualPower: false, inputCircuits: 0, pduCount: 0,
    })
  })


  it('allows a blank rack code and marks it for server-side generation', () => {
    const [result] = prepareRackImportRows([{
      rowNumber: 2,
      values: { dataCenterCode: 'DC-01', roomCode: 'ROOM-01', name: '自动编号机柜' },
    }], [dataCenter], [], [])

    expect(result.errors).toEqual([])
    expect(result.summary).toBe('自动编码 · 自动编号机柜')
    expect(result.payload).toMatchObject({
      roomId: 'room-1',
      payload: { code: '', autoGenerateCode: true },
    })
  })

  it('does not reject imports when the existing rack count exceeds the retired global threshold', () => {
    const existingRacks = Array.from({ length: 201 }, (_, index) => ({ code: `EXISTING-${index}` } as Rack))
    const [result] = prepareRackImportRows([{
      rowNumber: 2,
      values: { dataCenterCode: 'DC-01', roomCode: 'ROOM-01', name: '超过历史阈值的机柜' },
    }], [dataCenter], [], existingRacks)

    expect(result.errors).toEqual([])
    expect(result.payload).toMatchObject({ payload: { autoGenerateCode: true } })
  })

  it('applies the shared duplicate and number validation rules', () => {
    const rows: ParsedExcelRow[] = [
      { rowNumber: 2, values: { dataCenterCode: 'DC-01', roomCode: 'ROOM-01', code: 'RACK-01', name: 'A', widthMm: 0 } },
      { rowNumber: 3, values: { dataCenterCode: 'DC-01', roomCode: 'ROOM-01', code: 'rack-01', name: 'B', uHeight: 42.5 } },
    ]

    const results = prepareRackImportRows(rows, [dataCenter], [], [])

    expect(results[0].errors).toContain('Excel 内机柜编码重复')
    expect(results[0].errors).toContain('宽度mm数值无效')
    expect(results[1].errors).toContain('Excel 内机柜编码重复')
    expect(results[1].errors).toContain('U位高度必须是不小于 1 的整数且不能大于 100')
  })
})

import { beforeEach, expect, it, vi } from 'vitest'
import { collectDeviceExport } from './deviceExportData'
import type { Device } from '@/types/device'
const mocks = vi.hoisted(() => ({ listDevices: vi.fn(), getDevice: vi.fn() }))
vi.mock('@/api/device', () => mocks)
const rows = Array.from({ length: 205 }, (_, index) => ({ id: `id-${index}` } as Device))
beforeEach(() => vi.resetAllMocks())
it('fetches every page with identical filters', async () => {
  mocks.listDevices.mockImplementation(async ({ page, pageSize }) => ({ items: rows.slice((page - 1) * pageSize, page * pageSize), total: rows.length }))
  expect(await collectDeviceExport({ typeId: 'server', search: '主机' })).toEqual(rows)
  expect(mocks.listDevices).toHaveBeenCalledTimes(3)
  for (const [query] of mocks.listDevices.mock.calls) expect(query).toMatchObject({ typeId: 'server', search: '主机', pageSize: 100 })
})
it('rejects duplicate or incomplete pages', async () => {
  mocks.listDevices.mockResolvedValue({ items: rows.slice(0, 100), total: 205 })
  await expect(collectDeviceExport({})).rejects.toThrow('设备数据不完整')
})
it('propagates page failures without returning partial data', async () => {
  mocks.listDevices.mockResolvedValueOnce({ items: rows.slice(0, 100), total: 205 }).mockRejectedValue(new Error('网络失败'))
  await expect(collectDeviceExport({})).rejects.toThrow('网络失败')
})
it('reloads unique selected IDs across pages', async () => {
  mocks.getDevice.mockImplementation(async id => ({ id, name: '最新信息' }))
  expect(await collectDeviceExport({}, ['a', 'b', 'a'])).toEqual([{ id: 'a', name: '最新信息' }, { id: 'b', name: '最新信息' }])
  expect(mocks.getDevice).toHaveBeenCalledTimes(2)
  expect(mocks.listDevices).not.toHaveBeenCalled()
})
it('rejects deleted selections', async () => {
  mocks.getDevice.mockRejectedValue(new Error('设备不存在'))
  await expect(collectDeviceExport({}, ['deleted'])).rejects.toThrow('设备不存在')
})

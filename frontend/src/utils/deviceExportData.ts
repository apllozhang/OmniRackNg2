import { getDevice, listDevices, type DeviceQuery } from '@/api/device'
import type { Device } from '@/types/device'

// Fetch every page or reload selected IDs; never silently export partial results.
export async function collectDeviceExport(query: DeviceQuery, selectedIds?: string[]): Promise<Device[]> {
  if (selectedIds) {
    const rows: Device[] = []
    const ids = [...new Set(selectedIds)]
    for (let start = 0; start < ids.length; start += 5) {
      rows.push(...await Promise.all(ids.slice(start, start + 5).map(id => getDevice(id))))
    }
    return rows
  }
  const first = await listDevices({ ...query, page: 1, pageSize: 100 })
  const rows = [...first.items]
  const pages = Math.ceil(first.total / 100)
  for (let start = 2; start <= pages; start += 5) {
    const results = await Promise.all(Array.from({ length: Math.min(5, pages - start + 1) }, (_, index) =>
      listDevices({ ...query, page: start + index, pageSize: 100 })))
    if (results.some(result => result.total !== first.total)) throw new Error('导出期间设备数据已变更，请刷新后重试')
    results.forEach(result => rows.push(...result.items))
  }
  if (rows.length !== first.total || new Set(rows.map(item => item.id)).size !== first.total) {
    throw new Error('设备数据不完整，请刷新后重新导出')
  }
  return rows
}

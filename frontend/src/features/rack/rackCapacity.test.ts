import { describe, expect, it } from 'vitest'
import { analyzeRackCapacity } from './rackCapacity'
import type { Device, RackDevicePosition, RackULayout } from '@/types/device'
import type { Rack } from '@/types/resource'

const timestamp = '2026-09-01T00:00:00Z'

function createRack(overrides: Partial<Rack> = {}): Rack {
  return {
    id: 'rack-1', createdAt: timestamp, updatedAt: timestamp, version: 1,
    code: 'RACK-01', name: '01号机柜', status: 'PARTIAL', sortOrder: 0,
    dataCenterId: 'dc-1', roomId: 'room-1', type: 'STANDARD', uHeight: 42,
    widthMm: 600, depthMm: 1200, heightMm: 2000, loadCapacityKg: 1000,
    rotation: 0, dualPower: true, inputCircuits: 2, ratedPowerKw: 10,
    peakPowerKw: 12, pduCount: 2, ...overrides,
  }
}

function createDevice(id: string, overrides: Partial<Device> = {}): Device {
  return {
    id, createdAt: timestamp, updatedAt: timestamp, version: 1,
    typeId: 'server', code: id.toUpperCase(), name: `设备${id}`,
    lifecycleStatus: 'RUNNING', heightU: 2, weightKg: 100,
    ratedPowerW: 1000, peakPowerW: 1200, dualPowerRequired: true,
    ...overrides,
  }
}

function createPosition(device: Device, startU: number, heightU = device.heightU): RackDevicePosition {
  return {
    id: `position-${device.id}-${startU}`, createdAt: timestamp, updatedAt: timestamp, version: 1,
    deviceId: device.id, rackId: 'rack-1', roomId: 'room-1', dataCenterId: 'dc-1',
    startU, endU: startU + heightU - 1, heightU, orientation: 'NORMAL', installedAt: timestamp, device,
  }
}

function createLayout(rack: Rack, positions: RackDevicePosition[]): RackULayout {
  return { rack, positions, free: [] }
}

function metric(result: ReturnType<typeof analyzeRackCapacity>, key: string) {
  const found = result.metrics.find((item) => item.key === key)
  if (!found) throw new Error(`metric ${key} not found`)
  return found
}

describe('analyzeRackCapacity', () => {
  it('正常计算 U 位利用率', () => {
    const rack = createRack({ uHeight: 10 })
    const layout = createLayout(rack, [createPosition(createDevice('a'), 1, 2), createPosition(createDevice('b'), 5, 3)])
    const result = analyzeRackCapacity({ rack, layout, layoutState: 'loaded' })
    expect(metric(result, 'u')).toMatchObject({ status: 'ready', used: 5, capacity: 10, percentage: 50 })
  })

  it('U 位区间重叠时不重复统计', () => {
    const rack = createRack({ uHeight: 10 })
    const layout = createLayout(rack, [createPosition(createDevice('a'), 1, 3), createPosition(createDevice('b'), 3, 2)])
    expect(metric(analyzeRackCapacity({ rack, layout, layoutState: 'loaded' }), 'u').used).toBe(4)
  })

  it('额定功耗从 W 转为 kW 后计算', () => {
    const rack = createRack({ ratedPowerKw: 5 })
    const layout = createLayout(rack, [createPosition(createDevice('a', { ratedPowerW: 1000 }), 1), createPosition(createDevice('b', { ratedPowerW: 1500 }), 4)])
    expect(metric(analyzeRackCapacity({ rack, layout, layoutState: 'loaded' }), 'ratedPower')).toMatchObject({ status: 'ready', used: 2.5, capacity: 5, percentage: 50 })
  })

  it('机柜缺少额定功率时输出原因', () => {
    const rack = createRack({ ratedPowerKw: undefined })
    const layout = createLayout(rack, [createPosition(createDevice('a'), 1)])
    const result = metric(analyzeRackCapacity({ rack, layout, layoutState: 'loaded' }), 'ratedPower')
    expect(result.status).toBe('unavailable')
    expect(result.reasons).toContain('机柜未填写额定功率容量')
  })

  it('部分设备缺少额定功耗时输出设备名称', () => {
    const rack = createRack()
    const layout = createLayout(rack, [createPosition(createDevice('a', { name: '服务器A', ratedPowerW: undefined }), 1), createPosition(createDevice('b'), 4)])
    const result = metric(analyzeRackCapacity({ rack, layout, layoutState: 'loaded' }), 'ratedPower')
    expect(result.status).toBe('unavailable')
    expect(result.reasons).toContain('设备“服务器A”未填写额定功耗')
    expect(result.used).toBe(1)
  })

  it('设备缺少峰值功耗时输出原因', () => {
    const rack = createRack()
    const layout = createLayout(rack, [createPosition(createDevice('a', { name: '交换机A', peakPowerW: undefined }), 1)])
    expect(metric(analyzeRackCapacity({ rack, layout, layoutState: 'loaded' }), 'peakPower').reasons).toContain('设备“交换机A”未填写峰值功耗')
  })

  it('正常计算承重利用率', () => {
    const rack = createRack({ loadCapacityKg: 500 })
    const layout = createLayout(rack, [createPosition(createDevice('a', { weightKg: 100 }), 1), createPosition(createDevice('b', { weightKg: 150 }), 4)])
    expect(metric(analyzeRackCapacity({ rack, layout, layoutState: 'loaded' }), 'weight')).toMatchObject({ status: 'ready', used: 250, capacity: 500, percentage: 50 })
  })

  it('设备缺少重量时输出原因', () => {
    const rack = createRack()
    const layout = createLayout(rack, [createPosition(createDevice('a', { name: '存储A', weightKg: undefined }), 1)])
    expect(metric(analyzeRackCapacity({ rack, layout, layoutState: 'loaded' }), 'weight').reasons).toContain('设备“存储A”未填写重量')
  })

  it('PDU 利用率不伪造并说明缺少插座容量和占用数据', () => {
    const rack = createRack({ pduCount: 0 })
    const layout = createLayout(rack, [])
    const result = metric(analyzeRackCapacity({ rack, layout, layoutState: 'loaded' }), 'pdu')
    expect(result.status).toBe('unavailable')
    expect(result.percentage).toBeNull()
    expect(result.reasons).toEqual(['机柜未填写 PDU 数量', '系统未维护 PDU 插座总数和设备实际占用插座数'])
  })

  it('超过容量时保留真实百分比并标记超限', () => {
    const rack = createRack({ ratedPowerKw: 1 })
    const layout = createLayout(rack, [createPosition(createDevice('a', { ratedPowerW: 1500 }), 1)])
    expect(metric(analyzeRackCapacity({ rack, layout, layoutState: 'loaded' }), 'ratedPower')).toMatchObject({ percentage: 150, overCapacity: true })
  })

  it('U 位布局加载失败时所有依赖设备的指标都输出原因', () => {
    const rack = createRack()
    const result = analyzeRackCapacity({ rack, layout: null, layoutState: 'error' })
    expect(metric(result, 'u').reasons).toContain('U 位布局读取失败，无法确认已安装设备')
    expect(metric(result, 'ratedPower').reasons).toContain('U 位布局读取失败，无法确认已安装设备')
  })
})

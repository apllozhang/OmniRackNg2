import type { Device, RackDevicePosition, RackULayout } from '@/types/device'
import type { Rack } from '@/types/resource'

export type RackLayoutLoadState = 'loading' | 'loaded' | 'error' | undefined
export type RackMetricStatus = 'ready' | 'unavailable'

export interface RackCapacityMetric {
  key: 'u' | 'ratedPower' | 'peakPower' | 'weight' | 'pdu'
  label: string
  status: RackMetricStatus
  percentage: number | null
  used: number | null
  capacity: number | null
  unit: string
  reasons: string[]
  overCapacity: boolean
}

export interface RackCapacityAnalysis {
  metrics: RackCapacityMetric[]
  installedDeviceCount: number
  dualPowerDeviceCount: number
  devices: Device[]
}

interface AnalyzeOptions {
  rack: Rack
  layout?: RackULayout | null
  layoutState?: RackLayoutLoadState
}

function roundPercentage(value: number) {
  return Math.round(value * 10) / 10
}

function readyMetric(
  key: RackCapacityMetric['key'],
  label: string,
  used: number,
  capacity: number,
  unit: string,
): RackCapacityMetric {
  const percentage = roundPercentage((used / capacity) * 100)
  return { key, label, status: 'ready', percentage, used, capacity, unit, reasons: [], overCapacity: percentage > 100 }
}

function unavailableMetric(
  key: RackCapacityMetric['key'],
  label: string,
  unit: string,
  reasons: string[],
  used: number | null = null,
  capacity: number | null = null,
): RackCapacityMetric {
  return { key, label, status: 'unavailable', percentage: null, used, capacity, unit, reasons, overCapacity: false }
}

function layoutUnavailableReason(state: RackLayoutLoadState) {
  if (state === 'loading') return 'U 位布局正在加载，暂时无法读取已安装设备'
  if (state === 'error') return 'U 位布局读取失败，无法确认已安装设备'
  return '尚未读取 U 位布局，无法确认已安装设备'
}

function installedDevices(layout?: RackULayout | null) {
  const result: Device[] = []
  const seen = new Set<string>()
  for (const position of layout?.positions ?? []) {
    if (seen.has(position.deviceId)) continue
    seen.add(position.deviceId)
    if (position.device) result.push(position.device)
  }
  return result
}

function missingDeviceDetailReasons(layout: RackULayout | null | undefined) {
  const missing = new Set<string>()
  for (const position of layout?.positions ?? []) {
    if (!position.device) missing.add(position.deviceId)
  }
  return [...missing].map((deviceId) => `设备“${deviceId}”详细信息未加载`)
}

function missingFieldDeviceNames(devices: Device[], field: keyof Device) {
  return devices
    .filter((device) => {
      const value = device[field]
      return typeof value !== 'number' || !Number.isFinite(value) || value <= 0
    })
    .map((device) => device.name || device.code || device.id)
}

function aggregateMetric(options: {
  rackCapacity: number | undefined
  rackMissingReason: string
  devices: Device[]
  deviceField: 'ratedPowerW' | 'peakPowerW' | 'weightKg'
  deviceMissingLabel: string
  key: 'ratedPower' | 'peakPower' | 'weight'
  label: string
  unit: string
  divisor?: number
  layout?: RackULayout | null
  layoutState?: RackLayoutLoadState
}) {
  const {
    rackCapacity, rackMissingReason, devices, deviceField, deviceMissingLabel,
    key, label, unit, divisor = 1, layout, layoutState,
  } = options
  const reasons: string[] = []
  if (layoutState !== 'loaded' || !layout) reasons.push(layoutUnavailableReason(layoutState))
  else reasons.push(...missingDeviceDetailReasons(layout))
  if (!Number.isFinite(rackCapacity) || Number(rackCapacity) <= 0) reasons.push(rackMissingReason)

  const missingNames = missingFieldDeviceNames(devices, deviceField)
  if (missingNames.length) reasons.push(`设备“${missingNames.join('、')}”未填写${deviceMissingLabel}`)

  const completeValues = devices
    .map((device) => device[deviceField])
    .filter((value): value is number => typeof value === 'number' && Number.isFinite(value) && value > 0)
  const used = completeValues.reduce((sum, value) => sum + value / divisor, 0)
  const capacity = Number.isFinite(rackCapacity) && Number(rackCapacity) > 0 ? Number(rackCapacity) : null

  if (reasons.length) return unavailableMetric(key, label, unit, reasons, layoutState === 'loaded' ? used : null, capacity)
  return readyMetric(key, label, used, Number(rackCapacity), unit)
}

function uMetric(rack: Rack, layout?: RackULayout | null, layoutState?: RackLayoutLoadState) {
  const reasons: string[] = []
  if (layoutState !== 'loaded' || !layout) reasons.push(layoutUnavailableReason(layoutState))
  if (!Number.isFinite(rack.uHeight) || rack.uHeight <= 0) reasons.push('机柜未填写有效的 U 位高度')

  const occupied = new Set<number>()
  if (layout) {
    for (const position of layout.positions) {
      const start = Math.max(1, Math.min(position.startU, position.endU))
      const end = Math.min(rack.uHeight, Math.max(position.startU, position.endU))
      for (let u = start; u <= end; u += 1) occupied.add(u)
    }
  }
  if (reasons.length) return unavailableMetric('u', 'U 位利用率', 'U', reasons, layout ? occupied.size : null, rack.uHeight > 0 ? rack.uHeight : null)
  return readyMetric('u', 'U 位利用率', occupied.size, rack.uHeight, 'U')
}

export function analyzeRackCapacity({ rack, layout, layoutState }: AnalyzeOptions): RackCapacityAnalysis {
  const devices = installedDevices(layout)
  const metrics: RackCapacityMetric[] = [
    uMetric(rack, layout, layoutState),
    aggregateMetric({
      rackCapacity: rack.ratedPowerKw,
      rackMissingReason: '机柜未填写额定功率容量',
      devices,
      deviceField: 'ratedPowerW',
      deviceMissingLabel: '额定功耗',
      key: 'ratedPower',
      label: '额定功耗利用率',
      unit: 'kW',
      divisor: 1000,
      layout,
      layoutState,
    }),
    aggregateMetric({
      rackCapacity: rack.peakPowerKw,
      rackMissingReason: '机柜未填写峰值功率容量',
      devices,
      deviceField: 'peakPowerW',
      deviceMissingLabel: '峰值功耗',
      key: 'peakPower',
      label: '峰值功耗利用率',
      unit: 'kW',
      divisor: 1000,
      layout,
      layoutState,
    }),
    aggregateMetric({
      rackCapacity: rack.loadCapacityKg,
      rackMissingReason: '机柜未填写承重上限',
      devices,
      deviceField: 'weightKg',
      deviceMissingLabel: '重量',
      key: 'weight',
      label: '承重利用率',
      unit: 'kg',
      layout,
      layoutState,
    }),
    unavailableMetric('pdu', 'PDU 端口利用率', '端口', [
      ...(rack.pduCount > 0 ? [] : ['机柜未填写 PDU 数量']),
      '系统未维护 PDU 插座总数和设备实际占用插座数',
    ]),
  ]

  return {
    metrics,
    installedDeviceCount: layout?.positions.length ?? 0,
    dualPowerDeviceCount: devices.filter((device) => device.dualPowerRequired).length,
    devices,
  }
}

export function formatRackMetricValue(value: number | null, unit: string) {
  if (value === null) return '-'
  const maximumFractionDigits = unit === 'U' || unit === '端口' ? 0 : 2
  return `${value.toLocaleString('zh-CN', { maximumFractionDigits })}${unit}`
}

export function rackMetricTone(metric: RackCapacityMetric) {
  if (metric.status === 'unavailable') return 'unavailable'
  if (metric.overCapacity || Number(metric.percentage) >= 90) return 'danger'
  if (Number(metric.percentage) >= 70) return 'warning'
  return 'normal'
}

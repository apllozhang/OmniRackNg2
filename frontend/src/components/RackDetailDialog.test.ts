import ElementPlus from 'element-plus'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, describe, expect, it } from 'vitest'
import RackDetailDialog from './RackDetailDialog.vue'
import type { Device, RackDevicePosition, RackULayout } from '@/types/device'
import type { Rack } from '@/types/resource'

const timestamp = '2026-09-01T00:00:00Z'
const rack: Rack = {
  id: 'rack-1', createdAt: timestamp, updatedAt: timestamp, version: 1,
  code: 'RACK-A01', name: '01号机柜', status: 'PARTIAL', sortOrder: 0,
  dataCenterId: 'dc-1', roomId: 'room-1', type: 'STANDARD', manufacturer: '测试厂商', modelNumber: 'M42',
  uHeight: 4, widthMm: 600, depthMm: 1200, heightMm: 2000, loadCapacityKg: 800,
  zone: 'A区', row: '1', column: '1', rotation: 0, dualPower: true, inputCircuits: 2,
  ratedVoltage: 220, ratedCurrent: 32, ratedPowerKw: 10, peakPowerKw: 12, pduCount: 2,
}
const device: Device = {
  id: 'device-1', createdAt: timestamp, updatedAt: timestamp, version: 1,
  typeId: 'server', code: 'DEV-001', name: '业务服务器A', serialNumber: 'SN001',
  lifecycleStatus: 'RUNNING', heightU: 2, ratedPowerW: 1000,
  dualPowerRequired: true,
}
const position: RackDevicePosition = {
  id: 'position-1', createdAt: timestamp, updatedAt: timestamp, version: 1,
  deviceId: device.id, rackId: rack.id, roomId: 'room-1', dataCenterId: 'dc-1',
  startU: 2, endU: 3, heightU: 2, orientation: 'NORMAL', installedAt: timestamp, device,
}
const layout: RackULayout = { rack, positions: [position], free: [{ startU: 1, endU: 1, heightU: 1 }, { startU: 4, endU: 4, heightU: 1 }] }

afterEach(() => { document.body.innerHTML = '' })

describe('RackDetailDialog', () => {
  it('左侧展示机柜图，右侧展示参数、利用率和无法计算原因', async () => {
    const wrapper = mount(RackDetailDialog, {
      attachTo: document.body,
      props: { modelValue: true, rack, layout, layoutState: 'loaded', roomName: 'A机房' },
      global: { plugins: [ElementPlus] },
    })
    await flushPromises()

    const text = document.body.textContent?.replace(/\s+/g, '') ?? ''
    expect(text).toContain('机柜详情与容量分析')
    expect(text).toContain('完整U位图')
    expect(text).toContain('业务服务器A')
    expect(text).toContain('机柜参数')
    expect(text).toContain('U位利用率')
    expect(text).toContain('50%')
    expect(text).toContain('额定功耗利用率')
    expect(text).toContain('10%')
    expect(text).toContain('无法计算')
    expect(text).toContain('设备“业务服务器A”未填写峰值功耗')
    expect(text).toContain('设备“业务服务器A”未填写重量')
    expect(text).toContain('系统未维护PDU插座总数和设备实际占用插座数')
    expect(text).toContain('PDU与供电信息')

    await wrapper.findAllComponents({ name: 'ElButton' }).find((button) => button.text().includes('编辑机柜'))?.trigger('click')
    expect(wrapper.emitted('edit')?.[0]?.[0]).toEqual(rack)
    wrapper.unmount()
  })

  it('布局读取失败时显示机柜图和利用率的明确失败原因', async () => {
    const wrapper = mount(RackDetailDialog, {
      attachTo: document.body,
      props: { modelValue: true, rack, layout: null, layoutState: 'error', roomName: 'A机房' },
      global: { plugins: [ElementPlus] },
    })
    await flushPromises()
    expect(document.body.textContent).toContain('U 位布局读取失败，左侧机柜图暂时无法展示')
    expect(document.body.textContent).toContain('U 位布局读取失败，无法确认已安装设备')
    wrapper.unmount()
  })
})

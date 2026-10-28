import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import DeviceDetailFields from '@/components/DeviceDetailFields.vue'
import { deviceEditableFieldKeys, deviceFieldLabels } from '@/features/device/deviceForm'
import type { Device, RackDevicePosition } from '@/types/device'

const timestamp = '2026-09-04T08:00:00Z'

function fullDevice(): Device {
  return {
    id: 'device-1', createdAt: timestamp, updatedAt: timestamp, version: 7,
    typeId: 'type-1',
    type: {
      id: 'type-1', createdAt: timestamp, updatedAt: timestamp, version: 1,
      code: 'SERVER', name: '服务器', category: 'SERVER', status: 'ACTIVE',
      defaultHeightU: 2, defaultDualPower: true, sortOrder: 1,
    },
    code: 'DEV-001', name: '核心服务器', assetNumber: 'ASSET-001', serialNumber: 'SN-001',
    manufacturer: '示例厂商', modelNumber: 'X1', specification: '双路 CPU / 512GB 内存',
    firmwareVersion: 'V1.2', purchaseBatch: '2026-Q3', warrantyExpiresAt: '2029-08-31T00:00:00Z',
    organization: '技术中心', manager: '张三', contact: '13800000000', businessSystem: '门户',
    applicationName: '门户应用', lifecycleStatus: 'RUNNING', heightU: 2, widthMm: 440, depthMm: 800,
    heightMm: 88, weightKg: 25.5, ratedPowerW: 800, peakPowerW: 1200, inputVoltage: 220,
    dualPowerRequired: true, managementIp: '10.0.0.1', businessIp: '172.16.0.1',
    macAddress: '00:11:22:33:44:55', managementProtocol: 'SSH', monitoringStatus: 'ONLINE',
    externalQrCode: 'QR-001', externalQrCodeUrl: 'https://example.test/qr/1', tags: '核心,生产', remarks: '重点保障设备',
  }
}

function position(): RackDevicePosition {
  return {
    id: 'position-1', createdAt: timestamp, updatedAt: timestamp, version: 1,
    deviceId: 'device-1', rackId: 'rack-1', roomId: 'room-1', dataCenterId: 'dc-1',
    startU: 2, endU: 3, heightU: 2, orientation: 'NORMAL', installedAt: timestamp, reason: '首次上架',
  }
}

describe('DeviceDetailFields', () => {
  it('renders the complete shared backend device field contract with formatted values', () => {
    const wrapper = mount(DeviceDetailFields, {
      props: { device: fullDevice(), position: position(), rackName: '01号机柜' },
    })
    const text = wrapper.text()

    for (const key of deviceEditableFieldKeys) expect(text).toContain(deviceFieldLabels[key])
    for (const value of [
      '采购批次', '2026-Q3', '保修到期日', '2029-08-31', '固件版本', 'V1.2',
      '应用名称', '门户应用', 'MAC 地址', '00:11:22:33:44:55', '管理协议', 'SSH',
      '监控状态', 'ONLINE', '外部二维码', 'QR-001', '标签', '核心,生产', '备注', '重点保障设备',
      '2U', '440 mm', '25.5 kg', '800 W', '01号机柜 · 2-3U', '首次上架',
      '系统信息', '设备 ID', 'device-1',
    ]) expect(text).toContain(value)
  })

  it('shows placeholders for optional empty fields and an unmounted position', () => {
    const device = fullDevice()
    device.assetNumber = ''
    device.serialNumber = undefined
    device.currentPosition = undefined
    const wrapper = mount(DeviceDetailFields, { props: { device } })

    expect(wrapper.text()).toContain('未上架')
    const emptyValues = wrapper.findAll('.device-detail-grid > div')
      .filter((item) => ['资产编号', '序列号'].some((label) => item.text().includes(label)))
      .map((item) => item.find('b').text())
    expect(emptyValues).toEqual(['-', '-'])
  })
})

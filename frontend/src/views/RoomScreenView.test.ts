import ElementPlus, { ElMessage, ElMessageBox } from 'element-plus'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import RoomScreenView from '@/views/RoomScreenView.vue'
import type { Device, RackDevicePosition, RackULayout } from '@/types/device'
import type { DataCenter, Rack, Room } from '@/types/resource'

const mocks = vi.hoisted(() => ({
  getResourceTree: vi.fn(),
  getRackLayout: vi.fn(),
  listDevices: vi.fn(),
  getDevice: vi.fn(),
  listDeviceTypes: vi.fn(),
  updateDevice: vi.fn(),
  assignDevice: vi.fn(),
  moveDevice: vi.fn(),
  decommissionDevice: vi.fn(),
  createRack: vi.fn(),
  updateRack: vi.fn(),
  deleteRack: vi.fn(),
  routerPush: vi.fn(),
}))

vi.mock('@/api/resource', () => ({
  getResourceTree: mocks.getResourceTree,
  createRack: mocks.createRack,
  updateRack: mocks.updateRack,
  deleteRack: mocks.deleteRack,
}))

vi.mock('@/api/device', () => ({
  getRackLayout: mocks.getRackLayout,
  listDevices: mocks.listDevices,
  getDevice: mocks.getDevice,
  listDeviceTypes: mocks.listDeviceTypes,
  updateDevice: mocks.updateDevice,
  assignDevice: mocks.assignDevice,
  moveDevice: mocks.moveDevice,
  decommissionDevice: mocks.decommissionDevice,
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: mocks.routerPush }),
}))

const timestamp = '2026-08-31T00:00:00Z'

function rack(id: string, code: string, name: string, status: Rack['status'], sortOrder: number): Rack {
  return {
    id,
    createdAt: timestamp,
    updatedAt: timestamp,
    version: 3,
    code,
    name,
    status,
    sortOrder,
    dataCenterId: 'dc-1',
    roomId: 'room-1',
    type: 'STANDARD',
    uHeight: 4,
    widthMm: 600,
    depthMm: 1200,
    heightMm: 2000,
    rotation: 0,
    dualPower: true,
    inputCircuits: 2,
    pduCount: 2,
  }
}

const racks = [
  rack('rack-1', 'RACK-01', '01号机柜', 'PARTIAL', 0),
  rack('rack-2', 'RACK-02', '02号机柜', 'AVAILABLE', 1),
  rack('rack-3', 'RACK-03', '03号机柜', 'MAINTENANCE', 2),
]

const device: Device = {
  id: 'device-1',
  createdAt: timestamp,
  updatedAt: timestamp,
  version: 5,
  typeId: 'type-server',
  type: {
    id: 'type-server',
    createdAt: timestamp,
    updatedAt: timestamp,
    version: 1,
    code: 'SERVER',
    name: '服务器',
    category: 'SERVER',
    status: 'ACTIVE',
    defaultHeightU: 2,
    defaultDualPower: true,
    sortOrder: 0,
  },
  code: 'DEV-001',
  name: '业务服务器01',
  lifecycleStatus: 'RUNNING',
  heightU: 2,
  dualPowerRequired: true,
  managementIp: '10.0.0.10',
}

const position: RackDevicePosition = {
  id: 'position-1',
  createdAt: timestamp,
  updatedAt: timestamp,
  version: 1,
  deviceId: device.id,
  rackId: racks[0].id,
  roomId: 'room-1',
  dataCenterId: 'dc-1',
  startU: 2,
  endU: 3,
  heightU: 2,
  orientation: 'NORMAL',
  installedAt: timestamp,
  device,
  rack: racks[0],
}

const room: Room = {
  id: 'room-1',
  createdAt: timestamp,
  updatedAt: timestamp,
  version: 7,
  code: 'ROOM-A',
  name: 'A机房',
  status: 'OPERATING',
  sortOrder: 0,
  dataCenterId: 'dc-1',
  building: '1号楼',
  floor: '2层',
  floorPlanEnabled: true,
  floorPlanFormat: 'GRID',
  racksPerRow: 2,
  racks,
}

const dataCenter: DataCenter = {
  id: 'dc-1',
  createdAt: timestamp,
  updatedAt: timestamp,
  version: 2,
  code: 'DC-A',
  name: '测试数据中心',
  status: 'OPERATING',
  sortOrder: 0,
  rooms: [room],
}

function rackLayout(rackItem: Rack): RackULayout {
  return {
    rack: rackItem,
    positions: rackItem.id === 'rack-1' ? [position] : [],
    free: [],
  }
}

async function mountScreen(): Promise<VueWrapper> {
  const wrapper = mount(RoomScreenView, {
    attachTo: document.body,
    global: { plugins: [ElementPlus] },
  })
  await flushPromises()
  return wrapper
}

function dispatchPointer(
  element: Element,
  type: string,
  init: PointerEventInit & { pointerId: number },
) {
  element.dispatchEvent(new PointerEvent(type, { bubbles: true, cancelable: true, ...init }))
}

beforeEach(() => {
  mocks.getResourceTree.mockImplementation(async () => ({ items: [structuredClone(dataCenter)] }))
  mocks.getRackLayout.mockImplementation(async (rackId: string) => {
    const target = racks.find((item) => item.id === rackId)
    if (!target) throw new Error('rack not found')
    return rackLayout(target)
  })
  mocks.listDevices.mockResolvedValue({ items: [], total: 0, page: 1, pageSize: 100 })
  mocks.getDevice.mockResolvedValue(structuredClone(device))
  mocks.listDeviceTypes.mockResolvedValue({ items: [structuredClone(device.type)] })
  mocks.updateDevice.mockImplementation(async (id: string, payload: Partial<Device>) => ({ ...structuredClone(device), ...payload, id, version: device.version + 1 }))
  mocks.updateRack.mockImplementation(async (rackId: string, payload: Partial<Rack>) => {
    const original = racks.find((item) => item.id === rackId)
    if (!original) throw new Error('rack not found')
    return { ...original, ...payload, id: rackId, version: original.version + 1 }
  })
})

afterEach(() => {
  document.body.classList.remove('is-rack-dragging')
  document.body.innerHTML = ''
})

describe('RoomScreenView', () => {
  it.each([false,true])('机房大屏按下架结果显示待审批或执行成功：%s',async executed=>{
    const prompt=vi.spyOn(ElMessageBox,'prompt').mockImplementation(async()=>({value:'设备退役',action:'confirm'} as never))
    const message=vi.spyOn(ElMessage,'success')
    mocks.decommissionDevice.mockResolvedValue({executed})
    const w=await mountScreen()
    await w.find('.device-block').trigger('click',{clientX:320,clientY:180});await flushPromises()
    const button=Array.from(document.body.querySelectorAll('.rack-context-menu button')).find(b=>b.textContent?.includes('下架设备')) as HTMLButtonElement
    button.click();await flushPromises()
    expect(mocks.decommissionDevice).toHaveBeenCalled()
    expect(message).toHaveBeenCalledWith(executed?'设备已下架':'下架申请已提交，审批通过前保留原 U 位')
    w.unmount();prompt.mockRestore();message.mockRestore()
  })
  it('加载机房、机柜和 U 位数据，并按 racksPerRow 生成固定不重叠槽位', async () => {
    const wrapper = await mountScreen()

    expect(mocks.getResourceTree).toHaveBeenCalledTimes(1)
    expect(mocks.getRackLayout).toHaveBeenCalledTimes(3)
    expect(wrapper.findAll('.rack-card')).toHaveLength(3)
    expect(wrapper.findAll('.layout-row-guide')).toHaveLength(2)
    expect(wrapper.find('.canvas-info').text()).toContain('2 排 · 3 个机柜')
    expect(wrapper.find('.stats-grid').text().replace(/\s+/g, '')).toContain('2/12U')

    const positions = wrapper.findAll('.rack-card').map((card) => ({
      left: card.element.getAttribute('style')?.match(/left:\s*([^;]+)/)?.[1],
      top: card.element.getAttribute('style')?.match(/top:\s*([^;]+)/)?.[1],
    }))
    expect(new Set(positions.map(({ left, top }) => `${left}-${top}`)).size).toBe(3)
    expect(positions[0].top).toBe(positions[1].top)
    expect(positions[2].top).not.toBe(positions[0].top)

    wrapper.unmount()
  })

  it('左键点击已有设备会打开设备快捷菜单', async () => {
    const wrapper = await mountScreen()

    await wrapper.find('.device-block').trigger('click', { clientX: 320, clientY: 180 })
    await flushPromises()

    const menu = document.body.querySelector('.rack-context-menu')
    expect(menu).not.toBeNull()
    expect(menu?.textContent).toContain('设备快捷操作')
    expect(menu?.textContent).toContain('业务服务器01')
    expect(menu?.textContent).toContain('查看设备详情')
    expect(menu?.textContent).toContain('调整 / 迁移位置')
    expect(menu?.textContent).toContain('下架设备')

    wrapper.unmount()
  })

  it('查看设备详情会读取后端最新数据并展示统一完整字段', async () => {
    mocks.getDevice.mockResolvedValueOnce({
      ...structuredClone(device),
      assetNumber: 'ASSET-001', serialNumber: 'SN-001', manufacturer: '示例厂商', modelNumber: 'X1',
      specification: '双路 CPU', firmwareVersion: 'V1.2', purchaseBatch: '2026-Q3',
      warrantyExpiresAt: '2029-08-31T00:00:00Z', organization: '技术中心', manager: '张三',
      contact: '13800000000', businessSystem: '门户', applicationName: '门户应用',
      widthMm: 440, depthMm: 800, heightMm: 88, weightKg: 25.5, ratedPowerW: 800,
      peakPowerW: 1200, inputVoltage: 220, businessIp: '172.16.0.1',
      macAddress: '00:11:22:33:44:55', managementProtocol: 'SSH', monitoringStatus: 'ONLINE',
      externalQrCode: 'QR-001', externalQrCodeUrl: 'https://example.test/qr/1',
      tags: '核心,生产', remarks: '重点保障设备',
    })
    const wrapper = await mountScreen()

    await wrapper.find('.device-block').trigger('click', { clientX: 320, clientY: 180 })
    await flushPromises()
    const menu = document.body.querySelector('.rack-context-menu')
    const detailButton = Array.from(menu?.querySelectorAll('button') ?? [])
      .find((button) => button.textContent?.includes('查看设备详情'))
    expect(detailButton).toBeTruthy()

    detailButton?.click()
    await flushPromises()

    const text = document.body.textContent?.replace(/\s+/g, '') ?? ''
    expect(mocks.getDevice).toHaveBeenCalledWith('device-1')
    for (const value of [
      '设备详细信息', '资产与规格', '采购批次', '2026-Q3', '保修到期日', '2029-08-31',
      '固件版本', 'V1.2', '应用名称', '门户应用', 'MAC地址', '00:11:22:33:44:55',
      '管理协议', 'SSH', '监控状态', 'ONLINE', '外部二维码', 'QR-001', '标签', '核心,生产',
      '尺寸与电力', '800W', '上架与位置', '01号机柜（RACK-01）·2-3U', '系统信息',
    ]) expect(text).toContain(value.replace(/\s+/g, ''))

    wrapper.unmount()
  })

  it('设备快捷菜单可以打开编辑弹窗并保存设备信息', async () => {
    const wrapper = await mountScreen()

    await wrapper.find('.device-block').trigger('click', { clientX: 320, clientY: 180 })
    await flushPromises()
    const menu = document.body.querySelector('.rack-context-menu')
    const editButton = Array.from(menu?.querySelectorAll('button') ?? []).find((button) => button.textContent?.includes('编辑设备信息'))
    expect(editButton).toBeTruthy()

    editButton?.click()
    await flushPromises()
    expect(document.body.textContent).toContain('编辑设备信息')
    expect(document.body.textContent).toContain('设备编码')
    expect(document.body.textContent).toContain('额定功率(W)')

    const nameItem = Array.from(document.querySelectorAll('.screen-device-editor-dialog .el-form-item'))
      .find((item) => item.textContent?.includes('设备名称'))
    const nameInput = nameItem?.querySelector('input') as HTMLInputElement | null
    expect(nameInput).not.toBeNull()
    nameInput!.value = '更新后的业务服务器'
    nameInput!.dispatchEvent(new Event('input', { bubbles: true }))
    await flushPromises()

    const dialog = document.querySelector('.screen-device-editor-dialog')
    const saveButton = Array.from(dialog?.querySelectorAll('button') ?? []).find((button) => button.textContent?.includes('保存设备'))
    expect(saveButton).toBeTruthy()
    saveButton?.click()
    await flushPromises()

    expect(mocks.updateDevice).toHaveBeenCalledWith('device-1', expect.objectContaining({
      name: '更新后的业务服务器',
      code: 'DEV-001',
      version: 5,
    }))
    expect(document.body.textContent).toContain('设备信息已保存')
    expect(mocks.getResourceTree).toHaveBeenCalledTimes(2)

    wrapper.unmount()
  })

  it('左键点击空 U 位会打开上架操作菜单', async () => {
    const wrapper = await mountScreen()
    const secondRack = wrapper.findAll('.rack-card')[1]

    await secondRack.find('.u-line').trigger('click', { clientX: 420, clientY: 220 })
    await flushPromises()

    const menu = document.body.querySelector('.rack-context-menu')
    expect(menu).not.toBeNull()
    expect(menu?.textContent).toContain('空 U 位快捷操作')
    expect(menu?.textContent).toContain('02号机柜 · 4U')
    expect(menu?.textContent).toContain('上架设备到此 U 位')
    expect(menu?.textContent).toContain('查看完整 U 位详情')

    wrapper.unmount()
  })

  it('点击机柜卡片详情会打开双栏详情并展示无法计算原因', async () => {
    const wrapper = await mountScreen()

    await wrapper.findAll('.rack-card-footer button')[0].trigger('click')
    await flushPromises()

    const text = document.body.textContent?.replace(/\s+/g, '') ?? ''
    expect(text).toContain('机柜详情与容量分析')
    expect(text).toContain('完整U位图')
    expect(text).toContain('业务服务器01')
    expect(text).toContain('机柜参数')
    expect(text).toContain('容量与利用率')
    expect(text).toContain('无法计算')
    expect(text).toContain('机柜未填写额定功率容量')
    expect(text).toContain('系统未维护PDU插座总数和设备实际占用插座数')

    wrapper.unmount()
  })

  it('点击机柜状态标签会打开机柜编辑弹窗', async () => {
    const wrapper = await mountScreen()

    await wrapper.findAll('.rack-status-chip')[1].trigger('click')
    await flushPromises()

    expect(document.body.textContent).toContain('编辑机柜')
    expect(document.body.textContent).toContain('RACK-02')
    expect(document.body.textContent).toContain('02号机柜')

    wrapper.unmount()
  })

  it('拖动机柜到已占用槽位时交换两个机柜并持久化坐标', async () => {
    const wrapper = await mountScreen()
    const canvas = wrapper.find('.room-layout-canvas').element as HTMLElement
    Object.defineProperty(canvas, 'scrollWidth', { configurable: true, value: 900 })
    Object.defineProperty(canvas, 'scrollHeight', { configurable: true, value: 600 })
    vi.spyOn(canvas, 'getBoundingClientRect').mockReturnValue({
      x: 0,
      y: 0,
      left: 0,
      top: 0,
      right: 900,
      bottom: 600,
      width: 900,
      height: 600,
      toJSON: () => ({}),
    })

    const firstHandle = wrapper.findAll('.rack-drag-handle')[0].element
    dispatchPointer(firstHandle, 'pointerdown', { pointerId: 9, button: 0, clientX: 48, clientY: 82 })
    dispatchPointer(canvas, 'pointermove', { pointerId: 9, button: 0, clientX: 294, clientY: 82 })
    dispatchPointer(canvas, 'pointerup', { pointerId: 9, button: 0, clientX: 294, clientY: 82 })
    await flushPromises()

    expect(mocks.updateRack).toHaveBeenCalledTimes(2)
    expect(mocks.updateRack).toHaveBeenCalledWith('rack-1', expect.objectContaining({ xCoordinate: 284, yCoordinate: 72 }))
    expect(mocks.updateRack).toHaveBeenCalledWith('rack-2', expect.objectContaining({ xCoordinate: 38, yCoordinate: 72 }))

    wrapper.unmount()
  })
})




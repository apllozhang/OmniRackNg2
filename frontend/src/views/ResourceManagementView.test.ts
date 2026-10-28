import ElementPlus from 'element-plus'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import ResourceManagementView from '@/views/ResourceManagementView.vue'
import type { DataCenter, Rack, Room } from '@/types/resource'

const mocks = vi.hoisted(() => ({
  getResourceTree: vi.fn(), getRackTemplates: vi.fn(),
  copyDataCenter: vi.fn(), copyRoom: vi.fn(), copyRack: vi.fn(), moveRoom: vi.fn(), moveRack: vi.fn(),
  createDataCenter: vi.fn(), createRoom: vi.fn(), createRack: vi.fn(), updateDataCenter: vi.fn(), updateRoom: vi.fn(), updateRack: vi.fn(),
  deleteDataCenter: vi.fn(), deleteRoom: vi.fn(), deleteRack: vi.fn(),
}))

vi.mock('@/api/resource', () => ({
  getResourceTree: mocks.getResourceTree, copyDataCenter: mocks.copyDataCenter, copyRoom: mocks.copyRoom, copyRack: mocks.copyRack,
  moveRoom: mocks.moveRoom, moveRack: mocks.moveRack, createDataCenter: mocks.createDataCenter, createRoom: mocks.createRoom,
  createRack: mocks.createRack, updateDataCenter: mocks.updateDataCenter, updateRoom: mocks.updateRoom, updateRack: mocks.updateRack,
  deleteDataCenter: mocks.deleteDataCenter, deleteRoom: mocks.deleteRoom, deleteRack: mocks.deleteRack,
}))
vi.mock('@/api/rack-template', () => ({ getRackTemplates: mocks.getRackTemplates }))

const timestamp = '2026-09-04T08:00:00Z'
const rack: Rack = { id: 'rack-1', createdAt: timestamp, updatedAt: timestamp, version: 4, code: 'RACK-01', name: 'A01', status: 'PARTIAL', sortOrder: 0, dataCenterId: 'dc-1', roomId: 'room-1', type: 'STANDARD', uHeight: 42, widthMm: 600, depthMm: 1200, heightMm: 2000, rotation: 0, dualPower: true, inputCircuits: 2, pduCount: 2 }
const room: Room = { id: 'room-1', createdAt: timestamp, updatedAt: timestamp, version: 3, code: 'ROOM-01', name: '一号机房', status: 'OPERATING', sortOrder: 0, dataCenterId: 'dc-1', floorPlanEnabled: false, floorPlanFormat: 'SVG', racksPerRow: 8, racks: [rack] }
const targetRoom: Room = { ...room, id: 'room-2', code: 'ROOM-02', name: '二号机房', dataCenterId: 'dc-2', racks: [] }
const sourceDC: DataCenter = { id: 'dc-1', createdAt: timestamp, updatedAt: timestamp, version: 2, code: 'DC-01', name: '主中心', status: 'OPERATING', sortOrder: 0, rooms: [room] }
const targetDC: DataCenter = { ...sourceDC, id: 'dc-2', code: 'DC-02', name: '灾备中心', rooms: [targetRoom] }

beforeEach(() => {
  mocks.getResourceTree.mockResolvedValue({ items: [sourceDC, targetDC] })
  mocks.getRackTemplates.mockResolvedValue({ items: [] })
  mocks.copyDataCenter.mockResolvedValue({ resource: { ...sourceDC, id: 'dc-copy' }, copiedRooms: 1, copiedRacks: 1 })
  mocks.moveRack.mockResolvedValue({ ...rack, roomId: 'room-2', dataCenterId: 'dc-2', version: 5 })
})
afterEach(() => { document.body.innerHTML = ''; vi.clearAllMocks() })

function mountView() {
  return mount(ResourceManagementView, { attachTo: document.body, global: { plugins: [ElementPlus] } })
}

describe('ResourceManagementView copy and cross-location move', () => {
  it('submits a data-center structural copy with auto-generated code', async () => {
    const wrapper = mountView(); await flushPromises()
    ;(wrapper.vm as unknown as { openTransfer: Function }).openTransfer('data-center', 'copy', sourceDC)
    await flushPromises()
    expect(document.body.textContent).toContain('不会复制设备、PDU 实例和资产唯一编号')
    const confirm = Array.from(document.querySelectorAll('.el-dialog button')).find((item) => item.textContent?.trim() === '确认复制') as HTMLButtonElement
    confirm.click(); await flushPromises()
    expect(mocks.copyDataCenter).toHaveBeenCalledWith('dc-1', expect.objectContaining({ version: 2, name: '主中心-副本', autoGenerateCode: true }))
    wrapper.unmount()
  })

  it('moves a rack to a room in another data center while preserving its identity', async () => {
    const wrapper = mountView(); await flushPromises()
    ;(wrapper.vm as unknown as { openTransfer: Function }).openTransfer('rack', 'move', rack)
    await flushPromises()
    ;(wrapper.vm as unknown as { transferForm: { targetRoomId: string } }).transferForm.targetRoomId = 'room-2'
    await wrapper.vm.$nextTick()
    const confirm = Array.from(document.querySelectorAll('.el-dialog button')).find((item) => item.textContent?.trim() === '确认移动') as HTMLButtonElement
    confirm.click(); await flushPromises()
    expect(mocks.moveRack).toHaveBeenCalledWith('rack-1', expect.objectContaining({ version: 4, targetRoomId: 'room-2', code: 'RACK-01', autoGenerateCode: false }))
    wrapper.unmount()
  })
})

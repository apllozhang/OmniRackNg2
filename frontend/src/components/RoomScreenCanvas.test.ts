import { nextTick, ref } from 'vue'
import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import RoomScreenCanvas from './RoomScreenCanvas.vue'

function createScreen() {
  const rack = {
    id: 'rack-1', name: '01号机柜', code: 'RACK-01', uHeight: 2, status: 'AVAILABLE',
  }
  const selectedRoom = ref<any>(null)
  const filteredRacks = ref<any[]>([rack])
  const searchRackOrDevice = () => undefined
  const screen: Record<string, any> = {
    loading: ref(false), selectedRoom, rackKeyword: ref(''), searchRackOrDevice, openCreateRack: () => undefined,
    selectedRack: ref(rack), openRackEditor: () => undefined, openRackDetail: () => undefined,
    statusFilter: ref('ALL'), legends: [], setStatusFilter: () => undefined, statusCount: () => 0,
    rowLayouts: ref([{ key: 'row-1', label: '第 1 排', index: 0, racks: [rack] }]),
    roomRacks: ref([rack]), filteredRacks, roomUsedU: ref(0), totalU: ref(2), filterLabel: ref('全部'),
    layoutCanvasStyle: ref({ width: '900px', height: '600px' }), roomBoundaryStyle: ref({}),
    layoutRowTop: () => 0, visibleLayoutSlots: ref([{ key: '0-0', row: 0, column: 0, rack }]),
    rackSlotFrameStyle: () => ({}), rackCanvasStyle: () => ({ left: '0px', top: '0px' }),
    handleRackCardClick: () => undefined, selectRack: () => undefined, handleUClick: () => undefined,
    openEmptyContextMenu: () => undefined, openRackContextMenu: () => undefined,
    draggingRackId: ref(null), dragSaving: ref(false), startRackDrag: () => undefined,
    shortName: (value: string) => value, rackColor: () => '#24d6a1', statusLabel: (value: string) => value,
    rackMetrics: () => ({ used: 0, free: 2, rate: 0 }), isUOccupied: () => false,
    uNumbers: () => [2, 1], rackFrameStyle: () => ({}), visiblePositions: () => [],
    deviceBlockStyle: () => ({}), isAbnormalStatus: () => false, layoutState: () => 'loaded',
    deviceName: () => '', positionRange: () => '', positionTitle: () => '', deviceClass: () => '',
    openDeviceContextMenu: () => undefined, openRackUView: () => undefined,
    rackPosition: () => '自动排布', roomLocation: ref('测试位置'),
  }
  return { screen, selectedRoom }
}
function mountCanvas(screen: Record<string, any>) {
  return mount(RoomScreenCanvas, {
    props: { screen },
    global: {
      stubs: {
        'el-input': { template: '<input />' },
      },
      directives: {
        loading: () => undefined,
      },
    },
  })
}
describe('RoomScreenCanvas', () => {
  it('在异步机房数据渲染后向父组件提供真实画布节点', async () => {
    const { screen, selectedRoom } = createScreen()
    const wrapper = mountCanvas(screen)
    expect(wrapper.emitted('canvas-ready')).toBeUndefined()

    selectedRoom.value = { id: 'room-1', name: 'A机房', code: 'ROOM-A' }
    await nextTick()
    await nextTick()

    const readyEvents = wrapper.emitted('canvas-ready') ?? []
    expect(readyEvents[readyEvents.length - 1]?.[0]).toBe(wrapper.find('.room-layout-canvas').element)
    wrapper.unmount()
  })

  it('点击查询按钮会触发机柜和设备定位查询', async () => {
    const { screen } = createScreen()
    const search = vi.spyOn(screen, 'searchRackOrDevice')
    const wrapper = mountCanvas(screen)

    await wrapper.get('.canvas-search-button').trigger('click')

    expect(search).toHaveBeenCalledTimes(1)
    wrapper.unmount()
  })

  it('将画布指针事件转发给父组件，避免拆分后拖动逻辑失效', async () => {
    const { screen, selectedRoom } = createScreen()
    selectedRoom.value = { id: 'room-1', name: 'A机房', code: 'ROOM-A' }
    const wrapper = mountCanvas(screen)
    await nextTick()

    await wrapper.find('.room-layout-canvas').trigger('pointermove', { pointerId: 7 })
    await wrapper.find('.room-layout-canvas').trigger('pointerup', { pointerId: 7 })

    expect(wrapper.emitted('canvas-pointermove')).toHaveLength(1)
    expect(wrapper.emitted('canvas-pointerup')).toHaveLength(1)
    wrapper.unmount()
  })
})
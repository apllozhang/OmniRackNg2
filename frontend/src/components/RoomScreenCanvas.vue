<template>
  <section class="canvas-shell" v-loading="loading" element-loading-background="rgba(5, 18, 38, .82)">
    <div class="canvas-toolbar">
      <div><span class="pulse-dot"></span>机柜 U 位总览</div>
      <div class="toolbar-right">
        <el-input v-model="rackKeyword" clearable size="small" class="rack-search" placeholder="查询机柜编码、名称或设备名称" />
        <button type="button" class="canvas-action canvas-search-button" @click="searchRackOrDevice">查询</button>
        <button type="button" class="canvas-action primary" :disabled="!selectedRoom" @click="openCreateRack">＋ 新增机柜</button>
        <button type="button" class="canvas-action" :disabled="!selectedRack" @click="selectedRack && openRackEditor(selectedRack)">编辑</button>
        <button type="button" class="canvas-action" :disabled="!selectedRack" @click="selectedRack && openRackDetail(selectedRack)">详情</button>
        <span class="canvas-info">{{ statusFilter === 'ALL' ? rowLayouts.length + ' 排 · ' + roomRacks.length + ' 个机柜' : '已筛选 ' + filteredRacks.length + '/' + roomRacks.length + ' 个机柜' }} · {{ roomUsedU }}/{{ totalU }}U 已使用</span>
        <span class="drag-tip">拖动标题可与其他机柜交换槽位<span v-if="dragSaving"> · 正在保存</span></span>
      </div>
    </div>

    <div v-if="selectedRoom" class="filter-bar" role="group" aria-label="机柜状态快速筛选">
      <button type="button" class="filter-chip" :class="{ active: statusFilter === 'ALL' }" @click="setStatusFilter('ALL')">全部<span>{{ roomRacks.length }}</span></button>
      <button v-for="item in legends" :key="item.value" type="button" class="filter-chip" :class="{ active: statusFilter === item.value }" @click="setStatusFilter(item.value)"><i :style="{ background: item.color }"></i>{{ item.label }}<span>{{ statusCount(item.value) }}</span></button>
    </div>

    <div v-if="!selectedRoom" class="empty-screen"><div class="empty-icon">⌗</div><h3>请选择需要展示的机房</h3><p>在左侧依次选择数据中心和机房</p></div>
    <div v-else-if="!roomRacks.length" class="empty-screen"><div class="empty-icon">▥</div><h3>该机房还没有机柜</h3><p>请先在资源层级或机柜管理中添加机柜</p></div>
    <div v-else-if="!filteredRacks.length" class="empty-screen"><div class="empty-icon">◍</div><h3>没有“{{ filterLabel }}”状态的机柜</h3><p>点击筛选栏中的“全部”查看该机房的全部机柜</p></div>

    <div v-else class="rack-room-scroller">
      <div class="room-boundary" :style="roomBoundaryStyle">
        <div class="room-boundary-header">
          <div><strong>{{ selectedRoom.name }}</strong><span>{{ selectedRoom.code }} · {{ roomLocation }}</span></div>
          <div class="capacity-summary"><span>U 位总容量</span><strong>{{ roomUsedU }} <small>/ {{ totalU }}U</small></strong></div>
        </div>

        <div ref="canvasElement" class="room-layout-canvas" :style="layoutCanvasStyle" @pointermove="emit('canvas-pointermove', $event)" @pointerup="emit('canvas-pointerup', $event)" @pointercancel="emit('canvas-pointerup', $event)">
          <div v-for="row in rowLayouts" :key="`guide-${row.key}`" class="layout-row-guide" :style="{ top: `${layoutRowTop(row.index)}px` }"><span>{{ row.label }}</span><i></i></div>
          <div v-for="slot in visibleLayoutSlots" :key="`slot-${slot.key}`" class="rack-slot-frame" :style="rackSlotFrameStyle(slot)" aria-hidden="true"></div>
          <article v-for="(rack, index) in filteredRacks" :key="rack.id" :data-rack-id="rack.id" class="rack-card rack-card-floating" :class="{ selected: selectedRack?.id === rack.id, dragging: draggingRackId === rack.id }" :style="rackCanvasStyle(rack, index)" tabindex="0" role="button" :aria-label="`${rack.name}，${rackMetrics(rack).used}U 已使用，${rackMetrics(rack).free}U 空闲`" @click="handleRackCardClick(rack)" @contextmenu.prevent.stop="openRackContextMenu($event, rack)" @keydown.enter="selectRack(rack)">
              <header class="rack-card-header rack-drag-handle" title="拖动调整机柜位置" @pointerdown.stop="startRackDrag($event, rack, index)">
                <div><strong :title="rack.name">{{ shortName(rack.name, 13) }}</strong><span>{{ rack.code }} · {{ rack.uHeight }}U</span></div>
                <button type="button" class="rack-status-chip" :style="{ '--status-color': rackColor(rack.status) }" :title="`点击修改状态：${statusLabel(rack.status)}`" @pointerdown.stop @click.stop="openRackEditor(rack, true)"><i></i>{{ statusLabel(rack.status) }}</button>
              </header>
              <div class="rack-usage-line"><span>已用 {{ rackMetrics(rack).used }}U</span><b>{{ rackMetrics(rack).rate }}%</b><span>剩余 {{ rackMetrics(rack).free }}U</span></div>
              <div class="rack-usage-bar"><i :style="{ width: `${rackMetrics(rack).rate}%` }"></i></div>

              <div class="cabinet-shell" :class="{ 'is-full': rack.status === 'FULL' }">
                <div class="cabinet-top"><span></span><b>U 位面板</b><span></span></div>
                <div class="cabinet-frame" :style="rackFrameStyle(rack)">
                  <div class="rack-rail left"></div><div class="rack-rail right"></div>
                  <div class="u-scale" aria-hidden="true"><span v-for="u in uNumbers(rack)" :key="`${rack.id}-scale-${u}`">{{ u }}</span></div>
                  <div class="u-grid">
                    <button v-for="u in uNumbers(rack)" :key="`${rack.id}-grid-${u}`" type="button" class="u-line" :class="{ major: u % 5 === 0, occupied: isUOccupied(rack, u) }" :title="isUOccupied(rack, u) ? `${u}U 已占用` : `${u}U 空闲，点击或右键可进行上架操作`" @click.stop="handleUClick($event, rack, u)" @contextmenu.prevent.stop="openEmptyContextMenu($event, rack, u)"></button>
                    <button v-for="position in visiblePositions(rack)" :key="`${rack.id}-${position.id}`" :data-position-id="position.id" type="button" class="device-block" :class="[deviceClass(position), { 'search-hit': isDeviceSearchTarget(position) }]" :style="deviceBlockStyle(rack, position)" :title="`${positionTitle(position)} · 右键快速操作`" @click.stop="openDeviceContextMenu($event, rack, position)" @contextmenu.prevent.stop="openDeviceContextMenu($event, rack, position)">
                      <strong>{{ shortName(deviceName(position), position.heightU <= 1 ? 12 : 18) }}</strong><small v-if="position.heightU >= 2">{{ positionRange(position) }}</small>
                    </button>
                  </div>
                  <div v-if="isAbnormalStatus(rack.status)" class="rack-status-veil" :class="`veil-${rack.status.toLowerCase()}`"><span>{{ statusLabel(rack.status) }}</span></div>
                  <div v-if="layoutState(rack) === 'loading'" class="rack-state-overlay"><i class="loading-ring"></i><span>正在加载 U 位</span></div>
                  <div v-else-if="layoutState(rack) === 'error'" class="rack-state-overlay error"><strong>!</strong><span>U 位数据加载失败</span></div>
                </div>
                <div class="cabinet-base"><span></span><span></span></div>
              </div>

              <footer class="rack-card-footer"><span>{{ rackPosition(rack) }}</span><button type="button" @click.stop="openRackDetail(rack)">详情</button></footer>
            </article>
        </div>
        <div class="room-door"><i></i><span>机房入口</span></div>
      </div>
    </div>
  </section>
</template>
<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'

const props = defineProps<{ screen: Record<string, any> }>()
const emit = defineEmits<{
  (event: 'canvas-ready', element: HTMLElement | null): void
  (name: 'canvas-pointermove', payload: PointerEvent): void
  (name: 'canvas-pointerup', payload: PointerEvent): void
}>()
const canvasElement = ref<HTMLElement | null>(null)
const {
  loading, selectedRoom, rackKeyword, openCreateRack, selectedRack, openRackEditor,
  openRackDetail, statusFilter, legends, setStatusFilter, statusCount, rowLayouts, roomRacks, filteredRacks, roomUsedU, totalU,
  filterLabel, layoutCanvasStyle, roomBoundaryStyle, layoutRowTop, visibleLayoutSlots, rackSlotFrameStyle,
  rackCanvasStyle, handleRackCardClick, selectRack, handleUClick, openEmptyContextMenu, openRackContextMenu, draggingRackId, dragSaving,
  startRackDrag, shortName, rackColor, statusLabel, rackMetrics, isUOccupied,
  uNumbers, rackFrameStyle, visiblePositions, deviceBlockStyle, isAbnormalStatus,
  layoutState, deviceName, positionRange, isDeviceSearchTarget, positionTitle, deviceClass, openDeviceContextMenu, openRackUView, rackPosition,
  roomLocation, searchRackOrDevice,
} = props.screen

watch(canvasElement, (element) => emit('canvas-ready', element), { flush: 'post' })
onBeforeUnmount(() => emit('canvas-ready', null))
</script>
<style>
</style>
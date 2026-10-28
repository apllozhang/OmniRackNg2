<template>
  <div class="screen-page" @contextmenu.prevent>
    <RoomScreenSidebar :screen="sidebarScreen" />

    <main class="visual-area">
      <header class="screen-header">
        <div>
          <h2>{{ selectedDataCenter?.name || '暂无数据中心' }}<span>/</span>{{ selectedRoom?.name || '请选择机房' }}</h2>
          <p>机柜 U 位立面图 · 拖动机柜标题即可交换固定槽位</p>
        </div>
        <div class="header-status">
          <div class="online"><i></i>数据实时在线</div>
          <div class="clock"><strong>{{ currentTime }}</strong><span>{{ currentDate }}</span></div>
        </div>
      </header>

      <RoomScreenCanvas :screen="canvasScreen" @canvas-ready="setCanvasElement" @canvas-pointermove="handleCanvasPointerMove" @canvas-pointerup="handleCanvasPointerUp" />
    </main>

    <teleport to="body">
      <div v-if="contextMenu.visible && contextRack" class="rack-context-menu" :style="{ left: `${contextMenu.x}px`, top: `${contextMenu.y}px` }" @click.stop @contextmenu.prevent>
        <div class="context-menu-header">
          <small>{{ contextDevicePosition ? '设备快捷操作' : contextIsRack ? '机柜快捷操作' : '空 U 位快捷操作' }}</small>
          <strong>{{ contextDevicePosition ? deviceName(contextDevicePosition) : contextIsRack ? `${contextRack.name}（${contextRack.code}）` : `${contextRack.name} · ${contextU}U` }}</strong>
          <span>{{ contextDevicePosition ? `${positionRange(contextDevicePosition)} · 当前设备位置` : contextIsRack ? `${statusLabel(contextRack.status)} · ${rackPosition(contextRack)}` : '当前 U 位空闲' }}</span>
        </div>
        <template v-if="contextIsRack">
          <button type="button" @click="openRackDetail(contextRack)"><b>查看机柜详情</b><span>规格、位置与使用情况</span></button>
          <button type="button" @click="openRackEditor(contextRack, true)"><b>修改机柜状态</b><span>直接调整规划、维护或停用状态</span></button>
          <button type="button" @click="openRackEditor(contextRack)"><b>编辑机柜信息</b><span>修改编码、名称、规格与位置</span></button>
          <button type="button" class="danger" @click="removeRack(contextRack)"><b>删除机柜</b><span>机柜内有设备时后端会拒绝删除</span></button>
        </template>
        <template v-else>
          <button v-if="contextDevicePosition" type="button" @click="openDeviceDetail"><b>查看设备详情</b><span>资产、网络与业务信息</span></button>
          <button v-if="contextDevicePosition" type="button" @click="openDeviceEditor"><b>编辑设备信息</b><span>修改名称、编码、规格、功耗与台账信息</span></button>
          <button v-if="contextDevicePosition" type="button" @click="openQuickMove"><b>调整 / 迁移位置</b><span>变更机柜或 U 位</span></button>
          <button v-if="contextDevicePosition" type="button" class="danger" @click="quickDecommission()"><b>下架设备</b><span>按审批规则提交或执行下架</span></button>
          <button v-else type="button" :disabled="!canInstallInRack(contextRack)" :title="canInstallInRack(contextRack) ? '' : '当前机柜状态不允许上架设备'" @click="openQuickAssign"><b>上架设备到此 U 位</b><span>选择待上架设备并确认占用高度</span></button>
          <button type="button" @click="openContextRackUView"><b>查看完整 U 位详情</b><span>进入设备与 U 位管理页面</span></button>
        </template>
      </div>
    </teleport>

    <el-dialog v-model="positionDialog" :title="operationMode === 'ASSIGN' ? '快速上架设备' : '调整 / 迁移设备'" width="640px" class="screen-operation-dialog" append-to-body destroy-on-close>
      <el-alert v-if="operationDevice" :title="`${operationDevice.name} · ${operationDevice.code} · ${operationDevice.heightU}U`" type="info" :closable="false" />
      <el-form :model="positionForm" label-width="100px" class="quick-position-form">
        <el-form-item v-if="operationMode === 'ASSIGN'" label="待上架设备" required>
          <el-select v-model="positionDeviceId" filterable placeholder="请选择待上架或已下架设备" style="width: 100%" :loading="assignableLoading" @change="syncPositionDevice">
            <el-option v-for="item in assignableDevices" :key="item.id" :label="`${item.name}（${item.code}）· ${item.heightU}U`" :value="item.id" />
          </el-select>
          <div v-if="!assignableLoading && !assignableDevices.length" class="form-tip">暂无可上架设备，请先在设备台账中新增设备。</div>
        </el-form-item>
        <el-form-item label="目标机柜" required>
          <el-cascader v-model="positionPath" :options="rackOptions" :props="{ emitPath: true }" filterable style="width: 100%" @change="syncPositionRack" />
        </el-form-item>
        <el-alert title="机柜位置统一按 U 位区间管理。" type="info" :closable="false" />
        <el-form-item label="起始 U">
          <el-input-number v-model="positionForm.startU" :min="1" :max="targetRack?.uHeight ?? 100" />
          <span class="form-suffix">U</span>
        </el-form-item>
        <el-form-item label="占用高度">
          <el-input-number v-model="positionForm.heightU" :min="1" :max="targetRack?.uHeight ?? 100" :disabled="Boolean(operationDevice)" />
          <span class="form-suffix">U</span>
        </el-form-item>
        <el-form-item label="安装方向"><el-radio-group v-model="positionForm.orientation"><el-radio value="NORMAL">正常</el-radio><el-radio value="REVERSE">倒装</el-radio></el-radio-group></el-form-item>
        <el-form-item label="操作原因"><el-input v-model="positionForm.reason" type="textarea" :rows="3" placeholder="可选；开启审批时将随申请一并提交" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="positionDialog = false">取消</el-button><el-button type="primary" :loading="operationBusy" :disabled="operationMode === 'ASSIGN' && !operationDevice" @click="submitQuickPosition">提交</el-button></template>
    </el-dialog>

    <el-dialog v-model="deviceEditorDialog" title="编辑设备信息" width="980px" class="screen-device-editor-dialog" append-to-body destroy-on-close>
      <el-alert v-if="deviceEditorTarget" :title="`${deviceEditorTarget.name} · ${deviceEditorTarget.code}`" type="info" :closable="false" />
      <el-form :model="deviceEditorForm" label-width="108px" class="device-editor-form">
        <DeviceFormFields
          v-model="deviceEditorForm"
          :types="deviceEditorTypes"
          mode="edit"
          :loading="deviceEditorLoading"
          positioned
        />
      </el-form>
      <template #footer><el-button @click="deviceEditorDialog = false">取消</el-button><el-button type="primary" :loading="deviceEditorBusy" :disabled="deviceEditorLoading" @click="saveDeviceEditor">保存设备</el-button></template>
    </el-dialog>

    <el-dialog v-model="rackEditorDialog" :title="rackEditorMode === 'create' ? '新增机柜' : '编辑机柜'" width="760px" class="screen-rack-editor-dialog" append-to-body destroy-on-close>
      <el-alert v-if="rackEditorMode === 'create' && selectedRoom" :title="`将新增到：${selectedRoom.name}（${selectedRoom.code}）`" type="info" :closable="false" />
      <el-form :model="rackEditorForm" label-width="88px" class="rack-editor-form">
        <div class="rack-form-grid">
          <el-form-item label="编码" :required="rackEditorMode === 'edit' || !rackEditorAutoCode"><div class="code-field"><el-input v-model="rackEditorForm.code" :disabled="rackEditorMode === 'create' && rackEditorAutoCode" :placeholder="rackEditorMode === 'create' && rackEditorAutoCode ? '保存时自动生成' : '例如 RACK-A01'" /><el-checkbox v-if="rackEditorMode === 'create'" v-model="rackEditorAutoCode">自动生成</el-checkbox></div></el-form-item>
          <el-form-item label="名称" required><el-input v-model="rackEditorForm.name" placeholder="例如 A排01号机柜" /></el-form-item>
          <el-form-item label="状态"><el-select v-model="rackEditorForm.status" style="width: 100%"><el-option v-for="item in legends" :key="item.value" :label="item.label" :value="item.value" /></el-select></el-form-item>
          <el-form-item label="机柜类型"><el-input v-model="rackEditorForm.type" placeholder="STANDARD" /></el-form-item>
          <el-form-item label="U 位数量"><el-input-number v-model="rackEditorForm.uHeight" :min="1" :max="100" controls-position="right" /></el-form-item>
          <el-form-item label="排序"><el-input-number v-model="rackEditorForm.sortOrder" :min="0" controls-position="right" /></el-form-item>
          <el-form-item label="宽度(mm)"><el-input-number v-model="rackEditorForm.widthMm" :min="1" controls-position="right" /></el-form-item>
          <el-form-item label="深度(mm)"><el-input-number v-model="rackEditorForm.depthMm" :min="1" controls-position="right" /></el-form-item>
          <el-form-item label="高度(mm)"><el-input-number v-model="rackEditorForm.heightMm" :min="1" controls-position="right" /></el-form-item>
          <el-form-item label="区域"><el-input v-model="rackEditorForm.zone" /></el-form-item>
          <el-form-item label="排"><el-input v-model="rackEditorForm.row" /></el-form-item>
          <el-form-item label="列"><el-input v-model="rackEditorForm.column" /></el-form-item>
          <el-form-item label="通道"><el-input v-model="rackEditorForm.aisle" /></el-form-item>
          <el-form-item label="厂商"><el-input v-model="rackEditorForm.manufacturer" /></el-form-item>
          <el-form-item label="型号"><el-input v-model="rackEditorForm.modelNumber" /></el-form-item>
          <el-form-item label="负责人"><el-input v-model="rackEditorForm.manager" /></el-form-item>
          <el-form-item label="用途"><el-input v-model="rackEditorForm.purpose" /></el-form-item>
          <el-form-item label="双路供电"><el-switch v-model="rackEditorForm.dualPower" /></el-form-item>
          <el-form-item label="PDU 数量"><el-input-number v-model="rackEditorForm.pduCount" :min="0" controls-position="right" /></el-form-item>
          <el-form-item label="备注" class="full-width"><el-input v-model="rackEditorForm.remarks" type="textarea" :rows="3" /></el-form-item>
        </div>
      </el-form>
      <template #footer><el-button @click="rackEditorDialog = false">取消</el-button><el-button type="primary" :loading="rackEditorBusy" @click="saveRack">保存机柜</el-button></template>
    </el-dialog>

    <RackDiagramImportDialog
      v-model="rackDiagramImportDialog"
      :room-id="selectedRoomId"
      :data-center-id="selectedDataCenterId"
      :device-types="rackDiagramDeviceTypes"
      @success="handleRackDiagramImportSuccess"
    />

    <RackDetailDialog
      v-model="rackDetailDrawer"
      :rack="rackDetailTarget"
      :layout="rackDetailTarget ? rackLayouts[rackDetailTarget.id] : null"
      :layout-state="rackDetailTarget ? rackLayoutStates[rackDetailTarget.id] : undefined"
      :room-name="selectedRoom?.name"
      @edit="openRackEditor"
      @remove="removeRack"
    />

    <el-drawer v-model="detailDrawer" title="设备详细信息" size="720px" class="screen-detail-drawer" append-to-body>
      <div v-loading="detailLoading" class="device-detail-panel">
        <DeviceDetailFields
          v-if="detailDevice"
          :device="detailDevice"
          :position="detailPosition"
          :rack-name="detailPosition ? contextRackName(detailPosition.rackId) : undefined"
        />
      </div>
      <template #footer><div class="detail-footer"><el-button @click="detailDrawer = false">关闭</el-button><el-button v-if="detailPosition" type="danger" :loading="operationBusy" @click="quickDecommission(detailPosition)">下架设备</el-button></div></template>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useRouter } from 'vue-router'
import { assignDevice, decommissionDevice, getDevice, getRackLayout, listDevices, listDeviceTypes, moveDevice, updateDevice } from '@/api/device'
import { createRack, deleteRack, getResourceTree, updateRack } from '@/api/resource'
import type { Device, DeviceCategory, DevicePayload, DeviceType, PositionChangePayload, RackDevicePosition, RackULayout } from '@/types/device'
import type { DataCenter, Rack, RackPayload, Room } from '@/types/resource'
import { createRackPayload } from '@/features/rack/rackDefaults'
import { downloadRackDiagramExcel } from '@/utils/rackDiagramExport'
import { hasRequiredCodeAndName, withAutoGeneratedCode } from '@/features/code/autoCode'
import { createDevicePayload, toDeviceApiPayload, toDevicePayload, validateDevicePayload } from '@/features/device/deviceForm'
import { findRackSearchTarget, rackSearchableText } from '@/features/rack/rackSearch'
import RoomScreenCanvas from '@/components/RoomScreenCanvas.vue'
import RoomScreenSidebar from '@/components/RoomScreenSidebar.vue'
import RackDetailDialog from '@/components/RackDetailDialog.vue'
import RackDiagramImportDialog from '@/components/RackDiagramImportDialog.vue'
import DeviceFormFields from '@/components/DeviceFormFields.vue'
import DeviceDetailFields from '@/components/DeviceDetailFields.vue'

interface RackRowLayout { key: string; label: string; index: number; racks: Rack[] }
type LayoutLoadState = 'loading' | 'loaded' | 'error'
type ContextTarget = { kind: 'rack'; rack: Rack } | { kind: 'device'; rack: Rack; position: RackDevicePosition } | { kind: 'empty'; rack: Rack; u: number }
type OperationMode = 'ASSIGN' | 'MOVE'
type RackEditorMode = 'create' | 'edit'

const router = useRouter()
const tree = ref<DataCenter[]>([])
const loading = ref(false)
const rackDiagramExporting = ref(false)
const rackDiagramImportDialog = ref(false)
const rackDiagramDeviceTypes = ref<DeviceType[]>([])
const rackDiagramDeviceTypesLoading = ref(false)
const selectedDataCenterId = ref('')
const selectedRoomId = ref('')
const selectedRack = ref<Rack | null>(null)
const selectedDevicePosition = ref<RackDevicePosition | null>(null)
const rackLayouts = ref<Record<string, RackULayout>>({})
const rackLayoutStates = ref<Record<string, LayoutLoadState>>({})
const now = ref(new Date())
const isFullscreen = ref(false)
const contextMenu = ref<{ visible: boolean; x: number; y: number; target: ContextTarget | null }>({ visible: false, x: 0, y: 0, target: null })
const positionDialog = ref(false)
const operationMode = ref<OperationMode>('ASSIGN')
const operationDevice = ref<Device | null>(null)
const positionDeviceId = ref('')
const assignableDevices = ref<Device[]>([])
const assignableLoading = ref(false)
const operationBusy = ref(false)
const positionPath = ref<string[]>([])
const positionForm = ref<PositionChangePayload>(newPositionForm())
const deviceEditorDialog = ref(false)
const deviceEditorLoading = ref(false)
const deviceEditorBusy = ref(false)
const deviceEditorTarget = ref<Device | null>(null)
const deviceEditorTypes = ref<DeviceType[]>([])
const deviceEditorForm = ref<DevicePayload>(createDevicePayload())
const deviceEditorOriginalHeightU = ref<number | undefined>()
const detailDrawer = ref(false)
const detailLoading = ref(false)
const detailDevice = ref<Device | null>(null)
const detailPosition = ref<RackDevicePosition | null>(null)
const rackKeyword = ref('')
const rackEditorDialog = ref(false)
const rackEditorMode = ref<RackEditorMode>('create')
const rackEditorAutoCode = ref(true)
const rackEditorBusy = ref(false)
const rackEditorTarget = ref<Rack | null>(null)
const rackEditorForm = ref<RackPayload>(createRackPayload())
const rackDetailDrawer = ref(false)
const rackDetailTarget = ref<Rack | null>(null)
const uUnitHeight = 9
const rackCardWidth = 218
const rackCardChromeHeight = 152
const rackSlotWidth = 246
const rackRowGap = 32
const rackOriginX = 38
const rackOriginY = 72
const rackCanvasPositions = ref<Record<string, { x: number; y: number }>>({})
const rackSlotAssignments = ref<Record<string, { row: number; column: number }>>({})
const canvasRef = ref<HTMLElement | null>(null)
const draggingRackId = ref<string | null>(null)
const dragSaving = ref(false)
const suppressRackClick = ref(false)
let dragPointerId: number | null = null
let dragOffset = { x: 0, y: 0 }
let dragOrigin = { x: 0, y: 0 }
let dragMoved = false
let timer: number | undefined
let layoutRequestSerial = 0

const legends = [
  { label: '空闲', value: 'AVAILABLE', color: '#24d6a1' },
  { label: '部分使用', value: 'PARTIAL', color: '#2dc8f0' },
  { label: '已满', value: 'FULL', color: '#ffb547' },
  { label: '维护中', value: 'MAINTENANCE', color: '#a78bfa' },
  { label: '规划中', value: 'PLANNING', color: '#7188a8' },
  { label: '停用', value: 'DISABLED', color: '#ef6470' },
]
const deviceLegends: Array<{ label: string; value: DeviceCategory; color: string }> = [
  { label: '服务器', value: 'SERVER', color: '#2d8cff' },
  { label: '网络设备', value: 'NETWORK', color: '#18c7a2' },
  { label: '存储设备', value: 'STORAGE', color: '#a56eff' },
  { label: '安全设备', value: 'SECURITY', color: '#ff9f43' },
  { label: '动力环境', value: 'POWER_ENVIRONMENT', color: '#ef6470' },
  { label: '其他设备', value: 'OTHER', color: '#5f86a5' },
]

const selectedDataCenter = computed(() => tree.value.find((item) => item.id === selectedDataCenterId.value))
const rooms = computed<Room[]>(() => selectedDataCenter.value?.rooms ?? [])
const selectedRoom = computed(() => rooms.value.find((item) => item.id === selectedRoomId.value))
const roomRacks = computed(() => selectedRoom.value?.racks ?? [])
const statusFilter = ref<string>('ALL')
const filteredRacks = computed(() => roomRacks.value.filter((rack) => {
  const statusMatched = statusFilter.value === 'ALL' || rack.status === statusFilter.value
  const keyword = rackKeyword.value.trim().toLowerCase()
  if (!keyword) return statusMatched
  const searchable = rackSearchableText(rack, visiblePositions(rack))
  return statusMatched && searchable.includes(keyword)
}))
const filterLabel = computed(() => (statusFilter.value === 'ALL' ? '全部' : statusLabel(statusFilter.value)))
const totalU = computed(() => roomRacks.value.reduce((sum, item) => sum + item.uHeight, 0))
const roomUsedU = computed(() => roomRacks.value.reduce((sum, rack) => sum + rackMetrics(rack).used, 0))
const roomUsageRate = computed(() => totalU.value ? Math.round((roomUsedU.value / totalU.value) * 100) : 0)
const selectedRackMetrics = computed(() => selectedRack.value ? rackMetrics(selectedRack.value) : { used: 0, free: 0, rate: 0 })
const contextTarget = computed(() => contextMenu.value.target)
const contextRack = computed(() => contextTarget.value?.rack ?? null)
const contextDevicePosition = computed(() => contextTarget.value?.kind === 'device' ? contextTarget.value.position : null)
const contextIsRack = computed(() => contextTarget.value?.kind === 'rack')
const contextU = computed(() => contextTarget.value?.kind === 'empty' ? contextTarget.value.u : 0)
const rackOptions = computed(() => tree.value.map((dc) => ({
  value: dc.id,
  label: dc.name,
  children: (dc.rooms ?? []).map((room) => ({
    value: room.id,
    label: room.name,
    children: (room.racks ?? []).map((rack) => ({ value: rack.id, label: `${rack.name}（${rack.code}）`, disabled: !canInstallInRack(rack) })),
  })),
})))
const targetRack = computed(() => findRack(positionForm.value.rackId))
const roomLocation = computed(() => {
  const room = selectedRoom.value
  if (!room) return '-'
  return [room.building, room.floor, room.roomNumber].filter(Boolean).join(' / ') || '未设置物理位置'
})
const currentTime = computed(() => now.value.toLocaleTimeString('zh-CN', { hour12: false }))
const currentDate = computed(() => now.value.toLocaleDateString('zh-CN', { year: 'numeric', month: '2-digit', day: '2-digit', weekday: 'long' }))
const racksPerRow = computed(() => {
  const configured = Number(selectedRoom.value?.racksPerRow)
  return Math.min(20, Math.max(1, Number.isFinite(configured) ? Math.round(configured) : 8))
})
function slotForIndex(index: number) {
  const safeIndex = Math.max(0, index)
  return { row: Math.floor(safeIndex / racksPerRow.value), column: safeIndex % racksPerRow.value }
}
const layoutSlots = computed(() => roomRacks.value.map((rack, index) => {
  const slot = rackSlotAssignments.value[rack.id] ?? slotForIndex(index)
  return { key: `${slot.row}-${slot.column}`, rack, ...slot }
}))
const visibleLayoutSlots = computed(() => filteredRacks.value.map((rack) => {
  const roomIndex = roomRacks.value.findIndex((candidate) => candidate.id === rack.id)
  const slot = rackSlotAssignments.value[rack.id] ?? slotForIndex(roomIndex)
  return { key: `${slot.row}-${slot.column}`, rack, ...slot }
}))
const rowLayouts = computed<RackRowLayout[]>(() => {
  const rowCount = Math.ceil(roomRacks.value.length / racksPerRow.value)
  return Array.from({ length: rowCount }, (_, index) => ({
    key: `visual-${index + 1}`,
    label: `第 ${index + 1} 排`,
    index,
    racks: layoutSlots.value.filter((slot) => slot.row === index).sort((a, b) => a.column - b.column).map((slot) => slot.rack),
  }))
})
const layoutRowHeights = computed(() => rowLayouts.value.map((row) => Math.max(...row.racks.map(rackCardHeight))))
const layoutRowTops = computed(() => {
  const tops: number[] = []
  let top = rackOriginY
  layoutRowHeights.value.forEach((height) => {
    tops.push(top)
    top += height + rackRowGap
  })
  return tops
})
const layoutCanvasSize = computed(() => {
  const rackCount = roomRacks.value.length
  const columnCount = Math.min(rackCount, racksPerRow.value)
  const width = columnCount
    ? rackOriginX + (columnCount - 1) * rackSlotWidth + rackCardWidth + 38
    : 900
  const lastRowIndex = rowLayouts.value.length - 1
  const height = lastRowIndex >= 0
    ? (layoutRowTops.value[lastRowIndex] ?? rackOriginY) + (layoutRowHeights.value[lastRowIndex] ?? 0) + 36
    : 600
  return { width: Math.max(900, width), height: Math.max(600, height) }
})
const layoutCanvasStyle = computed(() => ({ width: `${layoutCanvasSize.value.width}px`, height: `${layoutCanvasSize.value.height}px` }))
const roomBoundaryStyle = computed(() => ({ minWidth: `${layoutCanvasSize.value.width + 40}px` }))
// Keep the two large visual regions presentational. The parent remains the single
// owner of API calls and mutations, which makes the split low-risk and testable.
const sidebarScreen = {
  tree, selectedDataCenterId, changeDataCenter, rooms, selectedRoomId, changeRoom,
  selectedRoom, roomLocation, openCreateRack, loadTree, exportRackDiagram, rackDiagramExporting, openRackDiagramImport, rackDiagramDeviceTypesLoading, roomRacks, statusCount,
  roomUsedU, totalU, roomUsageRate, statusFilter, legends, setStatusFilter,
  deviceLegends, selectedRack, rackColor, openRackEditor, statusLabel,
  selectedRackMetrics, rackPosition, selectedDevicePosition, deviceName,
  deviceCategoryLabel, positionRange, openRackDetail, removeRack, openUView,
  goBack, isFullscreen, toggleFullscreen,
}
const canvasScreen = {
  loading, selectedRoom, rackKeyword, openCreateRack, selectedRack, openRackEditor,
  openRackDetail, statusFilter, legends, setStatusFilter, statusCount, rowLayouts, roomRacks, filteredRacks, roomUsedU, totalU,
  filterLabel, layoutCanvasStyle, roomBoundaryStyle, layoutRowTop, visibleLayoutSlots,
  rackSlotFrameStyle, rackCanvasStyle, handleRackCardClick, openRackContextMenu,
  draggingRackId, dragSaving, startRackDrag, shortName, rackColor, statusLabel, rackMetrics, selectRack, handleUClick, openEmptyContextMenu, positionTitle, openDeviceContextMenu,
  isUOccupied, uNumbers, rackFrameStyle, visiblePositions, deviceBlockStyle,
  isAbnormalStatus, layoutState, deviceName, positionRange, isDeviceSearchTarget, deviceClass,
  openRackUView, rackPosition, roomLocation, searchRackOrDevice,
}
function setCanvasElement(element: HTMLElement | null) { canvasRef.value = element }

async function loadTree() {
  loading.value = true
  const previousRackId = selectedRack.value?.id
  try {
    tree.value = (await getResourceTree()).items ?? []
    if (!tree.value.some((item) => item.id === selectedDataCenterId.value)) selectedDataCenterId.value = tree.value[0]?.id ?? ''
    const nextRooms = tree.value.find((item) => item.id === selectedDataCenterId.value)?.rooms ?? []
    if (!nextRooms.some((item) => item.id === selectedRoomId.value)) selectedRoomId.value = nextRooms[0]?.id ?? ''
    selectedRack.value = roomRacks.value.find((rack) => rack.id === previousRackId) ?? roomRacks.value[0] ?? null
    selectedDevicePosition.value = null
    syncRackCanvasPositions()
    await loadRoomLayouts()
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    loading.value = false
  }
}

async function loadRoomLayouts() {
  const serial = ++layoutRequestSerial
  const racks = [...roomRacks.value]
  rackLayouts.value = {}
  rackLayoutStates.value = Object.fromEntries(racks.map((rack) => [rack.id, 'loading'])) as Record<string, LayoutLoadState>
  if (!racks.length) return

  await Promise.all(racks.map(async (rack) => {
    try {
      const layout = await getRackLayout(rack.id)
      if (serial !== layoutRequestSerial) return
      rackLayouts.value = { ...rackLayouts.value, [rack.id]: layout }
      rackLayoutStates.value = { ...rackLayoutStates.value, [rack.id]: 'loaded' }
    } catch {
      if (serial !== layoutRequestSerial) return
      rackLayoutStates.value = { ...rackLayoutStates.value, [rack.id]: 'error' }
    }
  }))
}

async function openRackDiagramImport() {
  if (!selectedDataCenter.value || !selectedRoom.value) { ElMessage.warning('请先选择机房'); return }
  rackDiagramDeviceTypesLoading.value = true
  try {
    rackDiagramDeviceTypes.value = (await listDeviceTypes()).items ?? []
    if (!rackDiagramDeviceTypes.value.some((item) => item.status === 'ACTIVE')) {
      ElMessage.warning('当前没有启用的设备类型，请先在设备类型管理中新增或启用设备类型')
      return
    }
    rackDiagramImportDialog.value = true
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    rackDiagramDeviceTypesLoading.value = false
  }
}
async function handleRackDiagramImportSuccess() {
  await loadTree()
  ElMessage.success('机房大屏数据已刷新')
}

async function exportRackDiagram() {
  const dataCenter = selectedDataCenter.value
  const room = selectedRoom.value
  if (!dataCenter || !room) { ElMessage.warning('请先选择机房'); return }
  if (!roomRacks.value.length) { ElMessage.warning('当前机房暂无可导出的机柜'); return }

  rackDiagramExporting.value = true
  try {
    const orderedRacks = rowLayouts.value.flatMap((row) => row.racks)
    const missingRacks = orderedRacks.filter((rack) => !rackLayouts.value[rack.id])
    if (missingRacks.length) {
      const loadedLayouts = await Promise.all(missingRacks.map(async (rack) => {
        try {
          return [rack.id, await getRackLayout(rack.id)] as const
        } catch {
          throw new Error(`无法读取机柜“${rack.name}”的 U 位数据`)
        }
      }))
      rackLayouts.value = { ...rackLayouts.value, ...Object.fromEntries(loadedLayouts) }
    }

    downloadRackDiagramExcel({
      dataCenter,
      room,
      racks: orderedRacks,
      layouts: rackLayouts.value,
      racksPerRow: room.racksPerRow,
    })
    ElMessage.success(`已导出 ${orderedRacks.length} 个机柜的机柜图`)
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    rackDiagramExporting.value = false
  }
}
function defaultRackCanvasPosition(_rack: Rack, index: number) {
  return slotPosition(slotForIndex(index))
}
function slotKey(slot: { row: number; column: number }) { return `${slot.row}-${slot.column}` }
function rowTop(row: number) {
  if (layoutRowTops.value[row] !== undefined) return layoutRowTops.value[row]
  const lastIndex = layoutRowTops.value.length - 1
  if (lastIndex < 0) return rackOriginY
  return layoutRowTops.value[lastIndex] + layoutRowHeights.value[lastIndex] + rackRowGap * Math.max(1, row - lastIndex)
}
function slotPosition(slot: { row: number; column: number }) {
  return { x: rackOriginX + slot.column * rackSlotWidth, y: rowTop(slot.row) }
}
function slotFromPosition(position: { x: number; y: number }) {
  const maxIndex = Math.max(0, roomRacks.value.length - 1)
  const nearestRow = layoutRowTops.value.reduce((best, top, index) =>
    Math.abs(position.y - top) < Math.abs(position.y - (layoutRowTops.value[best] ?? rackOriginY)) ? index : best, 0)
  const column = Math.min(racksPerRow.value - 1, Math.max(0, Math.round((position.x - rackOriginX) / rackSlotWidth)))
  return slotForIndex(Math.min(maxIndex, nearestRow * racksPerRow.value + column))
}
function rackCanvasPosition(rack: Rack, index: number) {
  const saved = rackCanvasPositions.value[rack.id]
  if (draggingRackId.value === rack.id && saved) return saved
  const slot = rackSlotAssignments.value[rack.id]
  if (slot) return slotPosition(slot)
  if (saved) return saved
  return defaultRackCanvasPosition(rack, index)
}
function findRackAtSlot(slot: { row: number; column: number }, excludeRackId?: string) {
  return roomRacks.value.find((rack) => rack.id !== excludeRackId && slotKey(rackSlotAssignments.value[rack.id] ?? { row: -1, column: -1 }) === slotKey(slot))
}
function syncRackCanvasPositions() {
  const orderedRacks = roomRacks.value.map((rack, index) => ({
    rack,
    index,
    x: Number(rack.xCoordinate),
    y: Number(rack.yCoordinate),
    persisted: Number.isFinite(Number(rack.xCoordinate)) && Number.isFinite(Number(rack.yCoordinate)),
  })).sort((a, b) => {
    if (a.persisted !== b.persisted) return a.persisted ? -1 : 1
    if (a.persisted && b.persisted) return a.y - b.y || a.x - b.x || a.index - b.index
    return a.index - b.index
  })
  const nextAssignments: Record<string, { row: number; column: number }> = {}
  orderedRacks.forEach(({ rack }, index) => { nextAssignments[rack.id] = slotForIndex(index) })
  rackSlotAssignments.value = nextAssignments
  rackCanvasPositions.value = rackPositionMap(nextAssignments)
}
function layoutRowTop(index: number) { return rowTop(index) - 22 }
function rackCanvasStyle(rack: Rack, index: number) {
  const position = rackCanvasPosition(rack, index)
  return {
    left: `${position.x}px`,
    top: `${position.y}px`,
    '--rack-height': `${rack.uHeight * uUnitHeight}px`,
    '--rack-card-height': `${rackCardHeight(rack)}px`,
  }
}
function rackSlotFrameStyle(slot: { row: number; column: number; rack: Rack }) {
  const position = slotPosition(slot)
  return { left: `${position.x}px`, top: `${position.y}px`, width: `218px`, height: `${rackCardHeight(slot.rack)}px` }
}
function handleRackCardClick(rack: Rack) {
  if (suppressRackClick.value) {
    suppressRackClick.value = false
    return
  }
  selectRack(rack)
}

function isDeviceSearchTarget(position: RackDevicePosition) {
  return selectedDevicePosition.value?.id === position.id && Boolean(rackKeyword.value.trim())
}

async function searchRackOrDevice() {
  const keyword = rackKeyword.value.trim()
  if (!keyword) {
    selectedDevicePosition.value = null
    selectedRack.value = roomRacks.value[0] ?? null
    ElMessage.info('已清除搜索条件')
    return
  }

  const layoutsIncomplete = roomRacks.value.some((rack) => !rackLayouts.value[rack.id] && rackLayoutStates.value[rack.id] !== 'error')
  if (layoutsIncomplete) await loadRoomLayouts()

  const target = findRackSearchTarget(roomRacks.value, rackLayouts.value, keyword)
  if (!target) {
    selectedDevicePosition.value = null
    ElMessage.warning(`未找到与“${keyword}”匹配的机柜或设备`)
    return
  }

  statusFilter.value = 'ALL'
  selectedRack.value = target.rack
  selectedDevicePosition.value = target.position ?? null
  await nextTick()

  const card = Array.from(canvasRef.value?.querySelectorAll<HTMLElement>('[data-rack-id]') ?? [])
    .find((item) => item.dataset.rackId === target.rack.id)
  const device = target.position
    ? card?.querySelector<HTMLElement>(`[data-position-id=\"${target.position.id}\"]`)
    : null
  ;(device ?? card)?.scrollIntoView({ behavior: 'smooth', block: 'center', inline: 'center' })
  card?.focus({ preventScroll: true })

  if (target.position) {
    ElMessage.success(`已定位设备“${deviceName(target.position)}”：${target.rack.name} · ${positionRange(target.position)}`)
  } else {
    ElMessage.success(`已定位机柜“${target.rack.name}”`)
  }
}
function startRackDrag(event: PointerEvent, rack: Rack, index: number) {
  if (event.button !== 0 || dragSaving.value || draggingRackId.value) return
  const canvas = canvasRef.value
  if (!canvas) return
  const rect = canvas.getBoundingClientRect()
  const position = rackCanvasPosition(rack, index)
  dragPointerId = event.pointerId
  dragOffset = { x: event.clientX - rect.left - position.x, y: event.clientY - rect.top - position.y }
  dragOrigin = { ...position }
  dragMoved = false
  draggingRackId.value = rack.id
  closeContextMenu()
  document.body.classList.add('is-rack-dragging')
  try { canvas.setPointerCapture(event.pointerId) } catch { /* pointer capture is optional */ }
}
function handleCanvasPointerMove(event: PointerEvent) {
  if (!draggingRackId.value || dragPointerId !== event.pointerId) return
  const canvas = canvasRef.value
  if (!canvas) return
  const rack = findRack(draggingRackId.value)
  if (!rack) return
  const rect = canvas.getBoundingClientRect()
  const maxX = Math.max(16, canvas.scrollWidth - rackCardWidth - 16)
  const maxY = Math.max(16, canvas.scrollHeight - rackCardHeight(rack) - 16)
  const x = Math.max(16, Math.min(event.clientX - rect.left - dragOffset.x, maxX))
  const y = Math.max(16, Math.min(event.clientY - rect.top - dragOffset.y, maxY))
  if (Math.abs(x - dragOrigin.x) > 3 || Math.abs(y - dragOrigin.y) > 3) dragMoved = true
  rackCanvasPositions.value = { ...rackCanvasPositions.value, [rack.id]: { x, y } }
}
async function handleCanvasPointerUp(event: PointerEvent) {
  if (!draggingRackId.value || dragPointerId !== event.pointerId) return
  const rackId = draggingRackId.value
  const sourceSlot = { ...(rackSlotAssignments.value[rackId] ?? slotFromPosition(dragOrigin)) }
  const originPositions = rackPositionMap(rackSlotAssignments.value)
  const current = rackCanvasPositions.value[rackId] ?? dragOrigin
  const moved = dragMoved
  const canvas = canvasRef.value
  try { if (canvas?.hasPointerCapture(event.pointerId)) canvas.releasePointerCapture(event.pointerId) } catch { /* pointer capture is optional */ }
  draggingRackId.value = null
  dragPointerId = null
  dragMoved = false
  document.body.classList.remove('is-rack-dragging')
  if (!moved) return
  suppressRackClick.value = true
  const rack = findRack(rackId)
  if (rack) await moveRackToSlot(rack, slotFromPosition(current), sourceSlot, originPositions)
}
function rackPositionMap(assignments: Record<string, { row: number; column: number }>) {
  return Object.fromEntries(roomRacks.value.map((rack) => [rack.id, slotPosition(assignments[rack.id] ?? { row: 0, column: 0 })])) as Record<string, { x: number; y: number }>
}
async function moveRackToSlot(rack: Rack, targetSlot: { row: number; column: number }, sourceSlot: { row: number; column: number }, originPositions: Record<string, { x: number; y: number }>) {
  const occupant = findRackAtSlot(targetSlot, rack.id)
  const previousAssignments = { ...rackSlotAssignments.value }
  if (slotKey(targetSlot) === slotKey(sourceSlot)) {
    rackCanvasPositions.value = rackPositionMap(previousAssignments)
    return
  }
  const nextAssignments = { ...previousAssignments, [rack.id]: targetSlot }
  if (occupant) nextAssignments[occupant.id] = sourceSlot
  const nextPositions = rackPositionMap(nextAssignments)
  rackSlotAssignments.value = nextAssignments
  rackCanvasPositions.value = nextPositions
  dragSaving.value = true
  try {
    const changes = [
      { rack, position: nextPositions[rack.id] },
      ...(occupant ? [{ rack: occupant, position: nextPositions[occupant.id] }] : []),
    ]
    const updates = await Promise.all(changes.map(({ rack: item, position }) => updateRack(item.id, rackPayload(item, { xCoordinate: position.x, yCoordinate: position.y }))))
    updates.forEach((updated) => {
      const localRack = findRack(updated.id)
      if (localRack) Object.assign(localRack, updated)
    })
    ElMessage.success(occupant ? `${rack.name} 与 ${occupant.name} 已交换位置` : '机柜位置已保存')
  } catch (error) {
    rackSlotAssignments.value = previousAssignments
    rackCanvasPositions.value = originPositions
    ElMessage.error(errorMessage(error))
  } finally {
    dragSaving.value = false
  }
}
function rackPayload(rack: Rack, patch: Partial<RackPayload> = {}): RackPayload {
  return {
    id: rack.id,
    version: rack.version,
    code: rack.code,
    name: rack.name,
    status: rack.status,
    sortOrder: rack.sortOrder,
    type: rack.type,
    uHeight: rack.uHeight,
    widthMm: rack.widthMm,
    depthMm: rack.depthMm,
    heightMm: rack.heightMm,
    rotation: rack.rotation,
    dualPower: rack.dualPower,
    inputCircuits: rack.inputCircuits,
    pduCount: rack.pduCount,
    manufacturer: rack.manufacturer,
    modelNumber: rack.modelNumber,
    serialNumber: rack.serialNumber,
    assetNumber: rack.assetNumber,
    loadCapacityKg: rack.loadCapacityKg,
    zone: rack.zone,
    row: rack.row,
    column: rack.column,
    aisle: rack.aisle,
    xCoordinate: rack.xCoordinate,
    yCoordinate: rack.yCoordinate,
    manager: rack.manager,
    department: rack.department,
    purpose: rack.purpose,
    ratedVoltage: rack.ratedVoltage,
    ratedCurrent: rack.ratedCurrent,
    ratedPowerKw: rack.ratedPowerKw,
    peakPowerKw: rack.peakPowerKw,
    remarks: rack.remarks,
    ...patch,
  }
}
function changeDataCenter() {
  selectedRoomId.value = rooms.value[0]?.id ?? ''
  selectedRack.value = rooms.value[0]?.racks?.[0] ?? null
  selectedDevicePosition.value = null
  syncRackCanvasPositions()
  void loadRoomLayouts()
}
function changeRoom() {
  selectedRack.value = selectedRoom.value?.racks?.[0] ?? null
  selectedDevicePosition.value = null
  syncRackCanvasPositions()
  void loadRoomLayouts()
}
function selectRack(rack: Rack) { selectedRack.value = rack; selectedDevicePosition.value = null }
function selectDevice(rack: Rack, position: RackDevicePosition) { selectedRack.value = rack; selectedDevicePosition.value = position }
function handleUClick(event: MouseEvent, rack: Rack, u: number) {
  const position = visiblePositions(rack).find((item) => item.startU <= u && item.endU >= u)
  if (position) {
    openDeviceContextMenu(event, rack, position)
    return
  }
  openEmptyContextMenu(event, rack, u)
}

const emptyRackPayload = (): RackPayload => createRackPayload()

function newPositionForm(): PositionChangePayload {
  return { version: 0, rackId: '', startU: 1, heightU: 1, orientation: 'NORMAL', reason: '' }
}
function showContextMenu(event: MouseEvent, target: ContextTarget) {
  if (target.kind === 'device') selectDevice(target.rack, target.position)
  else selectRack(target.rack)
  const menuWidth = 244
  const menuHeight = target.kind === 'device' ? 350 : target.kind === 'rack' ? 270 : 210
  contextMenu.value = {
    visible: true,
    x: Math.max(8, Math.min(event.clientX, window.innerWidth - menuWidth - 8)),
    y: Math.max(8, Math.min(event.clientY, window.innerHeight - menuHeight - 8)),
    target,
  }
}
function openRackContextMenu(event: MouseEvent, rack: Rack) { showContextMenu(event, { kind: 'rack', rack }) }
function openDeviceContextMenu(event: MouseEvent, rack: Rack, position: RackDevicePosition) { showContextMenu(event, { kind: 'device', rack, position }) }
function openEmptyContextMenu(event: MouseEvent, rack: Rack, u: number) {
  const position = visiblePositions(rack).find((item) => item.startU <= u && item.endU >= u)
  if (position) return openDeviceContextMenu(event, rack, position)
  showContextMenu(event, { kind: 'empty', rack, u })
}
function closeContextMenu() { contextMenu.value.visible = false }
function isUOccupied(rack: Rack, u: number) { return visiblePositions(rack).some((item) => item.startU <= u && item.endU >= u) }
function canInstallInRack(rack: Rack) { return rack.status === 'AVAILABLE' || rack.status === 'PARTIAL' }
function findRack(id?: string) {
  if (!id) return undefined
  for (const dc of tree.value) for (const room of dc.rooms ?? []) {
    const rack = (room.racks ?? []).find((item) => item.id === id)
    if (rack) return rack
  }
  return undefined
}
function rackPath(id: string) {
  for (const dc of tree.value) for (const room of dc.rooms ?? []) if ((room.racks ?? []).some((rack) => rack.id === id)) return [dc.id, room.id, id]
  return []
}
function contextRackName(id: string) { const rack = findRack(id); return rack ? `${rack.name}（${rack.code}）` : id }
function syncPositionRack(value: unknown) {
  const path = Array.isArray(value) ? value as string[] : []
  positionForm.value.rackId = path[path.length - 1] ?? ''
  const rack = targetRack.value
  if (rack && positionForm.value.startU > rack.uHeight) positionForm.value.startU = rack.uHeight
}
function syncPositionDevice(id: string) {
  operationDevice.value = assignableDevices.value.find((item) => item.id === id) ?? null
  if (!operationDevice.value) return
  positionForm.value.version = operationDevice.value.version
  positionForm.value.heightU = operationDevice.value.heightU
}
async function listAllDevicesByStatus(lifecycleStatus: 'WAITING_RACK' | 'OFF_RACK') {
  const first = await listDevices({ page: 1, pageSize: 100, lifecycleStatus })
  const pages = Math.ceil(first.total / first.pageSize)
  if (pages <= 1) return first.items
  const rest = await Promise.all(Array.from({ length: pages - 1 }, (_, index) => listDevices({ page: index + 2, pageSize: 100, lifecycleStatus })))
  return [first, ...rest].flatMap((result) => result.items)
}
async function loadAssignableDevices() {
  assignableLoading.value = true
  try {
    const items = (await Promise.all([listAllDevicesByStatus('WAITING_RACK'), listAllDevicesByStatus('OFF_RACK')])).flat()
    assignableDevices.value = items.filter((item) => !item.currentPosition).sort((a, b) => naturalCompare(a.name, b.name))
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    assignableLoading.value = false
  }
}
async function openQuickAssign() {
  const rack = contextRack.value
  if (!rack || !canInstallInRack(rack)) return
  const startU = contextU.value
  closeContextMenu()
  operationMode.value = 'ASSIGN'
  operationDevice.value = null
  positionDeviceId.value = ''
  positionForm.value = { ...newPositionForm(), rackId: rack.id, startU }
  positionPath.value = rackPath(rack.id)
  positionDialog.value = true
  await loadAssignableDevices()
}
async function openQuickMove() {
  const position = contextDevicePosition.value
  if (!position) return
  closeContextMenu()
  operationMode.value = 'MOVE'
  try {
    operationDevice.value = position.device ?? await getDevice(position.deviceId)
  } catch (error) {
    ElMessage.error(errorMessage(error)); return
  }
  positionDeviceId.value = operationDevice.value.id
  positionForm.value = { version: operationDevice.value.version, rackId: position.rackId, startU: position.startU, heightU: operationDevice.value.heightU, orientation: position.orientation, reason: '' }
  positionPath.value = rackPath(position.rackId)
  positionDialog.value = true
}
function hasPositionConflict(rackId: string, startU: number, heightU: number, excludedDeviceId?: string) {
  const endU = startU + heightU - 1
  return (rackLayouts.value[rackId]?.positions ?? []).some((position) => {
    if (position.deviceId === excludedDeviceId) return false
    return startU <= position.endU && endU >= position.startU
  })
}
async function submitQuickPosition() {
  const device = operationDevice.value
  const rack = targetRack.value
  if (!device) { ElMessage.warning('请选择需要上架的设备'); return }
  if (!rack) { ElMessage.warning('请选择目标机柜'); return }
  if (!canInstallInRack(rack)) { ElMessage.warning('目标机柜当前状态不允许上架或迁入设备'); return }
  const endU = positionForm.value.startU + device.heightU - 1
  if (endU > rack.uHeight) { ElMessage.warning(`目标位置超出 ${rack.uHeight}U 范围`); return }
  if (hasPositionConflict(rack.id, positionForm.value.startU, device.heightU, operationMode.value === 'MOVE' ? device.id : undefined)) {
    ElMessage.warning('目标 U 位与现有设备冲突，请选择其他位置'); return
  }
  operationBusy.value = true
  try {
    const payload = { ...positionForm.value, version: device.version, heightU: device.heightU }
    const result = operationMode.value === 'ASSIGN' ? await assignDevice(device.id, payload) : await moveDevice(device.id, payload)
    ElMessage.success(result.executed ? (operationMode.value === 'ASSIGN' ? '设备已上架' : '设备位置已调整') : '操作已提交审批，机柜位置暂未变化')
    positionDialog.value = false
    await refreshAfterOperation(rack.id)
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    operationBusy.value = false
  }
}
async function openDeviceDetail() {
  const position = contextDevicePosition.value
  if (!position) return
  detailPosition.value = position
  detailDevice.value = position.device ?? null
  closeContextMenu()
  detailDrawer.value = true
  detailLoading.value = true
  try { detailDevice.value = await getDevice(position.deviceId) } catch (error) { ElMessage.error(errorMessage(error)) } finally { detailLoading.value = false }
}
async function openDeviceEditor() {
  const position = contextDevicePosition.value
  if (!position) return
  closeContextMenu()
  const current = position.device
  deviceEditorTarget.value = current ?? null
  deviceEditorForm.value = current ? toDevicePayload(current) : createDevicePayload()
  deviceEditorOriginalHeightU.value = current?.heightU
  deviceEditorDialog.value = true
  deviceEditorLoading.value = true
  try {
    const [device, typeResult] = await Promise.all([getDevice(position.deviceId), listDeviceTypes()])
    deviceEditorTarget.value = device
    deviceEditorForm.value = toDevicePayload(device)
    deviceEditorOriginalHeightU.value = device.heightU
    deviceEditorTypes.value = typeResult.items ?? []
  } catch (error) {
    deviceEditorDialog.value = false
    ElMessage.error(errorMessage(error))
  } finally {
    deviceEditorLoading.value = false
  }
}
async function saveDeviceEditor() {
  const target = deviceEditorTarget.value
  if (!target) return
  const errors = validateDevicePayload(deviceEditorForm.value, {
    positioned: true,
    originalHeightU: deviceEditorOriginalHeightU.value,
  })
  if (errors.length) { ElMessage.warning(errors[0]); return }
  deviceEditorBusy.value = true
  try {
    const updated = await updateDevice(target.id, toDeviceApiPayload(deviceEditorForm.value))
    deviceEditorTarget.value = updated
    deviceEditorForm.value = toDevicePayload(updated)
    deviceEditorOriginalHeightU.value = updated.heightU
    if (detailDevice.value?.id === updated.id) detailDevice.value = updated
    if (detailPosition.value?.deviceId === updated.id) detailPosition.value = { ...detailPosition.value, device: updated }
    deviceEditorDialog.value = false
    ElMessage.success('设备信息已保存')
    await loadTree()
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    deviceEditorBusy.value = false
  }
}
async function quickDecommission(source?: RackDevicePosition) {
  const position = source ?? contextDevicePosition.value ?? detailPosition.value
  if (!position) return
  let device = position.device ?? detailDevice.value
  try {
    if (!device || device.id !== position.deviceId) device = await getDevice(position.deviceId)
    closeContextMenu()
    const result = await ElMessageBox.prompt(`确认将“${device.name}”从 ${contextRackName(position.rackId)} 下架吗？`, '设备下架', { inputPlaceholder: '下架原因（可选）', confirmButtonText: '提交下架', cancelButtonText: '取消', type: 'warning' })
    operationBusy.value = true
    const operation = await decommissionDevice(device.id, { version: device.version, reason: result.value })
    ElMessage.success(operation.executed ? '设备已下架' : '下架申请已提交，审批通过前保留原 U 位')
    detailDrawer.value = false
    detailPosition.value = null
    selectedDevicePosition.value = null
    await refreshAfterOperation(position.rackId)
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') ElMessage.error(errorMessage(error))
  } finally {
    operationBusy.value = false
  }
}
async function refreshAfterOperation(preferredRackId?: string) {
  await loadTree()
  selectedRack.value = roomRacks.value.find((rack) => rack.id === preferredRackId) ?? selectedRack.value
}
async function openContextRackUView() { const rack = contextRack.value; closeContextMenu(); if (rack) await openRackUView(rack) }
function deviceLifecycleLabel(status: string) { return ({ WAITING_RACK: '待上架', RUNNING: '运行中', MAINTENANCE: '维护中', PENDING_REMOVAL: '待下架', OFF_RACK: '已下架', SCRAPPED: '已报废' } as Record<string, string>)[status] ?? status }
function formatTime(value?: string) { return value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '-' }
function handleGlobalKey(event: KeyboardEvent) { if (event.key === 'Escape') closeContextMenu() }

function visiblePositions(rack: Rack) {
  return (rackLayouts.value[rack.id]?.positions ?? [])
    .filter((item) => item.startU <= rack.uHeight && item.endU >= 1)
    .sort((a, b) => b.endU - a.endU)
}
function rackMetrics(rack: Rack) {
  const occupied = new Set<number>()
  visiblePositions(rack).forEach((position) => {
    const start = Math.max(1, position.startU)
    const end = Math.min(rack.uHeight, position.endU)
    for (let u = start; u <= end; u += 1) occupied.add(u)
  })
  const used = occupied.size
  const free = Math.max(0, rack.uHeight - used)
  return { used, free, rate: rack.uHeight ? Math.round((used / rack.uHeight) * 100) : 0 }
}
function uNumbers(rack: Rack) { return Array.from({ length: rack.uHeight }, (_, index) => rack.uHeight - index) }
function rackCardHeight(rack: Rack) { return rackCardChromeHeight + rack.uHeight * uUnitHeight }
function rackFrameStyle(rack: Rack) { return { '--rack-height': `${rack.uHeight * uUnitHeight}px`, '--u-height': `${uUnitHeight}px` } }
function deviceBlockStyle(rack: Rack, position: RackDevicePosition) {
  const end = Math.min(rack.uHeight, position.endU)
  const start = Math.max(1, position.startU)
  return {
    top: `${(rack.uHeight - end) * uUnitHeight}px`,
    height: `${Math.max(1, end - start + 1) * uUnitHeight}px`,
    '--device-color': deviceColor(position.device?.type?.category),
  }
}
function layoutState(rack: Rack) { return rackLayoutStates.value[rack.id] }
function statusCount(status: string) { return roomRacks.value.filter((item) => item.status === status).length }
function setStatusFilter(status: string) {
  statusFilter.value = status === 'ALL' ? 'ALL' : (statusFilter.value === status ? 'ALL' : status)
  if (selectedRack.value && !filteredRacks.value.some((rack) => rack.id === selectedRack.value!.id)) {
    selectedRack.value = filteredRacks.value[0] ?? null
    selectedDevicePosition.value = null
  }
}
function isAbnormalStatus(status: string) { return status === 'MAINTENANCE' || status === 'DISABLED' || status === 'PLANNING' }
function rackColor(status: string) { return legends.find((item) => item.value === status)?.color ?? '#7188a8' }
function statusLabel(status: string) { return legends.find((item) => item.value === status)?.label ?? status }
function deviceColor(category?: DeviceCategory) { return deviceLegends.find((item) => item.value === category)?.color ?? '#5f86a5' }
function deviceCategoryLabel(category?: DeviceCategory) {
  if (category === 'ACCESSORY') return '附件'
  return deviceLegends.find((item) => item.value === category)?.label ?? '其他设备'
}
function deviceClass(position: RackDevicePosition) { return `category-${(position.device?.type?.category ?? 'OTHER').toLowerCase().replace('_', '-')}` }
function deviceName(position: RackDevicePosition) { return position.device?.name || position.device?.code || position.deviceId }
function positionRange(position: RackDevicePosition) { return position.startU === position.endU ? `${position.startU}U` : `${position.startU}-${position.endU}U` }
function positionTitle(position: RackDevicePosition) {
  return `${deviceName(position)} - ${deviceCategoryLabel(position.device?.type?.category)} - ${positionRange(position)} - ${position.heightU}U`
}
function rackPosition(rack: Rack) {
  const parts = [rack.row && `${rack.row}排`, rack.column && `${rack.column}列`].filter(Boolean)
  return parts.join(' ') || '自动排布'
}
function shortName(name: string, maxLength = 9) { return name.length > maxLength ? `${name.slice(0, maxLength - 1)}…` : name }
function naturalCompare(a: string, b: string) { return a.localeCompare(b, 'zh-CN', { numeric: true, sensitivity: 'base' }) }
function openCreateRack() {
  if (!selectedRoom.value) { ElMessage.warning('请先选择机房'); return }
  closeContextMenu()
  rackEditorMode.value = 'create'
  rackEditorTarget.value = null
  rackEditorForm.value = { ...emptyRackPayload(), sortOrder: roomRacks.value.length }
  rackEditorAutoCode.value = true
  rackEditorDialog.value = true
}
function openRackEditor(rack: Rack, focusStatus = false) {
  closeContextMenu()
  rackEditorMode.value = 'edit'
  rackEditorTarget.value = rack
  rackEditorForm.value = rackPayload(rack)
  rackEditorDialog.value = true
  if (focusStatus) ElMessage.info('可在弹窗中直接选择新的机柜状态')
}
function openRackDetail(rack: Rack) {
  closeContextMenu()
  rackDetailTarget.value = rack
  rackDetailDrawer.value = true
  const state = rackLayoutStates.value[rack.id]
  if (state !== 'loading' && (state !== 'loaded' || !rackLayouts.value[rack.id])) void reloadRackDetailLayout(rack)
}
async function reloadRackDetailLayout(rack: Rack) {
  rackLayoutStates.value = { ...rackLayoutStates.value, [rack.id]: 'loading' }
  try {
    const layout = await getRackLayout(rack.id)
    rackLayouts.value = { ...rackLayouts.value, [rack.id]: layout }
    rackLayoutStates.value = { ...rackLayoutStates.value, [rack.id]: 'loaded' }
  } catch (error) {
    rackLayoutStates.value = { ...rackLayoutStates.value, [rack.id]: 'error' }
    ElMessage.error(`无法读取机柜“${rack.name}”的 U 位布局：${errorMessage(error)}`)
  }
}
async function saveRack() {
  const form = rackEditorForm.value
  if (!hasRequiredCodeAndName(form.code, form.name, rackEditorMode.value === 'create' && rackEditorAutoCode.value)) { ElMessage.warning(rackEditorMode.value === 'create' && rackEditorAutoCode.value ? '请填写机柜名称' : '请填写机柜编码和名称'); return }
  if (!Number.isFinite(Number(form.uHeight)) || Number(form.uHeight) < 1) { ElMessage.warning('U 位数量必须大于 0'); return }
  rackEditorBusy.value = true
  try {
    if (rackEditorMode.value === 'create') {
      if (!selectedRoom.value) { ElMessage.warning('请先选择机房'); return }
      await createRack(selectedRoom.value.id, withAutoGeneratedCode({ ...form, id: undefined }, rackEditorAutoCode.value))
      ElMessage.success('机柜已新增')
    } else if (rackEditorTarget.value) {
      const updated = await updateRack(rackEditorTarget.value.id, rackPayload(rackEditorTarget.value, form))
      selectedRack.value = updated
      ElMessage.success('机柜信息已保存')
    }
    rackEditorDialog.value = false
    await loadTree()
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    rackEditorBusy.value = false
  }
}
async function removeRack(rack: Rack) {
  try {
    await ElMessageBox.confirm(`确定删除机柜“${rack.name}”吗？如果机柜内仍有设备，后端会拒绝删除。`, '删除确认', { type: 'warning' })
    await deleteRack(rack.id, rack.version)
    if (selectedRack.value?.id === rack.id) selectedRack.value = null
    rackDetailDrawer.value = false
    ElMessage.success('机柜已删除')
    await loadTree()
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') ElMessage.error(errorMessage(error))
  }
}
async function openRackUView(rack: Rack) { await router.push({ path: '/devices', query: { tab: 'layout', rackId: rack.id } }) }
async function openUView() { if (selectedRack.value) await openRackUView(selectedRack.value) }
async function goBack() { await router.push('/') }
async function toggleFullscreen() {
  try {
    if (!document.fullscreenElement) await document.documentElement.requestFullscreen()
    else await document.exitFullscreen()
  } catch { ElMessage.warning('当前浏览器不支持自动进入全屏，请使用浏览器全屏功能') }
}
function syncFullscreen() { isFullscreen.value = Boolean(document.fullscreenElement) }
function errorMessage(error: unknown) {
  const value = error as { response?: { data?: { message?: string } }; message?: string }
  return value.response?.data?.message ?? value.message ?? '加载数据失败'
}

onMounted(() => {
  void loadTree()
  timer = window.setInterval(() => { now.value = new Date() }, 1000)
  document.addEventListener('fullscreenchange', syncFullscreen)
  document.addEventListener('click', closeContextMenu)
  document.addEventListener('keydown', handleGlobalKey)
  window.addEventListener('resize', closeContextMenu)
})
onBeforeUnmount(() => {
  layoutRequestSerial += 1
  if (timer !== undefined) window.clearInterval(timer)
  document.removeEventListener('fullscreenchange', syncFullscreen)
  document.removeEventListener('click', closeContextMenu)
  document.removeEventListener('keydown', handleGlobalKey)
  window.removeEventListener('resize', closeContextMenu)
})
</script>

<style>
.screen-page { width: 100vw; min-width: 1180px; height: 100vh; min-height: 720px; display: flex; overflow: hidden; color: #d8efff; background: radial-gradient(circle at 75% 15%, #0a3153 0, #061a31 34%, #030d1d 75%); font-family: "Microsoft YaHei", "PingFang SC", sans-serif; }
.control-panel { width: 330px; flex: 0 0 330px; height: 100%; padding: 24px 22px 18px; box-sizing: border-box; overflow-y: auto; border-right: 1px solid rgba(45, 200, 240, .28); background: linear-gradient(180deg, rgba(7, 27, 49, .98), rgba(3, 14, 29, .98)); box-shadow: 12px 0 40px rgba(0, 0, 0, .28); }
.control-panel::-webkit-scrollbar, .rack-room-scroller::-webkit-scrollbar, .rack-strip::-webkit-scrollbar { width: 7px; height: 7px; }
.control-panel::-webkit-scrollbar-thumb, .rack-room-scroller::-webkit-scrollbar-thumb, .rack-strip::-webkit-scrollbar-thumb { border-radius: 8px; background: #174666; }
.brand-block { display: flex; align-items: center; gap: 13px; padding-bottom: 22px; border-bottom: 1px solid rgba(80, 170, 210, .2); }
.brand-mark { width: 48px; height: 48px; display: grid; place-items: center; border: 1px solid #2dc8f0; color: #65dcff; font-weight: 800; font-size: 18px; background: rgba(45, 200, 240, .1); box-shadow: inset 0 0 18px rgba(45, 200, 240, .16), 0 0 15px rgba(45, 200, 240, .12); clip-path: polygon(12% 0, 88% 0, 100% 12%, 100% 88%, 88% 100%, 12% 100%, 0 88%, 0 12%); }
.brand-block h1 { margin: 0 0 4px; font-size: 21px; letter-spacing: 2px; color: #e8f8ff; }.brand-block p { margin: 0; color: #547c99; font-size: 10px; letter-spacing: 1.4px; }
.selector-section { display: grid; gap: 9px; padding: 20px 0 6px; }.selector-section label { margin-top: 3px; color: #7da5bf; font-size: 12px; }.screen-select { width: 100%; }
.screen-select .el-select__wrapper { min-height: 40px; border: 1px solid #174d70; background: #071d34; box-shadow: none; }.screen-select .el-select__selected-item { color: #d8efff; }
.room-card, .legend-card, .selected-card { margin-top: 16px; padding: 15px; border: 1px solid rgba(50, 139, 184, .28); border-radius: 5px; background: linear-gradient(135deg, rgba(12, 44, 72, .72), rgba(4, 22, 40, .66)); }
.section-title { display: flex; align-items: center; gap: 8px; margin-bottom: 12px; color: #92b8d0; font-size: 12px; }.section-title > span { width: 3px; height: 13px; background: #2dc8f0; box-shadow: 0 0 8px #2dc8f0; }
.room-name { color: #f0fbff; font-size: 18px; font-weight: 700; }.room-code { margin-top: 3px; color: #4f7894; font-size: 11px; }
.room-meta { display: grid; grid-template-columns: 1fr 1fr; gap: 12px 10px; margin-top: 15px; }.room-meta span, .selected-grid span, .selected-device span { display: block; color: #557b94; font-size: 10px; }.room-meta strong { display: block; margin-top: 3px; overflow: hidden; color: #a8ccdf; font-size: 11px; text-overflow: ellipsis; white-space: nowrap; }
.stats-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; margin-top: 16px; }.stat-item { min-height: 70px; padding: 10px 11px; border: 1px solid rgba(45, 119, 153, .24); background: rgba(5, 28, 49, .72); box-sizing: border-box; }.stat-item span { display: block; color: #6f98b1; font-size: 11px; }.stat-item strong { display: inline-block; margin-top: 5px; color: currentColor; font-size: 27px; }.stat-item small { margin-left: 7px; color: #496c84; font-size: 8px; }.cyan { color: #2dc8f0; }.green { color: #24d6a1; }.blue { color: #4d8fff; }.orange { color: #ffb547; }
.device-legend-list { display: grid; grid-template-columns: 1fr 1fr; gap: 9px 14px; }.device-legend-list div { display: flex; align-items: center; gap: 7px; color: #89adc3; font-size: 11px; }.device-legend-list i { width: 9px; height: 9px; border-radius: 2px; box-shadow: 0 0 7px currentColor; }.u-note { margin-top: 11px; padding-top: 10px; border-top: 1px dashed #174562; color: #4f7891; font-size: 10px; line-height: 1.6; }
.selected-card { border-color: rgba(45, 200, 240, .5); }.selected-header { display: flex; align-items: flex-start; justify-content: space-between; }.selected-header small { display: block; color: #547d98; font-size: 10px; }.selected-header strong { display: block; margin-top: 3px; color: #e9f9ff; font-size: 16px; }.selected-header > span { font-size: 11px; }
.selected-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; margin-top: 13px; }.selected-grid b { display: block; margin-top: 3px; overflow: hidden; color: #a9ccdf; font-size: 11px; text-overflow: ellipsis; white-space: nowrap; }.selected-progress { height: 4px; margin-top: 12px; overflow: hidden; border-radius: 4px; background: #0a2a44; }.selected-progress i { display: block; height: 100%; background: linear-gradient(90deg, #24d6a1, #2dc8f0, #ffb547); box-shadow: 0 0 8px #2dc8f0; }
.selected-device { display: grid; grid-template-columns: 1fr 1fr; gap: 8px 10px; margin-top: 13px; padding: 11px; border: 1px solid #1d5879; background: rgba(13, 51, 77, .65); }.selected-device > small, .selected-device > strong { grid-column: 1 / -1; }.selected-device > small { color: #4f7894; font-size: 9px; }.selected-device > strong { overflow: hidden; color: #9be9ff; font-size: 13px; text-overflow: ellipsis; white-space: nowrap; }.selected-device b { display: block; margin-top: 2px; overflow: hidden; color: #a9ccdf; font-size: 10px; text-overflow: ellipsis; white-space: nowrap; }
.u-button { width: 100%; margin-top: 14px; padding: 9px; border: 1px solid #268db5; color: #7bdfff; background: rgba(24, 117, 154, .16); cursor: pointer; }.u-button:hover { background: rgba(45, 200, 240, .24); }
.panel-actions { display: flex; gap: 8px; margin-top: 16px; }.panel-actions button { flex: 1; padding: 9px 5px; border: 1px solid #174b6b; color: #719ab4; background: #061a2d; cursor: pointer; }.panel-actions button:hover { color: #bfeaff; border-color: #2b90b8; }
.visual-area { min-width: 0; flex: 1; display: flex; flex-direction: column; padding: 0 28px 24px; }
.screen-header { height: 86px; flex: 0 0 86px; display: flex; align-items: center; justify-content: space-between; border-bottom: 1px solid rgba(54, 140, 182, .2); }.screen-header h2 { margin: 0 0 5px; color: #e6f7ff; font-size: 22px; letter-spacing: 1px; }.screen-header h2 span { margin: 0 12px; color: #2dc8f0; }.screen-header p { margin: 0; color: #527a94; font-size: 11px; }.header-status { display: flex; align-items: center; gap: 28px; }.online { color: #58bda5; font-size: 11px; }.online i { display: inline-block; width: 7px; height: 7px; margin-right: 7px; border-radius: 50%; background: #24d6a1; box-shadow: 0 0 9px #24d6a1; }.clock { text-align: right; }.clock strong { display: block; color: #d9f3ff; font-family: Consolas, monospace; font-size: 22px; font-weight: 500; letter-spacing: 2px; }.clock span { color: #587e97; font-size: 10px; }
.canvas-shell { min-height: 0; flex: 1; display: flex; flex-direction: column; margin-top: 20px; overflow: hidden; border: 1px solid rgba(41, 136, 180, .36); background: rgba(4, 18, 34, .76); box-shadow: inset 0 0 50px rgba(5, 50, 79, .22), 0 10px 45px rgba(0, 0, 0, .2); }
.canvas-toolbar { min-height: 48px; flex: 0 0 48px; display: flex; align-items: center; justify-content: space-between; gap: 16px; padding: 0 18px; border-bottom: 1px solid rgba(35, 121, 159, .25); color: #98bbd0; font-size: 12px; }.pulse-dot { display: inline-block; width: 6px; height: 6px; margin-right: 8px; border-radius: 50%; background: #2dc8f0; box-shadow: 0 0 9px #2dc8f0; }.toolbar-right { display: flex; align-items: center; gap: 16px; }.canvas-info { color: #527b96; }
.rack-room-scroller { min-height: 0; flex: 1; overflow: auto; padding: 16px; }
.room-layout-canvas { position: relative; margin-top: 18px; overflow: visible; border: 1px dashed rgba(45, 200, 240, .22); background: rgba(4, 20, 35, .28); }
.layout-row-guide { position: absolute; left: 12px; right: 12px; display: flex; align-items: center; gap: 12px; pointer-events: none; }
.layout-row-guide span { flex: 0 0 auto; padding: 3px 7px; border: 1px solid rgba(45, 200, 240, .28); border-radius: 3px; color: #5b92ad; font-size: 9px; background: rgba(5, 24, 41, .86); }
.layout-row-guide i { flex: 1; border-top: 1px dashed rgba(45, 200, 240, .16); }
.rack-slot-frame { position: absolute; z-index: 1; width: 230px; height: 548px; box-sizing: border-box; border: 1px dashed rgba(45, 200, 240, .34); border-radius: 10px; background: linear-gradient(180deg, rgba(18, 75, 105, .2), rgba(5, 24, 42, .08)); box-shadow: inset 0 0 22px rgba(45, 200, 240, .05); pointer-events: none; }
.rack-slot-frame::before { content: '固定机柜槽位'; position: absolute; top: 7px; left: 9px; padding: 2px 5px; border-radius: 3px; color: rgba(105, 171, 199, .72); font-size: 8px; letter-spacing: 1px; background: rgba(5, 24, 41, .72); }
.rack-card-floating { position: absolute; z-index: 2; margin: 0; }
.rack-card-floating:hover { z-index: 10; }
.rack-card-floating.dragging { z-index: 20; transform: none; border-color: #6ce5ff; box-shadow: 0 0 0 2px rgba(45, 200, 240, .25), 0 18px 32px rgba(0, 0, 0, .42); }
.rack-drag-handle { cursor: grab; touch-action: none; }
.rack-card-floating.dragging .rack-drag-handle { cursor: grabbing; }
.drag-tip { color: #5a8da7; font-size: 10px; white-space: nowrap; }
:global(body.is-rack-dragging) { user-select: none; cursor: grabbing; }
.room-boundary { position: relative; min-width: 980px; min-height: 100%; padding: 18px 20px 64px; border: 1px solid #174b70; border-radius: 8px; box-sizing: border-box; background-color: #07172b; background-image: linear-gradient(rgba(21, 49, 80, .45) 1px, transparent 1px), linear-gradient(90deg, rgba(21, 49, 80, .45) 1px, transparent 1px); background-size: 24px 24px; box-shadow: inset 0 0 60px rgba(8, 55, 83, .18); }
.room-boundary-header { display: flex; justify-content: space-between; align-items: flex-start; padding: 5px 4px 18px; border-bottom: 1px solid #173c5a; }.room-boundary-header strong { display: block; color: #bdeeff; font-size: 16px; }.room-boundary-header span { display: block; margin-top: 4px; color: #4f7894; font-size: 10px; }.capacity-summary { text-align: right; }.capacity-summary strong { color: #2dc8f0; font-size: 22px; }.capacity-summary strong small { color: #567b93; font-size: 11px; }
.rack-row-section { margin-top: 18px; padding: 14px 14px 0; border: 1px solid rgba(32, 105, 143, .42); border-radius: 8px; background: rgba(6, 30, 51, .8); }.rack-row-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 12px; }.rack-row-header div { display: flex; align-items: baseline; gap: 10px; }.rack-row-header b { color: #7bdfff; font-size: 14px; }.rack-row-header span { color: #4d7690; font-size: 10px; }
.rack-strip { display: flex; align-items: flex-end; gap: 14px; overflow-x: auto; padding: 2px 2px 16px; }.screen-page .rack-card { width: 218px; flex: 0 0 218px; padding: 10px; border: 1px solid #1b4d6b; border-radius: 7px; outline: none; background: linear-gradient(180deg, #0a263f, #061a2d); box-shadow: 0 7px 18px rgba(0, 0, 0, .25); box-sizing: border-box; cursor: pointer; transition: border-color .18s, transform .18s, box-shadow .18s; }.screen-page .rack-card:hover, .screen-page .rack-card.selected { border-color: #2dc8f0; box-shadow: 0 0 0 1px rgba(45, 200, 240, .22), 0 8px 24px rgba(0, 0, 0, .3); transform: translateY(-2px); }.screen-page .rack-card.selected { background: linear-gradient(180deg, #0c3150, #071d33); }
.rack-card-header { display: flex; align-items: flex-start; justify-content: space-between; padding: 1px 2px 8px; }.rack-card-header strong { display: block; max-width: 168px; overflow: hidden; color: #d9f5ff; font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }.rack-card-header span { display: block; margin-top: 3px; color: #557d96; font-size: 9px; }.rack-card-header i { width: 8px; height: 8px; margin-top: 3px; border-radius: 50%; box-shadow: 0 0 8px currentColor; }
.rack-usage-line { display: flex; align-items: center; justify-content: space-between; color: #557d96; font-size: 8px; }.rack-usage-line b { color: #8fdfff; font-size: 10px; }.rack-usage-bar { height: 3px; margin: 5px 0 9px; overflow: hidden; background: #081929; }.rack-usage-bar i { display: block; height: 100%; background: linear-gradient(90deg, #24d6a1, #2dc8f0, #ffb547); }
.cabinet-shell { padding: 0 7px; }.cabinet-top { height: 18px; display: flex; align-items: center; justify-content: space-between; padding: 0 7px; border: 1px solid #3d5c70; border-bottom: 0; border-radius: 4px 4px 0 0; color: #7698aa; background: linear-gradient(#293f4d, #162d3d); box-shadow: inset 0 1px rgba(255,255,255,.12); }.cabinet-top span { width: 5px; height: 5px; border-radius: 50%; background: #071521; box-shadow: inset 0 0 0 1px #5d7787; }.cabinet-top b { font-size: 8px; font-weight: 500; letter-spacing: 2px; }
.cabinet-frame { position: relative; height: var(--rack-height); padding: 0 12px 0 30px; border: 3px solid #38576b; border-top-width: 2px; border-bottom-width: 2px; box-sizing: content-box; background: #06111d; box-shadow: inset 0 0 12px #000, 0 5px 12px rgba(0,0,0,.35); }.rack-rail { position: absolute; z-index: 4; top: 0; bottom: 0; width: 6px; background: repeating-linear-gradient(to bottom, #324d5e 0 2px, #0b1821 2px 5px); box-shadow: inset 0 0 2px #7992a0; pointer-events: none; }.rack-rail.left { left: 20px; }.rack-rail.right { right: 3px; }.u-scale { position: absolute; top: 0; left: 0; width: 20px; height: var(--rack-height); display: flex; flex-direction: column; background: #102536; }.u-scale span { height: var(--u-height); flex: 0 0 var(--u-height); display: grid; place-items: center; border-bottom: 1px solid #294355; box-sizing: border-box; color: #6e91a5; font-family: Consolas, monospace; font-size: 6px; line-height: 1; }.u-grid { position: relative; height: var(--rack-height); overflow: hidden; border: 1px solid #315167; box-sizing: border-box; background: #0a1825; }.u-line { width: 100%; height: var(--u-height); display: block; padding: 0; box-sizing: border-box; border: 0; border-bottom: 1px solid #263c4c; background: linear-gradient(90deg, rgba(20, 48, 67, .35), rgba(8, 23, 35, .2)); cursor: context-menu; }.u-line:hover:not(.occupied) { background: linear-gradient(90deg, rgba(45, 200, 240, .22), rgba(45, 200, 240, .08)); }.u-line.major { border-bottom-color: #496b7e; }.u-line.occupied { cursor: default; }
.device-block { position: absolute; z-index: 2; left: 1px; right: 1px; width: calc(100% - 2px); display: flex; align-items: center; justify-content: space-between; gap: 3px; min-height: 8px; padding: 0 4px; overflow: hidden; border: 1px solid color-mix(in srgb, var(--device-color) 72%, #d9f7ff); box-sizing: border-box; color: #f2fbff; background: linear-gradient(90deg, color-mix(in srgb, var(--device-color) 78%, #071523), color-mix(in srgb, var(--device-color) 52%, #071523)); box-shadow: inset 3px 0 var(--device-color), inset 0 0 5px rgba(255,255,255,.12); cursor: pointer; }.device-block:hover { z-index: 5; filter: brightness(1.22); box-shadow: inset 3px 0 var(--device-color), 0 0 8px var(--device-color); }.device-block strong { min-width: 0; overflow: hidden; font-size: 7px; font-weight: 600; line-height: 1; text-overflow: ellipsis; white-space: nowrap; }.device-block small { flex: 0 0 auto; font-size: 6px; opacity: .82; white-space: nowrap; }
.rack-state-overlay { position: absolute; z-index: 8; inset: 0; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 7px; color: #6f9ab4; font-size: 9px; background: rgba(5, 17, 29, .83); }.rack-state-overlay.error { color: #ef8790; }.rack-state-overlay.error strong { display: grid; width: 22px; height: 22px; place-items: center; border: 1px solid #ef6470; border-radius: 50%; }.loading-ring { width: 20px; height: 20px; border: 2px solid #1c506d; border-top-color: #2dc8f0; border-radius: 50%; animation: rack-loading .8s linear infinite; }
.cabinet-base { height: 12px; display: flex; justify-content: space-between; padding: 0 10px; border: 1px solid #3d5c70; border-top: 0; border-radius: 0 0 4px 4px; background: #172d3c; }.cabinet-base span { width: 17px; height: 5px; margin-top: 11px; border-radius: 0 0 3px 3px; background: #203d50; box-shadow: 0 2px 3px #000; }
.rack-card-footer { display: flex; align-items: center; justify-content: space-between; margin-top: 13px; padding: 0 2px; color: #547a91; font-size: 9px; }.rack-card-footer button { padding: 3px 8px; border: 1px solid #1e5d7e; color: #68cbe9; background: #0a2940; cursor: pointer; }.rack-card-footer button:hover { border-color: #2dc8f0; color: #c9f7ff; }
.aisle-divider { display: grid; grid-template-columns: 1fr auto 1fr; align-items: center; gap: 14px; padding: 15px 6px 14px; }.aisle-divider span { border-top: 1px dashed #1d5879; }.aisle-divider b { color: #347393; font-size: 9px; font-weight: 500; letter-spacing: 7px; }
.room-door { position: absolute; right: 35px; bottom: 20px; display: flex; align-items: center; gap: 9px; color: #4f8ba9; font-size: 10px; }.room-door i { width: 36px; height: 20px; border-top: 3px solid #2dc8f0; border-left: 3px solid #2dc8f0; border-radius: 18px 0 0; }
.empty-screen { flex: 1; display: grid; place-content: center; text-align: center; }.empty-icon { width: 70px; height: 70px; display: grid; place-items: center; margin: 0 auto 18px; border: 1px solid #1e5372; color: #2e84a8; font-size: 30px; background: rgba(15, 61, 91, .24); transform: rotate(45deg); }.empty-screen h3 { margin: 0 0 9px; color: #86aec5; font-size: 17px; }.empty-screen p { margin: 0; color: #426980; font-size: 11px; }
.filter-hint { margin-left: auto; color: #4f7891; font-size: 10px; font-weight: 400; }
.status-legend-list { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; }
.status-legend-item { display: flex; align-items: center; gap: 7px; padding: 7px 9px; border: 1px solid #173f5c; border-radius: 4px; color: #89adc3; font-size: 11px; font-family: inherit; text-align: left; background: rgba(6, 27, 47, .55); cursor: pointer; transition: border-color .18s, background .18s; }
.status-legend-item:hover { border-color: #2dc8f0; }
.status-legend-item.active { border-color: #2dc8f0; background: rgba(45, 200, 240, .14); color: #d9f6ff; }
.status-legend-item i { width: 9px; height: 9px; flex: 0 0 9px; border-radius: 2px; box-shadow: 0 0 7px currentColor; }
.status-legend-item i.all-dot { background: #d8efff; }
.status-legend-item b { margin-left: auto; color: #d5ecf8; font-family: Consolas, monospace; font-size: 11px; }
.screen-page .filter-bar { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; padding: 8px 16px; border-bottom: 1px solid rgba(45, 119, 153, .25); background: rgba(4, 17, 31, .55); }
.filter-chip { display: inline-flex; align-items: center; gap: 6px; padding: 4px 12px; border: 1px solid #173f5c; border-radius: 999px; color: #89adc3; font-size: 11px; font-family: inherit; background: rgba(6, 27, 47, .55); cursor: pointer; transition: border-color .18s, color .18s, background .18s; }
.filter-chip:hover { border-color: #2dc8f0; color: #d9f6ff; }
.filter-chip.active { border-color: #2dc8f0; color: #d9f6ff; background: rgba(45, 200, 240, .16); box-shadow: 0 0 8px rgba(45, 200, 240, .25); }
.filter-chip i { width: 8px; height: 8px; border-radius: 2px; }
.filter-chip span { min-width: 16px; padding: 0 4px; border-radius: 8px; text-align: center; color: #dff4ff; font-family: Consolas, monospace; font-size: 10px; background: rgba(45, 200, 240, .18); }
.rack-card-header .rack-status-chip { display: inline-flex; align-items: center; gap: 4px; margin-top: 2px; padding: 2px 7px; border: 1px solid var(--status-color); border-radius: 999px; color: var(--status-color); font-size: 9px; font-weight: 600; background: color-mix(in srgb, var(--status-color) 12%, transparent); }
.rack-status-chip i { width: 6px; height: 6px; margin: 0; border-radius: 50%; background: var(--status-color); box-shadow: 0 0 6px var(--status-color); }
.rack-status-veil { position: absolute; z-index: 6; inset: 0; display: grid; place-items: center; pointer-events: none; background: repeating-linear-gradient(135deg, rgba(6, 17, 29, .55) 0 14px, rgba(6, 17, 29, .78) 14px 28px); }
.rack-status-veil span { padding: 4px 14px; border: 1px solid currentColor; color: #ffd9dc; font-size: 11px; font-weight: 700; letter-spacing: 3px; background: rgba(5, 14, 24, .88); box-shadow: 0 0 12px rgba(0, 0, 0, .5); }
.veil-maintenance span { color: #d9c7ff; border-color: #a78bfa; box-shadow: 0 0 10px rgba(167, 139, 250, .35); }
.veil-disabled span { color: #ffc4c9; border-color: #ef6470; box-shadow: 0 0 10px rgba(239, 100, 112, .35); }
.veil-planning span { color: #c3d4e2; border-color: #7188a8; box-shadow: 0 0 10px rgba(113, 136, 168, .35); }
.cabinet-shell.is-full .cabinet-frame { border-color: rgba(255, 181, 71, .55); box-shadow: inset 0 0 12px #000, 0 5px 12px rgba(0, 0, 0, .35), 0 0 10px rgba(255, 181, 71, .22); }
.rack-context-menu { position: fixed; z-index: 5000; width: 236px; padding: 7px; border: 1px solid rgba(45, 200, 240, .55); border-radius: 8px; color: #dff7ff; background: linear-gradient(180deg, rgba(8, 32, 53, .99), rgba(3, 16, 29, .99)); box-shadow: 0 16px 42px rgba(0, 0, 0, .58), inset 0 0 24px rgba(45, 200, 240, .06); font-family: "Microsoft YaHei", "PingFang SC", sans-serif; }
.context-menu-header { padding: 8px 10px 10px; border-bottom: 1px solid rgba(45, 200, 240, .18); }.context-menu-header small { display: block; color: #53819c; font-size: 10px; }.context-menu-header strong { display: block; margin-top: 3px; overflow: hidden; color: #dcf8ff; font-size: 13px; text-overflow: ellipsis; white-space: nowrap; }.context-menu-header span { display: block; margin-top: 4px; color: #729bb3; font-size: 10px; }
.rack-context-menu > button { width: 100%; display: block; margin-top: 4px; padding: 8px 10px; border: 0; border-radius: 5px; color: #bfe8f7; text-align: left; background: transparent; cursor: pointer; }.rack-context-menu > button:hover { color: #ecfbff; background: rgba(45, 200, 240, .13); }.rack-context-menu > button b, .rack-context-menu > button span { display: block; }.rack-context-menu > button b { font-size: 12px; }.rack-context-menu > button span { margin-top: 3px; color: #527b94; font-size: 9px; }.rack-context-menu > button.danger b { color: #ff9099; }.rack-context-menu > button.danger:hover { background: rgba(239, 100, 112, .13); }.rack-context-menu > button:disabled { opacity: .42; cursor: not-allowed; background: transparent; }
.form-tip { margin-top: 6px; color: #909399; font-size: 12px; }.form-suffix { margin-left: 8px; color: #606266; }.quick-position-form { margin-top: 18px; }.device-detail-panel { min-height: 260px; }.detail-footer { display: flex; justify-content: flex-end; gap: 8px; }
@keyframes rack-loading { to { transform: rotate(360deg); } }
@media (max-width: 1350px) { .control-panel { width: 300px; flex-basis: 300px; padding-left: 17px; padding-right: 17px; }.visual-area { padding-left: 18px; padding-right: 18px; }.screen-page { min-width: 1100px; } }
.room-card-actions { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px; margin-top: 14px; }.room-card-actions button, .selected-actions button { flex: 1; padding: 7px 6px; border: 1px solid #1e5d7e; color: #7bdfff; background: rgba(10, 41, 64, .8); cursor: pointer; }.room-card-actions button:hover, .selected-actions button:hover { border-color: #2dc8f0; background: rgba(45, 200, 240, .18); }.selected-status-button { padding: 2px 0; border: 0; font: inherit; background: transparent; cursor: pointer; }.selected-status-button:hover { text-decoration: underline; }.selected-actions { display: flex; gap: 6px; margin-top: 12px; }.selected-actions button.danger { color: #ff9ca3; border-color: rgba(239, 100, 112, .55); }.canvas-toolbar { align-items: center; }.toolbar-right { flex-wrap: wrap; justify-content: flex-end; }.rack-search { width: 220px; }.rack-search .el-input__wrapper { min-height: 28px; border: 1px solid #174d70; box-shadow: none; background: #061a2e; }.rack-search .el-input__inner { color: #d8efff; }.canvas-action { padding: 6px 9px; border: 1px solid #174d70; border-radius: 4px; color: #8db5ca; font: inherit; font-size: 11px; background: #061a2e; cursor: pointer; }.canvas-action:hover:not(:disabled) { border-color: #2dc8f0; color: #d9f7ff; }.canvas-action.primary { border-color: #268db5; color: #9eeeff; background: rgba(24, 117, 154, .22); }.canvas-action:disabled { opacity: .42; cursor: not-allowed; }.screen-page .rack-card { width: 218px; min-width: 218px; max-width: 218px; height: var(--rack-card-height); min-height: var(--rack-card-height); display: flex; flex-direction: column; overflow: hidden; box-sizing: border-box; align-self: start; }.rack-card-floating:hover, .rack-card-floating.selected { transform: none; }.rack-card-floating.dragging { transform: none; }.rack-card-header { min-height: 31px; box-sizing: border-box; }.rack-card-header > div { min-width: 0; }.rack-card-header .rack-status-chip { flex: 0 0 auto; appearance: none; white-space: nowrap; font-family: inherit; cursor: pointer; }.rack-card-header .rack-status-chip:hover { filter: brightness(1.18); }.rack-slot-frame { width: 218px; height: auto; }.rack-editor-form { margin-top: 18px; }.rack-form-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 2px 18px; }.rack-form-grid .el-input-number { width: 100%; }.rack-form-grid .full-width { grid-column: 1 / -1; }.rack-detail-heading { display: flex; justify-content: space-between; gap: 18px; padding: 4px 0 18px; border-bottom: 1px solid #e5e7eb; }.rack-detail-heading small { color: #6b7280; }.rack-detail-heading h3 { margin: 5px 0 3px; color: #1f2937; font-size: 21px; }.rack-detail-heading p { margin: 0; color: #9ca3af; }.rack-detail-heading > span { padding-top: 5px; font-size: 13px; font-weight: 600; }.rack-detail-grid { padding-top: 20px; }
@media (max-width: 900px) { .toolbar-right { justify-content: flex-start; }.rack-search { width: 180px; }.rack-form-grid { grid-template-columns: 1fr; }.rack-form-grid .full-width { grid-column: auto; } }
.code-field { width:100%; display:flex; align-items:center; gap:10px; }
.code-field .el-input { flex:1; }
.code-field .el-checkbox { flex:none; }
.device-editor-form { margin-top: 18px; max-height: 64vh; overflow-y: auto; padding-right: 8px; }
</style>

<style>
.screen-page .canvas-search-button { min-width: 48px; }
.screen-page .device-block.search-hit { z-index: 4; outline: 2px solid #ffe08a; box-shadow: 0 0 0 3px rgba(255, 224, 138, .22), 0 0 18px rgba(255, 224, 138, .75); animation: device-search-hit 1.1s ease-in-out infinite alternate; }
@keyframes device-search-hit { from { filter: brightness(1); } to { filter: brightness(1.45); } }
</style>

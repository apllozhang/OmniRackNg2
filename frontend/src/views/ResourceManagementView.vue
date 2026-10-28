<template>
  <div class="resource-page">
    <div class="page-header">
      <div>
        <h2>基础资源管理</h2>
        <p>按数据中心、机房、机柜逐级维护资源，当前支持整 U 机柜和可选电源参数。</p>
      </div>
      <div class="header-actions">
        <el-button :loading="loading" @click="loadTree">刷新</el-button>
        <el-button type="primary" @click="openDataCenterCreate">新增数据中心</el-button>
      </div>
    </div>

    <el-row :gutter="16" class="summary-row">
      <el-col :span="8"><el-card shadow="never"><div class="summary"><span>数据中心</span><strong>{{ tree.length }}</strong></div></el-card></el-col>
      <el-col :span="8"><el-card shadow="never"><div class="summary"><span>机房</span><strong>{{ totalRooms }}</strong></div></el-card></el-col>
      <el-col :span="8"><el-card shadow="never"><div class="summary"><span>机柜</span><strong>{{ totalRacks }}</strong></div></el-card></el-col>
    </el-row>

    <el-row :gutter="16" class="resource-columns" v-loading="loading">
      <el-col :span="7">
        <el-card class="resource-card" shadow="never">
          <template #header><div class="card-title"><span>数据中心</span><el-tag size="small">{{ tree.length }} 个</el-tag></div></template>
          <el-empty v-if="!tree.length" description="暂无数据中心" />
          <div
            v-for="item in tree"
            :key="item.id"
            class="resource-item"
            :class="{ active: selectedDataCenterId === item.id }"
            @click="selectDataCenter(item.id)"
          >
            <div class="item-main">
              <div class="item-name">{{ item.name }}</div>
              <div class="item-code">{{ item.code }} · {{ item.rooms?.length ?? 0 }} 个机房</div>
            </div>
            <el-tag size="small" :type="statusType(item.status)">{{ statusLabel(item.status) }}</el-tag>
            <el-dropdown trigger="click" @command="(command: string) => handleDataCenterCommand(command, item)" @click.stop>
              <el-button link>•••</el-button>
              <template #dropdown><el-dropdown-menu><el-dropdown-item command="edit">编辑</el-dropdown-item><el-dropdown-item command="copy">复制数据中心</el-dropdown-item><el-dropdown-item command="delete" divided>删除</el-dropdown-item></el-dropdown-menu></template>
            </el-dropdown>
          </div>
        </el-card>
      </el-col>

      <el-col :span="7">
        <el-card class="resource-card" shadow="never">
          <template #header>
            <div class="card-title">
              <span>机房</span>
              <el-button link type="primary" :disabled="!selectedDataCenter || parentDisabled(selectedDataCenter?.status)" @click="openRoomCreate">新增</el-button>
            </div>
          </template>
          <el-empty v-if="!selectedDataCenter" description="请先选择数据中心" />
          <el-empty v-else-if="!rooms.length" description="暂无机房" />
          <div
            v-for="item in rooms"
            :key="item.id"
            class="resource-item"
            :class="{ active: selectedRoomId === item.id }"
            @click="selectedRoomId = item.id"
          >
            <div class="item-main">
              <div class="item-name">{{ item.name }}</div>
              <div class="item-code">{{ item.code }} · {{ item.racks?.length ?? 0 }} 个机柜</div>
              <div class="item-meta">{{ [item.building, item.floor, item.roomNumber].filter(Boolean).join(' / ') || '未设置物理位置' }}</div>
            </div>
            <el-tag size="small" :type="statusType(item.status)">{{ statusLabel(item.status) }}</el-tag>
            <el-dropdown trigger="click" @command="(command: string) => handleRoomCommand(command, item)" @click.stop>
              <el-button link>•••</el-button>
              <template #dropdown><el-dropdown-menu><el-dropdown-item command="edit">编辑</el-dropdown-item><el-dropdown-item command="copy">复制机房</el-dropdown-item><el-dropdown-item command="move">移动到其他数据中心</el-dropdown-item><el-dropdown-item command="delete" divided>删除</el-dropdown-item></el-dropdown-menu></template>
            </el-dropdown>
          </div>
        </el-card>
      </el-col>

      <el-col :span="10">
        <el-card class="resource-card rack-card" shadow="never">
          <template #header>
            <div class="card-title">
              <span>机柜</span>
              <el-button link type="primary" :disabled="!selectedRoom || parentDisabled(selectedRoom?.status)" @click="openRackCreate">新增</el-button>
            </div>
          </template>
          <el-empty v-if="!selectedRoom" description="请先选择机房" />
          <el-empty v-else-if="!racks.length" description="暂无机柜" />
          <el-table v-else :data="racks" size="small" stripe>
            <el-table-column prop="code" label="编码" min-width="90" />
            <el-table-column prop="name" label="名称" min-width="110" />
            <el-table-column label="模板" min-width="120"><template #default="scope">{{ scope.row.templateCode ? `${scope.row.templateCode} V${scope.row.templateRevision}` : '自定义参数' }}</template></el-table-column>
            <el-table-column label="位置" min-width="100">
              <template #default="scope">{{ [scope.row.zone, scope.row.row, scope.row.column].filter(Boolean).join('-') || '-' }}</template>
            </el-table-column>
            <el-table-column label="容量" width="70"><template #default="scope">{{ scope.row.uHeight }}U</template></el-table-column>
            <el-table-column label="状态" width="88"><template #default="scope"><el-tag size="small" :type="statusType(scope.row.status)">{{ statusLabel(scope.row.status) }}</el-tag></template></el-table-column>
            <el-table-column label="操作" width="130" fixed="right">
              <template #default="scope">
                <el-button link type="primary" @click="openRackEdit(scope.row)">编辑</el-button>
                <el-dropdown trigger="click" @command="(command: string) => handleRackCommand(command, scope.row)">
                  <el-button link type="primary">更多</el-button>
                  <template #dropdown><el-dropdown-menu><el-dropdown-item command="copy">复制机柜</el-dropdown-item><el-dropdown-item command="move">移动到其他机房</el-dropdown-item><el-dropdown-item command="delete" divided>删除</el-dropdown-item></el-dropdown-menu></template>
                </el-dropdown>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-col>
    </el-row>

    <el-dialog v-model="dataCenterDialog" :title="dataCenterMode === 'create' ? '新增数据中心' : '编辑数据中心'" width="640px" destroy-on-close>
      <el-form label-width="100px">
        <el-row :gutter="16"><el-col :span="12"><el-form-item label="编码" :required="dataCenterMode === 'edit' || !dataCenterAutoCode"><div class="code-field"><el-input v-model="dcForm.code" maxlength="50" :disabled="dataCenterMode === 'create' && dataCenterAutoCode" :placeholder="dataCenterMode === 'create' && dataCenterAutoCode ? '保存时自动生成' : '请输入数据中心编码'" /><el-checkbox v-if="dataCenterMode === 'create'" v-model="dataCenterAutoCode">自动生成</el-checkbox></div></el-form-item></el-col><el-col :span="12"><el-form-item label="名称" required><el-input v-model="dcForm.name" maxlength="150" /></el-form-item></el-col></el-row>
        <el-row :gutter="16"><el-col :span="12"><el-form-item label="状态"><el-select v-model="dcForm.status" style="width:100%"><el-option v-for="item in resourceStatuses" :key="item.value" :label="item.label" :value="item.value" /></el-select></el-form-item></el-col><el-col :span="12"><el-form-item label="负责人"><el-input v-model="dcForm.manager" /></el-form-item></el-col></el-row>
        <el-form-item label="地址"><el-input v-model="dcForm.address" /></el-form-item>
        <el-row :gutter="16"><el-col :span="12"><el-form-item label="联系方式"><el-input v-model="dcForm.contact" /></el-form-item></el-col><el-col :span="12"><el-form-item label="服务商"><el-input v-model="dcForm.serviceProvider" /></el-form-item></el-col></el-row>
        <el-form-item label="备注"><el-input v-model="dcForm.remarks" type="textarea" :rows="3" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="dataCenterDialog = false">取消</el-button><el-button type="primary" :loading="saving" @click="submitDataCenter">保存</el-button></template>
    </el-dialog>

    <el-dialog v-model="roomDialog" :title="roomMode === 'create' ? '新增机房' : '编辑机房'" width="720px" destroy-on-close>
      <el-form label-width="110px">
        <el-row :gutter="16"><el-col :span="12"><el-form-item label="编码" :required="roomMode === 'edit' || !roomAutoCode"><div class="code-field"><el-input v-model="roomForm.code" :disabled="roomMode === 'create' && roomAutoCode" :placeholder="roomMode === 'create' && roomAutoCode ? '保存时自动生成' : '请输入机房编码'" /><el-checkbox v-if="roomMode === 'create'" v-model="roomAutoCode">自动生成</el-checkbox></div></el-form-item></el-col><el-col :span="12"><el-form-item label="名称" required><el-input v-model="roomForm.name" /></el-form-item></el-col></el-row>
        <el-row :gutter="16"><el-col :span="12"><el-form-item label="状态"><el-select v-model="roomForm.status" style="width:100%"><el-option v-for="item in resourceStatuses" :key="item.value" :label="item.label" :value="item.value" /></el-select></el-form-item></el-col><el-col :span="12"><el-form-item label="面积(㎡)"><el-input-number v-model="roomForm.areaSquareMeters" :min="0" style="width:100%" /></el-form-item></el-col></el-row>
        <el-row :gutter="16"><el-col :span="8"><el-form-item label="楼栋"><el-input v-model="roomForm.building" /></el-form-item></el-col><el-col :span="8"><el-form-item label="楼层"><el-input v-model="roomForm.floor" /></el-form-item></el-col><el-col :span="8"><el-form-item label="房间号"><el-input v-model="roomForm.roomNumber" /></el-form-item></el-col></el-row>
        <el-row :gutter="16"><el-col :span="12"><el-form-item label="负责人"><el-input v-model="roomForm.manager" /></el-form-item></el-col><el-col :span="12"><el-form-item label="联系方式"><el-input v-model="roomForm.contact" /></el-form-item></el-col></el-row>
        <el-row :gutter="16"><el-col :span="12"><el-form-item label="平面图"><el-switch v-model="roomForm.floorPlanEnabled" active-text="启用" inactive-text="关闭" /></el-form-item></el-col><el-col :span="12"><el-form-item label="默认格式"><el-select v-model="roomForm.floorPlanFormat" style="width:100%"><el-option label="SVG" value="SVG" /><el-option label="PNG" value="PNG" /><el-option label="JPG" value="JPG" /></el-select></el-form-item></el-col></el-row>
        <el-row :gutter="16"><el-col :span="12"><el-form-item label="每排机柜数" required><el-input-number v-model="roomForm.racksPerRow" :min="1" :max="20" :step="1" step-strictly style="width:100%" /></el-form-item></el-col><el-col :span="12"><div class="room-layout-tip">用于控制机房资源大屏每排最多展示的机柜数量，超过后自动换行。</div></el-col></el-row>
        <el-form-item label="用途"><el-input v-model="roomForm.purpose" /></el-form-item>
        <el-form-item label="备注"><el-input v-model="roomForm.remarks" type="textarea" :rows="3" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="roomDialog = false">取消</el-button><el-button type="primary" :loading="saving" @click="submitRoom">保存</el-button></template>
    </el-dialog>

    <el-dialog v-model="rackDialog" :title="rackMode === 'create' ? '新增机柜' : '编辑机柜'" width="820px" destroy-on-close>
      <el-form label-width="110px">
        <el-row :gutter="16"><el-col :span="8"><el-form-item label="编码" :required="rackMode === 'edit' || !rackAutoCode"><div class="code-field"><el-input v-model="rackForm.code" :disabled="rackMode === 'create' && rackAutoCode" :placeholder="rackMode === 'create' && rackAutoCode ? '保存时自动生成' : '请输入机柜编码'" /><el-checkbox v-if="rackMode === 'create'" v-model="rackAutoCode">自动生成</el-checkbox></div></el-form-item></el-col><el-col :span="8"><el-form-item label="名称" required><el-input v-model="rackForm.name" /></el-form-item></el-col><el-col :span="8"><el-form-item label="状态"><el-select v-model="rackForm.status" style="width:100%"><el-option v-for="item in rackStatuses" :key="item.value" :label="item.label" :value="item.value" /></el-select></el-form-item></el-col></el-row>
        <el-form-item v-if="rackMode === 'create'" label="机柜模板"><el-select v-model="rackForm.templateVersionId" clearable placeholder="不选择则使用自定义参数" style="width:100%" @change="applySelectedTemplate"><el-option v-for="option in templateOptions" :key="option.version.id" :label="`${option.template.name} · V${option.version.revision} · ${option.version.uHeight}U`" :value="option.version.id" /></el-select></el-form-item>
        <el-alert v-else-if="rackForm.templateCode" :title="`实例快照：${rackForm.templateName} V${rackForm.templateRevision}`" type="info" :closable="false" class="rack-template-alert" />
        <el-row :gutter="16"><el-col :span="6"><el-form-item label="U 位"><el-input-number v-model="rackForm.uHeight" :min="1" :max="100" :step="1" step-strictly controls-position="right" style="width:100%" /></el-form-item></el-col><el-col :span="6"><el-form-item label="宽(mm)"><el-input-number v-model="rackForm.widthMm" :min="1" :step="10" controls-position="right" style="width:100%" /></el-form-item></el-col><el-col :span="6"><el-form-item label="深(mm)"><el-input-number v-model="rackForm.depthMm" :min="1" :step="10" controls-position="right" style="width:100%" /></el-form-item></el-col><el-col :span="6"><el-form-item label="高(mm)"><el-input-number v-model="rackForm.heightMm" :min="1" :step="10" controls-position="right" style="width:100%" /></el-form-item></el-col></el-row>
        <el-row :gutter="16"><el-col :span="12"><el-form-item label="厂商"><el-input v-model="rackForm.manufacturer" /></el-form-item></el-col><el-col :span="12"><el-form-item label="型号"><el-input v-model="rackForm.modelNumber" /></el-form-item></el-col></el-row>
        <el-row :gutter="16"><el-col :span="6"><el-form-item label="区域"><el-input v-model="rackForm.zone" /></el-form-item></el-col><el-col :span="6"><el-form-item label="排"><el-input v-model="rackForm.row" /></el-form-item></el-col><el-col :span="6"><el-form-item label="列"><el-input v-model="rackForm.column" /></el-form-item></el-col><el-col :span="6"><el-form-item label="通道"><el-input v-model="rackForm.aisle" /></el-form-item></el-col></el-row>
        <el-row :gutter="16"><el-col :span="8"><el-form-item label="双路电源"><el-switch v-model="rackForm.dualPower" /></el-form-item></el-col><el-col :span="8"><el-form-item label="输入路数"><el-input-number v-model="rackForm.inputCircuits" :min="0" :step="1" step-strictly controls-position="right" style="width:100%" /></el-form-item></el-col><el-col :span="8"><el-form-item label="PDU 数量"><el-input-number v-model="rackForm.pduCount" :min="0" :step="1" step-strictly controls-position="right" style="width:100%" /></el-form-item></el-col></el-row>
        <el-row :gutter="16"><el-col :span="12"><el-form-item label="负责人"><el-input v-model="rackForm.manager" /></el-form-item></el-col><el-col :span="12"><el-form-item label="用途"><el-input v-model="rackForm.purpose" /></el-form-item></el-col></el-row>
        <el-form-item label="备注"><el-input v-model="rackForm.remarks" type="textarea" :rows="3" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="rackDialog = false">取消</el-button><el-button type="primary" :loading="saving" @click="submitRack">保存</el-button></template>
    </el-dialog>

    <el-dialog v-model="transferDialog" :title="transferTitle" width="560px" destroy-on-close>
      <el-alert :title="transferTip" type="info" :closable="false" show-icon class="transfer-tip" />
      <el-form label-width="120px">
        <el-form-item v-if="transferKind === 'room'" label="目标数据中心" required>
          <el-select v-model="transferForm.targetDataCenterId" filterable style="width:100%" @change="transferForm.targetRoomId = ''">
            <el-option v-for="item in availableDataCenters" :key="item.id" :label="`${item.name}（${item.code}）`" :value="item.id" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="transferKind === 'rack'" label="目标机房" required>
          <el-select v-model="transferForm.targetRoomId" filterable style="width:100%">
            <el-option-group v-for="dc in availableRackTargets" :key="dc.id" :label="dc.name">
              <el-option v-for="room in dc.rooms" :key="room.id" :label="`${room.name}（${room.code}）`" :value="room.id" />
            </el-option-group>
          </el-select>
        </el-form-item>
        <el-form-item v-if="transferAction === 'copy'" label="副本名称" required><el-input v-model="transferForm.name" maxlength="150" /></el-form-item>
        <el-form-item label="资源编码" :required="!transferForm.autoGenerateCode">
          <div class="code-field"><el-input v-model="transferForm.code" maxlength="50" :disabled="transferForm.autoGenerateCode" :placeholder="transferForm.autoGenerateCode ? '提交时自动生成' : '请输入目标编码'" /><el-checkbox v-model="transferForm.autoGenerateCode">自动生成</el-checkbox></div>
        </el-form-item>
      </el-form>
      <template #footer><el-button @click="transferDialog = false">取消</el-button><el-button type="primary" :loading="saving" @click="submitTransfer">确认{{ transferAction === 'copy' ? '复制' : '移动' }}</el-button></template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  copyDataCenter, copyRack, copyRoom,
  createDataCenter, createRack, createRoom,
  deleteDataCenter, deleteRack, deleteRoom,
  getResourceTree, moveRack, moveRoom, updateDataCenter, updateRack, updateRoom,
} from '@/api/resource'
import type { DataCenter, DataCenterPayload, Rack, RackPayload, ResourceCopyPayload, ResourceMovePayload, Room, RoomPayload } from '@/types/resource'
import { getRackTemplates } from '@/api/rack-template'
import type { RackTemplate, RackTemplateVersion } from '@/types/rack-template'
import { createRackPayload, latestRackTemplateVersion, applyRackTemplateSpec } from '@/features/rack/rackDefaults'
import { hasRequiredCodeAndName, withAutoGeneratedCode } from '@/features/code/autoCode'

const tree = ref<DataCenter[]>([])
const rackTemplates = ref<RackTemplate[]>([])
const loading = ref(false)
const saving = ref(false)
const selectedDataCenterId = ref('')
const selectedRoomId = ref('')

const resourceStatuses = [
  { label: '规划中', value: 'PLANNING' }, { label: '运行中', value: 'OPERATING' },
  { label: '维护中', value: 'MAINTENANCE' }, { label: '停用', value: 'DISABLED' }, { label: '归档', value: 'ARCHIVED' },
]
const rackStatuses = [
  { label: '规划中', value: 'PLANNING' }, { label: '空闲', value: 'AVAILABLE' },
  { label: '部分使用', value: 'PARTIAL' }, { label: '已满', value: 'FULL' },
  { label: '维护中', value: 'MAINTENANCE' }, { label: '停用', value: 'DISABLED' },
]

const selectedDataCenter = computed(() => tree.value.find((item) => item.id === selectedDataCenterId.value))
const rooms = computed(() => selectedDataCenter.value?.rooms ?? [])
const selectedRoom = computed(() => rooms.value.find((item) => item.id === selectedRoomId.value))
const racks = computed(() => selectedRoom.value?.racks ?? [])
const totalRooms = computed(() => tree.value.reduce((sum, item) => sum + (item.rooms?.length ?? 0), 0))
const totalRacks = computed(() => tree.value.reduce((sum, item) => sum + (item.rooms ?? []).reduce((roomSum, room) => roomSum + (room.racks?.length ?? 0), 0), 0))
const templateOptions = computed(() => rackTemplates.value.filter((item) => item.status === 'ACTIVE').map((template) => ({ template, version: latestTemplateVersion(template) })).filter((item): item is { template: RackTemplate; version: RackTemplateVersion } => Boolean(item.version)))

const emptyDataCenter = (): DataCenterPayload => ({ code: '', name: '', status: 'OPERATING', sortOrder: 0, version: 0, address: '', manager: '', contact: '', serviceProvider: '', remarks: '' })
const emptyRoom = (): RoomPayload => ({ code: '', name: '', status: 'OPERATING', sortOrder: 0, version: 0, floorPlanEnabled: false, floorPlanFormat: 'SVG', racksPerRow: 8, building: '', floor: '', roomNumber: '', manager: '', contact: '', purpose: '', remarks: '' })
const emptyRack = (): RackPayload => createRackPayload()

const dataCenterDialog = ref(false); const dataCenterMode = ref<'create' | 'edit'>('create'); const dcForm = ref<DataCenterPayload>(emptyDataCenter()); const dataCenterAutoCode = ref(true)
const roomDialog = ref(false); const roomMode = ref<'create' | 'edit'>('create'); const roomForm = ref<RoomPayload>(emptyRoom()); const roomAutoCode = ref(true)
const rackDialog = ref(false); const rackMode = ref<'create' | 'edit'>('create'); const rackForm = ref<RackPayload>(emptyRack()); const rackAutoCode = ref(true)

type TransferKind = 'data-center' | 'room' | 'rack'
type TransferAction = 'copy' | 'move'
type TransferSource = DataCenter | Room | Rack
const transferDialog = ref(false)
const transferKind = ref<TransferKind>('rack')
const transferAction = ref<TransferAction>('copy')
const transferSource = ref<TransferSource | null>(null)
const transferForm = ref({ targetDataCenterId: '', targetRoomId: '', code: '', name: '', autoGenerateCode: true })
const transferTitle = computed(() => `${transferAction.value === 'copy' ? '复制' : '移动'}${transferKind.value === 'data-center' ? '数据中心' : transferKind.value === 'room' ? '机房' : '机柜'}`)
const transferTip = computed(() => {
  if (transferAction.value === 'move') return transferKind.value === 'room' ? '移动后，下属机柜、设备 U 位和 PDU 关系保持不变，并同步到目标数据中心。' : '移动后，设备、U 位和 PDU 关系保持不变；机柜在大屏中的坐标会清空。'
  if (transferKind.value === 'data-center') return '复制数据中心及其下属机房、机柜结构；不会复制设备、PDU 实例和资产唯一编号。'
  if (transferKind.value === 'room') return '复制机房及其下属机柜结构；不会复制设备、PDU 实例和资产唯一编号。'
  return '只复制机柜参数和模板快照，不复制设备、PDU 实例、序列号和资产编号。'
})
const availableDataCenters = computed(() => tree.value.filter((item) => !parentDisabled(item.status) && (transferAction.value === 'copy' || item.id !== (transferSource.value as Room | null)?.dataCenterId)))
const availableRackTargets = computed(() => tree.value.map((dc) => ({ ...dc, rooms: (dc.rooms ?? []).filter((room) => !parentDisabled(room.status) && (transferAction.value === 'copy' || room.id !== (transferSource.value as Rack | null)?.roomId)) })).filter((dc) => (dc.rooms?.length ?? 0) > 0))

async function loadRackTemplates() {
  try { rackTemplates.value = (await getRackTemplates()).items ?? [] }
  catch (error) { ElMessage.error(errorMessage(error)) }
}

const latestTemplateVersion = latestRackTemplateVersion

function applySelectedTemplate(versionId?: string) {
  if (!versionId) return
  const option = templateOptions.value.find((item) => item.version.id === versionId)
  if (!option) return
  applyRackTemplateSpec(rackForm.value, option.version)
}

async function loadTree() {
  loading.value = true
  try {
    const result = await getResourceTree()
    tree.value = result.items ?? []
    if (!tree.value.some((item) => item.id === selectedDataCenterId.value)) selectedDataCenterId.value = tree.value[0]?.id ?? ''
    const nextRooms = tree.value.find((item) => item.id === selectedDataCenterId.value)?.rooms ?? []
    if (!nextRooms.some((item) => item.id === selectedRoomId.value)) selectedRoomId.value = nextRooms[0]?.id ?? ''
  } catch (error) { ElMessage.error(errorMessage(error)) } finally { loading.value = false }
}

function selectDataCenter(id: string) {
  selectedDataCenterId.value = id
  selectedRoomId.value = tree.value.find((item) => item.id === id)?.rooms?.[0]?.id ?? ''
}

function openDataCenterCreate() { dataCenterMode.value = 'create'; dcForm.value = emptyDataCenter(); dataCenterAutoCode.value = true; dataCenterDialog.value = true }
function openDataCenterEdit(item: DataCenter) { const { rooms: _rooms, createdAt: _createdAt, updatedAt: _updatedAt, ...form } = item; dataCenterMode.value = 'edit'; dcForm.value = form; dataCenterDialog.value = true }
function openRoomCreate() { if (!selectedDataCenter.value) return; roomMode.value = 'create'; roomForm.value = emptyRoom(); roomAutoCode.value = true; roomDialog.value = true }
function openRoomEdit(item: Room) { const { racks: _racks, dataCenterId: _dataCenterId, createdAt: _createdAt, updatedAt: _updatedAt, ...form } = item; roomMode.value = 'edit'; roomForm.value = form; roomDialog.value = true }
function openRackCreate() { if (!selectedRoom.value) return; rackMode.value = 'create'; rackForm.value = emptyRack(); rackAutoCode.value = true; rackDialog.value = true }
function openRackEdit(item: Rack) { const { dataCenterId: _dataCenterId, roomId: _roomId, createdAt: _createdAt, updatedAt: _updatedAt, templateId: _templateId, templateSnapshot: _templateSnapshot, ...form } = item; rackMode.value = 'edit'; rackForm.value = form; rackDialog.value = true }

async function submitDataCenter() {
  if (!validRequired(dcForm.value.code, dcForm.value.name, dataCenterMode.value === 'create' && dataCenterAutoCode.value)) return
  saving.value = true
  try {
    const saved = dataCenterMode.value === 'create' ? await createDataCenter(withAutoGeneratedCode(dcForm.value, dataCenterAutoCode.value)) : await updateDataCenter(dcForm.value.id!, dcForm.value)
    selectedDataCenterId.value = saved.id; dataCenterDialog.value = false; ElMessage.success('数据中心已保存'); await loadTree()
  } catch (error) { ElMessage.error(errorMessage(error)) } finally { saving.value = false }
}

async function submitRoom() {
  if (!selectedDataCenter.value || !validRequired(roomForm.value.code, roomForm.value.name, roomMode.value === 'create' && roomAutoCode.value)) return
  saving.value = true
  try {
    const saved = roomMode.value === 'create' ? await createRoom(selectedDataCenter.value.id, withAutoGeneratedCode(roomForm.value, roomAutoCode.value)) : await updateRoom(roomForm.value.id!, roomForm.value)
    selectedRoomId.value = saved.id; roomDialog.value = false; ElMessage.success('机房已保存'); await loadTree()
  } catch (error) { ElMessage.error(errorMessage(error)) } finally { saving.value = false }
}

async function submitRack() {
  if (!selectedRoom.value || !validRequired(rackForm.value.code, rackForm.value.name, rackMode.value === 'create' && rackAutoCode.value)) return
  saving.value = true
  try {
    await (rackMode.value === 'create' ? createRack(selectedRoom.value.id, withAutoGeneratedCode({ ...rackForm.value, overrideTemplateSpec: Boolean(rackForm.value.templateVersionId) }, rackAutoCode.value)) : updateRack(rackForm.value.id!, rackForm.value))
    rackDialog.value = false; ElMessage.success('机柜已保存'); await loadTree()
  } catch (error) { ElMessage.error(errorMessage(error)) } finally { saving.value = false }
}

async function handleDataCenterCommand(command: string, item: DataCenter) {
  if (command === 'edit') openDataCenterEdit(item)
  else if (command === 'copy') openTransfer('data-center', 'copy', item)
  else await removeDataCenter(item)
}
async function handleRoomCommand(command: string, item: Room) {
  if (command === 'edit') openRoomEdit(item)
  else if (command === 'copy' || command === 'move') openTransfer('room', command, item)
  else await removeRoom(item)
}
async function handleRackCommand(command: string, item: Rack) {
  if (command === 'copy' || command === 'move') openTransfer('rack', command, item)
  else await removeRack(item)
}

function openTransfer(kind: TransferKind, action: TransferAction, source: TransferSource) {
  transferKind.value = kind; transferAction.value = action; transferSource.value = source
  transferForm.value = {
    targetDataCenterId: kind === 'room' && action === 'copy' ? (source as Room).dataCenterId : '',
    targetRoomId: kind === 'rack' && action === 'copy' ? (source as Rack).roomId : '',
    code: action === 'move' ? source.code : '', name: action === 'copy' ? `${source.name}-副本` : '', autoGenerateCode: action === 'copy',
  }
  transferDialog.value = true
}

async function submitTransfer() {
  const source = transferSource.value
  if (!source) return
  if (transferKind.value === 'room' && !transferForm.value.targetDataCenterId) { ElMessage.warning('请选择目标数据中心'); return }
  if (transferKind.value === 'rack' && !transferForm.value.targetRoomId) { ElMessage.warning('请选择目标机房'); return }
  if (transferAction.value === 'copy' && !transferForm.value.name.trim()) { ElMessage.warning('副本名称不能为空'); return }
  if (!transferForm.value.autoGenerateCode && !transferForm.value.code.trim()) { ElMessage.warning('资源编码不能为空'); return }
  saving.value = true
  try {
    if (transferAction.value === 'copy') {
      const payload: ResourceCopyPayload = { version: source.version, code: transferForm.value.code, name: transferForm.value.name, autoGenerateCode: transferForm.value.autoGenerateCode, targetDataCenterId: transferForm.value.targetDataCenterId || undefined, targetRoomId: transferForm.value.targetRoomId || undefined }
      if (transferKind.value === 'data-center') await copyDataCenter(source.id, payload)
      else if (transferKind.value === 'room') await copyRoom(source.id, payload)
      else await copyRack(source.id, payload)
    } else {
      const payload: ResourceMovePayload = { version: source.version, code: transferForm.value.code, autoGenerateCode: transferForm.value.autoGenerateCode, targetDataCenterId: transferForm.value.targetDataCenterId || undefined, targetRoomId: transferForm.value.targetRoomId || undefined }
      if (transferKind.value === 'room') await moveRoom(source.id, payload)
      else await moveRack(source.id, payload)
    }
    transferDialog.value = false; ElMessage.success(transferAction.value === 'copy' ? '复制成功' : '移动成功'); await loadTree()
  } catch (error) { ElMessage.error(errorMessage(error)) } finally { saving.value = false }
}

async function removeDataCenter(item: DataCenter) {
  try { await ElMessageBox.confirm(`确定删除数据中心“${item.name}”吗？有下属机房时将禁止删除。`, '删除确认', { type: 'warning' }); await deleteDataCenter(item.id, item.version); ElMessage.success('已删除'); await loadTree() } catch (error) { handleCancelOrError(error) }
}
async function removeRoom(item: Room) {
  try { await ElMessageBox.confirm(`确定删除机房“${item.name}”吗？有下属机柜时将禁止删除。`, '删除确认', { type: 'warning' }); await deleteRoom(item.id, item.version); ElMessage.success('已删除'); await loadTree() } catch (error) { handleCancelOrError(error) }
}
async function removeRack(item: Rack) {
  try { await ElMessageBox.confirm(`确定删除机柜“${item.name}”吗？`, '删除确认', { type: 'warning' }); await deleteRack(item.id, item.version); ElMessage.success('已删除'); await loadTree() } catch (error) { handleCancelOrError(error) }
}

function validRequired(code: string, name: string, autoGenerateCode = false) { if (!hasRequiredCodeAndName(code, name, autoGenerateCode)) { ElMessage.warning(autoGenerateCode ? '名称不能为空' : '编码和名称不能为空'); return false } return true }
function parentDisabled(status?: string) { return status === 'DISABLED' || status === 'ARCHIVED' }
function statusLabel(status: string) { return [...resourceStatuses, ...rackStatuses].find((item) => item.value === status)?.label ?? status }
function statusType(status: string): 'success' | 'warning' | 'danger' | 'info' | 'primary' { if (status === 'OPERATING' || status === 'AVAILABLE') return 'success'; if (status === 'MAINTENANCE' || status === 'PARTIAL' || status === 'PLANNING') return 'warning'; if (status === 'DISABLED' || status === 'FULL') return 'danger'; return 'info' }
function errorMessage(error: unknown) { const value = error as { response?: { data?: { message?: string } }; message?: string }; return value.response?.data?.message ?? value.message ?? '操作失败' }
function handleCancelOrError(error: unknown) { if (error !== 'cancel' && error !== 'close') ElMessage.error(errorMessage(error)) }

onMounted(() => { void loadTree(); void loadRackTemplates() })
</script>

<style scoped>
.resource-page { max-width: 1500px; margin: 0 auto; }
.page-header { display:flex; justify-content:space-between; align-items:flex-start; margin-bottom:18px; }
.page-header h2 { margin:0 0 8px; }
.page-header p { margin:0; color:var(--color-text-muted); }
.header-actions { display:flex; gap:10px; }
.summary-row { margin-bottom:16px; }
.summary { display:flex; align-items:baseline; gap:12px; }
.summary span { color:var(--color-text-muted); }
.summary strong { font-size:28px; color:var(--color-text-primary); }
.summary small { margin-left:auto; color:var(--color-text-muted); }
.resource-columns { min-height:560px; }
.resource-card { height:100%; min-height:560px; }
.card-title { display:flex; align-items:center; justify-content:space-between; font-weight:700; }
.resource-item { display:flex; align-items:flex-start; gap:10px; padding:13px 10px; margin-bottom:8px; border:1px solid var(--color-border-soft); border-radius:8px; cursor:pointer; transition:.15s ease; }
.resource-item:hover { border-color:#93c5fd; background:#f8fbff; }
.resource-item.active { border-color:var(--color-action); background:#ecf5ff; }
.item-main { min-width:0; flex:1; }
.item-name { font-weight:600; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
.item-code, .item-meta { margin-top:4px; color:var(--color-text-muted); font-size:12px; }
.rack-card :deep(.el-card__body) { padding-top:12px; }
.rack-template-alert { margin-bottom:16px; }
.transfer-tip { margin-bottom:18px; }
.room-layout-tip { padding: 7px 0 0; color: #909399; font-size: 12px; line-height: 1.55; }
.code-field { width:100%; display:flex; align-items:center; gap:10px; }
.code-field .el-input { flex:1; }
.code-field .el-checkbox { flex:none; }
</style>

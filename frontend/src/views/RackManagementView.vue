<template>
  <div class="rack-page">
    <div class="page-header">
      <div>
        <h2>机柜管理</h2>
        <p>集中查看和维护全部机柜台账，支持跨数据中心筛选、模板建柜和 U 位入口。</p>
      </div>
      <div class="header-actions">
        <el-button :loading="loading" @click="loadData">刷新</el-button>
        <el-button :disabled="!availableRooms.length" @click="openRackImport">批量导入</el-button>
        <el-button type="primary" :disabled="!availableRooms.length" @click="openCreate">新增机柜</el-button>
      </div>
    </div>

    <el-row :gutter="16" class="summary-row">
      <el-col :xs="12" :sm="12" :md="6">
        <el-card shadow="never" class="summary-card total-card">
          <div class="summary-title">机柜总数</div>
          <div class="summary-value">{{ rackRows.length }}</div>
          <div class="summary-hint">覆盖 {{ rackDataCenterCount }} 个数据中心、{{ rackRoomCount }} 个机房</div>
        </el-card>
      </el-col>
      <el-col :xs="12" :sm="12" :md="6">
        <el-card shadow="never" class="summary-card available-card">
          <div class="summary-title">空闲机柜</div>
          <div class="summary-value">{{ availableRackCount }}</div>
          <div class="summary-hint">可安排设备上架</div>
        </el-card>
      </el-col>
      <el-col :xs="12" :sm="12" :md="6">
        <el-card shadow="never" class="summary-card power-card">
          <div class="summary-title">双路供电</div>
          <div class="summary-value">{{ dualPowerCount }}</div>
          <div class="summary-hint">配置双路电源的机柜</div>
        </el-card>
      </el-col>
      <el-col :xs="12" :sm="12" :md="6">
        <el-card shadow="never" class="summary-card warning-card">
          <div class="summary-title">需关注</div>
          <div class="summary-value">{{ attentionCount }}</div>
          <div class="summary-hint">已满、维护中或已停用</div>
        </el-card>
      </el-col>
    </el-row>

    <el-card shadow="never" class="filter-card">
      <div class="filter-bar">
        <el-select v-model="filters.dataCenterId" clearable placeholder="全部数据中心" style="width: 190px" @change="onDataCenterFilterChange">
          <el-option v-for="item in tree" :key="item.id" :label="item.name" :value="item.id" />
        </el-select>
        <el-select v-model="filters.roomId" clearable placeholder="全部机房" style="width: 190px">
          <el-option v-for="item in filterRooms" :key="item.id" :label="item.name" :value="item.id" />
        </el-select>
        <el-select v-model="filters.status" clearable placeholder="全部状态" style="width: 150px">
          <el-option v-for="item in rackStatuses" :key="item.value" :label="item.label" :value="item.value" />
        </el-select>
        <el-input v-model="filters.keyword" clearable placeholder="搜索编码、名称、位置、厂商或负责人" class="keyword-input" @keyup.enter="applySearch" />
        <el-button type="primary" @click="applySearch">查询</el-button>
        <el-button @click="resetFilters">重置</el-button>
      </div>
    </el-card>

    <el-card shadow="never" class="table-card" v-loading="loading">
      <template #header>
        <div class="table-header">
          <div>
            <strong>机柜台账</strong>
            <span>共 {{ filteredRows.length }} 条，合计 {{ filteredUHeight }}U</span>
          </div>
          <el-tag type="info" effect="plain">跨机房汇总视图</el-tag>
        </div>
      </template>

      <el-empty v-if="!filteredRows.length" description="没有符合条件的机柜" />
      <el-table v-else :data="pagedRows" stripe table-layout="fixed">
        <el-table-column label="机柜" min-width="190" fixed="left">
          <template #default="{ row }">
            <button class="rack-name-button" type="button" @click="openDetail(row)">{{ row.name }}</button>
            <div class="cell-subtitle">{{ row.code }}</div>
          </template>
        </el-table-column>
        <el-table-column label="所属位置" min-width="230">
          <template #default="{ row }">
            <div>{{ row.dataCenterName }} / {{ row.roomName }}</div>
            <div class="cell-subtitle">{{ physicalLocation(row) }}</div>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="110">
          <template #default="{ row }"><el-tag :type="statusType(row.status)">{{ statusLabel(row.status) }}</el-tag></template>
        </el-table-column>
        <el-table-column label="规格" width="145">
          <template #default="{ row }">
            <div>{{ row.uHeight }}U · {{ row.type || '标准机柜' }}</div>
            <div class="cell-subtitle">{{ row.widthMm }}×{{ row.depthMm }}×{{ row.heightMm }} mm</div>
          </template>
        </el-table-column>
        <el-table-column label="模板" min-width="150">
          <template #default="{ row }">
            <span v-if="row.templateName">{{ row.templateName }} V{{ row.templateRevision }}</span>
            <span v-else class="muted">自定义参数</span>
          </template>
        </el-table-column>
        <el-table-column label="电源 / PDU" width="135">
          <template #default="{ row }">
            <div>{{ row.dualPower ? '双路' : '单路' }}{{ row.inputCircuits > 0 ? ' · ' + row.inputCircuits + ' 路' : '供电' }}</div>
            <div class="cell-subtitle" :class="{ muted: !row.pduCount }">{{ row.pduCount > 0 ? row.pduCount + ' 个 PDU' : 'PDU 未配置' }}</div>
          </template>
        </el-table-column>
        <el-table-column prop="manager" label="负责人" width="110"><template #default="{ row }">{{ row.manager || '-' }}</template></el-table-column>
        <el-table-column label="操作" width="250" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="openDetail(row)">详情</el-button>
            <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
            <el-button link type="primary" @click="openUView(row)">U 位</el-button>
            <el-button link type="danger" @click="removeRack(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>

      <div v-if="filteredRows.length > pageSize" class="pagination">
        <el-pagination v-model:current-page="page" v-model:page-size="pageSize" layout="total, sizes, prev, pager, next" :page-sizes="[10, 20, 50, 100]" :total="filteredRows.length" />
      </div>
    </el-card>

    <el-dialog v-model="rackDialog" :title="rackMode === 'create' ? '新增机柜' : '编辑机柜'" width="900px" destroy-on-close>
      <el-form label-width="105px">
        <div v-if="rackMode === 'create'" class="batch-entry">
          <div><strong>需要一次新增多个机柜？</strong><span>使用 Excel 模板批量校验并导入，模板中已标明必填和可选字段。</span></div>
          <div class="batch-entry-actions"><el-button @click="downloadRackTemplate">下载模板</el-button><el-button type="primary" plain @click="openRackImport">批量导入机柜</el-button></div>
        </div>
        <div class="form-section-title">归属与基本信息</div>
        <el-row v-if="rackMode === 'create'" :gutter="16">
          <el-col :span="12"><el-form-item label="数据中心" required><el-select v-model="formDataCenterId" style="width:100%" @change="onFormDataCenterChange"><el-option v-for="item in creatableDataCenters" :key="item.id" :label="item.name" :value="item.id" /></el-select></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="机房" required><el-select v-model="formRoomId" style="width:100%"><el-option v-for="item in formRooms" :key="item.id" :label="item.name" :value="item.id" /></el-select></el-form-item></el-col>
        </el-row>
        <el-alert v-else :title="`所属位置：${editingLocation}`" type="info" :closable="false" class="location-alert" />
        <el-row :gutter="16">
          <el-col :span="8"><el-form-item label="编码" :required="rackMode === 'edit' || !rackAutoCode"><div class="code-field"><el-input v-model="rackForm.code" :disabled="rackMode === 'create' && rackAutoCode" :placeholder="rackMode === 'create' && rackAutoCode ? '保存时自动生成' : '请输入机柜编码'" /><el-checkbox v-if="rackMode === 'create'" v-model="rackAutoCode">自动生成</el-checkbox></div></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="名称" required><el-input v-model="rackForm.name" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="状态"><el-select v-model="rackForm.status" style="width:100%"><el-option v-for="item in rackStatuses" :key="item.value" :label="item.label" :value="item.value" /></el-select></el-form-item></el-col>
        </el-row>
        <el-form-item v-if="rackMode === 'create'" label="机柜模板"><el-select v-model="rackForm.templateVersionId" clearable placeholder="不选择则使用自定义参数" style="width:100%" @change="applySelectedTemplate"><el-option v-for="option in templateOptions" :key="option.version.id" :label="`${option.template.name} · V${option.version.revision} · ${option.version.uHeight}U`" :value="option.version.id" /></el-select></el-form-item>
        <el-alert v-else-if="rackForm.templateCode" :title="`实例模板快照：${rackForm.templateName} V${rackForm.templateRevision}`" type="info" :closable="false" class="location-alert" />

        <div class="form-section-title">规格与位置</div>
        <el-row :gutter="16">
          <el-col :span="6"><el-form-item label="U 位"><el-input-number v-model="rackForm.uHeight" :min="1" :max="100" :step="1" step-strictly controls-position="right" style="width:100%" /></el-form-item></el-col>
          <el-col :span="6"><el-form-item label="宽(mm)"><el-input-number v-model="rackForm.widthMm" :min="1" :step="10" controls-position="right" style="width:100%" /></el-form-item></el-col>
          <el-col :span="6"><el-form-item label="深(mm)"><el-input-number v-model="rackForm.depthMm" :min="1" :step="10" controls-position="right" style="width:100%" /></el-form-item></el-col>
          <el-col :span="6"><el-form-item label="高(mm)"><el-input-number v-model="rackForm.heightMm" :min="1" :step="10" controls-position="right" style="width:100%" /></el-form-item></el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="8"><el-form-item label="厂商"><el-input v-model="rackForm.manufacturer" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="型号"><el-input v-model="rackForm.modelNumber" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="承重(kg)"><el-input-number v-model="rackForm.loadCapacityKg" :min="0" controls-position="right" style="width:100%" /></el-form-item></el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="6"><el-form-item label="区域"><el-input v-model="rackForm.zone" /></el-form-item></el-col>
          <el-col :span="6"><el-form-item label="排"><el-input v-model="rackForm.row" /></el-form-item></el-col>
          <el-col :span="6"><el-form-item label="列"><el-input v-model="rackForm.column" /></el-form-item></el-col>
          <el-col :span="6"><el-form-item label="通道"><el-input v-model="rackForm.aisle" /></el-form-item></el-col>
        </el-row>

        <div class="form-section-title">电源与管理信息</div>
        <el-row :gutter="16">
          <el-col :span="6"><el-form-item label="双路电源"><el-switch v-model="rackForm.dualPower" /></el-form-item></el-col>
          <el-col :span="6"><el-form-item label="输入路数"><el-input-number v-model="rackForm.inputCircuits" :min="0" :step="1" step-strictly controls-position="right" style="width:100%" /></el-form-item></el-col>
          <el-col :span="6"><el-form-item label="PDU 数量"><el-input-number v-model="rackForm.pduCount" :min="0" :step="1" step-strictly controls-position="right" style="width:100%" /></el-form-item></el-col>
          <el-col :span="6"><el-form-item label="额定电压(V)"><el-input-number v-model="rackForm.ratedVoltage" :min="0" controls-position="right" style="width:100%" /></el-form-item></el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="8"><el-form-item label="额定电流(A)"><el-input-number v-model="rackForm.ratedCurrent" :min="0" controls-position="right" style="width:100%" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="额定功率(kW)"><el-input-number v-model="rackForm.ratedPowerKw" :min="0" :precision="2" controls-position="right" style="width:100%" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="峰值功率(kW)"><el-input-number v-model="rackForm.peakPowerKw" :min="0" :precision="2" controls-position="right" style="width:100%" /></el-form-item></el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="12"><el-form-item label="负责人"><el-input v-model="rackForm.manager" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="用途"><el-input v-model="rackForm.purpose" /></el-form-item></el-col>
        </el-row>
        <el-form-item label="备注"><el-input v-model="rackForm.remarks" type="textarea" :rows="3" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="rackDialog = false">取消</el-button><el-button type="primary" :loading="saving" @click="submitRack">保存</el-button></template>
    </el-dialog>

    <BatchImportDialog
      v-model="rackImportDialog"
      title="批量导入机柜"
      subject="机柜"
      description="请使用机柜导入模板填写数据。机柜编码可留空，系统会自动生成；手工填写时校验编码唯一性。系统还会校验数据中心、机房、模板和数值范围，只有校验通过的行才会提交。"
      template-file-name="机柜批量导入模板"
      result-file-name="机柜批量导入结果"
      :fields="rackImportFields"
      :prepare-rows="prepareRackRows"
      :submit-row="submitRackImport"
      @imported="onRackImported"
    />

    <el-drawer v-model="detailDrawer" title="机柜详情" size="520px">
      <template v-if="detailRack">
        <div class="detail-heading">
          <div><h3>{{ detailRack.name }}</h3><p>{{ detailRack.code }}</p></div>
          <el-tag :type="statusType(detailRack.status)">{{ statusLabel(detailRack.status) }}</el-tag>
        </div>
        <el-descriptions :column="2" border>
          <el-descriptions-item label="数据中心" :span="2">{{ detailRack.dataCenterName }}</el-descriptions-item>
          <el-descriptions-item label="机房" :span="2">{{ detailRack.roomName }}</el-descriptions-item>
          <el-descriptions-item label="物理位置" :span="2">{{ physicalLocation(detailRack) }}</el-descriptions-item>
          <el-descriptions-item label="模板" :span="2">{{ detailRack.templateName ? `${detailRack.templateName} V${detailRack.templateRevision}` : '自定义参数' }}</el-descriptions-item>
          <el-descriptions-item label="规格">{{ detailRack.uHeight }}U</el-descriptions-item>
          <el-descriptions-item label="类型">{{ detailRack.type || '-' }}</el-descriptions-item>
          <el-descriptions-item label="尺寸" :span="2">{{ detailRack.widthMm }} × {{ detailRack.depthMm }} × {{ detailRack.heightMm }} mm</el-descriptions-item>
          <el-descriptions-item label="厂商">{{ detailRack.manufacturer || '-' }}</el-descriptions-item>
          <el-descriptions-item label="型号">{{ detailRack.modelNumber || '-' }}</el-descriptions-item>
          <el-descriptions-item label="电源">{{ detailRack.dualPower ? '双路电源' : '单路电源' }}</el-descriptions-item>
          <el-descriptions-item label="输入路数">{{ detailRack.inputCircuits > 0 ? detailRack.inputCircuits + ' 路' : '未配置' }}</el-descriptions-item>
          <el-descriptions-item label="PDU">{{ detailRack.pduCount > 0 ? detailRack.pduCount + ' 个' : '未配置' }}</el-descriptions-item>
          <el-descriptions-item label="额定功率">{{ displayPower(detailRack.ratedPowerKw) }}</el-descriptions-item>
          <el-descriptions-item label="负责人">{{ detailRack.manager || '-' }}</el-descriptions-item>
          <el-descriptions-item label="用途">{{ detailRack.purpose || '-' }}</el-descriptions-item>
          <el-descriptions-item label="备注" :span="2">{{ detailRack.remarks || '-' }}</el-descriptions-item>
        </el-descriptions>
        <div class="drawer-actions"><el-button @click="openEdit(detailRack)">编辑机柜</el-button><el-button type="primary" @click="openUView(detailRack)">查看 U 位</el-button></div>
      </template>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useRouter } from 'vue-router'
import BatchImportDialog from '@/components/BatchImportDialog.vue'
import { createRack, deleteRack, getResourceTree, updateRack } from '@/api/resource'
import { getRackTemplates } from '@/api/rack-template'
import type { DataCenter, Rack, RackPayload, Room } from '@/types/resource'
import type { RackTemplate, RackTemplateVersion } from '@/types/rack-template'
import { createRackPayload, latestRackTemplateVersion, applyRackTemplateSpec } from '@/features/rack/rackDefaults'
import { prepareRackImportRows, rackImportFields } from '@/features/import/rackImport'
import type { RackImportPayload } from '@/features/import/rackImport'
import { downloadExcelTemplate } from '@/utils/excelImport'
import { hasRequiredCodeAndName, withAutoGeneratedCode } from '@/features/code/autoCode'
import type { ParsedExcelRow } from '@/utils/excelImport'

type TagType = 'success' | 'warning' | 'danger' | 'info' | 'primary'
interface RackRow extends Rack {
  dataCenterName: string
  roomName: string
  dataCenterStatus: string
  roomStatus: string
}

const router = useRouter()
const tree = ref<DataCenter[]>([])
const rackTemplates = ref<RackTemplate[]>([])
const loading = ref(false)
const saving = ref(false)
const rackDialog = ref(false)
const rackImportDialog = ref(false)
const detailDrawer = ref(false)
const rackMode = ref<'create' | 'edit'>('create')
const rackAutoCode = ref(true)
const detailRack = ref<RackRow | null>(null)
const editingRack = ref<RackRow | null>(null)
const formDataCenterId = ref('')
const formRoomId = ref('')
const page = ref(1)
const pageSize = ref(20)
const filters = reactive({ dataCenterId: '', roomId: '', status: '', keyword: '' })
const appliedKeyword = ref('')

const rackStatuses = [
  { label: '规划中', value: 'PLANNING' }, { label: '空闲', value: 'AVAILABLE' },
  { label: '部分使用', value: 'PARTIAL' }, { label: '已满', value: 'FULL' },
  { label: '维护中', value: 'MAINTENANCE' }, { label: '停用', value: 'DISABLED' },
]

const emptyRack = (): RackPayload => createRackPayload()
const rackForm = ref<RackPayload>(emptyRack())

const rackRows = computed<RackRow[]>(() => tree.value.flatMap((dataCenter) =>
  (dataCenter.rooms ?? []).flatMap((room) => (room.racks ?? []).map((rack) => ({
    ...rack,
    dataCenterName: dataCenter.name,
    roomName: room.name,
    dataCenterStatus: dataCenter.status,
    roomStatus: room.status,
  }))),
))
const filterRooms = computed(() => filters.dataCenterId
  ? tree.value.find((item) => item.id === filters.dataCenterId)?.rooms ?? []
  : tree.value.flatMap((item) => item.rooms ?? []))
const availableRooms = computed(() => tree.value.flatMap((dataCenter) =>
  isParentAvailable(dataCenter.status)
    ? (dataCenter.rooms ?? []).filter((room) => isParentAvailable(room.status)).map((room) => ({ dataCenter, room }))
    : [],
))
const creatableDataCenters = computed(() => tree.value.filter((item) =>
  isParentAvailable(item.status) && (item.rooms ?? []).some((room) => isParentAvailable(room.status)),
))
const formRooms = computed(() => (tree.value.find((item) => item.id === formDataCenterId.value)?.rooms ?? []).filter((room) => isParentAvailable(room.status)))
const filteredRows = computed(() => {
  const keyword = appliedKeyword.value.trim().toLowerCase()
  return rackRows.value.filter((row) => {
    if (filters.dataCenterId && row.dataCenterId !== filters.dataCenterId) return false
    if (filters.roomId && row.roomId !== filters.roomId) return false
    if (filters.status && row.status !== filters.status) return false
    if (!keyword) return true
    return [row.code, row.name, row.dataCenterName, row.roomName, row.zone, row.row, row.column, row.aisle, row.manufacturer, row.modelNumber, row.manager]
      .some((value) => String(value ?? '').toLowerCase().includes(keyword))
  })
})
const pagedRows = computed(() => filteredRows.value.slice((page.value - 1) * pageSize.value, page.value * pageSize.value))
const availableRackCount = computed(() => rackRows.value.filter((item) => item.status === 'AVAILABLE').length)
const dualPowerCount = computed(() => rackRows.value.filter((item) => item.dualPower).length)
const attentionCount = computed(() => rackRows.value.filter((item) => ['FULL', 'MAINTENANCE', 'DISABLED'].includes(item.status)).length)
const rackDataCenterCount = computed(() => new Set(rackRows.value.map((item) => item.dataCenterId)).size)
const rackRoomCount = computed(() => new Set(rackRows.value.map((item) => item.roomId)).size)
const filteredUHeight = computed(() => filteredRows.value.reduce((sum, item) => sum + item.uHeight, 0))
const editingLocation = computed(() => editingRack.value ? `${editingRack.value.dataCenterName} / ${editingRack.value.roomName}` : '-')
const templateOptions = computed(() => rackTemplates.value
  .filter((item) => item.status === 'ACTIVE')
  .map((template) => ({ template, version: latestTemplateVersion(template) }))
  .filter((item): item is { template: RackTemplate; version: RackTemplateVersion } => Boolean(item.version)))

watch([() => filters.dataCenterId, () => filters.roomId, () => filters.status, pageSize], () => { page.value = 1 })

async function loadData() {
  loading.value = true
  try {
    const [resourceResult, templateResult] = await Promise.all([getResourceTree(), getRackTemplates()])
    tree.value = resourceResult.items ?? []
    rackTemplates.value = templateResult.items ?? []
    if (filters.dataCenterId && !tree.value.some((item) => item.id === filters.dataCenterId)) filters.dataCenterId = ''
    if (filters.roomId && !filterRooms.value.some((item) => item.id === filters.roomId)) filters.roomId = ''
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    loading.value = false
  }
}

const latestTemplateVersion = latestRackTemplateVersion

function applySelectedTemplate(versionId?: string) {
  if (!versionId) return
  const option = templateOptions.value.find((item) => item.version.id === versionId)
  if (!option) return
  applyRackTemplateSpec(rackForm.value, option.version)
}

function downloadRackTemplate() {
  downloadExcelTemplate('机柜批量导入模板', '机柜', rackImportFields)
}

function openRackImport() {
  rackDialog.value = false
  rackImportDialog.value = true
}

function prepareRackRows(rows: ParsedExcelRow[]) {
  return prepareRackImportRows(rows, tree.value, rackTemplates.value, rackRows.value)
}

async function submitRackImport(value: unknown) {
  const item = value as RackImportPayload
  if (!item?.roomId || !item.payload) throw new Error('机柜所属机房或导入数据无效')
  return createRack(item.roomId, item.payload)
}

async function onRackImported() {
  await loadData()
}

function openCreate() {
  rackMode.value = 'create'
  editingRack.value = null
  rackForm.value = emptyRack()
  rackAutoCode.value = true
  const preferred = availableRooms.value.find((item) => item.dataCenter.id === filters.dataCenterId && (!filters.roomId || item.room.id === filters.roomId)) ?? availableRooms.value[0]
  formDataCenterId.value = preferred?.dataCenter.id ?? ''
  formRoomId.value = preferred?.room.id ?? ''
  rackDialog.value = true
}

function openEdit(item: RackRow) {
  detailDrawer.value = false
  rackMode.value = 'edit'
  editingRack.value = item
  const {
    dataCenterId: _dataCenterId, roomId: _roomId, createdAt: _createdAt, updatedAt: _updatedAt,
    templateId: _templateId, templateSnapshot: _templateSnapshot, dataCenterName: _dataCenterName,
    roomName: _roomName, dataCenterStatus: _dataCenterStatus, roomStatus: _roomStatus, ...form
  } = item
  rackForm.value = form
  formDataCenterId.value = item.dataCenterId
  formRoomId.value = item.roomId
  rackDialog.value = true
}

function openDetail(item: RackRow) {
  detailRack.value = item
  detailDrawer.value = true
}

function onFormDataCenterChange() {
  formRoomId.value = formRooms.value[0]?.id ?? ''
}

function onDataCenterFilterChange() {
  filters.roomId = ''
}

function applySearch() {
  appliedKeyword.value = filters.keyword
  page.value = 1
}

function resetFilters() {
  filters.dataCenterId = ''
  filters.roomId = ''
  filters.status = ''
  filters.keyword = ''
  appliedKeyword.value = ''
  page.value = 1
}

async function submitRack() {
  if (!hasRequiredCodeAndName(rackForm.value.code, rackForm.value.name, rackMode.value === 'create' && rackAutoCode.value)) {
    ElMessage.warning(rackMode.value === 'create' && rackAutoCode.value ? '名称不能为空' : '编码和名称不能为空')
    return
  }
  if (rackMode.value === 'create' && !formRoomId.value) {
    ElMessage.warning('请选择机柜所属机房')
    return
  }
  saving.value = true
  try {
    if (rackMode.value === 'create') await createRack(formRoomId.value, withAutoGeneratedCode({ ...rackForm.value, overrideTemplateSpec: Boolean(rackForm.value.templateVersionId) }, rackAutoCode.value))
    else if (editingRack.value) await updateRack(editingRack.value.id, rackForm.value)
    rackDialog.value = false
    ElMessage.success('机柜已保存')
    await loadData()
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    saving.value = false
  }
}

async function removeRack(item: RackRow) {
  try {
    await ElMessageBox.confirm(`确定删除机柜“${item.name}”吗？如果机柜内仍有设备将禁止删除。`, '删除确认', { type: 'warning' })
    await deleteRack(item.id, item.version)
    ElMessage.success('机柜已删除')
    await loadData()
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') ElMessage.error(errorMessage(error))
  }
}

async function openUView(item: RackRow) {
  await router.push({ path: '/devices', query: { tab: 'layout', rackId: item.id } })
}

function physicalLocation(item: Rack) {
  const parts = [item.zone && `${item.zone}区`, item.row && `${item.row}排`, item.column && `${item.column}列`, item.aisle && `${item.aisle}通道`].filter(Boolean)
  return parts.join(' · ') || '未设置机房内位置'
}

function isParentAvailable(status: string) {
  return status !== 'DISABLED' && status !== 'ARCHIVED'
}

function statusLabel(status: string) {
  return rackStatuses.find((item) => item.value === status)?.label ?? status
}

function statusType(status: string): TagType {
  if (status === 'AVAILABLE') return 'success'
  if (status === 'PARTIAL' || status === 'PLANNING' || status === 'MAINTENANCE') return 'warning'
  if (status === 'FULL' || status === 'DISABLED') return 'danger'
  return 'info'
}

function displayPower(value?: number) {
  return value === undefined || value === null ? '-' : `${value} kW`
}

function errorMessage(error: unknown) {
  const value = error as { response?: { data?: { message?: string } }; message?: string }
  return value.response?.data?.message ?? value.message ?? '操作失败'
}

onMounted(() => { void loadData() })
</script>

<style scoped>
.rack-page { max-width: 1560px; margin: 0 auto; }
.page-header { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 18px; }
.page-header h2 { margin: 0 0 8px; }
.page-header p { margin: 0; color: #6b7280; }
.header-actions { display: flex; gap: 10px; }
.summary-row { margin-bottom: 16px; }
.summary-card { position: relative; overflow: hidden; border-top: 3px solid #409eff; }
.summary-card::after { content: ''; position: absolute; width: 86px; height: 86px; border-radius: 50%; right: -28px; top: -36px; background: currentColor; opacity: .07; }
.total-card { color: #409eff; }
.available-card { color: #10b981; border-top-color: #10b981; }
.power-card { color: #8b5cf6; border-top-color: #8b5cf6; }
.warning-card { color: #f59e0b; border-top-color: #f59e0b; }
.summary-title { color: #6b7280; font-size: 14px; }
.summary-value { margin-top: 8px; color: #111827; font-size: 30px; line-height: 1.1; font-weight: 700; }
.summary-value small { color: #9ca3af; font-size: 14px; font-weight: 400; }
.summary-hint { margin-top: 9px; color: #9ca3af; font-size: 12px; }
.filter-card { margin-bottom: 16px; }
.filter-bar { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
.keyword-input { min-width: 260px; flex: 1; }
.table-card { min-height: 470px; }
.table-header { display: flex; align-items: center; justify-content: space-between; }
.table-header strong { margin-right: 12px; font-size: 16px; }
.table-header span { color: #9ca3af; font-size: 13px; }
.rack-name-button { padding: 0; border: 0; background: transparent; color: #2563eb; font: inherit; font-weight: 600; cursor: pointer; }
.rack-name-button:hover { text-decoration: underline; }
.cell-subtitle { margin-top: 4px; color: #9ca3af; font-size: 12px; line-height: 1.35; }
.muted { color: #9ca3af; }
.pagination { display: flex; justify-content: flex-end; margin-top: 16px; }
.form-section-title { margin: 4px 0 16px; padding-left: 10px; border-left: 3px solid #409eff; color: #374151; font-weight: 700; }
.batch-entry { display: flex; align-items: center; justify-content: space-between; gap: 16px; margin-bottom: 18px; padding: 14px 16px; border: 1px dashed #93c5fd; border-radius: 8px; background: #eff6ff; }
.batch-entry strong { display: block; color: #1d4ed8; }
.batch-entry span { display: block; margin-top: 4px; color: #64748b; font-size: 13px; }
.batch-entry-actions { display: flex; gap: 8px; flex-shrink: 0; }
.location-alert { margin-bottom: 16px; }
.detail-heading { display: flex; align-items: flex-start; justify-content: space-between; margin-bottom: 20px; }
.detail-heading h3 { margin: 0 0 6px; font-size: 20px; }
.detail-heading p { margin: 0; color: #9ca3af; }
.drawer-actions { display: flex; justify-content: flex-end; gap: 10px; margin-top: 22px; }
@media (max-width: 900px) {
  .page-header { gap: 14px; }
  .header-actions { flex-shrink: 0; }
}
.code-field { width:100%; display:flex; align-items:center; gap:10px; }
.code-field .el-input { flex:1; }
.code-field .el-checkbox { flex:none; }
</style>

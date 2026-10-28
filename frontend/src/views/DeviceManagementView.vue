<template>
  <div class="device-page">
    <div class="page-header">
      <div><h2>{{ scope === 'ROOM' ? '机房设备与 U 位管理' : scope === 'PC' ? '终端设备档案' : '设备与 U 位管理' }}</h2><p>{{ scope === 'PC' ? '登记终端基础档案；使用人变更请在 终端管理办理领用归还。' : '维护设备台账、统一 U 位占用和设备履历。' }}</p><router-link v-if="scope==='PC'" to="/terminal-assets">返回终端领用归还</router-link></div>
      <div class="actions"><el-button :loading="loading" @click="refreshAll">刷新</el-button><el-button :disabled="total >= 3000 || !activeTypes.length" :loading="deviceImportLoading" @click="openDeviceImport">批量导入</el-button><el-button :disabled="deviceLoading" :loading="deviceExportLoading" @click="openDeviceExport">导出设备信息</el-button><el-button type="primary" :disabled="total >= 3000" @click="openDevice()">新增设备</el-button></div>
    </div>
    <el-row v-if="scope!=='PC'" :gutter="16" class="summary-row">
      <el-col :span="6"><el-card shadow="never"><div class="summary"><span>设备总数</span><strong>{{ total }}</strong><small>/ 3000</small></div></el-card></el-col>
      <el-col :span="6"><el-card shadow="never"><div class="summary"><span>当前页已上架</span><strong>{{ devices.filter(i => i.currentPosition).length }}</strong></div></el-card></el-col>
      <el-col :span="6"><el-card shadow="never"><div class="summary"><span>设备类型</span><strong>{{ types.length }}</strong></div></el-card></el-col>
      <el-col :span="6"><el-card shadow="never"><div class="summary"><span>未上架设备</span><strong>{{ devices.filter(i => !i.currentPosition).length }}</strong></div></el-card></el-col>
    </el-row>

    <el-card shadow="never">
      <el-tabs v-model="tab">
        <el-tab-pane label="设备台账" name="ledger">
          <div class="toolbar">
            <el-input v-model="filters.search" clearable placeholder="名称、编码、资产号、序列号或 IP" style="width:320px" @keyup.enter="search" />
            <el-select v-model="filters.typeId" clearable placeholder="设备类型" style="width:170px" @change="search"><el-option v-for="item in shownTypes" :key="item.id" :label="item.name" :value="item.id" /></el-select>
            <el-select v-if="scope!=='PC'" v-model="filters.status" clearable placeholder="生命周期" style="width:150px" @change="search"><el-option v-for="item in lifecycleOptions" :key="item.value" :label="item.label" :value="item.value" /></el-select>
            <el-button type="primary" @click="search">查询</el-button><el-button @click="resetSearch">重置</el-button>
          </div>
          <div class="selection-bar"><span>已勾选 {{ selectedDevices.length }} 台（支持跨页）</span><el-button link :disabled="!selectedDevices.length" @click="clearDeviceSelection">清空勾选</el-button><span v-if="filtersPending" class="muted">筛选条件已修改，查询或导出时生效</span></div>
          <el-table ref="deviceTable" v-loading="deviceLoading" :data="devices" row-key="id" stripe @selection-change="onDeviceSelection">
            <el-table-column type="selection" :reserve-selection="true" width="48" fixed="left" />
            <el-table-column prop="code" label="编码" min-width="120" fixed="left" />
            <el-table-column prop="name" label="设备名称" min-width="150" show-overflow-tooltip />
            <el-table-column label="类型" min-width="110"><template #default="{ row }">{{ row.type?.name ?? '-' }}</template></el-table-column>
            <el-table-column prop="assetNumber" label="资产编号" min-width="120" />
            <el-table-column v-if="scope!=='PC'" label="状态" width="105"><template #default="{ row }"><el-tag :type="statusType(row.lifecycleStatus)">{{ statusLabel(row.lifecycleStatus) }}</el-tag></template></el-table-column>
            <el-table-column v-if="scope!=='PC'" label="高度" width="70"><template #default="{ row }">{{ row.heightU }}U</template></el-table-column>
            <el-table-column v-if="scope!=='PC'" label="当前位置" min-width="200"><template #default="{ row }"><span v-if="row.currentPosition">{{ rackName(row.currentPosition.rackId) }} · {{ row.currentPosition.startU }}-{{ row.currentPosition.endU }}U</span><span v-else class="muted">未上架</span></template></el-table-column>
            <el-table-column prop="managementIp" label="管理 IP" min-width="120" />
            <el-table-column label="操作" width="270" fixed="right"><template #default="{ row }">
              <el-button link type="primary" @click="openDevice(row)">编辑</el-button>
              <el-button v-if="scope!=='PC' && row.type?.category!=='PC' && !row.currentPosition" link type="success" :disabled="!['WAITING_RACK','OFF_RACK'].includes(row.lifecycleStatus)" @click="openPosition(row,'ASSIGN')">上架</el-button>
              <el-button v-else-if="row.currentPosition" link type="warning" @click="openPosition(row,'MOVE')">迁移</el-button>
              <el-button v-if="row.currentPosition" link type="danger" @click="offRack(row)">下架</el-button>
              <el-button v-if="scope!=='PC'" link @click="showHistory(row)">履历</el-button>
              <el-button link type="danger" :disabled="Boolean(row.currentPosition)" @click="removeDevice(row)">删除</el-button>
            </template></el-table-column>
          </el-table>
          <div class="pagination"><el-pagination v-model:current-page="page" v-model:page-size="pageSize" :total="total" :page-sizes="[10,20,50,100]" layout="total, sizes, prev, pager, next" @change="loadDevices" /></div>
        </el-tab-pane>

        <el-tab-pane label="设备类型" name="types">
          <div class="tab-action"><el-button type="primary" @click="openType()">新增设备类型</el-button></div>
          <el-table :data="shownTypes" stripe>
            <el-table-column prop="code" label="编码" width="140" /><el-table-column prop="name" label="名称" min-width="140" />
            <el-table-column label="分类" width="150"><template #default="{ row }">{{ categoryLabel(row.category) }}</template></el-table-column>
            <el-table-column label="默认高度" width="100"><template #default="{ row }">{{ row.defaultHeightU }}U</template></el-table-column>
            <el-table-column label="双路电源" width="100"><template #default="{ row }">{{ row.defaultDualPower ? '是' : '否' }}</template></el-table-column>
            <el-table-column label="状态" width="90"><template #default="{ row }"><el-tag :type="row.status === 'ACTIVE' ? 'success' : 'info'">{{ row.status === 'ACTIVE' ? '启用' : '停用' }}</el-tag></template></el-table-column>
            <el-table-column prop="description" label="说明" min-width="180" show-overflow-tooltip />
            <el-table-column label="操作" width="130"><template #default="{ row }"><el-button link type="primary" @click="openType(row)">编辑</el-button><el-button link type="danger" @click="removeType(row)">删除</el-button></template></el-table-column>
          </el-table>
        </el-tab-pane>

        <el-tab-pane v-if="scope!=='PC'" label="机柜 U 位" name="layout">
          <div class="toolbar"><el-cascader v-model="layoutPath" :options="rackOptions" :props="{ emitPath:true }" clearable filterable placeholder="选择数据中心 / 机房 / 机柜" style="width:430px" @change="loadLayout" /><el-button :disabled="!layoutRackId" :loading="layoutLoading" @click="loadLayout">刷新 U 位</el-button></div>
          <RackUView :layout="layout" />
        </el-tab-pane>


      </el-tabs>
    </el-card>

    <el-dialog v-model="deviceExportDialog" title="导出设备信息" width="min(620px, 94vw)" :close-on-click-modal="!deviceExportLoading" :show-close="!deviceExportLoading">
      <el-form label-position="top">
        <el-form-item label="导出范围"><el-radio-group v-model="exportScope" :disabled="deviceExportLoading"><el-radio value="filtered">当前筛选结果（全部分页，共 {{ total }} 台）</el-radio><el-radio value="selected" :disabled="!selectedDevices.length">仅勾选设备（{{ selectedDevices.length }} 台）</el-radio></el-radio-group></el-form-item>
        <p class="muted">{{ exportFilterSummary }}</p>
        <el-form-item label="设备分类（可多选）"><el-checkbox-group v-model="exportCategories" :disabled="deviceExportLoading"><el-checkbox v-for="item in categoryOptions" :key="item.value" :value="item.value">{{ item.label }}</el-checkbox></el-checkbox-group><el-button link :disabled="deviceExportLoading" @click="exportCategories=categoryOptions.map(item=>item.value)">选择全部分类</el-button></el-form-item>
        <el-alert title="分类会进一步限定导出范围；没有匹配设备时不生成文件。筛选查询变更后会清空勾选。" type="info" :closable="false" />
      </el-form>
      <template #footer><el-button :disabled="deviceExportLoading" @click="deviceExportDialog=false">取消</el-button><el-button type="primary" :loading="deviceExportLoading" :disabled="!exportCategories.length || (exportScope==='selected' && !selectedDevices.length)" @click="exportDevices">确认导出</el-button></template>
    </el-dialog>

    <el-dialog v-model="deviceDialog" :title="editingDevice ? '编辑设备' : '新增设备'" width="980px" destroy-on-close>
      <div v-if="!editingDevice" class="batch-entry">
        <div><strong>需要一次新增多个设备？</strong><span>下载 Excel 模板批量导入，模板中已清楚标明必填字段、可选字段和填写规则。</span></div>
        <div class="batch-entry-actions"><el-button @click="downloadDeviceTemplate">下载模板</el-button><el-button type="primary" plain :loading="deviceImportLoading" @click="openDeviceImport">批量导入设备</el-button></div>
      </div>
      <el-form :model="deviceForm" label-width="108px" class="device-editor-form">
        <DeviceFormFields
          v-model="deviceForm"
          v-model:auto-code="deviceAutoCode"
          v-model:terminal="terminalForm"
          :terminal-mode="scope==='PC'"
          :sites="terminalSites"
          :types="shownTypes"
          :mode="editingDevice ? 'edit' : 'create'"
          :loading="deviceFormLoading"
          :positioned="editingDevicePositioned"
          :device-id="editingDevice"
        />
      </el-form>
      <template #footer><el-button @click="deviceDialog=false">取消</el-button><el-button type="primary" :loading="saving" :disabled="deviceFormLoading" @click="saveDevice">保存</el-button></template>
    </el-dialog>

    <BatchImportDialog
      v-model="deviceImportDialog"
      title="批量导入设备"
      subject="设备"
      description="支持新增和更新设备。系统优先按设备编码匹配已有设备，编码为空时按序列号或资产编号匹配；匹配到已有设备则更新，未匹配到则新增并自动生成编码。导入前会校验重复标识和冲突，更新时空白可选字段默认保持原值，设备位置不会被修改。"
      template-file-name="设备批量导入模板"
      result-file-name="设备批量导入结果"
      :fields="importFields"
      :prepare-rows="prepareDeviceRows"
      :submit-row="submitDeviceImport"
      @imported="onDeviceImported"
    />

    <el-dialog v-model="typeDialog" :title="editingType ? '编辑设备类型' : '新增设备类型'" width="580px"><el-form :model="typeForm" label-width="110px">
      <el-form-item label="类型编码" required><el-input v-model="typeForm.code" /></el-form-item><el-form-item label="类型名称" required><el-input v-model="typeForm.name" /></el-form-item>
      <el-form-item label="分类"><el-select v-model="typeForm.category" style="width:100%"><el-option v-for="item in categoryOptions" :key="item.value" :label="item.label" :value="item.value" /></el-select></el-form-item>
      <el-form-item label="默认 U 高度"><el-input-number v-model="typeForm.defaultHeightU" :min="1" :max="100" /></el-form-item><el-form-item label="默认双路电源"><el-switch v-model="typeForm.defaultDualPower" /></el-form-item>
      <el-form-item label="状态"><el-radio-group v-model="typeForm.status"><el-radio value="ACTIVE">启用</el-radio><el-radio value="DISABLED">停用</el-radio></el-radio-group></el-form-item><el-form-item label="说明"><el-input v-model="typeForm.description" type="textarea" /></el-form-item>
    </el-form><template #footer><el-button @click="typeDialog=false">取消</el-button><el-button type="primary" :loading="saving" @click="saveType">保存</el-button></template></el-dialog>

    <el-dialog v-model="positionDialog" :title="positionMode === 'ASSIGN' ? '设备上架' : '设备迁移'" width="600px"><el-alert v-if="positionDevice" :title="`${positionDevice.name} · ${positionDevice.heightU}U`" type="info" :closable="false" /><el-form :model="positionForm" label-width="100px" class="position-form">
      <el-form-item label="目标机柜" required><el-cascader v-model="positionPath" :options="rackOptions" :props="{ emitPath:true }" filterable style="width:100%" @change="syncPositionRack" /></el-form-item>
      <el-form-item label="起始 U"><el-input-number v-model="positionForm.startU" :min="1" :max="targetRack?.uHeight ?? 100" /></el-form-item><el-form-item label="占用高度"><el-input-number v-model="positionForm.heightU" :min="1" :max="targetRack?.uHeight ?? 100" /> U</el-form-item>
      <el-form-item label="方向"><el-radio-group v-model="positionForm.orientation"><el-radio value="NORMAL">正常</el-radio><el-radio value="REVERSE">倒装</el-radio></el-radio-group></el-form-item><el-form-item label="原因"><el-input v-model="positionForm.reason" type="textarea" /></el-form-item>
    </el-form><template #footer><el-button @click="positionDialog=false">取消</el-button><el-button type="primary" :loading="saving" @click="submitPosition">提交</el-button></template></el-dialog>

    <el-drawer v-model="historyDrawer" :title="`${historyDevice?.name ?? ''} · 位置履历`" size="620px"><el-timeline v-if="histories.length"><el-timeline-item v-for="item in histories" :key="item.id" :timestamp="formatTime(item.createdAt)"><el-card shadow="never"><strong>{{ operationLabel(item.operation) }}</strong><p>{{ historyPosition(item.fromPosition) }} → {{ historyPosition(item.toPosition) }}</p><small>{{ item.reason || '未填写原因' }}</small></el-card></el-timeline-item></el-timeline><el-empty v-else description="暂无位置履历" /></el-drawer>
  </div>
</template>
<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox, type TableInstance } from 'element-plus'
import { useRoute } from 'vue-router'
import RackUView from '@/components/RackUView.vue'
import BatchImportDialog from '@/components/BatchImportDialog.vue'
import DeviceFormFields from '@/components/DeviceFormFields.vue'
import { getResourceTree } from '@/api/resource'
import { createTerminal, getTerminalState, listTerminalSites, type TerminalRegistration, assignDevice, createDevice, createDeviceType, decommissionDevice, deleteDevice, deleteDeviceType, getRackLayout, getDevice, listDeviceHistory, listDevices, listDeviceTypes, moveDevice, updateDevice, updateDeviceType } from '@/api/device'
import type { DataCenter } from '@/types/resource'
import type { Device, DeviceCategory, DeviceCreatePayload, DevicePayload, DeviceType, DeviceTypePayload, PositionChangePayload, PositionHistory, PositionOperation, PositionSnapshot, RackULayout } from '@/types/device'
import { deviceImportFields, prepareDeviceImportRows } from '@/features/import/deviceImport'
import { downloadExcelTemplate } from '@/utils/excelImport'
import type { ParsedExcelRow, PreparedImportRow } from '@/utils/excelImport'
import { withAutoGeneratedCode } from '@/features/code/autoCode'
import { createDevicePayload, deviceLifecycleOptions, toDeviceApiPayload, toDevicePayload, validateDevicePayload } from '@/features/device/deviceForm'
import { downloadDeviceExportExcel } from '@/utils/deviceExport'
import { collectDeviceExport } from '@/utils/deviceExportData'

const route = useRoute()
const scope = computed<'PC'|'ROOM'|undefined>(() => route.meta?.deviceScope === 'ROOM' ? 'ROOM' : route.query.scope === 'PC' ? 'PC' : undefined)
const shownTypes = computed(() => types.value.filter(item => !scope.value || (scope.value === 'PC' ? item.category === 'PC' : item.category !== 'PC')))
const tab = ref('ledger'), loading = ref(false), deviceLoading = ref(false), deviceImportLoading = ref(false), deviceExportLoading = ref(false), layoutLoading = ref(false), saving = ref(false)
const devices = ref<Device[]>([]), deviceImportExisting = ref<Device[]>([]), types = ref<DeviceType[]>([]), tree = ref<DataCenter[]>([]), total = ref(0), page = ref(1), pageSize = ref(20)
const filters = reactive({ search:'', typeId:'', status:'' })
const appliedFilters = ref({ ...filters })
const filtersPending = computed(() => JSON.stringify(filters) !== JSON.stringify(appliedFilters.value))
const deviceTable = ref<TableInstance>()
const selectedDevices = ref<Device[]>([])
const deviceExportDialog = ref(false), exportScope = ref<'filtered'|'selected'>('filtered')
const exportCategories = ref<DeviceCategory[]>([])
const exportFilterSummary = computed(() => `查询范围：${appliedFilters.value.search || '全部关键词'} / ${types.value.find(item => item.id === appliedFilters.value.typeId)?.name || '全部类型'} / ${statusLabel(appliedFilters.value.status) || '全部状态'}`)
let deviceRequest = 0
function clearDeviceSelection() { deviceTable.value?.clearSelection(); selectedDevices.value = [] }
function onDeviceSelection(rows: Device[]) { selectedDevices.value = rows }
function deviceQuery() { return { ...(scope.value ? { scope: scope.value } : {}), search: appliedFilters.value.search || undefined, typeId: appliedFilters.value.typeId || undefined, lifecycleStatus: appliedFilters.value.status || undefined } }
const deviceDialog = ref(false), deviceImportDialog = ref(false), editingDevice = ref(''), deviceForm = ref<DevicePayload>(createDevicePayload()), deviceAutoCode = ref(true)
const deviceFormLoading = ref(false), editingDevicePositioned = ref(false), editingDeviceOriginalHeightU = ref<number | undefined>()
const typeDialog = ref(false), editingType = ref(''), typeForm = ref<DeviceTypePayload>(newType())
const terminalForm = ref<TerminalRegistration>({ status:'IDLE', assignee:'', department:'', siteId:null, position:'' })
const terminalSites = ref<Array<{id:string;name:string;region:string}>>([])
const terminalEditor = computed(() => scope.value==='PC' || types.value.find(t => t.id===deviceForm.value.typeId)?.category==='PC')
watch(terminalEditor, async value => { if (value) { try { terminalSites.value=(await listTerminalSites()).items } catch(e) { fail(e) } } }, { immediate:true })
const positionDialog = ref(false), positionMode = ref<'ASSIGN'|'MOVE'>('ASSIGN'), positionDevice = ref<Device|null>(null), positionPath = ref<string[]>([]), positionForm = ref<PositionChangePayload>(newPosition())
const layoutPath = ref<string[]>([]), layout = ref<RackULayout|null>(null)
const historyDrawer = ref(false), historyDevice = ref<Device|null>(null), histories = ref<PositionHistory[]>([])

const importFields = computed(() => scope.value==='PC' ? deviceImportFields.filter(f => !['heightU','dualPowerRequired'].includes(f.key)).map(f => f.key==='lifecycleStatus' ? {...f,title:'使用状态',aliases:['生命周期状态'],defaultValue:'在库',allowedValues:['在库','维修中','已报废'],example:'在库',description:'新增支持在库、维修中、已报废；使用中请单条登记使用人和位置。更新基础档案时清空该列，状态变更走终端业务操作'} : f.key==='typeCode' ? {...f,example:'TERMINAL-LAPTOP'} : f.key==='name' ? {...f,example:'办公笔记本'} : f) : deviceImportFields)
const lifecycleOptions = deviceLifecycleOptions
const categoryOptions: Array<{value:DeviceCategory;label:string}> = [
  {value:'PC',label:'终端'},
  {value:'SERVER',label:'服务器'},{value:'NETWORK',label:'网络设备'},{value:'STORAGE',label:'存储设备'},
  {value:'SECURITY',label:'安全设备'},{value:'POWER_ENVIRONMENT',label:'配电与环境设备'},{value:'ACCESSORY',label:'机柜配件'},{value:'OTHER',label:'其他'},
]
const racks = computed(() => tree.value.flatMap(dc => (dc.rooms ?? []).flatMap(room => room.racks ?? [])))
const activeTypes = computed(() => shownTypes.value.filter(item => item.status === 'ACTIVE' || item.id === deviceForm.value.typeId))
const layoutRackId = computed(() => layoutPath.value[layoutPath.value.length - 1] ?? '')
const targetRack = computed(() => racks.value.find(item => item.id === positionForm.value.rackId))
const rackOptions = computed(() => tree.value.map(dc => ({ value:dc.id,label:dc.name,children:(dc.rooms ?? []).map(room => ({ value:room.id,label:room.name,children:(room.racks ?? []).map(rack => ({ value:rack.id,label:`${rack.name}（${rack.uHeight}U）` })) })) })))

function newType(): DeviceTypePayload { return { version:1,code:'',name:'',category:scope.value==='PC'?'PC':'SERVER',status:'ACTIVE',defaultHeightU:1,defaultDualPower:false,description:'',sortOrder:0 } }
function newPosition(): PositionChangePayload { return { version:1,rackId:'',startU:1,heightU:1,orientation:'NORMAL',reason:'' } }

async function refreshAll() { loading.value=true; try { await Promise.all([loadTree(),loadTypes(),loadDevices()]) } finally { loading.value=false } }
async function loadTree() { try { tree.value=(await getResourceTree()).items } catch(e) { fail(e) } }
async function loadTypes() { try { types.value=(await listDeviceTypes()).items } catch(e) { fail(e) } }
async function loadDevices() {
  const request = ++deviceRequest
  deviceLoading.value = true
  try {
    const result = await listDevices({ ...deviceQuery(), page: page.value, pageSize: pageSize.value })
    if (request !== deviceRequest) return false
    devices.value = result.items; total.value = result.total
    return true
  } catch (e) { if (request === deviceRequest) fail(e); return false }
  finally { if (request === deviceRequest) deviceLoading.value = false }
}
async function search() {
  if (filtersPending.value) clearDeviceSelection()
  appliedFilters.value = { ...filters }; page.value = 1
  return loadDevices()
}
function resetSearch() { filters.search='';filters.typeId='';filters.status='';void search() }
async function openDeviceExport() {
  if (!await search()) return
  if (!total.value && !selectedDevices.value.length) { ElMessage.info('当前筛选条件下没有可导出的设备'); return }
  exportScope.value = selectedDevices.value.length ? 'selected' : 'filtered'
  exportCategories.value = categoryOptions.map(item => item.value)
  deviceExportDialog.value = true
}
async function exportDevices() {
  deviceExportLoading.value = true
  const query = deviceQuery()
  const ids = exportScope.value === 'selected' ? selectedDevices.value.map(item => item.id) : undefined
  const categories = [...exportCategories.value]
  try {
    const all = await collectDeviceExport(query, ids)
    const rows = all.filter(item => categories.includes(item.type?.category ?? types.value.find(type => type.id === item.typeId)?.category ?? 'OTHER'))
    if (!rows.length) { ElMessage.info('所选范围和分类下没有可导出的设备，请调整选择'); return }
    const terminals = rows.filter(d => d.type?.category==='PC')
    const terminalStates: Record<string,TerminalRegistration> = {}
    for (let start=0; start<terminals.length; start+=5) {
      const batch=terminals.slice(start,start+5)
      const states=await Promise.all(batch.map(d => getTerminalState(d.id)))
      batch.forEach((d,index) => { terminalStates[d.id]=states[index]! })
    }
    downloadDeviceExportExcel({ devices: rows, resourceTree: tree.value, ...(terminals.length?{terminalStates}:{}) })
    ElMessage.success(`已导出 ${rows.length} 台设备`)
    deviceExportDialog.value = false
  } catch (e) { fail(e) }
  finally { deviceExportLoading.value = false }
}

function downloadDeviceTemplate(){downloadExcelTemplate('设备批量导入模板','设备',importFields.value)}
async function openDeviceImport(){
  deviceDialog.value=false
  deviceImportLoading.value=true
  try{
    const first=await listDevices({page:1,pageSize:100})
    const all=[...first.items]
    const pages=Math.ceil(first.total/100)
    for(let start=2;start<=pages;start+=5){
      const pageNumbers=Array.from({length:Math.min(5,pages-start+1)},(_,index)=>start+index)
      const results=await Promise.all(pageNumbers.map((pageNumber)=>listDevices({page:pageNumber,pageSize:100})))
      results.forEach((result)=>all.push(...result.items))
    }
    deviceImportExisting.value=all
    deviceImportDialog.value=true
  }catch(e){fail(e)}finally{deviceImportLoading.value=false}
}
function prepareDeviceRows(rows:ParsedExcelRow[]){return prepareDeviceImportRows(rows,shownTypes.value,deviceImportExisting.value,total.value)}
async function submitDeviceImport(value:unknown, row?:PreparedImportRow){
  const payload = toDeviceApiPayload(value as DevicePayload)
  if (row?.operation === 'UPDATE' && row.matchedDeviceId) return updateDevice(row.matchedDeviceId, payload)
  if (types.value.find(t => t.id===payload.typeId)?.category==='PC') {
    if (payload.lifecycleStatus==='RUNNING') throw new Error('使用中终端请单条登记使用人、部门和位置')
    const status = payload.lifecycleStatus==='SCRAPPED' ? 'RETIRED' : payload.lifecycleStatus==='MAINTENANCE' ? 'REPAIR' : 'IDLE'
    return createTerminal({...payload as DeviceCreatePayload,terminal:{status,assignee:'',department:'',siteId:null,position:''}})
  }
  return createDevice(payload as DeviceCreatePayload)
}
async function onDeviceImported(){deviceImportExisting.value=[];await loadDevices()}

async function openDevice(item?: Device) {
  editingDevice.value = item?.id ?? ''
  editingDevicePositioned.value = Boolean(item?.currentPosition)
  editingDeviceOriginalHeightU.value = item?.heightU
  deviceAutoCode.value = !item
  terminalForm.value={ status:'IDLE', assignee:'', department:'', siteId:null, position:'' }
  deviceForm.value = item ? toDevicePayload(item) : createDevicePayload()
  if (!item && scope.value === 'ROOM') {
    const type = activeTypes.value[0]
    if (type) { deviceForm.value.typeId = type.id; deviceForm.value.heightU = type.defaultHeightU; deviceForm.value.dualPowerRequired = type.defaultDualPower }
  }
  deviceDialog.value = true
  if (!item) return

  deviceFormLoading.value = true
  try {
    const latest = await getDevice(item.id)
    deviceForm.value = toDevicePayload(latest)
    if (latest.type?.category==='PC' || types.value.find(t => t.id===latest.typeId)?.category==='PC') terminalForm.value=await getTerminalState(latest.id)
    editingDevicePositioned.value = Boolean(latest.currentPosition)
    editingDeviceOriginalHeightU.value = latest.heightU
  } catch (e) {
    deviceDialog.value = false
    fail(e)
  } finally {
    deviceFormLoading.value = false
  }
}
async function saveDevice() {
  const autoCode = !editingDevice.value && deviceAutoCode.value
  const errors = validateDevicePayload(deviceForm.value, {
    terminal: terminalEditor.value,
    autoGenerateCode: autoCode,
    positioned: editingDevicePositioned.value,
    originalHeightU: editingDeviceOriginalHeightU.value,
  })
  if (errors.length) { ElMessage.warning(errors[0]); return }
  if (!editingDevice.value && terminalEditor.value && terminalForm.value.status==='IN_USE' && (!terminalForm.value.assignee.trim() || !terminalForm.value.department.trim() || !terminalForm.value.siteId)) { ElMessage.warning('使用中的终端必须填写使用人、部门和位置'); return }
  saving.value = true
  try {
    const payload = toDeviceApiPayload(deviceForm.value)
    if (editingDevice.value) await updateDevice(editingDevice.value, payload)
    else if (terminalEditor.value) await createTerminal({ ...withAutoGeneratedCode(payload, deviceAutoCode.value), terminal:{...terminalForm.value} })
    else await createDevice(withAutoGeneratedCode(payload, deviceAutoCode.value))
    ElMessage.success('设备保存成功')
    deviceDialog.value = false
    await loadDevices()
  } catch (e) { fail(e) } finally { saving.value = false }
}
async function removeDevice(item:Device){try{await ElMessageBox.confirm(`确认删除设备“${item.name}”吗？删除数据按策略保留 90 天。`,'删除设备',{type:'warning'});await deleteDevice(item.id,item.version);ElMessage.success('设备已删除');await loadDevices()}catch(e){cancelOrFail(e)}}

function openType(item?:DeviceType){editingType.value=item?.id??'';if(item){const{id:_i,createdAt:_c,updatedAt:_u,...payload}=item;typeForm.value={...payload}}else typeForm.value=newType();typeDialog.value=true}
async function saveType(){if(!typeForm.value.code.trim()||!typeForm.value.name.trim()){ElMessage.warning('请填写类型编码和名称');return}saving.value=true;try{if(editingType.value)await updateDeviceType(editingType.value,typeForm.value);else await createDeviceType(typeForm.value);ElMessage.success('设备类型保存成功');typeDialog.value=false;await loadTypes()}catch(e){fail(e)}finally{saving.value=false}}
async function removeType(item:DeviceType){try{await ElMessageBox.confirm(`确认删除设备类型“${item.name}”吗？`,'删除类型',{type:'warning'});await deleteDeviceType(item.id,item.version);ElMessage.success('设备类型已删除');await loadTypes()}catch(e){cancelOrFail(e)}}

function openPosition(item:Device,mode:'ASSIGN'|'MOVE'){positionDevice.value=item;positionMode.value=mode;const p=item.currentPosition;positionForm.value={version:item.version,rackId:p?.rackId??'',startU:p?.startU??1,heightU:item.heightU,orientation:p?.orientation??'NORMAL',reason:''};positionPath.value=p?.rackId?rackPath(p.rackId):[];positionDialog.value=true}
function syncPositionRack(value:unknown){const path=Array.isArray(value)?value as string[]:[];positionForm.value.rackId=path[path.length-1]??''}
async function submitPosition(){if(!positionDevice.value||!positionForm.value.rackId){ElMessage.warning('请选择目标机柜');return}const rack=targetRack.value;if(rack&&positionForm.value.startU+positionForm.value.heightU-1>rack.uHeight){ElMessage.warning(`目标位置超出 ${rack.uHeight}U 范围`);return}saving.value=true;try{const r=positionMode.value==='ASSIGN'?await assignDevice(positionDevice.value.id,positionForm.value):await moveDevice(positionDevice.value.id,positionForm.value);ElMessage.success(r.executed?'位置操作完成':'审批申请已提交，正式位置尚未改变');positionDialog.value=false;await loadDevices();if(layoutRackId.value===positionForm.value.rackId)await loadLayout()}catch(e){fail(e)}finally{saving.value=false}}
async function offRack(item:Device){try{const r=await ElMessageBox.prompt(`确认将“${item.name}”下架吗？`,'设备下架',{inputPlaceholder:'下架原因（可选）'});const result=await decommissionDevice(item.id,{version:item.version,reason:r.value});ElMessage.success(result.executed?'设备已下架':'下架申请已提交，审批通过前保留原 U 位');await loadDevices();if(layoutRackId.value===item.currentPosition?.rackId)await loadLayout()}catch(e){cancelOrFail(e)}}
async function showHistory(item:Device){historyDevice.value=item;historyDrawer.value=true;try{histories.value=(await listDeviceHistory(item.id)).items}catch(e){fail(e)}}

async function loadLayout(){layout.value=null;if(!layoutRackId.value)return;layoutLoading.value=true;try{layout.value=await getRackLayout(layoutRackId.value)}catch(e){fail(e)}finally{layoutLoading.value=false}}

function rackPath(id:string){for(const dc of tree.value)for(const room of dc.rooms??[])if((room.racks??[]).some(r=>r.id===id))return[dc.id,room.id,id];return[]}
function rackName(id?:string){const r=racks.value.find(item=>item.id===id);return r?`${r.name}（${r.code}）`:id??'-'}
function statusLabel(v:string){return lifecycleOptions.find(i=>i.value===v)?.label??v}
function statusType(v:string){if(v==='RUNNING')return'success';if(v==='MAINTENANCE'||v==='PENDING_REMOVAL')return'warning';if(v==='SCRAPPED')return'danger';return'info'}
function categoryLabel(v:string){return categoryOptions.find(i=>i.value===v)?.label??v}
function operationLabel(v:PositionOperation){return({ASSIGN:'上架',MOVE:'迁移',DECOMMISSION:'下架'}as Record<string,string>)[v]??v}
function historyPosition(v?:PositionSnapshot){return v?.rackId?`${rackName(v.rackId)} ${v.startU}-${v.endU}U`:'未上架'}
function formatTime(v?:string){return v?new Date(v).toLocaleString('zh-CN',{hour12:false}):'-'}
function message(e:unknown){const x=e as{response?:{data?:{message?:string}};message?:string};return x.response?.data?.message??x.message??'操作失败'}
function fail(e:unknown){ElMessage.error(message(e))} function cancelOrFail(e:unknown){if(e!=='cancel'&&e!=='close')fail(e)}
watch(scope, () => { filters.typeId=''; clearDeviceSelection(); void search() })
onMounted(async()=>{
  await refreshAll()
  const rackId=typeof route.query.rackId==='string'?route.query.rackId:''
  if(route.query.tab==='layout'&&rackId){
    const path=rackPath(rackId)
    if(path.length){tab.value='layout';layoutPath.value=path;await loadLayout()}
  }
})
</script>

<style scoped>
.selection-bar{display:flex;align-items:center;gap:12px;margin-bottom:12px;font-size:13px;color:#64748b}.device-page{max-width:1560px;margin:0 auto}.page-header{display:flex;justify-content:space-between;align-items:flex-start;margin-bottom:18px}.page-header h2{margin:0 0 8px}.page-header p{margin:0;color:#6b7280}.actions,.toolbar{display:flex;gap:10px;align-items:center}.summary-row{margin-bottom:16px}.summary{display:flex;align-items:baseline;gap:10px}.summary span{color:#6b7280}.summary strong{font-size:28px;color:#111827}.summary small{margin-left:auto;color:#9ca3af}.toolbar{margin-bottom:16px}.tab-action{display:flex;justify-content:flex-end;margin-bottom:12px}.pagination{display:flex;justify-content:flex-end;margin-top:16px}.muted{color:#9ca3af}.policy-card{min-height:420px}.policy-form,.position-form{margin-top:20px}.el-timeline p{color:#4b5563}.el-timeline small{color:#9ca3af}.batch-entry{display:flex;align-items:center;justify-content:space-between;gap:16px;margin-bottom:18px;padding:14px 16px;border:1px dashed #93c5fd;border-radius:8px;background:#eff6ff}.batch-entry strong{display:block;color:#1d4ed8}.batch-entry span{display:block;margin-top:4px;color:#64748b;font-size:13px}.batch-entry-actions{display:flex;gap:8px;flex-shrink:0}.device-editor-form{max-height:64vh;overflow-y:auto;padding-right:8px}
</style>







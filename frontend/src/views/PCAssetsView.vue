<script setup lang="ts">
import { useColumnWidths } from '@/features/table/useColumnWidths'

import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, type TableInstance } from 'element-plus'
import { useAuthStore } from '@/stores/auth'
import { ops, listOps, usageLabels, exportSheet, printAssetLabels, errorMessage } from '@/api/asset-ops'
import type { Device } from '@/types/device'
import '@/styles/asset-ops.css'
import TableSortHeader from '@/components/TableSortHeader.vue'
import { useDebouncedValue, useTableView, type TableOrder } from '@/features/table/useTableView'

type Operation = 'ISSUE' | 'RETURN' | 'TRANSFER' | 'REPAIR' | 'RESTORE' | 'RETIRE' | 'REGISTER'
interface Custody { usageStatus: string; assignee: string; department: string; siteId?: string | null; siteName?: string; position: string }
interface PCAsset { device: Device; profile: Custody & { site?: { name: string; region: string }; cpu?: string; memoryGb?: number; disk?: string; operatingSystem?: string }; warranty: { label: string; status: string } }
interface Handover { id: string; operation: Operation; deviceCode: string; serialNumber: string; before: Custody; after: Custody; condition: string; note: string; confirmedAt: string; confirmedByName: string }
interface HistoryPage { items: Handover[]; total: number }
const operationLabels: Record<Operation, string> = { ISSUE: '领用', RETURN: '归还', TRANSFER: '换人交接', REPAIR:'登记维修', RESTORE:'维修完成', RETIRE:'报废', REGISTER:'初始状态登记' }
const router = useRouter(), auth = useAuthStore()
const canWrite = computed(() => auth.user?.roles?.some(r => ['system_admin', 'asset_manager'].includes(r.code)))
const assets = ref<PCAsset[]>([]), sites = ref<{ id: string; name: string; region: string }[]>([]), selected = ref<PCAsset[]>([])
const loading = ref(false), saving = ref(false), exporting = ref(false), search = ref(''), status = ref(''), siteId = ref(''), typeId = ref('')
const current = ref<PCAsset>(), dialog = ref(false), operation = ref<Operation>('ISSUE')
const needsRecipient = computed(() => operation.value==='ISSUE' || operation.value==='TRANSFER')
const form = reactive({ assignee: '', department: '', siteId: '', position: '', condition: 'NORMAL', note: '' })
const historyDevice = ref<PCAsset>(), historyOpen = ref(false), historyLoading = ref(false), history = ref<Handover[]>([]), historyPage = ref(1), historyTotal = ref(0)
const assetTable = ref<TableInstance>()
let historyRequest = 0, loadRequest = 0
function clearSelection() { assetTable.value?.clearSelection(); selected.value = [] }
const terminalTypes = computed(() => [...new Map(assets.value.filter(a => a.device.type).map(a => [a.device.type!.id, a.device.type!])).values()])
const query = useDebouncedValue(search)
const filtered = computed(() => assets.value.filter(a => {
  const q = query.value.trim().toLowerCase()
  return (!q || [a.device.name, a.device.code, a.device.serialNumber, a.device.assetNumber, a.profile.assignee, a.profile.department].some(v => (v || '').toLowerCase().includes(q))) && (!status.value || displayStatus(a) === status.value) && (!siteId.value || a.profile.siteId === siteId.value) && (!typeId.value || a.device.typeId === typeId.value)
}))
const { page, pageSize, sortKey, sortOrder, sortedRows, total, pageRows, setSort, nextSort } = useTableView(filtered, 20, { status: row => usageLabels[displayStatus(row)] || displayStatus(row), location: row => location(row.profile), configuration: row => row.profile.cpu || row.device.modelNumber || row.device.specification || '' })
watch([page, pageSize, sortKey, sortOrder, status, siteId, typeId, () => query.value], clearSelection)
function onSortChange(event: { prop: string; order: TableOrder }) { setSort(event.prop || '', event.order) }
function sortBy(key: string) {
  const order = nextSort(key)
  setSort(key, order)
  if (order) assetTable.value?.sort(key, order)
  else assetTable.value?.clearSort()
}
function resetFilters() { query.value = ''; search.value = ''; status.value = ''; siteId.value = ''; typeId.value = ''; clearSelection() }
function displayStatus(a: PCAsset) { return a.device.lifecycleStatus === 'SCRAPPED' ? 'RETIRED' : a.profile.usageStatus }
function count(state: string) { return assets.value.filter(a => displayStatus(a) === state).length }
async function load() {
  const request = ++loadRequest
  loading.value = true
  try {
    const [rows, locations] = await Promise.all([listOps('devices'), listOps('sites')])
    if (request !== loadRequest) return
    clearSelection()
    assets.value = rows.filter(a => a.device.type?.category === 'PC'); sites.value = locations; selected.value = []
  } catch (e) { if (request === loadRequest) ElMessage.error(errorMessage(e)) }
  finally { if (request === loadRequest) loading.value = false }
}
function canOperate(a: PCAsset, action: Operation) {
  if (a.device.lifecycleStatus === 'SCRAPPED' || a.device.currentPosition) return false
  if (['ISSUE','TRANSFER'].includes(action) && a.device.lifecycleStatus === 'MAINTENANCE') return false
  if (action==='REPAIR') return a.profile.usageStatus==='IDLE'
  if (action==='RESTORE') return a.profile.usageStatus==='REPAIR'
  if (action==='RETIRE') return ['IDLE','REPAIR'].includes(a.profile.usageStatus)
  if (action==='REGISTER') return false
  return action === 'ISSUE' ? a.profile.usageStatus === 'IDLE' : a.profile.usageStatus === 'IN_USE' && !!a.profile.assignee
}
const actions: Operation[] = ['ISSUE','RETURN','TRANSFER','REPAIR','RESTORE','RETIRE']
function availableActions(a:PCAsset) { return actions.filter(action => canOperate(a,action)) }
function open(a: PCAsset, action: Operation) {
  current.value = a; operation.value = action
  Object.assign(form, { assignee: '', department: ['ISSUE','TRANSFER'].includes(action) ? a.profile.department || '' : '', siteId: a.profile.siteId || '', position: a.profile.position || '', condition: action==='REPAIR' ? 'FAULTY' : 'NORMAL', note: '' })
  dialog.value = true
}
async function confirm() {
  if (!current.value || saving.value) return
  if (!form.siteId || !form.note.trim() || (needsRecipient.value && (!form.assignee.trim() || !form.department.trim()))) {
    ElMessage.warning('请填写位置和操作说明；领用或换人交接还需填写接收人、部门'); return
  }
  saving.value = true
  try {
    await ops(`devices/${current.value.device.id}/pc-handover`, 'POST', { version: current.value.device.version, operation: operation.value, ...form })
    dialog.value = false; ElMessage.success(`${operationLabels[operation.value]}已确认并保留使用履历`); await load()
  } catch (e) { ElMessage.error(errorMessage(e)) }
  finally { saving.value = false }
}
function location(p: Custody & { site?: { name: string; region: string } }) { return [p.siteName || (p.site ? `${p.site.region} / ${p.site.name}` : sites.value.find(s => s.id === p.siteId)?.name), p.position].filter(Boolean).join(' · ') || '未登记' }
function custody(p: Custody) { return `${usageLabels[p.usageStatus] || p.usageStatus || '未登记'} · ${p.assignee || '未分配'}${p.department ? ' / ' + p.department : ''} · ${location(p)}` }
function conditionLabel(value:string) { return ({NORMAL:'正常',FAULTY:'故障'} as Record<string,string>)[value] || '未单独登记' }
function time(v: string) { return new Date(v).toLocaleString('zh-CN', { hour12: false }) }
function showHistory(a: PCAsset) { historyDevice.value = a; historyPage.value = 1; historyOpen.value = true; void loadHistory() }
async function loadHistory() {
  if (!historyDevice.value) return
  const id = historyDevice.value.device.id, request = ++historyRequest
  historyLoading.value = true; history.value = []
  try {
    const result = await ops<HistoryPage>(`devices/${id}/pc-history?page=${historyPage.value}`)
    if (request !== historyRequest) return
    history.value = result.items; historyTotal.value = result.total
  } catch (e) { if (request === historyRequest) ElMessage.error(errorMessage(e)) }
  finally { if (request === historyRequest) historyLoading.value = false }
}
async function exportHistory() {
  if (!historyDevice.value) return
  const id = historyDevice.value.device.id
  exporting.value = true
  try {
    const first = await ops<HistoryPage>(`devices/${id}/pc-history?page=1`), rows = [...first.items]
    for (let page = 2; rows.length < first.total; page++) {
      const next = await ops<HistoryPage>(`devices/${id}/pc-history?page=${page}`)
      if (next.total !== first.total || !next.items.length) throw new Error('使用履历已变化，请重新导出')
      rows.push(...next.items)
    }
    if (!rows.length) { ElMessage.info('暂无使用履历'); return }
    exportSheet('终端使用履历', rows.map(h => ({ 设备编码: h.deviceCode, SN: h.serialNumber, 操作: operationLabels[h.operation], 交接前: custody(h.before), 交接后: custody(h.after), 设备状况: conditionLabel(h.condition), 操作说明: h.note, 确认人: h.confirmedByName, 确认时间: time(h.confirmedAt) })))
  } catch (e) { ElMessage.error(errorMessage(e)) }
  finally { exporting.value = false }
}
function exportAssets() {
  const rows = selected.value.length ? selected.value : sortedRows.value
  if (!rows.length) { ElMessage.info('没有可导出的终端'); return }
  exportSheet('终端台账', rows.map(a => ({ 设备编码: a.device.code, 名称: a.device.name, 终端类型: a.device.type?.name, 资产编号: a.device.assetNumber, SN: a.device.serialNumber, 型号: a.device.modelNumber, 状态: usageLabels[displayStatus(a)], 使用人: a.profile.assignee, 部门: a.profile.department, 位置: location(a.profile), CPU: a.profile.cpu, '内存GB': a.profile.memoryGb, 硬盘: a.profile.disk, 操作系统: a.profile.operatingSystem, 保修: a.warranty.label })))
}
async function labels() { try { await printAssetLabels('device', selected.value.map(a => a.device)) } catch (e) { ElMessage.error(errorMessage(e)) } }
onMounted(load)
const { columnWidths, resizeColumn } = useColumnWidths({"device.name": 205, "device.type.name": 130, "status": 110, "profile.assignee": 160, "location": 200, "configuration": 220})
</script>

<template>
  <div class="asset-ops pc-assets">
    <div class="ops-heading"><div><span class="ops-eyebrow">TERMINAL ASSET MANAGEMENT</span><h1>终端管理</h1><p>统一管理显示器、主机、打印机、笔记本、鼠标、键盘等，管理员确认领用、归还和交接。</p></div><div><el-button :loading="loading" @click="load">刷新</el-button><el-button v-if="canWrite" type="primary" @click="router.push('/devices?scope=PC')">登记 / 编辑终端档案</el-button></div></div>
    <div class="ops-metrics"><div><span>终端总数</span><b>{{ assets.length }}</b></div><div><span>在库待领用</span><b>{{ count('IDLE') }}</b></div><div><span>使用中</span><b>{{ count('IN_USE') }}</b></div><div><span>维修中</span><b>{{ count('REPAIR') }}</b></div></div>
    <el-card shadow="never" v-loading="loading">
      <div class="ops-toolbar"><el-input v-model="search" clearable placeholder="名称 / 编码 / SN / 使用人 / 部门" @input="clearSelection"/><el-select v-model="typeId" clearable placeholder="终端类型" @change="clearSelection"><el-option v-for="type in terminalTypes" :key="type.id" :label="type.name" :value="type.id"/></el-select><el-select v-model="status" clearable placeholder="使用状态" @change="clearSelection"><el-option v-for="(label,key) in usageLabels" :key="key" :label="label" :value="key"/></el-select><el-select v-model="siteId" clearable placeholder="所在位置" @change="clearSelection"><el-option v-for="site in sites" :key="site.id" :label="site.region+' / '+site.name" :value="site.id"/></el-select></div>
      <div class="ops-toolbar between"><span class="ops-muted">共 {{ filtered.length }} 件；已选 {{ selected.length }} 件。故障归还自动进入维修中。</span><div><el-button :disabled="!selected.length || search !== query" @click="labels">打印二维码</el-button><el-button :disabled="search !== query" @click="exportAssets">{{ selected.length ? '导出勾选终端' : '导出筛选终端' }}</el-button><el-button @click="router.push('/asset-operations?category=PC')">配置与保修</el-button></div></div>
      <el-table ref="assetTable" :data="pageRows" row-key="device.id" @sort-change="onSortChange" @selection-change="selected=$event">
        <el-table-column type="selection" width="45"/><el-table-column label="终端 / SN" prop="device.name" sortable="custom" :width="columnWidths['device.name']"><template #header><TableSortHeader label="终端 / SN" :width="columnWidths['device.name']" @resize="resizeColumn('device.name', $event)" @sort="sortBy('device.name')"/></template><template #default="{row}"><b>{{ row.device.name }}</b><small class="ops-block ops-mono">{{ row.device.code }} · {{ row.device.serialNumber || '无 SN 时使用设备编码' }}</small></template></el-table-column>
        <el-table-column prop="device.type.name" label="终端类型" sortable="custom" :width="columnWidths['device.type.name']"><template #header><TableSortHeader label="终端类型" :width="columnWidths['device.type.name']" @resize="resizeColumn('device.type.name', $event)" @sort="sortBy('device.type.name')"/></template></el-table-column><el-table-column label="使用状态" prop="status" sortable="custom" :width="columnWidths['status']"><template #header><TableSortHeader label="使用状态" :width="columnWidths['status']" @resize="resizeColumn('status', $event)" @sort="sortBy('status')"/></template><template #default="{row}"><el-tag :type="displayStatus(row)==='IN_USE'?'success':displayStatus(row)==='REPAIR'?'warning':'info'">{{ usageLabels[displayStatus(row)] || '待核实' }}</el-tag></template></el-table-column>
        <el-table-column label="使用人 / 部门" prop="profile.assignee" sortable="custom" :width="columnWidths['profile.assignee']"><template #header><TableSortHeader label="使用人 / 部门" :width="columnWidths['profile.assignee']" @resize="resizeColumn('profile.assignee', $event)" @sort="sortBy('profile.assignee')"/></template><template #default="{row}">{{ row.profile.assignee || '未分配' }}<small class="ops-block ops-muted">{{ row.profile.department }}</small></template></el-table-column>
        <el-table-column label="所在位置 / 工位" prop="location" sortable="custom" :width="columnWidths['location']"><template #header><TableSortHeader label="所在位置 / 工位" :width="columnWidths['location']" @resize="resizeColumn('location', $event)" @sort="sortBy('location')"/></template><template #default="{row}">{{ location(row.profile) }}</template></el-table-column>
        <el-table-column label="配置 / 保修" prop="configuration" sortable="custom" :width="columnWidths['configuration']"><template #header><TableSortHeader label="配置 / 保修" :width="columnWidths['configuration']" @resize="resizeColumn('configuration', $event)" @sort="sortBy('configuration')"/></template><template #default="{row}">{{ row.profile.cpu || row.device.modelNumber || row.device.specification || '—' }}<small class="ops-block ops-muted"><template v-if="row.profile.memoryGb">{{ row.profile.memoryGb }}GB · </template>{{ row.profile.disk ? row.profile.disk + ' · ' : '' }}{{ row.warranty.label }}</small></template></el-table-column>
        <el-table-column label="操作" width="340" fixed="right"><template #default="{row}"><template v-if="canWrite"><el-button v-for="action in availableActions(row)" :key="action" link :type="action==='RETIRE'?'danger':['RETURN','REPAIR'].includes(action)?'warning':'primary'" @click="open(row,action)">{{ operationLabels[action] }}</el-button></template><el-button link @click="showHistory(row)">使用履历</el-button></template></el-table-column>
        <template #empty><el-empty :description="assets.length ? '没有匹配的终端，请调整或清除筛选' : '暂无终端，请先登记终端档案'"><el-button v-if="assets.length" @click="resetFilters">清除筛选</el-button></el-empty></template>
      </el-table>
      <el-pagination v-model:current-page="page" v-model:page-size="pageSize" :page-sizes="[20,50,100]" :total="total" layout="total, sizes, prev, pager, next" aria-label="终端主台账分页"/>
    </el-card>
    <el-dialog v-model="dialog" :title="'确认终端'+operationLabels[operation]" width="min(620px,95vw)" :close-on-click-modal="!saving" :close-on-press-escape="!saving" :show-close="!saving">
      <el-alert :title="`${current?.device.name} · SN：${current?.device.serialNumber || '未登记'}`" type="info" :closable="false"/><p>当前使用人：{{ current?.profile.assignee || '未分配' }}。确认后立即更新台账并保留使用履历。</p>
      <el-form label-position="top"><template v-if="needsRecipient"><el-form-item label="接收人" required><el-input v-model="form.assignee" maxlength="120"/></el-form-item><el-form-item label="接收部门" required><el-input v-model="form.department" maxlength="120"/></el-form-item></template><el-form-item :label="needsRecipient?'使用位置':'存放位置'" required><el-select v-model="form.siteId" filterable><el-option v-for="s in sites" :key="s.id" :label="s.region+' / '+s.name" :value="s.id"/></el-select></el-form-item><el-form-item label="具体位置 / 工位"><el-input v-model="form.position" maxlength="160"/></el-form-item><el-form-item label="设备状况" required><el-radio-group v-model="form.condition"><el-radio value="NORMAL" :disabled="operation==='REPAIR'">正常</el-radio><el-radio v-if="['RETURN','REPAIR','RETIRE'].includes(operation)" value="FAULTY">故障</el-radio></el-radio-group></el-form-item><el-form-item label="操作说明" required><el-input v-model="form.note" type="textarea" :rows="3" placeholder="例如：新员工领用 / 离职归还 / 更换使用人；归还时登记配件、设备状况"/></el-form-item></el-form>
      <template #footer><el-button :disabled="saving" @click="dialog=false">取消</el-button><el-button type="primary" :loading="saving" @click="confirm">确认{{ operationLabels[operation] }}</el-button></template>
    </el-dialog>
    <el-drawer v-model="historyOpen" :title="`${historyDevice?.device.name || ''} · 使用履历`" size="min(760px,95vw)" @closed="historyRequest++">
      <el-button :loading="exporting" @click="exportHistory">导出此终端全部履历</el-button><p class="ops-muted">登记、领用、归还、维修、报废统一留痕；不补造此前的业务记录。</p>
      <div v-loading="historyLoading"><el-timeline v-if="history.length"><el-timeline-item v-for="h in history" :key="h.id" :timestamp="time(h.confirmedAt)"><el-card shadow="never"><strong>{{ operationLabels[h.operation] }} · {{ h.confirmedByName || '管理员' }}</strong><p>交接前：{{ custody(h.before) }}</p><p>交接后：{{ custody(h.after) }}</p><p>{{ conditionLabel(h.condition) }} · {{ h.note }}</p></el-card></el-timeline-item></el-timeline><el-empty v-else description="暂无使用履历"/></div>
      <el-pagination v-model:current-page="historyPage" :total="historyTotal" :page-size="50" layout="total, prev, pager, next" @change="loadHistory"/>
    </el-drawer>
  </div>
</template>

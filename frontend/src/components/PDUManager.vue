<template>  <section class="pdu-manager">    <div class="pdu-manager-heading">      <div><small>PDU & POWER CONNECTIONS</small><h3>PDU 与供电连接</h3></div>      <div class="pdu-heading-actions"><el-button size="small" :loading="loading" @click="load">刷新</el-button><el-button size="small" type="primary" @click="openPDUCreate">新增 PDU</el-button></div>    </div>    <el-alert v-if="error" type="warning" :closable="false" show-icon>{{ error }}</el-alert>    <div v-if="!loading && pdus.length === 0" class="pdu-empty">当前机柜尚未配置 PDU，PDU 参数为可选项。</div>    <div v-for="pdu in pdus" :key="pdu.id" class="pdu-card">      <div class="pdu-card-head">        <div><strong>{{ pdu.name }}</strong><span>{{ pdu.code }}</span></div>        <div class="pdu-card-actions"><el-tag size="small" :type="pdu.status === 'ACTIVE' ? 'success' : 'info'">{{ pdu.status === 'ACTIVE' ? '启用' : '停用' }}</el-tag><el-button link type="primary" @click="openPDUEdit(pdu)">编辑</el-button><el-button link type="danger" @click="removePDU(pdu)">删除</el-button></div>      </div>      <div class="pdu-meta"><span>厂商/型号：{{ [pdu.manufacturer, pdu.modelNumber].filter(Boolean).join(' / ') || '-' }}</span><span>额定功率：{{ pdu.ratedPowerW ? `${pdu.ratedPowerW} W` : '未填写' }}</span><span>输入电流：{{ pdu.ratedCurrentA ? `${pdu.ratedCurrentA} A` : '未填写' }}</span></div>      <div class="socket-toolbar"><span>插座明细（{{ (pdu.sockets || []).length }} 个）</span><el-button link type="primary" @click="openSocketCreate(pdu)">新增插座</el-button></div>      <PDUSocketTable :sockets="pdu.sockets||[]" :connection-for="socketConnection" :group-label="pdu.name+'（'+pdu.code+'）'"><template #actions="{socket}"><el-button link type="primary" @click="openSocketEdit(pdu, socket)">编辑</el-button><el-button v-if="socketConnection(socket)" link type="warning" @click="disconnect(socketConnection(socket)!)">断开</el-button><el-button v-else link type="success" :disabled="socket.status === 'DISABLED'" @click="openConnect(socket)">连接设备</el-button><el-button link type="danger" @click="removeSocket(socket)">删除</el-button></template></PDUSocketTable>    </div>    <el-dialog v-model="pduDialog" :title="pduMode === 'create' ? '新增 PDU' : '编辑 PDU'" width="560px" append-to-body destroy-on-close>      <el-form :model="pduForm" label-width="100px"><el-form-item label="自动生成编码"><el-switch v-model="pduForm.autoGenerateCode" /></el-form-item><el-form-item label="PDU 编码"><el-input v-model="pduForm.code" :disabled="pduMode === 'create' && pduForm.autoGenerateCode" placeholder="可留空自动生成" /></el-form-item><el-form-item label="名称" required><el-input v-model="pduForm.name" /></el-form-item><el-row :gutter="12"><el-col :span="12"><el-form-item label="厂商"><el-input v-model="pduForm.manufacturer" /></el-form-item></el-col><el-col :span="12"><el-form-item label="型号"><el-input v-model="pduForm.modelNumber" /></el-form-item></el-col></el-row><el-row :gutter="12"><el-col :span="8"><el-form-item label="输入电压"><el-input-number v-model="pduForm.inputVoltage" :min="0" controls-position="right" /></el-form-item></el-col><el-col :span="8"><el-form-item label="额定功率"><el-input-number v-model="pduForm.ratedPowerW" :min="0" controls-position="right" /></el-form-item></el-col><el-col :span="8"><el-form-item label="额定电流"><el-input-number v-model="pduForm.ratedCurrentA" :min="0" controls-position="right" /></el-form-item></el-col></el-row><el-form-item label="状态"><el-radio-group v-model="pduForm.status"><el-radio label="ACTIVE">启用</el-radio><el-radio label="DISABLED">停用</el-radio></el-radio-group></el-form-item><el-form-item label="备注"><el-input v-model="pduForm.remarks" type="textarea" /></el-form-item></el-form>      <template #footer><el-button @click="pduDialog = false">取消</el-button><el-button type="primary" :loading="saving" @click="savePDU">保存</el-button></template>    </el-dialog>    <el-dialog v-model="socketDialog" :title="socketMode === 'create' ? '新增插座' : '编辑插座'" width="460px" append-to-body destroy-on-close>      <el-form :model="socketForm" label-width="90px"><el-form-item label="插座编号" required><el-input-number v-model="socketForm.socketNo" :min="1" controls-position="right" /></el-form-item><el-form-item label="插座制式" required><el-select v-model="socketForm.standard"><el-option label="国标" value="CN" /><el-option label="欧标" value="EU" /></el-select></el-form-item><el-form-item label="额定电流" required><el-radio-group v-model="socketForm.amperageA"><el-radio :label="10">10A</el-radio><el-radio :label="16">16A</el-radio></el-radio-group></el-form-item><el-form-item label="标签"><el-input v-model="socketForm.label" /></el-form-item><el-form-item label="状态"><el-radio-group v-model="socketForm.status"><el-radio label="AVAILABLE">可用</el-radio><el-radio label="DISABLED">停用</el-radio></el-radio-group></el-form-item></el-form>      <template #footer><el-button @click="socketDialog = false">取消</el-button><el-button type="primary" :loading="saving" @click="saveSocket">保存</el-button></template>    </el-dialog>    <el-dialog v-model="connectDialog" title="连接设备到插座" width="500px" append-to-body destroy-on-close>      <el-form :model="connectionForm" label-width="100px"><el-form-item label="设备" required><el-select v-model="connectionForm.deviceId" filterable placeholder="请选择已上架设备" style="width:100%"><el-option v-for="device in devices" :key="device.id" :label="`${device.name}（${device.code}） · ${positionText(device)}`" :value="device.id" /></el-select></el-form-item><el-form-item label="供电角色"><el-radio-group v-model="connectionForm.redundancyRole"><el-radio label="PRIMARY">主路</el-radio><el-radio label="REDUNDANT">冗余路</el-radio></el-radio-group></el-form-item><el-form-item label="功耗 W"><el-input-number v-model="connectionForm.powerW" :min="0" controls-position="right" /></el-form-item><el-form-item label="回路"><el-input v-model="connectionForm.circuit" placeholder="例如 A 路" /></el-form-item></el-form>      <el-alert type="info" :closable="false" show-icon>只能选择当前机柜中已经上架的设备，一个插座只能建立一个有效连接。</el-alert>      <template #footer><el-button @click="connectDialog = false">取消</el-button><el-button type="primary" :loading="saving" @click="saveConnection">建立连接</el-button></template>    </el-dialog>  </section></template><script setup lang="ts">
import PDUSocketTable from './PDUSocketTable.vue'

import { computed, onMounted, reactive, ref, watch, onBeforeUnmount } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { createPDU, createPDUSocket, connectPDUSocket, deletePDU, deletePDUSocket, disconnectPDUConnection, listPDUConnections, listPDUSockets, listPDUs, updatePDU, updatePDUSocket } from '@/api/pdu'
import type { PDU, PDUConnection, PDUSocket, PDUCreatePayload, PDUSocketPayload } from '@/types/pdu'
import type { RackDevicePosition, Device } from '@/types/device'

const props = defineProps<{ rackId: string; positions?: RackDevicePosition[] }>()
const pdus = ref<PDU[]>([])
const connections = ref<PDUConnection[]>([])
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const pduDialog = ref(false)
const socketDialog = ref(false)
const connectDialog = ref(false)
const pduMode = ref<'create' | 'edit'>('create')
const socketMode = ref<'create' | 'edit'>('create')
const selectedPDU = ref<PDU | null>(null)
const selectedSocket = ref<PDUSocket | null>(null)
const pduForm = reactive<any>({ id: '', version: 1, code: '', name: '', manufacturer: '', modelNumber: '', inputVoltage: undefined, ratedPowerW: undefined, ratedCurrentA: undefined, status: 'ACTIVE', remarks: '', autoGenerateCode: true })
const socketForm = reactive<any>({ id: '', version: 1, pduId: '', socketNo: 1, standard: 'CN', amperageA: 10, label: '', status: 'AVAILABLE' })
const connectionForm = reactive<any>({ deviceId: '', powerW: undefined, circuit: '', redundancyRole: 'PRIMARY' })
const devices = computed(() => (props.positions || []).map((position) => position.device).filter((device): device is Device => Boolean(device)))

function socketConnection(socket: PDUSocket) { return socket.connection || connections.value.find((connection) => connection.socketId === socket.id) }
function positionText(device: Device) { const pos = device.currentPosition; return pos ? `${pos.startU}-${pos.endU}U` : '已上架' }
function standardLabel(value: string) { return value === 'CN' ? '国标' : '欧标' }
function socketStatusLabel(value: string) { return ({ AVAILABLE: '可用', CONNECTED: '已连接', DISABLED: '停用' } as Record<string, string>)[value] || value }
function socketTagType(value: string) { return value === 'CONNECTED' ? 'success' : value === 'DISABLED' ? 'info' : 'warning' }

let loadRequest=0
onBeforeUnmount(()=>{loadRequest++})
async function load() {
 const request=++loadRequest,rack=props.rackId
 pdus.value=[];connections.value=[]
 if(!rack){loading.value=false;return}
 loading.value=true;error.value=''
 try {
  const [items,nextConnections]=await Promise.all([listPDUs(rack),listPDUConnections(rack)])
  const nextPDUs=await Promise.all(items.map(async pdu=>({...pdu,sockets:await listPDUSockets(pdu.id)})))
  if(request!==loadRequest||rack!==props.rackId)return
  pdus.value=nextPDUs;connections.value=nextConnections
 }catch{if(request===loadRequest)error.value='PDU 信息加载失败，请稍后重试'}
 finally{if(request===loadRequest)loading.value=false}
}

function openPDUCreate() {
  pduMode.value = 'create'
  Object.assign(pduForm, { id: '', version: 1, code: '', name: '', manufacturer: '', modelNumber: '', inputVoltage: undefined, ratedPowerW: undefined, ratedCurrentA: undefined, status: 'ACTIVE', remarks: '', autoGenerateCode: true })
  pduDialog.value = true
}
function openPDUEdit(pdu: PDU) { pduMode.value = 'edit'; selectedPDU.value = pdu; Object.assign(pduForm, { ...pdu, autoGenerateCode: false }); pduDialog.value = true }
async function savePDU() {
  if (!pduForm.name?.trim()) { ElMessage.warning('请输入 PDU 名称'); return }
  saving.value = true
  try {
    if (pduMode.value === 'create') {
      const data: PDUCreatePayload = { code: pduForm.code, name: pduForm.name, manufacturer: pduForm.manufacturer, modelNumber: pduForm.modelNumber, serialNumber: pduForm.serialNumber, inputVoltage: pduForm.inputVoltage, ratedPowerW: pduForm.ratedPowerW, ratedCurrentA: pduForm.ratedCurrentA, status: pduForm.status, remarks: pduForm.remarks, autoGenerateCode: pduForm.autoGenerateCode }
      await createPDU(props.rackId, data)
    } else if (selectedPDU.value) await updatePDU(selectedPDU.value.id, { ...pduForm, rackId: props.rackId })
    pduDialog.value = false
    ElMessage.success('PDU 已保存')
    await load()
  } catch { ElMessage.error('PDU 保存失败，请检查编码和版本后重试') } finally { saving.value = false }
}
async function removePDU(pdu: PDU) {
  try { await ElMessageBox.confirm(`确定删除 PDU“${pdu.name}”吗？请先清理插座和连接。`, '删除确认', { type: 'warning' }); await deletePDU(pdu.id, pdu.version); ElMessage.success('PDU 已删除'); await load() } catch (e) { if (e !== 'cancel' && e !== 'close') ElMessage.error('PDU 删除失败，可能仍存在插座或连接') }
}
function openSocketCreate(pdu: PDU) { socketMode.value = 'create'; selectedPDU.value = pdu; Object.assign(socketForm, { id: '', version: 1, pduId: pdu.id, socketNo: (pdu.sockets?.length || 0) + 1, standard: 'CN', amperageA: 10, label: '', status: 'AVAILABLE' }); socketDialog.value = true }
function openSocketEdit(pdu: PDU, socket: PDUSocket) { socketMode.value = 'edit'; selectedPDU.value = pdu; selectedSocket.value = socket; Object.assign(socketForm, socket); socketDialog.value = true }
async function saveSocket() {
  saving.value = true
  try {
    const data: PDUSocketPayload = { socketNo: socketForm.socketNo, standard: socketForm.standard, amperageA: socketForm.amperageA, label: socketForm.label, status: socketForm.status }
    if (socketMode.value === 'create') await createPDUSocket(selectedPDU.value!.id, data)
    else await updatePDUSocket(selectedSocket.value!.id, { ...data, id: selectedSocket.value!.id, pduId: selectedPDU.value!.id, version: selectedSocket.value!.version } as PDUSocket)
    socketDialog.value = false; ElMessage.success('插座已保存'); await load()
  } catch { ElMessage.error('插座保存失败，请检查编号、制式和电流参数') } finally { saving.value = false }
}
async function removeSocket(socket: PDUSocket) {
  try { await ElMessageBox.confirm(`确定删除 ${socket.socketNo}# 插座吗？`, '删除确认', { type: 'warning' }); await deletePDUSocket(socket.id, socket.version); ElMessage.success('插座已删除'); await load() } catch (e) { if (e !== 'cancel' && e !== 'close') ElMessage.error('插座删除失败，已连接插座不能直接删除') }
}
function openConnect(socket: PDUSocket) { selectedSocket.value = socket; Object.assign(connectionForm, { version:socket.version, deviceId: '', powerW: undefined, circuit: '', redundancyRole: 'PRIMARY' }); connectDialog.value = true }
async function saveConnection() { if (!connectionForm.deviceId) { ElMessage.warning('请选择已上架设备'); return }; saving.value = true; try { await connectPDUSocket(selectedSocket.value!.id, connectionForm); connectDialog.value = false; ElMessage.success('供电连接已建立'); await load() } catch { ElMessage.error('供电连接失败，请确认设备已上架且未重复连接') } finally { saving.value = false } }
async function disconnect(connection: PDUConnection) { try { await ElMessageBox.confirm('确定断开该设备的供电连接吗？', '断开确认', { type: 'warning' }); await disconnectPDUConnection(connection.id, connection.version); ElMessage.success('供电连接已断开'); await load() } catch (e) { if (e !== 'cancel' && e !== 'close') ElMessage.error('断开供电连接失败') } }
watch(() => props.rackId, () => {pduDialog.value=false;socketDialog.value=false;connectDialog.value=false;selectedPDU.value=null;selectedSocket.value=null;void load()})
onMounted(load)
</script>
<style scoped>.pdu-manager { margin-top: 14px; padding: 16px 18px; border: 1px solid var(--el-border-color); border-radius: 10px; background: var(--color-surface); box-shadow: 0 5px 18px rgba(35,72,96,.05); }.pdu-manager-heading, .pdu-card-head, .socket-toolbar { display:flex; align-items:center; justify-content:space-between; gap:12px; }.pdu-manager-heading small { color:var(--el-text-color-secondary); font-size:10px; letter-spacing:1.2px; }.pdu-manager-heading h3 { margin:3px 0 0; color:var(--el-text-color-primary); font-size:16px; }.pdu-heading-actions, .pdu-card-actions { display:flex; align-items:center; gap:4px; }.pdu-empty { margin-top:10px; padding:12px; color:var(--el-text-color-secondary); background:var(--el-fill-color-light); border-radius:6px; font-size:12px; }.pdu-card { margin-top:12px; padding:12px; border:1px solid var(--el-border-color-light); border-radius:8px; background:var(--color-surface); }.pdu-card-head strong { display:block; color:var(--el-text-color-primary); font-size:13px; }.pdu-card-head span { display:block; margin-top:3px; color:var(--el-text-color-secondary); font-size:11px; }.pdu-meta { display:flex; flex-wrap:wrap; gap:12px; margin:10px 0; color:var(--el-text-color-secondary); font-size:11px; }.socket-toolbar { margin:8px 0; color:var(--el-text-color-regular); font-size:11px; font-weight:700; }@media(max-width:600px){.pdu-manager{padding:12px;min-width:0}.pdu-manager-heading,.pdu-card-head{flex-wrap:wrap}.pdu-heading-actions,.pdu-card-actions{flex-wrap:wrap}.pdu-card{min-width:0}}</style>
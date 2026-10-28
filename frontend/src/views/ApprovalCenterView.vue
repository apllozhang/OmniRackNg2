<script setup lang="ts">
import {onMounted,ref} from 'vue'
import {ElMessage,ElMessageBox} from 'element-plus'
import {listAdminApprovals,approveAdminOperation,rejectAdminOperation} from '@/api/admin'
import {getResourceTree} from '@/api/resource'
import {errorMessage} from '@/api/asset-ops'
import type {ApprovalRecord} from '@/types/device'
import '@/styles/asset-ops.css'
const racks=ref<Record<string,string>>({})
const rows=ref<ApprovalRecord[]>([]),status=ref('PENDING'),loading=ref(false),deciding=ref('')
const operationLabels:Record<string,string>={ASSIGN:'上架',MOVE:'迁移',DECOMMISSION:'下架'}
const statusLabels:Record<string,string>={PENDING:'待审批',APPROVED:'已批准',REJECTED:'已驳回',CANCELLED:'已取消'}
async function load(){loading.value=true;try{rows.value=(await listAdminApprovals(status.value||undefined)).items}catch(e){ElMessage.error(errorMessage(e))}finally{loading.value=false}}
function position(row:ApprovalRecord){const p=row.operation==='DECOMMISSION'?row.sourcePosition:null;const rack=p?.rackId||row.targetRackId;const start=p?.startU||row.targetStartU;const height=p?.heightU||row.targetHeightU;return (racks.value[rack]||rack)+' · '+start+'-'+(start+height-1)+'U'}
async function decide(row:ApprovalRecord,approve:boolean){
  if(deciding.value)return
  let comment:string
  try{const r=await ElMessageBox.prompt(approve?(row.operation==='DECOMMISSION'?'批准后将释放原 U 位；设备或原位置变化会阻止执行。':'批准时重新校验设备版本和目标 U 位。'):'确认驳回该申请？设备位置保持不变。',approve?'批准申请':'驳回申请',{inputPlaceholder:'审批意见（可选）'});comment=r.value}catch{return}
  deciding.value=row.id
  try{if(approve)await approveAdminOperation(row.id,row.version,comment);else await rejectAdminOperation(row.id,row.version,comment);ElMessage.success(approve?'审批已通过并执行':'申请已驳回');await load()}catch(e){ElMessage.error(errorMessage(e))}finally{deciding.value=''}
}
onMounted(async()=>{await Promise.all([load(),(async()=>{try{const tree=await getResourceTree();for(const dc of tree.items||[])for(const room of dc.rooms||[])for(const rack of room.racks||[])racks.value[rack.id]=dc.name+' / '+room.name+' / '+rack.name}catch{}})()])})
</script>
<template><div class="asset-ops"><div class="ops-heading"><div><h1>审批中心</h1><p>集中处理上架、迁移和下架申请，审批规则及通知渠道在系统管理中维护。</p></div></div><el-card shadow="never"><div class="ops-toolbar between"><el-select v-model="status" style="width:160px" @change="load"><el-option label="全部状态" value=""/><el-option v-for="(label,key) in statusLabels" :key="key" :label="label" :value="key"/></el-select><el-button :loading="loading" @click="load">刷新</el-button></div><el-table :data="rows" v-loading="loading"><el-table-column label="设备 / 编码" min-width="180"><template #default="{row}">{{row.device?.name||row.deviceId}}<small class="ops-block ops-muted">{{row.device?.code}}</small></template></el-table-column><el-table-column label="操作" width="85"><template #default="{row}"><el-tag :type="row.operation==='DECOMMISSION'?'warning':'info'">{{operationLabels[row.operation]||row.operation}}</el-tag></template></el-table-column><el-table-column label="位置（下架显示原位置）" min-width="220"><template #default="{row}">{{position(row)}}</template></el-table-column><el-table-column prop="reason" label="申请原因" min-width="160"/><el-table-column label="状态" width="100"><template #default="{row}"><el-tag :type="row.status==='APPROVED'?'success':row.status==='REJECTED'?'danger':'info'">{{statusLabels[row.status]||row.status}}</el-tag></template></el-table-column><el-table-column label="申请时间" width="175"><template #default="{row}">{{new Date(row.requestedAt).toLocaleString('zh-CN')}}</template></el-table-column><el-table-column prop="decisionComment" label="审批意见" min-width="150"/><el-table-column label="处理" width="140" fixed="right"><template #default="{row}"><template v-if="row.status==='PENDING'"><el-button link type="success" :disabled="!!deciding" @click="decide(row,true)">通过</el-button><el-button link type="danger" :disabled="!!deciding" @click="decide(row,false)">驳回</el-button></template><span v-else class="ops-muted">已处理</span></template></el-table-column></el-table></el-card></div></template>

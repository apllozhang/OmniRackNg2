<script setup lang="ts">
import {ref,reactive,computed,onMounted,onBeforeUnmount} from 'vue'
import {ElMessage,ElMessageBox} from 'element-plus'
import {useAuthStore} from '@/stores/auth'
import {ops,listOps,matchScan,checkLabels,errorMessage,exportSheet} from '@/api/asset-ops'
import AssetScanner from '@/components/AssetScanner.vue'
import ScanAccessLinks from '@/components/ScanAccessLinks.vue'
import '@/styles/asset-ops.css'
const auth=useAuthStore(),tasks=ref<any[]>([]),sites=ref<any[]>([]),operators=ref<any[]>([]),current=ref<any>(null),lines=ref<any[]>([]),discoveries=ref<any[]>([]),entry=ref(''),loading=ref(false),busy=ref(false),createDialog=ref(false),scanDialog=ref(false),resolveDialog=ref(false),line=ref<any>(null),form=reactive<any>({}),scanForm=reactive<any>({}),resolveForm=reactive<any>({})
const detailLoading=ref(false),detailReady=ref(false),scanTaskId=ref('')
let detailRequest=0
const canWrite=computed(()=>auth.user?.roles?.some(r=>['system_admin','asset_manager'].includes(r.code)))
const canScan=computed(()=>detailReady.value&&!detailLoading.value&&!busy.value&&current.value?.state==='ACTIVE'&&current.value?.assigneeId===auth.user?.id&&canWrite.value)
const scanned=computed(()=>lines.value.filter(l=>l.result!=='UNSCANNED').length)
async function load(){loading.value=true;try{[tasks.value,sites.value,operators.value]=await Promise.all(['checks','sites','operators'].map(listOps))}catch(e){ElMessage.error(errorMessage(e))}finally{loading.value=false}}
async function open(task:any){
  const request=++detailRequest
  current.value=task||null;lines.value=[];discoveries.value=[];entry.value='';detailReady.value=false;scanTaskId.value=''
  if(!task){detailLoading.value=false;return}
  detailLoading.value=true
  try{
    const [nextLines,nextDiscoveries]=await Promise.all([listOps(`checks/${task.id}/lines`),listOps(`checks/${task.id}/discoveries`)])
    if(request!==detailRequest)return
    lines.value=nextLines;discoveries.value=nextDiscoveries;detailReady.value=true
  }catch(e){if(request===detailRequest)ElMessage.error(errorMessage(e))}
  finally{if(request===detailRequest)detailLoading.value=false}
}
function create(){Object.assign(form,{name:'',siteId:sites.value[0]?.id,assigneeId:auth.user?.id});createDialog.value=true}
async function save(){busy.value=true;try{const task=await ops('checks','POST',form);createDialog.value=false;await load();await open(task);ElMessage.success('任务与盘点快照已建立')}catch(e){ElMessage.error(errorMessage(e))}finally{busy.value=false}}
function identify(value:string){if(!canScan.value||scanDialog.value)return;entry.value=value;const matches=matchScan(lines.value,value);if(matches.length!==1){ElMessage.warning(matches.length?'匹配多条记录，请核对并在明细中选择':'不在任务范围内，可登记发现记录');return}const found=matches[0];if(found.result!=='UNSCANNED'){ElMessage.info('该对象已盘，不重复计数');return}openScan(found)}
function openScan(l:any){if(!canScan.value||scanDialog.value||!lines.value.some(item=>item.id===l.id))return;scanTaskId.value=current.value.id;line.value=l;Object.assign(scanForm,{lineId:l.id,actual:l.expected,difference:false,configurationVerified:false,note:''});scanDialog.value=true}
async function scan(){
  if(busy.value||!canScan.value||current.value?.id!==scanTaskId.value)return
  const taskId=scanTaskId.value,payload={...scanForm}
  busy.value=true
  try{
    const r=await ops(`checks/${taskId}/scan`,'POST',payload)
    scanDialog.value=false;entry.value=''
    if(current.value?.id===taskId)await open(current.value)
    ElMessage.success(r.duplicate?'已盘，未重复计数':'盘点记录已保存')
  }catch(e){ElMessage.error(errorMessage(e))}finally{busy.value=false}
}
async function discover(){try{const r=await ElMessageBox.prompt('说明现场识别内容、位置及发现情况，不会自动新建资产。','登记发现记录',{inputType:'textarea',inputValidator:(v:string)=>!!v?.trim()||'请填写说明'});await ops(`checks/${current.value.id}/discoveries`,'POST',{code:entry.value,note:r.value});await open(current.value);ElMessage.success('发现记录已保存')}catch(e){if(e instanceof Error||typeof e==='object')ElMessage.error(errorMessage(e))}}
function resolve(l:any){line.value=l;Object.assign(resolveForm,{resolution:'',adjustQuantity:false});resolveDialog.value=true}
async function saveResolution(){busy.value=true;try{await ops(`checks/${current.value.id}/lines/${line.value.id}/resolve`,'POST',resolveForm);resolveDialog.value=false;await open(current.value);ElMessage.success('差异已复核')}catch(e){ElMessage.error(errorMessage(e))}finally{busy.value=false}}
async function resolveDiscovery(d:any){try{const r=await ElMessageBox.prompt('填写核查结果及对应台账或单据依据。','复核发现记录',{inputType:'textarea'});await ops(`checks/${current.value.id}/discoveries/${d.id}/resolve`,'POST',{resolution:r.value});await open(current.value)}catch(e){if(e instanceof Error||typeof e==='object')ElMessage.error(errorMessage(e))}}
async function close(){try{await ElMessageBox.confirm('结案前会再次检查台账版本，未盘及差异必须先处理。','确认结案');await ops(`checks/${current.value.id}/close`,'POST');await load();await open(tasks.value.find(t=>t.id===current.value.id));ElMessage.success('盘点已结案')}catch(e){if(e instanceof Error||typeof e==='object'){ElMessage.error(errorMessage(e));await open(current.value)}}}
function exportLines(){exportSheet('盘点结果',lines.value.map(l=>({任务:current.value.name,对象类型:l.kind,编码:l.code,名称:l.name,SN:l.serialNumber,账面数量:l.expected,实盘数量:l.actual,结果:checkLabels[l.result],配置核验:l.configurationVerified?'已核验':'未核验',现场情况:l.note,盘点时间:l.scannedAt,处理依据:l.resolution,复核人:l.resolvedBy})));if(discoveries.value.length)exportSheet('盘点发现记录',discoveries.value.map(d=>({任务:current.value.name,识别码:d.code,说明:d.note,处理结果:d.resolution})))}
onMounted(load)
onBeforeUnmount(()=>{detailRequest++})
</script>
<template><div class="asset-ops"><div class="ops-heading"><div><span class="ops-eyebrow">PHYSICAL INVENTORY</span><h1>扫码盘点</h1><p>先确定范围，再核对实物；扫码不直接覆盖台账。</p></div><el-button v-if="canWrite" type="primary" @click="create">创建盘点任务</el-button></div><ScanAccessLinks/><div class="ops-check-layout"><el-card shadow="never" class="ops-task-list" v-loading="loading"><h3>盘点任务</h3><button v-for="t in tasks" :key="t.id" :disabled="busy||scanDialog||resolveDialog" class="ops-task-card" :class="{selected:current?.id===t.id}" @click="open(t)"><el-tag :type="t.state==='ACTIVE'?'primary':'success'">{{t.state==='ACTIVE'?'进行中':'已结案'}}</el-tag><h4>{{t.name}}</h4><p>{{t.site?.name}}</p><small>{{new Date(t.createdAt).toLocaleDateString()}}</small></button><el-empty v-if="!tasks.length" description="暂无盘点任务" :image-size="70"/></el-card><el-card shadow="never" class="ops-check-main" v-loading="detailLoading"><template v-if="current"><div class="ops-toolbar between"><div><h3>{{current.name}}</h3><p class="ops-muted">{{current.site?.name}} · 已盘 {{scanned}} / {{lines.length}}</p></div><div><el-button @click="exportLines">导出结果</el-button><el-button v-if="canWrite&&current.state==='ACTIVE'" @click="close">复核结案</el-button></div></div><div v-if="canScan" class="ops-scan-box"><div class="ops-toolbar"><el-input v-model="entry" placeholder="扫描标签，或输入资产编号 / SN" @keyup.enter="identify(entry)"/><el-button type="primary" :disabled="!entry.trim()" @click="identify(entry)">识别核对</el-button></div><AssetScanner :key="current.id" :disabled="busy||scanDialog||detailLoading" @result="identify"/><el-button text :disabled="!entry.trim()" @click="discover">登记范围外 / 未建账设备</el-button><p class="ops-muted">仅指定盘点人可执行；配置未现场核验时，不标记为配置一致。</p></div><el-table :data="lines"><el-table-column label="设备 / 备件" min-width="180"><template #default="{row}"><b>{{row.name}}</b><small class="ops-block ops-mono">{{row.serialNumber||row.code}}</small></template></el-table-column><el-table-column prop="expected" label="账面" width="65"/><el-table-column prop="actual" label="实盘" width="65"/><el-table-column label="结果" width="160"><template #default="{row}"><el-tag :type="row.result==='NORMAL'?'success':row.result==='UNSCANNED'?'info':'warning'">{{checkLabels[row.result]}}</el-tag></template></el-table-column><el-table-column label="操作" width="120"><template #default="{row}"><el-button v-if="canScan&&row.result==='UNSCANNED'" link type="primary" @click="openScan(row)">核对</el-button><el-button v-if="canWrite&&current.state==='ACTIVE'&&row.result!=='NORMAL'&&!row.resolvedBy" link type="primary" @click="resolve(row)">复核</el-button><span v-if="row.resolvedBy" class="ops-muted">已复核</span></template></el-table-column></el-table><h3 class="ops-section-title">发现记录</h3><el-table :data="discoveries"><el-table-column prop="code" label="识别码" min-width="180"/><el-table-column prop="note" label="现场情况" min-width="200"/><el-table-column prop="resolution" label="处理结果" min-width="180"/><el-table-column v-if="canWrite&&current.state==='ACTIVE'" label="操作" width="90"><template #default="{row}"><el-button v-if="!row.resolvedBy" link type="primary" @click="resolveDiscovery(row)">复核</el-button></template></el-table-column></el-table></template><el-empty v-else description="选择任务开始盘点"/></el-card></div>
<el-dialog v-model="createDialog" title="创建盘点任务" width="min(560px,95vw)"><el-form label-position="top"><el-form-item label="任务名称"><el-input v-model="form.name"/></el-form-item><el-form-item label="资产位置"><el-select v-model="form.siteId"><el-option v-for="s in sites" :key="s.id" :label="s.region+' / '+s.name" :value="s.id"/></el-select></el-form-item><el-form-item label="指定盘点人"><el-select v-model="form.assigneeId"><el-option v-for="u in operators.filter(u=>u.canCheck)" :key="u.id" :label="u.name" :value="u.id"/></el-select></el-form-item></el-form><el-alert title="关联机房时纳入该机房上架设备；同时纳入该位置的未上架资产与在库备件。" :closable="false"/><template #footer><el-button type="primary" :loading="busy" @click="save">创建任务与快照</el-button></template></el-dialog>
<el-dialog v-model="scanDialog" title="核对现场实物" width="min(560px,95vw)"><p><b>{{line?.name}}</b> · SN {{line?.serialNumber||'待补全'}}</p><el-form label-position="top"><el-form-item label="实盘数量"><el-input-number v-model="scanForm.actual" :min="0" :precision="0" :max="line?.kind==='DEVICE'||line?.serialNumber?1:1000000"/></el-form-item><el-checkbox v-model="scanForm.difference">位置、责任人或配置存在差异</el-checkbox><el-checkbox v-model="scanForm.configurationVerified">已现场检查具体配置</el-checkbox><el-form-item label="现场情况 / 差异说明"><el-input v-model="scanForm.note" type="textarea" :rows="3"/></el-form-item></el-form><template #footer><el-button @click="scanDialog=false">取消</el-button><el-button type="primary" :loading="busy" @click="scan">确认盘点</el-button></template></el-dialog>
<el-dialog v-model="resolveDialog" title="差异复核" width="min(560px,95vw)"><el-alert title="差异不能由原盘点人自我复核；设备信息更正请先通过对应业务流程处理。" :closable="false"/><p>{{line?.name}} · {{line?.note}}</p><el-input v-model="resolveForm.resolution" type="textarea" :rows="4" placeholder="填写核查过程和处理依据"/><p v-if="line?.kind==='SPARE'&&!line?.serialNumber&&line?.actual!==undefined"><el-checkbox v-model="resolveForm.adjustQuantity">根据实盘数量生成库存调整流水（要求台账未变化）</el-checkbox></p><template #footer><el-button type="primary" :loading="busy" @click="saveResolution">确认复核</el-button></template></el-dialog></div></template>

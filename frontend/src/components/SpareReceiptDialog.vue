<script setup lang="ts">
import {computed,reactive,ref,watch} from 'vue'
import {submissionId} from '@/utils/submissionId'
import {ElMessage} from 'element-plus'
import {ops,errorMessage} from '@/api/asset-ops'
const props=defineProps<{modelValue:boolean;specs:any[];sites:any[]}>()
const emit=defineEmits<{'update:modelValue':[boolean];saved:[]}>()
const saving=ref(false),existing=ref(false)
const form=reactive({submissionId:'',specId:'',siteId:'',category:'',name:'',modelNumber:'',quantity:1,serialNumber:'',notes:'',state:'AVAILABLE'})
const spec=computed(()=>props.specs.find(s=>s.id===form.specId))
const initialStates={AVAILABLE:'可用',TESTING:'待检测',FAULTY:'故障',REPAIR:'维修中'}
const serial=computed(()=>existing.value?spec.value?.tracking==='SERIAL':!!form.serialNumber.trim())
watch(()=>props.modelValue,open=>{if(open){existing.value=false;Object.assign(form,{submissionId:submissionId(),specId:'',siteId:'',category:'',name:'',modelNumber:'',quantity:1,serialNumber:'',notes:'',state:'AVAILABLE'})}},{immediate:true})
watch(()=>form.specId,()=>{form.serialNumber='';if(spec.value?.tracking==='SERIAL')form.quantity=1})
watch(existing,value=>{if(value&&spec.value?.tracking==='QUANTITY')form.serialNumber=''})
async function save(){
 if(!form.siteId||(existing.value&&!spec.value)||(!existing.value&&(!form.category.trim()||!form.name.trim()))||form.quantity<1||!Number.isInteger(form.quantity)||(serial.value&&(form.quantity!==1||!form.serialNumber.trim()))||new TextEncoder().encode(form.notes).length>1000){ElMessage.warning('请填写位置、备件及有效数量；按件备件须填写 SN 且数量为 1，备注不超过 1000 字节');return}
 saving.value=true
 try{await ops('stock-receipts','POST',{...form,specId:existing.value?form.specId:null});ElMessage.success('备件已登记，库存和流水已同步更新');emit('update:modelValue',false);emit('saved')}catch(e){ElMessage.error(errorMessage(e))}finally{saving.value=false}
}
</script>
<template>
  <el-dialog :model-value="modelValue" title="录入备件" width="min(720px,95vw)" :close-on-click-modal="!saving" :close-on-press-escape="!saving" :show-close="!saving" @update:model-value="emit('update:modelValue',$event)">
    <div class="receipt-intro"><p>填写备件基本信息即可保存，规格资料及入库流水自动关联。</p><el-button v-if="specs.length" link type="primary" @click="existing=!existing">{{existing?'直接填写':'从已有型号带入'}}</el-button></div>
    <el-form label-position="top"><p v-if="!sites.length" class="receipt-help">暂无存放位置，请先在基础资料的区域与资产位置中登记。</p>
      <el-form-item v-if="existing" label="选择已有型号" required><el-select v-model="form.specId" filterable placeholder="选择后自动带入类型、名称、型号"><el-option v-for="s in specs" :key="s.id" :label="s.category+' / '+s.name+' / '+(s.modelNumber||s.specification)+(s.tracking==='SERIAL'?' · 按件':' · 数量')" :value="s.id"/></el-select></el-form-item>
      <div class="receipt-grid">
        <el-form-item label="区域 / 存放位置" required><el-select v-model="form.siteId" filterable placeholder="选择所在区域及具体位置"><el-option v-for="site in sites" :key="site.id" :label="site.region+' / '+site.name" :value="site.id"/></el-select></el-form-item>
        <el-form-item label="类型" required><el-input v-if="existing" :model-value="spec?.category||''" disabled/><el-input v-else v-model="form.category" placeholder="如：硬盘、内存、电源"/></el-form-item>
        <el-form-item label="名称" required><el-input v-if="existing" :model-value="spec?.name||''" disabled/><el-input v-else v-model="form.name" placeholder="填写备件名称"/></el-form-item>
        <el-form-item label="型号"><el-input v-if="existing" :model-value="spec?.modelNumber||''" disabled/><el-input v-else v-model="form.modelNumber" placeholder="填写具体型号"/></el-form-item>
        <el-form-item label="数量" required><el-input-number v-model="form.quantity" :min="1" :max="serial?1:1000000" :precision="0"/></el-form-item>
        <el-form-item label="序列号" :required="serial"><el-input v-model="form.serialNumber" :disabled="existing&&spec?.tracking==='QUANTITY'" placeholder="有 SN 填写，无 SN 可留空"/></el-form-item>
        <el-form-item label="状态" required><el-select v-model="form.state"><el-option v-for="(label,value) in initialStates" :key="value" :label="label" :value="value"/></el-select></el-form-item>
      </div>
      <el-form-item label="备注"><el-input v-model="form.notes" type="textarea" :rows="2" maxlength="330" show-word-limit placeholder="用途、兼容说明或其他补充信息"/></el-form-item>
      <p class="receipt-help">一个 SN 对应一件；无 SN 按数量管理。这里登记当前实际状态，后续调拨、维修和状态变化在库存操作中办理，保留完整流水。</p>
    </el-form>
    <template #footer><el-button :disabled="saving" @click="emit('update:modelValue',false)">取消</el-button><el-button type="primary" :loading="saving" @click="save">保存备件</el-button></template>
  </el-dialog>
</template>
<style scoped>.receipt-intro{display:flex;align-items:center;justify-content:space-between;gap:12px;color:#748399;font-size:12px;margin-bottom:18px}.receipt-grid{display:grid;grid-template-columns:1fr 1fr;gap:0 20px}.receipt-help{font-size:12px;color:#748399;line-height:1.8;margin-left:8px}.el-select,.el-input-number{width:100%}@media(max-width:600px){.receipt-intro{display:flex;align-items:center;justify-content:space-between;gap:12px;color:#748399;font-size:12px;margin-bottom:18px}.receipt-grid{grid-template-columns:1fr}}</style>

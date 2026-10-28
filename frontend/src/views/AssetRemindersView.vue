<script setup lang="ts">
import {ref,computed,onMounted} from 'vue'
import {ElMessage,ElMessageBox} from 'element-plus'
import {useAuthStore} from '@/stores/auth'
import {ops,listOps,errorMessage} from '@/api/asset-ops'
const auth=useAuthStore(),reminders=ref<any[]>([]),busy=ref(false),loading=ref(false)
const canWrite=computed(()=>auth.user?.roles?.some(r=>['system_admin','asset_manager'].includes(r.code)))
async function load(){loading.value=true;try{reminders.value=await listOps('reminders')}catch(e){ElMessage.error(errorMessage(e))}finally{loading.value=false}}
async function refresh(){busy.value=true;try{await ops('reminders/refresh','POST');await load();ElMessage.success('提醒已更新')}catch(e){ElMessage.error(errorMessage(e))}finally{busy.value=false}}
async function acknowledge(a:any){try{const r=await ElMessageBox.prompt('填写处理依据，处理不会改变真实保修状态。','记录提醒处理',{inputType:'textarea',inputValidator:(v:string)=>!!v?.trim()||'请填写处理依据'});await ops(`reminders/${a.id}/acknowledge`,'POST',{note:r.value});await load()}catch(e){if(e instanceof Error||typeof e==='object')ElMessage.error(errorMessage(e))}}
onMounted(load)
import '@/styles/asset-ops.css'
</script>
<template><div class="asset-ops"><div class="ops-heading"><div><h1>提醒中心</h1><p>集中处理设备保修、U 位容量及备件库存风险；通知渠道在系统管理的告警通知中配置。</p></div></div><el-card shadow="never"><div v-loading="loading"><div class="ops-toolbar between"><p class="ops-muted">每 15 分钟检查保修与 U 位容量风险；已处理提醒停止外部通知。</p><el-button v-if="canWrite" :loading="busy" @click="refresh">立即检查</el-button></div><el-table :data="reminders"><el-table-column prop="title" label="提醒" min-width="260"/><el-table-column prop="detail" label="详情" min-width="300"/><el-table-column label="级别" width="100"><template #default="{row}"><el-tag :type="row.severity==='DANGER'?'danger':'warning'">{{row.severity==='DANGER'?'高关注':'提醒'}}</el-tag></template></el-table-column><el-table-column prop="acknowledgement" label="处理依据" min-width="200"/><el-table-column v-if="canWrite" label="操作" width="110"><template #default="{row}"><el-button link type="primary" @click="acknowledge(row)">记录处理</el-button></template></el-table-column></el-table></div></el-card></div></template>

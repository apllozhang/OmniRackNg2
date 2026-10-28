<script setup lang="ts">
import {ref,onMounted} from 'vue'
import {ElMessage} from 'element-plus'
import {ops,listOps,errorMessage,exportSheet} from '@/api/asset-ops'
const rows=ref<any[]>([]),users=ref<any[]>([]),page=ref(1),total=ref(0),loading=ref(false)
async function load(){loading.value=true;try{const r=await ops(`audit?page=${page.value}`);rows.value=r.items;total.value=r.total}catch(e){ElMessage.error(errorMessage(e))}finally{loading.value=false}}
function format(value:string){try{return JSON.stringify(JSON.parse(value),null,2)}catch{return value}}
function name(id:string){return users.value.find(u=>u.id===id)?.name||id||'系统任务'}
onMounted(async()=>{await load();try{users.value=await listOps('operators')}catch{}})
</script>
<template><div class="ops-toolbar between"><span class="ops-muted">资产与备件变更记录 · 不展示账号口令及 LDAP 配置内容</span><el-button @click="exportSheet('资产操作审计',rows.map(r=>({时间:r.createdAt,操作人:name(r.userId),操作:r.action,对象类型:r.resourceType,对象ID:r.resourceId,变更前:r.before,变更后:r.after,请求ID:r.requestId})))">导出当前页</el-button></div><el-table :data="rows" v-loading="loading"><el-table-column label="时间" min-width="180"><template #default="{row}">{{new Date(row.createdAt).toLocaleString()}}</template></el-table-column><el-table-column label="操作人" min-width="140"><template #default="{row}">{{name(row.userId)}}</template></el-table-column><el-table-column prop="action" label="操作" min-width="140"/><el-table-column prop="resourceType" label="对象类型" min-width="150"/><el-table-column label="变更详情" min-width="260"><template #default="{row}"><details><summary>查看变更前后</summary><pre class="ops-audit-json">{{format(row.before)}}</pre><pre class="ops-audit-json">{{format(row.after)}}</pre></details></template></el-table-column></el-table><div class="ops-toolbar between"><span class="ops-muted">共 {{total}} 条，每页 50 条</span><el-pagination v-model:current-page="page" :total="total" :page-size="50" layout="prev,pager,next" @current-change="load"/></div></template>
<style scoped>summary{color:#537cbd;cursor:pointer;font-size:12px}.ops-audit-json{padding:12px;background:var(--color-canvas);max-height:260px;overflow:auto;white-space:pre-wrap;overflow-wrap:anywhere;font-size:10px}</style>

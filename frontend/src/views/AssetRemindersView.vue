<script setup lang="ts">
import { useColumnWidths } from '@/features/table/useColumnWidths'

import TableSortHeader from '@/components/TableSortHeader.vue'
import TablePager from '@/components/TablePager.vue'
import { useDebouncedValue } from '@/features/table/useTableView'
import { useClientTable } from '@/features/table/useClientTable'

import {ref,computed,onMounted} from 'vue'
import {ElMessage,ElMessageBox} from 'element-plus'
import {useAuthStore} from '@/stores/auth'
import {ops,listOps,errorMessage} from '@/api/asset-ops'
const auth=useAuthStore(),reminders=ref<any[]>([]),busy=ref(false),loading=ref(false)
const canWrite=computed(()=>auth.user?.roles?.some(r=>['system_admin','asset_manager'].includes(r.code)))
async function load(){loading.value=true;try{reminders.value=await listOps('reminders')}catch(e){ElMessage.error(errorMessage(e))}finally{loading.value=false}}
async function refresh(){busy.value=true;try{await ops('reminders/refresh','POST');await load();ElMessage.success('提醒已更新')}catch(e){ElMessage.error(errorMessage(e))}finally{busy.value=false}}
async function acknowledge(a:any){try{const r=await ElMessageBox.prompt('填写处理依据，处理不会改变真实保修状态。','记录提醒处理',{inputType:'textarea',inputValidator:(v:string)=>!!v?.trim()||'请填写处理依据'});await ops(`reminders/${a.id}/acknowledge`,'POST',{note:r.value});await load()}catch(e){if(e instanceof Error||typeof e==='object')ElMessage.error(errorMessage(e))}}

const search = ref('')
const query = useDebouncedValue(search)
const filteredRows = computed(() => reminders.value.filter(row => !query.value.trim() || [row.title, row.detail, row.acknowledgement].some(value => String(value || '').toLowerCase().includes(query.value.trim().toLowerCase()))))
function clearSearch() { search.value = ''; query.value = '' }
const { page, pageSize, total, pageRows, tableRef, sortBy, onSortChange } = useClientTable(filteredRows, {severityLabel: row => row.severity==='DANGER'?'高关注':'提醒'})

onMounted(load)
import '@/styles/asset-ops.css'
const { columnWidths, resizeColumn } = useColumnWidths({"title": 260, "detail": 300, "severityLabel": 100, "acknowledgement": 200})
</script>
<template><div class="asset-ops"><div class="ops-heading"><div><h1>提醒中心</h1><p>集中处理设备保修、U 位容量及备件库存风险；通知渠道在系统管理的告警通知中配置。</p></div></div><el-card shadow="never"><div v-loading="loading"><div class="ops-toolbar between"><p class="ops-muted">每 15 分钟检查保修与 U 位容量风险；已处理提醒停止外部通知。</p><el-button v-if="canWrite" :loading="busy" @click="refresh">立即检查</el-button></div><el-input v-model="search" clearable aria-label="搜索提醒或处理依据" placeholder="搜索提醒或处理依据" style="max-width:360px;margin-bottom:12px"/><el-table :data="pageRows" :aria-busy="loading||search!==query" ref="tableRef" @sort-change="onSortChange"><el-table-column label="提醒" prop="title" sortable="custom" :width="columnWidths['title']"><template #header><TableSortHeader label="提醒" :width="columnWidths['title']" @resize="resizeColumn('title', $event)" @sort="sortBy('title')"/></template></el-table-column><el-table-column label="详情" prop="detail" sortable="custom" :width="columnWidths['detail']"><template #header><TableSortHeader label="详情" :width="columnWidths['detail']" @resize="resizeColumn('detail', $event)" @sort="sortBy('detail')"/></template></el-table-column><el-table-column label="级别" prop="severityLabel" sortable="custom" :width="columnWidths['severityLabel']"><template #header><TableSortHeader label="级别" :width="columnWidths['severityLabel']" @resize="resizeColumn('severityLabel', $event)" @sort="sortBy('severityLabel')"/></template><template #default="{row}"><el-tag :type="row.severity==='DANGER'?'danger':'warning'">{{row.severity==='DANGER'?'高关注':'提醒'}}</el-tag></template></el-table-column><el-table-column label="处理依据" prop="acknowledgement" sortable="custom" :width="columnWidths['acknowledgement']"><template #header><TableSortHeader label="处理依据" :width="columnWidths['acknowledgement']" @resize="resizeColumn('acknowledgement', $event)" @sort="sortBy('acknowledgement')"/></template></el-table-column><el-table-column v-if="canWrite" label="操作" width="110"><template #default="{row}"><el-button link type="primary" @click="acknowledge(row)">记录处理</el-button></template></el-table-column><template #empty><el-empty :description="reminders.length ? '没有匹配结果' : '暂无记录'"><el-button v-if="query" @click="clearSearch">清除筛选</el-button></el-empty></template></el-table><TablePager v-model:page="page" v-model:page-size="pageSize" :total="total" label="提醒"/></div></el-card></div></template>

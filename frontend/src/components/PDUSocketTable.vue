<script setup lang="ts">
import {computed,ref} from 'vue'
import type {PDUSocket,PDUConnection} from '@/types/pdu'
import TableSortHeader from './TableSortHeader.vue'
import TablePager from './TablePager.vue'
import {useClientTable} from '@/features/table/useClientTable'
import {useColumnWidths} from '@/features/table/useColumnWidths'
import {useDebouncedValue} from '@/features/table/useTableView'
const props=defineProps<{sockets:PDUSocket[];connectionFor:(socket:PDUSocket)=>PDUConnection|undefined;groupLabel:string}>()
const search=ref(''),query=useDebouncedValue(search)
const stateLabel=(value:string)=>({AVAILABLE:'可用',CONNECTED:'已连接',DISABLED:'停用'}[value]||value)
const deviceName=(row:PDUSocket)=>{const device=props.connectionFor(row)?.device;return device?.name||device?.code||'未连接'}
const filtered=computed(()=>props.sockets.filter(row=>!query.value.trim()||[row.socketNo,row.label,deviceName(row),stateLabel(row.status)].some(value=>String(value||'').toLowerCase().includes(query.value.trim().toLowerCase()))))
const {page,pageSize,total,pageRows,tableRef,sortBy,onSortChange}=useClientTable(filtered,{standard:row=>(row.standard==='CN'?'国标':'欧标')+' / '+row.amperageA+'A',status:row=>stateLabel(row.status),device:row=>deviceName(row)})
const {columnWidths,resizeColumn}=useColumnWidths({socketNo:110,standard:145,status:120,device:180})
const fields=[{key:'socketNo',label:'插座'},{key:'standard',label:'制式/电流'},{key:'status',label:'状态'},{key:'device',label:'供电设备'}]
</script>
<template>
 <el-input v-model="search" clearable :aria-label="`搜索${groupLabel}插座`" placeholder="搜索插座、标签或供电设备" style="max-width:360px;margin:10px 0"/>
 <el-table :max-height="300" :data="pageRows" ref="tableRef" @sort-change="onSortChange" :aria-busy="search!==query" size="small" border empty-text="暂无匹配插座">
  <el-table-column v-for="field in fields" :key="field.key" :prop="field.key" :label="field.label" sortable="custom" :width="columnWidths[field.key]">
   <template #header><TableSortHeader :label="field.label" :width="columnWidths[field.key]" @sort="sortBy(field.key)" @resize="resizeColumn(field.key,$event)"/></template>
   <template #default="{row}"><span v-if="field.key==='socketNo'">{{row.socketNo}}#</span><span v-else-if="field.key==='standard'">{{row.standard==='CN'?'国标':'欧标'}} / {{row.amperageA}}A</span><el-tag v-else-if="field.key==='status'" :type="row.status==='AVAILABLE'?'success':row.status==='DISABLED'?'info':'warning'" size="small">{{stateLabel(row.status)}}</el-tag><span v-else>{{deviceName(row)}}</span></template>
  </el-table-column>
  <el-table-column label="操作" width="220" fixed="right"><template #default="{row}"><slot name="actions" :socket="row"/></template></el-table-column>
 </el-table>
 <TablePager v-model:page="page" v-model:page-size="pageSize" :total="total" :label="groupLabel+'插座'"/>
</template>

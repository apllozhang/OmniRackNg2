<script setup lang="ts">
import {moveLabels,stockStates} from '@/api/asset-ops'
const props=defineProps<{rows:any[];sites:any[];stockId?:string}>()
function siteName(id:string){const site=props.sites.find(s=>s.id===id);return site?site.region+' / '+site.name:'—'}
function destination(row:any){return !!props.stockId&&row.operation==='TRANSFER'&&row.destinationStockId===props.stockId}
</script>
<template>
  <el-table :data="rows" empty-text="暂无库存流水">
    <el-table-column label="备件 / 序列号" min-width="180"><template #default="{row}">{{row.stock?.spec?.name||'—'}}<small class="ops-block ops-mono">{{row.stock?.serialNumber||'数量管理'}}</small></template></el-table-column>
    <el-table-column label="操作" width="105"><template #default="{row}">{{moveLabels[row.operation]||row.operation}}</template></el-table-column>
    <el-table-column label="数量变化" width="120"><template #default="{row}">{{destination(row)?0:row.beforeQuantity}} → {{destination(row)?row.quantity:row.afterQuantity}}</template></el-table-column>
    <el-table-column label="状态变化" min-width="160"><template #default="{row}">{{destination(row)?'新增调拨批次':stockStates[row.beforeState]||'未记录'}} → {{destination(row)?stockStates.TRANSIT:stockStates[row.afterState]||'未记录'}}</template></el-table-column>
    <el-table-column label="位置 / 调拨目标" min-width="220"><template #default="{row}">{{siteName(row.sourceSiteId)}}<small v-if="row.destinationSiteId" class="ops-block ops-muted">→ {{siteName(row.destinationSiteId)}}</small></template></el-table-column>
    <el-table-column prop="reason" label="事由 / 依据" min-width="200" show-overflow-tooltip/>
    <el-table-column label="时间" min-width="175"><template #default="{row}">{{row.createdAt?new Date(row.createdAt).toLocaleString('zh-CN'):'—'}}</template></el-table-column>
  </el-table>
</template>

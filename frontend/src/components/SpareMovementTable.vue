<script setup lang="ts">
import { useColumnWidths } from '@/features/table/useColumnWidths'

import TableSortHeader from '@/components/TableSortHeader.vue'
import TablePager from '@/components/TablePager.vue'
import { useClientTable } from '@/features/table/useClientTable'

import {moveLabels,stockStates} from '@/api/asset-ops'
const props=defineProps<{rows:any[];sites:any[];stockId?:string}>()
function siteName(id:string){const site=props.sites.find(s=>s.id===id);return site?site.region+' / '+site.name:'—'}
function destination(row:any){return !!props.stockId&&row.operation==='TRANSFER'&&row.destinationStockId===props.stockId}

const { page, pageSize, total, pageRows, sortedRows, tableRef, sortBy, onSortChange } = useClientTable(() => props.rows, {operationLabel: row => moveLabels[row.operation]||row.operation, quantityAfter: row => destination(row)?row.quantity:row.afterQuantity, stateAfter: row => destination(row)?stockStates.TRANSIT:stockStates[row.afterState], siteLabel: row => siteName(row.sourceSiteId)+(row.destinationSiteId?' → '+siteName(row.destinationSiteId):'')})
const { columnWidths, resizeColumn } = useColumnWidths({"stock.spec.name": 180, "operationLabel": 105, "quantityAfter": 120, "stateAfter": 160, "siteLabel": 220, "reason": 200, "createdAt": 175})
defineExpose({ exportRows: () => sortedRows.value })
</script>
<template>
  <el-table :data="pageRows" ref="tableRef" @sort-change="onSortChange" empty-text="暂无库存流水">
    <el-table-column label="备件 / 序列号" prop="stock.spec.name" sortable="custom" :width="columnWidths['stock.spec.name']"><template #header><TableSortHeader label="备件 / 序列号" :width="columnWidths['stock.spec.name']" @resize="resizeColumn('stock.spec.name', $event)" @sort="sortBy('stock.spec.name')"/></template><template #default="{row}">{{row.stock?.spec?.name||'—'}}<small class="ops-block ops-mono">{{row.stock?.serialNumber||'数量管理'}}</small></template></el-table-column>
    <el-table-column label="操作" prop="operationLabel" sortable="custom" :width="columnWidths['operationLabel']"><template #header><TableSortHeader label="操作" :width="columnWidths['operationLabel']" @resize="resizeColumn('operationLabel', $event)" @sort="sortBy('operationLabel')"/></template><template #default="{row}">{{moveLabels[row.operation]||row.operation}}</template></el-table-column>
    <el-table-column label="数量变化" prop="quantityAfter" sortable="custom" :width="columnWidths['quantityAfter']"><template #header><TableSortHeader label="数量变化" :width="columnWidths['quantityAfter']" @resize="resizeColumn('quantityAfter', $event)" @sort="sortBy('quantityAfter')"/></template><template #default="{row}">{{destination(row)?0:row.beforeQuantity}} → {{destination(row)?row.quantity:row.afterQuantity}}</template></el-table-column>
    <el-table-column label="状态变化" prop="stateAfter" sortable="custom" :width="columnWidths['stateAfter']"><template #header><TableSortHeader label="状态变化" :width="columnWidths['stateAfter']" @resize="resizeColumn('stateAfter', $event)" @sort="sortBy('stateAfter')"/></template><template #default="{row}">{{destination(row)?'新增调拨批次':stockStates[row.beforeState]||'未记录'}} → {{destination(row)?stockStates.TRANSIT:stockStates[row.afterState]||'未记录'}}</template></el-table-column>
    <el-table-column label="位置 / 调拨目标" prop="siteLabel" sortable="custom" :width="columnWidths['siteLabel']"><template #header><TableSortHeader label="位置 / 调拨目标" :width="columnWidths['siteLabel']" @resize="resizeColumn('siteLabel', $event)" @sort="sortBy('siteLabel')"/></template><template #default="{row}">{{siteName(row.sourceSiteId)}}<small v-if="row.destinationSiteId" class="ops-block ops-muted">→ {{siteName(row.destinationSiteId)}}</small></template></el-table-column>
    <el-table-column label="事由 / 依据" show-overflow-tooltip prop="reason" sortable="custom" :width="columnWidths['reason']"><template #header><TableSortHeader label="事由 / 依据" :width="columnWidths['reason']" @resize="resizeColumn('reason', $event)" @sort="sortBy('reason')"/></template></el-table-column>
    <el-table-column label="时间" prop="createdAt" sortable="custom" :width="columnWidths['createdAt']"><template #header><TableSortHeader label="时间" :width="columnWidths['createdAt']" @resize="resizeColumn('createdAt', $event)" @sort="sortBy('createdAt')"/></template><template #default="{row}">{{row.createdAt?new Date(row.createdAt).toLocaleString('zh-CN'):'—'}}</template></el-table-column>
  </el-table><TablePager v-model:page="page" v-model:page-size="pageSize" :total="total" label="库存流水"/>
</template>

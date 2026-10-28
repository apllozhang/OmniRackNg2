<script setup lang="ts">
import AssetAuditPanel from '@/components/AssetAuditPanel.vue'
import '@/styles/asset-ops.css'
</script>
<template><div class="asset-ops"><div class="ops-heading"><div><h1>资产操作记录</h1><p>查询资产、位置、备件和盘点等业务变更，追溯操作人及变更前后内容。</p></div></div><el-card shadow="never"><AssetAuditPanel/></el-card></div></template>

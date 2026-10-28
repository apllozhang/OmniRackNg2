<script setup lang="ts">
defineProps<{ page: number; pageSize: number; total: number; label: string }>()
defineEmits<{ 'update:page': [value: number]; 'update:pageSize': [value: number] }>()
</script>
<template>
  <nav class="table-pager" :aria-label="`${label}分页`">
    <el-pagination :current-page="page" :page-size="pageSize" :total="total" :page-sizes="[20,50,100]" layout="total, sizes, prev, pager, next" @update:current-page="$emit('update:page', $event)" @update:page-size="$emit('update:pageSize', $event)" />
  </nav>
</template>
<style scoped>
.table-pager{display:flex;justify-content:flex-end;max-width:100%;overflow-x:auto;padding:16px 2px 4px}
@media(max-width:600px){.table-pager{justify-content:flex-start}.table-pager :deep(.el-pagination){flex-wrap:wrap;gap:8px}}
</style>

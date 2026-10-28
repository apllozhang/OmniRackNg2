<script setup lang="ts">
defineProps<{ label: string }>()
defineEmits<{ sort: [] }>()
</script>
<template><button type="button" class="table-sort-header" :aria-label="`按${label}排序`" @click.stop="$emit('sort')">{{ label }}</button></template>
<style scoped>
.table-sort-header{font:inherit;font-weight:inherit;color:inherit;background:transparent;border:0;padding:6px 0;cursor:pointer;text-align:left;min-height:36px}
.table-sort-header:focus-visible{outline:2px solid var(--color-action);outline-offset:2px}
</style>

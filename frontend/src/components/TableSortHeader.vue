<script setup lang="ts">
defineProps<{ label: string; width?: number }>()
const emit = defineEmits<{ sort: []; resize: [delta: number] }>()
function resize(event: KeyboardEvent) {
  if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight') return
  event.preventDefault(); event.stopPropagation()
  emit('resize', (event.key === 'ArrowRight' ? 1 : -1) * (event.shiftKey ? 1 : 10))
}
</script>
<template>
  <span class="table-header-controls">
    <button type="button" class="table-sort-header" :aria-label="`按${label}排序`" @click.stop="$emit('sort')">{{ label }}</button>
    <span v-if="width !== undefined" role="separator" tabindex="0" aria-orientation="vertical" :aria-label="`调整${label}列宽`" :aria-valuenow="width" :aria-valuemin="80" :aria-valuemax="800" title="左右方向键调整10像素，Shift加方向键调整1像素" class="table-column-resizer" @click.stop @keydown="resize">⋮</span>
  </span>
</template>
<style scoped>
.table-header-controls{display:inline-flex;align-items:center;gap:6px;max-width:100%}
.table-sort-header{font:inherit;font-weight:inherit;color:inherit;background:transparent;border:0;padding:6px 0;cursor:pointer;text-align:left;min-height:36px;overflow:hidden;text-overflow:ellipsis}
.table-column-resizer{display:inline-flex;align-items:center;justify-content:center;width:18px;min-height:32px;flex:none;cursor:col-resize;color:var(--color-text-muted)}
.table-sort-header:focus-visible,.table-column-resizer:focus-visible{outline:2px solid var(--color-action);outline-offset:2px}
</style>

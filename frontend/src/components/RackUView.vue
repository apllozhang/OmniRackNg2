<template>
  <div v-if="layout" class="rack-u-view">
    <div class="rack-title">
      <div>
        <strong>{{ layout.rack.name }}</strong>
        <span>{{ layout.rack.code }} · {{ layout.rack.uHeight }}U</span>
      </div>
      <el-tag :type="rackStatusType(layout.rack.status)">{{ rackStatusLabel(layout.rack.status) }}</el-tag>
    </div>
    <section class="u-panel">
      <header><strong>统一 U 位</strong><span>空闲 {{ freeCount() }}U</span></header>
      <div class="u-grid">
        <div v-for="u in uNumbers" :key="u" class="u-row">
          <span class="u-number">{{ u }}U</span>
          <div class="u-cell" :class="{ occupied: occupant(u), start: isDisplayStart(u) }" :style="deviceStyle(occupant(u)?.deviceId)" :title="cellTitle(u)">
            <template v-if="isDisplayStart(u)">
              <strong>{{ occupant(u)?.device?.name ?? '设备' }}</strong>
              <small>{{ positionLabel(occupant(u)) }}</small>
            </template>
          </div>
        </div>
      </div>
    </section>
    <div class="legend"><span><i class="empty-dot"></i>空闲 U 位</span><span><i class="device-dot"></i>已占用</span><span>机柜位置统一按 U 位区间管理</span></div>
  </div>
  <el-empty v-else description="请选择机柜查看 U 位" />
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { RackDevicePosition, RackULayout } from '@/types/device'

const props = defineProps<{ layout?: RackULayout | null }>()
const colors = ['#409eff', '#67c23a', '#e6a23c', '#9b59b6', '#00a6a6', '#f56c6c', '#5b8ff9']
const uNumbers = computed(() => {
  const height = props.layout?.rack.uHeight ?? 0
  return Array.from({ length: height }, (_, index) => height - index)
})
function positions() { return props.layout?.positions ?? [] }
function occupant(u: number): RackDevicePosition | undefined { return positions().find((item) => item.startU <= u && item.endU >= u) }
function isDisplayStart(u: number) { const item = occupant(u); return item?.endU === u }
function freeCount() { return (props.layout?.free ?? []).reduce((sum, range) => sum + range.heightU, 0) }
function positionLabel(item?: RackDevicePosition) { return item ? `${item.startU}-${item.endU}U · ${item.heightU}U` : '' }
function cellTitle(u: number) { const item = occupant(u); return item ? `${item.device?.name ?? item.deviceId}：${positionLabel(item)}` : `${u}U 空闲` }
function deviceStyle(deviceId?: string) {
  if (!deviceId) return undefined
  let hash = 0
  for (const char of deviceId) hash = (hash * 31 + char.charCodeAt(0)) >>> 0
  return { '--device-color': colors[hash % colors.length] }
}
function rackStatusLabel(status: string) { return ({ AVAILABLE: '空闲', PARTIAL: '部分占用', FULL: '已满', PLANNING: '规划中', MAINTENANCE: '维护中', DISABLED: '已停用' } as Record<string, string>)[status] ?? status }
function rackStatusType(status: string) { if (status === 'AVAILABLE') return 'success'; if (status === 'FULL' || status === 'DISABLED') return 'danger'; if (status === 'PARTIAL') return 'warning'; return 'info' }
</script>

<style scoped>
.rack-u-view { min-width: 720px; }
.rack-title { display:flex; justify-content:space-between; align-items:center; margin-bottom:14px; }
.rack-title div { display:flex; gap:12px; align-items:baseline; }
.rack-title span { color:var(--color-text-muted); font-size:13px; }
.u-panel { border:1px solid #dfe4ea; border-radius:10px; overflow:hidden; background:var(--color-surface); }
.u-panel header { display:flex; justify-content:space-between; padding:10px 12px; background:#172033; color:var(--color-surface); }
.u-panel header span { color:#cbd5e1; font-size:12px; }
.u-grid { padding:8px; background:#f3f4f6; }
.u-row { min-height:26px; display:grid; grid-template-columns:42px 1fr; align-items:stretch; }
.u-number { display:flex; align-items:center; justify-content:center; color:var(--color-text-muted); font-size:11px; border-bottom:1px solid #d1d5db; }
.u-cell { position:relative; display:flex; align-items:center; gap:8px; min-height:26px; padding:2px 8px; border:1px solid #d7dce2; border-top:0; background:var(--color-surface); color:var(--color-surface); overflow:hidden; }
.u-row:first-child .u-cell { border-top:1px solid #d7dce2; }
.u-cell.occupied { border-color:color-mix(in srgb, var(--device-color) 76%, #172033); background:color-mix(in srgb, var(--device-color) 86%, white); }
.u-cell.start { min-height:30px; }
.u-cell strong { font-size:12px; white-space:nowrap; overflow:hidden; text-overflow:ellipsis; }
.u-cell small { margin-left:auto; white-space:nowrap; opacity:.9; }
.legend { display:flex; gap:18px; align-items:center; margin-top:12px; color:var(--color-text-muted); font-size:12px; }
.legend span { display:flex; align-items:center; gap:6px; }
.legend i { width:12px; height:12px; border-radius:3px; display:inline-block; border:1px solid #d1d5db; }
.empty-dot { background:var(--color-surface); }
.device-dot { background:var(--color-action); border-color:var(--color-action) !important; }
</style>

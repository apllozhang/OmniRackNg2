<template>
  <div class="device-detail-fields">
    <header class="device-detail-heading">
      <div>
        <small>{{ deviceTypeName }}</small>
        <h3>{{ device.name }}</h3>
        <p>{{ device.code }}</p>
      </div>
      <span :class="`status-${device.lifecycleStatus.toLowerCase()}`">{{ lifecycleLabel }}</span>
    </header>

    <section v-for="section in deviceFieldGroups" :key="section.title">
      <h4>{{ section.title }}</h4>
      <div class="device-detail-grid">
        <div v-for="key in section.keys" :key="key" :class="{ 'full-width': fullWidthFields.has(key) }">
          <span>{{ deviceFieldLabels[key] }}</span>
          <b>{{ displayValue(key) }}</b>
        </div>
      </div>
    </section>

    <section>
      <h4>上架与位置</h4>
      <div class="device-detail-grid">
        <div><span>当前位置</span><b>{{ positionText }}</b></div>
        <div><span>占用 U 位</span><b>{{ activePosition ? `${activePosition.startU}-${activePosition.endU}U` : '-' }}</b></div>
        <div><span>安装时间</span><b>{{ formatDateTime(activePosition?.installedAt) }}</b></div>
        <div><span>上架原因</span><b>{{ activePosition?.reason || '-' }}</b></div>
      </div>
    </section>

    <section>
      <h4>系统信息</h4>
      <div class="device-detail-grid">
        <div><span>创建时间</span><b>{{ formatDateTime(device.createdAt) }}</b></div>
        <div><span>更新时间</span><b>{{ formatDateTime(device.updatedAt) }}</b></div>
        <div><span>数据版本</span><b>{{ device.version }}</b></div>
        <div><span>设备 ID</span><b>{{ device.id }}</b></div>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { Device, RackDevicePosition } from '@/types/device'
import {
  deviceFieldGroups,
  deviceFieldLabels,
  deviceLifecycleOptions,
  type DeviceEditableFieldKey,
} from '@/features/device/deviceForm'

const props = defineProps<{
  device: Device
  position?: RackDevicePosition | null
  rackName?: string
}>()

const activePosition = computed(() => props.position ?? props.device.currentPosition ?? null)
const deviceTypeName = computed(() => {
  const type = props.device.type
  if (!type) return props.device.typeId || '-'
  return type.name ? `${type.name}${type.code ? `（${type.code}）` : ''}` : type.code || props.device.typeId
})
const lifecycleLabel = computed(() => deviceLifecycleOptions.find((item) => item.value === props.device.lifecycleStatus)?.label ?? props.device.lifecycleStatus)
const positionText = computed(() => {
  const position = activePosition.value
  if (!position) return '未上架'
  const rack = props.rackName || position.rack?.name || position.rack?.code || position.rackId
  return `${rack} · ${position.startU}-${position.endU}U`
})

const fullWidthFields = new Set<DeviceEditableFieldKey>(['specification', 'externalQrCodeUrl', 'tags', 'remarks'])
const unitByField: Partial<Record<DeviceEditableFieldKey, string>> = {
  heightU: 'U',
  widthMm: ' mm',
  depthMm: ' mm',
  heightMm: ' mm',
  weightKg: ' kg',
  ratedPowerW: ' W',
  peakPowerW: ' W',
  inputVoltage: ' V',
}

function displayValue(key: DeviceEditableFieldKey) {
  if (key === 'typeId') return deviceTypeName.value
  if (key === 'lifecycleStatus') return lifecycleLabel.value
  if (key === 'dualPowerRequired') return props.device.dualPowerRequired ? '是' : '否'
  if (key === 'warrantyExpiresAt') return formatDate(props.device.warrantyExpiresAt)
  const value = props.device[key]
  if (value === undefined || value === null || value === '') return '-'
  const unit = unitByField[key] ?? ''
  return `${String(value)}${unit}`
}

function formatDate(value?: string) {
  if (!value) return '-'
  return value.match(/^\d{4}-\d{2}-\d{2}/)?.[0] ?? value
}

function formatDateTime(value?: string) {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString('zh-CN', { hour12: false })
}
</script>

<style scoped>
.device-detail-fields { color: #374151; }
.device-detail-heading { display: flex; justify-content: space-between; gap: 20px; padding: 4px 0 18px; border-bottom: 1px solid #e5e7eb; }
.device-detail-heading small { color: #6b7280; }
.device-detail-heading h3 { margin: 5px 0 2px; color: #1f2937; font-size: 22px; }
.device-detail-heading p { margin: 0; color: #9ca3af; font-family: Consolas, monospace; }
.device-detail-heading > span { align-self: flex-start; padding: 5px 10px; border-radius: 999px; color: #075985; background: #e0f2fe; font-size: 12px; white-space: nowrap; }
.device-detail-heading > span.status-running { color: #067647; background: #ecfdf3; }
.device-detail-heading > span.status-maintenance, .device-detail-heading > span.status-pending_removal { color: #92400e; background: #fffbeb; }
.device-detail-heading > span.status-off_rack, .device-detail-heading > span.status-scrapped { color: #991b1b; background: #fef2f2; }
.device-detail-fields section { padding-top: 20px; }
.device-detail-fields section + section { margin-top: 2px; }
.device-detail-fields h4 { margin: 0 0 12px; padding-bottom: 7px; border-bottom: 1px solid #eef0f3; color: #1f2937; font-size: 14px; }
.device-detail-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 16px 22px; }
.device-detail-grid > div { min-width: 0; }
.device-detail-grid .full-width { grid-column: 1 / -1; }
.device-detail-grid span { display: block; color: #9ca3af; font-size: 12px; }
.device-detail-grid b { display: block; margin-top: 5px; overflow-wrap: anywhere; color: #374151; font-size: 13px; line-height: 1.55; white-space: pre-wrap; }
@media (max-width: 640px) {
  .device-detail-grid { grid-template-columns: 1fr; }
  .device-detail-grid .full-width { grid-column: auto; }
}
</style>

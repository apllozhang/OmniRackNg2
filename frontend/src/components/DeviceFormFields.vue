<template>
  <div v-loading="loading" class="device-form-fields">
    <el-alert v-if="positioned" title="设备已上架：U 高度和位置相关状态受后端保护；如需调整 U 高度，请通过迁移功能重新校验机柜空间。" type="warning" :closable="false" show-icon />

    <section>
      <h4>基本信息</h4>
      <div class="device-form-grid">
        <el-form-item :label="deviceFieldLabels.typeId" required><el-select v-model="model.typeId" filterable style="width:100%"><el-option v-for="item in activeTypes" :key="item.id" :label="item.name" :value="item.id" /></el-select></el-form-item>
        <el-form-item :label="deviceFieldLabels.code" :required="mode === 'edit' || !autoCode"><div class="code-field"><el-input v-model="model.code" maxlength="80" :disabled="mode === 'create' && autoCode" :placeholder="mode === 'create' && autoCode ? '保存时自动生成' : '请输入设备编码'" /><el-checkbox v-if="mode === 'create'" v-model="autoCode">自动生成</el-checkbox></div></el-form-item>
        <el-form-item :label="deviceFieldLabels.name" required><el-input v-model="model.name" maxlength="150" /></el-form-item>
        <el-form-item v-if="isTerminal" label="使用状态"><el-select v-model="terminal.status" :disabled="mode==='edit'" style="width:100%"><el-option v-for="item in terminalStatuses" :key="item.value" :label="item.label" :value="item.value"/></el-select></el-form-item>
        <el-form-item v-else :label="deviceFieldLabels.lifecycleStatus"><el-select v-model="model.lifecycleStatus" style="width:100%"><el-option v-for="item in deviceLifecycleOptions" :key="item.value" :label="item.label" :value="item.value" :disabled="lifecycleDisabled(item.value)" /></el-select></el-form-item>
      </div>
    </section>

    <el-alert v-if="isTerminal && mode==='edit'" title="这里显示领用台账的实际状态；变更请在终端管理办理领用、归还、维修或报废，保留完整记录。" type="info" :closable="false"/>
    <section v-if="isTerminal && mode==='create'">
      <h4>终端初始使用信息</h4>
      <div class="device-form-grid">
        <template v-if="terminal.status==='IN_USE'"><el-form-item label="当前使用人" required><el-input v-model="terminal.assignee" maxlength="120"/></el-form-item><el-form-item label="使用部门" required><el-input v-model="terminal.department" maxlength="120"/></el-form-item></template>
        <el-form-item label="所在位置" :required="terminal.status==='IN_USE'"><el-select v-model="terminal.siteId" clearable @clear="terminal.siteId=null" filterable><el-option v-for="site in sites" :key="site.id" :label="site.region+' / '+site.name" :value="site.id"/></el-select></el-form-item>
        <el-form-item label="具体位置 / 工位"><el-input v-model="terminal.position" maxlength="160"/></el-form-item>
      </div>
      <p>使用中设备需登记使用人、部门和位置；设备档案、初始状态与登记记录一起保存。</p>
    </section>
    <section>
      <h4>资产与规格</h4>
      <div class="device-form-grid">
        <el-form-item :label="deviceFieldLabels.assetNumber"><el-input v-model="model.assetNumber" maxlength="100" /></el-form-item>
        <el-form-item :label="deviceFieldLabels.serialNumber"><el-input v-model="model.serialNumber" maxlength="120" /></el-form-item>
        <el-form-item :label="deviceFieldLabels.manufacturer"><el-input v-model="model.manufacturer" maxlength="100" /></el-form-item>
        <el-form-item :label="deviceFieldLabels.modelNumber"><el-input v-model="model.modelNumber" maxlength="100" /></el-form-item>
        <el-form-item :label="deviceFieldLabels.specification"><el-input v-model="model.specification" maxlength="500" /></el-form-item>
        <el-form-item :label="deviceFieldLabels.firmwareVersion"><el-input v-model="model.firmwareVersion" maxlength="100" /></el-form-item>
        <el-form-item :label="deviceFieldLabels.purchaseBatch"><el-input v-model="model.purchaseBatch" maxlength="100" /></el-form-item>
        <el-form-item :label="deviceFieldLabels.warrantyExpiresAt"><el-date-picker v-model="warrantyDate" type="date" value-format="YYYY-MM-DD" placeholder="选择日期" clearable style="width:100%" /></el-form-item>
      </div>
    </section>

    <section>
      <h4>尺寸与电力</h4>
      <div class="device-form-grid device-form-grid--three">
        <el-form-item v-if="!isTerminal" :label="deviceFieldLabels.heightU" required><el-input-number v-model="model.heightU" :min="1" :max="100" :disabled="positioned" controls-position="right" /></el-form-item>
        <el-form-item :label="deviceFieldLabels.widthMm"><el-input-number v-model="model.widthMm" :min="1" controls-position="right" /></el-form-item>
        <el-form-item :label="deviceFieldLabels.depthMm"><el-input-number v-model="model.depthMm" :min="1" controls-position="right" /></el-form-item>
        <el-form-item :label="deviceFieldLabels.heightMm"><el-input-number v-model="model.heightMm" :min="1" controls-position="right" /></el-form-item>
        <el-form-item :label="deviceFieldLabels.weightKg"><el-input-number v-model="model.weightKg" :min="0" :precision="2" controls-position="right" /></el-form-item>
        <el-form-item :label="deviceFieldLabels.ratedPowerW"><el-input-number v-model="model.ratedPowerW" :min="0" :precision="2" controls-position="right" /></el-form-item>
        <el-form-item :label="deviceFieldLabels.peakPowerW"><el-input-number v-model="model.peakPowerW" :min="0" :precision="2" controls-position="right" /></el-form-item>
        <el-form-item :label="deviceFieldLabels.inputVoltage"><el-input-number v-model="model.inputVoltage" :min="0" :precision="2" controls-position="right" /></el-form-item>
        <el-form-item v-if="!isTerminal" :label="deviceFieldLabels.dualPowerRequired"><el-switch v-model="model.dualPowerRequired" /></el-form-item>
      </div>
    </section>

    <section>
      <h4>组织与业务</h4>
      <div class="device-form-grid">
        <el-form-item :label="deviceFieldLabels.organization"><el-input v-model="model.organization" maxlength="150" /></el-form-item>
        <el-form-item :label="deviceFieldLabels.manager"><el-input v-model="model.manager" maxlength="100" /></el-form-item>
        <el-form-item :label="deviceFieldLabels.contact"><el-input v-model="model.contact" maxlength="100" /></el-form-item>
        <el-form-item :label="deviceFieldLabels.businessSystem"><el-input v-model="model.businessSystem" maxlength="150" /></el-form-item>
        <el-form-item :label="deviceFieldLabels.applicationName"><el-input v-model="model.applicationName" maxlength="150" /></el-form-item>
      </div>
    </section>

    <section>
      <h4>网络与管理</h4>
      <div class="device-form-grid">
        <el-form-item :label="deviceFieldLabels.managementIp"><el-input v-model="model.managementIp" maxlength="64" /></el-form-item>
        <el-form-item :label="deviceFieldLabels.businessIp"><el-input v-model="model.businessIp" maxlength="64" /></el-form-item>
        <el-form-item :label="deviceFieldLabels.macAddress"><el-input v-model="model.macAddress" maxlength="64" /></el-form-item>
        <el-form-item :label="deviceFieldLabels.managementProtocol"><el-input v-model="model.managementProtocol" maxlength="100" placeholder="如 SSH、SNMP、HTTPS" /></el-form-item>
        <el-form-item :label="deviceFieldLabels.monitoringStatus"><el-input v-model="model.monitoringStatus" maxlength="50" /></el-form-item>
      </div>
    </section>

    <section>
      <h4>二维码与标签</h4>
      <DeviceQrBinding v-model="model" :device-id="deviceId" />
    </section>
    <section>
      <h4>扩展信息</h4>
      <div class="device-form-grid">
        <el-form-item :label="deviceFieldLabels.tags" class="full-width"><el-input v-model="model.tags" maxlength="500" placeholder="多个标签可使用逗号分隔" /></el-form-item>
        <el-form-item :label="deviceFieldLabels.remarks" class="full-width"><el-input v-model="model.remarks" type="textarea" :rows="3" /></el-form-item>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, watch } from 'vue'
import type { TerminalRegistration } from '@/api/device'
import DeviceQrBinding from '@/components/DeviceQrBinding.vue'
import type { DeviceLifecycleStatus, DevicePayload, DeviceType } from '@/types/device'
import { deviceFieldLabels, deviceLifecycleOptions } from '@/features/device/deviceForm'

const props = withDefaults(defineProps<{
  deviceId?: string
  types: DeviceType[]
  mode?: 'create' | 'edit'
  loading?: boolean
  terminalMode?: boolean
  sites?: Array<{id:string;name:string;region:string}>
  positioned?: boolean
}>(), {
  terminalMode: false,
  sites: () => [],
  mode: 'edit',
  loading: false,
  positioned: false,
})

const model = defineModel<DevicePayload>({ required: true })
const terminal = defineModel<TerminalRegistration>('terminal', { default: () => ({ status: 'IDLE', assignee: '', department: '', siteId: null, position: '' }) })
const isTerminal = computed(() => props.terminalMode || props.types.find(t => t.id === model.value.typeId)?.category === 'PC')
const terminalStatuses = [{ value: 'IDLE', label: '在库' }, { value: 'IN_USE', label: '使用中' }, { value: 'REPAIR', label: '维修中' }, { value: 'RETIRED', label: '已报废' }]
watch(() => terminal.value.status, status => { if (props.mode==='create' && status!=='IN_USE') { terminal.value.assignee=''; terminal.value.department='' } })
const autoCode = defineModel<boolean>('autoCode', { default: false })
const activeTypes = computed(() => props.types.filter((item) => item.status === 'ACTIVE' || item.id === model.value.typeId))
const warrantyDate = computed({
  get: () => model.value.warrantyExpiresAt?.slice(0, 10) ?? '',
  set: (value: string) => { model.value.warrantyExpiresAt = value || undefined },
})

function lifecycleDisabled(value: DeviceLifecycleStatus) {
  if (props.positioned) return ['WAITING_RACK', 'OFF_RACK', 'SCRAPPED'].includes(value)
  return value === 'RUNNING'
}
</script>

<style scoped>
.device-form-fields { min-height: 120px; }
.device-form-fields > .el-alert { margin-bottom: 14px; }
.device-form-fields section + section { margin-top: 18px; }
.device-form-fields h4 { margin: 0 0 12px; padding-bottom: 7px; border-bottom: 1px solid var(--el-border-color-lighter); color: var(--el-text-color-primary); font-size: 14px; }
.device-form-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 0 18px; }
.device-form-grid--three { grid-template-columns: repeat(3, minmax(0, 1fr)); }
.device-form-grid :deep(.el-form-item) { min-width: 0; }
.device-form-grid :deep(.el-input-number) { width: 100%; }
.full-width { grid-column: 1 / -1; }
.code-field { display: flex; align-items: center; gap: 10px; width: 100%; }
.code-field .el-input { flex: 1; }
.code-field .el-checkbox { flex: none; margin-right: 0; }
@media (max-width: 820px) {
  .device-form-grid, .device-form-grid--three { grid-template-columns: 1fr; }
  .full-width { grid-column: auto; }
}
</style>

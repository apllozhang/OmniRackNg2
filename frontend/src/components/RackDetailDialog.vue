<template>
  <el-dialog
    :model-value="modelValue"
    title="机柜详情与容量分析"
    width="min(1420px, 96vw)"
    top="3vh"
    class="rack-capacity-dialog"
    append-to-body
    destroy-on-close
    @update:model-value="emit('update:modelValue', $event)"
  >
    <template #header>
      <div class="dialog-title">
        <div>
          <small>RACK DETAIL & CAPACITY ANALYSIS</small>
          <h2>机柜详情与容量分析</h2>
        </div>
        <span v-if="rack" class="status-pill" :style="{ '--status-color': statusColor(rack.status) }">{{ statusLabel(rack.status) }}</span>
      </div>
    </template>

    <div v-if="rack" class="rack-detail-shell">
      <section class="rack-visual-column">
        <div class="rack-identity">
          <div><small>{{ rack.code }}</small><h3>{{ rack.name }}</h3></div>
          <span>{{ rack.uHeight }}U</span>
        </div>
        <p class="rack-location">{{ roomName || '当前机房' }} · {{ rackPosition(rack) }}</p>

        <div class="rack-diagram-panel">
          <div class="rack-diagram-toolbar">
            <span>完整 U 位图</span>
            <b v-if="analysis">{{ analysis.installedDeviceCount }} 台设备</b>
          </div>
          <div v-if="layoutState === 'loading'" class="rack-diagram-state"><span class="loading-ring"></span>正在加载 U 位布局…</div>
          <div v-else-if="layoutState === 'error'" class="rack-diagram-state error">U 位布局读取失败，左侧机柜图暂时无法展示</div>
          <div v-else-if="layout" class="rack-frame">
            <div class="rack-frame-top"><i></i><span>U 位立面图</span><i></i></div>
            <div class="rack-u-stage" :style="rackStageStyle">
              <div v-for="u in uNumbers" :key="u" class="rack-u-line" :style="uLineStyle(u)">
                <span>{{ u }}</span><i></i>
              </div>
              <div
                v-for="position in layout.positions"
                :key="position.id"
                class="rack-device-block"
                :class="deviceCategoryClass(position)"
                :style="deviceBlockStyle(position)"
                :title="deviceTitle(position)"
              >
                <strong>{{ position.device?.name || position.device?.code || position.deviceId }}</strong>
                <span>{{ deviceSupplement(position) }}</span>
                <small>{{ positionRange(position) }}</small>
              </div>
            </div>
            <div class="rack-frame-bottom"><i></i><i></i></div>
          </div>
          <div v-else class="rack-diagram-state">尚未读取 U 位布局</div>
          <div class="rack-legend"><span><i class="empty"></i>空闲 U 位</span><span><i class="occupied"></i>设备占用</span><span>U 位从上到下递减</span></div>
        </div>
      </section>

      <section class="rack-information-column">
        <div class="info-section">
          <div class="section-heading"><div><small>BASIC PARAMETERS</small><h3>机柜参数</h3></div><span>{{ rack.templateName || '独立规格' }}</span></div>
          <div class="parameter-grid">
            <div><span>机柜编码</span><b>{{ rack.code }}</b></div>
            <div><span>机柜名称</span><b>{{ rack.name }}</b></div>
            <div><span>类型</span><b>{{ rack.type || '-' }}</b></div>
            <div><span>尺寸（宽×深×高）</span><b>{{ dimensionText }}</b></div>
            <div><span>厂商 / 型号</span><b>{{ joinValues(rack.manufacturer, rack.modelNumber) }}</b></div>
            <div><span>序列号 / 资产编号</span><b>{{ joinValues(rack.serialNumber, rack.assetNumber) }}</b></div>
            <div><span>区域 / 排 / 列</span><b>{{ joinValues(rack.zone, rack.row, rack.column) }}</b></div>
            <div><span>通道 / 坐标</span><b>{{ aisleCoordinateText }}</b></div>
            <div><span>负责人 / 部门</span><b>{{ joinValues(rack.manager, rack.department) }}</b></div>
            <div><span>用途</span><b>{{ rack.purpose || '-' }}</b></div>
            <div><span>模板版本</span><b>{{ templateText }}</b></div>
            <div><span>旋转角度</span><b>{{ rack.rotation ?? 0 }}°</b></div>
          </div>
        </div>

        <div class="info-section capacity-section">
          <div class="section-heading">
            <div><small>CAPACITY UTILIZATION</small><h3>容量与利用率</h3></div>
            <span class="calculation-rule">缺少必需字段时不按 0 估算</span>
          </div>
          <div v-if="analysis" class="metric-grid">
            <article v-for="metric in analysis.metrics" :key="metric.key" class="metric-card" :class="rackMetricTone(metric)">
              <div class="metric-card-head"><span>{{ metric.label }}</span><b>{{ metric.status === 'ready' ? (metric.overCapacity ? '已超限' : '可计算') : '无法计算' }}</b></div>
              <div v-if="metric.status === 'ready'" class="metric-value"><strong>{{ metric.percentage }}%</strong><span>{{ formatRackMetricValue(metric.used, metric.unit) }} / {{ formatRackMetricValue(metric.capacity, metric.unit) }}</span></div>
              <div v-else class="metric-value unavailable-value"><strong>无法计算</strong><span v-if="metric.used !== null || metric.capacity !== null">已知值：{{ formatRackMetricValue(metric.used, metric.unit) }} / {{ formatRackMetricValue(metric.capacity, metric.unit) }}</span></div>
              <div v-if="metric.status === 'ready'" class="metric-progress"><i :style="{ width: progressWidth(metric.percentage) }"></i></div>
              <ul v-if="metric.reasons.length" class="metric-reasons"><li v-for="reason in metric.reasons" :key="reason">{{ reason }}</li></ul>
            </article>
          </div>
        </div>

        <div class="info-section power-section">
          <div class="section-heading"><div><small>POWER & PDU</small><h3>PDU 与供电信息</h3></div><span>容量数据完整后自动计算</span></div>
          <div class="power-grid">
            <div><span>PDU 数量</span><b>{{ positiveValue(rack.pduCount, '台') }}</b></div>
            <div><span>输入回路</span><b>{{ positiveValue(rack.inputCircuits, '路') }}</b></div>
            <div><span>机柜双路供电</span><b>{{ rack.dualPower ? '是' : '否' }}</b></div>
            <div><span>双路供电设备</span><b>{{ analysis?.dualPowerDeviceCount ?? 0 }} 台</b></div>
            <div><span>额定电压</span><b>{{ positiveValue(rack.ratedVoltage, 'V') }}</b></div>
            <div><span>额定电流</span><b>{{ positiveValue(rack.ratedCurrent, 'A') }}</b></div>
            <div><span>机柜额定功率</span><b>{{ positiveValue(rack.ratedPowerKw, 'kW') }}</b></div>
            <div><span>机柜峰值功率</span><b>{{ positiveValue(rack.peakPowerKw, 'kW') }}</b></div>
          </div>
          <div class="pdu-notice"><b>PDU 利用率说明</b><p>当前模型仅记录 PDU 数量和输入回路，未记录每台 PDU 的插座总数、已占用插座及实际电流，因此系统会明确显示“无法计算”，不会生成可能误导的百分比。</p></div>
        </div>

        <PDUManager :rack-id="rack.id" :positions="layout?.positions" />        <div class="info-section remarks-section"><div class="section-heading"><div><small>REMARKS</small><h3>备注</h3></div></div><p>{{ rack.remarks || '暂无备注' }}</p></div>
      </section>
    </div>

    <template #footer>
      <div class="dialog-actions">
        <el-button @click="emit('update:modelValue', false)">关闭</el-button>
        <el-button v-if="rack" @click="emit('edit', rack)">编辑机柜</el-button>
        <el-button v-if="rack" type="danger" @click="emit('remove', rack)">删除机柜</el-button>
      </div>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { RackDevicePosition, RackULayout } from '@/types/device'
import type { Rack } from '@/types/resource'
import { analyzeRackCapacity, formatRackMetricValue, rackMetricTone, type RackLayoutLoadState } from '@/features/rack/rackCapacity'
import PDUManager from '@/components/PDUManager.vue'

const props = defineProps<{
  modelValue: boolean
  rack: Rack | null
  layout?: RackULayout | null
  layoutState?: RackLayoutLoadState
  roomName?: string
}>()
const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  edit: [rack: Rack]
  remove: [rack: Rack]
}>()

const unitHeight = computed(() => {
  const height = props.rack?.uHeight ?? 0
  if (!height) return 14
  return Math.max(13, Math.min(22, Math.floor(620 / height)))
})
const rackStageStyle = computed(() => ({ height: `${Math.max(1, props.rack?.uHeight ?? 1) * unitHeight.value}px` }))
const uNumbers = computed(() => Array.from({ length: props.rack?.uHeight ?? 0 }, (_, index) => (props.rack?.uHeight ?? 0) - index))
const analysis = computed(() => props.rack ? analyzeRackCapacity({ rack: props.rack, layout: props.layout, layoutState: props.layoutState }) : null)
const dimensionText = computed(() => props.rack ? `${props.rack.widthMm || '-'} × ${props.rack.depthMm || '-'} × ${props.rack.heightMm || '-'} mm` : '-')
const aisleCoordinateText = computed(() => {
  if (!props.rack) return '-'
  const coordinate = Number.isFinite(props.rack.xCoordinate) && Number.isFinite(props.rack.yCoordinate) ? `(${props.rack.xCoordinate}, ${props.rack.yCoordinate})` : ''
  return joinValues(props.rack.aisle, coordinate)
})
const templateText = computed(() => {
  if (!props.rack?.templateName && !props.rack?.templateCode) return '-'
  const revision = props.rack.templateRevision ? `V${props.rack.templateRevision}` : ''
  return joinValues(props.rack.templateName || props.rack.templateCode, revision)
})

function uLineStyle(u: number) { return { bottom: `${(u - 1) * unitHeight.value}px`, height: `${unitHeight.value}px` } }
function deviceBlockStyle(position: RackDevicePosition) {
  const safeStart = Math.max(1, position.startU)
  const safeHeight = Math.max(1, position.heightU || position.endU - position.startU + 1)
  return { bottom: `${(safeStart - 1) * unitHeight.value + 1}px`, height: `${safeHeight * unitHeight.value - 2}px` }
}
function deviceCategoryClass(position: RackDevicePosition) { return `category-${(position.device?.type?.category ?? 'OTHER').toLowerCase().replace(/_/g, '-')}` }
function deviceSupplement(position: RackDevicePosition) {
  const device = position.device
  if (!device) return '设备信息未加载'
  return [device.serialNumber && `SN ${device.serialNumber}`, device.ratedPowerW && `${device.ratedPowerW}W`].filter(Boolean).join(' · ') || device.code
}
function deviceTitle(position: RackDevicePosition) { return `${position.device?.name || position.deviceId} · ${positionRange(position)} · ${deviceSupplement(position)}` }
function positionRange(position: RackDevicePosition) { return position.startU === position.endU ? `${position.startU}U` : `${position.startU}-${position.endU}U` }
function progressWidth(percentage: number | null) { return `${Math.min(100, Math.max(0, percentage ?? 0))}%` }
function joinValues(...values: Array<string | number | undefined | null>) { return values.filter((value) => value !== undefined && value !== null && String(value).trim() !== '').join(' / ') || '-' }
function positiveValue(value: number | undefined, unit: string) { return typeof value === 'number' && Number.isFinite(value) && value > 0 ? `${value}${unit}` : '未填写' }
function rackPosition(rack: Rack) { return [rack.zone, rack.row && `${rack.row}排`, rack.column && `${rack.column}列`, rack.aisle].filter(Boolean).join(' · ') || '自动排布' }
function statusLabel(status: string) { return ({ AVAILABLE: '空闲', PARTIAL: '部分使用', FULL: '已满', PLANNING: '规划中', MAINTENANCE: '维护中', DISABLED: '停用' } as Record<string, string>)[status] ?? status }
function statusColor(status: string) { return ({ AVAILABLE: '#24d6a1', PARTIAL: '#2dc8f0', FULL: '#ffb547', PLANNING: '#7188a8', MAINTENANCE: '#a78bfa', DISABLED: '#ef6470' } as Record<string, string>)[status] ?? '#7bdfff' }
</script>

<style scoped>
.dialog-title { display:flex; align-items:center; justify-content:space-between; gap:18px; padding-right:34px; }
.dialog-title small, .section-heading small { color:#6991a8; font-size:10px; letter-spacing:1.2px; }
.dialog-title h2 { margin:4px 0 0; color:var(--el-text-color-primary); font-size:22px; }
.status-pill { align-self:center; padding:6px 13px; border:1px solid color-mix(in srgb, var(--status-color) 70%, transparent); border-radius:999px; color:var(--status-color); background:color-mix(in srgb, var(--status-color) 12%, white); font-size:12px; font-weight:700; }
:global(.rack-capacity-dialog) { display:flex; height:94vh; max-height:94vh; flex-direction:column; box-sizing:border-box; margin-bottom:0; overflow:hidden; }
:global(.rack-capacity-dialog .el-dialog__header), :global(.rack-capacity-dialog .el-dialog__footer) { flex:0 0 auto; }
:global(.rack-capacity-dialog .el-dialog__body) { flex:1 1 auto; min-height:0; padding-top:0; padding-bottom:0; overflow:hidden; }
:global(.rack-capacity-dialog .el-dialog__footer) { position:relative; z-index:3; border-top:1px solid #e5edf2; background:var(--color-surface); }
.rack-detail-shell { display:grid; height:100%; min-height:0; grid-template-columns:minmax(350px, 430px) minmax(0, 1fr); gap:22px; overflow:hidden; color:var(--el-text-color-primary); }
.rack-visual-column, .rack-information-column { min-width:0; }
.rack-visual-column { display:flex; flex-direction:column; padding:18px; border:1px solid #164968; border-radius:12px; background:linear-gradient(180deg, #071f35, #051726); box-shadow:inset 0 0 28px rgba(18,123,173,.12); }
.rack-identity { display:flex; align-items:end; justify-content:space-between; gap:12px; }
.rack-identity small { color:#4a7c9a; font-family:Consolas, monospace; }
.rack-identity h3 { margin:4px 0 0; color:#d9f4ff; font-size:22px; }
.rack-identity > span { color:#40d7ff; font-size:18px; font-weight:800; }
.rack-location { margin:7px 0 14px; color:#6f9bb4; font-size:12px; }
.rack-diagram-panel { display:flex; flex:1; min-height:0; flex-direction:column; padding:10px; border:1px solid #123d59; border-radius:8px; background:#041321; overflow:auto; }
.rack-diagram-toolbar { display:flex; justify-content:space-between; padding:0 4px 9px; color:#8dc9e6; font-size:12px; }.rack-diagram-toolbar b { color:#36d5ff; }
.rack-frame { width:min(100%, 340px); margin:auto; padding:8px 9px 10px; border:2px solid #375e76; border-radius:4px; background:#0b1c2a; box-shadow:0 0 20px rgba(28,151,204,.13); }
.rack-frame-top, .rack-frame-bottom { display:flex; align-items:center; justify-content:space-between; height:18px; color:#7493a5; font-size:9px; letter-spacing:2px; }.rack-frame-top i, .rack-frame-bottom i { width:7px; height:7px; border:1px solid #6f8795; border-radius:50%; background:#07131c; }
.rack-u-stage { position:relative; border:1px solid #29495c; background:repeating-linear-gradient(90deg, rgba(23,73,101,.12) 0, rgba(23,73,101,.12) 1px, transparent 1px, transparent 18px); overflow:hidden; }
.rack-u-line { position:absolute; right:0; left:0; display:grid; grid-template-columns:34px 1fr; box-sizing:border-box; border-top:1px solid rgba(80,126,151,.28); }
.rack-u-line span { display:flex; align-items:center; justify-content:center; border-right:1px solid #29495c; color:#55798e; font:9px Consolas, monospace; }.rack-u-line i { margin:3px 5px; border:1px solid rgba(50,94,119,.42); background:rgba(6,27,42,.48); }
.rack-device-block { position:absolute; right:7px; left:42px; z-index:2; display:flex; align-items:flex-start; flex-direction:column; justify-content:center; box-sizing:border-box; padding:2px 34px 2px 8px; border:1px solid #3f98d9; border-radius:2px; color:#effaff; background:linear-gradient(90deg, #196bb3, #2489d5); box-shadow:inset 3px 0 rgba(255,255,255,.13); overflow:hidden; }
.rack-device-block strong, .rack-device-block span { max-width:100%; white-space:nowrap; overflow:hidden; text-overflow:ellipsis; }.rack-device-block strong { font-size:10px; }.rack-device-block span { color:#cbeaff; font-size:8px; }.rack-device-block small { position:absolute; right:5px; bottom:3px; font-size:8px; opacity:.88; }
.rack-device-block.category-network { border-color:#31d7b4; background:linear-gradient(90deg, #128e78, #1fb798); }.rack-device-block.category-storage { border-color:#bd8cff; background:linear-gradient(90deg, #7440b8, #9960da); }.rack-device-block.category-security { border-color:#ffb15c; background:linear-gradient(90deg, #bb6a22, #e88c35); }.rack-device-block.category-power-environment { border-color:#ff7f8a; background:linear-gradient(90deg, #a93e4b, #d75b67); }.rack-device-block.category-other, .rack-device-block.category-accessory { border-color:#83a6bc; background:linear-gradient(90deg, #456a82, #618ba5); }
.rack-frame-bottom { height:12px; align-items:end; }.rack-legend { display:flex; flex-wrap:wrap; gap:12px; padding:10px 3px 0; color:#628ba2; font-size:10px; }.rack-legend span { display:flex; align-items:center; gap:5px; }.rack-legend i { width:9px; height:9px; border:1px solid #31546a; }.rack-legend .empty { background:#0a1b28; }.rack-legend .occupied { border-color:#2e91d6; background:#237fc4; }
.rack-diagram-state { display:flex; flex:1; min-height:360px; align-items:center; justify-content:center; gap:9px; color:#6d99b2; font-size:12px; text-align:center; }.rack-diagram-state.error { color:#ff929b; }.loading-ring { width:16px; height:16px; border:2px solid rgba(45,200,240,.24); border-top-color:#2dc8f0; border-radius:50%; animation:rack-detail-spin .8s linear infinite; }
.rack-information-column { display:flex; min-height:0; flex-direction:column; gap:14px; padding:0 8px 14px 0; overflow:auto; overscroll-behavior:contain; scrollbar-gutter:stable; }
.info-section { padding:16px 18px; border:1px solid #dbe7ee; border-radius:10px; background:#fbfdff; box-shadow:0 5px 18px rgba(35,72,96,.05); }
.section-heading { display:flex; align-items:start; justify-content:space-between; gap:14px; margin-bottom:13px; }.section-heading h3 { margin:3px 0 0; color:#19384e; font-size:16px; }.section-heading > span { color:#7890a0; font-size:11px; }.calculation-rule { padding:4px 8px; border-radius:4px; color:#a06614 !important; background:#fff6df; }
.parameter-grid, .power-grid { display:grid; grid-template-columns:repeat(4, minmax(0, 1fr)); gap:11px; }.parameter-grid > div, .power-grid > div { min-width:0; padding:10px; border:1px solid #edf2f5; border-radius:6px; background:var(--color-surface); }.parameter-grid span, .power-grid span { display:block; color:#8a9aa5; font-size:10px; }.parameter-grid b, .power-grid b { display:block; margin-top:5px; color:var(--el-text-color-primary); font-size:12px; overflow-wrap:anywhere; }
.metric-grid { display:grid; grid-template-columns:repeat(2, minmax(0, 1fr)); gap:11px; }.metric-card { --metric-color:#19a879; min-width:0; padding:13px; border:1px solid color-mix(in srgb, var(--metric-color) 30%, #e5edf2); border-radius:8px; background:linear-gradient(145deg, color-mix(in srgb, var(--metric-color) 5%, white), white); }.metric-card.warning { --metric-color:#d78a17; }.metric-card.danger { --metric-color:#dc4554; }.metric-card.unavailable { --metric-color:#a87620; background:#fffaf0; }.metric-card-head { display:flex; justify-content:space-between; gap:10px; }.metric-card-head span { color:#536b7a; font-size:11px; font-weight:700; }.metric-card-head b { color:var(--metric-color); font-size:10px; }.metric-value { display:flex; align-items:end; justify-content:space-between; gap:10px; margin-top:8px; }.metric-value strong { color:var(--metric-color); font-size:24px; line-height:1; }.metric-value span { color:#718692; font-size:10px; text-align:right; }.unavailable-value strong { font-size:17px; }.metric-progress { height:5px; margin-top:9px; border-radius:99px; background:#e8eff2; overflow:hidden; }.metric-progress i { display:block; height:100%; border-radius:inherit; background:var(--metric-color); }.metric-reasons { margin:9px 0 0; padding:8px 8px 8px 23px; border-radius:5px; color:#8a641f; background:rgba(255,241,207,.7); font-size:10px; line-height:1.55; }
.power-grid { grid-template-columns:repeat(4, minmax(0, 1fr)); }.pdu-notice { margin-top:11px; padding:10px 12px; border-left:3px solid #e8a63b; color:#6d5a34; background:#fff8e8; }.pdu-notice b { font-size:11px; }.pdu-notice p { margin:4px 0 0; font-size:10px; line-height:1.55; }.remarks-section p { margin:0; color:#5b7180; font-size:12px; line-height:1.7; white-space:pre-wrap; }.dialog-actions { display:flex;flex-wrap:wrap; justify-content:flex-end; gap:8px; }
@keyframes rack-detail-spin { to { transform:rotate(360deg); } }
@media (max-width:1100px) { .rack-detail-shell { grid-template-columns:340px minmax(0, 1fr); }.parameter-grid, .power-grid { grid-template-columns:repeat(2, minmax(0, 1fr)); } }
@media (max-width:760px) { :global(.rack-capacity-dialog) { height:96vh; max-height:96vh; }.rack-detail-shell { display:block; height:100%; overflow:auto; }.rack-visual-column { min-height:620px; margin-bottom:14px; }.rack-information-column { padding-right:0; overflow:visible; }.metric-grid, .parameter-grid, .power-grid { grid-template-columns:1fr; } }
</style>

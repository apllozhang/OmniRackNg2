<template>
  <el-dialog
    :model-value="modelValue"
    title="导入机柜图"
    width="min(1180px, 94vw)"
    append-to-body
    destroy-on-close
    :close-on-click-modal="!busy" :close-on-press-escape="!busy" :show-close="!busy"
    class="rack-diagram-import-dialog"
    @close="closeDialog"
  >
    <div class="import-guide">
      <strong>仅支持本页面“导出机柜图”生成的 .xlsx 文件</strong>
      <span>可以直接回导，也可以在 Excel 中增删、改名或移动设备。预校验阶段不会写入数据；新增设备的编号在正式提交时自动生成。</span>
    </div>

    <div class="import-controls">
      <el-form-item label="新增设备默认类型" required>
        <el-select v-model="defaultTypeId" placeholder="请选择设备类型" filterable :disabled="busy">
          <el-option
            v-for="item in activeDeviceTypes"
            :key="item.id"
            :label="item.name + '（' + item.code + '）'"
            :value="item.id"
          />
        </el-select>
      </el-form-item>
      <div class="file-control">
        <input ref="fileInput" class="native-file-input" type="file" accept=".xlsx" :disabled="busy" @change="selectFile">
        <el-button :disabled="busy" @click="fileInput?.click()">选择机柜图</el-button>
        <span :class="{ muted: !fileName }">{{ fileName || '尚未选择文件' }}</span>
      </div>
      <el-button type="primary" :loading="validating" :disabled="!parsed || !defaultTypeId || busy" @click="validateImport">
        解析并校验
      </el-button>
    </div>

    <el-alert
      v-if="parseError"
      :title="parseError"
      type="error"
      :closable="false"
      show-icon
      class="import-alert"
    />

    <template v-if="validation">
      <div class="validation-summary">
        <div><span>总项数</span><strong>{{ validation.summary.total }}</strong></div>
        <div class="success"><span>保持不变</span><strong>{{ validation.summary.unchanged }}</strong></div>
        <div class="success"><span>新增设备</span><strong>{{ validation.summary.create }}</strong></div>
        <div><span>位置迁移</span><strong>{{ validation.summary.move }}</strong></div>
        <div class="warning"><span>待人工确认</span><strong>{{ validation.summary.decisions }}</strong></div>
        <div class="warning"><span>删除确认</span><strong>{{ validation.summary.removals }}</strong></div>
        <div class="danger"><span>错误</span><strong>{{ validation.summary.errors }}</strong></div>
      </div>

      <el-alert
        v-if="validation.summary.errors"
        title="存在阻断错误，请修改 Excel 后重新选择并校验。"
        type="error"
        :closable="false"
        show-icon
        class="import-alert"
      />
      <el-alert
        v-else-if="unresolvedCount"
        :title="'还有 ' + unresolvedCount + ' 项需要确认。所有待确认项处理完成后才可正式导入。'"
        type="warning"
        :closable="false"
        show-icon
        class="import-alert"
      />
      <el-alert
        v-else
        title="校验通过。请确认下方变更内容后执行正式导入。"
        type="success"
        :closable="false"
        show-icon
        class="import-alert"
      />

      <el-input v-model="search" clearable :disabled="busy" aria-label="搜索机柜图校验预览" placeholder="搜索机柜图校验预览" style="max-width:360px;margin:12px 0"/><el-table :data="pageRows" ref="tableRef" @sort-change="onSortChange" :aria-busy="busy||search!==query" border max-height="470" row-key="id" class="validation-table">
        <el-table-column label="状态" fixed="left" prop="kindLabel" sortable="custom" :width="columnWidths['kindLabel']"><template #header><TableSortHeader label="状态" :width="columnWidths['kindLabel']" @sort="sortBy('kindLabel')" @resize="resizeColumn('kindLabel',$event)"/></template>
          <template #default="scope">
            <el-tag :type="statusTagType(scope.row)" effect="dark">{{ statusLabel(scope.row.kind) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="目标位置" prop="positionLabel" sortable="custom" :width="columnWidths['positionLabel']"><template #header><TableSortHeader label="目标位置" :width="columnWidths['positionLabel']" @sort="sortBy('positionLabel')" @resize="resizeColumn('positionLabel',$event)"/></template>
          <template #default="scope">
            <span v-if="scope.row.rackCode">{{ scope.row.rackCode }} · {{ rangeLabel(scope.row) }}</span>
            <span v-else>-</span>
          </template>
        </el-table-column>
        <el-table-column label="导入设备" prop="name" sortable="custom" :width="columnWidths['name']"><template #header><TableSortHeader label="导入设备" :width="columnWidths['name']" @sort="sortBy('name')" @resize="resizeColumn('name',$event)"/></template>
          <template #default="scope">
            <strong class="device-name">{{ scope.row.name || '-' }}</strong>
            <small>{{ importDeviceMeta(scope.row) }}</small>
          </template>
        </el-table-column>
        <el-table-column label="匹配到的原设备" prop="existingDeviceName" sortable="custom" :width="columnWidths['existingDeviceName']"><template #header><TableSortHeader label="匹配到的原设备" :width="columnWidths['existingDeviceName']" @sort="sortBy('existingDeviceName')" @resize="resizeColumn('existingDeviceName',$event)"/></template>
          <template #default="scope">
            <template v-if="scope.row.existingDeviceId">
              <strong class="device-name">{{ scope.row.existingDeviceName || '-' }}</strong>
              <small>{{ scope.row.existingDeviceCode || '无设备编号' }}</small>
            </template>
            <span v-else>-</span>
          </template>
        </el-table-column>
        <el-table-column label="校验说明" show-overflow-tooltip  prop="message" sortable="custom" :width="columnWidths['message']"><template #header><TableSortHeader label="校验说明" :width="columnWidths['message']" @sort="sortBy('message')" @resize="resizeColumn('message',$event)"/></template></el-table-column>
        <el-table-column label="人工确认" width="235" fixed="right">
          <template #default="scope">
            <el-select
              v-if="scope.row.requiresDecision"
              v-model="decisions[scope.row.id]" :disabled="busy"
              placeholder="必须选择处理方式"
              size="small"
            >
              <el-option
                v-for="action in scope.row.allowedDecisions || []"
                :key="action"
                :label="decisionLabel(action, scope.row.kind)"
                :value="action"
              />
            </el-select>
            <span v-else class="no-decision">无需确认</span>
          </template>
        </el-table-column>
      </el-table><TablePager v-model:page="page" v-model:page-size="pageSize" :total="total" label="机柜图校验预览"/>
    </template>

    <template #footer>
      <div class="dialog-footer">
        <span class="footer-tip">{{ footerTip }}</span>
        <el-button :disabled="submitting" @click="closeDialog">取消</el-button>
        <el-button type="primary" :loading="submitting" :disabled="!canCommit" @click="commitImport">确认导入</el-button>
      </div>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import TableSortHeader from '@/components/TableSortHeader.vue'
import TablePager from '@/components/TablePager.vue'
import {useClientTable} from '@/features/table/useClientTable'
import {useColumnWidths} from '@/features/table/useColumnWidths'
import {useDebouncedValue} from '@/features/table/useTableView'

import { computed, nextTick, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { commitRackDiagramImport, validateRackDiagramImport } from '@/api/device'
import type { DeviceType } from '@/types/device'
import type {
  RackDiagramImportDecision,
  RackDiagramImportItem,
  RackDiagramImportItemKind,
  RackDiagramImportValidation,
} from '@/types/rackDiagramImport'
import { parseRackDiagramExcel } from '@/utils/rackDiagramImport'

const props = defineProps<{
  modelValue: boolean
  roomId: string
  dataCenterId: string
  deviceTypes: DeviceType[]
}>()
const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  success: []
}>()

const search=ref(''),query=useDebouncedValue(search)
const previewRows=computed(()=>(validation.value?.items||[]).filter(row=>!query.value.trim()||[row.name,row.existingDeviceName,row.rackCode,row.message].some(value=>String(value||'').toLowerCase().includes(query.value.trim().toLowerCase()))))
const fileInput = ref<HTMLInputElement | null>(null)
const fileName = ref('')
const parsed = ref<Awaited<ReturnType<typeof parseRackDiagramExcel>> | null>(null)
const parseError = ref('')
const defaultTypeId = ref('')
const validating = ref(false)
const submitting = ref(false)
const validation = ref<RackDiagramImportValidation | null>(null)
const decisions = ref<Record<string, RackDiagramImportDecision | undefined>>({})

const activeDeviceTypes = computed(() => props.deviceTypes.filter((item) => item.status === 'ACTIVE'))
const busy = computed(() => validating.value || submitting.value)
const unresolvedCount = computed(() => validation.value?.items.filter((item) => item.requiresDecision && !decisions.value[item.id]).length ?? 0)
const canCommit = computed(() => Boolean(
  validation.value?.token &&
  validation.value.summary.errors === 0 &&
  unresolvedCount.value === 0 &&
  !busy.value,
))
const footerTip = computed(() => {
  if (!validation.value) return '请先选择导出的机柜图并执行校验'
  if (validation.value.summary.errors) return '存在阻断错误，不能导入'
  if (unresolvedCount.value) return '请完成全部人工确认'
  return '提交时将再次检查数据版本，并在一个事务中完成全部变更'
})

watch(() => props.modelValue, (open, wasOpen) => {
  if (!open) return
  if (!wasOpen) resetState()
  defaultTypeId.value = activeDeviceTypes.value[0]?.id ?? ''
}, { immediate: true })
watch(() => props.deviceTypes, () => {
  if (props.modelValue && !activeDeviceTypes.value.some((item) => item.id === defaultTypeId.value)) {
    defaultTypeId.value = activeDeviceTypes.value[0]?.id ?? ''
  }
})

function resetState() {
  search.value='';query.value=''
  fileName.value = ''
  parsed.value = null
  parseError.value = ''
  validation.value = null
  decisions.value = {}
  if (fileInput.value) fileInput.value.value = ''
}
function closeDialog() {
  if (busy.value) return
  emit('update:modelValue', false)
}
async function selectFile(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  resetState()
  if (!file) return
  fileName.value = file.name
  try {
    parsed.value = await parseRackDiagramExcel(file, props.roomId, props.dataCenterId)
  } catch (error) {
    parseError.value = errorMessage(error)
  } finally {
    await nextTick()
    if (fileInput.value) fileInput.value.value = ''
  }
}
async function validateImport() {
  if (!parsed.value || !defaultTypeId.value) return
  validating.value = true
  validation.value = null
  decisions.value = {}
  try {
    validation.value = await validateRackDiagramImport(props.roomId, {
      ...parsed.value.payload,
      defaultTypeId: defaultTypeId.value,
    })
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    validating.value = false
  }
}
async function commitImport() {
  if (!validation.value || !canCommit.value) return
  const confirmText = buildConfirmText(validation.value)
  try {
    await ElMessageBox.confirm(confirmText, '确认导入机柜图', {
      confirmButtonText: '确认执行',
      cancelButtonText: '返回检查',
      type: 'warning',
      dangerouslyUseHTMLString: false,
    })
  } catch {
    return
  }
  submitting.value = true
  try {
    const result = await commitRackDiagramImport(props.roomId, {
      token: validation.value.token,
      decisions: validation.value.items
        .filter((item) => item.requiresDecision)
        .map((item) => ({ itemId: item.id, action: decisions.value[item.id]! })),
    })
    ElMessage.success('导入完成：新增 ' + result.created + '，更新 ' + result.updated + '，迁移 ' + result.moved + '，下架 ' + result.decommissioned)
    emit('update:modelValue', false)
    emit('success')
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    submitting.value = false
  }
}
function buildConfirmText(result: RackDiagramImportValidation) {
  return '本次将新增 ' + result.summary.create + ' 项、迁移 ' + result.summary.move + ' 项，并按人工选择处理 ' + result.summary.decisions + ' 个冲突和 ' + result.summary.removals + ' 个删除项。是否继续？'
}
function statusLabel(kind: RackDiagramImportItemKind) {
  return ({
    UNCHANGED: '保持不变', CREATE_NEW: '新增', MOVE_EXISTING: '迁移',
    NEEDS_DECISION: '待确认', REMOVE_MISSING: '删除确认', ERROR: '错误',
  } as Record<RackDiagramImportItemKind, string>)[kind]
}
function statusTagType(item: RackDiagramImportItem) {
  if (item.kind === 'ERROR') return 'danger'
  if (item.requiresDecision) return 'warning'
  if (item.kind === 'UNCHANGED' || item.kind === 'CREATE_NEW') return 'success'
  return 'info'
}
function rangeLabel(item: RackDiagramImportItem) {
  if (!item.startU || !item.endU) return '-'
  return item.startU === item.endU ? item.startU + 'U' : item.startU + '-' + item.endU + 'U'
}
function importDeviceMeta(item: RackDiagramImportItem) {
  const values = []
  if (item.serialNumber) values.push('SN: ' + item.serialNumber)
  if (item.ratedPowerW !== undefined) values.push('功耗: ' + item.ratedPowerW + 'W')
  return values.join(' · ') || '无 SN / 功耗信息'
}
function decisionLabel(action: RackDiagramImportDecision, kind: RackDiagramImportItemKind) {
  if (action === 'UPDATE_EXISTING') return '保留编号，修改原设备'
  if (action === 'CREATE_NEW') return '下架原设备，创建新设备'
  if (action === 'DECOMMISSION') return '确认下架该设备'
  if (action === 'IGNORE') return kind === 'REMOVE_MISSING' ? '忽略删除，保留原设备' : '忽略本项'
  return action
}
function errorMessage(error: unknown) {
  if (error instanceof Error) return error.message
  if (typeof error === 'object' && error) {
    const candidate = error as { response?: { data?: { message?: string } }; message?: string }
    return candidate.response?.data?.message || candidate.message || '操作失败'
  }
  return '操作失败'
}
const {page,pageSize,total,pageRows,tableRef,sortBy,onSortChange}=useClientTable(previewRows,{kindLabel:row=>statusLabel(row.kind),positionLabel:row=>(row.rackCode||'')+' '+rangeLabel(row)})
const {columnWidths,resizeColumn}=useColumnWidths({"kindLabel": 110, "positionLabel": 145, "name": 190, "existingDeviceName": 190, "message": 260})
</script>

<style scoped>
.import-guide { display: flex; flex-direction: column; gap: 7px; padding: 13px 15px; border: 1px solid #b9def0; border-radius: 6px; color: #315a72; background: #eef9ff; }
.import-guide strong { color: #155e83; }
.import-controls { display: grid; grid-template-columns: minmax(250px, 1fr) minmax(300px, 1.4fr) auto; align-items: end; gap: 14px; margin: 18px 0; }
.import-controls :deep(.el-form-item) { margin-bottom: 0; }
.import-controls :deep(.el-select) { width: 100%; }
.file-control { display: flex; align-items: center; gap: 10px; min-width: 0; }
.file-control span { overflow: hidden; color: var(--el-text-color-regular); text-overflow: ellipsis; white-space: nowrap; }
.file-control span.muted { color: #9aa5b1; }
.native-file-input { display: none; }
.import-alert { margin: 12px 0; }
.validation-summary { display: grid; grid-template-columns: repeat(7, 1fr); gap: 8px; margin: 14px 0; }
.validation-summary div { padding: 11px 8px; border: 1px solid #d9e4ec; border-radius: 5px; text-align: center; background: var(--el-fill-color-light); }
.validation-summary span { display: block; color: #718096; font-size: 12px; }
.validation-summary strong { display: block; margin-top: 3px; color: var(--el-text-color-primary); font-size: 20px; }
.validation-summary .success strong { color: #1a8f62; }
.validation-summary .warning strong { color: #d97706; }
.validation-summary .danger strong { color: #dc2626; }
.validation-table .device-name { display: block; color: var(--el-text-color-primary); }
.validation-table small { display: block; margin-top: 4px; color: #8492a6; }
.no-decision { color: #9aa5b1; font-size: 12px; }
.dialog-footer { display: flex; align-items: center; justify-content: flex-end; gap: 10px; }
.footer-tip { flex: 1; color: #7a8793; font-size: 12px; text-align: left; }
@media (max-width: 900px) {
  .import-controls { grid-template-columns: 1fr; align-items: stretch; }
  .validation-summary { grid-template-columns: repeat(2, 1fr); }
}
:global(.rack-diagram-import-dialog){display:flex;flex-direction:column;max-height:90vh;margin-top:5vh;max-width:95vw}
:global(.rack-diagram-import-dialog .el-dialog__body){min-height:0;overflow-y:auto}
@media(max-width:600px){.dialog-footer{flex-wrap:wrap}.footer-tip{flex-basis:100%}}
</style>

<template>
  <el-dialog class="batch-import-dialog" :model-value="modelValue" :title="title" width="980px" destroy-on-close :close-on-click-modal="!importing" :close-on-press-escape="!importing" :show-close="!importing" @update:model-value="emit('update:modelValue', $event)">
    <el-alert :title="description" type="info" :closable="false" show-icon class="import-alert" />
    <div class="import-toolbar">
      <el-button type="primary" plain @click="downloadTemplate">下载 Excel 导入模板</el-button>
      <el-upload action="#" accept=".xlsx,.xls" :auto-upload="false" :show-file-list="false" :disabled="parsing || importing" :on-change="handleFileChange">
        <el-button :loading="parsing">选择 Excel 文件</el-button>
      </el-upload>
      <span v-if="fileName" class="file-name">{{ fileName }}</span>
      <el-button v-if="rows.length" link type="primary" @click="downloadResult">导出校验/导入结果</el-button>
    </div>

    <el-row v-if="rows.length" :gutter="12" class="import-summary">
      <el-col :span="6"><el-card shadow="never"><span>总行数</span><strong>{{ rows.length }}</strong></el-card></el-col>
      <el-col :span="6"><el-card shadow="never"><span>可导入</span><strong class="success-text">{{ validCount }}</strong></el-card></el-col>
      <el-col :span="6"><el-card shadow="never"><span>校验错误</span><strong class="danger-text">{{ invalidCount }}</strong></el-card></el-col>
      <el-col :span="6"><el-card shadow="never"><span>已成功</span><strong class="success-text">{{ successCount }}</strong></el-card></el-col>
    </el-row>

    <el-empty v-if="!rows.length && !parsing" description="请下载模板填写后，选择 Excel 文件进行校验" />
    <template v-else>
      <el-alert title="分页和搜索仅影响预览；完整数据仍参与校验与导入。" type="info" :closable="false"/>
      <el-input v-model="search" clearable :disabled="importing||parsing" aria-label="搜索批量导入预览" placeholder="搜索批量导入预览" style="max-width:360px;margin:12px 0"/><el-table v-loading="parsing" :data="pageRows" ref="tableRef" @sort-change="onSortChange" :aria-busy="importing||parsing||search!==query" stripe height="420px">
        <el-table-column label="Excel 行"  prop="rowNumber" sortable="custom" :width="columnWidths['rowNumber']"><template #header><TableSortHeader label="Excel 行" :width="columnWidths['rowNumber']" @sort="sortBy('rowNumber')" @resize="resizeColumn('rowNumber',$event)"/></template></el-table-column>
                <el-table-column label="数据标识" show-overflow-tooltip  prop="summary" sortable="custom" :width="columnWidths['summary']"><template #header><TableSortHeader label="数据标识" :width="columnWidths['summary']" @sort="sortBy('summary')" @resize="resizeColumn('summary',$event)"/></template></el-table-column>
        <el-table-column label="操作" prop="operationLabel" sortable="custom" :width="columnWidths['operationLabel']"><template #header><TableSortHeader label="操作" :width="columnWidths['operationLabel']" @sort="sortBy('operationLabel')" @resize="resizeColumn('operationLabel',$event)"/></template>
          <template #default="{ row }"><el-tag :type="row.operation === 'UPDATE' ? 'warning' : 'success'">{{ row.operation === 'UPDATE' ? '更新' : '新增' }}</el-tag></template>
        </el-table-column>
        <el-table-column label="状态" prop="statusLabel" sortable="custom" :width="columnWidths['statusLabel']"><template #header><TableSortHeader label="状态" :width="columnWidths['statusLabel']" @sort="sortBy('statusLabel')" @resize="resizeColumn('statusLabel',$event)"/></template>
          <template #default="{ row }"><el-tag :type="statusType(row)">{{ statusLabel(row) }}</el-tag></template>
        </el-table-column>
        <el-table-column label="说明" show-overflow-tooltip prop="message" sortable="custom" :width="columnWidths['message']"><template #header><TableSortHeader label="说明" :width="columnWidths['message']" @sort="sortBy('message')" @resize="resizeColumn('message',$event)"/></template>
          <template #default="{ row }"><span :class="{ 'danger-text': row.errors.length || row.importError }">{{ row.errors.join('；') || row.importError || '校验通过，可以导入' }}</span></template>
        </el-table-column>
      </el-table><TablePager v-model:page="page" v-model:page-size="pageSize" :total="total" label="批量导入预览"/>
    </template>

    <template #footer>
      <el-button :disabled="importing" @click="emit('update:modelValue', false)">关闭</el-button>
      <el-button type="primary" :loading="importing" :disabled="!importableCount" @click="startImport">
        开始导入（{{ importableCount }} 条）
      </el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import TableSortHeader from '@/components/TableSortHeader.vue'
import TablePager from '@/components/TablePager.vue'
import {useClientTable} from '@/features/table/useClientTable'
import {useColumnWidths} from '@/features/table/useColumnWidths'
import {useDebouncedValue} from '@/features/table/useTableView'

import { computed, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import type { UploadFile } from 'element-plus'
import { downloadExcelTemplate, downloadImportResult, parseExcelFile } from '@/utils/excelImport'
import type { ExcelFieldDefinition, ParsedExcelRow, PreparedImportRow } from '@/utils/excelImport'

type RuntimeStatus = 'ready' | 'invalid' | 'importing' | 'success' | 'failed'
interface RuntimeRow extends PreparedImportRow {
  status: RuntimeStatus
  importError?: string
}

const props = defineProps<{
  modelValue: boolean
  title: string
  description: string
  templateFileName: string
  resultFileName: string
  subject: string
  fields: ExcelFieldDefinition[]
  prepareRows: (rows: ParsedExcelRow[]) => PreparedImportRow[]
  submitRow: (payload: unknown, row?: PreparedImportRow) => Promise<unknown>
}>()
const emit = defineEmits<{ 'update:modelValue': [value: boolean]; imported: [count: number] }>()

const rows = ref<RuntimeRow[]>([])
const fileName = ref('')
const parsing = ref(false)
const importing = ref(false)
const search=ref(''),query=useDebouncedValue(search)
const previewRows = computed(() => rows.value.filter(row=>!query.value.trim()||[row.rowNumber,row.summary,row.errors.join('；'),row.importError].some(value=>String(value||'').toLowerCase().includes(query.value.trim().toLowerCase()))))
const validCount = computed(() => rows.value.filter((row) => !row.errors.length).length)
const invalidCount = computed(() => rows.value.filter((row) => row.errors.length).length)
const successCount = computed(() => rows.value.filter((row) => row.status === 'success').length)
const importableCount = computed(() => rows.value.filter((row) => !row.errors.length && row.status !== 'success' && row.status !== 'importing').length)

watch(() => props.modelValue, (visible) => {
  if (visible) {
    rows.value = []
    fileName.value = ''
  }
})

function downloadTemplate() {
  downloadExcelTemplate(props.templateFileName, props.subject, props.fields)
}

async function handleFileChange(uploadFile: UploadFile) {
  const file = uploadFile.raw
  if (!file) return
  if (!/\.(xlsx|xls)$/i.test(file.name)) {
    ElMessage.warning('请选择 .xlsx 或 .xls 文件')
    return
  }
  parsing.value = true
  fileName.value = file.name
  try {
    const parsed = await parseExcelFile(file, props.fields)
    rows.value = props.prepareRows(parsed).map((row) => ({ ...row, status: row.errors.length ? 'invalid' : 'ready' }))
    if (!rows.value.length) ElMessage.warning('导入数据工作表中没有待导入数据')
    else if (invalidCount.value) ElMessage.warning(`已读取 ${rows.value.length} 行，其中 ${invalidCount.value} 行需要修正`)
    else ElMessage.success(`已读取并校验 ${rows.value.length} 行数据`)
  } catch (error) {
    rows.value = []
    ElMessage.error(error instanceof Error ? error.message : 'Excel 文件解析失败')
  } finally {
    parsing.value = false
  }
}

async function startImport() {
  const targets = rows.value.filter((row) => !row.errors.length && row.status !== 'success' && row.status !== 'importing')
  if (!targets.length) return
  importing.value = true
  let cursor = 0
  let importedNow = 0
  const workerCount = Math.min(5, targets.length)
  const workers = Array.from({ length: workerCount }, async () => {
    while (cursor < targets.length) {
      const row = targets[cursor++]
      row.status = 'importing'
      row.importError = ''
      try {
        await props.submitRow(row.payload, row)
        row.status = 'success'
        importedNow += 1
      } catch (error) {
        row.status = 'failed'
        const value = error as { response?: { data?: { message?: string } }; message?: string }
        row.importError = value.response?.data?.message ?? value.message ?? '提交失败'
      }
    }
  })
  await Promise.all(workers)
  importing.value = false
  if (importedNow) emit('imported', importedNow)
  const failed = targets.length - importedNow
  if (failed) ElMessage.warning(`导入完成：成功 ${importedNow} 条，失败 ${failed} 条，可导出结果查看原因并重试失败项`)
  else ElMessage.success(`成功导入 ${importedNow} 条数据`)
}

function downloadResult() {
  downloadImportResult(props.resultFileName, rows.value.map((row) => ({
    rowNumber: row.rowNumber,
    summary: row.summary,
    status: statusLabel(row),
    errors: row.errors.join('；') || row.importError || '',
  })))
}

function statusLabel(row: RuntimeRow) {
  if (row.status === 'success') return '导入成功'
  if (row.status === 'failed') return '导入失败'
  if (row.status === 'importing') return '导入中'
  if (row.errors.length) return '校验错误'
  return '待导入'
}

function statusType(row: RuntimeRow) {
  if (row.status === 'success') return 'success'
  if (row.status === 'failed' || row.errors.length) return 'danger'
  if (row.status === 'importing') return 'warning'
  return 'info'
}
const {page,pageSize,total,pageRows,tableRef,sortBy,onSortChange}=useClientTable(previewRows,{operationLabel:row=>row.operation==='UPDATE'?'更新':'新增',statusLabel:row=>statusLabel(row),message:row=>row.errors.join('；')||row.importError||'校验通过，可以导入'})
const {columnWidths,resizeColumn}=useColumnWidths({"rowNumber": 110, "summary": 260, "operationLabel": 110, "statusLabel": 110, "message": 420})
</script>

<style scoped>
.import-alert,.preview-alert{margin-bottom:14px}.import-toolbar{display:flex;align-items:center;gap:10px;flex-wrap:wrap;margin-bottom:14px}.file-name{max-width:320px;color:var(--el-text-color-secondary);overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.import-summary{margin-bottom:14px}.import-summary :deep(.el-card__body){display:flex;align-items:center;justify-content:space-between;padding:14px 16px}.import-summary span{color:var(--color-text-muted)}.import-summary strong{font-size:24px}.success-text{color:#16a34a}.danger-text{color:#dc2626}
@media(max-width:600px){.import-summary :deep(.el-col){flex:0 0 50%;max-width:50%;margin-bottom:8px}.import-summary :deep(.el-card__body){padding:10px;gap:8px}.import-summary strong{font-size:20px}}
:global(.batch-import-dialog){display:flex;flex-direction:column;max-height:90vh;margin-top:5vh;max-width:95vw}
:global(.batch-import-dialog .el-dialog__body){min-height:0;overflow-y:auto}
</style>


<template>
  <el-dialog :model-value="modelValue" :title="title" width="980px" destroy-on-close :close-on-click-modal="!importing" @update:model-value="emit('update:modelValue', $event)">
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
      <el-alert v-if="rows.length > previewLimit" :title="`为保证页面流畅，仅预览前 ${previewLimit} 行；全部 ${rows.length} 行仍会参与校验和导入。`" type="warning" :closable="false" class="preview-alert" />
      <el-table v-loading="parsing" :data="previewRows" stripe height="420px">
        <el-table-column prop="rowNumber" label="Excel 行" width="90" />
                <el-table-column prop="summary" label="数据标识" min-width="260" show-overflow-tooltip />
        <el-table-column label="操作" width="90">
          <template #default="{ row }"><el-tag :type="row.operation === 'UPDATE' ? 'warning' : 'success'">{{ row.operation === 'UPDATE' ? '更新' : '新增' }}</el-tag></template>
        </el-table-column>
        <el-table-column label="状态" width="105">
          <template #default="{ row }"><el-tag :type="statusType(row)">{{ statusLabel(row) }}</el-tag></template>
        </el-table-column>
        <el-table-column label="说明" min-width="420" show-overflow-tooltip>
          <template #default="{ row }"><span :class="{ 'danger-text': row.errors.length || row.importError }">{{ row.errors.join('；') || row.importError || '校验通过，可以导入' }}</span></template>
        </el-table-column>
      </el-table>
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
const previewLimit = 200
const previewRows = computed(() => rows.value.slice(0, previewLimit))
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
</script>

<style scoped>
.import-alert,.preview-alert{margin-bottom:14px}.import-toolbar{display:flex;align-items:center;gap:10px;flex-wrap:wrap;margin-bottom:14px}.file-name{max-width:320px;color:#606266;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.import-summary{margin-bottom:14px}.import-summary :deep(.el-card__body){display:flex;align-items:center;justify-content:space-between;padding:14px 16px}.import-summary span{color:#6b7280}.import-summary strong{font-size:24px}.success-text{color:#16a34a}.danger-text{color:#dc2626}
</style>


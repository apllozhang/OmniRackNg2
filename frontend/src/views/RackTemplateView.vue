<template>
  <div class="template-page">
    <div class="page-header">
      <div>
        <h2>机柜模板</h2>
        <p>维护可复用的机柜规格。模板版本发布后不可修改，已创建机柜保留当时的参数快照。</p>
      </div>
      <div class="header-actions">
        <el-button :loading="loading" @click="loadTemplates">刷新</el-button>
        <el-button type="primary" @click="openCreate">新增模板</el-button>
      </div>
    </div>

    <el-alert title="系统内置“标准 42U 机柜”模板不可删除或停用；新增版本不会自动修改已创建机柜。" type="info" show-icon :closable="false" />

    <el-card shadow="never" class="table-card" v-loading="loading">
      <el-table :data="templates" stripe>
        <el-table-column prop="code" label="模板编码" min-width="145" />
        <el-table-column prop="name" label="模板名称" min-width="170">
          <template #default="scope">
            <span>{{ scope.row.name }}</span>
            <el-tag v-if="scope.row.isSystem" size="small" class="system-tag">系统</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="当前版本" width="100">
          <template #default="scope">V{{ scope.row.currentRevision }}</template>
        </el-table-column>
        <el-table-column label="主要规格" min-width="230">
          <template #default="scope">
            <span v-if="latestVersion(scope.row)">
              {{ latestVersion(scope.row)?.uHeight }}U ·
              {{ latestVersion(scope.row)?.widthMm }}×{{ latestVersion(scope.row)?.depthMm }}×{{ latestVersion(scope.row)?.heightMm }}mm
            </span>
            <span v-else>-</span>
          </template>
        </el-table-column>
        <el-table-column label="电源/PDU" min-width="150">
          <template #default="scope">
            {{ latestVersion(scope.row)?.dualPower ? '双路' : '单路/未配置' }} · PDU {{ latestVersion(scope.row)?.pduCount ?? 0 }}
          </template>
        </el-table-column>
        <el-table-column label="状态" width="90">
          <template #default="scope"><el-tag :type="scope.row.status === 'ACTIVE' ? 'success' : 'info'">{{ scope.row.status === 'ACTIVE' ? '启用' : '停用' }}</el-tag></template>
        </el-table-column>
        <el-table-column label="操作" width="220" fixed="right">
          <template #default="scope">
            <el-button link type="primary" :disabled="scope.row.status !== 'ACTIVE'" @click="openVersion(scope.row)">新版本</el-button>
            <el-button link type="primary" @click="openEdit(scope.row)">编辑</el-button>
            <el-button link type="danger" :disabled="scope.row.isSystem" @click="removeTemplate(scope.row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-if="!loading && !templates.length" description="暂无机柜模板" />
    </el-card>

    <el-dialog v-model="templateDialog" :title="mode === 'create' ? '新增机柜模板' : '编辑模板信息'" width="920px" destroy-on-close>
      <el-form label-width="110px">
        <el-row :gutter="16">
          <el-col :span="10"><el-form-item label="模板编码" :required="mode === 'edit' || !templateAutoCode"><div class="code-field"><el-input v-model="form.code" maxlength="50" :disabled="mode === 'create' && templateAutoCode" :placeholder="mode === 'create' && templateAutoCode ? '保存时自动生成' : '请输入模板编码'" /><el-checkbox v-if="mode === 'create'" v-model="templateAutoCode">自动生成</el-checkbox></div></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="模板名称" required><el-input v-model="form.name" maxlength="150" /></el-form-item></el-col>
          <el-col :span="6"><el-form-item label="状态"><el-select v-model="form.status" style="width:100%" :disabled="editingSystem"><el-option label="启用" value="ACTIVE" /><el-option label="停用" value="DISABLED" /></el-select></el-form-item></el-col>
        </el-row>
        <el-form-item label="说明"><el-input v-model="form.description" type="textarea" :rows="2" /></el-form-item>

        <template v-if="mode === 'create'">
          <el-divider content-position="left">初始版本参数</el-divider>
          <rack-template-spec-form v-model="form.spec" />
        </template>
        <el-form-item label="备注"><el-input v-model="form.remarks" type="textarea" :rows="2" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="templateDialog = false">取消</el-button><el-button type="primary" :loading="saving" @click="submitTemplate">保存</el-button></template>
    </el-dialog>

    <el-dialog v-model="versionDialog" :title="`发布新版本 · ${versionTarget?.name ?? ''}`" width="820px" destroy-on-close>
      <el-alert :title="`将创建 V${(versionTarget?.currentRevision ?? 0) + 1}，历史版本不会被覆盖。`" type="warning" show-icon :closable="false" />
      <el-form label-width="110px" class="version-form"><rack-template-spec-form v-model="versionForm" /></el-form>
      <template #footer><el-button @click="versionDialog = false">取消</el-button><el-button type="primary" :loading="saving" @click="submitVersion">发布版本</el-button></template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import RackTemplateSpecForm from '@/components/RackTemplateSpecForm.vue'
import { createRackTemplate, createRackTemplateVersion, deleteRackTemplate, getRackTemplates, updateRackTemplate } from '@/api/rack-template'
import type { RackTemplate, RackTemplateCreatePayload, RackTemplateSpecPayload, RackTemplateUpdatePayload, RackTemplateVersion } from '@/types/rack-template'
import { createRackTemplateSpec } from '@/features/rack/rackDefaults'
import { hasRequiredCodeAndName, withAutoGeneratedCode } from '@/features/code/autoCode'

const emptySpec = (): RackTemplateSpecPayload => createRackTemplateSpec()
const emptyForm = (): RackTemplateCreatePayload & { version?: number; id?: string } => ({
  code: '', name: '', description: '', status: 'ACTIVE', remarks: '', spec: emptySpec(),
})

const templates = ref<RackTemplate[]>([])
const loading = ref(false)
const saving = ref(false)
const templateDialog = ref(false)
const versionDialog = ref(false)
const mode = ref<'create' | 'edit'>('create')
const templateAutoCode = ref(true)
const form = ref(emptyForm())
const versionTarget = ref<RackTemplate>()
const versionForm = ref<RackTemplateSpecPayload>(emptySpec())
const editingSystem = computed(() => mode.value === 'edit' && templates.value.find((item) => item.id === form.value.id)?.isSystem)

function latestVersion(item: RackTemplate): RackTemplateVersion | undefined {
  return [...(item.versions ?? [])].sort((a, b) => b.revision - a.revision)[0]
}

async function loadTemplates() {
  loading.value = true
  try { templates.value = (await getRackTemplates()).items ?? [] }
  catch (error) { ElMessage.error(errorMessage(error)) }
  finally { loading.value = false }
}

function openCreate() { mode.value = 'create'; templateAutoCode.value = true; form.value = emptyForm(); templateDialog.value = true }
function openEdit(item: RackTemplate) {
  mode.value = 'edit'
  templateAutoCode.value = false
  form.value = { id: item.id, version: item.version, code: item.code, name: item.name, description: item.description ?? '', status: item.status, remarks: item.remarks ?? '', spec: emptySpec() }
  templateDialog.value = true
}
function openVersion(item: RackTemplate) {
  const latest = latestVersion(item)
  versionTarget.value = item
  versionForm.value = latest ? {
    type: latest.type, manufacturer: latest.manufacturer ?? '', modelNumber: latest.modelNumber ?? '',
    uHeight: latest.uHeight, widthMm: latest.widthMm, depthMm: latest.depthMm, heightMm: latest.heightMm,
    loadCapacityKg: latest.loadCapacityKg, dualPower: latest.dualPower, inputCircuits: latest.inputCircuits,
    ratedVoltage: latest.ratedVoltage, ratedCurrent: latest.ratedCurrent, ratedPowerKw: latest.ratedPowerKw,
    peakPowerKw: latest.peakPowerKw, pduCount: latest.pduCount, changeNote: '',
  } : emptySpec()
  versionDialog.value = true
}

async function submitTemplate() {
  if (!hasRequiredCodeAndName(form.value.code, form.value.name, mode.value === 'create' && templateAutoCode.value)) { ElMessage.warning(mode.value === 'create' && templateAutoCode.value ? '模板名称不能为空' : '模板编码和名称不能为空'); return }
  saving.value = true
  try {
    if (mode.value === 'create') await createRackTemplate(withAutoGeneratedCode(form.value, templateAutoCode.value))
    else {
      const payload: RackTemplateUpdatePayload = { version: form.value.version!, code: form.value.code, name: form.value.name, description: form.value.description, status: form.value.status, remarks: form.value.remarks }
      await updateRackTemplate(form.value.id!, payload)
    }
    templateDialog.value = false; ElMessage.success('模板已保存'); await loadTemplates()
  } catch (error) { ElMessage.error(errorMessage(error)) } finally { saving.value = false }
}

async function submitVersion() {
  if (!versionTarget.value) return
  saving.value = true
  try {
    await createRackTemplateVersion(versionTarget.value.id, versionTarget.value.version, versionForm.value)
    versionDialog.value = false; ElMessage.success('新版本已发布'); await loadTemplates()
  } catch (error) { ElMessage.error(errorMessage(error)) } finally { saving.value = false }
}

async function removeTemplate(item: RackTemplate) {
  try {
    await ElMessageBox.confirm(`确定删除模板“${item.name}”吗？已创建机柜的快照不会受影响。`, '删除确认', { type: 'warning' })
    await deleteRackTemplate(item.id, item.version); ElMessage.success('模板已删除'); await loadTemplates()
  } catch (error) { if (error !== 'cancel' && error !== 'close') ElMessage.error(errorMessage(error)) }
}

function errorMessage(error: unknown) {
  const value = error as { response?: { data?: { message?: string } }; message?: string }
  return value.response?.data?.message ?? value.message ?? '操作失败'
}

onMounted(loadTemplates)
</script>

<style scoped>
.template-page { max-width: 1500px; margin: 0 auto; }
.page-header { display:flex; justify-content:space-between; align-items:flex-start; margin-bottom:18px; }
.page-header h2 { margin:0 0 8px; }
.page-header p { margin:0; color:var(--color-text-muted); }
.header-actions { display:flex; gap:10px; }
.table-card { margin-top:16px; }
.system-tag { margin-left:8px; }
.version-form { margin-top:18px; }
.code-field { width:100%; display:flex; align-items:center; gap:10px; }
.code-field .el-input { flex:1; }
.code-field .el-checkbox { flex:none; }
</style>
<script setup lang="ts">
import {onMounted,ref} from 'vue'
import {ElMessage} from 'element-plus'
import {getAdminApprovalPolicy,updateAdminApprovalPolicy} from '@/api/admin'
import {errorMessage} from '@/api/asset-ops'
import type {OperationPolicy} from '@/types/device'
import ApprovalEmailSettings from '@/components/ApprovalEmailSettings.vue'
import '@/styles/asset-ops.css'
const policy=ref<OperationPolicy|null>(null),loading=ref(false),saving=ref(false)
async function load(){loading.value=true;try{policy.value=await getAdminApprovalPolicy()}catch(e){ElMessage.error(errorMessage(e))}finally{loading.value=false}}
async function save(){if(!policy.value)return;saving.value=true;try{policy.value=await updateAdminApprovalPolicy({...policy.value,defaultApproverRole:'system_admin'});ElMessage.success('审批规则已保存')}catch(e){ElMessage.error(errorMessage(e))}finally{saving.value=false}}
onMounted(load)
</script>
<template><div class="asset-ops"><div class="ops-heading"><div><h1>审批规则</h1><p>配置设备位置变更的审批要求及待办通知；申请处理请进入工作台的审批中心。</p></div></div><div class="approval-rules-grid"><el-card shadow="never" v-loading="loading"><template #header><strong>单级审批策略</strong></template><el-alert title="当前由系统管理员处理审批。启用对应规则后，所有账号提交该操作均需审批；关闭规则后直接执行。已提交的申请仍需处理。" type="info" :closable="false"/><el-form v-if="policy" label-position="top" class="policy-form"><el-form-item label="上架需要审批"><el-switch v-model="policy.assignApprovalEnabled"/></el-form-item><el-form-item label="迁移需要审批"><el-switch v-model="policy.moveApprovalEnabled"/></el-form-item><el-form-item label="下架需要审批"><el-switch v-model="policy.decommissionApprovalEnabled"/></el-form-item><el-button type="primary" :loading="saving" @click="save">保存审批规则</el-button></el-form><el-button v-else-if="!loading" @click="load">重新加载</el-button></el-card><ApprovalEmailSettings/></div></div></template>
<style scoped>.approval-rules-grid{display:grid;grid-template-columns:minmax(280px,1fr) minmax(360px,1fr);gap:24px;align-items:start}:deep(.approval-email){margin-top:0}.policy-form{margin-top:24px}@media(max-width:1000px){.approval-rules-grid{grid-template-columns:1fr}}</style>

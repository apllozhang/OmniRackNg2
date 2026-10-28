<script setup lang="ts">
import {ElMessage} from 'element-plus'
import {exportBundle,errorMessage} from '@/api/asset-ops'
async function fullExport(){try{await exportBundle();ElMessage.success('已导出完整业务数据，登录凭据不在业务导出中')}catch(e){ElMessage.error(errorMessage(e))}}

import '@/styles/asset-ops.css'
</script>
<template><div class="asset-ops"><div class="ops-heading"><div><h1>数据导出与备份</h1><p>各业务页面保留按筛选或勾选导出；此处统一提供完整业务数据导出和系统备份指引。</p></div></div><el-card shadow="never"><div class="ops-backup"><h3>业务导出</h3><p>配置台账和备件、盘点页面支持 Excel；完整业务导出包含资产配置、服务保障、库存流水和盘点快照，不包含用户密码、LDAP 配置或 JWT。</p><el-button type="primary" @click="fullExport">导出完整业务 JSON</el-button><h3>完整系统备份</h3><p>通过受限运维脚本备份 PostgreSQL 和部署配置，以 GPG 加密。数据库内包含 LDAP 配置，因此备份密钥必须独立保管。备份前停止应用写入，完成后恢复运行。</p><code>bash scripts/backup-assets.sh</code><h3>校验与恢复</h3><p>默认只解密校验，不覆盖数据库；完整恢复需显式确认，并应先在隔离环境演练。详见扩展部署说明。</p><code>bash scripts/restore-assets.sh /path/to/backup.tar.gpg --verify-only</code><el-alert title="Excel 与 JSON 用于业务资料导出；完整恢复请使用系统备份，并先在隔离环境验证。" type="info" :closable="false"/></div></el-card></div></template>

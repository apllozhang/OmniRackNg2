<template>
  <el-card shadow="never" class="approval-email" v-loading="loading">
    <template #header><strong>审批通知</strong></template>
    <el-alert v-if="settings && !settings.emailEnabled" title="请先启用 SMTP 邮件，再开启审批通知。" type="warning" :closable="false" show-icon />
    <p class="help">复用<router-link to="/notification-settings">告警通知配置</router-link>中的 SMTP 和企业微信机器人。审批邮件收件人独立设置，通知链接仍需管理员登录。</p>
    <el-form v-if="settings" label-position="top" @submit.prevent="save">
      <el-form-item label="新申请邮件通知"><el-switch v-model="settings.enabled" :disabled="!settings.emailEnabled" /></el-form-item>
      <el-form-item label="新申请企业微信通知"><el-switch v-model="settings.weComEnabled" :disabled="!settings.weComReady" /></el-form-item>
      <p v-if="!settings.weComReady" class="help">企业微信尚未启用，请先在告警通知中保存机器人配置。</p>
      <el-form-item label="管理员收件邮箱" :required="settings.enabled"><el-input v-model="recipients" type="textarea" :rows="3" placeholder="admin@example.com，每行一个，最多 20 个" /></el-form-item>
      <el-form-item label="系统访问地址" :required="settings.enabled||settings.weComEnabled"><el-input v-model="settings.baseUrl" placeholder="https://it.example.com 或 http://IP:15173" /></el-form-item>
      <p class="help">填写管理员可访问的系统根地址。可同时启用两种渠道；每种渠道独立发送和记录，失败约 15 分钟后重试。启用后也会通知当前尚未处理的申请。</p>
      <el-button type="primary" :loading="saving" @click="save">保存通知设置</el-button>
      <el-button :disabled="saving" @click="load">重新加载</el-button>
      <div class="test-actions"><el-button :disabled="saving||!!testing||!settings.enabled" @click="test('EMAIL')">测试审批邮件</el-button><el-button :disabled="saving||!!testing||!settings.weComEnabled" @click="test('WECOM')">测试审批企微</el-button></div>
      <p class="help">测试使用已保存配置，请先保存。邮件“通道已接收”表示 SMTP 接收，可用发送记录中的收件邮箱和 Message-ID 向邮箱管理员查询投递。</p>
      <p class="help">发送结果可在<router-link to="/notification-deliveries">通知发送记录</router-link>中查看。修改 SMTP 或通知设置后，请重新加载以获取最新版本。</p>
    </el-form>
    <el-button v-else-if="!loading" @click="load">重新加载</el-button>
  </el-card>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { getApprovalEmailSettings, saveApprovalEmailSettings, testApprovalNotification } from '@/api/notifications'
import type { ApprovalEmailSettings } from '@/api/notifications'
import { errorMessage } from '@/api/asset-ops'

const settings = ref<ApprovalEmailSettings>(), recipients = ref(''), loading = ref(false), saving = ref(false),testing=ref('')
async function test(channel:'EMAIL'|'WECOM'){
  try{await ElMessageBox.confirm('将使用已保存的审批通知配置发送测试消息，是否发送？','审批通知测试',{confirmButtonText:'发送测试',cancelButtonText:'取消'})}catch{return}
  testing.value=channel
  try{await testApprovalNotification(channel);ElMessage.success('审批通知测试已被通道接收，请核对实际收件结果')}catch(e){ElMessage.error(errorMessage(e))}finally{testing.value=''}
}
async function load() {
  loading.value = true
  try { settings.value = await getApprovalEmailSettings(); recipients.value = settings.value.recipients.join('\n') }
  catch (e) { ElMessage.error(errorMessage(e)) }
  finally { loading.value = false }
}
async function save() {
  if (!settings.value) return
  const addresses = [...new Set(recipients.value.split(/[\s,;，；]+/).filter(Boolean))]
  if (settings.value.enabled && (!addresses.length || addresses.length > 20 || addresses.some(a => !/^[^@\s]+@[^@\s]+$/.test(a)) || !settings.value.baseUrl.trim())) {
    ElMessage.warning('请填写 1–20 个有效收件邮箱和系统访问地址'); return
  }
  if(settings.value.weComEnabled&&!settings.value.baseUrl.trim()){ElMessage.warning('请填写系统访问地址');return}
  saving.value = true
  try {
    settings.value = await saveApprovalEmailSettings({ ...settings.value, recipients: addresses })
    recipients.value = settings.value.recipients.join('\n')
    ElMessage.success('审批通知设置已保存')
  } catch (e) { ElMessage.error(errorMessage(e)) }
  finally { saving.value = false }
}
onMounted(load)
</script>

<style scoped>
.approval-email{margin-top:18px}.test-actions{display:flex;gap:10px;flex-wrap:wrap;margin-top:16px}.help{font-size:13px;line-height:1.7;color:var(--color-text-secondary);overflow-wrap:anywhere}.help a{color:#4d77be}
</style>

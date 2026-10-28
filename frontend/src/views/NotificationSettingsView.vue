<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { getNotificationSettings, saveNotificationSettings, testNotification } from '@/api/notifications'
import type { NotificationForm, NotificationSettings } from '@/api/notifications'
import { errorMessage } from '@/api/asset-ops'

const loading = ref(false), saving = ref(false), testing = ref(''), loaded = ref(false)
const form = reactive<NotificationForm>({ version: 0, emailEnabled: false, smtpHost: '', smtpPort: 587, smtpSecurity: 'STARTTLS', smtpUsername: '', smtpPassword: '', emailFrom: '', emailRecipients: [], hasSmtpPassword: false, weComEnabled: false, weComWebhook: '', hasWeComWebhook: false, warrantyEnabled: true, rackUEnabled: true, rackWarningPercent: 70, rackCriticalPercent: 90, repeatHours: 24, clearSmtpPassword: false, clearWeComWebhook: false })
function apply(settings: NotificationSettings) {
  Object.assign(form, settings, { smtpPassword: '', weComWebhook: '', clearSmtpPassword: false, clearWeComWebhook: false })
}
async function load() {
  loading.value = true
  try { apply(await getNotificationSettings()); loaded.value = true } catch (e) { ElMessage.error(errorMessage(e)) }
  finally { loading.value = false }
}
async function save() {
  if (form.rackWarningPercent >= form.rackCriticalPercent) { ElMessage.warning('U 位预警阈值必须小于严重阈值'); return }
  saving.value = true
  try { apply(await saveNotificationSettings({ ...form })); ElMessage.success('通知配置已保存，下次检查时生效') }
  catch (e) { ElMessage.error(errorMessage(e)) } finally { saving.value = false }
}
async function test(channel: 'EMAIL' | 'WECOM') {
  try { await ElMessageBox.confirm('将使用已保存的配置，向收件邮箱或企微群发送一条测试消息。未保存的修改不会参与测试。', '发送渠道测试', { confirmButtonText: '发送测试', cancelButtonText: '取消' }) }
  catch { return }
  testing.value = channel
  try { await testNotification(channel); ElMessage.success('服务已接受测试消息，请核对实际接收结果') }
  catch (e) { ElMessage.error(errorMessage(e)) } finally { testing.value = '' }
}
onMounted(load)
</script>

<template>
  <div class="notification-page" v-loading="loading">
    <div class="page-header">
      <div><span class="page-kicker">NOTIFICATION CENTER</span><h1>告警通知</h1><p>把保修与机柜容量风险，发送到负责处理的人。</p></div>
      <el-button type="primary" :loading="saving" :disabled="!loaded" @click="save">保存配置</el-button>
    </div>
    <div class="notification-banner"><span class="banner-icon">↗</span><div><strong>两种渠道，统一告警规则</strong><p>每 15 分钟检查一次。新告警或风险变化会通知，持续风险按设定间隔重复提醒；已记录处理的告警不再发送。</p></div><el-tag type="info" effect="plain">管理员设置</el-tag></div>
    <div class="notification-grid">
      <el-card shadow="never" class="channel-card">
        <template #header><div class="channel-heading"><div class="channel-title"><span class="channel-icon email-icon">✉</span><div><h2>邮件通知</h2><p>连接企业 SMTP 邮箱或内网中继</p></div></div><el-switch v-model="form.emailEnabled" aria-label="启用邮件通知" /></div></template>
        <el-form label-position="top" :disabled="!loaded || !form.emailEnabled">
          <div class="notification-fields"><el-form-item label="SMTP 服务器"><el-input v-model="form.smtpHost" placeholder="smtp.example.com" maxlength="253" /></el-form-item><el-form-item label="端口"><el-input-number v-model="form.smtpPort" :min="1" :max="65535" controls-position="right" /></el-form-item></div>
          <el-form-item label="连接加密"><el-select v-model="form.smtpSecurity"><el-option label="STARTTLS（常用端口 587）" value="STARTTLS"/><el-option label="TLS（常用端口 465）" value="TLS"/><el-option label="无加密（仅无认证内网中继）" value="NONE"/></el-select></el-form-item>
          <el-form-item label="SMTP 账号"><el-input v-model="form.smtpUsername" autocomplete="off" placeholder="无认证中继可留空" maxlength="255" /></el-form-item>
          <el-form-item label="密码 / 授权码"><el-input v-model="form.smtpPassword" type="password" show-password autocomplete="new-password" :placeholder="form.hasSmtpPassword ? '已配置，留空保留原凭据' : '请输入邮箱密码或授权码'" /><el-checkbox v-if="form.hasSmtpPassword" v-model="form.clearSmtpPassword">清除已保存密码</el-checkbox></el-form-item>
          <el-form-item label="发件邮箱"><el-input v-model="form.emailFrom" placeholder="it-assets@example.com" /></el-form-item>
          <el-form-item label="收件邮箱"><el-select v-model="form.emailRecipients" multiple filterable allow-create default-first-option :multiple-limit="20" placeholder="输入邮箱后按回车，最多 20 个" /></el-form-item>
        </el-form>
        <div class="channel-footer"><span :class="form.hasSmtpPassword ? 'credential-ready' : 'muted'">{{ form.hasSmtpPassword ? '● 凭据已加密保存' : '支持企业邮箱授权码' }}</span><el-button :disabled="!loaded || !form.emailEnabled || !!testing" :loading="testing === 'EMAIL'" @click="test('EMAIL')">发送测试邮件</el-button></div>
      </el-card>
      <div class="notification-right">
        <el-card shadow="never" class="channel-card">
          <template #header><div class="channel-heading"><div class="channel-title"><span class="channel-icon wecom-icon">企</span><div><h2>企业微信</h2><p>通过群机器人将告警发送到运维群</p></div></div><el-switch v-model="form.weComEnabled" aria-label="启用企业微信通知" /></div></template>
          <el-form label-position="top" :disabled="!loaded || !form.weComEnabled"><el-form-item label="群机器人 Webhook"><el-input v-model="form.weComWebhook" type="password" show-password autocomplete="new-password" :placeholder="form.hasWeComWebhook ? '已配置，留空保留原地址' : 'https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=…'"/><el-checkbox v-if="form.hasWeComWebhook" v-model="form.clearWeComWebhook">清除已保存地址</el-checkbox></el-form-item></el-form>
          <p class="form-help">在企业微信目标群添加机器人，复制 Webhook 地址后保存。服务器需能访问企业微信接口。</p>
          <div class="channel-footer"><span :class="form.hasWeComWebhook ? 'credential-ready' : 'muted'">{{ form.hasWeComWebhook ? '● 机器人地址已加密保存' : '机器人地址不会回显' }}</span><el-button :disabled="!loaded || !form.weComEnabled || !!testing" :loading="testing === 'WECOM'" @click="test('WECOM')">发送企微测试</el-button></div>
        </el-card>
        <el-card shadow="never" class="rule-card">
          <template #header><div class="section-heading"><h2>告警规则</h2><span class="muted">两个渠道共用</span></div></template>
          <el-form label-position="top" :disabled="!loaded">
            <div class="rule-switch"><div><strong>保修风险通知</strong><p>提前 30 天、已过保和保障空档，包含按件备件</p></div><el-switch v-model="form.warrantyEnabled" aria-label="保修风险通知"/></div>
            <div class="rule-switch"><div><strong>U 位容量通知</strong><p>按已占物理 U 位计算，排除未投用和停用机柜</p></div><el-switch v-model="form.rackUEnabled" aria-label="U位容量通知"/></div>
            <div class="notification-fields equal"><el-form-item label="U 位预警阈值（%）"><el-input-number v-model="form.rackWarningPercent" :min="1" :max="99" controls-position="right"/></el-form-item><el-form-item label="U 位严重阈值（%）"><el-input-number v-model="form.rackCriticalPercent" :min="2" :max="100" controls-position="right"/></el-form-item></div>
            <el-form-item label="相同告警重复提醒间隔（小时）"><el-input-number v-model="form.repeatHours" :min="1" :max="168" controls-position="right"/></el-form-item>
            <p class="form-help">达到阈值即告警。通知开关仅控制外部发送，提醒中心持续展示风险。失败后最早 15 分钟重试，发送成功表示通道接收，仍需核对最终投递。</p>
          </el-form>
        </el-card>
      </div>
    </div>

  </div>
</template>

<style scoped>
.notification-page{max-width:1400px;margin:auto}.notification-banner{display:flex;align-items:center;gap:16px;padding:20px 24px;border:1px solid #dbe6f5;background:#f0f5fd;border-radius:12px;margin-bottom:24px}.banner-icon{display:grid;place-items:center;width:40px;height:40px;flex:none;background:#dfeafa;border-radius:10px;color:#3d6cac;font-size:24px}.notification-banner strong{font-size:14px}.notification-banner p{margin:6px 0 0;font-size:12px;line-height:1.8;color:#5b708c}.notification-banner .el-tag{margin-left:auto;flex:none}.notification-grid{display:grid;grid-template-columns:1fr 1fr;gap:24px;align-items:start}.notification-right{display:grid;gap:24px}.channel-heading,.channel-title,.channel-footer{display:flex;align-items:center;justify-content:space-between;gap:16px}.channel-title{justify-content:flex-start}.channel-icon{display:grid;place-items:center;flex:none;width:43px;height:43px;border-radius:11px;font-size:24px}.email-icon{background:#ecf2ff;color:#4776c4}.wecom-icon{background:#e6f5ed;color:#288458;font-size:19px;font-weight:650}h2{font-size:15px;margin:0}.channel-title p p{font-size:12px;margin:6px 0 0;color:#75859b}.notification-fields{display:grid;grid-template-columns:minmax(0,1fr) 125px;gap:16px}.notification-fields.equal{grid-template-columns:1fr 1fr}.notification-page .el-select,.notification-page .el-input-number{width:100%}.channel-footer{border-top:1px solid var(--color-border-soft);padding-top:18px;margin-top:8px;font-size:12px}.credential-ready{color:#318362}.form-help{font-size:12px;color:#748399;line-height:1.9;margin:0 0 16px}.rule-switch{display:flex;align-items:center;justify-content:space-between;gap:16px;margin-bottom:20px}.rule-switch strong{font-size:13px;font-weight:550}.rule-switch p{font-size:12px;color:#78889c;line-height:1.7;margin:5px 0 0}.muted{color:#7c8a9e;font-size:12px}
@media(max-width:1100px){.notification-grid{grid-template-columns:1fr}.notification-banner .el-tag{display:none}}@media(max-width:760px){.notification-banner{padding:16px;align-items:flex-start}.notification-fields{grid-template-columns:1fr}.notification-grid,.notification-right{gap:16px}.channel-footer{flex-wrap:wrap}.notification-fields.equal{gap:10px}.notification-banner .banner-icon{display:none}}
</style>

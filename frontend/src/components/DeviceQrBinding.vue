<script setup lang="ts">
import { ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import QRCode from 'qrcode'
import AssetScanner from '@/components/AssetScanner.vue'
import { getScanAccessSettings } from '@/api/scan-access'
import { assetLabelUrl } from '@/utils/scanAccess'
import { printAssetLabels, errorMessage, systemScanTarget } from '@/api/asset-ops'
import type { DevicePayload } from '@/types/device'
const props = withDefaults(defineProps<{ deviceId?: string }>(), { deviceId: '' })
const model = defineModel<DevicePayload>({ required: true })
const loading = ref(false), link = ref(''), image = ref(''), scanOpen = ref(false)
watch(() => props.deviceId, () => { link.value = ''; image.value = ''; scanOpen.value = false })
async function showQr() {
  if (!props.deviceId) return
  const id = props.deviceId
  link.value = ''; image.value = ''
  loading.value = true
  try {
    const settings = await getScanAccessSettings()
    const address = assetLabelUrl(settings.baseUrl, 'device', id)
    const png = await QRCode.toDataURL(address, { width: 200, margin: 4, errorCorrectionLevel: 'M' })
    if (id === props.deviceId) { link.value = address; image.value = png }
  } catch (e) { ElMessage.error(errorMessage(e)) } finally { loading.value = false }
}
async function printLabel() {
  if (!props.deviceId) return
  loading.value = true
  try { await printAssetLabels('device', [{ id: props.deviceId }], { reloadDevices: true }); ElMessage.success('标签已生成，请在标签窗口打印或保存 PDF') }
  catch (e) { ElMessage.error(errorMessage(e)) } finally { loading.value = false }
}
function bindScan(value: string) {
  if (systemScanTarget(value)) { ElMessage.warning('这是系统二维码，无需绑定为外部标签，请在扫码查询或盘点页面使用'); return }
  const raw = value.trim(), isLink = /^https?:\/\//i.test(raw)
  if (!raw || raw.length > (isLink ? 500 : 255)) { ElMessage.warning('标签内容为空或超出字段长度，请核对后录入'); return }
  if (isLink) model.value.externalQrCodeUrl = raw
  else model.value.externalQrCode = raw
  scanOpen.value = false
  ElMessage.success('已填入外部标签，请保存设备后生效')
}
</script>
<template>
  <div class="device-qr-binding">
    <div class="system-label"><div><strong>系统二维码 · 自动生成</strong><p>使用“扫码访问设置”的统一地址定位设备档案，无需填写下面两个字段。</p><p v-if="!deviceId">保存设备后可重新打开编辑，查看和打印系统标签。</p></div><div class="qr-actions"><el-button :disabled="!deviceId" :loading="loading" @click="showQr">查看系统二维码</el-button><el-button :disabled="!deviceId" :loading="loading" @click="printLabel">打印系统标签</el-button></div></div>
    <div v-if="image" class="system-qr"><img :src="image" alt="设备系统二维码" width="160" height="160"/><div><strong>系统二维码链接（只读）</strong><p>{{ link }}</p><small>外部标签不会替换此链接；统一地址变更后请重新生成。</small></div></div>
    <div class="external-heading"><strong>绑定已有外部标签（选填）</strong><el-button link type="primary" @click="scanOpen=!scanOpen">{{ scanOpen?'收起扫码录入':'扫描已有标签' }}</el-button></div>
    <AssetScanner v-if="scanOpen" @result="bindScan"/>
    <div class="binding-fields"><el-form-item label="外部二维码内容"><el-input v-model="model.externalQrCode" maxlength="255" placeholder="已有标签实际编码，例如 QR-2026-001"/></el-form-item><el-form-item label="外部二维码链接"><el-input v-model="model.externalQrCodeUrl" maxlength="500" placeholder="已有标签实际包含的完整链接"/></el-form-item></div>
    <p class="binding-note">两项可只填一项。请在平台扫码页面扫描外部标签，按原始内容匹配设备；手机相机直接扫描外部链接仍访问原网站。修改后需保存。新绑定用于后续创建的盘点任务，已有任务仍可使用系统码、编号或 SN。</p>
  </div>
</template>
<style scoped>
.device-qr-binding{margin-bottom:18px}.system-label{display:flex;justify-content:space-between;align-items:flex-start;gap:14px;padding:14px;background:#f3f7fd;border:1px solid #dfe8f5;border-radius:8px;margin-bottom:16px}.system-label p,.binding-note{font-size:12px;color:#718197;line-height:1.8;margin:6px 0 0}.system-label strong,.external-heading strong{font-size:13px}.qr-actions{display:flex;flex-wrap:wrap;gap:8px;flex-shrink:0}.qr-actions .el-button+.el-button{margin-left:0}.external-heading{display:flex;align-items:center;justify-content:space-between;margin-bottom:10px}.binding-fields{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:18px}.system-qr{display:flex;align-items:center;gap:16px;margin-bottom:14px;font-size:12px}.system-qr p{overflow-wrap:anywhere;color:#4d77bf}.system-qr img{flex:none}.system-qr small{color:#718197}.binding-note{margin-top:-8px}@media(max-width:820px){.system-label{display:block}.qr-actions{margin-top:10px}.binding-fields{grid-template-columns:1fr;gap:0}.system-qr{align-items:flex-start;flex-direction:column}}
</style>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import QRCode from 'qrcode'
import { getScanAccessSettings, saveScanAccessSettings } from '@/api/scan-access'
import { normalizeScanBaseUrl, scanEntryUrl } from '@/utils/scanAccess'
import { errorMessage } from '@/api/asset-ops'
const form = reactive({ version: 0, baseUrl: '' }), loaded = ref(false), loading = ref(false), saving = ref(false)
const savedBase = ref(''), scanQr = ref(''), checkQr = ref('')
const scanUrl = computed(() => scanEntryUrl(savedBase.value, 'asset-scan'))
const checkUrl = computed(() => scanEntryUrl(savedBase.value, 'asset-checks'))
const insecure = computed(() => scanUrl.value.startsWith('http:'))
async function showSaved(value: { version: number; baseUrl: string }) {
  Object.assign(form, value); savedBase.value = value.baseUrl
  ;[scanQr.value, checkQr.value] = await Promise.all([scanUrl.value, checkUrl.value].map(url => QRCode.toDataURL(url, { width: 240, margin: 4, errorCorrectionLevel: 'M' })))
}
async function load() {
  loading.value = true
  try { await showSaved(await getScanAccessSettings()); loaded.value = true }
  catch (e) { ElMessage.error(errorMessage(e)) } finally { loading.value = false }
}
async function save() {
  if (!loaded.value || saving.value) return
  let baseUrl: string
  try { baseUrl = normalizeScanBaseUrl(form.baseUrl) } catch (e) { ElMessage.warning(errorMessage(e)); return }
  saving.value = true
  try { await showSaved(await saveScanAccessSettings({ version: form.version, baseUrl })); ElMessage.success('扫码访问地址已保存，新生成的二维码立即使用该地址') }
  catch (e) { ElMessage.error(errorMessage(e)) } finally { saving.value = false }
}
onMounted(load)
</script>
<template>
  <div class="scan-access-page" v-loading="loading">
    <div class="access-heading"><div><h1>扫码访问设置</h1><p>统一设备、备件二维码和手机扫码入口的访问地址。</p></div><el-button type="primary" :disabled="!loaded" :loading="saving" @click="save">保存配置</el-button></div>
    <el-card shadow="never"><el-form label-position="top" @submit.prevent="save"><el-form-item label="统一扫码访问地址"><el-input v-model="form.baseUrl" :disabled="!loaded || saving" clearable placeholder="例如 https://assets.example.com 或 http://公网IP:15173" maxlength="512"/><p class="access-muted">留空时使用生成二维码时浏览器的地址。仅填写协议、IP 或域名及可选端口，不含 /asset-scan 等路径。</p></el-form-item></el-form>
      <el-alert title="配置地址不会自动开通公网或修改网络。请先完成域名解析、HTTPS 证书及反向代理或 VPN 访问，再从手机实际验证。" type="info" :closable="false"/>
    </el-card>
    <div v-if="loaded" class="access-entries"><el-card shadow="never" v-for="item in [{name:'扫码查询入口',url:scanUrl,image:scanQr},{name:'扫码盘点入口',url:checkUrl,image:checkQr}]" :key="item.name"><h2>{{ item.name }}</h2><img :src="item.image" :alt="item.name+'二维码'" width="180" height="180"/><p class="entry-url">{{ item.url }}</p><a :href="item.url" target="_blank" rel="noopener noreferrer">打开入口验证</a><p class="access-muted">以上入口按已保存的配置生成，需要登录并具有相应权限。</p></el-card></div>
    <el-alert v-if="loaded && insecure" title="当前入口使用 HTTP，手机浏览器通常不能调用摄像头。请使用受信任的 HTTPS；扫码枪和手工输入仍可使用。" type="warning" :closable="false"/>
    <el-card shadow="never"><h2>使用说明</h2><p>配置保存于服务器，所有操作员共用。修改后，新打印的设备和备件标签使用新地址。</p><p>旧标签内的地址不会自动改变。可以重新打印，或先进入新的扫码查询／盘点入口，再用页面摄像头扫描旧标签，系统仍按原设备标识识别。</p><p>建议使用固定域名；内网与公网可通过各自 DNS 指向对应入口，避免服务器 IP 变化后重打标签。</p><p>手机只需能访问入口，无需与设备处在同一网络。没有网络或入口不通时，仍不能在线查询或提交盘点；本次未增加离线 App。</p></el-card>
  </div>
</template>
<style scoped>
.scan-access-page{max-width:1100px;margin:auto;display:grid;gap:20px}.access-heading{display:flex;justify-content:space-between;align-items:center;gap:16px}.access-heading h1{margin:0;font-size:26px}.access-heading p,.access-muted{color:#75859a;font-size:13px;line-height:1.8}.access-entries{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:20px}.scan-access-page h2{font-size:17px;margin-top:0}.entry-url{word-break:break-all;font-size:14px}.scan-access-page a{color:#4d77bf}.scan-access-page .el-card p{line-height:1.8}@media(max-width:700px){.access-entries{grid-template-columns:1fr}}
</style>

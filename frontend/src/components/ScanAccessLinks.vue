<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { getScanAccessSettings } from '@/api/scan-access'
import { scanEntryUrl } from '@/utils/scanAccess'
const base = ref<string | null>(null), failed = ref(false)
onMounted(async () => { try { base.value = (await getScanAccessSettings()).baseUrl } catch { failed.value = true } })
</script>
<template><div class="scan-access-links"><template v-if="base!==null">手机入口：<a :href="scanEntryUrl(base,'asset-scan')" target="_blank" rel="noopener noreferrer">扫码查询</a><span> · </span><a :href="scanEntryUrl(base,'asset-checks')" target="_blank" rel="noopener noreferrer">扫码盘点</a><span>（{{ base || '当前访问地址' }}）</span></template><template v-else-if="failed">扫码入口配置加载失败，请刷新重试。</template><p>手机须能访问入口，摄像头需要受信任的 HTTPS。管理员可在“系统管理 → 扫码访问设置”中更改地址。</p></div></template>
<style scoped>.scan-access-links{font-size:12px;color:#75859a;line-height:1.8;margin-bottom:16px;overflow-wrap:anywhere}.scan-access-links a{color:var(--color-action)}.scan-access-links p{margin:4px 0 0}</style>

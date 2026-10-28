<script setup lang="ts">
import {ref,watch,onBeforeUnmount} from 'vue'
import {useRoute,useRouter} from 'vue-router'
import {ElMessage} from 'element-plus'
import {listOps,parsedScan,matchesAssetScan,errorMessage,stockStates,usageLabels} from '@/api/asset-ops'
import AssetScanner from '@/components/AssetScanner.vue'
import ScanAccessLinks from '@/components/ScanAccessLinks.vue'
import '@/styles/asset-ops.css'
const route=useRoute(),router=useRouter(),entry=ref(''),loading=ref(false),result=ref<any>(null),kind=ref(''),notFound=ref(false)
let requestVersion=0
async function find(value:string){
  const version=++requestVersion
  entry.value=value;result.value=null;notFound.value=false
  if(!parsedScan(value)){loading.value=false;return}
  loading.value=true
  try{
    const [assets,stocks]=await Promise.all([listOps('devices'),listOps('stocks')])
    if(version!==requestVersion)return
    const a=assets.filter(a=>matchesAssetScan(a.device,value,'device'))
    const s=stocks.filter(s=>matchesAssetScan(s,value,'stock'))
    if(a.length+s.length>1){ElMessage.warning('识别码匹配多个对象，请到台账核查编号、SN 或外部标签绑定');return}
    if(a.length){result.value=a[0];kind.value='device'}else if(s.length){result.value=s[0];kind.value='stock'}else notFound.value=true
  }catch(e){if(version===requestVersion)ElMessage.error(errorMessage(e))}
  finally{if(version===requestVersion)loading.value=false}
}
watch(()=>[route.query.device,route.query.stock],()=>{void find(route.query.device||route.query.stock?`${location.origin}${route.fullPath}`:'')},{immediate:true})
onBeforeUnmount(()=>{requestVersion++})

</script>
<template><div class="asset-ops"><div class="ops-heading"><div><span class="ops-eyebrow">SCAN AND IDENTIFY</span><h1>扫码查询</h1><p>系统二维码用于定位记录，查看详情仍需要登录和权限。</p></div><el-button @click="router.push('/asset-checks')">进入盘点任务</el-button></div><ScanAccessLinks/><div class="ops-scan-layout"><el-card shadow="never"><div class="ops-scan-placeholder">▦</div><h2>识别设备或备件</h2><p class="ops-muted">手机摄像头、扫码枪和手工输入均可。</p><div class="ops-toolbar"><el-input v-model="entry" placeholder="资产编号 / SN / 系统码或外部标签内容" @keyup.enter="find(entry)"/><el-button type="primary" :loading="loading" @click="find(entry)">查询</el-button></div><AssetScanner :disabled="loading" @result="find"/><el-alert title="摄像头需要受信任的 HTTPS。手机须能访问系统入口，可使用办公 Wi-Fi、VPN 或已开通的公网入口；当前不支持离线盘点。" type="info" :closable="false"/></el-card><el-card shadow="never"><template v-if="result"><h2>{{kind==='device'?result.device.name:result.spec.name}}</h2><el-descriptions :column="1" border><el-descriptions-item label="SN">{{kind==='device'?result.device.serialNumber:result.serialNumber||'数量管理'}}</el-descriptions-item><el-descriptions-item label="编号">{{kind==='device'?result.device.code:result.id}}</el-descriptions-item><el-descriptions-item v-if="kind==='device'" label="资产位置">{{result.profile.site?.name||result.device.currentPosition?.rack?.name||'待补全'}}</el-descriptions-item><el-descriptions-item v-if="kind==='device'" label="配置">{{result.profile.cpu}} / {{result.profile.memoryGb||'—'}}GB / {{result.profile.disk}}</el-descriptions-item><el-descriptions-item v-if="kind==='device'" label="责任人">{{result.profile.assignee||result.device.manager||'待补全'}}</el-descriptions-item><el-descriptions-item v-if="kind==='device'" label="保修">{{result.warranty.label}}</el-descriptions-item><el-descriptions-item v-if="kind==='stock'" label="位置">{{result.state==='INSTALLED'?'随设备 '+result.installedDevice?.code:result.site.name}}</el-descriptions-item><el-descriptions-item v-if="kind==='stock'" label="状态 / 数量">{{stockStates[result.state]}} / {{result.quantity}}</el-descriptions-item></el-descriptions></template><template v-else-if="notFound"><el-empty description="未找到记录"/><p>厂商条码通常只能读取 SN，完整配置需要核实后录入。可到设备台账建档，或在盘点任务登记发现记录。</p></template><el-empty v-else description="扫码结果将在此显示"/></el-card></div></div></template>

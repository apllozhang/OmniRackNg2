<template><div class="asset-scanner"><video ref="video" v-show="active" playsinline muted></video><el-button :disabled="disabled||(!active&&opening)" @click="active?stop():start()">{{active?'关闭摄像头':opening?'等待摄像头请求结束…':'打开摄像头扫码'}}</el-button><p v-if="error" class="scanner-error" role="alert">{{error}}</p></div></template>
<script setup lang="ts">
import {ref,watch,onMounted,onBeforeUnmount,onDeactivated} from 'vue'
import {BrowserMultiFormatReader,type IScannerControls} from '@zxing/browser'
const props=withDefaults(defineProps<{disabled?:boolean}>(),{disabled:false})
const emit=defineEmits<{(e:'result',value:string):void}>(),video=ref<HTMLVideoElement>(),active=ref(false),error=ref(''),opening=ref(false)
let controls:IScannerControls|undefined,stream:MediaStream|undefined,generation=0
function stop(){
  generation++;active.value=false
  const previous=controls;controls=undefined;previous?.stop()
  stream?.getTracks().forEach(track=>track.stop());stream=undefined
  if(video.value)video.value.srcObject=null
}
async function start(){
  if(props.disabled||active.value||opening.value)return
  error.value='';active.value=true;opening.value=true;const run=++generation
  try{
    if(globalThis.isSecureContext===false||!navigator.mediaDevices?.getUserMedia)throw new Error('请使用受信任的 HTTPS 访问，或改用扫码枪 / 手工输入')
    const media=await navigator.mediaDevices.getUserMedia({video:{facingMode:{ideal:'environment'}},audio:false})
    if(run!==generation){media.getTracks().forEach(track=>track.stop());return}
    stream=media
    const reader=new BrowserMultiFormatReader()
    const handle=await reader.decodeFromStream(media,video.value!,result=>{
      if(result&&run===generation){const content=result.getText();stop();emit('result',content)}
    })
    if(run!==generation)handle.stop();else controls=handle
  }catch(e:any){
    if(run!==generation)return
    const messages:Record<string,string>={NotAllowedError:'摄像头权限被拒绝，请在浏览器设置中允许访问，或使用扫码枪 / 手工输入',NotFoundError:'未找到摄像头，请连接摄像头或使用扫码枪 / 手工输入',NotReadableError:'摄像头被占用或无法启动，请关闭其他使用摄像头的应用后重试',OverconstrainedError:'当前摄像头不支持所需模式，请使用其他摄像头或扫码枪'}
    error.value=messages[e?.name]||e?.message||'摄像头启动失败，请重试或手工输入'
    stop()
  }finally{opening.value=false}
}
function visibilityChanged(){if(document.hidden)stop()}
watch(()=>props.disabled,value=>{if(value)stop()})
onMounted(()=>document.addEventListener('visibilitychange',visibilityChanged))
onDeactivated(stop)
onBeforeUnmount(()=>{document.removeEventListener('visibilitychange',visibilityChanged);stop()})
</script>
<style scoped>.asset-scanner video{display:block;width:100%;max-height:280px;background:#172033;border-radius:8px;margin-bottom:12px}.scanner-error{font-size:12px;color:#ba6060;line-height:1.7}.asset-scanner{margin:12px 0}</style>

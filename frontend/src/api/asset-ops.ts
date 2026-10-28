import { request } from './http'
import http from './http'
import * as XLSX from 'xlsx-js-style'
import QRCode from 'qrcode'
import { getScanAccessSettings } from './scan-access'
import { getDevice } from './device'
import { assetLabelUrl } from '@/utils/scanAccess'
export function ops<T=any>(path:string,method='GET',data?:unknown){return request<T>({url:`/api/v1/asset-ops/${path}`,method,data})}
export async function listOps(path:string):Promise<any[]>{return (await ops<{items:any[]}>(path)).items}
export const usageLabels:Record<string,string>={IDLE:'在库',IN_USE:'使用中',REPAIR:'维修中',RETIRED:'已报废'}
export const stockStates:Record<string,string>={AVAILABLE:'可用',RESERVED:'已预留',TESTING:'待检测',FAULTY:'故障',REPAIR:'维修中',RETIRED:'已报废',INSTALLED:'已安装',TRANSIT:'在途'}
export const moveLabels:Record<string,string>={IN:'入库',OUT:'领用出库',INSTALL:'安装',REMOVE:'拆回',TRANSFER:'调拨发出',RECEIVE:'收货确认',RESERVE:'预留',RELEASE:'释放预留',PASS:'检测合格',FAULT:'故障登记',REPAIR:'外送维修',RETIRE:'报废',ADJUST:'盘点调整'}
export const checkLabels:Record<string,string>={UNSCANNED:'未盘',NORMAL:'正常',DIFFERENCE:'存在差异',CONFLICT:'台账变化待核查'}
export function exportSheet(name:string,rows:Record<string,unknown>[],headers?:string[]){
  const safe=rows.map(row=>Object.fromEntries(Object.entries(row).map(([k,v])=>[k,typeof v==='object'&&v!==null?JSON.stringify(v):typeof v==='string'&&/^[=+@\-\t\r]/.test(v)?"'"+v:v])))
  const wb=XLSX.utils.book_new();const ws=XLSX.utils.json_to_sheet(safe,headers?{header:headers}:{})
  ws['!cols']=(headers||Object.keys(safe[0]||{})).map(()=>({wch:24}));ws['!autofilter']={ref:ws['!ref']||'A1:A1'};ws['!freeze']={xSplit:0,ySplit:1}
  for(const key of Object.keys(ws)){if(/^[A-Z]+1$/.test(key))ws[key].s={font:{bold:true,color:{rgb:'FFFFFF'}},fill:{fgColor:{rgb:'193451'}},alignment:{wrapText:true}}}
  XLSX.utils.book_append_sheet(wb,ws,name.slice(0,31));XLSX.writeFile(wb,`${name}-${new Date().toISOString().slice(0,10)}.xlsx`)
}
export type ScanKind='device'|'stock'
export function systemScanTarget(value:string):{kind:ScanKind;id:string}|null {
  try {
    const url=new URL(value.trim())
    if(!['http:','https:'].includes(url.protocol)||url.pathname!=='/asset-scan'||url.username||url.password)return null
    const keys=(['device','stock'] as const).filter(key=>url.searchParams.has(key))
    if(keys.length!==1||url.searchParams.getAll(keys[0]).length!==1)return null
    const id=url.searchParams.get(keys[0])!
    return /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(id)?{kind:keys[0],id:id.toLowerCase()}:null
  }catch{return null}
}
export function parsedScan(value:string){return systemScanTarget(value)?.id||value.trim()}
export function matchesAssetScan(item:{id?:string;code?:string;assetNumber?:string;serialNumber?:string;externalQrCode?:string;externalQrCodeUrl?:string},value:string,kind?:ScanKind){
  const raw=value.trim(),target=systemScanTarget(raw)
  if(!raw)return false
  // A system link identifies one object and must never fall through to another object's SN or alias.
  if(target)return (!kind||kind===target.kind)&&item.id?.toLowerCase()===target.id
  return item.id?.toLowerCase()===raw.toLowerCase()||Boolean(item.assetNumber&&item.assetNumber.toUpperCase()===raw.toUpperCase())||Boolean(item.code&&item.code.toUpperCase()===raw.toUpperCase())||Boolean(item.serialNumber&&item.serialNumber.toUpperCase()===raw.toUpperCase())||Boolean(item.externalQrCode&&item.externalQrCode.trim()===raw)||Boolean(item.externalQrCodeUrl&&item.externalQrCodeUrl.trim()===raw)
}
export function matchScan(lines:any[],value:string){return lines.filter(line=>{
  let device:any={}
  if(line.kind==='DEVICE'&&line.snapshot){try{const snapshot=typeof line.snapshot==='string'?JSON.parse(line.snapshot):line.snapshot;device=snapshot?.device||{}}catch{}}
  const kind=line.kind==='DEVICE'?'device':line.kind==='SPARE'?'stock':undefined
  return matchesAssetScan({...device,id:line.targetId,code:line.code,serialNumber:line.serialNumber},value,kind)
})}
export async function printAssetLabels(kind:ScanKind,items:any[],options:{reloadDevices?:boolean}={}){
  if(items.length<1||items.length>100)throw new Error('每次选择 1–100 个标签')
  // Reserve the window in the user's click, before requests/QR rendering lose the browser gesture.
  const popup=window.open('', '_blank')
  if(!popup)throw new Error('标签窗口被浏览器拦截，请允许本网站弹出窗口后重试')
  try{
    popup.opener=null
    popup.document.body.textContent='标签生成中，请稍候……'
    const settings=await getScanAccessSettings()
    let records=items
    if(kind==='device'&&options.reloadDevices){
      records=[]
      for(let start=0;start<items.length;start+=5)records.push(...await Promise.all(items.slice(start,start+5).map(item=>getDevice(item.id))))
    }
    const cards=await Promise.all(records.map(async item=>{
      const content=assetLabelUrl(settings.baseUrl,kind,item.id)
      const image=await QRCode.toDataURL(content,{width:360,margin:4,errorCorrectionLevel:'M'})
      return {image,code:String(item.code||item.serialNumber||item.id.slice(0,8)),name:String(item.name||item.spec?.name||''),sn:String(item.serialNumber||'')}
    }))
    function escape(value:string){return value.replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]!))}
    const html=`<!doctype html><html lang="zh-CN"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>IT 基础设施管理平台 · 资产标签</title><style>body{font-family:Arial,"Microsoft YaHei",sans-serif;margin:16px}.labels{display:flex;gap:10px;flex-wrap:wrap}article{display:flex;align-items:center;width:310px;min-height:126px;border:1px solid #bbb;break-inside:avoid;page-break-inside:avoid}img{height:120px;width:120px;flex:none}.label-text{min-width:0;padding:6px;overflow-wrap:anywhere}b{font-size:12px}p{font:12px monospace;margin:5px 0}small{font-size:11px}button{margin-bottom:16px}@media print{body{margin:0}button{display:none}}</style><button onclick="print()">打印 / 保存 PDF</button><div class="labels">${cards.map(c=>`<article><img src="${c.image}" alt="系统二维码"><div class="label-text"><b>IT 基础设施管理平台</b><p>${escape(c.code)}</p><small>${escape(c.name)}</small>${c.sn&&c.sn!==c.code?`<p>SN: ${escape(c.sn)}</p>`:''}</div></article>`).join('')}</div></html>`
    if(popup.closed)throw new Error('标签窗口已关闭，请重新生成')
    popup.document.open();popup.document.write(html);popup.document.close()
  }catch(e){if(!popup.closed)popup.close();throw e}
}
export async function exportBundle(){const r=await http.post('/api/v1/asset-ops/export',{}, {responseType:'blob'});const url=URL.createObjectURL(r.data);const a=document.createElement('a');a.href=url;a.download='asset-operations.json';a.click();setTimeout(()=>URL.revokeObjectURL(url),1000)}
export function errorMessage(e:any){return e?.response?.data?.message||e?.message||'操作失败'}


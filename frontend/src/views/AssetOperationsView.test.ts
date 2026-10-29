import ElementPlus from 'element-plus'
import { mount, flushPromises } from '@vue/test-utils'
import { afterEach, expect, it, vi } from 'vitest'
import AssetOperationsView from './AssetOperationsView.vue'

const api=vi.hoisted(()=>({list:vi.fn(),push:vi.fn(),ops:vi.fn(),exportSheet:vi.fn()}))
vi.mock('@/api/asset-ops',()=>({listOps:api.list,ops:api.ops,exportSheet:api.exportSheet,exportBundle:vi.fn(),printAssetLabels:vi.fn(),errorMessage:(e:Error)=>e.message,usageLabels:{IDLE:'在库',IN_USE:'使用中',REPAIR:'维修中',RETIRED:'已报废'}}))
vi.mock('@/api/resource',()=>({getResourceTree:async()=>({items:[]})}))
vi.mock('@/stores/auth',()=>({useAuthStore:()=>({user:{roles:[{code:'system_admin'}]}})}))
vi.mock('vue-router',()=>({useRouter:()=>({push:api.push}),useRoute:()=>({query:{}})}))
const wrappers: ReturnType<typeof mount>[]=[]
afterEach(()=>{wrappers.splice(0).forEach(w=>w.unmount());document.body.innerHTML='';vi.clearAllMocks()})

it('keeps the shared configuration overview without duplicate ledgers or common management tabs',async()=>{
  const row=(id:string,category:string)=>({device:{id,code:id,name:category==='PC'?'研发笔记本':'交易服务器',type:{category,name:category==='PC'?'笔记本':'服务器'}},profile:{usageStatus:'IDLE'},warranty:{status:'UNKNOWN',label:'信息待补全'},coverages:[],components:[]})
  api.list.mockImplementation(async(path:string)=>path==='devices'?[row('terminal-1','PC'),row('server-1','SERVER')]:[])
  const w=mount(AssetOperationsView,{attachTo:document.body,global:{plugins:[ElementPlus],stubs:{AssetAuditPanel:true}}});wrappers.push(w);await flushPromises()
  expect(w.text()).toContain('配置与保修中心')
  expect(w.text()).toContain('共用同一份设备档案')
  expect(w.text()).toContain('研发笔记本');expect(w.text()).toContain('交易服务器')
  expect(w.findAll('button').some(b=>b.text()==='登记设备资产')).toBe(false)
  expect(api.list.mock.calls.filter(c=>c[0]==='devices')).toHaveLength(1)
  expect(w.findAll('button').some(b=>['终端台账','机房设备台账'].includes(b.text()))).toBe(false)
  for(const text of ['区域与资产位置','操作记录','数据导出与备份','提醒中心']) expect(w.text()).not.toContain(text)
  expect(api.list.mock.calls.map(c=>c[0])).toEqual(['devices','sites'])
})

 it('exports the full sorted configuration collection and pages linked parts without changing the profile',async()=>{
 const components=Array.from({length:61},(_,i)=>({id:`part-${i+1}`,serialNumber:`SN${i+1}`,spec:{name:`部件${i+1}`,specification:'DDR5'}})).reverse()
 const rows=Array.from({length:61},(_,i)=>({device:{id:`d${i+1}`,code:`D${i+1}`,name:`设备${i+1}`,type:{category:'SERVER',name:'服务器'}},profile:{usageStatus:'IDLE',version:5},warranty:{status:'UNKNOWN',label:'待补全'},components,coverages:[]})).reverse()
 api.list.mockImplementation(async(path:string)=>path==='devices'?rows:[])
 const w=mount(AssetOperationsView,{attachTo:document.body,global:{plugins:[ElementPlus],stubs:{AssetAuditPanel:true}}});wrappers.push(w);await flushPromises()
 await w.get('button[aria-label="按设备 / 编号排序"]').trigger('click');await flushPromises()
 expect(w.findAll('.el-table__body tbody tr')).toHaveLength(20)
 expect(w.findAll('.el-table__body tbody tr')[0]?.text()).toContain('设备1')
 const vm=w.vm as any;vm.assetPage=4;await flushPromises()
 expect(w.findAll('.el-table__body tbody tr')).toHaveLength(1)
 vm.exportAssets()
 expect(api.exportSheet.mock.calls[api.exportSheet.mock.calls.length-1]?.[1]).toHaveLength(61)
 expect(api.exportSheet.mock.calls[api.exportSheet.mock.calls.length-1]?.[1][60].设备名称).toBe('设备61')
 vm.edit(rows[0]);await flushPromises()
 vm.sortComponentBy('spec.name');vm.componentPage=4;await flushPromises()
 expect(vm.componentRows).toHaveLength(1)
 expect(vm.componentRows[0].serialNumber).toBe('SN61')
 expect(vm.profile.version).toBe(5)
 expect(vm.current.components).toHaveLength(61)
 })

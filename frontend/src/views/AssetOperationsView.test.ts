import ElementPlus from 'element-plus'
import { mount, flushPromises } from '@vue/test-utils'
import { afterEach, expect, it, vi } from 'vitest'
import AssetOperationsView from './AssetOperationsView.vue'

const api=vi.hoisted(()=>({list:vi.fn(),push:vi.fn(),ops:vi.fn()}))
vi.mock('@/api/asset-ops',()=>({listOps:api.list,ops:api.ops,exportSheet:vi.fn(),exportBundle:vi.fn(),printAssetLabels:vi.fn(),errorMessage:(e:Error)=>e.message,usageLabels:{IDLE:'在库',IN_USE:'使用中',REPAIR:'维修中',RETIRED:'已报废'}}))
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

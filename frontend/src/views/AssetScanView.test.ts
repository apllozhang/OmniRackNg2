import ElementPlus, { ElMessage } from 'element-plus'
import { mount, flushPromises } from '@vue/test-utils'
import { beforeEach, expect, it, vi } from 'vitest'
import AssetScanView from './AssetScanView.vue'
const mocks=vi.hoisted(()=>({ list:vi.fn(), route:{query:{} as Record<string,string>,fullPath:'/asset-scan'} }))
vi.mock('@/api/asset-ops',async importOriginal=>({ ...await importOriginal<typeof import('@/api/asset-ops')>(),listOps:mocks.list }))
vi.mock('vue-router',async()=>{ const {reactive}=await import('vue');mocks.route=reactive(mocks.route);return {useRoute:()=>mocks.route,useRouter:()=>({push:vi.fn()})} })
const item={device:{id:'device-1',code:'DEV-001',name:'绑定设备',externalQrCode:'QR-2026-001',externalQrCodeUrl:'https://vendor.example/QR/001'},profile:{},warranty:{label:'在保'}}
beforeEach(()=>{ mocks.route.query={};mocks.route.fullPath='/asset-scan';vi.clearAllMocks(); mocks.list.mockImplementation(async path=>path==='devices'?[item]:[]) })
async function scan(value:string){
 const wrapper=mount(AssetScanView,{global:{plugins:[ElementPlus],stubs:{AssetScanner:true,ScanAccessLinks:true}}})
 await wrapper.find('input').setValue(value)
 await wrapper.findAll('button').find(b=>b.text()==='查询')!.trigger('click');await flushPromises()
 return wrapper
}
it.each(['QR-2026-001','https://vendor.example/QR/001'])('finds the same device using external content %s',async value=>{
 const wrapper=await scan(value)
 expect(wrapper.text()).toContain('绑定设备');expect(wrapper.text()).toContain('DEV-001');wrapper.unmount()
})
it('warns instead of picking the first duplicate external binding',async()=>{
 mocks.list.mockImplementation(async path=>path==='devices'?[item,{...item,device:{...item.device,id:'device-2'}}]:[])
 const warning=vi.spyOn(ElMessage,'warning')
 const wrapper=await scan('QR-2026-001')
 expect(warning).toHaveBeenCalledWith(expect.stringContaining('匹配多个对象'))
 expect(wrapper.text()).not.toContain('绑定设备');wrapper.unmount()
})

it('does not let an older lookup overwrite the latest input',async()=>{
 let first!:(items:unknown[])=>void
 let count=0
 mocks.list.mockImplementation(async path=>{
  if(path!=='devices')return []
  count++
  if(count===1)return new Promise(resolve=>{first=resolve})
  return [{...item,device:{...item.device,code:'NEW',name:'最新查询设备',externalQrCode:'NEW'}}]
 })
 const wrapper=mount(AssetScanView,{global:{plugins:[ElementPlus],stubs:{AssetScanner:true,ScanAccessLinks:true}}})
 await wrapper.find('input').setValue('QR-2026-001');await wrapper.find('input').trigger('keyup.enter')
 await wrapper.find('input').setValue('NEW');await wrapper.find('input').trigger('keyup.enter');await flushPromises()
 expect(wrapper.text()).toContain('最新查询设备')
 first([item]);await flushPromises()
 expect(wrapper.text()).toContain('最新查询设备');expect(wrapper.text()).not.toContain('绑定设备');wrapper.unmount()
})
it('clears a pending lookup when input becomes empty',async()=>{
 let first!:(items:unknown[])=>void
 mocks.list.mockImplementation(async path=>path==='devices'?new Promise(resolve=>{first=resolve}):[])
 const wrapper=mount(AssetScanView,{global:{plugins:[ElementPlus],stubs:{AssetScanner:true,ScanAccessLinks:true}}})
 await wrapper.find('input').setValue('QR-2026-001');await wrapper.find('input').trigger('keyup.enter')
 await wrapper.find('input').setValue('');await wrapper.find('input').trigger('keyup.enter')
 first([item]);await flushPromises()
 expect(wrapper.text()).not.toContain('绑定设备');wrapper.unmount()
})

it('clears a QR result when navigating back to the plain scan entry',async()=>{
 const id='a4345ec9-b768-4e60-a706-0a84e3246d21'
 mocks.list.mockImplementation(async path=>path==='devices'?[{...item,device:{...item.device,id}}]:[])
 mocks.route.fullPath=`/asset-scan?device=${id}`;mocks.route.query={device:id}
 const wrapper=mount(AssetScanView,{global:{plugins:[ElementPlus],stubs:{AssetScanner:true,ScanAccessLinks:true}}});await flushPromises()
 expect(wrapper.text()).toContain('绑定设备')
 mocks.route.fullPath='/asset-scan';mocks.route.query={};await flushPromises()
 expect(wrapper.text()).not.toContain('绑定设备');expect(wrapper.find('input').element.value).toBe('');wrapper.unmount()
})

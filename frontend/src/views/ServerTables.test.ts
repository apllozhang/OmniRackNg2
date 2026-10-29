import ElementPlus, {ElPagination} from 'element-plus'
import {mount,flushPromises} from '@vue/test-utils'
import {afterEach,expect,it,vi} from 'vitest'
import AssetAuditPanel from '@/components/AssetAuditPanel.vue'
import NotificationDeliveriesView from './NotificationDeliveriesView.vue'
const api=vi.hoisted(()=>({ops:vi.fn(),export:vi.fn(),history:vi.fn()}))
vi.mock('@/api/asset-ops',()=>({ops:api.ops,listOps:async()=>[],exportSheet:api.export,errorMessage:(e:Error)=>e.message}))
vi.mock('@/api/notifications',()=>({getNotificationDeliveries:api.history}))
const wrappers:ReturnType<typeof mount>[]=[]
afterEach(()=>{wrappers.splice(0).forEach(w=>w.unmount());document.body.innerHTML='';vi.clearAllMocks()})
it('requests global audit sorting and paging, and prevents exporting stale data after a load failure',async()=>{
 api.ops.mockResolvedValue({items:[{id:'a',createdAt:'2026-01-01',action:'CREATE',resourceType:'device',before:'{}',after:'{}'}],total:121})
 const w=mount(AssetAuditPanel,{attachTo:document.body,global:{plugins:[ElementPlus]}});wrappers.push(w);await flushPromises()
 const pager=w.findComponent(ElPagination)
 pager.vm.$emit('update:current-page',2);await flushPromises()
 expect(new URLSearchParams(api.ops.mock.calls[api.ops.mock.calls.length-1]![0].split('?')[1]).get('page')).toBe('2')
 await w.get('button[aria-label="按操作排序"]').trigger('click');await flushPromises()
 const query=new URLSearchParams(api.ops.mock.calls[api.ops.mock.calls.length-1]![0].split('?')[1])
 expect(query.get('page')).toBe('1');expect(query.get('sortBy')).toBe('action');expect(query.get('sortOrder')).toBe('asc')
 api.ops.mockRejectedValueOnce(new Error('offline'))
 pager.vm.$emit('update:current-page',2);await flushPromises()
 const exportButton=w.findAll('button').find(b=>b.text()==='导出当前页')!
 expect(exportButton.attributes('disabled')).toBeDefined();await exportButton.trigger('click');expect(api.export).not.toHaveBeenCalled()
 expect(w.text()).toContain('加载失败')
})
it('sends notification sorting to the API and resets to page one when sorting or filtering',async()=>{
 api.history.mockResolvedValue({items:[{id:'n',createdAt:'2026-01-01',channel:'EMAIL',status:'FAILED',summary:'通知',errorMessage:'失败'}],total:61})
 const w=mount(NotificationDeliveriesView,{attachTo:document.body,global:{plugins:[ElementPlus]}});wrappers.push(w);await flushPromises()
 w.findComponent(ElPagination).vm.$emit('update:current-page',3);await flushPromises()
 expect(api.history).toHaveBeenLastCalledWith(3,{page:3,pageSize:20})
 await w.get('button[aria-label="按发送时间排序"]').trigger('click');await flushPromises()
 expect(api.history).toHaveBeenLastCalledWith(1,{page:1,pageSize:20,sortBy:'createdAt',sortOrder:'asc'})
 await w.get('input[aria-label="搜索通知发送记录"]').setValue('特定通知')
 await new Promise(resolve=>setTimeout(resolve,330));await flushPromises()
 expect(api.history).toHaveBeenLastCalledWith(1,{page:1,pageSize:20,sortBy:'createdAt',sortOrder:'asc',search:'特定通知'})
})

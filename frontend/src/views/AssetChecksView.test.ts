import ElementPlus from 'element-plus'
import {mount,flushPromises} from '@vue/test-utils'
import {beforeEach,afterEach,expect,it,vi} from 'vitest'
import AssetChecksView from './AssetChecksView.vue'
const mocks=vi.hoisted(()=>({list:vi.fn(),ops:vi.fn()}))
vi.mock('@/api/asset-ops',async importOriginal=>({...await importOriginal<typeof import('@/api/asset-ops')>(),listOps:mocks.list,ops:mocks.ops}))
vi.mock('@/stores/auth',()=>({useAuthStore:()=>({user:{id:'u1',roles:[{code:'asset_manager'}]}})}))
const tasks=['a','b'].map(id=>({id,name:`任务${id}`,state:'ACTIVE',assigneeId:'u1',site:{name:'示例机房'}}))
const row=(taskId:string,name:string)=>({id:`line-${taskId}`,taskId,name,kind:'DEVICE',targetId:taskId,code:`DEV-${taskId}`,expected:1,result:'UNSCANNED'})
beforeEach(()=>{
 vi.clearAllMocks();mocks.ops.mockResolvedValue({duplicate:false})
 mocks.list.mockImplementation(async path=>path==='checks'?tasks:path.endsWith('/lines')?[row(path.split('/')[1],'设备'+path.split('/')[1])]:[])
})
afterEach(()=>{document.body.innerHTML=''})
async function render(){const wrapper=mount(AssetChecksView,{attachTo:document.body,global:{plugins:[ElementPlus],stubs:{AssetScanner:true,ScanAccessLinks:true}}});await flushPromises();return wrapper}
it('discards stale task details after a rapid task switch',async()=>{
 let release!:(value:unknown[])=>void
 mocks.list.mockImplementation(async path=>path==='checks'?tasks:path==='checks/a/lines'?new Promise(resolve=>{release=resolve}):path==='checks/b/lines'?[row('b','当前设备B')]:[])
 const wrapper=await render()
 await wrapper.findAll('.ops-task-card')[0].trigger('click')
 expect(wrapper.findComponent({name:'AssetScanner'}).exists()).toBe(false)
 await wrapper.findAll('.ops-task-card')[1].trigger('click');await flushPromises()
 expect(wrapper.text()).toContain('当前设备B')
 release([row('a','旧设备A')]);await flushPromises()
 expect(wrapper.text()).toContain('当前设备B');expect(wrapper.text()).not.toContain('旧设备A');wrapper.unmount()
})
it('clears prior rows and does not enable scanning when details fail to load',async()=>{
 const wrapper=await render();await wrapper.findAll('.ops-task-card')[0].trigger('click');await flushPromises()
 mocks.list.mockImplementation(async path=>{if(path==='checks/b/lines')throw new Error('明细不可用');return []})
 await wrapper.findAll('.ops-task-card')[1].trigger('click');await flushPromises()
 expect(wrapper.findComponent({name:'AssetScanner'}).exists()).toBe(false)
 expect(wrapper.text()).not.toContain('设备a');wrapper.unmount()
})
it('stops scanning during confirmation and submits once to the captured task',async()=>{
 let finish!:(value:unknown)=>void
 mocks.ops.mockImplementation(()=>new Promise(resolve=>{finish=resolve}))
 const wrapper=await render();await wrapper.findAll('.ops-task-card')[0].trigger('click');await flushPromises()
 await wrapper.findAll('button').find(b=>b.text()==='核对')!.trigger('click');await flushPromises()
 expect(wrapper.findComponent({name:'AssetScanner'}).attributes('disabled')).toBe('true')
 expect(wrapper.findAll('.ops-task-card').every(button=>button.attributes('disabled')!==undefined)).toBe(true)
 const confirm=Array.from(document.querySelectorAll('button')).find(b=>b.textContent?.trim()==='确认盘点')!
 confirm.click();confirm.click();await flushPromises()
 expect(mocks.ops).toHaveBeenCalledTimes(1)
 expect(mocks.ops).toHaveBeenCalledWith('checks/a/scan','POST',expect.objectContaining({lineId:'line-a',actual:1}))
 finish({duplicate:false});await flushPromises();wrapper.unmount()
})

it('opens the new task after creation while its save operation is still finishing',async()=>{
 mocks.ops.mockResolvedValue({id:'c',name:'新建任务C',state:'ACTIVE',assigneeId:'u1',site:{name:'示例机房'}})
 const wrapper=await render()
 await wrapper.findAll('button').find(b=>b.text()==='创建盘点任务')!.trigger('click');await flushPromises()
 const create=Array.from(document.querySelectorAll('button')).find(b=>b.textContent?.trim()==='创建任务与快照')!
 create.click();await flushPromises()
 expect(mocks.list).toHaveBeenCalledWith('checks/c/lines')
 expect(wrapper.text()).toContain('新建任务C');expect(wrapper.text()).toContain('设备c');wrapper.unmount()
})

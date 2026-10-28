import ElementPlus from 'element-plus'
import {mount,flushPromises} from '@vue/test-utils'
import {beforeEach,afterEach,expect,it,vi} from 'vitest'
import AssetChecksView from './AssetChecksView.vue'
const mocks=vi.hoisted(()=>({list:vi.fn(),ops:vi.fn(),export:vi.fn()}))
vi.mock('@/api/asset-ops',async importOriginal=>({...await importOriginal<typeof import('@/api/asset-ops')>(),listOps:mocks.list,ops:mocks.ops,exportSheet:mocks.export}))
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

it('pages and filters both check tables while scanning and export retain the full task scope',async()=>{
 const lines=Array.from({length:61},(_,i)=>({...row('a',`设备${61-i}`),id:`line-${61-i}`,code:`DEV${61-i}`,targetId:`d-${61-i}`}))
 const discoveries=Array.from({length:61},(_,i)=>({id:`f-${61-i}`,code:`F-${61-i}`,note:`现场${61-i}`,resolution:''}))
 mocks.list.mockImplementation(async path=>path==='checks'?tasks:path==='checks/a/lines'?lines:path==='checks/a/discoveries'?discoveries:[])
 const w=await render();await w.findAll('.ops-task-card')[0]!.trigger('click');await flushPromises()
 const tables=()=>w.findAll('.ops-check-main .el-table')
 expect(tables()[0]!.findAll('.el-table__body tbody tr')).toHaveLength(20)
 expect(tables()[1]!.findAll('.el-table__body tbody tr')).toHaveLength(20)
 await w.get('button[aria-label="按设备 / 备件排序"]').trigger('click');await flushPromises()
 expect(tables()[0]!.find('.el-table__body tbody tr').text()).toContain('设备1')
 const pagers=()=>w.findAll('nav.table-pager')
 await pagers()[0]!.get('.btn-next').trigger('click');await flushPromises()
 expect(tables()[0]!.find('.el-table__body tbody tr').text()).toContain('设备21')
 await w.get('button[aria-label="按识别码排序"]').trigger('click');await flushPromises()
 await pagers()[1]!.get('.btn-next').trigger('click');await flushPromises()
 expect(tables()[1]!.find('.el-table__body tbody tr').text()).toContain('F-21')
 expect(pagers()[0]!.get('.el-pager .is-active').text()).toBe('2')
 await w.get('input[aria-label="搜索盘点明细"]').setValue('设备1')
 await w.get('input[aria-label="搜索发现记录"]').setValue('现场61')
 await new Promise(resolve=>setTimeout(resolve,330));await flushPromises()
 await w.findAll('button').find(b=>b.text()==='导出全部结果')!.trigger('click')
 const exported=mocks.export.mock.calls.find(call=>call[0]==='盘点结果')![1]
 const findings=mocks.export.mock.calls.find(call=>call[0]==='盘点发现记录')![1]
 expect(exported).toHaveLength(61);expect(exported[0].名称).toBe('设备1');expect(exported[60].名称).toBe('设备61')
 expect(findings).toHaveLength(61);expect(findings[0].识别码).toBe('F-1');expect(findings[60].识别码).toBe('F-61')
 const scan=w.get('input[placeholder="扫描标签，或输入资产编号 / SN"]')
 await scan.setValue('DEV61');await w.findAll('button').find(b=>b.text()==='识别核对')!.trigger('click');await flushPromises()
 expect(document.body.textContent).toContain('核对现场实物')
 expect(document.body.textContent).toContain('设备61')
 expect(mocks.ops).not.toHaveBeenCalled()
 w.unmount()
})

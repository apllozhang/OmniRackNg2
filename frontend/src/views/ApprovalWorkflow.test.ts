import ElementPlus,{ElMessageBox} from 'element-plus'
import {mount,flushPromises} from '@vue/test-utils'
import {afterEach,beforeEach,expect,it,vi} from 'vitest'
import ApprovalCenterView from './ApprovalCenterView.vue'
import ApprovalRulesView from './ApprovalRulesView.vue'
import NotificationDeliveriesView from './NotificationDeliveriesView.vue'

const api=vi.hoisted(()=>({list:vi.fn(),approve:vi.fn(),reject:vi.fn(),policy:vi.fn(),save:vi.fn(),history:vi.fn()}))
vi.mock('@/api/admin',()=>({listAdminApprovals:api.list,approveAdminOperation:api.approve,rejectAdminOperation:api.reject,getAdminApprovalPolicy:api.policy,updateAdminApprovalPolicy:api.save}))
vi.mock('@/api/notifications',()=>({getNotificationDeliveries:api.history}))
vi.mock('@/api/resource',()=>({getResourceTree:async()=>({items:[{name:'上海',rooms:[{name:'办公机房',racks:[{id:'rack-1',name:'A01'}]}]}]})}))
vi.mock('@/api/asset-ops',()=>({errorMessage:(e:Error)=>e.message}))
const wrappers:ReturnType<typeof mount>[]=[]
const policy={id:'policy',version:3,assignApprovalEnabled:true,moveApprovalEnabled:true,decommissionApprovalEnabled:true,defaultApproverRole:'system_admin',approvalMode:'SINGLE'}
function render(view:any){const w=mount(view,{attachTo:document.body,global:{plugins:[ElementPlus],stubs:{ApprovalEmailSettings:{template:'<p>审批邮件通知</p>'}}}});wrappers.push(w);return w}
beforeEach(()=>{
 api.list.mockResolvedValue({items:[{id:'a1',version:2,operation:'DECOMMISSION',status:'PENDING',device:{name:'交易服务器',code:'DEV-001'},sourcePosition:{rackId:'rack-1',startU:5,heightU:2},targetRackId:'wrong-target',targetStartU:0,targetHeightU:0,requestedAt:'2026-10-20T08:00:00Z',reason:'设备退役'}]})
 api.approve.mockResolvedValue({});api.reject.mockResolvedValue({});api.policy.mockResolvedValue({...policy});api.save.mockResolvedValue({...policy,version:4})
 api.history.mockResolvedValue({items:[{id:'n1',channel:'EMAIL',createdAt:'2026-10-20T08:00:00Z',test:false,status:'FAILED',summary:'待审批 · a1',errorMessage:'SMTP 认证失败'}],total:21})
})
afterEach(()=>{wrappers.splice(0).forEach(w=>w.unmount());document.body.innerHTML='';vi.restoreAllMocks();vi.clearAllMocks()})

it('shows decommission source location and approves with optimistic version',async()=>{
 vi.spyOn(ElMessageBox,'prompt').mockImplementation(async()=>({value:'已确认退役',action:'confirm'} as never))
 const w=render(ApprovalCenterView);await flushPromises()
 expect(w.text()).toContain('下架');expect(w.text()).toContain('上海 / 办公机房 / A01 · 5-6U');expect(w.text()).not.toContain('wrong-target')
 await w.findAll('button').find(b=>b.text()==='通过')!.trigger('click');await flushPromises()
 expect(api.approve).toHaveBeenCalledWith('a1',2,'已确认退役')
 expect(api.list).toHaveBeenCalledTimes(2)
})
it('rejects through the reject endpoint and leaves failures visible for retry',async()=>{
 vi.spyOn(ElMessageBox,'prompt').mockImplementation(async()=>({value:'继续使用',action:'confirm'} as never))
 api.reject.mockRejectedValue(new Error('记录已更新，请刷新'))
 const w=render(ApprovalCenterView);await flushPromises()
 await w.findAll('button').find(b=>b.text()==='驳回')!.trigger('click');await flushPromises()
 expect(api.reject).toHaveBeenCalledWith('a1',2,'继续使用');expect(api.list).toHaveBeenCalledTimes(1)
 expect(w.findAll('button').find(b=>b.text()==='驳回')!.attributes('disabled')).toBeUndefined()
})
it('saves the independent decommission approval switch and version',async()=>{
 const w=render(ApprovalRulesView);await flushPromises()
 const item=w.findAll('.el-form-item').find(e=>e.text().includes('下架需要审批'))!
 await item.find('input').setValue(false)
 await w.findAll('button').find(b=>b.text()==='保存审批规则')!.trigger('click');await flushPromises()
 expect(api.save).toHaveBeenCalledWith(expect.objectContaining({version:3,decommissionApprovalEnabled:false,defaultApproverRole:'system_admin'}))
 expect(w.text()).toContain('审批邮件通知')
})
it('does not save defaults after approval settings fail to load',async()=>{
 api.policy.mockRejectedValue(new Error('连接失败'))
 const w=render(ApprovalRulesView);await flushPromises()
 expect(w.findAll('button').some(b=>b.text()==='保存审批规则')).toBe(false);expect(api.save).not.toHaveBeenCalled()
})
it('shows delivery failures independently and loads server pagination',async()=>{
 const w=render(NotificationDeliveriesView);await flushPromises()
 expect(w.text()).toContain('SMTP 认证失败');expect(w.text()).toContain('发送失败');expect(w.text()).toContain('待审批 · a1')
 await w.find('.el-pagination .btn-next').trigger('click');await flushPromises()
 expect(api.history).toHaveBeenLastCalledWith(2,{page:2,pageSize:20})
})

it('sorts and pages all approvals, preserving decision ID/version and filter recovery',async()=>{
 const rows=Array.from({length:61},(_,i)=>{const n=61-i;return {id:`a${n}`,version:n,operation:'ASSIGN',status:'PENDING',deviceId:`d${n}`,device:{name:`设备${n}`,code:`DEV${n}`},targetRackId:'rack-1',targetStartU:1,targetHeightU:1,requestedAt:'2026-01-01T00:00:00Z',reason:'验收'}})
 api.list.mockResolvedValue({items:rows})
 vi.spyOn(ElMessageBox,'prompt').mockImplementation(async()=>({value:'核查通过',action:'confirm'} as never))
 const w=render(ApprovalCenterView);await flushPromises()
 const rendered=()=>w.findAll('.el-table__body tbody tr')
 expect(rendered()).toHaveLength(20)
 await w.get('button[aria-label="按设备 / 编码排序"]').trigger('click');await flushPromises()
 expect(rendered()[0]!.text()).toContain('设备1')
 await w.get('nav[aria-label="审批记录分页"] .btn-next').trigger('click');await flushPromises()
 expect(rendered()[0]!.text()).toContain('设备21')
 await rendered()[0]!.findAll('button').find(b=>b.text()==='通过')!.trigger('click');await flushPromises()
 expect(api.approve).toHaveBeenCalledWith('a21',21,'核查通过')
 await w.get('input[aria-label="搜索审批记录"]').setValue('设备61')
 await new Promise(resolve=>setTimeout(resolve,330));await flushPromises()
 expect(rendered()).toHaveLength(1);expect(rendered()[0]!.text()).toContain('设备61')
 await w.get('input[aria-label="搜索审批记录"]').setValue('missing')
 await new Promise(resolve=>setTimeout(resolve,330));await flushPromises()
 await w.findAll('button').find(b=>b.text()==='清除筛选')!.trigger('click');await flushPromises()
 expect(rendered()).toHaveLength(20)
})

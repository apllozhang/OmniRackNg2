import ElementPlus, { ElMessageBox } from 'element-plus'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import AssetRemindersView from './AssetRemindersView.vue'
import AssetDataView from './AssetDataView.vue'
import AssetAuditView from './AssetAuditView.vue'
import DefaultLayout from '@/layouts/DefaultLayout.vue'

const api=vi.hoisted(()=>({list:vi.fn(),ops:vi.fn(),bundle:vi.fn(),role:'system_admin',path:'/pc-assets'}))
vi.mock('@/api/asset-ops',()=>({listOps:api.list,ops:api.ops,exportBundle:api.bundle,errorMessage:(e:Error)=>e.message}))
vi.mock('@/stores/auth',()=>({useAuthStore:()=>({user:{displayName:'管理员',roles:[{code:api.role}]},isAdmin:api.role==='system_admin'})}))
vi.mock('vue-router',()=>({useRouter:()=>({push:vi.fn()}),useRoute:()=>({path:api.path})}))
const wrappers:ReturnType<typeof mount>[]=[]
function render(view:any){const w=mount(view,{attachTo:document.body,global:{plugins:[ElementPlus],stubs:{AssetAuditPanel:{template:'<p>审计明细组件</p>'},RouterLink:{template:'<a><slot/></a>'},RouterView:true}}});wrappers.push(w);return w}
beforeEach(()=>{api.role='system_admin';api.path='/pc-assets';api.list.mockResolvedValue([{id:'r1',title:'即将过保',severity:'DANGER'}]);api.ops.mockResolvedValue({});api.bundle.mockResolvedValue(undefined)})
afterEach(()=>{wrappers.splice(0).forEach(w=>w.unmount());document.body.innerHTML='';vi.restoreAllMocks();vi.clearAllMocks()})

it('refreshes reminders and records acknowledgement from the standalone page',async()=>{
  vi.spyOn(ElMessageBox,'prompt').mockImplementation(async()=>({value:'已联系服务商',action:'confirm'} as never))
  const w=render(AssetRemindersView);await flushPromises()
  expect(w.text()).toContain('即将过保')
  await w.findAll('button').find(b=>b.text()==='立即检查')!.trigger('click');await flushPromises()
  expect(api.ops).toHaveBeenCalledWith('reminders/refresh','POST')
  await w.findAll('button').find(b=>b.text()==='记录处理')!.trigger('click');await flushPromises()
  expect(api.ops).toHaveBeenCalledWith('reminders/r1/acknowledge','POST',{note:'已联系服务商'})
  expect(api.list.mock.calls.every(c=>c[0]==='reminders')).toBe(true)
})
it('keeps reminder actions read only for auditors while retaining business export',async()=>{
  api.role='asset_auditor'
  const w=render(AssetRemindersView);await flushPromises()
  expect(w.text()).toContain('即将过保');expect(w.findAll('button').map(b=>b.text())).not.toContain('立即检查');expect(w.findAll('button').map(b=>b.text())).not.toContain('记录处理');expect(api.ops).not.toHaveBeenCalled()
  const data=render(AssetDataView)
  await data.findAll('button').find(b=>b.text()==='导出完整业务 JSON')!.trigger('click');await flushPromises()
  expect(api.bundle).toHaveBeenCalledTimes(1)
  expect(data.text()).toContain('backup-assets.sh');expect(data.text()).toContain('--verify-only')
})
it('reuses the audit panel on its dedicated page',()=>{
  expect(render(AssetAuditView).text()).toContain('审计明细组件')
})
it('has one canonical menu entry for each ledger and exposes common pages by role',async()=>{
  const w=render(DefaultLayout);await flushPromises()
  const entries=w.findAll('.el-menu-item').map(e=>e.text())
  for(const text of ['终端台账','机房设备台账','区域与资产位置','资产操作记录','数据导出与备份']) expect(entries.filter(e=>e===text)).toHaveLength(1)
  expect(w.find('.el-menu-item.is-active').text()).toBe('终端台账')
  expect(w.findAll('.nav-section').map(e=>e.text())).toEqual(['工作台','资产管理','机房资源','基础资料','系统管理'])
  expect(w.text()).toContain('系统管理')
  for(const label of ['审批中心','审批规则','通知发送记录'])expect(entries.filter(e=>e===label)).toHaveLength(1)
})
it.each(['asset_manager','asset_auditor'])('retains common pages for %s without admin settings',async role=>{
  api.role=role
  const w=render(DefaultLayout);await flushPromises()
  expect(w.text()).toContain('系统管理');expect(w.text()).toContain('区域与资产位置')
  expect(w.text()).not.toContain('账号与认证')
  for(const label of ['审批中心','审批规则','通知发送记录'])expect(w.text()).not.toContain(label)
})
it('hides asset administration pages from users without asset roles',async()=>{
  api.role='viewer'
  const w=render(DefaultLayout);await flushPromises()
  expect(w.text()).not.toContain('系统管理');expect(w.text()).not.toContain('区域与资产位置')
})

import ElementPlus, { ElPagination } from 'element-plus'
import { mount, flushPromises } from '@vue/test-utils'
import { afterEach, expect, it, vi } from 'vitest'
import AssetRemindersView from './AssetRemindersView.vue'
import RackTemplateView from './RackTemplateView.vue'

const api = vi.hoisted(() => ({ list: vi.fn(), templates: vi.fn(), write: vi.fn() }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ user: { roles: [{ code: 'asset_auditor' }] } }) }))
vi.mock('@/api/asset-ops', () => ({ listOps: api.list, ops: api.write, errorMessage: (e: Error) => e.message }))
vi.mock('@/api/rack-template', () => ({ getRackTemplates: api.templates, createRackTemplate: api.write, createRackTemplateVersion: api.write, deleteRackTemplate: api.write, updateRackTemplate: api.write }))
const wrappers: ReturnType<typeof mount>[] = []
afterEach(() => { wrappers.splice(0).forEach(w => w.unmount()); document.body.innerHTML = ''; vi.clearAllMocks() })

it('reminders page and sort displayed severity, preserving read-only permissions and filter recovery', async () => {
  api.list.mockResolvedValue(Array.from({length:61}, (_,index) => ({id:`r-${index}`,title:`提醒${61-index}`,detail:'现场检查',severity:index===60?'DANGER':'WARNING'})))
  const w = mount(AssetRemindersView,{attachTo:document.body,global:{plugins:[ElementPlus]}});wrappers.push(w);await flushPromises()
  const rows = () => w.findAll('.el-table__body tbody tr')
  const pager = w.findComponent(ElPagination)
  expect(rows()).toHaveLength(20)
  await w.get('button[aria-label="按提醒排序"]').trigger('click');await flushPromises()
  expect(rows()[0]!.text()).toContain('提醒1')
  pager.vm.$emit('update:current-page',2);await flushPromises()
  expect(rows()[0]!.text()).toContain('提醒21')
  await w.get('button[aria-label="按级别排序"]').trigger('click');await flushPromises()
  expect(pager.props('currentPage')).toBe(1)
  expect(rows()[0]!.text()).toContain('高关注')
  const search=w.get('input[aria-label="搜索提醒或处理依据"]')
  await search.setValue('missing');await new Promise(resolve=>setTimeout(resolve,330));await flushPromises()
  expect(rows()).toHaveLength(0)
  await w.findAll('button').find(b=>b.text()==='清除筛选')!.trigger('click');await flushPromises()
  expect(pager.props('total')).toBe(61)
  expect(w.findAll('button').map(b=>b.text())).not.toContain('记录处理')
  expect(api.write).not.toHaveBeenCalled()
})

it('rack templates sort numeric revision/specification globally and filter without publishing or editing', async () => {
  const items=Array.from({length:61},(_,index)=>({id:`t-${index}`,code:`T${61-index}`,name:`模板${61-index}`,status:'ACTIVE',currentRevision:61-index,versions:[{revision:61-index,uHeight:61-index,pduCount:61-index}]}))
  api.templates.mockResolvedValue({items})
  const w=mount(RackTemplateView,{attachTo:document.body,global:{plugins:[ElementPlus]}});wrappers.push(w);await flushPromises()
  const rows=()=>w.findAll('.el-table__body tbody tr')
  const pager=w.findComponent(ElPagination)
  await w.get('button[aria-label="按当前版本排序"]').trigger('click');await flushPromises()
  expect(rows()[0]!.text()).toContain('V1')
  pager.vm.$emit('update:current-page',3);await flushPromises()
  expect(rows()[0]!.text()).toContain('V41')
  await w.get('button[aria-label="按主要规格排序"]').trigger('click');await flushPromises()
  expect(pager.props('currentPage')).toBe(1)
  expect(rows()[0]!.text()).toContain('1U')
  await w.get('input[aria-label="搜索模板编码或名称"]').setValue('模板61')
  await new Promise(resolve=>setTimeout(resolve,330));await flushPromises()
  expect(pager.props('total')).toBe(1)
  expect(rows()).toHaveLength(1)
  expect(rows()[0]!.text()).toContain('模板61')
  expect(api.write).not.toHaveBeenCalled()
})

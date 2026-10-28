import ElementPlus, { ElMessageBox, ElPagination } from 'element-plus'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import AssetLocationsView from './AssetLocationsView.vue'

const mocks = vi.hoisted(() => ({ ops: vi.fn(), list: vi.fn(), role: 'asset_manager' }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ user: { roles: [{ code: mocks.role }] } }) }))
vi.mock('vue-router', () => ({ useRouter: () => ({ push: vi.fn() }), useRoute: () => ({ query: { tab: 'sites' } }) }))
vi.mock('@/api/resource', () => ({ getResourceTree: async () => ({ items: [] }) }))
vi.mock('@/api/asset-ops', () => ({ ops: mocks.ops, listOps: mocks.list, errorMessage: (e: Error) => e.message, usageLabels: {}, exportSheet: vi.fn(), exportBundle: vi.fn(), printAssetLabels: vi.fn() }))
const site = { id: 'site-one', version: 4, name: '上海办公区', region: '上海', kind: 'OFFICE', roomId: null, address: '原地址' }
const wrappers: ReturnType<typeof mount>[] = []
const render = async () => { const w = mount(AssetLocationsView, { attachTo: document.body, global: { plugins: [ElementPlus] } }); wrappers.push(w); await flushPromises(); return w }
beforeEach(() => { mocks.role = 'asset_manager'; mocks.list.mockImplementation(async (key: string) => key === 'sites' ? [{ ...site }] : []); mocks.ops.mockResolvedValue({}) })
afterEach(() => { wrappers.splice(0).forEach(w => w.unmount()); document.body.innerHTML = '' })

describe('asset location maintenance', () => {
  it('edits an existing location through PUT with its current version', async () => {
    const w = await render()
    await w.findAll('button').find(b => b.text() === '编辑')!.trigger('click'); await flushPromises()
    const field = w.findAll('.el-dialog .el-form-item').find(x => x.text().startsWith('区域'))!
    await field.find('input').setValue('杭州')
    await w.findAll('.el-dialog button').find(b => b.text() === '保存修改')!.trigger('click'); await flushPromises()
    expect(mocks.ops).toHaveBeenCalledWith('sites/site-one', 'PUT', expect.objectContaining({ version: 4, region: '杭州', roomId: null }))
  })
  it('requires confirmation and passes the version when deleting', async () => {
    vi.spyOn(ElMessageBox, 'confirm').mockImplementation(async () => undefined as never)
    const w = await render()
    await w.findAll('button').find(b => b.text() === '删除')!.trigger('click'); await flushPromises()
    expect(mocks.ops).toHaveBeenCalledWith('sites/site-one?version=4', 'DELETE')
  })
  it('does not show mutation controls to an auditor', async () => {
    mocks.role = 'asset_auditor'
    const w = await render()
    const controls = w.findAll('button').map(button => button.text())
    expect(controls).not.toContain('新增位置'); expect(controls).not.toContain('编辑'); expect(controls).not.toContain('删除')
  })
})

it('sorts the complete location collection before paging and resets after filtering', async () => {
  const sites = Array.from({length:61}, (_, index) => ({...site, id:`site-${index+1}`, name:`位置${61-index}`}))
  mocks.list.mockResolvedValue(sites)
  const w = await render()
  const rows = () => w.findAll('.el-table__body tbody tr')
  const pager = w.findComponent(ElPagination)
  expect(rows()).toHaveLength(20)
  expect(rows()[0]!.text()).toContain('位置61')
  await w.get('button[aria-label="按位置名称排序"]').trigger('click'); await flushPromises()
  expect(rows()[0]!.text()).toContain('位置1')
  pager.vm.$emit('update:current-page', 3); await flushPromises()
  expect(rows()[0]!.text()).toContain('位置41')
  await w.get('input[aria-label="搜索区域、位置或地址"]').setValue('位置61')
  await new Promise(resolve => setTimeout(resolve, 330)); await flushPromises()
  expect(pager.props('currentPage')).toBe(1)
  expect(pager.props('total')).toBe(1)
  expect(rows()).toHaveLength(1); expect(rows()[0]!.text()).toContain('位置61')
  await w.get('input[aria-label="搜索区域、位置或地址"]').setValue('不存在')
  await new Promise(resolve => setTimeout(resolve, 330)); await flushPromises()
  expect(w.text()).toContain('没有匹配结果')
  await w.findAll('button').find(button => button.text()==='清除筛选')!.trigger('click'); await flushPromises()
  expect(pager.props('total')).toBe(61)
  pager.vm.$emit('update:page-size',50); await flushPromises()
  expect(rows()).toHaveLength(50); expect(pager.props('currentPage')).toBe(1)
  expect(mocks.ops).not.toHaveBeenCalled()
})

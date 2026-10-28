import ElementPlus, { ElMessageBox } from 'element-plus'
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
    expect(w.findAll('button')).toHaveLength(0)
  })
})

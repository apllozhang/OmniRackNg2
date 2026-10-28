import ElementPlus, { ElMessageBox, ElSelect, ElDropdown } from 'element-plus'
import { flushPromises, mount, DOMWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import SparePartsView from './SparePartsView.vue'

const mocks = vi.hoisted(() => ({ ops: vi.fn(), list: vi.fn(),export:vi.fn() }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ user: { roles: [{ code: 'asset_manager' }] } }) }))
vi.mock('@/api/asset-ops', () => ({ ops: mocks.ops, listOps: mocks.list, errorMessage: (e: Error) => e.message, stockStates: { AVAILABLE: '可用',TRANSIT:'在途',TESTING:'待检测',FAULTY:'故障',REPAIR:'维修中' }, moveLabels: {IN:'入库',TRANSFER:'调拨发出',RECEIVE:'收货确认'}, exportSheet: mocks.export, printAssetLabels: vi.fn() }))
const spec = { id: 'spec-one', version: 3, name: '服务器内存', category: '内存', partNumber: 'MEM-32G', specification: '32GB', tracking: 'SERIAL', unit: '件', compatibility: '原说明' }
const site = { id: 'site-one', name: '上海机房', region: '上海' }
const stock = { id: 'stock-one', version: 7, specId: spec.id, spec, siteId: site.id, site, quantity: 1, state: 'AVAILABLE', serialNumber: 'MEM-001', notes:'交易服务器备用',bin: 'S01', batch: '批次A', warrantyExpiresAt: null }
const draft = { ...stock, id: 'stock-draft', quantity: 0, serialNumber: 'MEM-002', version: 1 }
const minimum = { id: 'minimum-one', version: 5, specId: spec.id, spec, siteId: site.id, site, minimum: 2 }
const body=()=>new DOMWrapper(document.body)
const wrappers: ReturnType<typeof mount>[] = []
const render = async () => { const w = mount(SparePartsView, { attachTo: document.body, global: { plugins: [ElementPlus] } }); wrappers.push(w); await flushPromises(); return w }
beforeEach(() => { mocks.list.mockImplementation(async (key: string) => ({ stocks: [stock, draft], specs: [spec], sites: [site], minimums: [minimum], movements: [{ id: 'move', stockId: stock.id, stock, beforeQuantity: 0, afterQuantity: 1 }], devices: [] }[key] || [])); mocks.ops.mockResolvedValue({}) })
afterEach(() => { wrappers.splice(0).forEach(w => w.unmount()); document.body.innerHTML = '' })

describe('spare catalog maintenance', () => {
  it('provides direct transfer and full-batch receipt with explicit inventory effects',async()=>{
    const target={id:'site-two',region:'杭州',name:'杭州机房'}
    const transit={...stock,id:'in-transit',spec:{...spec,tracking:'QUANTITY'},quantity:5,state:'TRANSIT',destinationSiteId:target.id}
    mocks.list.mockImplementation(async(key:string)=>({stocks:[stock,transit],specs:[spec],sites:[site,target],minimums:[],movements:[],devices:[]}[key]||[]))
    const w=await render()
    await w.find('#pane-stocks').findAll('button').find(b=>b.text()==='调拨')!.trigger('click');await flushPromises()
    expect(w.find('.el-dialog').text()).toContain('不计入可用库存')
    await w.findAll('.el-dialog button').find(b=>b.text()==='取消')!.trigger('click');await flushPromises()
    await w.find('#pane-stocks').findAll('button').find(b=>b.text()==='确认收货')!.trigger('click');await flushPromises()
    expect(w.find('.el-dialog').text()).toContain('杭州机房')
    await w.find('.el-dialog textarea').setValue('实物已收到')
    await w.findAll('.el-dialog button').find(b=>b.text()==='确认收货')!.trigger('click');await flushPromises()
    expect(mocks.ops).toHaveBeenCalledWith('stocks/in-transit/move','POST',expect.objectContaining({operation:'RECEIVE',quantity:5,reason:'实物已收到'}))
  })
  it('filters existing stock by region and type without changing records',async()=>{
    const otherSite={id:'site-two',region:'杭州',name:'杭州备件库'}
    const otherSpec={...spec,id:'spec-two',name:'键盘',category:'键盘'}
    const other={...stock,id:'other',serialNumber:'KEY-001',spec:otherSpec,specId:otherSpec.id,site:otherSite,siteId:otherSite.id}
    mocks.list.mockImplementation(async(key:string)=>({stocks:[stock,other],specs:[spec,otherSpec],sites:[site,otherSite],minimums:[],movements:[],devices:[]}[key]||[]))
    const w=await render()
    const region=w.findAllComponents(ElSelect).find(c=>c.props('placeholder')==='全部区域')!
    region.vm.$emit('update:modelValue','杭州');region.vm.$emit('change','杭州');await flushPromises()
    expect(w.find('#pane-stocks').text()).toContain('KEY-001');expect(w.find('#pane-stocks').text()).not.toContain('MEM-001')
    const category=w.findAllComponents(ElSelect).find(c=>c.props('placeholder')==='全部类型')!
    category.vm.$emit('update:modelValue','内存');await flushPromises()
    expect(w.find('#pane-stocks').text()).not.toContain('KEY-001');expect(mocks.ops).not.toHaveBeenCalled()
  })
  it('presents the eight requested columns and exports notes without creating a new workflow',async()=>{
    const w=await render()
    const header=w.find('#pane-stocks .el-table__header').text()
    for(const field of ['区域','类型','名称','型号','数量','序列号','备注','状态'])expect(header).toContain(field)
    expect(w.find('#pane-stocks').text()).toContain('交易服务器备用')
    await w.findAll('button').find(b=>b.text()==='导出库存')!.trigger('click')
    expect(mocks.export).toHaveBeenCalledWith('机房备件',expect.arrayContaining([expect.objectContaining({备注:'交易服务器备用',区域:'上海'})]))
  })
  it('edits metadata without posting quantity or stock state', async () => {
    const w = await render()
    const row=w.find('#pane-stocks').findAll('.el-table__body-wrapper tbody tr').find(r=>r.text().includes('MEM-001'))!;row.findComponent(ElDropdown).vm.$emit('command','EDIT'); await flushPromises()
    const field = w.findAll('.el-dialog .el-form-item').find(x => x.text().includes('柜位 / 料盒'))!
    await field.find('input').setValue('S09')
    const notes=w.findAll('.el-dialog .el-form-item').find(x=>x.text().startsWith('备注'))!
    await notes.find('textarea').setValue('兼容型号已核验')
    await w.findAll('.el-dialog button').find(b => b.text() === '确认保存')!.trigger('click'); await flushPromises()
    expect(mocks.ops).toHaveBeenCalledWith('stocks/stock-one', 'PUT', expect.objectContaining({ version: 7, bin: 'S09', notes:'兼容型号已核验',serialNumber: 'MEM-001' }))
    const payload = mocks.ops.mock.calls[0][2]
    expect(payload).not.toHaveProperty('quantity'); expect(payload).not.toHaveProperty('state')
  })
  it('supports specification corrections and preserves the tracking mode', async () => {
    const w = await render(); await w.findAll('button').find(b=>b.text()==='资料与预警设置')!.trigger('click'); await flushPromises();await body().find('#tab-specs').trigger('click'); await flushPromises()
    await body().find('#pane-specs').findAll('button').find(b => b.text() === '编辑')!.trigger('click'); await flushPromises()
    const field = w.findAll('.el-dialog .el-form-item').find(x => x.text().includes('兼容设备型号'))!
    await field.find('input').setValue('经核验适配 R750')
    await w.findAll('.el-dialog button').find(b => b.text() === '确认保存')!.trigger('click'); await flushPromises()
    expect(mocks.ops).toHaveBeenCalledWith('specs/spec-one', 'PUT', expect.objectContaining({ version: 3, tracking: 'SERIAL', compatibility: '经核验适配 R750' }))
  })
  it('keeps deletion of an unused legacy draft in the more menu',async()=>{
    vi.spyOn(ElMessageBox,'confirm').mockImplementation(async()=>undefined as never)
    const w=await render();const row=w.find('#pane-stocks').findAll('.el-table__body-wrapper tbody tr').find(r=>r.text().includes('MEM-002'))!
    row.findComponent(ElDropdown).vm.$emit('command','DELETE');await flushPromises()
    expect(mocks.ops).toHaveBeenCalledWith('stocks/stock-draft?version=1','DELETE')
  })
  it('deletes a minimum rule with its version', async () => {
    vi.spyOn(ElMessageBox, 'confirm').mockImplementation(async () => undefined as never)
    const w = await render();await w.findAll('button').find(b=>b.text()==='资料与预警设置')!.trigger('click');await flushPromises(); await body().find('#tab-minimums').trigger('click'); await flushPromises()
    await body().find('#pane-minimums').findAll('button').find(b => b.text() === '删除')!.trigger('click'); await flushPromises()
    expect(mocks.ops).toHaveBeenCalledWith('minimums/minimum-one?version=5', 'DELETE')
  })
})

it('keeps three daily tabs and matches destination history with its export',async()=>{
 const dest={...stock,quantity:3,serialNumber:'',state:'TRANSIT',spec:{...spec,tracking:'QUANTITY'}}
 const movement={id:'transfer',operation:'TRANSFER',stockId:'source-stock',destinationStockId:dest.id,stock,quantity:3,beforeQuantity:10,afterQuantity:7,beforeState:'AVAILABLE',afterState:'AVAILABLE',sourceSiteId:site.id,destinationSiteId:'site-two'}
 mocks.list.mockImplementation(async(key:string)=>({stocks:[dest],specs:[spec],sites:[site],minimums:[],movements:[movement],devices:[]}[key]||[]))
 const w=await render()
 expect(w.findAll('[role="tab"]').map(e=>e.text())).toEqual(['库存明细','机房统计','库存流水'])
 await w.find('#pane-stocks').findAll('button').find(b=>b.text()==='详情')!.trigger('click');await flushPromises()
 expect(body().find('.spare-details').text()).toContain('数量管理')
 expect(body().find('.spare-details').text()).toContain('交易服务器备用')
 await body().find('.spare-details #tab-history').trigger('click');await flushPromises()
 expect(body().find('.spare-details').text()).toContain('0 → 3')
 await body().find('.spare-details').findAll('button').find(b=>b.text()==='在库存流水页查看')!.trigger('click');await flushPromises()
 await w.findAll('button').find(b=>b.text()==='导出流水')!.trigger('click');await flushPromises()
 expect(mocks.export).toHaveBeenCalledWith('库存流水',expect.arrayContaining([expect.objectContaining({操作前:0,操作后:3,新状态:'在途'})]))
})

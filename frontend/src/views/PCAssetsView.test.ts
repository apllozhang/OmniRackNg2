import ElementPlus, { ElFormItem, ElInput, ElRadioGroup, ElSelect, ElPagination } from 'element-plus'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import PCAssetsView from './PCAssetsView.vue'

const api = vi.hoisted(() => ({ list: vi.fn(), ops: vi.fn(), export: vi.fn(), labels: vi.fn(), roles: [{ code: 'system_admin' }], push: vi.fn() }))
vi.mock('@/api/asset-ops', () => ({ listOps: api.list, ops: api.ops, exportSheet: api.export, printAssetLabels: api.labels, errorMessage: (e: Error) => e.message, usageLabels: { IDLE: '在库', IN_USE: '使用中', REPAIR: '维修中', RETIRED: '已报废' } }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ user: { roles: api.roles } }) }))
vi.mock('vue-router', () => ({ useRouter: () => ({ push: api.push }) }))
const pc = { device: { id: 'pc-1', version: 7, name: '办公电脑', code: 'PC-001', serialNumber: 'SN-001', lifecycleStatus: 'WAITING_RACK', type: { category: 'PC' } }, profile: { usageStatus: 'IN_USE', assignee: '张三', department: '研究部', siteId: 'site-1', position: '8F-01' }, warranty: { status: 'VALID', label: '在保' } }
const wrappers: ReturnType<typeof mount>[] = []
async function render() { const w = mount(PCAssetsView, { attachTo: document.body, global: { plugins: [ElementPlus] } }); wrappers.push(w); await flushPromises(); return w }
beforeEach(() => {
  vi.clearAllMocks(); api.roles = [{ code: 'system_admin' }]
  api.list.mockImplementation(async (path: string) => path === 'sites' ? [{ id: 'site-1', name: '总部', region: '上海' }] : [structuredClone(pc), { ...pc, device: { ...pc.device, id: 'srv-1', name: '机房服务器', type: { category: 'SERVER' } } }])
  api.ops.mockResolvedValue({ items: [], total: 0 })
})
afterEach(() => { wrappers.splice(0).forEach(w => w.unmount()); document.body.innerHTML = '' })

it('separates PC assets from servers and exports the actual filtered PC ledger', async () => {
  const w = await render()
  expect(w.text()).toContain('办公电脑'); expect(w.text()).not.toContain('机房服务器')
  await w.findAll('button').find(b => b.text() === '导出筛选终端')!.trigger('click')
  expect(api.export).toHaveBeenCalledWith('终端台账', [expect.objectContaining({ SN: 'SN-001', 使用人: '张三' })])
})
it('confirms a faulty return with the device version, preserving the current owner on screen until confirmed', async () => {
  const w = await render()
  await w.findAll('button').find(b => b.text() === '归还')!.trigger('click'); await flushPromises()
  const condition = w.findAllComponents(ElFormItem).find(f => f.text().includes('设备状况'))!
  condition.findComponent(ElRadioGroup).vm.$emit('update:modelValue', 'FAULTY')
  const note = w.findAllComponents(ElFormItem).find(f => f.text().includes('操作说明'))!
  note.findComponent(ElInput).vm.$emit('update:modelValue', '离职归还，屏幕故障')
  await flushPromises()
  const button = [...document.querySelectorAll('button')].find(b => b.textContent?.trim() === '确认归还')!
  button.click(); await flushPromises()
  expect(api.ops).toHaveBeenCalledWith('devices/pc-1/pc-handover', 'POST', expect.objectContaining({ version: 7, operation: 'RETURN', condition: 'FAULTY', siteId: 'site-1', note: '离职归还，屏幕故障' }))
})
it('requires a new recipient and note before confirming a transfer', async () => {
  const w = await render()
  await w.findAll('button').find(b => b.text() === '换人交接')!.trigger('click'); await flushPromises()
  const button = [...document.querySelectorAll('button')].find(b => b.textContent?.trim() === '确认换人交接')!
  button.click(); await flushPromises(); expect(api.ops).not.toHaveBeenCalled()
  for (const [label, value] of [['接收人', '李四'], ['接收部门', 'IT'], ['操作说明', '换人使用']]) {
    w.findAllComponents(ElFormItem).find(f => f.text().includes(label!))!.findComponent(ElInput).vm.$emit('update:modelValue', value)
  }
  await flushPromises(); button.click(); await flushPromises()
  expect(api.ops).toHaveBeenCalledWith('devices/pc-1/pc-handover', 'POST', expect.objectContaining({ operation: 'TRANSFER', assignee: '李四', department: 'IT' }))
})
it('keeps a rejected stale handover open for review instead of reporting success', async () => {
  api.ops.mockRejectedValue(new Error('版本冲突，请刷新'))
  const w = await render()
  await w.findAll('button').find(b => b.text() === '归还')!.trigger('click'); await flushPromises()
  w.findAllComponents(ElFormItem).find(f => f.text().includes('操作说明'))!.findComponent(ElInput).vm.$emit('update:modelValue', '归还')
  await flushPromises(); [...document.querySelectorAll('button')].find(b => b.textContent?.trim() === '确认归还')!.click(); await flushPromises()
  expect(document.body.textContent).toContain('确认终端归还')
  expect(api.list).toHaveBeenCalledTimes(2)
})
it('allows audit readers to inspect history but not perform handovers', async () => {
  api.roles = [{ code: 'asset_auditor' }]
  const w = await render()
  const labels = w.findAll('button').map(b => b.text())
  expect(labels).not.toContain('领用'); expect(labels).not.toContain('归还'); expect(labels).toContain('使用履历')
  await w.findAll('button').find(b => b.text() === '使用履历')!.trigger('click'); await flushPromises()
  expect(api.ops).toHaveBeenCalledWith('devices/pc-1/pc-history?page=1')
})
it('exports every page of immutable handover history', async () => {
  const row = { id: 'h', operation: 'RETURN', deviceCode: 'PC-001', serialNumber: 'SN-001', before: { ...pc.profile, siteName: '上海 / 总部' }, after: { ...pc.profile, usageStatus: 'IDLE', assignee: '' }, condition: 'NORMAL', note: '归还', confirmedAt: '2026-10-20T00:00:00Z', confirmedByName: '管理员' }
  api.ops.mockImplementation(async (path: string) => ({ items: path.endsWith('page=2') ? [{ ...row, id: 'last' }] : Array.from({ length: 50 }, (_, i) => ({ ...row, id: String(i) })), total: 51 }))
  const w = await render(); await w.findAll('button').find(b => b.text() === '使用履历')!.trigger('click'); await flushPromises()
  ;[...document.querySelectorAll('button')].find(b => b.textContent?.trim() === '导出此终端全部履历')!.click(); await flushPromises()
  expect(api.ops).toHaveBeenCalledWith('devices/pc-1/pc-history?page=2')
  expect(api.export.mock.calls[0]![1]).toHaveLength(51)
})

it('groups monitors, desktops, printers, laptops, mice and keyboards as terminals and filters exports by type', async () => {
  const labels = ['显示器', '主机', '打印机', '笔记本', '鼠标', '键盘', '其他终端']
  const devices = labels.map((label, i) => ({ ...structuredClone(pc), device: { ...pc.device, id: `terminal-${i}`, name: label, typeId: `type-${i}`, serialNumber: '', type: { id: `type-${i}`, name: label, category: 'PC' } }, profile: { ...pc.profile, usageStatus: 'IDLE', assignee: '', department: '' } }))
  api.list.mockImplementation(async (path: string) => path === 'sites' ? [{ id: 'site-1', name: '总部', region: '上海' }] : devices)
  const w = await render()
  expect(w.text()).toContain('终端管理')
  for (const label of labels) expect(w.text()).toContain(label)
  const selector = w.findAllComponents(ElSelect).find(s => s.props('placeholder') === '终端类型')!
  selector.vm.$emit('update:modelValue', 'type-5'); await flushPromises()
  await w.findAll('button').find(b => b.text() === '导出筛选终端')!.trigger('click')
  expect(api.export).toHaveBeenCalledWith('终端台账', [expect.objectContaining({ 名称: '键盘', 终端类型: '键盘', SN: '' })])
  // Peripherals without manufacturer SN still use their unique system asset ID.
  await w.findAll('button').find(b => b.text() === '领用')!.trigger('click'); await flushPromises()
  for (const [label, value] of [['接收人', '王五'], ['接收部门', '研究部'], ['操作说明', '键盘领用']]) {
    w.findAllComponents(ElFormItem).find(f => f.text().includes(label!))!.findComponent(ElInput).vm.$emit('update:modelValue', value)
  }
  await flushPromises(); [...document.querySelectorAll('button')].find(b => b.textContent?.trim() === '确认领用')!.click(); await flushPromises()
  expect(api.ops).toHaveBeenCalledWith('devices/terminal-5/pc-handover', 'POST', expect.objectContaining({ operation: 'ISSUE', assignee: '王五' }))
})


it('handles maintenance and retirement through explicit recorded actions',async()=>{
  api.list.mockImplementation(async(path:string)=>path==='sites'?[{id:'site-1',name:'库房',region:'上海'}]:[{...structuredClone(pc),profile:{...pc.profile,usageStatus:'REPAIR',assignee:'',department:''}}])
  const w=await render()
  expect(w.findAll('button').some(b=>b.text()==='领用')).toBe(false)
  expect(w.findAll('button').some(b=>b.text()==='报废')).toBe(true)
  await w.findAll('button').find(b=>b.text()==='维修完成')!.trigger('click');await flushPromises()
  w.findAllComponents(ElFormItem).find(f=>f.text().includes('操作说明'))!.findComponent(ElInput).vm.$emit('update:modelValue','维修检测正常')
  await flushPromises();[...document.querySelectorAll('button')].find(b=>b.textContent?.trim()==='确认维修完成')!.click();await flushPromises()
  expect(api.ops).toHaveBeenCalledWith('devices/pc-1/pc-handover','POST',expect.objectContaining({operation:'RESTORE',condition:'NORMAL',note:'维修检测正常'}))
})
it('does not offer retirement while the terminal is still assigned',async()=>{
  const w=await render()
  expect(w.findAll('button').some(b=>b.text()==='报废')).toBe(false)
  expect(w.findAll('button').some(b=>b.text()==='归还')).toBe(true)
})


it('paginates the main ledger, sorts all rows before paging, and exports the whole filtered result', async () => {
  const rows = Array.from({ length: 61 }, (_, i) => { const n = 61 - i; return { ...structuredClone(pc), device: { ...pc.device, id: `pc-${n}`, code: `PC-${n}`, name: `终端${n}` } } })
  api.list.mockImplementation(async (path: string) => path === 'sites' ? [] : rows)
  const w = await render()
  const firstName = () => w.find('.el-table__body-wrapper .el-table__row b').text()
  expect(w.findAll('.el-table__body-wrapper .el-table__row')).toHaveLength(20)
  expect(firstName()).toBe('终端61')
  await w.get('button[aria-label="按终端 / SN排序"]').trigger('click'); await flushPromises()
  expect(firstName()).toBe('终端1')
  const paging = w.findAllComponents(ElPagination).find(p => p.attributes('aria-label') === '终端主台账分页')!
  paging.vm.$emit('update:current-page', 2); await flushPromises()
  expect(firstName()).toBe('终端21')
  await w.findAll('button').find(b => b.text() === '导出筛选终端')!.trigger('click')
  expect(api.export.mock.calls[0]![1]).toHaveLength(61)
  expect(api.export.mock.calls[0]![1][0].名称).toBe('终端1')
  await w.get('button[aria-label="按终端 / SN排序"]').trigger('click'); await flushPromises()
  expect(firstName()).toBe('终端61'); expect(paging.props('currentPage')).toBe(1)
  await w.get('button[aria-label="按终端 / SN排序"]').trigger('click'); await flushPromises()
  expect(firstName()).toBe('终端61')
})
it('debounces ledger filtering, resets the page and provides a working empty-state reset', async () => {
  const rows = Array.from({ length: 61 }, (_, i) => ({ ...structuredClone(pc), device: { ...pc.device, id: `pc-${i}`, name: `终端${i}` } }))
  api.list.mockImplementation(async (path: string) => path === 'sites' ? [] : rows)
  const w = await render()
  const paging = w.findAllComponents(ElPagination).find(p => p.attributes('aria-label') === '终端主台账分页')!
  paging.vm.$emit('update:current-page', 3); await flushPromises()
  const input = w.get('input[placeholder="名称 / 编码 / SN / 使用人 / 部门"]')
  await input.setValue('不存在的终端')
  expect(w.findAll('button').find(b => b.text() === '导出筛选终端')!.attributes('disabled')).toBeDefined()
  await new Promise(resolve => setTimeout(resolve, 330)); await flushPromises()
  expect(paging.props('currentPage')).toBe(1)
  expect(w.findAll('.el-table__body-wrapper .el-table__row')).toHaveLength(0)
  await w.findAll('button').find(b => b.text() === '清除筛选')!.trigger('click'); await flushPromises()
  expect(paging.props('total')).toBe(61)
  expect(w.findAll('.el-table__body-wrapper .el-table__row')).toHaveLength(20)
})

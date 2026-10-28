import ElementPlus, { ElMessage, ElMessageBox } from 'element-plus'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import DeviceManagementView from '@/views/DeviceManagementView.vue'
import type { Device, DeviceType } from '@/types/device'

const mocks = vi.hoisted(() => ({
  route: { query: {} as Record<string, string>, meta: {} as Record<string, string> },
  createTerminal: vi.fn(), getTerminalState: vi.fn(), listTerminalSites: vi.fn(),
  download: vi.fn(), getResourceTree: vi.fn(), listDevices: vi.fn(), getDevice: vi.fn(), listDeviceTypes: vi.fn(),
  updateDevice: vi.fn(), createDevice: vi.fn(), createDeviceType: vi.fn(), updateDeviceType: vi.fn(),
  deleteDevice: vi.fn(), deleteDeviceType: vi.fn(), assignDevice: vi.fn(), moveDevice: vi.fn(),
  decommissionDevice: vi.fn(), getRackLayout: vi.fn(), listDeviceHistory: vi.fn(),
}))

vi.mock('@/utils/deviceExport', () => ({ downloadDeviceExportExcel: mocks.download }))
vi.mock('@/api/resource', () => ({ getResourceTree: mocks.getResourceTree }))
vi.mock('@/api/device', () => ({
  createTerminal:mocks.createTerminal,getTerminalState:mocks.getTerminalState,listTerminalSites:mocks.listTerminalSites,
  listDevices: mocks.listDevices, getDevice: mocks.getDevice, listDeviceTypes: mocks.listDeviceTypes,
  updateDevice: mocks.updateDevice, createDevice: mocks.createDevice, createDeviceType: mocks.createDeviceType,
  updateDeviceType: mocks.updateDeviceType, deleteDevice: mocks.deleteDevice, deleteDeviceType: mocks.deleteDeviceType,
  assignDevice: mocks.assignDevice, moveDevice: mocks.moveDevice, decommissionDevice: mocks.decommissionDevice,
  getRackLayout: mocks.getRackLayout, listDeviceHistory: mocks.listDeviceHistory,
}))
vi.mock('vue-router', () => ({ useRoute: () => mocks.route }))

const timestamp = '2026-09-04T08:00:00Z'
const type: DeviceType = {
  id: 'type-1', createdAt: timestamp, updatedAt: timestamp, version: 1, code: 'SERVER', name: '服务器',
  category: 'SERVER', status: 'ACTIVE', defaultHeightU: 2, defaultDualPower: true, sortOrder: 0,
}
const listDevice: Device = {
  id: 'device-1', createdAt: timestamp, updatedAt: timestamp, version: 4, typeId: type.id, type,
  code: 'DEV-001', name: '列表旧名称', lifecycleStatus: 'WAITING_RACK', heightU: 2, dualPowerRequired: true,
}
const detailDevice: Device = {
  ...listDevice, version: 5, name: '详情最新名称', purchaseBatch: '2026-Q3', warrantyExpiresAt: '2029-08-31T00:00:00Z',
  managementIp: '10.0.0.1', businessIp: '172.16.0.1', specification: '双路 CPU / 256GB',
}

beforeEach(() => {
  mocks.route.query = {}; mocks.route.meta = {}
  mocks.listTerminalSites.mockResolvedValue({items:[{id:'site-1',name:'办公区',region:'上海'}]})
  mocks.getTerminalState.mockResolvedValue({status:'IDLE',assignee:'',department:'',siteId:null,position:''})
  mocks.createTerminal.mockResolvedValue({...listDevice,type:{...type,category:'PC'}})
  mocks.getResourceTree.mockResolvedValue({ items: [] })
  mocks.listDeviceTypes.mockResolvedValue({ items: [type] })
  mocks.listDevices.mockResolvedValue({ items: [listDevice], total: 1, page: 1, pageSize: 20 })
  mocks.getDevice.mockResolvedValue(detailDevice)
  mocks.updateDevice.mockImplementation(async (id: string, payload: Partial<Device>) => ({ ...detailDevice, ...payload, id, version: 6 }))
})

afterEach(() => { document.body.innerHTML = '' })

describe('DeviceManagementView shared editor', () => {
  it.each([false,true])('reports decommission execution accurately: %s',async executed=>{
    const prompt=vi.spyOn(ElMessageBox,'prompt').mockImplementation(async()=>({value:'设备退役',action:'confirm'} as never))
    const message=vi.spyOn(ElMessage,'success')
    mocks.listDevices.mockResolvedValue({items:[{...listDevice,lifecycleStatus:'RUNNING',currentPosition:{rackId:'rack-1',startU:1,endU:2}}],total:1,page:1,pageSize:20})
    mocks.decommissionDevice.mockResolvedValue({executed})
    const w=mount(DeviceManagementView,{attachTo:document.body,global:{plugins:[ElementPlus]}});await flushPromises()
    await w.findAll('button').find(b=>b.text()==='下架')!.trigger('click');await flushPromises()
    expect(mocks.decommissionDevice).toHaveBeenCalledWith('device-1',{version:4,reason:'设备退役'})
    expect(message).toHaveBeenCalledWith(executed?'设备已下架':'下架申请已提交，审批通过前保留原 U 位')
    w.unmount();prompt.mockRestore();message.mockRestore()
  })
  it('reloads latest details and saves the complete backend-aligned payload', async () => {
    const wrapper = mount(DeviceManagementView, {
      attachTo: document.body,
      global: { plugins: [ElementPlus], stubs: { BatchImportDialog: true, RackUView: true } },
    })
    await flushPromises()

    const editButton = wrapper.findAll('button').find((button) => button.text().trim() === '编辑')
    expect(editButton).toBeTruthy()
    await editButton!.trigger('click')
    await flushPromises()

    expect(mocks.getDevice).toHaveBeenCalledWith('device-1')
    const text = document.body.textContent ?? ''
    const nameItem = Array.from(document.querySelectorAll('.el-form-item')).find((item) => item.textContent?.includes('设备名称'))
    expect((nameItem?.querySelector('input') as HTMLInputElement | null)?.value).toBe('详情最新名称')
    expect(text).toContain('采购批次')
    expect(text).toContain('保修到期日')
    expect(text).toContain('外部二维码')

    const businessIpItem = Array.from(document.querySelectorAll('.el-form-item')).find((item) => item.textContent?.includes('业务 IP'))
    const businessIpInput = businessIpItem?.querySelector('input') as HTMLInputElement | null
    expect(businessIpInput).not.toBeNull()
    businessIpInput!.value = '172.16.0.99'
    businessIpInput!.dispatchEvent(new Event('input', { bubbles: true }))
    await flushPromises()

    const saveButton = Array.from(document.querySelectorAll('.el-dialog button')).find((button) => button.textContent?.trim() === '保存') as HTMLButtonElement | undefined
    expect(saveButton).toBeTruthy()
    saveButton!.click()
    await flushPromises()

    expect(mocks.updateDevice).toHaveBeenCalledWith('device-1', expect.objectContaining({
      version: 5, name: '详情最新名称', purchaseBatch: '2026-Q3', specification: '双路 CPU / 256GB',
      warrantyExpiresAt: '2029-08-31T00:00:00Z', businessIp: '172.16.0.99',
    }))
    wrapper.unmount()
  })
})

describe('device export scopes', () => {
  async function setup() {
    mocks.download.mockClear()
    const wrapper = mount(DeviceManagementView, { attachTo: document.body, global: { plugins: [ElementPlus], stubs: { BatchImportDialog: true, RackUView: true } } })
    await flushPromises(); return wrapper
  }
  async function click(text: string) {
    const button = Array.from(document.querySelectorAll('button')).find(item => item.textContent?.trim() === text)
    expect(button).toBeTruthy(); button!.click(); await flushPromises()
  }
  it('applies pending keywords and prevents an empty download', async () => {
    const wrapper = await setup()
    await wrapper.find('input[placeholder="名称、编码、资产号、序列号或 IP"]').setValue('不存在')
    mocks.listDevices.mockResolvedValue({ items: [], total: 0 })
    await click('导出设备信息')
    expect(mocks.listDevices).toHaveBeenLastCalledWith(expect.objectContaining({ search: '不存在' }))
    expect(mocks.download).not.toHaveBeenCalled()
    wrapper.unmount()
  })
  it('exports latest selected details across pages', async () => {
    const wrapper = await setup()
    wrapper.findComponent({ name: 'ElTable' }).vm.$emit('selection-change', [listDevice]); await flushPromises()
    wrapper.findComponent({ name: 'ElPagination' }).vm.$emit('change'); await flushPromises()
    expect(wrapper.text()).toContain('已勾选 1 台')
    await click('导出设备信息'); await click('确认导出')
    expect(mocks.download).toHaveBeenCalledWith(expect.objectContaining({ devices: [detailDevice] }))
    wrapper.unmount()
  })
  it('filters export by category checkboxes', async () => {
    const wrapper = await setup(); await click('导出设备信息')
    const group = wrapper.findComponent({ name: 'ElCheckboxGroup' })
    group.vm.$emit('update:modelValue', ['PC']); await flushPromises(); await click('确认导出')
    expect(mocks.download).not.toHaveBeenCalled()
    group.vm.$emit('update:modelValue', ['SERVER']); await flushPromises(); await click('确认导出')
    expect(mocks.download).toHaveBeenCalledWith(expect.objectContaining({ devices: [listDevice] }))
    wrapper.unmount()
  })
  it('clears checked devices when query changes', async () => {
    const wrapper = await setup()
    wrapper.findComponent({ name: 'ElTable' }).vm.$emit('selection-change', [listDevice]); await flushPromises()
    await wrapper.find('input[placeholder="名称、编码、资产号、序列号或 IP"]').setValue('新查询')
    await click('查询'); expect(wrapper.text()).toContain('已勾选 0 台')
    wrapper.unmount()
  })
})


describe('device ledger scopes', () => {
  it('queries room devices with a server-side scope', async () => {
    mocks.route.meta = { deviceScope: 'ROOM' }
    mocks.listDevices.mockClear()
    const wrapper = mount(DeviceManagementView, { global: { plugins: [ElementPlus], stubs: { BatchImportDialog: true, RackUView: true } } })
    await flushPromises()
    expect(wrapper.text()).toContain('机房设备与 U 位管理')
    expect(mocks.listDevices).toHaveBeenCalledWith(expect.objectContaining({ scope: 'ROOM' }))
    wrapper.unmount()
  })
  it('keeps PC registration separate from rack positioning', async () => {
    mocks.route.query = { scope: 'PC' }
    mocks.listDevices.mockClear()
    mocks.listDevices.mockResolvedValue({ items: [{ ...listDevice, type: { ...type, category: 'PC' } }], total: 1 })
    mocks.listDeviceTypes.mockResolvedValue({ items: [{ ...type, category: 'PC' }] })
    const wrapper = mount(DeviceManagementView, { global: { plugins: [ElementPlus], stubs: { BatchImportDialog: true, RackUView: true, RouterLink: true } } })
    await flushPromises()
    expect(mocks.listDevices).toHaveBeenCalledWith(expect.objectContaining({ scope: 'PC' }))
    expect(wrapper.text()).toContain('终端设备档案')
    expect(wrapper.findAll('button').some(b => b.text() === '上架')).toBe(false)
    expect(wrapper.findAll('[role="tab"]').some(b => b.text() === '机柜 U 位')).toBe(false)
    wrapper.unmount()
  })
})


describe('terminal initial state registration', () => {
  it('validates current ownership and sends one atomic registration request', async () => {
    mocks.route.query={scope:'PC'}
    mocks.listDeviceTypes.mockResolvedValue({items:[{...type,category:'PC',name:'笔记本'}]})
    mocks.createTerminal.mockClear();mocks.createDevice.mockClear()
    const w=mount(DeviceManagementView,{attachTo:document.body,global:{plugins:[ElementPlus],stubs:{BatchImportDialog:true,RackUView:true,RouterLink:true}}})
    await flushPromises()
    await w.findAll('button').find(b=>b.text()==='新增设备')!.trigger('click');await flushPromises()
    const fields=w.findComponent({name:'DeviceFormFields'})
    fields.vm.$emit('update:modelValue',{...fields.props('modelValue'),typeId:type.id,name:'存量笔记本'})
    fields.vm.$emit('update:terminal',{status:'IN_USE',assignee:'',department:'',siteId:null,position:''})
    await flushPromises()
    ;[...document.querySelectorAll('button')].find(b=>b.textContent?.trim()==='保存')!.click();await flushPromises()
    expect(mocks.createTerminal).not.toHaveBeenCalled()
    fields.vm.$emit('update:terminal',{status:'IN_USE',assignee:'张三',department:'研究部',siteId:'site-1',position:'8F'})
    await flushPromises()
    ;[...document.querySelectorAll('button')].find(b=>b.textContent?.trim()==='保存')!.click();await flushPromises()
    expect(mocks.createTerminal).toHaveBeenCalledWith(expect.objectContaining({name:'存量笔记本',terminal:expect.objectContaining({status:'IN_USE',assignee:'张三',siteId:'site-1'})}))
    expect(mocks.createDevice).not.toHaveBeenCalled()
    w.unmount()
  })
  it('loads the current custody state when editing a terminal with a legacy rack lifecycle', async () => {
    mocks.route.query={scope:'PC'}
    const terminal={...detailDevice,type:{...type,category:'PC'}}
    mocks.getDevice.mockResolvedValue(terminal)
    mocks.listDevices.mockResolvedValue({items:[terminal],total:1})
    mocks.listDeviceTypes.mockResolvedValue({items:[terminal.type]})
    mocks.getTerminalState.mockResolvedValue({status:'REPAIR',assignee:'',department:'',siteId:null,position:''})
    const w=mount(DeviceManagementView,{attachTo:document.body,global:{plugins:[ElementPlus],stubs:{BatchImportDialog:true,RackUView:true,RouterLink:true}}})
    await flushPromises();await w.findAll('button').find(b=>b.text()==='编辑')!.trigger('click');await flushPromises()
    expect(mocks.getTerminalState).toHaveBeenCalledWith(detailDevice.id)
    expect(w.findComponent({name:'DeviceFormFields'}).props('terminal')).toMatchObject({status:'REPAIR'})
    w.unmount()
  })
})

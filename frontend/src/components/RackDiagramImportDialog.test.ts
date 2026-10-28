import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { ElMessageBox } from 'element-plus'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import RackDiagramImportDialog from '@/components/RackDiagramImportDialog.vue'
import type { DeviceType } from '@/types/device'
import type { RackDiagramImportValidation } from '@/types/rackDiagramImport'

const mocks = vi.hoisted(() => ({
  parseRackDiagramExcel: vi.fn(),
  validateRackDiagramImport: vi.fn(),
  commitRackDiagramImport: vi.fn(),
}))
vi.mock('@/utils/rackDiagramImport', () => ({ parseRackDiagramExcel: mocks.parseRackDiagramExcel }))
vi.mock('@/api/device', () => ({
  validateRackDiagramImport: mocks.validateRackDiagramImport,
  commitRackDiagramImport: mocks.commitRackDiagramImport,
}))

const timestamp = '2026-09-02T08:00:00Z'
const deviceTypes: DeviceType[] = [{
  id: 'type-1', createdAt: timestamp, updatedAt: timestamp, version: 1,
  code: 'SERVER', name: '服务器', category: 'SERVER', status: 'ACTIVE',
  defaultHeightU: 1, defaultDualPower: true, sortOrder: 0,
}]
const parsed = {
  fileName: '机柜图.xlsx',
  payload: { formatVersion: '1', dataCenterId: 'dc-1', roomId: 'room-1', coveredRackIds: ['rack-1'], devices: [] },
}
function result(items: RackDiagramImportValidation['items'], errors = 0): RackDiagramImportValidation {
  return {
    token: 'token-1', expiresAt: '2026-09-02T08:30:00Z',
    summary: {
      total: items.length, unchanged: 0, create: 0, move: 0,
      decisions: items.filter((item) => item.kind === 'NEEDS_DECISION').length,
      removals: items.filter((item) => item.kind === 'REMOVE_MISSING').length,
      errors,
    },
    items,
  }
}
const decisionItem: RackDiagramImportValidation['items'][number] = {
  id: 'item-1', kind: 'NEEDS_DECISION', severity: 'warning', message: '设备名称发生变化',
  rackId: 'rack-1', rackCode: 'RACK-01', startU: 2, endU: 3, name: '新名称',
  existingDeviceId: 'device-1', existingDeviceCode: 'DEV-001', existingDeviceName: '旧名称',
  requiresDecision: true, allowedDecisions: ['UPDATE_EXISTING', 'CREATE_NEW'],
}

async function mountDialog(): Promise<VueWrapper> {
  const wrapper = mount(RackDiagramImportDialog, {
    attachTo: document.body,
    props: { modelValue: true, roomId: 'room-1', dataCenterId: 'dc-1', deviceTypes },
    global: {
      stubs: {
        ElDialog: { props: ['modelValue'], emits: ['close'], template: '<div v-if="modelValue" class="dialog-stub"><slot/><slot name="footer"/></div>' },
        ElButton: { props: ['disabled', 'loading'], emits: ['click'], template: `<button :disabled="disabled" @click="$emit('click')"><slot/></button>` },
        ElSelect: { props: ['modelValue'], emits: ['update:modelValue'], template: `<select :value="modelValue" @change="$emit('update:modelValue', $event.target.value)"><slot/></select>` },
        ElOption: { props: ['label', 'value'], template: '<option :value="value">{{ label }}</option>' },
        ElFormItem: { template: '<label><slot/></label>' },
        ElAlert: { props: ['title'], template: '<div class="alert-stub">{{ title }}</div>' },
        ElTable: { template: '<div class="table-stub"></div>' },
        ElTableColumn: { template: '<div></div>' },
        ElTag: { template: '<span><slot/></span>' },
      },
    },
  })
  await flushPromises()
  return wrapper
}
async function chooseAndValidate(wrapper: VueWrapper) {
  const input = wrapper.find('input[type="file"]')
  Object.defineProperty(input.element, 'files', { configurable: true, value: [new File(['x'], '机柜图.xlsx')] })
  await input.trigger('change')
  await flushPromises()
  const validateButton = wrapper.findAll('button').find((button) => button.text().includes('解析并校验'))
  expect(validateButton).toBeDefined()
  await validateButton!.trigger('click')
  await flushPromises()
}
function commitButton(wrapper: VueWrapper) {
  const button = wrapper.findAll('button').find((item) => item.text().includes('确认导入'))
  if (!button) throw new Error('commit button not found')
  return button
}

beforeEach(() => {
  mocks.parseRackDiagramExcel.mockResolvedValue(parsed)
  mocks.commitRackDiagramImport.mockResolvedValue({ created: 0, updated: 1, moved: 0, decommissioned: 0, unchanged: 0 })
  vi.spyOn(ElMessageBox, 'confirm').mockResolvedValue('confirm' as never)
})
afterEach(() => { document.body.innerHTML = '' })

describe('RackDiagramImportDialog', () => {
  it('disables commit until every required decision is selected', async () => {
    mocks.validateRackDiagramImport.mockResolvedValue(result([decisionItem]))
    const wrapper = await mountDialog()
    await chooseAndValidate(wrapper)

    expect(commitButton(wrapper).attributes('disabled')).toBeDefined()
    ;(wrapper.vm as any).decisions['item-1'] = 'UPDATE_EXISTING'
    await wrapper.vm.$nextTick()
    expect(commitButton(wrapper).attributes('disabled')).toBeUndefined()
    wrapper.unmount()
  })

  it('keeps commit disabled when validation contains an error', async () => {
    const errorItem = { ...decisionItem, id: 'error-1', kind: 'ERROR' as const, severity: 'error' as const, requiresDecision: false, allowedDecisions: undefined }
    mocks.validateRackDiagramImport.mockResolvedValue(result([errorItem], 1))
    const wrapper = await mountDialog()
    await chooseAndValidate(wrapper)

    expect(commitButton(wrapper).attributes('disabled')).toBeDefined()
    expect(document.body.textContent).toContain('存在阻断错误')
    wrapper.unmount()
  })

  it('submits selected decisions and emits success after transaction commit', async () => {
    mocks.validateRackDiagramImport.mockResolvedValue(result([decisionItem]))
    const wrapper = await mountDialog()
    await chooseAndValidate(wrapper)
    ;(wrapper.vm as any).decisions['item-1'] = 'UPDATE_EXISTING'
    await wrapper.vm.$nextTick()
    await commitButton(wrapper).trigger('click')
    await flushPromises()

    expect(mocks.commitRackDiagramImport).toHaveBeenCalledWith('room-1', {
      token: 'token-1', decisions: [{ itemId: 'item-1', action: 'UPDATE_EXISTING' }],
    })
    expect(wrapper.emitted('success')).toHaveLength(1)
    expect(wrapper.emitted('update:modelValue')).toContainEqual([false])
    wrapper.unmount()
  })
})

it('requires and submits decisions from all 61 rows even when the preview hides them',async()=>{
 const items=Array.from({length:61},(_,i)=>({...decisionItem,id:`item-${i+1}`,name:`设备${i+1}`}))
 mocks.validateRackDiagramImport.mockResolvedValue(result(items))
 const wrapper=await mountDialog();await chooseAndValidate(wrapper)
 const vm=wrapper.vm as any
 for(let i=1;i<=60;i++)vm.decisions[`item-${i}`]='UPDATE_EXISTING'
 vm.search='设备1';await new Promise(r=>setTimeout(r,350));await flushPromises()
 expect(commitButton(wrapper).attributes('disabled')).toBeDefined()
 vm.decisions['item-61']='CREATE_NEW';await flushPromises()
 expect(commitButton(wrapper).attributes('disabled')).toBeUndefined()
 await commitButton(wrapper).trigger('click');await flushPromises()
 const payload=mocks.commitRackDiagramImport.mock.calls[mocks.commitRackDiagramImport.mock.calls.length-1]?.[1]
 expect(payload.decisions).toHaveLength(61)
 expect(payload.decisions[60]).toEqual({itemId:'item-61',action:'CREATE_NEW'})
 wrapper.unmount()
})

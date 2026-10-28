import ElementPlus from 'element-plus'
import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import DeviceFormFields from '@/components/DeviceFormFields.vue'
import { createDevicePayload } from '@/features/device/deviceForm'
import type { DeviceType } from '@/types/device'

const timestamp = '2026-09-04T08:00:00Z'
const types: DeviceType[] = [{
  id: 'type-1', createdAt: timestamp, updatedAt: timestamp, version: 1,
  code: 'SERVER', name: '服务器', category: 'SERVER', status: 'ACTIVE',
  defaultHeightU: 2, defaultDualPower: true, sortOrder: 1,
}]

describe('DeviceFormFields', () => {

  it('uses terminal custody states and hides cabinet controls when registering a terminal', () => {
    const wrapper = mount(DeviceFormFields, {
      props: { modelValue: { ...createDevicePayload(), typeId:'type-1' }, terminalMode:true, types:[{...types[0]!,category:'PC'}], mode:'create', terminal:{status:'IN_USE',assignee:'张三',department:'研究部',siteId:'site-1',position:'8F'} },
      global:{plugins:[ElementPlus]},
    })
    const labels=wrapper.findAllComponents({name:'ElOption'}).map(o=>o.props('label'))
    for(const label of ['在库','使用中','维修中','已报废']) expect(labels).toContain(label)
    expect(labels).not.toContain('待上架')
    expect(wrapper.text()).not.toContain('U 高度')
    expect(wrapper.text()).not.toContain('双路电源')
    expect(wrapper.text()).toContain('当前使用人')
    expect(wrapper.text()).toContain('使用部门')
    wrapper.unmount()
  })

  it('shows actual terminal state independently of the raw rack lifecycle and directs edits to the workflow', () => {
    const wrapper=mount(DeviceFormFields,{props:{modelValue:{...createDevicePayload(),typeId:'type-1',lifecycleStatus:'WAITING_RACK'},types:[{...types[0]!,category:'PC'}],mode:'edit',terminal:{status:'REPAIR',assignee:'',department:'',siteId:null,position:''}},global:{plugins:[ElementPlus]}})
    const field=wrapper.findAllComponents({name:'ElFormItem'}).find(i=>i.props('label')==='使用状态')!
    expect(field.findComponent({name:'ElSelect'}).props('modelValue')).toBe('REPAIR')
    expect(field.findComponent({name:'ElSelect'}).props('disabled')).toBe(true)
    expect(wrapper.text()).toContain('领用台账的实际状态')
    wrapper.unmount()
  })
  it('renders the complete shared device editor field set', () => {
    const wrapper = mount(DeviceFormFields, {
      props: { modelValue: { ...createDevicePayload(), typeId: 'type-1' }, types, mode: 'edit' },
      global: { plugins: [ElementPlus] },
    })
    const text = wrapper.text()
    for (const label of [
      '设备类型', '设备编码', '设备名称', '生命周期', '资产编号', '序列号', '厂商', '型号',
      '规格', '固件版本', '采购批次', '保修到期日', 'U 高度', '宽度(mm)', '深度(mm)',
      '设备高度(mm)', '重量(kg)', '额定功率(W)', '峰值功率(W)', '输入电压(V)', '双路电源',
      '所属组织', '负责人', '联系方式', '业务系统', '应用名称', '管理 IP', '业务 IP',
      'MAC 地址', '管理协议', '监控状态', '外部二维码', '二维码链接', '标签', '备注',
    ]) expect(text).toContain(label)
  })

  it('shows backend protection guidance and disables U height for positioned devices', () => {
    const wrapper = mount(DeviceFormFields, {
      props: { modelValue: { ...createDevicePayload(), typeId: 'type-1', lifecycleStatus: 'RUNNING' }, types, positioned: true },
      global: { plugins: [ElementPlus] },
    })
    expect(wrapper.text()).toContain('设备已上架')
    const heightItem = wrapper.findAll('.el-form-item').find((item) => item.text().includes('U 高度'))
    expect(heightItem?.find('input').attributes('disabled')).toBeDefined()
  })
})

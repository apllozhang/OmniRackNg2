import ElementPlus from 'element-plus'
import { mount, flushPromises } from '@vue/test-utils'
import { beforeEach, expect, it, vi } from 'vitest'
import DeviceQrBinding from './DeviceQrBinding.vue'
import { createDevicePayload } from '@/features/device/deviceForm'
const mocks = vi.hoisted(() => ({ settings:vi.fn(), qr:vi.fn(), print:vi.fn(), detail:vi.fn() }))
vi.mock('@/api/scan-access', () => ({ getScanAccessSettings:mocks.settings }))
vi.mock('@/api/device', () => ({ getDevice:mocks.detail }))
vi.mock('@/api/asset-ops', async importOriginal => ({ ...await importOriginal<typeof import('@/api/asset-ops')>(), printAssetLabels:mocks.print, errorMessage:(e:Error)=>e.message }))
vi.mock('qrcode', () => ({ default:{ toDataURL:mocks.qr } }))
beforeEach(() => { vi.clearAllMocks(); mocks.settings.mockResolvedValue({baseUrl:'https://assets.example.test'}); mocks.qr.mockResolvedValue('data:image/png;base64,fixture'); mocks.detail.mockResolvedValue({id:'device-1',code:'LATEST-001',name:'已保存设备'}) })
function render(deviceId='') { return mount(DeviceQrBinding,{ props:{ modelValue:createDevicePayload(),deviceId }, global:{ plugins:[ElementPlus],stubs:{AssetScanner:true} } }) }
it('keeps system generation disabled until the device has been saved', () => {
 const wrapper=render(); expect(wrapper.text()).toContain('保存设备后')
 expect(wrapper.findAll('button').filter(b=>b.text().includes('系统')).every(b=>b.attributes('disabled')!==undefined)).toBe(true)
 expect(mocks.settings).not.toHaveBeenCalled();wrapper.unmount()
})
it('previews a system QR with the configured address independent of external fields', async () => {
 const wrapper=render('device-1')
 await wrapper.findAll('button').find(b=>b.text()==='查看系统二维码')!.trigger('click'); await flushPromises()
 expect(mocks.qr).toHaveBeenCalledWith('https://assets.example.test/asset-scan?device=device-1',expect.anything())
 expect(wrapper.text()).toContain('系统二维码链接（只读）');wrapper.unmount()
})
it('classifies a scanned external URL or code without navigating to it', async () => {
 const wrapper=render()
 await wrapper.findAll('button').find(b=>b.text()==='扫描已有标签')!.trigger('click')
 wrapper.findComponent({name:'AssetScanner'}).vm.$emit('result','https://vendor.example/item/1');await flushPromises()
 expect(wrapper.props('modelValue').externalQrCodeUrl).toBe('https://vendor.example/item/1')
 await wrapper.findAll('button').find(b=>b.text()==='扫描已有标签')!.trigger('click')
 wrapper.findComponent({name:'AssetScanner'}).vm.$emit('result','QR-2026-001');await flushPromises()
 expect(wrapper.props('modelValue').externalQrCode).toBe('QR-2026-001');wrapper.unmount()
})
it('prints the saved record instead of unsaved editor changes', async () => {
 const wrapper=render('device-1')
 await wrapper.findAll('button').find(b=>b.text()==='打印系统标签')!.trigger('click'); await flushPromises()
 expect(mocks.print).toHaveBeenCalledWith('device',[{id:'device-1'}],{reloadDevices:true});wrapper.unmount()
})

it('does not bind an existing system QR as an external label',async()=>{
 const wrapper=render()
 await wrapper.findAll('button').find(b=>b.text()==='扫描已有标签')!.trigger('click')
 wrapper.findComponent({name:'AssetScanner'}).vm.$emit('result','https://assets.example/asset-scan?device=a4345ec9-b768-4e60-a706-0a84e3246d21');await flushPromises()
 expect(wrapper.props('modelValue').externalQrCodeUrl).toBe('');wrapper.unmount()
})

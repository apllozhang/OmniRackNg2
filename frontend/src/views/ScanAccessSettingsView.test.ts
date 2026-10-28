import ElementPlus from 'element-plus'
import { mount, flushPromises } from '@vue/test-utils'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import ScanAccessSettingsView from './ScanAccessSettingsView.vue'
const mocks = vi.hoisted(() => ({ load: vi.fn(), save: vi.fn(), qr: vi.fn() }))
vi.mock('@/api/scan-access', () => ({ getScanAccessSettings: mocks.load, saveScanAccessSettings: mocks.save }))
vi.mock('qrcode', () => ({ default: { toDataURL: mocks.qr } }))
vi.mock('@/api/asset-ops', () => ({ errorMessage: (e: Error) => e.message }))
beforeEach(() => { vi.resetAllMocks(); mocks.qr.mockResolvedValue('data:image/png;base64,fixture'); mocks.load.mockResolvedValue({ version: 3, baseUrl: 'https://assets.example.test' }); mocks.save.mockImplementation(async data => ({ ...data, version: data.version + 1 })) })
afterEach(() => { document.body.innerHTML = '' })
async function render() { const wrapper = mount(ScanAccessSettingsView, { attachTo: document.body, global: { plugins: [ElementPlus] } }); await flushPromises(); return wrapper }
it('shows saved query/check links and QR contents', async () => {
 const wrapper = await render()
 expect(wrapper.text()).toContain('https://assets.example.test/asset-scan')
 expect(mocks.qr).toHaveBeenCalledWith('https://assets.example.test/asset-checks', expect.anything())
 wrapper.unmount()
})
it('keeps previews on saved config until save succeeds, preserving version', async () => {
 const wrapper = await render()
 await wrapper.find('input').setValue('https://public.example.test/')
 expect(wrapper.text()).toContain('https://assets.example.test/asset-scan')
 await wrapper.findAll('button').find(b => b.text() === '保存配置')!.trigger('click'); await flushPromises()
 expect(mocks.save).toHaveBeenCalledWith({ version: 3, baseUrl: 'https://public.example.test' })
 expect(wrapper.text()).toContain('https://public.example.test/asset-scan')
 wrapper.unmount()
})
it('does not permit overwriting defaults after load fails', async () => {
 mocks.load.mockRejectedValue(new Error('配置不可用'))
 const wrapper = await render()
 expect(wrapper.findAll('button').find(b => b.text() === '保存配置')!.attributes('disabled')).toBeDefined()
 expect(mocks.save).not.toHaveBeenCalled(); wrapper.unmount()
})
it('rejects a path without sending a save request', async () => {
 const wrapper = await render(); await wrapper.find('input').setValue('https://public.example.test/asset-scan')
 await wrapper.findAll('button').find(b => b.text() === '保存配置')!.trigger('click'); await flushPromises()
 expect(mocks.save).not.toHaveBeenCalled(); wrapper.unmount()
})
it('supports clearing to the current origin and warns for HTTP camera access', async () => {
 mocks.load.mockResolvedValue({ version: 0, baseUrl: 'http://203.0.113.10:15173' })
 const wrapper = await render(); expect(wrapper.text()).toContain('手机浏览器通常不能调用摄像头')
 await wrapper.find('input').setValue('')
 await wrapper.findAll('button').find(b => b.text() === '保存配置')!.trigger('click'); await flushPromises()
 expect(mocks.save).toHaveBeenCalledWith({ version: 0, baseUrl: '' }); wrapper.unmount()
})

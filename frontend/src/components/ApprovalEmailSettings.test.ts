import ElementPlus, { ElSwitch, ElMessageBox } from 'element-plus'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import ApprovalEmailSettings from './ApprovalEmailSettings.vue'

const api = vi.hoisted(() => ({ load: vi.fn(), save: vi.fn(), test:vi.fn() }))
vi.mock('@/api/notifications', () => ({ getApprovalEmailSettings: api.load, saveApprovalEmailSettings: api.save, testApprovalNotification: api.test }))
vi.mock('@/api/asset-ops', () => ({ errorMessage: (e: Error) => e.message }))
const config = { version: 7, enabled: true, recipients: ['admin@example.test'], baseUrl: 'https://it.example.test', emailEnabled: true,weComEnabled:false,weComReady:true }
const wrappers: ReturnType<typeof mount>[] = []
async function render() {
  const w = mount(ApprovalEmailSettings, { global: { plugins: [ElementPlus], stubs: { RouterLink: { template: '<a><slot /></a>' } } } })
  wrappers.push(w); await flushPromises(); return w
}
beforeEach(() => { vi.clearAllMocks(); api.load.mockResolvedValue({ ...config }); api.save.mockResolvedValue({ ...config, version: 8 }) })
afterEach(() => { wrappers.splice(0).forEach(w => w.unmount()) })

it('saves independent recipients and the optimistic concurrency version', async () => {
  const w = await render()
  await w.find('textarea').setValue('admin@example.test;ops@example.test\nops@example.test')
  await w.findAll('button').find(b => b.text() === '保存通知设置')!.trigger('click'); await flushPromises()
  expect(api.save).toHaveBeenCalledWith({ ...config, recipients: ['admin@example.test', 'ops@example.test'] })
})
it('requires valid recipients and a usable address before enabling', async () => {
  const w = await render(); await w.find('textarea').setValue('wrong-address')
  await w.findAll('button').find(b => b.text() === '保存通知设置')!.trigger('click')
  expect(api.save).not.toHaveBeenCalled()
})
it('directs the administrator to SMTP settings and disables enabling before SMTP is ready', async () => {
  api.load.mockResolvedValue({ ...config, enabled: false, emailEnabled: false })
  const w = await render()
  expect(w.text()).toContain('请先启用 SMTP 邮件')
  expect(w.findComponent(ElSwitch).props('disabled')).toBe(true)
})
it('retains unsaved input after save failure and allows a version reload', async () => {
  api.save.mockRejectedValue(new Error('配置已被修改，请重新加载'))
  const w = await render(); await w.find('textarea').setValue('other@example.test')
  await w.findAll('button').find(b => b.text() === '保存通知设置')!.trigger('click'); await flushPromises()
  expect((w.find('textarea').element as HTMLTextAreaElement).value).toBe('other@example.test')
  await w.findAll('button').find(b => b.text() === '重新加载')!.trigger('click'); await flushPromises()
  expect((w.find('textarea').element as HTMLTextAreaElement).value).toBe('admin@example.test')
})
it('saves email and WeCom independently and tests the saved WeCom channel',async()=>{
  api.save.mockResolvedValue({...config,weComEnabled:true,version:8});api.test.mockResolvedValue({status:'SUCCESS'})
  vi.spyOn(ElMessageBox,'confirm').mockImplementation(async()=>undefined as never)
  const w=await render()
  w.findAllComponents(ElSwitch)[1]!.vm.$emit('update:modelValue',true);await flushPromises()
  await w.findAll('button').find(b=>b.text()==='保存通知设置')!.trigger('click');await flushPromises()
  expect(api.save).toHaveBeenCalledWith(expect.objectContaining({enabled:true,weComEnabled:true,version:7}))
  await w.findAll('button').find(b=>b.text()==='测试审批企微')!.trigger('click');await flushPromises()
  expect(api.test).toHaveBeenCalledWith('WECOM')
})
it('prevents enabling approval WeCom before the robot is configured',async()=>{
  api.load.mockResolvedValue({...config,weComReady:false})
  const w=await render()
  expect(w.findAllComponents(ElSwitch)[1]!.props('disabled')).toBe(true)
})

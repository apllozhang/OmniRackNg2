import ElementPlus, { ElInputNumber, ElMessage, ElMessageBox } from 'element-plus'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import NotificationSettingsView from './NotificationSettingsView.vue'

const mocks = vi.hoisted(() => ({ load: vi.fn(), save: vi.fn(), test: vi.fn(), history: vi.fn() }))
vi.mock('@/api/notifications', () => ({ getNotificationSettings: mocks.load, saveNotificationSettings: mocks.save, testNotification: mocks.test, getNotificationDeliveries: mocks.history }))
vi.mock('@/api/asset-ops', () => ({ errorMessage: (e: Error) => e.message }))
const settings = { version: 3, emailEnabled: true, smtpHost: 'smtp.example.test', smtpPort: 587, smtpSecurity: 'STARTTLS', smtpUsername: 'assets@example.test', emailFrom: 'assets@example.test', emailRecipients: ['ops@example.test'], hasSmtpPassword: true, weComEnabled: true, hasWeComWebhook: true, warrantyEnabled: true, rackUEnabled: true, rackWarningPercent: 70, rackCriticalPercent: 90, repeatHours: 24 }
const wrappers: ReturnType<typeof mount>[] = []
const render = async () => { const wrapper = mount(NotificationSettingsView, { attachTo: document.body, global: { plugins: [ElementPlus] } }); wrappers.push(wrapper); await flushPromises(); return wrapper }
beforeEach(() => { mocks.load.mockResolvedValue({ ...settings }); mocks.save.mockResolvedValue({ ...settings, version: 4 }); mocks.history.mockResolvedValue({ items: [], total: 0 }); mocks.test.mockResolvedValue({ status: 'SUCCESS' }) })
afterEach(() => { wrappers.splice(0).forEach(w => w.unmount()); document.body.innerHTML = '' })

describe('notification settings', () => {
  it('preserves saved secrets through blank replacement fields and retains the version', async () => {
    const wrapper = await render()
    expect(wrapper.find('input[autocomplete="new-password"]').element).toHaveProperty('value', '')
    expect(wrapper.text()).toContain('凭据已加密保存')
    const button = wrapper.findAll('button').find(b => b.text() === '保存配置')!
    await button.trigger('click'); await flushPromises()
    expect(mocks.save).toHaveBeenCalledWith(expect.objectContaining({ version: 3, smtpPassword: '', weComWebhook: '', clearSmtpPassword: false, clearWeComWebhook: false, emailRecipients: ['ops@example.test'] }))
  })
  it('blocks inverted U thresholds before saving', async () => {
    const wrapper = await render()
    const warning = wrapper.findAll('.el-form-item').find(item => item.text().includes('U 位预警阈值'))!
    warning.findComponent(ElInputNumber).vm.$emit('update:modelValue', 95)
    await flushPromises()
    await wrapper.findAll('button').find(b => b.text() === '保存配置')!.trigger('click')
    expect(mocks.save).not.toHaveBeenCalled()
  })
  it('shows channel failures without loading delivery history in configuration', async () => {
    vi.spyOn(ElMessageBox, 'confirm').mockImplementation(async () => undefined as never)
    const error = vi.spyOn(ElMessage, 'error')
    const success = vi.spyOn(ElMessage, 'success')
    mocks.test.mockRejectedValue(new Error('SMTP 认证失败'))
    const wrapper = await render()
    await wrapper.findAll('button').find(b => b.text() === '发送测试邮件')!.trigger('click'); await flushPromises()
    expect(mocks.test).toHaveBeenCalledWith('EMAIL')
    expect(error).toHaveBeenCalledWith('SMTP 认证失败')
    expect(success).not.toHaveBeenCalled()
    expect(mocks.history).not.toHaveBeenCalled()
  })
  it('does not permit saving fallback defaults after settings fail to load', async () => {
    mocks.load.mockRejectedValue(new Error('服务不可用'))
    const wrapper = await render()
    expect(wrapper.findAll('button').find(b => b.text() === '保存配置')!.attributes('disabled')).toBeDefined()
  })
})

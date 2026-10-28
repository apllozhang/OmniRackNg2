import { ops } from './asset-ops'

export interface NotificationSettings {
  version: number
  emailEnabled: boolean
  smtpHost: string
  smtpPort: number
  smtpSecurity: 'STARTTLS' | 'TLS' | 'NONE'
  smtpUsername: string
  emailFrom: string
  emailRecipients: string[]
  hasSmtpPassword: boolean
  weComEnabled: boolean
  hasWeComWebhook: boolean
  warrantyEnabled: boolean
  rackUEnabled: boolean
  rackWarningPercent: number
  rackCriticalPercent: number
  repeatHours: number
  updatedAt?: string
}
export interface NotificationForm extends NotificationSettings {
  smtpPassword: string
  weComWebhook: string
  clearSmtpPassword: boolean
  clearWeComWebhook: boolean
}
export interface NotificationDelivery {
	messageId?: string
	recipients?: string[]
  id: string
  createdAt: string
  channel: 'EMAIL' | 'WECOM'
  status: 'SUCCESS' | 'FAILED'
  summary: string
  errorMessage: string
  alertCount: number
  test: boolean
}
export const getNotificationSettings = () => ops<NotificationSettings>('notifications/settings')
export const saveNotificationSettings = (form: NotificationForm) => ops<NotificationSettings>('notifications/settings', 'PUT', form)
export const testNotification = (channel: 'EMAIL' | 'WECOM') => ops('notifications/test', 'POST', { channel })
export const getNotificationDeliveries = (page: number) => ops<{ items: NotificationDelivery[]; total: number }>(`notifications/deliveries?page=${page}`)

export interface ApprovalEmailSettings {
	weComEnabled: boolean
	weComReady: boolean
  version: number
  enabled: boolean
  recipients: string[]
  baseUrl: string
  emailEnabled: boolean
}
export const getApprovalEmailSettings = () => ops<ApprovalEmailSettings>('notifications/approval-settings')
export const saveApprovalEmailSettings = (form: ApprovalEmailSettings) => ops<ApprovalEmailSettings>('notifications/approval-settings', 'PUT', form)
export const testApprovalNotification = (channel: 'EMAIL' | 'WECOM') => ops<NotificationDelivery>('notifications/approval-test', 'POST', {channel})

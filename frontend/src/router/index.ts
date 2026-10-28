import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const LoginView = () => import('@/views/LoginView.vue')
const DashboardView = () => import('@/views/DashboardView.vue')
const DefaultLayout = () => import('@/layouts/DefaultLayout.vue')
const ResourceManagementView = () => import('@/views/ResourceManagementView.vue')
const RackManagementView = () => import('@/views/RackManagementView.vue')
const RackTemplateView = () => import('@/views/RackTemplateView.vue')
const DeviceManagementView = () => import('@/views/DeviceManagementView.vue')
const RoomScreenView = () => import('@/views/RoomScreenView.vue')
const AdminView = () => import('@/views/AdminView.vue')

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', name: 'login', component: LoginView, meta: { public: true } },
    { path: '/room-screen', name: 'room-screen', component: RoomScreenView },
    {
      path: '/',
      component: DefaultLayout,
      children: [
        { path: '', name: 'dashboard', component: DashboardView },
        { path: 'data-centers', name: 'data-centers', component: ResourceManagementView },
        { path: 'racks', name: 'racks', component: RackManagementView },
        { path: 'rack-templates', name: 'rack-templates', component: RackTemplateView },
        { path: 'devices', name: 'devices', component: DeviceManagementView },
        { path: 'room-devices', name: 'room-devices', component: DeviceManagementView, meta: { deviceScope: 'ROOM' } },
        { path: 'pc-assets', alias: 'terminal-assets', name: 'pc-assets', component: () => import('@/views/PCAssetsView.vue') },
        { path: 'asset-operations', name: 'asset-operations', component: () => import('@/views/AssetOperationsView.vue') },
        { path: 'asset-locations', name: 'asset-locations', component: () => import('@/views/AssetLocationsView.vue'), meta: { requiresAssets: true } },
        { path: 'asset-reminders', name: 'asset-reminders', component: () => import('@/views/AssetRemindersView.vue'), meta: { requiresAssets: true } },
        { path: 'asset-audit', name: 'asset-audit', component: () => import('@/views/AssetAuditView.vue'), meta: { requiresAssets: true } },
        { path: 'asset-data', name: 'asset-data', component: () => import('@/views/AssetDataView.vue'), meta: { requiresAssets: true } },
        { path: 'spare-parts', name: 'spare-parts', component: () => import('@/views/SparePartsView.vue') },
        { path: 'asset-checks', name: 'asset-checks', component: () => import('@/views/AssetChecksView.vue') },
        { path: 'asset-scan', name: 'asset-scan', component: () => import('@/views/AssetScanView.vue') },
        { path: 'approvals', name: 'approvals', component: () => import('@/views/ApprovalCenterView.vue'), meta: { requiresAdmin: true } },
        { path: 'approval-rules', name: 'approval-rules', component: () => import('@/views/ApprovalRulesView.vue'), meta: { requiresAdmin: true } },
        { path: 'notification-deliveries', name: 'notification-deliveries', component: () => import('@/views/NotificationDeliveriesView.vue'), meta: { requiresAdmin: true } },
        { path: 'admin', name: 'admin', component: AdminView, meta: { requiresAdmin: true } },
        { path: 'scan-access-settings', name: 'scan-access-settings', component: () => import('@/views/ScanAccessSettingsView.vue'), meta: { requiresAdmin: true } },
        { path: 'notification-settings', name: 'notification-settings', component: () => import('@/views/NotificationSettingsView.vue'), meta: { requiresAdmin: true } },
      ],
    },
  ],
})

export function legacyAssetDestination(path: string, tab: unknown): string | undefined {
  if (path === '/admin' && tab === 'approval') return '/approvals'
  if (path === '/notification-settings' && tab === 'history') return '/notification-deliveries'
  if (path !== '/asset-operations') return
  switch (tab) {
    case 'sites': return '/asset-locations'
    case 'reminders': return '/asset-reminders'
    case 'audit': return '/asset-audit'
    case 'backup': return '/asset-data'
  }
}

router.beforeEach(async (to) => {
  const destination = legacyAssetDestination(to.path, to.query.tab)
  if (destination) {
    const { tab, ...query } = to.query
    return { path: destination, query, hash: to.hash, replace: true }
  }
  const auth = useAuthStore()
  if (to.meta.public) {
    if (to.name === 'login' && auth.isAuthenticated && auth.user) return { name: 'dashboard' }
    return true
  }
  if (!auth.isAuthenticated) return { name: 'login', query: { redirect: to.fullPath } }
  if (!auth.user) {
    try {
      await auth.loadUser()
    } catch {
      return { name: 'login', query: { redirect: to.fullPath, reason: 'service-unavailable' } }
    }
  }
  if (!auth.isAuthenticated) return { name: 'login' }
  if (to.meta.requiresAdmin && !auth.isAdmin) return { name: 'dashboard' }
  if (to.meta.requiresAssets && !auth.user?.roles?.some(r => ['system_admin', 'asset_manager', 'asset_auditor'].includes(r.code))) return { name: 'dashboard' }
  return true
})

export default router

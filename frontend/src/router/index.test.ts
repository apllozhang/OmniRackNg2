import { describe, expect, it } from 'vitest'
import router, { legacyAssetDestination } from './index'

const lazyRouteNames = [
  'login',
  'room-screen',
  'dashboard',
  'data-centers',
  'racks',
  'rack-templates',
  'devices',
  'admin',
  'asset-locations',
  'asset-reminders',
  'asset-audit',
  'asset-data',
  'approvals',
  'approval-rules',
  'notification-deliveries',
]

describe('router lazy loading', () => {
  it.each([['sites','/asset-locations'],['reminders','/asset-reminders'],['audit','/asset-audit'],['backup','/asset-data']])('preserves the old %s bookmark', (tab,path) => {
    expect(legacyAssetDestination('/asset-operations',tab)).toBe(path)
    expect(router.resolve(path).meta.requiresAssets).toBe(true)
  })
  it('redirects old approval links and restricts administration routes',()=>{
    expect(legacyAssetDestination('/admin','approval')).toBe('/approvals')
    expect(legacyAssetDestination('/notification-settings','history')).toBe('/notification-deliveries')
    for(const path of ['/approvals','/approval-rules','/notification-deliveries'])expect(router.resolve(path).meta.requiresAdmin).toBe(true)
  })
  it('does not redirect the configuration overview or other pages', () => {
    expect(legacyAssetDestination('/asset-operations','devices')).toBeUndefined()
    expect(legacyAssetDestination('/admin','sites')).toBeUndefined()
    expect(legacyAssetDestination('/asset-operations','constructor')).toBeUndefined()
    expect(legacyAssetDestination('/asset-operations',['sites','audit'])).toBeUndefined()
  })
	 it('preserves the old terminal entry while exposing the unified terminal URL', () => {
	   expect(router.resolve('/terminal-assets').matched.some(r => r.name === 'pc-assets')).toBe(true)
	   expect(router.resolve('/pc-assets').matched.some(r => r.name === 'pc-assets')).toBe(true)
	 })
  it.each(lazyRouteNames)('loads the %s route through a dynamic import', (routeName) => {
    const route = router.getRoutes().find((item) => item.name === routeName)

    expect(route).toBeDefined()
    expect(typeof route?.components?.default).toBe('function')
  })

  it('loads the shared application layout through a dynamic import', () => {
    const rootRoute = router.getRoutes().find((item) => item.path === '/')

    expect(rootRoute).toBeDefined()
    expect(typeof rootRoute?.components?.default).toBe('function')
  })
})

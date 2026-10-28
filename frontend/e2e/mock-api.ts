import type { Page, Route } from '@playwright/test'
import { currentUser, device, rackLayouts, racks, resourceTree } from './fixtures'

function envelope<T>(data: T) {
  return { code: 'OK', message: 'success', data }
}

async function fulfill(route: Route, data: unknown, status = 200) {
  await route.fulfill({
    status,
    contentType: 'application/json',
    body: JSON.stringify(envelope(data)),
  })
}

export async function installMockApi(page: Page) {
  await page.route('**/api/v1/**', async (route) => {
    const request = route.request()
    const url = new URL(request.url())
    const path = url.pathname

    if (request.method() === 'GET' && path === '/api/v1/auth/me') return fulfill(route, currentUser)
    if (request.method() === 'GET' && path === '/api/v1/resource-tree') return fulfill(route, resourceTree)
    if (request.method() === 'GET' && path === '/api/v1/rack-templates') return fulfill(route, { items: [], total: 0, page: 1, pageSize: 100 })
    if (request.method() === 'GET' && path === '/api/v1/devices') {
      return fulfill(route, { items: [device], total: 1, page: 1, pageSize: 100 })
    }

    const layoutMatch = path.match(/^\/api\/v1\/racks\/([^/]+)\/u-layout$/)
    if (request.method() === 'GET' && layoutMatch) {
      const layout = rackLayouts[decodeURIComponent(layoutMatch[1]) as keyof typeof rackLayouts]
      return layout ? fulfill(route, layout) : fulfill(route, null, 404)
    }

    const rackMatch = path.match(/^\/api\/v1\/racks\/([^/]+)$/)
    if (request.method() === 'PUT' && rackMatch) {
      const rackId = decodeURIComponent(rackMatch[1])
      const original = racks.find((item) => item.id === rackId)
      const payload = request.postDataJSON() as Record<string, unknown>
      return original
        ? fulfill(route, { ...original, ...payload, id: rackId, version: original.version + 1 })
        : fulfill(route, null, 404)
    }

    return fulfill(route, null, 404)
  })
}

import { expect, test as setup } from '@playwright/test'
import { mkdir } from 'node:fs/promises'

const authFile = '.auth/user.json'
const liveMode = process.env.E2E_LIVE === 'true'
const username = process.env.E2E_USERNAME ?? 'admin'
const password = process.env.E2E_PASSWORD ?? 'playwright-local-only'

function envelope<T>(data: T) {
  return { code: 'OK', message: 'success', data }
}

setup('通过登录页建立可复用的已登录状态', async ({ page }) => {
  if (liveMode && (!process.env.E2E_USERNAME || !process.env.E2E_PASSWORD)) {
    throw new Error('E2E_LIVE=true 时必须提供 E2E_USERNAME 和 E2E_PASSWORD')
  }

  if (!liveMode) {
    await page.route('**/api/v1/auth/login', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(envelope({
          token: 'playwright-authenticated-token',
          user: {
            id: 'user-e2e',
            username,
            displayName: 'Playwright 回归用户',
            authSource: 'local',
            enabled: true,
            roles: [{ id: 'role-admin', code: 'ADMIN', name: '管理员' }],
          },
        })),
      })
    })
    await page.route('**/api/v1/auth/me', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(envelope({
          id: 'user-e2e',
          username,
          displayName: 'Playwright 回归用户',
          authSource: 'local',
          enabled: true,
          roles: [{ id: 'role-admin', code: 'ADMIN', name: '管理员' }],
        })),
      })
    })
    await page.route('**/api/v1/resource-tree', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(envelope({ items: [] })),
      })
    })
  }

  await page.goto('/login?redirect=/room-screen')
  await page.getByLabel('用户名').fill(username)
  await page.getByLabel('密码').fill(password)
  await page.getByRole('button', { name: '登录' }).click()

  await expect(page).toHaveURL(/\/room-screen(?:\?.*)?$/)
  await expect.poll(() => page.evaluate(() => localStorage.getItem('cabinet_access_token'))).not.toBeNull()
  await mkdir('.auth', { recursive: true })
  await page.context().storageState({ path: authFile })
})

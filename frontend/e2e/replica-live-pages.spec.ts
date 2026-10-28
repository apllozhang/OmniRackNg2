import { expect, test } from '@playwright/test'

// Real backend only. No route.fulfill(), fabricated tokens, or production URL.
const routes = [
  '/', '/room-screen', '/data-centers', '/racks', '/rack-templates', '/devices',
  '/room-devices', '/pc-assets', '/terminal-assets', '/asset-operations',
  '/asset-locations', '/asset-reminders', '/asset-audit', '/asset-data',
  '/spare-parts', '/asset-checks', '/asset-scan', '/approvals', '/approval-rules',
  '/notification-deliveries', '/admin', '/scan-access-settings', '/notification-settings',
]

for (const path of routes) {
  test(`真实接口加载页面 ${path}`, async ({ page }, testInfo) => {
    const errors: string[] = []
    const apiFailures: string[] = []
    page.on('pageerror', error => errors.push(error.message))
    page.on('response', response => {
      if (new URL(response.url()).pathname.startsWith('/api/') && response.status() >= 400) {
        apiFailures.push(`${response.status()} ${response.url()}`)
      }
    })
    await page.goto(path)
    await page.waitForLoadState('networkidle')
    await expect(page).toHaveURL(new RegExp(`${path.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}$`))
    await expect(page.locator('h1, h2').first()).toBeVisible()
    await expect(page.locator('.el-loading-mask')).toHaveCount(0, { timeout: 20000 })
    await expect(page.locator('.el-message--error')).toHaveCount(0)
    expect(errors).toEqual([])
    expect(apiFailures).toEqual([])
    if (['/', '/pc-assets', '/spare-parts', '/room-screen'].includes(path)) {
      await testInfo.attach(`page-${path.replaceAll('/', '') || 'dashboard'}.png`, {
        body: await page.screenshot({ fullPage: true }), contentType: 'image/png',
      })
    }
  })
}

test('WebUI theme persists and navigation fits narrow screens', async ({ page }, testInfo) => {
  await page.goto('/')
  const toggle = page.getByRole('button', { name: '切换亮暗主题' })
  await expect(toggle).toBeVisible()
  const original = await page.locator('html').getAttribute('data-theme')
  const backgroundBefore = await page.locator('.main-content').evaluate(el => getComputedStyle(el).backgroundColor)
  await toggle.click()
  const changed = await page.locator('html').getAttribute('data-theme')
  expect(changed).not.toBe(original)
  expect(await page.locator('.main-content').evaluate(el => getComputedStyle(el).backgroundColor)).not.toBe(backgroundBefore)
  await page.reload()
  await expect(page.locator('html')).toHaveAttribute('data-theme', changed!)
  for (const width of [1440, 768, 320]) {
    await page.setViewportSize({ width, height: 900 })
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(width)
    await testInfo.attach(`webui-${changed}-${width}.png`, { body: await page.screenshot({ fullPage: true }), contentType: 'image/png' })
  }
  await page.getByRole('button', { name: '打开导航' }).click()
  await expect(page.getByRole('button', { name: '打开导航' })).toHaveAttribute('aria-expanded', 'true')
  await page.keyboard.press('Escape')
  await expect(page.getByRole('button', { name: '打开导航' })).toHaveAttribute('aria-expanded', 'false')
})

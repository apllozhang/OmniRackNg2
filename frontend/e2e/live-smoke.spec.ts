import { expect, test } from '@playwright/test'

test('真实服务器登录后可无损加载大屏与资源树', async ({ page }, testInfo) => {
  const resourceResponse = page.waitForResponse((response) =>
    response.url().includes('/api/v1/resource-tree') && response.request().method() === 'GET',
  )

  await page.goto('/room-screen')
  const response = await resourceResponse

  expect(response.ok()).toBeTruthy()
  await expect(page).toHaveURL(/\/room-screen$/)
  await expect(page.getByRole('heading', { name: '机房资源大屏' })).toBeVisible()
  await expect(page.locator('.screen-page')).toBeVisible()
  await expect(page.locator('.el-loading-mask')).toHaveCount(0, { timeout: 20_000 })

  const token = await page.evaluate(() => localStorage.getItem('cabinet_access_token'))
  expect(token).toBeTruthy()

  const screenshot = await page.screenshot({ fullPage: true })
  await testInfo.attach('deployed-room-screen.png', { body: screenshot, contentType: 'image/png' })
})

import { expect, test } from '@playwright/test'
import { installMockApi } from './mock-api'

async function openRoomScreen(page: import('@playwright/test').Page) {
  await installMockApi(page)
  await page.setViewportSize({ width: 1440, height: 1000 })
  await page.goto('/room-screen')
  await expect(page).toHaveURL(/\/room-screen$/)
  await expect(page.locator('.rack-card')).toHaveCount(4)
  await expect(page.locator('.rack-state-overlay')).toHaveCount(0)
}

test.beforeEach(async ({ page }) => {
  await openRoomScreen(page)
})

test('复用登录状态访问受保护的大屏并加载机房数据', async ({ page }) => {
  await expect(page.getByRole('heading', { name: '机房资源大屏' })).toBeVisible()
  await expect(page.getByRole('heading', { name: /自动化测试数据中心.*UI 回归机房/ })).toBeVisible()
  await expect(page.locator('.canvas-info')).toContainText('2 排 · 4 个机柜')
  await expect(page.locator('.canvas-info')).toContainText('2/48U 已使用')
  await expect.poll(() => page.evaluate(() => localStorage.getItem('cabinet_access_token'))).toBeTruthy()
})

test('已有设备与空 U 位左键菜单可直接操作', async ({ page }) => {
  await page.locator('.device-block').click()
  const deviceMenu = page.locator('.rack-context-menu')
  await expect(deviceMenu).toContainText('设备快捷操作')
  await expect(deviceMenu).toContainText('核心业务服务器01')
  await expect(deviceMenu.getByRole('button', { name: /查看设备详情/ })).toBeVisible()
  await expect(deviceMenu.getByRole('button', { name: '调整 / 迁移位置' })).toBeVisible()
  await expect(deviceMenu.getByRole('button', { name: /下架设备/ })).toBeVisible()

  await page.locator('.rack-card').nth(1).locator('.u-line').first().click()
  const emptyMenu = page.locator('.rack-context-menu')
  await expect(emptyMenu).toContainText('空 U 位快捷操作')
  await expect(emptyMenu).toContainText('02号机柜')
  await expect(emptyMenu.getByRole('button', { name: /上架设备到此 U 位/ })).toBeVisible()
  await expect(emptyMenu.getByRole('button', { name: /查看完整 U 位详情/ })).toBeVisible()
})

test('机柜卡片详情展示机柜图、参数和无法计算原因', async ({ page }) => {
  await page.locator('.rack-card').first().locator('.rack-card-footer button').click()

  const dialog = page.locator('.rack-capacity-dialog')
  await expect(dialog).toBeVisible()
  await expect(dialog.getByText('完整 U 位图')).toBeVisible()
  await expect(dialog.getByText('核心业务服务器01', { exact: true })).toBeVisible()
  await expect(dialog.getByText('机柜参数')).toBeVisible()
  await expect(dialog.getByText('容量与利用率')).toBeVisible()
  await expect(dialog.getByText('无法计算').first()).toBeVisible()
  await expect(dialog.getByText('机柜未填写额定功率容量')).toBeVisible()
  await expect(dialog.getByText('系统未维护 PDU 插座总数和设备实际占用插座数')).toBeVisible()

  const dialogBox = await dialog.boundingBox()
  const shellBox = await dialog.locator('.rack-detail-shell').boundingBox()
  const footerBox = await dialog.locator('.el-dialog__footer').boundingBox()
  expect(dialogBox).not.toBeNull()
  expect(shellBox).not.toBeNull()
  expect(footerBox).not.toBeNull()
  expect(dialogBox!.y + dialogBox!.height).toBeLessThanOrEqual(1001)
  expect(shellBox!.y + shellBox!.height).toBeLessThanOrEqual(footerBox!.y + 1)

  await dialog.locator('.rack-information-column').evaluate((element) => {
    element.scrollTop = element.scrollHeight
  })
  await expect(dialog.locator('.remarks-section')).toBeVisible()
  const remarksBox = await dialog.locator('.remarks-section').boundingBox()
  expect(remarksBox).not.toBeNull()
  expect(remarksBox!.y + remarksBox!.height).toBeLessThanOrEqual(footerBox!.y + 1)
})

test('机柜状态编辑入口和状态筛选可用', async ({ page }) => {
  await page.locator('.rack-status-chip').nth(1).click()
  const dialog = page.getByRole('dialog', { name: '编辑机柜' })
  await expect(dialog).toBeVisible()
  await expect(dialog.getByLabel('编码')).toHaveValue('RACK-A02')
  await expect(dialog.getByLabel('名称')).toHaveValue('02号机柜')
  await dialog.getByRole('button', { name: '取消' }).click()
  await expect(dialog).toBeHidden()

  await page.locator('.filter-chip').filter({ hasText: '维护中' }).click()
  await expect(page.locator('.rack-card')).toHaveCount(1)
  await expect(page.locator('.rack-card')).toContainText('03号机柜')
  await page.locator('.filter-chip').filter({ hasText: '全部' }).click()
  await expect(page.locator('.rack-card')).toHaveCount(4)
})

test('可导出当前机房全部机柜的 Excel 机柜图', async ({ page }) => {
  const downloadPromise = page.waitForEvent('download')
  await page.getByRole('button', { name: '导出机柜图' }).click()
  const download = await downloadPromise

  expect(download.suggestedFilename()).toMatch(/^DC-E2E-ROOM-E2E-机柜图-\d{14}\.xlsx$/)
  await expect(page.getByText('已导出 4 个机柜的机柜图')).toBeVisible()
})

test('大屏整体与机房边界视觉基线', async ({ page }) => {
  await page.addStyleTag({ content: `
    *, *::before, *::after { animation: none !important; transition: none !important; }
    .clock strong, .clock span { visibility: hidden !important; }
  ` })
  await expect(page).toHaveScreenshot('room-screen-overview.png', { fullPage: true })
  await expect(page.locator('.room-boundary')).toHaveScreenshot('room-boundary.png')
})

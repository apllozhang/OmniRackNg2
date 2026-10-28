import { expect, test } from '@playwright/test'
import { installMockApi } from './mock-api'

test.beforeEach(async ({ page }) => {
  await installMockApi(page)
  await page.setViewportSize({ width: 1440, height: 1000 })
})

test('资源层级页的机柜卡片不受大屏机柜固定宽度样式影响', async ({ page }) => {
  await page.goto('/data-centers')
  await expect(page.getByRole('heading', { name: '基础资源管理' })).toBeVisible()

  const rackCard = page.locator('.resource-card.rack-card')
  await expect(rackCard).toBeVisible()
  await expect(rackCard.locator('.el-table')).toBeVisible()

  const layout = await rackCard.evaluate((element) => {
    const style = getComputedStyle(element)
    const rect = element.getBoundingClientRect()
    const tableRect = element.querySelector('.el-table')?.getBoundingClientRect()
    return {
      width: rect.width,
      minWidth: style.minWidth,
      maxWidth: style.maxWidth,
      backgroundColor: style.backgroundColor,
      tableRight: tableRect?.right ?? 0,
      cardRight: rect.right,
    }
  })

  expect(layout.width).toBeGreaterThan(400)
  expect(layout.minWidth).not.toBe('218px')
  expect(layout.maxWidth).not.toBe('218px')
  expect(layout.backgroundColor).toBe('rgb(255, 255, 255)')
  expect(layout.tableRight).toBeLessThanOrEqual(layout.cardRight + 1)
})

test('机柜管理筛选栏保持普通白色卡片样式', async ({ page }) => {
  await page.goto('/racks')
  await expect(page.getByRole('heading', { name: '机柜管理' })).toBeVisible()

  const filterCard = page.locator('.filter-card')
  const filterBar = filterCard.locator('.filter-bar')
  await expect(filterCard).toBeVisible()
  await expect(filterBar).toBeVisible()

  const colors = await filterCard.evaluate((element) => {
    const body = element.querySelector('.el-card__body') as HTMLElement | null
    const bar = element.querySelector('.filter-bar') as HTMLElement | null
    const bodyStyle = body ? getComputedStyle(body) : null
    const barStyle = bar ? getComputedStyle(bar) : null
    return {
      cardBackground: getComputedStyle(element).backgroundColor,
      bodyBackground: bodyStyle?.backgroundColor,
      barBackground: barStyle?.backgroundColor,
      barPaddingTop: barStyle?.paddingTop,
      barBorderBottomWidth: barStyle?.borderBottomWidth,
    }
  })

  expect(colors.cardBackground).toBe('rgb(255, 255, 255)')
  expect(colors.bodyBackground).toBe('rgba(0, 0, 0, 0)')
  expect(colors.barBackground).toBe('rgba(0, 0, 0, 0)')
  expect(colors.barPaddingTop).toBe('0px')
  expect(colors.barBorderBottomWidth).toBe('0px')
})

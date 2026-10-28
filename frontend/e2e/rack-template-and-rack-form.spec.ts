import { expect, test } from '@playwright/test'
import { installMockApi } from './mock-api'

const timestamp = '2026-09-01T08:00:00Z'
const rackTemplate = {
  id: 'template-ui-1', code: 'TPL-UI-42U', name: 'UI 测试模板', status: 'ACTIVE', isSystem: false,
  currentRevision: 1, version: 1, createdAt: timestamp, updatedAt: timestamp,
  versions: [{
    id: 'template-version-ui-1', templateId: 'template-ui-1', revision: 1, version: 1,
    createdAt: timestamp, updatedAt: timestamp, type: 'STANDARD', manufacturer: '测试厂商', modelNumber: 'UI-42U',
    uHeight: 42, widthMm: 800, depthMm: 1200, heightMm: 2200,
    dualPower: true, inputCircuits: 2, ratedVoltage: 220, ratedCurrent: 32, ratedPowerKw: 7, peakPowerKw: 9, pduCount: 4,
  }],
}

function envelope<T>(data: T) {
  return { code: 'OK', message: 'success', data }
}

async function fillNumber(locator: import('@playwright/test').Locator, value: string) {
  await locator.click()
  await locator.fill(value)
  await locator.press('Tab')
}

test('机柜模板默认自动编号且尺寸输入框可编辑并正确提交', async ({ page }) => {
  await installMockApi(page)
  let submitted: Record<string, any> | undefined
  await page.route('**/api/v1/rack-templates', async (route) => {
    const request = route.request()
    if (request.method() === 'GET') {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(envelope({ items: [] })) })
      return
    }
    if (request.method() === 'POST') {
      submitted = request.postDataJSON() as Record<string, any>
      await route.fulfill({
        status: 201, contentType: 'application/json',
        body: JSON.stringify(envelope({ ...submitted, id: 'template-new', code: 'TPL-A1B2C3D4E5F6', version: 1, currentRevision: 1, isSystem: false, versions: [] })),
      })
      return
    }
    await route.fallback()
  })

  await page.goto('/rack-templates')
  await page.getByRole('button', { name: '新增模板' }).click()
  const dialog = page.getByRole('dialog', { name: '新增机柜模板' })
  await expect(dialog).toBeVisible()
  await expect(dialog.getByLabel('模板编码')).toBeDisabled()
  await expect(dialog.getByLabel('宽(mm)')).toHaveValue('600')
  await expect(dialog.getByLabel('深(mm)')).toHaveValue('1200')
  await expect(dialog.getByLabel('高(mm)')).toHaveValue('2000')

  await dialog.getByLabel('模板名称').fill('自动编号可调规格模板')
  await fillNumber(dialog.getByLabel('宽(mm)'), '750')
  await fillNumber(dialog.getByLabel('深(mm)'), '1300')
  await fillNumber(dialog.getByLabel('高(mm)'), '2100')
  await dialog.getByRole('button', { name: '保存' }).click()

  await expect.poll(() => submitted).toBeTruthy()
  expect(submitted).toMatchObject({ code: '', autoGenerateCode: true, name: '自动编号可调规格模板' })
  expect(submitted?.spec).toMatchObject({ widthMm: 750, depthMm: 1300, heightMm: 2100 })
})

test('新增机柜选择模板后仍可调整实例尺寸并提交覆盖标记', async ({ page }) => {
  await installMockApi(page)
  let submitted: Record<string, any> | undefined
  await page.route('**/api/v1/rack-templates', async (route) => {
    if (route.request().method() === 'GET') {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(envelope({ items: [rackTemplate] })) })
      return
    }
    await route.fallback()
  })
  await page.route('**/api/v1/rooms/*/racks', async (route) => {
    if (route.request().method() !== 'POST') return route.fallback()
    submitted = route.request().postDataJSON() as Record<string, any>
    await route.fulfill({
      status: 201, contentType: 'application/json',
      body: JSON.stringify(envelope({ ...submitted, id: 'rack-new', code: 'RACK-A1B2C3D4E5F6', version: 1, createdAt: timestamp, updatedAt: timestamp })),
    })
  })

  await page.goto('/racks')
  await page.getByRole('button', { name: '新增机柜', exact: true }).first().click()
  const dialog = page.getByRole('dialog', { name: '新增机柜' })
  await expect(dialog).toBeVisible()
  await dialog.locator('.el-form-item').filter({ hasText: '机柜模板' }).locator('.el-select').click()
  await page.getByRole('option', { name: /UI 测试模板/ }).click()
  await expect(dialog.getByLabel('宽(mm)')).toHaveValue('800')
  await expect(dialog.getByLabel('深(mm)')).toHaveValue('1200')
  await expect(dialog.getByLabel('高(mm)')).toHaveValue('2200')

  await dialog.getByLabel('名称').fill('实例规格覆盖机柜')
  await fillNumber(dialog.getByLabel('宽(mm)'), '900')
  await fillNumber(dialog.getByLabel('深(mm)'), '1300')
  await fillNumber(dialog.getByLabel('高(mm)'), '2300')
  await dialog.getByRole('button', { name: '保存' }).click()

  await expect.poll(() => submitted).toBeTruthy()
  expect(submitted).toMatchObject({
    code: '', autoGenerateCode: true, overrideTemplateSpec: true,
    templateVersionId: 'template-version-ui-1', name: '实例规格覆盖机柜',
    widthMm: 900, depthMm: 1300, heightMm: 2300,
  })
})


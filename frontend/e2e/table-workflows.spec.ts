import { expect, test } from '@playwright/test'
import XLSX from 'xlsx-js-style'

test('terminal ledger sorts, paginates, filters and exports real disposable data', async ({ page }) => {
  test.setTimeout(120_000)
  const target = new URL(process.env.API_PROXY_TARGET || 'http://invalid')
  if (process.env.E2E_FIXTURE_SCOPE !== 'owned-disposable-pg' || target.hostname !== '127.0.0.1' || ['15174', '15173', '19173', '18080'].includes(target.port)) {
    throw new Error('Write fixtures only through scripts/integration-replica.py owned disposable PostgreSQL runner')
  }
  await page.goto('/pc-assets')
  const token = await page.evaluate(() => localStorage.getItem('cabinet_access_token'))
  expect(token).toBeTruthy()
  async function api(method: 'GET' | 'POST', path: string, data?: object) {
    const response = await page.request.fetch(`${target.origin}/api/v1${path}`, { method, headers: { Authorization: `Bearer ${token}` }, data })
    expect(response.ok(), `${method} ${path} returned ${response.status()}`).toBeTruthy()
    return (await response.json()).data
  }
  const prefix = `TABLE-${Date.now()}`
  const site = await api('POST', '/asset-ops/sites', { region: '隔离表格验收', name: prefix, kind: 'STORE' })
  const types = (await api('GET', '/device-types')).items
  const pcType = types.find((item: { category: string }) => item.category === 'PC')
  expect(pcType).toBeTruthy()
  for (let index = 61; index >= 1; index--) {
    await api('POST', '/asset-ops/terminal-devices', { name: `${prefix}-${index}`, typeId: pcType.id, autoGenerateCode: true, terminal: { status: 'IDLE', siteId: site.id } })
  }
  await page.getByRole('button', { name: '刷新', exact: true }).click()
  const search = page.getByPlaceholder('名称 / 编码 / SN / 使用人 / 部门')
  await search.fill(prefix)
  const pagination = page.locator('[aria-label="终端主台账分页"]')
  await expect(pagination.locator('.el-pagination__total')).toContainText('61')
  const rows = page.locator('.pc-assets .el-table__body-wrapper .el-table__row')
  await expect(rows).toHaveCount(20)
  const header = page.getByRole('button', { name: '按终端 / SN排序', exact: true })
  await header.focus(); await page.keyboard.press('Enter')
  await expect(rows.first().locator('b')).toHaveText(`${prefix}-1`)
  await expect(header.locator('xpath=ancestor::th')).toHaveAttribute('aria-sort', 'ascending')
  await pagination.locator('.btn-next').click()
  await expect(rows.first().locator('b')).toHaveText(`${prefix}-21`)
  await rows.first().locator('.el-checkbox').click()
  await expect(page.getByRole('button', { name: '导出勾选终端', exact: true })).toBeVisible()
  await pagination.locator('.btn-next').click()
  await expect(rows.first().locator('b')).toHaveText(`${prefix}-41`)
  const exportButton = page.getByRole('button', { name: '导出筛选终端', exact: true })
  await expect(exportButton).toBeVisible()
  const downloadEvent = page.waitForEvent('download')
  await exportButton.click()
  const download = await downloadEvent
  const stream = await download.createReadStream()
  if (!stream) throw new Error('No exported workbook stream')
  const chunks: Buffer[] = []
  for await (const chunk of stream) chunks.push(Buffer.from(chunk))
  const workbook = XLSX.read(Buffer.concat(chunks), { type: 'buffer' })
  const exported = XLSX.utils.sheet_to_json<{ 名称: string }>(workbook.Sheets[workbook.SheetNames[0]!]!)
  expect(exported).toHaveLength(61)
  expect(exported[0]!.名称).toBe(`${prefix}-1`)
  expect(exported[60]!.名称).toBe(`${prefix}-61`)
  await header.press('Enter')
  await expect(rows.first().locator('b')).toHaveText(`${prefix}-61`)
  await expect(header.locator('xpath=ancestor::th')).toHaveAttribute('aria-sort', 'descending')
  await search.fill(`${prefix}-missing`)
  await expect(rows).toHaveCount(0)
  await page.getByRole('button', { name: '清除筛选', exact: true }).click()
  await expect(rows).toHaveCount(20)
  await expect(search).toHaveValue('')
})

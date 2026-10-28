import { describe, expect, it } from 'vitest'
import { assetLabelUrl, normalizeScanBaseUrl, scanEntryUrl } from './scanAccess'
import { matchScan, parsedScan } from '@/api/asset-ops'
const id = 'a4345ec9-b768-4e60-a706-0a84e3246d21'
describe('scan entry and label links', () => {
 it('uses the configured origin instead of the printing PC origin', () => {
  expect(assetLabelUrl('https://Assets.example.com/', 'device', id, 'http://192.168.1.1:15173')).toBe(`https://assets.example.com/asset-scan?device=${id}`)
  expect(assetLabelUrl('http://203.0.113.10:15173', 'stock', id)).toBe(`http://203.0.113.10:15173/asset-scan?stock=${id}`)
  expect(scanEntryUrl('https://assets.example.com', 'asset-checks')).toBe('https://assets.example.com/asset-checks')
 })
 it('uses the current origin only when no shared setting is saved', () => {
  expect(assetLabelUrl('', 'device', id, 'http://10.1.1.2:15173')).toBe(`http://10.1.1.2:15173/asset-scan?device=${id}`)
 })
 it.each(['example.com', 'javascript:alert(1)', 'ftp://host', 'https://u:p@host', 'https://host/path', 'https://host/a/..', 'https://host?x=1', 'https://host#x', 'https://host:0', 'https://host:'])('rejects invalid origin %s', value => {
  expect(() => normalizeScanBaseUrl(value)).toThrow()
 })
 it('recognizes old printed links without navigating to the old hostname', () => {
  const old = `http://192.168.1.10:15173/asset-scan?device=${id}`
  expect(parsedScan(old)).toBe(id)
  expect(matchScan([{ targetId: id }], old)).toHaveLength(1)
 })
})

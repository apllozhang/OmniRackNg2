import {describe,it,expect} from 'vitest'
import {parsedScan,matchScan,matchesAssetScan,systemScanTarget} from './asset-ops'

describe('asset labels and inventory matching',()=>{
  const id='a4345ec9-b768-4e60-a706-0a84e3246d21'
  it('reads a stable asset identifier from the generated link',()=>{
    expect(parsedScan(`https://assets.example.internal/asset-scan?device=${id}`)).toBe(id)
    expect(parsedScan(`https://assets.example.internal/asset-scan?stock=${id}`)).toBe(id)
  })
  it('preserves raw manufacturer serial numbers',()=>{
    expect(parsedScan('  ABC-123  ')).toBe('ABC-123')
    expect(parsedScan('https://vendor.example/unknown')).toBe('https://vendor.example/unknown')
  })
  it('matches code or SN case-insensitively',()=>{
    const lines=[{targetId:id,code:'IT-001',serialNumber:'ABCD'}]
    expect(matchScan(lines,'it-001')).toHaveLength(1)
    expect(matchScan(lines,'abcd')).toHaveLength(1)
    expect(matchScan(lines,`https://assets.example/asset-scan?device=${id}`)).toHaveLength(1)
    expect(matchScan(lines,'UNKNOWN')).toHaveLength(0)
  })
  it('does not silently pick one object when SN is duplicated',()=>{
    expect(matchScan([{targetId:id,serialNumber:'DUP'},{targetId:'other',serialNumber:'DUP'}],'dup')).toHaveLength(2)
  })
})

describe('external labels bind to device records', () => {
 const device = { id:'a4345ec9-b768-4e60-a706-0a84e3246d21', code:'DEV-001', assetNumber:'IT-100', externalQrCode:'QR-2026-001', externalQrCodeUrl:'https://vendor.example/QR/001?key=AbC' }
 it('matches external raw code and URL without replacing the system ID', () => {
  expect(matchesAssetScan(device, 'QR-2026-001')).toBe(true)
  expect(matchesAssetScan(device, 'https://vendor.example/QR/001?key=AbC')).toBe(true)
  expect(matchesAssetScan(device, 'https://vendor.example/qr/001?key=abc')).toBe(false)
  expect(matchesAssetScan(device, 'qr-2026-001')).toBe(false)
  expect(matchesAssetScan(device, `https://assets.example/asset-scan?device=${device.id}`)).toBe(true)
  expect(matchesAssetScan(device, 'it-100')).toBe(true)
 })
 it('does not let empty external fields match an empty scan', () => {
  expect(matchesAssetScan({ externalQrCode:'', externalQrCodeUrl:'' }, '')).toBe(false)
 })
 it('matches inventory external bindings from the immutable device snapshot', () => {
  const lines = [{ targetId:device.id, kind:'DEVICE', code:device.code, snapshot:JSON.stringify({ device }) }]
  expect(matchScan(lines, 'QR-2026-001')).toHaveLength(1)
  expect(matchScan(lines, device.externalQrCodeUrl)).toHaveLength(1)
  expect(matchScan([{ ...lines[0], snapshot:'invalid json' }], 'QR-2026-001')).toHaveLength(0)
  expect(matchScan([{ ...lines[0], snapshot:'invalid json' }], device.code)).toHaveLength(1)
 })
 it('keeps collisions visible rather than silently choosing one record', () => {
  const lines = ['a','b'].map(targetId => ({ targetId, kind:'DEVICE', snapshot:JSON.stringify({device}) }))
  expect(matchScan(lines, 'QR-2026-001')).toHaveLength(2)
 })
 it('does not rewrite third party links into a different system address', () => {
  expect(parsedScan(device.externalQrCodeUrl)).toBe(device.externalQrCodeUrl)
 })
})

describe('typed system QR identification',()=>{
 const id='a4345ec9-b768-4e60-a706-0a84e3246d21'
 it('keeps the device/stock type and never falls through to another SN',()=>{
  const url=`https://old.example/asset-scan?stock=${id}`
  expect(systemScanTarget(url)).toEqual({kind:'stock',id})
  expect(matchesAssetScan({id},url,'device')).toBe(false)
  expect(matchesAssetScan({id},url,'stock')).toBe(true)
  expect(matchesAssetScan({id:'other',serialNumber:id,externalQrCodeUrl:url},url,'stock')).toBe(false)
  expect(matchScan([{targetId:id,kind:'DEVICE'},{targetId:id,kind:'SPARE'}],url)).toEqual([{targetId:id,kind:'SPARE'}])
 })
 it('rejects ambiguous, duplicated or malformed system parameters',()=>{
  for(const query of [`device=${id}&stock=${id}`,`device=${id}&device=${id}`,'device=------------------------------------'])
   expect(systemScanTarget(`https://assets.example/asset-scan?${query}`)).toBeNull()
  expect(systemScanTarget(`ftp://assets.example/asset-scan?device=${id}`)).toBeNull()
 })
 it('recognizes uppercase UUIDs from older labels',()=>{
  expect(matchesAssetScan({id},`https://old.example/asset-scan?device=${id.toUpperCase()}`,'device')).toBe(true)
 })
})

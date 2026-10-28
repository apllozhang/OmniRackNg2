import { request } from './http'
export interface ScanAccessSettings { version: number; baseUrl: string; updatedAt?: string }
export const getScanAccessSettings = () => request<ScanAccessSettings>({ url: '/api/v1/asset-ops/access/settings', method: 'GET' })
export const saveScanAccessSettings = (data: ScanAccessSettings) => request<ScanAccessSettings>({ url: '/api/v1/asset-ops/access/settings', method: 'PUT', data })

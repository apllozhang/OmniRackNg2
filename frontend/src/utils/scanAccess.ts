export function normalizeScanBaseUrl(input: string): string {
  const value = input.trim()
  if (!value) return ''
  const invalid = () => new Error('请填写完整的 HTTP/HTTPS IP 或域名，可含端口；不包含账号密码、路径、查询参数或片段')
  if (value.length > 512 || /[\s\\]/.test(value) || !/^https?:\/\/[^/?#]+\/?$/.test(value)) throw invalid()
  let url: URL
  try { url = new URL(value) } catch { throw invalid() }
  if (url.port === '0' || value.replace(/\/$/, '').endsWith(':')) throw invalid()
  if (!url.hostname || url.username || url.password || url.pathname !== '/' || value.includes('?') || value.includes('#')) throw invalid()
  return url.origin
}
export function scanEntryUrl(baseUrl: string, page: 'asset-scan' | 'asset-checks', currentOrigin = location.origin) {
  return `${normalizeScanBaseUrl(baseUrl) || currentOrigin}/${page}`
}
export function assetLabelUrl(baseUrl: string, kind: 'device' | 'stock', id: string, currentOrigin = location.origin) {
  return `${scanEntryUrl(baseUrl, 'asset-scan', currentOrigin)}?${kind}=${encodeURIComponent(id)}`
}

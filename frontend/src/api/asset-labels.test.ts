import { beforeEach, expect, it, vi } from 'vitest'
import { printAssetLabels } from './asset-ops'
const mocks = vi.hoisted(() => ({ get: vi.fn(), qr: vi.fn(), detail:vi.fn() }))
vi.mock('./scan-access', () => ({ getScanAccessSettings: mocks.get }))
vi.mock('./device', () => ({ getDevice:mocks.detail }))
vi.mock('qrcode', () => ({ default: { toDataURL: mocks.qr } }))
const popup={opener:{} as unknown,closed:false,close:vi.fn(),document:{body:{textContent:''},open:vi.fn(),write:vi.fn(),close:vi.fn()}}
beforeEach(() => {
 vi.clearAllMocks();popup.closed=false;popup.opener={}
 mocks.get.mockResolvedValue({ version: 1, baseUrl: 'https://public.example.test' });mocks.qr.mockResolvedValue('data:image/png;base64,fixture')
 mocks.detail.mockResolvedValue({id:'id-1',code:'LATEST-001',name:'最新名称'})
 vi.spyOn(window, 'open').mockReturnValue(popup as unknown as Window)
})
it.each(['device', 'stock'] as const)('reserves a window before requests and encodes the configured %s address', async kind => {
 const pending=printAssetLabels(kind,[{id:'id-1',code:'DEMO-001',name:'设备'}])
 expect(window.open).toHaveBeenCalledWith('','_blank')
 expect(vi.mocked(window.open).mock.invocationCallOrder[0]).toBeLessThan(mocks.get.mock.invocationCallOrder[0])
 await pending
 expect(mocks.qr).toHaveBeenCalledWith(`https://public.example.test/asset-scan?${kind}=id-1`,{width:360,margin:4,errorCorrectionLevel:'M'})
 expect(popup.opener).toBeNull()
})
it('reports blocked windows without issuing requests',async()=>{
 vi.mocked(window.open).mockReturnValue(null)
 await expect(printAssetLabels('device',[{id:'a'}])).rejects.toThrow('浏览器拦截')
 expect(mocks.get).not.toHaveBeenCalled()
})
it('closes the reserved window on configuration failure',async()=>{
 mocks.get.mockRejectedValue(new Error('配置不可用'))
 await expect(printAssetLabels('device',[{id:'a'}])).rejects.toThrow('配置不可用')
 expect(popup.close).toHaveBeenCalled();expect(mocks.qr).not.toHaveBeenCalled()
})
it('reloads saved fields and escapes label content',async()=>{
 mocks.detail.mockResolvedValue({id:'id-1',code:'LATEST-001',name:'<script>bad()</script>',serialNumber:'SN-123'})
 await printAssetLabels('device',[{id:'id-1',name:'未保存名称'}],{reloadDevices:true})
 const html=popup.document.write.mock.calls[0][0]
 expect(html).toContain('LATEST-001');expect(html).toContain('SN-123');expect(html).toContain('&lt;script&gt;')
 expect(html).not.toContain('未保存名称');expect(html).not.toContain('<script>')
})
it('does not write into a window the user already closed',async()=>{
 mocks.qr.mockImplementation(async()=>{popup.closed=true;return 'data:image/png;base64,fixture'})
 await expect(printAssetLabels('device',[{id:'a'}])).rejects.toThrow('窗口已关闭')
 expect(popup.document.write).not.toHaveBeenCalled()
})

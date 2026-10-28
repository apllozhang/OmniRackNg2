import ElementPlus from 'element-plus'
import { mount,flushPromises } from '@vue/test-utils'
import { beforeEach,expect,it,vi } from 'vitest'
import AssetScanner from './AssetScanner.vue'
const mocks=vi.hoisted(()=>({media:vi.fn(),decode:vi.fn(),stop:vi.fn(),track:vi.fn()}))
vi.mock('@zxing/browser',()=>({BrowserMultiFormatReader:class{decodeFromStream=mocks.decode}}))
const stream={getTracks:()=>[{stop:mocks.track}]} as unknown as MediaStream
beforeEach(()=>{
 vi.clearAllMocks();mocks.media.mockResolvedValue(stream);mocks.decode.mockResolvedValue({stop:mocks.stop})
 Object.defineProperty(navigator,'mediaDevices',{configurable:true,value:{getUserMedia:mocks.media}})
 Object.defineProperty(document,'hidden',{configurable:true,value:false})
})
function render(){return mount(AssetScanner,{global:{plugins:[ElementPlus]}})}
it('stops a late permission result after the user closes the scanner',async()=>{
 let release!:(stream:MediaStream)=>void
 mocks.media.mockImplementation(()=>new Promise(resolve=>{release=resolve}))
 const wrapper=render();await wrapper.find('button').trigger('click');await wrapper.find('button').trigger('click')
 release(stream);await flushPromises()
 expect(mocks.track).toHaveBeenCalled();expect(mocks.decode).not.toHaveBeenCalled();wrapper.unmount()
})
it('emits only one result and releases the camera before a second decode',async()=>{
 let callback!:(value:{getText:()=>string})=>void
 mocks.decode.mockImplementation(async(_stream,_video,cb)=>{callback=cb;return {stop:mocks.stop}})
 const wrapper=render();await wrapper.find('button').trigger('click');await flushPromises()
 callback({getText:()=> 'QR-001'});callback({getText:()=> 'QR-001'})
 expect(wrapper.emitted('result')).toEqual([['QR-001']]);expect(mocks.track).toHaveBeenCalled();wrapper.unmount()
})
it('releases tracks on decoder initialization failure',async()=>{
 mocks.decode.mockRejectedValue(new Error('解码器启动失败'))
 const wrapper=render();await wrapper.find('button').trigger('click');await flushPromises()
 expect(mocks.track).toHaveBeenCalled();expect(wrapper.text()).toContain('解码器启动失败');wrapper.unmount()
})
it.each(['NotAllowedError','NotFoundError','NotReadableError'])('explains camera error %s in Chinese',async name=>{
 mocks.media.mockRejectedValue(Object.assign(new Error('browser raw error'),{name}))
 const wrapper=render();await wrapper.find('button').trigger('click');await flushPromises()
 expect(wrapper.find('[role="alert"]').text()).not.toContain('browser raw error')
 expect(wrapper.find('button').text()).toBe('打开摄像头扫码');wrapper.unmount()
})
it('stops on page backgrounding and when disabled by a confirmation dialog',async()=>{
 const wrapper=render();await wrapper.find('button').trigger('click');await flushPromises()
 Object.defineProperty(document,'hidden',{configurable:true,value:true});document.dispatchEvent(new Event('visibilitychange'))
 expect(mocks.stop).toHaveBeenCalled();expect(mocks.track).toHaveBeenCalled()
 Object.defineProperty(document,'hidden',{configurable:true,value:false})
 await wrapper.find('button').trigger('click');await flushPromises();await wrapper.setProps({disabled:true})
 expect(wrapper.find('button').attributes('disabled')).toBeDefined();expect(wrapper.find('button').text()).toBe('打开摄像头扫码');wrapper.unmount()
})

it('waits for old decoder cleanup before allowing a fresh start',async()=>{
 let ready!:(value:{stop:()=>void})=>void
 mocks.decode.mockImplementation(()=>new Promise(resolve=>{ready=resolve}))
 const wrapper=render();await wrapper.find('button').trigger('click');await flushPromises()
 await wrapper.find('button').trigger('click')
 expect(wrapper.find('button').attributes('disabled')).toBeDefined()
 expect(mocks.media).toHaveBeenCalledTimes(1)
 ready({stop:mocks.stop});await flushPromises()
 expect(mocks.stop).toHaveBeenCalled();expect(wrapper.find('button').attributes('disabled')).toBeUndefined();wrapper.unmount()
})

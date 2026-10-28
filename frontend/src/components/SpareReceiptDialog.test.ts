import ElementPlus,{ElSelect} from 'element-plus'
import {mount,flushPromises} from '@vue/test-utils'
import {afterEach,expect,it,vi} from 'vitest'
import SpareReceiptDialog from './SpareReceiptDialog.vue'
const api=vi.hoisted(()=>({ops:vi.fn()}))
vi.mock('@/api/asset-ops',()=>({ops:api.ops,errorMessage:(e:Error)=>e.message}))
const wrappers:ReturnType<typeof mount>[]=[]
const spec={id:'s1',category:'内存',name:'服务器内存',modelNumber:'32GB',tracking:'QUANTITY'}
async function render(){const w=mount(SpareReceiptDialog,{attachTo:document.body,props:{modelValue:true,specs:[spec],sites:[{id:'site1',region:'上海',name:'备件库'}]},global:{plugins:[ElementPlus]}});wrappers.push(w);await flushPromises();return w}
afterEach(()=>{wrappers.splice(0).forEach(w=>w.unmount());document.body.innerHTML='';vi.clearAllMocks()})
it('registers first-time spare data and stock with one request and retains the retry key',async()=>{
 api.ops.mockRejectedValueOnce(new Error('暂时不可用')).mockResolvedValue({})
 const w=await render();expect(w.findAll('.el-form-item').map(e=>e.text()).join(' ')).toContain('类型')
 const selects=w.findAllComponents({name:'ElSelect'});selects[0]!.vm.$emit('update:modelValue','site1')
 for(const [label,value] of [['类型','内存'],['名称','服务器内存'],['型号','DDR5-32G']] as const){const item=w.findAll('.el-form-item').find(e=>e.text().startsWith(label))!;await item.find('input').setValue(value)}
 const save=w.findAll('button').find(b=>b.text()==='保存备件')!
 await save.trigger('click');await flushPromises();await save.trigger('click');await flushPromises()
 expect(api.ops).toHaveBeenCalledTimes(2)
 expect(api.ops.mock.calls[0]![2]).toEqual(api.ops.mock.calls[1]![2])
 expect(api.ops).toHaveBeenLastCalledWith('stock-receipts','POST',expect.objectContaining({specId:null,siteId:'site1',category:'内存',name:'服务器内存',modelNumber:'DDR5-32G',quantity:1,state:'AVAILABLE',submissionId:expect.any(String)}))
 expect(w.emitted('saved')).toHaveLength(1)
})
it('blocks missing location rather than creating an empty stock draft',async()=>{
 const w=await render();await w.findAll('button').find(b=>b.text()==='保存备件')!.trigger('click');await flushPromises()
 expect(api.ops).not.toHaveBeenCalled();expect(w.emitted('saved')).toBeUndefined()
})

it('records an actual initial state without first choosing a specification',async()=>{
 api.ops.mockResolvedValue({})
 const w=await render()
 w.findAllComponents(ElSelect).find(e=>e.props('placeholder')==='选择所在区域及具体位置')!.vm.$emit('update:modelValue','site1')
 for(const [label,value] of [['类型','电源'],['名称','备用电源']] as const){await w.findAll('.el-form-item').find(e=>e.text().startsWith(label))!.find('input').setValue(value)}
 const state=w.findAll('.el-form-item').find(e=>e.text().startsWith('状态'))!
 state.findComponent(ElSelect).vm.$emit('update:modelValue','FAULTY');await flushPromises()
 await w.findAll('button').find(b=>b.text()==='保存备件')!.trigger('click');await flushPromises()
 expect(api.ops).toHaveBeenCalledWith('stock-receipts','POST',expect.objectContaining({specId:null,state:'FAULTY',name:'备用电源'}))
})

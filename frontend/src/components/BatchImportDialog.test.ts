import ElementPlus from 'element-plus'
import {mount,flushPromises} from '@vue/test-utils'
import {describe,it,expect,vi} from 'vitest'
import BatchImportDialog from './BatchImportDialog.vue'
import TablePager from './TablePager.vue'
const mocks=vi.hoisted(()=>({parseExcelFile:vi.fn(),downloadImportResult:vi.fn()}))
vi.mock('@/utils/excelImport',()=>({...mocks,downloadExcelTemplate:vi.fn()}))
describe('complete batch import with paginated preview',()=>{
 it('imports every row beyond the former 200 limit and retries only failed rows despite search',async()=>{
  const rows=Array.from({length:241},(_,i)=>({rowNumber:i+2,summary:`设备${i+1}`,operation:'CREATE',errors:[],payload:{id:i+1}}))
  mocks.parseExcelFile.mockResolvedValue([])
  let fail=true
  const submit=vi.fn(async(p:any)=>{if(p.id===241&&fail)throw new Error('retry')})
  const w=mount(BatchImportDialog,{props:{modelValue:true,title:'导入',description:'校验',templateFileName:'a',resultFileName:'b',subject:'设备',fields:[],prepareRows:()=>rows as any,submitRow:submit},global:{plugins:[ElementPlus]}})
  await (w.vm as any).handleFileChange({raw:new File(['x'],'data.xlsx')});await flushPromises()
  expect(w.findComponent(TablePager).props('total')).toBe(241)
  w.findComponent(TablePager).vm.$emit('update:page',13);await flushPromises()
  expect(w.findAll('.el-table__body tbody tr')).toHaveLength(1)
  await (w.vm as any).startImport();await flushPromises()
  expect(submit).toHaveBeenCalledTimes(241)
  ;(w.vm as any).search='不存在';await new Promise(r=>setTimeout(r,350));await flushPromises()
  expect(w.findComponent(TablePager).props('total')).toBe(0)
  fail=false;await (w.vm as any).startImport();await flushPromises()
  expect(submit).toHaveBeenCalledTimes(242)
  expect(submit.mock.calls[241]?.[0]).toEqual({id:241})
  ;(w.vm as any).downloadResult()
  expect(mocks.downloadImportResult.mock.calls[mocks.downloadImportResult.mock.calls.length-1]?.[1]).toHaveLength(241)
  expect(w.emitted('imported')).toEqual([[240],[1]])
  w.unmount()
 })
})

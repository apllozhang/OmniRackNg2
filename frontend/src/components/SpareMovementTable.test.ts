import ElementPlus from 'element-plus'
import {mount,flushPromises} from '@vue/test-utils'
import {expect,it,vi} from 'vitest'
import SpareMovementTable from './SpareMovementTable.vue'
vi.mock('@/api/asset-ops',()=>({moveLabels:{TRANSFER:'调拨发出',IN:'入库'},stockStates:{AVAILABLE:'可用',TRANSIT:'在途',FAULTY:'故障'}}))
it('shows destination batch creation separately from source quantity reduction',async()=>{
 const row={operation:'TRANSFER',stockId:'source',destinationStockId:'target',quantity:3,beforeQuantity:10,afterQuantity:7,beforeState:'AVAILABLE',afterState:'AVAILABLE',sourceSiteId:'a',destinationSiteId:'b',stock:{spec:{name:'硬盘'}},createdAt:'2026-10-21T00:00:00Z'}
 const sites=[{id:'a',region:'上海',name:'A库'},{id:'b',region:'杭州',name:'B库'}]
 const w=mount(SpareMovementTable,{props:{rows:[row],sites,stockId:'target'},global:{plugins:[ElementPlus]}})
 await flushPromises()
 expect(w.text()).toContain('0 → 3');expect(w.text()).toContain('新增调拨批次 → 在途');expect(w.text()).toContain('上海 / A库');expect(w.text()).toContain('杭州 / B库')
 w.unmount()
})
it('does not fabricate state history for old movement rows',async()=>{
 const w=mount(SpareMovementTable,{props:{rows:[{operation:'IN',beforeQuantity:0,afterQuantity:1}],sites:[]},global:{plugins:[ElementPlus]}})
 await flushPromises()
 expect(w.text()).toContain('未记录 → 未记录');w.unmount()
})

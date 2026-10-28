import ElementPlus, {ElPagination} from 'element-plus'
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

it('sorts all movement quantities and timestamps before pagination without changing the source order',async()=>{
 const rows=Array.from({length:61},(_,index)=>({id:`m-${index}`,operation:'IN',beforeQuantity:0,afterQuantity:61-index,createdAt:new Date(Date.UTC(2026,0,61-index)).toISOString()}))
 const before=rows.map(row=>row.id)
 const w=mount(SpareMovementTable,{props:{rows,sites:[]},global:{plugins:[ElementPlus]}})
 await flushPromises()
 const rendered=()=>w.findAll('.el-table__body tbody tr')
 expect(rendered()).toHaveLength(20)
 await w.get('button[aria-label="按数量变化排序"]').trigger('click');await flushPromises()
 expect(rendered()[0]!.text()).toContain('0 → 1')
 const pager=w.findComponent(ElPagination)
 pager.vm.$emit('update:current-page',2);await flushPromises()
 expect(rendered()[0]!.text()).toContain('0 → 21')
 await w.get('button[aria-label="按时间排序"]').trigger('click');await flushPromises()
 expect(pager.props('currentPage')).toBe(1)
 expect(rendered()[0]!.text()).toContain('0 → 1')
 await w.get('button[aria-label="按时间排序"]').trigger('click');await flushPromises()
 expect(rendered()[0]!.text()).toContain('0 → 61')
 expect(rows.map(row=>row.id)).toEqual(before)
 await w.setProps({rows:rows.slice(0,1)})
 expect(pager.props('total')).toBe(1)
 w.unmount()
})

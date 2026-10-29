import ElementPlus from 'element-plus'
import {mount,flushPromises} from '@vue/test-utils'
import {expect,it,vi} from 'vitest'
import RackManagementView from './RackManagementView.vue'
const mocks=vi.hoisted(()=>({getResourceTree:vi.fn(),push:vi.fn()}))
vi.mock('@/api/resource',()=>({...mocks,createRack:vi.fn(),deleteRack:vi.fn(),updateRack:vi.fn()}))
vi.mock('@/api/rack-template',()=>({getRackTemplates:async()=>({items:[]})}))
vi.mock('vue-router',()=>({useRouter:()=>({push:mocks.push})}))
it('sorts the entire rack collection, keeps full summary counts, and resets filtered pages',async()=>{
 const racks=Array.from({length:61},(_,i)=>({id:`r${i+1}`,version:3,name:`机柜${i+1}`,code:`R${i+1}`,status:'AVAILABLE',uHeight:42,pduCount:0,roomId:'room',dataCenterId:'dc',sortOrder:0})).reverse()
 mocks.getResourceTree.mockResolvedValue({items:[{id:'dc',name:'中心',status:'OPERATING',rooms:[{id:'room',name:'机房',status:'OPERATING',racks}]}]})
 const w=mount(RackManagementView,{global:{plugins:[ElementPlus],stubs:{BatchImportDialog:true}}});await flushPromises()
 const vm=w.vm as any
 expect(vm.rackRows).toHaveLength(61)
 await w.get('button[aria-label="按机柜排序"]').trigger('click');await flushPromises()
 expect(vm.rackPageRows[0].id).toBe('r1')
 vm.page=4;await flushPromises();expect(vm.rackPageRows).toHaveLength(1);expect(vm.rackPageRows[0].id).toBe('r61')
 vm.appliedKeyword='机柜2';await flushPromises();expect(vm.page).toBe(1);expect(vm.filteredRows).toHaveLength(11)
 expect(vm.rackRows).toHaveLength(61)
 w.unmount()
})

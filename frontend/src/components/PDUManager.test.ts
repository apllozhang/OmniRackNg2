import {mount,flushPromises} from '@vue/test-utils'
import {expect,it,vi} from 'vitest'
import ElementPlus from 'element-plus'
import PDUManager from './PDUManager.vue'
const api=vi.hoisted(()=>({listPDUs:vi.fn(),listPDUSockets:vi.fn(),listPDUConnections:vi.fn()}))
vi.mock('@/api/pdu',()=>({...api,createPDU:vi.fn(),updatePDU:vi.fn(),deletePDU:vi.fn(),createPDUSocket:vi.fn(),updatePDUSocket:vi.fn(),deletePDUSocket:vi.fn(),connectPDUSocket:vi.fn(),disconnectPDUConnection:vi.fn()}))
it('discards delayed rack responses and closes mutations when changing rack',async()=>{
 let release!:(value:any)=>void
 api.listPDUs.mockImplementation((rack:string)=>rack==='old'?new Promise(resolve=>{release=resolve}):Promise.resolve([{id:'new-pdu',name:'新柜PDU',code:'N',status:'ACTIVE'}]))
 api.listPDUSockets.mockResolvedValue([]);api.listPDUConnections.mockResolvedValue([])
 const w=mount(PDUManager,{props:{rackId:'old'},global:{plugins:[ElementPlus]}});await flushPromises()
 const vm=w.vm as any
 vm.pduDialog=true;vm.socketDialog=true;vm.connectDialog=true;vm.selectedSocket={id:'old-socket'}
 await w.setProps({rackId:'new'});await flushPromises()
 expect(vm.pdus.map((p:any)=>p.id)).toEqual(['new-pdu'])
 expect(vm.pduDialog||vm.socketDialog||vm.connectDialog).toBe(false)
 expect(vm.selectedSocket).toBeNull()
 release([{id:'old-pdu'}]);await flushPromises()
 expect(vm.pdus.map((p:any)=>p.id)).toEqual(['new-pdu'])
 expect(vm.loading).toBe(false)
 w.unmount()
})

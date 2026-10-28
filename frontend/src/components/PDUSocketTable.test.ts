import ElementPlus from 'element-plus'
import {mount,flushPromises} from '@vue/test-utils'
import {describe,it,expect} from 'vitest'
import PDUSocketTable from './PDUSocketTable.vue'
import TablePager from './TablePager.vue'
import type {PDUSocket} from '@/types/pdu'
const sockets=Array.from({length:61},(_,i)=>({id:`s${i+1}`,version:7,pduId:'p1',createdAt:'',updatedAt:'',socketNo:i+1,standard:'CN',amperageA:10,status:'AVAILABLE'} as PDUSocket)).reverse()
describe('PDU socket complete collection controls',()=>{
 it('sorts before paging and passes the actual socket identity to actions',async()=>{
  const w=mount(PDUSocketTable,{props:{sockets,connectionFor:()=>undefined,groupLabel:'PDU A'},global:{plugins:[ElementPlus]},slots:{actions:'<template #default="{socket}"><button :data-socket="socket.id">{{socket.version}}</button></template>'}})
  await flushPromises()
  await w.get('button[aria-label="按插座排序"]').trigger('click');await flushPromises()
  expect(w.findAll('.el-table__body tbody tr')).toHaveLength(20)
  expect(w.find('[data-socket="s1"]').exists()).toBe(true)
  w.findComponent(TablePager).vm.$emit('update:page',4);await flushPromises()
  expect(w.findAll('.el-table__body tbody tr')).toHaveLength(1)
  expect(w.get('[data-socket="s61"]').text()).toBe('7')
  await w.get('input').setValue('61');await new Promise(r=>setTimeout(r,350));await flushPromises()
  expect(w.findComponent(TablePager).props('page')).toBe(1)
  expect(w.findAll('.el-table__body tbody tr')).toHaveLength(1)
  expect(sockets[0]?.socketNo).toBe(61)
  w.unmount()
 })
 it('keeps separate PDU group paging independent',async()=>{
  const options={props:{sockets,connectionFor:()=>undefined,groupLabel:'PDU'},global:{plugins:[ElementPlus]}}
  const a=mount(PDUSocketTable,options),b=mount(PDUSocketTable,options);await flushPromises()
  a.findComponent(TablePager).vm.$emit('update:page',3);await flushPromises()
  expect(a.findComponent(TablePager).props('page')).toBe(3)
  expect(b.findComponent(TablePager).props('page')).toBe(1)
  a.unmount();b.unmount()
 })
})

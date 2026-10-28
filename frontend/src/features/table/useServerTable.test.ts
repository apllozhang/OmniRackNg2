import { effectScope, nextTick } from 'vue'
import { afterEach, expect, it, vi } from 'vitest'
import { flushPromises } from '@vue/test-utils'
import { useServerTable, type ServerTableQuery } from './useServerTable'
afterEach(() => vi.useRealTimers())
it('ignores a slow old page after sorting and resets pagination on debounced filtering and page size', async () => {
  const pending: {query:ServerTableQuery;resolve:(value:{items:{id:string}[];total:number})=>void}[]=[]
  const fetch=vi.fn((query:ServerTableQuery)=>new Promise<{items:{id:string}[];total:number}>(resolve=>pending.push({query,resolve})))
  const scope=effectScope(),onError=vi.fn()
  const table=scope.run(()=>useServerTable(fetch,20,onError))!
  pending[0]!.resolve({items:[{id:'initial'}],total:121});await flushPromises()
  table.page.value=3;await nextTick()
  expect(pending[1]!.query.page).toBe(3)
  table.sortBy('createdAt');await nextTick()
  expect(pending[2]!.query).toEqual({page:1,pageSize:20,sortBy:'createdAt',sortOrder:'asc'})
  pending[2]!.resolve({items:[{id:'sorted'}],total:121});await flushPromises()
  pending[1]!.resolve({items:[{id:'stale'}],total:121});await flushPromises()
  expect(table.rows.value).toEqual([{id:'sorted'}])
  table.page.value=4;await nextTick()
  vi.useFakeTimers();table.search.value='filter';await nextTick()
  expect(table.settled.value).toBe(false)
  await vi.advanceTimersByTimeAsync(299)
  expect(pending[pending.length-1]!.query.page).toBe(4)
  await vi.advanceTimersByTimeAsync(1);await nextTick()
  expect(pending[pending.length-1]!.query).toMatchObject({page:1,search:'filter'})
  table.pageSize.value=50;await nextTick()
  expect(pending[pending.length-1]!.query).toMatchObject({page:1,pageSize:50,search:'filter'})
  scope.stop()
  pending[pending.length-1]!.resolve({items:[{id:'after-unmount'}],total:1});await flushPromises()
  expect(table.rows.value).toEqual([{id:'sorted'}]);expect(onError).not.toHaveBeenCalled()
})
it('keeps failed queries unsettled and recovers after retry without claiming old rows are current', async () => {
  const fetch=vi.fn().mockResolvedValueOnce({items:[{id:'old'}],total:21}).mockRejectedValueOnce(new Error('offline')).mockResolvedValue({items:[{id:'fresh'}],total:21})
  const scope=effectScope(),onError=vi.fn();const table=scope.run(()=>useServerTable<{id:string}>(fetch,20,onError))!
  await flushPromises();table.page.value=2;await flushPromises()
  expect(table.failed.value).toBe(true);expect(table.settled.value).toBe(false);expect(table.rows.value[0]!.id).toBe('old')
  await table.reload();expect(table.settled.value).toBe(true);expect(table.rows.value[0]!.id).toBe('fresh')
  expect(onError).toHaveBeenCalledOnce();scope.stop()
})

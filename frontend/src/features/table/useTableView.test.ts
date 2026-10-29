import { effectScope, ref } from 'vue'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { useDebouncedValue, useTableView } from './useTableView'
afterEach(() => vi.useRealTimers())
describe('complete filtered client table', () => {
  it('sorts the complete result before slicing, preserves ties and leaves input unchanged', () => {
    const scope = effectScope()
    scope.run(() => {
      const input = ref([{ id: 1, asset: { code: 'PC-10' } }, { id: 2, asset: { code: 'PC-2' } }, { id: 3, asset: { code: 'PC-2' } }, { id: 4, asset: {} }])
      const view = useTableView(input, 2)
      view.setSort('asset.code', 'ascending')
      expect(view.pageRows.value.map(r => r.id)).toEqual([2, 3])
      view.page.value = 2; expect(view.pageRows.value.map(r => r.id)).toEqual([1, 4])
      view.setSort('asset.code', 'descending'); expect(view.pageRows.value.map(r => r.id)).toEqual([1, 2])
      expect(view.sortedRows.value[view.sortedRows.value.length - 1]?.id).toBe(4)
      expect(input.value.map(r => r.id)).toEqual([1, 2, 3, 4])
      view.setSort('asset.code', null); expect(view.pageRows.value.map(r => r.id)).toEqual([1, 2])
    }); scope.stop()
  })
  it('resets pages on filtering and page size changes and clamps invalid page requests', () => {
    const scope = effectScope(); scope.run(() => {
      const input = ref(Array.from({ length: 61 }, (_, id) => ({ id })))
      const view = useTableView(input, 20); view.page.value = 4
      expect(view.pageRows.value).toEqual([{ id: 60 }])
      input.value = input.value.filter(r => r.id === 45)
      expect(view.page.value).toBe(1); expect(view.pageRows.value).toEqual([{ id: 45 }])
      input.value = []; expect(view.total.value).toBe(0); expect(view.pageRows.value).toEqual([])
      view.page.value = 99; expect(view.page.value).toBe(1)
      input.value = Array.from({ length: 61 }, (_, id) => ({ id })); view.page.value = 3; view.pageSize.value = 50
      expect(view.page.value).toBe(1); expect(view.pageRows.value.length).toBe(50)
    }); scope.stop()
  })
  it('uses the last query after 300ms and cancels pending work on disposal', async () => {
    vi.useFakeTimers(); const scope = effectScope(); const query = ref(''); let result!: { value: string }
    scope.run(() => { result = useDebouncedValue(query) })
    query.value = 'PC'; await Promise.resolve(); await vi.advanceTimersByTimeAsync(200)
    query.value = 'PC-2'; await Promise.resolve(); await vi.advanceTimersByTimeAsync(299)
    expect(result.value).toBe(''); await vi.advanceTimersByTimeAsync(1); expect(result.value).toBe('PC-2')
    query.value = 'pending'; await Promise.resolve(); scope.stop(); await vi.runAllTimersAsync(); expect(result.value).toBe('PC-2')
  })
})

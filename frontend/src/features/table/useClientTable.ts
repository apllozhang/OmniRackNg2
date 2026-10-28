import { ref, type MaybeRefOrGetter } from 'vue'
import type { TableInstance } from 'element-plus'
import { useTableView, type TableOrder } from './useTableView'

/** Complete client collections only. Server-paged collections need API sorting. */
export function useClientTable<T>(source: MaybeRefOrGetter<readonly T[]>, accessors: Record<string, (row: T) => unknown> = {}) {
  const view = useTableView(source, 20, accessors)
  const tableRef = ref<TableInstance>()
  function onSortChange(event: { prop: string; order: TableOrder }) {
    view.setSort(event.prop || '', event.order)
  }
  function sortBy(key: string) {
    const order = view.nextSort(key)
    view.setSort(key, order)
    if (order) tableRef.value?.sort(key, order)
    else tableRef.value?.clearSort()
  }
  return { ...view, tableRef, onSortChange, sortBy }
}

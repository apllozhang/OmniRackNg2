import { computed, onScopeDispose, ref, shallowRef, toValue, watch, type MaybeRefOrGetter } from 'vue'
export type TableOrder = 'ascending' | 'descending' | null
const collator = new Intl.Collator('zh-CN', { numeric: true, sensitivity: 'base' })
export function fieldValue(row: unknown, path: string): unknown {
  return path.split('.').reduce<unknown>((value, key) => value != null && typeof value === 'object' ? (value as Record<string, unknown>)[key] : undefined, row)
}
export function useDebouncedValue<T>(source: MaybeRefOrGetter<T>, delay = 300) {
  const result = shallowRef(toValue(source))
  let timer: ReturnType<typeof setTimeout> | undefined
  watch(() => toValue(source), value => { clearTimeout(timer); timer = setTimeout(() => { result.value = value }, delay) })
  onScopeDispose(() => clearTimeout(timer))
  return result
}
/** Client-side table view. The source must contain the complete filtered result;
 * never apply this to one server page and describe it as global sorting. */
export function useTableView<T>(source: MaybeRefOrGetter<readonly T[]>, initialPageSize = 20, accessors: Record<string, (row: T) => unknown> = {}) {
  const page = ref(1), pageSize = ref(initialPageSize), sortKey = ref(''), sortOrder = ref<TableOrder>(null)
  const sortedRows = computed(() => {
    const rows = [...toValue(source)]
    if (!sortKey.value || !sortOrder.value) return rows
    const direction = sortOrder.value === 'ascending' ? 1 : -1
    return rows.map((row, index) => ({ row, index })).sort((a, b) => {
      const get = accessors[sortKey.value] || ((row: T) => fieldValue(row, sortKey.value))
      const x = get(a.row), y = get(b.row)
      const emptyX = x == null || x === '', emptyY = y == null || y === ''
      if (emptyX !== emptyY) return emptyX ? 1 : -1
      if (emptyX) return a.index - b.index
      const result = typeof x === 'number' && typeof y === 'number' ? x - y : collator.compare(String(x), String(y))
      return result * direction || a.index - b.index
    }).map(item => item.row)
  })
  const total = computed(() => sortedRows.value.length)
  const pageCount = computed(() => Math.max(1, Math.ceil(total.value / pageSize.value)))
  const pageRows = computed(() => sortedRows.value.slice((page.value - 1) * pageSize.value, page.value * pageSize.value))
  watch(() => toValue(source), () => { page.value = 1 }, { flush: 'sync' })
  watch(pageSize, () => { page.value = 1 }, { flush: 'sync' })
  watch([page, pageCount], () => { page.value = Math.min(Math.max(1, page.value), pageCount.value) }, { flush: 'sync' })
  function setSort(key: string, order: TableOrder) { sortKey.value = key; sortOrder.value = order; page.value = 1 }
  function nextSort(key: string): TableOrder { return sortKey.value !== key || !sortOrder.value ? 'ascending' : sortOrder.value === 'ascending' ? 'descending' : null }
  return { page, pageSize, sortKey, sortOrder, sortedRows, total, pageCount, pageRows, setSort, nextSort }
}

import { computed, onScopeDispose, ref, watch } from 'vue'
import { useDebouncedValue, type TableOrder } from './useTableView'
import type { TableInstance } from 'element-plus'
export interface ServerTableQuery { page: number; pageSize: number; search?: string; sortBy?: string; sortOrder?: 'asc' | 'desc' }
export function useServerTable<T>(fetchPage: (query: ServerTableQuery) => Promise<{items: T[]; total: number}>, initialSize: number, onError: (error: unknown) => void) {
  const rows = ref<T[]>([]), total = ref(0), page = ref(1), pageSize = ref(initialSize)
  const loading = ref(false), failed = ref(false), search = ref(''), sortKey = ref(''), sortOrder = ref<TableOrder>(null)
  const query = useDebouncedValue(search), tableRef = ref<TableInstance>()
  let request = 0
  onScopeDispose(() => { request++ })
  watch([query, pageSize, sortKey, sortOrder], () => { page.value = 1 }, {flush: 'sync'})
  async function reload() {
    const current = ++request
    loading.value = true; failed.value = false
    const params: ServerTableQuery = {page: page.value, pageSize: pageSize.value}
    if (query.value.trim()) params.search = query.value.trim()
    if (sortKey.value && sortOrder.value) { params.sortBy = sortKey.value; params.sortOrder = sortOrder.value === 'ascending' ? 'asc' : 'desc' }
    try {
      const result = await fetchPage(params)
      if (current !== request) return
      total.value = result.total
      const lastPage = Math.max(1, Math.ceil(result.total / pageSize.value))
      if (page.value > lastPage) { page.value = lastPage; return }
      rows.value = result.items as typeof rows.value
    } catch (error) { if (current === request) { failed.value = true; onError(error) } }
    finally { if (current === request) loading.value = false }
  }
  watch([query, page, pageSize, sortKey, sortOrder], reload, {immediate: true})
  function onSortChange(event: {prop: string; order: TableOrder}) { sortKey.value = event.prop || ''; sortOrder.value = event.order }
  function sortBy(key: string) {
    const order = sortKey.value !== key || !sortOrder.value ? 'ascending' : sortOrder.value === 'ascending' ? 'descending' : null
    onSortChange({prop: key, order})
    if (order) tableRef.value?.sort(key, order)
    else tableRef.value?.clearSort()
  }
  function clearSearch() { search.value = ''; query.value = '' }
  const settled = computed(() => !loading.value && !failed.value && search.value === query.value)
  return {rows, total, page, pageSize, loading, failed, search, query, tableRef, sortBy, onSortChange, reload, clearSearch, settled}
}

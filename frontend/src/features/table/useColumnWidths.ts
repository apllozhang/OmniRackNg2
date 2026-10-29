import { reactive } from 'vue'
/** Keep width changes in the column props so Element Plus performs its normal layout. */
export function useColumnWidths(initial: Record<string, number>) {
  const columnWidths = reactive({ ...initial })
  function resizeColumn(key: string, delta: number) {
    columnWidths[key] = Math.max(80, Math.min(800, (columnWidths[key] || 150) + delta))
  }
  return { columnWidths, resizeColumn }
}

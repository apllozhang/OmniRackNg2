import { effectScope } from 'vue'
import { expect, it } from 'vitest'
import { useColumnWidths } from './useColumnWidths'
it('keeps column widths bounded without changing other columns or the initial configuration', () => {
  const initial = { name: 205, location: 200 }
  const scope = effectScope()
  const view = scope.run(() => useColumnWidths(initial))!
  view.resizeColumn('name', 10); view.resizeColumn('name', -1)
  expect(view.columnWidths.name).toBe(214)
  expect(view.columnWidths.location).toBe(200)
  expect(initial.name).toBe(205)
  view.resizeColumn('name', -10000); expect(view.columnWidths.name).toBe(80)
  view.resizeColumn('name', 10000); expect(view.columnWidths.name).toBe(800)
  scope.stop()
})

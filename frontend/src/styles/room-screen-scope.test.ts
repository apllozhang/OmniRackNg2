import { describe, expect, it } from 'vitest'
import roomScreenSource from '../views/RoomScreenView.vue?raw'

const styleBlock = roomScreenSource.match(/<style>([\s\S]*?)<\/style>/)?.[1] ?? ''

describe('RoomScreenView style isolation', () => {
  it('does not leak the rack card layout into resource pages', () => {
    expect(styleBlock).toContain('.screen-page .rack-card {')
    expect(styleBlock).not.toMatch(/(?:^|[,{}])\s*\.rack-card(?:[.:\s,{])/)
  })

  it('does not leak the dark screen filter bar into rack management', () => {
    expect(styleBlock).toContain('.screen-page .filter-bar {')
    expect(styleBlock).not.toMatch(/(?:^|[,{}])\s*\.filter-bar(?:[.:\s,{])/)
  })
})

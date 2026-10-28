import { vi } from 'vitest'

class ResizeObserverMock implements ResizeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
}

class PointerEventMock extends MouseEvent {
  readonly pointerId: number

  constructor(type: string, init: PointerEventInit = {}) {
    super(type, init)
    this.pointerId = init.pointerId ?? 0
  }
}

Object.defineProperty(globalThis, 'ResizeObserver', { value: ResizeObserverMock, configurable: true })
Object.defineProperty(globalThis, 'PointerEvent', { value: PointerEventMock, configurable: true })
Object.defineProperty(window, 'matchMedia', {
  configurable: true,
  value: vi.fn().mockImplementation((query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: vi.fn(),
    removeListener: vi.fn(),
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    dispatchEvent: vi.fn(),
  })),
})

Object.defineProperty(HTMLElement.prototype, 'scrollTo', {
  configurable: true,
  value: vi.fn(),
})

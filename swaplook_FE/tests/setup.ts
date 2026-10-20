import { afterEach, vi } from 'vitest'
import { cleanup } from '@testing-library/react'

afterEach(() => { cleanup(); window.history.replaceState({}, '', '/'); vi.restoreAllMocks() })
Object.defineProperty(Element.prototype, 'scrollIntoView', { value: vi.fn(), writable: true })

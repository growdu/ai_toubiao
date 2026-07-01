// Vitest setup: runs before every test file. Resets localStorage between
// tests so the persisted Zustand store doesn't bleed state across cases.
import { afterEach } from 'vitest'

afterEach(() => {
  localStorage.clear()
  sessionStorage.clear()
})
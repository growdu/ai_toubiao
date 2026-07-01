// Vitest setup: runs before every test file.
// - Resets localStorage / sessionStorage between tests so the persisted
//   Zustand store doesn't bleed state across cases.
// - Calls @testing-library/react cleanup so each render() mounts into a
//   fresh DOM (otherwise multi-render tests fail with duplicate-element
//   errors).
import { afterEach } from 'vitest'
import { cleanup } from '@testing-library/react'
import '@testing-library/jest-dom/vitest'

afterEach(() => {
  localStorage.clear()
  sessionStorage.clear()
  cleanup()
})
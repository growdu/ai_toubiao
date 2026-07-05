import { describe, it, expect, beforeEach } from 'vitest'
import { useCommandPalette } from './CommandPalette'

describe('command palette store', () => {
  beforeEach(() => {
    useCommandPalette.setState({ open: false })
  })

  it('starts closed', () => {
    expect(useCommandPalette.getState().open).toBe(false)
  })

  it('setOpen toggles the visibility', () => {
    useCommandPalette.getState().setOpen(true)
    expect(useCommandPalette.getState().open).toBe(true)
    useCommandPalette.getState().setOpen(false)
    expect(useCommandPalette.getState().open).toBe(false)
  })

  it('toggle flips the state', () => {
    useCommandPalette.getState().toggle()
    expect(useCommandPalette.getState().open).toBe(true)
    useCommandPalette.getState().toggle()
    expect(useCommandPalette.getState().open).toBe(false)
  })
})
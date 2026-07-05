import { describe, it, expect, vi } from 'vitest'
import { renderHook } from '@testing-library/react'
import { useHotkey } from './useHotkey'

function fireKey(opts: KeyboardEventInit) {
  window.dispatchEvent(new KeyboardEvent('keydown', { bubbles: true, cancelable: true, ...opts }))
}

describe('useHotkey', () => {
  it('fires when the configured chord is pressed', () => {
    const handler = vi.fn()
    renderHook(() => useHotkey('mod+s', handler))
    fireKey({ key: 's', ctrlKey: true })
    expect(handler).toHaveBeenCalledTimes(1)
  })

  it('does not fire when the modifier is missing', () => {
    const handler = vi.fn()
    renderHook(() => useHotkey('mod+s', handler))
    fireKey({ key: 's' })
    expect(handler).not.toHaveBeenCalled()
  })

  it('respects the enabled flag', () => {
    const handler = vi.fn()
    const { rerender } = renderHook(({ enabled }) => useHotkey('mod+s', handler, { enabled }), {
      initialProps: { enabled: true },
    })
    rerender({ enabled: false })
    fireKey({ key: 's', ctrlKey: true })
    expect(handler).not.toHaveBeenCalled()
  })

  it('ignores presses that originate from an editable element', () => {
    const handler = vi.fn()
    renderHook(() => useHotkey('mod+s', handler))

    // Simulate the keydown event happening *from* the textarea (so e.target
    // points at it). For window-level listeners, e.target is the element
    // where the event originated.
    const ta = document.createElement('textarea')
    document.body.appendChild(ta)
    ta.focus()
    ta.dispatchEvent(new KeyboardEvent('keydown', {
      key: 's', ctrlKey: true, bubbles: true, cancelable: true,
    }))
    expect(handler).not.toHaveBeenCalled()
    ta.remove()
  })
})
import { useEffect } from 'react'

/**
 * Bind a keyboard shortcut. The handler only fires when the user's
 * focus is not inside a text input / textarea / contenteditable.
 */
export function useHotkey(
  combo: string,
  handler: (e: KeyboardEvent) => void,
  options: { preventDefault?: boolean; enabled?: boolean } = {},
) {
  const { preventDefault = true, enabled = true } = options

  useEffect(() => {
    if (!enabled) return
    const parts = combo.toLowerCase().split('+').map(s => s.trim())
    const key = parts[parts.length - 1]
    const needCtrl = parts.includes('ctrl') || parts.includes('cmd') || parts.includes('mod')
    const needShift = parts.includes('shift')
    const needAlt = parts.includes('alt') || parts.includes('option')

    function isEditable(el: EventTarget | null): boolean {
      if (!(el instanceof HTMLElement)) return false
      const tag = el.tagName
      if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return true
      if (el.isContentEditable) return true
      return false
    }

    function onKey(e: KeyboardEvent) {
      // Ignore modified shortcuts when in an input, unless they need a modifier.
      if (needCtrl && isEditable(e.target)) return
      if (!needCtrl && isEditable(e.target)) return

      if (needCtrl && !(e.ctrlKey || e.metaKey)) return
      if (needShift && !e.shiftKey) return
      if (needAlt && !e.altKey) return
      if (!needCtrl && (e.ctrlKey || e.metaKey)) return

      if (e.key.toLowerCase() !== key) return
      if (preventDefault) e.preventDefault()
      handler(e)
    }

    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [combo, handler, preventDefault, enabled])
}
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { useToastStore, toast } from './toast'

describe('toast store', () => {
  beforeEach(() => {
    useToastStore.setState({ toasts: [] })
    vi.useFakeTimers()
  })
  afterEach(() => {
    vi.useRealTimers()
  })

  it('push adds a toast and returns its id', () => {
    const id = toast.success('Saved', 'changes')
    const state = useToastStore.getState()
    expect(state.toasts).toHaveLength(1)
    expect(state.toasts[0]).toMatchObject({ tone: 'success', title: 'Saved', description: 'changes' })
    expect(state.toasts[0].id).toBe(id)
  })

  it('dismiss removes the toast by id', () => {
    const id = toast.error('Oops')
    useToastStore.getState().dismiss(id)
    expect(useToastStore.getState().toasts).toHaveLength(0)
  })

  it('auto-dismisses after the configured duration', () => {
    useToastStore.getState().push({ tone: 'info', title: 'temporary', duration: 1000 })
    expect(useToastStore.getState().toasts).toHaveLength(1)
    vi.advanceTimersByTime(1100)
    expect(useToastStore.getState().toasts).toHaveLength(0)
  })

  it('exposes success/error/info/warning helpers with the right tones', () => {
    toast.success('a')
    toast.error('b')
    toast.info('c')
    toast.warning('d')
    const tones = useToastStore.getState().toasts.map(t => t.tone)
    expect(tones).toEqual(['success', 'error', 'info', 'warning'])
  })

  // ---- Round 6+ additions: sticky / action / opts signature ----

  it('sticky toasts never auto-dismiss', () => {
    // The whole point of sticky is the user gets to read the result.
    // Even advancing time by an hour shouldn't remove it.
    toast.success('Approved', 'v3', undefined, { sticky: true })
    expect(useToastStore.getState().toasts).toHaveLength(1)
    vi.advanceTimersByTime(60 * 60 * 1000) // 1 hour
    expect(useToastStore.getState().toasts).toHaveLength(1)
  })

  it('sticky toasts default to duration=0 (not the 4s/8s fallback)', () => {
    toast.success('Pinned', undefined, undefined, { sticky: true })
    const t = useToastStore.getState().toasts[0]
    expect(t.sticky).toBe(true)
    expect(t.duration).toBe(0)
  })

  it('action-bearing toasts default to 8s duration', () => {
    const handler = vi.fn()
    toast.warning('Save failed', 'Try again?', { label: 'Retry', onClick: handler })
    const t = useToastStore.getState().toasts[0]
    expect(t.action?.label).toBe('Retry')
    expect(t.action?.onClick).toBe(handler)
    expect(t.duration).toBe(8000)
  })

  it('sticky overrides the action-bearing 8s default', () => {
    // When both sticky and action are set, sticky wins — the user
    // explicitly asked for permanence. The action is still tappable.
    toast.info('Manual review required', 'Approve when ready', { label: 'Approve', onClick: () => {} }, { sticky: true })
    const t = useToastStore.getState().toasts[0]
    expect(t.sticky).toBe(true)
    expect(t.action?.label).toBe('Approve')
    expect(t.duration).toBe(0)
  })

  it('action callback only fires when invoked — auto-dismiss does not trigger it', () => {
    const handler = vi.fn()
    toast.info('Background sync', undefined, { label: 'View', onClick: handler })
    // Auto-dismiss after 8s — handler should NOT have fired (we only
    // want it to fire on explicit click, not on timeout).
    vi.advanceTimersByTime(8100)
    expect(handler).not.toHaveBeenCalled()
    expect(useToastStore.getState().toasts).toHaveLength(0)
  })

  it('passes opts through correctly to all four tone helpers', () => {
    toast.success('s', undefined, undefined, { sticky: true })
    toast.error('e', undefined, undefined, { sticky: true })
    toast.info('i', undefined, undefined, { sticky: true })
    toast.warning('w', undefined, undefined, { sticky: true })
    const toasts = useToastStore.getState().toasts
    expect(toasts).toHaveLength(4)
    expect(toasts.every(t => t.sticky === true)).toBe(true)
    // All durations should be 0 (sticky overrides)
    expect(toasts.every(t => t.duration === 0)).toBe(true)
  })
})
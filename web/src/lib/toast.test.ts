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
})
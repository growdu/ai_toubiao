import { describe, it, expect, beforeEach } from 'vitest'
import { useAuthStore } from './auth'

describe('useAuthStore', () => {
  beforeEach(() => {
    // Reset store between tests (Zustand state persists across calls in the same process).
    useAuthStore.setState({ token: null, userId: null, tenantId: null })
  })

  it('starts with no auth state', () => {
    const s = useAuthStore.getState()
    expect(s.token).toBeNull()
    expect(s.userId).toBeNull()
    expect(s.tenantId).toBeNull()
  })

  it('setAuth stores all three fields', () => {
    useAuthStore.getState().setAuth('tok-123', 'user-1', 'tenant-A')
    const s = useAuthStore.getState()
    expect(s.token).toBe('tok-123')
    expect(s.userId).toBe('user-1')
    expect(s.tenantId).toBe('tenant-A')
  })

  it('logout clears all three fields', () => {
    useAuthStore.getState().setAuth('tok', 'u', 't')
    useAuthStore.getState().logout()
    const s = useAuthStore.getState()
    expect(s.token).toBeNull()
    expect(s.userId).toBeNull()
    expect(s.tenantId).toBeNull()
  })

  it('persists across re-imports (localStorage)', async () => {
    useAuthStore.getState().setAuth('persist-tok', 'persist-u', 'persist-t')

    // Trigger Zustand persist flush — the `persist` middleware writes to
    // localStorage synchronously on set(), but we read it explicitly to
    // pin the contract: "auth-storage" key holds the serialized state.
    const raw = localStorage.getItem('auth-storage')
    expect(raw).not.toBeNull()
    const parsed = JSON.parse(raw!)
    expect(parsed.state.token).toBe('persist-tok')
    expect(parsed.state.userId).toBe('persist-u')
    expect(parsed.state.tenantId).toBe('persist-t')
  })
})
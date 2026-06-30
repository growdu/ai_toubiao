import { create } from 'zustand'
import { persist } from 'zustand/middleware'

interface AuthState {
  token: string | null
  userId: string | null
  tenantId: string | null
  setAuth: (token: string, userId: string, tenantId: string) => void
  logout: () => void
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      token: null,
      userId: null,
      tenantId: null,
      setAuth: (token, userId, tenantId) => set({ token, userId, tenantId }),
      logout: () => set({ token: null, userId: null, tenantId: null }),
    }),
    {
      name: 'auth-storage',
    }
  )
)
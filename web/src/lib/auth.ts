import { create } from 'zustand'
import { persist } from 'zustand/middleware'

interface AuthState {
  token: string | null
  refreshToken: string | null
  userId: string | null
  tenantId: string | null
  setAuth: (token: string, userId: string, tenantId: string, refreshToken?: string) => void
  setTokens: (token: string, refreshToken: string) => void
  logout: () => void
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      token: null,
      refreshToken: null,
      userId: null,
      tenantId: null,
      setAuth: (token, userId, tenantId, refreshToken) => set({ token, userId, tenantId, refreshToken: refreshToken ?? null }),
      setTokens: (token, refreshToken) => set({ token, refreshToken }),
      logout: () => set({ token: null, refreshToken: null, userId: null, tenantId: null }),
    }),
    {
      name: 'auth-storage',
    }
  )
)
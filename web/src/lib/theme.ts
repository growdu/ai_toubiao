import { create } from 'zustand'
import { persist } from 'zustand/middleware'

export type ThemeMode = 'light' | 'dark' | 'system'
export type ResolvedTheme = 'light' | 'dark'

interface ThemeState {
  mode: ThemeMode
  resolved: ResolvedTheme
  setMode: (mode: ThemeMode) => void
  /** Apply the current mode to <html>. Called once on mount and whenever
   *  mode / system preference changes. */
  apply: () => void
}

function resolveSystem(): ResolvedTheme {
  if (typeof window === 'undefined') return 'light'
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

function applyResolved(theme: ResolvedTheme) {
  if (typeof document === 'undefined') return
  const root = document.documentElement
  root.classList.toggle('dark', theme === 'dark')
  root.style.colorScheme = theme
}

export const useThemeStore = create<ThemeState>()(
  persist(
    (set, get) => ({
      mode: 'light',
      resolved: 'light',
      setMode: (mode) => {
        const resolved = mode === 'system' ? resolveSystem() : mode
        applyResolved(resolved)
        set({ mode, resolved })
      },
      apply: () => {
        const resolved = get().mode === 'system' ? resolveSystem() : (get().mode as ResolvedTheme)
        applyResolved(resolved)
        set({ resolved })
      },
    }),
    {
      name: 'theme-storage',
      onRehydrateStorage: () => (state) => {
        // Apply on first paint after rehydration
        if (state) state.apply()
      },
    },
  ),
)

// React to OS theme changes when in "system" mode
if (typeof window !== 'undefined') {
  const mq = window.matchMedia('(prefers-color-scheme: dark)')
  mq.addEventListener('change', () => {
    const state = useThemeStore.getState()
    if (state.mode === 'system') state.apply()
  })
}
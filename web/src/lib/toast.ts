import { create } from 'zustand'

export type ToastTone = 'success' | 'error' | 'info' | 'warning'

export interface Toast {
  id: string
  tone: ToastTone
  title: string
  description?: string
  duration?: number
}

interface ToastState {
  toasts: Toast[]
  push: (toast: Omit<Toast, 'id'>) => string
  dismiss: (id: string) => void
  clear: () => void
}

export const useToastStore = create<ToastState>((set, get) => ({
  toasts: [],
  push: (toast) => {
    const id = `${Date.now()}-${Math.random().toString(36).slice(2, 7)}`
    const next: Toast = { duration: 4000, ...toast, id }
    set({ toasts: [...get().toasts, next] })
    if (next.duration && next.duration > 0) {
      setTimeout(() => get().dismiss(id), next.duration)
    }
    return id
  },
  dismiss: (id) => set({ toasts: get().toasts.filter(t => t.id !== id) }),
  clear: () => set({ toasts: [] }),
}))

// Convenience helpers (use inside components, no React required)
export const toast = {
  success: (title: string, description?: string) =>
    useToastStore.getState().push({ tone: 'success', title, description }),
  error:   (title: string, description?: string) =>
    useToastStore.getState().push({ tone: 'error', title, description, duration: 6000 }),
  info:    (title: string, description?: string) =>
    useToastStore.getState().push({ tone: 'info', title, description }),
  warning: (title: string, description?: string) =>
    useToastStore.getState().push({ tone: 'warning', title, description }),
}
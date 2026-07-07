import { create } from 'zustand'

export type ToastTone = 'success' | 'error' | 'info' | 'warning'

/** A button shown inside the toast card. Rendered as a compact link. */
export interface ToastAction {
  label: string
  onClick: () => void
}

export interface Toast {
  id: string
  tone: ToastTone
  title: string
  description?: string
  duration?: number
  /** When true, the toast never auto-dismisses — the user must click
   *  the X button (or another action). Use for operationally
   *  important events where the user needs to read the result before
   *  the UI moves on (e.g. "approve succeeded — v3" with new version
   *  number they should note). */
  sticky?: boolean
  /** Optional CTA button. Pair with a longer duration so the user has
   *  time to read and act. */
  action?: ToastAction
}

export type ToastPosition = 'top-right' | 'top-center' | 'bottom-right' | 'bottom-center'

interface ToastState {
  toasts: Toast[]
  position: ToastPosition
  setPosition: (position: ToastPosition) => void
  push: (toast: Omit<Toast, 'id'>) => string
  dismiss: (id: string) => void
  clear: () => void
}

export const useToastStore = create<ToastState>((set, get) => ({
  toasts: [],
  position: 'top-right',
  setPosition: (position) => set({ position }),
  push: (toast) => {
    const id = `${Date.now()}-${Math.random().toString(36).slice(2, 7)}`
    // Sticky toasts: no auto-dismiss (user must click X). No progress bar.
    // Action-bearing toasts default to 8s — long enough to read & click.
    // Otherwise the previous 4s default applies.
    //
    // Important precedence: if the caller passes an explicit `duration`
    // (e.g. error helper passes 6000 to override the default), respect
    // it UNLESS the toast is also sticky. Sticky wins because the user
    // explicitly asked for permanence and the auto-dismiss is the
    // single behaviour sticky is meant to override.
    let defaultDuration: number
    if (toast.sticky) {
      defaultDuration = 0
    } else if (toast.duration !== undefined) {
      defaultDuration = toast.duration
    } else if (toast.action) {
      defaultDuration = 8000
    } else {
      defaultDuration = 4000
    }
    const next: Toast = { ...toast, duration: defaultDuration, id }
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
  success: (title: string, description?: string, action?: ToastAction, opts?: { sticky?: boolean }) =>
    useToastStore.getState().push({ tone: 'success', title, description, ...(action ? { action } : {}), ...(opts?.sticky ? { sticky: true } : {}) }),
  error:   (title: string, description?: string, action?: ToastAction, opts?: { sticky?: boolean }) =>
    useToastStore.getState().push({ tone: 'error', title, description, duration: 6000, ...(action ? { action } : {}), ...(opts?.sticky ? { sticky: true } : {}) }),
  info:    (title: string, description?: string, action?: ToastAction, opts?: { sticky?: boolean }) =>
    useToastStore.getState().push({ tone: 'info', title, description, ...(action ? { action } : {}), ...(opts?.sticky ? { sticky: true } : {}) }),
  warning: (title: string, description?: string, action?: ToastAction, opts?: { sticky?: boolean }) =>
    useToastStore.getState().push({ tone: 'warning', title, description, ...(action ? { action } : {}), ...(opts?.sticky ? { sticky: true } : {}) }),
}
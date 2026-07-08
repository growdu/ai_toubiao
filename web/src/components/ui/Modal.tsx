import { ReactNode, useEffect } from 'react'
import { createPortal } from 'react-dom'

interface ModalProps {
  open: boolean
  onClose: () => void
  title?: ReactNode
  /** Subheader rendered below the title with smaller font. */
  description?: ReactNode
  size?: 'sm' | 'md' | 'lg' | 'xl'
  children: ReactNode
  footer?: ReactNode
  closeOnBackdrop?: boolean
  /** Optional icon shown in the header next to the title. */
  icon?: ReactNode
  /** Hide the default close button (use when footer has its own). */
  hideClose?: boolean
}

const sizeClass: Record<NonNullable<ModalProps['size']>, string> = {
  sm: 'max-w-sm',
  md: 'max-w-md',
  lg: 'max-w-2xl',
  xl: 'max-w-4xl',
}

export function Modal({ open, onClose, title, description, size = 'md', children, footer, closeOnBackdrop = true, icon, hideClose }: ModalProps) {
  useEffect(() => {
    if (!open) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', onKey)
    const prev = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => {
      document.removeEventListener('keydown', onKey)
      document.body.style.overflow = prev
    }
  }, [open, onClose])

  if (!open) return null

  return createPortal(
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 animate-fade-in">
      <div
        className="absolute inset-0 bg-ink-900/50 backdrop-blur-sm"
        onClick={closeOnBackdrop ? onClose : undefined}
      />
      <div
        className={`relative w-full ${sizeClass[size]} bg-white dark:bg-ink-800 rounded-2xl shadow-glow border border-ink-100 dark:border-ink-700 animate-scale-in max-h-[90vh] flex flex-col`}
        role="dialog"
        aria-modal="true"
      >
        {(title || !hideClose) && (
          <div className="flex items-start justify-between gap-3 px-5 py-4 border-b border-ink-100 dark:border-ink-700 shrink-0">
            <div className="flex items-start gap-3 min-w-0">
              {icon && (
                <div className="shrink-0 w-9 h-9 rounded-xl bg-brand-gradient-soft text-brand-600 grid place-items-center">
                  {icon}
                </div>
              )}
              <div className="min-w-0">
                {title && <h2 className="text-base font-semibold text-ink-900 dark:text-ink-100 truncate">{title}</h2>}
                {description && <p className="text-xs text-ink-500 mt-0.5">{description}</p>}
              </div>
            </div>
            {!hideClose && (
              <button
                onClick={onClose}
                className="text-ink-400 hover:text-ink-700 dark:hover:text-ink-200 transition-colors rounded-md p-1 hover:bg-ink-100 dark:hover:bg-ink-700 focus-ring-brand"
                aria-label="关闭"
              >
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round">
                  <path d="M6 6l12 12M6 18L18 6" />
                </svg>
              </button>
            )}
          </div>
        )}
        <div className="px-5 py-4 overflow-y-auto scrollbar-thin flex-1">{children}</div>
        {footer && (
          <div className="px-5 py-3 bg-ink-50/60 dark:bg-ink-900/40 border-t border-ink-100 dark:border-ink-700 rounded-b-2xl flex items-center justify-end gap-2 shrink-0">
            {footer}
          </div>
        )}
      </div>
    </div>,
    document.body,
  )
}
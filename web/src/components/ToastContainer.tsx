import { useToastStore, ToastTone, Toast } from '../lib/toast'

// Local selector to avoid TS warnings about implicit any in callbacks.
const selectToasts = (s: { toasts: Toast[] }) => s.toasts
const selectDismiss = (s: { dismiss: (id: string) => void }) => s.dismiss
const selectPosition = (s: { position: 'top-right' | 'top-center' | 'bottom-right' | 'bottom-center' }) => s.position

const ICONS: Record<ToastTone, React.ReactNode> = {
  success: (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
      <polyline points="20 6 9 17 4 12" />
    </svg>
  ),
  error: (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="12" cy="12" r="10" />
      <line x1="12" y1="8" x2="12" y2="12" />
      <line x1="12" y1="16" x2="12.01" y2="16" />
    </svg>
  ),
  info: (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="12" cy="12" r="10" />
      <line x1="12" y1="16" x2="12" y2="12" />
      <line x1="12" y1="8" x2="12.01" y2="8" />
    </svg>
  ),
  warning: (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z" />
      <line x1="12" y1="9" x2="12" y2="13" />
      <line x1="12" y1="17" x2="12.01" y2="17" />
    </svg>
  ),
}

const TONE: Record<ToastTone, string> = {
  success: 'bg-white dark:bg-ink-800 border-emerald-200 dark:border-emerald-800/50 text-emerald-700 dark:text-emerald-300',
  error:   'bg-white dark:bg-ink-800 border-red-200 dark:border-red-800/50 text-red-700 dark:text-red-300',
  info:    'bg-white dark:bg-ink-800 border-brand-200 dark:border-brand-800/50 text-brand-700 dark:text-brand-300',
  warning: 'bg-white dark:bg-ink-800 border-amber-200 dark:border-amber-800/50 text-amber-700 dark:text-amber-300',
}

const ICON_BG: Record<ToastTone, string> = {
  success: 'bg-emerald-50 dark:bg-emerald-900/40 text-emerald-600 dark:text-emerald-300',
  error:   'bg-red-50 dark:bg-red-900/40 text-red-600 dark:text-red-300',
  info:    'bg-brand-50 dark:bg-brand-900/40 text-brand-600 dark:text-brand-300',
  warning: 'bg-amber-50 dark:bg-amber-900/40 text-amber-600 dark:text-amber-300',
}

const TONE_ACCENT: Record<ToastTone, string> = {
  success: 'bg-emerald-500',
  error:   'bg-red-500',
  info:    'bg-brand-500',
  warning: 'bg-amber-500',
}

const positionClass: Record<string, string> = {
  'top-right':    'top-4 right-4',
  'top-center':   'top-4 left-1/2 -translate-x-1/2',
  'bottom-right': 'bottom-4 right-4',
  'bottom-center':'bottom-4 left-1/2 -translate-x-1/2',
}

export function ToastContainer() {
  const toasts = useToastStore(selectToasts)
  const dismiss = useToastStore(selectDismiss)
  const position = useToastStore(selectPosition)

  return (
    <div className={`fixed z-[100] flex flex-col gap-2 w-80 max-w-[calc(100vw-2rem)] pointer-events-none ${positionClass[position]}`}>
      {toasts.map(t => {
        const dur = (t.duration ?? 4000)
        return (
          <div
            key={t.id}
            role="alert"
            className={`relative pointer-events-auto overflow-hidden flex items-start gap-3 p-3 pr-2 rounded-xl shadow-pop border animate-slide-down ${TONE[t.tone]}`}
          >
            <div className={`shrink-0 w-7 h-7 rounded-lg flex items-center justify-center ${ICON_BG[t.tone]}`}>
              {ICONS[t.tone]}
            </div>
            <div className="min-w-0 flex-1">
              <div className="text-sm font-semibold text-ink-900 dark:text-white">{t.title}</div>
              {t.description && <div className="text-xs text-ink-500 dark:text-ink-400 mt-0.5">{t.description}</div>}
              {t.action && (
                <button
                  onClick={() => {
                    // Run the CTA synchronously, then dismiss the toast
                    // so the user can see the next state (success / error)
                    // appear in its place. We use a microtask so the
                    // dismiss doesn't fire before the action's state
                    // updates land.
                    t.action!.onClick()
                    queueMicrotask(() => dismiss(t.id))
                  }}
                  className="mt-1.5 inline-flex items-center gap-1 text-xs font-semibold text-brand-700 dark:text-brand-300 hover:text-brand-800 dark:hover:text-brand-200 transition-colors"
                >
                  {t.action.label}
                  <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
                    <polyline points="9 18 15 12 9 6" />
                  </svg>
                </button>
              )}
              {/* Sticky badge — appears on toasts that won't auto-dismiss.
                  Combined with the missing progress bar, it tells the
                  user "I won't disappear on you; click X when done". */}
              {t.sticky && (
                <span className="absolute top-2 right-9 inline-flex items-center gap-0.5 text-[9px] font-semibold uppercase tracking-wider text-ink-500 dark:text-ink-400 bg-ink-100 dark:bg-ink-800 px-1.5 py-0.5 rounded">
                  <svg width="8" height="8" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
                    <path d="M5 12h14" />
                  </svg>
                  持久
                </span>
              )}
              {/* dismiss progress bar — only shown for non-sticky toasts.
                  Sticky toasts intentionally skip the visual countdown
                  so the user knows nothing is fading. */}
              {!t.sticky && dur > 0 && (
                <div
                  className={`absolute left-0 bottom-0 h-0.5 ${TONE_ACCENT[t.tone]} opacity-50`}
                  style={{ animation: `toastProgress ${dur}ms linear forwards` }}
                />
              )}
            </div>
            <button
              onClick={() => dismiss(t.id)}
              className="shrink-0 text-ink-400 hover:text-ink-700 dark:hover:text-ink-200 transition-colors p-0.5 rounded hover:bg-ink-100 dark:hover:bg-ink-700"
              aria-label="关闭"
            >
              <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round">
                <path d="M6 6l12 12M6 18L18 6" />
              </svg>
            </button>
          </div>
        )
      })}
      <style>{`
        @keyframes toastProgress {
          from { width: 100%; }
          to   { width: 0%; }
        }
      `}</style>
    </div>
  )
}
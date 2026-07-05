import { useToastStore, ToastTone, Toast } from '../lib/toast'

// Local selector to avoid TS warnings about implicit any in callbacks.
const selectToasts = (s: { toasts: Toast[] }) => s.toasts
const selectDismiss = (s: { dismiss: (id: string) => void }) => s.dismiss

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
  success: 'bg-white border-emerald-200 text-emerald-700',
  error:   'bg-white border-red-200 text-red-700',
  info:    'bg-white border-brand-200 text-brand-700',
  warning: 'bg-white border-amber-200 text-amber-700',
}

const ICON_BG: Record<ToastTone, string> = {
  success: 'bg-emerald-50 text-emerald-600',
  error:   'bg-red-50 text-red-600',
  info:    'bg-brand-50 text-brand-600',
  warning: 'bg-amber-50 text-amber-600',
}

export function ToastContainer() {
  const toasts = useToastStore(selectToasts)
  const dismiss = useToastStore(selectDismiss)

  return (
    <div className="fixed top-4 right-4 z-[100] flex flex-col gap-2 w-80 max-w-[calc(100vw-2rem)] pointer-events-none">
      {toasts.map(t => (
        <div
          key={t.id}
          role="alert"
          className={`pointer-events-auto flex items-start gap-3 p-3 rounded-xl shadow-pop border animate-slide-down ${TONE[t.tone]}`}
        >
          <div className={`shrink-0 w-7 h-7 rounded-lg flex items-center justify-center ${ICON_BG[t.tone]}`}>
            {ICONS[t.tone]}
          </div>
          <div className="min-w-0 flex-1">
            <div className="text-sm font-semibold text-ink-900">{t.title}</div>
            {t.description && <div className="text-xs text-ink-500 mt-0.5">{t.description}</div>}
          </div>
          <button
            onClick={() => dismiss(t.id)}
            className="shrink-0 text-ink-400 hover:text-ink-700 transition-colors p-0.5 rounded hover:bg-ink-100"
            aria-label="关闭"
          >
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round">
              <path d="M6 6l12 12M6 18L18 6" />
            </svg>
          </button>
        </div>
      ))}
    </div>
  )
}
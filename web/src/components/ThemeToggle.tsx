import { useThemeStore, ThemeMode } from '../lib/theme'
import { useEffect, useState } from 'react'

const OPTIONS: { value: ThemeMode; label: string; icon: React.ReactNode }[] = [
  {
    value: 'light',
    label: '浅色',
    icon: (
      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <circle cx="12" cy="12" r="4" />
        <line x1="12" y1="2" x2="12" y2="4" />
        <line x1="12" y1="20" x2="12" y2="22" />
        <line x1="4.93" y1="4.93" x2="6.34" y2="6.34" />
        <line x1="17.66" y1="17.66" x2="19.07" y2="19.07" />
        <line x1="2" y1="12" x2="4" y2="12" />
        <line x1="20" y1="12" x2="22" y2="12" />
        <line x1="4.93" y1="19.07" x2="6.34" y2="17.66" />
        <line x1="17.66" y1="6.34" x2="19.07" y2="4.93" />
      </svg>
    ),
  },
  {
    value: 'dark',
    label: '深色',
    icon: (
      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z" />
      </svg>
    ),
  },
  {
    value: 'system',
    label: '跟随系统',
    icon: (
      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <rect x="3" y="4" width="18" height="12" rx="2" />
        <line x1="8" y1="20" x2="16" y2="20" />
        <line x1="12" y1="16" x2="12" y2="20" />
      </svg>
    ),
  },
]

interface ThemeToggleProps {
  /** "cycle" = three-state cycle button. "menu" = dropdown menu. */
  variant?: 'cycle' | 'menu'
  /** Override the button surface (e.g. when sitting inside a dark sidebar). */
  surface?: 'auto' | 'dark'
}

export function ThemeToggle({ variant = 'menu', surface = 'auto' }: ThemeToggleProps) {
  const mode = useThemeStore(s => s.mode)
  const resolved = useThemeStore(s => s.resolved)
  const setMode = useThemeStore(s => s.setMode)
  const apply = useThemeStore(s => s.apply)
  const [open, setOpen] = useState(false)

  // Apply theme once on mount (the persist middleware handles rehydration,
  // but the listener still needs to be initialized).
  useEffect(() => { apply() }, [apply])

  const current = OPTIONS.find(o => o.value === mode) ?? OPTIONS[0]

  // Resolve button color based on surface context.
  // "dark" = always use light icon on dark surface (e.g. inside the sidebar).
  // "auto" = follow the resolved theme.
  const buttonClass = surface === 'dark'
    ? 'p-2 rounded-lg text-white/60 hover:text-white hover:bg-white/5 transition-colors'
    : 'p-2 rounded-lg text-ink-500 hover:bg-ink-100 transition-colors dark:text-ink-300 dark:hover:bg-ink-800'

  if (variant === 'cycle') {
    return (
      <button
        onClick={() => {
          const next: ThemeMode = mode === 'light' ? 'dark' : mode === 'dark' ? 'system' : 'light'
          setMode(next)
        }}
        className={buttonClass}
        title={`主题: ${current.label}`}
        aria-label={`当前主题 ${current.label}，点击切换`}
      >
        {current.icon}
      </button>
    )
  }

  return (
    <div className="relative">
      <button
        onClick={() => setOpen(!open)}
        className={buttonClass}
        title="切换主题"
        aria-label="切换主题"
        aria-expanded={open}
      >
        {current.icon}
      </button>
      {open && (
        <>
          <div className="fixed inset-0 z-30" onClick={() => setOpen(false)} />
          <div className="absolute right-0 mt-1 w-36 bg-white dark:bg-ink-800 border border-ink-100 dark:border-ink-700 rounded-xl shadow-pop z-40 py-1 animate-slide-down">
            {OPTIONS.map(opt => (
              <button
                key={opt.value}
                onClick={() => { setMode(opt.value); setOpen(false) }}
                className={[
                  'w-full flex items-center gap-2 px-3 py-1.5 text-xs text-left transition-colors',
                  mode === opt.value
                    ? 'bg-brand-50 text-brand-700 dark:bg-brand-900 dark:text-brand-200'
                    : 'text-ink-600 hover:bg-ink-50 dark:text-ink-300 dark:hover:bg-ink-700',
                ].join(' ')}
              >
                {opt.icon}
                <span>{opt.label}</span>
                {opt.value === 'system' && resolved && (
                  <span className="ml-auto text-[10px] text-ink-400 dark:text-ink-500">
                    {resolved === 'dark' ? '暗' : '亮'}
                  </span>
                )}
              </button>
            ))}
          </div>
        </>
      )}
    </div>
  )
}
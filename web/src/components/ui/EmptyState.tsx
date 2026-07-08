import { ReactNode } from 'react'

interface EmptyStateProps {
  icon?: ReactNode
  title: string
  description?: string
  action?: ReactNode
  /** Render a tinted backdrop (brand/amber) for visual interest. */
  tone?: 'default' | 'brand' | 'amber'
}

const toneBg: Record<NonNullable<EmptyStateProps['tone']>, string> = {
  default: 'bg-brand-gradient-soft text-brand-500 border-brand-100',
  brand:   'bg-brand-gradient-soft text-brand-500 border-brand-100',
  amber:   'bg-amber-50 text-amber-500 border-amber-100',
}

export function EmptyState({ icon, title, description, action, tone = 'default' }: EmptyStateProps) {
  return (
    <div className="flex flex-col items-center justify-center text-center py-16 px-4 animate-fade-in">
      <div className={`relative w-20 h-20 rounded-2xl border grid place-items-center text-3xl mb-5 shadow-soft ${toneBg[tone]}`}>
        {icon ?? (
          <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
            <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
            <polyline points="14 2 14 8 20 8" />
            <line x1="9" y1="13" x2="15" y2="13" />
            <line x1="9" y1="17" x2="15" y2="17" />
          </svg>
        )}
        {/* soft halo behind the icon for depth */}
        <div className="absolute inset-0 rounded-2xl bg-current opacity-[0.05] blur-xl scale-150 -z-10" />
      </div>
      <h3 className="text-base font-semibold text-ink-800 dark:text-ink-100 mb-1">{title}</h3>
      {description && <p className="text-sm text-ink-500 dark:text-ink-400 max-w-sm mb-5">{description}</p>}
      {action}
    </div>
  )
}

interface StatCardProps {
  label: string
  value: ReactNode
  hint?: ReactNode
  tone?: 'blue' | 'green' | 'amber' | 'purple' | 'rose'
  icon?: ReactNode
  /** Trend indicator (e.g. +12%) shown right of value. */
  trend?: { value: string; direction: 'up' | 'down' | 'flat' }
}

const toneIconBg: Record<NonNullable<StatCardProps['tone']>, string> = {
  blue:   'bg-brand-50 text-brand-600 dark:bg-brand-900/30 dark:text-brand-300',
  green:  'bg-emerald-50 text-emerald-600 dark:bg-emerald-900/30 dark:text-emerald-300',
  amber:  'bg-amber-50 text-amber-600 dark:bg-amber-900/30 dark:text-amber-300',
  purple: 'bg-violet-50 text-violet-600 dark:bg-violet-900/30 dark:text-violet-300',
  rose:   'bg-rose-50 text-rose-600 dark:bg-rose-900/30 dark:text-rose-300',
}

export function StatCard({ label, value, hint, tone = 'blue', icon, trend }: StatCardProps) {
  return (
    <div className="card p-4 flex items-center gap-3">
      {icon && (
        <div className={`w-10 h-10 rounded-xl flex items-center justify-center shrink-0 ${toneIconBg[tone]}`}>
          {icon}
        </div>
      )}
      <div className="min-w-0 flex-1">
        <div className="text-xs text-ink-500 dark:text-ink-400">{label}</div>
        <div className="flex items-baseline gap-2">
          <div className="text-xl font-bold text-ink-900 dark:text-white leading-tight">{value}</div>
          {trend && (
            <span className={[
              'text-[11px] font-semibold tabular-nums inline-flex items-center gap-0.5',
              trend.direction === 'up' ? 'text-emerald-600' : trend.direction === 'down' ? 'text-rose-600' : 'text-ink-400',
            ].join(' ')}>
              {trend.direction === 'up' ? '↑' : trend.direction === 'down' ? '↓' : '·'}
              {trend.value}
            </span>
          )}
        </div>
        {hint && <div className="text-[11px] text-ink-400 dark:text-ink-500 mt-0.5">{hint}</div>}
      </div>
    </div>
  )
}
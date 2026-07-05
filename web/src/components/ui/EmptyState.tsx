import { ReactNode } from 'react'

interface EmptyStateProps {
  icon?: ReactNode
  title: string
  description?: string
  action?: ReactNode
}

export function EmptyState({ icon, title, description, action }: EmptyStateProps) {
  return (
    <div className="flex flex-col items-center justify-center text-center py-16 px-4 animate-fade-in">
      <div className="w-20 h-20 rounded-2xl bg-brand-gradient-soft border border-brand-100 flex items-center justify-center text-brand-500 text-3xl mb-5 shadow-soft">
        {icon ?? (
          <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
            <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
            <polyline points="14 2 14 8 20 8" />
            <line x1="9" y1="13" x2="15" y2="13" />
            <line x1="9" y1="17" x2="15" y2="17" />
          </svg>
        )}
      </div>
      <h3 className="text-base font-semibold text-ink-800 mb-1">{title}</h3>
      {description && <p className="text-sm text-ink-500 max-w-sm mb-5">{description}</p>}
      {action}
    </div>
  )
}

interface StatCardProps {
  label: string
  value: ReactNode
  hint?: ReactNode
  tone?: 'blue' | 'green' | 'amber' | 'purple'
  icon?: ReactNode
}

const toneIconBg: Record<NonNullable<StatCardProps['tone']>, string> = {
  blue:   'bg-brand-50 text-brand-600',
  green:  'bg-emerald-50 text-emerald-600',
  amber:  'bg-amber-50 text-amber-600',
  purple: 'bg-violet-50 text-violet-600',
}

export function StatCard({ label, value, hint, tone = 'blue', icon }: StatCardProps) {
  return (
    <div className="card p-4 flex items-center gap-3">
      {icon && (
        <div className={`w-10 h-10 rounded-xl flex items-center justify-center shrink-0 ${toneIconBg[tone]}`}>
          {icon}
        </div>
      )}
      <div className="min-w-0 flex-1">
        <div className="text-xs text-ink-500">{label}</div>
        <div className="text-xl font-bold text-ink-900 leading-tight">{value}</div>
        {hint && <div className="text-[11px] text-ink-400 mt-0.5">{hint}</div>}
      </div>
    </div>
  )
}
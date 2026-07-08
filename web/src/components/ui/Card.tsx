import { ReactNode, HTMLAttributes } from 'react'

interface CardProps extends HTMLAttributes<HTMLDivElement> {
  hover?: boolean
  /** When false the card skips default padding. Useful for tabbed headers / lists. */
  padded?: boolean
  children: ReactNode
  /** Subtle elevated look used for hero sections and selected states. */
  elevated?: boolean
  /** Tinted background (e.g. brand/amber). */
  tone?: 'default' | 'brand' | 'amber' | 'rose' | 'emerald'
}

const toneClass: Record<NonNullable<CardProps['tone']>, string> = {
  default: 'bg-white dark:bg-ink-800 border-ink-100/70 dark:border-ink-700/60',
  brand:   'bg-gradient-to-br from-brand-50 to-white border-brand-100 dark:from-brand-950/30 dark:to-ink-800 dark:border-brand-900/40',
  amber:   'bg-gradient-to-br from-amber-50 to-white border-amber-100 dark:from-amber-900/20 dark:to-ink-800 dark:border-amber-900/40',
  rose:    'bg-gradient-to-br from-rose-50 to-white border-rose-100 dark:from-rose-900/20 dark:to-ink-800 dark:border-rose-900/40',
  emerald: 'bg-gradient-to-br from-emerald-50 to-white border-emerald-100 dark:from-emerald-900/20 dark:to-ink-800 dark:border-emerald-900/40',
}

export function Card({ hover, padded = true, elevated, tone = 'default', className = '', children, ...rest }: CardProps) {
  return (
    <div
      className={[
        'rounded-2xl border shadow-soft',
        elevated ? 'shadow-pop' : '',
        toneClass[tone],
        padded ? 'p-5' : '',
        hover ? 'transition-all duration-200 hover:shadow-card-hover hover:-translate-y-0.5 hover:border-brand-200 dark:hover:border-brand-700' : '',
        className,
      ].join(' ')}
      {...rest}
    >
      {children}
    </div>
  )
}

export function CardHeader({ title, subtitle, action, icon, iconTone = 'brand', className = '' }: {
  title: ReactNode
  subtitle?: ReactNode
  action?: ReactNode
  icon?: ReactNode
  iconTone?: 'brand' | 'amber' | 'emerald' | 'rose' | 'purple' | 'ink'
  className?: string
}) {
  const toneBg: Record<NonNullable<typeof iconTone>, string> = {
    brand:   'bg-brand-50 text-brand-600 dark:bg-brand-900/40 dark:text-brand-300',
    amber:   'bg-amber-50 text-amber-600 dark:bg-amber-900/40 dark:text-amber-300',
    emerald: 'bg-emerald-50 text-emerald-600 dark:bg-emerald-900/40 dark:text-emerald-300',
    rose:    'bg-rose-50 text-rose-600 dark:bg-rose-900/40 dark:text-rose-300',
    purple:  'bg-violet-50 text-violet-600 dark:bg-violet-900/40 dark:text-violet-300',
    ink:     'bg-ink-100 text-ink-600 dark:bg-ink-700 dark:text-ink-300',
  }
  return (
    <div className={`flex items-start justify-between gap-3 mb-3 ${className}`}>
      <div className="flex items-start gap-3 min-w-0">
        {icon && (
          <div className={`shrink-0 w-9 h-9 rounded-lg grid place-items-center ${toneBg[iconTone]}`}>
            {icon}
          </div>
        )}
        <div className="min-w-0">
          <div className="text-sm font-semibold text-ink-800 dark:text-ink-100 truncate">{title}</div>
          {subtitle && <div className="text-xs text-ink-500 dark:text-ink-400 mt-0.5">{subtitle}</div>}
        </div>
      </div>
      {action && <div className="shrink-0">{action}</div>}
    </div>
  )
}
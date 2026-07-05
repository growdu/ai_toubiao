import { ReactNode, HTMLAttributes } from 'react'

interface CardProps extends HTMLAttributes<HTMLDivElement> {
  hover?: boolean
  padded?: boolean
  children: ReactNode
}

export function Card({ hover, padded = true, className = '', children, ...rest }: CardProps) {
  return (
    <div
      className={[
        'bg-white rounded-2xl shadow-soft border border-ink-100/70',
        padded ? 'p-5' : '',
        hover ? 'transition-all duration-200 hover:shadow-pop hover:-translate-y-0.5 hover:border-brand-200' : '',
        className,
      ].join(' ')}
      {...rest}
    >
      {children}
    </div>
  )
}

export function CardHeader({ title, subtitle, action, icon, className = '' }: {
  title: ReactNode
  subtitle?: ReactNode
  action?: ReactNode
  icon?: ReactNode
  className?: string
}) {
  return (
    <div className={`flex items-start justify-between gap-3 mb-3 ${className}`}>
      <div className="flex items-start gap-3 min-w-0">
        {icon && (
          <div className="shrink-0 w-9 h-9 rounded-lg bg-brand-50 text-brand-600 flex items-center justify-center">
            {icon}
          </div>
        )}
        <div className="min-w-0">
          <div className="text-sm font-semibold text-ink-800 truncate">{title}</div>
          {subtitle && <div className="text-xs text-ink-500 mt-0.5">{subtitle}</div>}
        </div>
      </div>
      {action && <div className="shrink-0">{action}</div>}
    </div>
  )
}
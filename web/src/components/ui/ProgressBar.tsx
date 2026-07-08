interface ProgressBarProps {
  value: number   // 0-100
  size?: 'sm' | 'md' | 'lg'
  showLabel?: boolean
  tone?: 'brand' | 'success' | 'amber' | 'rose' | 'rainbow'
  /** Whether to apply a moving shimmer overlay. */
  shimmer?: boolean
  /** Show a percentage indicator on the right (alias for showLabel). */
  showPercent?: boolean
}

const toneBar: Record<NonNullable<ProgressBarProps['tone']>, string> = {
  brand:   'bg-gradient-to-r from-brand-500 to-brand-600',
  success: 'bg-gradient-to-r from-emerald-500 to-emerald-600',
  amber:   'bg-gradient-to-r from-amber-500 to-amber-600',
  rose:    'bg-gradient-to-r from-rose-500 to-rose-600',
  rainbow: 'bg-gradient-to-r from-brand-500 via-violet-500 to-emerald-500 bg-[length:200%_100%] animate-gradient-x',
}

const sizeBar: Record<NonNullable<ProgressBarProps['size']>, string> = {
  sm: 'h-1',
  md: 'h-1.5',
  lg: 'h-2.5',
}

export function ProgressBar({ value, size = 'md', showLabel, tone = 'brand', shimmer, showPercent }: ProgressBarProps) {
  const v = Math.max(0, Math.min(100, value))
  const showPct = showLabel || showPercent
  return (
    <div className="flex items-center gap-2">
      <div className={`relative flex-1 bg-ink-100 dark:bg-ink-700 rounded-full overflow-hidden ${sizeBar[size]}`}>
        <div
          className={`h-full rounded-full transition-all duration-500 ${toneBar[tone]} ${shimmer ? 'progress-bar-shimmer' : ''}`}
          style={{ width: `${v}%` }}
        />
      </div>
      {showPct && <span className="text-xs text-ink-500 dark:text-ink-400 tabular-nums w-9 text-right font-medium">{v}%</span>}
    </div>
  )
}

/** Circular progress — value is 0-100. */
export function RingProgress({ value, size = 48, strokeWidth = 4, tone = 'brand', children }: {
  value: number
  size?: number
  strokeWidth?: number
  tone?: 'brand' | 'success' | 'amber'
  children?: React.ReactNode
}) {
  const v = Math.max(0, Math.min(100, value))
  const r = (size - strokeWidth) / 2
  const c = 2 * Math.PI * r
  const offset = c * (1 - v / 100)
  const color = {
    brand: 'stroke-brand-600',
    success: 'stroke-emerald-600',
    amber: 'stroke-amber-500',
  }[tone]
  return (
    <div className="relative inline-flex items-center justify-center" style={{ width: size, height: size }}>
      <svg className="ring-progress" width={size} height={size}>
        <circle cx={size / 2} cy={size / 2} r={r} fill="none" className="stroke-ink-100 dark:stroke-ink-700" strokeWidth={strokeWidth} />
        <circle
          cx={size / 2} cy={size / 2} r={r} fill="none"
          className={`${color} transition-all duration-500`}
          strokeWidth={strokeWidth}
          strokeLinecap="round"
          strokeDasharray={c}
          strokeDashoffset={offset}
        />
      </svg>
      {children && <div className="absolute inset-0 grid place-items-center text-xs font-semibold text-ink-700 dark:text-ink-200">{children}</div>}
    </div>
  )
}
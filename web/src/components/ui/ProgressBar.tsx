interface ProgressBarProps {
  value: number   // 0-100
  size?: 'sm' | 'md'
  showLabel?: boolean
  tone?: 'brand' | 'success'
}

const toneBar: Record<NonNullable<ProgressBarProps['tone']>, string> = {
  brand:   'bg-gradient-to-r from-brand-500 to-brand-600',
  success: 'bg-gradient-to-r from-emerald-500 to-emerald-600',
}

const sizeBar: Record<NonNullable<ProgressBarProps['size']>, string> = {
  sm: 'h-1',
  md: 'h-1.5',
}

export function ProgressBar({ value, size = 'md', showLabel, tone = 'brand' }: ProgressBarProps) {
  const v = Math.max(0, Math.min(100, value))
  return (
    <div className="flex items-center gap-2">
      <div className={`flex-1 bg-ink-100 rounded-full overflow-hidden ${sizeBar[size]}`}>
        <div
          className={`h-full rounded-full transition-all duration-500 ${toneBar[tone]}`}
          style={{ width: `${v}%` }}
        />
      </div>
      {showLabel && <span className="text-xs text-ink-500 tabular-nums w-9 text-right">{v}%</span>}
    </div>
  )
}
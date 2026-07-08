interface DividerProps {
  /** Orientation of the divider. Defaults to horizontal. */
  orientation?: 'horizontal' | 'vertical'
  className?: string
  /** Decorative label rendered in the middle. */
  label?: React.ReactNode
}

/**
 * Hairline divider. When `label` is set, render a centered chip in the
 * middle of the line so sections inside dense forms can still feel airy.
 */
export function Divider({ orientation = 'horizontal', className = '', label }: DividerProps) {
  if (orientation === 'vertical') {
    return <div className={`inline-block w-px h-full bg-ink-200 dark:bg-ink-700 ${className}`} aria-hidden />
  }
  if (label) {
    return (
      <div className={`flex items-center gap-3 my-3 ${className}`}>
        <div className="flex-1 h-px bg-ink-200 dark:bg-ink-700" />
        <span className="text-[11px] uppercase tracking-wider text-ink-400 font-medium">{label}</span>
        <div className="flex-1 h-px bg-ink-200 dark:bg-ink-700" />
      </div>
    )
  }
  return <div className={`h-px w-full bg-ink-200 dark:bg-ink-700 my-3 ${className}`} aria-hidden />
}
import { ButtonHTMLAttributes, forwardRef } from 'react'

type Variant = 'primary' | 'secondary' | 'ghost' | 'danger' | 'gradient' | 'outline'
type Size = 'xs' | 'sm' | 'md' | 'lg'

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant
  size?: Size
  loading?: boolean
  leftIcon?: React.ReactNode
  rightIcon?: React.ReactNode
  /** Render as full-width block. */
  block?: boolean
}

const variantClass: Record<Variant, string> = {
  primary:   'bg-brand-600 text-white shadow-sm hover:bg-brand-700 hover:shadow focus-visible:ring-brand-400',
  secondary: 'bg-white text-ink-700 border border-ink-200 hover:bg-ink-50 hover:border-ink-300 focus-visible:ring-brand-300 dark:bg-ink-800 dark:text-ink-100 dark:border-ink-700 dark:hover:bg-ink-700',
  ghost:     'bg-transparent text-ink-600 hover:bg-ink-100 focus-visible:ring-brand-300 dark:text-ink-300 dark:hover:bg-ink-800',
  danger:    'bg-red-600 text-white hover:bg-red-700 focus-visible:ring-red-400 shadow-sm',
  // Filled gradient: hero CTAs. Animates the gradient sweep so it feels alive.
  gradient:  'bg-gradient-to-r from-brand-600 via-brand-500 to-brand-700 text-white shadow-pop hover:shadow-glow-soft bg-[length:200%_100%] hover:bg-[position:100%_0] focus-visible:ring-brand-400 transition-all',
  // Outlined brand (transparent fill, brand border + text). Used in compact contexts.
  outline:   'bg-transparent text-brand-700 border border-brand-300 hover:bg-brand-50 hover:border-brand-400 focus-visible:ring-brand-300 dark:text-brand-200 dark:border-brand-700 dark:hover:bg-brand-900',
}

const sizeClass: Record<Size, string> = {
  xs: 'px-2 py-0.5 text-[11px] gap-1',
  sm: 'px-2.5 py-1 text-xs gap-1.5',
  md: 'px-4 py-2 text-sm gap-2',
  lg: 'px-5 py-2.5 text-base gap-2',
}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(function Button(
  { variant = 'secondary', size = 'md', loading, leftIcon, rightIcon, disabled, block, className = '', children, ...rest },
  ref,
) {
  const isDisabled = disabled || loading
  return (
    <button
      ref={ref}
      disabled={isDisabled}
      className={[
        'inline-flex items-center justify-center rounded-lg font-medium select-none whitespace-nowrap',
        'transition-all duration-150 active:scale-[0.98]',
        'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-offset-1',
        'disabled:opacity-50 disabled:cursor-not-allowed disabled:pointer-events-none',
        block ? 'w-full' : '',
        variantClass[variant],
        sizeClass[size],
        className,
      ].join(' ')}
      {...rest}
    >
      {loading ? (
        <svg className="animate-spin w-3.5 h-3.5 shrink-0" viewBox="0 0 24 24" fill="none">
          <circle cx="12" cy="12" r="10" stroke="currentColor" strokeOpacity="0.25" strokeWidth="4" />
          <path d="M22 12a10 10 0 0 1-10 10" stroke="currentColor" strokeWidth="4" strokeLinecap="round" />
        </svg>
      ) : leftIcon}
      {children}
      {!loading && rightIcon}
    </button>
  )
})
import { useState, ReactNode } from 'react'

export interface TabItem {
  id: string
  label: ReactNode
  /** Optional icon (rendered left of label). */
  icon?: ReactNode
  /** Optional badge/count rendered right-aligned in the tab. */
  badge?: ReactNode
  /** Disabled state — tab is rendered but not clickable. */
  disabled?: boolean
}

interface TabsProps {
  items: TabItem[]
  /** Controlled active id. */
  value?: string
  /** Uncontrolled default. */
  defaultValue?: string
  onChange?: (id: string) => void
  /** Visual variant. "underline" = nav tabs (default), "pill" = segmented. */
  variant?: 'underline' | 'pill'
  className?: string
  /** Stretch tab strip to full width (distributes items evenly). */
  fullWidth?: boolean
}

/**
 * Lightweight tabs. The tab strip is owned by this component; the panels
 * are rendered by the caller (we don't render children) so callers can
 * keep panel content co-located with the state that selects it.
 */
export function Tabs({ items, value, defaultValue, onChange, variant = 'underline', className = '', fullWidth }: TabsProps) {
  const [internal, setInternal] = useState(defaultValue ?? items[0]?.id)
  const active = value ?? internal

  const handleClick = (id: string, disabled?: boolean) => {
    if (disabled) return
    if (value === undefined) setInternal(id)
    onChange?.(id)
  }

  if (variant === 'pill') {
    return (
      <div className={`inline-flex items-center gap-1 p-1 bg-ink-100 dark:bg-ink-800 rounded-xl ${className}`}>
        {items.map(item => {
          const isActive = active === item.id
          return (
            <button
              key={item.id}
              type="button"
              disabled={item.disabled}
              onClick={() => handleClick(item.id, item.disabled)}
              className={[
                'inline-flex items-center gap-2 px-3 py-1.5 text-xs font-medium rounded-lg transition-all',
                fullWidth ? 'flex-1 justify-center' : '',
                isActive
                  ? 'bg-white dark:bg-ink-700 text-brand-700 dark:text-brand-200 shadow-sm'
                  : 'text-ink-600 dark:text-ink-300 hover:text-ink-800 dark:hover:text-white',
                item.disabled ? 'opacity-40 cursor-not-allowed' : '',
              ].join(' ')}
            >
              {item.icon}
              <span>{item.label}</span>
              {item.badge}
            </button>
          )
        })}
      </div>
    )
  }

  // underline variant
  return (
    <div className={`relative border-b border-ink-200 dark:border-ink-700 ${className}`}>
      <nav className={`flex items-center gap-1 ${fullWidth ? 'w-full' : ''}`}>
        {items.map(item => {
          const isActive = active === item.id
          return (
            <button
              key={item.id}
              type="button"
              disabled={item.disabled}
              onClick={() => handleClick(item.id, item.disabled)}
              className={[
                'relative inline-flex items-center gap-2 px-4 py-2.5 text-sm font-medium transition-colors',
                fullWidth ? 'flex-1 justify-center' : '',
                isActive ? 'text-brand-700 dark:text-brand-300' : 'text-ink-500 dark:text-ink-400 hover:text-ink-800 dark:hover:text-ink-100',
                item.disabled ? 'opacity-40 cursor-not-allowed' : '',
              ].join(' ')}
            >
              {item.icon}
              <span>{item.label}</span>
              {item.badge}
              {isActive && (
                <span className="absolute -bottom-px left-2 right-2 h-0.5 bg-brand-600 rounded-full" />
              )}
            </button>
          )
        })}
      </nav>
    </div>
  )
}
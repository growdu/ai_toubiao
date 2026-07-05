import { ReactNode } from 'react'

type StatusTone =
  | 'blue' | 'green' | 'amber' | 'red' | 'purple' | 'gray'
  | 'planned' | 'pending' | 'running' | 'succeeded' | 'failed'
  | 'skipped' | 'paused' | 'done' | 'facts' | 'auditing'
  | 'exporting' | 'generating' | 'parsing' | 'outlining'

const toneClass: Record<string, string> = {
  blue:   'bg-brand-50 text-brand-700',
  green:  'bg-emerald-50 text-emerald-700',
  amber:  'bg-amber-50 text-amber-700',
  red:    'bg-red-50 text-red-700',
  purple: 'bg-violet-50 text-violet-700',
  gray:   'bg-ink-100 text-ink-600',

  planned:    'bg-ink-100 text-ink-600',
  pending:    'bg-amber-50 text-amber-700',
  running:    'bg-brand-50 text-brand-700',
  succeeded:  'bg-emerald-50 text-emerald-700',
  failed:     'bg-red-50 text-red-700',
  skipped:    'bg-ink-100 text-ink-400',
  paused:     'bg-amber-50 text-amber-700',
  done:       'bg-emerald-50 text-emerald-700',
  facts:      'bg-violet-50 text-violet-700',
  auditing:   'bg-violet-50 text-violet-700',
  exporting:  'bg-brand-50 text-brand-700',
  generating: 'bg-brand-50 text-brand-700',
  parsing:    'bg-brand-50 text-brand-700',
  outlining:  'bg-brand-50 text-brand-700',
}

const STATUS_DOT: Record<string, string> = {
  running: 'bg-brand-500 animate-pulse-soft',
  generating: 'bg-brand-500 animate-pulse-soft',
  parsing: 'bg-brand-500 animate-pulse-soft',
  outlining: 'bg-brand-500 animate-pulse-soft',
  auditing: 'bg-violet-500 animate-pulse-soft',
  facts: 'bg-violet-500 animate-pulse-soft',
  pending: 'bg-amber-500',
  failed: 'bg-red-500',
  done: 'bg-emerald-500',
  succeeded: 'bg-emerald-500',
}

interface BadgeProps {
  tone?: StatusTone
  status?: string
  showDot?: boolean
  children?: ReactNode
  className?: string
}

export function Badge({ tone, status, showDot, children, className = '' }: BadgeProps) {
  const cls = toneClass[tone ?? status ?? 'gray'] ?? toneClass.gray
  const dotCls = STATUS_DOT[status ?? '']
  return (
    <span className={`inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium ${cls} ${className}`}>
      {(showDot || dotCls) && (
        <span className={`inline-block w-1.5 h-1.5 rounded-full ${dotCls ?? 'bg-current opacity-60'}`} />
      )}
      {children ?? status}
    </span>
  )
}

interface StatusBadgeProps {
  status: string
  labels?: Record<string, string>
  showDot?: boolean
}

export function StatusBadge({ status, labels = {}, showDot = true }: StatusBadgeProps) {
  const label = labels[status] ?? status
  return <Badge status={status} showDot={showDot}>{label}</Badge>
}
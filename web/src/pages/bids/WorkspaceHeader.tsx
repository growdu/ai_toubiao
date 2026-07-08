import { Link } from 'react-router-dom'
import { ProgressBar, StatusBadge } from '../../components/ui'
import { BID_STATUS_LABELS, WORKFLOW_STEPS, workflowStepIndex } from './workspace-helpers'

interface WorkspaceHeaderProps {
  bidId: string
  projectName: string
  bidStatus: string
  doneChapters: number
  totalChapters: number
  rightActions?: React.ReactNode
}

export function WorkspaceHeader({
  projectName, bidStatus, doneChapters, totalChapters, rightActions,
}: WorkspaceHeaderProps) {
  const progress = totalChapters > 0 ? Math.round((doneChapters / totalChapters) * 100) : 0
  const stepIdx = workflowStepIndex(bidStatus)
  const isFailed = bidStatus === 'failed'

  return (
    <header className="bg-white dark:bg-ink-800 border-b border-ink-100 dark:border-ink-700 shrink-0 shadow-soft">
      {/* Top row: breadcrumb + actions */}
      <div className="flex items-center justify-between px-6 py-3">
        <div className="flex items-center gap-3 min-w-0">
          <Link
            to="/bids"
            className="shrink-0 inline-flex items-center gap-1 text-xs text-ink-500 dark:text-ink-400 hover:text-ink-800 dark:hover:text-ink-200 transition-colors"
          >
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <line x1="19" y1="12" x2="5" y2="12" />
              <polyline points="12 19 5 12 12 5" />
            </svg>
            <span>标书列表</span>
          </Link>
          <span className="text-ink-200 dark:text-ink-700">/</span>
          <div className="min-w-0 flex items-center gap-2">
            <h1 className="text-base font-semibold text-ink-900 dark:text-white truncate">{projectName || '标书'}</h1>
            <StatusBadge status={bidStatus} labels={BID_STATUS_LABELS} />
          </div>
        </div>

        <div className="flex items-center gap-3 shrink-0">
          <div className="hidden md:flex items-center gap-3 text-xs text-ink-500 dark:text-ink-400">
            <div className="flex items-center gap-2 min-w-[140px]">
              <ProgressBar
                value={progress}
                size="sm"
                showLabel
                tone={bidStatus === 'done' ? 'success' : bidStatus === 'failed' ? 'rose' : 'brand'}
              />
            </div>
            <span className="tabular-nums text-ink-600 dark:text-ink-300">
              <strong className="text-ink-800 dark:text-white">{doneChapters}</strong>/{totalChapters} 章节
            </span>
          </div>
          {rightActions}
        </div>
      </div>

      {/* Workflow stepper */}
      <div className="px-6 pb-3">
        <ol className="flex items-center gap-1 overflow-x-auto scrollbar-none">
          {WORKFLOW_STEPS.map((step, i) => {
            const reached = i <= stepIdx
            const active = i === stepIdx
            const isLast = i === WORKFLOW_STEPS.length - 1
            return (
              <li key={step.id} className="flex items-center gap-1 shrink-0">
                <div className={[
                  'flex items-center gap-2 px-2.5 py-1 rounded-full text-xs font-medium transition-all',
                  active ? 'bg-brand-50 dark:bg-brand-900/40 text-brand-700 dark:text-brand-300 shadow-sm ring-1 ring-brand-200 dark:ring-brand-800' :
                  reached ? 'text-emerald-700 dark:text-emerald-400' :
                  'text-ink-400 dark:text-ink-500',
                ].join(' ')}>
                  <span className={[
                    'inline-flex items-center justify-center w-4 h-4 rounded-full text-[10px] font-bold transition-all',
                    active ? 'bg-brand-600 text-white shadow-pop' :
                    reached ? 'bg-emerald-500 text-white' :
                    'bg-ink-200 dark:bg-ink-700 text-ink-500 dark:text-ink-400',
                  ].join(' ')}>
                    {reached && !active ? (
                      <svg width="9" height="9" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round">
                        <polyline points="20 6 9 17 4 12" />
                      </svg>
                    ) : i + 1}
                  </span>
                  <span>{step.label}</span>
                </div>
                {!isLast && (
                  <div className={[
                    'w-6 h-px transition-colors',
                    i < stepIdx ? 'bg-emerald-300 dark:bg-emerald-700' : 'bg-ink-200 dark:bg-ink-700',
                  ].join(' ')} />
                )}
              </li>
            )
          })}
        </ol>
        {isFailed && (
          <div className="mt-2 px-3 py-2 rounded-lg bg-red-50 dark:bg-red-900/20 border border-red-100 dark:border-red-900/40 text-xs text-red-700 dark:text-red-300 inline-flex items-center gap-2">
            <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="shrink-0">
              <circle cx="12" cy="12" r="10" />
              <line x1="12" y1="8" x2="12" y2="12" />
              <line x1="12" y1="16" x2="12.01" y2="16" />
            </svg>
            当前工作流失败。可点击底部按钮重试，或在右侧针对单章节重新生成。
          </div>
        )}
      </div>
    </header>
  )
}
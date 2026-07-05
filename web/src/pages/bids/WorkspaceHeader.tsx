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
    <header className="bg-white border-b border-ink-100 shrink-0">
      {/* Top row: breadcrumb + actions */}
      <div className="flex items-center justify-between px-6 py-3">
        <div className="flex items-center gap-3 min-w-0">
          <Link
            to="/bids"
            className="shrink-0 inline-flex items-center gap-1 text-xs text-ink-500 hover:text-ink-800 transition-colors"
          >
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <line x1="19" y1="12" x2="5" y2="12" />
              <polyline points="12 19 5 12 12 5" />
            </svg>
            <span>标书列表</span>
          </Link>
          <span className="text-ink-200">/</span>
          <div className="min-w-0 flex items-center gap-2">
            <h1 className="text-base font-semibold text-ink-900 truncate">{projectName || '标书'}</h1>
            <StatusBadge status={bidStatus} labels={BID_STATUS_LABELS} />
          </div>
        </div>

        <div className="flex items-center gap-3 shrink-0">
          <div className="hidden md:flex items-center gap-3 text-xs text-ink-500">
            <div className="flex items-center gap-2 min-w-[140px]">
              <ProgressBar
                value={progress}
                size="sm"
                showLabel
                tone={bidStatus === 'done' ? 'success' : 'brand'}
              />
            </div>
            <span className="tabular-nums">
              <strong className="text-ink-800">{doneChapters}</strong>/{totalChapters} 章节
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
                  'flex items-center gap-2 px-2.5 py-1 rounded-full text-xs font-medium transition-colors',
                  active ? 'bg-brand-50 text-brand-700' :
                  reached ? 'text-emerald-700' :
                  'text-ink-400',
                ].join(' ')}>
                  <span className={[
                    'inline-flex items-center justify-center w-4 h-4 rounded-full text-[10px] font-bold',
                    active ? 'bg-brand-600 text-white' :
                    reached ? 'bg-emerald-500 text-white' :
                    'bg-ink-200 text-ink-500',
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
                    'w-6 h-px',
                    i < stepIdx ? 'bg-emerald-300' : 'bg-ink-200',
                  ].join(' ')} />
                )}
              </li>
            )
          })}
        </ol>
        {isFailed && (
          <div className="mt-2 px-3 py-2 rounded-lg bg-red-50 border border-red-100 text-xs text-red-700">
            当前工作流失败。可点击底部按钮重试，或在右侧针对单章节重新生成。
          </div>
        )}
      </div>
    </header>
  )
}
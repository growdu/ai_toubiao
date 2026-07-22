// 4 步标书向导顶部进度条：解析材料 -> 审核材料 -> 生成标书 -> 精修章节。
// current 为当前步骤索引（0-3）。WizardSteps 在步骤1/2 间切换时传入 0/1；
// BidWorkspace 根据工作流状态映射到 2/3。
const STEP_LABELS = ['解析材料', '审核材料', '生成标书', '精修章节']

// 把工作流状态映射到 4 步向导索引（0-3）。
// outlining/facts -> 2（生成标书）；generating 及之后 -> 3（精修章节）。
// pending/parsing 由 WizardSteps 自行管理（0/1），不经过本函数。
export function stepFromStatus(status: string | undefined): number {
  switch (status) {
    case 'outlining':
    case 'facts':
      return 2
    case 'generating':
    case 'awaiting_review':
    case 'auditing':
    case 'exporting':
    case 'done':
      return 3
    default:
      return 2
  }
}

export function BidStepper({ current }: { current: number }) {
  return (
    <div className="border-b border-ink-100 dark:border-ink-700 bg-white dark:bg-ink-800 px-6 py-3 shrink-0">
      <ol className="max-w-5xl mx-auto flex items-center gap-2">
        {STEP_LABELS.map((label, i) => {
          const done = i < current
          const active = i === current
          return (
            <li key={label} className="flex items-center gap-2 flex-1">
              <span
                className={[
                  'shrink-0 w-7 h-7 rounded-full flex items-center justify-center text-xs font-semibold transition-colors',
                  active
                    ? 'bg-brand-600 text-white'
                    : done
                    ? 'bg-emerald-500 text-white'
                    : 'bg-ink-200 dark:bg-ink-600 text-ink-500 dark:text-ink-300',
                ].join(' ')}
              >
                {done ? '✓' : i + 1}
              </span>
              <span
                className={[
                  'text-xs whitespace-nowrap',
                  active
                    ? 'text-brand-700 dark:text-brand-300 font-semibold'
                    : 'text-ink-500 dark:text-ink-400',
                ].join(' ')}
              >
                {label}
              </span>
              {i < STEP_LABELS.length - 1 && (
                <span
                  className={['flex-1 h-px', done ? 'bg-emerald-400' : 'bg-ink-200 dark:bg-ink-700'].join(' ')}
                />
              )}
            </li>
          )
        })}
      </ol>
    </div>
  )
}

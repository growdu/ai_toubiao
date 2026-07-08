interface SkeletonProps {
  className?: string
  rounded?: 'sm' | 'md' | 'lg' | 'xl' | 'full' | 'none'
}

const roundedClass = {
  sm: 'rounded',
  md: 'rounded-md',
  lg: 'rounded-lg',
  xl: 'rounded-xl',
  full: 'rounded-full',
  none: '',
}

export function Skeleton({ className = '', rounded = 'md' }: SkeletonProps) {
  return <div className={`skeleton ${roundedClass[rounded]} ${className}`} />
}

export function SkeletonText({ lines = 3, className = '' }: { lines?: number; className?: string }) {
  const widths = ['w-full', 'w-11/12', 'w-4/5', 'w-2/3', 'w-3/4']
  return (
    <div className={`space-y-2 ${className}`}>
      {Array.from({ length: lines }).map((_, i) => (
        <Skeleton
          key={i}
          className={`h-3 ${widths[i % widths.length]}`}
          rounded="sm"
        />
      ))}
    </div>
  )
}

export function SkeletonCard({ className = '' }: { className?: string }) {
  return (
    <div className={`card p-5 ${className}`}>
      <div className="flex items-start gap-3 mb-4">
        <Skeleton className="w-10 h-10 shrink-0" rounded="lg" />
        <div className="flex-1 space-y-2">
          <Skeleton className="h-4 w-2/3" />
          <Skeleton className="h-3 w-1/2" />
        </div>
        <Skeleton className="h-5 w-16" rounded="full" />
      </div>
      <SkeletonText lines={2} />
    </div>
  )
}

export function SkeletonList({ rows = 5, className = '' }: { rows?: number; className?: string }) {
  return (
    <div className={`space-y-2 ${className}`}>
      {Array.from({ length: rows }).map((_, i) => (
        <div key={i} className="flex items-center gap-3 p-3 rounded-xl bg-white dark:bg-ink-800 border border-ink-100 dark:border-ink-700">
          <Skeleton className="w-9 h-9 shrink-0" rounded="lg" />
          <div className="flex-1 space-y-2">
            <Skeleton className="h-3.5 w-1/3" />
            <Skeleton className="h-3 w-1/2" />
          </div>
          <Skeleton className="h-5 w-14" rounded="full" />
        </div>
      ))}
    </div>
  )
}

export function SkeletonStatCard() {
  return (
    <div className="card p-4 flex items-center gap-3">
      <Skeleton className="w-10 h-10 shrink-0" rounded="xl" />
      <div className="flex-1 space-y-2">
        <Skeleton className="h-3 w-16" />
        <Skeleton className="h-6 w-12" />
      </div>
    </div>
  )
}

export function SkeletonTableRow({ columns = 4 }: { columns?: number }) {
  return (
    <div className="flex items-center gap-3 py-3 border-b border-ink-100">
      {Array.from({ length: columns }).map((_, i) => (
        <Skeleton key={i} className={`h-4 ${i === 0 ? 'w-1/3' : 'flex-1'}`} />
      ))}
    </div>
  )
}
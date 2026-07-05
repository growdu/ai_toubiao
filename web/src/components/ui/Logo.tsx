interface LogoProps {
  size?: number
  withWordmark?: boolean
}

export function Logo({ size = 28, withWordmark = true }: LogoProps) {
  return (
    <div className="flex items-center gap-2.5">
      <svg width={size} height={size} viewBox="0 0 32 32" fill="none" aria-hidden>
        <defs>
          <linearGradient id="logo-grad" x1="0" y1="0" x2="32" y2="32" gradientUnits="userSpaceOnUse">
            <stop offset="0%" stopColor="#3567f6" />
            <stop offset="100%" stopColor="#1c3bb4" />
          </linearGradient>
        </defs>
        <rect width="32" height="32" rx="8" fill="url(#logo-grad)" />
        <path d="M9 9h5v14H9z M16 9h7v8h-7z" fill="white" fillOpacity="0.96" />
        <circle cx="22" cy="22" r="2" fill="white" fillOpacity="0.95" />
      </svg>
      {withWordmark && (
        <div className="leading-tight">
          <div className="text-[15px] font-bold tracking-tight text-ink-900">AI 标书系统</div>
          <div className="text-[10px] uppercase tracking-wider text-ink-400 font-medium">Bid Composer</div>
        </div>
      )}
    </div>
  )
}
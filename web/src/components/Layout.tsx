import { Outlet, Link, useLocation, useNavigate } from 'react-router-dom'
import { useAuthStore } from '../lib/auth'
import { ThemeToggle } from './ThemeToggle'
import { CommandPalette, useCommandPaletteHotkey, useCommandPalette, Command } from './CommandPalette'
import { ReactNode, useMemo } from 'react'

interface NavItem {
  path: string
  label: string
  icon: ReactNode
  description?: string
}

const navItems: NavItem[] = [
  {
    path: '/bids',
    label: '标书管理',
    description: '创建与管理工作流',
    icon: (
      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
        <polyline points="14 2 14 8 20 8" />
        <line x1="9" y1="13" x2="15" y2="13" />
        <line x1="9" y1="17" x2="13" y2="17" />
      </svg>
    ),
  },
  {
    path: '/knowledge',
    label: '知识库',
    description: '素材与案例管理',
    icon: (
      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20" />
        <path d="M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2z" />
      </svg>
    ),
  },
  {
    path: '/settings',
    label: '设置',
    description: '偏好与账户',
    icon: (
      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <circle cx="12" cy="12" r="3" />
        <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 1 1-4 0v-.09a1.65 1.65 0 0 0-1-1.51 1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 1 1 0-4h.09a1.65 1.65 0 0 0 1.51-1 1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33h0a1.65 1.65 0 0 0 1-1.51V3a2 2 0 1 1 4 0v.09a1.65 1.65 0 0 0 1 1.51h0a1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82v0a1.65 1.65 0 0 0 1.51 1H21a2 2 0 1 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" />
      </svg>
    ),
  },
]

export default function Layout() {
  const location = useLocation()
  const navigate = useNavigate()
  const { logout, userId, tenantId } = useAuthStore()

  // Command palette commands
  useCommandPaletteHotkey()
  const commands: Command[] = useMemo(() => [
    {
      id: 'nav-bids', group: '导航', label: '标书管理', description: '查看与创建标书',
      keywords: ['bids', '标书', 'tender'],
      perform: () => navigate('/bids'),
    },
    {
      id: 'nav-knowledge', group: '导航', label: '知识库', description: '管理素材与案例',
      keywords: ['knowledge', 'kb', '知识'],
      perform: () => navigate('/knowledge'),
    },
    {
      id: 'nav-settings', group: '导航', label: '设置', description: '偏好与账户',
      keywords: ['settings', 'preferences', '设置'],
      perform: () => navigate('/settings'),
    },
    {
      id: 'palette-toggle-theme', group: '外观', label: '切换主题', description: '浅色 / 深色 / 跟随系统',
      keywords: ['theme', 'dark', 'light', '主题'],
      shortcut: '⌘K',
      perform: () => useCommandPalette.getState().setOpen(false),
    },
    {
      id: 'logout', group: '账户', label: '退出登录',
      keywords: ['logout', 'sign out', '退出'],
      perform: () => { logout(); navigate('/login') },
    },
  ], [navigate, logout])

  const initials = (userId || 'U').slice(0, 1).toUpperCase()

  return (
    <div className="min-h-screen flex bg-ink-50">
      {/* Sidebar */}
      <aside className="w-64 shrink-0 bg-ink-900 text-white flex flex-col">
        {/* Brand */}
        <div className="px-5 py-5 border-b border-white/5">
          <div className="flex items-center gap-2.5">
            <svg width="32" height="32" viewBox="0 0 32 32" fill="none">
              <defs>
                <linearGradient id="sidebar-grad" x1="0" y1="0" x2="32" y2="32" gradientUnits="userSpaceOnUse">
                  <stop offset="0%" stopColor="#5b8bff" />
                  <stop offset="100%" stopColor="#1c3bb4" />
                </linearGradient>
              </defs>
              <rect width="32" height="32" rx="8" fill="url(#sidebar-grad)" />
              <path d="M9 9h5v14H9z M16 9h7v8h-7z" fill="white" fillOpacity="0.96" />
              <circle cx="22" cy="22" r="2" fill="white" fillOpacity="0.95" />
            </svg>
            <div className="leading-tight">
              <div className="text-sm font-bold tracking-tight">AI 标书系统</div>
              <div className="text-[10px] uppercase tracking-wider text-white/40 font-medium">Bid Composer</div>
            </div>
          </div>
        </div>

        {/* Nav */}
        <nav className="flex-1 px-3 py-4 space-y-1 overflow-y-auto scrollbar-thin">
          <button
            onClick={() => useCommandPalette.getState().setOpen(true)}
            className="w-full mb-3 flex items-center gap-2 px-2.5 py-1.5 text-xs text-white/40 hover:text-white/80 bg-white/5 hover:bg-white/10 border border-white/5 rounded-lg transition-colors"
          >
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <circle cx="11" cy="11" r="8" />
              <line x1="21" y1="21" x2="16.65" y2="16.65" />
            </svg>
            <span className="flex-1 text-left">搜索命令…</span>
            <kbd className="text-[10px] font-mono px-1 py-0.5 rounded bg-white/10 text-white/40">⌘K</kbd>
          </button>
          <div className="px-2 mb-2 text-[10px] uppercase tracking-wider text-white/30 font-semibold">主菜单</div>
          {navItems.map((item) => {
            const active = location.pathname.startsWith(item.path)
            return (
              <Link
                key={item.path}
                to={item.path}
                className={[
                  'group flex items-start gap-3 px-3 py-2.5 rounded-xl text-sm transition-all',
                  active
                    ? 'bg-brand-600 text-white shadow-pop'
                    : 'text-white/70 hover:text-white hover:bg-white/5',
                ].join(' ')}
              >
                <span className={[
                  'shrink-0 mt-0.5 transition-colors',
                  active ? 'text-white' : 'text-white/50 group-hover:text-white/80',
                ].join(' ')}>
                  {item.icon}
                </span>
                <span className="leading-tight">
                  <span className="block font-medium">{item.label}</span>
                  {item.description && (
                    <span className={[
                      'block text-[11px] mt-0.5',
                      active ? 'text-white/80' : 'text-white/40',
                    ].join(' ')}>
                      {item.description}
                    </span>
                  )}
                </span>
              </Link>
            )
          })}
        </nav>

        {/* Footer / User */}
        <div className="p-3 border-t border-white/5 space-y-2">
          <div className="flex items-center justify-end px-1">
            <ThemeToggle variant="cycle" surface="dark" />
          </div>
          <div className="flex items-center gap-3 px-2 py-2 rounded-lg">
            <div className="w-9 h-9 rounded-full bg-gradient-to-br from-brand-400 to-brand-700 flex items-center justify-center text-sm font-semibold text-white shrink-0">
              {initials}
            </div>
            <div className="min-w-0 flex-1">
              <div className="text-sm font-medium truncate">{userId || '未登录'}</div>
              <div className="text-[11px] text-white/40 truncate">{tenantId ? `租户 ${tenantId}` : ''}</div>
            </div>
          </div>
          <button
            onClick={() => { logout(); navigate('/login') }}
            className="w-full flex items-center justify-center gap-2 px-3 py-2 text-xs text-white/60 hover:text-white hover:bg-white/5 rounded-lg transition-colors"
          >
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4" />
              <polyline points="16 17 21 12 16 7" />
              <line x1="21" y1="12" x2="9" y2="12" />
            </svg>
            <span>退出登录</span>
          </button>
        </div>
      </aside>

      {/* Main */}
      <main className="flex-1 min-w-0 flex flex-col overflow-hidden">
        <Outlet />
      </main>

      {/* Global command palette */}
      <CommandPalette commands={commands} />
    </div>
  )
}
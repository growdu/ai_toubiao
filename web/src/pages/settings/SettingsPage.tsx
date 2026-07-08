import { useState } from 'react'
import { useAuthStore } from '../../lib/auth'
import { useThemeStore, ThemeMode } from '../../lib/theme'
import { Card, StatusBadge, Tabs } from '../../components/ui'
import { ThemeToggle } from '../../components/ThemeToggle'
import { usePageMeta } from '../../lib/usePageMeta'

type SettingsTab = 'account' | 'appearance' | 'notifications' | 'about'

export default function SettingsPage() {
  usePageMeta({
    title: '设置',
    description: '管理账号、通知偏好、主题与系统信息。',
    noindex: true,
  })

  const { userId, tenantId, token } = useAuthStore()
  const mode = useThemeStore(s => s.mode)
  const resolved = useThemeStore(s => s.resolved)
  const setMode = useThemeStore(s => s.setMode)
  const [tab, setTab] = useState<SettingsTab>('appearance')

  return (
    <div className="flex-1 overflow-y-auto scrollbar-thin bg-ink-50 dark:bg-ink-900">
      <div className="max-w-3xl mx-auto px-6 py-8">
        <div className="mb-6 animate-fade-in">
          <div className="text-xs font-medium uppercase tracking-wider text-brand-600 dark:text-brand-400 mb-1">偏好</div>
          <h1 className="text-2xl font-bold text-ink-900 dark:text-white tracking-tight">设置</h1>
          <p className="text-sm text-ink-500 dark:text-ink-400 mt-1">账户信息、主题、租户与通知偏好</p>
        </div>

        <Tabs
          variant="underline"
          value={tab}
          onChange={(v) => setTab(v as SettingsTab)}
          items={[
            { id: 'appearance', label: '外观', icon: <AppearanceIcon /> },
            { id: 'account', label: '账户', icon: <AccountIcon /> },
            { id: 'notifications', label: '通知', icon: <BellIcon /> },
            { id: 'about', label: '关于', icon: <InfoIcon /> },
          ]}
          className="mb-5 animate-slide-up"
        />

        <div className="animate-slide-up">
          {/* Appearance */}
          {tab === 'appearance' && (
            <Card padded={false}>
              <div className="px-5 py-4 border-b border-ink-100 dark:border-ink-700 flex items-center justify-between">
                <div>
                  <h2 className="text-sm font-semibold text-ink-800 dark:text-ink-100">主题外观</h2>
                  <p className="text-xs text-ink-500 dark:text-ink-400 mt-0.5">选择浅色、深色或跟随系统</p>
                </div>
                <ThemeToggle variant="menu" />
              </div>
              <div className="p-5">
                <div className="grid grid-cols-3 gap-3">
                  {(['light', 'dark', 'system'] as ThemeMode[]).map(m => (
                    <button
                      key={m}
                      onClick={() => setMode(m)}
                      className={[
                        'group relative p-4 rounded-xl border-2 text-center transition-all overflow-hidden',
                        mode === m
                          ? 'border-brand-400 bg-brand-50 dark:bg-brand-900/30 shadow-pop -translate-y-0.5'
                          : 'border-ink-200 dark:border-ink-700 text-ink-600 dark:text-ink-300 hover:border-ink-300 dark:hover:border-ink-600 hover:-translate-y-0.5',
                      ].join(' ')}
                    >
                      {/* mini preview */}
                      <div className={[
                        'mb-3 h-16 rounded-lg overflow-hidden border',
                        m === 'dark' ? 'bg-ink-900 border-ink-700'
                          : m === 'light' ? 'bg-white border-ink-200'
                          : 'bg-gradient-to-br from-white to-ink-900 border-ink-300',
                      ].join(' ')}>
                        <div className="flex h-full">
                          <div className={[
                            'w-1/4',
                            m === 'dark' ? 'bg-ink-800' : m === 'light' ? 'bg-ink-100' : 'bg-ink-300',
                          ].join(' ')} />
                          <div className="flex-1 p-1.5 space-y-1">
                            <div className={[
                              'h-1.5 rounded w-2/3',
                              m === 'dark' ? 'bg-ink-600' : 'bg-ink-300',
                            ].join(' ')} />
                            <div className={[
                              'h-1 rounded w-1/2',
                              m === 'dark' ? 'bg-ink-700' : 'bg-ink-200',
                            ].join(' ')} />
                          </div>
                        </div>
                      </div>
                      <div className="text-xs font-medium mb-0.5">
                        {m === 'light' ? '浅色' : m === 'dark' ? '深色' : '跟随系统'}
                      </div>
                      <div className="text-[10px] text-ink-400 dark:text-ink-500">
                        {m === 'light' ? '明亮界面' : m === 'dark' ? '暗色护眼' : `当前 ${resolved === 'dark' ? '深色' : '浅色'}`}
                      </div>
                      {mode === m && (
                        <div className="absolute top-2 right-2 w-5 h-5 rounded-full bg-brand-600 text-white grid place-items-center text-[10px] shadow-pop">
                          ✓
                        </div>
                      )}
                    </button>
                  ))}
                </div>
              </div>
            </Card>
          )}

          {/* Account */}
          {tab === 'account' && (
            <Card>
              <h2 className="text-sm font-semibold text-ink-800 dark:text-ink-100 mb-4 pb-3 border-b border-ink-100 dark:border-ink-700 inline-flex items-center gap-2">
                <AccountIcon />
                账户信息
              </h2>
              <dl className="space-y-3 text-sm">
                <Info k="用户 ID" v={<span className="font-mono text-ink-700 dark:text-ink-300">{userId || '-'}</span>} />
                <Info k="租户 ID" v={<span className="font-mono text-ink-700 dark:text-ink-300">{tenantId || '-'}</span>} />
                <Info
                  k="登录状态"
                  v={token
                    ? <StatusBadge status="succeeded" showDot labels={{ succeeded: '已登录' }} />
                    : <StatusBadge status="failed" showDot labels={{ failed: '未登录' }} />}
                />
              </dl>
            </Card>
          )}

          {/* Notifications */}
          {tab === 'notifications' && (
            <Card>
              <h2 className="text-sm font-semibold text-ink-800 dark:text-ink-100 mb-4 pb-3 border-b border-ink-100 dark:border-ink-700 inline-flex items-center gap-2">
                <BellIcon />
                通知偏好
              </h2>
              <div className="flex items-start gap-3 p-4 rounded-xl bg-brand-50/50 dark:bg-brand-900/20 border border-brand-100 dark:border-brand-900/40">
                <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="text-brand-600 dark:text-brand-400 shrink-0 mt-0.5">
                  <circle cx="12" cy="12" r="10" />
                  <line x1="12" y1="16" x2="12" y2="12" />
                  <line x1="12" y1="8" x2="12.01" y2="8" />
                </svg>
                <div className="flex-1">
                  <p className="text-sm text-ink-700 dark:text-ink-300 leading-relaxed">
                    邮件、钉钉、企业微信等渠道的订阅设置位于
                    <span className="font-mono text-ink-900 dark:text-white mx-1">通知服务 (notify-svc)</span>，
                    当前 UI 版本暂未开放配置入口。
                  </p>
                  <p className="text-xs text-ink-500 dark:text-ink-400 mt-2">
                    如需调整，请在 API 层面修改 <span className="font-mono">notification_preferences</span>。
                  </p>
                </div>
              </div>
            </Card>
          )}

          {/* About */}
          {tab === 'about' && (
            <Card>
              <h2 className="text-sm font-semibold text-ink-800 dark:text-ink-100 mb-4 pb-3 border-b border-ink-100 dark:border-ink-700 inline-flex items-center gap-2">
                <InfoIcon />
                关于
              </h2>
              <dl className="space-y-3 text-sm">
                <Info k="系统版本" v={<span className="font-mono text-ink-700 dark:text-ink-300">v0.1.0 · MVP</span>} />
                <Info k="前端栈" v={<span className="text-ink-600 dark:text-ink-300">React 18 · TypeScript · Vite · Tailwind</span>} />
                <Info k="后端栈" v={<span className="text-ink-600 dark:text-ink-300">Go 1.22 · Gin · pgvector · Asynq</span>} />
                <Info k="许可证" v={<span className="font-mono text-ink-700 dark:text-ink-300">AGPL-3.0</span>} />
              </dl>
            </Card>
          )}
        </div>
      </div>
    </div>
  )
}

function Info({ k, v }: { k: string; v: React.ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-4 py-2 px-3 -mx-3 rounded hover:bg-ink-50 dark:hover:bg-ink-800 transition-colors">
      <dt className="text-ink-500 dark:text-ink-400 text-xs">{k}</dt>
      <dd className="text-sm">{v}</dd>
    </div>
  )
}

function AppearanceIcon() { return <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><circle cx="12" cy="12" r="5" /><line x1="12" y1="1" x2="12" y2="3" /><line x1="12" y1="21" x2="12" y2="23" /><line x1="4.22" y1="4.22" x2="5.64" y2="5.64" /><line x1="18.36" y1="18.36" x2="19.78" y2="19.78" /><line x1="1" y1="12" x2="3" y2="12" /><line x1="21" y1="12" x2="23" y2="12" /><line x1="4.22" y1="19.78" x2="5.64" y2="18.36" /><line x1="18.36" y1="5.64" x2="19.78" y2="4.22" /></svg> }
function AccountIcon()    { return <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2" /><circle cx="12" cy="7" r="4" /></svg> }
function BellIcon()       { return <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9" /><path d="M13.73 21a2 2 0 0 1-3.46 0" /></svg> }
function InfoIcon()       { return <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><circle cx="12" cy="12" r="10" /><line x1="12" y1="16" x2="12" y2="12" /><line x1="12" y1="8" x2="12.01" y2="8" /></svg> }
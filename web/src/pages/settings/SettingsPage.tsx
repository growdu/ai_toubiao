import { useAuthStore } from '../../lib/auth'
import { useThemeStore, ThemeMode } from '../../lib/theme'
import { Card, StatusBadge } from '../../components/ui'
import { ThemeToggle } from '../../components/ThemeToggle'

export default function SettingsPage() {
  const { userId, tenantId, token } = useAuthStore()
  const mode = useThemeStore(s => s.mode)
  const resolved = useThemeStore(s => s.resolved)
  const setMode = useThemeStore(s => s.setMode)

  return (
    <div className="flex-1 overflow-y-auto scrollbar-thin">
      <div className="max-w-3xl mx-auto px-6 py-8">
        <div className="mb-6 animate-fade-in">
          <div className="text-xs font-medium uppercase tracking-wider text-brand-600 mb-1">偏好</div>
          <h1 className="text-2xl font-bold text-ink-900 dark:text-white tracking-tight">设置</h1>
          <p className="text-sm text-ink-500 mt-1">账户信息、主题、租户与通知偏好</p>
        </div>

        <div className="space-y-4 animate-slide-up">
          {/* Theme */}
          <Card>
            <div className="flex items-center justify-between mb-3 pb-3 border-b border-ink-100 dark:border-ink-700">
              <h2 className="text-sm font-semibold text-ink-800 dark:text-ink-100">主题</h2>
              <ThemeToggle variant="menu" />
            </div>
            <div className="grid grid-cols-3 gap-2">
              {(['light', 'dark', 'system'] as ThemeMode[]).map(m => (
                <button
                  key={m}
                  onClick={() => setMode(m)}
                  className={[
                    'p-3 rounded-xl border text-center transition-all',
                    mode === m
                      ? 'border-brand-400 bg-brand-50 text-brand-700 dark:bg-brand-900 dark:border-brand-500 dark:text-brand-200 shadow-sm'
                      : 'border-ink-200 dark:border-ink-700 text-ink-600 dark:text-ink-300 hover:border-ink-300 dark:hover:border-ink-600',
                  ].join(' ')}
                >
                  <div className="text-xs font-medium mb-1">
                    {m === 'light' ? '浅色' : m === 'dark' ? '深色' : '跟随系统'}
                  </div>
                  <div className="text-[10px] text-ink-400">
                    {m === 'light' ? '明亮界面' : m === 'dark' ? '暗色护眼' : `当前 ${resolved === 'dark' ? '深色' : '浅色'}`}
                  </div>
                </button>
              ))}
            </div>
          </Card>

          {/* Account */}
          <Card>
            <h2 className="text-sm font-semibold text-ink-800 dark:text-ink-100 mb-4 pb-3 border-b border-ink-100 dark:border-ink-700">账户信息</h2>
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

          {/* Notifications */}
          <Card>
            <h2 className="text-sm font-semibold text-ink-800 dark:text-ink-100 mb-4 pb-3 border-b border-ink-100 dark:border-ink-700">通知偏好</h2>
            <p className="text-xs text-ink-500 leading-relaxed">
              邮件、钉钉、企业微信等渠道的订阅设置位于 <span className="font-mono text-ink-700 dark:text-ink-300">通知服务 (notify-svc)</span>，
              当前 UI 版本暂未开放配置入口。如需调整，请在 API 层面修改 <span className="font-mono">notification_preferences</span>。
            </p>
          </Card>

          {/* About */}
          <Card>
            <h2 className="text-sm font-semibold text-ink-800 dark:text-ink-100 mb-4 pb-3 border-b border-ink-100 dark:border-ink-700">关于</h2>
            <dl className="space-y-3 text-sm">
              <Info k="系统版本" v={<span className="font-mono text-ink-700 dark:text-ink-300">v0.1.0 · MVP</span>} />
              <Info k="前端栈" v={<span className="text-ink-600 dark:text-ink-300">React 18 · TypeScript · Vite · Tailwind</span>} />
              <Info k="后端栈" v={<span className="text-ink-600 dark:text-ink-300">Go 1.22 · Gin · pgvector · Asynq</span>} />
            </dl>
          </Card>
        </div>
      </div>
    </div>
  )
}

function Info({ k, v }: { k: string; v: React.ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-4">
      <dt className="text-ink-500 text-xs">{k}</dt>
      <dd className="text-sm">{v}</dd>
    </div>
  )
}
import { useState, FormEvent, useRef, useMemo } from 'react'
import { useNavigate, Link } from 'react-router-dom'
import { authApi } from '../../api/auth'
import { useAuthStore } from '../../lib/auth'
import { Button, TextInput } from '../../components/ui'
import { usePageMeta } from '../../lib/usePageMeta'

// Quick-fill demo accounts. The seed bcrypt for these is `password123`
// (see db/seed_dev.sql). Surfacing them as one-click buttons trims 30s
// of "what was the password again?" friction during demos.
const DEMO_ACCOUNTS = [
  { tenant: 'demo-a', email: 'admin@demo-a.test',   label: 'Admin · demo-a',   tone: 'brand' },
  { tenant: 'demo-a', email: 'member@demo-a.test', label: 'Member · demo-a',  tone: 'emerald' },
  { tenant: 'demo-b', email: 'admin@demo-b.test',  label: 'Admin · demo-b',   tone: 'amber' },
] as const

function getPasswordStrength(p: string): { score: 0 | 1 | 2 | 3 | 4; label: string; color: string } {
  if (!p) return { score: 0, label: '请输入密码', color: 'bg-ink-200' }
  let s: 0 | 1 | 2 | 3 | 4 = 1
  if (p.length >= 8) s = (s + 1) as 0 | 1 | 2 | 3 | 4
  if (/[A-Z]/.test(p) && /[a-z]/.test(p)) s = (s + 1) as 0 | 1 | 2 | 3 | 4
  if (/\d/.test(p)) s = (s + 1) as 0 | 1 | 2 | 3 | 4
  if (/[^A-Za-z0-9]/.test(p)) s = (s + 1) as 0 | 1 | 2 | 3 | 4
  const meta: Record<number, { label: string; color: string }> = {
    1: { label: '弱',   color: 'bg-red-500' },
    2: { label: '一般', color: 'bg-amber-500' },
    3: { label: '良好', color: 'bg-brand-500' },
    4: { label: '强',   color: 'bg-emerald-500' },
  }
  return { score: s, ...(meta[s] ?? meta[1]) }
}

export default function LoginPage() {
  usePageMeta({
    title: '登录',
    description: '登录 BidWriter 工作区，继续你的 AI 标书编制流程。',
    noindex: true,
  })

  const navigate = useNavigate()
  const { setAuth } = useAuthStore()
  const [tenantSlug, setTenantSlug] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  // Counter incremented on every failed submit so the wrapper <form>
  // re-runs its key-based animation. Reset on the next successful submit.
  const [shakeKey, setShakeKey] = useState(0)
  const passwordRef = useRef<HTMLInputElement | null>(null)

  const strength = useMemo(() => getPasswordStrength(password), [password])

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)
    try {
      const res = await authApi.login({ tenant_slug: tenantSlug, email, password })
      const { token, userId, tenantId } = authApi.extractAuthInfo(res)
      if (!token) {
        setError('登录返回无效，未收到 token')
        setShakeKey(k => k + 1)
        return
      }
      setAuth(token, userId, tenantId)
      navigate('/bids')
    } catch (err: any) {
      setError(err.response?.data?.message || err.response?.data?.error?.message || '登录失败')
      setShakeKey(k => k + 1)
      // Pull focus back to password so the user can immediately correct it.
      // Don't fire during loading transitions where the input is unmounted.
      setTimeout(() => passwordRef.current?.focus(), 0)
    } finally {
      setLoading(false)
    }
  }

  const fillDemo = (acc: typeof DEMO_ACCOUNTS[number]) => {
    setTenantSlug(acc.tenant)
    setEmail(acc.email)
    setPassword('password123')
    setError('')
  }

  return (
    <div className="min-h-screen flex bg-ink-50 dark:bg-ink-900">
      {/* Brand panel */}
      <div className="hidden lg:flex lg:w-1/2 xl:w-[55%] relative overflow-hidden bg-brand-gradient text-white">
        <div className="absolute inset-0 bg-mesh-1 opacity-90" />
        <div className="absolute inset-0 opacity-[0.07]" style={{
          backgroundImage: 'radial-gradient(rgba(255,255,255,0.6) 1px, transparent 1px)',
          backgroundSize: '24px 24px',
        }} />
        {/* Floating orbs */}
        <div className="absolute top-1/3 left-1/4 w-48 h-48 rounded-full bg-violet-400/20 blur-3xl animate-pulse-soft" />
        <div className="absolute bottom-1/4 right-1/4 w-56 h-56 rounded-full bg-emerald-400/20 blur-3xl animate-pulse-soft" style={{ animationDelay: '1.5s' }} />

        <div className="relative z-10 flex flex-col justify-between p-12 w-full">
          {/* Logo */}
          <Link to="/" className="flex items-center gap-3 w-fit">
            <svg width="36" height="36" viewBox="0 0 32 32" fill="none">
              <rect width="32" height="32" rx="8" fill="white" fillOpacity="0.18" />
              <path d="M9 9h5v14H9z M16 9h7v8h-7z" fill="white" />
              <circle cx="22" cy="22" r="2" fill="white" />
            </svg>
            <div className="leading-tight">
              <div className="text-base font-bold tracking-tight">AI 标书系统</div>
              <div className="text-[11px] uppercase tracking-wider text-white/60 font-medium">Bid Composer</div>
            </div>
          </Link>

          {/* Hero copy */}
          <div className="max-w-lg animate-slide-up">
            <div className="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-white/15 backdrop-blur-sm border border-white/10 text-xs font-medium mb-6">
              <span className="relative flex w-2 h-2">
                <span className="absolute inline-flex w-full h-full rounded-full bg-emerald-300 opacity-75 animate-ping-slow" />
                <span className="relative inline-flex rounded-full w-2 h-2 bg-emerald-300" />
              </span>
              <span>AI 驱动 · 多模型路由 · 人在回路</span>
            </div>
            <h1 className="text-4xl xl:text-5xl font-bold leading-tight tracking-tight mb-4">
              让标书编制<br />
              <span className="bg-gradient-to-r from-brand-100 via-white to-emerald-200 bg-clip-text text-transparent bg-[length:200%_100%] animate-gradient-x">
                快 10 倍，准确 100%
              </span>
            </h1>
            <p className="text-base xl:text-lg text-white/80 leading-relaxed mb-8">
              自动解析招标文件、生成章节大纲与内容、检索企业知识库作为证据链、
              一致性与合规审计，全流程留痕、可追溯、可信交付。
            </p>

            <div className="grid grid-cols-3 gap-3 max-w-md">
              <Feature icon="📄" title="智能大纲" hint="AI 生成结构" />
              <Feature icon="🔍" title="证据检索" hint="RAG + 向量" />
              <Feature icon="✅" title="合规审计" hint="废标项扫描" />
            </div>

            {/* Trust line */}
            <div className="mt-10 flex items-center gap-6 text-xs text-white/60">
              <div className="flex items-center gap-1.5">
                <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />
                </svg>
                <span>租户隔离 · 数据安全</span>
              </div>
              <div className="flex items-center gap-1.5">
                <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <circle cx="12" cy="12" r="10" />
                  <polyline points="12 6 12 12 16 14" />
                </svg>
                <span>5 分钟出标书</span>
              </div>
            </div>
          </div>

          <div className="text-xs text-white/50">
            © {new Date().getFullYear()} AI 标书系统 · Powered by 多模型 LLM 路由
          </div>
        </div>
      </div>

      {/* Form panel */}
      <div className="flex-1 flex items-center justify-center p-6 lg:p-12 bg-ink-50 dark:bg-ink-900">
        <div className="w-full max-w-sm animate-slide-up">
          {/* Mobile brand */}
          <div className="lg:hidden flex items-center justify-center gap-2 mb-8">
            <svg width="32" height="32" viewBox="0 0 32 32" fill="none">
              <rect width="32" height="32" rx="8" fill="#224be0" />
              <path d="M9 9h5v14H9z M16 9h7v8h-7z" fill="white" />
            </svg>
            <span className="text-lg font-bold dark:text-white">AI 标书系统</span>
          </div>

          <h2 className="text-2xl font-bold text-ink-900 dark:text-white mb-1">欢迎回来</h2>
          <p className="text-sm text-ink-500 dark:text-ink-400 mb-6">使用您的租户账号登录系统</p>

          {/* Demo accounts quick-fill */}
          <div className="mb-5 p-3 rounded-xl bg-ink-50 dark:bg-ink-800 border border-dashed border-ink-200 dark:border-ink-700">
            <div className="flex items-center gap-1.5 text-[10px] uppercase tracking-wider text-ink-500 dark:text-ink-400 font-semibold mb-2">
              <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <path d="M13 2L3 14h9l-1 8 10-12h-9l1-8z" />
              </svg>
              一键填充演示账号
            </div>
            <div className="flex flex-wrap gap-1.5">
              {DEMO_ACCOUNTS.map(acc => {
                const toneClass = {
                  brand:   'bg-brand-50 text-brand-700 hover:bg-brand-100 dark:bg-brand-900/40 dark:text-brand-300 dark:hover:bg-brand-900/60',
                  emerald: 'bg-emerald-50 text-emerald-700 hover:bg-emerald-100 dark:bg-emerald-900/40 dark:text-emerald-300 dark:hover:bg-emerald-900/60',
                  amber:   'bg-amber-50 text-amber-700 hover:bg-amber-100 dark:bg-amber-900/40 dark:text-amber-300 dark:hover:bg-amber-900/60',
                }[acc.tone]
                const dotClass = {
                  brand:   'bg-brand-500',
                  emerald: 'bg-emerald-500',
                  amber:   'bg-amber-500',
                }[acc.tone]
                return (
                  <button
                    key={acc.email}
                    type="button"
                    onClick={() => fillDemo(acc)}
                    className={`inline-flex items-center gap-1 px-2 py-1 text-[11px] font-medium rounded-md transition-all hover:scale-105 active:scale-95 ${toneClass}`}
                  >
                    <span className={`w-1.5 h-1.5 rounded-full ${dotClass}`} />
                    {acc.label}
                  </button>
                )
              })}
            </div>
            <p className="text-[10px] text-ink-400 dark:text-ink-500 mt-2">默认密码：<span className="font-mono">password123</span></p>
          </div>

          <form key={shakeKey} onSubmit={handleSubmit} className="space-y-4">
            <TextInput
              label="租户标识"
              value={tenantSlug}
              onChange={(e) => setTenantSlug(e.target.value)}
              placeholder="demo-a"
              required
              autoComplete="organization"
              leftIcon={<TenantIcon />}
            />
            <TextInput
              label="邮箱"
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="admin@example.com"
              required
              autoComplete="email"
              leftIcon={<MailIcon />}
            />
            <div>
              <TextInput
                ref={passwordRef}
                label="密码"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder="••••••••"
                required
                autoComplete="current-password"
                leftIcon={<LockIcon />}
              />
              {/* Password strength bar — visualises getPasswordStrength() */}
              {password && (
                <div className="mt-2 flex items-center gap-2 animate-fade-in">
                  <div className="flex-1 grid grid-cols-4 gap-1">
                    {[1, 2, 3, 4].map(i => (
                      <div
                        key={i}
                        className={[
                          'h-1 rounded-full transition-colors',
                          strength.score >= i ? strength.color : 'bg-ink-200 dark:bg-ink-700',
                        ].join(' ')}
                      />
                    ))}
                  </div>
                  <span className="text-[11px] text-ink-500 dark:text-ink-400 font-medium w-8">{strength.label}</span>
                </div>
              )}
            </div>

            {error && (
              <div
                role="alert"
                aria-live="assertive"
                data-form-error
                className="flex items-start gap-2.5 px-3 py-2.5 bg-red-50 dark:bg-red-900/30 border-2 border-red-200 dark:border-red-800 rounded-lg text-sm text-red-700 dark:text-red-300 animate-shake"
              >
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="shrink-0 mt-0.5 text-red-500">
                  <circle cx="12" cy="12" r="10" />
                  <line x1="12" y1="8" x2="12" y2="12" />
                  <line x1="12" y1="16" x2="12.01" y2="16" />
                </svg>
                <div className="flex-1 leading-snug">
                  <div className="font-semibold text-red-800 dark:text-red-200">登录失败</div>
                  <div className="text-red-700 dark:text-red-300">{error}</div>
                  {error.includes('密码') || error.includes('邮箱') ? (
                    <div className="mt-1 text-xs text-red-600/80 dark:text-red-400/80">
                      请检查租户标识、邮箱、密码是否正确（默认 demo 账号密码为 password123）
                    </div>
                  ) : null}
                </div>
                <button
                  type="button"
                  onClick={() => setError('')}
                  className="shrink-0 -mr-1 -mt-1 p-1 rounded text-red-400 hover:text-red-600 hover:bg-red-100 dark:hover:bg-red-900/40 transition-colors"
                  aria-label="关闭错误提示"
                >
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round">
                    <line x1="18" y1="6" x2="6" y2="18" />
                    <line x1="6" y1="6" x2="18" y2="18" />
                  </svg>
                </button>
              </div>
            )}

            <Button
              type="submit"
              variant="primary"
              size="lg"
              loading={loading}
              className="w-full mt-2"
            >
              {loading ? '登录中...' : '登录'}
            </Button>

            <div className="text-center pt-2">
              <p className="text-xs text-ink-400 dark:text-ink-500">
                没有账号？
                <Link to="/register" className="ml-1 text-brand-600 dark:text-brand-400 hover:underline font-medium">
                  免费试用 14 天
                </Link>
              </p>
            </div>
          </form>
        </div>
      </div>
    </div>
  )
}

function Feature({ icon, title, hint }: { icon: string; title: string; hint: string }) {
  return (
    <div className="px-3 py-2.5 rounded-xl bg-white/10 backdrop-blur-sm border border-white/15 hover:bg-white/15 transition-colors">
      <div className="text-xl mb-1">{icon}</div>
      <div className="text-sm font-semibold leading-tight">{title}</div>
      <div className="text-[11px] text-white/65">{hint}</div>
    </div>
  )
}

function TenantIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="text-ink-400">
      <path d="M3 9l9-7 9 7v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z" />
      <polyline points="9 22 9 12 15 12 15 22" />
    </svg>
  )
}

function MailIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="text-ink-400">
      <path d="M4 4h16c1.1 0 2 .9 2 2v12c0 1.1-.9 2-2 2H4c-1.1 0-2-.9-2-2V6c0-1.1.9-2 2-2z" />
      <polyline points="22,6 12,13 2,6" />
    </svg>
  )
}

function LockIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="text-ink-400">
      <rect x="3" y="11" width="18" height="11" rx="2" ry="2" />
      <path d="M7 11V7a5 5 0 0 1 10 0v4" />
    </svg>
  )
}
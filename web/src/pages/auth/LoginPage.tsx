import { useState, FormEvent, useRef } from 'react'
import { useNavigate, Link } from 'react-router-dom'
import { authApi } from '../../api/auth'
import { useAuthStore } from '../../lib/auth'
import { Button, TextInput } from '../../components/ui'
import { usePageMeta } from '../../lib/usePageMeta'

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

  return (
    <div className="min-h-screen flex bg-ink-50">
      {/* Brand panel */}
      <div className="hidden lg:flex lg:w-1/2 xl:w-[55%] relative overflow-hidden bg-brand-gradient text-white">
        <div className="absolute inset-0 bg-mesh-1 opacity-90" />
        <div className="absolute inset-0 opacity-[0.07]" style={{
          backgroundImage: 'radial-gradient(rgba(255,255,255,0.6) 1px, transparent 1px)',
          backgroundSize: '24px 24px',
        }} />

        <div className="relative z-10 flex flex-col justify-between p-12 w-full">
          {/* Logo */}
          <div className="flex items-center gap-3">
            <svg width="36" height="36" viewBox="0 0 32 32" fill="none">
              <rect width="32" height="32" rx="8" fill="white" fillOpacity="0.18" />
              <path d="M9 9h5v14H9z M16 9h7v8h-7z" fill="white" />
              <circle cx="22" cy="22" r="2" fill="white" />
            </svg>
            <div className="leading-tight">
              <div className="text-base font-bold tracking-tight">AI 标书系统</div>
              <div className="text-[11px] uppercase tracking-wider text-white/60 font-medium">Bid Composer</div>
            </div>
          </div>

          {/* Hero copy */}
          <div className="max-w-lg animate-slide-up">
            <div className="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-white/15 backdrop-blur-sm text-xs font-medium mb-6">
              <span className="w-1.5 h-1.5 rounded-full bg-emerald-300 animate-pulse-soft" />
              <span>AI 驱动 · 多模型路由 · 人在回路</span>
            </div>
            <h1 className="text-4xl xl:text-5xl font-bold leading-tight tracking-tight mb-4">
              让标书编制<br />
              <span className="text-brand-100">快 10 倍，准确 100%</span>
            </h1>
            <p className="text-base xl:text-lg text-white/80 leading-relaxed mb-8">
              自动解析招标文件、生成章节大纲与内容、检索企业知识库作为证据链、
              一致性与合规审计，全流程留痕、可追溯、可信交付。
            </p>

            <div className="grid grid-cols-3 gap-4 max-w-md">
              <Feature icon="📄" title="智能大纲" hint="AI 生成结构" />
              <Feature icon="🔍" title="证据检索" hint="RAG + 向量" />
              <Feature icon="✅" title="合规审计" hint="废标项扫描" />
            </div>
          </div>

          <div className="text-xs text-white/50">
            © {new Date().getFullYear()} AI 标书系统 · Powered by 多模型 LLM 路由
          </div>
        </div>
      </div>

      {/* Form panel */}
      <div className="flex-1 flex items-center justify-center p-6 lg:p-12">
        <div className="w-full max-w-sm animate-slide-up">
          {/* Mobile brand */}
          <div className="lg:hidden flex items-center justify-center gap-2 mb-8">
            <svg width="32" height="32" viewBox="0 0 32 32" fill="none">
              <rect width="32" height="32" rx="8" fill="#224be0" />
              <path d="M9 9h5v14H9z M16 9h7v8h-7z" fill="white" />
            </svg>
            <span className="text-lg font-bold">AI 标书系统</span>
          </div>

          <h2 className="text-2xl font-bold text-ink-900 mb-1">欢迎回来</h2>
          <p className="text-sm text-ink-500 mb-8">使用您的租户账号登录系统</p>

          <form key={shakeKey} onSubmit={handleSubmit} className="space-y-4">
            <TextInput
              label="租户标识"
              value={tenantSlug}
              onChange={(e) => setTenantSlug(e.target.value)}
              placeholder="demo-a"
              required
              autoComplete="organization"
            />
            <TextInput
              label="邮箱"
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="admin@example.com"
              required
              autoComplete="email"
            />
            <TextInput
              ref={passwordRef}
              label="密码"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="••••••••"
              required
              autoComplete="current-password"
            />

            {error && (
              <div
                role="alert"
                aria-live="assertive"
                className="flex items-start gap-2.5 px-3 py-2.5 bg-red-50 border-2 border-red-200 rounded-lg text-sm text-red-700 animate-shake"
              >
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="shrink-0 mt-0.5 text-red-500">
                  <circle cx="12" cy="12" r="10" />
                  <line x1="12" y1="8" x2="12" y2="12" />
                  <line x1="12" y1="16" x2="12.01" y2="16" />
                </svg>
                <div className="flex-1 leading-snug">
                  <div className="font-semibold text-red-800">登录失败</div>
                  <div className="text-red-700">{error}</div>
                  {error.includes('密码') || error.includes('邮箱') ? (
                    <div className="mt-1 text-xs text-red-600/80">
                      请检查租户标识、邮箱、密码是否正确（默认 demo 账号密码为 password123）
                    </div>
                  ) : null}
                </div>
                <button
                  type="button"
                  onClick={() => setError('')}
                  className="shrink-0 -mr-1 -mt-1 p-1 rounded text-red-400 hover:text-red-600 hover:bg-red-100 transition-colors"
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
              <p className="text-xs text-ink-400">
                没有账号？
                <Link to="/register" className="ml-1 text-brand-600 hover:underline font-medium">
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
    <div className="px-3 py-2.5 rounded-xl bg-white/10 backdrop-blur-sm border border-white/15">
      <div className="text-xl mb-1">{icon}</div>
      <div className="text-sm font-semibold leading-tight">{title}</div>
      <div className="text-[11px] text-white/65">{hint}</div>
    </div>
  )
}
import { useState, FormEvent } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { Button, TextInput } from '../../components/ui'
import { useAuthStore } from '../../lib/auth'
import { decodeJWT } from '../../lib/jwt'
import api from '../../api/client'

// RegisterPage lets a prospect create a tenant + owner account. On success
// it stores the JWT in the auth store (just like LoginPage does) and
// navigates straight into /bids. Field validation is mirrored from the
// backend so the user sees client-side errors before the round-trip.
//
// Plan (?plan=pro|enterprise|trial) is surfaced in the subtitle and also
// forwarded to the backend as initial_plan; the server decides what to
// actually persist (defaults to "free" if missing/invalid).
export default function RegisterPage() {
  const [params] = useSearchParams()
  const plan = params.get('plan') || 'trial'
  const { setAuth } = useAuthStore()

  const [tenantName, setTenantName] = useState('')
  const [tenantSlug, setTenantSlug] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [agree, setAgree] = useState(false)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [shakeKey, setShakeKey] = useState(0)

  // Auto-derive tenant slug from tenant name as the user types, until
  // they manually edit the slug (then we leave it alone).
  const onTenantNameChange = (v: string) => {
    setTenantName(v)
    if (!tenantSlug || tenantSlug === slugify(tenantName)) {
      setTenantSlug(slugify(v))
    }
  }

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    setError('')

    // Client-side guardrails mirror the backend's validateRegisterInput.
    // The user gets feedback before any round-trip; the server is the
    // source of truth and may still reject if the client is wrong.
    if (!tenantName.trim()) {
      setError('请填写工作区名称')
      setShakeKey(k => k + 1)
      return
    }
    if (!slugRegex.test(tenantSlug)) {
      setError('工作区标识只能包含小写字母、数字、连字符（3-32 位，首尾不能是连字符）')
      setShakeKey(k => k + 1)
      return
    }
    if (password.length < 8) {
      setError('密码至少 8 位')
      setShakeKey(k => k + 1)
      return
    }
    if (!agree) {
      setError('请先同意服务条款和隐私政策')
      setShakeKey(k => k + 1)
      return
    }

    setLoading(true)
    try {
      const res = await api.post<RegisterResponse>('/auth/register', {
        tenant_name: tenantName.trim(),
        tenant_slug: tenantSlug,
        email: email.trim(),
        password,
        initial_plan: plan === 'trial' ? 'free' : plan,
      })
      const { access_token, user } = res.data
      // Decode tenant_id from the JWT — backend doesn't echo it back as a
      // separate field on the user object, but the Register response
      // does include a tenant block. We use that directly here.
      const claims = decodeJWT(access_token)
      setAuth(access_token, user.id, claims.tenant_id as string || '')
      // Navigate to the workspace; using window.location would also
      // work but the in-app router preserves any in-memory state.
      window.location.href = '/bids'
    } catch (err: any) {
      const code = err?.response?.data?.error?.code
      const msg = err?.response?.data?.error?.message || '注册失败，请稍后重试'
      // Map ALREADY_EXISTS messages to the field they apply to so the
      // hint line in the banner can name the right input.
      let hint = ''
      if (code === 'ALREADY_EXISTS' && msg.includes('工作区标识')) {
        hint = '换一个工作区标识再试'
      } else if (code === 'ALREADY_EXISTS' && msg.includes('邮箱')) {
        hint = '可改用其他邮箱，或直接登录'
      }
      setError(hint ? `${msg}（${hint}）` : msg)
      setShakeKey(k => k + 1)
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

          <div className="max-w-lg animate-slide-up">
            <div className="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-white/15 backdrop-blur-sm text-xs font-medium mb-6">
              <span className="w-1.5 h-1.5 rounded-full bg-emerald-300 animate-pulse-soft" />
              <span>14 天免费试用 · 无需信用卡</span>
            </div>
            <h1 className="text-4xl xl:text-5xl font-bold leading-tight tracking-tight mb-4">
              5 分钟<br />开始你的第一份<br />
              <span className="text-brand-100">AI 标书</span>
            </h1>
            <p className="text-base xl:text-lg text-white/80 leading-relaxed mb-8">
              注册账号，创建你的工作区，立即体验 AI 撰写标书的完整流程。
            </p>
            <div className="grid grid-cols-3 gap-4 max-w-md">
              <PlanBadge icon="📄" title="智能大纲" hint="30 秒" />
              <PlanBadge icon="🔍" title="RAG 证据" hint="自动引用" />
              <PlanBadge icon="✅" title="废标扫描" hint="一键检查" />
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
          <div className="lg:hidden flex items-center justify-center gap-2 mb-8">
            <Link to="/" className="flex items-center gap-2">
              <svg width="32" height="32" viewBox="0 0 32 32" fill="none">
                <rect width="32" height="32" rx="8" fill="#224be0" />
                <path d="M9 9h5v14H9z M16 9h7v8h-7z" fill="white" />
              </svg>
              <span className="text-lg font-bold">AI 标书系统</span>
            </Link>
          </div>

          <h2 className="text-2xl font-bold text-ink-900 mb-1">创建账号</h2>
          <p className="text-sm text-ink-500 mb-6">
            {plan === 'pro' && '专业版 · 14 天免费试用'}
            {plan === 'enterprise' && '企业版 · 销售对接'}
            {plan === 'trial' && '免费试用 14 天 · 完整功能'}
          </p>

          <form key={shakeKey} onSubmit={handleSubmit} className="space-y-4">
            <TextInput
              label="工作区名称"
              value={tenantName}
              onChange={(e) => onTenantNameChange(e.target.value)}
              placeholder="如：建工集团投标部"
              required
            />
            <TextInput
              label="工作区标识"
              value={tenantSlug}
              onChange={(e) => setTenantSlug(e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, ''))}
              placeholder="如：jiangong"
              required
              autoComplete="off"
              hint="小写字母、数字、连字符。注册后无法修改。"
            />
            <TextInput
              label="工作邮箱"
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="you@company.com"
              required
              autoComplete="email"
            />
            <TextInput
              label="密码"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="至少 8 位"
              required
              autoComplete="new-password"
              hint="建议包含大小写字母、数字、符号"
            />

            <label className="flex items-start gap-2 text-sm text-ink-600 cursor-pointer pt-1">
              <input
                type="checkbox"
                checked={agree}
                onChange={(e) => setAgree(e.target.checked)}
                className="mt-0.5 rounded border-ink-300 text-brand-600 focus:ring-brand-500"
              />
              <span>
                我已阅读并同意{' '}
                <a href="#" className="text-brand-600 hover:underline">服务条款</a>
                {' '}和{' '}
                <a href="#" className="text-brand-600 hover:underline">隐私政策</a>
              </span>
            </label>

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
                  <div className="font-semibold text-red-800">注册失败</div>
                  <div className="text-red-700">{error}</div>
                </div>
              </div>
            )}

            <Button
              type="submit"
              variant="primary"
              size="lg"
              loading={loading}
              className="w-full mt-2"
            >
              {loading ? '创建中...' : '创建账号并开始试用'}
            </Button>

            <div className="text-center pt-2">
              <p className="text-xs text-ink-500">
                已有账号？{' '}
                <Link to="/login" className="text-brand-600 hover:underline font-medium">
                  直接登录
                </Link>
              </p>
            </div>
          </form>
        </div>
      </div>
    </div>
  )
}

interface RegisterResponse {
  access_token: string
  refresh_token: string
  expires_in: number
  token_type: string
  tenant: { id: string; name: string; slug: string; plan: string }
  user: { id: string; email: string; role: string }
}

function PlanBadge({ icon, title, hint }: { icon: string; title: string; hint: string }) {
  return (
    <div className="px-3 py-2.5 rounded-xl bg-white/10 backdrop-blur-sm border border-white/15">
      <div className="text-xl mb-1">{icon}</div>
      <div className="text-sm font-semibold leading-tight">{title}</div>
      <div className="text-[11px] text-white/65">{hint}</div>
    </div>
  )
}

// slugRegex mirrors the backend's validateRegisterInput regex so the
// client-side error message fires before the round-trip.
const slugRegex = /^[a-z0-9][a-z0-9-]{1,30}[a-z0-9]$/

// slugify converts a tenant name into a URL-safe identifier.
function slugify(s: string): string {
  return s
    .toLowerCase()
    .trim()
    .replace(/[\s_]+/g, '-')
    .replace(/[^a-z0-9\u4e00-\u9fff-]/g, '')
    .replace(/-+/g, '-')
    .replace(/^-|-$/g, '')
    .slice(0, 32)
}

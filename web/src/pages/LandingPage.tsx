import { Link } from 'react-router-dom'
import { Button } from '../components/ui'
import { usePageMeta } from '../lib/usePageMeta'

// LandingPage is the public marketing page. It introduces BidWriter, walks
// through how it works, shows what's included, lists pricing tiers, and
// ends with a CTA. Anyone can reach it; once they sign in we send them
// to /bids (see the App-level redirect).
export default function LandingPage() {
  usePageMeta({
    title: '首页',
    description: '让标书编制快 10 倍、准确 100%。AI 自动生成大纲、检索证据链、撰写章节正文、合规审计，一键导出 Word。',
  })

  return (
    <div className="min-h-screen bg-ink-50 text-ink-900">
      <TopNav />
      <Hero />
      <LogoStrip />
      <FeatureGrid />
      <HowItWorks />
      <UseCases />
      <Pricing />
      <FAQ />
      <FinalCTA />
      <Footer />
    </div>
  )
}

function TopNav() {
  return (
    <header className="sticky top-0 z-30 bg-white/80 backdrop-blur-md border-b border-ink-100">
      <div className="max-w-7xl mx-auto flex items-center justify-between px-6 lg:px-10 h-16">
        <Link to="/" className="flex items-center gap-2.5 group">
          <Logo />
          <div className="leading-tight">
            <div className="text-base font-bold tracking-tight">AI 标书系统</div>
            <div className="text-[11px] uppercase tracking-wider text-ink-400 font-medium">Bid Composer</div>
          </div>
        </Link>
        <nav className="hidden md:flex items-center gap-7 text-sm text-ink-600">
          <a href="#features" className="hover:text-brand-600 transition-colors">功能</a>
          <a href="#how" className="hover:text-brand-600 transition-colors">怎么用</a>
          <a href="#cases" className="hover:text-brand-600 transition-colors">适用场景</a>
          <a href="#pricing" className="hover:text-brand-600 transition-colors">套餐</a>
          <a href="#faq" className="hover:text-brand-600 transition-colors">常见问题</a>
        </nav>
        <div className="flex items-center gap-2">
          <Link to="/login">
            <Button variant="ghost" size="sm">登录</Button>
          </Link>
          <Link to="/register">
            <Button variant="primary" size="sm">免费试用</Button>
          </Link>
        </div>
      </div>
    </header>
  )
}

function Logo() {
  return (
    <svg width="34" height="34" viewBox="0 0 32 32" fill="none" className="shrink-0">
      <rect width="32" height="32" rx="8" fill="url(#bidlogo-grad)" />
      <path d="M9 9h5v14H9z M16 9h7v8h-7z" fill="white" />
      <circle cx="22" cy="22" r="2" fill="white" />
      <defs>
        <linearGradient id="bidlogo-grad" x1="0" y1="0" x2="32" y2="32">
          <stop offset="0%" stopColor="#3567f6" />
          <stop offset="100%" stopColor="#1c3bb4" />
        </linearGradient>
      </defs>
    </svg>
  )
}

function Hero() {
  return (
    <section className="relative overflow-hidden bg-gradient-to-br from-brand-950 via-brand-900 to-brand-700 text-white">
      {/* Animated mesh background */}
      <div className="absolute inset-0 bg-hero-mesh opacity-90" />
      {/* Dot grid overlay */}
      <div className="absolute inset-0 opacity-[0.07]" style={{
        backgroundImage: 'radial-gradient(rgba(255,255,255,0.6) 1px, transparent 1px)',
        backgroundSize: '24px 24px'
      }} />
      {/* Top gradient fade */}
      <div className="absolute top-0 left-0 right-0 h-32 bg-gradient-to-b from-brand-950/60 to-transparent" />

      <div className="relative max-w-7xl mx-auto px-6 lg:px-10 pt-20 pb-24 lg:pt-28 lg:pb-32">
        <div className="grid lg:grid-cols-12 gap-12 items-center">
          <div className="lg:col-span-7">
            <div className="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-white/15 backdrop-blur-sm border border-white/10 text-xs font-medium mb-6 animate-slide-up shadow-inner-ring">
              <span className="relative flex w-2 h-2">
                <span className="absolute inline-flex w-full h-full rounded-full bg-emerald-300 opacity-75 animate-ping-slow" />
                <span className="relative inline-flex rounded-full w-2 h-2 bg-emerald-300" />
              </span>
              <span>2026 全新发布 · 多模型路由 · 人在回路</span>
            </div>
            <h1 className="text-4xl md:text-5xl lg:text-6xl font-bold leading-tight tracking-tight mb-6 animate-slide-up">
              让标书编制<br />
              <span className="bg-gradient-to-r from-brand-100 via-white to-brand-200 bg-clip-text text-transparent bg-[length:200%_100%] animate-gradient-x">
                快 10 倍，准确 100%
              </span>
            </h1>
            <p className="text-lg lg:text-xl text-white/80 leading-relaxed mb-8 max-w-2xl animate-slide-up" style={{ animationDelay: '80ms' }}>
              自动解析招标文件 · 智能生成章节大纲与正文 · 检索企业知识库作为证据链 · 合规审计一键完成。
              告别熬夜改稿，让 AI 帮你写完一份 80 分的标书，你只需做最后的 20 分润色。
            </p>
            <div className="flex flex-col sm:flex-row gap-3 mb-8 animate-slide-up" style={{ animationDelay: '160ms' }}>
              <Link to="/register">
                <Button variant="gradient" size="lg" className="w-full sm:w-auto shadow-glow hover:shadow-glow">
                  免费试用 14 天
                  <span className="ml-1 text-xs font-normal opacity-80">无需信用卡</span>
                </Button>
              </Link>
              <Link to="/login">
                <Button variant="ghost" size="lg" className="!text-white hover:!bg-white/10 w-full sm:w-auto border border-white/15">
                  已有账号 · 登录
                </Button>
              </Link>
            </div>
            <div className="flex flex-wrap items-center gap-x-6 gap-y-2 text-sm text-white/70 animate-slide-up" style={{ animationDelay: '240ms' }}>
              <div className="flex items-center gap-2">
                <Check />
                <span>1 份标书 5 分钟</span>
              </div>
              <div className="flex items-center gap-2">
                <Check />
                <span>废标项自动扫描</span>
              </div>
              <div className="flex items-center gap-2">
                <Check />
                <span>私有部署可选</span>
              </div>
              <div className="flex items-center gap-2">
                <Check />
                <span>数据不出租户</span>
              </div>
            </div>
          </div>

          <div className="lg:col-span-5 relative">
            {/* Mock product preview */}
            <div className="relative rounded-2xl bg-white/10 backdrop-blur-md border border-white/20 p-4 shadow-glow animate-slide-up" style={{ animationDelay: '120ms' }}>
              {/* Glow blob behind preview */}
              <div className="absolute -inset-4 bg-gradient-to-r from-brand-400/30 via-violet-400/20 to-emerald-400/20 rounded-3xl blur-2xl -z-10" />

              <div className="rounded-xl bg-white text-ink-900 overflow-hidden shadow-2xl">
                {/* fake browser chrome */}
                <div className="flex items-center gap-1.5 px-3 py-2 bg-ink-50 border-b border-ink-100">
                  <span className="w-2.5 h-2.5 rounded-full bg-red-400" />
                  <span className="w-2.5 h-2.5 rounded-full bg-yellow-400" />
                  <span className="w-2.5 h-2.5 rounded-full bg-green-400" />
                  <div className="ml-3 px-3 py-0.5 rounded-md bg-white border border-ink-100 text-[11px] text-ink-500 font-mono">
                    bidwriter.app/bids/sz-metro-12
                  </div>
                </div>
                <div className="p-5 space-y-3">
                  <div className="flex items-center justify-between">
                    <div>
                      <div className="text-xs text-ink-400">深圳地铁 12 号线 · 投标文件</div>
                      <div className="text-base font-semibold">章节生成中 · 7 / 12</div>
                    </div>
                    <span className="px-2 py-0.5 rounded-full bg-emerald-50 text-emerald-700 text-[11px] font-medium">
                      进行中
                    </span>
                  </div>
                  {/* progress bar with shimmer */}
                  <div className="relative h-1.5 rounded-full bg-ink-100 overflow-hidden">
                    <div className="absolute inset-y-0 left-0 right-[40%] bg-gradient-to-r from-brand-500 to-brand-700 rounded-full" />
                    <div className="absolute inset-y-0 left-0 right-[40%] bg-gradient-to-r from-transparent via-white/40 to-transparent bg-[length:200%_100%] animate-shimmer rounded-full" />
                  </div>
                  {/* fake chapter list */}
                  <div className="space-y-1.5 pt-1">
                    {[
                      { name: '项目背景与理解', done: true },
                      { name: '总体技术方案', done: true },
                      { name: '施工组织设计', done: true },
                      { name: '质量保证体系', done: false, active: true },
                      { name: '进度计划与保障', done: false },
                      { name: '安全管理方案', done: false },
                    ].map(c => (
                      <div key={c.name} className="flex items-center gap-2 px-2.5 py-1.5 rounded-lg text-xs">
                        {c.done ? (
                          <span className="w-4 h-4 rounded-full bg-emerald-500 text-white grid place-items-center text-[10px]">✓</span>
                        ) : c.active ? (
                          <span className="w-4 h-4 rounded-full bg-brand-500 text-white grid place-items-center text-[10px] animate-pulse-soft">●</span>
                        ) : (
                          <span className="w-4 h-4 rounded-full border border-ink-200" />
                        )}
                        <span className={c.active ? 'font-medium text-brand-700' : 'text-ink-700'}>{c.name}</span>
                      </div>
                    ))}
                  </div>
                  {/* KB hit preview */}
                  <div className="rounded-lg bg-brand-50 border border-brand-100 p-2.5 mt-2">
                    <div className="flex items-center gap-1.5 text-[10px] text-brand-700 font-medium mb-1">
                      <span>🔍</span>
                      <span>检索证据链 · ISO 9001 体系认证</span>
                    </div>
                    <div className="text-[11px] text-ink-600 leading-snug">
                      "通过 ISO 9001 质量管理体系、ISO 14001 环境管理体系、OHSAS 18001 职业健康安全管理体系认证..."
                      <span className="text-ink-400 ml-1">— 资质文件 §3.2</span>
                    </div>
                  </div>
                </div>
              </div>
            </div>
            {/* floating stat — top right */}
            <div className="hidden md:flex absolute -top-3 -right-3 bg-white text-ink-900 rounded-xl px-3 py-2 shadow-xl border border-ink-100 items-center gap-2 animate-float">
              <div className="w-8 h-8 rounded-lg bg-emerald-50 grid place-items-center text-emerald-600 text-base">📈</div>
              <div>
                <div className="text-[10px] text-ink-400 leading-none">平均节省时间</div>
                <div className="text-lg font-bold text-emerald-600 leading-tight tabular-nums">72%</div>
              </div>
            </div>
            {/* floating stat — bottom left */}
            <div className="hidden md:flex absolute -bottom-3 -left-3 bg-white text-ink-900 rounded-xl px-3 py-2 shadow-xl border border-ink-100 items-center gap-2 animate-float" style={{ animationDelay: '1.5s' }}>
              <div className="w-8 h-8 rounded-lg bg-brand-50 grid place-items-center text-brand-600 text-base">⚡</div>
              <div>
                <div className="text-[10px] text-ink-400 leading-none">5 分钟</div>
                <div className="text-xs font-semibold text-brand-600 leading-tight">出一份标书</div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>
  )
}

function Check() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round">
      <polyline points="20 6 9 17 4 12" />
    </svg>
  )
}

function LogoStrip() {
  const logos = ['中建八局', '中铁建工', '上海建工', '中国电信', '中国移动', '国家电网', '中交一公局', '中冶集团']
  // Duplicate for the marquee effect — the second copy is purely visual filler
  // so the strip can scroll infinitely without showing an empty gap.
  const looped = [...logos, ...logos]
  return (
    <section className="bg-white dark:bg-ink-900 border-b border-ink-100 dark:border-ink-800 py-8 overflow-hidden">
      <div className="max-w-7xl mx-auto px-6 lg:px-10">
        <div className="text-center text-xs uppercase tracking-wider text-ink-400 dark:text-ink-500 mb-5">
          已被这些企业的投标团队使用
        </div>
        <div className="relative overflow-hidden mask-fade">
          <div className="flex items-center gap-x-12 gap-y-4 whitespace-nowrap animate-marquee will-change-transform">
            {looped.map((name, i) => (
              <span key={`${name}-${i}`} className="text-base lg:text-lg font-semibold text-ink-700 dark:text-ink-300 tracking-wide opacity-70 hover:opacity-100 transition-opacity shrink-0">
                {name}
              </span>
            ))}
          </div>
          {/* edge fade masks */}
          <div className="pointer-events-none absolute inset-y-0 left-0 w-16 bg-gradient-to-r from-white dark:from-ink-900 to-transparent" />
          <div className="pointer-events-none absolute inset-y-0 right-0 w-16 bg-gradient-to-l from-white dark:from-ink-900 to-transparent" />
        </div>
      </div>
    </section>
  )
}

function FeatureGrid() {
  const features = [
    {
      icon: '📄',
      title: '智能大纲生成',
      desc: '解析招标文件后 30 秒生成符合招标要求的章节大纲，结构、层级、字数全自动规划。',
      accent: 'from-blue-500 to-blue-700',
      stat: '30s',
      statLabel: '生成大纲',
    },
    {
      icon: '✍️',
      title: '章节正文撰写',
      desc: '基于知识库证据链撰写章节正文，每段都有出处标记，支持多模型路由选最优输出。',
      accent: 'from-purple-500 to-purple-700',
      stat: '4 模型',
      statLabel: '动态路由',
    },
    {
      icon: '🔍',
      title: 'RAG 证据检索',
      desc: '向量 + BM25 + RRF 混合检索，自动从你的企业资质库、过往标书、专利证书中找证据。',
      accent: 'from-emerald-500 to-emerald-700',
      stat: '3 路',
      statLabel: '混合检索',
    },
    {
      icon: '🖼️',
      title: '图表自动渲染',
      desc: '甘特图、组织架构、网络拓扑，Mermaid / go-echarts 一键渲染并嵌入 Word。',
      accent: 'from-amber-500 to-amber-700',
      stat: '12 类',
      statLabel: '图表模板',
    },
    {
      icon: '✅',
      title: '合规审计',
      desc: '废标项扫描、一致性校验、资质合规检查，把 90% 的常见废标原因挡在提交前。',
      accent: 'from-rose-500 to-rose-700',
      stat: '120+',
      statLabel: '审计规则',
    },
    {
      icon: '👥',
      title: '人在回路 (HIL)',
      desc: '3 个暂停点：解析后、生成前、提交前。关键决策由人拍板，AI 只做初稿。',
      accent: 'from-indigo-500 to-indigo-700',
      stat: '3 步',
      statLabel: '人工审核',
    },
  ]

  return (
    <section id="features" className="relative py-20 lg:py-28 bg-ink-50 dark:bg-ink-900 overflow-hidden">
      {/* decorative blurred orbs */}
      <div className="absolute top-1/4 -left-20 w-72 h-72 bg-brand-200/30 dark:bg-brand-900/20 rounded-full blur-3xl pointer-events-none" />
      <div className="absolute bottom-1/4 -right-20 w-72 h-72 bg-violet-200/30 dark:bg-violet-900/20 rounded-full blur-3xl pointer-events-none" />

      <div className="relative max-w-7xl mx-auto px-6 lg:px-10">
        <SectionHeader
          eyebrow="核心功能"
          title="为什么投标团队选 BidWriter"
          desc="不是又一个 GPT 套壳应用。我们围绕标书编制的真实流程，从 RFP 解析到 Word 导出，每个环节都做过一遍。"
        />
        <div className="grid md:grid-cols-2 lg:grid-cols-3 gap-5 mt-12">
          {features.map((f, i) => (
            <div
              key={f.title}
              className="group relative p-6 rounded-2xl bg-white dark:bg-ink-800 border border-ink-100 dark:border-ink-700 hover:border-brand-200 dark:hover:border-brand-700 hover:shadow-card-hover hover:-translate-y-1 transition-all duration-300"
              style={{ animationDelay: `${i * 50}ms` }}
            >
              {/* Top-row: icon + floating stat */}
              <div className="flex items-start justify-between mb-4">
                <div className={`relative inline-flex items-center justify-center w-12 h-12 rounded-xl bg-gradient-to-br ${f.accent} text-white text-2xl shadow-md group-hover:scale-110 group-hover:rotate-3 transition-transform duration-300`}>
                  {f.icon}
                  {/* glow on hover */}
                  <div className={`absolute inset-0 rounded-xl bg-gradient-to-br ${f.accent} opacity-0 group-hover:opacity-40 blur-xl transition-opacity -z-10`} />
                </div>
                <div className="text-right">
                  <div className="text-lg font-bold text-ink-900 dark:text-white tabular-nums leading-none">{f.stat}</div>
                  <div className="text-[10px] text-ink-400 mt-0.5">{f.statLabel}</div>
                </div>
              </div>
              <h3 className="text-base font-semibold mb-2 text-ink-900 dark:text-white">{f.title}</h3>
              <p className="text-sm text-ink-500 dark:text-ink-400 leading-relaxed">{f.desc}</p>
              {/* hover arrow */}
              <div className="mt-4 flex items-center gap-1 text-xs font-medium text-brand-600 dark:text-brand-400 opacity-0 group-hover:opacity-100 -translate-x-2 group-hover:translate-x-0 transition-all duration-300">
                <span>了解详情</span>
                <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
                  <line x1="5" y1="12" x2="19" y2="12" />
                  <polyline points="12 5 19 12 12 19" />
                </svg>
              </div>
            </div>
          ))}
        </div>
      </div>
    </section>
  )
}

function SectionHeader({ eyebrow, title, desc }: { eyebrow: string; title: string; desc: string }) {
  return (
    <div className="max-w-2xl">
      <div className="inline-flex items-center gap-2 px-2.5 py-1 rounded-full bg-brand-50 text-brand-700 text-xs font-medium mb-3">
        <span className="w-1 h-1 rounded-full bg-brand-500" />
        {eyebrow}
      </div>
      <h2 className="text-3xl lg:text-4xl font-bold tracking-tight mb-3">{title}</h2>
      <p className="text-base text-ink-500 leading-relaxed">{desc}</p>
    </div>
  )
}

function HowItWorks() {
  const steps = [
    { n: '01', t: '上传招标文件', d: '支持 .pdf / .docx / .txt，系统自动解析出评分项、资质要求、技术规格。', icon: '📤', tone: 'brand' },
    { n: '02', t: '导入知识库', d: '上传过往标书、企业资质、专利证书、团队简历，AI 建立可检索的证据库。', icon: '📚', tone: 'emerald' },
    { n: '03', t: 'AI 生成 + 人在回路', d: '一键生成大纲和初稿，三个暂停点让你审核关键决策。', icon: '⚡', tone: 'purple' },
    { n: '04', t: '审计 + 导出 Word', d: '废标项扫描、证据链核对、一致性校验后导出符合招标格式的 .docx。', icon: '✅', tone: 'amber' },
  ]
  const toneBg: Record<string, string> = {
    brand:   'bg-brand-50 text-brand-600 dark:bg-brand-900/30 dark:text-brand-300',
    emerald: 'bg-emerald-50 text-emerald-600 dark:bg-emerald-900/30 dark:text-emerald-300',
    purple:  'bg-violet-50 text-violet-600 dark:bg-violet-900/30 dark:text-violet-300',
    amber:   'bg-amber-50 text-amber-600 dark:bg-amber-900/30 dark:text-amber-300',
  }
  return (
    <section id="how" className="py-20 lg:py-28 bg-white dark:bg-ink-900 relative overflow-hidden">
      {/* subtle grid background */}
      <div className="absolute inset-0 bg-grid-fine opacity-50 dark:opacity-30 pointer-events-none" />
      <div className="relative max-w-7xl mx-auto px-6 lg:px-10">
        <SectionHeader
          eyebrow="使用流程"
          title="从 RFP 到投标文件，4 步搞定"
          desc="不需要懂提示词工程，不需要学新工具，按流程走就行。"
        />
        <div className="grid md:grid-cols-2 lg:grid-cols-4 gap-4 mt-12">
          {steps.map((s, i) => (
            <div key={s.n} className="group relative">
              {i < steps.length - 1 && (
                <div className="hidden lg:block absolute top-10 left-full w-full h-px bg-gradient-to-r from-brand-200 via-brand-100 to-transparent -translate-x-4" />
              )}
              <div className="card p-5 hover:-translate-y-1 hover:shadow-card-hover hover:border-brand-200 dark:hover:border-brand-700 transition-all duration-300 h-full">
                <div className="flex items-center justify-between mb-3">
                  <div className={`w-12 h-12 rounded-xl ${toneBg[s.tone]} grid place-items-center text-2xl group-hover:scale-110 transition-transform`}>
                    {s.icon}
                  </div>
                  <div className="text-2xl font-bold text-ink-200 dark:text-ink-700 tabular-nums">{s.n}</div>
                </div>
                <h3 className="text-base font-semibold mb-2 text-ink-900 dark:text-white">{s.t}</h3>
                <p className="text-sm text-ink-500 dark:text-ink-400 leading-relaxed">{s.d}</p>
              </div>
            </div>
          ))}
        </div>
      </div>
    </section>
  )
}

function UseCases() {
  const cases = [
    { industry: '建筑工程', desc: '施工组织设计、进度计划、质量方案', icon: '🏗️' },
    { industry: '轨道交通', desc: '技术方案、车辆配置、信号系统说明', icon: '🚇' },
    { industry: 'IT 服务', desc: '系统架构、SLA 承诺、运维方案', icon: '💻' },
    { industry: '政企采购', desc: '资质应答、实施方案、培训服务', icon: '🏛️' },
    { industry: '医疗设备', desc: '产品技术参数、临床方案、售后服务', icon: '🏥' },
    { industry: '咨询服务', desc: '项目方法论、团队配置、案例展示', icon: '📊' },
  ]
  return (
    <section id="cases" className="py-20 lg:py-28 bg-ink-50">
      <div className="max-w-7xl mx-auto px-6 lg:px-10">
        <SectionHeader
          eyebrow="适用场景"
          title="无论哪个行业，标书的逻辑是相通的"
          desc="内置行业知识模板，新行业也能快速适配。"
        />
        <div className="grid sm:grid-cols-2 lg:grid-cols-3 gap-4 mt-12">
          {cases.map(c => (
            <div key={c.industry} className="p-5 rounded-xl bg-white border border-ink-100 hover:border-brand-200 hover:shadow-md transition-all flex items-start gap-4">
              <span className="text-3xl shrink-0">{c.icon}</span>
              <div>
                <div className="font-semibold mb-1">{c.industry}</div>
                <div className="text-sm text-ink-500">{c.desc}</div>
              </div>
            </div>
          ))}
        </div>
      </div>
    </section>
  )
}

function Pricing() {
  const tiers = [
    {
      name: '免费试用',
      price: '¥0',
      period: '14 天',
      desc: '完整功能体验，不限章节数',
      cta: '开始试用',
      href: '/register',
      highlight: false,
      features: [
        '1 个工作区',
        '3 份标书',
        '10 万 token 用量',
        '知识库 100 篇',
        '邮件支持',
      ],
    },
    {
      name: '专业版',
      price: '¥299',
      period: '/ 月',
      desc: '中小型投标团队的标配',
      cta: '立即购买',
      href: '/register?plan=pro',
      highlight: true,
      features: [
        '5 个工作区',
        '不限标书数',
        '200 万 token / 月',
        '知识库不限',
        '工作日 8h 响应',
        'DeepSeek / GPT-4 切换',
        'API 接入',
      ],
    },
    {
      name: '企业版',
      price: '定制',
      period: '联系销售',
      desc: '私有部署、SSO、专属 SLA',
      cta: '联系销售',
      href: '/register?plan=enterprise',
      highlight: false,
      features: [
        '不限工作区',
        '不限标书 / token',
        'VPC 私有部署',
        'SSO / SAML',
        '7×24 专属支持',
        'Claude / GPT-4 全模型',
        '定制模型微调',
        '审计日志 12 个月',
      ],
    },
  ]
  return (
    <section id="pricing" className="py-20 lg:py-28 bg-white">
      <div className="max-w-7xl mx-auto px-6 lg:px-10">
        <SectionHeader
          eyebrow="套餐"
          title="先试用，喜欢再付费"
          desc="14 天免费试用，无需信用卡。专业版可月付可年付（年付 8 折）。"
        />
        <div className="grid md:grid-cols-3 gap-5 mt-12">
          {tiers.map(t => (
            <div
              key={t.name}
              className={`relative p-7 rounded-2xl border ${
                t.highlight
                  ? 'bg-gradient-to-br from-brand-600 to-brand-800 text-white border-brand-700 shadow-xl scale-[1.02]'
                  : 'bg-white text-ink-900 border-ink-100'
              }`}
            >
              {t.highlight && (
                <div className="absolute -top-3 left-1/2 -translate-x-1/2 px-3 py-1 rounded-full bg-amber-400 text-ink-900 text-xs font-semibold">
                  最受欢迎
                </div>
              )}
              <div className={`text-sm font-medium mb-1 ${t.highlight ? 'text-white/80' : 'text-ink-500'}`}>{t.name}</div>
              <div className="flex items-baseline gap-1.5 mb-2">
                <span className="text-4xl font-bold">{t.price}</span>
                <span className={`text-sm ${t.highlight ? 'text-white/70' : 'text-ink-400'}`}>{t.period}</span>
              </div>
              <p className={`text-sm mb-6 ${t.highlight ? 'text-white/80' : 'text-ink-500'}`}>{t.desc}</p>
              <Link to={t.href} className="block">
                <Button
                  variant={t.highlight ? 'primary' : 'secondary'}
                  className={`w-full ${t.highlight ? 'bg-white !text-brand-700 hover:!bg-brand-50' : ''}`}
                >
                  {t.cta}
                </Button>
              </Link>
              <ul className="mt-6 space-y-2.5 text-sm">
                {t.features.map(f => (
                  <li key={f} className="flex items-start gap-2">
                    <span className={`shrink-0 mt-0.5 ${t.highlight ? 'text-brand-200' : 'text-brand-600'}`}>
                      <Check />
                    </span>
                    <span className={t.highlight ? 'text-white/90' : 'text-ink-700'}>{f}</span>
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </div>
      </div>
    </section>
  )
}

function FAQ() {
  const qs = [
    {
      q: '14 天试用结束后会发生什么？',
      a: '试用结束后账号转为只读状态，所有数据保留 30 天。你可以选择升级到专业版继续使用，或导出你的数据（标书 + 知识库）后停止。',
    },
    {
      q: '生成的内容会不会和别人一样？',
      a: '不会。生成基于你的知识库（上传的资质、过往标书、案例），RAG 检索确保每段都有出处。两个不同公司的生成结果几乎不可能雷同。',
    },
    {
      q: '可以用我们自己的 LLM 吗？',
      a: '专业版支持接入 OpenAI-compatible 接口（如通义、文心、Kimi）。企业版可接入私有化部署的模型，所有调用走你的 VPC。',
    },
    {
      q: '数据安全怎么保证？',
      a: '所有数据存储在你的专属租户下，租户隔离到数据库行级。专业版支持数据驻留区域选择。企业版提供私有部署，模型调用和数据存储都不出你的网络。',
    },
    {
      q: '和直接用 ChatGPT 写标书有什么不同？',
      a: 'ChatGPT 没有你的企业资质库、没有过往标书、不知道招标评分项、不会做废标项扫描、不能导出符合格式的 Word。BidWriter 把这些环节全部串起来。',
    },
  ]
  return (
    <section id="faq" className="py-20 lg:py-28 bg-ink-50">
      <div className="max-w-3xl mx-auto px-6 lg:px-10">
        <SectionHeader
          eyebrow="常见问题"
          title="买之前你可能想知道的"
          desc="还有其他问题？发邮件到 hello@bidwriter.app 或右下角发消息。"
        />
        <div className="mt-12 space-y-3">
          {qs.map(item => (
            <details
              key={item.q}
              className="group p-5 rounded-xl bg-white border border-ink-100 hover:border-brand-200 [&_summary::-webkit-details-marker]:hidden"
            >
              <summary className="flex items-center justify-between gap-4 cursor-pointer list-none">
                <span className="font-medium">{item.q}</span>
                <span className="shrink-0 w-5 h-5 grid place-items-center rounded-full bg-ink-100 group-open:bg-brand-100 group-open:text-brand-700 transition-colors">
                  <span className="block group-open:hidden">+</span>
                  <span className="hidden group-open:block">−</span>
                </span>
              </summary>
              <p className="mt-3 text-sm text-ink-500 leading-relaxed">{item.a}</p>
            </details>
          ))}
        </div>
      </div>
    </section>
  )
}

function FinalCTA() {
  return (
    <section className="relative py-20 lg:py-28 overflow-hidden bg-ink-950 text-white">
      {/* Animated gradient mesh */}
      <div className="absolute inset-0 bg-gradient-animated opacity-95" />
      <div className="absolute inset-0 opacity-30" style={{
        backgroundImage: 'radial-gradient(at 30% 20%, rgba(139,180,255,0.4) 0px, transparent 50%), radial-gradient(at 70% 80%, rgba(53,103,246,0.3) 0px, transparent 50%)'
      }} />
      <div className="absolute inset-0 opacity-[0.05]" style={{
        backgroundImage: 'radial-gradient(rgba(255,255,255,0.6) 1px, transparent 1px)',
        backgroundSize: '24px 24px'
      }} />
      {/* floating orbs */}
      <div className="absolute top-1/2 left-1/4 -translate-y-1/2 w-32 h-32 rounded-full bg-emerald-400/20 blur-3xl animate-pulse-soft" />
      <div className="absolute top-1/2 right-1/4 -translate-y-1/2 w-40 h-40 rounded-full bg-violet-400/20 blur-3xl animate-pulse-soft" style={{ animationDelay: '1s' }} />

      <div className="relative max-w-4xl mx-auto px-6 lg:px-10 text-center">
        <div className="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-white/15 backdrop-blur-sm border border-white/10 text-xs font-medium mb-6 animate-slide-up">
          <span className="w-1.5 h-1.5 rounded-full bg-emerald-300 animate-pulse-soft" />
          <span>无需信用卡 · 14 天免费</span>
        </div>
        <h2 className="text-3xl md:text-4xl lg:text-5xl font-bold tracking-tight mb-4 animate-slide-up">
          下一份标书，<span className="bg-gradient-to-r from-white via-brand-100 to-white bg-clip-text text-transparent">让 AI 来写</span>
        </h2>
        <p className="text-lg text-white/80 mb-8 max-w-2xl mx-auto animate-slide-up" style={{ animationDelay: '80ms' }}>
          14 天免费试用，完整体验所有功能。觉得好用再付费，不喜欢直接走人。
        </p>
        <div className="flex flex-col sm:flex-row gap-3 justify-center animate-slide-up" style={{ animationDelay: '160ms' }}>
          <Link to="/register">
            <Button variant="gradient" size="lg" className="shadow-glow w-full sm:w-auto">
              免费试用 14 天
            </Button>
          </Link>
          <Link to="/login">
            <Button variant="ghost" size="lg" className="!text-white hover:!bg-white/10 border border-white/15 w-full sm:w-auto">
              已有账号 · 直接登录
            </Button>
          </Link>
        </div>
        <p className="text-xs text-white/50 mt-6 animate-slide-up" style={{ animationDelay: '240ms' }}>
          已被 200+ 投标团队使用 · 平均节省 72% 编制时间
        </p>
      </div>
    </section>
  )
}

function Footer() {
  return (
    <footer className="bg-ink-900 text-ink-300 relative overflow-hidden">
      {/* top accent line */}
      <div className="h-px bg-gradient-to-r from-transparent via-brand-500/40 to-transparent" />

      <div className="max-w-7xl mx-auto px-6 lg:px-10 py-12 lg:py-16">
        {/* Top row: brand + newsletter */}
        <div className="grid md:grid-cols-12 gap-8 mb-10 pb-10 border-b border-ink-700/60">
          <div className="md:col-span-5">
            <div className="flex items-center gap-2 mb-3">
              <Logo />
              <div className="text-white font-bold">AI 标书系统</div>
            </div>
            <p className="text-sm text-ink-400 leading-relaxed mb-4 max-w-sm">
              AI 驱动的智能标书编制平台。<br />
              让每一份标书都快、准、美。
            </p>
            <div className="flex items-center gap-3">
              <SocialButton label="GitHub">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor">
                  <path d="M12 .5C5.65.5.5 5.65.5 12c0 5.08 3.29 9.39 7.86 10.91.58.1.79-.25.79-.56v-2c-3.2.7-3.87-1.36-3.87-1.36-.52-1.33-1.27-1.69-1.27-1.69-1.04-.71.08-.7.08-.7 1.15.08 1.76 1.18 1.76 1.18 1.02 1.75 2.69 1.25 3.35.95.1-.74.4-1.25.72-1.54-2.55-.29-5.24-1.28-5.24-5.69 0-1.26.45-2.29 1.18-3.1-.12-.29-.51-1.46.11-3.05 0 0 .96-.31 3.16 1.18a10.94 10.94 0 0 1 5.75 0c2.2-1.49 3.16-1.18 3.16-1.18.62 1.59.23 2.76.11 3.05.74.81 1.18 1.84 1.18 3.1 0 4.42-2.69 5.4-5.25 5.68.41.36.78 1.07.78 2.15v3.19c0 .31.21.67.79.56A11.5 11.5 0 0 0 23.5 12C23.5 5.65 18.35.5 12 .5z" />
                </svg>
              </SocialButton>
              <SocialButton label="Twitter">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor">
                  <path d="M22 5.8a8.5 8.5 0 0 1-2.36.65 4.13 4.13 0 0 0 1.8-2.27 8.21 8.21 0 0 1-2.6 1 4.1 4.1 0 0 0-7 3.74 11.64 11.64 0 0 1-8.45-4.29 4.16 4.16 0 0 0-.55 2.07 4.09 4.09 0 0 0 1.82 3.41 4.05 4.05 0 0 1-1.86-.51v.05a4.1 4.1 0 0 0 3.29 4 4.16 4.16 0 0 1-1.85.07 4.11 4.11 0 0 0 3.83 2.84A8.22 8.22 0 0 1 2 18.34a11.62 11.62 0 0 0 6.29 1.84A11.59 11.59 0 0 0 20 8.45v-.53a8.18 8.18 0 0 0 2-2.12z" />
                </svg>
              </SocialButton>
              <SocialButton label="邮箱">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <path d="M4 4h16c1.1 0 2 .9 2 2v12c0 1.1-.9 2-2 2H4c-1.1 0-2-.9-2-2V6c0-1.1.9-2 2-2z" />
                  <polyline points="22,6 12,13 2,6" />
                </svg>
              </SocialButton>
            </div>
          </div>

          {/* Newsletter */}
          <div className="md:col-span-7">
            <div className="max-w-md">
              <h3 className="text-sm font-semibold text-white mb-1">订阅产品更新</h3>
              <p className="text-xs text-ink-400 mb-3">每月一封，分享新功能、行业案例、最佳实践。</p>
              <form className="flex items-stretch gap-2" onSubmit={(e) => e.preventDefault()}>
                <input
                  type="email"
                  placeholder="your@email.com"
                  className="flex-1 px-3 py-2 bg-ink-800 border border-ink-700 rounded-lg text-sm text-white placeholder:text-ink-500 focus:outline-none focus:border-brand-400 focus:ring-2 focus:ring-brand-900"
                />
                <button
                  type="submit"
                  className="px-4 py-2 rounded-lg bg-brand-600 hover:bg-brand-700 text-white text-sm font-medium transition-colors shadow-sm"
                >
                  订阅
                </button>
              </form>
            </div>
          </div>
        </div>

        {/* Link columns */}
        <div className="grid md:grid-cols-4 gap-8 mb-8">
          <FooterCol
            title="产品"
            links={[
              { name: '功能', href: '#features' },
              { name: '怎么用', href: '#how' },
              { name: '适用场景', href: '#cases' },
              { name: '套餐', href: '#pricing' },
              { name: '更新日志', href: '#' },
            ]}
          />
          <FooterCol
            title="公司"
            links={[
              { name: '关于我们', href: '#' },
              { name: '联系销售', href: 'mailto:hello@bidwriter.app' },
              { name: '加入我们', href: '#' },
              { name: '媒体资源', href: '#' },
            ]}
          />
          <FooterCol
            title="支持"
            links={[
              { name: '帮助中心', href: '#' },
              { name: 'API 文档', href: '#' },
              { name: '状态页', href: '#' },
              { name: '服务条款', href: '#' },
            ]}
          />
          <FooterCol
            title="开发者"
            links={[
              { name: '开源仓库', href: '#' },
              { name: '变更日志', href: '#' },
              { name: '贡献指南', href: '#' },
              { name: '隐私政策', href: '#' },
            ]}
          />
        </div>
        <div className="pt-8 border-t border-ink-700/60 flex flex-col md:flex-row items-center justify-between gap-4 text-xs text-ink-500">
          <div>© {new Date().getFullYear()} AI 标书系统 · AGPL-3.0 License · Made with ❤️ in China</div>
          <div className="flex items-center gap-3">
            <span className="inline-flex items-center gap-1.5">
              <span className="relative flex w-1.5 h-1.5">
                <span className="absolute inline-flex w-full h-full rounded-full bg-emerald-400 opacity-75 animate-ping-slow" />
                <span className="relative inline-flex rounded-full w-1.5 h-1.5 bg-emerald-400" />
              </span>
              系统状态: 正常
            </span>
            <span>·</span>
            <span>v1.0.0</span>
          </div>
        </div>
      </div>
    </footer>
  )
}

function SocialButton({ children, label }: { children: React.ReactNode; label: string }) {
  return (
    <a
      href="#"
      aria-label={label}
      className="w-8 h-8 rounded-lg bg-ink-800 border border-ink-700 hover:border-brand-500 hover:bg-ink-700 text-ink-300 hover:text-brand-300 grid place-items-center transition-all"
    >
      {children}
    </a>
  )
}

function FooterCol({ title, links }: { title: string; links: { name: string; href: string }[] }) {
  return (
    <div>
      <div className="text-white text-sm font-semibold mb-3">{title}</div>
      <ul className="space-y-2 text-sm">
        {links.map(l => (
          <li key={l.name}>
            <a href={l.href} className="text-ink-400 hover:text-white transition-colors">{l.name}</a>
          </li>
        ))}
      </ul>
    </div>
  )
}

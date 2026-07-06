import { Link } from 'react-router-dom'
import { Button } from '../components/ui'

// LandingPage is the public marketing page. It introduces BidWriter, walks
// through how it works, shows what's included, lists pricing tiers, and
// ends with a CTA. Anyone can reach it; once they sign in we send them
// to /bids (see the App-level redirect).
export default function LandingPage() {
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
      {/* mesh background */}
      <div className="absolute inset-0 opacity-30" style={{
        backgroundImage: 'radial-gradient(at 20% 20%, rgba(139,180,255,0.4) 0px, transparent 50%), radial-gradient(at 80% 0%, rgba(53,103,246,0.3) 0px, transparent 50%), radial-gradient(at 80% 100%, rgba(139,180,255,0.2) 0px, transparent 50%)'
      }} />
      <div className="absolute inset-0 opacity-[0.07]" style={{
        backgroundImage: 'radial-gradient(rgba(255,255,255,0.6) 1px, transparent 1px)',
        backgroundSize: '24px 24px'
      }} />

      <div className="relative max-w-7xl mx-auto px-6 lg:px-10 pt-20 pb-24 lg:pt-28 lg:pb-32">
        <div className="grid lg:grid-cols-12 gap-12 items-center">
          <div className="lg:col-span-7">
            <div className="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-white/15 backdrop-blur-sm text-xs font-medium mb-6 animate-slide-up">
              <span className="w-1.5 h-1.5 rounded-full bg-emerald-300 animate-pulse-soft" />
              <span>2026 全新发布 · 多模型路由 · 人在回路</span>
            </div>
            <h1 className="text-4xl md:text-5xl lg:text-6xl font-bold leading-tight tracking-tight mb-6 animate-slide-up">
              让标书编制<br />
              <span className="bg-gradient-to-r from-brand-100 via-white to-brand-200 bg-clip-text text-transparent">
                快 10 倍，准确 100%
              </span>
            </h1>
            <p className="text-lg lg:text-xl text-white/80 leading-relaxed mb-8 max-w-2xl">
              自动解析招标文件 · 智能生成章节大纲与正文 · 检索企业知识库作为证据链 · 合规审计一键完成。
              告别熬夜改稿，让 AI 帮你写完一份 80 分的标书，你只需做最后的 20 分润色。
            </p>
            <div className="flex flex-col sm:flex-row gap-3 mb-8">
              <Link to="/register">
                <Button variant="primary" size="lg" className="bg-white !text-brand-700 hover:!bg-brand-50 w-full sm:w-auto">
                  免费试用 14 天
                  <span className="ml-1 text-xs font-normal opacity-70">无需信用卡</span>
                </Button>
              </Link>
              <Link to="/login">
                <Button variant="ghost" size="lg" className="!text-white hover:!bg-white/10 w-full sm:w-auto">
                  已有账号 · 登录
                </Button>
              </Link>
            </div>
            <div className="flex items-center gap-6 text-sm text-white/70">
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
            </div>
          </div>

          <div className="lg:col-span-5 relative">
            {/* Mock product preview */}
            <div className="relative rounded-2xl bg-white/10 backdrop-blur-md border border-white/20 p-4 shadow-2xl animate-slide-up">
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
                  {/* progress bar */}
                  <div className="h-1.5 rounded-full bg-ink-100 overflow-hidden">
                    <div className="h-full w-3/5 bg-gradient-to-r from-brand-500 to-brand-700 rounded-full" />
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
            {/* floating stat */}
            <div className="hidden md:block absolute -bottom-4 -left-4 bg-white text-ink-900 rounded-xl px-4 py-3 shadow-xl border border-ink-100 animate-fade-in">
              <div className="text-xs text-ink-400">平均节省时间</div>
              <div className="text-2xl font-bold text-brand-600">72%</div>
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
  return (
    <section className="bg-white border-b border-ink-100 py-8">
      <div className="max-w-7xl mx-auto px-6 lg:px-10">
        <div className="text-center text-xs uppercase tracking-wider text-ink-400 mb-5">
          已被这些企业的投标团队使用
        </div>
        <div className="flex flex-wrap items-center justify-center gap-x-10 gap-y-4 opacity-60">
          {['中建八局', '中铁建工', '上海建工', '中国电信', '中国移动', '国家电网', '中交一公局'].map(name => (
            <span key={name} className="text-base lg:text-lg font-semibold text-ink-700 tracking-wide">{name}</span>
          ))}
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
    },
    {
      icon: '✍️',
      title: '章节正文撰写',
      desc: '基于知识库证据链撰写章节正文，每段都有出处标记，支持多模型路由选最优输出。',
      accent: 'from-purple-500 to-purple-700',
    },
    {
      icon: '🔍',
      title: 'RAG 证据检索',
      desc: '向量 + BM25 + RRF 混合检索，自动从你的企业资质库、过往标书、专利证书中找证据。',
      accent: 'from-emerald-500 to-emerald-700',
    },
    {
      icon: '🖼️',
      title: '图表自动渲染',
      desc: '甘特图、组织架构、网络拓扑，Mermaid / go-echarts 一键渲染并嵌入 Word。',
      accent: 'from-amber-500 to-amber-700',
    },
    {
      icon: '✅',
      title: '合规审计',
      desc: '废标项扫描、一致性校验、资质合规检查，把 90% 的常见废标原因挡在提交前。',
      accent: 'from-rose-500 to-rose-700',
    },
    {
      icon: '👥',
      title: '人在回路 (HIL)',
      desc: '3 个暂停点：解析后、生成前、提交前。关键决策由人拍板，AI 只做初稿。',
      accent: 'from-indigo-500 to-indigo-700',
    },
  ]

  return (
    <section id="features" className="py-20 lg:py-28 bg-ink-50">
      <div className="max-w-7xl mx-auto px-6 lg:px-10">
        <SectionHeader
          eyebrow="核心功能"
          title="为什么投标团队选 BidWriter"
          desc="不是又一个 GPT 套壳应用。我们围绕标书编制的真实流程，从 RFP 解析到 Word 导出，每个环节都做过一遍。"
        />
        <div className="grid md:grid-cols-2 lg:grid-cols-3 gap-5 mt-12">
          {features.map((f, i) => (
            <div
              key={f.title}
              className="group relative p-6 rounded-2xl bg-white border border-ink-100 hover:border-brand-200 hover:shadow-lg hover:-translate-y-0.5 transition-all"
              style={{ animationDelay: `${i * 50}ms` }}
            >
              <div className={`inline-flex items-center justify-center w-12 h-12 rounded-xl bg-gradient-to-br ${f.accent} text-white text-2xl mb-4 shadow-md`}>
                {f.icon}
              </div>
              <h3 className="text-base font-semibold mb-2">{f.title}</h3>
              <p className="text-sm text-ink-500 leading-relaxed">{f.desc}</p>
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
    { n: '01', t: '上传招标文件', d: '支持 .pdf / .docx / .txt，系统自动解析出评分项、资质要求、技术规格。' },
    { n: '02', t: '导入知识库', d: '上传过往标书、企业资质、专利证书、团队简历，AI 建立可检索的证据库。' },
    { n: '03', t: 'AI 生成 + 人在回路', d: '一键生成大纲和初稿，三个暂停点让你审核关键决策。' },
    { n: '04', t: '审计 + 导出 Word', d: '废标项扫描、证据链核对、一致性校验后导出符合招标格式的 .docx。' },
  ]
  return (
    <section id="how" className="py-20 lg:py-28 bg-white">
      <div className="max-w-7xl mx-auto px-6 lg:px-10">
        <SectionHeader
          eyebrow="使用流程"
          title="从 RFP 到投标文件，4 步搞定"
          desc="不需要懂提示词工程，不需要学新工具，按流程走就行。"
        />
        <div className="grid md:grid-cols-2 lg:grid-cols-4 gap-4 mt-12">
          {steps.map((s, i) => (
            <div key={s.n} className="relative">
              {i < steps.length - 1 && (
                <div className="hidden lg:block absolute top-8 left-full w-full h-px bg-gradient-to-r from-brand-200 to-transparent -translate-x-4" />
              )}
              <div className="text-3xl font-bold text-brand-600/30 mb-2">{s.n}</div>
              <h3 className="text-base font-semibold mb-2">{s.t}</h3>
              <p className="text-sm text-ink-500 leading-relaxed">{s.d}</p>
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
    <section className="py-20 lg:py-28 bg-gradient-to-br from-brand-700 via-brand-800 to-brand-950 text-white relative overflow-hidden">
      <div className="absolute inset-0 opacity-30" style={{
        backgroundImage: 'radial-gradient(at 30% 20%, rgba(139,180,255,0.4) 0px, transparent 50%), radial-gradient(at 70% 80%, rgba(53,103,246,0.3) 0px, transparent 50%)'
      }} />
      <div className="relative max-w-4xl mx-auto px-6 lg:px-10 text-center">
        <h2 className="text-3xl md:text-4xl lg:text-5xl font-bold tracking-tight mb-4">
          下一份标书，让 AI 来写
        </h2>
        <p className="text-lg text-white/80 mb-8 max-w-2xl mx-auto">
          14 天免费试用，完整体验所有功能。觉得好用再付费，不喜欢直接走人。
        </p>
        <div className="flex flex-col sm:flex-row gap-3 justify-center">
          <Link to="/register">
            <Button variant="primary" size="lg" className="bg-white !text-brand-700 hover:!bg-brand-50">
              免费试用 14 天
            </Button>
          </Link>
          <Link to="/pricing">
            <Button variant="ghost" size="lg" className="!text-white hover:!bg-white/10">
              查看套餐详情
            </Button>
          </Link>
        </div>
      </div>
    </section>
  )
}

function Footer() {
  return (
    <footer className="bg-ink-900 text-ink-300 py-12">
      <div className="max-w-7xl mx-auto px-6 lg:px-10">
        <div className="grid md:grid-cols-4 gap-8 mb-8">
          <div className="md:col-span-1">
            <div className="flex items-center gap-2 mb-3">
              <Logo />
              <div className="text-white font-bold">AI 标书系统</div>
            </div>
            <p className="text-sm text-ink-400 leading-relaxed">
              AI 驱动的智能标书编制平台。
              <br />让每一份标书都快、准、美。
            </p>
          </div>
          <FooterCol
            title="产品"
            links={[
              { name: '功能', href: '#features' },
              { name: '怎么用', href: '#how' },
              { name: '适用场景', href: '#cases' },
              { name: '套餐', href: '#pricing' },
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
              { name: '服务条款', href: '#' },
              { name: '隐私政策', href: '#' },
            ]}
          />
        </div>
        <div className="pt-8 border-t border-ink-700 flex flex-col md:flex-row items-center justify-between gap-4 text-xs text-ink-500">
          <div>© {new Date().getFullYear()} AI 标书系统 · AGPL-3.0 License</div>
          <div className="flex items-center gap-4">
            <span>hello@bidwriter.app</span>
            <span>·</span>
            <span>v1.0.0</span>
          </div>
        </div>
      </div>
    </footer>
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

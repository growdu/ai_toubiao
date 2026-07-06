import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import LandingPage from './LandingPage'

// LandingPage smoke tests. We don't try to exercise every interaction
// (those would be brittle and the marketing copy changes often); instead
// we verify the page mounts, all six marketing sections are present, and
// the CTAs route to /register or /login as expected.

function renderLanding() {
  return render(
    <MemoryRouter>
      <LandingPage />
    </MemoryRouter>
  )
}

describe('LandingPage', () => {
  it('renders the hero h1 + dual CTAs', () => {
    renderLanding()
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent(/让标书编制/)
    // Hero CTA: "免费试用 14 天..." appears in two places (hero + final
    // CTA section); assert the first match (hero) and that it routes to
    // /register.
    const heroCta = screen.getAllByRole('link', { name: /免费试用 14 天/ })[0]
    expect(heroCta).toHaveAttribute('href', '/register')
    expect(screen.getByRole('link', { name: /已有账号 · 登录/ })).toHaveAttribute('href', '/login')
  })

  it('lists the six feature cards', () => {
    renderLanding()
    for (const t of ['智能大纲生成', '章节正文撰写', 'RAG 证据检索', '图表自动渲染', '合规审计']) {
      expect(screen.getByRole('heading', { name: t })).toBeInTheDocument()
    }
    // "人在回路 (HIL)" appears as both a feature-card heading AND in
    // the hero pill "AI 驱动 · 多模型路由 · 人在回路". We only care
    // about the heading here; getAllByRole + filter narrows the match.
    const hilHeadings = screen.getAllByRole('heading', { name: /人在回路/ }).filter(
      h => h.tagName.toLowerCase() === 'h3'
    )
    expect(hilHeadings.length).toBeGreaterThanOrEqual(1)
  })

  it('renders the four-step "how it works" timeline', () => {
    renderLanding()
    for (const t of ['上传招标文件', '导入知识库', 'AI 生成 + 人在回路', '审计 + 导出 Word']) {
      expect(screen.getByRole('heading', { name: t })).toBeInTheDocument()
    }
  })

  it('renders the three pricing tiers with prices', () => {
    renderLanding()
    expect(screen.getByText('¥0')).toBeInTheDocument()
    expect(screen.getByText('¥299')).toBeInTheDocument()
    expect(screen.getByText('定制')).toBeInTheDocument()
    // Pro tier CTA should pass through the plan query so the register
    // page can read it later.
    expect(screen.getByRole('link', { name: /立即购买/ })).toHaveAttribute('href', '/register?plan=pro')
  })

  it('has at least 4 FAQ items in collapsible sections', () => {
    renderLanding()
    expect(screen.getAllByRole('group').length).toBeGreaterThanOrEqual(4)
  })

  it('topnav Login button routes to /login', () => {
    renderLanding()
    const topnavLogin = screen.getAllByRole('link', { name: /^登录$/ })[0]
    expect(topnavLogin).toHaveAttribute('href', '/login')
  })

  it('topnav 免费试用 button routes to /register', () => {
    renderLanding()
    const topnavCta = screen.getAllByRole('link', { name: /^免费试用$/ })[0]
    expect(topnavCta).toHaveAttribute('href', '/register')
  })

  it('footer renders the three columns', () => {
    renderLanding()
    for (const t of ['产品', '公司', '支持']) {
      expect(screen.getByText(t)).toBeInTheDocument()
    }
  })
})

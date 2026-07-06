import { describe, it, expect, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import Layout from './Layout'
import { useAuthStore } from '../lib/auth'

function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route element={<Layout />}>
          {/* trailing-path routes so /bids/abc still hits the Outlet */}
          <Route path="/bids/*" element={<div>bids-page</div>} />
          <Route path="/knowledge/*" element={<div>knowledge-page</div>} />
          <Route path="/settings/*" element={<div>settings-page</div>} />
        </Route>
        {/* catch-all so logout's navigate('/login') resolves to a node
            instead of throwing "No routes matched location /login" */}
        <Route path="*" element={<div data-testid="route-fallback" />} />
      </Routes>
    </MemoryRouter>
  )
}

describe('Layout', () => {
  beforeEach(() => {
    useAuthStore.setState({ token: 'tok', userId: 'user-42', tenantId: 't-1' })
  })

  it('renders the brand, all three nav items, and the user id', () => {
    renderAt('/bids')
    expect(screen.getByText('AI 标书系统')).toBeInTheDocument()
    expect(screen.getByText('标书管理')).toBeInTheDocument()
    expect(screen.getByText('知识库')).toBeInTheDocument()
    expect(screen.getByText('设置')).toBeInTheDocument()
    expect(screen.getByText('user-42')).toBeInTheDocument()
  })

  it('applies the brand-active class to the nav item matching the current path', () => {
    renderAt('/knowledge')
    const navLinks = screen.getAllByRole('link')
    const active = navLinks.find((a) => a.getAttribute('href') === '/knowledge')
    const inactive = navLinks.find((a) => a.getAttribute('href') === '/bids')
    expect(active!.className).toContain('bg-brand-600')
    expect(inactive!.className).not.toContain('bg-brand-600')
  })

  it('renders the child route via <Outlet />', () => {
    renderAt('/settings')
    expect(screen.getByText('settings-page')).toBeInTheDocument()
  })

  it('logout button clears the auth store and navigates to /login', async () => {
    const user = userEvent.setup()
    renderAt('/bids')

    await user.click(screen.getByRole('button', { name: '退出登录' }))
    const s = useAuthStore.getState()
    expect(s.token).toBeNull()
    expect(s.userId).toBeNull()
    expect(s.tenantId).toBeNull()
  })

  it('includes trailing-path matches in the active class (startsWith semantics)', () => {
    // /bids/abc should still highlight the "标书管理" nav item, because
    // Layout uses location.pathname.startsWith(item.path).
    renderAt('/bids/abc-123')
    const navLinks = screen.getAllByRole('link')
    const active = navLinks.find((a) => a.getAttribute('href') === '/bids')
    expect(active!.className).toContain('bg-brand-600')
  })
})
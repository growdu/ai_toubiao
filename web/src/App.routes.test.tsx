import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { useAuthStore } from './lib/auth'

// We mock the layout to forward Outlet so the inner route element is
// what the test asserts on. Mocking Layout as a static element would
// swallow Outlet and hide the page — exactly the bug shape we are
// guarding against.
vi.mock('./components/Layout', () => ({
  default: () => {
    const { Outlet } = require('react-router-dom')
    return <div data-testid="layout"><Outlet /></div>
  },
}))
vi.mock('./components/ToastContainer', () => ({ ToastContainer: () => null }))
vi.mock('./pages/LandingPage', () => ({ default: () => <div data-testid="landing" /> }))
vi.mock('./pages/auth/LoginPage', () => ({ default: () => <div data-testid="login" /> }))
vi.mock('./pages/auth/RegisterPage', () => ({ default: () => <div data-testid="register" /> }))
vi.mock('./pages/bids/BidsPage', () => ({ default: () => <div data-testid="bids" /> }))
vi.mock('./pages/bids/BidWorkspace', () => ({ default: () => <div data-testid="workspace" /> }))
vi.mock('./pages/bids/ExportPage', () => ({ default: () => <div data-testid="export" /> }))
vi.mock('./pages/knowledge/KnowledgePage', () => ({ default: () => <div data-testid="knowledge" /> }))
vi.mock('./pages/settings/SettingsPage', () => ({ default: () => <div data-testid="settings" /> }))

import App from './App'

describe('App routing', () => {
  beforeEach(() => {
    useAuthStore.setState({ token: 't', refreshToken: 'r', userId: 'u', tenantId: 't' } as any)
  })

  it('renders BidsPage at /bids (no double-prefix /bids/bids)', () => {
    render(<MemoryRouter initialEntries={['/bids']}><App /></MemoryRouter>)
    expect(screen.queryByTestId('layout')).toBeTruthy()
    expect(screen.queryByTestId('bids')).toBeTruthy()
    expect(screen.queryByTestId('landing')).toBeNull()
  })

  it('renders KnowledgePage at /knowledge (not nested under /bids)', () => {
    render(<MemoryRouter initialEntries={['/knowledge']}><App /></MemoryRouter>)
    expect(screen.queryByTestId('layout')).toBeTruthy()
    expect(screen.queryByTestId('knowledge')).toBeTruthy()
    expect(screen.queryByTestId('landing')).toBeNull()
  })

  it('renders SettingsPage at /settings', () => {
    render(<MemoryRouter initialEntries={['/settings']}><App /></MemoryRouter>)
    expect(screen.queryByTestId('layout')).toBeTruthy()
    expect(screen.queryByTestId('settings')).toBeTruthy()
    expect(screen.queryByTestId('landing')).toBeNull()
  })

  it('renders BidWorkspace at /bids/:id', () => {
    render(<MemoryRouter initialEntries={['/bids/abc']}><App /></MemoryRouter>)
    expect(screen.queryByTestId('workspace')).toBeTruthy()
  })
})

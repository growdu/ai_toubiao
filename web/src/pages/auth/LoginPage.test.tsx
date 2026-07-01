import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import LoginPage from './LoginPage'
import { useAuthStore } from '../../lib/auth'

// Hoisted mocks: react-router-dom and the auth API. We don't render a real
// router, just enough to satisfy LoginPage's useNavigate hook.
const navigateMock = vi.fn()

vi.mock('react-router-dom', async (importOriginal) => {
  const actual = await importOriginal<typeof import('react-router-dom')>()
  return { ...actual, useNavigate: () => navigateMock }
})

vi.mock('../../api/auth', () => ({
  authApi: {
    login: vi.fn(),
  },
}))

import { authApi } from '../../api/auth'

// Import after vi.mock so we get the mocked version.
const mockedLogin = vi.mocked(authApi.login)

function renderLogin() {
  return render(
    <MemoryRouter initialEntries={['/login']}>
      <LoginPage />
    </MemoryRouter>
  )
}

describe('LoginPage', () => {
  beforeEach(() => {
    useAuthStore.setState({ token: null, userId: null, tenantId: null })
    navigateMock.mockClear()
    mockedLogin.mockReset()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('renders email + password inputs and a submit button', () => {
    renderLogin()
    expect(screen.getByPlaceholderText('admin@example.com')).toBeInTheDocument()
    expect(screen.getByPlaceholderText('••••••••')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '登录' })).toBeInTheDocument()
  })

  it('submits credentials, calls setAuth, and navigates to /bids on success', async () => {
    mockedLogin.mockResolvedValueOnce({
      data: { token: 'tok', user_id: 'u-1', tenant_id: 't-1' },
    } as any)

    const user = userEvent.setup()
    renderLogin()

    await user.type(screen.getByPlaceholderText('admin@example.com'), 'admin@example.com')
    await user.type(screen.getByPlaceholderText('••••••••'), 'hunter2')
    await user.click(screen.getByRole('button', { name: '登录' }))

    await waitFor(() => {
      expect(navigateMock).toHaveBeenCalledWith('/bids')
    })
    expect(mockedLogin).toHaveBeenCalledWith({ email: 'admin@example.com', password: 'hunter2' })
    expect(useAuthStore.getState().token).toBe('tok')
    expect(useAuthStore.getState().userId).toBe('u-1')
    expect(useAuthStore.getState().tenantId).toBe('t-1')
  })

  it('displays server error message when login fails', async () => {
    mockedLogin.mockRejectedValueOnce({
      response: { data: { message: '账号或密码错误' } },
    })

    const user = userEvent.setup()
    renderLogin()

    await user.type(screen.getByPlaceholderText('admin@example.com'), 'bad@x.com')
    await user.type(screen.getByPlaceholderText('••••••••'), 'wrong')
    await user.click(screen.getByRole('button', { name: '登录' }))

    expect(await screen.findByText('账号或密码错误')).toBeInTheDocument()
    expect(navigateMock).not.toHaveBeenCalled()
    expect(useAuthStore.getState().token).toBeNull()
  })

  it('falls back to a generic message when the error has no response body', async () => {
    mockedLogin.mockRejectedValueOnce(new Error('network down'))

    const user = userEvent.setup()
    renderLogin()

    await user.type(screen.getByPlaceholderText('admin@example.com'), 'a@b.com')
    await user.type(screen.getByPlaceholderText('••••••••'), 'p')
    await user.click(screen.getByRole('button', { name: '登录' }))

    expect(await screen.findByText('登录失败')).toBeInTheDocument()
  })

  it('disables the submit button and shows "登录中..." while a request is in flight', async () => {
    // Never-resolving promise: keeps the page in the "loading" state.
    mockedLogin.mockImplementation(() => new Promise(() => {}))

    const user = userEvent.setup()
    renderLogin()

    await user.type(screen.getByPlaceholderText('admin@example.com'), 'a@b.com')
    await user.type(screen.getByPlaceholderText('••••••••'), 'p')
    await user.click(screen.getByRole('button', { name: '登录' }))

    expect(await screen.findByText('登录中...')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '登录中...' })).toBeDisabled()
  })
})
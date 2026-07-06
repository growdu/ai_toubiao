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
    extractAuthInfo: vi.fn(),
  },
}))

import { authApi } from '../../api/auth'

// Import after vi.mock so we get the mocked version.
const mockedLogin = vi.mocked(authApi.login)
const mockedExtract = vi.mocked(authApi.extractAuthInfo)

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
    mockedExtract.mockReset()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('renders tenant + email + password inputs and a submit button', () => {
    renderLogin()
    expect(screen.getByPlaceholderText('demo-a')).toBeInTheDocument()
    expect(screen.getByPlaceholderText('admin@example.com')).toBeInTheDocument()
    expect(screen.getByPlaceholderText('••••••••')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '登录' })).toBeInTheDocument()
  })

  it('submits credentials, calls setAuth, and navigates to /bids on success', async () => {
    mockedLogin.mockResolvedValueOnce({ data: {} } as any)
    mockedExtract.mockReturnValueOnce({ token: 'tok', userId: 'u-1', tenantId: 't-1' })

    const user = userEvent.setup()
    renderLogin()

    // Default form values match demo-a / admin@demo-a.test / password123, so we
    // just submit. Override password to verify it's sent.
    await user.clear(screen.getByPlaceholderText('••••••••'))
    await user.type(screen.getByPlaceholderText('••••••••'), 'hunter2')

    await user.click(screen.getByRole('button', { name: '登录' }))

    await waitFor(() => {
      expect(navigateMock).toHaveBeenCalledWith('/bids')
    })
    expect(mockedLogin).toHaveBeenCalledWith({
      tenant_slug: 'demo-a',
      email: 'admin@demo-a.test',
      password: 'hunter2',
    })
    expect(mockedExtract).toHaveBeenCalled()
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

    await user.click(screen.getByRole('button', { name: '登录' }))

    // The error banner now has a heading + body; both should render so the
    // user immediately knows what went wrong without squinting.
    expect(await screen.findByText('账号或密码错误')).toBeInTheDocument()
    expect(screen.getByText('登录失败')).toBeInTheDocument()
    expect(navigateMock).not.toHaveBeenCalled()
    expect(useAuthStore.getState().token).toBeNull()
  })

  it('shows a credential-specific hint when the server message mentions 邮箱 or 密码', async () => {
    mockedLogin.mockRejectedValueOnce({
      response: { data: { message: '邮箱或密码错误' } },
    })

    const user = userEvent.setup()
    renderLogin()

    await user.click(screen.getByRole('button', { name: '登录' }))

    // The banner includes a hint line that names the default demo password.
    // This was the literal missing feedback that caused the "clicked login
    // but nothing happened" report — now it surfaces in the same view.
    expect(await screen.findByText(/默认 demo 账号密码为 password123/)).toBeInTheDocument()
  })

  it('returns focus to the password input after a failed submit so the user can retype immediately', async () => {
    mockedLogin.mockRejectedValueOnce({
      response: { data: { message: '邮箱或密码错误' } },
    })

    const user = userEvent.setup()
    renderLogin()

    // The submit button is the active element after .click(); the focus pull-
    // back happens on a setTimeout(0). Note: the form re-mounts due to
    // key={shakeKey}, so we re-query the input by placeholder — the new one
    // will be the focused element after the remount + timeout.
    await user.click(screen.getByRole('button', { name: '登录' }))
    const pwInputAfter = await screen.findByPlaceholderText('••••••••') as HTMLInputElement
    await waitFor(() => expect(document.activeElement).toBe(pwInputAfter))
  })

  it('triggers the shake animation on the form when submit fails (key-based replay)', async () => {
    mockedLogin.mockRejectedValueOnce({ response: { data: { message: 'no' } } })

    const user = userEvent.setup()
    renderLogin()
    const formBefore = document.querySelector('form')!

    await user.click(screen.getByRole('button', { name: '登录' }))
    await screen.findByText('no')

    // The form's `key={shakeKey}` is bumped on every failed submit; React
    // re-mounts the <form> element, so it must now be a different DOM node.
    // (An identical reference would mean the key didn't change.)
    const formAfter = document.querySelector('form')!
    expect(formAfter).not.toBe(formBefore)
    expect(formAfter.className).toContain('space-y-4')
  })

  it('falls back to a generic message when the error has no response body', async () => {
    mockedLogin.mockRejectedValueOnce(new Error('network down'))

    const user = userEvent.setup()
    renderLogin()

    await user.click(screen.getByRole('button', { name: '登录' }))

    // LoginPage's fallback chain (response.data.message → response.data.error.message
    // → '登录失败') doesn't unwrap a plain Error.message, so the banner shows
    // the generic Chinese string twice — once as the banner title, once as
    // the body. Assert the banner title exists; the body text is '登录失败'
    // too, which getAllByText confirms.
    expect(await screen.findAllByText('登录失败')).not.toHaveLength(0)
  })

  it('disables the submit button and shows "登录中..." while a request is in flight', async () => {
    // Never-resolving promise: keeps the page in the "loading" state.
    mockedLogin.mockImplementation(() => new Promise(() => {}))

    const user = userEvent.setup()
    renderLogin()

    await user.click(screen.getByRole('button', { name: '登录' }))

    expect(await screen.findByText('登录中...')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '登录中...' })).toBeDisabled()
  })

  it('shows the inline error when login returns no token', async () => {
    mockedLogin.mockResolvedValueOnce({ data: {} } as any)
    mockedExtract.mockReturnValueOnce({ token: '', userId: '', tenantId: '' })

    const user = userEvent.setup()
    renderLogin()

    await user.click(screen.getByRole('button', { name: '登录' }))

    expect(await screen.findByText('登录返回无效，未收到 token')).toBeInTheDocument()
    expect(navigateMock).not.toHaveBeenCalled()
  })
})
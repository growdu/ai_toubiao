import { describe, it, expect } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import RegisterPage from './RegisterPage'

// Helper: type a string into a controlled React input by character. This
// is the most reliable cross-version way to drive a controlled input in
// jsdom — each char triggers its own React onChange with the new value
// built up character-by-character. We avoid userEvent.type because v14
// parses `[` as a keyboard-shortcut prefix and chokes on email strings
// like "[email protected]".
async function typeInto(el: HTMLInputElement, value: string) {
  for (const ch of value) {
    fireEvent.input(el, { target: { value: el.value + ch } })
  }
}

// RegisterPage tests. The form is currently a stub (no backend endpoint),
// so we assert the UX guardrails: tenant slug auto-derives from tenant
// name, password length validation runs client-side, and the agree-to-
// terms checkbox blocks submit.

function renderRegister(initialEntries: string[] = ['/register']) {
  return render(
    <MemoryRouter initialEntries={initialEntries}>
      <RegisterPage />
    </MemoryRouter>
  )
}

describe('RegisterPage', () => {
  it('renders all four required inputs and a submit button', () => {
    renderRegister()
    expect(screen.getByLabelText(/工作区名称/)).toBeInTheDocument()
    expect(screen.getByLabelText(/工作区标识/)).toBeInTheDocument()
    expect(screen.getByLabelText(/工作邮箱/)).toBeInTheDocument()
    expect(screen.getByLabelText(/密码/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /创建账号/ })).toBeInTheDocument()
  })

  it('auto-derives tenant slug from the tenant name as you type', async () => {
    renderRegister()
    const tenantInput = screen.getByLabelText(/工作区名称/) as HTMLInputElement
    await typeInto(tenantInput, '建工集团')
    expect(screen.getByLabelText(/工作区标识/)).toHaveValue('建工集团')

    // Manually set a different slug. Auto-derive should stop
    // overriding once the user has typed something that doesn't match
    // the derived-from-name value.
    const slugInput = screen.getByLabelText(/工作区标识/) as HTMLInputElement
    await typeInto(slugInput, 'custom-slug')
    await typeInto(tenantInput, '建工集团投标部')
    expect(screen.getByLabelText(/工作区标识/)).toHaveValue('custom-slug')
  })

  it('blocks submit when terms checkbox is not checked', async () => {
    renderRegister()
    // Fill all required fields with valid data. validation order in
    // the component is tenant_name → tenant_slug → password → terms;
    // since we satisfy everything except terms, the terms error must
    // be the one that surfaces.
    const tenantInput = screen.getByLabelText(/工作区名称/) as HTMLInputElement
    const slugInput = screen.getByLabelText(/工作区标识/) as HTMLInputElement
    const emailInput = screen.getByLabelText(/工作邮箱/) as HTMLInputElement
    const pwInput = screen.getByLabelText(/密码/) as HTMLInputElement
    await typeInto(tenantInput, '测试租户')
    await typeInto(slugInput, 'testco')
    await typeInto(emailInput, 'admin')
    await typeInto(emailInput, '@')
    await typeInto(emailInput, 'example.com')
    await typeInto(pwInput, 'longenoughpw')
    // deliberately leave the agree checkbox unchecked

    fireEvent.click(screen.getByRole('button', { name: /创建账号/ }))
    expect(await screen.findByText('请先同意服务条款和隐私政策')).toBeInTheDocument()
  })

  it('blocks submit when password is shorter than 8 chars', async () => {
    renderRegister()
    // tenant + slug are valid, email is valid, password is too short.
    // Validation short-circuits on the password rule so the password
    // error must be the one that surfaces — even without ticking the
    // agree checkbox (which is checked later in the chain).
    const tenantInput = screen.getByLabelText(/工作区名称/) as HTMLInputElement
    const slugInput = screen.getByLabelText(/工作区标识/) as HTMLInputElement
    const emailInput = screen.getByLabelText(/工作邮箱/) as HTMLInputElement
    const pwInput = screen.getByLabelText(/密码/) as HTMLInputElement
    await typeInto(tenantInput, '测试租户')
    await typeInto(slugInput, 'testco')
    await typeInto(emailInput, 'admin')
    await typeInto(emailInput, '@')
    await typeInto(emailInput, 'example.com')
    await typeInto(pwInput, 'short')

    fireEvent.click(screen.getByRole('button', { name: /创建账号/ }))
    expect(await screen.findByText('密码至少 8 位')).toBeInTheDocument()
  })

  it('shows plan subtitle from the ?plan= query param', () => {
    renderRegister(['/register?plan=pro'])
    expect(screen.getByText(/专业版 · 14 天免费试用/)).toBeInTheDocument()
  })

  it('offers a link to /login for users who already have an account', () => {
    renderRegister()
    const link = screen.getByRole('link', { name: /直接登录/ })
    expect(link).toHaveAttribute('href', '/login')
  })
})

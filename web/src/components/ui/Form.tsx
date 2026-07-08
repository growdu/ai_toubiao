import { InputHTMLAttributes, TextareaHTMLAttributes, SelectHTMLAttributes, forwardRef, useId, ReactNode } from 'react'

const baseField = 'w-full bg-white border border-ink-200 rounded-lg text-sm text-ink-800 shadow-inset-soft transition focus:outline-none focus:border-brand-400 focus:ring-2 focus:ring-brand-100 disabled:bg-ink-50 disabled:text-ink-400 disabled:cursor-not-allowed'

interface FieldProps {
  label?: string
  hint?: string
  error?: string
  required?: boolean
  leftIcon?: React.ReactNode
  rightIcon?: React.ReactNode
}

// Field is the labelled wrapper used by TextInput / TextArea / Select.
// We use `useId` to wire the inner control to the <label htmlFor=...> so
// screen readers and @testing-library/react's getByLabelText can find
// inputs by their visible label. The previous implementation just
// wrapped the input in <label>, which RTL didn't recognise.
//
// A11y (WCAG 1.3.1 + 3.3.1):
//   - When `error` is set, the error span gets role="alert" so screen
//     readers announce it the instant it appears (not on next tab).
//   - The error/hint span carries a `*-error` / `*-hint` id; the
//     control wires `aria-describedby` to that id so AT reads the
//     supporting text after the field name.
//   - Each control sets `aria-invalid="true"` when in error state;
//     this changes the speech-rhythm cue before the message itself.
export function Field({ label, hint, error, required, children, htmlFor }: FieldProps & { children: ReactNode; htmlFor?: string }) {
  return (
    <div className="block">
      {label && (
        <label htmlFor={htmlFor} className="block text-sm font-medium text-ink-700 mb-1.5">
          {label}
          {required && <span className="text-red-500 ml-0.5" aria-hidden>*</span>}
        </label>
      )}
      {children}
      {error ? (
        <span
          id={htmlFor ? `${htmlFor}-error` : undefined}
          role="alert"
          className="block mt-1 text-xs text-red-600"
        >
          {error}
        </span>
      ) : hint ? (
        <span id={htmlFor ? `${htmlFor}-hint` : undefined} className="block mt-1 text-xs text-ink-400">
          {hint}
        </span>
      ) : null}
    </div>
  )
}

export const TextInput = forwardRef<HTMLInputElement, InputHTMLAttributes<HTMLInputElement> & FieldProps>(function TextInput(
  { label, hint, error, required, leftIcon, rightIcon, className = '', id, ...rest }, ref,
) {
  const autoId = useId()
  const inputId = id ?? autoId
  // aria-invalid + aria-describedby wire the input to its error/hint
  // so screen readers announce "email: invalid entry — 邮箱格式不对"
  // rather than only the visible message. Set only when the relevant
  // state is present (don't ship aria-invalid="false" — that's noise).
  const inputEl = (
    <div className="relative">
      {leftIcon && (
        <span className="absolute left-3 top-1/2 -translate-y-1/2 text-ink-400 pointer-events-none">
          {leftIcon}
        </span>
      )}
      <input
        id={inputId}
        ref={ref}
        aria-invalid={error ? 'true' : undefined}
        aria-describedby={error ? `${inputId}-error` : hint ? `${inputId}-hint` : undefined}
        className={`${baseField} ${leftIcon ? 'pl-9' : ''} ${rightIcon ? 'pr-9' : ''} ${className}`}
        {...rest}
      />
      {rightIcon && (
        <span className="absolute right-3 top-1/2 -translate-y-1/2 text-ink-400 pointer-events-none">
          {rightIcon}
        </span>
      )}
    </div>
  )
  if (!label && !hint && !error) return inputEl
  return <Field label={label} hint={hint} error={error} required={required} htmlFor={inputId}>{inputEl}</Field>
})

export const TextArea = forwardRef<HTMLTextAreaElement, TextareaHTMLAttributes<HTMLTextAreaElement> & FieldProps>(function TextArea(
  { label, hint, error, required, className = '', id, ...rest }, ref,
) {
  const autoId = useId()
  const inputId = id ?? autoId
  const el = (
    <textarea
      id={inputId}
      ref={ref}
      aria-invalid={error ? 'true' : undefined}
      aria-describedby={error ? `${inputId}-error` : hint ? `${inputId}-hint` : undefined}
      className={`${baseField} px-3 py-2 resize-y ${className}`}
      {...rest}
    />
  )
  if (!label && !hint && !error) return el
  return <Field label={label} hint={hint} error={error} required={required} htmlFor={inputId}>{el}</Field>
})

export const Select = forwardRef<HTMLSelectElement, SelectHTMLAttributes<HTMLSelectElement> & FieldProps>(function Select(
  { label, hint, error, required, className = '', id, children, ...rest }, ref,
) {
  const autoId = useId()
  const inputId = id ?? autoId
  const el = (
    <select
      id={inputId}
      ref={ref}
      aria-invalid={error ? 'true' : undefined}
      aria-describedby={error ? `${inputId}-error` : hint ? `${inputId}-hint` : undefined}
      className={`${baseField} px-3 py-2 pr-8 bg-no-repeat bg-[length:14px_14px] bg-[right_10px_center] ${className}`}
      style={{
        backgroundImage:
          "url(\"data:image/svg+xml;utf8,<svg xmlns='http://www.w3.org/2000/svg' width='14' height='14' viewBox='0 0 24 24' fill='none' stroke='%235e6a82' stroke-width='2.2' stroke-linecap='round' stroke-linejoin='round'><polyline points='6 9 12 15 18 9'/></svg>\")",
      }}
      {...rest}
    >
      {children}
    </select>
  )
  if (!label && !hint && !error) return el
  return <Field label={label} hint={hint} error={error} required={required} htmlFor={inputId}>{el}</Field>
})

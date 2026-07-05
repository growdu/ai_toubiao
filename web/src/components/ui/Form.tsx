import { InputHTMLAttributes, TextareaHTMLAttributes, SelectHTMLAttributes, forwardRef } from 'react'

const baseField = 'w-full bg-white border border-ink-200 rounded-lg text-sm text-ink-800 shadow-inset-soft transition focus:outline-none focus:border-brand-400 focus:ring-2 focus:ring-brand-100 disabled:bg-ink-50 disabled:text-ink-400 disabled:cursor-not-allowed'

interface FieldProps {
  label?: string
  hint?: string
  error?: string
  required?: boolean
}

export function Field({ label, hint, error, required, children }: FieldProps & { children: React.ReactNode }) {
  return (
    <label className="block">
      {label && (
        <span className="block text-sm font-medium text-ink-700 mb-1.5">
          {label}
          {required && <span className="text-red-500 ml-0.5">*</span>}
        </span>
      )}
      {children}
      {error ? (
        <span className="block mt-1 text-xs text-red-600">{error}</span>
      ) : hint ? (
        <span className="block mt-1 text-xs text-ink-400">{hint}</span>
      ) : null}
    </label>
  )
}

export const TextInput = forwardRef<HTMLInputElement, InputHTMLAttributes<HTMLInputElement> & FieldProps>(function TextInput(
  { label, hint, error, required, className = '', ...rest }, ref,
) {
  const inputEl = (
    <input
      ref={ref}
      className={`${baseField} px-3 py-2 ${className}`}
      {...rest}
    />
  )
  if (!label && !hint && !error) return inputEl
  return <Field label={label} hint={hint} error={error} required={required}>{inputEl}</Field>
})

export const TextArea = forwardRef<HTMLTextAreaElement, TextareaHTMLAttributes<HTMLTextAreaElement> & FieldProps>(function TextArea(
  { label, hint, error, required, className = '', ...rest }, ref,
) {
  const el = (
    <textarea
      ref={ref}
      className={`${baseField} px-3 py-2 resize-y ${className}`}
      {...rest}
    />
  )
  if (!label && !hint && !error) return el
  return <Field label={label} hint={hint} error={error} required={required}>{el}</Field>
})

export const Select = forwardRef<HTMLSelectElement, SelectHTMLAttributes<HTMLSelectElement> & FieldProps>(function Select(
  { label, hint, error, required, className = '', children, ...rest }, ref,
) {
  const el = (
    <select
      ref={ref}
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
  return <Field label={label} hint={hint} error={error} required={required}>{el}</Field>
})
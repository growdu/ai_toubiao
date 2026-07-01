import { describe, it, expect } from 'vitest'
import { filenameFromDisposition } from './bids'

describe('filenameFromDisposition', () => {
  it('returns null for missing header', () => {
    expect(filenameFromDisposition(undefined)).toBeNull()
    expect(filenameFromDisposition('')).toBeNull()
  })

  it('parses unquoted filename=', () => {
    expect(filenameFromDisposition('attachment; filename=bid.docx')).toBe('bid.docx')
  })

  it('parses double-quoted filename=', () => {
    expect(filenameFromDisposition('attachment; filename="bid final.docx"')).toBe('bid final.docx')
  })

  it('parses RFC 5987 filename* with UTF-8 encoding', () => {
    // RFC 5987: filename*=UTF-8''<percent-encoded>
    expect(filenameFromDisposition("attachment; filename*=UTF-8''%E6%A0%87%E4%B9%A6.docx")).toBe('标书.docx')
  })

  it('parses filename* without UTF-8 prefix (plain URI)', () => {
    expect(filenameFromDisposition('attachment; filename*=report.pdf')).toBe('report.pdf')
  })

  it('returns the raw value when percent-decoding fails', () => {
    // '%E6%A0' is an incomplete UTF-8 sequence; decoder should fall back gracefully.
    const raw = '%E6%A0'
    const result = filenameFromDisposition(`attachment; filename="${raw}"`)
    expect(result).toBe(raw) // caught decode error returns m[1] as-is
  })

  it('returns null for header without filename param', () => {
    expect(filenameFromDisposition('attachment')).toBeNull()
    expect(filenameFromDisposition('inline; size=12345')).toBeNull()
  })

  it('is case-insensitive on the parameter name', () => {
    expect(filenameFromDisposition('attachment; FILENAME=upper.docx')).toBe('upper.docx')
  })

  it('trims trailing quote artifacts before decoding', () => {
    // The regex captures up to the first `"` or `;`, so trailing junk is ignored.
    expect(filenameFromDisposition('attachment; filename="a.docx"; size=10')).toBe('a.docx')
  })
})
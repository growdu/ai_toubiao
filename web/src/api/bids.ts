import api from './client'

export interface BidJob {
  id: string
  project_id: string
  status: string
  current_step: string
  project_name: string
  industry: string
  total_chapters: number
  done_chapters: number
  created_at: string
  updated_at: string
}

export interface CreateBidRequest {
  project_id: string
  rfp_document_id?: string
}

export const bidsApi = {
  list: (params?: { project_id?: string; limit?: number; cursor?: string }) =>
    api.get<{ data: BidJob[]; meta: { count: number } }>('/bids', { params }),

  get: (id: string) => api.get<{ data: BidJob }>(`/bids/${id}`),

  create: (data: CreateBidRequest) => api.post<{ data: BidJob }>('/bids', data),

  pause: (id: string) => api.post<{ data: BidJob }>(`/bids/${id}/pause`),

  resume: (id: string) => api.post<{ data: BidJob }>(`/bids/${id}/resume`),

  getOutline: (id: string) =>
    api.get<{ data: ChapterSpec[] }>(`/bids/${id}/outline`),

  updateOutline: (id: string, data: ChapterSpec[]) =>
    api.put<{ data: ChapterSpec[] }>(`/bids/${id}/outline`, { chapters: data }),

  getAuditReport: (id: string) =>
    api.get<{ data: AuditReport }>(`/bids/${id}/audit-report`),

  resolveIssues: (id: string, data: { issue_id: string; action: string }[]) =>
    api.post<{ data: AuditReport }>(`/bids/${id}/resolve-issues`, { issues: data }),

  /** Export the bid as a Word (.docx) file. Returns the raw blob and suggested filename. */
  exportWord: async (id: string): Promise<{ blob: Blob; filename: string }> => {
    const res = await api.get<Blob>(`/bids/${id}/export/word`, {
      responseType: 'blob',
    })
    return { blob: res.data, filename: filenameFromDisposition(res.headers['content-disposition']) ?? `bid_${id}.docx` }
  },

  /** Export the bid as a PDF file. MVP falls back to .docx until LibreOffice is wired in. */
  exportPdf: async (id: string): Promise<{ blob: Blob; filename: string }> => {
    const res = await api.get<Blob>(`/bids/${id}/export/pdf`, {
      responseType: 'blob',
    })
    return { blob: res.data, filename: filenameFromDisposition(res.headers['content-disposition']) ?? `bid_${id}.pdf` }
  },
}

/** Parse `filename=...` (RFC 6266) from a Content-Disposition header value; supports quoted and unquoted forms.
 *  Exported for unit testing. */
export function filenameFromDisposition(header: string | undefined): string | null {
  if (!header) return null
  const m = header.match(/filename\*?=(?:UTF-8'')?"?([^";]+)"?/i)
  if (!m) return null
  try {
    return decodeURIComponent(m[1])
  } catch {
    return m[1]
  }
}

export interface ChapterSpec {
  id: string
  title: string
  level: number
  order_index: number
  chapter_type: string
  target_word_count: number
  min_word_count: number
  status: string
  priority: string
}

export interface AuditReport {
  bid_job_id: string
  total_issues: number
  critical: number
  major: number
  minor: number
  issues: AuditIssue[]
}

export interface AuditIssue {
  id: string
  chapter_id: string
  chapter_title: string
  dimension: string
  severity: string
  issue: string
  suggestion: string
  status: string
}
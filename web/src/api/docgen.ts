import api from './client'

export interface DocgenChapter {
  title: string
  content: string
  level?: number
  sort_order?: number
}

export interface DocgenRenderRequest {
  title?: string
  format?: string
  chapters: DocgenChapter[]
}

export interface DocgenRenderResult {
  download_url: string
  task_id: string
  chapters: number
  figures: number
}

export interface DocgenLearnRequest {
  chapters: Array<{
    title: string
    content: string
    word_count?: number
  }>
  industry?: string
  rfp_type?: string
  label?: string
}

export interface BidPattern {
  id: string
  industry: string
  rfp_type: string
  outline_template: string
  quality_score: number
  label: string
}

export const docgenApi = {
  /** Render chapters into a rich .docx (mermaid + chart rendering, title page). */
  render: (data: DocgenRenderRequest) =>
    api.post<DocgenRenderResult>('/docgen/render', data),

  /** Download a rendered document by task ID. */
  download: async (taskId: string): Promise<{ blob: Blob; filename: string }> => {
    const res = await api.get<Blob>(`/docgen/download/${taskId}`, { responseType: 'blob' })
    const cd = res.headers['content-disposition'] as string | undefined
    const m = cd?.match(/filename\*?=(?:UTF-8'')?"?([^";]+)"?/i)
    const filename = m ? decodeURIComponent(m[1]) : `document_${taskId}.docx`
    return { blob: res.data, filename }
  },

  /** Submit a completed bid for pattern extraction + Bandit update. */
  learn: (data: DocgenLearnRequest) =>
    api.post<{ status: string; quality_score: number; pattern_id: string }>('/docgen/learn', data),

  /** Retrieve historical bid patterns for outline reference. */
  patterns: (params?: { industry?: string; rfp_type?: string; top_k?: number }) =>
    api.get<{ patterns: BidPattern[]; count: number }>('/docgen/patterns', { params }),
}

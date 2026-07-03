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
  version: number
}

export interface CreateBidRequest {
  project_id: string
  rfp_document_id?: string
}

export interface ChapterSpec {
  id: string
  bid_job_id: string
  parent_id?: string
  title: string
  level: number
  order_index: number
  chapter_type: string
  target_word_count: number
  min_word_count: number
  writing_style: string
  priority: string
  status: string
}

export interface ChapterContent {
  chapter_spec_id: string
  version: number
  content_text: string
  word_count: number
  min_word_met: boolean
  generated_by: string
  llm_model?: string
  generation_duration_ms?: number
  status?: string
}

export interface AddChapterRequest {
  title: string
  level?: number
  order_index?: number
  parent_id?: string
  target_word_count?: number
  min_word_count?: number
  writing_style?: string
  priority?: string
}

export interface UpdateChapterRequest {
  title?: string
  level?: number
  order_index?: number
  target_word_count?: number
  min_word_count?: number
  priority?: string
  status?: string
}

export const bidsApi = {
  list: (params?: { project_id?: string; limit?: number; cursor?: string }) =>
    api.get<{ data: BidJob[]; meta: { count: number } }>('/bids', { params }),

  get: (id: string) => api.get<{ data: BidJob }>(`/bids/${id}`),

  create: (data: CreateBidRequest) => api.post<{ data: BidJob }>('/bids', data),

  // Chapter outline CRUD
  getOutline: (id: string) =>
    api.get<{ data: ChapterSpec[] }>(`/bids/${id}/outline`),

  addChapter: (id: string, data: AddChapterRequest) =>
    api.post<{ data: ChapterSpec }>(`/bids/${id}/outline`, data),

  updateChapter: (bidId: string, chapterId: string, data: UpdateChapterRequest) =>
    api.put<{ data: any }>(`/bids/${bidId}/chapters/${chapterId}`, data),

  deleteChapter: (bidId: string, chapterId: string) =>
    api.delete<{ data: any }>(`/bids/${bidId}/chapters/${chapterId}`),

  // Save chapter content (user edit)
  saveChapterContent: (bidId: string, chapterId: string, contentText: string) =>
    api.put<{ data: ChapterContent }>(`/bids/${bidId}/chapters/${chapterId}/content`, { content_text: contentText }),

  // Save RFP material text
  saveMaterial: (bidId: string, materialText: string) =>
    api.put<{ data: { status: string } }>(`/bids/${bidId}/material`, { material_text: materialText }),

  // Chapter generation
  generateChapter: (bidId: string, chapterId: string) =>
    api.post<{ data: { chapter_id: string; status: string; message: string } }>(`/bids/${bidId}/chapters/${chapterId}/generate`),

  // Chapter content
  getChapterContent: (bidId: string, chapterId: string) =>
    api.get<{ data: ChapterContent }>(`/bids/${bidId}/chapters/${chapterId}/content`),

  // Workflow control
  pause: (id: string) => api.post<{ data: BidJob }>(`/bids/${id}/pause`),
  resume: (id: string) => api.post<{ data: BidJob }>(`/bids/${id}/resume`),

  transition: (id: string, to: string, version?: number, reason?: string) =>
    api.post<{ data: BidJob }>(`/bids/${id}/transition${version ? `?version=${version}` : ''}`, { to, reason: reason || '' }),

  // Export
  exportWord: async (id: string): Promise<{ blob: Blob; filename: string }> => {
    const res = await api.get<Blob>(`/bids/${id}/export/word`, { responseType: 'blob' })
    return { blob: res.data, filename: filenameFromDisposition(res.headers['content-disposition']) ?? `bid_${id}.docx` }
  },

  exportPdf: async (id: string): Promise<{ blob: Blob; filename: string }> => {
    const res = await api.get<Blob>(`/bids/${id}/export/pdf`, { responseType: 'blob' })
    return { blob: res.data, filename: filenameFromDisposition(res.headers['content-disposition']) ?? `bid_${id}.pdf` }
  },
}

export function filenameFromDisposition(header: string | undefined): string | null {
  if (!header) return null
  const m = header.match(/filename\*?=(?:UTF-8'')?"?([^";]+)"?/i)
  if (!m) return null
  try { return decodeURIComponent(m[1]) } catch { return m[1] }
}

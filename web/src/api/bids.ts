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
  approved_at?: string
  approved_by?: string
  rejection_reason?: string
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

  // Approval state — frontend tracks it in the chapter's status field.
  // The backend exposes approve/reject endpoints (see approveChapter and
  // rejectChapter below). A chapter with status='approved' has been
  // human-vetted and is allowed to flow into the audit step.
  approveChapter: (bidId: string, chapterId: string) =>
    api.post<{ data: { id: string; status: string; approved_at: string } }>(
      `/bids/${bidId}/chapters/${chapterId}/approve`, {}),
  rejectChapter: (bidId: string, chapterId: string, reason?: string) =>
    api.post<{ data: { id: string; status: string } }>(
      `/bids/${bidId}/chapters/${chapterId}/reject`, { reason: reason || '' }),
  reorderOutline: (bidId: string, ordered: Array<{ id: string; parent_id?: string | null }>) =>
    api.post<{ data: { ok: true } }>(`/bids/${bidId}/outline/reorder`, { ordered }),

  deleteChapter: (bidId: string, chapterId: string) =>
    api.delete<{ data: any }>(`/bids/${bidId}/chapters/${chapterId}`),

  // Save chapter content (user edit)
  //
  // We send `expected_version` so the backend can reject stale saves
  // with 409 VERSION_CONFLICT — guards against two browser tabs
  // silently clobbering each other. The frontend catches 409 in
  // BidWorkspace.saveContentMutation and re-reads + prompts the user.
  saveChapterContent: (bidId: string, chapterId: string, contentText: string, expectedVersion?: number) =>
    api.put<{ data: ChapterContent }>(`/bids/${bidId}/chapters/${chapterId}/content`, {
      content_text: contentText,
      ...(expectedVersion !== undefined ? { expected_version: expectedVersion } : {}),
    }),

  // Save RFP material text
  saveMaterial: (bidId: string, materialText: string) =>
    api.put<{ data: { status: string } }>(`/bids/${bidId}/material`, { material_text: materialText }),

  // Chapter generation
  // `prompt` is an optional per-chapter user instruction surfaced by
  // ChapterInspector "提示词" tab. The backend forwards it to the LLM
  // as an explicit user-message section so the generated content
  // reflects what the user asked for. Empty string = no custom prompt.
  generateChapter: (bidId: string, chapterId: string, prompt?: string) =>
    api.post<{ data: { chapter_id: string; status: string; message: string } }>(`/bids/${bidId}/chapters/${chapterId}/generate`,
      prompt ? { prompt } : {}),

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

// 材料解析（4 步向导步骤1/2）：解析、读取、更新 bid_jobs.parse_result。
// parseMaterial 调 router-svc 把原始材料提取为结构化字段；getParse 读取
// 已保存结果供步骤2审核编辑；updateParse 保存用户编辑后的结果。
export const parseApi = {
  parseMaterial: (id: string, materialText?: string) =>
    api.post<{ data: { material_text: string; parsed: any } }>(`/bids/${id}/parse`, materialText ? { material_text: materialText } : {}),
  getParse: (id: string) =>
    api.get<{ data: { material_text: string; parsed: any } }>(`/bids/${id}/parse`),
  updateParse: (id: string, data: { material_text?: string; parsed: any }) =>
    api.put<{ data: { status: string } }>(`/bids/${id}/parse`, data),
}

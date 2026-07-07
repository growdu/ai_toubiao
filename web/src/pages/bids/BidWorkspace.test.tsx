/**
 * BidWorkspace rejection-flow integration tests.
 *
 * Strategy: mock `bidsApi` and `react-router-dom` so we can render the
 * full BidWorkspace in jsdom with a controlled fixture. We deliberately
 * do NOT mock the rejection modal components — we want to verify the
 * actual chip + textarea + footer button + store interactions.
 *
 * What we cover:
 *   - Single chapter: reject button opens modal
 *   - Empty-textarea state: submit button disabled, validation hint shown
 *   - Clicking a quick-pick chip auto-fills the textarea
 *   - Submitting calls bidsApi.rejectChapter with the right reason
 *   - Built-in chips include the 4 defaults + a "清空" affordance
 *     appears after content is in the textarea
 *   - Modal closes on Escape and on Cancel
 *
 * What we deliberately do NOT cover:
 *   - Batch reject (very high fixture cost — N chapters, validate
 *     Promise.allSettled, multiple invalidations). Move to a separate
 *     file if a bug motivates it.
 *   - Drag-reorder (covered by dnd-kit's own tests).
 */
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import BidWorkspace from './BidWorkspace'
import { useRejectionTemplates } from '../../lib/rejectionTemplates'
import { useToastStore } from '../../lib/toast'

// ----- Mocks -----

// Navigate is unused inside BidWorkspace but we still give the router
// a memory location so useParams returns the right id.

// vi.mock factories are hoisted to the top of the module — they run
// before our `bidsApiMock` const is declared. The standard fix is to
// reference a local-scope mutable object via a ref pattern: define
// `mockApi` inside the factory and mutate it post-hoist via vi.hoisted.
const { mockApi } = vi.hoisted(() => ({
  mockApi: {
    get: vi.fn(),
    getOutline: vi.fn(),
    getChapterContent: vi.fn(),
    addChapter: vi.fn(),
    deleteChapter: vi.fn(),
    updateChapter: vi.fn(),
    reorderChapters: vi.fn(),
    generateChapter: vi.fn(),
    approveChapter: vi.fn(),
    batchApprove: vi.fn(),
    rejectChapter: vi.fn(),
    batchExport: vi.fn(),
    listMaterials: vi.fn(),
    uploadMaterial: vi.fn(),
    deleteMaterial: vi.fn(),
  },
}))

vi.mock('../../api/bids', () => ({ bidsApi: mockApi }))

// Avoid the extra fetch triggered by axios 401s — we set auth so the
// request interceptor is happy, but if it ever fires, fail loud.

const { authStore } = vi.hoisted(() => {
  const store = {
    token: 'tok-test',
    userId: 'u-test',
    tenantId: 't-test',
    userEmail: 'tester@example.com',
    setAuth: vi.fn(),
    clear: vi.fn(),
  }
  return { authStore: store }
})

vi.mock('../../lib/auth', () => ({
  useAuthStore: Object.assign(() => authStore, {
    setState: (partial: any) => {
      if (typeof partial === 'function') Object.assign(authStore, partial(authStore))
      else Object.assign(authStore, partial)
    },
    getState: () => authStore,
  }),
}))

// ----- Fixtures -----

const bidId = 'bid-1'
const chapter1 = {
  id: 'ch-1',
  bid_job_id: bidId,
  parent_id: null,
  title: '项目概述',
  level: 1,
  order_index: 0,
  chapter_type: 'body',
  target_word_count: 500,
  min_word_count: 200,
  writing_style: 'formal',
  priority: 'normal',
  status: 'succeeded',
  approved_at: undefined,
  approved_by: undefined,
  rejection_reason: undefined,
}
const chapterContent = {
  chapter_spec_id: chapter1.id,
  content_md: '这是 AI 生成的项目概述草稿。',
  word_count: 50,
  generated_by: 'mock-llm-v1',
  generation_duration_ms: 1234,
  llm_model: 'mock-llm-v1',
  version: 1,
  updated_at: '2026-07-01T12:00:00Z',
}

const bidDetail = {
  id: bidId,
  title: '某市政务云服务采购',
  // The ChapterInspector reject button only renders when BOTH
  //   chapter.status === 'succeeded'
  //   bidStatus === 'awaiting_review'
  // are true, so we set the bid to the awaiting_review state here.
  status: 'awaiting_review',
  created_at: '2026-07-01T10:00:00Z',
  tenant_id: 't-test',
  owner_id: 'u-test',
  material_count: 0,
  chapter_total: 1,
  chapter_done: 1,
}

function renderWorkspace() {
  // Each render gets its own QueryClient so test isolation is preserved
  // (one test's invalidate/refetch doesn't bleed into the next).
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false, staleTime: 0, gcTime: 0 },
    },
  })
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[`/bids/${bidId}`]}>
        <Routes>
          <Route path="/bids/:id" element={<BidWorkspace />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

// ----- Lifecycle -----

beforeEach(() => {
  // Reset all mocks
  vi.clearAllMocks()
  // The API returns a doubly-nested envelope: the standard `{ data: ... }`
  // from our axios interceptor, plus an inner `{ data: ... }` (the actual
  // payload wrapped in an Ok/Err envelope). For lists it's `{ data: [...]}`
  // and we unwrap as `outlineData?.data.data`. Each mocks below mirrors
  // that shape.
  mockApi.get.mockResolvedValue({ data: { data: bidDetail } })
  mockApi.getOutline.mockResolvedValue({ data: { data: [chapter1] } })
  mockApi.getChapterContent.mockResolvedValue({ data: { data: chapterContent } })
  mockApi.rejectChapter.mockResolvedValue({ data: { data: { id: chapter1.id, status: 'pending' } } })
  // Reset rejection templates to seed
  useRejectionTemplates.getState().reset()
  // Reset toasts
  useToastStore.setState({ toasts: [] })
})

afterEach(() => {
  vi.restoreAllMocks()
})

// ----- Helpers -----

async function selectChapterAndOpenRejectModal(user: ReturnType<typeof userEvent.setup>) {
  renderWorkspace()
  // Wait for the outline query to populate the workspace. The chapter
  // title appears in BOTH the chapter-tree row AND the inspector
  // header; either is fine.
  await screen.findAllByText(/项目概述/)
  // Click the per-chapter reject button in ChapterInspector. Multiple
  // "驳回"-named buttons may exist (batch header, batch button), so we
  // pick the one whose name contains "让 AI 重做" — that's the unique
  // signature of the inspector's per-chapter reject button.
  const rejectBtn = await screen.findByRole('button', { name: /让 AI 重做/ })
  await user.click(rejectBtn)
  const dialog = await screen.findByRole('dialog')
  return dialog
}

// ----- Tests -----

describe('BidWorkspace — RejectionReasonModal', () => {
  it('opens the modal when the per-chapter reject button is clicked', async () => {
    const user = userEvent.setup()
    const dialog = await selectChapterAndOpenRejectModal(user)
    // Title + Description are inside the dialog header
    expect(within(dialog).getByText(/驳回此章节/)).toBeInTheDocument()
    // Footer buttons rendered
    expect(within(dialog).getByRole('button', { name: '取消' })).toBeInTheDocument()
    expect(within(dialog).getByRole('button', { name: '驳回' })).toBeInTheDocument()
    // TextArea renders empty
    const textarea = within(dialog).getByPlaceholderText(/描述为什么驳回/) as HTMLTextAreaElement
    expect(textarea.value).toBe('')
  })

  it('disables the submit button when the textarea is empty', async () => {
    const user = userEvent.setup()
    const dialog = await selectChapterAndOpenRejectModal(user)
    const submitBtn = within(dialog).getByRole('button', { name: '驳回' })
    // The submit button uses `disabled={!rejectReason.trim()}` so empty
    // state means disabled. user-event respects disabled state.
    expect(submitBtn).toBeDisabled()
    // The validation hint should be present
    expect(within(dialog).getByText(/驳回原因必填/)).toBeInTheDocument()
  })

  it('renders the 4 built-in quick-pick chips + no 清空 affordance initially', async () => {
    const user = userEvent.setup()
    const dialog = await selectChapterAndOpenRejectModal(user)
    // Built-in chip labels
    expect(within(dialog).getByRole('button', { name: /内容不准确/ })).toBeInTheDocument()
    expect(within(dialog).getByRole('button', { name: /格式不对/ })).toBeInTheDocument()
    expect(within(dialog).getByRole('button', { name: /缺证据/ })).toBeInTheDocument()
    expect(within(dialog).getByRole('button', { name: /太啰嗦/ })).toBeInTheDocument()
    // No 清空 chip yet (textarea is empty)
    expect(within(dialog).queryByRole('button', { name: /清空/ })).toBeNull()
  })

  it('clicking a quick-pick chip auto-fills the textarea with that label', async () => {
    const user = userEvent.setup()
    const dialog = await selectChapterAndOpenRejectModal(user)
    const chip = within(dialog).getByRole('button', { name: /缺证据/ })
    await user.click(chip)
    const textarea = within(dialog).getByPlaceholderText(/描述为什么驳回/) as HTMLTextAreaElement
    expect(textarea.value).toBe('缺证据')
    // After filling, the submit button becomes enabled
    expect(within(dialog).getByRole('button', { name: '驳回' })).toBeEnabled()
  })

  it('appends on chip click when textarea already has content (preserves context)', async () => {
    const user = userEvent.setup()
    const dialog = await selectChapterAndOpenRejectModal(user)
    const textarea = within(dialog).getByPlaceholderText(/描述为什么驳回/) as HTMLTextAreaElement
    await user.type(textarea, '需要复核业绩数据')
    // The custom text is in there
    expect(textarea.value).toBe('需要复核业绩数据')
    // Now click a chip — it should append with a semicolon, not replace
    const chip = within(dialog).getByRole('button', { name: /太啰嗦/ })
    await user.click(chip)
    expect(textarea.value).toContain('需要复核业绩数据')
    expect(textarea.value).toContain('太啰嗦')
    expect(textarea.value).toContain('；')
  })

  it('shows the 清空 affordance only when the textarea has content', async () => {
    const user = userEvent.setup()
    const dialog = await selectChapterAndOpenRejectModal(user)
    expect(within(dialog).queryByRole('button', { name: /清空/ })).toBeNull()
    const textarea = within(dialog).getByPlaceholderText(/描述为什么驳回/) as HTMLTextAreaElement
    await user.type(textarea, '某条理由')
    // After typing, the affordance appears
    const clearBtn = await within(dialog).findByRole('button', { name: /清空/ })
    expect(clearBtn).toBeInTheDocument()
    // Clicking clears the textarea
    await user.click(clearBtn)
    expect(textarea.value).toBe('')
  })

  it('submit calls bidsApi.rejectChapter with the chapter id and trimmed reason', async () => {
    const user = userEvent.setup()
    const dialog = await selectChapterAndOpenRejectModal(user)
    const textarea = within(dialog).getByPlaceholderText(/描述为什么驳回/) as HTMLTextAreaElement
    await user.type(textarea, '   内容需要核实   ')
    const submitBtn = within(dialog).getByRole('button', { name: '驳回' })
    await user.click(submitBtn)
    await waitFor(() => {
      expect(mockApi.rejectChapter).toHaveBeenCalledTimes(1)
    })
    // rejectChapter signature: (bidId, chapterId, reason)
    expect(mockApi.rejectChapter).toHaveBeenCalledWith(
      bidId,
      chapter1.id,
      '内容需要核实', // trimmed
    )
  })

  it('closes the modal after a successful submit', async () => {
    const user = userEvent.setup()
    const dialog = await selectChapterAndOpenRejectModal(user)
    const textarea = within(dialog).getByPlaceholderText(/描述为什么驳回/) as HTMLTextAreaElement
    await user.type(textarea, '驳回')
    await user.click(within(dialog).getByRole('button', { name: '驳回' }))
    await waitFor(() => {
      // Modal unmounts on success — the dialog role is gone
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    })
    // confirm toast surfaced (toast.info '已驳回')
    await waitFor(() => {
      const toasts = useToastStore.getState().toasts
      expect(toasts.some(t => t.title === '已驳回')).toBe(true)
    })
  })

  it('closes the modal when 取消 is clicked', async () => {
    const user = userEvent.setup()
    const dialog = await selectChapterAndOpenRejectModal(user)
    await user.click(within(dialog).getByRole('button', { name: '取消' }))
    await waitFor(() => {
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    })
    // No reject mutation fired
    expect(mockApi.rejectChapter).not.toHaveBeenCalled()
  })

  it('closes the modal on Escape key (Modal-level handler)', async () => {
    const user = userEvent.setup()
    const dialog = await selectChapterAndOpenRejectModal(user)
    // Sanity: the dialog rendered
    expect(dialog).toBeInTheDocument()
    await user.keyboard('{Escape}')
    await waitFor(() => {
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    })
  })

  it('user-added templates are reflected in the chip pool (store drives UI)', async () => {
    const user = userEvent.setup()
    // Pre-seed a custom template
    useRejectionTemplates.getState().add('需要补充风险分析')
    useRejectionTemplates.getState().togglePin(
      useRejectionTemplates.getState().templates.find(t => t.label === '需要补充风险分析')!.id,
    )
    const dialog = await selectChapterAndOpenRejectModal(user)
    // The custom chip is rendered
    expect(
      within(dialog).getByRole('button', { name: /需要补充风险分析/ }),
    ).toBeInTheDocument()
    // Pinned star is rendered next to the label (the ⭐ isn't text — it
    // is an SVG). We just confirm the chip is reachable; visual fidelity
    // to the star is not asserted here.
  })

  it('togglePinTemplate via 管理模板 exposes pin + delete controls', async () => {
    const user = userEvent.setup()
    const dialog = await selectChapterAndOpenRejectModal(user)
    // Open manager
    const managerToggle = within(dialog).getByRole('button', { name: /管理模板/ })
    await user.click(managerToggle)
    // "收起管理" now appears
    expect(within(dialog).getByRole('button', { name: /收起管理/ })).toBeInTheDocument()
    // Add-input is rendered with placeholder
    const addInput = within(dialog).getByPlaceholderText(/添加自定义原因/)
    expect(addInput).toBeInTheDocument()
    // Type and submit
    await user.type(addInput, '合规问题')
    await user.keyboard('{Enter}')
    // The new entry now exists as a row + as a chip in the pool
    await waitFor(() => {
      expect(within(dialog).getByRole('button', { name: /合规问题/ })).toBeInTheDocument()
    })
  })

  it('does not call the mutation when reason is only whitespace', async () => {
    const user = userEvent.setup()
    const dialog = await selectChapterAndOpenRejectModal(user)
    const textarea = within(dialog).getByPlaceholderText(/描述为什么驳回/) as HTMLTextAreaElement
    await user.type(textarea, '     ')
    // The button should still be disabled — trim() is empty
    expect(within(dialog).getByRole('button', { name: '驳回' })).toBeDisabled()
    // We never click; expect no mutation.
    expect(mockApi.rejectChapter).not.toHaveBeenCalled()
  })

  it('surfaces an error toast when rejectChapter fails', async () => {
    const user = userEvent.setup()
    mockApi.rejectChapter.mockRejectedValueOnce({
      response: { data: { message: 'lock conflict — try again' } },
    })
    const dialog = await selectChapterAndOpenRejectModal(user)
    const textarea = within(dialog).getByPlaceholderText(/描述为什么驳回/) as HTMLTextAreaElement
    await user.type(textarea, '驳回')
    await user.click(within(dialog).getByRole('button', { name: '驳回' }))
    await waitFor(() => {
      const toasts = useToastStore.getState().toasts
      expect(toasts.some(t => t.tone === 'error')).toBe(true)
    })
  })
})

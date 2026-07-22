import { useState, useCallback, useEffect, useMemo } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { bidsApi, AddChapterRequest } from '../../api/bids'
import { toast } from '../../lib/toast'
import { useHotkey } from '../../hooks/useHotkey'
import { Button, Modal } from '../../components/ui'
import { useRejectionTemplates, sortTemplates } from '../../lib/rejectionTemplates'
import { BID_STATUS_LABELS } from './workspace-helpers'
import { WorkspaceHeader } from './WorkspaceHeader'
import { BidStepper, stepFromStatus } from './BidStepper'
import { MaterialPanel } from './MaterialPanel'
import { ChapterTree } from './ChapterTree'
import { ChapterEditor } from './ChapterEditor'
import { ChapterInspector } from './ChapterInspector'
import { usePageMeta } from '../../lib/usePageMeta'

// Polling frequency — chapter content polls faster than outline because
// users actively watch it. Background tab suppression is handled by
// the visibility hook in useDocumentVisible below.
const POLL_OUTLINE_MS = 5000
const POLL_CONTENT_MS = 3000

/**
 * Returns true when the document is visible. Used by React Query's
 * refetchIntervalInBackground option to suspend polling when the user
 * has the tab backgrounded — important because we have two poller hooks
 * running every 3-5s and they were costing real $$ in API calls + RF
 * re-renders while users had the tab minimized.
 */
function useDocumentVisible(): boolean {
  const [visible, setVisible] = useState(
    typeof document !== 'undefined' ? !document.hidden : true,
  )
  useEffect(() => {
    const handler = () => setVisible(!document.hidden)
    document.addEventListener('visibilitychange', handler)
    return () => document.removeEventListener('visibilitychange', handler)
  }, [])
  return visible
}

export default function BidWorkspace() {
  usePageMeta({
    title: '标书工作区',
    description: '编辑标书章节、查看大纲、运行 AI 生成与导出 Word。',
    noindex: true,
  })

  const { id } = useParams<{ id: string }>()
  const queryClient = useQueryClient()
  const documentVisible = useDocumentVisible()
  const [selectedChapterId, setSelectedChapterIdRaw] = useState<string | null>(null)
  const [materialText, setMaterialText] = useState('')
  const [showMaterial, setShowMaterial] = useState(false)

  // Rejection reason modal — opens when the user clicks 驳回 on a single
  // chapter OR 批量驳回. We keep two separate flags because the modal
  // body changes (single: "reject this chapter"; batch: "reject N
  // chapters"). Both share the same TextArea + quick-pick chips.
  const [rejectModal, setRejectModal] = useState<
    | { kind: 'single'; chapterId: string; chapterTitle: string }
    | { kind: 'batch'; count: number }
    | null
  >(null)
  // Pending reason text — bound to the modal's TextArea. Reset on
  // close so the next invocation starts blank.
  const [rejectReason, setRejectReason] = useState('')

  // Rejection templates — user-customizable pool of quick-pick chips
  // surfaced in RejectionReasonModal. Persisted to localStorage so
  // the user's custom reasons survive reloads. The modal renders a
  // "管理模板" link that opens a small panel for adding/pinning/
  // removing custom entries.
  const templates = useRejectionTemplates(s => s.templates)
  const addTemplate = useRejectionTemplates(s => s.add)
  const removeTemplate = useRejectionTemplates(s => s.remove)
  const togglePinTemplate = useRejectionTemplates(s => s.togglePin)
  const [templateManagerOpen, setTemplateManagerOpen] = useState(false)
  const [newTemplateLabel, setNewTemplateLabel] = useState('')

  // Wrapped setter — when the user switches chapters we proactively evict
  // the previous chapter's content query from the cache so a fast clicker
  // can't see the old chapter's stale data briefly rendered while the
  // new fetch is in flight. Without this, chapter-content caching with
  // stale-while-revalidate can flash the wrong body in the editor.
  const setSelectedChapterId = useCallback((next: string | null) => {
    setSelectedChapterIdRaw(prev => {
      if (prev !== next) {
        queryClient.removeQueries({ queryKey: ['chapter-content', id, prev], exact: true })
      }
      return next
    })
  }, [id, queryClient])

  // ============ Queries ============
  //
  // staleTime tuning: the workspace has two refresh strategies running
  // in parallel — background polling (refetchInterval) AND manual user
  // actions (clicking, editing). Without staleTime, every user action
  // re-fires the query even when the polled background already
  // produced a fresh value milliseconds ago, causing a race between
  // the optimistic UI update and the network response.
  //
  //   bid         → 4s  : changes rarely except on workflow transitions
  //   outline     → 1s  : changes when chapters are added/edited
  //   content     → 1s  : changes when user edits (autosave → refetch)
  const { data: bidData } = useQuery({
    queryKey: ['bid', id],
    queryFn: () => bidsApi.get(id!),
    enabled: !!id,
    refetchInterval: POLL_OUTLINE_MS,
    refetchIntervalInBackground: false,
    staleTime: 4_000,
  })
  const bid = (bidData?.data?.data ?? null) as any

  const { data: outlineData, isLoading: outlineLoading } = useQuery({
    queryKey: ['outline', id],
    queryFn: () => bidsApi.getOutline(id!),
    enabled: !!id,
    refetchInterval: POLL_OUTLINE_MS,
    refetchIntervalInBackground: false,
    staleTime: 1_000,
  })
  const chapters: any[] = Array.isArray(outlineData?.data?.data) ? outlineData!.data!.data! : []

  const { data: contentData } = useQuery({
    queryKey: ['chapter-content', id, selectedChapterId],
    queryFn: () => bidsApi.getChapterContent(id!, selectedChapterId!),
    enabled: !!id && !!selectedChapterId,
    // Faster polling only when the tab is in focus; suspends entirely
    // when the user switches away to another tab.
    refetchInterval: documentVisible ? POLL_CONTENT_MS : false,
    refetchIntervalInBackground: false,
    staleTime: 1_000,
  })
  const content = (contentData?.data?.data ?? null) as any

  // Auto-select first chapter on load or when none selected
  useEffect(() => {
    if (!selectedChapterId && chapters.length > 0) {
      setSelectedChapterId(chapters[0].id)
    }
  }, [chapters, selectedChapterId])

  const selectedChapter = chapters.find(c => c.id === selectedChapterId) || null

  // ============ Mutations ============
  const addMutation = useMutation({
    mutationFn: (data: AddChapterRequest) => bidsApi.addChapter(id!, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['outline', id] })
      queryClient.invalidateQueries({ queryKey: ['bid', id] })
      toast.success('章节已添加')
    },
    onError: (err: any) => {
      toast.error('添加失败', err?.response?.data?.message)
    },
  })
  const deleteMutation = useMutation({
    mutationFn: (chapterId: string) => bidsApi.deleteChapter(id!, chapterId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['outline', id] })
      queryClient.invalidateQueries({ queryKey: ['bid', id] })
      setSelectedChapterId(null)
      toast.success('章节已删除')
    },
    onError: (err: any) => toast.error('删除失败', err?.response?.data?.message),
  })

  // Reorder mutation — accepts a fully-described ordered tree (flat list
  // with parent_id re-derived by the caller). On success we invalidate
  // the outline + bid queries so the refetched state reflects the new
  // order_index the backend assigned.
  const reorderMutation = useMutation({
    mutationFn: (ordered: Array<{ id: string; parent_id?: string | null }>) =>
      bidsApi.reorderOutline(id!, ordered),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['outline', id] })
      queryClient.invalidateQueries({ queryKey: ['bid', id] })
      toast.success('章节顺序已更新')
    },
    onError: (err: any) => {
      queryClient.invalidateQueries({ queryKey: ['outline', id] })
      toast.error('排序失败', err?.response?.data?.message || '请稍后再试')
    },
  })

  // Per-chapter approve — the HIL human-gate. Each chapter needs an
  // explicit approve before the user can advance the bid from
  // `awaiting_review` to `auditing`. Approving is granular (one
  // chapter at a time) so users can re-read each section in detail.
  const approveMutation = useMutation({
    mutationFn: (chapterId: string) => bidsApi.approveChapter(id!, chapterId),
    onSuccess: (res) => {
      queryClient.invalidateQueries({ queryKey: ['outline', id] })
      queryClient.invalidateQueries({ queryKey: ['bid', id] })
      queryClient.invalidateQueries({ queryKey: ['chapter-content', id, selectedChapterId] })
      // Sticky toast — the version number is informationally
      // important (audit trail) and we want the user to register the
      // change rather than have it flash by in 4 seconds.
      toast.success('已审核', `章节已标记为已审 · v${(res.data?.data as any)?.approved_at?.slice(0, 16) ?? ''}`, undefined, { sticky: true })
    },
    onError: (err: any) => toast.error('审核失败', err?.response?.data?.message),
  })

  // Per-chapter reject — sends the chapter back into the generating
  // pipeline with the user's reason. The reason is optional but
  // shown in audit logs server-side so we surface a prompt.
  const rejectMutation = useMutation({
    mutationFn: ({ chapterId, reason }: { chapterId: string; reason: string }) =>
      bidsApi.rejectChapter(id!, chapterId, reason),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['outline', id] })
      queryClient.invalidateQueries({ queryKey: ['bid', id] })
      toast.info('已驳回', '该章节已送回生成队列')
    },
    onError: (err: any) => toast.error('驳回失败', err?.response?.data?.message),
  })

  // Batch reject — symmetrical to batch approve but inverted. Rejects
  // every `succeeded` chapter with a single reason (the user picks
  // once via RejectionReasonModal). We deliberately don't allow per-
  // chapter reasons in batch mode — that would require opening N modals.
  // Users who need fine-grained reasons use the per-chapter inspector.
  const batchRejectMutation = useMutation({
    mutationFn: async (reason: string) => {
      const targets = chapters.filter(c => c.status === 'succeeded')
      if (targets.length === 0) return { submitted: 0 }
      await Promise.allSettled(
        targets.map(c => bidsApi.rejectChapter(id!, c.id, reason)),
      )
      return { submitted: targets.length }
    },
    onSuccess: ({ submitted }) => {
      if (submitted === 0) {
        toast.info('没有可驳回的章节')
      } else {
        toast.success(`已驳回 ${submitted} 个章节`, '全部送回生成队列')
        queryClient.invalidateQueries({ queryKey: ['outline', id] })
        queryClient.invalidateQueries({ queryKey: ['bid', id] })
      }
    },
    onError: () => toast.error('批量驳回失败'),
  })

  // Batch approve — applies to every chapter that is `succeeded` but
  // not yet `approved`. The inverse of the batch retry button. Saves
  // users the click-fest when they trust the bulk of the output and
  // only need to scrutinize a few chapters.
  const approveAllMutation = useMutation({
    mutationFn: async () => {
      const targets = chapters.filter(c => c.status === 'succeeded')
      if (targets.length === 0) return { submitted: 0 }
      await Promise.allSettled(targets.map(c => bidsApi.approveChapter(id!, c.id)))
      return { submitted: targets.length }
    },
    onSuccess: ({ submitted }) => {
      if (submitted === 0) {
        toast.info('没有待审核的章节')
      } else {
        toast.success(`已审核 ${submitted} 个章节`)
        queryClient.invalidateQueries({ queryKey: ['outline', id] })
        queryClient.invalidateQueries({ queryKey: ['bid', id] })
      }
    },
    onError: () => toast.error('批量审核失败'),
  })
  const updateMutation = useMutation({
    mutationFn: ({ chapterId, data }: { chapterId: string; data: any }) =>
      bidsApi.updateChapter(id!, chapterId, data),
    onSuccess: (_res, vars) => {
      // Always invalidate outline (chapter metadata). We also invalidate
      // the content query ONLY if the user changed a field that the
      // server's chapter-content payload exposes (priority, writing_style,
      // target_word_count, min_word_count) — title etc. don't appear in
      // content, so we save a round-trip for those.
      queryClient.invalidateQueries({ queryKey: ['outline', id] })
      queryClient.invalidateQueries({ queryKey: ['bid', id] })
      const contentAffectingFields = ['priority', 'writing_style', 'target_word_count', 'min_word_count']
      if (vars.data && contentAffectingFields.some(f => f in vars.data)) {
        queryClient.invalidateQueries({ queryKey: ['chapter-content', id, selectedChapterId] })
      }
    },
    onError: (err: any) => toast.error('更新失败', err?.response?.data?.message),
  })
  const transitionMutation = useMutation({
    // The backend uses optimistic locking: every transition bumps the bid's
    // `version` column, and a request carrying the old version gets 409
    // VERSION_CONFLICT. The common failure mode is a stale closure — the
    // user's button handler captured bid.version from a previous render and
    // the backend has since moved on. We retry on 409 by re-reading the
    // current version via React Query's cached `bid` entry and replaying.
    // Hard cap at 1 retry so a wedged state surfaces as a toast instead of
    // spinning forever.
    mutationFn: async ({ to, version }: { to: string; version: number }) => {
      const call = (v: number) =>
        bidsApi.transition(id!, to, v, `transition to ${to}${v !== version ? ' (retry)' : ''}`)
      try {
        return await call(version)
      } catch (err: any) {
        const status = err?.response?.status
        const code = err?.response?.data?.error?.code
        const isConflict = status === 409 || code === 'VERSION_CONFLICT'
        if (!isConflict) throw err
        // Pull the freshest cached bid.version. invalidateQueries was called
        // by a prior successful transition, so the cache is usually one step
        // ahead of the user's button click.
        await queryClient.invalidateQueries({ queryKey: ['bid', id] })
        const fresh = queryClient.getQueryData<any>(['bid', id])
        const freshVersion = fresh?.data?.data?.version
        if (typeof freshVersion !== 'number' || freshVersion === version) throw err
        return await call(freshVersion)
      }
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['bid', id] })
      toast.success('工作流已推进')
    },
    onError: (err: any) => {
      const code = err?.response?.data?.error?.code
      const msg =
        code === 'VERSION_CONFLICT'
          ? '版本冲突且无法自动重试（请刷新页面）'
          : err?.response?.data?.error?.message || err?.response?.data?.message || '未知错误'
      toast.error('推进失败', msg)
    },
  })
  const saveMaterialMutation = useMutation({
    mutationFn: (text: string) => bidsApi.saveMaterial(id!, text),
    onSuccess: () => {
      setShowMaterial(false)
      toast.success('材料已保存', `${materialText.length} 字`)
    },
    onError: (err: any) => toast.error('保存失败', err?.response?.data?.message),
  })
  const generateMutation = useMutation({
    mutationFn: ({ chapterId, prompt }: { chapterId: string; prompt?: string }) =>
      bidsApi.generateChapter(id!, chapterId, prompt),
    onSuccess: (_, vars) => {
      queryClient.invalidateQueries({ queryKey: ['outline', id] })
      queryClient.invalidateQueries({ queryKey: ['bid', id] })
      queryClient.invalidateQueries({ queryKey: ['chapter-content', id, selectedChapterId] })
      const withPrompt = vars.prompt && vars.prompt.trim().length > 0
      toast.info(
        withPrompt ? '已带提示词提交' : '生成任务已提交',
        withPrompt ? `AI 正在按提示词编写（共 ${vars.prompt!.trim().length} 字）…` : 'AI 正在编写内容…',
      )
    },
    onError: (err: any, vars) => {
      // Surface the failure with a one-click retry CTA. The chapter id
      // and the original prompt (if any) are captured via the onError
      // second arg so the retry re-submits with identical parameters.
      const description = err?.response?.data?.message || err?.response?.data?.error?.message
      toast.error('提交失败', description, {
        label: '重试此章节',
        onClick: () => generateMutation.mutate({ chapterId: vars.chapterId, prompt: vars.prompt }),
      })
    },
  })

  // Batch retry — the common case is "I clicked generate and 3 chapters
  // failed because the LLM hit rate limits / connection dropped". Walking
  // through each one to click retry is friction; this fires all failed
  // chapters in parallel so the workflow catches up in one round-trip.
  // Per-chapter CustomPrompts (from ChapterInspector prompt tab) are
  // intentionally NOT retried — the batch path has no prompt context.
  const retryAllFailedMutation = useMutation({
    mutationFn: async () => {
      const failed = chapters.filter(c => c.status === 'failed')
      if (failed.length === 0) return { submitted: 0 }
      await Promise.allSettled(failed.map(c =>
        generateMutation.mutateAsync({ chapterId: c.id })))
      return { submitted: failed.length }
    },
    onSuccess: ({ submitted }) => {
      if (submitted === 0) {
        toast.info('没有失败章节')
      } else {
        toast.success(`已重新提交 ${submitted} 个失败章节`)
        queryClient.invalidateQueries({ queryKey: ['outline', id] })
        queryClient.invalidateQueries({ queryKey: ['bid', id] })
      }
    },
    onError: () => toast.error('批量重试失败，请稍后再试'),
  })
  const saveContentMutation = useMutation({
    mutationFn: async ({ chapterId, text }: { chapterId: string; text: string }) => {
      // Send expected_version so the backend can reject concurrent edits
      // with 409. Fall back to no-version (legacy clients) if the field is
      // missing from the polled content — the backend should accept both.
      const expectedVersion = content?.version
      try {
        return await bidsApi.saveChapterContent(id!, chapterId, text, expectedVersion)
      } catch (err: any) {
        if (err?.response?.status === 409 || err?.response?.data?.error?.code === 'VERSION_CONFLICT') {
          // Surface the conflict so the editor can prompt the user.
          // Re-throw with the original error so onError still has it.
          err.isVersionConflict = true
          throw err
        }
        throw err
      }
    },
    onSuccess: (res) => {
      queryClient.invalidateQueries({ queryKey: ['chapter-content', id, selectedChapterId] })
      queryClient.invalidateQueries({ queryKey: ['bid', id] })
      toast.success('内容已保存', `v${(res.data?.data as any)?.version}`)
    },
    onError: (err: any) => {
      if (err?.isVersionConflict) {
        // Force-refresh the content so the user sees what the server has,
        // then show a conflict toast with a Reload action.
        queryClient.invalidateQueries({ queryKey: ['chapter-content', id, selectedChapterId] })
        toast.error('版本冲突', '其他会话已修改此章节，正在刷新…')
        return
      }
      toast.error('保存失败', err?.response?.data?.message)
    },
  })

  const handleAddChapter = useCallback((data: { title: string; level: number; parent_id?: string }) => {
    addMutation.mutate(data)
  }, [addMutation])

  const handleSaveContent = useCallback(async (text: string) => {
    if (!selectedChapter) return
    await saveContentMutation.mutateAsync({ chapterId: selectedChapter.id, text })
  }, [selectedChapter, saveContentMutation])

  // Cmd/Ctrl+S to save current edit (when editing content) — fall back to a
  // hint toast if no chapter is selected.
  useHotkey('mod+s', (e) => {
    e.preventDefault()
    const active = document.activeElement
    if (active instanceof HTMLTextAreaElement) {
      // The editor uses its own textarea; trigger a blur to commit, then save.
      active.blur()
    }
    toast.info('已保存当前编辑')
  }, { enabled: !!selectedChapter })

  // ============ In-flight generation stats ============
  const inFlight = useMemo(
    () => chapters.filter(c => c.status === 'running' || c.status === 'pending').length,
    [chapters],
  )
  const failedCount = useMemo(
    () => chapters.filter(c => c.status === 'failed').length,
    [chapters],
  )

  // ============ Top bar workflow actions ============
  //
  // The transition model is now: ... → generating → awaiting_review →
  // auditing → exporting → done. awaiting_review is a mandatory
  // pause for the user to inspect the AI output before the system
  // commits to the audit/export step. We surface:
  //   • review progress (X/N chapters approved) inline as a chip
  //   • "审核通过" button (advances to auditing, disabled if 0 approved)
  //   • "驳回" button (sends back to generating for re-runs)
  // approvedCount = chapters with `approved` status (human-gate passed)
  // The awaiting_review → auditing transition requires approvedCount
  // to equal totalChapters. The header chip shows the approved count
  // so users see the gap between "generated" and "vetted".
  // reviewedCount (succeeded) is kept for status-strip/UI; pendingReviewCount
  // is the same as reviewedCount but is the actionable subset.
  const reviewedCount = useMemo(
    () => chapters.filter(c => c.status === 'succeeded').length,
    [chapters],
  )
  const approvedCount = useMemo(
    () => chapters.filter(c => c.status === 'approved').length,
    [chapters],
  )
  const pendingReviewCount = reviewedCount
  const totalChapters = chapters.length
  const allChaptersApproved = totalChapters > 0 && approvedCount === totalChapters
  const canBatchApprove = pendingReviewCount > 0

  const renderWorkflowActions = () => {
    if (!bid) return null
    const actions: React.ReactNode[] = []
    if (bid.status === 'pending' || bid.status === 'parsing') {
      actions.push(
        <Button key="parse" size="sm" variant="secondary" loading={transitionMutation.isPending}
          onClick={() => transitionMutation.mutate({ to: 'parsing', version: bid.version })}>
          开始解析
        </Button>
      )
    }
    if (bid.status === 'parsing' || bid.status === 'outlining') {
      actions.push(
        <Button key="outline" size="sm" variant="secondary" loading={transitionMutation.isPending}
          onClick={() => transitionMutation.mutate({ to: 'outlining', version: bid.version })}>
          生成大纲
        </Button>
      )
    }
    if (bid.status === 'facts') {
      actions.push(
        <Button key="gen" size="sm" variant="primary" loading={transitionMutation.isPending}
          onClick={() => transitionMutation.mutate({ to: 'generating', version: bid.version })}>
          批量生成内容
        </Button>
      )
    }
    // When generating finishes (backend auto-transitions to
    // awaiting_review) or user is already in awaiting_review, show the
    // review summary chip + batch approve + workflow advance.
    if (bid.status === 'awaiting_review') {
      // Two-tier chip: shows approved/total. If approved equals total
      // we switch to a green "ready to advance" chip.
      const allReady = allChaptersApproved
      actions.push(
        <span
          key="review-chip"
          className={[
            'inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium',
            allReady
              ? 'bg-emerald-50 dark:bg-emerald-900/30 text-emerald-700 dark:text-emerald-300'
              : 'bg-amber-50 dark:bg-amber-900/30 text-amber-700 dark:text-amber-300',
          ].join(' ')}
        >
          {allReady ? (
            <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
              <polyline points="20 6 9 17 4 12" />
            </svg>
          ) : (
            <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <circle cx="12" cy="12" r="10" />
              <polyline points="12 6 12 12 16 14" />
            </svg>
          )}
          已审 <strong className="tabular-nums">{approvedCount}/{totalChapters}</strong>
        </span>
      )
      // Batch approve — covers the common case where the user has
      // skimmed the outline and just wants to mark everything green.
      // Skipped if nothing's pending; disabled while in flight.
      if (canBatchApprove) {
        actions.push(
          <Button
            key="approve-all"
            size="sm"
            variant="outline"
            loading={approveAllMutation.isPending}
            onClick={() => approveAllMutation.mutate()}
            leftIcon={
              <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
                <polyline points="20 6 9 17 4 12" />
                <path d="M5 12l5 5L20 7" />
              </svg>
            }
          >
            全部通过 ({pendingReviewCount})
          </Button>
        )
      }
      // Batch reject — mirror of the approve-all button. Same gating
      // (only show when there's something to reject), but opens the
      // reason modal first since the reason is required. We pass a
      // no-op when there are no pending chapters to keep the header
      // layout stable.
      if (canBatchApprove) {
        actions.push(
          <Button
            key="reject-all"
            size="sm"
            variant="ghost"
            onClick={() => {
              setRejectReason('')
              setRejectModal({ kind: 'batch', count: pendingReviewCount })
            }}
            leftIcon={
              <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <circle cx="12" cy="12" r="10" />
                <line x1="15" y1="9" x2="9" y2="15" />
                <line x1="9" y1="9" x2="15" y2="15" />
              </svg>
            }
          >
            全部驳回 ({pendingReviewCount})
          </Button>
        )
      }
      // 驳回重做 (workflow-level — sends the WHOLE bid back to
      // generating, not a single chapter; for per-chapter reject use
      // the inspector's "驳回" button).
      actions.push(
        <Button key="reject" size="sm" variant="ghost" loading={transitionMutation.isPending}
          onClick={() => transitionMutation.mutate({ to: 'generating', version: bid.version })}>
          驳回重做
        </Button>
      )
      // 推进到审计 (auditing) — only enabled when every chapter has
      // been explicitly approved by the user.
      actions.push(
        <Button
          key="advance"
          size="sm"
          variant="primary"
          loading={transitionMutation.isPending}
          disabled={!allChaptersApproved}
          title={!allChaptersApproved ? `还有 ${totalChapters - approvedCount} 个章节未通过审核` : '进入审计阶段'}
          onClick={() => transitionMutation.mutate({ to: 'auditing', version: bid.version })}
        >
          进入审计
        </Button>
      )
    }
    if (bid.status === 'auditing') {
      actions.push(
        <Button key="export" size="sm" variant="primary" loading={transitionMutation.isPending}
          onClick={() => transitionMutation.mutate({ to: 'exporting', version: bid.version })}>
          导出文档
        </Button>
      )
    }
    if (bid.status === 'done') {
      actions.push(
        <Link key="dl" to={`/bids/${id}/export`}>
          <Button size="sm" variant="primary" leftIcon={
            <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
              <polyline points="7 10 12 15 17 10" />
              <line x1="12" y1="15" x2="12" y2="3" />
            </svg>
          }>下载文档</Button>
        </Link>
      )
    }
    return actions.length > 0 ? <div className="flex items-center gap-2">{actions}</div> : null
  }

  return (
    <div className="h-screen flex flex-col bg-ink-50">
      <WorkspaceHeader
        bidId={id!}
        projectName={bid?.project_name || ''}
        bidStatus={bid?.status || ''}
        doneChapters={bid?.done_chapters ?? 0}
        totalChapters={bid?.total_chapters ?? 0}
        rightActions={renderWorkflowActions()}
      />
      <BidStepper current={stepFromStatus(bid?.status)} />

{/* Live status strip — only shows when there's something to report.
       * Now expanded with: progress bar, per-status counts, and batch
       * retry button for failed chapters. The polling hooks above
       * (refetchInterval + refetchIntervalInBackground:false) drive
       * the auto-refresh of these numbers. */}
      {(inFlight > 0 || failedCount > 0 || generateMutation.isPending) && (
        <div className="px-6 py-3 bg-gradient-to-r from-brand-50/60 to-violet-50/60 dark:from-brand-900/20 dark:to-violet-900/20 border-b border-brand-100 dark:border-brand-900/40 animate-slide-down">
          <div className="max-w-6xl mx-auto flex items-center gap-4">
            {/* Animated dot */}
            <span className="relative flex h-2.5 w-2.5 shrink-0">
              {(generateMutation.isPending || inFlight > 0) ? (
                <>
                  <span className="absolute inline-flex h-full w-full rounded-full bg-brand-400 opacity-75 animate-ping-slow" />
                  <span className="relative inline-flex rounded-full h-2.5 w-2.5 bg-brand-600" />
                </>
              ) : (
                <span className="relative inline-flex rounded-full h-2.5 w-2.5 bg-red-500" />
              )}
            </span>

            {/* Status text */}
            <div className="flex-1 min-w-0">
              <div className="flex items-center gap-3 text-xs">
                <strong className="text-brand-800 dark:text-brand-200 font-semibold">
                  {generateMutation.isPending || inFlight > 0
                    ? 'AI 正在生成内容'
                    : `${failedCount} 个章节生成失败`}
                </strong>
                {inFlight > 0 && (
                  <span className="inline-flex items-center gap-1 text-brand-700 dark:text-brand-300">
                    <span className="w-1 h-1 rounded-full bg-brand-500" />
                    {inFlight} 进行中
                  </span>
                )}
                {chapters.filter(c => c.status === 'succeeded').length > 0 && (
                  <span className="inline-flex items-center gap-1 text-emerald-700 dark:text-emerald-400">
                    <span className="w-1 h-1 rounded-full bg-emerald-500" />
                    {chapters.filter(c => c.status === 'succeeded').length} 已完成
                  </span>
                )}
                {failedCount > 0 && (
                  <span className="inline-flex items-center gap-1 text-red-700 dark:text-red-400">
                    <span className="w-1 h-1 rounded-full bg-red-500" />
                    {failedCount} 失败
                  </span>
                )}
              </div>
              {/* Mini progress bar — width is overall completion % */}
              {chapters.length > 0 && (
                <div className="relative h-1 mt-1.5 rounded-full bg-ink-100 dark:bg-ink-700 overflow-hidden">
                  <div
                    className="absolute inset-y-0 left-0 bg-gradient-to-r from-brand-500 to-emerald-500 rounded-full transition-all duration-500"
                    style={{
                      width: `${Math.round((chapters.filter(c => c.status === 'succeeded').length / chapters.length) * 100)}%`,
                    }}
                  />
                </div>
              )}
            </div>

            {/* Batch retry button — only when there are failed chapters */}
            {failedCount > 0 && (
              <Button
                size="xs"
                variant="danger"
                onClick={() => retryAllFailedMutation.mutate()}
                loading={retryAllFailedMutation.isPending}
                leftIcon={
                  <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
                    <polyline points="23 4 23 10 17 10" />
                    <path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10" />
                  </svg>
                }
              >
                重试全部 ({failedCount})
              </Button>
            )}
          </div>
        </div>
      )}

      {/* Three-pane layout */}
      <div className="flex flex-1 overflow-hidden">
        {/* Left: Material + Chapter tree */}
        <aside className="w-80 shrink-0 bg-white border-r border-ink-100 flex flex-col overflow-hidden">
          <MaterialPanel
            materialText={materialText}
            onChange={setMaterialText}
            onSave={() => saveMaterialMutation.mutate(materialText)}
            saving={saveMaterialMutation.isPending}
            open={showMaterial}
            onToggle={() => setShowMaterial(!showMaterial)}
          />
          <div className="flex-1 min-h-0">
            <ChapterTree
              chapters={chapters}
              selectedId={selectedChapterId}
              onSelect={setSelectedChapterId}
              onDelete={(cid) => deleteMutation.mutate(cid)}
              onAdd={handleAddChapter}
              onReorder={(ordered) => reorderMutation.mutateAsync(ordered)}
              adding={addMutation.isPending}
            />
          </div>
          {outlineLoading && chapters.length === 0 && (
            <div className="px-3 py-2 text-xs text-ink-400 border-t border-ink-100 bg-ink-50">
              ⏳ 加载章节目录…
            </div>
          )}
        </aside>

        {/* Center: Editor */}
        <section className="flex-1 min-w-0 bg-white overflow-hidden">
          {selectedChapter ? (
            <ChapterEditor
              chapter={selectedChapter}
              content={content}
              bidId={id!}
              onSaveContent={handleSaveContent}
              onUpdateChapter={(data) => updateMutation.mutate({ chapterId: selectedChapter.id, data })}
            />
          ) : (
            <div className="h-full flex items-center justify-center px-6">
              <div className="text-center max-w-sm">
                <div className="mx-auto w-20 h-20 rounded-2xl bg-brand-gradient-soft border border-brand-100 flex items-center justify-center text-3xl mb-5">
                  📖
                </div>
                <h3 className="text-base font-semibold text-ink-800 mb-1">未选择章节</h3>
                <p className="text-sm text-ink-500">从左侧目录中选择一个章节开始查看或编辑</p>
              </div>
            </div>
          )}
        </section>

        {/* Right: Inspector */}
        <aside className="w-80 shrink-0 bg-ink-50 border-l border-ink-100 overflow-hidden">
          {selectedChapter ? (
            <ChapterInspector
              chapter={selectedChapter}
              content={content}
              bidStatus={bid?.status || ''}
              onGenerate={() => generateMutation.mutate({ chapterId: selectedChapter.id })}
              generating={generateMutation.isPending}
              onApprove={() => approveMutation.mutate(selectedChapter.id)}
              onReject={() => {
                // Open the rejection-reason modal rather than the
                // generic `confirm()` prompt — we want a structured
                // reason that's persisted to the chapter record.
                if (!selectedChapter) return
                setRejectReason('')
                setRejectModal({ kind: 'single', chapterId: selectedChapter.id, chapterTitle: selectedChapter.title })
              }}
              approving={approveMutation.isPending || rejectMutation.isPending}
              onUpdateChapter={(data) => updateMutation.mutate({ chapterId: selectedChapter.id, data })}
            />
          ) : (
            <div className="h-full flex items-center justify-center px-6">
              <p className="text-sm text-ink-400 text-center">选择章节<br/>查看配置</p>
            </div>
          )}
        </aside>
      </div>

      {/* Rejection reason modal — opens for both single-chapter and
          batch-reject flows. The body changes by `kind`. Quick-pick
          reason chips (4 common reasons) speed up the workflow. The
          "其他..." radio lets the user type a custom reason. Reason is
          REQUIRED to actually submit — empty reasons get an inline
          hint. We dispatch through the existing single/batch mutations
          so all the existing invalidation + toast logic applies. */}
      <Modal
        open={rejectModal !== null}
        onClose={() => {
          if (rejectMutation.isPending || batchRejectMutation.isPending) return
          setRejectModal(null)
        }}
        title={rejectModal?.kind === 'batch' ? '批量驳回章节' : '驳回此章节'}
        description={
          rejectModal?.kind === 'batch'
            ? `将为 ${rejectModal.count} 个章节标记驳回，并送回 AI 生成队列。`
            : `「${rejectModal?.kind === 'single' ? rejectModal.chapterTitle : ''}」将被送回 AI 生成队列。`
        }
        size="md"
        icon={
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <circle cx="12" cy="12" r="10" />
            <line x1="15" y1="9" x2="9" y2="15" />
            <line x1="9" y1="9" x2="15" y2="15" />
          </svg>
        }
        footer={
          <>
            <Button
              variant="secondary"
              onClick={() => setRejectModal(null)}
              disabled={rejectMutation.isPending || batchRejectMutation.isPending}
            >
              取消
            </Button>
            <Button
              variant="danger"
              loading={rejectMutation.isPending || batchRejectMutation.isPending}
              disabled={!rejectReason.trim()}
              onClick={() => {
                if (!rejectModal) return
                const reason = rejectReason.trim()
                if (rejectModal.kind === 'single') {
                  rejectMutation.mutate({ chapterId: rejectModal.chapterId, reason })
                } else {
                  batchRejectMutation.mutate(reason)
                }
                setRejectModal(null)
              }}
            >
              {rejectModal?.kind === 'batch' ? '全部驳回' : '驳回'}
            </Button>
          </>
        }
      >
        <div className="space-y-3">
          <div>
            <div className="flex items-center justify-between mb-2">
              <span className="text-sm font-medium text-ink-700 dark:text-ink-200">
                驳回原因 <span className="text-red-500">*</span>
              </span>
              {/* "管理模板" link — opens a small panel for adding,
                  pinning, and removing custom reasons. We only show
                  this if the user has at least one custom template OR
                  they've expanded the manager. */}
              <button
                type="button"
                onClick={() => setTemplateManagerOpen(v => !v)}
                className="text-[11px] text-ink-500 dark:text-ink-400 hover:text-brand-600 dark:hover:text-brand-400 transition-colors inline-flex items-center gap-1"
              >
                <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <circle cx="12" cy="12" r="3" />
                  <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 1 1-4 0v-.09a1.65 1.65 0 0 0-1-1.51 1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 1 1 0-4h.09a1.65 1.65 0 0 0 1.51-1 1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33h0a1.65 1.65 0 0 0 1-1.51V3a2 2 0 1 1 4 0v.09a1.65 1.65 0 0 0 1 1.51h0a1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82v0a1.65 1.65 0 0 0 1.51 1H21a2 2 0 1 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" />
                </svg>
                {templateManagerOpen ? '收起管理' : '管理模板'}
              </button>
            </div>

            {/* Template manager panel — collapsible. Visible by default
                if user has 0 templates (so they can add the first
                one), collapsed otherwise to keep the modal focused. */}
            {templateManagerOpen && (
              <div className="mb-3 p-3 rounded-lg bg-ink-50 dark:bg-ink-800/50 border border-ink-200 dark:border-ink-700 animate-slide-down">
                <div className="text-[10px] uppercase tracking-wider text-ink-500 dark:text-ink-400 font-semibold mb-2">
                  自定义驳回原因模板
                </div>
                <div className="space-y-1 max-h-32 overflow-y-auto scrollbar-thin">
                  {sortTemplates(templates).map(t => (
                    <div key={t.id} className="flex items-center gap-2 text-xs">
                      <button
                        type="button"
                        onClick={() => togglePinTemplate(t.id)}
                        title={t.pinned ? '取消置顶' : '置顶（显示在最前）'}
                        className={[
                          'shrink-0 w-6 h-6 grid place-items-center rounded transition-colors',
                          t.pinned
                            ? 'bg-brand-100 dark:bg-brand-900/40 text-brand-600 dark:text-brand-400 hover:bg-brand-200 dark:hover:bg-brand-900/60'
                            : 'text-ink-300 dark:text-ink-600 hover:bg-ink-100 dark:hover:bg-ink-700',
                        ].join(' ')}
                      >
                        <svg width="11" height="11" viewBox="0 0 24 24" fill={t.pinned ? 'currentColor' : 'none'} stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                          <polygon points="12 2 15 8.5 22 9.3 17 14.1 18.2 21 12 17.8 5.8 21 7 14.1 2 9.3 9 8.5 12 2" />
                        </svg>
                      </button>
                      <span className="shrink-0 w-5 text-center">{t.emoji || '✏️'}</span>
                      <span className="flex-1 truncate text-ink-700 dark:text-ink-200">{t.label}</span>
                      {t.builtin && (
                        <span className="text-[9px] text-ink-400 dark:text-ink-500 px-1 py-0.5 rounded bg-ink-100 dark:bg-ink-800">默认</span>
                      )}
                      {!t.builtin && (
                        <button
                          type="button"
                          onClick={() => removeTemplate(t.id)}
                          title="删除模板"
                          className="shrink-0 w-5 h-5 grid place-items-center rounded text-ink-400 hover:text-red-500 hover:bg-red-50 dark:hover:bg-red-900/30 transition-colors"
                        >
                          <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                            <polyline points="3 6 5 6 21 6" />
                            <path d="M19 6l-2 14a2 2 0 0 1-2 2H9a2 2 0 0 1-2-2L5 6" />
                          </svg>
                        </button>
                      )}
                    </div>
                  ))}
                </div>
                {/* Add new template — single line input with auto-trim. */}
                <div className="mt-2 pt-2 border-t border-ink-200 dark:border-ink-700 flex items-center gap-1.5">
                  <input
                    type="text"
                    value={newTemplateLabel}
                    onChange={(e) => setNewTemplateLabel(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter' && newTemplateLabel.trim()) {
                        addTemplate(newTemplateLabel)
                        setNewTemplateLabel('')
                      }
                    }}
                    placeholder="添加自定义原因，例如：'需要补充风险分析'"
                    className="flex-1 px-2 py-1 text-xs bg-white dark:bg-ink-900 border border-ink-200 dark:border-ink-700 rounded focus:outline-none focus:border-brand-400 text-ink-900 dark:text-white placeholder:text-ink-400"
                  />
                  <Button
                    size="xs"
                    variant="primary"
                    disabled={!newTemplateLabel.trim()}
                    onClick={() => {
                      addTemplate(newTemplateLabel)
                      setNewTemplateLabel('')
                    }}
                  >
                    添加
                  </Button>
                </div>
                <p className="mt-2 text-[10px] text-ink-400 dark:text-ink-500">
                  ⭐ 置顶的模板显示在最前 · 内置 4 个不可删除 · 模板存储在浏览器本地
                </p>
              </div>
            )}

            {/* Quick-pick chips — sourced from the templates store. */}
            <div className="flex flex-wrap gap-1.5 mb-2">
              {sortTemplates(templates).map(t => (
                <button
                  key={t.id}
                  type="button"
                  onClick={() => {
                    setRejectReason(prev => {
                      // Append if user already has a custom reason,
                      // otherwise replace. Avoids losing typed context.
                      if (prev && !prev.includes(t.label)) {
                        return `${prev}；${t.label}`
                      }
                      return t.label
                    })
                  }}
                  title={t.desc}
                  className={[
                    'inline-flex items-center gap-1 px-2 py-1 text-xs rounded-md border transition-colors',
                    t.pinned
                      ? 'border-brand-300 dark:border-brand-700 bg-brand-50/50 dark:bg-brand-900/20 text-brand-700 dark:text-brand-300 hover:bg-brand-100 dark:hover:bg-brand-900/40'
                      : 'border-ink-200 dark:border-ink-700 hover:border-brand-300 dark:hover:border-brand-700 hover:bg-brand-50 dark:hover:bg-brand-900/20 text-ink-600 dark:text-ink-300',
                  ].join(' ')}
                >
                  <span>{t.emoji || '✏️'}</span>
                  <span className="font-medium">{t.label}</span>
                  {t.desc && <span className="text-ink-400 dark:text-ink-500 text-[10px]">{t.desc}</span>}
                  {t.pinned && (
                    <svg width="9" height="9" viewBox="0 0 24 24" fill="currentColor" className="text-brand-500 shrink-0">
                      <polygon points="12 2 15 8.5 22 9.3 17 14.1 18.2 21 12 17.8 5.8 21 7 14.1 2 9.3 9 8.5 12 2" />
                    </svg>
                  )}
                </button>
              ))}
              {/* "Clear reason" affordance — only shown when textarea has
                  content. Equivalent to clicking the "其他" pseudo-chip
                  in the previous version. */}
              {rejectReason && (
                <button
                  type="button"
                  onClick={() => setRejectReason('')}
                  title="清空已填写的驳回原因"
                  className="inline-flex items-center gap-1 px-2 py-1 text-xs rounded-md border border-dashed border-ink-300 dark:border-ink-600 text-ink-500 dark:text-ink-400 hover:border-red-300 hover:text-red-600 dark:hover:text-red-400 transition-colors"
                >
                  <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
                    <polyline points="3 6 5 6 21 6" />
                    <path d="M19 6l-2 14a2 2 0 0 1-2 2H9a2 2 0 0 1-2-2L5 6" />
                  </svg>
                  清空
                </button>
              )}
            </div>
            <textarea
              value={rejectReason}
              onChange={(e) => setRejectReason(e.target.value)}
              rows={3}
              placeholder="描述为什么驳回（必填），例如：'业绩数据应改为 2024 年；招标编号与公告不一致'"
              className="w-full px-3 py-2 text-sm bg-white dark:bg-ink-900 border border-ink-200 dark:border-ink-700 rounded-lg focus:outline-none focus:border-brand-400 focus:ring-2 focus:ring-brand-100 dark:focus:ring-brand-900 text-ink-900 dark:text-white placeholder:text-ink-400 dark:placeholder:text-ink-500 resize-y"
            />
            <div className="flex items-center justify-between mt-1.5 text-[11px] text-ink-400 dark:text-ink-500">
              <span>
                {!rejectReason.trim() ? (
                  <span className="text-amber-600 dark:text-amber-400">驳回原因必填</span>
                ) : (
                  <span className="text-emerald-600 dark:text-emerald-400 inline-flex items-center gap-1">
                    <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
                      <polyline points="20 6 9 17 4 12" />
                    </svg>
                    已填写 {rejectReason.trim().length} 字
                  </span>
                )}
              </span>
              <span className="tabular-nums">将写入章节的 rejection_reason 字段</span>
            </div>
          </div>
        </div>
      </Modal>
    </div>
  )
}

// Re-export for tests / convenience
export { BID_STATUS_LABELS }
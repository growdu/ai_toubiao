import { useState, useCallback, useEffect, useMemo } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { bidsApi, AddChapterRequest } from '../../api/bids'
import { toast } from '../../lib/toast'
import { useHotkey } from '../../hooks/useHotkey'
import { Button } from '../../components/ui'
import { BID_STATUS_LABELS } from './workspace-helpers'
import { WorkspaceHeader } from './WorkspaceHeader'
import { MaterialPanel } from './MaterialPanel'
import { ChapterTree } from './ChapterTree'
import { ChapterEditor } from './ChapterEditor'
import { ChapterInspector } from './ChapterInspector'

export default function BidWorkspace() {
  const { id } = useParams<{ id: string }>()
  const queryClient = useQueryClient()
  const [selectedChapterId, setSelectedChapterId] = useState<string | null>(null)
  const [materialText, setMaterialText] = useState('')
  const [showMaterial, setShowMaterial] = useState(false)

  // ============ Queries ============
  const { data: bidData } = useQuery({
    queryKey: ['bid', id],
    queryFn: () => bidsApi.get(id!),
    enabled: !!id,
  })
  const bid = bidData?.data.data

  const { data: outlineData, isLoading: outlineLoading } = useQuery({
    queryKey: ['outline', id],
    queryFn: () => bidsApi.getOutline(id!),
    enabled: !!id,
    refetchInterval: 5000,
  })
  const chapters = outlineData?.data.data || []

  const { data: contentData } = useQuery({
    queryKey: ['chapter-content', id, selectedChapterId],
    queryFn: () => bidsApi.getChapterContent(id!, selectedChapterId!),
    enabled: !!id && !!selectedChapterId,
    refetchInterval: 3000,
  })
  const content = contentData?.data.data || null

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
  const updateMutation = useMutation({
    mutationFn: ({ chapterId, data }: { chapterId: string; data: any }) =>
      bidsApi.updateChapter(id!, chapterId, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['outline', id] })
    },
    onError: (err: any) => toast.error('更新失败', err?.response?.data?.message),
  })
  const transitionMutation = useMutation({
    mutationFn: ({ to, version }: { to: string; version: number }) =>
      bidsApi.transition(id!, to, version, `transition to ${to}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['bid', id] })
      toast.success('工作流已推进')
    },
    onError: (err: any) => toast.error('推进失败', err?.response?.data?.message),
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
    mutationFn: (chapterId: string) => bidsApi.generateChapter(id!, chapterId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['outline', id] })
      queryClient.invalidateQueries({ queryKey: ['bid', id] })
      queryClient.invalidateQueries({ queryKey: ['chapter-content', id, selectedChapterId] })
      toast.info('生成任务已提交', 'AI 正在编写内容…')
    },
    onError: (err: any) => toast.error('提交失败', err?.response?.data?.message),
  })
  const saveContentMutation = useMutation({
    mutationFn: ({ chapterId, text }: { chapterId: string; text: string }) =>
      bidsApi.saveChapterContent(id!, chapterId, text),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['chapter-content', id, selectedChapterId] })
      toast.success('内容已保存')
    },
    onError: (err: any) => toast.error('保存失败', err?.response?.data?.message),
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
    if (bid.status === 'generating') {
      actions.push(
        <Button key="audit" size="sm" variant="secondary" loading={transitionMutation.isPending}
          onClick={() => transitionMutation.mutate({ to: 'auditing', version: bid.version })}>
          开始审计
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

      {/* Live status strip — only shows when there's something to report */}
      {(inFlight > 0 || failedCount > 0 || generateMutation.isPending) && (
        <div className="px-6 py-2 bg-brand-50/60 border-b border-brand-100 flex items-center gap-3 text-xs text-brand-800 animate-slide-down">
          {generateMutation.isPending || inFlight > 0 ? (
            <>
              <span className="relative flex h-2 w-2">
                <span className="absolute inline-flex h-full w-full rounded-full bg-brand-400 opacity-75 animate-ping" />
                <span className="relative inline-flex rounded-full h-2 w-2 bg-brand-600" />
              </span>
              <span>
                <strong className="font-semibold">AI 正在生成</strong>
                {inFlight > 0 && <> · {inFlight} 个章节进行中</>}
                {failedCount > 0 && <> · {failedCount} 个失败</>}
              </span>
            </>
          ) : (
            <>
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="text-red-500">
                <circle cx="12" cy="12" r="10" />
                <line x1="12" y1="8" x2="12" y2="12" />
                <line x1="12" y1="16" x2="12.01" y2="16" />
              </svg>
              <span><strong className="font-semibold">{failedCount} 个章节生成失败</strong>，可在右侧重新生成</span>
            </>
          )}
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
              onGenerate={() => generateMutation.mutate(selectedChapter.id)}
              generating={generateMutation.isPending}
              onUpdateChapter={(data) => updateMutation.mutate({ chapterId: selectedChapter.id, data })}
            />
          ) : (
            <div className="h-full flex items-center justify-center px-6">
              <p className="text-sm text-ink-400 text-center">选择章节<br/>查看配置</p>
            </div>
          )}
        </aside>
      </div>
    </div>
  )
}

// Re-export for tests / convenience
export { BID_STATUS_LABELS }
import { useState, useEffect, useRef, useCallback, useMemo } from 'react'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import { Button, RingProgress } from '../../components/ui'
import { ChapterSpec, ChapterContent } from '../../api/bids'
import { useHotkey } from '../../hooks/useHotkey'

interface ChapterEditorProps {
  chapter: ChapterSpec
  content: ChapterContent | null
  bidId: string
  onSaveContent: (text: string) => Promise<unknown>
  onUpdateChapter: (data: any) => void
}

// Autosave delay — long enough to skip "type a few words then stop"
// saves but short enough that a tab refresh doesn't lose work.
const AUTOSAVE_DELAY_MS = 1500
// Cap the in-memory undo stack so the editor doesn't balloon the heap
// during a long editing session. 100 entries ≈ tens of KB each at most.
const MAX_UNDO = 100

export function ChapterEditor({ chapter, content, onSaveContent, onUpdateChapter }: ChapterEditorProps) {
  const [editingTitle, setEditingTitle] = useState(false)
  const [editTitle, setEditTitle] = useState(chapter.title)
  const [editingContent, setEditingContent] = useState(false)
  const [editContent, setEditContent] = useState('')
  const [saving, setSaving] = useState(false)
  const [dirty, setDirty] = useState(false)
  const [lastSavedAt, setLastSavedAt] = useState<Date | null>(null)
  const [editorMode, setEditorMode] = useState<'edit' | 'preview'>('edit')
  const autosaveTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  // Undo/redo stacks. Each stack entry holds a snapshot of editContent
  // *before* that keystroke was applied — so undo pops back to that value.
  // We don't snapshot on every keystroke (too expensive); we snapshot
  // every 600ms of inactivity within a "change burst" plus at the start
  // of each burst. This is the same heuristic Google Docs uses.
  const undoStack = useRef<string[]>([])
  const redoStack = useRef<string[]>([])
  const lastSnapshotAt = useRef<number>(0)
  const lastSnapshotValue = useRef<string>('')

  useEffect(() => {
    setEditTitle(chapter.title)
    setEditingTitle(false)
  }, [chapter.id, chapter.title])

  // Sync incoming content → local state, but only when we're not actively
  // editing. Otherwise typing would clobber itself when the polling query
  // (3s interval) refreshes the parent state mid-keystroke.
  useEffect(() => {
    if (!editingContent) {
      setEditContent(content?.content_text ?? '')
      // Reset undo/redo when switching to a fresh chapter
      undoStack.current = []
      redoStack.current = []
      lastSnapshotValue.current = content?.content_text ?? ''
    }
  }, [content?.chapter_spec_id, content?.content_text, editingContent])

  // Reset dirty + autosave timer when switching chapters
  useEffect(() => {
    setDirty(false)
    setLastSavedAt(null)
    setEditorMode('edit')
    if (autosaveTimer.current) {
      clearTimeout(autosaveTimer.current)
      autosaveTimer.current = null
    }
  }, [chapter.id])

  // Save callback — used by both autosave debounce and the manual "保存"
  // button. Notifies the parent so it can invalidate polling queries.
  const performSave = useCallback(async (text: string) => {
    if (!dirty && text === (content?.content_text ?? '')) return
    setSaving(true)
    try {
      await onSaveContent(text)
      setDirty(false)
      setLastSavedAt(new Date())
    } finally {
      setSaving(false)
    }
  }, [dirty, content?.content_text, onSaveContent])

  // Autosave: when dirty, schedule a save after AUTOSAVE_DELAY_MS. Reset
  // the timer on every keystroke so we only save once after the user
  // stops typing. The `dirty` dep is OK because we read its current
  // value through a ref pattern below.
  const editContentRef = useRef(editContent)
  editContentRef.current = editContent
  const performSaveRef = useRef(performSave)
  performSaveRef.current = performSave

  useEffect(() => {
    if (!dirty || !editingContent) return
    if (autosaveTimer.current) clearTimeout(autosaveTimer.current)
    autosaveTimer.current = setTimeout(() => {
      void performSaveRef.current(editContentRef.current)
    }, AUTOSAVE_DELAY_MS)
    return () => {
      if (autosaveTimer.current) {
        clearTimeout(autosaveTimer.current)
        autosaveTimer.current = null
      }
    }
  }, [editContent, dirty, editingContent])

  // beforeunload — guard against losing unsaved edits when the user
  // closes the tab, refreshes, or navigates away. We compare current
  // editContent against the saved snapshot we last saw.
  useEffect(() => {
    if (!dirty) return
    const handler = (e: BeforeUnloadEvent) => {
      e.preventDefault()
      // Modern browsers ignore the custom message but still show their
      // own confirmation dialog when returnValue is set.
      e.returnValue = ''
    }
    window.addEventListener('beforeunload', handler)
    return () => window.removeEventListener('beforeunload', handler)
  }, [dirty])

  // Cmd/Ctrl+Z / Shift+Cmd+Z hotkeys. Bound only when the user is in
  // edit mode. We use useHotkey directly (it correctly fires inside
  // textareas when the combo includes a modifier key).
  useHotkey('mod+z', (e) => { e.preventDefault(); undo() }, { enabled: editingContent })
  useHotkey('mod+shift+z', (e) => { e.preventDefault(); redo() }, { enabled: editingContent })
  useHotkey('mod+y', (e) => { e.preventDefault(); redo() }, { enabled: editingContent }) // Windows convention

  const hasContent = !!(content && content.content_text && content.content_text.length > 0)
  const meetsMin = !!(content && content.min_word_met)
  const currentWords = content?.word_count ?? 0
  const targetWords = chapter.target_word_count || 1
  const progressPercent = Math.min(100, Math.round((currentWords / targetWords) * 100))

  // Push current value onto undo stack and apply the next one (taken
  // from redo). No-op when the stacks are empty.
  const undo = useCallback(() => {
    if (undoStack.current.length === 0) return
    const popped = undoStack.current.pop()!
    redoStack.current.push(lastSnapshotValue.current)
    if (redoStack.current.length > MAX_UNDO) redoStack.current.shift()
    lastSnapshotValue.current = popped
    setEditContent(popped)
    setDirty(true)
  }, [])

  const redo = useCallback(() => {
    if (redoStack.current.length === 0) return
    const popped = redoStack.current.pop()!
    undoStack.current.push(lastSnapshotValue.current)
    if (undoStack.current.length > MAX_UNDO) undoStack.current.shift()
    lastSnapshotValue.current = popped
    setEditContent(popped)
    setDirty(true)
  }, [])

  const handleEditContentChange = (v: string) => {
    // Snapshot the previous value if this is a new burst (>=600ms since
    // last snapshot) or if it represents a "big enough" diff (e.g. the
    // user pasted something). Without this heuristic we'd either capture
    // every keystroke (wasteful) or never capture (broken undo).
    const now = Date.now()
    const isBurst = now - lastSnapshotAt.current > 600
    const isPaste = v.length - lastSnapshotValue.current.length > 20
    if (isBurst || isPaste) {
      undoStack.current.push(lastSnapshotValue.current)
      if (undoStack.current.length > MAX_UNDO) undoStack.current.shift()
      // Any new edit invalidates redo history (standard editor behaviour)
      redoStack.current = []
      lastSnapshotAt.current = now
    }
    setEditContent(v)
    setDirty(v !== (content?.content_text ?? ''))
  }

  // Markdown preview HTML — recomputed on demand. `marked` is sync for
  // typical chapter sizes (<1ms). DOMPurify strips XSS vectors before
  // the HTML is injected, so user-pasted <script> tags can't escape
  // the preview container.
  //
  // Two input sources:
  // • edit mode: source = local editContent state (real-time as user types)
  // • read-only mode: source = server-fetched content.content_text
  //   (renders saved content even when the user hasn't clicked 编辑)
  const previewSource = editingContent ? editContent : (content?.content_text ?? '')
  const previewHtml = useMemo(() => {
    if (!previewSource) return ''
    const raw = marked.parse(previewSource, { async: false }) as string
    return DOMPurify.sanitize(raw)
  }, [previewSource])

  return (
    <div className="flex flex-col h-full">
      {/* Editor header */}
      <div className="px-6 py-4 border-b border-ink-100 dark:border-ink-700 bg-white dark:bg-ink-800">
        <div className="flex items-center justify-between gap-3 mb-3">
          {editingTitle ? (
            <div className="flex-1 flex gap-2">
              <input
                value={editTitle}
                onChange={(e) => setEditTitle(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') { onUpdateChapter({ title: editTitle }); setEditingTitle(false) }
                  if (e.key === 'Escape') { setEditTitle(chapter.title); setEditingTitle(false) }
                }}
                className="flex-1 px-2 py-1 border border-ink-200 dark:border-ink-700 rounded text-lg font-bold bg-white dark:bg-ink-900 text-ink-900 dark:text-white focus:outline-none focus:border-brand-400 focus:ring-2 focus:ring-brand-100 dark:focus:ring-brand-900"
                autoFocus
              />
              <Button size="sm" variant="primary" onClick={() => { onUpdateChapter({ title: editTitle }); setEditingTitle(false) }}>保存</Button>
              <Button size="sm" variant="ghost" onClick={() => { setEditTitle(chapter.title); setEditingTitle(false) }}>取消</Button>
            </div>
          ) : (
            <>
              <h2 className="text-xl font-bold text-ink-900 dark:text-white tracking-tight">{chapter.title}</h2>
              <button
                onClick={() => setEditingTitle(true)}
                className="shrink-0 inline-flex items-center gap-1 text-xs text-ink-500 dark:text-ink-400 hover:text-brand-600 dark:hover:text-brand-400 transition-colors"
              >
                <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7" />
                  <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z" />
                </svg>
                编辑标题
              </button>
            </>
          )}
        </div>

        {/* Meta row with ring progress on the right */}
        <div className="flex items-center gap-5">
          <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-ink-500 dark:text-ink-400 flex-1 min-w-0">
            <span className="inline-flex items-center gap-1.5">
              <span className="text-ink-400">层级</span>
              <strong className="text-ink-700 dark:text-ink-200">L{chapter.level}</strong>
            </span>
            <span className="inline-flex items-center gap-1.5">
              <span className="text-ink-400">目标</span>
              <strong className="text-ink-700 dark:text-ink-200 tabular-nums">{chapter.target_word_count}</strong> 字
            </span>
            <span className="inline-flex items-center gap-1.5">
              <span className="text-ink-400">最低</span>
              <strong className="text-ink-700 dark:text-ink-200 tabular-nums">{chapter.min_word_count}</strong> 字
            </span>
            <span className="inline-flex items-center gap-1.5">
              <span className="text-ink-400">当前</span>
              <strong className={`tabular-nums ${meetsMin ? 'text-emerald-600 dark:text-emerald-400' : 'text-amber-600 dark:text-amber-400'}`}>
                {currentWords}
              </strong>
              字
              {!meetsMin && hasContent && (
                <span className="inline-flex items-center gap-0.5 text-red-600 dark:text-red-400 ml-1">
                  <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
                    <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z" />
                    <line x1="12" y1="9" x2="12" y2="13" />
                    <line x1="12" y1="17" x2="12.01" y2="17" />
                  </svg>
                  未达标
                </span>
              )}
            </span>
            {content?.generation_duration_ms ? (
              <span className="inline-flex items-center gap-1.5">
                <span className="text-ink-400">耗时</span>
                <strong className="text-ink-700 dark:text-ink-200 tabular-nums">{(content.generation_duration_ms / 1000).toFixed(1)}s</strong>
              </span>
            ) : null}
          </div>

          {/* Word-count ring on the right */}
          <RingProgress
            value={progressPercent}
            size={52}
            strokeWidth={4}
            tone={meetsMin ? 'success' : currentWords === 0 ? 'brand' : 'amber'}
          >
            <div className="text-center leading-none">
              <div className={`text-sm font-bold tabular-nums ${meetsMin ? 'text-emerald-600' : 'text-ink-700 dark:text-ink-200'}`}>
                {currentWords}
              </div>
              <div className="text-[8px] text-ink-400 dark:text-ink-500 mt-0.5">/ {chapter.target_word_count}</div>
            </div>
          </RingProgress>
        </div>
      </div>

      {/* Content body */}
      <div className="flex-1 overflow-y-auto scrollbar-thin relative">
        {!hasContent ? (
          <div className="h-full flex items-center justify-center px-6 py-12">
            <div className="text-center max-w-sm">
              <div className="mx-auto w-20 h-20 rounded-2xl bg-gradient-to-br from-brand-100 to-brand-50 dark:from-brand-900/40 dark:to-ink-800 border border-brand-200 dark:border-brand-800 flex items-center justify-center text-3xl mb-5 shadow-soft">
                ✍️
              </div>
              <h3 className="text-base font-semibold text-ink-800 dark:text-white mb-1">本章节尚无内容</h3>
              <p className="text-sm text-ink-500 dark:text-ink-400 mb-1">在右侧面板点击"生成此章节"使用 AI 自动撰写</p>
              <p className="text-xs text-ink-400 dark:text-ink-500">也可以手动编写后保存</p>
            </div>
          </div>
        ) : editingContent ? (
          <div className="h-full flex flex-col p-6">
            {/* Toolbar: edit/preview tab + undo/redo buttons.
                Positioned above the textarea so it doesn't fight the
                user for vertical space. */}
            <div className="flex items-center gap-1 mb-3">
              <div className="inline-flex items-center gap-0.5 p-0.5 bg-ink-100 dark:bg-ink-800 rounded-lg">
                <button
                  onClick={() => setEditorMode('edit')}
                  className={[
                    'inline-flex items-center gap-1.5 px-2.5 py-1 text-xs font-medium rounded-md transition-all',
                    editorMode === 'edit'
                      ? 'bg-white dark:bg-ink-700 text-brand-700 dark:text-brand-300 shadow-sm'
                      : 'text-ink-600 dark:text-ink-300 hover:text-ink-900 dark:hover:text-white',
                  ].join(' ')}
                >
                  <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                    <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7" />
                    <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z" />
                  </svg>
                  编辑
                </button>
                <button
                  onClick={() => setEditorMode('preview')}
                  className={[
                    'inline-flex items-center gap-1.5 px-2.5 py-1 text-xs font-medium rounded-md transition-all',
                    editorMode === 'preview'
                      ? 'bg-white dark:bg-ink-700 text-brand-700 dark:text-brand-300 shadow-sm'
                      : 'text-ink-600 dark:text-ink-300 hover:text-ink-900 dark:hover:text-white',
                  ].join(' ')}
                >
                  <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                    <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z" />
                    <circle cx="12" cy="12" r="3" />
                  </svg>
                  预览
                </button>
              </div>
              <div className="ml-2 flex items-center gap-0.5">
                <button
                  onClick={undo}
                  disabled={undoStack.current.length === 0}
                  title="撤销 (⌘Z)"
                  aria-label="撤销"
                  className="inline-flex items-center justify-center w-7 h-7 rounded-md text-ink-500 dark:text-ink-400 hover:bg-ink-100 dark:hover:bg-ink-700 hover:text-ink-800 dark:hover:text-ink-100 disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
                >
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                    <polyline points="9 14 4 9 9 4" />
                    <path d="M20 20v-7a4 4 0 0 0-4-4H4" />
                  </svg>
                </button>
                <button
                  onClick={redo}
                  disabled={redoStack.current.length === 0}
                  title="重做 (⌘⇧Z)"
                  aria-label="重做"
                  className="inline-flex items-center justify-center w-7 h-7 rounded-md text-ink-500 dark:text-ink-400 hover:bg-ink-100 dark:hover:bg-ink-700 hover:text-ink-800 dark:hover:text-ink-100 disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
                >
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                    <polyline points="15 14 20 9 15 4" />
                    <path d="M4 20v-7a4 4 0 0 1 4-4h12" />
                  </svg>
                </button>
              </div>
              <span className="text-[10px] text-ink-400 dark:text-ink-500 ml-2 tabular-nums">
                {editorMode === 'edit'
                  ? '支持 Markdown · ⌘Z 撤销'
                  : '只读预览'}
              </span>
            </div>

            {/* Editor / Preview body */}
            <div className="flex-1 min-h-0">
              {editorMode === 'edit' ? (
                <textarea
                  value={editContent}
                  onChange={(e) => handleEditContentChange(e.target.value)}
                  className="w-full h-full p-4 border border-ink-200 dark:border-ink-700 rounded-xl font-mono text-sm leading-relaxed resize-none focus:outline-none focus:border-brand-400 focus:ring-2 focus:ring-brand-100 dark:focus:ring-brand-900 bg-white dark:bg-ink-900 text-ink-900 dark:text-white"
                />
              ) : (
                <div
                  className="md-preview w-full h-full p-6 border border-ink-200 dark:border-ink-700 rounded-xl bg-white dark:bg-ink-900 overflow-y-auto scrollbar-thin text-ink-800 dark:text-ink-100 leading-7"
                  dangerouslySetInnerHTML={{ __html: previewHtml || '<p class="text-ink-400 italic">无内容预览</p>' }}
                />
              )}
            </div>

            <div className="flex items-center justify-between mt-3 px-1 gap-3">
              <div className="flex items-center gap-3 text-xs">
                <span className="inline-flex items-center gap-2 text-ink-500 dark:text-ink-400">
                  字数：
                  <strong className="text-ink-700 dark:text-ink-200 tabular-nums">{editContent.length}</strong>
                  <span className={`tabular-nums ${editContent.length >= chapter.min_word_count ? 'text-emerald-600 dark:text-emerald-400' : 'text-amber-600 dark:text-amber-400'}`}>
                    / 最低 {chapter.min_word_count}
                  </span>
                </span>
                {/* Save status indicator — green for saved, amber for dirty,
                    blue spinner for autosave-in-flight. Positioned inline so
                    the user can see at a glance whether their keystrokes
                    have been persisted. */}
                {saving ? (
                  <span className="inline-flex items-center gap-1 text-brand-600 dark:text-brand-400">
                    <svg className="animate-spin w-3 h-3" viewBox="0 0 24 24" fill="none">
                      <circle cx="12" cy="12" r="10" stroke="currentColor" strokeOpacity="0.25" strokeWidth="4" />
                      <path d="M22 12a10 10 0 0 1-10 10" stroke="currentColor" strokeWidth="4" strokeLinecap="round" />
                    </svg>
                    <span>正在保存…</span>
                  </span>
                ) : dirty ? (
                  <span className="inline-flex items-center gap-1 text-amber-600 dark:text-amber-400 font-medium">
                    <span className="w-1.5 h-1.5 rounded-full bg-amber-500" />
                    有未保存修改
                  </span>
                ) : lastSavedAt ? (
                  <span className="inline-flex items-center gap-1 text-emerald-600 dark:text-emerald-400">
                    <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
                      <polyline points="20 6 9 17 4 12" />
                    </svg>
                    已保存 · {lastSavedAt.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })}
                  </span>
                ) : null}
              </div>
              <div className="flex gap-2">
                <Button
                  variant="ghost"
                  onClick={() => {
                    if (dirty && !confirm('放弃本次未保存的修改？')) return
                    setEditContent(content?.content_text ?? '')
                    setDirty(false)
                    setEditingContent(false)
                  }}
                >
                  取消
                </Button>
                <Button
                  variant="primary"
                  loading={saving}
                  onClick={() => performSave(editContent)}
                >
                  保存内容
                </Button>
              </div>
            </div>
          </div>
        ) : (
          <div className="max-w-3xl mx-auto px-8 py-8">
            <div className="flex justify-end mb-4">
              <button
                onClick={() => setEditingContent(true)}
                className="inline-flex items-center gap-1.5 text-xs text-ink-500 dark:text-ink-400 hover:text-brand-600 dark:hover:text-brand-400 transition-colors px-2 py-1 rounded hover:bg-brand-50 dark:hover:bg-brand-900/40"
              >
                <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7" />
                  <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z" />
                </svg>
                编辑内容
              </button>
            </div>
            <article
              className="md-preview prose-equivalent max-w-none text-ink-800 dark:text-ink-100 leading-7"
              dangerouslySetInnerHTML={{ __html: previewHtml || '<p class="text-ink-400 italic">无内容</p>' }}
            />
          </div>
        )}
      </div>
    </div>
  )
}
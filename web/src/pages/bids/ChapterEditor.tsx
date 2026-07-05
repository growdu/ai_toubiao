import { useState, useEffect } from 'react'
import { Button, StatusBadge } from '../../components/ui'
import { ChapterSpec, ChapterContent } from '../../api/bids'
import { CHAPTER_STATUS_LABELS } from './workspace-helpers'

interface ChapterEditorProps {
  chapter: ChapterSpec
  content: ChapterContent | null
  bidId: string
  onSaveContent: (text: string) => Promise<unknown>
  onUpdateChapter: (data: any) => void
}

export function ChapterEditor({ chapter, content, onSaveContent, onUpdateChapter }: ChapterEditorProps) {
  const [editingTitle, setEditingTitle] = useState(false)
  const [editTitle, setEditTitle] = useState(chapter.title)
  const [editingContent, setEditingContent] = useState(false)
  const [editContent, setEditContent] = useState('')
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    setEditTitle(chapter.title)
    setEditingTitle(false)
  }, [chapter.id, chapter.title])

  useEffect(() => {
    setEditContent(content?.content_text ?? '')
  }, [content?.chapter_spec_id, content?.content_text])

  const hasContent = !!(content && content.content_text && content.content_text.length > 0)
  const meetsMin = !!(content && content.min_word_met)

  return (
    <div className="flex flex-col h-full">
      {/* Editor header */}
      <div className="px-6 py-4 border-b border-ink-100 bg-white">
        <div className="flex items-center justify-between gap-3 mb-2">
          {editingTitle ? (
            <div className="flex-1 flex gap-2">
              <input
                value={editTitle}
                onChange={(e) => setEditTitle(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') { onUpdateChapter({ title: editTitle }); setEditingTitle(false) }
                  if (e.key === 'Escape') { setEditTitle(chapter.title); setEditingTitle(false) }
                }}
                className="flex-1 px-2 py-1 border border-ink-200 rounded text-lg font-bold focus:outline-none focus:border-brand-400 focus:ring-2 focus:ring-brand-100"
                autoFocus
              />
              <Button size="sm" variant="primary" onClick={() => { onUpdateChapter({ title: editTitle }); setEditingTitle(false) }}>保存</Button>
              <Button size="sm" variant="ghost" onClick={() => { setEditTitle(chapter.title); setEditingTitle(false) }}>取消</Button>
            </div>
          ) : (
            <>
              <h2 className="text-xl font-bold text-ink-900 tracking-tight">{chapter.title}</h2>
              <button
                onClick={() => setEditingTitle(true)}
                className="shrink-0 inline-flex items-center gap-1 text-xs text-ink-500 hover:text-brand-600 transition-colors"
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

        {/* Meta row */}
        <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-ink-500">
          <StatusBadge status={chapter.status} labels={CHAPTER_STATUS_LABELS} />
          <span className="inline-flex items-center gap-1.5">
            <span className="text-ink-400">层级</span>
            <strong className="text-ink-700">L{chapter.level}</strong>
          </span>
          <span className="inline-flex items-center gap-1.5">
            <span className="text-ink-400">目标</span>
            <strong className="text-ink-700 tabular-nums">{chapter.target_word_count}</strong> 字
          </span>
          <span className="inline-flex items-center gap-1.5">
            <span className="text-ink-400">最低</span>
            <strong className="text-ink-700 tabular-nums">{chapter.min_word_count}</strong> 字
          </span>
          <span className="inline-flex items-center gap-1.5">
            <span className="text-ink-400">当前</span>
            <strong className={`tabular-nums ${meetsMin ? 'text-emerald-600' : 'text-amber-600'}`}>
              {content?.word_count ?? 0}
            </strong>
            字
            {!meetsMin && hasContent && (
              <span className="inline-flex items-center gap-0.5 text-red-600 ml-1">
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
              <strong className="text-ink-700 tabular-nums">{(content.generation_duration_ms / 1000).toFixed(1)}s</strong>
            </span>
          ) : null}
        </div>
      </div>

      {/* Content body */}
      <div className="flex-1 overflow-y-auto scrollbar-thin">
        {!hasContent ? (
          <div className="h-full flex items-center justify-center px-6 py-12">
            <div className="text-center max-w-sm">
              <div className="mx-auto w-20 h-20 rounded-2xl bg-brand-gradient-soft border border-brand-100 flex items-center justify-center text-3xl mb-5">
                ✍️
              </div>
              <h3 className="text-base font-semibold text-ink-800 mb-1">本章节尚无内容</h3>
              <p className="text-sm text-ink-500 mb-1">在右侧面板点击"生成此章节"使用 AI 自动撰写</p>
              <p className="text-xs text-ink-400">也可以手动编写后保存</p>
            </div>
          </div>
        ) : editingContent ? (
          <div className="h-full flex flex-col p-6">
            <textarea
              value={editContent}
              onChange={(e) => setEditContent(e.target.value)}
              className="flex-1 w-full p-4 border border-ink-200 rounded-xl font-mono text-sm leading-relaxed resize-none focus:outline-none focus:border-brand-400 focus:ring-2 focus:ring-brand-100 bg-white"
            />
            <div className="flex items-center justify-between mt-3">
              <span className="text-xs text-ink-400">
                字数：<strong className="text-ink-700 tabular-nums">{editContent.length}</strong>
              </span>
              <div className="flex gap-2">
                <Button variant="ghost" onClick={() => { setEditContent(content?.content_text ?? ''); setEditingContent(false) }}>
                  取消
                </Button>
                <Button
                  variant="primary"
                  loading={saving}
                  onClick={async () => {
                    setSaving(true)
                    try { await onSaveContent(editContent); setEditingContent(false) }
                    finally { setSaving(false) }
                  }}
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
                className="inline-flex items-center gap-1.5 text-xs text-ink-500 hover:text-brand-600 transition-colors px-2 py-1 rounded hover:bg-brand-50"
              >
                <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7" />
                  <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z" />
                </svg>
                编辑内容
              </button>
            </div>
            <article className="prose prose-sm max-w-none text-ink-800 leading-7 whitespace-pre-wrap">
              {content?.content_text}
            </article>
          </div>
        )}
      </div>
    </div>
  )
}
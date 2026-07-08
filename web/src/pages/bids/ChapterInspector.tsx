import { useState, useEffect } from 'react'
import { TextArea, Button, StatusBadge, Tabs } from '../../components/ui'
import { ChapterSpec, ChapterContent } from '../../api/bids'
import { CHAPTER_STATUS_LABELS } from './workspace-helpers'
import { chapterToEventLog } from '../../lib/eventSource'

interface ChapterInspectorProps {
  chapter: ChapterSpec
  content: ChapterContent | null
  bidStatus: string
  onGenerate: (prompt?: string) => void
  onApprove?: () => void
  onReject?: () => void
  generating: boolean
  approving?: boolean
  onUpdateChapter: (data: any) => void
}

type InspectorTab = 'config' | 'prompt' | 'status'

export function ChapterInspector({ chapter, content, bidStatus, onGenerate, onApprove, onReject, generating, approving, onUpdateChapter }: ChapterInspectorProps) {
  // useState (not useRef) so the live character counter in the tab
  // header can re-render as the user types. Persistence across tab
  // switches comes from keeping prompt in component state without
  // resetting it in the chapter.id effect — that effect only resets
  // when the user *switches chapters*, which is the correct moment
  // (a new chapter shouldn't inherit the previous chapter's prompt).
  const [prompt, setPrompt] = useState('')
  const [tab, setTab] = useState<InspectorTab>('config')

  useEffect(() => {
    // Reset prompt + tab when switching chapters
    setPrompt('')
    setTab('config')
  }, [chapter.id])

  const canGenerate = bidStatus === 'facts' || bidStatus === 'generating' || chapter.status === 'planned' || chapter.status === 'failed'

  return (
    <div className="flex flex-col h-full overflow-hidden">
      <Tabs
        variant="underline"
        value={tab}
        onChange={(v) => setTab(v as InspectorTab)}
        items={[
          { id: 'config', label: '配置', icon: <ConfigIcon /> },
          { id: 'prompt', label: '提示词', icon: <PromptIcon /> },
          { id: 'status', label: '状态', icon: <StatusIcon /> },
        ]}
        className="px-3 pt-2 bg-ink-50 dark:bg-ink-900"
      />

      <div className="flex-1 overflow-y-auto scrollbar-thin bg-ink-50 dark:bg-ink-900">
        {/* Config tab */}
        {tab === 'config' && (
          <section className="px-4 py-4 space-y-3 animate-fade-in">
            <ConfigRow label="优先级">
              <select
                defaultValue={chapter.priority}
                onChange={(e) => onUpdateChapter({ priority: e.target.value })}
                className="w-full px-2 py-1.5 bg-white dark:bg-ink-800 border border-ink-200 dark:border-ink-700 rounded-lg text-sm text-ink-700 dark:text-ink-200 focus:outline-none focus:border-brand-400 focus:ring-2 focus:ring-brand-100 dark:focus:ring-brand-900"
              >
                <option value="critical">关键</option>
                <option value="high">高</option>
                <option value="normal">普通</option>
                <option value="low">低</option>
              </select>
            </ConfigRow>
            <ConfigRow label="目标字数">
              <input
                type="number"
                defaultValue={chapter.target_word_count}
                onBlur={(e) => {
                  const v = parseInt(e.target.value, 10)
                  if (!isNaN(v) && v !== chapter.target_word_count) onUpdateChapter({ target_word_count: v })
                }}
                className="w-full px-2 py-1.5 bg-white dark:bg-ink-800 border border-ink-200 dark:border-ink-700 rounded-lg text-sm tabular-nums text-ink-700 dark:text-ink-200 focus:outline-none focus:border-brand-400 focus:ring-2 focus:ring-brand-100 dark:focus:ring-brand-900"
              />
            </ConfigRow>
            <ConfigRow label="最低字数">
              <input
                type="number"
                defaultValue={chapter.min_word_count}
                onBlur={(e) => {
                  const v = parseInt(e.target.value, 10)
                  if (!isNaN(v) && v !== chapter.min_word_count) onUpdateChapter({ min_word_count: v })
                }}
                className="w-full px-2 py-1.5 bg-white dark:bg-ink-800 border border-ink-200 dark:border-ink-700 rounded-lg text-sm tabular-nums text-ink-700 dark:text-ink-200 focus:outline-none focus:border-brand-400 focus:ring-2 focus:ring-brand-100 dark:focus:ring-brand-900"
              />
            </ConfigRow>
            <ConfigRow label="写作风格">
              <select
                defaultValue={chapter.writing_style}
                className="w-full px-2 py-1.5 bg-white dark:bg-ink-800 border border-ink-200 dark:border-ink-700 rounded-lg text-sm text-ink-700 dark:text-ink-200 focus:outline-none focus:border-brand-400 focus:ring-2 focus:ring-brand-100 dark:focus:ring-brand-900"
              >
                <option value="formal">正式严谨</option>
                <option value="concise">简洁精炼</option>
                <option value="detailed">详尽展开</option>
              </select>
            </ConfigRow>
          </section>
        )}

        {/* Prompt tab */}
        {tab === 'prompt' && (
          <section className="px-4 py-4 animate-fade-in">
            <div className="flex items-center justify-between mb-2">
              <h3 className="text-xs font-bold text-ink-500 dark:text-ink-400 uppercase tracking-wider">生成提示词</h3>
              <span className="text-[10px] text-ink-400 dark:text-ink-500 tabular-nums">{prompt.length} 字</span>
            </div>
            <TextArea
              value={prompt}
              onChange={(e) => setPrompt(e.target.value)}
              rows={10}
              placeholder={`为"${chapter.title}"指定重点，例如：\n• 突出技术优势与项目案例\n• 引用相关资质和认证\n• 强调团队配置与实施计划`}
              className="text-xs"
            />
            <p className="text-[10px] text-ink-400 dark:text-ink-500 mt-2 leading-relaxed">
              提示词仅影响本章节的生成内容，会作为额外上下文传给 LLM
            </p>
          </section>
        )}

        {/* Status tab */}
        {tab === 'status' && (
          <section className="px-4 py-4 animate-fade-in">
            <h3 className="text-xs font-bold text-ink-500 dark:text-ink-400 uppercase tracking-wider mb-3">内容状态</h3>
            <dl className="space-y-2 text-sm">
              <Row k="状态"><StatusBadge status={chapter.status} labels={CHAPTER_STATUS_LABELS} /></Row>
              <Row k="字数">
                <span className="tabular-nums">
                  <strong className="text-ink-800 dark:text-ink-100">{content?.word_count ?? 0}</strong>
                  <span className="text-ink-400 dark:text-ink-500"> / {chapter.target_word_count}</span>
                </span>
              </Row>
              {content?.llm_model && <Row k="模型"><span className="font-mono text-xs text-ink-600 dark:text-ink-300">{content.llm_model}</span></Row>}
              {content?.generation_duration_ms != null && (
                <Row k="耗时"><span className="tabular-nums">{(content.generation_duration_ms / 1000).toFixed(1)}s</span></Row>
              )}
              {content?.generated_by && <Row k="生成者"><span className="text-xs">{content.generated_by}</span></Row>}
              {!content && (
                <div className="mt-4 p-3 rounded-lg bg-ink-100 dark:bg-ink-800 text-center text-xs text-ink-500 dark:text-ink-400">
                  尚未生成内容
                </div>
              )}
            </dl>

            {/* 审核历史 timeline — driven from chapterToEventLog() so the
                shape matches future backend event-log entries. We
                still merge in the "AI 生成" event from content.gen*
                because the adapter today doesn't track generation
                history. When a real event log lands, this whole
                block becomes one <ol> map. */}
            {(() => {
              const events = chapterToEventLog({
                ...chapter,
                bid_id: chapter.bid_job_id,
              })
              // Synthesize an "AI 生成" event from content metadata.
              // Today this comes from content.llm_model + generated_by;
              // tomorrow it will be one entry per generation pass.
              if (content?.generated_by) {
                events.push({
                  id: `${chapter.id}:generated`,
                  bidId: chapter.bid_job_id,
                  chapterId: chapter.id,
                  kind: 'regenerated',
                  label: 'AI 生成',
                  description: content.llm_model
                    ? `${content.llm_model}${content.generation_duration_ms != null ? ` · ${(content.generation_duration_ms / 1000).toFixed(1)}s` : ''}`
                    : content.generated_by,
                  actor: { id: 'llm', name: 'AI', role: 'llm' },
                  at: events[0]?.at || new Date().toISOString(),
                })
              }
              if (events.length === 0) {
                return (
                  <ol className="space-y-2.5">
                    <li className="text-xs text-ink-400 dark:text-ink-500 text-center py-3">
                      暂无历史事件
                    </li>
                  </ol>
                )
              }
              return (
                <ol className="space-y-2.5">
                  {events.sort((a, b) => b.at.localeCompare(a.at)).map(ev => {
                    const isReject = ev.kind === 'rejected'
                    const isApprove = ev.kind === 'approved'
                    const isRegen = ev.kind === 'regenerated'
                    // Color the timestamp dot by event kind. The rest of
                    // the markup is identical to the pre-refactor version
                    // — visual parity was the priority here.
                    const ring = isReject
                      ? 'bg-red-100 dark:bg-red-900/40 text-red-600 dark:text-red-400'
                      : isApprove
                        ? 'bg-emerald-100 dark:bg-emerald-900/40 text-emerald-600 dark:text-emerald-400'
                        : isRegen
                          ? 'bg-brand-100 dark:bg-brand-900/40 text-brand-600 dark:text-brand-400'
                          : 'bg-ink-100 dark:bg-ink-800 text-ink-600 dark:text-ink-400'
                    const labelCls = isReject
                      ? 'text-red-700 dark:text-red-400'
                      : isApprove
                        ? 'text-emerald-700 dark:text-emerald-400'
                        : isRegen
                          ? 'text-brand-700 dark:text-brand-400'
                          : 'text-ink-700 dark:text-ink-300'
                    return (
                      <li key={ev.id} className="flex gap-2.5">
                        <div className="shrink-0 mt-0.5">
                          <div className={`w-6 h-6 rounded-full grid place-items-center ${ring}`}>
                            {isApprove ? (
                              <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
                                <polyline points="20 6 9 17 4 12" />
                              </svg>
                            ) : isReject ? (
                              <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
                                <circle cx="12" cy="12" r="10" />
                                <line x1="15" y1="9" x2="9" y2="15" />
                                <line x1="9" y1="9" x2="15" y2="15" />
                              </svg>
                            ) : isRegen ? (
                              <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                                <path d="M12 2L9 9 2 9.5l5.5 4.7L5.5 22 12 18l6.5 4-2-7.8L22 9.5 15 9z" />
                              </svg>
                            ) : (
                              <span className="text-[10px]">·</span>
                            )}
                          </div>
                        </div>
                        <div className="min-w-0 flex-1">
                          <div className="flex items-center justify-between gap-2">
                            <span className={`text-xs font-semibold ${labelCls}`}>{ev.label}</span>
                            <span className="text-[10px] text-ink-400 dark:text-ink-500 tabular-nums">
                              {new Date(ev.at).toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })}
                            </span>
                          </div>
                          {ev.description && (
                            <div className={`mt-1 text-[11px] ${isReject ? 'px-2 py-1.5 rounded bg-red-50 dark:bg-red-900/20 border border-red-100 dark:border-red-900/30 text-ink-700 dark:text-ink-300' : 'text-[10px] text-ink-500 dark:text-ink-400'}`}>
                              {ev.description}
                            </div>
                          )}
                          {!isReject && ev.description && ev.kind !== 'regenerated' && (
                            <div className="text-[10px] text-ink-500 dark:text-ink-400 mt-0.5">
                              操作人 <span className="font-mono text-ink-700 dark:text-ink-300">{ev.actor.name === '系统' ? '—' : ev.actor.name}</span>
                            </div>
                          )}
                        </div>
                      </li>
                    )
                  })}
                </ol>
              )
            })()}
          </section>
        )}
      </div>

      {/* Actions — sticky footer */}
      <section className="px-4 py-3 sticky bottom-0 bg-white dark:bg-ink-800 border-t border-ink-100 dark:border-ink-700 space-y-2 shrink-0">
        {bidStatus === 'facts' && (
          <div className="text-[11px] text-brand-700 dark:text-brand-300 bg-brand-50 dark:bg-brand-900/30 border border-brand-100 dark:border-brand-800/50 px-2.5 py-1.5 rounded-lg flex gap-1.5">
            <span>💡</span>
            <span>可使用顶部"批量生成内容"同时生成所有章节</span>
          </div>
        )}

        {/* When the chapter is in `awaiting_review` mode, the primary
            action is approve (human-gate). Reject + regenerate are
            secondary. This is the HIL pattern: the user must eyeball
            the AI output before the workflow can advance. */}
        {chapter.status === 'succeeded' && bidStatus === 'awaiting_review' && (onApprove || onReject) && (
          <>
            <Button
              variant="primary"
              className="w-full"
              loading={approving}
              onClick={onApprove}
              leftIcon={
                <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
                  <polyline points="20 6 9 17 4 12" />
                </svg>
              }
            >
              审核通过此章节
            </Button>
            {onReject && (
              <Button
                variant="ghost"
                className="w-full"
                size="sm"
                onClick={onReject}
                disabled={approving}
              >
                驳回（让 AI 重做）
              </Button>
            )}
            <div className="text-[10px] text-ink-400 dark:text-ink-500 text-center leading-relaxed">
              审核通过后，此章节标记为「已审」，不影响其他章节
            </div>
          </>
        )}

        {chapter.status === 'approved' && bidStatus === 'awaiting_review' && (
          <div className="text-[11px] text-emerald-700 dark:text-emerald-300 bg-emerald-50 dark:bg-emerald-900/30 border border-emerald-200 dark:border-emerald-800/50 px-2.5 py-1.5 rounded-lg flex items-start gap-1.5">
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" className="shrink-0 mt-0.5">
              <polyline points="20 6 9 17 4 12" />
            </svg>
            <span>已审核{chapter.approved_at && ` · ${new Date(chapter.approved_at).toLocaleString('zh-CN', { month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit' })}`}</span>
          </div>
        )}

        {/* Generate / regenerate — only show when not in awaiting_review,
            since at that point the user is reviewing, not generating.
            The onGenerate handler accepts an optional prompt; we pass
            the current prompt-tab text if any. Trimmed empties become
            undefined so the backend's "no custom prompt" branch fires. */}
        {chapter.status !== 'approved' && (
          <Button
            variant="primary"
            className="w-full"
            disabled={!canGenerate || generating}
            loading={generating}
            onClick={() => {
              const trimmed = prompt.trim()
              onGenerate(trimmed ? trimmed : undefined)
            }}
          >
            {chapter.status === 'succeeded' ? '🔄 重新生成' :
             chapter.status === 'failed' ? '🔄 重试生成' :
             '⚡ 生成此章节'}
            {prompt.trim() && (
              <span className="ml-1 text-[10px] font-normal opacity-80">（含提示词）</span>
            )}
          </Button>
        )}
        {!canGenerate && chapter.status !== 'approved' && (
          <p className="text-[11px] text-ink-400 dark:text-ink-500 text-center">当前工作流阶段不支持单章生成</p>
        )}
      </section>
    </div>
  )
}

function ConfigRow({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="block">
      <span className="block text-xs text-ink-500 dark:text-ink-400 mb-1">{label}</span>
      {children}
    </label>
  )
}

function Row({ k, children }: { k: string; children: React.ReactNode }) {
  return (
    <div className="flex items-center justify-between text-xs">
      <dt className="text-ink-500 dark:text-ink-400">{k}</dt>
      <dd className="text-ink-800 dark:text-ink-200">{children}</dd>
    </div>
  )
}

function ConfigIcon() {
  return <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><circle cx="12" cy="12" r="3" /><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 1 1-4 0v-.09a1.65 1.65 0 0 0-1-1.51 1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 1 1 0-4h.09a1.65 1.65 0 0 0 1.51-1 1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33h0a1.65 1.65 0 0 0 1-1.51V3a2 2 0 1 1 4 0v.09a1.65 1.65 0 0 0 1 1.51h0a1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82v0a1.65 1.65 0 0 0 1.51 1H21a2 2 0 1 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" /></svg>
}

function PromptIcon() {
  return <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M21 11.5a8.38 8.38 0 0 1-.9 3.8 8.5 8.5 0 0 1-7.6 4.7 8.38 8.38 0 0 1-3.8-.9L3 21l1.9-5.7a8.38 8.38 0 0 1-.9-3.8 8.5 8.5 0 0 1 4.7-7.6 8.38 8.38 0 0 1 3.8-.9h.5a8.48 8.48 0 0 1 8 8v.5z" /></svg>
}

function StatusIcon() {
  return <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><circle cx="12" cy="12" r="10" /><polyline points="12 6 12 12 16 14" /></svg>
}
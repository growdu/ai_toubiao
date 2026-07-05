import { useState, useEffect } from 'react'
import { TextArea, Button, StatusBadge } from '../../components/ui'
import { ChapterSpec, ChapterContent } from '../../api/bids'
import { CHAPTER_STATUS_LABELS } from './workspace-helpers'

interface ChapterInspectorProps {
  chapter: ChapterSpec
  content: ChapterContent | null
  bidStatus: string
  onGenerate: () => void
  generating: boolean
  onUpdateChapter: (data: any) => void
}

export function ChapterInspector({ chapter, content, bidStatus, onGenerate, generating, onUpdateChapter }: ChapterInspectorProps) {
  const [prompt, setPrompt] = useState('')

  useEffect(() => {
    // Reset prompt when switching chapters
    setPrompt('')
  }, [chapter.id])

  const canGenerate = bidStatus === 'facts' || bidStatus === 'generating' || chapter.status === 'planned' || chapter.status === 'failed'

  return (
    <div className="flex flex-col h-full overflow-y-auto scrollbar-thin">
      {/* Chapter meta */}
      <section className="px-4 py-4 border-b border-ink-100">
        <h3 className="text-xs font-bold text-ink-500 uppercase tracking-wider mb-3">章节配置</h3>
        <div className="space-y-3">
          <ConfigRow label="优先级">
            <select
              defaultValue={chapter.priority}
              onChange={(e) => onUpdateChapter({ priority: e.target.value })}
              className="w-full px-2 py-1.5 bg-white border border-ink-200 rounded-lg text-sm focus:outline-none focus:border-brand-400 focus:ring-2 focus:ring-brand-100"
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
              className="w-full px-2 py-1.5 bg-white border border-ink-200 rounded-lg text-sm tabular-nums focus:outline-none focus:border-brand-400 focus:ring-2 focus:ring-brand-100"
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
              className="w-full px-2 py-1.5 bg-white border border-ink-200 rounded-lg text-sm tabular-nums focus:outline-none focus:border-brand-400 focus:ring-2 focus:ring-brand-100"
            />
          </ConfigRow>
          <ConfigRow label="写作风格">
            <select
              defaultValue={chapter.writing_style}
              className="w-full px-2 py-1.5 bg-white border border-ink-200 rounded-lg text-sm focus:outline-none focus:border-brand-400 focus:ring-2 focus:ring-brand-100"
            >
              <option value="formal">正式严谨</option>
              <option value="concise">简洁精炼</option>
              <option value="detailed">详尽展开</option>
            </select>
          </ConfigRow>
        </div>
      </section>

      {/* Prompt */}
      <section className="px-4 py-4 border-b border-ink-100">
        <div className="flex items-center justify-between mb-2">
          <h3 className="text-xs font-bold text-ink-500 uppercase tracking-wider">生成提示词</h3>
          <span className="text-[10px] text-ink-400">{prompt.length} 字</span>
        </div>
        <TextArea
          value={prompt}
          onChange={(e) => setPrompt(e.target.value)}
          rows={7}
          placeholder={`为"${chapter.title}"指定重点，例如：\n• 突出技术优势与项目案例\n• 引用相关资质和认证\n• 强调团队配置与实施计划`}
        />
      </section>

      {/* Content status */}
      <section className="px-4 py-4 border-b border-ink-100">
        <h3 className="text-xs font-bold text-ink-500 uppercase tracking-wider mb-3">内容状态</h3>
        <dl className="space-y-2 text-sm">
          <Row k="状态"><StatusBadge status={chapter.status} labels={CHAPTER_STATUS_LABELS} /></Row>
          <Row k="字数">
            <span className="tabular-nums">
              <strong className="text-ink-800">{content?.word_count ?? 0}</strong>
              <span className="text-ink-400"> / {chapter.target_word_count}</span>
            </span>
          </Row>
          {content?.llm_model && <Row k="模型"><span className="font-mono text-xs text-ink-600">{content.llm_model}</span></Row>}
          {content?.generation_duration_ms != null && (
            <Row k="耗时"><span className="tabular-nums">{(content.generation_duration_ms / 1000).toFixed(1)}s</span></Row>
          )}
          {content?.generated_by && <Row k="生成者"><span className="text-xs">{content.generated_by}</span></Row>}
        </dl>
      </section>

      {/* Actions */}
      <section className="px-4 py-4 mt-auto sticky bottom-0 bg-white border-t border-ink-100 space-y-2">
        {bidStatus === 'facts' && (
          <div className="text-[11px] text-brand-700 bg-brand-50 border border-brand-100 px-2.5 py-1.5 rounded-lg flex gap-1.5">
            <span>💡</span>
            <span>可使用顶部"批量生成内容"同时生成所有章节</span>
          </div>
        )}
        <Button
          variant="primary"
          className="w-full"
          disabled={!canGenerate || generating}
          loading={generating}
          onClick={onGenerate}
        >
          {chapter.status === 'succeeded' ? '🔄 重新生成' :
           chapter.status === 'failed' ? '🔄 重试生成' :
           '⚡ 生成此章节'}
        </Button>
        {!canGenerate && (
          <p className="text-[11px] text-ink-400 text-center">当前工作流阶段不支持单章生成</p>
        )}
      </section>
    </div>
  )
}

function ConfigRow({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="block">
      <span className="block text-xs text-ink-500 mb-1">{label}</span>
      {children}
    </label>
  )
}

function Row({ k, children }: { k: string; children: React.ReactNode }) {
  return (
    <div className="flex items-center justify-between text-xs">
      <dt className="text-ink-500">{k}</dt>
      <dd className="text-ink-800">{children}</dd>
    </div>
  )
}
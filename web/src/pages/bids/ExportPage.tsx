import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { bidsApi } from '../../api/bids'
import { docgenApi } from '../../api/docgen'
import { useMutation } from '@tanstack/react-query'
import { toast } from '../../lib/toast'
import { Button, Card, ProgressBar, StatusBadge } from '../../components/ui'
import { BID_STATUS_LABELS, WORKFLOW_STEPS, workflowStepIndex } from './workspace-helpers'

type ExportFormat = 'word' | 'pdf'

const FORMAT_META: Record<ExportFormat, {
  label: string; ext: string; mime: string; tone: 'blue' | 'red'; icon: React.ReactNode; description: string; highlights: string[];
}> = {
  word: {
    label: 'Word 文档',
    ext: '.docx',
    mime: 'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
    tone: 'blue',
    icon: (
      <svg width="36" height="36" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round">
        <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
        <polyline points="14 2 14 8 20 8" />
        <line x1="8" y1="13" x2="16" y2="13" />
        <line x1="8" y1="17" x2="14" y2="17" />
      </svg>
    ),
    description: '适用于编辑、打印、加盖印章',
    highlights: ['可继续编辑修改', '支持签名盖章', '图表作为图片嵌入', '样式完全可控'],
  },
  pdf: {
    label: 'PDF 文档',
    ext: '.pdf',
    mime: 'application/pdf',
    tone: 'red',
    icon: (
      <svg width="36" height="36" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round">
        <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
        <polyline points="14 2 14 8 20 8" />
        <text x="7" y="18" fontSize="6" fontWeight="700" fill="currentColor" stroke="none">PDF</text>
      </svg>
    ),
    description: '适用于正式提交、存档、邮件发送',
    highlights: ['只读格式，安全可靠', '跨平台一致显示', '适合正式评审', '可设置打开密码'],
  },
}

export default function ExportPage() {
  const { id } = useParams<{ id: string }>()

  const { data, isLoading } = useQuery({
    queryKey: ['bid', id],
    queryFn: () => bidsApi.get(id!),
    enabled: !!id,
  })

  // Outline query — feeds the chapter preview section below. Fetched
  // independently of the bid status query so a slow outline doesn't
  // block the format-card UI from rendering.
  const { data: outlineData } = useQuery({
    queryKey: ['outline', id],
    queryFn: () => bidsApi.getOutline(id!),
    enabled: !!id,
  })
  const chapters: any[] = Array.isArray(outlineData?.data?.data) ? outlineData!.data!.data! : []

  const [showPreview, setShowPreview] = useState(false)

  const bid = (data?.data?.data ?? null) as any
  const ready = bid?.status === 'done'
  const progress = bid && bid.total_chapters > 0
    ? Math.round((bid.done_chapters / bid.total_chapters) * 100) : 0
  const stepIdx = workflowStepIndex(bid?.status ?? '')

  const [exporting, setExporting] = useState<ExportFormat | null>(null)
  const [error, setError] = useState<string | null>(null)

  const handleExport = async (format: ExportFormat) => {
    if (!id || exporting) return
    setError(null)
    setExporting(format)
    try {
      const { blob, filename } =
        format === 'word' ? await bidsApi.exportWord(id) : await bidsApi.exportPdf(id)
      triggerBrowserDownload(blob, filename)
      toast.success(`${FORMAT_META[format].label} 已下载`, filename)
    } catch (e) {
      const message = e instanceof Error ? e.message : '导出失败'
      setError(message)
      toast.error('导出失败', message)
    } finally {
      setExporting(null)
    }
  }

  if (isLoading) {
    return (
      <div className="flex-1 flex items-center justify-center">
        <div className="text-sm text-ink-400 dark:text-ink-500">加载中…</div>
      </div>
    )
  }

  return (
    <div className="flex-1 overflow-y-auto scrollbar-thin bg-ink-50 dark:bg-ink-900">
      <div className="max-w-4xl mx-auto px-6 py-8">
        {/* Breadcrumb */}
        <div className="flex items-center gap-2 text-xs text-ink-500 dark:text-ink-400 mb-4 animate-fade-in">
          <Link to="/bids" className="hover:text-ink-800 dark:hover:text-ink-200">标书列表</Link>
          <span className="text-ink-300 dark:text-ink-600">/</span>
          <Link to={`/bids/${id}`} className="hover:text-ink-800 dark:hover:text-ink-200 truncate max-w-xs">
            {bid?.project_name || '标书'}
          </Link>
          <span className="text-ink-300 dark:text-ink-600">/</span>
          <span className="text-ink-700 dark:text-ink-200">导出</span>
        </div>

        {/* Hero */}
        <div className="mb-8 animate-slide-up">
          <div className="text-xs font-medium uppercase tracking-wider text-brand-600 dark:text-brand-400 mb-1">最后一步</div>
          <div className="flex items-start justify-between gap-4">
            <div>
              <h1 className="text-2xl font-bold text-ink-900 dark:text-white tracking-tight">导出文档</h1>
              <p className="text-sm text-ink-500 dark:text-ink-400 mt-1">{bid?.project_name || '标书文档'}</p>
            </div>
            {bid && <StatusBadge status={bid.status} labels={BID_STATUS_LABELS} />}
          </div>

          {/* Workflow progress with stepper */}
          <div className="mt-5 p-4 rounded-2xl bg-white dark:bg-ink-800 border border-ink-100 dark:border-ink-700 shadow-soft">
            <div className="flex items-center justify-between mb-2">
              <div className="text-xs font-semibold text-ink-700 dark:text-ink-200">工作流进度</div>
              {bid && bid.total_chapters > 0 && (
                <div className="text-[11px] text-ink-500 dark:text-ink-400 tabular-nums">
                  已完成 <strong className="text-ink-700 dark:text-ink-200">{bid.done_chapters}</strong> / {bid.total_chapters} 章节
                </div>
              )}
            </div>
            <ProgressBar value={progress} showLabel tone={ready ? 'success' : bid?.status === 'failed' ? 'rose' : 'brand'} size="md" />
            {/* mini stepper */}
            <ol className="flex items-center justify-between mt-3 gap-1">
              {WORKFLOW_STEPS.map((step, i) => {
                const reached = i <= stepIdx
                const active = i === stepIdx
                return (
                  <li key={step.id} className="flex items-center gap-1.5 flex-1">
                    <span className={[
                      'inline-flex items-center justify-center w-4 h-4 rounded-full text-[9px] font-bold shrink-0',
                      active ? 'bg-brand-600 text-white shadow-pop' :
                      reached ? 'bg-emerald-500 text-white' :
                      'bg-ink-200 dark:bg-ink-700 text-ink-500 dark:text-ink-400',
                    ].join(' ')}>
                      {reached && !active ? '✓' : i + 1}
                    </span>
                    <span className={[
                      'text-[10px] truncate',
                      active ? 'text-brand-700 dark:text-brand-300 font-semibold' :
                      reached ? 'text-emerald-700 dark:text-emerald-400' :
                      'text-ink-400 dark:text-ink-500',
                    ].join(' ')}>{step.label}</span>
                  </li>
                )
              })}
            </ol>
          </div>
        </div>

        {/* Chapter preview — collapsed by default. Lets the user sanity-check
           what's about to be exported before committing. Critical for the
           "我在导出什么" question users always ask on first export. */}
        {chapters.length > 0 && (
          <Card padded={false} className="mb-6 animate-slide-up overflow-hidden">
            <button
              type="button"
              onClick={() => setShowPreview(!showPreview)}
              className="w-full flex items-center justify-between px-5 py-3 text-sm hover:bg-ink-50 dark:hover:bg-ink-800/50 transition-colors"
              aria-expanded={showPreview}
            >
              <span className="inline-flex items-center gap-2 font-medium text-ink-800 dark:text-ink-100">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
                  <polyline points="14 2 14 8 20 8" />
                  <line x1="8" y1="13" x2="16" y2="13" />
                  <line x1="8" y1="17" x2="14" y2="17" />
                </svg>
                章节目录预览
                <span className="text-xs text-ink-400 dark:text-ink-500 font-normal">({chapters.length} 章节)</span>
              </span>
              <span className="inline-flex items-center gap-2 text-xs text-ink-500 dark:text-ink-400">
                {showPreview ? '收起' : '展开'}
                <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round"
                  style={{ transform: showPreview ? 'rotate(180deg)' : 'rotate(0)', transition: 'transform 200ms' }}>
                  <polyline points="6 9 12 15 18 9" />
                </svg>
              </span>
            </button>
            {showPreview && (
              <div className="px-5 pb-4 pt-2 border-t border-ink-100 dark:border-ink-700 animate-slide-down">
                <ol className="space-y-1 max-h-72 overflow-y-auto scrollbar-thin">
                  {chapters.map((ch, i) => (
                    <li
                      key={ch.id}
                      className="flex items-center gap-3 text-xs py-1.5 px-2 rounded hover:bg-ink-50 dark:hover:bg-ink-800/50 transition-colors"
                      style={{ paddingLeft: `${Math.max(0, (ch.level - 1) * 16) + 8}px` }}
                    >
                      <span className="shrink-0 w-5 h-5 rounded bg-ink-100 dark:bg-ink-700 grid place-items-center text-[10px] font-mono text-ink-500 dark:text-ink-400 tabular-nums">
                        {i + 1}
                      </span>
                      <span className={[
                        'flex-1 truncate',
                        ch.level === 1 ? 'font-semibold text-ink-900 dark:text-white' : 'text-ink-700 dark:text-ink-200',
                      ].join(' ')}>
                        {ch.title}
                      </span>
                      <span className="shrink-0 inline-flex items-center gap-1.5 text-ink-500 dark:text-ink-400 tabular-nums">
                        <span>目标 {ch.target_word_count}</span>
                        {ch.priority === 'critical' && (
                          <span className="px-1.5 py-0.5 rounded bg-red-50 dark:bg-red-900/30 text-red-700 dark:text-red-400 text-[10px] font-medium">关键</span>
                        )}
                      </span>
                    </li>
                  ))}
                </ol>
                <div className="mt-3 pt-3 border-t border-ink-100 dark:border-ink-700 flex items-center justify-between text-[11px] text-ink-500 dark:text-ink-400">
                  <span>导出后可在 Word / PDF 中按此目录结构呈现</span>
                  <span className="tabular-nums">
                    总目标字数 <strong className="text-ink-700 dark:text-ink-200">{chapters.reduce((acc, c) => acc + c.target_word_count, 0).toLocaleString('zh-CN')}</strong>
                  </span>
                </div>
              </div>
            )}
          </Card>
        )}

        {/* Format cards */}
        <div className="grid md:grid-cols-2 gap-4 mb-6">
          {(['word', 'pdf'] as ExportFormat[]).map((fmt) => {
            const meta = FORMAT_META[fmt]
            const tone = meta.tone
            const disabled = !ready || exporting !== null
            const loading = exporting === fmt
            return (
              <Card
                key={fmt}
                className={[
                  'relative overflow-hidden animate-slide-up transition-all',
                  ready ? 'hover:shadow-card-hover hover:-translate-y-0.5 hover:border-brand-200' : '',
                ].join(' ')}
                style={{ animationDelay: `${fmt === 'word' ? 0 : 80}ms` }}
                tone={ready && fmt === 'word' ? 'brand' : 'default'}
              >
                {ready && fmt === 'word' && (
                  <div className="absolute top-3 right-3 px-2 py-0.5 rounded-full bg-brand-600 text-white text-[10px] font-semibold shadow-pop">
                    推荐
                  </div>
                )}
                <div className="flex items-start gap-4 mb-4">
                  <div className={[
                    'shrink-0 w-14 h-14 rounded-2xl flex items-center justify-center shadow-soft',
                    tone === 'blue' ? 'bg-brand-50 text-brand-600 dark:bg-brand-900/30 dark:text-brand-300' : 'bg-red-50 text-red-600 dark:bg-red-900/30 dark:text-red-300',
                  ].join(' ')}>
                    {meta.icon}
                  </div>
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2 mb-0.5">
                      <h3 className="text-base font-semibold text-ink-900 dark:text-white">{meta.label}</h3>
                      <span className={[
                        'text-[10px] font-mono font-semibold px-1.5 py-0.5 rounded',
                        tone === 'blue' ? 'bg-brand-50 text-brand-700 dark:bg-brand-900/30 dark:text-brand-300' : 'bg-red-50 text-red-700 dark:bg-red-900/30 dark:text-red-300',
                      ].join(' ')}>{meta.ext}</span>
                    </div>
                    <p className="text-xs text-ink-500 dark:text-ink-400">{meta.description}</p>
                  </div>
                </div>

                <ul className="space-y-1.5 mb-5">
                  {meta.highlights.map((h) => (
                    <li key={h} className="flex items-start gap-2 text-xs text-ink-600 dark:text-ink-300">
                      <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor"
                        strokeWidth="3" strokeLinecap="round" strokeLinejoin="round"
                        className={tone === 'blue' ? 'text-brand-500 mt-0.5 shrink-0' : 'text-red-500 mt-0.5 shrink-0'}>
                        <polyline points="20 6 9 17 4 12" />
                      </svg>
                      <span>{h}</span>
                    </li>
                  ))}
                </ul>

                <Button
                  variant={tone === 'blue' ? 'primary' : 'danger'}
                  size="lg"
                  className="w-full"
                  disabled={disabled}
                  loading={loading}
                  onClick={() => handleExport(fmt)}
                  leftIcon={!loading ? (
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
                      <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
                      <polyline points="7 10 12 15 17 10" />
                      <line x1="12" y1="15" x2="12" y2="3" />
                    </svg>
                  ) : undefined}
                >
                  下载 {meta.label}
                </Button>
              </Card>
            )
          })}
        </div>

        {/* Status messages */}
        {error && (
          <div className="flex items-start gap-2 px-4 py-3 bg-red-50 dark:bg-red-900/30 border border-red-100 dark:border-red-800 rounded-xl text-sm text-red-700 dark:text-red-300 animate-fade-in mb-4">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="shrink-0 mt-0.5">
              <circle cx="12" cy="12" r="10" />
              <line x1="12" y1="8" x2="12" y2="12" />
              <line x1="12" y1="16" x2="12.01" y2="16" />
            </svg>
            <span>{error}</span>
          </div>
        )}

        {!ready && (
          <Card className="bg-amber-50/50 dark:bg-amber-900/10 border-amber-100 dark:border-amber-900/30">
            <div className="flex items-start gap-3">
              <div className="shrink-0 w-9 h-9 rounded-lg bg-amber-100 dark:bg-amber-900/40 text-amber-700 dark:text-amber-300 flex items-center justify-center">
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <circle cx="12" cy="12" r="10" />
                  <polyline points="12 6 12 12 16 14" />
                </svg>
              </div>
              <div>
                <h4 className="font-semibold text-amber-900 dark:text-amber-200 text-sm">标书尚未就绪</h4>
                <p className="text-xs text-amber-800 dark:text-amber-300 mt-1">
                  请等待所有章节生成并通过审计。当前状态：
                  <strong>{BID_STATUS_LABELS[bid?.status ?? ''] ?? bid?.status}</strong>
                </p>
                <Link to={`/bids/${id}`}>
                  <Button size="sm" variant="secondary" className="mt-3">返回工作区</Button>
                </Link>
              </div>
            </div>
          </Card>
        )}

        {/* Learning Feedback - submit completed bid for pattern extraction */}
        {ready && (
          <LearningFeedback bidId={id!} chapters={chapters} />
        )}
      </div>
    </div>
  )
}

/** Learning feedback section: after export, submit the bid result for
 *  pattern extraction + Bandit update so future outlines improve. */
function LearningFeedback({ bidId, chapters }: { bidId: string; chapters: any[] }) {
  const [label, setLabel] = useState<'won' | 'lost' | 'draft'>('draft')
  const [submitted, setSubmitted] = useState(false)

  const learnMut = useMutation({
    mutationFn: async () => {
      // Fetch content for each chapter in parallel.
      const contents = await Promise.all(
        chapters.map(ch => bidsApi.getChapterContent(bidId, ch.id).then(r => r.data.data).catch(() => null))
      )
      const validContents = contents.filter(Boolean)
      if (validContents.length === 0) {
        throw new Error('无法获取章节内容')
      }
      return docgenApi.learn({
        chapters: validContents.map((c: any) => ({
          title: chapters.find(ch => ch.id === c.chapter_spec_id)?.title || '',
          content: c.content_text || '',
          word_count: c.word_count || 0,
        })),
        label,
      })
    },
    onSuccess: (res) => {
      const qs = res.data.quality_score?.toFixed(1) || '?'
      toast.success('学习反馈已提交', `质量评分: ${qs}`)
      setSubmitted(true)
    },
    onError: (e: any) => {
      toast.error('提交失败', e?.message || '请稍后重试')
    },
  })

  if (submitted) {
    return (
      <Card className="bg-green-50/50 dark:bg-green-900/10 border-green-100 dark:border-green-900/30 mt-6">
        <div className="flex items-center gap-3">
          <div className="shrink-0 w-9 h-9 rounded-lg bg-green-100 dark:bg-green-900/40 text-green-700 dark:text-green-300 flex items-center justify-center">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round"><polyline points="20 6 9 17 4 12" /></svg>
          </div>
          <div>
            <h4 className="font-semibold text-green-900 dark:text-green-200 text-sm">学习反馈已提交</h4>
            <p className="text-xs text-green-800 dark:text-green-300 mt-0.5">系统已从本次标书中提取模式，未来大纲生成将参考此经验</p>
          </div>
        </div>
      </Card>
    )
  }

  return (
    <Card className="mt-6">
      <div className="mb-4">
        <h3 className="text-sm font-semibold text-ink-800 dark:text-ink-100">学习反馈</h3>
        <p className="text-xs text-ink-500 dark:text-ink-400 mt-1">提交本次标书结果，帮助系统优化未来的大纲生成</p>
      </div>
      <div className="flex items-center gap-2 mb-4">
        {([
          { id: 'won', label: '中标', tone: 'green' },
          { id: 'lost', label: '落标', tone: 'red' },
          { id: 'draft', label: '草稿', tone: 'gray' },
        ] as const).map(opt => (
          <button
            key={opt.id}
            onClick={() => setLabel(opt.id)}
            className={[
              'px-3 py-1.5 rounded-lg text-xs font-medium transition-all',
              label === opt.id
                ? opt.tone === 'green' ? 'bg-green-500 text-white'
                  : opt.tone === 'red' ? 'bg-red-500 text-white'
                  : 'bg-ink-500 text-white'
                : 'bg-ink-100 dark:bg-ink-700 text-ink-600 dark:text-ink-300 hover:bg-ink-200 dark:hover:bg-ink-600',
            ].join(' ')}
          >
            {opt.label}
          </button>
        ))}
      </div>
      <Button
        variant="secondary"
        size="sm"
        loading={learnMut.isPending}
        onClick={() => learnMut.mutate()}
        leftIcon={!learnMut.isPending ? (
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" /><polyline points="17 8 12 3 7 8" /><line x1="12" y1="3" x2="12" y2="15" /></svg>
        ) : undefined}
      >
        提交学习反馈
      </Button>
    </Card>
  )
}

/** Save a Blob to disk via a transient anchor with the given filename. */
function triggerBrowserDownload(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  setTimeout(() => URL.revokeObjectURL(url), 0)
}
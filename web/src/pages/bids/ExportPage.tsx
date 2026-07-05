import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { bidsApi } from '../../api/bids'
import { toast } from '../../lib/toast'
import { Button, Card, ProgressBar, StatusBadge } from '../../components/ui'
import { BID_STATUS_LABELS } from './workspace-helpers'

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

  const bid = data?.data.data
  const ready = bid?.status === 'done'
  const progress = bid && bid.total_chapters > 0
    ? Math.round((bid.done_chapters / bid.total_chapters) * 100) : 0

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
        <div className="text-sm text-ink-400">加载中…</div>
      </div>
    )
  }

  return (
    <div className="flex-1 overflow-y-auto scrollbar-thin">
      <div className="max-w-4xl mx-auto px-6 py-8">
        {/* Breadcrumb */}
        <div className="flex items-center gap-2 text-xs text-ink-500 mb-4 animate-fade-in">
          <Link to="/bids" className="hover:text-ink-800">标书列表</Link>
          <span className="text-ink-300">/</span>
          <Link to={`/bids/${id}`} className="hover:text-ink-800 truncate max-w-xs">
            {bid?.project_name || '标书'}
          </Link>
          <span className="text-ink-300">/</span>
          <span className="text-ink-700">导出</span>
        </div>

        {/* Hero */}
        <div className="mb-8 animate-slide-up">
          <div className="text-xs font-medium uppercase tracking-wider text-brand-600 mb-1">最后一步</div>
          <div className="flex items-start justify-between gap-4">
            <div>
              <h1 className="text-2xl font-bold text-ink-900 tracking-tight">导出文档</h1>
              <p className="text-sm text-ink-500 mt-1">{bid?.project_name || '标书文档'}</p>
            </div>
            {bid && <StatusBadge status={bid.status} labels={BID_STATUS_LABELS} />}
          </div>
          {bid && bid.total_chapters > 0 && (
            <div className="mt-4 max-w-md">
              <ProgressBar value={progress} showLabel size="md" tone={ready ? 'success' : 'brand'} />
              <p className="text-xs text-ink-500 mt-1.5">
                已完成 <strong className="text-ink-700 tabular-nums">{bid.done_chapters}</strong> / {bid.total_chapters} 个章节
              </p>
            </div>
          )}
        </div>

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
                className="relative overflow-hidden animate-slide-up"
                style={{ animationDelay: `${fmt === 'word' ? 0 : 80}ms` }}
              >
                <div className="flex items-start gap-4 mb-4">
                  <div className={[
                    'shrink-0 w-14 h-14 rounded-2xl flex items-center justify-center',
                    tone === 'blue' ? 'bg-brand-50 text-brand-600' : 'bg-red-50 text-red-600',
                  ].join(' ')}>
                    {meta.icon}
                  </div>
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2 mb-0.5">
                      <h3 className="text-base font-semibold text-ink-900">{meta.label}</h3>
                      <span className={[
                        'text-[10px] font-mono font-semibold px-1.5 py-0.5 rounded',
                        tone === 'blue' ? 'bg-brand-50 text-brand-700' : 'bg-red-50 text-red-700',
                      ].join(' ')}>{meta.ext}</span>
                    </div>
                    <p className="text-xs text-ink-500">{meta.description}</p>
                  </div>
                </div>

                <ul className="space-y-1.5 mb-5">
                  {meta.highlights.map((h) => (
                    <li key={h} className="flex items-start gap-2 text-xs text-ink-600">
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
          <div className="flex items-start gap-2 px-4 py-3 bg-red-50 border border-red-100 rounded-xl text-sm text-red-700 animate-fade-in">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="shrink-0 mt-0.5">
              <circle cx="12" cy="12" r="10" />
              <line x1="12" y1="8" x2="12" y2="12" />
              <line x1="12" y1="16" x2="12.01" y2="16" />
            </svg>
            <span>{error}</span>
          </div>
        )}

        {!ready && (
          <Card className="bg-amber-50/50 border-amber-100">
            <div className="flex items-start gap-3">
              <div className="shrink-0 w-9 h-9 rounded-lg bg-amber-100 text-amber-700 flex items-center justify-center">
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <circle cx="12" cy="12" r="10" />
                  <polyline points="12 6 12 12 16 14" />
                </svg>
              </div>
              <div>
                <h4 className="font-semibold text-amber-900 text-sm">标书尚未就绪</h4>
                <p className="text-xs text-amber-800 mt-1">
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
      </div>
    </div>
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
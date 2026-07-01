import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { bidsApi } from '../../api/bids'

type ExportFormat = 'word' | 'pdf'

export default function ExportPage() {
  const { id } = useParams<{ id: string }>()

  const { data, isLoading } = useQuery({
    queryKey: ['bid', id],
    queryFn: () => bidsApi.get(id!),
    enabled: !!id,
  })

  const bid = data?.data.data
  const ready = bid?.status === 'done'

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
    } catch (e) {
      // 401 is handled by the axios interceptor (logout + redirect)
      const message = e instanceof Error ? e.message : '导出失败'
      setError(message)
    } finally {
      setExporting(null)
    }
  }

  if (isLoading) return <div className="p-6">加载中...</div>

  return (
    <div className="p-6">
      <div className="flex justify-between items-center mb-6">
        <h1 className="text-2xl font-bold">导出文档</h1>
        <Link to={`/bids/${id}`} className="px-4 py-2 border rounded-lg hover:bg-gray-50">
          返回
        </Link>
      </div>

      <div className="max-w-2xl">
        <div className="bg-white rounded-xl shadow-sm p-6 mb-6">
          <h2 className="text-lg font-medium mb-4">{bid?.project_name || '标书文档'}</h2>
          <p className="text-sm text-gray-500 mb-6">
            状态: {ready ? '已完成' : '未完成'}
          </p>

          <div className="space-y-4">
            <ExportRow
              icon="📄"
              tone="blue"
              title="Word 文档 (.docx)"
              description="适用于编辑和打印"
              disabled={!ready || exporting !== null}
              loading={exporting === 'word'}
              onClick={() => handleExport('word')}
            />
            <ExportRow
              icon="📕"
              tone="red"
              title="PDF 文档"
              description="适用于正式提交和存档（MVP 暂复用 Word 格式）"
              disabled={!ready || exporting !== null}
              loading={exporting === 'pdf'}
              onClick={() => handleExport('pdf')}
            />
          </div>

          {error && (
            <div className="mt-4 p-3 bg-red-50 border border-red-200 rounded-lg text-sm text-red-700">
              {error}
            </div>
          )}
        </div>

        {!ready && (
          <div className="bg-yellow-50 border border-yellow-200 rounded-lg p-4">
            <p className="text-sm text-yellow-800">
              标书尚未生成完成，请等待所有章节生成并通过审计后再导出。
            </p>
          </div>
        )}

        <div className="bg-gray-50 rounded-lg p-4 mt-6">
          <h3 className="font-medium mb-2">导出说明</h3>
          <ul className="text-sm text-gray-600 space-y-1">
            <li>• Word 文档支持后续编辑，可添加签名、盖章等</li>
            <li>• PDF 文档为只读格式，适合正式提交</li>
            <li>• 图表将作为图片嵌入文档中</li>
            <li>• 如需修改样式，可导出后使用 Word 编辑</li>
          </ul>
        </div>
      </div>
    </div>
  )
}

function ExportRow({
  icon,
  tone,
  title,
  description,
  disabled,
  loading,
  onClick,
}: {
  icon: string
  tone: 'blue' | 'red'
  title: string
  description: string
  disabled: boolean
  loading: boolean
  onClick: () => void
}) {
  const toneClass =
    tone === 'blue'
      ? 'bg-blue-600 hover:bg-blue-700'
      : 'bg-red-600 hover:bg-red-700'
  const iconBg = tone === 'blue' ? 'bg-blue-100' : 'bg-red-100'

  return (
    <div className="flex items-center justify-between p-4 border rounded-lg hover:bg-gray-50">
      <div className="flex items-center gap-4">
        <div className={`w-12 h-12 ${iconBg} rounded-lg flex items-center justify-center`}>
          <span className="text-2xl">{icon}</span>
        </div>
        <div>
          <h3 className="font-medium">{title}</h3>
          <p className="text-sm text-gray-500">{description}</p>
        </div>
      </div>
      <button
        onClick={onClick}
        disabled={disabled}
        className={`px-4 py-2 text-white rounded-lg disabled:opacity-50 disabled:cursor-not-allowed ${toneClass}`}
      >
        {loading ? '导出中…' : '下载'}
      </button>
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
  // Revoke after the click is processed to free memory.
  setTimeout(() => URL.revokeObjectURL(url), 0)
}

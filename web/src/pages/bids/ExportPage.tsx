import { useParams, Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { bidsApi } from '../../api/bids'

export default function ExportPage() {
  const { id } = useParams<{ id: string }>()

  const { data, isLoading } = useQuery({
    queryKey: ['bid', id],
    queryFn: () => bidsApi.get(id!),
    enabled: !!id,
  })

  const bid = data?.data.data

  const handleExportWord = () => {
    window.open(`/api/v1/bids/${id}/export/word`, '_blank')
  }

  const handleExportPdf = () => {
    window.open(`/api/v1/bids/${id}/export/pdf`, '_blank')
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
            状态: {bid?.status === 'done' ? '已完成' : '未完成'}
          </p>

          <div className="space-y-4">
            <div className="flex items-center justify-between p-4 border rounded-lg hover:bg-gray-50">
              <div className="flex items-center gap-4">
                <div className="w-12 h-12 bg-blue-100 rounded-lg flex items-center justify-center">
                  <span className="text-2xl">📄</span>
                </div>
                <div>
                  <h3 className="font-medium">Word 文档 (.docx)</h3>
                  <p className="text-sm text-gray-500">适用于编辑和打印</p>
                </div>
              </div>
              <button
                onClick={handleExportWord}
                disabled={bid?.status !== 'done'}
                className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
              >
                下载
              </button>
            </div>

            <div className="flex items-center justify-between p-4 border rounded-lg hover:bg-gray-50">
              <div className="flex items-center gap-4">
                <div className="w-12 h-12 bg-red-100 rounded-lg flex items-center justify-center">
                  <span className="text-2xl">📕</span>
                </div>
                <div>
                  <h3 className="font-medium">PDF 文档</h3>
                  <p className="text-sm text-gray-500">适用于正式提交和存档</p>
                </div>
              </div>
              <button
                onClick={handleExportPdf}
                disabled={bid?.status !== 'done'}
                className="px-4 py-2 bg-red-600 text-white rounded-lg hover:bg-red-700 disabled:opacity-50 disabled:cursor-not-allowed"
              >
                下载
              </button>
            </div>
          </div>
        </div>

        {bid?.status !== 'done' && (
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
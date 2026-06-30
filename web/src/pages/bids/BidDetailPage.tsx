import { useParams, Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { bidsApi } from '../../api/bids'

export default function BidDetailPage() {
  const { id } = useParams<{ id: string }>()
  const { data, isLoading } = useQuery({
    queryKey: ['bid', id],
    queryFn: () => bidsApi.get(id!),
    enabled: !!id,
  })

  const bid = data?.data.data

  const statusLabels: Record<string, string> = {
    pending: '等待中',
    parsing: '解析中',
    outlining: '生成大纲',
    generating: '生成内容',
    auditing: '审计中',
    exporting: '导出中',
    done: '已完成',
    failed: '失败',
    paused: '已暂停',
  }

  if (isLoading) {
    return <div className="p-6">加载中...</div>
  }

  if (!bid) {
    return <div className="p-6">标书不存在</div>
  }

  const progress =
    bid.total_chapters > 0
      ? Math.round((bid.done_chapters / bid.total_chapters) * 100)
      : 0

  return (
    <div className="p-6">
      <div className="flex justify-between items-center mb-6">
        <div>
          <h1 className="text-2xl font-bold">{bid.project_name || '未命名项目'}</h1>
          <p className="text-gray-500 text-sm mt-1">
            行业: {bid.industry || '-'} · 状态: {statusLabels[bid.status] || bid.status}
          </p>
        </div>
        <div className="flex gap-2">
          {bid.status === 'paused' && (
            <button className="px-4 py-2 bg-green-600 text-white rounded-lg hover:bg-green-700">
              恢复
            </button>
          )}
          {['pending', 'outlining', 'generating'].includes(bid.status) && (
            <button className="px-4 py-2 bg-yellow-600 text-white rounded-lg hover:bg-yellow-700">
              暂停
            </button>
          )}
          {bid.status === 'done' && (
            <Link
              to={`/bids/${id}/export`}
              className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700"
            >
              导出 Word
            </Link>
          )}
        </div>
      </div>

      {/* Progress bar */}
      <div className="bg-white rounded-xl shadow-sm p-6 mb-6">
        <div className="flex justify-between text-sm mb-2">
          <span>生成进度</span>
          <span>{progress}%</span>
        </div>
        <div className="w-full bg-gray-200 rounded-full h-2">
          <div
            className="bg-blue-600 h-2 rounded-full transition-all"
            style={{ width: `${progress}%` }}
          />
        </div>
        <p className="text-sm text-gray-500 mt-2">
          {bid.done_chapters}/{bid.total_chapters} 章节已完成
        </p>
      </div>

      {/* Quick actions */}
      <div className="grid grid-cols-4 gap-4">
        <Link
          to={`/bids/${id}/outline`}
          className="bg-white p-4 rounded-xl shadow-sm hover:shadow-md transition"
        >
          <h3 className="font-medium">章节大纲</h3>
          <p className="text-sm text-gray-500 mt-1">查看/调整章节结构</p>
        </Link>
        <Link
          to={`/bids/${id}/chapters`}
          className="bg-white p-4 rounded-xl shadow-sm hover:shadow-md transition"
        >
          <h3 className="font-medium">章节内容</h3>
          <p className="text-sm text-gray-500 mt-1">查看/编辑章节正文</p>
        </Link>
        <Link
          to={`/bids/${id}/audit`}
          className="bg-white p-4 rounded-xl shadow-sm hover:shadow-md transition"
        >
          <h3 className="font-medium">审计问题</h3>
          <p className="text-sm text-gray-500 mt-1">处理合规问题</p>
        </Link>
        <Link
          to={`/bids/${id}/export`}
          className="bg-white p-4 rounded-xl shadow-sm hover:shadow-md transition"
        >
          <h3 className="font-medium">导出文档</h3>
          <p className="text-sm text-gray-500 mt-1">下载 Word/PDF</p>
        </Link>
      </div>
    </div>
  )
}
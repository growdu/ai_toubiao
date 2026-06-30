import { useParams, Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { bidsApi } from '../../api/bids'

export default function OutlinePage() {
  const { id } = useParams<{ id: string }>()
  const queryClient = useQueryClient()

  const { data, isLoading } = useQuery({
    queryKey: ['outline', id],
    queryFn: () => bidsApi.getOutline(id!),
    enabled: !!id,
  })

  const confirmMutation = useMutation({
    mutationFn: () => bidsApi.updateOutline(id!, data?.data.data || []),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['bid', id] })
    },
  })

  if (isLoading) return <div className="p-6">加载中...</div>

  const chapters = data?.data.data || []

  const statusLabels: Record<string, string> = {
    planned: '待生成',
    pending: '等待中',
    running: '生成中',
    succeeded: '已生成',
    failed: '失败',
    skipped: '跳过',
  }

  return (
    <div className="p-6">
      <div className="flex justify-between items-center mb-6">
        <div>
          <h1 className="text-2xl font-bold">章节大纲</h1>
          <p className="text-gray-500 text-sm mt-1">
            共 {chapters.length} 个章节，确认后可开始生成内容
          </p>
        </div>
        <div className="flex gap-2">
          <Link
            to={`/bids/${id}`}
            className="px-4 py-2 border rounded-lg hover:bg-gray-50"
          >
            返回
          </Link>
          <button
            onClick={() => confirmMutation.mutate()}
            disabled={confirmMutation.isPending}
            className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50"
          >
            {confirmMutation.isPending ? '确认中...' : '确认大纲并生成'}
          </button>
        </div>
      </div>

      <div className="bg-white rounded-xl shadow-sm divide-y divide-gray-200">
        {chapters.map((chapter, index) => (
          <div key={chapter.id} className="p-4 flex items-start gap-4">
            <div className="flex-shrink-0 w-8 h-8 rounded-full bg-gray-100 flex items-center justify-center text-sm font-medium">
              {index + 1}
            </div>
            <div className="flex-1">
              <div className="flex items-center gap-2">
                <span className="font-medium">
                  {'　'.repeat(chapter.level - 1)}
                  {chapter.title}
                </span>
                <span className="text-xs px-2 py-0.5 bg-gray-100 rounded">
                  {chapter.chapter_type}
                </span>
                {chapter.priority === 'critical' && (
                  <span className="text-xs px-2 py-0.5 bg-red-100 text-red-700 rounded">
                    关键
                  </span>
                )}
              </div>
              <p className="text-sm text-gray-500 mt-1">
                目标字数: {chapter.target_word_count} · 最少: {chapter.min_word_count}
              </p>
            </div>
            <div className="flex-shrink-0">
              <span
                className={`inline-flex px-2 py-1 text-xs font-medium rounded-full ${
                  chapter.status === 'succeeded'
                    ? 'bg-green-100 text-green-700'
                    : chapter.status === 'failed'
                    ? 'bg-red-100 text-red-700'
                    : 'bg-gray-100 text-gray-700'
                }`}
              >
                {statusLabels[chapter.status] || chapter.status}
              </span>
            </div>
          </div>
        ))}
      </div>

      {chapters.length === 0 && (
        <div className="text-center py-12 text-gray-500">
          暂无章节大纲，请先上传 RFP 并等待解析完成
        </div>
      )}
    </div>
  )
}
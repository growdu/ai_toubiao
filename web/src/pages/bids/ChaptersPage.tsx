import { useParams, Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { bidsApi } from '../../api/bids'

export default function ChaptersPage() {
  const { id } = useParams<{ id: string }>()

  const { data } = useQuery({
    queryKey: ['outline', id],
    queryFn: () => bidsApi.getOutline(id!),
    enabled: !!id,
  })

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
        <h1 className="text-2xl font-bold">章节内容</h1>
        <Link to={`/bids/${id}`} className="px-4 py-2 border rounded-lg hover:bg-gray-50">
          返回
        </Link>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        {chapters.map((chapter) => (
          <Link
            key={chapter.id}
            to={`/bids/${id}/chapters/${chapter.id}`}
            className="bg-white p-4 rounded-xl shadow-sm hover:shadow-md transition"
          >
            <div className="flex items-start justify-between">
              <div>
                <h3 className="font-medium">{chapter.title}</h3>
                <p className="text-sm text-gray-500 mt-1">
                  目标: {chapter.target_word_count} 字
                </p>
              </div>
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
          </Link>
        ))}
      </div>

      {chapters.length === 0 && (
        <div className="text-center py-12 text-gray-500">
          暂无章节，请先确认大纲
        </div>
      )}
    </div>
  )
}
import { Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { bidsApi } from '../../api/bids'

export default function BidsPage() {
  const { data, isLoading } = useQuery({
    queryKey: ['bids'],
    queryFn: () => bidsApi.list(),
  })

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

  return (
    <div className="p-6">
      <div className="flex justify-between items-center mb-6">
        <h1 className="text-2xl font-bold">标书管理</h1>
        <Link
          to="/bids/new"
          className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700"
        >
          新建标书
        </Link>
      </div>

      {isLoading ? (
        <div className="text-center py-8 text-gray-500">加载中...</div>
      ) : data?.data.data.length === 0 ? (
        <div className="text-center py-8 text-gray-500">暂无标书，点击新建开始</div>
      ) : (
        <div className="bg-white rounded-xl shadow-sm overflow-hidden">
          <table className="w-full">
            <thead className="bg-gray-50">
              <tr>
                <th className="px-4 py-3 text-left text-sm font-medium text-gray-500">
                  项目名称
                </th>
                <th className="px-4 py-3 text-left text-sm font-medium text-gray-500">
                  状态
                </th>
                <th className="px-4 py-3 text-left text-sm font-medium text-gray-500">
                  进度
                </th>
                <th className="px-4 py-3 text-left text-sm font-medium text-gray-500">
                  创建时间
                </th>
                <th className="px-4 py-3 text-left text-sm font-medium text-gray-500">
                  操作
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-200">
              {data?.data.data.map((bid) => (
                <tr key={bid.id} className="hover:bg-gray-50">
                  <td className="px-4 py-3">
                    <Link
                      to={`/bids/${bid.id}`}
                      className="text-blue-600 hover:underline"
                    >
                      {bid.project_name || '未命名项目'}
                    </Link>
                  </td>
                  <td className="px-4 py-3">
                    <span
                      className={`inline-flex px-2 py-1 text-xs font-medium rounded-full ${
                        bid.status === 'done'
                          ? 'bg-green-100 text-green-700'
                          : bid.status === 'failed'
                          ? 'bg-red-100 text-red-700'
                          : 'bg-blue-100 text-blue-700'
                      }`}
                    >
                      {statusLabels[bid.status] || bid.status}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-sm text-gray-500">
                    {bid.done_chapters}/{bid.total_chapters} 章节
                  </td>
                  <td className="px-4 py-3 text-sm text-gray-500">
                    {new Date(bid.created_at).toLocaleDateString()}
                  </td>
                  <td className="px-4 py-3">
                    <Link
                      to={`/bids/${bid.id}`}
                      className="text-blue-600 hover:underline text-sm"
                    >
                      查看
                    </Link>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
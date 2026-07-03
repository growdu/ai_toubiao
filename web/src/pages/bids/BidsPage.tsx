import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { bidsApi, BidJob } from '../../api/bids'

const statusLabels: Record<string, string> = {
  pending: '等待中', parsing: '解析中', outlining: '生成大纲',
  generating: '生成内容', auditing: '审计中', exporting: '导出中',
  done: '已完成', failed: '失败', paused: '已暂停', facts: '审查中',
}

const statusColors: Record<string, string> = {
  done: 'bg-green-100 text-green-700', failed: 'bg-red-100 text-red-700',
  paused: 'bg-yellow-100 text-yellow-700', facts: 'bg-purple-100 text-purple-700',
}

export default function BidsPage() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [showCreate, setShowCreate] = useState(false)
  const [projectName, setProjectName] = useState('')

  const { data, isLoading } = useQuery({
    queryKey: ['bids'],
    queryFn: () => bidsApi.list(),
  })

  const bids: BidJob[] = data?.data.data || []

  const createMutation = useMutation({
    mutationFn: async (name: string) => {
      // 1. Create project via API
      const { default: apiClient } = await import('../../api/client')
      const projRes = await apiClient.post('/projects', { name })
      // 2. Create workflow (bid)
      return bidsApi.create({ project_id: projRes.data.data.id })
    },
    onSuccess: (res) => {
      queryClient.invalidateQueries({ queryKey: ['bids'] })
      setShowCreate(false)
      setProjectName('')
      navigate(`/bids/${res.data.data.id}`)
    },
  })


  return (
    <div className="p-6">
      <div className="flex justify-between items-center mb-6">
        <h1 className="text-2xl font-bold">标书管理</h1>
        <button
          onClick={() => setShowCreate(true)}
          className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700"
        >
          + 新建标书
        </button>
      </div>

      {showCreate && (
        <div className="fixed inset-0 bg-black/30 flex items-center justify-center z-50">
          <div className="bg-white rounded-xl shadow-lg p-6 w-96">
            <h2 className="text-lg font-bold mb-4">新建标书</h2>
            <input
              type="text"
              value={projectName}
              onChange={e => setProjectName(e.target.value)}
              onKeyDown={e => e.key === 'Enter' && projectName && createMutation.mutate(projectName)}
              placeholder="输入项目名称"
              className="w-full px-3 py-2 border rounded-lg mb-4 focus:outline-none focus:ring-2 focus:ring-blue-500"
              autoFocus
            />
            <div className="flex gap-2 justify-end">
              <button
                onClick={() => { setShowCreate(false); setProjectName('') }}
                className="px-4 py-2 text-sm border rounded-lg hover:bg-gray-50"
              >取消</button>
              <button
                onClick={() => projectName && createMutation.mutate(projectName)}
                disabled={!projectName || createMutation.isPending}
                className="px-4 py-2 text-sm bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50"
              >
                {createMutation.isPending ? '创建中...' : '创建'}
              </button>
            </div>
          </div>
        </div>
      )}

      {isLoading ? (
        <div className="text-center py-8 text-gray-500">加载中...</div>
      ) : bids.length === 0 ? (
        <div className="text-center py-12 text-gray-400">
          <p className="text-lg mb-2">暂无标书</p>
          <p className="text-sm">点击"新建标书"开始创建</p>
        </div>
      ) : (
        <div className="grid gap-3">
          {bids.map((bid) => {
            const progress = bid.total_chapters > 0
              ? Math.round((bid.done_chapters / bid.total_chapters) * 100)
              : 0
            return (
              <div
                key={bid.id}
                onClick={() => navigate(`/bids/${bid.id}`)}
                className="bg-white rounded-xl shadow-sm p-4 hover:shadow-md cursor-pointer transition"
              >
                <div className="flex items-center justify-between mb-2">
                  <h3 className="font-medium text-gray-800">
                    {bid.project_name || '未命名项目'}
                  </h3>
                  <span className={`text-xs px-2 py-0.5 rounded-full ${statusColors[bid.status] || 'bg-blue-100 text-blue-700'}`}>
                    {statusLabels[bid.status] || bid.status}
                  </span>
                </div>
                <div className="flex items-center gap-4 text-sm text-gray-400">
                  <span>{bid.done_chapters}/{bid.total_chapters} 章节</span>
                  <span>{progress}%</span>
                  <span>{new Date(bid.created_at).toLocaleDateString()}</span>
                </div>
                {bid.total_chapters > 0 && (
                  <div className="mt-2 w-full bg-gray-100 rounded-full h-1.5">
                    <div
                      className="bg-blue-600 h-1.5 rounded-full transition-all"
                      style={{ width: `${progress}%` }}
                    />
                  </div>
                )}
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}

import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import api from '../../api/client'

interface KBMaterial {
  id: string
  title: string
  category: string
  status: string
  chunk_count: number
  created_at: string
}

interface CreateMaterialRequest {
  category: string
  title: string
  content?: string
}

export default function KnowledgePage() {
  const queryClient = useQueryClient()
  const [showUpload, setShowUpload] = useState(false)
  const [form, setForm] = useState<CreateMaterialRequest>({ category: 'other', title: '' })

  const { data, isLoading } = useQuery({
    queryKey: ['kb-materials'],
    queryFn: () => api.get<{ data: KBMaterial[] }>('/kb/materials'),
  })

  const createMutation = useMutation({
    mutationFn: (data: CreateMaterialRequest) => api.post('/kb/materials', data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['kb-materials'] })
      setShowUpload(false)
      setForm({ category: 'other', title: '' })
    },
  })

  const categoryLabels: Record<string, string> = {
    certificate: '资质证书',
    case: '项目案例',
    patent: '专利',
    team: '团队成员',
    equipment: '设备',
    qualification: '资格认证',
    other: '其他',
  }

  return (
    <div className="p-6">
      <div className="flex justify-between items-center mb-6">
        <h1 className="text-2xl font-bold">知识库</h1>
        <button
          onClick={() => setShowUpload(true)}
          className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700"
        >
          上传素材
        </button>
      </div>

      {/* Upload modal */}
      {showUpload && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-white rounded-xl p-6 w-full max-w-md">
            <h2 className="text-lg font-bold mb-4">上传素材</h2>
            <form
              onSubmit={(e) => {
                e.preventDefault()
                createMutation.mutate(form)
              }}
              className="space-y-4"
            >
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">标题</label>
                <input
                  type="text"
                  value={form.title}
                  onChange={(e) => setForm({ ...form, title: e.target.value })}
                  className="w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
                  required
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">分类</label>
                <select
                  value={form.category}
                  onChange={(e) => setForm({ ...form, category: e.target.value })}
                  className="w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
                >
                  <option value="certificate">资质证书</option>
                  <option value="case">项目案例</option>
                  <option value="patent">专利</option>
                  <option value="team">团队成员</option>
                  <option value="equipment">设备</option>
                  <option value="qualification">资格认证</option>
                  <option value="other">其他</option>
                </select>
              </div>
              <div className="flex gap-2 justify-end">
                <button
                  type="button"
                  onClick={() => setShowUpload(false)}
                  className="px-4 py-2 border rounded-lg hover:bg-gray-50"
                >
                  取消
                </button>
                <button
                  type="submit"
                  disabled={createMutation.isPending}
                  className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50"
                >
                  {createMutation.isPending ? '上传中...' : '上传'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Material list */}
      {isLoading ? (
        <div className="text-center py-8 text-gray-500">加载中...</div>
      ) : data?.data.data.length === 0 ? (
        <div className="text-center py-8 text-gray-500">暂无素材，点击上传开始</div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {data?.data.data.map((material) => (
            <div key={material.id} className="bg-white p-4 rounded-xl shadow-sm">
              <div className="flex items-start justify-between">
                <div>
                  <h3 className="font-medium">{material.title}</h3>
                  <p className="text-sm text-gray-500 mt-1">
                    {categoryLabels[material.category] || material.category}
                  </p>
                  <p className="text-xs text-gray-400 mt-1">
                    {material.chunk_count} 个片段 · {new Date(material.created_at).toLocaleDateString()}
                  </p>
                </div>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
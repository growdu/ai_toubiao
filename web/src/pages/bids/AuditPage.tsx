import { useParams, Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { bidsApi } from '../../api/bids'

export default function AuditPage() {
  const { id } = useParams<{ id: string }>()

  const { data, isLoading } = useQuery({
    queryKey: ['audit-report', id],
    queryFn: () => bidsApi.getAuditReport(id!),
    enabled: !!id,
  })

  const report = data?.data.data

  if (isLoading) return <div className="p-6">加载中...</div>

  const severityColors: Record<string, string> = {
    critical: 'bg-red-50 border-red-200',
    major: 'bg-orange-50 border-orange-200',
    minor: 'bg-yellow-50 border-yellow-200',
  }

  return (
    <div className="p-6">
      <div className="flex justify-between items-center mb-6">
        <div>
          <h1 className="text-2xl font-bold">审计问题</h1>
          <p className="text-gray-500 text-sm mt-1">
            共 {report?.total_issues || 0} 个问题 · 严重 {report?.critical || 0} · 主要 {report?.major || 0} · 次要 {report?.minor || 0}
          </p>
        </div>
        <div className="flex gap-2">
          <Link to={`/bids/${id}`} className="px-4 py-2 border rounded-lg hover:bg-gray-50">
            返回
          </Link>
          <button
            disabled={!report?.issues?.length}
            className="px-4 py-2 bg-green-600 text-white rounded-lg hover:bg-green-700 disabled:opacity-50"
          >
            通过审计
          </button>
        </div>
      </div>

      <div className="space-y-4">
        {report?.issues?.map((issue) => (
          <div
            key={issue.id}
            className={`p-4 rounded-xl border ${severityColors[issue.severity] || 'bg-gray-50 border-gray-200'}`}
          >
            <div className="flex items-start justify-between">
              <div>
                <div className="flex items-center gap-2">
                  <span className="font-medium">{issue.chapter_title}</span>
                  <span className={`text-xs px-2 py-0.5 rounded ${
                    issue.severity === 'critical' ? 'bg-red-200 text-red-800' :
                    issue.severity === 'major' ? 'bg-orange-200 text-orange-800' :
                    'bg-yellow-200 text-yellow-800'
                  }`}>
                    {issue.severity === 'critical' ? '严重' : issue.severity === 'major' ? '主要' : '次要'}
                  </span>
                  <span className="text-xs px-2 py-0.5 bg-gray-200 text-gray-700 rounded">
                    {issue.dimension}
                  </span>
                </div>
                <p className="text-sm mt-2">{issue.issue}</p>
                <p className="text-sm text-blue-600 mt-1">建议: {issue.suggestion}</p>
              </div>
              <div className="flex gap-2">
                <button className="px-3 py-1 text-sm border rounded hover:bg-white">驳回</button>
                <button className="px-3 py-1 text-sm bg-blue-600 text-white rounded hover:bg-blue-700">确认</button>
              </div>
            </div>
          </div>
        ))}
      </div>

      {!report?.issues?.length && (
        <div className="text-center py-12 text-gray-500">
          暂无审计问题
        </div>
      )}
    </div>
  )
}
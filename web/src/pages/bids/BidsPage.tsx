import { useState, useMemo } from 'react'
import { useNavigate } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { bidsApi, BidJob } from '../../api/bids'
import apiClient from '../../api/client'
import { toast } from '../../lib/toast'
import {
  Button, Card, EmptyState, Modal, ProgressBar, StatCard,
  TextInput, StatusBadge, SkeletonCard,
} from '../../components/ui'

const statusLabels: Record<string, string> = {
  pending: '等待中', parsing: '解析中', outlining: '生成大纲',
  generating: '生成内容', auditing: '审计中', exporting: '导出中',
  done: '已完成', failed: '失败', paused: '已暂停', facts: '审查中',
}

type StatusFilter = 'all' | 'active' | 'done' | 'failed'

export default function BidsPage() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [showCreate, setShowCreate] = useState(false)
  const [projectName, setProjectName] = useState('')
  const [search, setSearch] = useState('')
  const [statusFilter, setStatusFilter] = useState<StatusFilter>('all')

  const { data, isLoading } = useQuery({
    queryKey: ['bids'],
    queryFn: () => bidsApi.list(),
  })

  const bids: BidJob[] = data?.data.data || []

  const createMutation = useMutation({
    mutationFn: async (name: string) => {
      const projRes = await apiClient.post('/projects', { name })
      return bidsApi.create({ project_id: projRes.data.data.id })
    },
    onSuccess: (res) => {
      queryClient.invalidateQueries({ queryKey: ['bids'] })
      setShowCreate(false)
      setProjectName('')
      toast.success('标书已创建', '正在跳转到工作区…')
      navigate(`/bids/${res.data.data.id}`)
    },
    onError: (err: any) => {
      toast.error('创建失败', err?.response?.data?.message || '请稍后重试')
    },
  })

  const filtered = useMemo(() => {
    let list = bids
    if (search.trim()) {
      const q = search.trim().toLowerCase()
      list = list.filter(b => (b.project_name || '').toLowerCase().includes(q))
    }
    if (statusFilter !== 'all') {
      list = list.filter(b => {
        if (statusFilter === 'active') {
          return ['pending', 'parsing', 'outlining', 'generating', 'auditing', 'exporting', 'facts', 'paused'].includes(b.status)
        }
        return b.status === statusFilter
      })
    }
    return list
  }, [bids, search, statusFilter])

  const stats = useMemo(() => {
    const total = bids.length
    const active = bids.filter(b => ['pending', 'parsing', 'outlining', 'generating', 'auditing', 'exporting', 'facts', 'paused'].includes(b.status)).length
    const done = bids.filter(b => b.status === 'done').length
    const failed = bids.filter(b => b.status === 'failed').length
    return { total, active, done, failed }
  }, [bids])

  return (
    <div className="flex-1 overflow-y-auto scrollbar-thin">
      <div className="max-w-6xl mx-auto px-6 py-8">
        {/* Header */}
        <div className="flex items-end justify-between mb-6 gap-4 animate-fade-in">
          <div>
            <div className="text-xs font-medium uppercase tracking-wider text-brand-600 mb-1">工作台</div>
            <h1 className="text-2xl font-bold text-ink-900 tracking-tight">标书管理</h1>
            <p className="text-sm text-ink-500 mt-1">创建新标书，跟踪每一份标书的编制进度</p>
          </div>
          <Button variant="primary" size="md" onClick={() => setShowCreate(true)} leftIcon={
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round">
              <line x1="12" y1="5" x2="12" y2="19" />
              <line x1="5" y1="12" x2="19" y2="12" />
            </svg>
          }>新建标书</Button>
        </div>

        {/* Stats */}
        <div className="grid grid-cols-2 md:grid-cols-4 gap-3 mb-6 animate-slide-up">
          <StatCard
            tone="blue"
            label="标书总数"
            value={stats.total}
            icon={<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" /><polyline points="14 2 14 8 20 8" /></svg>}
          />
          <StatCard
            tone="purple"
            label="进行中"
            value={stats.active}
            icon={<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><polygon points="5 3 19 12 5 21 5 3" /></svg>}
          />
          <StatCard
            tone="green"
            label="已完成"
            value={stats.done}
            icon={<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><polyline points="20 6 9 17 4 12" /></svg>}
          />
          <StatCard
            tone="amber"
            label="失败"
            value={stats.failed}
            icon={<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><circle cx="12" cy="12" r="10" /><line x1="12" y1="8" x2="12" y2="12" /><line x1="12" y1="16" x2="12.01" y2="16" /></svg>}
          />
        </div>

        {/* Filters */}
        <div className="flex items-center gap-3 mb-5">
          <div className="flex-1 max-w-xs">
            <TextInput
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="搜索标书名称…"
            />
          </div>
          <div className="flex items-center gap-1 p-1 bg-white rounded-xl border border-ink-100 shadow-soft">
            {(['all', 'active', 'done', 'failed'] as StatusFilter[]).map((s) => (
              <button
                key={s}
                onClick={() => setStatusFilter(s)}
                className={[
                  'px-3 py-1.5 text-xs font-medium rounded-lg transition-colors',
                  statusFilter === s
                    ? 'bg-brand-600 text-white shadow-sm'
                    : 'text-ink-600 hover:bg-ink-50',
                ].join(' ')}
              >
                {({ all: '全部', active: '进行中', done: '已完成', failed: '失败' } as Record<StatusFilter, string>)[s]}
              </button>
            ))}
          </div>
        </div>

        {/* List */}
        {isLoading ? (
          <div className="grid gap-3">
            {[1, 2, 3].map(i => <SkeletonCard key={i} />)}
          </div>
        ) : bids.length === 0 ? (
          <Card>
            <EmptyState
              icon="📋"
              title="还没有任何标书"
              description="从新建标书开始，体验 AI 驱动的标书编制流程"
              action={
                <Button variant="primary" onClick={() => setShowCreate(true)}>
                  新建第一个标书
                </Button>
              }
            />
          </Card>
        ) : filtered.length === 0 ? (
          <Card>
            <EmptyState
              icon="🔍"
              title="没有匹配的标书"
              description="试试调整搜索关键词或筛选条件"
            />
          </Card>
        ) : (
          <div className="grid gap-3">
            {filtered.map((bid, idx) => {
              const progress = bid.total_chapters > 0
                ? Math.round((bid.done_chapters / bid.total_chapters) * 100)
                : 0
              return (
                <Card
                  key={bid.id}
                  hover
                  onClick={() => navigate(`/bids/${bid.id}`)}
                  className="cursor-pointer animate-slide-up"
                  style={{ animationDelay: `${idx * 40}ms` }}
                >
                  <div className="flex items-center justify-between gap-4 mb-3">
                    <div className="flex items-center gap-3 min-w-0 flex-1">
                      <div className={[
                        'shrink-0 w-10 h-10 rounded-xl flex items-center justify-center text-base font-bold',
                        bid.status === 'done' ? 'bg-emerald-50 text-emerald-600' :
                        bid.status === 'failed' ? 'bg-red-50 text-red-600' :
                        'bg-brand-50 text-brand-600',
                      ].join(' ')}>
                        {(bid.project_name || '?').slice(0, 1).toUpperCase()}
                      </div>
                      <div className="min-w-0">
                        <h3 className="font-semibold text-ink-900 truncate">
                          {bid.project_name || '未命名项目'}
                        </h3>
                        <p className="text-xs text-ink-400 mt-0.5">
                          ID: <span className="font-mono">{bid.id.slice(0, 8)}</span>
                          {bid.industry && <span className="ml-2">· {bid.industry}</span>}
                        </p>
                      </div>
                    </div>
                    <StatusBadge status={bid.status} labels={statusLabels} />
                  </div>

                  <div className="flex items-center gap-5 text-xs text-ink-500 mb-3">
                    <div className="flex items-center gap-1.5">
                      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                        <path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20" />
                        <path d="M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2z" />
                      </svg>
                      <span><strong className="text-ink-700 tabular-nums">{bid.done_chapters}</strong>/{bid.total_chapters} 章节</span>
                    </div>
                    <div className="flex items-center gap-1.5">
                      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                        <rect x="3" y="4" width="18" height="18" rx="2" ry="2" />
                        <line x1="16" y1="2" x2="16" y2="6" />
                        <line x1="8" y1="2" x2="8" y2="6" />
                        <line x1="3" y1="10" x2="21" y2="10" />
                      </svg>
                      <span>{new Date(bid.created_at).toLocaleDateString('zh-CN')}</span>
                    </div>
                  </div>

                  {bid.total_chapters > 0 ? (
                    <ProgressBar value={progress} tone={bid.status === 'done' ? 'success' : 'brand'} showLabel size="sm" />
                  ) : (
                    <div className="text-xs text-ink-400 italic">尚未开始生成大纲</div>
                  )}
                </Card>
              )
            })}
          </div>
        )}
      </div>

      {/* Create modal */}
      <Modal
        open={showCreate}
        onClose={() => { if (!createMutation.isPending) { setShowCreate(false); setProjectName('') } }}
        title="新建标书"
        size="sm"
        footer={
          <>
            <Button variant="secondary" onClick={() => { setShowCreate(false); setProjectName('') }} disabled={createMutation.isPending}>
              取消
            </Button>
            <Button
              variant="primary"
              onClick={() => projectName.trim() && createMutation.mutate(projectName.trim())}
              disabled={!projectName.trim() || createMutation.isPending}
              loading={createMutation.isPending}
            >
              创建
            </Button>
          </>
        }
      >
        <div className="space-y-3">
          <TextInput
            label="项目名称"
            value={projectName}
            onChange={(e) => setProjectName(e.target.value)}
            onKeyDown={(e) => { if (e.key === 'Enter' && projectName.trim()) createMutation.mutate(projectName.trim()) }}
            placeholder="例如：XX 集团数字化转型项目"
            required
            autoFocus
            hint="命名后可在工作区内继续上传招标文件和材料"
          />
        </div>
      </Modal>
    </div>
  )
}
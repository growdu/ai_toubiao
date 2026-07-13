import { useState, useMemo } from 'react'
import { useNavigate } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { bidsApi, BidJob } from '../../api/bids'
import apiClient from '../../api/client'
import { toast } from '../../lib/toast'
import { usePageMeta } from '../../lib/usePageMeta'
import {
  Button, Card, EmptyState, Modal, ProgressBar, StatCard,
  TextInput, StatusBadge, SkeletonCard, Tabs,
} from '../../components/ui'

const statusLabels: Record<string, string> = {
  pending: '等待中', parsing: '解析中', outlining: '生成大纲',
  generating: '生成内容', auditing: '审计中', exporting: '导出中',
  done: '已完成', failed: '失败', paused: '已暂停', facts: '审查中',
}

type StatusFilter = 'all' | 'active' | 'done' | 'failed'
type SortBy = 'updated' | 'name' | 'progress'

export default function BidsPage() {
  usePageMeta({
    title: '标书管理',
    description: '管理所有投标文件，跟踪每一份标书的编制进度。',
    noindex: true,
  })

  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [showCreate, setShowCreate] = useState(false)
  const [projectName, setProjectName] = useState('')
  const [search, setSearch] = useState('')
  const [statusFilter, setStatusFilter] = useState<StatusFilter>('all')
  const [sortBy, setSortBy] = useState<SortBy>('updated')
  const [view, setView] = useState<'grid' | 'list'>('grid')

  const { data, isLoading } = useQuery({
    queryKey: ['bids'],
    queryFn: () => bidsApi.list(),
  })

  const bids: BidJob[] = Array.isArray(data?.data?.data) ? data!.data!.data! : []

  const createMutation = useMutation({
    mutationFn: async (name: string) => {
      const projRes = await apiClient.post('/projects', { name })
      return bidsApi.create({ project_id: (projRes.data?.data as any).id })
    },
    onSuccess: (res) => {
      queryClient.invalidateQueries({ queryKey: ['bids'] })
      setShowCreate(false)
      setProjectName('')
      toast.success('标书已创建', '正在跳转到工作区…')
      navigate(`/bids/${(res.data?.data as any).id}`)
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
    // Sort
    list = [...list].sort((a, b) => {
      if (sortBy === 'name') return (a.project_name || '').localeCompare(b.project_name || '')
      if (sortBy === 'progress') {
        const ap = a.total_chapters > 0 ? a.done_chapters / a.total_chapters : 0
        const bp = b.total_chapters > 0 ? b.done_chapters / b.total_chapters : 0
        return bp - ap
      }
      // 'updated' — fall back to created_at desc since the API doesn't expose updated_at
      return new Date(b.created_at).getTime() - new Date(a.created_at).getTime()
    })
    return list
  }, [bids, search, statusFilter, sortBy])

  const stats = useMemo(() => {
    const total = bids.length
    const active = bids.filter(b => ['pending', 'parsing', 'outlining', 'generating', 'auditing', 'exporting', 'facts', 'paused'].includes(b.status)).length
    const done = bids.filter(b => b.status === 'done').length
    const failed = bids.filter(b => b.status === 'failed').length
    const totalChapters = bids.reduce((acc, b) => acc + (b.total_chapters ?? 0), 0)
    const doneChapters = bids.reduce((acc, b) => acc + (b.done_chapters ?? 0), 0)
    const overallProgress = totalChapters > 0 ? Math.round((doneChapters / totalChapters) * 100) : 0
    return { total, active, done, failed, overallProgress }
  }, [bids])

  return (
    <div className="flex-1 overflow-y-auto scrollbar-thin">
      <div className="max-w-6xl mx-auto px-6 py-8">
        {/* Header */}
        <div className="flex items-end justify-between mb-6 gap-4 animate-fade-in">
          <div>
            <div className="text-xs font-medium uppercase tracking-wider text-brand-600 dark:text-brand-400 mb-1">工作台</div>
            <h1 className="text-2xl font-bold text-ink-900 dark:text-white tracking-tight">标书管理</h1>
            <p className="text-sm text-ink-500 dark:text-ink-400 mt-1">创建新标书，跟踪每一份标书的编制进度</p>
          </div>
          <Button variant="primary" size="md" onClick={() => setShowCreate(true)} leftIcon={
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round">
              <line x1="12" y1="5" x2="12" y2="19" />
              <line x1="5" y1="12" x2="19" y2="12" />
            </svg>
          }>新建标书</Button>
        </div>

        {/* Stats — now with overall progress card */}
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

        {/* Overall progress strip — visible only when there's at least one chapter */}
        {stats.total > 0 && (stats.total * 1) > 0 && (
          <Card padded={false} className="mb-5 animate-slide-up overflow-hidden">
            <div className="px-5 py-4 flex items-center gap-4">
              <div className="flex-1 min-w-0">
                <div className="flex items-center justify-between mb-1.5">
                  <div className="text-xs font-semibold text-ink-700 dark:text-ink-200">整体进度</div>
                  <div className="text-[11px] text-ink-500 dark:text-ink-400 tabular-nums">
                    已完成 <strong className="text-ink-700 dark:text-ink-200">{bids.reduce((a, b) => a + (b.done_chapters ?? 0), 0)}</strong> / {bids.reduce((a, b) => a + (b.total_chapters ?? 0), 0)} 章节
                  </div>
                </div>
                <ProgressBar value={stats.overallProgress} tone={stats.overallProgress === 100 ? 'success' : 'rainbow'} shimmer />
              </div>
              <div className="shrink-0 hidden sm:flex items-center gap-2 pl-4 border-l border-ink-100 dark:border-ink-700">
                <span className="text-3xl font-bold text-gradient tabular-nums">{stats.overallProgress}%</span>
              </div>
            </div>
          </Card>
        )}

        {/* Filters */}
        <div className="flex flex-col sm:flex-row items-stretch sm:items-center gap-3 mb-5">
          <div className="flex-1 max-w-xs relative">
            <span className="absolute left-3 top-1/2 -translate-y-1/2 text-ink-400 pointer-events-none">
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <circle cx="11" cy="11" r="8" />
                <line x1="21" y1="21" x2="16.65" y2="16.65" />
              </svg>
            </span>
            <TextInput
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="搜索标书名称…"
              className="pl-9"
            />
          </div>
          <Tabs
            variant="pill"
            value={statusFilter}
            onChange={(v) => setStatusFilter(v as StatusFilter)}
            items={[
              { id: 'all', label: '全部' },
              { id: 'active', label: '进行中' },
              { id: 'done', label: '已完成' },
              { id: 'failed', label: '失败' },
            ]}
          />
          <div className="flex items-center gap-1 ml-auto">
            {/* Sort dropdown */}
            <select
              value={sortBy}
              onChange={(e) => setSortBy(e.target.value as SortBy)}
              className="text-xs px-2.5 py-1.5 bg-white dark:bg-ink-800 border border-ink-200 dark:border-ink-700 rounded-lg text-ink-700 dark:text-ink-200 focus:outline-none focus:border-brand-400 cursor-pointer hover:bg-ink-50 dark:hover:bg-ink-700"
              aria-label="排序"
            >
              <option value="updated">最近创建</option>
              <option value="name">按名称</option>
              <option value="progress">按进度</option>
            </select>
            {/* View toggle */}
            <div className="flex items-center gap-0.5 p-1 bg-ink-100 dark:bg-ink-800 rounded-lg">
              <button
                onClick={() => setView('grid')}
                aria-label="网格视图"
                className={`p-1.5 rounded-md transition-colors ${view === 'grid' ? 'bg-white dark:bg-ink-700 text-brand-600 dark:text-brand-300 shadow-sm' : 'text-ink-500 dark:text-ink-400 hover:text-ink-700 dark:hover:text-ink-200'}`}
              >
                <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <rect x="3" y="3" width="7" height="7" /><rect x="14" y="3" width="7" height="7" /><rect x="14" y="14" width="7" height="7" /><rect x="3" y="14" width="7" height="7" />
                </svg>
              </button>
              <button
                onClick={() => setView('list')}
                aria-label="列表视图"
                className={`p-1.5 rounded-md transition-colors ${view === 'list' ? 'bg-white dark:bg-ink-700 text-brand-600 dark:text-brand-300 shadow-sm' : 'text-ink-500 dark:text-ink-400 hover:text-ink-700 dark:hover:text-ink-200'}`}
              >
                <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <line x1="8" y1="6" x2="21" y2="6" /><line x1="8" y1="12" x2="21" y2="12" /><line x1="8" y1="18" x2="21" y2="18" /><line x1="3" y1="6" x2="3.01" y2="6" /><line x1="3" y1="12" x2="3.01" y2="12" /><line x1="3" y1="18" x2="3.01" y2="18" />
                </svg>
              </button>
            </div>
          </div>
        </div>

        {/* List */}
        {isLoading ? (
          <div className={view === 'grid' ? 'grid md:grid-cols-2 gap-3' : 'space-y-2'}>
            {[1, 2, 3, 4].map(i => <SkeletonCard key={i} />)}
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
              action={
                <Button variant="ghost" size="sm" onClick={() => { setSearch(''); setStatusFilter('all') }}>
                  清除筛选
                </Button>
              }
            />
          </Card>
        ) : view === 'grid' ? (
          <div className="grid md:grid-cols-2 gap-3">
            {filtered.map((bid, idx) => {
              const progress = bid.total_chapters > 0
                ? Math.round((bid.done_chapters / bid.total_chapters) * 100)
                : 0
              return (
                <Card
                  key={bid.id}
                  hover
                  onClick={() => navigate(`/bids/${bid.id}`)}
                  className="cursor-pointer animate-slide-up relative overflow-hidden"
                  style={{ animationDelay: `${idx * 40}ms` }}
                >
                  {/* Subtle corner ribbon for done */}
                  {bid.status === 'done' && (
                    <div className="absolute top-0 right-0 w-12 h-12 pointer-events-none">
                      <div className="absolute top-2 right-2 w-7 h-7 rounded-full bg-emerald-500 text-white grid place-items-center text-xs shadow-pop">
                        ✓
                      </div>
                    </div>
                  )}
                  <div className="flex items-center justify-between gap-4 mb-3">
                    <div className="flex items-center gap-3 min-w-0 flex-1">
                      <div className={[
                        'shrink-0 w-10 h-10 rounded-xl flex items-center justify-center text-base font-bold transition-transform group-hover:scale-110',
                        bid.status === 'done' ? 'bg-emerald-50 text-emerald-600 dark:bg-emerald-900/30 dark:text-emerald-300' :
                        bid.status === 'failed' ? 'bg-red-50 text-red-600 dark:bg-red-900/30 dark:text-red-300' :
                        'bg-brand-50 text-brand-600 dark:bg-brand-900/30 dark:text-brand-300',
                      ].join(' ')}>
                        {(bid.project_name || '?').slice(0, 1).toUpperCase()}
                      </div>
                      <div className="min-w-0">
                        <h3 className="font-semibold text-ink-900 dark:text-white truncate">
                          {bid.project_name || '未命名项目'}
                        </h3>
                        <p className="text-xs text-ink-400 dark:text-ink-500 mt-0.5">
                          ID: <span className="font-mono">{bid.id.slice(0, 8)}</span>
                          {bid.industry && <span className="ml-2">· {bid.industry}</span>}
                        </p>
                      </div>
                    </div>
                    <StatusBadge status={bid.status} labels={statusLabels} />
                  </div>

                  <div className="flex items-center gap-4 text-xs text-ink-500 dark:text-ink-400 mb-3">
                    <div className="flex items-center gap-1.5">
                      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                        <path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20" />
                        <path d="M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2z" />
                      </svg>
                      <span><strong className="text-ink-700 dark:text-ink-200 tabular-nums">{bid.done_chapters}</strong>/{bid.total_chapters} 章节</span>
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
                    <ProgressBar value={progress} tone={bid.status === 'done' ? 'success' : bid.status === 'failed' ? 'rose' : 'brand'} showPercent size="sm" />
                  ) : (
                    <div className="text-xs text-ink-400 dark:text-ink-500 italic">尚未开始生成大纲</div>
                  )}

                  {/* hover-revealed actions */}
                  <div className="absolute bottom-2 right-2 opacity-0 group-hover:opacity-100 transition-opacity flex items-center gap-1 text-xs text-brand-600 dark:text-brand-400 font-medium">
                    打开
                    <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
                      <line x1="5" y1="12" x2="19" y2="12" />
                      <polyline points="12 5 19 12 12 19" />
                    </svg>
                  </div>
                </Card>
              )
            })}
          </div>
        ) : (
          // List view (compact rows)
          <Card padded={false} className="overflow-hidden">
            <div className="divide-y divide-ink-100 dark:divide-ink-700">
              {filtered.map((bid, idx) => {
                const progress = bid.total_chapters > 0
                  ? Math.round((bid.done_chapters / bid.total_chapters) * 100)
                  : 0
                return (
                  <div
                    key={bid.id}
                    onClick={() => navigate(`/bids/${bid.id}`)}
                    className="group flex items-center gap-3 px-4 py-3 hover:bg-ink-50 dark:hover:bg-ink-800/50 cursor-pointer transition-colors animate-slide-up"
                    style={{ animationDelay: `${idx * 20}ms` }}
                  >
                    <div className={[
                      'shrink-0 w-9 h-9 rounded-lg flex items-center justify-center text-sm font-bold',
                      bid.status === 'done' ? 'bg-emerald-50 text-emerald-600 dark:bg-emerald-900/30 dark:text-emerald-300' :
                      bid.status === 'failed' ? 'bg-red-50 text-red-600 dark:bg-red-900/30 dark:text-red-300' :
                      'bg-brand-50 text-brand-600 dark:bg-brand-900/30 dark:text-brand-300',
                    ].join(' ')}>
                      {(bid.project_name || '?').slice(0, 1).toUpperCase()}
                    </div>
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center gap-2">
                        <h3 className="font-medium text-ink-900 dark:text-white truncate text-sm">{bid.project_name || '未命名项目'}</h3>
                        <StatusBadge status={bid.status} labels={statusLabels} showDot={false} />
                      </div>
                      <div className="text-[11px] text-ink-400 dark:text-ink-500 mt-0.5 flex items-center gap-2">
                        <span className="font-mono">{bid.id.slice(0, 8)}</span>
                        <span>·</span>
                        <span>{bid.done_chapters}/{bid.total_chapters} 章节</span>
                        <span>·</span>
                        <span>{new Date(bid.created_at).toLocaleDateString('zh-CN')}</span>
                      </div>
                    </div>
                    {bid.total_chapters > 0 && (
                      <div className="w-32 shrink-0 hidden md:block">
                        <ProgressBar value={progress} tone={bid.status === 'done' ? 'success' : bid.status === 'failed' ? 'rose' : 'brand'} size="sm" />
                      </div>
                    )}
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="shrink-0 text-ink-300 dark:text-ink-600 group-hover:text-brand-500 transition-colors">
                      <polyline points="9 18 15 12 9 6" />
                    </svg>
                  </div>
                )
              })}
            </div>
          </Card>
        )}
      </div>

      {/* Create modal */}
      <Modal
        open={showCreate}
        onClose={() => { if (!createMutation.isPending) { setShowCreate(false); setProjectName('') } }}
        title="新建标书"
        description="为这次投标起一个名字，方便后面在工作区里区分"
        size="sm"
        icon={
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
            <polyline points="14 2 14 8 20 8" />
            <line x1="12" y1="11" x2="12" y2="17" />
            <line x1="9" y1="14" x2="15" y2="14" />
          </svg>
        }
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
            leftIcon={
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="text-ink-400">
                <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
                <polyline points="14 2 14 8 20 8" />
              </svg>
            }
          />
          {/* Quick templates */}
          <div className="text-xs text-ink-500 dark:text-ink-400">
            <div className="mb-1.5">快速填充：</div>
            <div className="flex flex-wrap gap-1.5">
              {['XX 集团数字化转型项目', '深圳地铁 12 号线施工总承包', '智慧城市基础平台采购'].map(t => (
                <button
                  key={t}
                  type="button"
                  onClick={() => setProjectName(t)}
                  className="px-2 py-1 rounded-md bg-ink-50 dark:bg-ink-800 text-[11px] text-ink-600 dark:text-ink-300 hover:bg-brand-50 hover:text-brand-700 dark:hover:bg-brand-900/40 dark:hover:text-brand-300 transition-colors"
                >
                  {t}
                </button>
              ))}
            </div>
          </div>
        </div>
      </Modal>
    </div>
  )
}
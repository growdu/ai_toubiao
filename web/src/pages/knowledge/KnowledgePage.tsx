import { useState, useMemo } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import api from '../../api/client'
import { toast } from '../../lib/toast'
import { usePageMeta } from '../../lib/usePageMeta'
import {
  Button, Card, EmptyState, Modal, StatCard, StatusBadge,
  TextArea, TextInput, Select, SkeletonCard,
} from '../../components/ui'

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

const CATEGORY_LABELS: Record<string, string> = {
  certificate: '资质证书',
  case: '项目案例',
  patent: '专利',
  team: '团队成员',
  equipment: '设备',
  qualification: '资格认证',
  other: '其他',
}

const CATEGORY_COLORS: Record<string, string> = {
  certificate: 'bg-amber-50 text-amber-700',
  case:        'bg-brand-50 text-brand-700',
  patent:      'bg-violet-50 text-violet-700',
  team:        'bg-emerald-50 text-emerald-700',
  equipment:   'bg-orange-50 text-orange-700',
  qualification: 'bg-cyan-50 text-cyan-700',
  other:       'bg-ink-100 text-ink-600',
}

const CATEGORY_ICONS: Record<string, string> = {
  certificate: '📜',
  case: '📂',
  patent: '💡',
  team: '👥',
  equipment: '🛠️',
  qualification: '🏆',
  other: '📄',
}

const STATUS_LABELS: Record<string, string> = {
  pending: '待处理',
  processing: '处理中',
  ready: '已就绪',
  failed: '失败',
}

export default function KnowledgePage() {
  usePageMeta({
    title: '知识库',
    description: '管理企业资质、过往标书、案例素材等可供 AI 引用的证据库。',
    noindex: true,
  })

  const queryClient = useQueryClient()
  const [showUpload, setShowUpload] = useState(false)
  const [form, setForm] = useState<CreateMaterialRequest>({ category: 'other', title: '', content: '' })
  const [search, setSearch] = useState('')
  const [catFilter, setCatFilter] = useState<string>('all')

  const { data, isLoading } = useQuery({
    queryKey: ['kb-materials'],
    queryFn: () => api.get<{ data: KBMaterial[] }>('/kb/materials'),
  })

  const createMutation = useMutation({
    mutationFn: (body: CreateMaterialRequest) => api.post('/kb/materials', body),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['kb-materials'] })
      setShowUpload(false)
      setForm({ category: 'other', title: '', content: '' })
      toast.success('素材已上传', '正在后台索引…')
    },
    onError: (err: any) => toast.error('上传失败', err?.response?.data?.message),
  })

  const materials = data?.data.data ?? []

  const filtered = useMemo(() => {
    let list = materials
    if (catFilter !== 'all') list = list.filter(m => m.category === catFilter)
    if (search.trim()) {
      const q = search.trim().toLowerCase()
      list = list.filter(m => m.title.toLowerCase().includes(q))
    }
    return list
  }, [materials, catFilter, search])

  const stats = useMemo(() => {
    const total = materials.length
    const chunks = materials.reduce((acc, m) => acc + m.chunk_count, 0)
    const ready = materials.filter(m => m.status === 'ready').length
    const categories = new Set(materials.map(m => m.category)).size
    return { total, chunks, ready, categories }
  }, [materials])

  return (
    <div className="flex-1 overflow-y-auto scrollbar-thin">
      <div className="max-w-6xl mx-auto px-6 py-8">
        {/* Header */}
        <div className="flex items-end justify-between mb-6 gap-4 animate-fade-in">
          <div>
            <div className="text-xs font-medium uppercase tracking-wider text-brand-600 mb-1">资源中心</div>
            <h1 className="text-2xl font-bold text-ink-900 tracking-tight">知识库</h1>
            <p className="text-sm text-ink-500 mt-1">管理企业素材、案例、资质，作为标书编制的证据源</p>
          </div>
          <Button variant="primary" onClick={() => setShowUpload(true)} leftIcon={
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
              <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
              <polyline points="17 8 12 3 7 8" />
              <line x1="12" y1="3" x2="12" y2="15" />
            </svg>
          }>上传素材</Button>
        </div>

        {/* Stats */}
        <div className="grid grid-cols-2 md:grid-cols-4 gap-3 mb-6 animate-slide-up">
          <StatCard
            tone="blue"
            label="素材总数"
            value={stats.total}
            icon={<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" /><polyline points="14 2 14 8 20 8" /></svg>}
          />
          <StatCard
            tone="purple"
            label="分类数"
            value={stats.categories}
            icon={<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><rect x="3" y="3" width="7" height="7" /><rect x="14" y="3" width="7" height="7" /><rect x="14" y="14" width="7" height="7" /><rect x="3" y="14" width="7" height="7" /></svg>}
          />
          <StatCard
            tone="green"
            label="已就绪"
            value={stats.ready}
            icon={<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><polyline points="20 6 9 17 4 12" /></svg>}
          />
          <StatCard
            tone="amber"
            label="总片段"
            value={stats.chunks}
            icon={<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><line x1="8" y1="6" x2="21" y2="6" /><line x1="8" y1="12" x2="21" y2="12" /><line x1="8" y1="18" x2="21" y2="18" /></svg>}
          />
        </div>

        {/* Filters */}
        <div className="flex flex-col sm:flex-row items-stretch sm:items-center gap-3 mb-5">
          <div className="flex-1 max-w-xs">
            <TextInput
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="搜索素材…"
            />
          </div>
          <div className="flex items-center gap-1 p-1 bg-white rounded-xl border border-ink-100 shadow-soft overflow-x-auto scrollbar-none">
            <CategoryPill active={catFilter === 'all'} onClick={() => setCatFilter('all')} label="全部分类" />
            {Object.entries(CATEGORY_LABELS).map(([k, v]) => (
              <CategoryPill key={k} active={catFilter === k} onClick={() => setCatFilter(k)} label={v} />
            ))}
          </div>
        </div>

        {/* Material grid */}
        {isLoading ? (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {[1, 2, 3, 4, 5, 6].map(i => <SkeletonCard key={i} />)}
          </div>
        ) : materials.length === 0 ? (
          <Card>
            <EmptyState
              icon="📚"
              title="知识库是空的"
              description="上传企业资质、项目案例、专利等素材，AI 会自动索引并用于证据检索"
              action={
                <Button variant="primary" onClick={() => setShowUpload(true)}>
                  上传第一批素材
                </Button>
              }
            />
          </Card>
        ) : filtered.length === 0 ? (
          <Card>
            <EmptyState
              icon="🔍"
              title="没有匹配的素材"
              description="尝试调整搜索关键词或切换分类"
            />
          </Card>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {filtered.map((m, idx) => (
              <Card
                key={m.id}
                hover
                className="animate-slide-up"
                style={{ animationDelay: `${idx * 30}ms` }}
              >
                <div className="flex items-start gap-3 mb-3">
                  <div className={[
                    'shrink-0 w-10 h-10 rounded-xl flex items-center justify-center text-lg',
                    CATEGORY_COLORS[m.category] ?? CATEGORY_COLORS.other,
                  ].join(' ')}>
                    {CATEGORY_ICONS[m.category] ?? '📄'}
                  </div>
                  <div className="min-w-0 flex-1">
                    <h3 className="font-semibold text-ink-900 truncate" title={m.title}>{m.title}</h3>
                    <span className={[
                      'inline-flex items-center text-[11px] px-1.5 py-0.5 rounded font-medium mt-1',
                      CATEGORY_COLORS[m.category] ?? CATEGORY_COLORS.other,
                    ].join(' ')}>
                      {CATEGORY_LABELS[m.category] ?? m.category}
                    </span>
                  </div>
                  <StatusBadge status={m.status} labels={STATUS_LABELS} showDot />
                </div>
                <div className="flex items-center justify-between text-xs text-ink-400 pt-3 border-t border-ink-100">
                  <span className="inline-flex items-center gap-1">
                    <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                      <line x1="8" y1="6" x2="21" y2="6" />
                      <line x1="8" y1="12" x2="21" y2="12" />
                      <line x1="8" y1="18" x2="21" y2="18" />
                    </svg>
                    <strong className="text-ink-700 tabular-nums">{m.chunk_count}</strong> 片段
                  </span>
                  <span>{new Date(m.created_at).toLocaleDateString('zh-CN')}</span>
                </div>
              </Card>
            ))}
          </div>
        )}
      </div>

      {/* Upload modal */}
      <Modal
        open={showUpload}
        onClose={() => { if (!createMutation.isPending) setShowUpload(false) }}
        title="上传素材"
        size="md"
        footer={
          <>
            <Button variant="secondary" onClick={() => setShowUpload(false)} disabled={createMutation.isPending}>取消</Button>
            <Button
              variant="primary"
              onClick={() => createMutation.mutate(form)}
              disabled={!form.title.trim() || createMutation.isPending}
              loading={createMutation.isPending}
            >
              上传
            </Button>
          </>
        }
      >
        <div className="space-y-4">
          <TextInput
            label="标题"
            value={form.title}
            onChange={(e) => setForm({ ...form, title: e.target.value })}
            placeholder="例如：ISO 9001 质量管理体系认证"
            required
            autoFocus
          />
          <Select
            label="分类"
            value={form.category}
            onChange={(e) => setForm({ ...form, category: e.target.value })}
          >
            {Object.entries(CATEGORY_LABELS).map(([k, v]) => (
              <option key={k} value={k}>{v}</option>
            ))}
          </Select>
          <TextArea
            label="内容"
            value={form.content ?? ''}
            onChange={(e) => setForm({ ...form, content: e.target.value })}
            rows={6}
            placeholder="粘贴素材正文，或上传文件 (PDF / Word / 文本)…"
            hint="上传后系统会自动切分为知识片段并建立向量索引"
          />
          <label className="block">
            <span className="block text-sm font-medium text-ink-700 mb-1.5">文件</span>
            <div className="border-2 border-dashed border-ink-200 rounded-lg p-6 text-center hover:border-brand-400 hover:bg-brand-50/30 transition-colors cursor-pointer">
              <input type="file" className="hidden" accept=".pdf,.docx,.txt,.md" />
              <div className="text-2xl mb-1">📎</div>
              <p className="text-xs text-ink-500">点击或拖拽文件到此处</p>
              <p className="text-[11px] text-ink-400 mt-0.5">PDF · DOCX · TXT · MD · 单文件 ≤ 20MB</p>
            </div>
          </label>
        </div>
      </Modal>
    </div>
  )
}

function CategoryPill({ active, onClick, label }: { active: boolean; onClick: () => void; label: string }) {
  return (
    <button
      onClick={onClick}
      className={[
        'shrink-0 px-3 py-1.5 text-xs font-medium rounded-lg transition-colors whitespace-nowrap',
        active ? 'bg-brand-600 text-white shadow-sm' : 'text-ink-600 hover:bg-ink-50',
      ].join(' ')}
    >
      {label}
    </button>
  )
}
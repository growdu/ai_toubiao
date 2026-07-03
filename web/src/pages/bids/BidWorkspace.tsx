import { useState, useCallback } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { bidsApi, ChapterSpec, ChapterContent, AddChapterRequest } from '../../api/bids'

const statusLabels: Record<string, string> = {
  planned: '待生成', pending: '等待中', running: '生成中',
  succeeded: '已生成', failed: '失败', skipped: '跳过',
}
const statusColors: Record<string, string> = {
  planned: 'bg-gray-100 text-gray-600', pending: 'bg-yellow-100 text-yellow-700',
  running: 'bg-blue-100 text-blue-700', succeeded: 'bg-green-100 text-green-700',
  failed: 'bg-red-100 text-red-700', skipped: 'bg-gray-100 text-gray-400',
}

export default function BidWorkspace() {
  const { id } = useParams<{ id: string }>()
  const queryClient = useQueryClient()
  const [selectedChapterId, setSelectedChapterId] = useState<string | null>(null)
  const [showAddForm, setShowAddForm] = useState(false)
  const [newChapterTitle, setNewChapterTitle] = useState('')
  const [newChapterLevel, setNewChapterLevel] = useState(1)
  const [newChapterParent, setNewChapterParent] = useState('')
  const [materialText, setMaterialText] = useState('')
  const [showMaterial, setShowMaterial] = useState(false)

  const { data: bidData } = useQuery({
    queryKey: ['bid', id],
    queryFn: () => bidsApi.get(id!),
    enabled: !!id,
  })
  const bid = bidData?.data.data

  const { data: outlineData, isLoading: outlineLoading } = useQuery({
    queryKey: ['outline', id],
    queryFn: () => bidsApi.getOutline(id!),
    enabled: !!id,
    refetchInterval: 5000,
  })
  const chapters: ChapterSpec[] = outlineData?.data.data || []

  const { data: contentData } = useQuery({
    queryKey: ['chapter-content', id, selectedChapterId],
    queryFn: () => bidsApi.getChapterContent(id!, selectedChapterId!),
    enabled: !!id && !!selectedChapterId,
    refetchInterval: 3000,
  })
  const content: ChapterContent | null = contentData?.data.data || null

  if (!selectedChapterId && chapters.length > 0) setSelectedChapterId(chapters[0].id)
  const selectedChapter = chapters.find(c => c.id === selectedChapterId)

  const addMutation = useMutation({
    mutationFn: (data: AddChapterRequest) => bidsApi.addChapter(id!, data),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['outline', id] }); setShowAddForm(false); setNewChapterTitle('') },
  })
  const deleteMutation = useMutation({
    mutationFn: (chapterId: string) => bidsApi.deleteChapter(id!, chapterId),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['outline', id] }); setSelectedChapterId(null) },
  })
  const updateMutation = useMutation({
    mutationFn: ({ chapterId, data }: { chapterId: string; data: any }) => bidsApi.updateChapter(id!, chapterId, data),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['outline', id] }),
  })
  const transitionMutation = useMutation({
    mutationFn: ({ to, version }: { to: string; version: number }) => bidsApi.transition(id!, to, version, `transition to ${to}`),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['bid', id] }),
  })
  const saveMaterialMutation = useMutation({
    mutationFn: (text: string) => bidsApi.saveMaterial(id!, text),
    onSuccess: () => setShowMaterial(false),
  })
  const generateMutation = useMutation({
    mutationFn: (chapterId: string) => bidsApi.generateChapter(id!, chapterId),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['outline', id] }); queryClient.invalidateQueries({ queryKey: ['chapter-content', id, selectedChapterId] }) },
  })

  const handleAddChapter = useCallback(() => {
    if (!newChapterTitle.trim()) return
    addMutation.mutate({ title: newChapterTitle, level: newChapterLevel, parent_id: newChapterParent || undefined })
  }, [newChapterTitle, newChapterLevel, newChapterParent, addMutation])

  const handleDelete = useCallback((chapterId: string) => {
    if (confirm('确定删除该章节？子章节也会一并删除。')) deleteMutation.mutate(chapterId)
  }, [deleteMutation])

  const tree = buildTree(chapters)
  const progress = bid && bid.total_chapters > 0 ? Math.round((bid.done_chapters / bid.total_chapters) * 100) : 0

  return (
    <div className="h-full flex flex-col">
      {/* Top bar */}
      <div className="flex items-center justify-between px-4 py-2 bg-white border-b shrink-0">
        <div className="flex items-center gap-3">
          <Link to="/bids" className="text-gray-400 hover:text-gray-600 text-sm">← 返回</Link>
          <h1 className="text-lg font-bold">{bid?.project_name || '标书'}</h1>
          <span className={`text-xs px-2 py-0.5 rounded-full ${bid ? statusColors[bid.status] || 'bg-gray-100' : ''}`}>
            {bid ? statusLabels[bid.status] || bid.status : ''}
          </span>
        </div>
        <div className="flex items-center gap-2">
          {bid && (
            <div className="flex items-center gap-1 text-xs text-gray-400 mr-2">
              <div className="w-20 bg-gray-200 rounded-full h-1.5"><div className="bg-blue-600 h-1.5 rounded-full" style={{ width: `${progress}%` }} /></div>
              <span>{bid.done_chapters}/{bid.total_chapters}</span>
            </div>
          )}
          {bid && (bid.status === 'pending' || bid.status === 'parsing') && (
            <button onClick={() => transitionMutation.mutate({ to: 'parsing', version: bid.version })} className="px-3 py-1 text-sm bg-gray-600 text-white rounded hover:bg-gray-700">开始解析</button>
          )}
          {bid && (bid.status === 'parsing' || bid.status === 'outlining') && (
            <button onClick={() => transitionMutation.mutate({ to: 'outlining', version: bid.version })} className="px-3 py-1 text-sm bg-indigo-600 text-white rounded hover:bg-indigo-700">生成大纲</button>
          )}
          {bid && bid.status === 'facts' && (
            <button onClick={() => transitionMutation.mutate({ to: 'generating', version: bid.version })} className="px-3 py-1 text-sm bg-blue-600 text-white rounded hover:bg-blue-700">批量生成内容</button>
          )}
          {bid && bid.status === 'generating' && (
            <button onClick={() => transitionMutation.mutate({ to: 'auditing', version: bid.version })} className="px-3 py-1 text-sm bg-purple-600 text-white rounded hover:bg-purple-700">开始审计</button>
          )}
          {bid && bid.status === 'auditing' && (
            <button onClick={() => transitionMutation.mutate({ to: 'exporting', version: bid.version })} className="px-3 py-1 text-sm bg-green-600 text-white rounded hover:bg-green-700">导出文档</button>
          )}
          {bid && bid.status === 'done' && (
            <Link to={`/bids/${id}/export`} className="px-3 py-1 text-sm bg-green-600 text-white rounded hover:bg-green-700">下载文档</Link>
          )}
        </div>
      </div>

      {/* Three-panel layout */}
      <div className="flex flex-1 overflow-hidden">
        {/* Left: Chapter directory */}
        <div className="w-72 bg-gray-50 border-r flex flex-col overflow-hidden">
          {/* Material upload toggle */}
          <div className="border-b">
            <button onClick={() => setShowMaterial(!showMaterial)} className="w-full flex items-center justify-between px-3 py-2 text-sm text-gray-600 hover:bg-gray-100">
              <span>📋 标书材料</span>
              <span className="text-gray-400">{showMaterial ? '▼' : '▶'}</span>
            </button>
            {showMaterial && (
              <div className="px-3 pb-2 space-y-2">
                <textarea
                  value={materialText}
                  onChange={e => setMaterialText(e.target.value)}
                  placeholder="粘贴招标文件内容、技术要求、评分标准等..."
                  className="w-full h-32 px-2 py-1 text-xs border rounded resize-none focus:outline-none focus:ring-1 focus:ring-blue-400"
                />
                <div className="flex gap-2">
                  <button className="flex-1 px-2 py-1 text-xs bg-gray-200 rounded hover:bg-gray-300">上传文件</button>
                  <button onClick={() => saveMaterialMutation.mutate(materialText)} disabled={!materialText.trim() || saveMaterialMutation.isPending} className="flex-1 px-2 py-1 text-xs bg-blue-600 text-white rounded hover:bg-blue-700 disabled:opacity-50">{saveMaterialMutation.isPending ? "保存中..." : "保存材料"}</button>
                </div>
              </div>
            )}
          </div>

          {/* Chapter tree header */}
          <div className="flex items-center justify-between px-3 py-2 border-b bg-white">
            <span className="text-sm font-medium text-gray-700">章节目录</span>
            <button onClick={() => setShowAddForm(!showAddForm)} className="text-blue-600 hover:text-blue-800 text-sm">+ 添加</button>
          </div>

          {showAddForm && (
            <div className="px-3 py-2 border-b bg-blue-50 space-y-2">
              <input type="text" value={newChapterTitle} onChange={e => setNewChapterTitle(e.target.value)} onKeyDown={e => e.key === 'Enter' && handleAddChapter()} placeholder="章节标题" className="w-full px-2 py-1 text-sm border rounded focus:outline-none focus:ring-1 focus:ring-blue-400" autoFocus />
              <div className="flex gap-2">
                <select value={newChapterLevel} onChange={e => setNewChapterLevel(Number(e.target.value))} className="text-sm border rounded px-1 py-1"><option value={1}>一级</option><option value={2}>二级</option><option value={3}>三级</option></select>
                <select value={newChapterParent} onChange={e => setNewChapterParent(e.target.value)} className="text-sm border rounded px-1 py-1 flex-1"><option value="">无父级</option>{chapters.filter(c => c.level < 3).map(c => <option key={c.id} value={c.id}>{c.title}</option>)}</select>
                <button onClick={handleAddChapter} className="px-2 py-1 text-sm bg-blue-600 text-white rounded hover:bg-blue-700">确定</button>
              </div>
            </div>
          )}

          <div className="flex-1 overflow-y-auto py-1">
            {outlineLoading ? <div className="text-center text-gray-400 text-sm py-4">加载中...</div> :
             tree.length === 0 ? <div className="text-center text-gray-400 text-sm py-4">暂无章节<br/>点击"+ 添加"创建</div> :
             tree.map(node => <ChapterTreeNode key={node.id} node={node} selectedId={selectedChapterId} onSelect={setSelectedChapterId} onDelete={handleDelete} />)}
          </div>
        </div>

        {/* Middle: Content editor */}
        <div className="flex-1 flex flex-col overflow-hidden bg-white">
          {selectedChapter ? <ChapterEditor chapter={selectedChapter} content={content} onUpdate={(data) => updateMutation.mutate({ chapterId: selectedChapter.id, data })} bidId={id!} /> :
            <div className="flex-1 flex items-center justify-center text-gray-400"><div className="text-center"><p className="text-lg mb-2">未选择章节</p><p className="text-sm">从左侧目录中选择一个章节</p></div></div>}
        </div>

        {/* Right: Prompt panel */}
        <div className="w-80 bg-gray-50 border-l flex flex-col overflow-hidden">
          {selectedChapter ? <PromptPanel chapter={selectedChapter} content={content} bidStatus={bid?.status || ''} onGenerate={() => generateMutation.mutate(selectedChapter.id)} generating={generateMutation.isPending} /> :
            <div className="flex-1 flex items-center justify-center text-gray-400 text-sm">选择章节查看配置</div>}
        </div>
      </div>
    </div>
  )
}

// ============ Tree ============
interface TreeNode extends ChapterSpec { children: TreeNode[] }
function buildTree(chapters: ChapterSpec[]): TreeNode[] {
  const map = new Map<string, TreeNode>(); const roots: TreeNode[] = []
  chapters.forEach(c => map.set(c.id, { ...c, children: [] }))
  chapters.forEach(c => { const node = map.get(c.id)!; if (c.parent_id && map.has(c.parent_id)) map.get(c.parent_id)!.children.push(node); else roots.push(node) })
  roots.sort((a, b) => a.order_index - b.order_index)
  return roots
}

function ChapterTreeNode({ node, selectedId, onSelect, onDelete, depth = 0 }: { node: TreeNode; selectedId: string | null; onSelect: (id: string) => void; onDelete: (id: string) => void; depth?: number }) {
  const isSelected = node.id === selectedId; const hasChildren = node.children.length > 0; const [expanded, setExpanded] = useState(true)
  return (
    <div>
      <div className={`group flex items-center gap-1 px-2 py-1.5 cursor-pointer hover:bg-blue-50 ${isSelected ? 'bg-blue-100 border-l-2 border-blue-600' : ''}`} style={{ paddingLeft: `${depth * 16 + 8}px` }} onClick={() => onSelect(node.id)}>
        {hasChildren ? <button onClick={e => { e.stopPropagation(); setExpanded(!expanded) }} className="text-gray-400 text-xs w-4">{expanded ? '▼' : '▶'}</button> : <span className="w-4" />}
        <span className={`flex-1 text-sm truncate ${isSelected ? 'font-medium' : ''}`}>{node.title}</span>
        <span className={`text-xs px-1 rounded ${statusColors[node.status] || ''}`}>{statusLabels[node.status] || node.status}</span>
        <button onClick={e => { e.stopPropagation(); onDelete(node.id) }} className="opacity-0 group-hover:opacity-100 text-red-400 text-xs">✕</button>
      </div>
      {expanded && hasChildren && node.children.map(child => <ChapterTreeNode key={child.id} node={child} selectedId={selectedId} onSelect={onSelect} onDelete={onDelete} depth={depth + 1} />)}
    </div>
  )
}

// ============ Chapter Editor (Middle) ============
function ChapterEditor({ chapter, content, onUpdate, bidId }: { chapter: ChapterSpec; content: ChapterContent | null; onUpdate: (data: any) => void; bidId: string }) {
  const [editing, setEditing] = useState(false); const [editTitle, setEditTitle] = useState(chapter.title)
  const [editingContent, setEditingContent] = useState(false); const [editContent, setEditContent] = useState('')

  const hasContent = content && content.content_text && content.content_text.length > 0

  return (
    <div className="flex flex-col h-full">
      <div className="px-4 py-3 border-b bg-white">
        {editing ? (
          <div className="flex gap-2">
            <input value={editTitle} onChange={e => setEditTitle(e.target.value)} className="flex-1 px-2 py-1 border rounded text-sm" autoFocus />
            <button onClick={() => { onUpdate({ title: editTitle }); setEditing(false) }} className="px-2 py-1 text-sm bg-blue-600 text-white rounded">保存</button>
            <button onClick={() => { setEditTitle(chapter.title); setEditing(false) }} className="px-2 py-1 text-sm border rounded">取消</button>
          </div>
        ) : (
          <div className="flex items-center justify-between">
            <h2 className="text-lg font-bold">{chapter.title}</h2>
            <button onClick={() => { setEditTitle(chapter.title); setEditing(true) }} className="text-sm text-gray-400 hover:text-blue-600">编辑标题</button>
          </div>
        )}
        <div className="flex gap-4 mt-1 text-xs text-gray-400">
          <span>层级: L{chapter.level}</span><span>目标: {chapter.target_word_count}字</span><span>最低: {chapter.min_word_count}字</span>
          <span>当前: <span className={content && content.word_count >= chapter.min_word_count ? 'text-green-600' : 'text-orange-500'}>{content?.word_count || 0}字</span></span>
          {content?.min_word_met === false && <span className="text-red-500">⚠ 未达最低字数</span>}
        </div>
      </div>

      <div className="flex-1 overflow-y-auto p-6">
        {!hasContent ? (
          <div className="text-center py-12 text-gray-400">
            <p className="text-4xl mb-3">📝</p><p className="text-lg mb-2">章节内容为空</p>
            <p className="text-sm">在右侧面板点击"生成内容"使用AI生成</p>
          </div>
        ) : editingContent ? (
          <div className="h-full flex flex-col">
            <textarea value={editContent} onChange={e => setEditContent(e.target.value)} className="flex-1 w-full p-4 border rounded text-sm resize-none focus:outline-none focus:ring-2 focus:ring-blue-400 font-mono" />
            <div className="flex gap-2 mt-2 justify-end">
              <button onClick={() => setEditingContent(false)} className="px-3 py-1 text-sm border rounded">取消</button>
              <button onClick={async () => { await bidsApi.saveChapterContent(bidId, chapter.id, editContent); setEditingContent(false) }} className="px-3 py-1 text-sm bg-blue-600 text-white rounded">保存内容</button>
            </div>
          </div>
        ) : (
          <div className="prose prose-sm max-w-none">
            <div className="flex justify-end mb-2">
              <button onClick={() => { setEditContent(content?.content_text || ''); setEditingContent(true) }} className="text-xs text-gray-400 hover:text-blue-600">✏ 编辑内容</button>
            </div>
            <div className="whitespace-pre-wrap leading-relaxed">{content?.content_text}</div>
          </div>
        )}
      </div>
    </div>
  )
}

// ============ Prompt Panel (Right) ============
function PromptPanel({ chapter, content, bidStatus, onGenerate, generating }: { chapter: ChapterSpec; content: ChapterContent | null; bidStatus: string; onGenerate: () => void; generating: boolean }) {
  const [prompt, setPrompt] = useState('')
  const canGenerate = bidStatus === 'facts' || bidStatus === 'generating' || chapter.status === 'planned' || chapter.status === 'failed'

  return (
    <div className="flex flex-col h-full overflow-y-auto p-4 space-y-4">
      <div>
        <h3 className="text-sm font-bold text-gray-700 mb-2">章节配置</h3>
        <div className="space-y-2 text-sm">
          <div><label className="text-gray-500">优先级</label><select defaultValue={chapter.priority} className="w-full px-2 py-1 border rounded text-sm"><option value="critical">关键</option><option value="high">高</option><option value="normal">普通</option><option value="low">低</option></select></div>
          <div><label className="text-gray-500">目标字数</label><input type="number" defaultValue={chapter.target_word_count} className="w-full px-2 py-1 border rounded text-sm" /></div>
          <div><label className="text-gray-500">写作风格</label><select defaultValue={chapter.writing_style} className="w-full px-2 py-1 border rounded text-sm"><option value="formal">正式</option><option value="concise">简洁</option><option value="detailed">详细</option></select></div>
        </div>
      </div>

      <div className="border-t pt-3">
        <h3 className="text-sm font-bold text-gray-700 mb-2">生成提示词</h3>
        <textarea value={prompt} onChange={e => setPrompt(e.target.value)} placeholder={`为"${chapter.title}"编写生成提示词...\n\n例如：\n- 重点突出技术优势\n- 包含项目实施时间表\n- 引用相关资质和案例`} className="w-full h-40 px-2 py-1 border rounded text-sm resize-none focus:outline-none focus:ring-1 focus:ring-blue-400" />
      </div>

      <div className="border-t pt-3">
        <h3 className="text-sm font-bold text-gray-700 mb-2">内容状态</h3>
        <div className="text-sm space-y-1">
          <div className="flex justify-between"><span className="text-gray-500">状态</span><span className={`px-2 rounded text-xs ${statusColors[chapter.status]}`}>{statusLabels[chapter.status] || chapter.status}</span></div>
          <div className="flex justify-between"><span className="text-gray-500">字数</span><span>{content?.word_count || 0} / {chapter.target_word_count}</span></div>
          {content?.llm_model && <div className="flex justify-between"><span className="text-gray-500">模型</span><span className="text-xs">{content.llm_model}</span></div>}
          {content?.generation_duration_ms ? <div className="flex justify-between"><span className="text-gray-500">耗时</span><span className="text-xs">{(content.generation_duration_ms / 1000).toFixed(1)}s</span></div> : null}
        </div>
      </div>

      <div className="border-t pt-3 space-y-2">
        {bidStatus === 'facts' && <div className="text-xs text-blue-600 bg-blue-50 p-2 rounded">💡 点击顶部"批量生成内容"可同时生成所有章节</div>}
        <button onClick={onGenerate} disabled={!canGenerate || generating} className="w-full px-3 py-2 text-sm bg-blue-600 text-white rounded hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed">
          {generating ? '提交中...' : chapter.status === 'succeeded' ? '🔄 重新生成' : chapter.status === 'failed' ? '🔄 重试生成' : '⚡ 生成此章节'}
        </button>
        {!canGenerate && <p className="text-xs text-gray-400 text-center">当前工作流状态不支持单章生成</p>}
      </div>
    </div>
  )
}

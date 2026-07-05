import { useState } from 'react'
import { Button, StatusBadge } from '../../components/ui'
import { ChapterSpec } from '../../api/bids'
import { buildTree, TreeNode, CHAPTER_STATUS_LABELS } from './workspace-helpers'

interface ChapterTreeProps {
  chapters: ChapterSpec[]
  selectedId: string | null
  onSelect: (id: string) => void
  onDelete: (id: string) => void
  onAdd: (data: { title: string; level: number; parent_id?: string }) => void
  adding: boolean
}

export function ChapterTree({ chapters, selectedId, onSelect, onDelete, onAdd, adding }: ChapterTreeProps) {
  const [showAdd, setShowAdd] = useState(false)
  const [title, setTitle] = useState('')
  const [level, setLevel] = useState(1)
  const [parentId, setParentId] = useState('')

  const tree = buildTree(chapters)
  const parents = chapters.filter(c => c.level < 3)

  const submit = () => {
    if (!title.trim()) return
    onAdd({ title: title.trim(), level, parent_id: parentId || undefined })
    setTitle('')
    setShowAdd(false)
  }

  return (
    <div className="flex flex-col h-full">
      {/* Header */}
      <div className="flex items-center justify-between px-3 py-2.5 border-b border-ink-100 bg-white shrink-0">
        <span className="text-sm font-semibold text-ink-800 inline-flex items-center gap-1.5">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <line x1="8" y1="6" x2="21" y2="6" />
            <line x1="8" y1="12" x2="21" y2="12" />
            <line x1="8" y1="18" x2="21" y2="18" />
            <line x1="3" y1="6" x2="3.01" y2="6" />
            <line x1="3" y1="12" x2="3.01" y2="12" />
            <line x1="3" y1="18" x2="3.01" y2="18" />
          </svg>
          章节目录
          <span className="text-xs text-ink-400 font-normal">({chapters.length})</span>
        </span>
        <button
          onClick={() => setShowAdd(!showAdd)}
          className="inline-flex items-center gap-0.5 text-xs font-medium text-brand-600 hover:text-brand-700 px-2 py-1 rounded hover:bg-brand-50 transition-colors"
        >
          {showAdd ? '取消' : (
            <>
              <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round">
                <line x1="12" y1="5" x2="12" y2="19" />
                <line x1="5" y1="12" x2="19" y2="12" />
              </svg>
              添加
            </>
          )}
        </button>
      </div>

      {/* Add form */}
      {showAdd && (
        <div className="px-3 py-3 bg-brand-50/60 border-b border-brand-100 space-y-2 animate-slide-down">
          <input
            type="text"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            onKeyDown={(e) => { if (e.key === 'Enter') submit(); if (e.key === 'Escape') setShowAdd(false) }}
            placeholder="章节标题"
            className="w-full px-2.5 py-1.5 text-sm border border-ink-200 rounded-lg bg-white focus:outline-none focus:border-brand-400 focus:ring-2 focus:ring-brand-100"
            autoFocus
          />
          <div className="grid grid-cols-2 gap-2">
            <select
              value={level}
              onChange={(e) => setLevel(Number(e.target.value))}
              className="px-2 py-1.5 text-xs border border-ink-200 rounded-lg bg-white focus:outline-none focus:border-brand-400"
            >
              <option value={1}>一级章节</option>
              <option value={2}>二级章节</option>
              <option value={3}>三级章节</option>
            </select>
            <select
              value={parentId}
              onChange={(e) => setParentId(e.target.value)}
              className="px-2 py-1.5 text-xs border border-ink-200 rounded-lg bg-white focus:outline-none focus:border-brand-400 truncate"
            >
              <option value="">无父级</option>
              {parents.map(c => <option key={c.id} value={c.id}>{c.title}</option>)}
            </select>
          </div>
          <Button
            variant="primary"
            size="sm"
            className="w-full"
            onClick={submit}
            disabled={!title.trim() || adding}
            loading={adding}
          >
            添加章节
          </Button>
        </div>
      )}

      {/* Tree */}
      <div className="flex-1 overflow-y-auto scrollbar-thin py-1.5">
        {chapters.length === 0 ? (
          <div className="px-4 py-8 text-center">
            <div className="text-2xl mb-1">📑</div>
            <p className="text-xs text-ink-400">还没有章节<br/>点击上方"添加"创建</p>
          </div>
        ) : (
          tree.map(node => (
            <TreeRow
              key={node.id}
              node={node}
              selectedId={selectedId}
              onSelect={onSelect}
              onDelete={onDelete}
            />
          ))
        )}
      </div>
    </div>
  )
}

function TreeRow({ node, selectedId, onSelect, onDelete, depth = 0 }: {
  node: TreeNode
  selectedId: string | null
  onSelect: (id: string) => void
  onDelete: (id: string) => void
  depth?: number
}) {
  const isSelected = node.id === selectedId
  const hasChildren = node.children.length > 0
  const [expanded, setExpanded] = useState(true)

  return (
    <div>
      <div
        className={[
          'group relative flex items-center gap-1.5 py-1.5 cursor-pointer transition-colors',
          isSelected
            ? 'bg-brand-50 before:absolute before:left-0 before:top-1 before:bottom-1 before:w-0.5 before:bg-brand-600'
            : 'hover:bg-ink-50',
        ].join(' ')}
        style={{ paddingLeft: `${depth * 14 + 8}px`, paddingRight: 8 }}
        onClick={() => onSelect(node.id)}
      >
        <button
          onClick={(e) => { e.stopPropagation(); if (hasChildren) setExpanded(!expanded) }}
          className={`shrink-0 w-4 h-4 flex items-center justify-center text-ink-400 ${hasChildren ? 'hover:text-ink-700' : 'invisible'}`}
        >
          <svg width="9" height="9" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round"
            style={{ transform: expanded ? 'rotate(90deg)' : 'rotate(0)', transition: 'transform 150ms' }}
          >
            <polyline points="9 18 15 12 9 6" />
          </svg>
        </button>
        <span className={[
          'flex-1 text-sm truncate',
          isSelected ? 'font-semibold text-brand-700' : 'text-ink-800',
        ].join(' ')}>
          {node.title}
        </span>
        <StatusBadge status={node.status} labels={CHAPTER_STATUS_LABELS} showDot={false} />
        <button
          onClick={(e) => { e.stopPropagation(); if (confirm(`确定删除「${node.title}」？子章节也会一并删除。`)) onDelete(node.id) }}
          className="shrink-0 opacity-0 group-hover:opacity-100 text-ink-400 hover:text-red-500 transition-all p-1 rounded hover:bg-red-50"
          aria-label="删除"
        >
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <polyline points="3 6 5 6 21 6" />
            <path d="M19 6l-2 14a2 2 0 0 1-2 2H9a2 2 0 0 1-2-2L5 6" />
            <path d="M10 11v6" />
            <path d="M14 11v6" />
          </svg>
        </button>
      </div>
      {expanded && hasChildren && node.children.map(child => (
        <TreeRow
          key={child.id}
          node={child}
          selectedId={selectedId}
          onSelect={onSelect}
          onDelete={onDelete}
          depth={depth + 1}
        />
      ))}
    </div>
  )
}
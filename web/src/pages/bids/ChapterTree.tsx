import { useState, useRef } from 'react'
import {
  DndContext, DragEndEvent, DragOverlay, DragStartEvent, PointerSensor,
  useSensor, useSensors, closestCenter,
} from '@dnd-kit/core'
import {
  arrayMove, SortableContext, useSortable, verticalListSortingStrategy,
} from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import { Button } from '../../components/ui'
import { ChapterSpec } from '../../api/bids'
import { buildTree, TreeNode } from './workspace-helpers'

interface ChapterTreeProps {
  chapters: ChapterSpec[]
  selectedId: string | null
  onSelect: (id: string) => void
  onDelete: (id: string) => void
  onAdd: (data: { title: string; level: number; parent_id?: string }) => void
  onReorder?: (ordered: Array<{ id: string; parent_id?: string | null }>) => Promise<unknown>
  adding: boolean
}

export function ChapterTree({ chapters, selectedId, onSelect, onDelete, onAdd, onReorder, adding }: ChapterTreeProps) {
  const [showAdd, setShowAdd] = useState(false)
  const [title, setTitle] = useState('')
  const [level, setLevel] = useState(1)
  const [parentId, setParentId] = useState('')

  const tree = buildTree(chapters)
  const parents = chapters.filter(c => c.level < 3)

  // Flatten the tree for SortableContext (preserving parent→child
  // grouping via depth). dnd-kit's arrayMove operates on flat arrays
  // so we re-build a chapter→order_index map after every drag-end.
  const flatRows: { node: TreeNode; depth: number }[] = []
  function flatten(nodes: TreeNode[], depth: number) {
    nodes.forEach(n => { flatRows.push({ node: n, depth }); flatten(n.children, depth + 1) })
  }
  flatten(tree, 0)
  const flatIds = flatRows.map(r => r.node.id)

  // Drag state — track the dragged chapter so we can show a drag
  // overlay (DragOverlay) with full styling instead of relying on the
  // dragging item's own box, which gets clipped by overflow:hidden on
  // its parent tree container.
  const [activeDragId, setActiveDragId] = useState<string | null>(null)
  const reorderingRef = useRef(false)

  // PointerSensor with a 5px activation distance — avoids accidental
  // drags when the user is just trying to click to select a chapter.
  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 5 } }),
  )

  // Optimistic local mirror so the dragged item visibly moves before
  // the server round-trip completes. We use a ref (not state) so the
  // tree doesn't re-render on every move tick — SortableContext handles
  // its own visual via CSS.Transform.
  const [, setLocalMirror] = useState<string[] | null>(null)

  const handleDragStart = (e: DragStartEvent) => {
    setActiveDragId(String(e.active.id))
  }

  const handleDragEnd = async (e: DragEndEvent) => {
    setActiveDragId(null)
    const { active, over } = e
    if (!over || active.id === over.id) return
    const oldIdx = flatIds.indexOf(String(active.id))
    const newIdx = flatIds.indexOf(String(over.id))
    if (oldIdx < 0 || newIdx < 0) return

    // Reorder the flat list, then re-derive parent_id for each row
    // from the resulting ordering (a chapter's parent is the closest
    // preceding row whose level is exactly one less). This keeps the
    // shape flat-arrays-friendly while still respecting tree semantics.
    const reordered = arrayMove(flatRows, oldIdx, newIdx)
    const ordered: Array<{ id: string; parent_id?: string | null }> = []
    // Walk the reordered list and assign parent_id based on nearest
    // preceding row with level === current.level - 1.
    const levelToLastId: Record<number, string | undefined> = {}
    for (const row of reordered) {
      const targetLevel = row.node.level - 1
      const parent = targetLevel >= 1 ? levelToLastId[targetLevel] : undefined
      ordered.push({ id: row.node.id, parent_id: parent ?? null })
      levelToLastId[row.node.level] = row.node.id
    }

    // Optimistic mirror — fire a re-render with the new ordering so the
    // tree doesn't snap back momentarily before the server round-trip.
    setLocalMirror(ordered.map(o => o.id))
    reorderingRef.current = true

    if (onReorder) {
      try {
        await onReorder(ordered)
      } catch {
        // On error, drop the mirror and let React Query's refetch restore.
      }
    }
    // Clear mirror after a tick so React Query's response can take over
    setTimeout(() => { setLocalMirror(null); reorderingRef.current = false }, 50)
  }

  // Summary stats for the header
  const total = chapters.length
  const doneCount = chapters.filter(c => c.status === 'succeeded').length
  const runningCount = chapters.filter(c => c.status === 'running' || c.status === 'pending').length
  const failedCount = chapters.filter(c => c.status === 'failed').length

  const submit = () => {
    if (!title.trim()) return
    onAdd({ title: title.trim(), level, parent_id: parentId || undefined })
    setTitle('')
    setShowAdd(false)
  }

  return (
    <div className="flex flex-col h-full">
      {/* Header */}
      <div className="flex items-center justify-between px-3 py-2.5 border-b border-ink-100 dark:border-ink-700 bg-white dark:bg-ink-800 shrink-0">
        <span className="text-sm font-semibold text-ink-800 dark:text-ink-100 inline-flex items-center gap-1.5">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <line x1="8" y1="6" x2="21" y2="6" />
            <line x1="8" y1="12" x2="21" y2="12" />
            <line x1="8" y1="18" x2="21" y2="18" />
            <line x1="3" y1="6" x2="3.01" y2="6" />
            <line x1="3" y1="12" x2="3.01" y2="12" />
            <line x1="3" y1="18" x2="3.01" y2="18" />
          </svg>
          章节目录
          <span className="text-xs text-ink-400 dark:text-ink-500 font-normal">({total})</span>
        </span>
        <button
          onClick={() => setShowAdd(!showAdd)}
          className="inline-flex items-center gap-0.5 text-xs font-medium text-brand-600 dark:text-brand-400 hover:text-brand-700 dark:hover:text-brand-300 px-2 py-1 rounded hover:bg-brand-50 dark:hover:bg-brand-900/40 transition-colors"
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

      {/* Mini status bar */}
      {total > 0 && (
        <div className="px-3 py-2 border-b border-ink-100 dark:border-ink-700 bg-ink-50 dark:bg-ink-900/30 flex items-center gap-2 text-[11px]">
          {doneCount > 0 && (
            <span className="inline-flex items-center gap-1 text-emerald-600 dark:text-emerald-400">
              <span className="w-1.5 h-1.5 rounded-full bg-emerald-500" />
              {doneCount} 完成
            </span>
          )}
          {runningCount > 0 && (
            <span className="inline-flex items-center gap-1 text-brand-600 dark:text-brand-400">
              <span className="relative flex w-2 h-2">
                <span className="absolute inline-flex w-full h-full rounded-full bg-brand-400 opacity-75 animate-ping-slow" />
                <span className="relative inline-flex rounded-full w-2 h-2 bg-brand-500" />
              </span>
              {runningCount} 进行中
            </span>
          )}
          {failedCount > 0 && (
            <span className="inline-flex items-center gap-1 text-red-600 dark:text-red-400">
              <span className="w-1.5 h-1.5 rounded-full bg-red-500" />
              {failedCount} 失败
            </span>
          )}
          <span className="ml-auto text-ink-400 dark:text-ink-500 tabular-nums">
            {Math.round((doneCount / total) * 100)}%
          </span>
        </div>
      )}

      {/* Add form */}
      {showAdd && (
        <div className="px-3 py-3 bg-brand-50/60 dark:bg-brand-900/20 border-b border-brand-100 dark:border-brand-900/40 space-y-2 animate-slide-down">
          <input
            type="text"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            onKeyDown={(e) => { if (e.key === 'Enter') submit(); if (e.key === 'Escape') setShowAdd(false) }}
            placeholder="章节标题"
            className="w-full px-2.5 py-1.5 text-sm border border-ink-200 dark:border-ink-700 rounded-lg bg-white dark:bg-ink-800 text-ink-900 dark:text-white focus:outline-none focus:border-brand-400 focus:ring-2 focus:ring-brand-100 dark:focus:ring-brand-900"
            autoFocus
          />
          <div className="grid grid-cols-2 gap-2">
            <select
              value={level}
              onChange={(e) => setLevel(Number(e.target.value))}
              className="px-2 py-1.5 text-xs border border-ink-200 dark:border-ink-700 rounded-lg bg-white dark:bg-ink-800 text-ink-700 dark:text-ink-200 focus:outline-none focus:border-brand-400"
            >
              <option value={1}>一级章节</option>
              <option value={2}>二级章节</option>
              <option value={3}>三级章节</option>
            </select>
            <select
              value={parentId}
              onChange={(e) => setParentId(e.target.value)}
              className="px-2 py-1.5 text-xs border border-ink-200 dark:border-ink-700 rounded-lg bg-white dark:bg-ink-800 text-ink-700 dark:text-ink-200 focus:outline-none focus:border-brand-400 truncate"
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
            <div className="mx-auto w-12 h-12 rounded-xl bg-brand-50 dark:bg-brand-900/30 grid place-items-center text-2xl mb-2">📑</div>
            <p className="text-xs text-ink-500 dark:text-ink-400 font-medium mb-1">还没有章节</p>
            <p className="text-[10px] text-ink-400 dark:text-ink-500">点击右上角"添加"创建第一个章节</p>
          </div>
        ) : (
          <DndContext
            sensors={sensors}
            collisionDetection={closestCenter}
            onDragStart={handleDragStart}
            onDragEnd={handleDragEnd}
          >
            <SortableContext items={flatIds} strategy={verticalListSortingStrategy}>
              {flatRows.map(row => (
                <SortableTreeRow
                  key={row.node.id}
                  row={row}
                  selectedId={selectedId}
                  onSelect={onSelect}
                  onDelete={onDelete}
                />
              ))}
            </SortableContext>
            {/* DragOverlay renders the dragged item as a portal-positioned
                ghost, immune to overflow:hidden clipping on the tree. */}
            <DragOverlay>
              {activeDragId ? (() => {
                const active = flatRows.find(r => r.node.id === activeDragId)
                if (!active) return null
                return (
                  <div className="rounded-md bg-white dark:bg-ink-800 shadow-pop border border-brand-300 dark:border-brand-700 px-3 py-2 text-sm text-ink-800 dark:text-ink-100 max-w-xs">
                    <span className="inline-flex items-center gap-1.5">
                      <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="text-ink-400">
                        <line x1="8" y1="6" x2="21" y2="6" /><line x1="8" y1="12" x2="21" y2="12" /><line x1="8" y1="18" x2="21" y2="18" /><line x1="3" y1="6" x2="3.01" y2="6" /><line x1="3" y1="12" x2="3.01" y2="12" /><line x1="3" y1="18" x2="3.01" y2="18" />
                      </svg>
                      {active.node.title}
                    </span>
                  </div>
                )
              })() : null}
            </DragOverlay>
          </DndContext>
        )}
      </div>
    </div>
  )
}

// Sortable wrapper — turns a TreeRow into a draggable item. Applies the
// CSS.Transform that dnd-kit computes while dragging so the row follows
// the cursor. We use transition: none during active drag to keep the
// ghost snappy (dnd-kit handles the animation back to final position).
function SortableTreeRow({ row, selectedId, onSelect, onDelete }: {
  row: { node: TreeNode; depth: number }
  selectedId: string | null
  onSelect: (id: string) => void
  onDelete: (id: string) => void
}) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id: row.node.id,
  })
  return (
    <div
      ref={setNodeRef}
      style={{
        transform: CSS.Transform.toString(transform),
        transition,
        opacity: isDragging ? 0.35 : 1,
      }}
    >
      <TreeRow
        node={row.node}
        selectedId={selectedId}
        onSelect={onSelect}
        onDelete={onDelete}
        depth={row.depth}
        dragHandleProps={{ ...attributes, ...listeners }}
      />
    </div>
  )
}

function TreeRow({ node, selectedId, onSelect, onDelete, depth = 0, dragHandleProps }: {
  node: TreeNode
  selectedId: string | null
  onSelect: (id: string) => void
  onDelete: (id: string) => void
  depth?: number
  dragHandleProps?: React.HTMLAttributes<HTMLButtonElement>
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
            ? 'bg-brand-50 dark:bg-brand-900/30 before:absolute before:left-0 before:top-1 before:bottom-1 before:w-0.5 before:bg-brand-600'
            : 'hover:bg-ink-50 dark:hover:bg-ink-800/50',
        ].join(' ')}
        style={{ paddingLeft: `${depth * 14 + 8}px`, paddingRight: 8 }}
        onClick={() => onSelect(node.id)}
      >
        {/* Drag handle — only visible on hover so it doesn't clutter the
            tree. Hidden on touch devices since dnd-kit's PointerSensor
            handles long-press automatically. */}
        {dragHandleProps && (
          <button
            {...dragHandleProps}
            aria-label="拖动重排"
            title="拖动重排"
            onClick={(e) => e.stopPropagation()}
            className="shrink-0 w-4 h-4 flex items-center justify-center text-ink-300 dark:text-ink-600 opacity-0 group-hover:opacity-100 cursor-grab active:cursor-grabbing hover:text-ink-600 dark:hover:text-ink-300 transition-all touch-none"
          >
            <svg width="10" height="10" viewBox="0 0 24 24" fill="currentColor">
              <circle cx="9" cy="6" r="1.5" /><circle cx="9" cy="12" r="1.5" /><circle cx="9" cy="18" r="1.5" />
              <circle cx="15" cy="6" r="1.5" /><circle cx="15" cy="12" r="1.5" /><circle cx="15" cy="18" r="1.5" />
            </svg>
          </button>
        )}
        <button
          onClick={(e) => { e.stopPropagation(); if (hasChildren) setExpanded(!expanded) }}
          className={`shrink-0 w-4 h-4 flex items-center justify-center text-ink-400 dark:text-ink-500 ${hasChildren ? 'hover:text-ink-700 dark:hover:text-ink-200' : 'invisible'}`}
        >
          <svg width="9" height="9" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round"
            style={{ transform: expanded ? 'rotate(90deg)' : 'rotate(0)', transition: 'transform 150ms' }}
          >
            <polyline points="9 18 15 12 9 6" />
          </svg>
        </button>
        {/* status dot */}
        <span className="shrink-0">
          <NodeStatusDot status={node.status} />
        </span>
        <span className={[
          'flex-1 text-sm truncate inline-flex items-center gap-1.5',
          isSelected ? 'font-semibold text-brand-700 dark:text-brand-300' : 'text-ink-800 dark:text-ink-100',
        ].join(' ')}>
          <span className="truncate">{node.title}</span>
          {/* "已审" badge — visually distinguishes human-approved chapters
              from raw "succeeded" AI output. Important for HIL workflow. */}
          {node.status === 'approved' && (
            <span className="shrink-0 inline-flex items-center gap-0.5 px-1.5 py-0.5 rounded text-[10px] font-medium bg-emerald-50 dark:bg-emerald-900/30 text-emerald-700 dark:text-emerald-300 border border-emerald-200 dark:border-emerald-800/50">
              <svg width="9" height="9" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round">
                <polyline points="20 6 9 17 4 12" />
              </svg>
              已审
            </span>
          )}
          {/* "驳回" badge — only when rejection_reason is still on the
              chapter. After re-generation it gets cleared, which is
              the right signal that the rejection has been addressed. */}
          {node.rejection_reason && (
            <span
              className="shrink-0 inline-flex items-center gap-0.5 px-1.5 py-0.5 rounded text-[10px] font-medium bg-red-50 dark:bg-red-900/30 text-red-700 dark:text-red-300 border border-red-200 dark:border-red-800/50"
              title="点击查看驳回原因"
            >
              <svg width="9" height="9" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
                <circle cx="12" cy="12" r="10" />
                <line x1="15" y1="9" x2="9" y2="15" />
                <line x1="9" y1="9" x2="15" y2="15" />
              </svg>
              驳回
            </span>
          )}
        </span>
        {node.status === 'succeeded' && (
          <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round" className="shrink-0 text-emerald-500">
            <polyline points="20 6 9 17 4 12" />
          </svg>
        )}
        {/* Rejection hover-tooltip anchor. CSS-only (no JS state)
            because the data is fully static once chapters are
            fetched. Hover & focus-within to keep keyboard users in
            the loop. Position: absolute, top=0, right=8, width 240px.
            The anchor is a span so it doesn't disrupt the flex flow
            used by the rest of the row. Click navigates selection
            (handled by the row click); the popover just exposes the
            previously-hidden "why was this rejected" reason. */}
        {node.rejection_reason && (
          <span
            tabIndex={0}
            className="absolute top-full mt-1 right-2 z-30 hidden group-hover:flex group-focus-within:flex flex-col w-60 p-2.5 rounded-md shadow-lg border border-red-200 dark:border-red-800/60 bg-white dark:bg-ink-900 text-left animate-slide-down"
            role="tooltip"
          >
            <span className="text-[10px] font-bold uppercase tracking-wider text-red-600 dark:text-red-400 mb-1">驳回原因</span>
            <span className="text-[11px] leading-relaxed text-ink-700 dark:text-ink-200 line-clamp-4">{node.rejection_reason}</span>
            <span className="mt-1.5 text-[9px] text-ink-400 dark:text-ink-500">点击章节查看完整历史</span>
          </span>
        )}
        <button
          onClick={(e) => { e.stopPropagation(); if (confirm(`确定删除「${node.title}」？子章节也会一并删除。`)) onDelete(node.id) }}
          className="shrink-0 opacity-0 group-hover:opacity-100 text-ink-400 hover:text-red-500 transition-all p-1 rounded hover:bg-red-50 dark:hover:bg-red-900/30"
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

function NodeStatusDot({ status }: { status: string }) {
  const map: Record<string, string> = {
    planned:    'bg-ink-300 dark:bg-ink-600',
    pending:    'bg-amber-500',
    running:    'bg-brand-500 animate-pulse-soft',
    succeeded:  'bg-emerald-500',
    approved:   'bg-emerald-600 ring-2 ring-emerald-200 dark:ring-emerald-800',
    failed:     'bg-red-500',
    skipped:    'bg-ink-300 dark:bg-ink-600',
  }
  const cls = map[status] ?? 'bg-ink-300'
  if (status === 'running') {
    return (
      <span className="relative flex w-2 h-2">
        <span className="absolute inline-flex w-full h-full rounded-full bg-brand-400 opacity-75 animate-ping-slow" />
        <span className={`relative inline-flex rounded-full w-2 h-2 ${cls}`} />
      </span>
    )
  }
  if (status === 'approved') {
    // Approved chapters get a distinctive treatment — emerald with a
    // ring. Inline tooltip via title because the dot is too small for
    // text. The "已审" badge in the row text (added separately) is the
    // primary indicator; this just makes the dot more noticeable.
    return <span className={`inline-block w-2 h-2 rounded-full ${cls}`} title="已审核" />
  }
  return <span className={`inline-block w-2 h-2 rounded-full ${cls}`} />
}
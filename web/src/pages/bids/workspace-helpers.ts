import { ChapterSpec } from '../../api/bids'

export const CHAPTER_STATUS_LABELS: Record<string, string> = {
  planned: '待生成', pending: '等待中', running: '生成中',
  succeeded: '已生成', failed: '失败', skipped: '跳过',
}

export const BID_STATUS_LABELS: Record<string, string> = {
  pending: '等待中', parsing: '解析中', outlining: '生成大纲',
  generating: '生成内容', awaiting_review: '等待审核',
  auditing: '审计中', exporting: '导出中',
  done: '已完成', failed: '失败', paused: '已暂停', facts: '审查中',
}

// 工作流阶段顺序（用于顶栏 stepper）。在生成内容之后、审计之前加
// 一个 "awaiting_review" 暂停点：让用户先人眼检查初稿质量，
// 不通过审核就不进入审计阶段。这是"人在回路 (HIL)"原则的实际落地 —
// 自动流程不应在用户没看过内容的情况下进入下一步。
export const WORKFLOW_STEPS: { id: string; label: string }[] = [
  { id: 'pending',          label: '初始化' },
  { id: 'parsing',          label: '解析材料' },
  { id: 'outlining',        label: '生成大纲' },
  { id: 'facts',            label: '证据检索' },
  { id: 'generating',       label: '生成内容' },
  { id: 'awaiting_review',  label: '等待审核' },
  { id: 'auditing',         label: '审计' },
  { id: 'done',             label: '完成' },
]

export function workflowStepIndex(status: string): number {
  const i = WORKFLOW_STEPS.findIndex(s => s.id === status)
  return i === -1 ? 0 : i
}

// ============ Tree ============
export interface TreeNode extends ChapterSpec { children: TreeNode[] }

export function buildTree(chapters: ChapterSpec[]): TreeNode[] {
  const map = new Map<string, TreeNode>()
  const roots: TreeNode[] = []
  chapters.forEach(c => map.set(c.id, { ...c, children: [] }))
  chapters.forEach(c => {
    const node = map.get(c.id)!
    if (c.parent_id && map.has(c.parent_id)) {
      map.get(c.parent_id)!.children.push(node)
    } else {
      roots.push(node)
    }
  })
  roots.sort((a, b) => a.order_index - b.order_index)
  return roots
}
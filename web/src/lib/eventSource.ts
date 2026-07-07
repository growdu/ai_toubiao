/**
 * Event source adapters — convert domain objects (chapters, bids) into
 * EventLog entries for the timeline UI. The UI only knows about EventLog;
 * the adapter layer owns the knowledge of what a "rejected" chapter
 * looks like and how to render it.
 *
 * Today these adapters work entirely off the in-memory chapter state
 * (no fetches). When the backend lands an event log endpoint, swap
 * these adapters for direct fetches — UI doesn't change.
 */
import type { EventActor, EventLog, EventLogSource } from './eventLog'

const ACTOR_SYSTEM: EventActor = { id: 'system', name: '系统', role: 'system' }
const ACTOR_LLM: EventActor = { id: 'llm', name: 'AI', role: 'llm' }

/**
 * Build a chapter-scoped event log from the chapter's latest state.
 * Used by ChapterInspector to render its inline timeline without
 * any backend calls.
 *
 * Two events are produced when present:
 *   - approved: when chapter.approved_at && !chapter.rejection_reason
 *   - rejected: when chapter.rejection_reason is non-empty
 *   - regenerated: never emitted today — kept in contract for future
 *     when we store a generation history table.
 *
 * Output is sorted newest-first.
 */
export function chapterToEventLog(chapter: {
  id: string
  bid_id?: string
  approved_at?: string | null
  approved_by?: string | null
  rejection_reason?: string | null
  updated_at?: string
  created_at?: string
}): EventLog[] {
  const out: EventLog[] = []
  const bidId = chapter.bid_id || 'self'

  if (chapter.rejection_reason) {
    out.push({
      id: `${chapter.id}:rejected`,
      bidId,
      chapterId: chapter.id,
      kind: 'rejected',
      label: '驳回',
      description: chapter.rejection_reason,
      actor: chapter.approved_by
        ? { id: chapter.approved_by, name: chapter.approved_by, role: 'reviewer' }
        : ACTOR_SYSTEM,
      at: chapter.updated_at || chapter.created_at || new Date().toISOString(),
      meta: { reason: chapter.rejection_reason },
    })
  }
  if (chapter.approved_at) {
    out.push({
      id: `${chapter.id}:approved`,
      bidId,
      chapterId: chapter.id,
      kind: 'approved',
      label: '已批准',
      description: chapter.approved_by ? `由 ${chapter.approved_by} 批准` : '已批准',
      actor: chapter.approved_by
        ? { id: chapter.approved_by, name: chapter.approved_by, role: 'reviewer' }
        : ACTOR_SYSTEM,
      at: chapter.approved_at,
    })
  }

  return out.sort((a, b) => b.at.localeCompare(a.at))
}

/**
 * HTTP source — placeholder. Wire up when /bids/:id/events lands.
 * Today this throws so any code path that accidentally hits it
 * fails fast instead of silently using a mock.
 */
export const HttpEventLogSource: EventLogSource = {
  async list() {
    throw new Error('HttpEventLogSource not implemented yet — falling back to a chapter-derived source')
  },
}

export { ACTOR_LLM, ACTOR_SYSTEM }

/**
 * Tests for the event-source adapters. We mock the current chapter's
 * latest state into the adapter and confirm:
 *   - An "rejected" entry is produced whenever rejection_reason is set
 *   - An "approved" entry is produced whenever approved_at is set
 *   - Both can co-exist (and the rejection sorts above the approval
 *     because we treat rejection_reason as the latest signal)
 *   - When neither is set, the adapter returns [] (not a placeholder —
 *     the UI will render its own empty state)
 *   - The actor defaults to "system" when no approved_by is recorded;
 *     with approved_by, actor is a reviewer with that id as name
 *   - Timestamps fall back through updated_at → created_at → now
 *   - Output is sorted newest-first by `at`
 */
import { describe, it, expect } from 'vitest'
import { chapterToEventLog } from './eventSource'
import type { EventLog } from './eventLog'

describe('chapterToEventLog', () => {
  it('returns [] when no audit fields are set', () => {
    const out = chapterToEventLog({ id: 'ch1' })
    expect(out).toEqual([])
  })

  it('treats whitespace-only rejection_reason as a present signal (truthy)', () => {
    // The adapter's contract is `if (chapter.rejection_reason)` — a
    // whitespace string IS truthy. We don't auto-trim because the
    // textual content the user wrote matters less than whether the
    // audit field was touched at all. UI is responsible for blanking
    // the field if no reason was actually provided. This test
    // documents that the adapter is deliberately permissive here.
    const out = chapterToEventLog({ id: 'ch1', rejection_reason: '   ' })
    expect(out).toHaveLength(1)
    expect(out[0].kind).toBe('rejected')
  })

  it('produces a rejected entry when rejection_reason is present', () => {
    const out = chapterToEventLog({
      id: 'ch-42',
      bid_id: 'bid-1',
      rejection_reason: '内容不准确',
      approved_by: 'reviewer-x',
      updated_at: '2026-07-01T12:00:00Z',
    })
    expect(out).toHaveLength(1)
    const e = out[0]
    expect(e.kind).toBe('rejected')
    expect(e.label).toBe('驳回')
    expect(e.description).toBe('内容不准确')
    expect(e.chapterId).toBe('ch-42')
    expect(e.bidId).toBe('bid-1')
    expect(e.actor.id).toBe('reviewer-x')
    expect(e.actor.role).toBe('reviewer')
    expect(e.meta?.reason).toBe('内容不准确')
    expect(e.at).toBe('2026-07-01T12:00:00Z')
    expect(e.id).toBe('ch-42:rejected')
  })

  it('produces an approved entry when approved_at is present', () => {
    const out = chapterToEventLog({
      id: 'ch-7',
      bid_id: 'bid-2',
      approved_at: '2026-06-15T09:30:00Z',
      approved_by: 'reviewer-y',
    })
    expect(out).toHaveLength(1)
    expect(out[0].kind).toBe('approved')
    expect(out[0].label).toBe('已批准')
    expect(out[0].description).toContain('reviewer-y')
    expect(out[0].actor.id).toBe('reviewer-y')
    expect(out[0].actor.role).toBe('reviewer')
    expect(out[0].at).toBe('2026-06-15T09:30:00Z')
  })

  it('produces both rejected + approved when both are set', () => {
    const out = chapterToEventLog({
      id: 'ch-100',
      bid_id: 'bid-3',
      rejection_reason: '需要修改',
      approved_at: '2026-05-01T08:00:00Z',
      approved_by: 'r',
    })
    expect(out).toHaveLength(2)
    expect(out.map(e => e.kind).sort()).toEqual(['approved', 'rejected'])
  })

  it('falls back actor to "系统" when approved_by is missing', () => {
    const out = chapterToEventLog({
      id: 'ch-1',
      approved_at: '2026-04-01T00:00:00Z',
    })
    expect(out[0].actor.id).toBe('system')
    expect(out[0].actor.role).toBe('system')
    expect(out[0].actor.name).toBe('系统')
  })

  it('falls back actor to "系统" on rejection when no approved_by', () => {
    const out = chapterToEventLog({
      id: 'ch-2',
      rejection_reason: '原因',
      // no approved_by
    })
    expect(out[0].actor.role).toBe('system')
  })

  it('falls back timestamp: updated_at → created_at → ISO now (most-recent first)', () => {
    const out = chapterToEventLog({
      id: 'ch-x',
      rejection_reason: 'X',
      created_at: '2025-01-01T00:00:00Z',
    })
    expect(out[0].at).toBe('2025-01-01T00:00:00Z')
  })

  it('orders newest-first when both rejected + approved are present', () => {
    const out = chapterToEventLog({
      id: 'ch-multi',
      rejection_reason: 'recently rejected',
      // updated_at is more recent than approved_at
      approved_at: '2026-01-01T00:00:00Z',
      updated_at: '2026-06-01T00:00:00Z',
    })
    expect(out).toHaveLength(2)
    // The rejection is the "newer" event because it's tied to updated_at
    expect(out[0].kind).toBe('rejected')
    expect(out[1].kind).toBe('approved')
    expect(out[0].at >= out[1].at).toBe(true)
  })

  it('handles missing bid_id by defaulting to "self"', () => {
    const out = chapterToEventLog({
      id: 'ch-no-bid',
      approved_at: '2026-01-01T00:00:00Z',
    })
    expect(out[0].bidId).toBe('self')
  })

  it('ids are deterministic so React can use them as keys', () => {
    // Same chapter + same state → same event id (so React reconciliation
    // doesn't unmount/remount when the chapter re-renders).
    const a = chapterToEventLog({ id: 'ch-d', rejection_reason: 'x' })
    const b = chapterToEventLog({ id: 'ch-d', rejection_reason: 'x' })
    expect(a[0].id).toBe(b[0].id)
    expect(a[0].id).toBe('ch-d:rejected')
  })

  it('ids are stable across the two event kinds', () => {
    const out = chapterToEventLog({
      id: 'ch-shared',
      rejection_reason: 'r',
      approved_at: '2026-01-01T00:00:00Z',
    })
    const ids = out.map(e => e.id).sort()
    expect(ids).toEqual(['ch-shared:approved', 'ch-shared:rejected'])
  })

  it('EventLog shape is preserved (catches accidental field drift)', () => {
    // This is a structural test — protects the public EventLog
    // contract from accidental field additions/removals.
    const out: EventLog[] = chapterToEventLog({
      id: 'ch-shape',
      approved_at: '2026-01-01T00:00:00Z',
    })
    expect(Object.keys(out[0]).sort()).toEqual(
      ['actor', 'at', 'bidId', 'chapterId', 'description', 'id', 'kind', 'label'].sort()
    )
  })
})

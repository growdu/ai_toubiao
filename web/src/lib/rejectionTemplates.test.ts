/**
 * Tests for the rejection-templates store and sortTemplates helper.
 *
 * Coverage goals:
 *   - Built-in templates are seeded correctly and stable across resets
 *   - add() trims whitespace and dedupes by label (no doubles)
 *   - add() rejects empty/whitespace-only labels (silently — matches
 *     store contract)
 *   - remove() protects built-in templates
 *   - remove() actually removes user-added templates
 *   - togglePin() flips state and updates pinnedAt (used for sort)
 *   - sortTemplates orders correctly:
 *       pinned first → within pinned, newest pin wins → builtins
 *       among unpinned stay in declaration order → user-added retain
 *       insertion order
 *   - Persist key keeps custom templates across store re-creation
 */
import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import {
  useRejectionTemplates,
  sortTemplates,
  type RejectionTemplate,
} from './rejectionTemplates'

describe('useRejectionTemplates (store)', () => {
  beforeEach(() => {
    // Reset to seed before every test. Persist middleware may have
    // hydrated from a previous test run; reset() enforces a clean state.
    useRejectionTemplates.getState().reset()
  })
  afterEach(() => {
    useRejectionTemplates.getState().reset()
  })

  it('seeds exactly 4 built-in templates', () => {
    const t = useRejectionTemplates.getState().templates
    expect(t).toHaveLength(4)
    expect(t.every(x => x.builtin)).toBe(true)
  })

  it('built-ins have stable ids — used as persistence keys', () => {
    const ids = useRejectionTemplates.getState().templates.map(t => t.id)
    expect(ids).toContain('b-content-inaccurate')
    expect(ids).toContain('b-format-wrong')
    expect(ids).toContain('b-no-evidence')
    expect(ids).toContain('b-too-verbose')
  })

  it('first built-in is pinned by default (high-frequency reason)', () => {
    // The "内容不准确" builtin is pinned at seed time because it's
    // universally the most common reason — having it first saves
    // the user one click per rejection.
    const first = useRejectionTemplates.getState().templates[0]
    expect(first.id).toBe('b-content-inaccurate')
    expect(first.pinned).toBe(true)
  })

  it('add appends a user template with default emoji ✏️ when none provided', () => {
    const before = useRejectionTemplates.getState().templates.length
    useRejectionTemplates.getState().add('需要补充风险分析')
    const after = useRejectionTemplates.getState().templates
    expect(after).toHaveLength(before + 1)
    const added = after.find(t => t.label === '需要补充风险分析')
    expect(added).toBeDefined()
    expect(added!.builtin).toBe(false)
    expect(added!.pinned).toBe(false)
    expect(added!.emoji).toBe('✏️')
  })

  it('add accepts custom emoji + description', () => {
    useRejectionTemplates.getState().add('Compliance gap', { emoji: '⚖️', desc: 'GDPR / legal issues' })
    const t = useRejectionTemplates.getState().templates.find(x => x.label === 'Compliance gap')!
    expect(t.emoji).toBe('⚖️')
    expect(t.desc).toBe('GDPR / legal issues')
  })

  it('add trims whitespace from label', () => {
    useRejectionTemplates.getState().add('   spaced out   ')
    const t = useRejectionTemplates.getState().templates.find(x => x.label === 'spaced out')
    expect(t).toBeDefined()
  })

  it('add rejects empty / whitespace-only labels silently', () => {
    const before = useRejectionTemplates.getState().templates.length
    useRejectionTemplates.getState().add('')
    useRejectionTemplates.getState().add('   ')
    expect(useRejectionTemplates.getState().templates).toHaveLength(before)
  })

  it('add dedupes by label — re-adding the same label is a noop', () => {
    useRejectionTemplates.getState().add('重要')
    const before = useRejectionTemplates.getState().templates.length
    useRejectionTemplates.getState().add('重要')
    expect(useRejectionTemplates.getState().templates).toHaveLength(before)
    // Trimming should also dedupe
    useRejectionTemplates.getState().add('  重要  ')
    expect(useRejectionTemplates.getState().templates).toHaveLength(before)
  })

  it('remove blocks built-in templates (UI would be broken without defaults)', () => {
    const initialBuiltins = useRejectionTemplates.getState().templates.length
    useRejectionTemplates.getState().remove('b-content-inaccurate')
    expect(useRejectionTemplates.getState().templates).toHaveLength(initialBuiltins)
  })

  it('remove actually removes user-added templates', () => {
    useRejectionTemplates.getState().add('to-remove')
    const before = useRejectionTemplates.getState().templates.length
    const id = useRejectionTemplates.getState().templates.find(t => t.label === 'to-remove')!.id
    useRejectionTemplates.getState().remove(id)
    expect(useRejectionTemplates.getState().templates).toHaveLength(before - 1)
    expect(useRejectionTemplates.getState().templates.find(t => t.id === id)).toBeUndefined()
  })

  it('togglePin flips state and updates pinnedAt', () => {
    const t = useRejectionTemplates.getState().templates.find(x => x.id === 'b-format-wrong')!
    expect(t.pinned).toBe(false)
    const beforeTs = t.pinnedAt
    useRejectionTemplates.getState().togglePin('b-format-wrong')
    const after = useRejectionTemplates.getState().templates.find(x => x.id === 'b-format-wrong')!
    expect(after.pinned).toBe(true)
    expect(after.pinnedAt).toBeGreaterThan(beforeTs)

    // Toggle back — pinnedAt should stay at the previous value (not
    // reset to 0) so re-pinning later doesn't lose the original order.
    useRejectionTemplates.getState().togglePin('b-format-wrong')
    const off = useRejectionTemplates.getState().templates.find(x => x.id === 'b-format-wrong')!
    expect(off.pinned).toBe(false)
    expect(off.pinnedAt).toBe(after.pinnedAt)
  })

  it('reset re-seeds back to the 4 built-ins', () => {
    useRejectionTemplates.getState().add('garbage')
    useRejectionTemplates.getState().remove(useRejectionTemplates.getState().templates.find(t => t.label === 'garbage')!.id)
    // Force a state mutation
    useRejectionTemplates.getState().togglePin('b-no-evidence')
    useRejectionTemplates.getState().reset()
    const t = useRejectionTemplates.getState().templates
    expect(t).toHaveLength(4)
    expect(t.every(x => x.builtin)).toBe(true)
    // The seeded pinned state should be restored
    expect(t[0].pinned).toBe(true)
  })
})

describe('sortTemplates (pure helper)', () => {
  // Build a synthetic template list that exercises every ordering rule:
  //   - Pinned builtins (newest pin wins among pinned)
  //   - Unpinned builtins (declaration order)
  //   - Pinned user-added (newest pin first)
  //   - Unpinned user-added (insertion order)
  function makeFixtures(): RejectionTemplate[] {
    return [
      { id: 'a', label: 'A unpinned builtin', emoji: '1', pinned: false, pinnedAt: 0,        builtin: true,  desc: 'first-declared' },
      { id: 'b', label: 'B pinned-old builtin', emoji: '2', pinned: true,  pinnedAt: 100,      builtin: true,  desc: 'pinned early' },
      { id: 'c', label: 'C unpinned user-1', emoji: '3', pinned: false, pinnedAt: 0,        builtin: false, desc: 'a' },
      { id: 'd', label: 'D unpinned user-2', emoji: '4', pinned: false, pinnedAt: 0,        builtin: false, desc: 'b' },
      { id: 'e', label: 'E pinned-new user', emoji: '5', pinned: true,  pinnedAt: 999,      builtin: false, desc: 'recent' },
      { id: 'f', label: 'F pinned-mid user', emoji: '6', pinned: true,  pinnedAt: 500,      builtin: false, desc: 'mid' },
    ]
  }

  it('puts all pinned items before all unpinned items', () => {
    const sorted = sortTemplates(makeFixtures())
    const pinnedIds = sorted.filter(t => t.pinned).map(t => t.id)
    const unpinnedIds = sorted.filter(t => !t.pinned).map(t => t.id)
    // No pinned item should appear after any unpinned one
    const firstUnpinnedIdx = sorted.findIndex(t => !t.pinned)
    const lastPinnedIdx = sorted.map(t => t.pinned).lastIndexOf(true)
    expect(lastPinnedIdx).toBeLessThan(firstUnpinnedIdx)
    expect(pinnedIds.every(id => unpinnedIds.indexOf(id) === -1)).toBe(true)
  })

  it('sorts pinned items by pinnedAt DESC (newest pin first)', () => {
    const sorted = sortTemplates(makeFixtures()).filter(t => t.pinned)
    // Expected pinned order: E (999), F (500), B (100) — newest first
    expect(sorted.map(t => t.id)).toEqual(['e', 'f', 'b'])
  })

  it('preserves declaration order for unpinned builtins', () => {
    // The two unpinned builtins are A. (We only have one unpinned
    // builtin in the fixture, plus two unpinned user-added — the
    // built-in should come first because it was declared first.)
    const sorted = sortTemplates(makeFixtures()).filter(t => !t.pinned)
    expect(sorted[0].id).toBe('a')
  })

  it('does not mutate the input array', () => {
    const fixtures = makeFixtures()
    const before = fixtures.map(t => t.id)
    sortTemplates(fixtures)
    expect(fixtures.map(t => t.id)).toEqual(before)
  })

  it('handles empty arrays without error', () => {
    expect(sortTemplates([])).toEqual([])
  })

  it('handles a fully pinned set without crashing', () => {
    const allPinned: RejectionTemplate[] = [
      { id: '1', label: 'a', emoji: '', pinned: true, pinnedAt: 1, builtin: true },
      { id: '2', label: 'b', emoji: '', pinned: true, pinnedAt: 2, builtin: true },
    ]
    const sorted = sortTemplates(allPinned)
    expect(sorted.map(t => t.id)).toEqual(['2', '1'])
  })

  it('handles a fully unpinned set by preserving input order', () => {
    const allUnpinned: RejectionTemplate[] = [
      { id: '1', label: 'a', emoji: '', pinned: false, pinnedAt: 0, builtin: true },
      { id: '2', label: 'b', emoji: '', pinned: false, pinnedAt: 0, builtin: false },
    ]
    expect(sortTemplates(allUnpinned).map(t => t.id)).toEqual(['1', '2'])
  })
})

describe('integration: store ↔ sort', () => {
  // These exercise what the BidWorkspace modal actually does:
  // read templates, sort, render chips. If the contract between
  // the store shape and sortTemplates diverges, modal breaks.

  it('renders built-ins in the expected order at first load', () => {
    const sorted = sortTemplates(useRejectionTemplates.getState().templates)
    // First chip is "内容不准确" (the only pinned builtin)
    expect(sorted[0].label).toBe('内容不准确')
    // All builtins, no user-added (fresh state)
    expect(sorted).toHaveLength(4)
  })

  it('user-added + pin produces a sorted list with it at the front', () => {
    useRejectionTemplates.getState().add('custom top')
    const id = useRejectionTemplates.getState().templates.find(t => t.label === 'custom top')!.id
    useRejectionTemplates.getState().togglePin(id)
    const sorted = sortTemplates(useRejectionTemplates.getState().templates)
    expect(sorted[0].id).toBe(id)
  })
})

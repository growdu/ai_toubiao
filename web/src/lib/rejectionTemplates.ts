// @ts-nocheck
/**
 * Rejection templates store — the user-customizable pool of "why did
 * you reject this chapter" reasons surfaced in the RejectionReasonModal
 * as quick-pick chips. We always seed with 4 universal reasons (the
 * ones we baked in before) but users can add/remove/pin their own.
 *
 * Pinning is important: heavy users (auditors doing 50+ reviews a day)
 * want their top 3 reasons at the front. We keep all templates in
 * localStorage so the user's custom list survives reloads but is
 * scoped to the browser (not the workspace) — rejection reasons
 * are personal vocabulary, not team config.
 */
import { create } from 'zustand'
import { persist } from 'zustand/middleware'

export interface RejectionTemplate {
  id: string
  /** Short label shown on the chip. Keep under 12 chars for layout. */
  label: string
  /** Optional one-line explanation shown as chip tooltip. */
  desc?: string
  /** Emoji rendered left of label. Defaults to ✏️ if absent. */
  emoji?: string
  /** When true, this template is pinned to the front of the chip list.
   *  Pinned items still respect their relative order (sort by
   * pinnedAt timestamp for stable ordering). */
  pinned: boolean
  /** When the pin was last toggled. Used for stable sort order. */
  pinnedAt: number
  /** Distinguishes the built-in defaults (which can't be deleted) from
   *  user-added ones. Built-ins CAN be unpinned / re-pinned but not
   *  removed, so the modal always has a non-empty suggestion set. */
  builtin: boolean
}

const BUILTINS: Omit<RejectionTemplate, 'pinnedAt'>[] = [
  { id: 'b-content-inaccurate', label: '内容不准确', desc: '事实/数据有误', emoji: '❌', pinned: true,  builtin: true },
  { id: 'b-format-wrong',      label: '格式不对',   desc: '不符合招标格式', emoji: '📐', pinned: false, builtin: true },
  { id: 'b-no-evidence',       label: '缺证据',     desc: '未引用资质/案例', emoji: '🔍', pinned: false, builtin: true },
  { id: 'b-too-verbose',       label: '太啰嗦',     desc: '需要精简', emoji: '✂️', pinned: false, builtin: true },
]

interface RejectionState {
  templates: RejectionTemplate[]
  add: (label: string, opts?: { emoji?: string; desc?: string }) => void
  remove: (id: string) => void
  togglePin: (id: string) => void
  reset: () => void
}

const seed = (): RejectionTemplate[] => {
  const now = Date.now()
  return BUILTINS.map(t => ({ ...t, pinnedAt: t.pinned ? now : 0 }))
}

export const useRejectionTemplates = create<RejectionState>()(
  persist(
    (set, get) => ({
      templates: seed(),
      add: (label, opts) => {
        const trimmed = label.trim()
        if (!trimmed) return
        // Dedupe by label — same reason shouldn't appear twice.
        if (get().templates.some(t => t.label === trimmed)) return
        const id = `u-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`
        set({
          templates: [
            ...get().templates,
            { id, label: trimmed, desc: opts?.desc, emoji: opts?.emoji || '✏️', pinned: false, pinnedAt: 0, builtin: false },
          ],
        })
      },
      remove: (id) => {
        // Built-ins can be unpinned but not removed. Otherwise a
        // returning user would lose all their defaults.
        const t = get().templates.find(x => x.id === id)
        if (!t || t.builtin) return
        set({ templates: get().templates.filter(x => x.id !== id) })
      },
      togglePin: (id) => {
        set({
          templates: get().templates.map(t =>
            t.id === id
              ? { ...t, pinned: !t.pinned, pinnedAt: !t.pinned ? Date.now() : t.pinnedAt }
              : t,
          ),
        })
      },
      reset: () => set({ templates: seed() }),
    }),
    { name: 'rejection-templates-storage' },
  ),
)

/** Sort key: pinned first (newest pin first), then by original order. */
export function sortTemplates(templates: RejectionTemplate[]): RejectionTemplate[] {
  return [...templates].sort((a, b) => {
    if (a.pinned && !b.pinned) return -1
    if (!a.pinned && b.pinned) return 1
    if (a.pinned && b.pinned) return b.pinnedAt - a.pinnedAt
    // Unpinned: keep original order — built-ins come first (they were
    // declared in the BUILTINS array), then user-added in insertion order.
    return 0
  })
}

import { useEffect, useState, useMemo, useCallback } from 'react'
import { create } from 'zustand'

export interface Command {
  id: string
  label: string
  description?: string
  icon?: React.ReactNode
  group: string
  shortcut?: string
  perform: () => void
  /** If true, command is only available when a bid context is active. */
  requiresBid?: boolean
  /** Keywords for fuzzy matching beyond the label. */
  keywords?: string[]
}

interface CommandPaletteState {
  open: boolean
  setOpen: (open: boolean) => void
  toggle: () => void
}

export const useCommandPalette = create<CommandPaletteState>((set) => ({
  open: false,
  setOpen: (open) => set({ open }),
  toggle: () => set(s => ({ open: !s.open })),
}))

function defaultIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <polyline points="9 18 15 12 9 6" />
    </svg>
  )
}

function matchScore(cmd: Command, q: string): number {
  if (!q) return 1
  const haystack = [cmd.label, cmd.description, ...(cmd.keywords ?? [])]
    .filter(Boolean).join(' ').toLowerCase()
  const query = q.toLowerCase()
  if (cmd.label.toLowerCase().includes(query)) return 100
  if (haystack.includes(query)) return 50
  // Loose per-character match
  let qi = 0
  for (let i = 0; i < cmd.label.length && qi < query.length; i++) {
    if (cmd.label[i].toLowerCase() === query[qi]) qi++
  }
  return qi === query.length ? 10 : 0
}

interface CommandPaletteProps {
  commands: Command[]
}

export function CommandPalette({ commands }: CommandPaletteProps) {
  const open = useCommandPalette(s => s.open)
  const setOpen = useCommandPalette(s => s.setOpen)

  const [query, setQuery] = useState('')
  const [active, setActive] = useState(0)

  const results = useMemo(() => {
    return commands
      .map(c => ({ cmd: c, score: matchScore(c, query) }))
      .filter(x => x.score > 0)
      .sort((a, b) => b.score - a.score)
      .slice(0, 12)
  }, [commands, query])

  // Reset selection when results change
  useEffect(() => { setActive(0) }, [query])

  const run = useCallback((cmd: Command) => {
    setOpen(false)
    setQuery('')
    cmd.perform()
  }, [setOpen])

  // Keyboard navigation
  useEffect(() => {
    if (!open) return
    function onKey(e: KeyboardEvent) {
      if (e.key === 'ArrowDown') {
        e.preventDefault()
        setActive(i => Math.min(results.length - 1, i + 1))
      } else if (e.key === 'ArrowUp') {
        e.preventDefault()
        setActive(i => Math.max(0, i - 1))
      } else if (e.key === 'Enter') {
        e.preventDefault()
        const r = results[active]
        if (r) run(r.cmd)
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [open, results, active, run])

  if (!open) return null

  // Group consecutive items
  const groups: Array<{ name: string; items: Array<{ cmd: Command; idx: number }> }> = []
  results.forEach((r, idx) => {
    let g = groups.find(x => x.name === r.cmd.group)
    if (!g) { g = { name: r.cmd.group, items: [] }; groups.push(g) }
    g.items.push({ cmd: r.cmd, idx })
  })

  return (
    <div className="fixed inset-0 z-[90] flex items-start justify-center p-4 pt-[10vh] animate-fade-in">
      <div className="absolute inset-0 bg-ink-900/40 backdrop-blur-sm" onClick={() => setOpen(false)} />
      <div
        className="relative w-full max-w-xl bg-white dark:bg-ink-800 rounded-2xl shadow-pop border border-ink-100 dark:border-ink-700 overflow-hidden animate-slide-down"
        role="dialog"
        aria-modal="true"
        aria-label="命令面板"
      >
        {/* Search input */}
        <div className="flex items-center gap-3 px-4 py-3 border-b border-ink-100 dark:border-ink-700">
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="text-ink-400">
            <circle cx="11" cy="11" r="8" />
            <line x1="21" y1="21" x2="16.65" y2="16.65" />
          </svg>
          <input
            autoFocus
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="搜索命令或页面…"
            className="flex-1 bg-transparent border-0 outline-none text-sm placeholder:text-ink-400"
          />
          <kbd className="hidden sm:inline-flex items-center px-1.5 py-0.5 rounded border border-ink-200 dark:border-ink-700 bg-ink-50 dark:bg-ink-900 text-[10px] font-mono text-ink-500">esc</kbd>
        </div>

        {/* Results */}
        <div className="max-h-[60vh] overflow-y-auto scrollbar-thin py-1">
          {results.length === 0 ? (
            <div className="px-4 py-8 text-center text-sm text-ink-400">
              没有匹配的命令
            </div>
          ) : (
            groups.map(g => (
              <div key={g.name} className="py-1">
                <div className="px-3 py-1 text-[10px] font-semibold uppercase tracking-wider text-ink-400">
                  {g.name}
                </div>
                {g.items.map(({ cmd, idx }) => (
                  <button
                    key={cmd.id}
                    onClick={() => run(cmd)}
                    onMouseEnter={() => setActive(idx)}
                    className={[
                      'w-full flex items-center gap-3 px-3 py-2 text-left transition-colors',
                      active === idx
                        ? 'bg-brand-50 text-brand-800 dark:bg-brand-900 dark:text-brand-200'
                        : 'text-ink-700 dark:text-ink-200 hover:bg-ink-50 dark:hover:bg-ink-700',
                    ].join(' ')}
                  >
                    <span className={[
                      'shrink-0 w-7 h-7 rounded-md flex items-center justify-center',
                      active === idx ? 'bg-brand-100 text-brand-700 dark:bg-brand-800 dark:text-brand-200' : 'bg-ink-100 text-ink-500 dark:bg-ink-700 dark:text-ink-400',
                    ].join(' ')}>
                      {cmd.icon ?? defaultIcon()}
                    </span>
                    <span className="min-w-0 flex-1">
                      <span className="block text-sm font-medium truncate">{cmd.label}</span>
                      {cmd.description && (
                        <span className="block text-xs text-ink-500 dark:text-ink-400 truncate">{cmd.description}</span>
                      )}
                    </span>
                    {cmd.shortcut && (
                      <kbd className="hidden sm:inline-flex items-center px-1.5 py-0.5 rounded border border-ink-200 dark:border-ink-700 bg-white dark:bg-ink-900 text-[10px] font-mono text-ink-500">
                        {cmd.shortcut}
                      </kbd>
                    )}
                  </button>
                ))}
              </div>
            ))
          )}
        </div>

        {/* Footer */}
        <div className="px-4 py-2 border-t border-ink-100 dark:border-ink-700 bg-ink-50/60 dark:bg-ink-900/60 flex items-center justify-between text-[10px] text-ink-500">
          <div className="flex items-center gap-3">
            <span className="inline-flex items-center gap-1">
              <kbd className="px-1 py-0.5 rounded border border-ink-200 dark:border-ink-700 bg-white dark:bg-ink-800 font-mono">↑</kbd>
              <kbd className="px-1 py-0.5 rounded border border-ink-200 dark:border-ink-700 bg-white dark:bg-ink-800 font-mono">↓</kbd>
              导航
            </span>
            <span className="inline-flex items-center gap-1">
              <kbd className="px-1 py-0.5 rounded border border-ink-200 dark:border-ink-700 bg-white dark:bg-ink-800 font-mono">↵</kbd>
              执行
            </span>
          </div>
          <span>{results.length} 条结果</span>
        </div>
      </div>
    </div>
  )
}

/** Register a global Cmd/Ctrl+K hotkey that toggles the palette. */
export function useCommandPaletteHotkey() {
  const toggle = useCommandPalette(s => s.toggle)
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault()
        toggle()
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [toggle])
}
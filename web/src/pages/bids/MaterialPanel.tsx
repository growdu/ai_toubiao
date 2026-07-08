import { useState } from 'react'
import { TextArea, Button } from '../../components/ui'

interface MaterialPanelProps {
  materialText: string
  onChange: (text: string) => void
  onSave: () => void
  saving: boolean
  open: boolean
  onToggle: () => void
}

export function MaterialPanel({ materialText, onChange, onSave, saving, open, onToggle }: MaterialPanelProps) {
  const [fileName, setFileName] = useState<string | null>(null)

  const handleFile = async (file: File) => {
    setFileName(file.name)
    const text = await file.text()
    onChange(text)
  }

  const wordCount = materialText.trim().length

  return (
    <div className="border-b border-ink-100 dark:border-ink-700 shrink-0 bg-white dark:bg-ink-800">
      <button
        onClick={onToggle}
        className="w-full flex items-center justify-between px-3 py-2.5 text-sm text-ink-700 dark:text-ink-200 hover:bg-ink-50 dark:hover:bg-ink-800 transition-colors"
      >
        <span className="inline-flex items-center gap-1.5 font-medium">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
            <polyline points="14 2 14 8 20 8" />
          </svg>
          标书材料
          {materialText && (
            <span className="text-[10px] px-1.5 py-0.5 rounded-full bg-brand-50 dark:bg-brand-900/30 text-brand-700 dark:text-brand-300 font-semibold tabular-nums">
              {wordCount} 字
            </span>
          )}
        </span>
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round"
          style={{ transform: open ? 'rotate(180deg)' : 'rotate(0)', transition: 'transform 200ms' }}
          className="text-ink-400 dark:text-ink-500"
        >
          <polyline points="6 9 12 15 18 9" />
        </svg>
      </button>
      {open && (
        <div className="px-3 pb-3 space-y-2 animate-slide-down">
          <TextArea
            value={materialText}
            onChange={(e) => onChange(e.target.value)}
            rows={6}
            placeholder="粘贴招标文件内容、技术要求、评分标准、企业资质等材料…"
            className="text-xs"
          />
          <div className="flex items-center gap-2">
            <label className="flex-1 cursor-pointer">
              <input
                type="file"
                accept=".txt,.md,.docx,.pdf"
                className="hidden"
                onChange={(e) => {
                  const f = e.target.files?.[0]
                  if (f) handleFile(f)
                }}
              />
              <span className="flex items-center justify-center gap-1.5 px-2 py-1.5 text-xs text-center bg-white dark:bg-ink-700 border border-ink-200 dark:border-ink-700 rounded-lg hover:bg-ink-50 dark:hover:bg-ink-600 transition-colors truncate text-ink-600 dark:text-ink-300">
                <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
                  <polyline points="17 8 12 3 7 8" />
                  <line x1="12" y1="3" x2="12" y2="15" />
                </svg>
                <span className="truncate">{fileName ?? '上传文件'}</span>
              </span>
            </label>
            <Button
              size="sm"
              variant="primary"
              onClick={onSave}
              disabled={!materialText.trim() || saving}
              loading={saving}
              className="flex-1"
            >
              保存材料
            </Button>
          </div>
          <div className="flex items-start gap-1.5 p-2 rounded-md bg-brand-50/50 dark:bg-brand-900/20 border border-brand-100/50 dark:border-brand-900/40">
            <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="text-brand-600 dark:text-brand-400 mt-0.5 shrink-0">
              <circle cx="12" cy="12" r="10" />
              <line x1="12" y1="16" x2="12" y2="12" />
              <line x1="12" y1="8" x2="12.01" y2="8" />
            </svg>
            <p className="text-[10px] text-ink-600 dark:text-ink-400 leading-relaxed">
              材料将作为 AI 生成章节内容的事实依据与证据源，证据链自动追踪到具体段落。
            </p>
          </div>
        </div>
      )}
    </div>
  )
}
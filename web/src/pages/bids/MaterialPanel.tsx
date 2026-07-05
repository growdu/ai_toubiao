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

  return (
    <div className="border-b border-ink-100 shrink-0">
      <button
        onClick={onToggle}
        className="w-full flex items-center justify-between px-3 py-2.5 text-sm text-ink-700 hover:bg-ink-50 transition-colors"
      >
        <span className="inline-flex items-center gap-1.5 font-medium">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
            <polyline points="14 2 14 8 20 8" />
          </svg>
          标书材料
          {materialText && (
            <span className="text-[10px] px-1.5 py-0.5 rounded-full bg-brand-50 text-brand-700 font-semibold">
              {materialText.length}字
            </span>
          )}
        </span>
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round"
          style={{ transform: open ? 'rotate(180deg)' : 'rotate(0)', transition: 'transform 200ms' }}>
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
              <span className="block px-2 py-1.5 text-xs text-center bg-white border border-ink-200 rounded-lg hover:bg-ink-50 transition-colors truncate">
                {fileName ?? '上传文件'}
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
          <p className="text-[10px] text-ink-400 leading-relaxed">
            材料将作为 AI 生成章节内容的事实依据与证据源，证据链自动追踪到具体段落。
          </p>
        </div>
      )}
    </div>
  )
}
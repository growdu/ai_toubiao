import { useState, useEffect } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { parseApi, bidsApi } from '../../api/bids'
import { BidStepper } from './BidStepper'
import { Button, TextArea, TextInput } from '../../components/ui'
import { toast } from '../../lib/toast'
import { extractErrMsg } from '../../lib/apiError'

interface ParsedFields {
  project_name?: string
  bid_no?: string
  industry?: string
  rfp_type?: string
  issuer?: string
  deadline?: string
  budget?: string
  overview?: string
  requirements?: string[]
  technical_specs?: string[]
  scoring_criteria?: string[]
  qualifications?: string[]
  [key: string]: any
}

const TEXT_FIELDS: { key: keyof ParsedFields; label: string }[] = [
  { key: 'project_name', label: '项目名称' },
  { key: 'bid_no', label: '招标编号' },
  { key: 'industry', label: '行业' },
  { key: 'rfp_type', label: '招标类型' },
  { key: 'issuer', label: '招标人' },
  { key: 'deadline', label: '投标截止' },
  { key: 'budget', label: '预算金额' },
]

const ARRAY_FIELDS: { key: keyof ParsedFields; label: string; placeholder: string }[] = [
  { key: 'requirements', label: '采购需求', placeholder: '每行一条采购需求…' },
  { key: 'technical_specs', label: '技术参数', placeholder: '每行一条技术参数…' },
  { key: 'scoring_criteria', label: '评分标准', placeholder: '每行一条评分标准…' },
  { key: 'qualifications', label: '资质要求', placeholder: '每行一条资质要求…' },
]

// 4 步向导的步骤1/2（仅 bid.status === 'pending'/'parsing' 时由
// BidWorkspaceWrapper 渲染）：
//  步骤1：粘贴/上传招标材料 -> "解析材料" -> 调 /bids/:id/parse 展示结构化结果
//  步骤2：审核、编辑、补充解析结果 -> "确认并生成大纲" -> transition outlining
// 步骤3/4（生成标书 + 单章精修）由 BidWorkspace 接管。
export function WizardSteps({ bidId }: { bidId: string }) {
  const queryClient = useQueryClient()
  const [step, setStep] = useState<0 | 1>(0)
  const [material, setMaterial] = useState('')
  const [edited, setEdited] = useState<ParsedFields>({})

  // 读取已保存的材料与解析结果（刷新后可继续）
  const { data: parseData } = useQuery({
    queryKey: ['parse', bidId],
    queryFn: () => parseApi.getParse(bidId),
  })
  const savedMaterial: string = parseData?.data?.data?.material_text || ''
  const savedParsed: ParsedFields = parseData?.data?.data?.parsed || {}

  // 首次拿到已保存的解析结果时，直接进入步骤2审核，避免重复解析。
  useEffect(() => {
    if (savedParsed && Object.keys(savedParsed).length > 0 && step === 0 && !material) {
      setEdited(savedParsed)
      setMaterial(savedMaterial)
      setStep(1)
    }
  }, [savedParsed, savedMaterial, step, material])

  // 步骤1：解析材料（调 router-svc 提取结构化字段）
  const parseMutation = useMutation({
    mutationFn: (text: string) => parseApi.parseMaterial(bidId, text),
    onSuccess: (res) => {
      setMaterial(res.data.data.material_text)
      setEdited(res.data.data.parsed || {})
      queryClient.invalidateQueries({ queryKey: ['parse', bidId] })
      setStep(1)
      toast.success('解析完成', '请审核解析结果')
    },
    onError: (err: any) => toast.error('解析失败', extractErrMsg(err)),
  })

  // 步骤2：保存草稿（用户编辑中途保存）
  const saveMutation = useMutation({
    mutationFn: (data: { material_text?: string; parsed: ParsedFields }) =>
      parseApi.updateParse(bidId, data),
    onSuccess: () => toast.success('草稿已保存'),
    onError: (err: any) => toast.error('保存失败', extractErrMsg(err)),
  })

  // 确认 -> 保存编辑 + 推进工作流到 outlining，触发后端大纲生成
  const proceedMutation = useMutation({
    mutationFn: async () => {
      await parseApi.updateParse(bidId, { material_text: material, parsed: edited })
      return bidsApi.transition(bidId, 'outlining')
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['bid', bidId] })
      toast.success('已开始生成大纲', '即将进入工作区…')
    },
    onError: (err: any) => toast.error('推进失败', extractErrMsg(err)),
  })

  const handleFile = async (file: File) => {
    const text = await file.text()
    setMaterial(text)
  }

  const setField = (key: keyof ParsedFields, val: any) =>
    setEdited((prev) => ({ ...prev, [key]: val }))

  const arrayToText = (arr?: string[]) => (arr || []).join('\n')
  const textToArray = (t: string) => t.split('\n').map((s) => s.trim()).filter(Boolean)

  return (
    <div className="flex-1 overflow-y-auto scrollbar-thin bg-ink-50 dark:bg-ink-900">
      <BidStepper current={step} />
      <div className="max-w-4xl mx-auto px-6 py-8">
        {step === 0 && (
          <section className="space-y-4 animate-fade-in">
            <div>
              <h1 className="text-2xl font-bold text-ink-900 dark:text-white">步骤 1 · 解析招标材料</h1>
              <p className="text-sm text-ink-500 dark:text-ink-400 mt-1">
                粘贴或上传招标文件，AI 将提取项目信息、技术要求、评分标准等关键内容，作为标书制作的依据。
              </p>
            </div>
            <TextArea
              value={material}
              onChange={(e) => setMaterial(e.target.value)}
              rows={12}
              placeholder="粘贴招标文件内容、技术要求、评分标准、企业资质等材料…"
            />
            <div className="flex items-center gap-2">
              <label className="cursor-pointer">
                <input
                  type="file"
                  accept=".txt,.md,.docx,.pdf"
                  className="hidden"
                  onChange={(e) => {
                    const f = e.target.files?.[0]
                    if (f) handleFile(f)
                  }}
                />
                <span className="inline-flex items-center gap-1.5 px-3 py-2 text-sm bg-white dark:bg-ink-700 border border-ink-200 dark:border-ink-700 rounded-lg hover:bg-ink-50 dark:hover:bg-ink-600 text-ink-600 dark:text-ink-300">
                  上传文件
                </span>
              </label>
              <Button
                variant="primary"
                loading={parseMutation.isPending}
                disabled={!material.trim()}
                onClick={() => parseMutation.mutate(material)}
              >
                解析材料
              </Button>
              {savedMaterial && !material && (
                <Button
                  variant="ghost"
                  onClick={() => {
                    setMaterial(savedMaterial)
                    setEdited(savedParsed)
                    setStep(1)
                  }}
                >
                  继续上次的解析结果
                </Button>
              )}
            </div>
          </section>
        )}

        {step === 1 && (
          <section className="space-y-5 animate-fade-in">
            <div className="flex items-center justify-between">
              <div>
                <h1 className="text-2xl font-bold text-ink-900 dark:text-white">步骤 2 · 审核解析结果</h1>
                <p className="text-sm text-ink-500 dark:text-ink-400 mt-1">
                  核对并补充 AI 提取的信息，确认无误后生成标书大纲。可随时返回修改原始材料。
                </p>
              </div>
              <Button variant="ghost" size="sm" onClick={() => setStep(0)}>返回修改材料</Button>
            </div>

            <div className="grid grid-cols-2 gap-3">
              {TEXT_FIELDS.map((f) => (
                <label key={f.key} className="block">
                  <span className="block text-xs text-ink-500 dark:text-ink-400 mb-1">{f.label}</span>
                  <TextInput
                    value={(edited[f.key] as string) || ''}
                    onChange={(e) => setField(f.key, e.target.value)}
                  />
                </label>
              ))}
            </div>

            <label className="block">
              <span className="block text-xs text-ink-500 dark:text-ink-400 mb-1">项目概述</span>
              <TextArea
                value={(edited.overview as string) || ''}
                onChange={(e) => setField('overview', e.target.value)}
                rows={3}
              />
            </label>

            <div className="grid grid-cols-2 gap-3">
              {ARRAY_FIELDS.map((f) => (
                <label key={f.key} className="block">
                  <span className="block text-xs text-ink-500 dark:text-ink-400 mb-1">{f.label}</span>
                  <TextArea
                    value={arrayToText(edited[f.key] as string[] | undefined)}
                    onChange={(e) => setField(f.key, textToArray(e.target.value))}
                    rows={5}
                    placeholder={f.placeholder}
                  />
                </label>
              ))}
            </div>

            <div className="flex items-center justify-between pt-2 border-t border-ink-100 dark:border-ink-700">
              <Button
                variant="ghost"
                onClick={() => saveMutation.mutate({ material_text: material, parsed: edited })}
                loading={saveMutation.isPending}
              >
                保存草稿
              </Button>
              <Button
                variant="primary"
                loading={proceedMutation.isPending}
                onClick={() => proceedMutation.mutate()}
              >
                确认并生成大纲 →
              </Button>
            </div>
          </section>
        )}
      </div>
    </div>
  )
}

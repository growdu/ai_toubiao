import { useParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { bidsApi } from '../../api/bids'
import BidWorkspace from './BidWorkspace'
import { WizardSteps } from './WizardSteps'

// 根据 bid.status 在"4 步向导（步骤1/2）"与"工作区（步骤3/4）"间切换：
//  - pending/parsing：渲染 WizardSteps（解析材料 + 审核编辑）
//  - 其他（outlining 及之后）：渲染 BidWorkspace（生成标书 + 单章精修）
// 两者共享 ['bid', id] 查询缓存，切换时无额外请求。
export default function BidWorkspaceWrapper() {
  const { id } = useParams<{ id: string }>()
  const { data, isLoading } = useQuery({
    queryKey: ['bid', id],
    queryFn: () => bidsApi.get(id!),
    enabled: !!id,
  })
  const status = data?.data?.data?.status

  if (isLoading) {
    return <div className="flex-1 flex items-center justify-center text-ink-500">加载中…</div>
  }
  if (status === 'pending' || status === 'parsing') {
    return <WizardSteps bidId={id!} />
  }
  return <BidWorkspace />
}

// 后端错误体统一为 { error: { code, message } }，部分旧端点用 { message }。
// 本 helper 兼容两种，返回可展示给用户的中文消息，统一前端错误解析。
export function extractErrMsg(err: any): string {
  const data = err?.response?.data
  return (
    data?.error?.message ||
    data?.message ||
    data?.error?.code ||
    err?.message ||
    '操作失败，请稍后重试'
  )
}

import api from './client'

export interface AuditIssue {
  id: string
  chapter_title: string
  severity: string
  dimension: string
  issue: string
  suggestion: string
  evidence?: string
}

export interface AuditReport {
  id: string
  bid_job_id: string
  status: string
  total_issues: number
  critical_count: number
  issues: AuditIssue[]
  created_at: string
}

export const auditApi = {
  /** Trigger a compliance audit for a bid job. */
  trigger: (bidJobId: string) =>
    api.post<{ data: AuditReport }>(`/audit/bidjobs/${bidJobId}/report`),

  /** Get the latest audit report for a bid job. */
  getReport: (bidJobId: string) =>
    api.get<{ data: AuditReport }>(`/audit/bidjobs/${bidJobId}/report`),
}

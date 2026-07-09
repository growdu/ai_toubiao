import api from './client'

export interface Budget {
  id: string
  tenant_id: string
  month: string
  limit_cents: number
  spent_cents: number
  created_at: string
  updated_at: string
}

export interface BudgetSummary {
  budget: Budget
  spent_cents: number
  limit_cents: number
  percent_used: number
}

export interface Transaction {
  id: string
  tenant_id: string
  provider: string
  model: string
  task: string
  prompt_tokens: number
  completion_tokens: number
  cost_cents: number
  created_at: string
}

export interface CheckoutResult {
  tenant_id: string
  plan: string
  upgraded_at: string
}

export const billingApi = {
  getCurrentBudget: () =>
    api.get<{ data: BudgetSummary }>('/billing/budget/current'),

  setBudget: (month: string, limitCents: number) =>
    api.post<{ data: Budget }>('/billing/budget', { month, limit_cents: limitCents }),

  getTransactions: (limit?: number) =>
    api.get<{ data: Transaction[] }>('/billing/transactions', { params: { limit } }),

  /** Upgrade the current tenant to a new plan (dev mode: instant, no payment). */
  checkout: (planId: string) =>
    api.post<{ data: CheckoutResult }>('/billing/checkout', { plan_id: planId }),

  /** Get the current plan (free / pro / enterprise). */
  getCurrentPlan: () =>
    api.get<{ data: { plan: string } }>('/billing/plan'),
}

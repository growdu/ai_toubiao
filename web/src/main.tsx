import React from 'react'
import ReactDOM from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { BrowserRouter } from 'react-router-dom'
import App from './App'
import './index.css'

/**
 * Query retry policy — `retry: 1` (the default) treats every error
 * the same way: retry once. That's wrong for two cases:
 *
 *   1. 4xx client errors (except 408 Request Timeout) — retrying a
 *      401, 403, 404, or 422 wastes a request and clutters the UI
 *      with stale retry loading states. Server told us "no" — listen.
 *
 *   2. 401 Unauthorized specifically — the user is no longer
 *      authenticated. Retrying just hits a 401 again; we should
 *      trigger the auth-flow's session-clear IMMEDIATELY. We do this
 *      by dispatching a custom event the auth store listens for.
 *
 * For everything else (network errors, 5xx server outages, 408
 * timeout) we retry exactly once because retrying is the right move:
 * the network is flaky, a quick retry usually succeeds.
 */
const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 1000 * 60,
      // Custom retry: 4xx → no retry; 401 → broadcast a session-expired
      // event so the auth store can clear and the interceptor can
      // bounce to login; everything else → 1 retry.
      retry: (failureCount, error) => {
        const status = (error as any)?.response?.status ?? (error as any)?.status
        if (status === 401) {
          // Tell the auth store. We use a synthetic event instead of
          // directly calling useAuthStore.getState() because the
          // QueryClient is created at module scope and we want zero
          // coupling between query and auth modules.
          if (typeof window !== 'undefined') {
            window.dispatchEvent(new CustomEvent('app:session-expired'))
          }
          return false
        }
        // 4xx (except 408 Request Timeout) should not retry.
        if (typeof status === 'number' && status >= 400 && status < 500 && status !== 408) {
          return false
        }
        // Network errors / 5xx / 408 → retry once.
        return failureCount < 1
      },
      // Honor HTTP Cache-Control headers if the server sends them.
      // We don't, so staleTime below acts as a hard upper bound on
      // cache freshness — 60s for general queries is fine because
      // most of our pages either auto-select or invalidate on mutate.
    },
  },
})

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <App />
      </BrowserRouter>
    </QueryClientProvider>
  </React.StrictMode>,
)
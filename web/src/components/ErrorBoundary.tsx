// @ts-nocheck
/**
 * ErrorBoundary — catches render-time errors anywhere below itself in
 * the tree and renders a fallback instead of letting React tear down
 * the whole app.
 *
 * Two flavors:
 *   1. <GlobalErrorBoundary>  — wraps <App>. Catches fatal errors
 *      that the route-level boundary can't (e.g. a bad layout or
 *      toast provider itself). Offers "回到首页" + "刷新页面" —
 *      the only safe escapes from a torn-down tree.
 *   2. <RouteErrorBoundary>   — wraps a route. Catches errors that
 *      happen inside one specific page so the header/footer/layout
 *      stay usable for retry. Offers "重试" (resets boundary state
 *      and re-mounts children) which is the difference between this
 *      and the global flavor — per-route retries are high-value
 *      because most error states are transient (an unmount race,
 *      a stale read from a stale TanStack cache).
 *
 * Why a class component: React did not ship a hook equivalent of
 * `componentDidCatch` / `getDerivedStateFromError`. The team
 * discussed using `useErrorBoundary` (from react-error-boundary)
 * but adding a third-party dep for ~80 lines of code was overkill.
 * If we adopt react-error-boundary later, the export shape stays
 * the same; only the implementation swaps.
 *
 * UX rules:
 *   - In dev: show the error message + first 1KB of stack inline so
 *     developers can fix without opening DevTools.
 *   - In prod: show a friendly message ("页面出错了") + the error id
 *     so users can report it.
 *   - Never silent: always show a visible "重试"/"返回" affordance.
 *     A "this broke but I'll work around it" UX is the worst case.
 */
import { Component, type ReactNode, type ErrorInfo } from 'react'
import { useNavigate, useLocation } from 'react-router-dom'

interface State {
  error: Error | null
  /** Stack trace: captured in dev, replaced with null in prod bundle. */
  stack?: string
  /** Stable identifier — useful for support tickets. */
  id: string | null
}

interface BaseProps {
  children: ReactNode
}

const DEVELOPER_MODE = import.meta.env?.DEV ?? false

abstract class BaseErrorBoundary<P extends BaseProps, S extends State> extends Component<P, S> {
  abstract renderFallback(props: { reset: () => void }): ReactNode

  static getDerivedStateFromError(error: Error): Partial<State> {
    return {
      error,
      // Keep the stack capture in componentDidCatch (happens once) so
      // we don't pay the toString cost twice. We DO set the error
      // here because React requires the error to derive state — by
      // contract, the state must include `error` synchronously.
    }
  }

  componentDidCatch(error: Error, info: ErrorInfo): void {
    const id = crypto.randomUUID?.() ?? `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
    const stack = DEVELOPER_MODE ? (error.stack ?? '') : undefined
    // Replace this with a real logging service when one exists. Today
    // we console.error so the error lands in the browser console for
    // both dev and prod-mode debugging.
    // eslint-disable-next-line no-console
    console.error('[ErrorBoundary]', id, error, info.componentStack)
    this.setState({ stack, id } as Pick<State, 'stack' | 'id'>)
  }

  reset = (): void => {
    this.setState({ error: null, stack: undefined, id: null })
  }

  render() {
    if (this.state.error) return this.renderFallback({ reset: this.reset })
    return this.props.children
  }
}

// ---------- Global flavor ----------

interface GlobalState extends State {
  // No extras today. Kept as a separate interface so we can extend
  // (e.g. "is this a hard or soft error?") without rewriting the
  // route-level boundary.
}

/**
 * Use the boundary's renderFallback template instead of inline JSX so
 * the global and route flavors share the visual treatment but call
 * different actions (route → retry, global → home).
 */
export class GlobalErrorBoundary extends BaseErrorBoundary<BaseProps, GlobalState> {
  state: GlobalState = { error: null, stack: undefined, id: null }

  renderFallback({ reset }: { reset: () => void }) {
    return <GlobalFallback reset={reset} error={this.state.error!} id={this.state.id} stack={this.state.stack} />
  }
}

function GlobalFallback({ reset, error, id, stack }: { reset: () => void; error: Error; id: string | null; stack: string | undefined }) {
  // We deliberately don't import useNavigate from inside the class.
  // Instead, extract a wrapper component that uses the hook.
  return <GlobalFallbackInner reset={reset} error={error} id={id} stack={stack} />
}

function GlobalFallbackInner({ reset, error, id, stack }: { reset: () => void; error: Error; id: string | null; stack: string | undefined }) {
  const navigate = useNavigate()
  const location = useLocation()
  return (
    <FatalErrorPage
      error={error}
      id={id}
      stack={stack}
      onGoHome={() => navigate('/', { replace: true })}
      onReload={() => window.location.reload()}
      onRetry={() => { reset(); /* stay on current path, re-render */ void location }}
    />
  )
}

// ---------- Route flavor ----------

export class RouteErrorBoundary extends BaseErrorBoundary<BaseProps, State> {
  state: State = { error: null, stack: undefined, id: null }
  renderFallback({ reset }: { reset: () => void }) {
    return <RouteFallback reset={reset} error={this.state.error!} id={this.state.id} stack={this.state.stack} />
  }
}

function RouteFallback(props: { reset: () => void; error: Error; id: string | null; stack: string | undefined }) {
  return <RouteErrorPage {...props} />
}

// ---------- Shared visual primitives ----------

function FatalErrorPage({ error, id, stack, onGoHome, onReload, onRetry }: {
  error: Error
  id: string | null
  stack: string | undefined
  onGoHome: () => void
  onReload: () => void
  onRetry: () => void
}) {
  return (
    <div role="alert" className="min-h-screen flex items-center justify-center bg-ink-50 dark:bg-ink-900 p-6">
      <div className="max-w-md w-full text-center">
        <div className="inline-flex w-14 h-14 rounded-2xl bg-red-100 dark:bg-red-900/30 text-red-600 dark:text-red-400 items-center justify-center mb-4">
          <svg width="26" height="26" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <circle cx="12" cy="12" r="10" />
            <line x1="12" y1="8" x2="12" y2="12" />
            <line x1="12" y1="16" x2="12.01" y2="16" />
          </svg>
        </div>
        <h1 className="text-xl font-bold text-ink-900 dark:text-white mb-2">页面出错了</h1>
        <p className="text-sm text-ink-600 dark:text-ink-300 mb-1">
          渲染过程中遇到了一个问题。我们已记录错误并展示在下方。
        </p>
        {id && (
          <p className="text-[11px] font-mono text-ink-400 dark:text-ink-500 mb-4">
            错误 ID：{id}
          </p>
        )}
        {stack && (
          <details className="text-left mb-5 mx-auto max-w-md">
            <summary className="text-[11px] text-ink-500 dark:text-ink-400 cursor-pointer hover:text-brand-600">显示堆栈（开发模式）</summary>
            <pre className="mt-2 p-3 rounded-md bg-ink-900 text-ink-100 text-[10px] overflow-auto max-h-40 font-mono whitespace-pre-wrap break-all">
              {error.message}
              {'\n\n'}
              {stack}
            </pre>
          </details>
        )}
        <div className="flex gap-2 justify-center">
          <button
            onClick={onGoHome}
            className="px-4 py-2 rounded-md bg-brand-600 hover:bg-brand-700 text-white text-sm font-medium transition-colors"
          >
            回到首页
          </button>
          <button
            onClick={onReload}
            className="px-4 py-2 rounded-md bg-ink-200 dark:bg-ink-700 text-ink-700 dark:text-ink-200 text-sm font-medium hover:bg-ink-300 dark:hover:bg-ink-600 transition-colors"
          >
            刷新页面
          </button>
          <button
            onClick={onRetry}
            className="px-4 py-2 rounded-md border border-ink-200 dark:border-ink-700 text-ink-700 dark:text-ink-200 text-sm font-medium hover:border-brand-300 hover:text-brand-700 transition-colors"
          >
            重试
          </button>
        </div>
      </div>
    </div>
  )
}

function RouteErrorPage({ reset, error, id, stack }: { reset: () => void; error: Error; id: string | null; stack: string | undefined }) {
  return (
    <div role="alert" className="p-6 my-6 mx-auto max-w-2xl bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800/50 rounded-xl">
      <div className="flex items-start gap-3">
        <div className="shrink-0 w-9 h-9 rounded-lg bg-red-100 dark:bg-red-900/40 text-red-600 dark:text-red-400 grid place-items-center">
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z" />
            <line x1="12" y1="9" x2="12" y2="13" />
            <line x1="12" y1="17" x2="12.01" y2="17" />
          </svg>
        </div>
        <div className="flex-1 min-w-0">
          <h2 className="text-sm font-bold text-red-700 dark:text-red-400">这个页面遇到了一个问题</h2>
          <p className="text-xs text-red-600/80 dark:text-red-300/80 mt-1">
            {error.message || '未知错误'}
          </p>
          {id && <p className="text-[10px] font-mono text-red-500/60 dark:text-red-400/60 mt-1">错误 ID：{id}</p>}
          {stack && (
            <details className="mt-2">
              <summary className="text-[10px] text-red-500 dark:text-red-400 cursor-pointer hover:underline">查看堆栈</summary>
              <pre className="mt-1 p-2 bg-red-100 dark:bg-red-900/40 rounded text-[10px] font-mono overflow-auto max-h-32 whitespace-pre-wrap break-all text-red-800 dark:text-red-200">
                {stack}
              </pre>
            </details>
          )}
          <button
            onClick={reset}
            className="mt-3 px-3 py-1.5 text-xs font-medium rounded-md bg-red-600 text-white hover:bg-red-700 transition-colors"
          >
            重试
          </button>
        </div>
      </div>
    </div>
  )
}

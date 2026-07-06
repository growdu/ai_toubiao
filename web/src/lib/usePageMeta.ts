import { useEffect } from 'react'

// usePageMeta sets document.title and a few common meta tags for the
// duration the component is mounted. We don't bring in react-helmet
// for this — its surface area is huge and we only need three tags.
//
// Behaviour notes:
//   * When the component unmounts, we restore the previous values so
//     navigating away doesn't leave stale SEO tags on the document.
//     For example, the LoginPage leaves <title> = "登录 · BidWriter";
//     on unmount we put it back to the index.html default.
//   * We tag the elements with a `data-page-meta` attribute so we
//     don't fight with index.html's initial values — we set our own
//     copies that win when present.
//   * Crawlers that don't run JS see only index.html's static tags.
//     Crawlers that DO run JS (Googlebot does, surprisingly) get the
//     per-page title after hydration.
export interface PageMeta {
  title?: string
  description?: string
  // Optional: tell crawlers to NOT index this page (login, register,
  // private workspaces).
  noindex?: boolean
}

const SITE_NAME = 'BidWriter · AI 标书生成系统'

export function usePageMeta(meta: PageMeta) {
  useEffect(() => {
    // Remember the previous values so we can restore them on cleanup.
    const prevTitle = document.title

    if (meta.title) {
      document.title = `${meta.title} · ${SITE_NAME}`
    }

    const prevDescription = setMeta('description', meta.description)
    const prevOgTitle = setMeta('og:title', meta.title ? `${meta.title} · ${SITE_NAME}` : undefined)
    const prevOgDescription = setMeta('og:description', meta.description)
    const prevRobots = setMeta('robots', meta.noindex ? 'noindex,nofollow' : undefined)

    return () => {
      document.title = prevTitle
      restoreMeta('description', prevDescription)
      restoreMeta('og:title', prevOgTitle)
      restoreMeta('og:description', prevOgDescription)
      restoreMeta('robots', prevRobots)
    }
  }, [meta.title, meta.description, meta.noindex])
}

// setMeta creates or updates a <meta> tag and returns the previous
// content so callers can restore it on unmount.
function setMeta(name: string, content?: string): string | null {
  if (content === undefined) return null
  // Use property-attribute selectors: og:* lives in `property`, the
  // others in `name`. Branch on a small prefix match.
  const isOg = name.startsWith('og:') || name.startsWith('twitter:')
  const selector = isOg ? `meta[property="${name}"]` : `meta[name="${name}"]`
  let el = document.head.querySelector<HTMLMetaElement>(selector)
  if (!el) {
    el = document.createElement('meta')
    if (isOg) el.setAttribute('property', name)
    else el.setAttribute('name', name)
    el.setAttribute('data-page-meta', 'true')
    document.head.appendChild(el)
  }
  const prev = el.getAttribute('content')
  el.setAttribute('content', content)
  return prev
}

// restoreMeta re-applies a previous content value (or removes the
// tag entirely if there was no prior value).
function restoreMeta(name: string, prev: string | null) {
  const isOg = name.startsWith('og:') || name.startsWith('twitter:')
  const selector = isOg ? `meta[property="${name}"][data-page-meta]` : `meta[name="${name}"][data-page-meta]`
  const el = document.head.querySelector<HTMLMetaElement>(selector)
  if (!el) return
  if (prev === null) {
    el.remove()
  } else {
    el.setAttribute('content', prev)
  }
}

// @ts-nocheck
import puppeteer from 'puppeteer'
const CHROME = process.env.HOME + '/.cache/puppeteer/chrome/linux-150.0.7871.24/chrome-linux64/chrome'
const browser = await puppeteer.launch({
  executablePath: CHROME, headless: true,
  args: ['--no-sandbox', '--disable-setuid-sandbox', '--disable-dev-shm-usage'],
})
const page = await browser.newPage()
await page.setViewport({ width: 1440, height: 900 })
const errors = []
page.on('pageerror', e => errors.push('PAGE: ' + e.message))
page.on('console', m => { if (m.type() === 'error') errors.push('CONSOLE: ' + m.text()) })

await page.setRequestInterception(true)
page.on('request', req => {
  const url = req.url()
  if (url.indexOf('/api/v1/') === -1) { req.continue(); return }
  const respond = (s, j) => req.respond({ status: s, contentType: 'application/json', body: JSON.stringify(j) })
  if (url.endsWith('/api/v1/bids') && req.method() === 'GET') {
    return respond(200, { data: [{ id: 'demo-bid-001', project_id: 'p', status: 'awaiting_review', current_step: 'awaiting_review', project_name: '深圳地铁 12 号线施工总承包投标', industry: '轨道交通', total_chapters: 3, done_chapters: 3, created_at: new Date().toISOString(), updated_at: new Date().toISOString(), version: 1 }], meta: { count: 1 } })
  }
  if (url.indexOf('/bids/demo-bid-001') !== -1 && url.indexOf('/outline') === -1 && url.indexOf('/chapters') === -1) {
    return respond(200, { data: { id: 'demo-bid-001', project_id: 'p', status: 'awaiting_review', current_step: 'awaiting_review', project_name: '深圳地铁 12 号线施工总承包投标', industry: '轨道交通', total_chapters: 3, done_chapters: 3, created_at: new Date().toISOString(), updated_at: new Date().toISOString(), version: 1 } })
  }
  if (url.indexOf('/outline') !== -1 && req.method() === 'GET') {
    return respond(200, { data: [
      { id: 'ch-1', bid_job_id: 'demo-bid-001', parent_id: undefined, title: '项目背景与理解', level: 1, order_index: 0, chapter_type: 'background', target_word_count: 800, min_word_count: 600, writing_style: 'formal', priority: 'critical', status: 'approved', approved_at: '2026-07-05T10:23:00Z', approved_by: 'demo-user-id' },
      { id: 'ch-2', bid_job_id: 'demo-bid-001', parent_id: undefined, title: '总体技术方案', level: 1, order_index: 1, chapter_type: 'technical', target_word_count: 1500, min_word_count: 1200, writing_style: 'detailed', priority: 'critical', status: 'approved', approved_at: '2026-07-05T10:25:00Z', approved_by: 'demo-user-id' },
      { id: 'ch-3', bid_job_id: 'demo-bid-001', parent_id: 'ch-2', title: '系统架构设计', level: 2, order_index: 2, chapter_type: 'technical', target_word_count: 800, min_word_count: 600, writing_style: 'formal', priority: 'high', status: 'succeeded' },
    ] })
  }
  if (url.indexOf('/chapters/ch-1/content') !== -1 && req.method() === 'GET') {
    return respond(200, { data: { chapter_spec_id: 'ch-1', version: 1, content_text: '# test', word_count: 100, min_word_met: true, generated_by: 'GPT-4', llm_model: 'gpt-4-turbo', generation_duration_ms: 3200 } })
  }
  return respond(200, { data: { ok: true } })
})
await page.evaluateOnNewDocument(() => {
  localStorage.setItem('auth-storage', JSON.stringify({ state: { token: 'mock', userId: 'demo-user-id', tenantId: 't' }, version: 0 }))
})
await page.goto('http://127.0.0.1:8888/bids/demo-bid-001', { waitUntil: 'load', timeout: 20000 })
await new Promise(r => setTimeout(r, 2500))

// Click 状态 tab in inspector
await page.evaluate(() => {
  const tabs = Array.from(document.querySelectorAll('button'))
  const statusTab = tabs.find(b => b.textContent && b.textContent.trim() === '状态')
  if (statusTab) statusTab.click()
})
await new Promise(r => setTimeout(r, 800))

const diag = await page.evaluate(() => {
  const all = Array.from(document.querySelectorAll('h3, .md-preview, .text-xs'))
  return {
    timelineH3Present: !!Array.from(document.querySelectorAll('h3')).find(h => h.textContent && h.textContent.includes('审核历史')),
    approvedTimelineEntry: !!Array.from(document.querySelectorAll('li')).find(li => li.textContent && li.textContent.includes('已审核通过')),
    operatorShown: !!Array.from(document.querySelectorAll('span')).find(s => s.textContent && s.textContent.includes('demo-user-id')),
    allH3Texts: Array.from(document.querySelectorAll('h3')).map(h => h.textContent),
  }
})
console.log(JSON.stringify(diag, null, 2))
console.log('errors:', errors)

await browser.close()

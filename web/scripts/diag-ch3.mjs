// @ts-nocheck
import puppeteer from 'puppeteer'
const CHROME = `${process.env.HOME}/.cache/puppeteer/chrome/linux-150.0.7871.24/chrome-linux64/chrome`
const browser = await puppeteer.launch({
  executablePath: CHROME, headless: true,
  args: ['--no-sandbox', '--disable-setuid-sandbox', '--disable-dev-shm-usage'],
})
const page = await browser.newPage()
await page.setViewport({ width: 1440, height: 900 })
await page.evaluateOnNewDocument(() => {
  localStorage.setItem('auth-storage', JSON.stringify({
    state: { token: 'mock', userId: 'demo', tenantId: 't' }, version: 0,
  }))
})
await page.setRequestInterception(true)
page.on('request', req => {
  const url = req.url()
  if (!url.includes('/api/v1/')) { req.continue(); return }
  const respond = (s, j) => req.respond({ status: s, contentType: 'application/json', body: JSON.stringify(j) })
  if (url.endsWith('/api/v1/bids') && req.method() === 'GET') {
    return respond(200, { data: [{
      id: 'demo-bid-001', project_id: 'p', status: 'awaiting_review', current_step: 'awaiting_review',
      project_name: '深圳地铁 12 号线', industry: '轨道交通', total_chapters: 5, done_chapters: 3,
      created_at: new Date().toISOString(), updated_at: new Date().toISOString(), version: 1,
    }], meta: { count: 1 } })
  }
  if (url.includes('/bids/demo-bid-001') && !url.includes('/outline') && !url.includes('/chapters')) {
    return respond(200, { data: {
      id: 'demo-bid-001', project_id: 'p', status: 'awaiting_review', current_step: 'awaiting_review',
      project_name: '深圳地铁 12 号线', industry: '轨道交通', total_chapters: 5, done_chapters: 3,
      created_at: new Date().toISOString(), updated_at: new Date().toISOString(), version: 1,
    } })
  }
  if (url.includes('/outline') && req.method() === 'GET') {
    return respond(200, { data: [
      { id: 'ch-1', bid_job_id: 'demo-bid-001', parent_id: undefined, title: '项目背景与理解', level: 1, order_index: 0, chapter_type: 'background', target_word_count: 800, min_word_count: 600, writing_style: 'formal', priority: 'critical', status: 'approved', approved_at: '2026-07-05T10:23:00Z', approved_by: 'demo' },
      { id: 'ch-2', bid_job_id: 'demo-bid-001', parent_id: undefined, title: '总体技术方案', level: 1, order_index: 1, chapter_type: 'technical', target_word_count: 1500, min_word_count: 1200, writing_style: 'detailed', priority: 'critical', status: 'approved', approved_at: '2026-07-05T10:25:00Z', approved_by: 'demo' },
      { id: 'ch-3', bid_job_id: 'demo-bid-001', parent_id: 'ch-2', title: '系统架构设计', level: 2, order_index: 2, chapter_type: 'technical', target_word_count: 800, min_word_count: 600, writing_style: 'formal', priority: 'high', status: 'succeeded' },
    ] })
  }
  if (url.includes('/chapters/ch-3/content') && req.method() === 'GET') {
    return respond(200, { data: { chapter_spec_id: 'ch-3', version: 1, content_text: '# 系统架构', word_count: 600, min_word_met: true } })
  }
  return respond(200, { data: { ok: true } })
})
await page.goto('http://127.0.0.1:8888/bids/demo-bid-001', { waitUntil: 'load', timeout: 20000 })
await new Promise(r => setTimeout(r, 2000))
// Click on ch-3 (the succeeded one) in the tree
await page.evaluate(() => {
  const allRows = Array.from(document.querySelectorAll('div'))
  const ch3 = allRows.find(d => d.textContent && d.textContent.trim() === '系统架构设计')
  if (ch3) ch3.click()
})
await new Promise(r => setTimeout(r, 1000))
const diag = await page.evaluate(() => {
  const allButtons = Array.from(document.querySelectorAll('button'))
  return {
    inspectorApproveBtn: allButtons.some(b => b.textContent && b.textContent.includes('审核通过此章节')),
    inspectorGenerateBtn: allButtons.some(b => b.textContent && (b.textContent.includes('重新生成') || b.textContent.includes('生成此章节'))),
    inspectorShowsReviewMode: allButtons.some(b => b.textContent && b.textContent.includes('驳回（让 AI 重做）')),
  }
})
console.log(JSON.stringify(diag, null, 2))
await page.screenshot({ path: '/tmp/screenshots/workspace-ch3-review-mode.png' })
await browser.close()

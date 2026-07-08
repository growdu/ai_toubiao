// @ts-nocheck
import puppeteer from 'puppeteer'
const CHROME = `${process.env.HOME}/.cache/puppeteer/chrome/linux-150.0.7871.24/chrome-linux64/chrome`

const browser = await puppeteer.launch({
  executablePath: CHROME,
  headless: true,
  args: ['--no-sandbox', '--disable-setuid-sandbox', '--disable-dev-shm-usage'],
})

const page = await browser.newPage()
await page.setViewport({ width: 1440, height: 900 })

const errors = []
page.on('pageerror', e => errors.push(`PAGE: ${e.message}`))
page.on('console', m => { if (m.type() === 'error') errors.push(`CONSOLE: ${m.text()}`) })

await page.evaluateOnNewDocument(() => {
  localStorage.setItem('auth-storage', JSON.stringify({
    state: { token: 'mock-token-123', userId: 'demo-user-id', tenantId: 'demo-tenant-id' },
    version: 0,
  }))
})

await page.setRequestInterception(true)
page.on('request', req => {
  const url = req.url()
  if (!url.includes('/api/v1/')) { req.continue(); return }
  const respond = (status, json) => req.respond({ status, contentType: 'application/json', body: JSON.stringify(json) })
  if (url.endsWith('/api/v1/bids') && req.method() === 'GET') {
    return respond(200, { data: [{
      id: 'demo-bid-001', project_id: 'demo-proj-001', status: 'awaiting_review', current_step: 'awaiting_review',
      project_name: '深圳地铁 12 号线施工总承包投标', industry: '轨道交通',
      total_chapters: 5, done_chapters: 3,
      created_at: new Date().toISOString(), updated_at: new Date().toISOString(), version: 1,
    }], meta: { count: 1 } })
  }
  if (url.includes('/outline') && req.method() === 'GET') {
    return respond(200, { data: [
      { id: 'ch-1', bid_job_id: 'demo-bid-001', parent_id: undefined, title: '项目背景与理解', level: 1, order_index: 0, chapter_type: 'background', target_word_count: 800, min_word_count: 600, writing_style: 'formal', priority: 'critical', status: 'succeeded' },
      { id: 'ch-2', bid_job_id: 'demo-bid-001', parent_id: undefined, title: '总体技术方案', level: 1, order_index: 1, chapter_type: 'technical', target_word_count: 1500, min_word_count: 1200, writing_style: 'detailed', priority: 'critical', status: 'succeeded' },
      { id: 'ch-3', bid_job_id: 'demo-bid-001', parent_id: 'ch-2', title: '系统架构设计', level: 2, order_index: 2, chapter_type: 'technical', target_word_count: 800, min_word_count: 600, writing_style: 'formal', priority: 'high', status: 'succeeded' },
      { id: 'ch-4', bid_job_id: 'demo-bid-001', parent_id: 'ch-2', title: '技术选型与对比', level: 2, order_index: 3, chapter_type: 'technical', target_word_count: 600, min_word_count: 400, writing_style: 'formal', priority: 'high', status: 'running' },
      { id: 'ch-5', bid_job_id: 'demo-bid-001', parent_id: undefined, title: '项目实施计划', level: 1, order_index: 4, chapter_type: 'plan', target_word_count: 800, min_word_count: 600, writing_style: 'concise', priority: 'normal', status: 'planned' },
    ] })
  }
  if (url.includes('/chapters/') && url.endsWith('/content') && req.method() === 'GET') {
    return respond(200, { data: {
      chapter_spec_id: 'ch-1', version: 1,
      content_text: '# 项目背景\n\n这是测试内容。', word_count: 720, min_word_met: true,
      generated_by: 'GPT-4', llm_model: 'gpt-4-turbo', generation_duration_ms: 4200,
    } })
  }
  return respond(200, { data: { ok: true } })
})

await page.goto('http://127.0.0.1:8888/bids/demo-bid-001', { waitUntil: 'load', timeout: 20000 })
await new Promise(r => setTimeout(r, 3000))

const diag = await page.evaluate(() => {
  return {
    bodyTextLength: document.body.innerText.length,
    bodyTextPreview: document.body.innerText.slice(0, 500),
    h1Count: document.querySelectorAll('h1').length,
    h2Count: document.querySelectorAll('h2').length,
    asideCount: document.querySelectorAll('aside').length,
    buttonCount: document.querySelectorAll('button').length,
    navPresent: !!document.querySelector('nav'),
    chapterEditorPresent: !!document.querySelector('.font-mono'),
    bodyClasses: document.body.className,
    mainText: document.querySelector('main')?.innerText?.slice(0, 300) ?? null,
  }
})

console.log('=== Diagnostics ===')
console.log(JSON.stringify(diag, null, 2))
console.log('=== Errors ===')
errors.forEach(e => console.log(' -', e))

await browser.close()
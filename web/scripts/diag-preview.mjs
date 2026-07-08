// @ts-nocheck
import puppeteer from 'puppeteer'
const CHROME = `${process.env.HOME}/.cache/puppeteer/chrome/linux-150.0.7871.24/chrome-linux64/chrome`

const browser = await puppeteer.launch({
  executablePath: CHROME, headless: true,
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
    ] })
  }
  if (url.includes('/chapters/ch-1/content') && req.method() === 'GET') {
    return respond(200, { data: {
      chapter_spec_id: 'ch-1', version: 1,
      content_text: `# 一、项目背景

深圳市地铁 12 号线是深圳市轨道交通线网中重要的骨干线路，全长约 40 公里，设站 33 座。

## 1.1 项目定位

12 号线定位为**市域快线**，设计时速 120 km/h。

### 关键特性

- 完善深圳轨道交通线网
- 缓解西部走廊交通压力
- 支撑前海合作区开发

> 引用：项目建成后预计日均客流量将达到 **80 万人次**。

\`\`\`bash
echo "深圳地铁 12 号线"
\`\`\`

| 项目 | 标准 |
|------|------|
| 设计时速 | 120 km/h |
| 站台长度 | 186 m |
`,
      word_count: 720, min_word_met: true, generated_by: 'DeepSeek-V3', llm_model: 'deepseek-chat', generation_duration_ms: 4200,
    } })
  }
  return respond(200, { data: { ok: true } })
})

await page.goto('http://127.0.0.1:8888/bids/demo-bid-001', { waitUntil: 'load', timeout: 20000 })
await new Promise(r => setTimeout(r, 2000))

// Click 预览 tab in the editor
await page.evaluate(() => {
  const buttons = Array.from(document.querySelectorAll('button'))
  const previewBtn = buttons.find(b => b.textContent && b.textContent.trim() === '预览' && b.closest('.flex-1'))
  if (previewBtn) previewBtn.click()
})
await new Promise(r => setTimeout(r, 800))

// Diagnostics — check markdown rendered HTML is present
const diag = await page.evaluate(() => {
  const md = document.querySelector('.md-preview')
  return {
    mdPreviewPresent: !!md,
    mdPreviewInnerHTML: md ? md.innerHTML.slice(0, 800) : null,
    mdPreviewH1: md ? md.querySelectorAll('h1').length : 0,
    mdPreviewH2: md ? md.querySelectorAll('h2').length : 0,
    mdPreviewH3: md ? md.querySelectorAll('h3').length : 0,
    mdPreviewTable: md ? md.querySelectorAll('table').length : 0,
    mdPreviewCode: md ? md.querySelectorAll('code').length : 0,
    mdPreviewBlockquote: md ? md.querySelectorAll('blockquote').length : 0,
    mdPreviewStrong: md ? md.querySelectorAll('strong').length : 0,
    mdPreviewListItems: md ? md.querySelectorAll('li').length : 0,
  }
})
console.log('=== Markdown preview diagnostics ===')
console.log(JSON.stringify(diag, null, 2))
console.log('=== Errors ===')
errors.forEach(e => console.log(' -', e))

await page.screenshot({ path: '/tmp/screenshots/chapter-editor-preview.png' })
await browser.close()
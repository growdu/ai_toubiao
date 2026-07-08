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
  if (url.indexOf('/chapters/ch-3/content') !== -1 && req.method() === 'GET') {
    return respond(200, { data: { chapter_spec_id: 'ch-3', version: 1, content_text: '# test', word_count: 100, min_word_met: true, generated_by: 'GPT-4', llm_model: 'gpt-4-turbo', generation_duration_ms: 3200 } })
  }
  return respond(200, { data: { ok: true } })
})
await page.evaluateOnNewDocument(() => {
  localStorage.setItem('auth-storage', JSON.stringify({ state: { token: 'mock', userId: 'demo-user-id', tenantId: 't' }, version: 0 }))
})
await page.goto('http://127.0.0.1:8888/bids/demo-bid-001', { waitUntil: 'load', timeout: 20000 })
await new Promise(r => setTimeout(r, 2500))

// Select ch-3
await page.evaluate(() => {
  const rows = Array.from(document.querySelectorAll('div'))
  const ch3 = rows.find(d => d.textContent && d.textContent.trim() === '系统架构设计' && d.style && d.style.paddingLeft)
  if (ch3) ch3.click()
})
await new Promise(r => setTimeout(r, 800))

// Open single reject modal
await page.evaluate(() => {
  const btn = Array.from(document.querySelectorAll('button')).find(b => b.textContent && b.textContent.indexOf('驳回（让 AI 重做）') !== -1)
  if (btn) btn.click()
})
await new Promise(r => setTimeout(r, 500))

const singleDiag = await page.evaluate(() => {
  return {
    modalOpen: !!document.querySelector('[role="dialog"]'),
    modalTitle: document.querySelector('[role="dialog"] h2') ? document.querySelector('[role="dialog"] h2').textContent : null,
    modalHasReason: !!document.querySelector('[role="dialog"] textarea'),
    modalHasQuickPick: Array.from(document.querySelectorAll('[role="dialog"] button')).some(b => b.textContent && b.textContent.includes('内容不准确')),
    modalCancelBtn: Array.from(document.querySelectorAll('[role="dialog"] button')).some(b => b.textContent && b.textContent.trim() === '取消'),
    submitDisabled: (function () {
      const btn = Array.from(document.querySelectorAll('[role="dialog"] button')).find(b => b.textContent && (b.textContent.trim() === '驳回' || b.textContent.trim() === '全部驳回'))
      return btn ? btn.disabled : null
    })(),
  }
})
console.log('SINGLE MODAL:', JSON.stringify(singleDiag, null, 2))

// Test quick-pick chip auto-fills reason
await page.evaluate(() => {
  const chip = Array.from(document.querySelectorAll('[role="dialog"] button')).find(b => b.textContent && b.textContent.indexOf('缺证据') !== -1)
  if (chip) chip.click()
})
await new Promise(r => setTimeout(r, 300))

const afterChip = await page.evaluate(() => {
  const ta = document.querySelector('[role="dialog"] textarea')
  return {
    reasonValue: ta ? ta.value : null,
    submitEnabledNow: (function () {
      const btn = Array.from(document.querySelectorAll('[role="dialog"] button')).find(b => b.textContent && b.textContent.trim() === '驳回')
      return btn ? !btn.disabled : null
    })(),
  }
})
console.log('AFTER CHIP:', JSON.stringify(afterChip, null, 2))

// Close modal
await page.evaluate(() => {
  const cancelBtn = Array.from(document.querySelectorAll('[role="dialog"] button')).find(b => b.textContent && b.textContent.trim() === '取消')
  if (cancelBtn) cancelBtn.click()
})
await new Promise(r => setTimeout(r, 400))

// Now test batch modal
await page.evaluate(() => {
  const btn = Array.from(document.querySelectorAll('button')).find(b => b.textContent && b.textContent.indexOf('全部驳回') !== -1)
  if (btn) btn.click()
})
await new Promise(r => setTimeout(r, 500))

const batchDiag = await page.evaluate(() => {
  return {
    modalOpen: !!document.querySelector('[role="dialog"]'),
    modalTitle: document.querySelector('[role="dialog"] h2') ? document.querySelector('[role="dialog"] h2').textContent : null,
    submitBtnText: (function () {
      const btn = Array.from(document.querySelectorAll('[role="dialog"] button')).find(b => b.textContent && (b.textContent.trim() === '驳回' || b.textContent.trim() === '全部驳回'))
      return btn ? btn.textContent.trim() : null
    })(),
  }
})
console.log('BATCH MODAL:', JSON.stringify(batchDiag, null, 2))

// Test empty reason validation
await page.evaluate(() => {
  const ta = document.querySelector('[role="dialog"] textarea')
  if (ta) {
    const setter = Object.getOwnPropertyDescriptor(window.HTMLTextAreaElement.prototype, 'value').set
    setter.call(ta, '')
    ta.dispatchEvent(new Event('input', { bubbles: true }))
  }
})
await new Promise(r => setTimeout(r, 300))

const emptyReason = await page.evaluate(() => {
  const btn = Array.from(document.querySelectorAll('[role="dialog"] button')).find(b => b.textContent && b.textContent.trim() === '全部驳回')
  const hint = document.body.innerText.includes('驳回原因必填')
  return { submitDisabled: btn ? btn.disabled : null, hintShown: hint }
})
console.log('EMPTY REASON:', JSON.stringify(emptyReason, null, 2))

console.log('errors:', errors)
await browser.close()

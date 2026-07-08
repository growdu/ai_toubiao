// @ts-nocheck
import puppeteer from 'puppeteer'
import { mkdirSync } from 'node:fs'

const CHROME = process.env.HOME + '/.cache/puppeteer/chrome/linux-150.0.7871.24/chrome-linux64/chrome'
const OUT = '/tmp/screenshots'
mkdirSync(OUT, { recursive: true })

const BID_ID = 'demo-bid-001'
const NOW = new Date().toISOString()

const CHAPTERS = [
  { id: 'ch-1', bid_job_id: BID_ID, parent_id: undefined, title: '项目背景与理解', level: 1, order_index: 0, chapter_type: 'background', target_word_count: 800, min_word_count: 600, writing_style: 'formal', priority: 'critical', status: 'approved', approved_at: '2026-07-05T10:23:00Z', approved_by: 'demo-user-id' },
  { id: 'ch-2', bid_job_id: BID_ID, parent_id: undefined, title: '总体技术方案', level: 1, order_index: 1, chapter_type: 'technical', target_word_count: 1500, min_word_count: 1200, writing_style: 'detailed', priority: 'critical', status: 'approved', approved_at: '2026-07-05T10:25:00Z', approved_by: 'demo-user-id' },
  { id: 'ch-3', bid_job_id: BID_ID, parent_id: 'ch-2', title: '系统架构设计', level: 2, order_index: 2, chapter_type: 'technical', target_word_count: 800, min_word_count: 600, writing_style: 'formal', priority: 'high', status: 'succeeded' },
]

function bidMock(status) {
  return { id: BID_ID, project_id: 'p', status: status, current_step: status, project_name: '深圳地铁 12 号线施工总承包投标', industry: '轨道交通', total_chapters: 3, done_chapters: 3, created_at: NOW, updated_at: NOW, version: 1 }
}

async function setupPage(browser) {
  const page = await browser.newPage()
  await page.setViewport({ width: 1440, height: 900 })
  await page.setRequestInterception(true)
  page.on('request', function (req) {
    const url = req.url()
    if (url.indexOf('/api/v1/') === -1) { req.continue(); return }
    const respond = function (s, j) { req.respond({ status: s, contentType: 'application/json', body: JSON.stringify(j) }) }
    if (url.endsWith('/api/v1/bids') && req.method() === 'GET') {
      return respond(200, { data: [bidMock('awaiting_review')], meta: { count: 1 } })
    }
    if (url.indexOf('/bids/' + BID_ID) !== -1 && url.indexOf('/outline') === -1 && url.indexOf('/chapters') === -1 && url.indexOf('/transition') === -1 && url.indexOf('/pause') === -1 && url.indexOf('/resume') === -1) {
      return respond(200, { data: bidMock('awaiting_review') })
    }
    if (url.indexOf('/outline') !== -1 && req.method() === 'GET') {
      return respond(200, { data: CHAPTERS })
    }
    if (url.indexOf('/chapters/ch-3/content') !== -1 && req.method() === 'GET') {
      return respond(200, { data: { chapter_spec_id: 'ch-3', version: 1, content_text: '# 系统架构', word_count: 600, min_word_met: true, generated_by: 'GPT-4', llm_model: 'gpt-4-turbo', generation_duration_ms: 3200 } })
    }
    return respond(200, { data: { ok: true } })
  })
  await page.evaluateOnNewDocument(function () {
    localStorage.setItem('auth-storage', JSON.stringify({ state: { token: 'mock', userId: 'demo-user-id', tenantId: 't' }, version: 0 }))
  })
  return page
}

async function gotoWorkspace(page) {
  await page.goto('http://127.0.0.1:8888/bids/' + BID_ID, { waitUntil: 'load', timeout: 20000 })
  await new Promise(function (r) { setTimeout(r, 2500) })
}

const browser = await puppeteer.launch({
  executablePath: CHROME, headless: true,
  args: ['--no-sandbox', '--disable-setuid-sandbox', '--disable-dev-shm-usage'],
})

// 1. Single-chapter reject modal — select ch-3, click 驳回, click 内容不准确 chip
{
  const page = await setupPage(browser)
  await gotoWorkspace(page)
  await page.evaluate(function () {
    const rows = Array.from(document.querySelectorAll('div'))
    const ch3 = rows.find(function (d) { return d.textContent && d.textContent.trim() === '系统架构设计' && d.style && d.style.paddingLeft })
    if (ch3) ch3.click()
  })
  await new Promise(function (r) { setTimeout(r, 800) })
  await page.evaluate(function () {
    const btn = Array.from(document.querySelectorAll('button')).find(function (b) { return b.textContent && b.textContent.indexOf('驳回（让 AI 重做）') !== -1 })
    if (btn) btn.click()
  })
  await new Promise(function (r) { setTimeout(r, 600) })
  await page.evaluate(function () {
    const chip = Array.from(document.querySelectorAll('button')).find(function (b) { return b.textContent && b.textContent.indexOf('内容不准确') !== -1 })
    if (chip) chip.click()
  })
  await new Promise(function (r) { setTimeout(r, 400) })
  await page.screenshot({ path: OUT + '/workspace-reject-modal.png' })
  console.log('   workspace-reject-modal.png saved')
  await page.close()
}

// 2. Batch reject modal — click 全部驳回 in header, type a custom reason
{
  const page = await setupPage(browser)
  await gotoWorkspace(page)
  await page.evaluate(function () {
    const btn = Array.from(document.querySelectorAll('button')).find(function (b) { return b.textContent && b.textContent.indexOf('全部驳回') !== -1 })
    if (btn) btn.click()
  })
  await new Promise(function (r) { setTimeout(r, 600) })
  await page.evaluate(function () {
    const ta = document.querySelector('textarea')
    if (ta) {
      const setter = Object.getOwnPropertyDescriptor(window.HTMLTextAreaElement.prototype, 'value').set
      setter.call(ta, '业绩数据应改为 2024 年；招标编号与公告不一致')
      ta.dispatchEvent(new Event('input', { bubbles: true }))
    }
  })
  await new Promise(function (r) { setTimeout(r, 400) })
  await page.screenshot({ path: OUT + '/workspace-batch-reject-modal.png' })
  console.log('   workspace-batch-reject-modal.png saved')
  await page.close()
}

// 3. Approval timeline in Status tab — ch-1 (approved) selected by default
{
  const page = await setupPage(browser)
  await gotoWorkspace(page)
  await page.evaluate(function () {
    const tabs = Array.from(document.querySelectorAll('button'))
    const statusTab = tabs.find(function (b) { return b.textContent && b.textContent.trim() === '状态' })
    if (statusTab) statusTab.click()
  })
  await new Promise(function (r) { setTimeout(r, 600) })
  await page.screenshot({ path: OUT + '/workspace-status-timeline.png', clip: { x: 1100, y: 50, width: 340, height: 850 } })
  console.log('   workspace-status-timeline.png saved')
  await page.close()
}

// 4. Sticky toast — click 审核通过此章节 on ch-3, capture the persistent toast
{
  const page = await setupPage(browser)
  await gotoWorkspace(page)
  await page.evaluate(function () {
    const rows = Array.from(document.querySelectorAll('div'))
    const ch3 = rows.find(function (d) { return d.textContent && d.textContent.trim() === '系统架构设计' && d.style && d.style.paddingLeft })
    if (ch3) ch3.click()
  })
  await new Promise(function (r) { setTimeout(r, 800) })
  await page.evaluate(function () {
    const btn = Array.from(document.querySelectorAll('button')).find(function (b) { return b.textContent && b.textContent.indexOf('审核通过此章节') !== -1 })
    if (btn) btn.click()
  })
  await new Promise(function (r) { setTimeout(r, 1500) })
  await page.screenshot({ path: OUT + '/workspace-sticky-toast.png' })
  console.log('   workspace-sticky-toast.png saved')
  await page.close()
}

await browser.close()
console.log('done')

// @ts-nocheck
/**
 * Workspace screenshot suite — captures the protected pages (BidWorkspace,
 * ChapterEditor, ExportPage) by injecting mock auth into localStorage and
 * stubbing the API responses via puppeteer route interception. Real
 * backend isn't required; we synthesize a realistic happy-path bid with
 * 5 chapters so the workspace renders end-to-end including all the
 * features we just built (workflow stepper, drag handle, edit/preview
 * tab, HIL review chip, export preview).
 */
import puppeteer from 'puppeteer'
import { mkdirSync } from 'node:fs'

const CHROME = `${process.env.HOME}/.cache/puppeteer/chrome/linux-150.0.7871.24/chrome-linux64/chrome`
const OUT = '/tmp/screenshots'
mkdirSync(OUT, { recursive: true })

// ---------- Mock data ----------
const NOW = new Date().toISOString()
const BID_ID = 'demo-bid-001'
const PROJECT_ID = 'demo-proj-001'
const CHAPTERS = [
  {
    id: 'ch-1', bid_job_id: BID_ID, parent_id: undefined, title: '项目背景与理解', level: 1, order_index: 0,
    chapter_type: 'background', target_word_count: 800, min_word_count: 600, writing_style: 'formal', priority: 'critical', status: 'succeeded',
  },
  {
    id: 'ch-2', bid_job_id: BID_ID, parent_id: undefined, title: '总体技术方案', level: 1, order_index: 1,
    chapter_type: 'technical', target_word_count: 1500, min_word_count: 1200, writing_style: 'detailed', priority: 'critical', status: 'succeeded',
  },
  {
    id: 'ch-3', bid_job_id: BID_ID, parent_id: 'ch-2', title: '系统架构设计', level: 2, order_index: 2,
    chapter_type: 'technical', target_word_count: 800, min_word_count: 600, writing_style: 'formal', priority: 'high', status: 'succeeded',
  },
  {
    id: 'ch-4', bid_job_id: BID_ID, parent_id: 'ch-2', title: '技术选型与对比', level: 2, order_index: 3,
    chapter_type: 'technical', target_word_count: 600, min_word_count: 400, writing_style: 'formal', priority: 'high', status: 'running',
  },
  {
    id: 'ch-5', bid_job_id: BID_ID, parent_id: undefined, title: '项目实施计划', level: 1, order_index: 4,
    chapter_type: 'plan', target_word_count: 800, min_word_count: 600, writing_style: 'concise', priority: 'normal', status: 'planned',
  },
]

const CONTENT_BY_CHAPTER = {
  'ch-1': {
    chapter_spec_id: 'ch-1', version: 1,
    content_text:
`# 一、项目背景

深圳市地铁 12 号线是深圳市轨道交通线网中重要的骨干线路，全长约 40 公里，设站 33 座，其中换乘站 18 座。本项目作为深圳市"十四五"规划重点工程，对缓解深圳西部交通压力、推动前海合作区发展具有重要意义。

## 1.1 项目定位

12 号线定位为**市域快线**，设计时速 120 km/h，采用 8 节 A 型车编组。线路起于 **海上田园东站**，止于 **松岗站**，串联宝安、光明、南山三个行政区。

## 1.2 建设意义

- 完善深圳轨道交通线网
- 缓解西部走廊交通压力
- 支撑前海合作区开发
- 落实粤港澳大湾区战略

> 项目建成后预计日均客流量将达到 **80 万人次**，对沿线区域经济社会发展具有显著带动作用。`,
    word_count: 720, min_word_met: true, generated_by: 'DeepSeek-V3', llm_model: 'deepseek-chat', generation_duration_ms: 4200,
  },
  'ch-2': {
    chapter_spec_id: 'ch-2', version: 1,
    content_text:
`# 二、总体技术方案

## 2.1 设计原则

本项目技术方案遵循"**安全、可靠、先进、经济、绿色**"五大原则，重点解决以下关键技术问题：

1. 大跨度桥梁结构抗震设计
2. 复杂地质条件下盾构掘进
3. 多专业系统集成与接口管理

## 2.2 主要技术标准

| 项目 | 标准 |
|------|------|
| 设计时速 | 120 km/h |
| 最小曲线半径 | 800 m |
| 最大坡度 | 30‰ |
| 站台长度 | 186 m |

## 2.3 系统配置

- **车辆**：8 节 A 型车，DC 1500V 接触网供电
- **信号**：CBTC 移动闭塞
- **通信**：LTE-M 综合承载`,
    word_count: 1480, min_word_met: true, generated_by: 'GPT-4', llm_model: 'gpt-4-turbo', generation_duration_ms: 5800,
  },
  'ch-3': {
    chapter_spec_id: 'ch-3', version: 1,
    content_text:
`## 2.1.1 系统架构设计

### 总体架构

本项目采用 **"云-边-端"三层架构**：

\`\`\`
┌─────────────────────────────┐
│   中心云平台（综合监控、调度） │
└─────────────────────────────┘
                │ 安全隔离
┌─────────────────────────────┐
│  车站边缘节点（实时控制）    │
└─────────────────────────────┘
                │ 现场总线
┌─────────────────────────────┐
│  现场设备（信号、供电、消防）  │
└─────────────────────────────┘
\`\`\`

### 关键子系统

1. **综合监控系统（ISCS）**：集成 12 个子系统
2. **电力监控系统（PSCADA）**：实现智能调度
3. **环境与设备监控系统（BAS）**：节能优化`,
    word_count: 760, min_word_met: true, generated_by: 'Claude', llm_model: 'claude-3.5-sonnet', generation_duration_ms: 3900,
  },
}

function ok(data) {
  return { status: 200, contentType: 'application/json', body: JSON.stringify({ data }) }
}

function buildBid(status) {
  const done = CHAPTERS.filter(c => c.status === 'succeeded').length
  return {
    id: BID_ID, project_id: PROJECT_ID, status, current_step: status,
    project_name: '深圳地铁 12 号线施工总承包投标', industry: '轨道交通',
    total_chapters: CHAPTERS.length, done_chapters: done,
    created_at: NOW, updated_at: NOW, version: 1,
  }
}

async function routeApi(page) {
  await page.setRequestInterception(true)
  page.on('request', req => {
    const url = req.url()
    if (!url.includes('/api/v1/')) {
      req.continue()
      return
    }
    const method = req.method()
    let body = null
    const respond = (status, json) => {
      req.respond({ status, contentType: 'application/json', body: JSON.stringify(json) })
    }

    // /auth/me or /auth/profile — not used in this app but harmless
    if (url.endsWith('/auth/me') || url.endsWith('/auth/profile')) {
      return respond(200, { data: { id: 'demo-user', email: 'demo@local.test' } })
    }

    // Bids list
    if (url.endsWith('/api/v1/bids') && method === 'GET') {
      return respond(200, { data: [buildBid('awaiting_review')], meta: { count: 1 } })
    }

    // Single bid
    const bidMatch = url.match(/\/api\/v1\/bids\/([^/?]+)(\?|$)/)
    if (bidMatch && !url.includes('/outline') && !url.includes('/material') && !url.includes('/export') && !url.includes('/chapters') && !url.includes('/transition') && !url.includes('/pause') && !url.includes('/resume')) {
      return respond(200, { data: buildBid('awaiting_review') })
    }

    // Outline
    if (url.includes(`/api/v1/bids/${BID_ID}/outline`) && method === 'GET') {
      return respond(200, { data: CHAPTERS })
    }

    // Chapter content
    const chMatch = url.match(/\/api\/v1\/bids\/[^/]+\/chapters\/([^/]+)\/content(\?|$)/)
    if (chMatch && method === 'GET') {
      const cid = chMatch[1]
      const content = CONTENT_BY_CHAPTER[cid]
      if (content) return respond(200, { data: content })
      return respond(404, { error: { code: 'NOT_FOUND', message: 'no content yet' } })
    }
    if (chMatch && method === 'PUT') {
      return respond(200, { data: { ...(CONTENT_BY_CHAPTER[chMatch[1]] || {}), chapter_spec_id: chMatch[1], version: 2 } })
    }

    // Default: OK with empty
    if (method === 'POST' || method === 'PUT') {
      return respond(200, { data: { ok: true } })
    }
    return respond(200, { data: [] })
  })
}

async function injectAuthAndNavigate(browser, path, opts = {}) {
  const page = await browser.newPage()
  await page.setViewport({ width: opts.width ?? 1440, height: opts.height ?? 900 })
  await routeApi(page)

  // Inject auth before any app script runs. We do this by navigating
  // to the page first, then setting localStorage, then reloading.
  // localStorage needs the right origin so the redirect to / from /
  // doesn't wipe it.
  await page.evaluateOnNewDocument(() => {
    localStorage.setItem('auth-storage', JSON.stringify({
      state: { token: 'mock-token-123', userId: 'demo-user-id', tenantId: 'demo-tenant-id' },
      version: 0,
    }))
    localStorage.setItem('bidwriter-theme', opts.dark ? 'dark' : 'light')
  })

  const url = `http://127.0.0.1:8888${path}`
  console.log(`-> ${url}`)
  await page.goto(url, { waitUntil: 'load', timeout: 20000 })
  // Wait for react to mount the workspace (we look for a known element)
  try {
    await page.waitForSelector(opts.waitFor || 'main, aside, section', { timeout: 10000 })
  } catch (e) {
    console.log(`  selector timeout: ${e.message}`)
  }
  await new Promise(r => setTimeout(r, opts.delay ?? 1500))
  return page
}

const browser = await puppeteer.launch({
  executablePath: CHROME,
  headless: true,
  args: ['--no-sandbox', '--disable-setuid-sandbox', '--disable-dev-shm-usage'],
  defaultViewport: { width: 1440, height: 900 },
})

// 1. Workspace overview (BidWorkspace main 3-pane view)
{
  const page = await injectAuthAndNavigate(browser, `/bids/${BID_ID}`)
  await page.screenshot({ path: `${OUT}/workspace-overview.png` })
  console.log('   workspace-overview.png saved')
  await page.close()
}

// 2. Workspace awaiting_review zoom — focus on header + review chip
{
  const page = await injectAuthAndNavigate(browser, `/bids/${BID_ID}`)
  await new Promise(r => setTimeout(r, 800))
  // Crop to header area to show the HIL chip + review buttons
  await page.screenshot({ path: `${OUT}/workspace-hil-review.png`, clip: { x: 0, y: 0, width: 1440, height: 220 } })
  console.log('   workspace-hil-review.png saved')
  await page.close()
}

// 3. Chapter editor — markdown preview tab active (click preview on the first chapter)
{
  const page = await injectAuthAndNavigate(browser, `/bids/${BID_ID}`)
  await new Promise(r => setTimeout(r, 1500))
  // Click the 预览 tab in the editor toolbar
  await page.evaluate(() => {
    // Find any button containing 预览 in the editor body (not the header chips)
    const buttons = Array.from(document.querySelectorAll('button'))
    const previewBtn = buttons.find(b => b.textContent && b.textContent.trim() === '预览' && b.closest('.flex-1'))
    if (previewBtn) previewBtn.click()
  })
  await new Promise(r => setTimeout(r, 800))
  await page.screenshot({ path: `${OUT}/chapter-editor-preview.png` })
  console.log('   chapter-editor-preview.png saved')
  await page.close()
}

// 4. Chapter editor — edit mode (default) with toolbar visible
{
  const page = await injectAuthAndNavigate(browser, `/bids/${BID_ID}`)
  await new Promise(r => setTimeout(r, 1500))
  // Click 编辑 button on the right side of chapter 1 header to enter edit mode
  await page.evaluate(() => {
    const btn = Array.from(document.querySelectorAll('button')).find(b => b.textContent && b.textContent.trim() === '编辑内容')
    if (btn) btn.click()
  })
  await new Promise(r => setTimeout(r, 800))
  await page.screenshot({ path: `${OUT}/chapter-editor-edit.png` })
  console.log('   chapter-editor-edit.png saved')
  await page.close()
}

// 5. Export page with chapter preview expanded
{
  const page = await injectAuthAndNavigate(browser, `/bids/${BID_ID}/export`)
  await new Promise(r => setTimeout(r, 1200))
  // Click 展开 to reveal the chapter outline preview
  await page.evaluate(() => {
    const btn = Array.from(document.querySelectorAll('button')).find(b => b.textContent && b.textContent.includes('章节目录预览'))
    if (btn) btn.click()
  })
  await new Promise(r => setTimeout(r, 800))
  await page.screenshot({ path: `${OUT}/export-preview-expanded.png`, fullPage: true })
  console.log('   export-preview-expanded.png saved')
  await page.close()
}

// 6. Workspace dark mode
{
  const page = await injectAuthAndNavigate(browser, `/bids/${BID_ID}`, { dark: true, delay: 1500 })
  await page.screenshot({ path: `${OUT}/workspace-dark.png` })
  console.log('   workspace-dark.png saved')
  await page.close()
}

  // 7. Drag handle visible on ChapterTree — hover over a row to show the grip
{
  const page = await injectAuthAndNavigate(browser, `/bids/${BID_ID}`)
  await new Promise(r => setTimeout(r, 1500))
  // Hover over first chapter row to make drag handle visible
  await page.mouse.move(290, 200)
  await new Promise(r => setTimeout(r, 600))
  // Crop the left sidebar
  await page.screenshot({ path: `${OUT}/workspace-tree-hover.png`, clip: { x: 0, y: 0, width: 320, height: 600 } })
  console.log('   workspace-tree-hover.png saved')
  await page.close()
}

// 8. Workspace awaiting_review with approved chapters — synthesizes
// a state where 2 of 3 chapters are approved so the new "已审 2/3"
// chip, the "全部通过" batch button, and the emerald-approved tree
// badge are all visible together.
{
  const page = await browser.newPage()
  await page.setViewport({ width: 1440, height: 900 })
  await routeApi(page)
  await page.evaluateOnNewDocument(() => {
    localStorage.setItem('auth-storage', JSON.stringify({
      state: { token: 'mock-token-123', userId: 'demo-user-id', tenantId: 'demo-tenant-id' },
      version: 0,
    }))
  })
  // Override /outline and /chapters responses after routeApi by hooking
  // setRequestInterception more aggressively — easier to inject
  // a request listener that returns approved-state mock data.
  page.removeAllListeners('request')
  await page.setRequestInterception(true)
  page.on('request', req => {
    const url = req.url()
    const respond = (s, j) => req.respond({ status: s, contentType: 'application/json', body: JSON.stringify(j) })
    if (!url.includes('/api/v1/')) { req.continue(); return }
    if (url.endsWith('/api/v1/bids') && req.method() === 'GET') {
      return respond(200, { data: [{ id: BID_ID, project_id: PROJECT_ID, status: 'awaiting_review', current_step: 'awaiting_review', project_name: '深圳地铁 12 号线施工总承包投标', industry: '轨道交通', total_chapters: 5, done_chapters: 3, created_at: NOW, updated_at: NOW, version: 1 }], meta: { count: 1 } })
    }
    const bidMatch = url.match(/\/api\/v1\/bids\/([^/?]+)(\?|$)/)
    if (bidMatch && !url.includes('/outline') && !url.includes('/material') && !url.includes('/export') && !url.includes('/chapters') && !url.includes('/transition') && !url.includes('/pause') && !url.includes('/resume')) {
      return respond(200, { data: { id: BID_ID, project_id: PROJECT_ID, status: 'awaiting_review', current_step: 'awaiting_review', project_name: '深圳地铁 12 号线施工总承包投标', industry: '轨道交通', total_chapters: 5, done_chapters: 3, created_at: NOW, updated_at: NOW, version: 1 } })
    }
    if (url.includes(`/api/v1/bids/${BID_ID}/outline`) && req.method() === 'GET') {
      return respond(200, { data: [
        { id: 'ch-1', bid_job_id: BID_ID, parent_id: undefined, title: '项目背景与理解', level: 1, order_index: 0, chapter_type: 'background', target_word_count: 800, min_word_count: 600, writing_style: 'formal', priority: 'critical', status: 'approved', approved_at: '2026-07-05T10:23:00Z', approved_by: 'demo-user-id' },
        { id: 'ch-2', bid_job_id: BID_ID, parent_id: undefined, title: '总体技术方案', level: 1, order_index: 1, chapter_type: 'technical', target_word_count: 1500, min_word_count: 1200, writing_style: 'detailed', priority: 'critical', status: 'approved', approved_at: '2026-07-05T10:25:00Z', approved_by: 'demo-user-id' },
        { id: 'ch-3', bid_job_id: BID_ID, parent_id: 'ch-2', title: '系统架构设计', level: 2, order_index: 2, chapter_type: 'technical', target_word_count: 800, min_word_count: 600, writing_style: 'formal', priority: 'high', status: 'succeeded' },
        { id: 'ch-4', bid_job_id: BID_ID, parent_id: 'ch-2', title: '技术选型与对比', level: 2, order_index: 3, chapter_type: 'technical', target_word_count: 600, min_word_count: 400, writing_style: 'formal', priority: 'high', status: 'running' },
        { id: 'ch-5', bid_job_id: BID_ID, parent_id: undefined, title: '项目实施计划', level: 1, order_index: 4, chapter_type: 'plan', target_word_count: 800, min_word_count: 600, writing_style: 'concise', priority: 'normal', status: 'planned' },
      ] })
    }
    if (url.includes('/chapters/') && url.endsWith('/content') && req.method() === 'GET') {
      return respond(200, { data: {
        chapter_spec_id: 'ch-3', version: 1,
        content_text: '# 系统架构设计\n\n## 总体架构\n\n本章描述系统整体设计：\n\n- **分层架构**：表现层 / 业务层 / 数据层\n- **微服务拆分**：用户、订单、支付、库存 4 个核心服务\n- **消息队列**：Kafka 用于异步解耦',
        word_count: 580, min_word_met: true, generated_by: 'GPT-4', llm_model: 'gpt-4-turbo', generation_duration_ms: 3200,
      } })
    }
    return respond(200, { data: { ok: true } })
  })
  await page.goto(`http://127.0.0.1:8888/bids/${BID_ID}`, { waitUntil: 'load', timeout: 20000 })
  await new Promise(r => setTimeout(r, 2000))
  // Full workspace overview with the new emerald approved badges
  await page.screenshot({ path: `${OUT}/workspace-approved-overview.png` })
  console.log('   workspace-approved-overview.png saved')
  // Crop to header to show the new "已审 2/3" + "全部通过" + "进入审计" buttons
  await page.screenshot({ path: `${OUT}/workspace-approved-header.png`, clip: { x: 0, y: 0, width: 1440, height: 220 } })
  console.log('   workspace-approved-header.png saved')
  await page.close()
}

await browser.close()
console.log('done')
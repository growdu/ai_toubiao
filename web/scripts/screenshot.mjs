// @ts-nocheck
import puppeteer from 'puppeteer'
import { mkdirSync, statSync, writeFileSync } from 'node:fs'

const CHROME = `${process.env.HOME}/.cache/puppeteer/chrome/linux-150.0.7871.24/chrome-linux64/chrome`
const OUT = '/tmp/screenshots'

const shots = [
  { name: 'landing',     path: '/',          selector: 'section', wait: 2500 },
  { name: 'login',       path: '/login',     selector: 'form',     wait: 2500 },
  { name: 'register',    path: '/register',  selector: 'form',     wait: 2500 },
]

mkdirSync(OUT, { recursive: true })

const browser = await puppeteer.launch({
  executablePath: CHROME,
  headless: true,
  args: ['--no-sandbox', '--disable-setuid-sandbox', '--disable-dev-shm-usage'],
  defaultViewport: { width: 1440, height: 900, deviceScaleFactor: 1 },
})

// Capture console errors for debugging
for (const s of shots) {
  const page = await browser.newPage()
  await page.setViewport({ width: 1440, height: 900 })
  const errors = []
  page.on('pageerror', e => errors.push(`PAGEERROR: ${e.message}`))
  page.on('console', m => { if (m.type() === 'error') errors.push(`CONSOLE: ${m.text()}`) })
  const url = `http://127.0.0.1:8888${s.path}`
  console.log(`-> ${url}`)
  // Use 'load' (faster than networkidle0), then explicitly wait for a real selector
  await page.goto(url, { waitUntil: 'load', timeout: 20000 })
  try {
    await page.waitForSelector(s.selector, { timeout: 10000 })
  } catch (e) {
    console.log(`  selector "${s.selector}" not found: ${e.message}`)
  }
  await new Promise(r => setTimeout(r, s.wait))
  // Verify content actually rendered
  const bodyText = await page.evaluate(() => document.body.innerText.length)
  console.log(`   body text length: ${bodyText}`)
  await page.screenshot({ path: `${OUT}/${s.name}.png`, fullPage: s.name === 'landing' })
  console.log(`   saved ${OUT}/${s.name}.png`)
  if (errors.length > 0) {
    console.log(`   ERRORS: ${errors.join(' | ')}`)
  }
  await page.close()
}

// Mobile preview
const mobile = await browser.newPage()
await mobile.setViewport({ width: 414, height: 896, deviceScaleFactor: 2, isMobile: true })
await mobile.goto('http://127.0.0.1:8888/', { waitUntil: 'load', timeout: 20000 })
await mobile.waitForSelector('section', { timeout: 10000 })
await new Promise(r => setTimeout(r, 2000))
await mobile.screenshot({ path: `${OUT}/landing-mobile.png`, fullPage: true })
console.log(`   saved ${OUT}/landing-mobile.png`)

// Dark mode login
const dark = await browser.newPage()
await dark.setViewport({ width: 1440, height: 900 })
await dark.evaluateOnNewDocument(() => {
  localStorage.setItem('bidwriter-theme', 'dark')
})
await dark.goto('http://127.0.0.1:8888/login', { waitUntil: 'load', timeout: 20000 })
await dark.waitForSelector('form', { timeout: 10000 })
await new Promise(r => setTimeout(r, 2000))
await dark.screenshot({ path: `${OUT}/login-dark.png` })
console.log(`   saved ${OUT}/login-dark.png`)

await browser.close()
console.log('done')
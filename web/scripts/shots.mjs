import { chromium } from 'playwright'
import fs from 'node:fs'

const BASE = process.env.BASE_URL || 'http://127.0.0.1:9090'
const OUT = process.env.SHOT_DIR || './shots'
fs.mkdirSync(OUT, { recursive: true })

const pages = [
  ['dashboard', '/'],
  ['models', '/models'],
  ['services', '/services'],
  ['apps', '/apps'],
  ['containers', '/containers'],
  ['observability', '/observability'],
  ['gateway', '/gateway'],
  ['admin', '/admin'],
]

const report = { consoleErrors: [], pageErrors: [], failedRequests: [], shots: [] }

const browser = await chromium.launch({ channel: 'msedge', headless: true })
const context = await browser.newContext({ viewport: { width: 1680, height: 1050 }, deviceScaleFactor: 1 })
const page = await context.newPage()
page.on('console', (msg) => {
  if (msg.type() === 'error') report.consoleErrors.push(msg.text().slice(0, 400))
})
page.on('pageerror', (err) => report.pageErrors.push(String(err).slice(0, 400)))
page.on('requestfailed', (req) => report.failedRequests.push(`${req.method()} ${req.url()} ${req.failure()?.errorText || ''}`))

await page.goto(`${BASE}/login`, { waitUntil: 'networkidle' })
await page.screenshot({ path: `${OUT}/00-login.png`, fullPage: false })

await page.fill('input[placeholder="admin"]', 'admin')
await page.fill('input[placeholder="请输入密码"]', 'admin123')
await page.click('button:has-text("进入控制台")')
await page.waitForURL(`${BASE}/`, { timeout: 20000 })
await page.waitForTimeout(3500)

for (const [name, path] of pages) {
  if (path !== '/') {
    await page.goto(`${BASE}${path}`, { waitUntil: 'networkidle' })
  }
  await page.waitForTimeout(name === 'observability' ? 3000 : 1800)
  await page.screenshot({ path: `${OUT}/${String(report.shots.length + 1).padStart(2, '0')}-${name}.png`, fullPage: false })
  report.shots.push(name)
}

// Deploy drawer and service drawer interactions
await page.goto(`${BASE}/models`, { waitUntil: 'networkidle' })
await page.waitForTimeout(1200)
const deployBtn = page.locator('.catalog-card button:has-text("一键部署")').first()
if (await deployBtn.count()) {
  await deployBtn.click()
  await page.waitForTimeout(1200)
  await page.screenshot({ path: `${OUT}/10-deploy-drawer.png` })
  await page.keyboard.press('Escape')
}

await page.goto(`${BASE}/services`, { waitUntil: 'networkidle' })
await page.waitForTimeout(1200)
const card = page.locator('.svc-card').first()
if (await card.count()) {
  await card.click()
  await page.waitForTimeout(1800)
  await page.screenshot({ path: `${OUT}/11-service-drawer.png` })
}

// Copilot drawer
await page.goto(`${BASE}/`, { waitUntil: 'networkidle' })
await page.waitForTimeout(1000)
const copilotBtn = page.locator('button:has-text("AI 运维助手")').first()
if (await copilotBtn.count()) {
  await copilotBtn.click()
  await page.waitForTimeout(1200)
  await page.screenshot({ path: `${OUT}/12-copilot.png` })
}

fs.writeFileSync(`${OUT}/report.json`, JSON.stringify(report, null, 2))
console.log(JSON.stringify(report, null, 2))
await browser.close()

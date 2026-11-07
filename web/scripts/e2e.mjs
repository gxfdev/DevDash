import { chromium } from 'playwright'
import fs from 'node:fs'

const BASE = process.env.BASE_URL || 'http://127.0.0.1:9090'
const OUT = './shots'
fs.mkdirSync(OUT, { recursive: true })

const report = { steps: [], consoleErrors: [], pageErrors: [], failedRequests: [], ok: false }
const browser = await chromium.launch({ channel: 'msedge', headless: true })
const context = await browser.newContext({ viewport: { width: 1680, height: 1050 } })
const page = await context.newPage()
page.on('console', (m) => { if (m.type() === 'error') report.consoleErrors.push(m.text().slice(0, 300)) })
page.on('pageerror', (e) => report.pageErrors.push(String(e).slice(0, 300)))
page.on('requestfailed', (r) => report.failedRequests.push(r.method() + ' ' + r.url()))

try {
  await page.goto(BASE + '/login', { waitUntil: 'networkidle' })
  await page.fill('input[placeholder="admin"]', 'admin')
  await page.fill('input[placeholder="请输入密码"]', 'admin123')
  await page.click('button:has-text("进入控制台")')
  await page.waitForURL(BASE + '/', { timeout: 20000 })
  report.steps.push('login ok')

  // --- one-click deploy through the UI (custom path keeps the button stable) ---
  await page.goto(BASE + '/models', { waitUntil: 'networkidle' })
  await page.waitForTimeout(800)
  const card = page.locator('.catalog-card', { hasText: 'Qwen2.5 Coder' }).first()
  await card.locator('button:has-text("自定义部署")').click()
  await page.waitForTimeout(900)
  const uniqueName = 'e2e-coder-' + String(Date.now()).slice(-6)
  await page.locator('.deploy input').first().fill(uniqueName)
  await page.click('button:has-text("开始部署")')
  await page.waitForURL(/\/services\?focus=/, { timeout: 20000 })
  await page.waitForTimeout(1200)
  const titleText = await page.locator('.sd-title').first().innerText({ timeout: 10000 })
  report.steps.push('deploy drawer opened: ' + titleText.replace(/\s+/g, ' ').slice(0, 80))
  await page.waitForSelector('.sd-title:has-text("运行中")', { timeout: 40000 })
  report.steps.push('deployed service reached running state')
  await page.screenshot({ path: OUT + '/20-deployed-service.png' })

  // --- container terminal ---
  await page.getByText('容器终端', { exact: true }).click()
  await page.waitForTimeout(2500)
  report.steps.push('container terminal mounted: ' + ((await page.locator('.xterm').count()) > 0))
  await page.screenshot({ path: OUT + '/21-service-terminal.png' })
  await page.keyboard.press('Escape')
  await page.waitForTimeout(600)

  // --- cleanup the test service before exercising Copilot ---
  await page.evaluate(async () => {
    const token = localStorage.getItem('aidash_token')
    const headers = { Authorization: 'Bearer ' + token }
    const res = await fetch('/api/services', { headers })
    const body = await res.json()
    for (const s of (body.data?.services || [])) {
      if (s.name.startsWith('e2e-coder-')) {
        await fetch('/api/services/' + s.id + '?keep_volumes=1', { method: 'DELETE', headers })
      }
    }
  })
  await page.waitForTimeout(800)

  // --- copilot ---
  await page.goto(BASE + '/', { waitUntil: 'networkidle' })
  await page.click('button:has-text("AI 运维助手")')
  await page.waitForTimeout(900)
  await page.fill('.cp-input textarea', '现在服务器状态如何？请简短回答')
  await page.click('.cp-input button')
  await page.waitForSelector('.cp-msg.assistant .bubble:has-text("演示模型回复")', { timeout: 40000 })
  report.steps.push('copilot replied')
  await page.screenshot({ path: OUT + '/22-copilot-reply.png' })

  // --- cleanup test services ---
  const removed = await page.evaluate(async () => {
    const token = localStorage.getItem('aidash_token')
    const headers = { Authorization: 'Bearer ' + token }
    const res = await fetch('/api/services', { headers })
    const body = await res.json()
    const targets = (body.data?.services || []).filter((s) => s.name.startsWith('e2e-coder-'))
    for (const s of targets) {
      await fetch('/api/services/' + s.id + '?keep_volumes=1', { method: 'DELETE', headers })
    }
    return targets.length
  })
  report.steps.push('cleaned up ' + removed + ' e2e service(s)')
  report.ok = true
} catch (e) {
  report.error = String(e).slice(0, 800)
  await page.screenshot({ path: OUT + '/99-failure.png' }).catch(() => {})
} finally {
  fs.writeFileSync(OUT + '/e2e-report.json', JSON.stringify(report, null, 2))
  console.log(JSON.stringify(report, null, 2))
  await browser.close()
}

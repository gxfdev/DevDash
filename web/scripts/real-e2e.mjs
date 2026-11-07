import { chromium } from 'playwright'
import fs from 'node:fs'
const BASE = 'http://127.0.0.1:9090'
fs.mkdirSync('./shots-real', { recursive: true })
const report = { consoleErrors: [], pageErrors: [] }
const browser = await chromium.launch({ channel: 'msedge', headless: true })
const page = await (await browser.newContext({ viewport: { width: 1680, height: 1050 } })).newPage()
page.on('console', (m) => { if (m.type() === 'error') report.consoleErrors.push(m.text().slice(0, 300)) })
page.on('pageerror', (e) => report.pageErrors.push(String(e).slice(0, 300)))
await page.goto(BASE + '/login', { waitUntil: 'networkidle' })
await page.fill('input[placeholder="admin"]', 'admin')
await page.fill('input[placeholder="请输入密码"]', 'admin123')
await page.click('button:has-text("进入控制台")')
await page.waitForURL(BASE + '/', { timeout: 20000 })
await page.waitForTimeout(2500)
await page.screenshot({ path: './shots-real/real-dashboard.png' })

await page.goto(BASE + '/hub', { waitUntil: 'networkidle' })
await page.waitForTimeout(1500)
await page.screenshot({ path: './shots-real/real-hub.png' })

await page.goto(BASE + '/services', { waitUntil: 'networkidle' })
await page.waitForTimeout(1500)
await page.screenshot({ path: './shots-real/real-services.png' })

await page.locator('.svc-card').first().click()
await page.waitForTimeout(1800)
await page.screenshot({ path: './shots-real/real-service-detail.png' })

await page.getByText('模型体验', { exact: true }).click()
await page.waitForTimeout(800)
await page.fill('.chat .n-input input, .chat input', '你好，请用两句话介绍你自己')
await page.click('.chat button:has-text("发送")')
await page.waitForSelector('.chat-msg.assistant .bubble', { timeout: 120000 })
await page.waitForTimeout(800)
await page.screenshot({ path: './shots-real/real-playground.png' })

await page.goto(BASE + '/gateway', { waitUntil: 'networkidle' })
await page.waitForTimeout(2000)
await page.screenshot({ path: './shots-real/real-gateway.png' })
fs.writeFileSync('./shots-real/report.json', JSON.stringify(report, null, 2))
console.log(JSON.stringify(report, null, 2))
await browser.close()
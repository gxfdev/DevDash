import { chromium } from 'playwright'
import fs from 'node:fs'
const BASE = 'http://127.0.0.1:9090'
fs.mkdirSync('./shots-real', { recursive: true })
const browser = await chromium.launch({ channel: 'msedge', headless: true })
const page = await (await browser.newContext({ viewport: { width: 1680, height: 1050 } })).newPage()
await page.goto(BASE + '/login', { waitUntil: 'networkidle' })
await page.fill('input[placeholder="admin"]', 'admin')
await page.fill('input[placeholder="请输入密码"]', 'admin123')
await page.click('button:has-text("进入控制台")')
await page.waitForURL(BASE + '/', { timeout: 20000 })
await page.goto(BASE + '/hub', { waitUntil: 'networkidle' })
await page.waitForTimeout(2500)
await page.screenshot({ path: './shots-real/real-hub.png' })
console.log('saved hub screenshot')
await browser.close()
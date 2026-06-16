import { test, expect } from '@playwright/test'

const baseURL = process.env.SKILLFORGE_E2E_BASE_URL || 'http://localhost:8082'

test('frontend smoke: navigation, auth, and key pages do not throw', async ({ page }) => {
  const consoleErrors = []
  const pageErrors = []
  const networkErrors = []

  page.on('console', msg => {
    if (msg.type() === 'error') {
      consoleErrors.push(msg.text())
    }
  })
  page.on('pageerror', err => {
    pageErrors.push(err.message)
  })
  page.on('requestfailed', req => {
    const url = req.url()
    const errorText = req.failure()?.errorText || ''
    if (url.startsWith(baseURL) && errorText !== 'net::ERR_ABORTED') {
      networkErrors.push(`${req.method()} ${url} ${errorText}`)
    }
  })
  page.on('response', res => {
    const url = res.url()
    if (url.startsWith(baseURL) && res.status() >= 400) {
      networkErrors.push(`${res.status()} ${url}`)
    }
  })

  async function visit(path, expectedText) {
    const res = await page.goto(`${baseURL}${path}`, { waitUntil: 'domcontentloaded' })
    expect(res?.status(), path).toBeLessThan(500)
    await expect(page.locator('#root')).toBeVisible()
    if (expectedText) {
      await expect(page.getByText(expectedText, { exact: false }).first()).toBeVisible({ timeout: 10000 })
    }
    await expect(page.getByText('Something went wrong', { exact: false })).toHaveCount(0)
  }

  const publicPages = [
    ['/', 'SkillForge'],
    ['/explore', 'Explore'],
    ['/trending', 'Trending'],
    ['/categories', 'Categories'],
    ['/activity', 'Activity'],
    ['/install', 'Install'],
    ['/login', 'Sign in'],
  ]
  for (const [path, text] of publicPages) {
    await visit(path, text)
  }

  await page.goto(`${baseURL}/login`, { waitUntil: 'domcontentloaded' })
  await page.fill('#username', 'admin')
  await page.fill('#password', 'changeme')
  await page.getByRole('button', { name: /sign in/i }).click()
  await expect(page).not.toHaveURL(/\/login$/, { timeout: 10000 })
  await expect.poll(() => page.evaluate(() => localStorage.getItem('sf_token'))).toBeTruthy()

  const authenticatedPages = [
    ['/account/tokens', 'Tokens'],
    ['/account/settings', 'Account'],
    ['/account/security', 'Account security'],
    ['/account/notifications', 'Notifications'],
    ['/admin', 'Admin'],
    ['/admin/audit', 'Audit'],
    ['/publish', 'Publish'],
    ['/namespace/admin', 'admin'],
    ['/namespace/admin/settings', 'Settings'],
    ['/namespace/admin/webhooks', 'Webhooks'],
    ['/namespace/admin/insights', 'Insights'],
    ['/namespace/admin/collections', 'Collections'],
  ]
  for (const [path, text] of authenticatedPages) {
    await visit(path, text)
  }

  expect(pageErrors, 'page errors').toEqual([])
  expect(networkErrors, 'local network errors').toEqual([])
  expect(consoleErrors, 'console errors').toEqual([])
})

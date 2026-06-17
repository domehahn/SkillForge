import { test, expect } from '@playwright/test'

const baseURL = process.env.SKILLFORGE_E2E_BASE_URL || 'http://localhost:8082'

async function authEnabled(page) {
  const res = await page.request.get(`${baseURL}/api/v1/capabilities`)
  expect(res.status()).toBeLessThan(500)
  if (!res.ok()) return true
  const capabilities = await res.json()
  return capabilities.auth_enabled !== false
}

test('frontend smoke: navigation, auth, and key pages do not throw', async ({ page }) => {
  const consoleErrors = []
  const pageErrors = []
  const networkErrors = []
  let isAuthEnabled = true

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
      if (!isAuthEnabled && res.status() === 401 && url === `${baseURL}/api/v1/tokens`) {
        return
      }
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

  isAuthEnabled = await authEnabled(page)
  if (isAuthEnabled) {
    await page.goto(`${baseURL}/login`, { waitUntil: 'domcontentloaded' })
    await page.fill('#username', 'admin')
    await page.fill('#password', 'changeme')
    await page.getByRole('button', { name: /sign in/i }).click()
    await expect(page).not.toHaveURL(/\/login$/, { timeout: 10000 })
  } else {
    await page.goto(`${baseURL}/`, { waitUntil: 'domcontentloaded' })
    await page.evaluate(() => {
      localStorage.setItem('sf_token', 'e2e-auth-disabled')
      localStorage.setItem('sf_user', 'admin')
      localStorage.setItem('sf_role', 'admin')
    })
  }
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
  ].filter(([path]) => isAuthEnabled || path !== '/account/tokens')
  for (const [path, text] of authenticatedPages) {
    await visit(path, text)
  }

  expect(pageErrors, 'page errors').toEqual([])
  expect(networkErrors, 'local network errors').toEqual([])
  expect(consoleErrors, 'console errors').toEqual([])
})

import { test, expect } from '@playwright/test'

test('mermaid block renders to SVG and no console/network errors', async ({ page }) => {
  const consoleMessages: Array<{ type: string; text: string }> = []
  const failedRequests: Array<{ url: string; error?: string }> = []
  let mermaidStatus: number | undefined

  page.on('console', msg => consoleMessages.push({ type: msg.type(), text: msg.text() }))
  page.on('requestfailed', req => failedRequests.push({ url: req.url(), error: req.failure()?.errorText }))
  page.on('response', res => {
    if (res.url().endsWith('/app/vendor/mermaid.min.js')) mermaidStatus = res.status()
  })

  // Open the guide file directly where we added the mermaid block.
  await page.goto('/file/docs/guide.md')

  // Wait for the mermaid container to be added to the DOM.
  await page.waitForSelector('.mermaid', { timeout: 5000 })

  // Wait for Mermaid to render an SVG inside the mermaid container.
  // Use a Locator so Playwright assertions work as intended.
  const svgLocator = page.locator('.mermaid svg')
  await expect(svgLocator).toBeVisible({ timeout: 10_000 })

  // Assertions: vendored mermaid served, no network failures, no console errors.
  expect(mermaidStatus).toBe(200)
  expect(failedRequests.length).toBe(0)
  const errors = consoleMessages.filter(m => m.type === 'error')
  expect(errors.length).toBe(0)
})

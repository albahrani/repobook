import { test, expect } from '@playwright/test'

test('loads README on startup and shows nav tree', async ({ page }) => {
  await page.goto('/')
  await expect(page.locator('#crumb')).toContainText('README.md')
  await expect(page.locator('#nav')).toContainText('docs')
  await expect(page.locator('#nav')).toContainText('guide.md')
})

test('navigation updates document and TOC', async ({ page }) => {
  await page.goto('/')
  await page.getByRole('link', { name: 'guide.md' }).click()
  await expect(page.locator('#crumb')).toContainText('docs/guide.md')
  await expect(page.locator('#toc')).toContainText('Part 1')
})

test('navigation highlights current file', async ({ page }) => {
  await page.goto('/')
  await page.getByRole('link', { name: 'guide.md' }).click()
  const link = page.locator('#nav a.nav-link[href="/file/docs/guide.md"]')
  await expect(link).toBeVisible()
  await expect(link.locator('..')).toHaveClass(/is-active/)
})

test('collapse button hides directories', async ({ page }) => {
  await page.goto('/')
  const btn = page.locator('#navToggle')
  await expect(btn).toBeVisible()
  await expect(btn).toHaveText('Collapse')

  const guide = page.locator('#nav a.nav-link[href="/file/docs/guide.md"]')
  await expect(guide).toBeVisible()

  await btn.click()
  await expect(btn).toHaveText('Expand')
  await expect(guide).toBeHidden()

  await page.locator('#nav summary.nav-dir-title', { hasText: 'docs' }).click()
  await expect(guide).toBeVisible()
})

test('markdown links are clickable and route internally', async ({ page }) => {
  await page.goto('/')
  await page.locator('#viewer').getByRole('link', { name: 'Guide', exact: true }).click()
  await expect(page.locator('#crumb')).toContainText('docs/guide.md')
  await page.locator('#viewer').getByRole('link', { name: 'Note', exact: true }).click()
  await expect(page.locator('#crumb')).toContainText('docs/note.md')
  await expect(page.locator('#viewer')).toContainText('Alpha appears here')
})

test('external autolinks open in new tab', async ({ page }) => {
  await page.goto('/')
  const a = page.locator('#viewer a[href="https://kubernetes.io/docs/reference/kubectl/"]')
  await expect(a).toHaveCount(1)
  await expect(a).toHaveAttribute('target', '_blank')
  await expect(a).toHaveAttribute('rel', /noopener/)
})

test('search finds results and clicking opens file', async ({ page }) => {
  await page.goto('/')

	// Search backend is optional; skip if rg is missing on this machine.
	const probe = await page.request.get('/api/search?q=Alpha')
	test.skip(probe.status() === 501, 'ripgrep (rg) not installed')

  await page.locator('#search').fill('Alpha')
  await expect(page.locator('#results')).toBeVisible()
  await expect(page.locator('#results')).toContainText('docs/note.md')
  await page.locator('#results').locator('a.result', { hasText: 'docs/note.md' }).click()
  await expect(page.locator('#crumb')).toContainText('docs/note.md')
})

test('code blocks are syntax highlighted', async ({ page }) => {
  await page.goto('/file/docs/guide.md')
  await expect(page.locator('.chroma')).toHaveCount(1)
})

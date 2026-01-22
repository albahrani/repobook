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

test('markdown links are clickable and route internally', async ({ page }) => {
  await page.goto('/')
  await page.locator('#viewer').getByRole('link', { name: 'Guide', exact: true }).click()
  await expect(page.locator('#crumb')).toContainText('docs/guide.md')
  await page.locator('#viewer').getByRole('link', { name: 'Note', exact: true }).click()
  await expect(page.locator('#crumb')).toContainText('docs/note.md')
  await expect(page.locator('#viewer')).toContainText('Alpha appears here')
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

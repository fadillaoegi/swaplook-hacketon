import { test, expect } from '@playwright/test'

test('shopper shortlist, comparison, simulator preview and persistent choice', async ({ page }) => {
  await page.goto('/')
  await page.getByRole('button', { name: 'Find alternatives', exact: true }).click()
  await expect(page.locator('.product-card')).toHaveCount(3)
  await expect(page.getByText('3 alternatives · Size M · Up to $60')).toBeVisible()
  await expect(page.getByText('All within your limits')).toBeAttached()
  const cards = page.locator('.product-card')
  await cards.nth(0).getByRole('checkbox').check()
  await cards.nth(1).getByRole('checkbox').check()
  await page.getByRole('button', { name: 'Compare tops' }).click()
  await expect(page.getByRole('dialog')).toBeVisible()
  await expect(page.locator('.comparison-item')).toHaveCount(2)
  await page.getByRole('button', { name: 'Close dialog' }).click()
  await cards.nth(0).getByRole('button', { name: 'Take another look' }).click()
  const illustration = page.getByRole('button', { name: 'Show demo illustration' })
  if (await illustration.count()) {
    await illustration.click()
    await expect(page.getByText('Simulator illustration ready.', { exact: true })).toBeVisible()
    await expect(page.getByText('SIMULATOR · ORIGINAL ILLUSTRATION', { exact: true })).toBeVisible()
  }
  await page.getByRole('button', { name: 'Close dialog' }).click()
  await page.screenshot({ path: test.info().outputPath('shortlist.png'), fullPage: true })
  await cards.nth(0).getByRole('button', { name: 'Choose this top' }).click()
  await expect(page.getByText('A NEW FAVORITE, FOUND.')).toBeVisible()
  await expect(page.getByText('Saved demo decision. This is not a purchase or a stock reservation.')).toBeVisible()
  await page.reload()
  await expect(page.getByText('A NEW FAVORITE, FOUND.')).toBeVisible()
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
})

test('zero match preserves constraints and shopper can leave feedback', async ({ page }) => {
  await page.goto('/')
  await page.getByRole('spinbutton', { name: 'Maximum budget in dollars' }).fill('1')
  await page.getByRole('button', { name: 'Find alternatives', exact: true }).click()
  await expect(page.getByText('No tops meet all your must-haves.')).toBeVisible()
  await expect(page.locator('.product-card')).toHaveCount(0)
  await expect(page.getByRole('spinbutton', { name: 'Maximum budget in dollars' })).toHaveValue('1')
  await page.getByRole('button', { name: 'Leave feedback' }).click()
  await page.getByRole('button', { name: 'Save feedback' }).click()
  await expect(page.getByText('THANKS FOR THE HONEST FEEDBACK.')).toBeVisible()
})

test('stock change through merchant API rejects stale shopper choice', async ({ page }) => {
  const token = process.env.MERCHANT_TOKEN
  test.skip(!token, 'MERCHANT_TOKEN required for inventory mutation check')
  const headers = { Authorization: `Bearer ${token}` }
  const before = await page.request.get('/api/catalog')
  expect(before.ok()).toBe(true)
  const products = await before.json()
  const originalQuantity = products.find((p: { id: string }) => p.id === 'SL-002').stock.M
  try {
    const set = await page.request.patch('/api/merchant/stock', { headers, data: { product_id: 'SL-002', size: 'M', quantity: 5 } })
    expect(set.ok()).toBe(true)
    await page.goto('/')
    await page.getByRole('button', { name: 'Find alternatives', exact: true }).click()
    const target = page.locator('.product-card').filter({ hasText: 'Harbor Stripe Popover' })
    await expect(target).toBeVisible()
    const changed = await page.request.patch('/api/merchant/stock', { headers, data: { product_id: 'SL-002', size: 'M', quantity: 0 } })
    expect(changed.ok()).toBe(true)
    await target.getByRole('button', { name: 'Choose this top' }).click()
    await expect(page.getByRole('alert')).toContainText('Harbor Stripe Popover just sold out in size M')
    await expect(target.getByRole('button', { name: 'Choose this top' })).toBeDisabled()
    await expect(page.getByRole('button', { name: 'Refresh alternatives' })).toBeVisible()
    await expect(page.getByText('A NEW FAVORITE, FOUND.')).toHaveCount(0)
    await page.goto('/merchant')
    await page.getByLabel('MERCHANT ACCESS TOKEN').fill(token!)
    await page.getByRole('button', { name: 'Open studio' }).click()
    await expect(page.getByText('DEMO OBSERVATIONS ONLY')).toBeVisible()
    await expect(page.getByText('Recent decisions.', { exact: true })).toBeVisible()
  } finally {
    await page.request.patch('/api/merchant/stock', { headers, data: { product_id: 'SL-002', size: 'M', quantity: originalQuantity } })
  }
})

test('side-by-side view tries every alternative on the same demo source', async ({ page }) => {
  await page.goto('/')
  await page.getByRole('button', { name: 'Find alternatives', exact: true }).click()
  await page.getByRole('button', { name: 'See them side by side' }).click()
  const dialog = page.getByRole('dialog')
  await expect(dialog.locator('.comparison-item')).toHaveCount(3)
  await dialog.getByRole('button', { name: /Try all on/ }).click()
  await expect(dialog.getByText('SIMULATOR ILLUSTRATION', { exact: true })).toHaveCount(3, { timeout: 20000 })
  await expect(dialog.getByText('3/3 preview slots used in this session.', { exact: false })).toBeVisible()
})

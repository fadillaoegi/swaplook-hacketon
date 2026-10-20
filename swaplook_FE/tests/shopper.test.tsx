import { describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { App, coverCrop } from '../src/presentation/App'
import { SwapLook } from '../src/application/swaplook'
import { APIError } from '../src/application/gateway'
import type { Gateway } from '../src/application/gateway'
import type { Constraints, Decision, Job, Product, Shortlist } from '../src/domain/models'

const c: Constraints = { original_id: 'SL-001', size: 'M', budget: 6000, color: '', material: '' }
const products: Product[] = ['The Everyday Shirt', 'Sunday Cotton Shirt', 'Soft Structure Shirt', 'The Linen Edit'].map((name, i) => ({ id: `SL-00${i + 1}`, name, price: 4800 - i * 200, currency: 'USD', color: 'cream', material: 'cotton', cut: 'relaxed', category: 'tops', image: `/catalog/SL-00${i + 1}.svg`, stock: { XS: 4, S: 4, M: i === 0 ? 0 : 4, L: 4, XL: 4 } }))
const list = (constraints = c): Shortlist => ({ constraints, candidates: products.slice(1).map(product => ({ product, reasons: ['Size M in stock', 'Within your budget', 'Same color: cream'], tradeoffs: [] })) })
const chosen = (product_id: string, kind: Decision['kind'] = 'selected', reason = ''): Decision => ({ id: 'demo-decision', product_id, original_id: 'SL-001', size: 'M', price: 4600, kind, reason, demo: true, created_at: new Date().toISOString() })
function fixture() {
  const session = { shortlist: null, decision: null, demo: true, expires_at: new Date(Date.now() + 86400000).toISOString() }
  const job: Job = { id: 'demo-job', product_id: 'SL-002', size: 'M', source_id: 'demo', provider: 'simulator', status: 'pending', message: '', polls: 0 }
  const gateway = {
    catalog: vi.fn(async () => products), health: vi.fn(async () => ({ renderer: 'simulator' })),
    state: vi.fn(async () => ({ session, jobs: [], photos: [], renderer: 'simulator' })), start: vi.fn(async () => session),
    shortlist: vi.fn(async (constraints: Constraints) => list(constraints)),
    decide: vi.fn(async (id: string, kind: Decision['kind'], reason: string) => chosen(id, kind, reason)),
    preview: vi.fn(async () => job), poll: vi.fn(async () => ({ ...job, status: 'success', image: '/catalog/preview-SL-002.svg' })),
    reconcile: vi.fn(async () => ({ ...job, status: 'success', reconciliations: 1, image: '/catalog/preview-SL-002.svg' })),
    upload: vi.fn(async () => ({ id: 'demo-photo', created_at: new Date().toISOString() })), deletePhotos: vi.fn(async () => {}),
    summary: vi.fn(async () => ({ sessions: 1, choices: 0, rejections: 0, decisions: [], demo: true })), stock: vi.fn(async () => {}), setAI: vi.fn(async () => {}), demoSummary: vi.fn(async () => ({ sessions: 7, choices: 3, rejections: 1, decisions: [], demo: true, previews: { provider: 'youcam-v4', available: true, enabled: true, generations: 5, today: 1, succeeded: 3, failed: 2, reused: 0, average_seconds: 18, daily_limit: 25, total_limit: 150, remaining_today: 24, remaining_total: 145 } })),
  } satisfies Gateway
  return { gateway, app: new SwapLook(gateway) }
}
async function find(user: ReturnType<typeof userEvent.setup>) {
  const button = await screen.findByRole('button', { name: 'Find alternatives', exact: true })
  await waitFor(() => expect((button as HTMLButtonElement).disabled).toBe(false))
  await user.click(button)
  await screen.findByText('A few we think you’ll love.')
}

describe('shopper DOM behavior with explicitly injected fixtures', () => {
  it('submits constraints, compares the intended products and saves a selected SKU', async () => {
    const { app, gateway } = fixture(); const user = userEvent.setup(); render(<App app={app}/>); await find(user)
    expect(gateway.shortlist).toHaveBeenCalledWith(c)
    expect(screen.getAllByRole('article')).toHaveLength(3)
    await user.click(screen.getByRole('checkbox', { name: 'Compare Sunday Cotton Shirt' }))
    await user.click(screen.getByRole('checkbox', { name: 'Compare Soft Structure Shirt' }))
    await user.click(screen.getByRole('button', { name: 'Compare tops' }))
    const dialog = screen.getByRole('dialog')
    expect(within(dialog).getByText('Sunday Cotton Shirt')).toBeTruthy()
    expect(within(dialog).getByText('Soft Structure Shirt')).toBeTruthy()
    await user.click(within(dialog).getAllByRole('button', { name: 'Choose this top' })[0])
    await screen.findByText('A NEW FAVORITE, FOUND.')
    expect(gateway.decide).toHaveBeenCalledWith('SL-002', 'selected', '')
    expect(screen.getByText('Saved demo decision. This is not a purchase or a stock reservation.')).toBeTruthy()
  })
  it('does not enable a stale shortlist after preferences change', async () => {
    const { app } = fixture(); const user = userEvent.setup(); render(<App app={app}/>); await find(user)
    await user.selectOptions(screen.getByRole('combobox', { name: 'Required color' }), 'cream')
    expect(screen.getByText('Your preferences changed. Find alternatives again to update your shortlist.')).toBeTruthy()
    expect(screen.getAllByRole('button', { name: 'Choose this top' }).every(b => (b as HTMLButtonElement).disabled)).toBe(true)
  })
  it('shows zero matches and saves rejection feedback without relaxing the budget', async () => {
    const { app, gateway } = fixture(); gateway.shortlist.mockImplementation(async constraints => ({ constraints, candidates: [] })); const user = userEvent.setup(); render(<App app={app}/>)
    const button = await screen.findByRole('button', { name: 'Find alternatives', exact: true }); await waitFor(() => expect((button as HTMLButtonElement).disabled).toBe(false))
    const budget = screen.getByRole('spinbutton', { name: 'Maximum budget in dollars' }); await user.clear(budget); await user.type(budget, '1'); await user.click(button)
    await screen.findByText('No tops meet all your must-haves.')
    expect(gateway.shortlist).toHaveBeenCalledWith({ ...c, budget: 100 })
    expect((budget as HTMLInputElement).value).toBe('1')
    await user.click(screen.getByRole('button', { name: 'Leave feedback' })); await user.click(screen.getByRole('button', { name: 'Save feedback' }))
    await screen.findByText('THANKS FOR THE HONEST FEEDBACK.')
    expect(gateway.decide).toHaveBeenCalledWith('', 'rejected', 'no_match')
  })
  it('keeps provider provenance visible and submits only on explicit preview action', async () => {
    const { app, gateway } = fixture(); const user = userEvent.setup(); render(<App app={app}/>); await find(user)
    expect(gateway.preview).not.toHaveBeenCalled()
    await user.click(screen.getAllByRole('button', { name: 'Take another look' })[0])
    expect(screen.getByText('Illustration mode')).toBeTruthy()
    expect(screen.queryByLabelText('Upload authorized person photo')).toBeNull()
    await user.click(screen.getByRole('button', { name: 'Show demo illustration' }))
    await screen.findByText('Simulator illustration ready.', {}, { timeout: 4000 })
    expect(gateway.preview).toHaveBeenCalledTimes(1)
    expect(gateway.preview).toHaveBeenCalledWith('SL-002', 'demo')
    expect(screen.getByText('SIMULATOR · ORIGINAL ILLUSTRATION')).toBeTruthy()
  })
  it('keeps a failed stock recheck visible without showing a saved decision', async () => {
    const { app, gateway } = fixture(); gateway.decide.mockRejectedValue(new APIError('Availability changed. Refresh alternatives.', 409)); const user = userEvent.setup(); render(<App app={app}/>); await find(user)
    await user.click(screen.getAllByRole('button', { name: 'Choose this top' })[0]); expect((await screen.findByRole('alert')).textContent).toContain('Availability changed')
    expect(screen.queryByText('A NEW FAVORITE, FOUND.')).toBeNull()
  })
  it('checks a timed-out preview without submitting another generation', async () => {
    const { app, gateway } = fixture()
    gateway.poll.mockResolvedValue({ id: 'demo-job', product_id: 'SL-002', size: 'M', source_id: 'demo', provider: 'simulator', status: 'timed_out', message: 'Automatic polling paused.', polls: 60, image: '' })
    const user = userEvent.setup(); render(<App app={app}/>); await find(user)
    await user.click(screen.getAllByRole('button', { name: 'Take another look' })[0])
    await user.click(screen.getByRole('button', { name: 'Show demo illustration' }))
    const check = await screen.findByRole('button', { name: 'Check the same preview' }, { timeout: 4000 })
    expect(gateway.reconcile).not.toHaveBeenCalled()
    await user.click(check)
    await screen.findByText('Simulator illustration ready.')
    expect(gateway.reconcile).toHaveBeenCalledWith('demo-job')
    expect(gateway.preview).toHaveBeenCalledTimes(1)
  })
  it('shows API startup failure instead of inventing a demo catalog', async () => {
    const { app, gateway } = fixture(); gateway.catalog.mockRejectedValue(new Error('Cannot reach SwapLook.')); render(<App app={app}/>)
    expect((await screen.findByRole('alert')).textContent).toContain('Cannot reach SwapLook.')
    expect(screen.queryAllByRole('article')).toHaveLength(0)
    expect(screen.getByRole('button', { name: 'Reconnect' })).toBeTruthy()
  })
  it('requires merchant authentication and keeps the token out of browser storage', async () => {
    window.history.replaceState({}, '', '/merchant'); const { app, gateway } = fixture(); gateway.summary.mockRejectedValueOnce(new APIError('Enter a valid merchant access token.', 401)); const user = userEvent.setup(); render(<App app={app}/>)
    const input = await screen.findByLabelText('MERCHANT ACCESS TOKEN'); await user.type(input, 'synthetic-token'); await user.click(screen.getByRole('button', { name: 'Open studio' }))
    expect((await screen.findByRole('alert')).textContent).toContain('valid merchant access token')
    expect(screen.queryByText('DEMO OBSERVATIONS ONLY')).toBeNull()
    await user.click(screen.getByRole('button', { name: 'Open studio' })); await screen.findByText('DEMO OBSERVATIONS ONLY')
    expect(localStorage.length).toBe(0); expect(sessionStorage.length).toBe(0)
    await user.click(screen.getByRole('button', { name: 'Lock studio' })); expect((screen.getByLabelText('MERCHANT ACCESS TOKEN') as HTMLInputElement).value).toBe('')
  })
  it('marks a top that sold out at selection and offers a refreshed shortlist without saving', async () => {
    const { app, gateway } = fixture(); gateway.decide.mockRejectedValueOnce(new APIError('This top is no longer available in your size.', 409, 'unavailable')); const user = userEvent.setup(); render(<App app={app}/>); await find(user)
    await user.click(screen.getAllByRole('button', { name: 'Choose this top' })[0])
    expect((await screen.findByRole('alert')).textContent).toContain('Sunday Cotton Shirt just sold out in size M')
    expect(screen.queryByText('A NEW FAVORITE, FOUND.')).toBeNull()
    expect((screen.getAllByRole('button', { name: 'Choose this top' })[0] as HTMLButtonElement).disabled).toBe(true)
    await user.click(screen.getByRole('button', { name: 'Refresh alternatives' }))
    await waitFor(() => expect(gateway.shortlist).toHaveBeenCalledTimes(2))
    await waitFor(() => expect(screen.queryByText(/just sold out/)).toBeNull())
  })
  it('tries every compared top on the same source from the side-by-side view', async () => {
    const { app, gateway } = fixture(); let n = 0
    gateway.preview.mockImplementation(async (product_id: string) => ({ id: `job-${++n}`, product_id, size: 'M', source_id: 'demo', provider: 'simulator', status: 'pending', message: '', polls: 0 }))
    gateway.poll.mockImplementation(async (id: string) => ({ id, product_id: `SL-00${Number(id.slice(4)) + 1}`, size: 'M', source_id: 'demo', provider: 'simulator', status: 'success', message: '', polls: 1, image: '/catalog/preview-SL-002.svg' }))
    const user = userEvent.setup(); render(<App app={app}/>); await find(user)
    await user.click(screen.getByRole('button', { name: 'See them side by side' }))
    const dialog = screen.getByRole('dialog'); expect(within(dialog).getAllByRole('button', { name: 'Choose this top' })).toHaveLength(3)
    await user.click(within(dialog).getByRole('button', { name: /Try all on/ }))
    await waitFor(() => expect(gateway.preview).toHaveBeenCalledTimes(3))
    expect(gateway.preview.mock.calls.map(c => c[1])).toEqual(['demo', 'demo', 'demo'])
    await waitFor(() => expect(within(dialog).getAllByText('SIMULATOR ILLUSTRATION')).toHaveLength(3), { timeout: 4000 })
  })
  it('offers model choices, photo upload and a live selfie in actual mode', async () => {
    const { app, gateway } = fixture(); gateway.health.mockResolvedValue({ renderer: 'youcam-v4' })
    gateway.state.mockResolvedValue({ session: { shortlist: null, decision: null, demo: true, expires_at: new Date(Date.now() + 86400000).toISOString() }, jobs: [], photos: [], renderer: 'youcam-v4', quota: { remaining_today: 5, client_remaining: 4 }, models: [{ id: 'model-1', name: 'Model 1', note: 'Consented portrait' }, { id: 'model-2', name: 'Model 2', note: '' }] })
    const user = userEvent.setup(); render(<App app={app}/>); await find(user)
    await user.click(screen.getAllByRole('button', { name: 'Take another look' })[0])
    expect(screen.getByRole('radio', { name: /Model 1/ }).getAttribute('aria-checked')).toBe('true')
    await user.click(screen.getByRole('radio', { name: /Model 2/ }))
    expect(screen.getByRole('button', { name: 'Show on Model 2' })).toBeTruthy()
    expect(screen.getByText(/5 AI previews left today/)).toBeTruthy()
    await user.click(screen.getByRole('tab', { name: /Upload photo/ }))
    expect(screen.getByText('Head to waist, both shoulders visible')).toBeTruthy()
    expect((screen.getByLabelText('Upload authorized person photo') as HTMLInputElement).disabled).toBe(true)
    await user.click(screen.getByRole('tab', { name: /Take a selfie/ }))
    expect((screen.getByRole('button', { name: 'Open camera' }) as HTMLButtonElement).disabled).toBe(true)
    await user.click(screen.getByRole('checkbox', { name: /permission and consent/ }))
    await user.click(screen.getByRole('button', { name: 'Open camera' }))
    expect((await screen.findByRole('alert')).textContent).toContain('camera is not available')
    expect(gateway.preview).not.toHaveBeenCalled(); expect(gateway.upload).not.toHaveBeenCalled()
  })
  it('lets the merchant switch AI previews off to save credits', async () => {
    window.history.replaceState({}, '', '/merchant'); const { app, gateway } = fixture()
    const previews = { provider: 'youcam-v4', available: true, enabled: true, generations: 4, today: 1, succeeded: 2, failed: 2, reused: 0, average_seconds: 20, daily_limit: 25, total_limit: 150, remaining_today: 24, remaining_total: 146 }
    gateway.summary.mockResolvedValueOnce({ sessions: 1, choices: 0, rejections: 0, decisions: [], demo: true, previews }).mockResolvedValueOnce({ sessions: 1, choices: 0, rejections: 0, decisions: [], demo: true, previews: { ...previews, enabled: false } })
    const user = userEvent.setup(); render(<App app={app}/>)
    await user.type(await screen.findByLabelText('MERCHANT ACCESS TOKEN'), 'synthetic-token'); await user.click(screen.getByRole('button', { name: 'Open studio' }))
    const toggle = await screen.findByRole('switch', { name: 'AI previews' }); expect(toggle.getAttribute('aria-checked')).toBe('true')
    await user.click(toggle)
    await waitFor(() => expect(gateway.setAI).toHaveBeenCalledWith('synthetic-token', false))
    await screen.findByText(/AI previews are off/)
    expect(screen.getByRole('switch', { name: 'AI previews' }).getAttribute('aria-checked')).toBe('false')
  })
  it('shows a read-only demo dashboard without a token and keeps changes locked', async () => {
    window.history.replaceState({}, '', '/merchant'); const { app, gateway } = fixture(); const user = userEvent.setup(); render(<App app={app}/>)
    await user.click(await screen.findByRole('button', { name: 'View demo dashboard' }))
    await screen.findByText('VIEW ONLY · DEMO DASHBOARD')
    expect(gateway.summary).not.toHaveBeenCalled()
    expect((screen.getByRole('switch', { name: 'AI previews' }) as HTMLButtonElement).disabled).toBe(true)
    expect((screen.getByRole('button', { name: /Save stock/ }) as HTMLButtonElement).disabled).toBe(true)
    expect(screen.getByText('Owner token required to make changes.')).toBeTruthy()
    await user.click(screen.getAllByRole('button', { name: /Unlock with token/ })[0])
    await user.type(screen.getByLabelText('OWNER TOKEN'), 'synthetic-token'); await user.click(screen.getByRole('button', { name: 'Unlock' }))
    await screen.findByText('DEMO OBSERVATIONS ONLY')
    expect(gateway.summary).toHaveBeenCalledWith('synthetic-token')
    expect((screen.getByRole('button', { name: /Save stock/ }) as HTMLButtonElement).disabled).toBe(false)
  })
  it('crops a landscape laptop camera frame to the visible portrait selfie frame', () => {
    expect(coverCrop(1280, 720, 4 / 5)).toEqual({ x: 352, y: 0, w: 576, h: 720 })
    expect(coverCrop(1920, 1080, 4 / 5)).toEqual({ x: 528, y: 0, w: 864, h: 1080 })
    expect(coverCrop(1080, 1920, 4 / 5)).toEqual({ x: 0, y: 285, w: 1080, h: 1350 })
  })
})

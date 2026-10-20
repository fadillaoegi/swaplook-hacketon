import type { Gateway } from '../application/gateway'
import { APIError } from '../application/gateway'
import type { Constraints, Decision, Job, Photo, Product, Session, Shortlist, Size, State, Summary } from '../domain/models'

export class HttpGateway implements Gateway {
  private async request<T>(path: string, method = 'GET', body?: unknown, token?: string): Promise<T> {
    const headers: Record<string, string> = {}
    if (body && !(body instanceof FormData)) headers['Content-Type'] = 'application/json'
    if (token) headers.Authorization = `Bearer ${token}`
    let res: Response
    try { res = await fetch(`/api${path}`, { method, headers, credentials: 'same-origin', body: body instanceof FormData ? body : body ? JSON.stringify(body) : undefined, signal: AbortSignal.timeout(145000) }) }
    catch { throw new Error('Cannot reach SwapLook. Check that the API is running, then try again.') }
    if (!res.ok) { const data = await res.json().catch(() => ({})); throw new APIError(data.error || 'The request could not be completed.', res.status, data.code || '') }
    return res.status === 204 ? undefined as T : res.json()
  }
  catalog = () => this.request<Product[]>('/catalog')
  health = () => this.request<{ renderer: string }>('/health')
  state = () => this.request<State>('/session')
  start = () => this.request<Session>('/session', 'POST')
  shortlist = (c: Constraints) => this.request<Shortlist>('/shortlist', 'POST', c)
  decide = (product_id: string, kind: Decision['kind'], reason: string) => this.request<Decision>('/decisions', 'POST', { product_id, kind, reason })
  preview = (product_id: string, source_id: string) => this.request<Job>('/previews', 'POST', { product_id, source_id })
  poll = (id: string) => this.request<Job>(`/previews/${encodeURIComponent(id)}/poll`, 'POST')
  reconcile = (id: string) => this.request<Job>(`/previews/${encodeURIComponent(id)}/reconcile`, 'POST')
  upload = (file: File, consent: boolean) => { const data = new FormData(); data.set('photo', file); data.set('consent', String(consent)); return this.request<Photo>('/photos', 'POST', data) }
  deletePhotos = () => this.request<void>('/photos', 'DELETE')
  summary = (token: string) => this.request<Summary>('/merchant/summary', 'GET', undefined, token)
  demoSummary = () => this.request<Summary>('/merchant/demo-summary')
  stock = (token: string, product_id: string, size: Size, quantity: number) => this.request<void>('/merchant/stock', 'PATCH', { product_id, size, quantity }, token)
  setAI = (token: string, enabled: boolean) => this.request<void>('/merchant/ai', 'PATCH', { enabled }, token)
}

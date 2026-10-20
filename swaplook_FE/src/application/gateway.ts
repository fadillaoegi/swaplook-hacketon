import type { Constraints, Decision, Job, Photo, Product, Session, Shortlist, Size, State, Summary } from '../domain/models'

export class APIError extends Error { constructor(message: string, readonly status: number, readonly code = '') { super(message) } }

export interface Gateway {
  catalog(): Promise<Product[]>
  health(): Promise<{ renderer: string }>
  state(): Promise<State>
  start(): Promise<Session>
  shortlist(c: Constraints): Promise<Shortlist>
  decide(product_id: string, kind: Decision['kind'], reason: string): Promise<Decision>
  preview(product_id: string, source_id: string): Promise<Job>
  poll(id: string): Promise<Job>
  reconcile(id: string): Promise<Job>
  upload(file: File, consent: boolean): Promise<Photo>
  deletePhotos(): Promise<void>
  summary(token: string): Promise<Summary>
  demoSummary(): Promise<Summary>
  stock(token: string, product_id: string, size: Size, quantity: number): Promise<void>
  setAI(token: string, enabled: boolean): Promise<void>
}

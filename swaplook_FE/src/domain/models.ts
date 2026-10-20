export type Size = 'XS' | 'S' | 'M' | 'L' | 'XL'
export const sizes: Size[] = ['XS', 'S', 'M', 'L', 'XL']
export interface Product { id: string; name: string; price: number; currency: 'USD'; color: string; material: string; cut: string; category: string; image: string; stock: Record<Size, number> }
export interface Constraints { original_id: string; size: Size; budget: number; color: string; material: string }
export interface Candidate { product: Product; reasons: string[]; tradeoffs: string[] }
export interface Shortlist { constraints: Constraints; candidates: Candidate[] }
export interface Decision { id: string; product_id: string; original_id: string; size: Size; price: number; kind: 'selected' | 'rejected'; reason: string; demo: boolean; created_at: string }
export interface Session { shortlist: Shortlist | null; decision: Decision | null; demo: boolean; expires_at: string }
export interface Job { id: string; product_id: string; size: Size; source_id: string; provider: string; status: string; image?: string; message: string; polls: number; reconciliations?: number; reused?: boolean }
export interface Photo { id: string; created_at: string }
export interface Quota { remaining_today: number; client_remaining: number }
export interface ModelInfo { id: string; name: string; note: string }
export interface State { session: Session; jobs: Job[]; photos: Photo[]; renderer: string; quota?: Quota; models?: ModelInfo[] }
export interface Substitution { original_id: string; product_id: string; count: number }
export interface PreviewStats { provider: string; available?: boolean; enabled?: boolean; generations: number; today: number; succeeded: number; failed: number; reused: number; average_seconds: number; daily_limit: number; total_limit: number; remaining_today: number; remaining_total: number }
export interface Summary { sessions: number; choices: number; rejections: number; decisions: Decision[]; demo: boolean; substitutions?: Substitution[]; previews?: PreviewStats }
export const money = (cents: number) => new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD', maximumFractionDigits: cents % 100 ? 2 : 0 }).format(cents / 100)

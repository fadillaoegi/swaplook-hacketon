import type { Gateway } from './gateway'
import type { Constraints } from '../domain/models'

export class SwapLook {
  constructor(readonly gateway: Gateway) {}
  async initialize() {
    const [catalog, health] = await Promise.all([this.gateway.catalog(), this.gateway.health()])
    return { catalog, renderer: health.renderer }
  }
  find(constraints: Constraints) {
    if (!Number.isInteger(constraints.budget) || constraints.budget < 1 || constraints.budget > 100000) throw new Error('Enter a valid budget between $0.01 and $1,000.')
    return this.gateway.shortlist(constraints)
  }
}

# SwapLook implementation

- Work in this project; preserve existing work. Discuss in Indonesian, write UI and submission material in English.
- Stack: React/Vite/TypeScript/Tailwind, Go, PostgreSQL. Domain has no HTTP, SQL or provider dependencies. Application owns ports; adapters implement them; composition roots wire dependencies.
- Read docs/PROGRESS.md before continuing. Update it with tested evidence and remaining blockers after every completed task, and before context/token limits run out, so another agent can continue. Keep its "Current handoff" section at the top current.
- One demo boutique, tops, 12–20 SKUs. Stock, size, budget and required attributes are hard constraints; return at most three alternatives. Never silently relax them.
- Money is integer USD cents. Recheck stock and price transactionally on final selection. Selection is neither payment nor inventory reservation.
- YouCam credentials stay server-side and ignored. Never log keys, signed URLs or photos. Explicit simulator is not actual integration evidence. Never fall back to it after provider errors.
- Bind previews to session, source image, SKU and size. Require consent for upload; enforce private image access and application deletion. Do not promise provider-wide deletion or physical fit.
- Merchant writes require server authentication. Separate demo events from genuine observations. Never label choices as revenue.
- Run Go tests/vet, TypeScript/build and meaningful PostgreSQL/browser checks for affected flows. No publication or submission is authorized by the handoff alone.

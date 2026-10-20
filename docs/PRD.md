# Product requirements

The main task is choosing an available substitute when the requested size is out of stock.

1. Show the source top, chosen size and current availability. A shopper can change source product and size.
2. Require a positive, bounded budget in USD cents. Size, budget, category, required color/material must all match. Exclude the source SKU. If the original size is available, explain that substitution is unnecessary. Zero candidates is an explicit state.
3. Rank eligible products by shared catalog attributes, then price difference and SKU for stable ties. Explain matching attributes and changed material/color without inferring from images.
4. Persist a shortlist before rendering. Bind each job to its exact SKU, size, source and session. Maximum three jobs per session; duplicate requests return the existing job. Simulator never spends units and never claims a real try-on.
5. A selection must belong to that session's shortlist, still meet constraints and be in stock at selection time. Repeated requests are idempotent. Record the current price but do not reserve stock.
6. Merchant writes and event reads require a server-validated bearer token. Show demo events separately and never call them sales.
7. Require consent before photo upload. JPEG/PNG only, bounded dimensions/size; private access by session cookie; delete application copies on request. Explain provider retention separately.
8. Provide mobile/desktop layouts, visible errors/loading/zero matches, keyboard controls and a clear restart path.

Acceptance evidence is recorded in PROGRESS.md. Real YouCam and user research remain unmet until observed directly.

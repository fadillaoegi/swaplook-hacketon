# SwapLook

**A different top. The same feeling.** Find an in-stock alternative, then see it on a person before you choose.

When a shopper's favorite top is sold out in their size, most stores show "out of stock" and lose the moment. SwapLook turns that dead end into a short, honest shortlist: at most three alternatives that respect the shopper's size, budget and must-have attributes, each with a plain-language reason and trade-off. The shopper can try every alternative on the same photo with **YouCam AI Clothes Try-On**, compare them side by side, and choose one. The final choice rechecks live stock and price, so a top that sold out in the meantime is never "chosen" by mistake. The merchant sees which substitutions shoppers accept.

Built for the **YouCam API Skin AI & eCommerce VTO Hackathon** (Perfect Corp × Devpost, 2026).

## How to test (for judges)

Live demo: **`<public HTTPS URL — added at deployment>`**. No account, no payment and no photo of yourself is needed.

1. **Shopper flow.** On the home page the favorite is preset to *Oxford Blue Shirt*, size **M**, which is sold out. Keep the $60 budget (optionally add a color or material) and press **Find alternatives**. You get up to three in-stock tops, each with why it fits and what differs.
2. **Try it on with AI.** Press **Take another look** on a card. Choose **Models** and pick Model A, B or C (AI-generated fictional people), then request the preview. A YouCam result usually arrives in 10–30 seconds and is labelled with its provider. Results for the same model and top are reused, so repeat views are instant and spend no credits.
3. **Side by side.** Press **See them side by side** to try every alternative on the same photo and compare.
4. **Choose.** Press **Choose this top**. Stock and price are rechecked on the server before the decision is saved. A saved choice is a demo decision, not a purchase or a stock reservation.
5. **Your own photo (optional).** In the preview dialog, **Upload photo** or **Take a selfie** after ticking the consent box. Use one person, standing, facing the camera, head to waist visible. **Delete application photo copies** removes the app's copies at any time.
6. **Merchant view.** Open **`/merchant`** and press **View demo dashboard** (read-only, no token). It shows demo sessions, accepted substitutions and AI preview usage. Editing stock and the AI switch require the owner token, which is not public.

Every preview is labelled either as a YouCam result or as an illustrated simulator preview. To protect the shared hackathon credits, AI previews are capped per day and per visitor; when a cap is reached the app says so instead of silently switching to an illustration.

## What uses YouCam

| YouCam API | Where | Purpose |
|---|---|---|
| AI Clothes Try-On (`cloth-v4`, `upper_body`) | Shopper preview and side-by-side view | Puts each alternative top on the chosen person photo |
| File upload (`/s2s/v2.0/file`) | Before each try-on | Sends the normalized person and garment images |
| Text-to-image (`youcam-image-v2`) | One-off, `make models-generate` | Created the three fictional demo models, so no real person's likeness is needed |
| Credit balance (`/s2s/v1.0/client/credit`) | Operator check | Read-only remaining-unit check |

Recommendations are deterministic rules, not AI: hard constraints are never relaxed, and the shortlist explains itself. AI is used where it changes a decision, showing how a substitute looks on a person.

Evidence for real provider calls (redacted): [docs/API-VERIFICATION.md](docs/API-VERIFICATION.md).

## Features

- **Constraint-safe shortlist:** stock, size, budget and required color/material are hard rules; at most three results; zero-match state keeps the shopper's inputs and asks for feedback instead of relaxing them.
- **AI try-on from three sources:** AI-generated demo models, a consented upload, or a live selfie cropped to the visible portrait frame. Phone photos are rotated using their EXIF orientation.
- **Side-by-side comparison** of alternatives on the same photo.
- **Stock recheck at choice:** a top that sells out while the shopper compares is rejected with a clear message and a refresh option.
- **Merchant studio:** demo metrics, substitution insight (which sold-out favorites lead to which choices), stock editing, and an **AI previews** switch to save credits. Read-only demo view needs no token.
- **Credit protection:** a generation ledger with daily, total and per-visitor caps, a PostgreSQL advisory lock against races, rate limiting, result reuse, and no automatic retry or fallback after provider errors.
- **Privacy:** consent before any upload; images are re-encoded (metadata stripped), stored privately, readable only by their session, and deleted on request or after 24 hours.

## Architecture

```
swaplook_FE  React 19 · Vite · TypeScript · Tailwind 4
             domain (DTOs) → application (gateway contracts) → infrastructure (fetch) → presentation
swaplook_BE  Go · PostgreSQL 17
             domain (catalog rules, ranking) → application (use cases, ports)
             adapters: postgres · private file storage · YouCam / simulator renderer · HTTP API
             cmd/api (server) · cmd/probe (one-off YouCam check) · cmd/modelgen (demo models)
```

The domain has no HTTP, SQL or provider dependencies. YouCam credentials stay on the server; the browser never sees them. Money is stored as integer USD cents. The Go server can also serve the built frontend from the same origin under a strict content security policy.

More context: [Scope](docs/SCOPE.md), [PRD](docs/PRD.md), [Spec](docs/SPEC.md), [Evaluation plan](docs/EVALUATION.md), [Brand](docs/BRAND.md), [Progress log](docs/PROGRESS.md).

## Run locally

Requirements: Go 1.26+, Node 22.12+, pnpm 11, Docker Compose (or PostgreSQL 17+).

1. Copy `.env.example` to `.env`. Set `MERCHANT_TOKEN` to a random value of at least 24 characters (`openssl rand -hex 32`). Never commit `.env`.
2. `make install`, then `make up`. Open **http://127.0.0.1:8088** (shopper) and **/merchant**. The database listens on loopback port **5548**. `make stop` stops only this project's containers.

Without a YouCam key the app runs in `RENDERER_MODE=simulator` and shows clearly labelled illustrations. To use YouCam:

- Put `YOUCAM_API_KEY` in `.env` (server only, never a `VITE_` variable) and set `RENDERER_MODE=youcam`.
- Run `make prepare-demo-assets` to copy the 12 licensed public catalog photos to `.data/garments` and the three repository-supplied fictional models to `.data/models`. This uses no API units and refuses to overwrite different existing files.
- Optional demo gallery: set `CONTAINER_DEMO_MODELS_DIR=/app/data/models` for Compose, or `DEMO_MODELS_DIR=../.data/models` for the native API. The model PNGs and labelled manifest are included under `demo-assets/models`; generating replacements with `make models-generate` is optional and spends API units.
- Budget caps: `PREVIEW_DAILY_LIMIT` (25), `PREVIEW_TOTAL_LIMIT` (150), `PREVIEW_CLIENT_DAILY_LIMIT` (5).
- `make youcam-check` validates inputs offline; `make youcam-probe` makes exactly one real try-on.

For a Vite dev workflow use `make db`, `make api` and `make web` (http://127.0.0.1:5173). Deployment needs one HTTPS origin, `APP_ORIGIN` set to it exactly, `COOKIE_SECURE=true`, and `TRUST_PROXY=true` only behind a trusted reverse proxy.

## Tests

- `make check`: race-enabled Go tests, Go vet, TypeScript check and React DOM tests.
- `TEST_DATABASE_URL=… make integration-test`: PostgreSQL integration tests in an isolated schema (persistence, stale stock/price rejection, concurrent choices, generation budget, cleanup).
- `E2E_DATABASE_URL=… make e2e`: Playwright browser flows on desktop and mobile against the built server in simulator mode (shortlist, comparison, choice persistence, zero match, stock change during comparison, side-by-side view).

## Honest limits

- The boutique, products, prices, stock and decisions are **synthetic demo data**. Choices are not orders, reservations or revenue.
- A try-on preview illustrates appearance. It cannot guarantee fit, size, or every garment detail.
- Deleting photos removes SwapLook's copies; it cannot promise deletion on the provider side.
- No user study results are claimed yet.

## License and credits

Source code: [MIT](LICENSE). **Catalog photographs are not covered by the MIT license.** They come from Wikimedia Commons under their own licenses (mostly CC BY-SA), modified by cropping, padding and resizing; see [docs/CATALOG-CREDITS.md](docs/CATALOG-CREDITS.md) and the in-app **Photo credits** page (`/credits`). Product names and details in the demo are fictional and do not describe the original items. Demo models are AI-generated fictional people. The SwapLook logo and illustrations are original project work.

## Repository contents

Keep source, tests, migrations, lockfiles, public catalog/brand assets, fictional demo-model inputs, licenses/credits, sanitized configuration examples and submission materials. Private photos, runtime data, credentials, provider receipts/results, dependencies, builds, caches and browser traces are ignored. Run `make repo-audit` before staging; see [repository policy and verification limits](docs/REPOSITORY.md). No API key or merchant token is needed in Git for judges to review the source.

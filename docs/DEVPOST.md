# Devpost submission draft

Draft text for the Devpost form. Replace every `<…>` placeholder before submitting, and recheck each claim against the deployed app on the day. Nothing here has been submitted.

**Project name:** SwapLook

**Tagline (≤ 60 characters):** Sold out? Find an in-stock swap and see it on you.

**Links:** Live demo `<HTTPS URL>` · Code `<GitHub URL>` · Video `<YouTube/Vimeo URL>`

**Built with:** YouCam AI Clothes Try-On API, YouCam AI Image Generator API, Go, PostgreSQL, React, TypeScript, Vite, Tailwind CSS, Docker, Playwright

---

## Inspiration

A shopper finds the top they love, picks their size, and sees "Sold out". For a small boutique that is often the end of the visit. Generic "you may also like" rows don't help: they ignore the shopper's size and budget, and a flat product photo can't answer the real question, *"would this other one look good on me?"* We wanted to turn that dead end into a confident second choice.

## What it does

SwapLook is a substitution assistant for a small online boutique.

1. **Start from the sold-out favorite.** The shopper keeps their size and budget and can mark a color or material as a must-have.
2. **Get at most three honest alternatives.** Stock, size, budget and must-haves are hard rules that are never silently relaxed. Each option says why it fits and what is different ("color: grey instead of blue"). If nothing fits, SwapLook says so and asks for feedback instead of padding the list.
3. **See them on a person with YouCam AI Clothes Try-On.** The shopper picks one of three AI-generated demo models, uploads their own photo with consent, or takes a selfie. **Side by side** puts every alternative on the same person so they can be compared directly.
4. **Choose with confidence.** Stock and price are rechecked on the server at the moment of choice. If the top sold out while the shopper was comparing, they get a clear message and a refresh, never a false success.
5. **The merchant learns from it.** A merchant studio shows which sold-out favorites led to which substitutes, AI preview usage, and stock controls. An **AI previews** switch lets the owner pause paid generations at any time.

## How we built it

- **YouCam AI Clothes Try-On (`cloth-v4`, upper body)** renders each alternative on the chosen person. The Go backend uploads the normalized person and garment images through the YouCam file API, submits the task, and polls with spaced, capped checks.
- **YouCam AI Image Generator (`youcam-image-v2`)** created the three fictional demo models, so judges and shoppers can try the experience without uploading a face and without using any real person's likeness.
- **Recommendations are deterministic, not generative.** Hard constraints and a transparent ranking produce the shortlist and its reasons, so the AI is spent where it changes a decision: showing how a substitute looks on a person.
- **Clean architecture in Go:** a pure domain (catalog rules, ranking), application use cases with ports, and adapters for PostgreSQL, private file storage, the YouCam renderer and HTTP. The React + TypeScript frontend mirrors the same layering.
- **Credit protection for a shared 1,000-unit budget:** a generation ledger with daily, total and per-visitor caps under a PostgreSQL advisory lock; rate limiting; reuse of an existing result for the same model and top; no automatic retry and no silent fallback to an illustration after a provider error.
- **Privacy by default:** consent before upload; images re-encoded to strip metadata, stored privately, readable only by their own session, deleted on request or after 24 hours. YouCam credentials never reach the browser.
- **Tested:** race-enabled Go unit tests, PostgreSQL integration tests, React DOM tests, and Playwright browser flows on desktop and mobile (shortlist, comparison, choice persistence, zero match, stock change during comparison, side-by-side).

## Challenges we ran into

- **Our first two in-app try-ons failed with a pose error even though the same photo worked from the command line.** The cause was our own image normalization: it produced 16-bit PNGs. Re-encoding to 8-bit fixed it, and the next attempt succeeded.
- **Selfies came out landscape** with a second person in them: laptop webcams deliver 16:9 frames while our camera frame is portrait. Captures are now cropped to exactly what the shopper sees, and phone photos are rotated using their EXIF orientation.
- **A limited credit budget for a public demo.** We designed caps, reuse and an owner switch so that judges can always try the real AI without one visitor exhausting it.
- **Licensed garment photos.** We built the demo catalog from openly licensed Wikimedia Commons photographs and credit every one in the app.

## Accomplishments that we're proud of

- A real YouCam try-on inside a complete, constraint-safe shopping flow, not a standalone demo.
- Every preview is labelled honestly: YouCam result, reused result, or illustration.
- The stock-change moment: a top that sells out mid-comparison is caught at the final check.
- No photo of the judge is required to see the AI work.

## What we learned

<Fill in after testing. Draft: input quality decides try-on success more than anything else, so guidance and cropping matter as much as the API call. Clear limits (three options, hard constraints, visible labels) build more trust than more options.>

## What's next for SwapLook

- Run the planned formative study (about five shoppers and one or two sellers) and publish the results honestly.
- Connect to a real store catalog (for example a Shopify product feed) with live stock.
- More garment categories using YouCam's other try-on APIs.
- Use the merchant substitution data to suggest which items to restock.

---

## Screenshots

In `docs/screenshots/` (1600 px wide):

1. `01-home.png`: landing and the sold-out favorite
2. `02-shortlist.png`: three alternatives with reasons and trade-offs
3. `03-preview-sources.png`: try-on dialog with Models / Upload / Selfie
4. `04-merchant.png`: read-only merchant dashboard
5. `05-credits.png`: photo credits page

Still needed: **one screenshot of a real YouCam try-on result on a demo model**, and one of the side-by-side view with AI results. These require generations (about 2 units each) and need owner approval. Never use a screenshot that shows the owner's or a tester's face without their consent.

## Labels to keep honest

- Catalog, prices, stock and decisions are synthetic demo data. Never call choices sales or revenue.
- If any video section is sped up or uses a precomputed or reused result, say so on screen.
- No user-study results are claimed until the study has actually been run.

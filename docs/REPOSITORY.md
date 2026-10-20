# Repository contents and privacy

Reviewed against the [official hackathon rules, Submission Requirements](https://youcam-api-skin-ai-ecommerce.devpost.com/rules) on 20 October 2026. The code repository must include the source, assets and instructions needed to run the project, with the relevant licensing; it may be public or privately shared as the organizer instructs. Ignoring files alone is not proof of eligibility or complete submission readiness.

| Keep in Git | Purpose |
|---|---|
| `swaplook_BE`, `swaplook_FE/src`, frontend configuration | Working source, automated tests, SQL migrations and runtime contracts |
| `go.mod`, `go.sum`, `package.json`, `pnpm-lock.yaml`, workspace file | Reproducible dependencies; dependency folders themselves are excluded |
| `swaplook_FE/public` | Required catalog photos, simulator illustrations, logo, favicon and licensed visual derivatives |
| `demo-assets/models` | Only Models A–C: labelled AI-generated fictional gallery inputs and manifest |
| `LICENSE`, catalog credits, visible in-app credits | Code license and separate photo attribution/license information |
| `.env.example`, Dockerfile, Compose, Makefile, scripts, CI | Sanitized setup and verification instructions/configuration |
| README, project docs, English Devpost draft, curated `docs/screenshots` | Judge instructions, architecture, evidence boundaries and submission material |
| `AGENTS.md` | Shared project rules for future maintenance |

`.gitignore` excludes real environment files and private keys, all `.data` and scratch `tmp` contents, user uploads/selfies, database files/dumps, provider task receipts and generated try-on results, dependencies/builds/caches, Playwright traces/reports, HAR files, test coverage, logs and machine/editor/agent state. These files remain on disk; nothing was deleted to implement this policy.

Do not blanket-ignore PNG/JPG/SVG, JSON, SQL, lockfiles or the entire docs folder: some of these are required runtime assets, migrations, manifests or submission evidence. Curated screenshots are separate from automatic browser recordings, which can contain private data.

## Reproduce the demo without private data

Run `make prepare-demo-assets` after cloning. It copies only the twelve licensed catalog images and three included fictional model images/manifest into `.data/garments` and `.data/models`. It neither calls YouCam nor overwrites different existing files. `python3 scripts/prepare_demo_assets.py --check` validates the plan without copying. Enable the optional model gallery as described in the README; keep operator credentials in server `.env`.

Only fictional model inputs were copied from the existing local model folder into `demo-assets/models`. No real person's photo, generated try-on output, provider receipt or signed URL belongs in that directory. Photo rights and credits remain governed by their individual licenses; `.gitignore` does not change them.

## Check before staging

Run `make repo-audit`. The script:

1. Uses a temporary Git directory outside this project to check actual Git ignore behavior, even before the project has a repository.
2. Verifies sensitive/generated paths are excluded and necessary source/assets/docs remain included.
3. Scans candidate file contents for known credential values from the ignored local `.env`, potential signed cloud URLs and private-key headers, without printing any matching values.
4. If this project has a Git repository, also checks tracked files: adding an ignore rule does not untrack an existing file.

The audit does not stage, commit, push, call a provider, or modify the project's Git metadata. It is a candidate/current-file check, not a complete secret-history scan, OCR scan, general secret detector or third-party license review. On 20 October 2026, no project Git repository existed, so there was no project index/history to inspect. Review new assets and screenshots before making them public.

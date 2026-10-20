.PHONY: install db api web test integration-test build check up stop e2e youcam-check youcam-probe youcam-resume models-generate prepare-demo-assets repo-audit
install:
	cd swaplook_BE && go mod download
	cd swaplook_FE && pnpm install
db:
	docker compose up -d --wait db
up:
	mkdir -p .data/garments
	docker compose --profile app up -d --build --wait
stop:
	docker compose --profile app stop
api:
	bash scripts/api.sh
web:
	cd swaplook_FE && pnpm dev
test:
	cd swaplook_BE && go test -race ./...
	cd swaplook_FE && pnpm test
integration-test:
	@test -n "$$TEST_DATABASE_URL" || (echo 'Set TEST_DATABASE_URL to an accessible disposable PostgreSQL database.' && exit 1)
	cd swaplook_BE && go test -race ./internal/adapters/postgres -v -count=1
build:
	mkdir -p tmp
	cd swaplook_BE && go build -o ../tmp/swaplook-api ./cmd/api
	cd swaplook_FE && pnpm build
check: test
	cd swaplook_BE && go vet ./...
	cd swaplook_FE && pnpm typecheck
e2e: build
	bash scripts/e2e.sh
youcam-check:
	bash scripts/youcam-probe.sh -check
youcam-probe:
	bash scripts/youcam-probe.sh -consent
youcam-resume:
	bash scripts/youcam-probe.sh -resume -consent
# Spends about one YouCam unit per missing synthetic model; skips models already generated.
models-generate:
	bash -c 'set -a; source .env; set +a; cd swaplook_BE && go run ./cmd/modelgen -confirm'
# Reuses only committed demo inputs; no API requests or overwrites.
prepare-demo-assets:
	python3 scripts/prepare_demo_assets.py
repo-audit:
	python3 scripts/audit_repository.py

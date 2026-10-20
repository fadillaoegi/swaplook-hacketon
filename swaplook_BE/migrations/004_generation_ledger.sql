-- Durable record of actual provider generations so credit limits survive session media cleanup.
-- client_key is a salted hash used only for per-client daily limits.
CREATE TABLE generation_ledger (
 id text PRIMARY KEY,
 provider text NOT NULL,
 client_key text NOT NULL DEFAULT '',
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX generation_ledger_provider_time ON generation_ledger(provider, created_at);
INSERT INTO generation_ledger(id,provider,created_at)
 SELECT id, state->>'provider', COALESCE((state->>'created_at')::timestamptz, now())
 FROM render_jobs WHERE state->>'provider' IS NOT NULL AND state->>'provider' <> 'simulator'
 ON CONFLICT DO NOTHING;
INSERT INTO schema_migrations(version) VALUES(4);

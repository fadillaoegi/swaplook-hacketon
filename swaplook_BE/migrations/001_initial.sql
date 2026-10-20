CREATE TABLE IF NOT EXISTS schema_migrations(version integer PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE products (
 id text PRIMARY KEY, name text NOT NULL, price integer NOT NULL CHECK(price>0),
 currency text NOT NULL CHECK(currency='USD'), color text NOT NULL, material text NOT NULL,
 cut text NOT NULL, category text NOT NULL CHECK(category='tops'), image text NOT NULL
);
CREATE TABLE variants (
 product_id text NOT NULL REFERENCES products(id), size text NOT NULL CHECK(size IN ('XS','S','M','L','XL')),
 quantity integer NOT NULL CHECK(quantity BETWEEN 0 AND 10000), PRIMARY KEY(product_id,size)
);
CREATE TABLE sessions (
 id text PRIMARY KEY, state jsonb NOT NULL, expires_at timestamptz NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE photos (
 id text PRIMARY KEY, session_id text NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX photos_session ON photos(session_id);
CREATE TABLE render_jobs (
 id text PRIMARY KEY, session_id text NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
 state jsonb NOT NULL, task_id text NOT NULL DEFAULT ''
);
CREATE INDEX jobs_session ON render_jobs(session_id);
INSERT INTO schema_migrations(version) VALUES(1);

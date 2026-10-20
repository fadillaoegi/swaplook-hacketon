-- Small key/value store for operator settings such as the AI preview switch.
CREATE TABLE settings (
 key text PRIMARY KEY,
 value text NOT NULL,
 updated_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO schema_migrations(version) VALUES(5);

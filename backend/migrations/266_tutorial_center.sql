-- A small ordered catalog, updated atomically to avoid partial publication.
CREATE TABLE IF NOT EXISTS tutorial_center (
    id SMALLINT PRIMARY KEY CHECK (id = 1),
    revision BIGINT NOT NULL DEFAULT 1,
    content JSONB NOT NULL DEFAULT '{"articles":[],"routes":[]}'::jsonb,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
INSERT INTO tutorial_center(id) VALUES(1) ON CONFLICT(id) DO NOTHING;
INSERT INTO settings(key,value) VALUES('tutorial_enabled','false') ON CONFLICT(key) DO NOTHING;

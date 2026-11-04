INSERT INTO settings(key,value) VALUES('voting_enabled','false')
ON CONFLICT(key) DO NOTHING;

CREATE TABLE voting_polls (
    id BIGSERIAL PRIMARY KEY,
    title VARCHAR(200) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    image TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL CHECK(status IN ('draft','open','closed')),
    options JSONB NOT NULL CHECK(jsonb_typeof(options) = 'array' AND jsonb_array_length(options) BETWEEN 2 AND 12),
    revision BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX voting_polls_status_id_idx ON voting_polls(status,id DESC);

CREATE TABLE voting_ballots (
    poll_id BIGINT NOT NULL REFERENCES voting_polls(id),
    user_id BIGINT NOT NULL REFERENCES users(id),
    option_ids JSONB NOT NULL CHECK(jsonb_typeof(option_ids) = 'array' AND jsonb_array_length(option_ids) BETWEEN 1 AND 12),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY(poll_id,user_id)
);

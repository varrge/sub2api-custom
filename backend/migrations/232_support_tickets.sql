CREATE TABLE IF NOT EXISTS support_tickets (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    title VARCHAR(200) NOT NULL,
    category VARCHAR(20) NOT NULL,
    priority VARCHAR(20) NOT NULL DEFAULT 'normal',
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT support_tickets_title_check CHECK (char_length(btrim(title)) BETWEEN 1 AND 200),
    CONSTRAINT support_tickets_category_check CHECK (category IN ('account', 'billing', 'feature', 'other')),
    CONSTRAINT support_tickets_priority_check CHECK (priority IN ('low', 'normal', 'high', 'urgent')),
    CONSTRAINT support_tickets_status_check CHECK (status IN ('pending', 'in_progress', 'closed'))
);

CREATE INDEX IF NOT EXISTS support_tickets_user_updated_id_idx
    ON support_tickets (user_id, updated_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS support_tickets_open_user_idx
    ON support_tickets (user_id) WHERE status <> 'closed';
CREATE INDEX IF NOT EXISTS support_tickets_category_idx ON support_tickets (category);
CREATE INDEX IF NOT EXISTS support_tickets_status_idx ON support_tickets (status);
CREATE INDEX IF NOT EXISTS support_tickets_priority_idx ON support_tickets (priority);
CREATE INDEX IF NOT EXISTS support_tickets_admin_order_idx ON support_tickets (
    (CASE priority WHEN 'urgent' THEN 0 WHEN 'high' THEN 1 WHEN 'normal' THEN 2 WHEN 'low' THEN 3 ELSE 4 END),
    updated_at DESC,
    id DESC
);

CREATE TABLE IF NOT EXISTS support_ticket_messages (
    id BIGSERIAL PRIMARY KEY,
    ticket_id BIGINT NOT NULL REFERENCES support_tickets(id) ON DELETE CASCADE,
    author_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    author_role VARCHAR(20) NOT NULL,
    body TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT support_ticket_messages_author_role_check CHECK (author_role IN ('user', 'admin')),
    CONSTRAINT support_ticket_messages_body_check CHECK (char_length(btrim(body)) BETWEEN 1 AND 10000)
);

CREATE INDEX IF NOT EXISTS support_ticket_messages_ticket_id_idx
    ON support_ticket_messages (ticket_id, id);
CREATE INDEX IF NOT EXISTS support_ticket_messages_unread_idx
    ON support_ticket_messages (ticket_id, author_role, id);
CREATE INDEX IF NOT EXISTS support_ticket_messages_author_user_id_idx
    ON support_ticket_messages (author_user_id);

CREATE TABLE IF NOT EXISTS support_ticket_attachments (
    id BIGSERIAL PRIMARY KEY,
    message_id BIGINT NOT NULL REFERENCES support_ticket_messages(id) ON DELETE CASCADE,
    data BYTEA NOT NULL,
    content_type VARCHAR(100) NOT NULL,
    size_bytes BIGINT NOT NULL,
    width INTEGER,
    height INTEGER,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT support_ticket_attachments_content_type_check CHECK (char_length(btrim(content_type)) > 0),
    CONSTRAINT support_ticket_attachments_size_check CHECK (size_bytes >= 0 AND octet_length(data) = size_bytes),
    CONSTRAINT support_ticket_attachments_width_check CHECK (width IS NULL OR width > 0),
    CONSTRAINT support_ticket_attachments_height_check CHECK (height IS NULL OR height > 0)
);

CREATE INDEX IF NOT EXISTS support_ticket_attachments_message_id_idx
    ON support_ticket_attachments (message_id, id);

CREATE TABLE IF NOT EXISTS support_ticket_reads (
    id BIGSERIAL PRIMARY KEY,
    ticket_id BIGINT NOT NULL REFERENCES support_tickets(id) ON DELETE CASCADE,
    reader_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    reader_role VARCHAR(20) NOT NULL,
    last_read_message_id BIGINT NOT NULL DEFAULT 0,
    read_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT support_ticket_reads_reader_role_check CHECK (reader_role IN ('user', 'admin')),
    CONSTRAINT support_ticket_reads_last_message_check CHECK (last_read_message_id >= 0),
    CONSTRAINT support_ticket_reads_unique_reader UNIQUE (ticket_id, reader_user_id, reader_role)
);

CREATE INDEX IF NOT EXISTS support_ticket_reads_unread_idx
    ON support_ticket_reads (reader_user_id, reader_role, ticket_id, last_read_message_id);
